// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// relayRecorder is a PeerNotifier that records the blocks it relays.
type relayRecorder struct {
	nopNotifier
	relayed map[chainhash.Hash]bool
}

func (r relayRecorder) RelayInventory(iv *wire.InvVect, _ interface{}) {
	if iv.Type == wire.InvTypeBlock {
		r.relayed[iv.Hash] = true
	}
}

// TestParkedChainPeer ensures the blocks of a parked chain are not relayed, as
// in Bitcoin Cash Node, and that a sync peer that may only be on that chain
// neither keeps the node from being current nor is preferred to a peer ahead
// of it.  The node doesn't follow that chain, so it would otherwise stop
// following its own chain and refuse getblocktemplate during a split.
func TestParkedChainPeer(t *testing.T) {
	h := newRequestTestHarness(t)
	relayed := make(map[chainhash.Hash]bool)
	h.sm.peerNotifier = relayRecorder{relayed: relayed}

	// The main chain up to block 5, and a chain of blocks 3 to 6 forking
	// at block 2, which is parked as it has one block too few.
	main := h.chainOn(h.tip, 2, 4, 0)
	side := h.chainOn(main[0].MsgBlock(), 3, 4, 1)
	for _, block := range append(main, side...) {
		_, _, err := h.sm.chain.ProcessBlock(block, blockchain.BFNone)
		if err != nil {
			t.Fatal(err)
		}
	}
	if h.sm.chain.BestSnapshot().Hash != *main[3].Hash() {
		t.Fatal("the parked chain was connected")
	}
	for _, block := range main {
		if !relayed[*block.Hash()] {
			t.Fatalf("main chain block %v not relayed", block.Hash())
		}
	}
	for _, block := range side {
		if relayed[*block.Hash()] {
			t.Fatalf("parked block %v relayed", block.Hash())
		}
	}
	if height := h.sm.chain.BestParkedHeight(); height != 6 {
		t.Fatalf("best parked height %d, want 6", height)
	}

	parked, ahead := h.peer(1), h.peer(2)
	parked.UpdateLastBlockHeight(6)
	ahead.UpdateLastBlockHeight(7)
	h.sm.peerStates[parked].syncCandidate = true
	h.sm.peerStates[ahead].syncCandidate = true
	h.sm.syncPeer.Store(parked)
	if !h.sm.current() {
		t.Fatal("not current with a sync peer as high as the parked chain")
	}
	h.sm.syncPeer.Store(ahead)
	if h.sm.current() {
		t.Fatal("current with a sync peer above the parked chain")
	}

	for range 20 {
		h.sm.syncPeer.Store(nil)
		h.sm.startSync()
		if h.sm.syncPeer.Load() != ahead {
			t.Fatal("sync did not start from the peer above the " +
				"parked chain")
		}
	}
}
