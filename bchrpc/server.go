package bchrpc

import (
	"bytes"
	"context"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math/big"
	"strconv"
	"sync/atomic"
)

var serviceMap = map[string]interface{}{
	"pb.bchrpc": &GrpcServer{},
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

type GrpcServerConfig struct {
	Server *grpc.Server

	TimeSource  blockchain.MedianTimeSource
	Chain       *blockchain.BlockChain
	ChainParams *chaincfg.Params
	DB          database.DB
	TxMemPool   *mempool.TxPool

	TxIndex   *indexers.TxIndex
	AddrIndex *indexers.AddrIndex
	CfIndex   *indexers.CfIndex
}

type GrpcServer struct {
	timeSource  blockchain.MedianTimeSource
	chain       *blockchain.BlockChain
	chainParams *chaincfg.Params
	db          database.DB
	txMemPool   *mempool.TxPool

	txIndex   *indexers.TxIndex
	addrIndex *indexers.AddrIndex
	cfIndex   *indexers.CfIndex

	ready uint32 // atomic
}

func NewGrpcServer(cfg *GrpcServerConfig) *GrpcServer {
	s := &GrpcServer{
		timeSource:  cfg.TimeSource,
		chain:       cfg.Chain,
		chainParams: cfg.ChainParams,
		db:          cfg.DB,
		txMemPool:   cfg.TxMemPool,
		txIndex:     cfg.TxIndex,
		addrIndex:   cfg.AddrIndex,
		cfIndex:     cfg.CfIndex,
	}
	pb.RegisterBchrpcServer(cfg.Server, s)
	serviceMap["pb.bchrpc"] = s
	return s
}

func (s *GrpcServer) Start() {
	if atomic.SwapUint32(&s.ready, 1) != 0 {
		panic("service already started")
	}
}

func (s *GrpcServer) checkReady() bool {
	return atomic.LoadUint32(&s.ready) != 0
}

// Get info about the mempool.
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

// GetBlockchainInfo info about the blockchain including the most recent
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
		MedianTime:    s.timeSource.AdjustedTime().Unix(),
	}
	return resp, nil
}

// Get info about the given block.
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

	resp := &pb.GetBlockInfoResponse{
		Info: marshalBlockInfo(block, s.chain.BestSnapshot().Height - block.Height(), s.chainParams),
	}

	nexBlock, err := s.chain.BlockByHeight(block.Height() + 1)
	if err == nil {
		resp.Info.NextBlockHash = nexBlock.Hash().CloneBytes()
	}

	return resp, nil
}

