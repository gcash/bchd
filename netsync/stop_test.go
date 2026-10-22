// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"
	"time"
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
