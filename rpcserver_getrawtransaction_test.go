// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// getRawTxTest is a regtest chain whose first coinbase, which pays to OP_TRUE,
// is mature, with a mempool and an RPC server on both.
type getRawTxTest struct {
	s     *rpcServer
	chain *blockchain.BlockChain
	pool  *mempool.TxPool
	first *bchutil.Block
}

// newGetRawTxTest returns a getRawTxTest, with a transaction index when
// withTxIndex is set.
func newGetRawTxTest(t *testing.T, withTxIndex bool) *getRawTxTest {
	t.Helper()
	params := &chaincfg.RegressionNetParams
	db := newRPCTestDB(t)
	var txIndex *indexers.TxIndex
	var indexManager blockchain.IndexManager
	if withTxIndex {
		txIndex = indexers.NewTxIndex(db)
		indexManager = indexers.NewManager(db,
			[]indexers.Indexer{txIndex})
	}
	blockchain.UseLogger(bchlog.Disabled)
	t.Cleanup(func() { blockchain.UseLogger(chanLog) })
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        params,
		TimeSource:         blockchain.NewMedianTime(),
		IndexManager:       indexManager,
		UtxoCacheMaxSize:   1 << 20,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatal(err)
	}

	first := mineTestBlock(t, chain, nil)
	for range params.CoinbaseMaturity {
		mineTestBlock(t, chain, nil)
	}
	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			MaxTxVersion:    2,
			AcceptNonStd:    true,
			MinRelayTxFee:   mempool.DefaultMinRelayTxFee,
			MaxOrphanTxs:    5,
			MaxOrphanTxSize: 1000,
		},
		ChainParams:   params,
		FetchUtxoView: chain.FetchUtxoView,
		BestHeight:    func() int32 { return chain.BestSnapshot().Height },
		MedianTimePast: func() time.Time {
			return chain.BestSnapshot().MedianTime
		},
		CalcSequenceLock: func(tx *bchutil.Tx, view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return chain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: chain.IsDeploymentActive,
	})
	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: params,
		Chain:       chain,
		DB:          db,
		TxMemPool:   pool,
	}}
	if withTxIndex {
		s.cfg.TxIndex = txIndex
	}
	return &getRawTxTest{s: s, chain: chain, pool: pool, first: first}
}

// getRawTxTestScript is long enough for the test transactions to pass the
// minimum size, and anyone can spend it.
var getRawTxTestScript = append(bytes.Repeat([]byte{txscript.OP_NOP}, 10),
	txscript.OP_TRUE)

// spendCoinbase returns a transaction spending the coinbase of blk into two
// outputs, leaving a fee of 1000 satoshis.
func spendCoinbase(blk *bchutil.Block) *wire.MsgTx {
	coinbase := blk.Transactions()[0]
	value := coinbase.MsgTx().TxOut[0].Value
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: *coinbase.Hash()}, nil))
	tx.AddTxOut(wire.NewTxOut(value/2, getRawTxTestScript, wire.TokenData{}))
	tx.AddTxOut(wire.NewTxOut(value/2-1000, getRawTxTestScript,
		wire.TokenData{}))
	return tx
}

