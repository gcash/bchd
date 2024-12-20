package bchrpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/merkleblock"
	"github.com/simpleledgerinc/goslp"
	"github.com/simpleledgerinc/goslp/v1parser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// maxAddressQuerySize is the max number of addresses
// to return per query.
const maxAddressQuerySize = 10000

var serviceMap = map[string]interface{}{
	"pb.bchrpc": &GrpcServer{},

	"grpc.reflection.v1alpha.ServerReflection": &reflectionServer{},
}

type reflectionServer struct{}

func (s *reflectionServer) checkReady() bool {
	return true
}

// ServiceReady returns nil when the service is ready and a gRPC error when not.
func ServiceReady(service string) error {
	s, ok := serviceMap[service]
	if !ok {
		return status.Errorf(codes.Unimplemented, "service %s not found", service)
	}
	type readyChecker interface {
		checkReady() bool
	}
	ready := true
	r, ok := s.(readyChecker)
	if ok {
		ready = r.checkReady()
	}
	if !ready {
		return status.Errorf(codes.FailedPrecondition, "service %v is not ready", service)
	}
	return nil
}

// NetManager is an interface which provides functions for handling new transactions.
// This is used by the SubmitTransaction RPC to notify the rest of the system a new
// transaction needs to be handled.
type NetManager interface {
	// AddRebroadcastInventory adds 'iv' to the list of inventories to be
	// rebroadcasted at random intervals until they show up in a block.
	AddRebroadcastInventory(iv *wire.InvVect, data interface{})

	// AnnounceNewTransactions generates and relays inventory vectors and notifies
	// both websocket and getblocktemplate long poll clients of the passed
	// transactions.  This function should be called whenever new transactions
	// are added to the mempool.
	AnnounceNewTransactions(txns []*mempool.TxDesc)
}

// GrpcServerConfig hols the various objects needed by the GrpcServer to
// perform its functions.
type GrpcServerConfig struct {
	Server     *grpc.Server
	HTTPServer *http.Server

	TimeSource  blockchain.MedianTimeSource
	Chain       *blockchain.BlockChain
	ChainParams *chaincfg.Params
	DB          database.DB
	TxMemPool   *mempool.TxPool
	NetMgr      NetManager

	TxIndex   *indexers.TxIndex
	AddrIndex *indexers.AddrIndex
	CfIndex   *indexers.CfIndex
	SlpIndex  *indexers.SlpIndex
}

// GrpcServer is the gRPC server implementation. It holds all the objects
// necessary to serve the RPCs and implements the bchrpc.proto interface.
type GrpcServer struct {
	timeSource  blockchain.MedianTimeSource
	chain       *blockchain.BlockChain
	chainParams *chaincfg.Params
	db          database.DB
	txMemPool   *mempool.TxPool
	netMgr      NetManager

	txIndex   *indexers.TxIndex
	addrIndex *indexers.AddrIndex
	cfIndex   *indexers.CfIndex
	slpIndex  *indexers.SlpIndex

	httpServer *http.Server
	subscribe  chan *rpcEventSubscription
	events     chan interface{}
	quit       chan struct{}

	wg       sync.WaitGroup
	ready    uint32 // atomic
	shutdown int32  // atomic
}

// NewGrpcServer returns a new GrpcServer which has not yet
// be started.
func NewGrpcServer(cfg *GrpcServerConfig) *GrpcServer {
	s := &GrpcServer{
		timeSource:  cfg.TimeSource,
		chain:       cfg.Chain,
		chainParams: cfg.ChainParams,
		db:          cfg.DB,
		txMemPool:   cfg.TxMemPool,
		netMgr:      cfg.NetMgr,
		txIndex:     cfg.TxIndex,
		addrIndex:   cfg.AddrIndex,
		cfIndex:     cfg.CfIndex,
		slpIndex:    cfg.SlpIndex,
		httpServer:  cfg.HTTPServer,
		subscribe:   make(chan *rpcEventSubscription),
		events:      make(chan interface{}),
		quit:        make(chan struct{}),
		wg:          sync.WaitGroup{},
	}
	reflection.Register(cfg.Server)
	pb.RegisterBchrpcServer(cfg.Server, s)
	serviceMap["pb.bchrpc"] = s

	// listen to changes in the mempool for adding/removing from slp entry cache
	go s.slpEventHandler()

	return s
}

// rpcEventTxAccepted indicates a new tx was accepted into the mempool.
type rpcEventTxAccepted struct {
	*mempool.TxDesc
}

// rpcEventBlockConnected indicates a new block connected to the current best
// chain.
type rpcEventBlockConnected struct {
	*bchutil.Block
}

// rpcEventBlockDisconnected indicates a block that was disconnected from the
// current best chain.
type rpcEventBlockDisconnected struct {
	*bchutil.Block
}

// rpcEventSubscription represents a subscription to events from the RPC server.
type rpcEventSubscription struct {
	in          chan interface{} // rpc events to be put by the dispatcher
	out         chan interface{} // rpc events to be read by the client
	unsubscribe chan struct{}    // close to unsubscribe
}

// Events returns the channel clients listen to to get new events.
func (s *rpcEventSubscription) Events() <-chan interface{} {
	return s.out
}

// Unsubscribe is to be called by the client to stop the subscription.
func (s *rpcEventSubscription) Unsubscribe() {
	close(s.unsubscribe)
}

// subscribeEvents returns a new subscription to all the events the RPC server
// receives.
func (s *GrpcServer) subscribeEvents() *rpcEventSubscription {
	sub := &rpcEventSubscription{
		in:          make(chan interface{}),
		out:         make(chan interface{}),
		unsubscribe: make(chan struct{}),
	}

	// Start a queue handler for the subscription so that slow connections don't
	// hold up faster ones.
	s.wg.Add(1)
	go func() {
		queueHandler(sub.in, sub.out, s.quit)
		s.wg.Done()
	}()

	select {
	case s.subscribe <- sub:
	case <-s.quit:
	}
	return sub
}

// runEventDispatcher runs a process that will forward new incoming events to
// all the currently active client processes.
//
// It should be run in a goroutine and calls Done on the wait group on finish.
func (s *GrpcServer) runEventDispatcher() {
	defer s.wg.Done()

	subscriptions := make(map[*rpcEventSubscription]struct{})
	for {
		select {
		case newSub := <-s.subscribe:
			subscriptions[newSub] = struct{}{}

		case event := <-s.events:
			// Dispatch to all clients.
			for sub := range subscriptions {
				select {
				case sub.in <- event:

				case <-sub.unsubscribe:
					// If client unsubscribed, just delete it.
					delete(subscriptions, sub)
				}
			}

		case <-s.quit:
			for sub := range subscriptions {
				close(sub.in)
			}
			return
		}
	}
}

// dispatchEvent dispatches an event and makes sure it doesn't block when the
// server is shutting down.
func (s *GrpcServer) dispatchEvent(event interface{}) {
	select {
	case s.events <- event:
	case <-s.quit:
	}
}

// NotifyNewTransactions is called by the server when new transactions
// are accepted in the mempool.
func (s *GrpcServer) NotifyNewTransactions(txs []*mempool.TxDesc) {
	for _, txDesc := range txs {
		s.dispatchEvent(&rpcEventTxAccepted{txDesc})
	}
}

// handleBlockchainNotification handles the callback from the blockchain package
// that notifies the RPC server about changes in the chain.
func (s *GrpcServer) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {

	case blockchain.NTBlockConnected:
		block, ok := notification.Data.(*bchutil.Block)
		if !ok {
			log.Warnf("Chain connected notification is not a block.")
			break
		}
		s.dispatchEvent(&rpcEventBlockConnected{block})

	case blockchain.NTBlockDisconnected:
		block, ok := notification.Data.(*bchutil.Block)
		if !ok {
			log.Warnf("Chain disconnected notification is not a block.")
			break
		}
		s.dispatchEvent(&rpcEventBlockDisconnected{block})
	}
}

// Start will start the GrpcServer, subscribe to blockchain notifications
// and start the EventDispatcher in a new goroutine.
func (s *GrpcServer) Start() {
	if atomic.SwapUint32(&s.ready, 1) != 0 {
		panic("service already started")
	}

	s.wg.Add(1)
	s.chain.Subscribe(s.handleBlockchainNotification)
	go s.runEventDispatcher()
}

// Stop is used by server.go to stop the gRPC listener.
func (s *GrpcServer) Stop() error {
	if atomic.AddInt32(&s.shutdown, 1) != 1 {
		log.Infof("gRPC server is already in the process of shutting down")
		return nil
	}
	log.Warnf("gRPC server shutting down")
	err := s.httpServer.Close()
	if err != nil {
		log.Errorf("Problem shutting down grpc: %v", err)
		return err
	}
	close(s.quit)
	s.wg.Wait()
	log.Infof("gRPC server shutdown complete")
	return nil
}

// checkReady returns if the server is ready to serve data.
func (s *GrpcServer) checkReady() bool {
	return atomic.LoadUint32(&s.ready) != 0
}

// GetMempoolInfo returns the state of the current mempool.
func (s *GrpcServer) GetMempoolInfo(ctx context.Context, req *pb.GetMempoolInfoRequest) (*pb.GetMempoolInfoResponse, error) {
	nBytes := uint32(0)
	for _, txDesc := range s.txMemPool.TxDescs() {
		nBytes += uint32(txDesc.Tx.MsgTx().SerializeSize())
	}
	resp := &pb.GetMempoolInfoResponse{
		Size:  uint32(s.txMemPool.Count()),
		Bytes: nBytes,
	}
	return resp, nil
}

// GetMempool returns information about all of the transactions currently in the memory pool.
// Offers an option to return full transactions or just transactions hashes.
func (s *GrpcServer) GetMempool(ctx context.Context, req *pb.GetMempoolRequest) (*pb.GetMempoolResponse, error) {
	rawMempool := s.txMemPool.MiningDescs()
	resp := &pb.GetMempoolResponse{}
	for _, txDesc := range rawMempool {
		if req.FullTransactions {
			respTx := marshalTransaction(txDesc.Tx, 0, nil, 0, s)
			stxos, err := s.txMemPool.FetchInputUtxos(txDesc.Tx)
			if err != nil {
				continue
			}
			for i, in := range txDesc.Tx.MsgTx().TxIn {
				entry := stxos.LookupEntry(in.PreviousOutPoint)
				if entry != nil {

					pk, cashToken, err := getTokenDataForInputIfExists(entry.PkScript())
					if err != nil {
						log.Debugf("could not parse token data for %v index: %v", txDesc.Tx.Hash(), uint32(i))
					}
					if err != nil || cashToken == nil {
						pk = entry.PkScript()
					} else { // cash token data exists.
						respTx.Inputs[i].CashToken = cashToken
					}

					respTx.Inputs[i].Value = entry.Amount()
					respTx.Inputs[i].PreviousScript = pk

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
					if err == nil && len(addrs) > 0 {
						respTx.Inputs[i].Address = addrs[0].String()
					}
				}
			}

			resp.TransactionData = append(resp.TransactionData, &pb.GetMempoolResponse_TransactionData{
				TxidsOrTxs: &pb.GetMempoolResponse_TransactionData_Transaction{
					Transaction: respTx,
				},
			})
		} else {
			resp.TransactionData = append(resp.TransactionData, &pb.GetMempoolResponse_TransactionData{
				TxidsOrTxs: &pb.GetMempoolResponse_TransactionData_TransactionHash{
					TransactionHash: txDesc.Tx.Hash().CloneBytes(),
				},
			})
		}
	}
	return resp, nil
}

// GetBlockchainInfo returns info about the blockchain including the most recent
// block hash and height.
func (s *GrpcServer) GetBlockchainInfo(ctx context.Context, req *pb.GetBlockchainInfoRequest) (*pb.GetBlockchainInfoResponse, error) {
	bestSnapShot := s.chain.BestSnapshot()

	var net pb.GetBlockchainInfoResponse_BitcoinNet
	switch s.chainParams {
	case &chaincfg.MainNetParams:
		net = pb.GetBlockchainInfoResponse_MAINNET
	case &chaincfg.TestNet3Params:
		net = pb.GetBlockchainInfoResponse_TESTNET3
	case &chaincfg.TestNet4Params:
		net = pb.GetBlockchainInfoResponse_TESTNET4
	case &chaincfg.RegressionNetParams:
		net = pb.GetBlockchainInfoResponse_REGTEST
	case &chaincfg.SimNetParams:
		net = pb.GetBlockchainInfoResponse_SIMNET
	default:
		return nil, status.Error(codes.Internal, "unknown network parameters")
	}

	gsEnabled := false
	if s.slpIndex != nil {
		gsEnabled = s.slpIndex.GraphSearchEnabled()
	}

	resp := &pb.GetBlockchainInfoResponse{
		AddrIndex:      s.addrIndex != nil,
		TxIndex:        s.txIndex != nil,
		SlpIndex:       s.slpIndex != nil,
		SlpGraphsearch: gsEnabled,
		BestHeight:     bestSnapShot.Height,
		BestBlockHash:  bestSnapShot.Hash[:],
		BitcoinNet:     net,
		Difficulty:     getDifficultyRatio(bestSnapShot.Bits, s.chainParams),
		MedianTime:     bestSnapShot.MedianTime.Unix(),
	}
	return resp, nil
}

// GetBlockInfo returns metadata and info for a specified block.
func (s *GrpcServer) GetBlockInfo(ctx context.Context, req *pb.GetBlockInfoRequest) (*pb.GetBlockInfoResponse, error) {
	var (
		block *bchutil.Block
		err   error
	)
	if len(req.GetHash()) == 0 {
		block, err = s.chain.BlockByHeight(req.GetHeight())
	} else {
		h, err := chainhash.NewHash(req.GetHash())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid hash: %v", err)
		}
		block, err = s.chain.BlockByHash(h)
	}
	if err != nil || block == nil {
		return nil, status.Error(codes.NotFound, "block not found")
	}

	medianTime, err := s.chain.MedianTimeByHash(block.Hash())
	if err != nil {
		return nil, status.Error(codes.Internal, "error calculating median time for block")
	}

	resp := &pb.GetBlockInfoResponse{
		Info: marshalBlockInfo(block, s.chain.BestSnapshot().Height-block.Height()+1, medianTime, s.chainParams),
	}

	nextHeader, err := s.chain.HeaderByHeight(block.Height() + 1)
	if err == nil {
		nextHash := nextHeader.BlockHash()
		resp.Info.NextBlockHash = nextHash.CloneBytes()
	}

	return resp, nil
}

// GetBlock returns detailed data for a block.
func (s *GrpcServer) GetBlock(ctx context.Context, req *pb.GetBlockRequest) (*pb.GetBlockResponse, error) {
	var (
		block *bchutil.Block
		err   error
	)
	if len(req.GetHash()) == 0 {
		block, err = s.chain.BlockByHeight(req.GetHeight())
	} else {
		h, err := chainhash.NewHash(req.GetHash())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid hash: %v", err)
		}
		block, err = s.chain.BlockByHash(h)
	}
	if err != nil || block == nil {
		return nil, status.Error(codes.NotFound, "block not found")
	}

	confirmations := s.chain.BestSnapshot().Height - block.Height() + 1
	medianTime, err := s.chain.MedianTimeByHash(block.Hash())
	if err != nil {
		return nil, status.Error(codes.Internal, "error calculating median time for block")
	}
	resp := &pb.GetBlockResponse{
		Block: &pb.Block{
			Info: marshalBlockInfo(block, confirmations, medianTime, s.chainParams),
		},
	}

	nextHeader, err := s.chain.HeaderByHeight(block.Height() + 1)
	if err == nil {
		nextHash := nextHeader.BlockHash()
		resp.Block.Info.NextBlockHash = nextHash.CloneBytes()
	}

	var spentTxos []blockchain.SpentTxOut
	if req.FullTransactions {
		spentTxos, err = s.chain.FetchSpendJournal(block)
		if err != nil {
			return nil, status.Error(codes.Internal, "error loading spend journal")
		}
	}
	spendIdx := 0
	for idx, tx := range block.Transactions() {
		if req.FullTransactions {
			header := block.MsgBlock().Header
			respTx := marshalTransaction(tx, confirmations, &header, block.Height(), s)
			for i := range tx.MsgTx().TxIn {
				if idx > 0 {
					stxo := spentTxos[spendIdx]

					pk, cashToken, err := getTokenDataForInputIfExists(stxo.PkScript)
					if err != nil {
						log.Debugf("could not parse token data for %v index: %v", tx.Hash(), uint32(i))
					}
					if err != nil || cashToken == nil {
						pk = stxo.PkScript
					} else { // cash token data exists.
						respTx.Inputs[i].CashToken = cashToken
					}

					respTx.Inputs[i].Value = stxo.Amount
					respTx.Inputs[i].PreviousScript = pk

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
					if err == nil && len(addrs) > 0 {
						respTx.Inputs[i].Address = addrs[0].String()
						s.setInputSlpTokenAddress(respTx.Inputs[i], addrs[0])
					}

					spendIdx++
				}
			}

			resp.Block.TransactionData = append(resp.Block.TransactionData, &pb.Block_TransactionData{
				TxidsOrTxs: &pb.Block_TransactionData_Transaction{
					Transaction: respTx,
				},
			})
		} else {
			resp.Block.TransactionData = append(resp.Block.TransactionData, &pb.Block_TransactionData{
				TxidsOrTxs: &pb.Block_TransactionData_TransactionHash{
					TransactionHash: tx.Hash().CloneBytes(),
				},
			})
		}
	}
	return resp, nil
}

