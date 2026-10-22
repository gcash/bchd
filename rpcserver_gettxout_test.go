// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestGetTxOutMempool ensures gettxout with the memory pool included answers
// as Bitcoin Cash Node does: null for an index past a memory pool
// transaction's outputs, and for an output a memory pool transaction spends,
// whether that output is confirmed or not.  Without the memory pool, a
// confirmed output a memory pool transaction spends is still found.
func TestGetTxOutMempool(t *testing.T) {
	chain, db := newRPCTestChain(t)
	params := &chaincfg.RegressionNetParams
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

	// The parent spends the first coinbase, which pays to OP_TRUE, into two
	// outputs, and the child spends the first of them.  The scripts are long
	// enough for the transactions to pass the minimum size.
	script := append(bytes.Repeat([]byte{txscript.OP_NOP}, 10), txscript.OP_TRUE)
	coinbase := first.Transactions()[0]
	coin := wire.OutPoint{Hash: *coinbase.Hash()}
	value := coinbase.MsgTx().TxOut[0].Value
	parent := wire.NewMsgTx(1)
	parent.AddTxIn(wire.NewTxIn(&coin, nil))
	parent.AddTxOut(wire.NewTxOut(value/2, script, wire.TokenData{}))
	parent.AddTxOut(wire.NewTxOut(value/2-10000, script, wire.TokenData{}))
	child := wire.NewMsgTx(1)
	child.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: parent.TxHash()}, nil))
	child.AddTxOut(wire.NewTxOut(value/2-10000, script, wire.TokenData{}))
	for _, tx := range []*wire.MsgTx{parent, child} {
		if _, err := pool.ProcessTransaction(bchutil.NewTx(tx), false, 0); err != nil {
			t.Fatalf("transaction not accepted: %v", err)
		}
	}

	noMempool := false
	tests := []struct {
		name           string
		out            wire.OutPoint
		includeMempool *bool
		confirmations  int64 // -1 for null
	}{
		{"index past the outputs", wire.OutPoint{Hash: parent.TxHash(), Index: 2},
			nil, -1},
		{"output a pool transaction spends",
			wire.OutPoint{Hash: parent.TxHash()}, nil, -1},
		{"unspent pool output",
			wire.OutPoint{Hash: parent.TxHash(), Index: 1}, nil, 0},
		{"confirmed output a pool transaction spends", coin, nil, -1},
		{"confirmed output without the pool", coin, &noMempool,
			int64(params.CoinbaseMaturity) + 1},
	}
	for _, test := range tests {
		result, err := handleGetTxOut(s, &btcjson.GetTxOutCmd{
			Txid:           test.out.Hash.String(),
			Vout:           test.out.Index,
			IncludeMempool: test.includeMempool,
		}, nil)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		reply, _ := result.(*btcjson.GetTxOutResult)
		switch {
		case test.confirmations < 0 && reply != nil:
			t.Errorf("%s: got an output, want null", test.name)
		case test.confirmations >= 0 && reply == nil:
			t.Errorf("%s: got null, want an output", test.name)
		case reply != nil && reply.Confirmations != test.confirmations:
			t.Errorf("%s: %d confirmations, want %d", test.name,
				reply.Confirmations, test.confirmations)
		}
	}
}
