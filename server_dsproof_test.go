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

// servePeers answers the server's peer queries with peers until the test
// ends.
func servePeers(t *testing.T, s *server, peers ...*serverPeer) {
	s.query = make(chan interface{})
	go func() {
		for {
			select {
			case q := <-s.query:
				if m, ok := q.(getPeersMsg); ok {
					m.reply <- peers
				}
			case <-s.quit:
				return
			}
		}
	}()
	t.Cleanup(func() { close(s.quit) })
}

// waitBanScore waits for the ban score of sp to reach want.
func waitBanScore(t *testing.T, sp *serverPeer, want uint32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for sp.banScore.Int() != want {
		if time.Now().After(deadline) {
			t.Fatalf("ban score %d, want %d", sp.banScore.Int(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAddDSProofBanScore checks that the ban score of orphan senders, such as
// expired orphans, is raised by peer id, summed over their orphans and
// applied once per peer, so a peer passing the threshold is banned once.
func TestAddDSProofBanScore(t *testing.T) {
	sp, _ := newDSProofServerPeer(t)
	// The first sum stays at the warning level, half the threshold.
	cfg.BanThreshold = 5
	s := sp.server
	s.banPeers = make(chan *serverPeer, 10)
	servePeers(t, s, sp)

	id := mempool.Tag(sp.ID())
	s.addDSProofBanScore([]mempool.Tag{id, id + 1, id}, 1, "dsproof-orphan-expired")
	waitBanScore(t, sp, 2)

	// A sum passing the threshold bans the peer once.
	s.addDSProofBanScore([]mempool.Tag{id, id, id, id, id}, 1, "dsproof-orphan-expired")
	select {
	case <-s.banPeers:
	case <-time.After(5 * time.Second):
		t.Fatal("peer not banned")
	}
	select {
	case <-s.banPeers:
		t.Fatal("peer banned twice")
	case <-time.After(100 * time.Millisecond):
	}
	if score := sp.banScore.Int(); score != 7 {
		t.Fatalf("ban score %d", score)
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

// TestWantsDSProof checks which peers are told about a proof: not those with
// transaction relay disabled, nor those whose bloom filter does not match
// the double spent transaction.
func TestWantsDSProof(t *testing.T) {
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	h := newMempoolTestHarness(t, 1)
	tx := bchutil.NewTx(h.spend(t, h.outputs[0], 1000))
	txHash := tx.Hash()

	sp := newBloomTestPeer()
	if !sp.wantsDSProof(tx) {
		t.Fatal("peer without a filter not told")
	}
	sp.setDisableRelayTx(true)
	if sp.wantsDSProof(tx) {
		t.Fatal("peer with transaction relay disabled told")
	}
	sp.setDisableRelayTx(false)

	other := bloom.NewFilter(10, 0, 0.0001, wire.BloomUpdateNone)
	other.Add([]byte{1, 2, 3})
	sp.filter = other
	if sp.wantsDSProof(tx) {
		t.Fatal("peer with a filter not matching told")
	}
	match := bloom.NewFilter(10, 0, 0.0001, wire.BloomUpdateNone)
	match.AddHash(txHash)
	sp.filter = match
	if !sp.wantsDSProof(tx) {
		t.Fatal("peer with a matching filter not told")
	}
}

// TestDSProofBadOrphanBanScore checks that the sender of an orphan proof
// that turns out invalid once its transaction arrives gets a ban score of
// 10, unless it is whitelisted.
func TestDSProofBadOrphanBanScore(t *testing.T) {
	for _, whitelisted := range []bool{false, true} {
		useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
		s := &server{quit: make(chan struct{})}
		h := newMempoolTestHarness(t, 2, func(c *mempool.Config) {
			c.OnBadDSProofs = s.onBadDSProofs
		})
		pool := h.server.cfg.TxMemPool
		s.txMemPool = pool
		sp := &serverPeer{
			Peer:          peer.NewInboundPeer(&peer.Config{}),
			server:        s,
			filter:        bloom.LoadFilter(nil),
			isWhitelisted: whitelisted,
		}
		servePeers(t, s, sp)

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

		// The signatures are for the first output, so the proof is
		// an orphan for the second until a transaction spends it.
		proof := descs[0].Proof
		bad := wire.NewMsgDSProof(h.outputs[1], proof.FirstSpender, proof.DoubleSpender)
		sp.OnDSProof(nil, bad)
		if hash := bad.ProofHash(); !pool.HaveDSProof(&hash) || sp.banScore.Int() != 0 {
			t.Fatal("orphan not stored")
		}
		tx3 := h.spend(t, h.outputs[1], 1000)
		if _, err := pool.ProcessTransaction(bchutil.NewTx(tx3), false, false, 0); err != nil {
			t.Fatal(err)
		}
		if whitelisted {
			time.Sleep(100 * time.Millisecond)
			if sp.banScore.Int() != 0 {
				t.Fatal("whitelisted peer scored")
			}
			continue
		}
		waitBanScore(t, sp, 10)
	}
}

// TestOnDSProofBlocksOnly checks that proofs are dropped in blocks only
// mode without touching the memory pool.
func TestOnDSProofBlocksOnly(t *testing.T) {
	sp, proof := newDSProofServerPeer(t)
	cfg.BlocksOnly = true
	sp.server.txMemPool = nil
	sp.OnDSProof(nil, invalidCopy(proof))
	if sp.banScore.Int() != 0 {
		t.Fatal("proof processed in blocks only mode")
	}
}