// GetRawBlock returns a block in a serialized format.
func (s *GrpcServer) GetRawBlock(ctx context.Context, req *pb.GetRawBlockRequest) (*pb.GetRawBlockResponse, error) {
	var (
		block *bchutil.Block
		err   error
	)
	if len(req.GetHash()) == 0 {
		block, err = s.chain.BlockByHeight(req.GetHeight())
	} else {
		h, err := chainhash.NewHash(req.GetHash())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid hash: %v", err)
		}
		block, err = s.chain.BlockByHash(h)
	}
	if err != nil || block == nil {
		return nil, status.Error(codes.NotFound, "block not found")
	}

	var buf bytes.Buffer
	if err := block.MsgBlock().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		return nil, status.Error(codes.Internal, "block serialization error")
	}
	resp := &pb.GetRawBlockResponse{
		Block: buf.Bytes(),
	}
	return resp, nil
}

// GetBlockFilter returns the compact filter (cf) of a block as a Golomb-Rice encoded set.
//
// **Requires CfIndex**
func (s *GrpcServer) GetBlockFilter(ctx context.Context, req *pb.GetBlockFilterRequest) (*pb.GetBlockFilterResponse, error) {
	if s.cfIndex == nil {
		return nil, status.Error(codes.Unavailable, "cfindex required")
	}

	var (
		blockHash *chainhash.Hash
		err       error
	)
	if len(req.GetHash()) == 0 {
		blockHash, err = s.chain.BlockHashByHeight(req.GetHeight())
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "block not found at height %d", req.GetHeight())
		}
	} else {
		blockHash, err = chainhash.NewHash(req.GetHash())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid hash: %v", err)
		}
	}
	filter, err := s.cfIndex.FilterByBlockHash(blockHash, wire.GCSFilterRegular)
	if err != nil {
		return nil, status.Error(codes.NotFound, "filter not found")
	}

	resp := &pb.GetBlockFilterResponse{
		Filter: filter,
	}
	return resp, nil
}

// GetHeaders takes a block locator object and returns a batch of no more than 2000
// headers. Upon parsing the block locator, if the server concludes there has been a
// fork, it will send headers starting at the fork point, or genesis if no blocks in
// the locator are in the best chain. If the locator is already at the tip no headers
// will be returned.
// see: bchd/bchrpc/documentation/wallet_operation.md
func (s *GrpcServer) GetHeaders(ctx context.Context, req *pb.GetHeadersRequest) (*pb.GetHeadersResponse, error) {
	var (
		locator blockchain.BlockLocator
		err     error
	)
	for _, b := range req.BlockLocatorHashes {
		blockHash, err := chainhash.NewHash(b)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid locator hash")
		}
		locator = append(locator, blockHash)
	}
	var stopHash chainhash.Hash
	if len(req.StopHash) > 0 {
		hash, err := chainhash.NewHash(req.StopHash)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid stop hash %v", err)
		}
		stopHash = *hash
	}

	headers := s.chain.LocateHeaders(locator, &stopHash)
	resp := &pb.GetHeadersResponse{}

	var startHeight int32
	if len(headers) > 0 {
		startHash := headers[0].BlockHash()
		startHeight, err = s.chain.BlockHeightByHash(&startHash)
		if err != nil {
			return nil, status.Error(codes.Internal, "error loading start header height")
		}
	}
	bestHeight := s.chain.BestSnapshot().Height
	for i, header := range headers {
		hash := header.BlockHash()
		resp.Headers = append(resp.Headers, &pb.BlockInfo{
			Difficulty:    getDifficultyRatio(header.Bits, s.chainParams),
			Hash:          hash.CloneBytes(),
			Height:        startHeight + int32(i),
			Version:       header.Version,
			Timestamp:     header.Timestamp.Unix(),
			MerkleRoot:    header.MerkleRoot.CloneBytes(),
			Nonce:         header.Nonce,
			Bits:          header.Bits,
			PreviousBlock: header.PrevBlock.CloneBytes(),
			Confirmations: bestHeight - (startHeight + int32(i)) + 1,
		})
	}

	return resp, nil
}

// GetTransaction returns a transaction given its hash.
//
// **Requires TxIndex**
// **Requires SlpIndex for all token metadata
func (s *GrpcServer) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	if s.txIndex == nil {
		return nil, status.Error(codes.Unavailable, "txindex required")
	}

	if req.IncludeTokenMetadata && s.slpIndex == nil {
		return nil, status.Error(codes.Unavailable, "slpindex required")
	}

	txHash, err := chainhash.NewHash(req.Hash)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction hash %v", err)
	}

	if txDesc, err := s.txMemPool.FetchTxDesc(txHash); err == nil {
		tx := marshalTransaction(txDesc.Tx, 0, nil, 0, s)
		tx.Timestamp = txDesc.Added.Unix()

		view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx)
		if err == nil {
			for i, in := range txDesc.Tx.MsgTx().TxIn {
				stxo := view.LookupEntry(in.PreviousOutPoint)
				if stxo != nil {

					pk, cashToken, err := getTokenDataForInputIfExists(stxo.PkScript())
					if err != nil {
						log.Debugf("could not parse token data for %v index: %v", txDesc.Tx.Hash(), uint32(i))
					}
					if err != nil || cashToken == nil {
						pk = stxo.PkScript()
					} else { // cash token data exists.
						tx.Inputs[i].CashToken = cashToken
					}

					tx.Inputs[i].Value = stxo.Amount()
					tx.Inputs[i].PreviousScript = pk

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
					if err == nil && len(addrs) > 0 {
						tx.Inputs[i].Address = addrs[0].String()
						s.setInputSlpTokenAddress(tx.Inputs[i], addrs[0])
					}
				}
			}
		}

		var tokenMetadata *pb.SlpTokenMetadata
		if req.IncludeTokenMetadata && tx.SlpTransactionInfo.ValidityJudgement == pb.SlpTransactionInfo_VALID {
			tokenID, err := chainhash.NewHash(tx.SlpTransactionInfo.TokenId)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "an unknown problem occurred when parsing token id: %s: %v", hex.EncodeToString(tx.SlpTransactionInfo.TokenId), err)
			}
			tokenMetadata, err = s.marshalTokenMetadata(*tokenID)
			if err != nil {
				msg := fmt.Sprintf("an unknown problem occurred when building token metadata for token id %s: %v", hex.EncodeToString(tx.SlpTransactionInfo.TokenId), err)
				log.Criticalf(msg)
				return nil, status.Error(codes.Internal, msg)
			}
		}

		resp := &pb.GetTransactionResponse{
			Transaction:   tx,
			TokenMetadata: tokenMetadata,
		}
		return resp, nil
	}

	txBytes, blockHeight, blockHash, err := s.fetchTransactionFromBlock(txHash)
	if err != nil {
		return nil, err
	}

	// Deserialize the transaction
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(txBytes))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deserialize transaction: %v", err)
	}

	header, err := s.chain.HeaderByHash(blockHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load block header %v", err)
	}

	respTx := marshalTransaction(bchutil.NewTx(&msgTx), s.chain.BestSnapshot().Height-blockHeight+1, &header, blockHeight, s)
	if s.txIndex != nil {
		if err := s.setInputMetadata(respTx); err != nil {
			return nil, err
		}
	}

	var tokenMetadata *pb.SlpTokenMetadata
	if req.IncludeTokenMetadata && respTx.SlpTransactionInfo.ValidityJudgement == pb.SlpTransactionInfo_VALID {
		tokenID, err := chainhash.NewHash(respTx.SlpTransactionInfo.TokenId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "an unknown problem occurred when parsing token id %s: %v", hex.EncodeToString(respTx.SlpTransactionInfo.TokenId), err)
		}
		tokenMetadata, err = s.marshalTokenMetadata(*tokenID)
		if err != nil {
			msg := fmt.Sprintf("an unknown problem occurred when building token metadata for token id %s: %v", hex.EncodeToString(respTx.SlpTransactionInfo.TokenId), err)
			log.Criticalf(msg)
			return nil, status.Error(codes.Internal, msg)
		}
	}

	resp := &pb.GetTransactionResponse{
		Transaction:   respTx,
		TokenMetadata: tokenMetadata,
	}

	return resp, nil
}

// GetRawTransaction returns a serialized transaction given a transaction hash.
//
// **Requires TxIndex**
func (s *GrpcServer) GetRawTransaction(ctx context.Context, req *pb.GetRawTransactionRequest) (*pb.GetRawTransactionResponse, error) {
	if s.txIndex == nil {
		return nil, status.Error(codes.Unavailable, "txindex required")
	}

	txHash, err := chainhash.NewHash(req.Hash)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction hash: %v", err)
	}

	if tx, err := s.txMemPool.FetchTransaction(txHash); err == nil {
		var buf bytes.Buffer
		if err := tx.MsgTx().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
			return nil, status.Error(codes.Internal, "error serializing transaction")
		}
		resp := &pb.GetRawTransactionResponse{
			Transaction: buf.Bytes(),
		}
		return resp, nil
	}

	txBytes, _, _, err := s.fetchTransactionFromBlock(txHash)
	if err != nil {
		return nil, err
	}

	resp := &pb.GetRawTransactionResponse{
		Transaction: txBytes,
	}

	return resp, nil
}

// GetAddressTransactions returns the transactions for the given address. Offers offset,
// limit, and from block options.
//
// **Requires AddressIndex**
func (s *GrpcServer) GetAddressTransactions(ctx context.Context, req *pb.GetAddressTransactionsRequest) (*pb.GetAddressTransactionsResponse, error) {
	if s.addrIndex == nil {
		return nil, status.Error(codes.Unavailable, "addrindex required")
	}

	if req.NbFetch > maxAddressQuerySize {
		return nil, status.Errorf(codes.InvalidArgument, "nbfetch exceeds max of %d", maxAddressQuerySize)
	}

	// Attempt to decode the supplied address.
	addr, err := bchutil.DecodeAddress(req.Address, s.chainParams)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

	// use cash address format
	addr, err = bchutil.ConvertSlpToCashAddress(addr, s.chainParams)
	if err != nil {
		return nil, status.Error(codes.Internal, "couldn't convert address to cash address format")
	}

	startHeight := int32(0)
	if len(req.GetHash()) == 0 {
		startHeight = req.GetHeight()
	} else {
		h, err := chainhash.NewHash(req.GetHash())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid hash: %v", err)
		}
		// If error here we'll just use the genesis
		startHeight, _ = s.chain.BlockHeightByHash(h)
	}

	confirmedTxs, err := s.fetchTransactionsByAddress(addr, startHeight, int(req.NbFetch), int(req.NbSkip))
	if err != nil {
		return nil, err
	}

	resp := &pb.GetAddressTransactionsResponse{}

	tip := s.chain.BestSnapshot().Height
	for _, cTx := range confirmedTxs {
		tx := marshalTransaction(bchutil.NewTx(&cTx.tx), tip-cTx.blockHeight+1, cTx.blockHeader, cTx.blockHeight, s)
		if s.txIndex != nil {
			if err := s.setInputMetadata(tx); err != nil {
				return nil, err
			}
		}
		resp.ConfirmedTransactions = append(resp.ConfirmedTransactions, tx)
	}

	unconfirmedTxs := s.addrIndex.UnconfirmedTxnsForAddress(addr)
	for _, uTx := range unconfirmedTxs {
		tx := marshalTransaction(uTx, 0, nil, 0, s)
		txDesc, err := s.txMemPool.FetchTxDesc(uTx.Hash())
		if err != nil {
			continue
		}
		view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx)
		if err == nil {
			for i, in := range txDesc.Tx.MsgTx().TxIn {
				stxo := view.LookupEntry(in.PreviousOutPoint)
				if stxo != nil {

					pk, cashToken, err := getTokenDataForInputIfExists(stxo.PkScript())
					if err != nil {
						log.Debugf("could not parse token data for %v index: %v", txDesc.Tx.Hash(), uint32(i))
					}
					if err != nil || cashToken == nil {
						pk = stxo.PkScript()
					} else { // cash token data exists.
						tx.Inputs[i].CashToken = cashToken
					}

					tx.Inputs[i].Value = stxo.Amount()
					tx.Inputs[i].PreviousScript = pk

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
					if err == nil && len(addrs) > 0 {
						tx.Inputs[i].Address = addrs[0].String()
						s.setInputSlpTokenAddress(tx.Inputs[i], addrs[0])
					}
				}
			}
		}
		mempoolTx := &pb.MempoolTransaction{
			Transaction:      tx,
			Fee:              txDesc.Fee,
			AddedTime:        txDesc.Added.Unix(),
			AddedHeight:      txDesc.Height,
			FeePerKb:         txDesc.Fee / int64(uTx.MsgTx().SerializeSize()),
			StartingPriority: txDesc.StartingPriority,
		}
		resp.UnconfirmedTransactions = append(resp.UnconfirmedTransactions, mempoolTx)
	}

	return resp, nil
}

// GetRawAddressTransactions returns the raw transactions for the given address. Offers offset,
// limit, and from block options.
//
// **Requires AddressIndex**
func (s *GrpcServer) GetRawAddressTransactions(ctx context.Context, req *pb.GetRawAddressTransactionsRequest) (*pb.GetRawAddressTransactionsResponse, error) {
	if s.addrIndex == nil {
		return nil, status.Error(codes.Unavailable, "addrindex required")
	}

	if req.NbFetch > maxAddressQuerySize {
		return nil, status.Errorf(codes.InvalidArgument, "nbfetch exceeds max of %d", maxAddressQuerySize)
	}

	// Attempt to decode the supplied address.
	addr, err := bchutil.DecodeAddress(req.Address, s.chainParams)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

	startHeight := int32(0)
	if len(req.GetHash()) == 0 {
		startHeight = req.GetHeight()
	} else {
		h, err := chainhash.NewHash(req.GetHash())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid hash: %v", err)
		}
		// If error here we'll just use the genesis
		startHeight, _ = s.chain.BlockHeightByHash(h)
	}

	confirmedTxs, err := s.fetchTransactionsByAddress(addr, startHeight, int(req.NbFetch), int(req.NbSkip))
	if err != nil {
		return nil, err
	}

	resp := &pb.GetRawAddressTransactionsResponse{}

	for _, cTx := range confirmedTxs {
		resp.ConfirmedTransactions = append(resp.ConfirmedTransactions, cTx.txBytes)
	}

	unconfirmedTxs := s.addrIndex.UnconfirmedTxnsForAddress(addr)
	for _, uTx := range unconfirmedTxs {
		var buf bytes.Buffer
		if err := uTx.MsgTx().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
			return nil, status.Error(codes.Internal, "error serializing mempool transaction")
		}
		resp.UnconfirmedTransactions = append(resp.UnconfirmedTransactions, buf.Bytes())
	}

	return resp, nil
}

