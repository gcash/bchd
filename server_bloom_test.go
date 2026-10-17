// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/bloom"
)

// newBloomTestPeer returns an inbound server peer attached to a server that
// offers bloom filtering and has no filter loaded.
func newBloomTestPeer() *serverPeer {
	return &serverPeer{
		Peer:   peer.NewInboundPeer(&peer.Config{}),
		server: &server{services: wire.SFNodeBloom},
		filter: bloom.LoadFilter(nil),
	}
}

// isDisconnected returns whether the peer was disconnected, waiting briefly
// for the disconnect to be signalled.
func isDisconnected(sp *serverPeer) bool {
	done := make(chan struct{})
	go func() {
		sp.WaitForDisconnect()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

// TestFilterHandlersDegenerateFilters ensures filterload accepts filters with
// an empty data array or no hash functions, as the reference implementation
// does, and that later filteradd messages and transaction matching on those
// filters neither panic nor disconnect the peer.
func TestFilterHandlersDegenerateFilters(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		hashFuncs uint32
	}{
		{name: "empty data", data: []byte{}, hashFuncs: 5},
		{name: "empty data max hash funcs", data: nil,
			hashFuncs: wire.MaxFilterLoadHashFuncs},
		{name: "empty data no hash funcs", data: nil, hashFuncs: 0},
		{name: "no hash funcs", data: make([]byte, 8), hashFuncs: 0},
	}

	prevHash := chainhash.Hash{0x01}
	msgTx := wire.NewMsgTx(1)
	msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, 0), nil))
	msgTx.AddTxOut(wire.NewTxOut(0, []byte{0x6a, 0x01, 0xaa},
		wire.TokenData{}))
	tx := bchutil.NewTx(msgTx)

	for _, test := range tests {
		sp := newBloomTestPeer()
		sp.setDisableRelayTx(true)

		panicked := true
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: unexpected panic: %v", test.name, r)
				}
			}()

			sp.OnFilterLoad(nil, wire.NewMsgFilterLoad(test.data,
				test.hashFuncs, 0, wire.BloomUpdateAll))
			sp.OnFilterAdd(nil, wire.NewMsgFilterAdd([]byte{1, 2, 3}))

			// Transaction relay and mempool replies match
			// transactions against the loaded filter this way.
			if !sp.filter.MatchTxAndUpdate(tx) {
				t.Errorf("%s: filter did not match everything",
					test.name)
			}
			panicked = false
		}()

		// A panic may leave the filter locked, so skip the remaining
		// checks rather than block on it.
		if panicked {
			continue
		}

		if !sp.filter.IsLoaded() {
			t.Errorf("%s: filter was not loaded", test.name)
		}
		if sp.relayTxDisabled() {
			t.Errorf("%s: filterload did not enable tx relay", test.name)
		}
		if isDisconnected(sp) {
			t.Errorf("%s: peer was disconnected", test.name)
		}
	}
}

// TestFilterAddWithoutFilter ensures filteradd without a loaded filter still
// disconnects the peer.
func TestFilterAddWithoutFilter(t *testing.T) {
	sp := newBloomTestPeer()
	sp.OnFilterAdd(nil, wire.NewMsgFilterAdd([]byte{1, 2, 3}))
	if !isDisconnected(sp) {
		t.Error("peer was not disconnected")
	}
}
