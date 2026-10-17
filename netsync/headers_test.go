// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// regtestBits is the compact target used for the headers in these tests.
const regtestBits = 0x207fffff

// headersTestHarness holds a sync manager in headers-first mode along with a
// sync peer and a second peer that is not the sync peer.
type headersTestHarness struct {
	sm       *SyncManager
	params   *chaincfg.Params
	syncPeer *peerpkg.Peer
	other    *peerpkg.Peer
}

// newHeadersTestHarness returns a sync manager on a fresh regression test
// chain that is in headers-first mode with the passed checkpoint as the next
// checkpoint.  The returned harness has a sync peer and a second peer
// registered with the sync manager.
func newHeadersTestHarness(t *testing.T,
	cp chaincfg.Checkpoint) *headersTestHarness {

	t.Helper()

	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		Checkpoints:        []chaincfg.Checkpoint{cp},
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatalf("failed to create blockchain: %v", err)
	}

	sm, err := New(&Config{
		Chain:       chain,
		ChainParams: &params,
		MaxPeers:    8,
	})
	if err != nil {
		t.Fatalf("failed to create SyncManager: %v", err)
	}
	if sm.nextCheckpoint == nil {
		t.Fatal("sync manager has no next checkpoint")
	}

	peerCfg := &peerpkg.Config{ChainParams: &params}
	h := &headersTestHarness{
		sm:       sm,
		params:   &params,
		syncPeer: peerpkg.NewInboundPeer(peerCfg),
		other:    peerpkg.NewInboundPeer(peerCfg),
	}
	for _, p := range []*peerpkg.Peer{h.syncPeer, h.other} {
		sm.peerStates[p] = &peerSyncState{
			syncCandidate:   true,
			requestedTxns:   make(map[chainhash.Hash]struct{}),
			requestedBlocks: make(map[chainhash.Hash]struct{}),
		}
	}
	sm.syncPeer = h.syncPeer
	sm.headersFirstMode = true

	// Disconnect both peers when the test finishes so no goroutine is
	// left waiting for them to disconnect.
	t.Cleanup(func() {
		h.syncPeer.Disconnect()
		h.other.Disconnect()
	})

	return h
}

// mineHeaders returns count headers that build on prev and each have valid
// proof of work for the regression test network.
func mineHeaders(t *testing.T, params *chaincfg.Params, prev chainhash.Hash,
	count int) []*wire.BlockHeader {

	t.Helper()

	headers := make([]*wire.BlockHeader, 0, count)
	for i := 0; i < count; i++ {
		header := wire.NewBlockHeader(1, &prev, &chainhash.Hash{},
			regtestBits, 0)
		for blockchain.CheckHeaderProofOfWork(header,
			params.PowLimit) != nil {

			header.Nonce++
		}
		headers = append(headers, header)
		prev = header.BlockHash()
	}
	return headers
}

// headersMessage returns a headers message that contains the passed headers.
func headersMessage(t *testing.T,
	headers []*wire.BlockHeader) *wire.MsgHeaders {

	t.Helper()

	msg := wire.NewMsgHeaders()
	for _, header := range headers {
		if err := msg.AddBlockHeader(header); err != nil {
			t.Fatalf("failed to add header: %v", err)
		}
	}
	return msg
}