// getRawTx runs getrawtransaction and returns its result as a client decodes
// it, with the JSON it was decoded from.
func getRawTx(t *testing.T, s *rpcServer, txid string, verbosity btcjson.VerboseLevel, blockHash *string) (btcjson.TxRawResult, string) {
	t.Helper()
	result, err := handleGetRawTransaction(s,
		btcjson.NewGetRawTransactionCmd(txid, &verbosity, blockHash, nil),
		nil)
	if err != nil {
		t.Fatalf("getrawtransaction %s %d: %v", txid, verbosity, err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var reply btcjson.TxRawResult
	if err := json.Unmarshal(raw, &reply); err != nil {
		t.Fatalf("getrawtransaction %s %d: %v: %s", txid, verbosity,
			err, raw)
	}
	return reply, string(raw)
}

// getRawTxError runs getrawtransaction and fails the test unless it returns
// the RPC error with the passed code and message.
func getRawTxError(t *testing.T, s *rpcServer, what string, cmd *btcjson.GetRawTransactionCmd, code btcjson.RPCErrorCode, msg string) {
	t.Helper()
	_, err := handleGetRawTransaction(s, cmd, nil)
	assertRPCError(t, what, err, code, msg)
}

// assertSpentOutput fails the test unless the input gives, as BCHN's
// getrawtransaction gives at verbosity 2, the value and script of the output
// it spends in the input itself, and no prevout object.
func assertSpentOutput(t *testing.T, what string, vin btcjson.Vin, out *wire.TxOut) {
	t.Helper()
	if vin.PrevOut != nil {
		t.Errorf("%s: unexpected prevout %+v", what, vin.PrevOut)
	}
	if want := bchutil.Amount(out.Value).ToBCH(); vin.Value == nil ||
		*vin.Value != want {

		t.Errorf("%s: value %v, want %v", what, vin.Value, want)
	}
	if want := hex.EncodeToString(out.PkScript); vin.ScriptPubKey == nil ||
		vin.ScriptPubKey.Hex != want {

		t.Errorf("%s: script %+v, want %s", what, vin.ScriptPubKey, want)
	}
}

// assertFee fails the test unless the result gives the passed fee in
// satoshis, or no fee for a fee of -1.
func assertFee(t *testing.T, what string, reply btcjson.TxRawResult, fee int64) {
	t.Helper()
	switch {
	case fee < 0 && reply.Fee != nil:
		t.Errorf("%s: unexpected fee %v", what, *reply.Fee)
	case fee >= 0 && (reply.Fee == nil ||
		*reply.Fee != bchutil.Amount(fee).ToBCH()):

		t.Errorf("%s: fee %v, want %v", what, reply.Fee,
			bchutil.Amount(fee).ToBCH())
	}
}

// TestGetRawTransactionBlockHash ensures getrawtransaction finds a confirmed
// transaction in the block it is given without the transaction index, gives
// at verbosity 2 the outputs its inputs spend and its fee as BCHN does, and
// answers a block that does not hold it, an unknown block and a block off the
// main chain as BCHN does.
func TestGetRawTransactionBlockHash(t *testing.T) {
	test := newGetRawTxTest(t, false)
	s := test.s
	spend := spendCoinbase(test.first)
	block := mineTestBlock(t, test.chain, nil, spend)
	blockHash := block.Hash().String()
	txid := spend.TxHash().String()

	// Verbosity 0 gives the serialized transaction.
	result, err := handleGetRawTransaction(s,
		btcjson.NewGetRawTransactionCmd(txid, nil, &blockHash, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := messageToHex(spend); result != want {
		t.Errorf("verbosity 0: got %v, want %s", result, want)
	}

	// Verbosity 1 gives the block, and that it is in the main chain, but
	// not the outputs the inputs spend.
	reply, raw := getRawTx(t, s, txid, 1, &blockHash)
	if reply.Txid != txid || reply.BlockHash != blockHash ||
		reply.Confirmations != 1 ||
		reply.Time != block.MsgBlock().Header.Timestamp.Unix() ||
		reply.InActiveChain == nil || !*reply.InActiveChain {

		t.Errorf("verbosity 1: unexpected result %s", raw)
	}
	assertFee(t, "verbosity 1", reply, -1)
	if reply.Vin[0].Value != nil || reply.Vin[0].ScriptPubKey != nil {
		t.Errorf("verbosity 1: unexpected spent output %s", raw)
	}

	// Verbosity 2 adds them from the spend journal, and the fee.
	reply, raw = getRawTx(t, s, txid, 2, &blockHash)
	if reply.InActiveChain == nil || !*reply.InActiveChain ||
		reply.Confirmations != 1 || reply.Hex == "" {

		t.Errorf("verbosity 2: unexpected result %s", raw)
	}
	coinbaseOut := test.first.MsgBlock().Transactions[0].TxOut[0]
	assertSpentOutput(t, "verbosity 2", reply.Vin[0], coinbaseOut)
	assertFee(t, "verbosity 2", reply, 1000)
	if bytes.Contains([]byte(raw), []byte(`"prevout"`)) {
		t.Errorf("verbosity 2: unexpected prevout %s", raw)
	}

	// A coinbase spends no output, so it has none and no fee.
	coinbaseID := block.Transactions()[0].Hash().String()
	reply, raw = getRawTx(t, s, coinbaseID, 2, &blockHash)
	assertFee(t, "coinbase", reply, -1)
	if reply.Vin[0].Coinbase == "" || reply.Vin[0].Value != nil {
		t.Errorf("coinbase: unexpected input %s", raw)
	}

	// Only the given block is searched.
	firstHash := test.first.Hash().String()
	getRawTxError(t, s, "wrong block",
		btcjson.NewGetRawTransactionCmd(txid, nil, &firstHash, nil),
		btcjson.ErrRPCInvalidAddressOrKey,
		"No such transaction found in the provided block. Use "+
			"gettransaction for wallet transactions.")
	unknown := chainhash.Hash{1}.String()
	getRawTxError(t, s, "unknown block",
		btcjson.NewGetRawTransactionCmd(txid, nil, &unknown, nil),
		btcjson.ErrRPCInvalidAddressOrKey, "Block hash not found")
	notHex := "zz"
	_, err = handleGetRawTransaction(s,
		btcjson.NewGetRawTransactionCmd(txid, nil, &notHex, nil), nil)
	if rpcErr, ok := err.(*btcjson.RPCError); !ok ||
		rpcErr.Code != btcjson.ErrRPCDecodeHexString {

		t.Errorf("block hash not hex: got %v, want a decode error", err)
	}

	// An empty block hash is no block hash, so without the transaction
	// index a confirmed transaction is not found.
	empty := ""
	getRawTxError(t, s, "empty block hash",
		btcjson.NewGetRawTransactionCmd(txid, nil, &empty, nil),
		btcjson.ErrRPCNoTxInfo, "The transaction index must be enabled "+
			"to query the blockchain (specify --txindex)")

	// A block off the main chain is searched too.  It has no spend
	// journal, so verbosity 2 gives no spent outputs and no fee, and as in
	// BCHN no confirmations and no times.
	sideSpend := spendCoinbase(blockAtHeight(t, test.chain, 2))
	prev := block.MsgBlock().Header.PrevBlock
	side := mineTestBlock(t, test.chain, &prev, sideSpend)
	if test.chain.MainChainHasBlock(side.Hash()) {
		t.Fatal("side block is in the main chain")
	}
	sideHash := side.Hash().String()
	reply, raw = getRawTx(t, s, sideSpend.TxHash().String(), 2, &sideHash)
	if reply.BlockHash != sideHash || reply.InActiveChain == nil ||
		*reply.InActiveChain || reply.Confirmations != 0 ||
		reply.Time != 0 || reply.Blocktime != 0 ||
		reply.Vin[0].Value != nil {

		t.Errorf("side block: unexpected result %s", raw)
	}
	assertFee(t, "side block", reply, -1)

	// A block known only by its header, as fastsync adds them, is not
	// available.
	header := block.MsgBlock().Header
	header.PrevBlock = *block.Hash()
	header.Timestamp = header.Timestamp.Add(time.Second)
	if err := test.chain.AddHeader(&header); err != nil {
		t.Fatal(err)
	}
	headerHash := header.BlockHash().String()
	getRawTxError(t, s, "block without data",
		btcjson.NewGetRawTransactionCmd(txid, nil, &headerHash, nil),
		btcjson.ErrRPCMisc, "Block not available")
}

// blockAtHeight returns the main chain block at the passed height.
func blockAtHeight(t *testing.T, chain *blockchain.BlockChain, height int32) *bchutil.Block {
	t.Helper()
	blk, err := chain.BlockByHeight(height)
	if err != nil {
		t.Fatal(err)
	}
	return blk
}

// TestGetRawTransactionParams ensures getrawtransaction refuses the
// verbosity levels BCHN refuses, byte code patterns, which bchd does not make,
// and the coinbase of the genesis block, with BCHN's errors.
func TestGetRawTransactionParams(t *testing.T) {
	test := newGetRawTxTest(t, false)
	s := test.s
	txid := test.first.Transactions()[0].Hash().String()
	blockHash := test.first.Hash().String()

	for _, level := range []btcjson.VerboseLevel{3, -1} {
		getRawTxError(t, s, "verbosity",
			btcjson.NewGetRawTransactionCmd(txid, &level, &blockHash, nil),
			btcjson.ErrRPCInvalidAddressOrKey,
			fmt.Sprintf("Wrong verbosity level %d", level))
	}

	getRawTxError(t, s, "patterns",
		btcjson.NewGetRawTransactionCmd(txid, nil, &blockHash,
			btcjson.Bool(true)),
		btcjson.ErrRPCInvalidParameter,
		"Byte code patterns are not supported")
	_, err := handleGetRawTransaction(s, btcjson.NewGetRawTransactionCmd(
		txid, nil, &blockHash, btcjson.Bool(false)), nil)
	if err != nil {
		t.Errorf("patterns false: %v", err)
	}

	params := &chaincfg.RegressionNetParams
	genesisTx := params.GenesisBlock.Header.MerkleRoot.String()
	genesisHash := params.GenesisHash.String()
	getRawTxError(t, s, "genesis coinbase",
		btcjson.NewGetRawTransactionCmd(genesisTx, nil, &genesisHash, nil),
		btcjson.ErrRPCInvalidAddressOrKey,
		"The genesis block coinbase is not considered an ordinary "+
			"transaction and cannot be retrieved")
}

// TestGetRawTransactionMempoolVerbosity2 ensures getrawtransaction gives at
// verbosity 2 the outputs the inputs of a mempool transaction spend, from the
// UTXO set and from the mempool, with their token data, and its fee, and that
// with a block hash the mempool is not searched.
func TestGetRawTransactionMempoolVerbosity2(t *testing.T) {
	test := newGetRawTxTest(t, false)
	s := test.s

	// The parent spends the first coinbase, making fungible tokens of the
	// category of that coinbase, and the child spends them.
	coinbase := test.first.Transactions()[0]
	value := coinbase.MsgTx().TxOut[0].Value
	amount := uint64(1000)
	tokenData, err := wire.NewTokenData(*coinbase.Hash(), &amount, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent := wire.NewMsgTx(2)
	parent.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: *coinbase.Hash()}, nil))
	parent.AddTxOut(wire.NewTxOut(value/2, getRawTxTestScript, *tokenData))
	parent.AddTxOut(wire.NewTxOut(value/2-1000, getRawTxTestScript,
		wire.TokenData{}))
	child := wire.NewMsgTx(2)
	child.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: parent.TxHash()}, nil))
	child.AddTxOut(wire.NewTxOut(value/2-2000, getRawTxTestScript,
		*tokenData))
	for _, tx := range []*wire.MsgTx{parent, child} {
		_, err := test.pool.ProcessTransaction(bchutil.NewTx(tx), false, 0)
		if err != nil {
			t.Fatalf("transaction not accepted: %v", err)
		}
	}

	reply, raw := getRawTx(t, s, parent.TxHash().String(), 2, nil)
	assertSpentOutput(t, "parent", reply.Vin[0], coinbase.MsgTx().TxOut[0])
	assertFee(t, "parent", reply, 1000)
	if reply.Vin[0].TokenData != nil || reply.BlockHash != "" ||
		reply.InActiveChain != nil {

		t.Errorf("parent: unexpected result %s", raw)
	}

	reply, raw = getRawTx(t, s, child.TxHash().String(), 2, nil)
	assertSpentOutput(t, "child", reply.Vin[0], parent.TxOut[0])
	assertFee(t, "child", reply, 2000)
	token := reply.Vin[0].TokenData
	if token == nil || token.Category != coinbase.Hash().String() ||
		token.Amount != "1000" {

		t.Errorf("child: unexpected token data %s", raw)
	}

	reply, raw = getRawTx(t, s, child.TxHash().String(), 1, nil)
	assertFee(t, "verbosity 1", reply, -1)
	if reply.Vin[0].Value != nil || reply.Vin[0].TokenData != nil {
		t.Errorf("verbosity 1: unexpected spent output %s", raw)
	}

	tip := test.chain.BestSnapshot().Hash.String()
	getRawTxError(t, s, "mempool transaction with a block hash",
		btcjson.NewGetRawTransactionCmd(child.TxHash().String(), nil,
			&tip, nil),
		btcjson.ErrRPCInvalidAddressOrKey,
		"No such transaction found in the provided block. Use "+
			"gettransaction for wallet transactions.")
}