// GetAddressUnspentOutputs returns all the unspent transaction outputs
// for the given address.
//
// **Requires AddressIndex**
func (s *GrpcServer) GetAddressUnspentOutputs(ctx context.Context, req *pb.GetAddressUnspentOutputsRequest) (*pb.GetAddressUnspentOutputsResponse, error) {
	if s.addrIndex == nil {
		return nil, status.Error(codes.Unavailable, "addrindex required")
	}

	if req.IncludeTokenMetadata && s.slpIndex == nil {
		return nil, status.Error(codes.Unavailable, "slpindex required")
	}

	// Attempt to decode the supplied address.
	addr, err := bchutil.DecodeAddress(req.Address, s.chainParams)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

	// use cash address format
	addr, err = bchutil.ConvertSlpToCashAddress(addr, s.chainParams)
	if err != nil {
		return nil, status.Error(codes.Internal, "couldn't convert address to cash address format")
	}

	tokenMetadataSet := make(map[chainhash.Hash]struct{})
	checkTxOutputs := func(tx *wire.MsgTx) ([]*pb.UnspentOutput, error) {
		txHash := tx.TxHash()
		var (
			utxoView *blockchain.UtxoViewpoint
			utxos    []*pb.UnspentOutput
		)
		if req.IncludeMempool {
			utxoView, err = s.txMemPool.FetchUtxoView(bchutil.NewTx(tx))
			if err != nil {
				return nil, err
			}
		} else {
			utxoView, err = s.chain.FetchUtxoView(bchutil.NewTx(tx))
			if err != nil {
				return nil, err
			}
		}
		entries := utxoView.Entries()
		for i, out := range tx.TxOut {
			op := wire.NewOutPoint(&txHash, uint32(i))
			entry := entries[*op]
			if entry == nil || entry.IsSpent() {
				continue
			}
			pkScript := make([]byte, len(out.PkScript))
			copy(pkScript, out.PkScript)

			_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript, s.chainParams)
			if err != nil || len(addrs) == 0 {
				continue
			}

			var slpToken *pb.SlpToken
			if s.slpIndex != nil {
				slpToken, _ = s.getSlpToken(&txHash, uint32(i), out.PkScript)
				if req.IncludeTokenMetadata && slpToken != nil {
					hash, err := chainhash.NewHash(slpToken.TokenId)
					if err != nil {
						return nil, status.Errorf(codes.Internal, "failed to parse token id: %s: %v", hex.EncodeToString(slpToken.TokenId), err)
					}
					tokenMetadataSet[*hash] = struct{}{}
				}
			}

			matchAddr := ""

			switch typedAddr := addrs[0].(type) {
			case *bchutil.AddressPubKeyHash, *bchutil.AddressScriptHash, *bchutil.AddressScriptHash32:
				matchAddr = addrs[0].EncodeAddress()

			case *bchutil.AddressPubKey:
				matchAddr = typedAddr.AddressPubKeyHash().EncodeAddress()
			}

			if matchAddr == addr.EncodeAddress() {
				utxo := &pb.UnspentOutput{
					Outpoint: &pb.Transaction_Input_Outpoint{
						Hash:  txHash.CloneBytes(),
						Index: uint32(i),
					},
					Value:        entry.Amount(),
					PubkeyScript: pkScript,
					IsCoinbase:   entry.IsCoinBase(),
					BlockHeight:  entry.BlockHeight(),
					SlpToken:     slpToken,
				}
				utxos = append(utxos, utxo)
			}
		}
		return utxos, nil
	}

	var (
		utxos []*pb.UnspentOutput
		skip  = 0
		fetch = 10000
	)
	for {
		if atomic.LoadInt32(&s.shutdown) > 0 {
			return nil, status.Error(codes.Canceled, "canceled by server")
		}
		confirmedTxs, err := s.fetchTransactionsByAddress(addr, 0, fetch, skip)
		if err != nil {
			return nil, err
		}
		if len(confirmedTxs) == 0 {
			break
		}
		for _, ret := range confirmedTxs {
			u, err := checkTxOutputs(&ret.tx)
			if err != nil {
				return nil, err
			}
			if len(u) > 0 {
				utxos = append(utxos, u...)
			}
		}
		skip += len(confirmedTxs)
	}
	if req.IncludeMempool {
		unconfirmedTxs := s.addrIndex.UnconfirmedTxnsForAddress(addr)
		for _, tx := range unconfirmedTxs {
			u, err := checkTxOutputs(tx.MsgTx())
			if err != nil {
				return nil, err
			}
			if len(u) > 0 {
				utxos = append(utxos, u...)
			}
		}
	}

	var tokenMetadata []*pb.SlpTokenMetadata
	if req.IncludeTokenMetadata && s.slpIndex != nil {
		tokenMetadata = make([]*pb.SlpTokenMetadata, 0)
		for hash := range tokenMetadataSet {
			tm, err := s.marshalTokenMetadata(hash)
			if err != nil {
				log.Debugf("Could not build slp token metadata for %v", hash)
			}
			if tm != nil && err == nil {
				tokenMetadata = append(tokenMetadata, tm)
			}
		}
	}

	resp := &pb.GetAddressUnspentOutputsResponse{
		Outputs:       utxos,
		TokenMetadata: tokenMetadata,
	}
	return resp, nil
}

// GetUnspentOutput takes an unspent output in the utxo set and returns
// the utxo metadata or not found.
func (s *GrpcServer) GetUnspentOutput(ctx context.Context, req *pb.GetUnspentOutputRequest) (*pb.GetUnspentOutputResponse, error) {

	if req.IncludeTokenMetadata && s.slpIndex == nil {
		return nil, status.Error(codes.Unavailable, "slpindex required")
	}

	txnHash, err := chainhash.NewHash(req.Hash)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction hash %v", err)
	}

	var (
		op             = wire.NewOutPoint(txnHash, req.Index)
		value          int64
		blockHeight    int32
		scriptPubkey   []byte
		coinbase       bool
		isSlpInMempool = false
	)
	if req.IncludeMempool && s.txMemPool.HaveTransaction(txnHash) {
		tx, err := s.txMemPool.FetchTransaction(txnHash)
		if err != nil {
			return nil, status.Error(codes.NotFound, "utxo not found")
		}
		if req.Index > uint32(len(tx.MsgTx().TxOut)) {
			return nil, status.Error(codes.InvalidArgument, "prev index greater than len outputs")
		}
		spendingTx := s.txMemPool.CheckSpend(*op)
		if spendingTx != nil {
			return nil, status.Error(codes.NotFound, "utxo spent in mempool")
		}
		value = tx.MsgTx().TxOut[req.Index].Value
		blockHeight = mining.UnminedHeight
		scriptPubkey = tx.MsgTx().TxOut[req.Index].PkScript
		coinbase = blockchain.IsCoinBase(tx)

		// check if this txn is possibly an slp transaction
		if len(tx.MsgTx().TxOut) > 0 {
			_, err = v1parser.ParseSLP(tx.MsgTx().TxOut[0].PkScript)
			if err == nil {
				isSlpInMempool = true
			}
		}
	} else {
		if req.IncludeMempool {
			spendingTx := s.txMemPool.CheckSpend(*op)
			if spendingTx != nil {
				return nil, status.Error(codes.NotFound, "utxo spent in mempool")
			}
		}
		entry, err := s.chain.FetchUtxoEntry(*op)
		if err != nil {
			return nil, err
		}
		if entry == nil || entry.IsSpent() {
			return nil, status.Error(codes.NotFound, "utxo not found")
		}

		value = entry.Amount()
		blockHeight = entry.BlockHeight()
		scriptPubkey = entry.PkScript()
		coinbase = entry.IsCoinBase()
	}

	var (
		slpToken *pb.SlpToken
		tm       *pb.SlpTokenMetadata
	)
	if s.slpIndex != nil && req.Index > 0 && isSlpInMempool && req.IncludeMempool {
		slpToken, err = s.getSlpToken(txnHash, req.Index, scriptPubkey)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot get slp token for txid: %v", txnHash)
		}
		tokenID, err := chainhash.NewHash(slpToken.TokenId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot create hash for token id: %s", hex.EncodeToString(slpToken.TokenId))
		}
		tm, err = s.marshalTokenMetadata(*tokenID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot build token metadata for token id: %s", hex.EncodeToString(slpToken.TokenId))
		}
	}

	ret := &pb.GetUnspentOutputResponse{
		Outpoint: &pb.Transaction_Input_Outpoint{
			Hash:  txnHash[:],
			Index: req.Index,
		},
		Value:         value,
		PubkeyScript:  scriptPubkey,
		BlockHeight:   blockHeight,
		IsCoinbase:    coinbase,
		SlpToken:      slpToken,
		TokenMetadata: tm,
	}
	return ret, nil
}

// GetMerkleProof returns a Merkle (SPV) proof for a specific transaction
// in the provided block.
//
// **Requires TxIndex***
func (s *GrpcServer) GetMerkleProof(ctx context.Context, req *pb.GetMerkleProofRequest) (*pb.GetMerkleProofResponse, error) {
	if s.txIndex == nil {
		return nil, status.Error(codes.Unavailable, "txindex required")
	}

	txnHash, err := chainhash.NewHash(req.TransactionHash)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction hash %v", err)
	}

	// lookup location of the transaction
	blockRegion, err := s.txIndex.TxBlockRegion(txnHash)
	if err != nil || blockRegion == nil {
		return nil, status.Error(codes.NotFound, "unable to find block for given transaction")
	}

	blkHash := blockRegion.Hash

	block, err := s.chain.BlockByHash(blkHash)
	if err != nil {
		return nil, status.Error(codes.NotFound, "unable to find block for given transaction")
	}

	// create merkle proof
	mBlock, _ := merkleblock.NewMerkleBlockWithTxnSet(block, []*chainhash.Hash{txnHash})

	// encode proof to hex
	var buf bytes.Buffer
	if err := mBlock.BchEncode(&buf, wire.ProtocolVersion, wire.LatestEncoding); err != nil {
		return nil, status.Error(codes.Internal, "failed to deserialize merkle block")
	}

	hashes := make([][]byte, 0, len(mBlock.Hashes))
	for _, h := range mBlock.Hashes {
		hashes = append(hashes, h.CloneBytes())
	}
	medianTime, err := s.chain.MedianTimeByHash(block.Hash())
	if err != nil {
		return nil, status.Error(codes.Internal, "error calculating median time for block")
	}
	resp := &pb.GetMerkleProofResponse{
		Block:  marshalBlockInfo(block, s.chain.BestSnapshot().Height-block.Height()+1, medianTime, s.chainParams),
		Flags:  mBlock.Flags,
		Hashes: hashes,
	}

	nextHeader, err := s.chain.HeaderByHeight(block.Height() + 1)
	if err == nil {
		nextHash := nextHeader.BlockHash()
		resp.Block.NextBlockHash = nextHash.CloneBytes()
	}

	return resp, nil
}

// GetSlpTokenMetadata returns metadata associated with a Token ID
func (s *GrpcServer) GetSlpTokenMetadata(ctx context.Context, req *pb.GetSlpTokenMetadataRequest) (*pb.GetSlpTokenMetadataResponse, error) {
	if s.slpIndex == nil {
		return nil, status.Error(codes.Unavailable, "slpindex required")
	}

	tokenMetadata := make([]*pb.SlpTokenMetadata, 0)
	for _, hash := range req.GetTokenIds() {
		tokenID, err := chainhash.NewHash(hash)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "token ID hash %s is invalid: %s", hex.EncodeToString(hash), err)
		}

		tm, err := s.marshalTokenMetadata(*tokenID)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "token ID %v does not exist", hex.EncodeToString(hash))
		}

		tokenMetadata = append(tokenMetadata, tm)
	}

	resp := &pb.GetSlpTokenMetadataResponse{
		TokenMetadata: tokenMetadata,
	}

	return resp, nil
}

// GetSlpParsedScript returns a parsed object from a provided serialized slp OP_RETURN message
func (s *GrpcServer) GetSlpParsedScript(ctx context.Context, req *pb.GetSlpParsedScriptRequest) (*pb.GetSlpParsedScriptResponse, error) {
	resp := &pb.GetSlpParsedScriptResponse{}
	slpMsg, err := v1parser.ParseSLP(req.GetSlpOpreturnScript())
	if err != nil {
		resp.ParsingError = err.Error()
		return resp, nil
	}
	resp.TokenType = getTokenType(slpMsg.TokenType())

	switch msg := slpMsg.(type) {
	case *v1parser.SlpGenesis:
		if slpMsg.TokenType() == v1parser.TokenTypeNft1Child41 {
			meta := &pb.GetSlpParsedScriptResponse_V1Nft1ChildGenesis{
				V1Nft1ChildGenesis: &pb.SlpV1Nft1ChildGenesisMetadata{
					Name:         msg.Name,
					Ticker:       msg.Ticker,
					DocumentUrl:  msg.DocumentURI,
					DocumentHash: msg.DocumentHash,
					Decimals:     uint32(msg.Decimals),
				},
			}
			resp.SlpMetadata = meta
			resp.SlpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_GENESIS
		} else {
			meta := &pb.GetSlpParsedScriptResponse_V1Genesis{
				V1Genesis: &pb.SlpV1GenesisMetadata{
					Name:          msg.Name,
					Ticker:        msg.Ticker,
					DocumentUrl:   msg.DocumentURI,
					DocumentHash:  msg.DocumentHash,
					MintAmount:    msg.Qty,
					MintBatonVout: uint32(msg.MintBatonVout),
					Decimals:      uint32(msg.Decimals),
				},
			}
			resp.SlpMetadata = meta
			resp.SlpAction = pb.SlpAction_SLP_V1_GENESIS
		}
	case *v1parser.SlpMint:
		meta := &pb.GetSlpParsedScriptResponse_V1Mint{
			V1Mint: &pb.SlpV1MintMetadata{
				MintAmount:    msg.Qty,
				MintBatonVout: uint32(msg.MintBatonVout),
			},
		}
		resp.TokenId = msg.TokenID()
		resp.SlpMetadata = meta
		resp.SlpAction = pb.SlpAction_SLP_V1_MINT
	case *v1parser.SlpSend:
		if slpMsg.TokenType() == v1parser.TokenTypeNft1Child41 {
			meta := &pb.GetSlpParsedScriptResponse_V1Nft1ChildSend{
				V1Nft1ChildSend: &pb.SlpV1Nft1ChildSendMetadata{},
			}
			resp.SlpMetadata = meta
			resp.TokenId = msg.TokenID()
			resp.SlpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_SEND
		} else {
			meta := &pb.GetSlpParsedScriptResponse_V1Send{
				V1Send: &pb.SlpV1SendMetadata{
					Amounts: msg.Amounts,
				},
			}
			resp.SlpMetadata = meta
			resp.TokenId = msg.TokenID()
			resp.SlpAction = pb.SlpAction_SLP_V1_SEND
		}
	}

	return resp, nil
}

