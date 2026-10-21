// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"math"
	"strconv"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// newRPCTestDB returns an empty regtest database in a temporary directory.
// The database and index loggers are disabled for the test, since the log
// rotator is not set up.
func newRPCTestDB(t *testing.T) database.DB {
	t.Helper()
	database.UseLogger(bchlog.Disabled)
	indexers.UseLogger(bchlog.Disabled)
	t.Cleanup(func() {
		database.UseLogger(bcdbLog)
		indexers.UseLogger(indxLog)
	})
	db, err := database.Create("ffldb", t.TempDir(),
		chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// newSearchRawTxTestServer returns an RPC server with an empty address index
// and an address that pays to a zero hash.
func newSearchRawTxTestServer(t *testing.T) (*rpcServer, bchutil.Address) {
	t.Helper()
	params := &chaincfg.RegressionNetParams
	db := newRPCTestDB(t)
	idx := indexers.NewAddrIndex(db, params)
	if err := db.Update(idx.Create); err != nil {
		t.Fatal(err)
	}
	addr, err := bchutil.NewAddressPubKeyHash(make([]byte, 20), params)
	if err != nil {
		t.Fatal(err)
	}
	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: params,
		DB:          db,
		AddrIndex:   idx,
	}}
	return s, addr
}

// addMempoolTxnsForAddress adds n unconfirmed transactions paying to addr to
// the address index.
func addMempoolTxnsForAddress(t *testing.T, s *rpcServer, addr bchutil.Address, n int) {
	t.Helper()
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		tx := wire.NewMsgTx(1)
		tx.LockTime = uint32(i) // distinct txids
		tx.AddTxOut(wire.NewTxOut(1000, pkScript, wire.TokenData{}))
		s.cfg.AddrIndex.AddUnconfirmedTx(bchutil.NewTx(tx),
			blockchain.NewUtxoViewpoint())
	}
}

// searchRawTransactions runs searchrawtransactions and turns a panic into a
// test failure.
func searchRawTransactions(t *testing.T, s *rpcServer, cmd *btcjson.SearchRawTransactionsCmd) (result interface{}, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	return handleSearchRawTransactions(s, cmd, nil)
}

// TestHandleSearchRawTransactionsCountLimit ensures a huge count is rejected
// instead of being used as an allocation size.
func TestHandleSearchRawTransactionsCountLimit(t *testing.T) {
	s, addr := newSearchRawTxTestServer(t)
	verbose := btcjson.VerboseLevel(0)
	for _, count := range []int{maxSearchRawTxCount + 1, math.MaxInt32, math.MaxInt} {
		cmd := btcjson.NewSearchRawTransactionsCmd(addr.EncodeAddress(),
			&verbose, nil, &count, nil, nil, nil)
		_, err := searchRawTransactions(t, s, cmd)
		rpcErr, ok := err.(*btcjson.RPCError)
		if !ok || rpcErr.Code != btcjson.ErrRPCInvalidParameter {
			t.Errorf("count %d: err %v, want an invalid parameter error",
				count, err)
		}
	}

	// The largest allowed count is still accepted.
	count := maxSearchRawTxCount
	addMempoolTxnsForAddress(t, s, addr, 1)
	cmd := btcjson.NewSearchRawTransactionsCmd(addr.EncodeAddress(),
		&verbose, nil, &count, nil, nil, nil)
	result, err := searchRawTransactions(t, s, cmd)
	if txns, ok := result.([]string); err != nil || !ok || len(txns) != 1 {
		t.Errorf("count %d: got %v, %v, want one transaction", count,
			result, err)
	}
}

