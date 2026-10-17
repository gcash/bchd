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
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil/bloom"
)

// useTestConfig sets the global config to c and turns off the server and
// peer logs, which have no backend in tests, for the duration of a test.
func useTestConfig(t *testing.T, c *config) {
	t.Helper()
	oldCfg := cfg
	oldSrvrLevel, oldPeerLevel := srvrLog.Level(), peerLog.Level()
	cfg = c
	srvrLog.SetLevel(bchlog.LevelOff)
	peerLog.SetLevel(bchlog.LevelOff)
	t.Cleanup(func() {
		cfg = oldCfg
		srvrLog.SetLevel(oldSrvrLevel)
		peerLog.SetLevel(oldPeerLevel)
	})
}

// TestOnGetDataUnknownInvType ensures a getdata message whose last entry has
// an unknown inventory type doesn't leave the handler waiting forever.
func TestOnGetDataUnknownInvType(t *testing.T) {
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	sp := &serverPeer{
		Peer:   peer.NewInboundPeer(&peer.Config{}),
		server: &server{},
		filter: bloom.LoadFilter(nil),
	}

	// A filtered block request without a loaded filter is answered with
	// nothing and counts as handled, so the unknown entry after it is
	// the last one that could signal completion.
	msg := wire.NewMsgGetData()
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeFilteredBlock,
		&chainhash.Hash{1}))
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvType(0x7f),
		&chainhash.Hash{2}))

	done := make(chan struct{})
	go func() {
		sp.OnGetData(nil, msg)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("getdata handler did not return")
	}
}