// GetSlpTrustedValidation returns slp validity information about a specific token output
func (s *GrpcServer) GetSlpTrustedValidation(ctx context.Context, req *pb.GetSlpTrustedValidationRequest) (*pb.GetSlpTrustedValidationResponse, error) {
	if s.slpIndex == nil {
		return nil, status.Error(codes.Unavailable, "slpindex required")
	}

	resp := &pb.GetSlpTrustedValidationResponse{}
	results := make([]*pb.GetSlpTrustedValidationResponse_ValidityResult, len(req.Queries))
	for i, query := range req.Queries {
		result := &pb.GetSlpTrustedValidationResponse_ValidityResult{}
		result.PrevOutHash = query.PrevOutHash
		result.PrevOutVout = query.PrevOutVout

		txid, err := chainhash.NewHash(query.PrevOutHash)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "invalid txn hash for txo %s: %v", query.GetPrevOutHash(), err)
		}

		entry, err := s.getSlpIndexEntry(txid)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "txid is missing from slp validity set for txo: %v:%s: %v", txid, fmt.Sprint(query.GetPrevOutVout()), err)
		}

		if query.PrevOutVout == 0 || query.PrevOutVout > 19 {
			return nil, status.Errorf(codes.Aborted, "slp output index cannot be 0 or > 19 txo: %v:%s", txid, fmt.Sprint(query.GetPrevOutVout()))
		}

		slpMsg, err := v1parser.ParseSLP(entry.SlpOpReturn)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not parse existing entry slp metadata scriptPubKey %s", hex.EncodeToString(entry.SlpOpReturn))
		}

		// set the proper slp version type
		switch slpMsg.TokenType() {
		case v1parser.TokenTypeFungible01:
			switch slpMsg.(type) {
			case *v1parser.SlpGenesis:
				result.SlpAction = pb.SlpAction_SLP_V1_GENESIS
			case *v1parser.SlpMint:
				result.SlpAction = pb.SlpAction_SLP_V1_MINT
			case *v1parser.SlpSend:
				result.SlpAction = pb.SlpAction_SLP_V1_SEND
			}
		case v1parser.TokenTypeNft1Child41:
			switch slpMsg.(type) {
			case *v1parser.SlpGenesis:
				result.SlpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_GENESIS
			case *v1parser.SlpSend:
				result.SlpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_SEND
			}
		case v1parser.TokenTypeNft1Group81:
			switch slpMsg.(type) {
			case *v1parser.SlpGenesis:
				result.SlpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_GENESIS
			case *v1parser.SlpMint:
				result.SlpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_MINT
			case *v1parser.SlpSend:
				result.SlpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_SEND
			}
		default:
			return nil, status.Error(codes.Aborted, "trusted validation cannot return result for unknown slp version type")
		}

		switch msg := slpMsg.(type) {
		case *v1parser.SlpSend:
			if len(msg.Amounts) < int(query.PrevOutVout) {
				return nil, status.Error(codes.Aborted, "vout is not a valid slp output")
			}
			result.TokenId = msg.TokenID()
			result.ValidityResultType = &pb.GetSlpTrustedValidationResponse_ValidityResult_V1TokenAmount{
				V1TokenAmount: msg.Amounts[query.PrevOutVout-1],
			}
		case *v1parser.SlpMint:
			result.TokenId = msg.TokenID()
			if query.PrevOutVout == 1 {
				result.ValidityResultType = &pb.GetSlpTrustedValidationResponse_ValidityResult_V1TokenAmount{
					V1TokenAmount: msg.Qty,
				}
			} else if int(query.PrevOutVout) == msg.MintBatonVout {
				result.ValidityResultType = &pb.GetSlpTrustedValidationResponse_ValidityResult_V1MintBaton{
					V1MintBaton: true,
				}
			} else {
				return nil, status.Error(codes.Aborted, "vout is not a valid slp output")
			}
		case *v1parser.SlpGenesis:
			hash := query.PrevOutHash
			for i := len(hash) - 1; len(result.TokenId) < len(hash); i-- {
				result.TokenId = append(result.TokenId, hash[i])
			}
			if query.PrevOutVout == 1 {
				result.ValidityResultType = &pb.GetSlpTrustedValidationResponse_ValidityResult_V1TokenAmount{
					V1TokenAmount: msg.Qty,
				}
			} else if int(query.PrevOutVout) == msg.MintBatonVout {
				result.ValidityResultType = &pb.GetSlpTrustedValidationResponse_ValidityResult_V1MintBaton{
					V1MintBaton: true,
				}
			} else {
				return nil, status.Error(codes.Aborted, "vout is not a valid slp output")
			}
		}

		result.SlpTxnOpreturn = entry.SlpOpReturn

		// include graph search count if client includes any value for excludes
		if req.IncludeGraphsearchCount {
			hash, err := chainhash.NewHash(query.PrevOutHash)
			if err != nil {
				return nil, status.Errorf(codes.Aborted, "slp graph search error: %v", err)
			}

			validityCache := make(map[chainhash.Hash]struct{})
			if query.GraphsearchValidHashes != nil {
				for _, validTxid := range query.GraphsearchValidHashes {
					hash, err := chainhash.NewHash(validTxid)
					if err != nil {
						return nil, status.Errorf(codes.Internal, "graph search validity txid %v, error: %v", hex.EncodeToString(validTxid), err)
					}
					validityCache[*hash] = struct{}{}
				}
			}

			gsDb, err := s.slpIndex.GetGraphSearchDb()
			if err != nil {
				return nil, status.Error(codes.Unavailable, err.Error())
			}
			txData, err := gsDb.Find(hash, &entry.TokenIDHash, &validityCache)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "%v", err)
			}
			log.Infof("SLP graph search count is %s transactions for txid %v", fmt.Sprint(len(txData)), hash)
			result.GraphsearchTxnCount = uint32(len(txData[:]))
		}

		results[i] = result
	}
	resp.Results = results
	return resp, nil
}

// GetSlpGraphSearch returns all transactions required for a client to validate locally
func (s *GrpcServer) GetSlpGraphSearch(ctx context.Context, req *pb.GetSlpGraphSearchRequest) (*pb.GetSlpGraphSearchResponse, error) {

	if !s.slpIndex.GraphSearchEnabled() {
		return nil, status.Error(codes.Unavailable, "slpgraphsearch must be enabled")
	}

	if s.slpIndex == nil || s.txIndex == nil {
		return nil, status.Error(codes.Unavailable, "slpindex and txindex must be enabled")
	}

	if _, err := s.slpIndex.GetGraphSearchDb(); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	// check slp validity, get graph tokenId
	hash, err := chainhash.NewHash(req.GetHash())
	if err != nil {
		return nil, status.Errorf(codes.Aborted, "graph search hash %s: %v", hex.EncodeToString(req.GetHash()), err)
	}
	log.Debugf("received graph search for txid: %v", hash)

	entry, err := s.getSlpIndexEntry(hash)
	if err != nil {
		return nil, status.Errorf(codes.Aborted, "txid is missing from slp validity set for txn: %s: %v", hash, err)
	}

	// get map for token and do graph search
	gsDb, err := s.slpIndex.GetGraphSearchDb()
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	// setup the validity cache
	validityCache := make(map[chainhash.Hash]struct{})
	for _, txHash := range req.GetValidHashes() {
		hash, err := chainhash.NewHash(txHash)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "graph search validity cache invalid hash %v", txHash)
		}
		validityCache[*hash] = struct{}{}
	}

	// perform the graph search
	txData, err := gsDb.Find(hash, &entry.TokenIDHash, &validityCache)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	res := &pb.GetSlpGraphSearchResponse{}
	res.Txdata = txData
	log.Infof("SLP graph search returned %s transactions for txid %v", fmt.Sprint(len(txData)), hash)

	return res, nil
}

func isMaybeSlpTransaction(txn *wire.MsgTx) bool {
	if len(txn.TxOut) > 0 {
		bchTagIDHex, _ := hex.DecodeString("534c5000")
		return bytes.Contains(txn.TxOut[0].PkScript, bchTagIDHex)
	}
	return false
}

// CheckSlpTransaction checks a supposed slp transaction for slp validity. The method returns the marshalled
// response including a slp validity boolean and a reason for invalid validity.
//
// Using the slp specification as a basis for validity judgement can lead to confusion for new users and
// result in accidental token burns.  use_spec_validity_judgement will cause the response's is_valid property
// to be returned according to the slp specification.  Therefore, use_spec_validity_judgement is false by
// default in order to avoid accidental token burns.  When use_spec_validity_judgement is false we return
// invalid in any case which would result in a burned token, unless the burn is explicitly included as an
// item in required_slp_burns property.
//
// When use_spec_validity_judgement is true, there are three cases where the is_valid response property
// will be returned as valid, instead of invalid, as per the slp specification.
//  1. inputs > outputs
//  2. missing transaction outputs
//  3. burned inputs from other tokens
//
// required_slp_burns is not used when use_spec_validity_judgement is set to true.
func (s *GrpcServer) CheckSlpTransaction(ctx context.Context, req *pb.CheckSlpTransactionRequest) (*pb.CheckSlpTransactionResponse, error) {

	if s.slpIndex == nil {
		return nil, status.Error(codes.Unavailable, "slpindex required")
	}

	msgTx := &wire.MsgTx{}
	if err := msgTx.BchDecode(bytes.NewReader(req.Transaction), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		return nil, status.Error(codes.InvalidArgument, "unable to deserialize transaction")
	}

	if len(msgTx.TxIn) == 0 || len(msgTx.TxOut) == 0 {
		return nil, status.Error(codes.InvalidArgument, "transaction is missing inputs or outputs")
	}

	return s.checkTransactionSlpValidity(msgTx, req.RequiredSlpBurns, true, !req.UseSpecValidityJudgement)
}

func (s *GrpcServer) checkTransactionSlpValidity(msgTx *wire.MsgTx, requiredBurns []*pb.SlpRequiredBurn, disableErrorResponse bool, useSafeValidityJudgement bool) (*pb.CheckSlpTransactionResponse, error) {

	// slpValid() and slpInvalid() are helpers to keep the return statements clean
	slpValid := func() *pb.CheckSlpTransactionResponse {
		return &pb.CheckSlpTransactionResponse{
			IsValid:    true,
			BestHeight: s.chain.BestSnapshot().Height,
		}
	}
	slpInvalid := func(reason string) *pb.CheckSlpTransactionResponse {
		return &pb.CheckSlpTransactionResponse{
			IsValid:       false,
			InvalidReason: reason,
			BestHeight:    s.chain.BestSnapshot().Height,
		}
	}

	if len(msgTx.TxOut) < 1 {
		return nil, status.Error(codes.InvalidArgument, "transaction has no outputs")
	}

	// check if the transaction is slp valid
	slpMd, err := v1parser.ParseSLP(msgTx.TxOut[0].PkScript)
	if err != nil {
		// check if transaction output index 0 contained slp magic bytes
		if isMaybeSlpTransaction(msgTx) {
			invalidReason := fmt.Sprintf("error parsing scriptPubKey as slp metadata, %v", err)
			if disableErrorResponse {
				return slpInvalid(invalidReason), nil
			}
			return nil, status.Error(codes.Aborted, invalidReason)
		}

		// check for slp burns
		for i, txIn := range msgTx.TxIn {
			idx := txIn.PreviousOutPoint.Index

			// we can always skip previous output index 0 since it cannot contain an slp token
			if idx == 0 {
				continue
			}

			// check to see if the input is a burn
			slpEntry, err := s.getSlpIndexEntryAndCheckBurnOtherToken(txIn.PreviousOutPoint, requiredBurns, nil, i)
			if slpEntry == nil {
				continue
			}
			if err != nil {
				invalidReason := "non-slp transaction, includes valid slp inputs"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Error(codes.Aborted, err.Error())
			}
		}

		// otherwise, we can assume this is a non-slp transaction attempt, return invalid without an error
		return slpInvalid("non-slp transaction"), nil
	}

	// check slp transactions for burn prevention
	switch md := slpMd.(type) {
	case *v1parser.SlpSend:
		inputVal := big.NewInt(0)

		// loop through inputs, accumulate input amount for tokenID, abort on slp input with wrong ID
		for i, txIn := range msgTx.TxIn {
			slpEntry, err := s.getSlpIndexEntryAndCheckBurnOtherToken(txIn.PreviousOutPoint, requiredBurns, md, i)
			if slpEntry == nil {
				continue
			}
			if err != nil && useSafeValidityJudgement {
				invalidReason := "transaction includes slp token burn with an input from the wrong token"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "%s, use SlpRequiredBurn to allow burns: %v", invalidReason, err)
			}

			inputSlpMsg, err := v1parser.ParseSLP(slpEntry.SlpOpReturn)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "could not parse previously stored slp entry %v having slp message %s", txIn.PreviousOutPoint.Hash, hex.EncodeToString(slpEntry.SlpOpReturn))
			}
			idx := txIn.PreviousOutPoint.Index
			amt, _ := inputSlpMsg.GetVoutValue(int(idx))
			if amt != nil {
				inputVal.Add(inputVal, amt)
			}
		}

		// check inputs != outputs (use check for explict burn requests i.e., 'req.AllowedSlpBurns')
		outputVal, err := slpMd.TotalSlpMsgOutputValue()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "an error occured when getting total slp amount in txn: %v, with error: %v", msgTx.TxHash(), err)
		}
		if inputVal.Cmp(outputVal) < 0 {
			invalidReason := "outputs greater than inputs"
			if disableErrorResponse {
				return slpInvalid(invalidReason), nil
			}
			return nil, status.Errorf(codes.Aborted, "invalid slp: %s", invalidReason)
		} else if inputVal.Cmp(outputVal) > 0 && useSafeValidityJudgement {

			// handle the simple case where user has provided no burn instructions
			if len(requiredBurns) == 0 {
				if disableErrorResponse {
					return slpInvalid("inputs are greater than outputs"), nil
				}
				return nil, status.Errorf(codes.Aborted, "inputs greater than outputs")

			}

			// check user specified burn amounts
			burnAmt := big.NewInt(0)
			for _, burn := range requiredBurns {
				burnAmt.Add(burnAmt, new(big.Int).SetUint64(burn.GetAmount()))
			}
			inputAmtUsed := inputVal.Sub(inputVal, burnAmt)
			if inputAmtUsed.Cmp(outputVal) < 0 {
				invalidReason := fmt.Sprintf("specified burn ammount %s is too high", burnAmt.String())
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "%s, use SlpRequiredBurn to allow burns", invalidReason)
			} else if inputAmtUsed.Cmp(outputVal) > 0 {
				invalidReason := fmt.Sprintf("specified burn ammount %s is too low", burnAmt.String())
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "%s, use SlpRequiredBurn to allow burns", invalidReason)
			}
		}

		// prevent missing token outputs
		if useSafeValidityJudgement && len(md.Amounts) > len(msgTx.TxOut)-1 {
			invalidReason := "transaction is missing outputs"
			if disableErrorResponse {
				return slpInvalid(invalidReason), nil
			}
			return nil, status.Errorf(codes.Aborted, "transaction includes slp token burn, %s", invalidReason)
		}

		// if we made it to this point then it is valid
		return slpValid(), nil

	case *v1parser.SlpMint:
		hasBaton := false

		// loop through inputs, check for input burns, look for mint baton is included,
		for i, txIn := range msgTx.TxIn {
			slpEntry, err := s.getSlpIndexEntryAndCheckBurnOtherToken(txIn.PreviousOutPoint, requiredBurns, slpMd, i)
			if slpEntry == nil {
				continue
			}
			if err != nil && useSafeValidityJudgement {
				invalidReason := "transaction includes slp token burn with an input from the wrong token"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "%s, use SlpRequiredBurn to allow burns: %v", invalidReason, err)
			}

			inpSlpMd, err := v1parser.ParseSLP(slpEntry.SlpOpReturn)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "could not parse previously stored slp entry %v having slp message %s", txIn.PreviousOutPoint.Hash, hex.EncodeToString(slpEntry.SlpOpReturn))
			}

			switch md := inpSlpMd.(type) {
			case *v1parser.SlpGenesis:
				if md.MintBatonVout == int(txIn.PreviousOutPoint.Index) {
					hasBaton = true
				}
			case *v1parser.SlpMint:
				if md.MintBatonVout == int(txIn.PreviousOutPoint.Index) {
					hasBaton = true
				}
			}
		}

		if !hasBaton {
			invalidReason := "missing valid baton"
			if disableErrorResponse {
				return slpInvalid(invalidReason), nil
			}
			return nil, status.Error(codes.Aborted, invalidReason)
		}

		// check for missing bch outputs
		if useSafeValidityJudgement {

			// prevent missing token output
			if len(msgTx.TxOut) < 2 {
				invalidReason := "transaction is missing outputs"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "transaction includes slp token burn, %s", invalidReason)
			}

			// prevent missing mint baton output
			batonVout := md.MintBatonVout
			if batonVout > 1 && batonVout > len(msgTx.TxOut)-1 {
				invalidReason := "transaction is missing mint baton output"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "transaction includes slp token burn: %s", invalidReason)
			}
		}

		// if we made it to this point then it is valid
		return slpValid(), nil

	case *v1parser.SlpGenesis:
		// loop through inputs, check for input burns
		for i, txIn := range msgTx.TxIn {
			slpEntry, err := s.getSlpIndexEntryAndCheckBurnOtherToken(txIn.PreviousOutPoint, requiredBurns, slpMd, i)
			if slpEntry == nil {
				continue
			}
			if err != nil && useSafeValidityJudgement {
				invalidReason := "transaction includes slp token burn with an input from the wrong token"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "%s, use SlpRequiredBurn to allow burns: %v", invalidReason, err)
			}

			// check for invalid nft genesis
			if i == 0 && slpMd.TokenType() == v1parser.TokenTypeNft1Child41 {
				if slpEntry.SlpVersionType != v1parser.TokenTypeNft1Group81 {
					invalidReason := "missing nft group input"
					if disableErrorResponse {
						return slpInvalid(invalidReason), nil

					}
					return nil, status.Error(codes.Aborted, invalidReason)

				}

				inpSlpMd, err := v1parser.ParseSLP(slpEntry.SlpOpReturn)
				if err != nil {
					return nil, status.Errorf(codes.Aborted, "could not parse group input in %v", msgTx.TxHash())
				}
				val, _ := inpSlpMd.GetVoutValue(int(txIn.PreviousOutPoint.Index))
				if val.Cmp(new(big.Int).SetUint64(1)) < 0 {
					invalidReason := "insufficient nft group tokens burned"
					if disableErrorResponse {
						return slpInvalid(invalidReason), nil
					}
					return nil, status.Error(codes.Aborted, invalidReason)
				}
			}
		}

		// check for missing bch outputs
		if useSafeValidityJudgement {

			// prevent missing token output
			if len(msgTx.TxOut) < 2 {
				invalidReason := "transaction is missing outputs"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "transaction includes slp token burn, %s", invalidReason)
			}

			// prevent missing mint baton output
			batonVout := md.MintBatonVout
			if batonVout > 1 && batonVout > len(msgTx.TxOut)-1 {
				invalidReason := "transaction is missing mint baton output"
				if disableErrorResponse {
					return slpInvalid(invalidReason), nil
				}
				return nil, status.Errorf(codes.Aborted, "transaction includes slp token burn: %s", invalidReason)
			}
		}

		// if we made it to this point then it is valid
		return slpValid(), nil
	}

	return nil, status.Errorf(codes.Internal, "an unknown error occured checking transaction %v", msgTx.TxHash())
}