// TestHandleSearchRawTransactionsSkipRange ensures skips past the 32-bit
// range skip every transaction instead of wrapping onto a small skip.
func TestHandleSearchRawTransactionsSkipRange(t *testing.T) {
	s, addr := newSearchRawTxTestServer(t)
	addMempoolTxnsForAddress(t, s, addr, 2)
	verbose := btcjson.VerboseLevel(0)
	count := 10
	// Skips past the 32-bit range only fit in an int on 64-bit platforms.
	skips := []int{2, math.MaxInt}
	if strconv.IntSize == 64 {
		for _, skip := range []uint64{math.MaxUint32, 1 << 32, 1<<32 + 1} {
			skips = append(skips, int(skip))
		}
	}
	for _, skip := range skips {
		for _, reverse := range []bool{false, true} {
			cmd := btcjson.NewSearchRawTransactionsCmd(
				addr.EncodeAddress(), &verbose, &skip, &count, nil,
				&reverse, nil)
			result, err := searchRawTransactions(t, s, cmd)
			rpcErr, ok := err.(*btcjson.RPCError)
			if !ok || rpcErr.Code != btcjson.ErrRPCNoTxInfo {
				t.Errorf("skip %d reverse %v: got %v, %v, want no "+
					"information error", skip, reverse, result, err)
			}
		}
	}

	// A negative skip skips nothing.
	skip := -1
	cmd := btcjson.NewSearchRawTransactionsCmd(addr.EncodeAddress(),
		&verbose, &skip, &count, nil, nil, nil)
	result, err := searchRawTransactions(t, s, cmd)
	if txns, ok := result.([]string); err != nil || !ok || len(txns) != 2 {
		t.Errorf("skip -1: got %v, %v, want two transactions", result, err)
	}
}

// TestFetchMempoolTxnsForAddressSkipOverflow ensures skip plus count can't
// wrap around and produce an inverted range.
func TestFetchMempoolTxnsForAddressSkipOverflow(t *testing.T) {
	s, addr := newSearchRawTxTestServer(t)
	addMempoolTxnsForAddress(t, s, addr, 12)

	tests := []struct {
		skip, count uint32
		want        int
	}{
		{10, math.MaxUint32 - 5, 2},
		{0, math.MaxUint32, 12},
		{11, 5, 1},
		{12, 5, 0},
		{math.MaxUint32, math.MaxUint32, 0},
	}
	for _, test := range tests {
		var txns []*bchutil.Tx
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("skip %d count %d: panic: %v",
						test.skip, test.count, r)
				}
			}()
			txns, _ = fetchMempoolTxnsForAddress(s, addr, test.skip,
				test.count)
		}()
		if len(txns) != test.want {
			t.Errorf("skip %d count %d: got %d txns, want %d",
				test.skip, test.count, len(txns), test.want)
		}
	}
}

// newRPCTestChain returns a regtest chain holding only the genesis block and
// the database backing it.
func newRPCTestChain(t *testing.T) (*blockchain.BlockChain, database.DB) {
	t.Helper()
	db := newRPCTestDB(t)
	blockchain.UseLogger(bchlog.Disabled)
	t.Cleanup(func() { blockchain.UseLogger(chanLog) })
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &chaincfg.RegressionNetParams,
		TimeSource:         blockchain.NewMedianTime(),
		UtxoCacheMaxSize:   1 << 20,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	return chain, db
}

// callTxOutProofHandler runs a gettxoutproof or verifytxoutproof handler and
// turns a panic into a test failure.
func callTxOutProofHandler(t *testing.T, handler commandHandler, s *rpcServer, cmd interface{}) (result interface{}, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	return handler(s, cmd, nil)
}

// assertRPCError fails the test unless err is an RPC error with the given
// code and message.
func assertRPCError(t *testing.T, what string, err error, code btcjson.RPCErrorCode, msg string) {
	t.Helper()
	rpcErr, ok := err.(*btcjson.RPCError)
	if !ok || rpcErr.Code != code || rpcErr.Message != msg {
		t.Errorf("%s: got error %v, want %d: %s", what, err, code, msg)
	}
}

