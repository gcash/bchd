// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
)

// indexInfoSlpStart is the height of the block the SLP index of
// newIndexInfoTestServer starts after.
const indexInfoSlpStart = 1000

// newIndexInfoTestServer returns an RPC server of a regtest chain that keeps a
// transaction, a committed filter, an SLP and a BCMR index, the SLP index
// starting after a block at height indexInfoSlpStart.
func newIndexInfoTestServer(t *testing.T) *rpcServer {
	t.Helper()
	db := newRPCTestDB(t)
	params := &chaincfg.RegressionNetParams
	txIndex := indexers.NewTxIndex(db)
	cfIndex := indexers.NewCfIndex(db, params)
	slpIndex := indexers.NewSlpIndex(db, &indexers.SlpConfig{
		StartHash:    &chainhash.Hash{1},
		StartHeight:  indexInfoSlpStart,
		AddrPrefix:   params.SlpAddressPrefix,
		MaxCacheSize: 100,
	})
	bcmrIndex := indexers.NewBcmrIndex(db, params)
	blockchain.UseLogger(bchlog.Disabled)
	t.Cleanup(func() { blockchain.UseLogger(chanLog) })
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        params,
		TimeSource:         blockchain.NewMedianTime(),
		UtxoCacheMaxSize:   1 << 20,
		ExcessiveBlockSize: 32000000,
		IndexManager: indexers.NewManager(db, []indexers.Indexer{
			txIndex, cfIndex, slpIndex, bcmrIndex,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &rpcServer{cfg: rpcserverConfig{
		ChainParams: params,
		Chain:       chain,
		DB:          db,
		TxIndex:     txIndex,
		CfIndex:     cfIndex,
		SlpIndex:    slpIndex,
		BcmrIndex:   bcmrIndex,
	}}
}

// getIndexInfo returns the result of getindexinfo with the passed name.
func getIndexInfo(t *testing.T, s *rpcServer, name *string) map[string]btcjson.GetIndexInfoResult {
	t.Helper()
	result, err := handleGetIndexInfo(s, btcjson.NewGetIndexInfoCmd(name), nil)
	if err != nil {
		t.Fatal(err)
	}
	return result.(map[string]btcjson.GetIndexInfoResult)
}

// TestGetIndexInfo ensures getindexinfo gives each index the node runs under
// the name of its option, synced once it has indexed the best block, which an
// index that starts after a later block hasn't, nor the BCMR index while its
// walker has a publication to walk behind; and that a name gives only that
// index, or none if the node doesn't run it, while an empty name gives all.
func TestGetIndexInfo(t *testing.T) {
	quietRPCLog(t)
	s := newIndexInfoTestServer(t)
	chain := s.cfg.Chain
	first := mineTestBlock(t, chain, nil)
	for range s.cfg.ChainParams.CoinbaseMaturity {
		mineTestBlock(t, chain, nil)
	}

	// The publication spends the identity output of the first coinbase,
	// which the BCMR index doesn't track, so it leaves the walker to walk
	// behind it.
	coinbase := first.Transactions()[0]
	script := append(bytes.Repeat([]byte{txscript.OP_NOP}, 10), txscript.OP_TRUE)
	publication := append([]byte{txscript.OP_RETURN, txscript.OP_DATA_4,
		'B', 'C', 'M', 'R', txscript.OP_DATA_32}, bytes.Repeat([]byte{1}, 32)...)
	pub := wire.NewMsgTx(1)
	pub.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: *coinbase.Hash()}, nil))
	pub.AddTxOut(wire.NewTxOut(coinbase.MsgTx().TxOut[0].Value-10000, script,
		wire.TokenData{}))
	pub.AddTxOut(wire.NewTxOut(0, publication, wire.TokenData{}))
	mineTestBlock(t, chain, nil, pub)
	height := chain.BestSnapshot().Height

	all := map[string]btcjson.GetIndexInfoResult{
		"txindex":   {Synced: true, BestBlockHeight: height},
		"cfindex":   {Synced: true, BestBlockHeight: height},
		"slpindex":  {Synced: false, BestBlockHeight: indexInfoSlpStart},
		"bcmrindex": {Synced: false, BestBlockHeight: height},
	}
	tests := []struct {
		name *string
		want map[string]btcjson.GetIndexInfoResult
	}{
		{nil, all},
		{btcjson.String(""), all},
		{btcjson.String("txindex"), map[string]btcjson.GetIndexInfoResult{
			"txindex": all["txindex"],
		}},
		{btcjson.String("slpindex"), map[string]btcjson.GetIndexInfoResult{
			"slpindex": all["slpindex"],
		}},
		{btcjson.String("addrindex"), map[string]btcjson.GetIndexInfoResult{}},
		{btcjson.String("coinstatsindex"), map[string]btcjson.GetIndexInfoResult{}},
	}
	for _, test := range tests {
		if got := getIndexInfo(t, s, test.name); !reflect.DeepEqual(got, test.want) {
			name := "<nil>"
			if test.name != nil {
				name = *test.name
			}
			t.Errorf("name %q: got %+v, want %+v", name, got, test.want)
		}
	}

	// The BCMR index is synced once the walker has walked behind the
	// publication.
	bcmrIndex := btcjson.String("bcmrindex")
	s.cfg.BcmrIndex.StartWalker()
	defer s.cfg.BcmrIndex.StopWalker()
	want := btcjson.GetIndexInfoResult{Synced: true, BestBlockHeight: height}
	for deadline := time.Now().Add(30 * time.Second); ; {
		got := getIndexInfo(t, s, bcmrIndex)["bcmrindex"]
		if got == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("bcmrindex: got %+v, want %+v", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// A node that runs no index has none to give.
	none := &rpcServer{cfg: rpcserverConfig{Chain: chain, DB: s.cfg.DB}}
	for _, name := range []*string{nil, btcjson.String("txindex")} {
		if got := getIndexInfo(t, none, name); len(got) != 0 {
			t.Errorf("no indexes: got %+v, want none", got)
		}
	}
}