// getSlpIndexEntryAndCheckBurnOtherToken checks for burns FROM OTHER TOKEN TYPES.
//
// This method does not check burn prevention for input qty > output qty, or missing vout,
// in valid slp send/mint.  Checking for burns of the same token id and versionType needs
// to be checked elsewhere.
//
// NOTE: nft1 child genesis is allowed without error as long as the burned outpoint is a
//
//	nft1 Group type and the quanity is 1.
func (s *GrpcServer) getSlpIndexEntryAndCheckBurnOtherToken(outpoint wire.OutPoint, requiredBurns []*pb.SlpRequiredBurn, txnSlpMsg v1parser.ParseResult, inputIdx int) (*indexers.SlpTxEntry, error) {

	slpEntry, err := s.getSlpIndexEntry(&outpoint.Hash)
	if err != nil {
		return nil, err
	}

	inputSlpMsg, err := v1parser.ParseSLP(slpEntry.SlpOpReturn)
	if err != nil {
		return nil, errors.New("could not parse slpMsg from and existing db entry, this should never happen")
	}

	// exit early if the outpoint is not an slp outpoint, or is a zero output slp
	amt, isBaton := inputSlpMsg.GetVoutValue(int(outpoint.Index))
	if !isBaton {
		if amt == nil {
			return nil, nil
		} else if amt.Cmp(new(big.Int).SetUint64(0)) == 0 {
			return nil, nil
		}
	}

	// exit without error if this outpoint is of the same token id/versionType
	if txnSlpMsg != nil {
		if slpEntry.SlpVersionType == txnSlpMsg.TokenType() {
			switch txnMsgData := txnSlpMsg.(type) {
			case *v1parser.SlpMint:
				switch inputMsgData := inputSlpMsg.(type) {
				case *v1parser.SlpGenesis:
					// check the input is for the same token ID and is the actual baton
					if bytes.Equal(txnMsgData.TokenID(), slpEntry.TokenIDHash[:]) &&
						inputMsgData.MintBatonVout == int(outpoint.Index) {

						// then check the mint baton is being spent as a valid mint baton
						// NOTE: We can't check vout exists here..
						if txnMsgData.MintBatonVout > 1 {
							return slpEntry, nil
						}
					}
				case *v1parser.SlpMint:
					// check the input is for the same token ID and is the actual baton
					if bytes.Equal(txnMsgData.TokenID(), slpEntry.TokenIDHash[:]) &&
						inputMsgData.MintBatonVout == int(outpoint.Index) {

						// then check the mint baton is being spent as a valid mint baton
						// NOTE: We can't check vout exists here..
						if txnMsgData.MintBatonVout > 1 {
							return slpEntry, nil
						}
					}
				}
			case *v1parser.SlpSend:
				switch inputMsgData := inputSlpMsg.(type) {
				case *v1parser.SlpGenesis:
					// check token id is the same, but make sure this isn't a minting baton
					if bytes.Equal(txnMsgData.TokenID(), slpEntry.TokenIDHash[:]) && inputMsgData.MintBatonVout != int(outpoint.Index) {
						return slpEntry, nil
					}
				case *v1parser.SlpMint:
					// check token id is the same, but make sure this isn't a minting baton
					if bytes.Equal(txnMsgData.TokenID(), slpEntry.TokenIDHash[:]) && inputMsgData.MintBatonVout != int(outpoint.Index) {
						return slpEntry, nil
					}
				case *v1parser.SlpSend:
					if bytes.Equal(txnMsgData.TokenID(), slpEntry.TokenIDHash[:]) {
						return slpEntry, nil
					}
				}
			}
		}
	}

	// exit without error if this outpoint is being spent for a nft child genesis burning 1 nft group token
	if txnSlpMsg != nil && inputIdx == 0 {
		if md, ok := txnSlpMsg.(*v1parser.SlpGenesis); ok {
			if md.TokenType() == v1parser.TokenTypeNft1Child41 && slpEntry.SlpVersionType == v1parser.TokenTypeNft1Group81 {
				val, _ := inputSlpMsg.GetVoutValue(int(outpoint.Index))
				if val != nil && val.Cmp(new(big.Int).SetUint64(1)) == 0 {
					log.Debugf("allowed nft group token burn in %s", hex.EncodeToString(txnSlpMsg.TokenID()))
					return slpEntry, nil
				}
			}
		}
	}

	canBurn := false
	for _, burn := range requiredBurns {

		// this will happen when client requires a burn associated with
		// burn of same token ID/version type where there isn't a specific outpoint being burned
		if burn.Outpoint == nil {
			continue
		}

		if bytes.Equal(burn.Outpoint.Hash, outpoint.Hash[:]) && burn.Outpoint.Index == outpoint.Index {
			// check token ID of the burn request matches the entry
			if !bytes.Equal(slpEntry.TokenIDHash[:], burn.GetTokenId()) {
				return slpEntry, status.Error(codes.InvalidArgument, "the requested burn token ID does not match the actual token ID")
			}

			// check token version type of the burn request matches the entry
			if int(slpEntry.SlpVersionType) != int(burn.GetTokenType()) {
				return slpEntry, status.Error(codes.InvalidArgument, "the requested burn token version type does not match the actual token version type")
			}

			// check burn intent (amount or mint)
			if _, isAmountBurn := burn.BurnIntention.(*pb.SlpRequiredBurn_Amount); isAmountBurn {
				amt, _ := inputSlpMsg.GetVoutValue(int(outpoint.Index))
				if amt != nil && amt.Cmp(new(big.Int).SetUint64(burn.GetAmount())) != 0 {
					return slpEntry, status.Error(codes.InvalidArgument, "the requested burn amount does not match the amount to be burned")
				}

				canBurn = true
				break
			} else if _, isMintBurn := burn.BurnIntention.(*pb.SlpRequiredBurn_MintBatonVout); isMintBurn {
				switch t := inputSlpMsg.(type) {
				case *v1parser.SlpGenesis:
					if t.MintBatonVout != int(burn.GetMintBatonVout()) {
						return slpEntry, status.Error(codes.InvalidArgument, "the requested burn minting baton vout is incorrect")
					}
				case *v1parser.SlpMint:
					if t.MintBatonVout != int(burn.GetMintBatonVout()) {
						return slpEntry, status.Error(codes.InvalidArgument, "the requested burn minting baton vout is incorrect")
					}
				default:
					return slpEntry, status.Error(codes.InvalidArgument, "the requested burn outpoint is not a minting baton")
				}

				canBurn = true
				break
			}
		}
	}

	if !canBurn {
		return slpEntry, errors.New("token burn from wrong token id")
	}

	return slpEntry, nil
}

// SubmitTransaction submits a transaction to all connected peers.
//
// If slp index is enabled it will not allow slp burns unless the burned token is
// included in req.RequiredSlpBurns, or if req.SkipSlpValidityCheck is set to true
func (s *GrpcServer) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {

	msgTx := &wire.MsgTx{}
	if err := msgTx.BchDecode(bytes.NewReader(req.Transaction), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		return nil, status.Error(codes.InvalidArgument, "unable to deserialize transaction")
	}

	if s.slpIndex != nil && !req.GetSkipSlpValidityCheck() {
		_, err := s.checkTransactionSlpValidity(msgTx, req.RequiredSlpBurns, false, true)
		if err != nil {
			return nil, err
		}
	}

	// Use 0 for the tag to represent local node.
	tx := bchutil.NewTx(msgTx)
	acceptedTxs, err := s.txMemPool.ProcessTransaction(tx, false, false, 0)
	if err != nil {
		// When the error is a rule error, it means the transaction was
		// simply rejected as opposed to something actually going wrong,
		// so log it as such.  Otherwise, something really did go wrong,
		// so log it as an actual error.  In both cases, a JSON-RPC
		// error is returned to the client with the deserialization
		// error code (to match bitcoind behavior).
		if _, ok := err.(mempool.RuleError); ok {
			log.Debugf("Rejected transaction %v: %v", tx.Hash(),
				err)
		} else {
			log.Errorf("Failed to process transaction %v: %v",
				tx.Hash(), err)
		}
		return nil, status.Errorf(codes.InvalidArgument, "tx rejected: %v", err)
	}

	// When the transaction was accepted it should be the first item in the
	// returned array of accepted transactions.  The only way this will not
	// be true is if the API for ProcessTransaction changes and this code is
	// not properly updated, but ensure the condition holds as a safeguard.
	//
	// Also, since an error is being returned to the caller, ensure the
	// transaction is removed from the memory pool.
	if len(acceptedTxs) == 0 || !acceptedTxs[0].Tx.Hash().IsEqual(tx.Hash()) {
		s.txMemPool.RemoveTransaction(tx, true)

		return nil, status.Errorf(codes.Internal, "transaction %v is not in accepted list", tx.Hash())
	}

	// Generate and relay inventory vectors for all newly accepted
	// transactions into the memory pool due to the original being
	// accepted.
	s.netMgr.AnnounceNewTransactions(acceptedTxs)

	// Keep track of all the sendrawtransaction request txns so that they
	// can be rebroadcast if they don't make their way into a block.
	txD := acceptedTxs[0]
	iv := wire.NewInvVect(wire.InvTypeTx, txD.Tx.Hash())
	s.netMgr.AddRebroadcastInventory(iv, txD)

	resp := &pb.SubmitTransactionResponse{
		Hash: tx.Hash().CloneBytes(),
	}
	return resp, nil
}

// SubscribeTransactions creates subscription to all relevant transactions based on
// the subscription filter.
//
// This RPC does not use bidirectional streams and therefore can be used
// with grpc-web. You will need to close and reopen the stream whenever
// you want to update the subscription filter. If you are not using grpc-web
// then SubscribeTransactionStream is more appropriate.
//
// **Requires TxIndex to receive input metadata**
func (s *GrpcServer) SubscribeTransactions(req *pb.SubscribeTransactionsRequest, stream pb.Bchrpc_SubscribeTransactionsServer) error {
	subscription := s.subscribeEvents()
	defer subscription.Unsubscribe()

	filter := newTxFilter()
	if err := filter.AddRPCFilter(req.GetSubscribe(), s.chainParams); err != nil {
		return err
	}
	includeMempool := req.IncludeMempool
	includeBlocks := req.IncludeInBlock
	serializeTx := req.SerializeTx
	for {
		select {
		case event := <-subscription.Events():

			switch event := event.(type) {
			case *rpcEventTxAccepted:
				if !includeMempool {
					continue
				}

				txDesc := event

				if s.slpIndex != nil {
					s.checkSlpTxOnEvent(txDesc.Tx.MsgTx(), "SubscribeTransactions rpcEventTxAccepted")
				}

				if !filter.MatchAndUpdate(txDesc.Tx, s.chainParams) {
					continue
				}

				toSend := &pb.TransactionNotification{}
				toSend.Type = pb.TransactionNotification_UNCONFIRMED

				if serializeTx {
					var buf bytes.Buffer
					if err := txDesc.Tx.MsgTx().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
						return status.Error(codes.Internal, "error serializing transaction")
					}

					toSend.Transaction = &pb.TransactionNotification_SerializedTransaction{
						SerializedTransaction: buf.Bytes(),
					}

				} else {
					respTx := marshalTransaction(txDesc.Tx, 0, nil, 0, s)

					if view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx); err == nil {
						s.setInputMetadataFromView(respTx, txDesc, view)
					}

					toSend.Transaction = &pb.TransactionNotification_UnconfirmedTransaction{
						UnconfirmedTransaction: &pb.MempoolTransaction{
							Transaction:      respTx,
							AddedTime:        txDesc.Added.Unix(),
							Fee:              txDesc.Fee,
							FeePerKb:         txDesc.FeePerKB,
							AddedHeight:      txDesc.Height,
							StartingPriority: txDesc.StartingPriority,
						},
					}
				}

				if err := stream.Send(toSend); err != nil {
					return err
				}

			case *rpcEventBlockConnected:
				if !includeBlocks {
					continue
				}
				// Search for all transactions.
				block := event

				for _, tx := range block.Transactions() {
					if !filter.MatchAndUpdate(tx, s.chainParams) {
						continue
					}

					if s.slpIndex != nil {
						s.checkSlpTxOnEvent(tx.MsgTx(), "SubscribeTransactions rpcEventBlockConnected")
					}

					toSend := &pb.TransactionNotification{}
					toSend.Type = pb.TransactionNotification_CONFIRMED

					if serializeTx {
						var buf bytes.Buffer
						if err := tx.MsgTx().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
							return status.Error(codes.Internal, "error serializing transaction")
						}
						toSend.Transaction = &pb.TransactionNotification_SerializedTransaction{
							SerializedTransaction: buf.Bytes(),
						}

					} else {
						header := block.MsgBlock().Header

						respTx := marshalTransaction(tx, s.chain.BestSnapshot().Height-block.Height()+1, &header, block.Height(), s)
						if s.txIndex != nil {
							if err := s.setInputMetadata(respTx); err != nil {
								return err
							}
						}
						toSend.Transaction = &pb.TransactionNotification_ConfirmedTransaction{
							ConfirmedTransaction: respTx,
						}
					}

					if err := stream.Send(toSend); err != nil {
						return err
					}
				}
			}

		case <-stream.Context().Done():
			return nil // client disconnected
		}
	}
}