// Get a block.
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

	confirmations := s.chain.BestSnapshot().Height - block.Height()
	resp := &pb.GetBlockResponse{
		Block: &pb.Block{
			Info: marshalBlockInfo(block, confirmations, s.chainParams),
		},
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
			respTx := marshalTransaction(tx, confirmations, block, s.chainParams)
			for i := range tx.MsgTx().TxIn {
				if idx > 0 {
					stxo := spentTxos[spendIdx]
					respTx.Inputs[i].Coinbase = stxo.IsCoinBase
					respTx.Inputs[i].Value = stxo.Amount
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
	return nil, nil
}

// Get a serialized block.
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

// **Requires CfIndex**
// Get a block filter.
func (s *GrpcServer) GetBlockFilter(ctx context.Context, req *pb.GetBlockFilterRequest) (*pb.GetBlockFilterResponse, error) {
	if s.cfIndex == nil {
		return nil, status.Error(codes.Unavailable, "cfindex required")
	}

	var (
		blockHash *chainhash.Hash
		err error
	)
	if len(req.GetHash()) == 0 {
		blockHash, err = s.chain.BlockHashByHeight(req.GetHeight())
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "block not found at height %s", req.GetHeight())
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

// This RPC sends a block locator object to the server and the server responds with
// a batch of no more than 2000 headers. Upon parsing the block locator, if the server
// concludes there has been a fork, it will send headers starting at the fork point,
// or genesis if no blocks in the locator are in the best chain. If the locator is
// already at the tip no headers will be returned.
func (s *GrpcServer) GetHeaders(ctx context.Context, req *pb.GetHeadersRequest) (*pb.GetHeadersResponse, error) {
	var (
		locator blockchain.BlockLocator
		err error
	)
	for _, b := range req.BlockLocatorHashes {
		blockHash, err := chainhash.NewHash(b)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid locator hash")
		}
		locator = append(locator, blockHash)
	}
	var stopHash *chainhash.Hash
	if len(req.StopHash) > 0 {
		stopHash, err = chainhash.NewHash(req.StopHash)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid stop hash")
		}
	}

	headers := s.chain.LocateHeaders(locator, stopHash)
	resp := &pb.GetHeadersResponse{}

	var startHeight int32
	if len(headers) > 0 {
		startHash := headers[0].BlockHash()
		startHeight, err = s.chain.BlockHeightByHash(&startHash)
		if err != nil {
			return nil, status.Error(codes.Internal, "error loading start header height")
		}
	}
	for i, header := range headers {
		resp.Headers = append(resp.Headers, &pb.BlockInfo{
			Difficulty:    getDifficultyRatio(header.Bits, s.chainParams),
			Hash:          header.BlockHash().CloneBytes(),
			Height:        startHeight + int32(i),
			Version:       header.Version,
			Timestamp:     header.Timestamp.Unix(),
			MerkleRoot:    header.MerkleRoot.CloneBytes(),
			Nonce:         header.Nonce,
			Bits:          header.Bits,
			PreviousBlock: header.PrevBlock.CloneBytes(),
			Confirmations: s.chain.BestSnapshot().Height - startHeight + int32(i),
		})
	}

	return resp, nil
}

// **Requires TxIndex**
// Get a transaction given its hash.
func (s *GrpcServer) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	if s.txIndex == nil {
		return nil, status.Error(codes.Unavailable, "txindex required")
	}

	txHash, err := chainhash.NewHash(req.Hash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid transaction hash")
	}

	if tx, err := s.txMemPool.FetchTransaction(txHash); err == nil {
		resp := &pb.GetTransactionResponse{
			Transaction: marshalTransaction(tx, 0, nil, s.chainParams),
		}
		return resp, nil
	}

	// Look up the location of the transaction.
	blockRegion, err := s.txIndex.TxBlockRegion(txHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "failed to retrieve transaction location")
	}
	if blockRegion == nil {
		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	// Load the raw transaction bytes from the database.
	var txBytes []byte
	err = s.db.View(func(dbTx database.Tx) error {
		var err error
		txBytes, err = dbTx.FetchBlockRegion(blockRegion)
		return err
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load transaction bytes")
	}


	// Grab the block height.
	blk, err := s.chain.BlockByHash(blockRegion.Hash)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve block")
	}

	// Deserialize the transaction
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(txBytes))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to deserialize transaction")
	}

	resp := &pb.GetTransactionResponse{
		Transaction: marshalTransaction(bchutil.NewTx(&msgTx), s.chain.BestSnapshot().Height-blk.Height(), blk, s.chainParams),
	}

	return resp, nil
}

// **Requires TxIndex**
// Get a serialized transaction given its hash.
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

	// Look up the location of the transaction.
	blockRegion, err := s.txIndex.TxBlockRegion(txHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "failed to retrieve transaction location")
	}
	if blockRegion == nil {
		return nil, status.Error(codes.NotFound, "transaction not found")
	}

	// Load the raw transaction bytes from the database.
	var txBytes []byte
	err = s.db.View(func(dbTx database.Tx) error {
		var err error
		txBytes, err = dbTx.FetchBlockRegion(blockRegion)
		return err
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load transaction bytes")
	}

	// Deserialize the transaction
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(txBytes))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to deserialize transaction")
	}

	var buf bytes.Buffer
	if err := msgTx.BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
		return nil, status.Error(codes.Internal, "error serializing transaction")
	}
	resp := &pb.GetRawTransactionResponse{
		Transaction: buf.Bytes(),
	}

	return resp, nil
}

