// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// syncHeightMgr is an rpcserverSyncManager with a fixed sync height.
type syncHeightMgr struct {
	rpcserverSyncManager
	height uint64
}

func (m *syncHeightMgr) SyncHeight() uint64 { return m.height }

// blockFieldsTest is a regtest chain of 102 blocks after the genesis block,
// ten minutes apart from start, whose last block also spends the coinbase of
// the first, and an RPC server of it.
type blockFieldsTest struct {
	s       *rpcServer
	syncMgr *syncHeightMgr
	start   time.Time
	blocks  []*bchutil.Block // by height, the genesis block's left out
}

// newBlockFieldsTest returns a blockFieldsTest.
func newBlockFieldsTest(t *testing.T) *blockFieldsTest {
	t.Helper()
	quietRPCLog(t)
	chain, db := newTestChainDB(t)
	test := &blockFieldsTest{
		syncMgr: &syncHeightMgr{},
		start:   time.Unix(time.Now().Add(-24*time.Hour).Unix(), 0),
		blocks:  []*bchutil.Block{nil},
	}
	test.s = &rpcServer{cfg: rpcserverConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Chain:       chain,
		DB:          db,
		SyncMgr:     test.syncMgr,
	}}

	for range chaincfg.RegressionNetParams.CoinbaseMaturity + 1 {
		test.mine(t)
	}
	coinbase := test.blocks[1].MsgBlock().Transactions[0]
	value := coinbase.TxOut[0].Value
	spend := wire.NewMsgTx(1)
	spend.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: coinbase.TxHash()}, nil))
	spend.AddTxOut(wire.NewTxOut(value/2, []byte{txscript.OP_TRUE},
		wire.TokenData{}))
	spend.AddTxOut(wire.NewTxOut(value/2-1000, []byte{txscript.OP_TRUE},
		wire.TokenData{}))
	test.mine(t, spend)
	return test
}

// at returns the time of the block at the passed height.
func (test *blockFieldsTest) at(height int) time.Time {
	return test.start.Add(time.Duration(height) * 10 * time.Minute)
}

// next returns a solved block on the tip, at its time, with the passed
// transactions after the coinbase.
func (test *blockFieldsTest) next(t *testing.T, txns ...*wire.MsgTx) *bchutil.Block {
	t.Helper()
	height := len(test.blocks)
	return solvedBlockWith(t, test.s.cfg.Chain, func(msg *wire.MsgBlock) {
		msg.Header.Timestamp = test.at(height)
		msg.Transactions = append(msg.Transactions, txns...)
	})
}

// mine connects the next block, with the passed transactions after the
// coinbase.
func (test *blockFieldsTest) mine(t *testing.T, txns ...*wire.MsgTx) {
	t.Helper()
	block := test.next(t, txns...)
	_, _, err := test.s.cfg.Chain.ProcessBlock(block, blockchain.BFNone)
	if err != nil {
		t.Fatalf("block %d: %v", len(test.blocks), err)
	}
	test.blocks = append(test.blocks, block)
}

// blockFields are the fields of a getblock or getblockheader result that
// come from the block index.
type blockFields struct {
	Height     int64   `json:"height"`
	MedianTime int64   `json:"mediantime"`
	ChainWork  string  `json:"chainwork"`
	NTx        *uint64 `json:"nTx"`
}

// decodeBlockFields returns the block index fields of a result.
func decodeBlockFields(t *testing.T, result interface{}) blockFields {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields blockFields
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields.NTx == nil {
		t.Fatalf("no nTx in %s", raw)
	}
	return fields
}

// workHex returns 64 hexadecimal digits ending in the passed ones.
func workHex(digits string) string {
	return strings.Repeat("0", 64-len(digits)) + digits
}

// wantBlockFields fails the test unless the fields are the passed ones.
func wantBlockFields(t *testing.T, what string, got blockFields, height int64,
	medianTime time.Time, chainWork string, nTx uint64) {

	t.Helper()
	if got.Height != height || got.MedianTime != medianTime.Unix() ||
		got.ChainWork != chainWork || *got.NTx != nTx {

		t.Errorf("%s: got height %d, mediantime %d, chainwork %s, nTx %d, "+
			"want %d, %d, %s, %d", what, got.Height, got.MedianTime,
			got.ChainWork, *got.NTx, height, medianTime.Unix(), chainWork,
			nTx)
	}
}