// SubscribeTransactionStream subscribes to relevant transactions based on
// the subscription requests. The parameters to filter transactions on can
// be updated by sending new SubscribeTransactionsRequest objects on the stream.
//
// Because this RPC is using bi-directional streaming it cannot be used with
// grpc-web.
//
// **Requires TxIndex to receive input metadata**
func (s *GrpcServer) SubscribeTransactionStream(stream pb.Bchrpc_SubscribeTransactionStreamServer) error {
	// Put the incoming messages on a channel.
	requests := make(chan *pb.SubscribeTransactionsRequest)
	go func() {
		for {
			req, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					log.Debugf("Error reading from client stream: %v", err)
				}
				close(requests)
				return
			}
			requests <- req
		}
	}()

	subscription := s.subscribeEvents()
	defer subscription.Unsubscribe()

	filter := newTxFilter()
	var includeMempool, includeBlocks, serializeTx bool
	for {
		select {
		case req := <-requests:
			if req == nil {
				return nil
			}
			includeMempool = req.IncludeMempool
			includeBlocks = req.IncludeInBlock
			serializeTx = req.SerializeTx

			// Update filter.
			if err := filter.AddRPCFilter(req.GetSubscribe(), s.chainParams); err != nil {
				return err
			}
			if err := filter.RemoveRPCFilter(req.GetUnsubscribe(), s.chainParams); err != nil {
				return err
			}

		case event := <-subscription.Events():

			switch event := event.(type) {
			case *rpcEventTxAccepted:
				if !includeMempool {
					continue
				}

				txDesc := event

				if s.slpIndex != nil {
					s.checkSlpTxOnEvent(txDesc.Tx.MsgTx(), "SubscribeTransactionStream rpcEventTxAccepted")
				}

				if !filter.MatchAndUpdate(txDesc.Tx, s.chainParams) {
					continue
				}

				toSend := &pb.TransactionNotification{}
				toSend.Type = pb.TransactionNotification_UNCONFIRMED

				if serializeTx {
					var buf bytes.Buffer
					if err := txDesc.Tx.MsgTx().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
						return status.Error(codes.Internal, "error serializing transaction")
					}

					toSend.Transaction = &pb.TransactionNotification_SerializedTransaction{
						SerializedTransaction: buf.Bytes(),
					}

				} else {
					respTx := marshalTransaction(txDesc.Tx, 0, nil, 0, s)

					if view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx); err == nil {
						s.setInputMetadataFromView(respTx, txDesc, view)
					}

					toSend.Transaction = &pb.TransactionNotification_UnconfirmedTransaction{
						UnconfirmedTransaction: &pb.MempoolTransaction{
							Transaction:      respTx,
							AddedTime:        txDesc.Added.Unix(),
							Fee:              txDesc.Fee,
							FeePerKb:         txDesc.FeePerKB,
							AddedHeight:      txDesc.Height,
							StartingPriority: txDesc.StartingPriority,
						},
					}
				}

				if err := stream.Send(toSend); err != nil {
					return err
				}

			case *rpcEventBlockConnected:
				if !includeBlocks {
					continue
				}
				// Search for all transactions.
				block := event

				for _, tx := range block.Transactions() {
					if !filter.MatchAndUpdate(tx, s.chainParams) {
						continue
					}

					if s.slpIndex != nil {
						s.checkSlpTxOnEvent(tx.MsgTx(), "SubscribeTransactionStream rpcEventBlockConnected")
					}

					toSend := &pb.TransactionNotification{}
					toSend.Type = pb.TransactionNotification_CONFIRMED

					if serializeTx {
						var buf bytes.Buffer
						if err := tx.MsgTx().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
							return status.Error(codes.Internal, "error serializing transaction")
						}
						toSend.Transaction = &pb.TransactionNotification_SerializedTransaction{
							SerializedTransaction: buf.Bytes(),
						}

					} else {
						header := block.MsgBlock().Header

						respTx := marshalTransaction(tx, s.chain.BestSnapshot().Height-block.Height()+1, &header, block.Height(), s)
						if s.txIndex != nil {
							if err := s.setInputMetadata(respTx); err != nil {
								return err
							}
						}
						toSend.Transaction = &pb.TransactionNotification_ConfirmedTransaction{
							ConfirmedTransaction: respTx,
						}
					}

					if err := stream.Send(toSend); err != nil {
						return err
					}
				}
			}

		case <-stream.Context().Done():
			return nil // client disconnected
		}
	}
}

// SubscribeBlocks creates a subscription for notifications of new blocks being
// connected to the blockchain or blocks being disconnected.
func (s *GrpcServer) SubscribeBlocks(req *pb.SubscribeBlocksRequest, stream pb.Bchrpc_SubscribeBlocksServer) error {
	subscription := s.subscribeEvents()
	defer subscription.Unsubscribe()

	for {
		select {
		case event := <-subscription.Events():

			switch event := event.(type) {
			case *rpcEventBlockConnected:
				// Search for all transactions.
				block := event.Block
				toSend := &pb.BlockNotification{}
				toSend.Type = pb.BlockNotification_CONNECTED

				medianTime, err := s.chain.MedianTimeByHash(block.Hash())
				if err != nil {
					return err
				}

				if req.FullBlock && !req.SerializeBlock {
					confirmations := s.chain.BestSnapshot().Height - block.Height() + 1
					respBlock := &pb.BlockNotification_MarshaledBlock{
						MarshaledBlock: &pb.Block{
							Info: marshalBlockInfo(block, confirmations, medianTime, s.chainParams),
						},
					}

					var spentTxos []blockchain.SpentTxOut
					var err error
					if req.FullTransactions {
						spentTxos, err = s.chain.FetchSpendJournal(block)
						if err != nil {
							return status.Error(codes.Internal, "error loading spend journal")
						}
					}

					spendIdx := 0
					for idx, tx := range block.Transactions() {
						if req.FullTransactions {
							header := block.MsgBlock().Header
							respTx := marshalTransaction(tx, confirmations, &header, block.Height(), s)

							for i := range tx.MsgTx().TxIn {
								if idx > 0 {
									stxo := spentTxos[spendIdx]
									pk, cashToken, err := getTokenDataForInputIfExists(stxo.PkScript)
									if err != nil {
										log.Debugf("could not parse token data for %v index: %v", tx.Hash(), uint32(i))
									}
									if err != nil || cashToken == nil {
										pk = stxo.PkScript
									} else { // cash token data exists.
										respTx.Inputs[i].CashToken = cashToken
									}

									respTx.Inputs[i].Value = stxo.Amount
									respTx.Inputs[i].PreviousScript = pk

									_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
									if err == nil && len(addrs) > 0 {
										respTx.Inputs[i].Address = addrs[0].String()
									}
									spendIdx++
								}
							}

							respBlock.MarshaledBlock.TransactionData = append(respBlock.MarshaledBlock.TransactionData, &pb.Block_TransactionData{
								TxidsOrTxs: &pb.Block_TransactionData_Transaction{
									Transaction: respTx,
								},
							})

						} else {
							respBlock.MarshaledBlock.TransactionData = append(respBlock.MarshaledBlock.TransactionData, &pb.Block_TransactionData{
								TxidsOrTxs: &pb.Block_TransactionData_TransactionHash{
									TransactionHash: tx.Hash().CloneBytes(),
								},
							})
						}
					}

					toSend.Block = respBlock

				} else {
					toSend.Block = &pb.BlockNotification_BlockInfo{
						BlockInfo: marshalBlockInfo(block, s.chain.BestSnapshot().Height-block.Height()+1, medianTime, s.chainParams),
					}
				}

				if req.SerializeBlock {
					bytes, err := block.Bytes()
					if err != nil {
						return status.Error(codes.Internal, "block serialization error")
					}

					toSend.Block = &pb.BlockNotification_SerializedBlock{
						SerializedBlock: bytes,
					}
				}

				if err := stream.Send(toSend); err != nil {
					return err
				}

			case *rpcEventBlockDisconnected:
				// Search for all transactions.
				block := event.Block
				toSend := &pb.BlockNotification{}
				toSend.Type = pb.BlockNotification_DISCONNECTED

				medianTime, err := s.chain.MedianTimeByHash(block.Hash())
				if err != nil {
					return err
				}

				if req.FullBlock && !req.SerializeBlock {
					confirmations := s.chain.BestSnapshot().Height - block.Height() + 1
					respBlock := &pb.BlockNotification_MarshaledBlock{
						MarshaledBlock: &pb.Block{
							Info: marshalBlockInfo(block, confirmations, medianTime, s.chainParams),
						},
					}

					var spentTxos []blockchain.SpentTxOut
					var err error
					if req.FullTransactions {
						spentTxos, err = s.chain.FetchSpendJournal(block)
						if err != nil {
							return status.Error(codes.Internal, "error loading spend journal")
						}
					}

					spendIdx := 0
					for idx, tx := range block.Transactions() {
						if req.FullTransactions {
							header := block.MsgBlock().Header
							respTx := marshalTransaction(tx, confirmations, &header, block.Height(), s)
							for i := range tx.MsgTx().TxIn {
								if idx > 0 {
									stxo := spentTxos[spendIdx]

									pk, cashToken, err := getTokenDataForInputIfExists(stxo.PkScript)
									if err != nil {
										log.Debugf("could not parse token data for %v index: %v", tx.Hash(), uint32(i))
									}
									if err != nil || cashToken == nil {
										pk = stxo.PkScript
									} else { // cash token data exists.
										respTx.Inputs[i].CashToken = cashToken
									}

									respTx.Inputs[i].Value = stxo.Amount
									respTx.Inputs[i].PreviousScript = pk

									_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
									if err == nil && len(addrs) > 0 {
										respTx.Inputs[i].Address = addrs[0].String()
									}
									spendIdx++
								}
							}

							respBlock.MarshaledBlock.TransactionData = append(respBlock.MarshaledBlock.TransactionData, &pb.Block_TransactionData{
								TxidsOrTxs: &pb.Block_TransactionData_Transaction{
									Transaction: respTx,
								},
							})
						} else {
							respBlock.MarshaledBlock.TransactionData = append(respBlock.MarshaledBlock.TransactionData, &pb.Block_TransactionData{
								TxidsOrTxs: &pb.Block_TransactionData_TransactionHash{
									TransactionHash: tx.Hash().CloneBytes(),
								},
							})
						}
					}

					toSend.Block = respBlock

				} else {
					toSend.Block = &pb.BlockNotification_BlockInfo{
						BlockInfo: marshalBlockInfo(block, s.chain.BestSnapshot().Height-block.Height()+1, medianTime, s.chainParams),
					}
				}

				if req.SerializeBlock {
					bytes, err := block.Bytes()
					if err != nil {
						return status.Error(codes.Internal, "block serialization error")
					}

					toSend.Block = &pb.BlockNotification_SerializedBlock{
						SerializedBlock: bytes,
					}
				}

				if err := stream.Send(toSend); err != nil {
					return err
				}
			}

		case <-stream.Context().Done():
			return nil // client disconnected
		}
	}
}

func (s *GrpcServer) fetchTransactionFromBlock(txHash *chainhash.Hash) ([]byte, int32, *chainhash.Hash, error) {
	// Look up the location of the transaction.
	blockRegion, err := s.txIndex.TxBlockRegion(txHash)
	if err != nil {
		return nil, 0, nil, status.Error(codes.InvalidArgument, "failed to retrieve transaction location")
	}
	if blockRegion == nil {
		return nil, 0, nil, status.Error(codes.NotFound, "transaction not found")
	}

	// Load the raw transaction bytes from the database.
	var txBytes []byte
	err = s.db.View(func(dbTx database.Tx) error {
		var err error
		txBytes, err = dbTx.FetchBlockRegion(blockRegion)
		return err
	})
	if err != nil {
		return nil, 0, nil, status.Error(codes.Internal, "failed to load transaction bytes")
	}

	// Grab the block height.
	blockHeight, err := s.chain.BlockHeightByHash(blockRegion.Hash)
	if err != nil {
		return nil, 0, nil, status.Error(codes.Internal, "failed to retrieve block")
	}

	return txBytes, blockHeight, blockRegion.Hash, nil
}

// setInputMetadata will set the value, previous script, and address for each input in the transaction
// by loading the previous transaction from the txindex and using its data.
func (s *GrpcServer) setInputMetadata(tx *pb.Transaction) error {
	inputTxMap := make(map[chainhash.Hash]*wire.MsgTx)
	for i, in := range tx.Inputs {
		ch, err := chainhash.NewHash(in.Outpoint.Hash)
		if err != nil {
			return status.Error(codes.Internal, "error marshaling chainhash")
		}
		if ch.IsEqual(&chainhash.Hash{}) { // Coinbase txs don't have an input.
			continue
		}
		if prevTx, ok := inputTxMap[*ch]; ok {

			pk, cashToken, err := getTokenDataForInputIfExists(prevTx.TxOut[in.Outpoint.Index].PkScript)
			if err != nil {
				log.Debugf("could not parse token data for %v index: %v", prevTx.TxHash(), uint32(i))
			}
			if err != nil || cashToken == nil {
				pk = prevTx.TxOut[in.Outpoint.Index].PkScript
			} else { // cash token data exists.
				tx.Inputs[i].CashToken = cashToken
			}

			tx.Inputs[i].Value = prevTx.TxOut[in.Outpoint.Index].Value
			tx.Inputs[i].PreviousScript = pk

			_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
			if err == nil && len(addrs) > 0 {
				tx.Inputs[i].Address = addrs[0].String()
				s.setInputSlpTokenAddress(tx.Inputs[i], addrs[0])
			}
		} else {
			blockRegion, err := s.txIndex.TxBlockRegion(ch)
			if err != nil {
				return status.Error(codes.InvalidArgument, "failed to retrieve transaction location")
			}
			if blockRegion == nil {
				return status.Error(codes.NotFound, "transaction not found")
			}

			var txBytes []byte
			err = s.db.View(func(dbTx database.Tx) error {
				var err error
				txBytes, err = dbTx.FetchBlockRegion(blockRegion)
				return err
			})
			if err != nil {
				return status.Error(codes.Internal, "failed to load transaction bytes")
			}

			loadedTx := wire.MsgTx{}
			if err := loadedTx.BchDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
				return status.Error(codes.Internal, "failed to unmarshal transaction")
			}

			pk, cashToken, err := getTokenDataForInputIfExists(loadedTx.TxOut[in.Outpoint.Index].PkScript)
			if err != nil {
				log.Debugf("could not parse token data for %v index: %v", prevTx.TxHash(), uint32(i))
			}
			if err != nil || cashToken == nil {
				pk = loadedTx.TxOut[in.Outpoint.Index].PkScript
			} else { // cash token data exists.
				tx.Inputs[i].CashToken = cashToken
			}

			tx.Inputs[i].Value = loadedTx.TxOut[in.Outpoint.Index].Value
			tx.Inputs[i].PreviousScript = pk

			_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
			if err == nil && len(addrs) > 0 {
				tx.Inputs[i].Address = addrs[0].String()
				s.setInputSlpTokenAddress(tx.Inputs[i], addrs[0])
			}

			inputTxMap[*ch] = &loadedTx
		}
	}
	return nil
}

type retrievedTx struct {
	tx          wire.MsgTx
	txBytes     []byte
	blockHeader *wire.BlockHeader
	blockHeight int32
}

func (s *GrpcServer) fetchTransactionsByAddress(addr bchutil.Address, startHeight int32, nbFetch, nbSkip int) ([]retrievedTx, error) {
	// Override the default number of requested entries if needed.  Also,
	// just return now if the number of requested entries is zero to avoid
	// extra work.
	numRequested := 100
	if nbFetch > 0 {
		numRequested = nbFetch
		if numRequested < 0 {
			numRequested = 1
		}
	}
	if numRequested == 0 {
		return nil, nil
	}

	// Override the default number of entries to skip if needed.
	var numToSkip int
	if nbSkip > 0 {
		numToSkip = nbSkip
		if numToSkip < 0 {
			numToSkip = 0
		}
	}

	// Add transactions from mempool first if client asked for reverse
	// order.  Otherwise, they will be added last (as needed depending on
	// the requested counts).
	//
	// NOTE: This code doesn't sort by dependency.  This might be something
	// to do in the future for the client's convenience, or leave it to the
	// client.
	numSkipped := uint32(0)
	addressTxns := make([]retrievedTx, 0, numRequested)

	// Fetch transactions from the database in the desired order if more are
	// needed.
	if len(addressTxns) < numRequested {
		err := s.db.View(func(dbTx database.Tx) error {
			regions, dbSkipped, err := s.addrIndex.TxRegionsForAddress(
				dbTx, addr, uint32(numToSkip)-numSkipped,
				uint32(numRequested-len(addressTxns)), true)
			if err != nil {
				return err
			}

			// Load the raw transaction bytes from the database.
			serializedTxns, err := dbTx.FetchBlockRegions(regions)
			if err != nil {
				return err
			}

			// Add the transaction and the hash of the block it is
			// contained in to the list.  Note that the transaction
			// is left serialized here since the caller might have
			// requested non-verbose output and hence there would be
			// no point in deserializing it just to reserialize it
			// later.
			for i, serializedTx := range serializedTxns {
				blockHeight, err := s.chain.BlockHeightByHash(regions[i].Hash)
				if err != nil {
					return err
				}
				if blockHeight >= startHeight {
					header, err := s.chain.HeaderByHash(regions[i].Hash)
					if err != nil {
						return err
					}
					tx := wire.MsgTx{}
					if err := tx.BchDecode(bytes.NewReader(serializedTx), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
						return err
					}
					addressTxns = append(addressTxns, retrievedTx{
						tx:          tx,
						txBytes:     serializedTx,
						blockHeight: blockHeight,
						blockHeader: &header,
					})
				}
			}
			numSkipped += dbSkipped

			return nil
		})
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "failed to load address index entries")
		}

	}

	return addressTxns, nil
}