// TestGetRawTransactionTxIndexVerbosity2 ensures getrawtransaction gives at
// verbosity 2 the outputs the inputs of a transaction it finds with the
// transaction index spend, from the spend journal of its block, without
// saying whether the block is in the main chain, as no block was given.
func TestGetRawTransactionTxIndexVerbosity2(t *testing.T) {
	test := newGetRawTxTest(t, true)
	spend := spendCoinbase(test.first)
	block := mineTestBlock(t, test.chain, nil, spend)
	mineTestBlock(t, test.chain, nil)

	txid := spend.TxHash().String()
	reply, raw := getRawTx(t, test.s, txid, 2, nil)
	if reply.BlockHash != block.Hash().String() ||
		reply.Confirmations != 2 || reply.InActiveChain != nil {

		t.Errorf("verbosity 2: unexpected result %s", raw)
	}
	coinbaseOut := test.first.MsgBlock().Transactions[0].TxOut[0]
	assertSpentOutput(t, "verbosity 2", reply.Vin[0], coinbaseOut)
	assertFee(t, "verbosity 2", reply, 1000)

	reply, raw = getRawTx(t, test.s, txid, 1, nil)
	assertFee(t, "verbosity 1", reply, -1)
	if reply.BlockHash != block.Hash().String() || reply.Vin[0].Value != nil {
		t.Errorf("verbosity 1: unexpected result %s", raw)
	}
}

// TestSearchRawTransactionsVerboseLevel ensures searchrawtransactions still
// refuses verbose levels other than 0 and 1, which its type now passes on
// since getrawtransaction takes level 2.
func TestSearchRawTransactionsVerboseLevel(t *testing.T) {
	s, addr := newSearchRawTxTestServer(t)
	for _, level := range []btcjson.VerboseLevel{2, -1} {
		cmd := btcjson.NewSearchRawTransactionsCmd(addr.EncodeAddress(),
			&level, nil, nil, nil, nil, nil)
		_, err := searchRawTransactions(t, s, cmd)
		rpcErr, ok := err.(*btcjson.RPCError)
		if !ok || rpcErr.Code != btcjson.ErrRPCInvalidParameter {
			t.Errorf("level %d: got %v, want an invalid parameter "+
				"error", level, err)
		}
	}
}
