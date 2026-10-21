// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	peerpkg "github.com/gcash/bchd/peer"
)

// TestBlockErrorForgetsOwnRequest ensures a block error from a peer forgets
// only that peer's request for the block, so a peer cannot make the node
// forget a block it asked another peer for.
func TestBlockErrorForgetsOwnRequest(t *testing.T) {
	asked, other := &peerpkg.Peer{}, &peerpkg.Peer{}
	hash := chainhash.Hash{1}
	sm := &SyncManager{
		requestedBlocks: map[chainhash.Hash]struct{}{hash: {}},
		peerStates: map[*peerpkg.Peer]*peerSyncState{
			asked: {requestedBlocks: map[chainhash.Hash]struct{}{hash: {}}},
			other: {requestedBlocks: make(map[chainhash.Hash]struct{})},
		},
	}

	sm.handleBlockError(&blockErrorMsg{hash: &hash, peer: other})
	if _, ok := sm.requestedBlocks[hash]; !ok {
		t.Fatal("another peer's block error forgot the request")
	}
	sm.handleBlockError(&blockErrorMsg{hash: &hash, peer: asked})
	if _, ok := sm.requestedBlocks[hash]; ok {
		t.Fatal("the request is still known after its peer's error")
	}
	if _, ok := sm.peerStates[asked].requestedBlocks[hash]; ok {
		t.Fatal("the peer's request is still known after its error")
	}
}
