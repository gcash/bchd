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

// TestMaybeAddDeadlineGetBlocksStopHash verifies that the stall deadline for an
// outgoing getblocks depends on its stop hash.  A getblocks with a zero stop
// hash is an open-ended request the protocol does not guarantee a response to --
// a fully-synced peer legitimately sends no inv, which is exactly what bchd
// itself does in serverPeer.OnGetBlocks -- so it must not arm a deadline
// (btcsuite/btcd#1317).  A getblocks with a non-zero stop hash targets a
// specific block we expect the peer to have, so it must arm an inv deadline.
func TestMaybeAddDeadlineGetBlocksStopHash(t *testing.T) {
	t.Parallel()

	// maybeAddDeadline does not dereference the peer, so a zero-value Peer
	// is sufficient to exercise it.
	p := &Peer{}

	// Zero stop hash: open-ended request the protocol need not answer, so
	// no deadline is armed.  This is what netsync sends to start a sync.
	zero := make(map[string]time.Time)
	p.maybeAddDeadline(zero, wire.NewMsgGetBlocks(&chainhash.Hash{}))
	if len(zero) != 0 {
		t.Fatalf("zero stopHash getblocks armed %d deadline(s); want 0: %v",
			len(zero), zero)
	}

	// Non-zero stop hash: targets a specific expected block, as netsync
	// does when resolving an orphan root, so an inv deadline is armed.
	stop := chainhash.Hash{0x01}
	nonZero := make(map[string]time.Time)
	p.maybeAddDeadline(nonZero, wire.NewMsgGetBlocks(&stop))
	if _, ok := nonZero[wire.CmdInv]; !ok {
		t.Fatalf("non-zero stopHash getblocks armed no inv deadline: %v",
			nonZero)
	}

	// Positive control: the other request commands must still arm a
	// deadline.  Only the open-ended getblocks is intentionally exempt, so
	// this guards against the deadline logic being removed wholesale by
	// mistake, or the exemption silently widening to other commands.
	na := &wire.NetAddress{}
	for _, msg := range []wire.Message{
		wire.NewMsgVersion(na, na, 0, 0),
		wire.NewMsgMemPool(),
		wire.NewMsgGetCFMempool(wire.GCSFilterRegular),
		wire.NewMsgGetBlockTxns(chainhash.Hash{}, nil),
		wire.NewMsgGetData(),
		wire.NewMsgGetHeaders(),
	} {
		pending := make(map[string]time.Time)
		p.maybeAddDeadline(pending, msg)
		if len(pending) == 0 {
			t.Fatalf("%s armed no stall deadline; want >= 1",
				msg.Command())
		}
	}
}

// TestGetDataAnsweredByDSProof verifies that a dsproof-beta message answers a
// getdata, so a getdata asking only for proofs does not stall the peer.
func TestGetDataAnsweredByDSProof(t *testing.T) {
	t.Parallel()

	p := &Peer{}
	pending := make(map[string]time.Time)
	p.maybeAddDeadline(pending, wire.NewMsgGetData())
	if len(pending) == 0 {
		t.Fatal("getdata armed no deadline")
	}
	removeExpectedResponse(pending, wire.CmdDSProof)
	if len(pending) != 0 {
		t.Fatalf("deadlines left after a dsproof-beta answer: %v", pending)
	}
}

// TestGetBlockTxnsAnsweredByBlock verifies that a block answers a
// getblocktxns, since a peer may send the full block instead of the
// transactions.
func TestGetBlockTxnsAnsweredByBlock(t *testing.T) {
	t.Parallel()

	p := &Peer{}
	pending := make(map[string]time.Time)
	p.maybeAddDeadline(pending, wire.NewMsgGetBlockTxns(chainhash.Hash{}, nil))
	removeExpectedResponse(pending, wire.CmdBlock)
	if len(pending) != 0 {
		t.Fatalf("deadlines left after a block answer: %v", pending)
	}

	// Other answers to a getdata don't answer a getblocktxns.
	for _, cmd := range []string{wire.CmdCmpctBlock, wire.CmdTx,
		wire.CmdNotFound} {

		pending := make(map[string]time.Time)
		p.maybeAddDeadline(pending,
			wire.NewMsgGetBlockTxns(chainhash.Hash{}, nil))
		removeExpectedResponse(pending, cmd)
		if _, ok := pending[wire.CmdBlockTxns]; !ok {
			t.Fatalf("%s removed the blocktxns deadline", cmd)
		}
	}
}

// TestStallHandlerWaitsForBothHandlers verifies that the stall handler keeps
// taking control messages after the input or the output handler is done, and
// leaves once both are.  A closed quit channel stays ready, so it took the
// first handler's twice and left, and the other handler, still handling a
// message after a disconnect, blocked forever on its next control message,
// holding the peer and the message.
func TestStallHandlerWaitsForBothHandlers(t *testing.T) {
	t.Parallel()

	for _, inFirst := range []bool{true, false} {
		p := newPeerBase(&Config{}, false)
		done := make(chan struct{})
		go func() {
			p.stallHandler()
			close(done)
		}()

		first, second := p.outQuit, p.inQuit
		if inFirst {
			first, second = p.inQuit, p.outQuit
		}
		close(first)
		select {
		case <-done:
			t.Fatalf("input handler done first %v: stall handler left "+
				"while the other handler ran", inFirst)
		case <-time.After(200 * time.Millisecond):
		}

		// A message received and handled after a disconnect.
		for _, command := range []stallControlCmd{sccReceiveMessage,
			sccHandlerStart, sccHandlerDone} {

			select {
			case p.stallControl <- stallControlMsg{command, wire.NewMsgPing(0)}:
			case <-time.After(5 * time.Second):
				t.Fatalf("input handler done first %v: control message "+
					"%d not taken", inFirst, command)
			}
		}

		close(second)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("input handler done first %v: stall handler stayed "+
				"after both handlers were done", inFirst)
		}
	}
}
