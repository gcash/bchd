// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
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
// an unknown inventory type doesn't leave the handler waiting forever: the
// entry before it is answered, the unknown one is ignored, and the handler
// returns once its answer has been sent.
func TestOnGetDataUnknownInvType(t *testing.T) {
	h := newGetDataHarness(t)

	// A filtered block request without a loaded filter is answered with a
	// merkle block matching every transaction, and the transaction, so the
	// unknown entry after it is the last one handled.
	msg := wire.NewMsgGetData()
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeFilteredBlock,
		&h.blocks[1]))
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvType(0x7f),
		&chainhash.Hash{2}))

	done := h.serve(msg)
	if returnedWithin(done, 200*time.Millisecond) {
		t.Fatal("getdata handler returned before its merkle block was sent")
	}
	merkle, ok := h.readMessage(t).(*wire.MsgMerkleBlock)
	if !ok || merkle.Header.BlockHash() != h.blocks[1] {
		t.Fatalf("got %v, want the merkle block of %v", merkle, h.blocks[1])
	}
	if _, ok := h.readMessage(t).(*wire.MsgTx); !ok {
		t.Fatal("the merkle block's transaction was not sent")
	}

	// Nothing is sent for the unknown entry: the next message is one
	// queued after the merkle block's transaction was read.
	h.sp.QueueMessage(wire.NewMsgPong(7), nil)
	if next := h.readMessage(t); next.Command() != wire.CmdPong {
		t.Fatalf("got %s after the merkle block's transaction, want the "+
			"pong", next.Command())
	}
	if !returnedWithin(done, 10*time.Second) {
		t.Fatal("getdata handler did not return once its merkle block " +
			"was sent")
	}
}
