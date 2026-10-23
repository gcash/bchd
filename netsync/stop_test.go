// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// returnsWithin fails the test unless fn returns within a few seconds.
func returnsWithin(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not return", what)
	}
}

// TestCallsAfterStop ensures IsCurrent and NewPeer return once the sync
// manager has stopped, rather than wait for the block handler that is gone.
// The server makes these calls from goroutines that may outlive the sync
// manager.
func TestCallsAfterStop(t *testing.T) {
	h := newRequestTestHarness(t)
	p := h.peer(1)
	h.sm.Start()
	if err := h.sm.Stop(); err != nil {
		t.Fatal(err)
	}

	returnsWithin(t, "IsCurrent", func() {
		if h.sm.IsCurrent() {
			t.Error("stopped sync manager is current")
		}
	})
	returnsWithin(t, "NewPeer", func() { h.sm.NewPeer(p, nil) })
	returnsWithin(t, "DonePeer", func() { h.sm.DonePeer(p, nil) })
}

// TestCallsGiveUpOnStop ensures IsCurrent and NewPeer stop waiting for the
// block handler to take their message when the sync manager stops, and that
// the block handler can still answer an IsCurrent that gave up.
func TestCallsGiveUpOnStop(t *testing.T) {
	h := newRequestTestHarness(t)
	p := h.peer(1)

	// Nothing takes the messages.
	sm := &SyncManager{msgChan: make(chan interface{}),
		quit: make(chan struct{})}
	current := make(chan bool, 1)
	go func() { current <- sm.IsCurrent() }()
	added := make(chan struct{})
	go func() {
		sm.NewPeer(p, nil)
		close(added)
	}()
	time.Sleep(50 * time.Millisecond)
	close(sm.quit)
	returnsWithin(t, "IsCurrent", func() {
		if <-current {
			t.Error("stopping sync manager is current")
		}
	})
	returnsWithin(t, "NewPeer", func() { <-added })

	// The block handler takes the message, but the sync manager stops
	// before it answers.
	sm = &SyncManager{msgChan: make(chan interface{}, 1),
		quit: make(chan struct{})}
	go func() { current <- sm.IsCurrent() }()
	msg := (<-sm.msgChan).(isCurrentMsg)
	close(sm.quit)
	returnsWithin(t, "IsCurrent", func() { <-current })
	returnsWithin(t, "the answer to an IsCurrent that gave up", func() {
		msg.reply <- true
	})
}

// TestQueueGivesUpOnStop ensures the calls that queue a peer's messages stop
// waiting for the block handler to take them when the sync manager stops, as
// it then takes no more, and signal a done channel passed to them.  A full
// queue made them wait for good.
func TestQueueGivesUpOnStop(t *testing.T) {
	p := peerpkg.NewInboundPeer(&peerpkg.Config{})
	block := bchutil.NewBlock(chaincfg.RegressionNetParams.GenesisBlock)
	tx := block.Transactions()[0]

	// Nothing takes the messages.
	sm := &SyncManager{msgChan: make(chan interface{}),
		quit: make(chan struct{})}
	calls := []struct {
		name string
		fn   func(done chan struct{})
	}{
		{"QueueTx", func(done chan struct{}) { sm.QueueTx(tx, p, done) }},
		{"QueueBlock", func(done chan struct{}) {
			sm.QueueBlock(block, p, done)
		}},
		{"QueueBlockError", func(chan struct{}) {
			sm.QueueBlockError(block.Hash(), p)
		}},
		{"QueueNotFound", func(chan struct{}) {
			sm.QueueNotFound(wire.NewMsgNotFound(), p)
		}},
		{"QueueInv", func(chan struct{}) { sm.QueueInv(wire.NewMsgInv(), p) }},
		{"QueueHeaders", func(chan struct{}) {
			sm.QueueHeaders(wire.NewMsgHeaders(), p)
		}},
		{"DonePeer", func(chan struct{}) { sm.DonePeer(p, nil) }},
	}
	returned := make([]chan struct{}, len(calls))
	dones := make([]chan struct{}, len(calls))
	for i, call := range calls {
		returned[i] = make(chan struct{})
		dones[i] = make(chan struct{}, 1)
		go func() {
			call.fn(dones[i])
			close(returned[i])
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(sm.quit)
	for i, call := range calls {
		returnsWithin(t, call.name, func() { <-returned[i] })
	}
	returnsWithin(t, "QueueTx's done", func() { <-dones[0] })
	returnsWithin(t, "QueueBlock's done", func() { <-dones[1] })
}

// TestQueuedAnsweredOnStop ensures the block handler answers the messages
// still queued when it stops, as their senders wait for the answer: a peer's
// input handler waits for each block and transaction it queued.  It took
// either a queued message or its quit, at random, so a block queued when a
// sync was stopped was left unanswered about half the time.
func TestQueuedAnsweredOnStop(t *testing.T) {
	h := newRequestTestHarness(t)
	connected := h.peer(1)
	p := peerpkg.NewInboundPeer(&peerpkg.Config{})
	block := bchutil.NewBlock(h.tip)
	h.sm.Start()

	// The paused block handler takes no message until the sync manager
	// is stopping.
	const queued = 20
	unpause := h.sm.Pause()
	dones := make([]chan struct{}, queued)
	for i := range dones {
		dones[i] = make(chan struct{}, 1)
		switch i % 4 {
		case 0:
			h.sm.QueueBlock(block, p, dones[i])
		case 1:
			h.sm.QueueTx(block.Transactions()[0], p, dones[i])
		case 2:
			h.sm.NewPeer(connected, dones[i])
		case 3:
			h.sm.DonePeer(p, dones[i])
		}
	}
	stopped := make(chan struct{})
	go func() {
		_ = h.sm.Stop()
		close(stopped)
	}()
	<-h.sm.quit
	close(unpause)
	returnsWithin(t, "Stop", func() { <-stopped })

	answered := 0
	for _, done := range dones {
		select {
		case <-done:
			answered++
		default:
		}
	}
	if answered != queued {
		t.Fatalf("%d of %d queued messages answered", answered, queued)
	}
}