// TestGetBlockIndexFields ensures getblock gives BCHN's mediantime, chainwork
// and nTx at each verbosity above 0.  The median time past of a block is the
// time of the sixth of the eleven up to it, and each regtest block, with the
// easiest target, adds 2 to the work, so the chain up to height 102 has 206,
// or 0xce.
func TestGetBlockIndexFields(t *testing.T) {
	test := newBlockFieldsTest(t)
	tests := []struct {
		height     int
		medianTime time.Time
		chainWork  string
		nTx        uint64
	}{
		{101, test.at(96), workHex("cc"), 1},
		{102, test.at(97), workHex("ce"), 2},
	}
	for _, block := range tests {
		hash := test.blocks[block.height].Hash().String()
		for verbosity := btcjson.VerbosityLevel(1); verbosity <= 3; verbosity++ {
			result, err := handleGetBlock(test.s, btcjson.NewGetBlockCmd(hash,
				btcjson.Verbositylevel(verbosity)), nil)
			if err != nil {
				t.Fatal(err)
			}
			wantBlockFields(t, "getblock", decodeBlockFields(t, result),
				int64(block.height), block.medianTime, block.chainWork,
				block.nTx)
		}
	}
}

// TestGetBlockHeaderIndexFields ensures getblockheader gives BCHN's
// mediantime, chainwork and nTx, with 0 transactions for a block without its
// data.
func TestGetBlockHeaderIndexFields(t *testing.T) {
	test := newBlockFieldsTest(t)
	header := func(hash string) blockFields {
		t.Helper()
		result, err := handleGetBlockHeader(test.s,
			btcjson.NewGetBlockHeaderCmd(hash, btcjson.Bool(true)), nil)
		if err != nil {
			t.Fatal(err)
		}
		return decodeBlockFields(t, result)
	}
	wantBlockFields(t, "block 102", header(test.blocks[102].Hash().String()),
		102, test.at(97), workHex("ce"), 2)

	// A header added as in fastsync is the tip's, without its block.
	next := test.next(t).MsgBlock().Header
	if err := test.s.cfg.Chain.AddHeader(&next); err != nil {
		t.Fatal(err)
	}
	wantBlockFields(t, "header without its block", header(next.BlockHash().String()),
		103, test.at(98), workHex("d0"), 0)
}

// getBlockChainInfo returns the getblockchaininfo result and its JSON.
func getBlockChainInfo(t *testing.T, s *rpcServer) (*btcjson.GetBlockChainInfoResult, string) {
	t.Helper()
	result, err := handleGetBlockChainInfo(s, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := result.(*btcjson.GetBlockChainInfoResult)
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	return info, string(raw)
}

// TestGetBlockChainInfoHeaders ensures getblockchaininfo counts the headers to
// the best header, which can be a kept header's, gives the tip's chain work,
// and estimates the progress against the higher of the sync peer's height and
// the best header's.
func TestGetBlockChainInfoHeaders(t *testing.T) {
	test := newBlockFieldsTest(t)
	info, raw := getBlockChainInfo(t, test.s)
	if info.Blocks != 102 || info.Headers != 102 ||
		info.ChainWork != workHex("ce") || info.VerificationProgress != 1 ||
		info.Pruned {

		t.Fatalf("without kept headers or peers: %s", raw)
	}

	// The header of the next block is kept until the block arrives.
	next := test.next(t).MsgBlock().Header
	_, _, err := test.s.cfg.Chain.ProcessBlockHeaders(
		[]*wire.BlockHeader{&next}, 1)
	if err != nil {
		t.Fatal(err)
	}
	info, raw = getBlockChainInfo(t, test.s)
	if info.Blocks != 102 || info.Headers != 103 ||
		info.VerificationProgress != 102.0/103.0 {

		t.Fatalf("with a kept header: %s", raw)
	}

	test.syncMgr.height = 204
	info, raw = getBlockChainInfo(t, test.s)
	if info.Headers != 103 || info.VerificationProgress != 0.5 ||
		info.SyncHeight != 204 {

		t.Fatalf("with a sync peer at height 204: %s", raw)
	}
}

// TestGetBlockChainInfoPruned ensures getblockchaininfo reports --prune, and a
// verification progress of 0, at the genesis block without peers, rather than
// leaving it out.
func TestGetBlockChainInfoPruned(t *testing.T) {
	quietRPCLog(t)
	blockchain.UseLogger(bchlog.Disabled)
	t.Cleanup(func() { blockchain.UseLogger(chanLog) })
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &chaincfg.RegressionNetParams,
		TimeSource:         blockchain.NewMedianTime(),
		UtxoCacheMaxSize:   16 * 1024 * 1024,
		ExcessiveBlockSize: 32000000,
		Prune:              true,
		PruneDepth:         288,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Chain:       chain,
		DB:          db,
		SyncMgr:     &syncHeightMgr{},
	}}

	info, raw := getBlockChainInfo(t, s)
	if !info.Pruned || info.Blocks != 0 || info.Headers != 0 ||
		info.ChainWork != workHex("2") ||
		!strings.Contains(raw, `"verificationprogress":0,`) {

		t.Fatalf("pruned chain at the genesis block: %s", raw)
	}
}
