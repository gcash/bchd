// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/bloom"
)

// newDSProofServerPeer returns a server peer on a server whose memory pool
// holds a transaction with a double spend proof, and that proof.
func newDSProofServerPeer(t *testing.T) (*serverPeer, *wire.MsgDSProof) {
	t.Helper()
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	h := newMempoolTestHarness(t, 1)
	pool := h.server.cfg.TxMemPool
	tx1 := h.spend(t, h.outputs[0], 1000)
	tx2 := h.spend(t, h.outputs[0], 2000)
	if _, err := pool.ProcessTransaction(bchutil.NewTx(tx1), false, false, 0); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.ProcessTransaction(bchutil.NewTx(tx2), false, false, 0)
	descs := pool.DSProofs(false)
	if len(descs) != 1 {
		t.Fatalf("%d proofs", len(descs))
	}
	sp := &serverPeer{
		Peer:   peer.NewInboundPeer(&peer.Config{}),
		server: &server{txMemPool: pool, quit: make(chan struct{})},
		filter: bloom.LoadFilter(nil),
	}
	return sp, descs[0].Proof
}

// invalidCopy returns a copy of proof for another outpoint with its spenders
// swapped, which puts them out of order.
func invalidCopy(proof *wire.MsgDSProof) *wire.MsgDSProof {
	return wire.NewMsgDSProof(wire.OutPoint{Hash: chainhash.Hash{9}},
		proof.DoubleSpender, proof.FirstSpender)
}

// TestOnDSProofBanScore checks that an invalid proof adds 10 to the sender's
// ban score, that a known one adds nothing, and that whitelisted peers are
// exempt.
func TestOnDSProofBanScore(t *testing.T) {
	sp, proof := newDSProofServerPeer(t)
	sp.OnDSProof(nil, proof)
	if score := sp.banScore.Int(); score != 0 {
		t.Fatalf("ban score %d for a known proof", score)
	}
	sp.OnDSProof(nil, invalidCopy(proof))
	if score := sp.banScore.Int(); score != 10 {
		t.Fatalf("ban score %d for an invalid proof", score)
	}

	white, proof2 := newDSProofServerPeer(t)
	white.isWhitelisted = true
	white.OnDSProof(nil, invalidCopy(proof2))
	if score := white.banScore.Int(); score != 0 {
		t.Fatalf("ban score %d for a whitelisted peer", score)
	}
}

// TestAddDSProofBanScore checks that the ban score of orphan senders, such as
// expired orphans, is raised by peer id.
func TestAddDSProofBanScore(t *testing.T) {
	sp, _ := newDSProofServerPeer(t)
	s := sp.server
	s.query = make(chan interface{})
	go func() {
		for {
			select {
			case q := <-s.query:
				if m, ok := q.(getPeersMsg); ok {
					m.reply <- []*serverPeer{sp}
				}
			case <-s.quit:
				return
			}
		}
	}()
	defer close(s.quit)

	s.addDSProofBanScore([]mempool.Tag{mempool.Tag(sp.ID())}, 1, "dsproof-orphan-expired")
	deadline := time.Now().Add(5 * time.Second)
	for sp.banScore.Int() != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("ban score %d", sp.banScore.Int())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestPushDSProofMsg checks that getdata for a known proof is answered and an
// unknown one is reported as not found, signalling completion either way.
func TestPushDSProofMsg(t *testing.T) {
	sp, proof := newDSProofServerPeer(t)
	known := proof.ProofHash()
	for _, test := range []struct {
		hash  chainhash.Hash
		found bool
	}{{known, true}, {chainhash.Hash{1}, false}} {
		done := make(chan struct{}, 1)
		err := sp.server.pushDSProofMsg(sp, &test.hash, done, nil, wire.BaseEncoding)
		if (err == nil) != test.found {
			t.Fatalf("%v: error %v", test.hash, err)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%v: no completion signal", test.hash)
		}
	}

	// A getdata with only an unknown proof returns.
	msg := wire.NewMsgGetData()
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeDSProof, &chainhash.Hash{2}))
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