// getSlpIndexEntry fetches an SlpIndexEntry object leveraging a cache of SlpIndexEntry items
func (s *GrpcServer) getSlpIndexEntry(hash *chainhash.Hash) (*indexers.SlpTxEntry, error) {

	if s.slpIndex == nil {
		return nil, errors.New("slpindex required")
	}

	var entry *indexers.SlpTxEntry

	// Otherwise, try to fetch from the db
	err := s.db.View(func(dbTx database.Tx) error {
		var err error
		entry, err = s.slpIndex.GetSlpIndexEntry(dbTx, hash)
		return err
	})
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// Get decimal amount from Genesis for convenience
func (s *GrpcServer) getDecimalsForTokenID(tokenID chainhash.Hash) (int, error) {
	var tokenIDHash []byte
	for i := len(tokenID) - 1; i >= 0; i-- {
		tokenIDHash = append(tokenIDHash, tokenID[i])
	}
	tokenIDRev, err := chainhash.NewHash(tokenIDHash)
	if err != nil {
		log.Criticalf("Failed to create chainhash from token ID from %s, with error: %v", hex.EncodeToString(tokenIDHash), err)
		return -1, err
	}
	genEntry, err := s.getSlpIndexEntry(tokenIDRev)
	if err != nil {
		log.Criticalf("Failed to fetch slp entry for %s, with error: %v, with error: %v", tokenIDRev, err)
		return -1, err
	}
	genSlpMsg, err := v1parser.ParseSLP(genEntry.SlpOpReturn)
	if err != nil {
		log.Criticalf("Failed to parse slp message for %v, with error: %v", tokenIDRev, err)
		return -1, err
	}
	decimals := genSlpMsg.(*v1parser.SlpGenesis).Decimals
	return decimals, nil
}

// getSlpToken fetches an SlpToken object leveraging a cache of SlpIndexEntry items
func (s *GrpcServer) getSlpToken(hash *chainhash.Hash, vout uint32, scriptPubKey []byte) (*pb.SlpToken, error) {

	if s.slpIndex == nil {
		return nil, errors.New("slpindex required")
	}

	if vout == 0 {
		return nil, errors.New("vout=0 is out of range for getSlpToken")
	}

	entry, err := s.getSlpIndexEntry(hash)
	if err != nil {
		return nil, err
	}

	var (
		isMintBaton bool
		slpAction   pb.SlpAction
		decimals    int
	)

	slpMsg, err := v1parser.ParseSLP(entry.SlpOpReturn)
	if err != nil {
		log.Criticalf("Failed to parse an slp entry message stored in the index db for txn: %v, this should never happen.", hash)
		return nil, err
	}

	// set isMintBaton and also check that vout is within proper range
	switch msg := slpMsg.(type) {
	case *v1parser.SlpGenesis:
		if msg.MintBatonVout == int(vout) {
			isMintBaton = true
		} else if vout != 1 {
			return nil, errors.New("vout is out of range for slp genesis")
		}
		if slpMsg.TokenType() == v1parser.TokenTypeFungible01 {
			slpAction = pb.SlpAction_SLP_V1_GENESIS
		} else if slpMsg.TokenType() == v1parser.TokenTypeNft1Child41 {
			slpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_GENESIS
		} else if slpMsg.TokenType() == v1parser.TokenTypeNft1Group81 {
			slpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_GENESIS
		}
		decimals = slpMsg.(*v1parser.SlpGenesis).Decimals
	case *v1parser.SlpMint:
		if msg.MintBatonVout == int(vout) {
			isMintBaton = true
		} else if vout != 1 {
			return nil, errors.New("vout is out of range for slp mint")
		}
		if slpMsg.TokenType() == v1parser.TokenTypeFungible01 {
			slpAction = pb.SlpAction_SLP_V1_MINT
		} else if slpMsg.TokenType() == v1parser.TokenTypeNft1Group81 {
			slpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_MINT
		}
		decimals, err = s.getDecimalsForTokenID(entry.TokenIDHash)
		if err != nil {
			return nil, err
		}
	case *v1parser.SlpSend:
		if int(vout) > len(msg.Amounts) {
			return nil, errors.New("vout is out of range for slp send transaction")
		}
		if slpMsg.TokenType() == v1parser.TokenTypeFungible01 {
			slpAction = pb.SlpAction_SLP_V1_SEND
		} else if slpMsg.TokenType() == v1parser.TokenTypeNft1Child41 {
			slpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_SEND
		} else if slpMsg.TokenType() == v1parser.TokenTypeNft1Group81 {
			slpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_SEND
		}
		decimals, err = s.getDecimalsForTokenID(entry.TokenIDHash)
		if err != nil {
			return nil, err
		}
	}

	// get amount value
	amount := uint64(0)
	amt, _ := slpMsg.GetVoutValue(int(vout))
	if amt != nil {
		amount = amt.Uint64()
	}

	// set the slp address string
	var address string
	if scriptPubKey != nil {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(scriptPubKey, s.chainParams)
		if err == nil && len(addrs) == 1 {
			slpAddr, err := bchutil.ConvertCashToSlpAddress(addrs[0], s.chainParams)
			if err != nil {
				log.Critical(err)
			}
			address = slpAddr.String()
		}
	}

	slpToken := &pb.SlpToken{
		TokenId:     entry.TokenIDHash[:],
		Amount:      amount,
		IsMintBaton: isMintBaton,
		Decimals:    uint32(decimals),
		SlpAction:   slpAction,
		TokenType:   getTokenType(slpMsg.TokenType()),
		Address:     address,
	}

	return slpToken, nil
}

// slpEventHandler handles valid slp transaction events from mempool and block
//
// NOTE: this is launched as a goroutine and does not return errors!
func (s *GrpcServer) slpEventHandler() {
	if s.slpIndex == nil {
		return
	}

	// track the first mempool event for starting certain services (e.g., graph search)
	firstMempoolTxnSeen := false

	// use this wait group to ensure sure the graph search db is created before we try to add
	// any txns to the graph search db
	initWg := sync.WaitGroup{}

	subscription := s.subscribeEvents()
	defer subscription.Unsubscribe()

	for {
		event := <-subscription.Events()
		switch event := event.(type) {
		case *rpcEventTxAccepted:
			txDesc := event
			log.Debugf("new mempool txn %v", txDesc.Tx.Hash())

			// kickoff slp graph search loading here
			if !firstMempoolTxnSeen {
				firstMempoolTxnSeen = true
				if s.slpIndex.GraphSearchEnabled() {
					log.Debug("starting slp graph search")
					fetchTxn := func(txnHash *chainhash.Hash) ([]byte, error) {
						txn, _, _, err := s.fetchTransactionFromBlock(txnHash)
						return txn, err
					}
					initWg.Add(1)
					go s.slpIndex.LoadSlpGraphSearchDb(fetchTxn, &initWg, &s.shutdown)
				}
			}

			// validate new slp txns
			isSlpValid := s.checkSlpTxOnEvent(txDesc.Tx.MsgTx(), "mempool")
			if isSlpValid && s.slpIndex.GraphSearchEnabled() {
				initWg.Wait()
				go s.slpIndex.AddGraphSearchTxn(txDesc.Tx.MsgTx())
			}

			continue
		case *rpcEventBlockConnected:
			continue
		}
	}
}

// checkSlpTxOnEvent is used to make sure slp information has been processed before
// returning subscriber event info to the client.  Without this, a race condition exists
// where the subscriber event can be returned before the slp validation is completed.
func (s *GrpcServer) checkSlpTxOnEvent(tx *wire.MsgTx, eventStr string) bool {
	if !isMaybeSlpTransaction(tx) {
		return false
	}
	log.Debugf("possible slp transaction added %v (%s)", tx.TxHash(), eventStr)
	err := s.db.View(func(dbTx database.Tx) error {
		valid, err := s.slpIndex.AddPotentialSlpEntries(dbTx, tx)
		if err != nil {
			return fmt.Errorf("invalid slp transaction %v (%s): %v", tx.TxHash(), eventStr, err)
		} else if valid {
			log.Debugf("valid slp transaction %v (%s)", tx.TxHash(), eventStr)
			return nil
		}
		return fmt.Errorf("invalid slp transaction in %v (%s)", tx.TxHash(), eventStr)
	})
	if err != nil {
		log.Debug(err)
		return false
	}
	return true
}

// marshalTokenMetadata returns marshalled token metadata for the provided tokenID hash
func (s *GrpcServer) marshalTokenMetadata(tokenID chainhash.Hash) (*pb.SlpTokenMetadata, error) {

	if s.slpIndex == nil {
		return nil, errors.New("slpindex required")
	}

	var tokenIDHash []byte
	for i := len(tokenID) - 1; i >= 0; i-- {
		tokenIDHash = append(tokenIDHash, tokenID[i])
	}
	tokenIDRev, err := chainhash.NewHash(tokenIDHash)
	if err != nil {
		log.Criticalf("Failed to parse token ID: %s, with error: %v", hex.EncodeToString(tokenIDHash), err)
		return nil, err
	}
	entry, err := s.getSlpIndexEntry(tokenIDRev)
	if err != nil {
		log.Criticalf("Failed to parse token ID: %s, with error: %v", hex.EncodeToString(tokenIDHash), err)
		return nil, err
	}

	slpMsg, err := v1parser.ParseSLP(entry.SlpOpReturn)
	if err != nil {
		return nil, err
	}

	genMsg, isGenesis := slpMsg.(*v1parser.SlpGenesis)
	if !isGenesis {
		return nil, errors.New("cannot build token metadata from a non-genesis entry")
	}

	tm := &pb.SlpTokenMetadata{
		TokenId:   tokenID[:],
		TokenType: getTokenType(slpMsg.TokenType()),
	}

	var dbTm *indexers.TokenMetadata
	err = s.db.View(func(dbTx database.Tx) error {
		dbTm, err = s.slpIndex.GetTokenMetadata(dbTx, entry)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Mint baton hash and NFT group id may be nil so we need to check this condition before taking a slice.
	var (
		mintBatonHash []byte
		nftGroupID    []byte
	)
	if dbTm.MintBatonHash != nil {
		mintBatonHash = dbTm.MintBatonHash[:]
	}
	if dbTm.NftGroupID != nil {
		nftGroupID = dbTm.NftGroupID[:]
	}

	switch slpMsg.TokenType() {
	case v1parser.TokenTypeFungible01:
		tm.TypeMetadata = &pb.SlpTokenMetadata_V1Fungible_{
			V1Fungible: &pb.SlpTokenMetadata_V1Fungible{
				TokenTicker:       string(genMsg.Ticker),
				TokenName:         string(genMsg.Name),
				TokenDocumentUrl:  string(genMsg.DocumentURI),
				TokenDocumentHash: genMsg.DocumentHash,
				Decimals:          uint32(genMsg.Decimals),
				MintBatonHash:     mintBatonHash,
				MintBatonVout:     dbTm.MintBatonVout,
			},
		}
	case v1parser.TokenTypeNft1Child41:
		tm.TypeMetadata = &pb.SlpTokenMetadata_V1Nft1Child{
			V1Nft1Child: &pb.SlpTokenMetadata_V1NFT1Child{
				TokenTicker:       string(genMsg.Ticker),
				TokenName:         string(genMsg.Name),
				TokenDocumentUrl:  string(genMsg.DocumentURI),
				TokenDocumentHash: genMsg.DocumentHash,
				GroupId:           nftGroupID,
			},
		}
	case v1parser.TokenTypeNft1Group81:
		tm.TypeMetadata = &pb.SlpTokenMetadata_V1Nft1Group{
			V1Nft1Group: &pb.SlpTokenMetadata_V1NFT1Group{
				TokenTicker:       string(genMsg.Ticker),
				TokenName:         string(genMsg.Name),
				TokenDocumentUrl:  string(genMsg.DocumentURI),
				TokenDocumentHash: genMsg.DocumentHash,
				Decimals:          uint32(genMsg.Decimals),
				MintBatonHash:     mintBatonHash,
				MintBatonVout:     dbTm.MintBatonVout,
			},
		}
	}

	return tm, nil
}

// getDifficultyRatio returns the proof-of-work difficulty as a multiple of the
// minimum difficulty using the passed bits field from the header of a block.
func getDifficultyRatio(bits uint32, params *chaincfg.Params) float64 {
	// The minimum difficulty is the max possible proof-of-work limit bits
	// converted back to a number.  Note this is not the same as the proof of
	// work limit directly because the block difficulty is encoded in a block
	// with the compact form which loses precision.
	max := blockchain.CompactToBig(params.PowLimitBits)
	target := blockchain.CompactToBig(bits)

	difficulty := new(big.Rat).SetFrac(max, target)
	outString := difficulty.FloatString(8)
	diff, err := strconv.ParseFloat(outString, 64)
	if err != nil {
		log.Errorf("Cannot get difficulty: %v", err)
		return 0
	}
	return diff
}

func marshalBlockInfo(block *bchutil.Block, confirmations int32, medianTime time.Time, params *chaincfg.Params) *pb.BlockInfo {
	return &pb.BlockInfo{
		Difficulty:    getDifficultyRatio(block.MsgBlock().Header.Bits, params),
		Hash:          block.Hash().CloneBytes(),
		Height:        block.Height(),
		Version:       block.MsgBlock().Header.Version,
		Timestamp:     block.MsgBlock().Header.Timestamp.Unix(),
		MerkleRoot:    block.MsgBlock().Header.MerkleRoot.CloneBytes(),
		Nonce:         block.MsgBlock().Header.Nonce,
		Bits:          block.MsgBlock().Header.Bits,
		PreviousBlock: block.MsgBlock().Header.PrevBlock.CloneBytes(),
		Confirmations: confirmations,
		Size:          int32(block.MsgBlock().SerializeSize()),
		MedianTime:    medianTime.Unix(),
	}
}

func marshalTransaction(tx *bchutil.Tx, confirmations int32, blockHeader *wire.BlockHeader, blockHeight int32, s *GrpcServer) *pb.Transaction {
	var (
		txid        = tx.Hash()
		slpMsg      v1parser.ParseResult
		params      = s.chainParams
		slpInfo     = &pb.SlpTransactionInfo{ValidityJudgement: pb.SlpTransactionInfo_UNKNOWN_OR_INVALID}
		inputAmount = big.NewInt(0)
		burnFlagSet = make(map[pb.SlpTransactionInfo_BurnFlags]struct{})
	)

	// always try to parse the transaction for slp attributes (even when slpindex is not enabled)
	if isMaybeSlpTransaction(tx.MsgTx()) {
		var err error
		slpMsg, err = v1parser.ParseSLP(tx.MsgTx().TxOut[0].PkScript)
		if err != nil {
			if err.Error() == v1parser.ErrUnsupportedSlpVersion.Error() {
				slpInfo.SlpAction = pb.SlpAction_SLP_UNSUPPORTED_VERSION
			} else {
				slpInfo.ParseError = err.Error()
				slpInfo.SlpAction = pb.SlpAction_SLP_PARSE_ERROR
			}
		} else {
			tokenID, err := goslp.GetSlpTokenID(tx.MsgTx())
			if err != nil {
				log.Criticalf("failed to parse token ID for transaction %v", txid)
			}
			slpInfo.TokenId = tokenID

			switch slpMsg.TokenType() {
			case v1parser.TokenTypeFungible01:
				switch msg := slpMsg.(type) {
				case *v1parser.SlpGenesis:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_GENESIS
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Genesis{
						V1Genesis: &pb.SlpV1GenesisMetadata{
							Name:          msg.Name,
							Ticker:        msg.Ticker,
							Decimals:      uint32(msg.Decimals),
							DocumentUrl:   msg.DocumentURI,
							DocumentHash:  msg.DocumentHash,
							MintAmount:    msg.Qty,
							MintBatonVout: uint32(msg.MintBatonVout),
						},
					}
				case *v1parser.SlpMint:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_MINT
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Mint{
						V1Mint: &pb.SlpV1MintMetadata{
							MintAmount:    msg.Qty,
							MintBatonVout: uint32(msg.MintBatonVout),
						},
					}
				case *v1parser.SlpSend:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_SEND
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Send{
						V1Send: &pb.SlpV1SendMetadata{
							Amounts: msg.Amounts,
						},
					}
				}
			case v1parser.TokenTypeNft1Child41:
				switch msg := slpMsg.(type) {
				case *v1parser.SlpGenesis:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_GENESIS
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Nft1ChildGenesis{
						V1Nft1ChildGenesis: &pb.SlpV1Nft1ChildGenesisMetadata{
							Name:         msg.Name,
							Ticker:       msg.Ticker,
							Decimals:     uint32(msg.Decimals),
							DocumentUrl:  msg.DocumentURI,
							DocumentHash: msg.DocumentHash,
							GroupTokenId: nil, // NOTE: this is populated below at the validity check
						},
					}
				case *v1parser.SlpSend:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_NFT1_UNIQUE_CHILD_SEND
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Nft1ChildSend{
						V1Nft1ChildSend: &pb.SlpV1Nft1ChildSendMetadata{
							GroupTokenId: nil, // NOTE: this is populated below at the validity check
						},
					}
				}
			case v1parser.TokenTypeNft1Group81:
				switch msg := slpMsg.(type) {
				case *v1parser.SlpGenesis:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_GENESIS
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Genesis{
						V1Genesis: &pb.SlpV1GenesisMetadata{
							Name:          msg.Name,
							Ticker:        msg.Ticker,
							Decimals:      uint32(msg.Decimals),
							DocumentUrl:   msg.DocumentURI,
							DocumentHash:  msg.DocumentHash,
							MintAmount:    msg.Qty,
							MintBatonVout: uint32(msg.MintBatonVout),
						},
					}
				case *v1parser.SlpMint:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_MINT
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Mint{
						V1Mint: &pb.SlpV1MintMetadata{
							MintAmount:    msg.Qty,
							MintBatonVout: uint32(msg.MintBatonVout),
						},
					}
				case *v1parser.SlpSend:
					slpInfo.SlpAction = pb.SlpAction_SLP_V1_NFT1_GROUP_SEND
					slpInfo.TxMetadata = &pb.SlpTransactionInfo_V1Send{
						V1Send: &pb.SlpV1SendMetadata{
							Amounts: msg.Amounts,
						},
					}
				}
			default:
				slpInfo.SlpAction = pb.SlpAction_SLP_UNSUPPORTED_VERSION
			}
		}
	} else {
		slpInfo.SlpAction = pb.SlpAction_NON_SLP
	}

	// check slp validity
	if s.slpIndex != nil {
		err := s.db.View(func(dbTx database.Tx) error {
			entry, err := s.slpIndex.GetSlpIndexEntry(dbTx, txid)
			if err != nil {
				return fmt.Errorf("slp entry does not exist for %v", txid)
			}
			slpInfo.ValidityJudgement = pb.SlpTransactionInfo_VALID

			// for nft children we populate the group token ID property in TxMetadata
			if entry.SlpVersionType == v1parser.TokenTypeNft1Child41 {
				tm, err := s.slpIndex.GetTokenMetadata(dbTx, entry)
				if err != nil {
					msg := fmt.Sprintf("missing group id metadata for nft child txid %v, tokenId: %v, tokenIdHash: %v, %v", txid, entry.TokenID, hex.EncodeToString(entry.TokenIDHash[:]), err)
					log.Critical(msg)
					return errors.New(msg)
				}
				if tm.NftGroupID != nil {
					if t, ok := slpInfo.TxMetadata.(*pb.SlpTransactionInfo_V1Nft1ChildGenesis); ok {
						t.V1Nft1ChildGenesis.GroupTokenId = tm.NftGroupID[:]
					} else if t, ok := slpInfo.TxMetadata.(*pb.SlpTransactionInfo_V1Nft1ChildSend); ok {
						t.V1Nft1ChildSend.GroupTokenId = tm.NftGroupID[:]
					} else {
						log.Criticalf("slpInfo has wrong TxMetadata type for nft child %v", txid)
					}
				} else {
					log.Criticalf("missing group id in token metadata for nft child %v", txid)
				}
			}

			return nil
		})
		if err != nil {
			log.Debug(err)
		}
	}

	respTx := &pb.Transaction{
		Hash:               txid.CloneBytes(),
		Confirmations:      confirmations,
		Version:            tx.MsgTx().Version,
		Size:               int32(tx.MsgTx().SerializeSize()),
		LockTime:           tx.MsgTx().LockTime,
		SlpTransactionInfo: slpInfo,
	}

	if blockHeader != nil {
		blockHash := blockHeader.BlockHash()
		respTx.Timestamp = blockHeader.Timestamp.Unix()
		respTx.BlockHash = blockHash.CloneBytes()
		respTx.BlockHeight = blockHeight
	}

	// loop through all inputs
	for i, input := range tx.MsgTx().TxIn {

		inputToken, err := s.getSlpToken(&input.PreviousOutPoint.Hash, input.PreviousOutPoint.Index, nil)
		if err != nil {
			log.Debugf("error: %v (input %v:%s)", err, input.PreviousOutPoint.Hash, fmt.Sprint(input.PreviousOutPoint.Index))
		}

		in := &pb.Transaction_Input{
			Index:           uint32(i),
			SignatureScript: input.SignatureScript,
			Sequence:        input.Sequence,
			Outpoint: &pb.Transaction_Input_Outpoint{
				Index: input.PreviousOutPoint.Index,
				Hash:  input.PreviousOutPoint.Hash.CloneBytes(),
			},
			SlpToken: inputToken,
		}

		signatureScript, cashToken, err := getTokenDataForInputIfExists(input.SignatureScript)
		if err != nil {
			log.Debugf("could not parse token data for %v index: %v", tx.Hash(), uint32(i))
		}
		if err != nil || cashToken == nil {
			in.SignatureScript = input.SignatureScript
		} else { // cash token data exists.
			respTx.Inputs[i].CashToken = cashToken
			in.SignatureScript = signatureScript
		}

		respTx.Inputs = append(respTx.Inputs, in)

		// add burn labels for destroyed slp inputs caused by wrong tokenID or invalid slp message
		//
		// NOTE: We do not add burn labels to 0 value slp inputs.
		if inputToken != nil && (inputToken.Amount > 0 || inputToken.IsMintBaton) {
			if slpInfo.ValidityJudgement == pb.SlpTransactionInfo_VALID {
				if !bytes.Equal(slpInfo.TokenId, inputToken.TokenId) || getTokenType(slpMsg.TokenType()) != inputToken.TokenType {
					burnFlagSet[pb.SlpTransactionInfo_BURNED_INPUTS_OTHER_TOKEN] = struct{}{}
				} else {
					inputAmount.Add(inputAmount, new(big.Int).SetUint64(inputToken.Amount))
				}
			} else if slpMsg == nil {
				burnFlagSet[pb.SlpTransactionInfo_BURNED_INPUTS_BAD_OPRETURN] = struct{}{}
			}
		}
	}

	// loop through outputs
	for i, output := range tx.MsgTx().TxOut {

		pkScript, err := output.TokenData.SeparateTokenDataFromPKScriptIfExists(output.PkScript, 0)
		if err != nil {
			log.Debugf("could not parse token data for %v index: %v", txid, uint32(i))
		}
		if pkScript != nil {
			output.PkScript = pkScript
		}

		outputToken, err := s.getSlpToken(tx.Hash(), uint32(i), output.PkScript)
		if err != nil {
			log.Debugf("no token stored for %v index: %v", txid, uint32(i))
		}

		out := &pb.Transaction_Output{
			Value:        output.Value,
			Index:        uint32(i),
			PubkeyScript: output.PkScript,
			SlpToken:     outputToken,
		}
		if !output.TokenData.IsEmpty() {
			cashToken := &pb.CashToken{
				CategoryId: output.TokenData.CategoryID[:],
				Amount:     output.TokenData.Amount,
				Commitment: output.TokenData.Commitment,
				Bitfield:   []byte{output.TokenData.BitField},
			}
			out.CashToken = cashToken
		}

		scriptClass, addrs, _, err := txscript.ExtractPkScriptAddrs(output.PkScript, params)
		if err == nil {
			if scriptClass == txscript.NullDataTy {
				out.ScriptClass = "datacarrier"
			} else {
				out.ScriptClass = scriptClass.String()
			}
			if len(addrs) > 0 {
				out.Address = addrs[0].String()
			}
		}
		disassm, err := txscript.DisasmString(output.PkScript)
		if err == nil {
			out.DisassembledScript = disassm
		}
		respTx.Outputs = append(respTx.Outputs, out)
	}

	// label slp burns caused by missing bch outputs, or input amt > output amt
	//
	// NOTE: For the sake of simplicity, the BURNED_OUTPUTS_MISSING_BCH_VOUT flag will be set even
	// when the slp output burned is a 0 token amount.
	if s.slpIndex != nil {
		if slpInfo.ValidityJudgement == pb.SlpTransactionInfo_VALID {
			switch t := slpMsg.(type) {
			case *v1parser.SlpSend:
				if len(t.Amounts) > len(tx.MsgTx().TxOut)-1 {
					burnFlagSet[pb.SlpTransactionInfo_BURNED_OUTPUTS_MISSING_BCH_VOUT] = struct{}{}
				}
				outputAmount := big.NewInt(0)
				for _, amt := range t.Amounts {
					outputAmount.Add(outputAmount, new(big.Int).SetUint64(amt))
				}
				if inputAmount.Cmp(outputAmount) > 0 {
					burnFlagSet[pb.SlpTransactionInfo_BURNED_INPUTS_GREATER_THAN_OUTPUTS] = struct{}{}
				}
			case *v1parser.SlpGenesis:
				if t.MintBatonVout > len(tx.MsgTx().TxOut)-1 {
					burnFlagSet[pb.SlpTransactionInfo_BURNED_OUTPUTS_MISSING_BCH_VOUT] = struct{}{}
				}
			case *v1parser.SlpMint:
				if t.MintBatonVout > len(tx.MsgTx().TxOut)-1 {
					burnFlagSet[pb.SlpTransactionInfo_BURNED_OUTPUTS_MISSING_BCH_VOUT] = struct{}{}
				}
			}
		} else if slpMsg != nil {
			switch t := slpMsg.(type) {
			case *v1parser.SlpSend:
				if len(t.Amounts) > len(tx.MsgTx().TxOut)-1 {
					burnFlagSet[pb.SlpTransactionInfo_BURNED_OUTPUTS_MISSING_BCH_VOUT] = struct{}{}
				}
				outputAmount := big.NewInt(0)
				for _, amt := range t.Amounts {
					outputAmount.Add(outputAmount, new(big.Int).SetUint64(amt))
				}
				if inputAmount.Cmp(outputAmount) < 0 {
					burnFlagSet[pb.SlpTransactionInfo_BURNED_INPUTS_OUTPUTS_TOO_HIGH] = struct{}{}
				}
			}
		} else if isMaybeSlpTransaction(tx.MsgTx()) && inputAmount.Cmp(big.NewInt(0)) > 0 {
			burnFlagSet[pb.SlpTransactionInfo_BURNED_INPUTS_BAD_OPRETURN] = struct{}{}
		}

		// marshal the burn flags seen in this transaction
		for flag := range burnFlagSet {
			slpInfo.BurnFlags = append(slpInfo.BurnFlags, flag)
		}
	}

	return respTx
}

// setInputMetadata will set the value, previous script, and address for each input in the mempool transaction
// from blockchain data adjusted upon the contents of the transaction pool.
// Used when no s.txIndex is available
func (s *GrpcServer) setInputMetadataFromView(respTx *pb.Transaction, txDesc *rpcEventTxAccepted, view *blockchain.UtxoViewpoint) {
	for i, in := range txDesc.Tx.MsgTx().TxIn {
		stxo := view.LookupEntry(in.PreviousOutPoint)
		if stxo != nil {

			pk, cashToken, err := getTokenDataForInputIfExists(stxo.PkScript())
			if err != nil {
				log.Debugf("could not parse token data for %v index: %v", txDesc.Tx.Hash(), uint32(i))
			}
			if err != nil || cashToken == nil {
				pk = stxo.PkScript()
			} else { // cash token data exists.
				respTx.Inputs[i].CashToken = cashToken
			}

			respTx.Inputs[i].Value = stxo.Amount()
			respTx.Inputs[i].PreviousScript = pk

			_, addrs, _, err := txscript.ExtractPkScriptAddrs(pk, s.chainParams)
			if err == nil && len(addrs) > 0 {
				respTx.Inputs[i].Address = addrs[0].String()
				s.setInputSlpTokenAddress(respTx.Inputs[i], addrs[0])
			}
		}
	}
}

// setInputSlpTokenAddress is used to apply the SlpToken.Aaddress to a transaction input
func (s *GrpcServer) setInputSlpTokenAddress(input *pb.Transaction_Input, addr bchutil.Address) {
	if s.slpIndex != nil && input.SlpToken != nil {
		slpAddr, err := bchutil.ConvertCashToSlpAddress(addr, s.chainParams)
		if err != nil {
			log.Debugf("could not convert address %s: %v", addr.String(), err)
		} else {
			input.SlpToken.Address = slpAddr.String()
		}
	}
}

// getTokenType is a helper used to map token type int to pb enum type
func getTokenType(t v1parser.TokenType) pb.SlpTokenType {
	switch t {
	case v1parser.TokenTypeFungible01:
		return pb.SlpTokenType_V1_FUNGIBLE
	case v1parser.TokenTypeNft1Group81:
		return pb.SlpTokenType_V1_NFT1_GROUP
	case v1parser.TokenTypeNft1Child41:
		return pb.SlpTokenType_V1_NFT1_CHILD
	default:
		return pb.SlpTokenType_VERSION_NOT_SET
	}
}

func getTokenDataForInputIfExists(fullBytes []byte) ([]byte, *pb.CashToken, error) {
	tokenData := wire.TokenData{}
	pk, err := tokenData.SeparateTokenDataFromPKScriptIfExists(fullBytes, 0)
	if err != nil || pk == nil {
		return fullBytes, nil, err
	}

	if !tokenData.IsEmpty() {
		cashToken := &pb.CashToken{
			CategoryId: tokenData.CategoryID[:],
			Amount:     tokenData.Amount,
			Commitment: tokenData.Commitment,
			Bitfield:   []byte{tokenData.BitField},
		}
		return pk, cashToken, nil
	}
	return fullBytes, nil, nil

}

// queueHandler manages a queue of empty interfaces, reading from in and
// sending the oldest unsent to out.  This handler stops when either of the
// in or quit channels are closed, and closes out before returning, without
// waiting to send any variables still remaining in the queue.
func queueHandler(in <-chan interface{}, out chan<- interface{}, quit <-chan struct{}) {
	var q []interface{}
	var dequeue chan<- interface{}
	skipQueue := out
	var next interface{}
out:
	for {
		select {
		case n, ok := <-in:
			if !ok {
				// Sender closed input channel.
				break out
			}

			// Either send to out immediately if skipQueue is
			// non-nil (queue is empty) and reader is ready,
			// or append to the queue and send later.
			select {
			case skipQueue <- n:
			default:
				q = append(q, n)
				dequeue = out
				skipQueue = nil
				next = q[0]
			}

		case dequeue <- next:
			copy(q, q[1:])
			q[len(q)-1] = nil // avoid leak
			q = q[:len(q)-1]
			if len(q) == 0 {
				dequeue = nil
				skipQueue = out
			} else {
				next = q[0]
			}

		case <-quit:
			break out
		}
	}
	close(out)
}