// **Requires AddressIndex**
// Returns the transactions for the given address. Offers offset,
// limit, and from block options.
func (s *GrpcServer) GetAddressTransactions(ctx context.Context, req *pb.GetAddressTransactionsRequest) (*pb.GetAddressTransactionsResponse, error) {
	return nil, nil
}

// **Requires AddressIndex**
// Returns the raw transactions for the given address. Offers offset,
// limit, and from block options.
func (s *GrpcServer) GetRawAddressTransactions(ctx context.Context, req *pb.GetRawAddressTransactionsRequest) (*pb.GetRawAddressTransactionsResponse, error) {
	return nil, nil
}

// **Requires TxIndex and AddressIndex**
// Returns all the unspent transaction outpoints for the given address.
// Offers offset, limit, and from block options.
func (s *GrpcServer) GetAddressUnspentOutputs(ctx context.Context, req *pb.GetAddressUnspentOutputsRequest) (*pb.GetAddressUnspentOutputsResponse, error) {
	return nil, nil
}

// **Requires TxIndex***
// Returns a merkle (SPV) proof that the given transaction is in the provided block.
func (s *GrpcServer) GetMerkleProof(ctx context.Context, req *pb.GetMerkleProofRequest) (*pb.GetMerkleProofResponse, error) {
	return nil, nil
}

// Submit a transaction to all connected peers.
func (s *GrpcServer) SubmitTransaction(ctx context.Context, req *pb.SubmitTransactionRequest) (*pb.SubmitTransactionResponse, error) {
	return nil, nil
}

// Subscribe to relevant transactions based on the subscription requests.
// The parameters to filter transactions on can be updated by sending new
// SubscribeTransactionsRequest objects on the stream.
func (s *GrpcServer) SubscribeTransactions(req *pb.SubscribeTransactionsRequest, svr pb.Bchrpc_SubscribeTransactionsServer) error {
	return nil
}

// Subscribe to notifications of new blocks being connected to the blockchain
// or blocks being disconnected.
func (s *GrpcServer) SubscribeBlocks(req *pb.SubscribeBlocksRequest, svr pb.Bchrpc_SubscribeBlocksServer) error {
	return nil
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

func marshalBlockInfo(block *bchutil.Block, confirmations int32, params *chaincfg.Params) *pb.BlockInfo {
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
	}
}

func marshalTransaction(tx *bchutil.Tx, confirmations int32, block *bchutil.Block, params *chaincfg.Params) *pb.Transaction {
	respTx := &pb.Transaction{
		Hash: tx.Hash().CloneBytes(),
		Confirmations: confirmations,
		Version: tx.MsgTx().Version,
		Size: int32(tx.MsgTx().SerializeSize()),
		LockTime: tx.MsgTx().LockTime,
	}
	if block != nil {
		respTx.Timestamp = block.MsgBlock().Header.Timestamp.Unix()
		respTx.BlockHash = block.Hash().CloneBytes()
		respTx.BlockHeight = block.Height()

	}
	for i, input := range tx.MsgTx().TxIn {
		in := &pb.Transaction_Input{
			Index: uint32(i),
			SignatureScript: input.SignatureScript,
			Sequence: input.Sequence,
			Outpoint: &pb.Transaction_Input_Outpoint{
				Index: input.PreviousOutPoint.Index,
				Hash: input.PreviousOutPoint.Hash.CloneBytes(),
			},
		}
		respTx.Inputs = append(respTx.Inputs, in)
	}
	for i, output := range tx.MsgTx().TxOut {
		out := &pb.Transaction_Output{
			Value: output.Value,
			Index: uint32(i),
			PubkeyScript: output.PkScript,
		}
		scriptClass, addrs, _, err := txscript.ExtractPkScriptAddrs(output.PkScript, params)
		if err == nil {
			out.ScriptClass = scriptClass.String()
			out.Address = addrs[0].String()
		}
		disassm, err := txscript.DisasmString(output.PkScript)
		if err == nil {
			out.DisassembledScript = disassm
		}
		respTx.Outputs = append(respTx.Outputs, out)
	}
	return respTx
}