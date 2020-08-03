package bchrpc

import (
	"bytes"
	"context"
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

	httpServer *http.Server
	subscribe  chan *rpcEventSubscription
	events     chan interface{}
	quit       chan struct{}

	wg       sync.WaitGroup
	ready    uint32 // atomic
	shutdown int32  //atomic
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
		httpServer:  cfg.HTTPServer,
		subscribe:   make(chan *rpcEventSubscription),
		events:      make(chan interface{}),
		quit:        make(chan struct{}),
		wg:          sync.WaitGroup{},
	}
	reflection.Register(cfg.Server)
	pb.RegisterBchrpcServer(cfg.Server, s)
	serviceMap["pb.bchrpc"] = s
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
	go func() {
		s.wg.Add(1)
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
			respTx := marshalTransaction(txDesc.Tx, 0, nil, 0, s.chainParams)
			stxos, err := s.txMemPool.FetchInputUtxos(txDesc.Tx)
			if err != nil {
				continue
			}
			for i, in := range txDesc.Tx.MsgTx().TxIn {
				entry := stxos.LookupEntry(in.PreviousOutPoint)
				if entry != nil {
					respTx.Inputs[i].Value = entry.Amount()
					respTx.Inputs[i].PreviousScript = entry.PkScript()

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(entry.PkScript(), s.chainParams)
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
	case &chaincfg.RegressionNetParams:
		net = pb.GetBlockchainInfoResponse_REGTEST
	case &chaincfg.SimNetParams:
		net = pb.GetBlockchainInfoResponse_SIMNET
	default:
		return nil, status.Error(codes.Internal, "unknown network parameters")
	}

	resp := &pb.GetBlockchainInfoResponse{
		AddrIndex:     s.addrIndex != nil,
		TxIndex:       s.txIndex != nil,
		BestHeight:    bestSnapShot.Height,
		BestBlockHash: bestSnapShot.Hash[:],
		BitcoinNet:    net,
		Difficulty:    getDifficultyRatio(bestSnapShot.Bits, s.chainParams),
		MedianTime:    bestSnapShot.MedianTime.Unix(),
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
			return nil, status.Error(codes.InvalidArgument, "invalid hash")
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
			return nil, status.Error(codes.InvalidArgument, "invalid hash")
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
			respTx := marshalTransaction(tx, confirmations, &header, block.Height(), s.chainParams)
			for i := range tx.MsgTx().TxIn {
				if idx > 0 {
					stxo := spentTxos[spendIdx]
					respTx.Inputs[i].Value = stxo.Amount
					respTx.Inputs[i].PreviousScript = stxo.PkScript

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(stxo.PkScript, s.chainParams)
					if err == nil && len(addrs) > 0 {
						respTx.Inputs[i].Address = addrs[0].String()
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
			return nil, status.Error(codes.InvalidArgument, "invalid hash")
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
			return nil, status.Error(codes.InvalidArgument, "invalid hash")
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
			return nil, status.Error(codes.InvalidArgument, "invalid stop hash")
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
func (s *GrpcServer) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	if s.txIndex == nil {
		return nil, status.Error(codes.Unavailable, "txindex required")
	}

	txHash, err := chainhash.NewHash(req.Hash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid transaction hash")
	}

	if txDesc, err := s.txMemPool.FetchTxDesc(txHash); err == nil {
		tx := marshalTransaction(txDesc.Tx, 0, nil, 0, s.chainParams)
		tx.Timestamp = txDesc.Added.Unix()

		view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx)
		if err == nil {
			for i, in := range txDesc.Tx.MsgTx().TxIn {
				stxo := view.LookupEntry(in.PreviousOutPoint)
				if stxo != nil {
					tx.Inputs[i].Value = stxo.Amount()
					tx.Inputs[i].PreviousScript = stxo.PkScript()

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(stxo.PkScript(), s.chainParams)
					if err == nil && len(addrs) > 0 {
						tx.Inputs[i].Address = addrs[0].String()
					}
				}
			}
		}

		resp := &pb.GetTransactionResponse{
			Transaction: tx,
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
		return nil, status.Error(codes.Internal, "failed to deserialize transaction")
	}

	header, err := s.chain.HeaderByHash(blockHash)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load block header")
	}

	respTx := marshalTransaction(bchutil.NewTx(&msgTx), s.chain.BestSnapshot().Height-blockHeight+1, &header, blockHeight, s.chainParams)
	if s.txIndex != nil {
		if err := s.setInputMetadata(respTx); err != nil {
			return nil, err
		}
	}

	resp := &pb.GetTransactionResponse{
		Transaction: respTx,
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
		return nil, status.Error(codes.InvalidArgument, "invalid transaction hash")
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
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("nbfetch exceeds max of %d", maxAddressQuerySize))
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
			return nil, status.Error(codes.InvalidArgument, "invalid hash")
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
		tx := marshalTransaction(bchutil.NewTx(&cTx.tx), tip-cTx.blockHeight+1, cTx.blockHeader, cTx.blockHeight, s.chainParams)
		if s.txIndex != nil {
			if err := s.setInputMetadata(tx); err != nil {
				return nil, err
			}
		}
		resp.ConfirmedTransactions = append(resp.ConfirmedTransactions, tx)
	}

	unconfirmedTxs := s.addrIndex.UnconfirmedTxnsForAddress(addr)
	for _, uTx := range unconfirmedTxs {
		tx := marshalTransaction(uTx, 0, nil, 0, s.chainParams)
		txDesc, err := s.txMemPool.FetchTxDesc(uTx.Hash())
		if err != nil {
			continue
		}
		view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx)
		if err == nil {
			for i, in := range txDesc.Tx.MsgTx().TxIn {
				stxo := view.LookupEntry(in.PreviousOutPoint)
				if stxo != nil {
					tx.Inputs[i].Value = stxo.Amount()
					tx.Inputs[i].PreviousScript = stxo.PkScript()

					_, addrs, _, err := txscript.ExtractPkScriptAddrs(stxo.PkScript(), s.chainParams)
					if err == nil && len(addrs) > 0 {
						tx.Inputs[i].Address = addrs[0].String()
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
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("nbfetch exceeds max of %d", maxAddressQuerySize))
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
			return nil, status.Error(codes.InvalidArgument, "invalid hash")
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

	// Attempt to decode the supplied address.
	addr, err := bchutil.DecodeAddress(req.Address, s.chainParams)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

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

			if addrs[0].EncodeAddress() == addr.EncodeAddress() {
				utxo := &pb.UnspentOutput{
					Outpoint: &pb.Transaction_Input_Outpoint{
						Hash:  txHash.CloneBytes(),
						Index: uint32(i),
					},
					Value:        entry.Amount(),
					PubkeyScript: pkScript,
					IsCoinbase:   entry.IsCoinBase(),
					BlockHeight:  entry.BlockHeight(),
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

	resp := &pb.GetAddressUnspentOutputsResponse{
		Outputs: utxos,
	}
	return resp, nil
}

// GetUnspentOutput takes an unspent output in the utxo set and returns
// the utxo metadata or not found.
func (s *GrpcServer) GetUnspentOutput(ctx context.Context, req *pb.GetUnspentOutputRequest) (*pb.GetUnspentOutputResponse, error) {
	txnHash, err := chainhash.NewHash(req.Hash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid transaction hash")
	}

	var (
		op           = wire.NewOutPoint(txnHash, req.Index)
		value        int64
		blockHeight  int32
		scriptPubkey []byte
		coinbase     bool
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

	ret := &pb.GetUnspentOutputResponse{
		Outpoint: &pb.Transaction_Input_Outpoint{
			Hash:  txnHash[:],
			Index: req.Index,
		},
		Value:        value,
		PubkeyScript: scriptPubkey,
		BlockHeight:  blockHeight,
		IsCoinbase:   coinbase,
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
		return nil, status.Error(codes.InvalidArgument, "invalid transaction hash")
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

// SubmitTransaction submits a transaction to all connected peers.
func (s *GrpcServer) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {

	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(req.Transaction)); err != nil {
		return nil, status.Error(codes.InvalidArgument, "unable to deserialize transaction")
	}

	// Use 0 for the tag to represent local node.
	tx := bchutil.NewTx(&msgTx)
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
		return nil, status.Errorf(codes.InvalidArgument, "tx rejected: %s", err.Error())
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
					respTx := marshalTransaction(txDesc.Tx, 0, nil, 0, s.chainParams)

					if view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx); err == nil {
						setInputMetadataFromView(respTx, txDesc, view, s.chainParams)
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

						respTx := marshalTransaction(tx, s.chain.BestSnapshot().Height-block.Height()+1, &header, block.Height(), s.chainParams)
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
					respTx := marshalTransaction(txDesc.Tx, 0, nil, 0, s.chainParams)

					if view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx); err == nil {
						setInputMetadataFromView(respTx, txDesc, view, s.chainParams)
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

						respTx := marshalTransaction(tx, s.chain.BestSnapshot().Height-block.Height()+1, &header, block.Height(), s.chainParams)
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
							respTx := marshalTransaction(tx, confirmations, &header, block.Height(), s.chainParams)

							for i := range tx.MsgTx().TxIn {
								if idx > 0 {
									stxo := spentTxos[spendIdx]
									respTx.Inputs[i].Value = stxo.Amount
									respTx.Inputs[i].PreviousScript = stxo.PkScript

									_, addrs, _, err := txscript.ExtractPkScriptAddrs(stxo.PkScript, s.chainParams)
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
							respTx := marshalTransaction(tx, confirmations, &header, block.Height(), s.chainParams)
							for i := range tx.MsgTx().TxIn {
								if idx > 0 {
									stxo := spentTxos[spendIdx]
									respTx.Inputs[i].Value = stxo.Amount
									respTx.Inputs[i].PreviousScript = stxo.PkScript

									_, addrs, _, err := txscript.ExtractPkScriptAddrs(stxo.PkScript, s.chainParams)
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
			tx.Inputs[i].Value = prevTx.TxOut[in.Outpoint.Index].Value
			tx.Inputs[i].PreviousScript = prevTx.TxOut[in.Outpoint.Index].PkScript

			_, addrs, _, err := txscript.ExtractPkScriptAddrs(prevTx.TxOut[in.Outpoint.Index].PkScript, s.chainParams)
			if err == nil && len(addrs) > 0 {
				tx.Inputs[i].Address = addrs[0].String()
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

			var loadedTx wire.MsgTx
			if err := loadedTx.BchDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.BaseEncoding); err != nil {
				return status.Error(codes.Internal, "failed to unmarshal transaction")
			}

			tx.Inputs[i].Value = loadedTx.TxOut[in.Outpoint.Index].Value
			tx.Inputs[i].PreviousScript = loadedTx.TxOut[in.Outpoint.Index].PkScript

			_, addrs, _, err := txscript.ExtractPkScriptAddrs(loadedTx.TxOut[in.Outpoint.Index].PkScript, s.chainParams)
			if err == nil && len(addrs) > 0 {
				tx.Inputs[i].Address = addrs[0].String()
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

func marshalTransaction(tx *bchutil.Tx, confirmations int32, blockHeader *wire.BlockHeader, blockHeight int32, params *chaincfg.Params) *pb.Transaction {
	respTx := &pb.Transaction{
		Hash:          tx.Hash().CloneBytes(),
		Confirmations: confirmations,
		Version:       tx.MsgTx().Version,
		Size:          int32(tx.MsgTx().SerializeSize()),
		LockTime:      tx.MsgTx().LockTime,
	}
	if blockHeader != nil {
		blockHash := blockHeader.BlockHash()
		respTx.Timestamp = blockHeader.Timestamp.Unix()
		respTx.BlockHash = blockHash.CloneBytes()
		respTx.BlockHeight = blockHeight

	}
	for i, input := range tx.MsgTx().TxIn {
		in := &pb.Transaction_Input{
			Index:           uint32(i),
			SignatureScript: input.SignatureScript,
			Sequence:        input.Sequence,
			Outpoint: &pb.Transaction_Input_Outpoint{
				Index: input.PreviousOutPoint.Index,
				Hash:  input.PreviousOutPoint.Hash.CloneBytes(),
			},
		}
		respTx.Inputs = append(respTx.Inputs, in)
	}
	for i, output := range tx.MsgTx().TxOut {
		out := &pb.Transaction_Output{
			Value:        output.Value,
			Index:        uint32(i),
			PubkeyScript: output.PkScript,
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
	return respTx
}

// setInputMetadata will set the value, previous script, and address for each input in the mempool transaction
// from blockchain data adjusted upon the contents of the transaction pool.
// Used when no s.txIndex is available
func setInputMetadataFromView(respTx *pb.Transaction, txDesc *rpcEventTxAccepted, view *blockchain.UtxoViewpoint, chainParams *chaincfg.Params) {
	for i, in := range txDesc.Tx.MsgTx().TxIn {
		stxo := view.LookupEntry(in.PreviousOutPoint)
		if stxo != nil {
			respTx.Inputs[i].Value = stxo.Amount()
			respTx.Inputs[i].PreviousScript = stxo.PkScript()

			_, addrs, _, err := txscript.ExtractPkScriptAddrs(stxo.PkScript(), chainParams)
			if err == nil && len(addrs) > 0 {
				respTx.Inputs[i].Address = addrs[0].String()
			}
		}
	}
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