// TestHandleGetTxOutProofErrors ensures an empty txid list without a block
// hash and a transaction missing from the index are answered with errors,
// and that an empty txid list with a block hash gets a proof.
func TestHandleGetTxOutProofErrors(t *testing.T) {
	chain, db := newRPCTestChain(t)
	txIndex := indexers.NewTxIndex(db)
	if err := db.Update(txIndex.Create); err != nil {
		t.Fatal(err)
	}
	for _, txIndex := range []*indexers.TxIndex{nil, txIndex} {
		s := &rpcServer{cfg: rpcserverConfig{
			ChainParams: &chaincfg.RegressionNetParams,
			Chain:       chain,
			DB:          db,
			TxIndex:     txIndex,
		}}
		cmd := btcjson.NewGetTxOutProofCmd([]string{}, nil)
		_, err := callTxOutProofHandler(t, handleGetTxOutProof, s, cmd)
		assertRPCError(t, "no block hash", err,
			btcjson.ErrRPCInvalidAddressOrKey, "Transaction not yet in block")

		genesisHash := chaincfg.RegressionNetParams.GenesisHash.String()
		cmd = btcjson.NewGetTxOutProofCmd([]string{}, &genesisHash)
		if _, err := callTxOutProofHandler(t, handleGetTxOutProof, s, cmd); err != nil {
			t.Errorf("genesis block hash: %v", err)
		}
	}

	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Chain:       chain,
		DB:          db,
		TxIndex:     txIndex,
	}}
	unknown := chainhash.Hash{1}
	cmd := btcjson.NewGetTxOutProofCmd([]string{unknown.String()}, nil)
	_, err := callTxOutProofHandler(t, handleGetTxOutProof, s, cmd)
	assertRPCError(t, "unknown txid", err, btcjson.ErrRPCNoTxInfo,
		"Transaction not yet in block")

	genesisHash := chaincfg.RegressionNetParams.GenesisHash.String()
	cmd = btcjson.NewGetTxOutProofCmd([]string{unknown.String()},
		&genesisHash)
	_, err = callTxOutProofHandler(t, handleGetTxOutProof, s, cmd)
	assertRPCError(t, "txid not in block", err,
		btcjson.ErrRPCInvalidAddressOrKey,
		"Not all transactions found in specified or retrieved block")
}

// mineTestBlock solves a block holding txns on prev, or on the chain's tip
// when prev is nil, and processes it.
func mineTestBlock(t *testing.T, chain *blockchain.BlockChain, prev *chainhash.Hash, txns ...*wire.MsgTx) *bchutil.Block {
	t.Helper()
	gen := mining.NewBlkTmplGenerator(&mining.Policy{BlockMaxSize: 1000000},
		&chaincfg.RegressionNetParams, noTxSource{}, chain,
		blockchain.NewMedianTime(), txscript.NewSigCache(10),
		txscript.NewHashCache(10))
	tmpl, err := gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatal(err)
	}
	msg := tmpl.Block
	msg.Transactions = append(msg.Transactions, txns...)
	merkles := blockchain.BuildMerkleTreeStore(
		bchutil.NewBlock(msg).Transactions())
	msg.Header.MerkleRoot = *merkles[len(merkles)-1]
	if prev != nil {
		msg.Header.PrevBlock = *prev
	}
	target := blockchain.CompactToBig(msg.Header.Bits)
	for {
		hash := msg.Header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			break
		}
		msg.Header.Nonce++
	}
	block := bchutil.NewBlock(msg)
	if _, _, err := chain.ProcessBlock(block, blockchain.BFNone); err != nil {
		t.Fatalf("process block: %v", err)
	}
	return block
}

// decodeTestProof decodes a hex encoded merkle block.
func decodeTestProof(t *testing.T, proof string) *wire.MsgMerkleBlock {
	t.Helper()
	var msg wire.MsgMerkleBlock
	err := msg.BchDecode(bytes.NewReader(mustDecodeHex(t, proof)),
		wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		t.Fatal(err)
	}
	return &msg
}

