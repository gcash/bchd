// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import "testing"

// TestCurrentWhileSyncPeerChanges ensures that current, which the chain's
// notifications call on an RPC goroutine, can run while the block handler
// changes the sync peer.  The race detector checks it, as does the nil check
// that a second read of the sync peer would get past.
func TestCurrentWhileSyncPeerChanges(t *testing.T) {
	h := newRequestTestHarness(t)
	p := h.peer(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100000 {
			h.sm.syncPeer.Store(p)
			h.sm.syncPeer.Store(nil)
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
			h.sm.current()
		}
	}
}