// isDisconnected returns whether the passed peer has been told to disconnect.
func isDisconnected(p *peerpkg.Peer) bool {
	done := make(chan struct{})
	go func() {
		p.WaitForDisconnect()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

// farCheckpoint is a checkpoint far enough ahead that the tests stay in
// headers-first mode without reaching it.
var farCheckpoint = chaincfg.Checkpoint{
	Height: 100000,
	Hash:   &chainhash.Hash{0x01},
}

// TestHeadersFromNonSyncPeerIgnored ensures headers received from a peer
// other than the sync peer during headers-first mode are ignored and do not
// affect the headers later received from the sync peer.
func TestHeadersFromNonSyncPeerIgnored(t *testing.T) {
	h := newHeadersTestHarness(t, farCheckpoint)
	sm := h.sm

	headers := mineHeaders(t, h.params, *h.params.GenesisHash, 20)
	msg := headersMessage(t, headers)

	sm.handleHeadersMsg(&headersMsg{headers: msg, peer: h.other})
	if got := sm.headerList.Len(); got != 1 {
		t.Fatalf("header list length after headers from non-sync "+
			"peer: got %d, want 1", got)
	}
	if isDisconnected(h.other) {
		t.Fatal("non-sync peer was disconnected")
	}

	// The same headers from the sync peer must still connect.
	sm.handleHeadersMsg(&headersMsg{headers: msg, peer: h.syncPeer})
	if isDisconnected(h.syncPeer) {
		t.Fatal("sync peer was disconnected")
	}
	if got, want := sm.headerList.Len(), len(headers)+1; got != want {
		t.Fatalf("header list length after headers from sync peer: "+
			"got %d, want %d", got, want)
	}
}

// TestHeadersInvalidProofOfWorkRejected ensures headers from the sync peer
// that do not have valid proof of work are not added to the header list and
// cause the sync peer to be disconnected.
func TestHeadersInvalidProofOfWorkRejected(t *testing.T) {
	tests := []struct {
		name   string
		header func(prev *chainhash.Hash) *wire.BlockHeader
	}{{
		name: "hash above target",
		header: func(prev *chainhash.Hash) *wire.BlockHeader {
			header := wire.NewBlockHeader(1, prev,
				&chainhash.Hash{}, regtestBits, 0)
			params := &chaincfg.RegressionNetParams
			for blockchain.CheckHeaderProofOfWork(header,
				params.PowLimit) == nil {

				header.Nonce++
			}
			return header
		},
	}, {
		name: "target above pow limit",
		header: func(prev *chainhash.Hash) *wire.BlockHeader {
			return wire.NewBlockHeader(1, prev, &chainhash.Hash{},
				0x2100ffff, 0)
		},
	}, {
		name: "zero target",
		header: func(prev *chainhash.Hash) *wire.BlockHeader {
			return wire.NewBlockHeader(1, prev, &chainhash.Hash{},
				0, 0)
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newHeadersTestHarness(t, farCheckpoint)
			sm := h.sm

			// Send some valid headers followed by the invalid
			// one.
			headers := mineHeaders(t, h.params,
				*h.params.GenesisHash, 5)
			prev := headers[len(headers)-1].BlockHash()
			headers = append(headers, test.header(&prev))

			sm.handleHeadersMsg(&headersMsg{
				headers: headersMessage(t, headers),
				peer:    h.syncPeer,
			})
			if !isDisconnected(h.syncPeer) {
				t.Fatal("sync peer was not disconnected")
			}
			got, want := sm.headerList.Len(), len(headers)
			if got != want {
				t.Fatalf("header list length: got %d, want %d",
					got, want)
			}
			last := sm.headerList.Back().Value.(*headerNode)
			if !last.hash.IsEqual(&prev) {
				t.Fatalf("last header in list: got %v, want %v",
					last.hash, prev)
			}
		})
	}
}

// TestHeadersFromSyncPeerAccepted ensures valid headers from the sync peer
// are added to the header list, and that reaching the next checkpoint still
// switches to fetching the blocks.
func TestHeadersFromSyncPeerAccepted(t *testing.T) {
	params := &chaincfg.RegressionNetParams
	headers := mineHeaders(t, params, *params.GenesisHash, 10)
	msg := headersMessage(t, headers)

	t.Run("before checkpoint", func(t *testing.T) {
		h := newHeadersTestHarness(t, farCheckpoint)
		sm := h.sm

		sm.handleHeadersMsg(&headersMsg{headers: msg, peer: h.syncPeer})
		if isDisconnected(h.syncPeer) {
			t.Fatal("sync peer was disconnected")
		}
		got, want := sm.headerList.Len(), len(headers)+1
		if got != want {
			t.Fatalf("header list length: got %d, want %d", got,
				want)
		}
		last := sm.headerList.Back().Value.(*headerNode)
		if last.height != int32(len(headers)) {
			t.Fatalf("last header height: got %d, want %d",
				last.height, len(headers))
		}
	})

	t.Run("at checkpoint", func(t *testing.T) {
		cpHash := headers[len(headers)-1].BlockHash()
		cp := chaincfg.Checkpoint{
			Height: int32(len(headers)),
			Hash:   &cpHash,
		}
		h := newHeadersTestHarness(t, cp)
		sm := h.sm

		sm.handleHeadersMsg(&headersMsg{headers: msg, peer: h.syncPeer})
		if isDisconnected(h.syncPeer) {
			t.Fatal("sync peer was disconnected")
		}

		// The genesis entry is removed once the checkpoint is
		// reached and all of the headers are requested as blocks.
		if got, want := sm.headerList.Len(), len(headers); got != want {
			t.Fatalf("header list length: got %d, want %d", got,
				want)
		}
		state := sm.peerStates[h.syncPeer]
		got, want := len(state.requestedBlocks), len(headers)
		if got != want {
			t.Fatalf("requested blocks: got %d, want %d", got,
				want)
		}
	})
}