// encodeTestProof hex encodes a merkle block.
func encodeTestProof(t *testing.T, msg *wire.MsgMerkleBlock) string {
	t.Helper()
	var buf bytes.Buffer
	err := msg.BchEncode(&buf, wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(buf.Bytes())
}

// TestHandleVerifyTxOutProof ensures verifytxoutproof works without the
// transaction index, and that it only returns matches for a proof whose
// header is in the main chain, whose merkle root matches the header and
// whose transaction count matches the block.
func TestHandleVerifyTxOutProof(t *testing.T) {
	chain, db := newRPCTestChain(t)
	mining.UseLogger(bchlog.Disabled)
	t.Cleanup(func() { mining.UseLogger(minrLog) })
	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Chain:       chain,
		DB:          db,
	}}
	getProof := func(blockHash *chainhash.Hash, txHash *chainhash.Hash) string {
		t.Helper()
		blockStr := blockHash.String()
		result, err := callTxOutProofHandler(t, handleGetTxOutProof, s,
			btcjson.NewGetTxOutProofCmd([]string{txHash.String()},
				&blockStr))
		if err != nil {
			t.Fatalf("gettxoutproof: %v", err)
		}
		return result.(string)
	}
	verifyProof := func(proof string) (interface{}, error) {
		t.Helper()
		return callTxOutProofHandler(t, handleVerifyTxOutProof, s,
			btcjson.NewVerifyTxOutProofCmd(proof))
	}
	assertMatches := func(what string, result interface{}, err error, want ...string) {
		t.Helper()
		txids, ok := result.([]string)
		if err != nil || !ok || len(txids) != len(want) {
			t.Errorf("%s: got %v, %v, want %v", what, result, err, want)
			return
		}
		for i := range want {
			if txids[i] != want[i] {
				t.Errorf("%s: got %v, want %v", what, txids, want)
			}
		}
	}

	// A block with two transactions, spending the anyone can spend
	// coinbase of the first block once it has matured.
	first := mineTestBlock(t, chain, nil)
	for i := 0; i < 100; i++ {
		mineTestBlock(t, chain, nil)
	}
	spend := wire.NewMsgTx(1)
	spend.AddTxIn(wire.NewTxIn(wire.NewOutPoint(first.Transactions()[0].Hash(), 0),
		nil))
	spend.AddTxOut(wire.NewTxOut(first.MsgBlock().Transactions[0].TxOut[0].Value-1000,
		[]byte{txscript.OP_TRUE}, wire.TokenData{}))
	padding, err := txscript.NullDataScript(make([]byte, 40))
	if err != nil {
		t.Fatal(err)
	}
	spend.AddTxOut(wire.NewTxOut(0, padding, wire.TokenData{}))
	block := mineTestBlock(t, chain, nil, spend)
	spendHash := spend.TxHash()
	proof := getProof(block.Hash(), &spendHash)
	result, err := verifyProof(proof)
	assertMatches("two transaction block", result, err, spendHash.String())

	// The genesis block, whose only transaction is the coinbase.
	genesis := chaincfg.RegressionNetParams.GenesisBlock
	coinbaseHash := genesis.Transactions[0].TxHash()
	genesisProof := getProof(chaincfg.RegressionNetParams.GenesisHash,
		&coinbaseHash)
	result, err = verifyProof(genesisProof)
	assertMatches("genesis block", result, err, coinbaseHash.String())

	// A header that isn't in the chain.
	msg := decodeTestProof(t, genesisProof)
	msg.Header.Nonce++
	_, err = verifyProof(encodeTestProof(t, msg))
	assertRPCError(t, "changed header", err,
		btcjson.ErrRPCInvalidAddressOrKey, "Block not found in chain")

	// A header of a side chain block.
	side := mineTestBlock(t, chain, chaincfg.RegressionNetParams.GenesisHash)
	if chain.MainChainHasBlock(side.Hash()) {
		t.Fatal("side chain block is in the main chain")
	}
	sideProof := getProof(side.Hash(), side.Transactions()[0].Hash())
	_, err = verifyProof(sideProof)
	assertRPCError(t, "side chain header", err,
		btcjson.ErrRPCInvalidAddressOrKey, "Block not found in chain")

	// A merkle root that doesn't match the header proves nothing, so, as
	// in BCHN, no transactions are returned, before the block is looked
	// for.
	msg = decodeTestProof(t, genesisProof)
	msg.Hashes[0] = &chainhash.Hash{1}
	result, err = verifyProof(encodeTestProof(t, msg))
	assertMatches("changed hash", result, err)
	msg.Header.Nonce++
	result, err = verifyProof(encodeTestProof(t, msg))
	assertMatches("changed hash and header", result, err)

	// Nor does a tree that can't be traversed.
	msg = decodeTestProof(t, genesisProof)
	msg.Flags = nil
	result, err = verifyProof(encodeTestProof(t, msg))
	assertMatches("bad tree", result, err)

	// A proof that matches no transaction returns none.
	msg = decodeTestProof(t, genesisProof)
	msg.Flags = []byte{0}
	result, err = verifyProof(encodeTestProof(t, msg))
	assertMatches("no match", result, err)

	// A proof of the two transaction block claiming a single transaction,
	// the merkle root, has the right root but the wrong count.
	msg = decodeTestProof(t, proof)
	msg.Transactions = 1
	msg.Hashes = []*chainhash.Hash{&msg.Header.MerkleRoot}
	msg.Flags = []byte{1}
	result, err = verifyProof(encodeTestProof(t, msg))
	assertMatches("wrong transaction count", result, err)
}
