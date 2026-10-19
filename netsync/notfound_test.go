// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// TestHandleNotFoundMsg ensures a notfound only forgets the blocks that were
// requested from the peer that sent it.
func TestHandleNotFoundMsg(t *testing.T) {
	h := newHeadersTestHarness(t, farCheckpoint)
	block, cmpct, other := chainhash.Hash{0x01}, chainhash.Hash{0x02},
		chainhash.Hash{0x03}
	for _, hash := range []chainhash.Hash{block, cmpct} {
		h.sm.requestedBlocks[hash] = struct{}{}
		h.sm.peerStates[h.other].requestedBlocks[hash] = struct{}{}
	}
	h.sm.requestedBlocks[other] = struct{}{}
	h.sm.peerStates[h.syncPeer].requestedBlocks[other] = struct{}{}

	notFound := wire.NewMsgNotFound()
	for _, iv := range []*wire.InvVect{
		wire.NewInvVect(wire.InvTypeBlock, &block),
		wire.NewInvVect(wire.InvTypeCmpctBlock, &cmpct),
		wire.NewInvVect(wire.InvTypeBlock, &other),
	} {
		if err := notFound.AddInvVect(iv); err != nil {
			t.Fatal(err)
		}
	}
	h.sm.handleNotFoundMsg(&notFoundMsg{notFound: notFound, peer: h.other})

	for _, hash := range []chainhash.Hash{block, cmpct} {
		if _, ok := h.sm.requestedBlocks[hash]; ok {
			t.Errorf("block %v still requested", hash)
		}
		if _, ok := h.sm.peerStates[h.other].requestedBlocks[hash]; ok {
			t.Errorf("block %v still requested from the peer", hash)
		}
	}
	if _, ok := h.sm.requestedBlocks[other]; !ok {
		t.Error("block requested from another peer was forgotten")
	}
	if _, ok := h.sm.peerStates[h.syncPeer].requestedBlocks[other]; !ok {
		t.Error("block forgotten for the peer it was requested from")
	}

	// A peer the sync manager doesn't know is ignored.
	unknown := peerpkg.NewInboundPeer(&peerpkg.Config{ChainParams: h.params})
	h.sm.handleNotFoundMsg(&notFoundMsg{notFound: notFound, peer: unknown})
}
