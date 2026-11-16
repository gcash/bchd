// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestGetChainTipsRPC ensures getchaintips gives the tip of the main chain and
// those of a reorganized branch, a branch never validated, an invalidated
// block and a kept header, with BCHN's fields and statuses, the highest first.
func TestGetChainTipsRPC(t *testing.T) {
	quietRPCLog(t)
	chain := newTestChain(t)
	s := &rpcServer{cfg: rpcserverConfig{Chain: chain}}
	process := func(block *bchutil.Block) {
		t.Helper()
		if _, _, err := chain.ProcessBlock(block, blockchain.BFNone); err != nil {
			t.Fatal(err)
		}
	}
	genesis := chaincfg.RegressionNetParams.GenesisHash

	// Each block pays a different amount below what it may, so that no
	// two coinbases are the same.  A branch of two blocks takes over from
	// a1, then its second block is invalidated, which makes b1 the tip.
	a1 := solvedBlock(t, chain, 0, nil)
	process(a1)
	b1 := solvedBlock(t, chain, -1, genesis)
	process(b1)
	b2 := solvedBlockWith(t, chain, func(msg *wire.MsgBlock) {
		msg.Transactions[0].TxOut[0].Value -= 2
		msg.Header.PrevBlock = *b1.Hash()
		msg.Header.Timestamp = b1.MsgBlock().Header.Timestamp.Add(time.Second)
	})
	process(b2)
	c1 := solvedBlock(t, chain, -3, genesis)
	process(c1)
	if err := chain.InvalidateBlock(b2.Hash()); err != nil {
		t.Fatal(err)
	}
	d2 := solvedBlock(t, chain, -4, nil)
	header := d2.MsgBlock().Header
	if _, _, err := chain.ProcessBlockHeaders([]*wire.BlockHeader{&header},
		1); err != nil {

		t.Fatal(err)
	}

	res, err := handleGetChainTips(s, btcjson.NewGetChainTipsCmd(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := res.([]btcjson.GetChainTipsResult)
	want := map[string]btcjson.GetChainTipsResult{}
	for _, tip := range []btcjson.GetChainTipsResult{
		{Height: 2, Hash: b2.Hash().String(), BranchLen: 1, Status: "invalid"},
		{Height: 2, Hash: d2.Hash().String(), BranchLen: 1, Status: "headers-only"},
		{Height: 1, Hash: b1.Hash().String(), BranchLen: 0, Status: "active"},
		{Height: 1, Hash: a1.Hash().String(), BranchLen: 1, Status: "valid-fork"},
		{Height: 1, Hash: c1.Hash().String(), BranchLen: 1, Status: "valid-headers"},
	} {
		want[tip.Hash] = tip
	}
	if len(got) != len(want) {
		t.Fatalf("got %d tips %+v, want %d", len(got), got, len(want))
	}
	for i, tip := range got {
		if tip != want[tip.Hash] {
			t.Errorf("got tip %+v, want %+v", tip, want[tip.Hash])
		}
		if i > 0 && got[i-1].Height < tip.Height {
			t.Errorf("tip %+v is listed after %+v", tip, got[i-1])
		}
	}
}
