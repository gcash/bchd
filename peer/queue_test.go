// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package peer

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// newQuitRacePeer returns a peer that looks connected but whose quit channel is
// already closed and whose output channels are full.
//
// This is the state the queue functions race with: Connected reports true, but
// the queue handler has already drained what was buffered and exited, so there
// is nothing left reading the channels.  A blocking send here wedges the caller,
// and since the server relays inventory from the same goroutine that handles
// shutdown, that prevents the server from ever shutting down (issue #479).
func newQuitRacePeer(t *testing.T) *Peer {
	t.Helper()

	p := &Peer{
		outputQueue:    make(chan outMsg, outputBufferSize),
		outputInvChan:  make(chan *wire.InvVect, outputBufferSize),
		quit:           make(chan struct{}),
		knownInventory: newMruInventoryMap(10),
	}
	p.connected.Store(1)

	// Fill both channels so any further send would block.
	filler := wire.NewInvVect(wire.InvTypeTx, &chainhash.Hash{0xff})
	for range outputBufferSize {
		p.outputInvChan <- filler
		p.outputQueue <- outMsg{msg: wire.NewMsgVerAck()}
	}

	close(p.quit)

	if !p.Connected() {
		t.Fatal("peer should still report connected for this race")
	}

	return p
}

// TestQueueInventoryDoesNotBlockAfterQuit ensures QueueInventory gives up when
// the peer has quit rather than blocking forever on a full channel.
func TestQueueInventoryDoesNotBlockAfterQuit(t *testing.T) {
	t.Parallel()

	p := newQuitRacePeer(t)

	done := make(chan struct{})
	go func() {
		p.QueueInventory(wire.NewInvVect(wire.InvTypeBlock, &chainhash.Hash{0x01}))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("QueueInventory blocked after the peer quit")
	}
}

// TestQueueMessageDoesNotBlockAfterQuit ensures QueueMessageWithEncoding gives
// up when the peer has quit, and that it still signals the done channel so a
// caller waiting on it is not left blocked either.
func TestQueueMessageDoesNotBlockAfterQuit(t *testing.T) {
	t.Parallel()

	p := newQuitRacePeer(t)

	doneChan := make(chan struct{}, 1)
	returned := make(chan struct{})
	go func() {
		p.QueueMessageWithEncoding(wire.NewMsgVerAck(), doneChan,
			wire.BaseEncoding)
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("QueueMessageWithEncoding blocked after the peer quit")
	}

	select {
	case <-doneChan:
	case <-time.After(5 * time.Second):
		t.Fatal("done channel was not signaled after the peer quit")
	}
}
