// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"math"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
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
	for _, skip := range []int{2, math.MaxUint32, 1 << 32, 1<<32 + 1, math.MaxInt} {
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
