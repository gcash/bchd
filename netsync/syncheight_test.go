// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	peerpkg "github.com/gcash/bchd/peer"
)

// syncHeightTestHarness returns a request test harness whose peers can become
// the sync peer, and n such peers at height 5, above the node's.  The peers
// are made before the block handler starts, as making one sets its state.
func syncHeightTestHarness(t *testing.T, n int) (*requestTestHarness, []*peerpkg.Peer) {
	t.Helper()
	h := newRequestTestHarness(t)

	// The regression test network's own parameters, rather than the
	// harness's copy, make a peer from any host a sync candidate.
	h.sm.chainParams = &chaincfg.RegressionNetParams
	h.sm.regTestSyncAnyHost = true
	peers := make([]*peerpkg.Peer, n)
	for i := range peers {
		peers[i] = h.peer(byte(i + 1))
		peers[i].UpdateLastBlockHeight(5)
	}
	return h, peers
}

// TestSyncHeightWhileSyncPeerChanges ensures SyncHeight can be called while
// the block handler changes the sync peer, as getblockchaininfo calls it from
// the RPC server.  It read the sync peer, which only the block handler may
// use, up to four times without a lock: a data race, which -race reports, and
// a nil dereference when the sync peer left between the reads.
func TestSyncHeightWhileSyncPeerChanges(t *testing.T) {
	const cycles = 4
	h, peers := syncHeightTestHarness(t, cycles+1)
	h.sm.Start()
	t.Cleanup(func() { _ = h.sm.Stop() })

	// Each peer becomes the sync peer and then ends, which leaves the node
	// without one.
	done := make(chan struct{}, 1)
	changed := make(chan struct{})
	go func() {
		defer close(changed)
		for _, p := range peers[:cycles] {
			h.sm.NewPeer(p, done)
			<-done
			h.sm.DonePeer(p, done)
			<-done
		}
	}()
	for running := true; running; {
		select {
		case <-changed:
			running = false
		default:
		}
		if height := h.sm.SyncHeight(); height != 0 && height != 5 {
			t.Fatalf("sync height %d, want 0 or 5", height)
		}
	}

	if height := h.sm.SyncHeight(); height != 0 {
		t.Fatalf("sync height %d without a sync peer, want 0", height)
	}
	h.sm.NewPeer(peers[cycles], done)
	<-done
	if height := h.sm.SyncHeight(); height != 5 {
		t.Fatalf("sync height %d, want the sync peer's 5", height)
	}
}

// TestSyncHeightAnsweredByBlockHandler ensures SyncHeight is answered by the
// block handler, which owns the sync peer, rather than read beside it: it
// waits while the block handler is paused.
func TestSyncHeightAnsweredByBlockHandler(t *testing.T) {
	h, peers := syncHeightTestHarness(t, 1)
	h.sm.Start()
	t.Cleanup(func() { _ = h.sm.Stop() })
	done := make(chan struct{}, 1)
	h.sm.NewPeer(peers[0], done)
	<-done

	unpause := h.sm.Pause()
	height := make(chan uint64, 1)
	go func() { height <- h.sm.SyncHeight() }()
	select {
	case got := <-height:
		close(unpause)
		t.Fatalf("SyncHeight returned %d while the block handler was "+
			"paused", got)
	case <-time.After(50 * time.Millisecond):
	}
	close(unpause)
	returnsWithin(t, "SyncHeight", func() {
		if got := <-height; got != 5 {
			t.Errorf("sync height %d, want 5", got)
		}
	})
}
