// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs/builder"
)

// mpTestTx returns a transaction that spends one output and pays script.
func mpTestTx(n byte, script []byte) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{n}, 0), nil))
	tx.AddTxOut(wire.NewTxOut(1000, script, wire.TokenData{}))
	return tx
}

// TestMpFilters checks that each interval with transactions gets the next
// number and a filter that matches the scripts its transactions pay and
// spend, keyed by the hash of their IDs; that an empty interval gets neither;
// that an interval closes early when it is full; and that only the latest
// intervals are kept.
func TestMpFilters(t *testing.T) {
	m := newMpFilters()
	m.nextSeq = 100
	m.now = func() time.Time { return time.Unix(1794461717, 0) }
	paid, spent := []byte{0x51}, []byte{0x52}

	m.tick()
	if len(m.kept) != 0 || m.nextSeq != 100 {
		t.Fatalf("an empty interval was kept or numbered: %d, %d", len(m.kept), m.nextSeq)
	}

	txA, txB := mpTestTx(1, paid), mpTestTx(2, paid)
	m.add(txA, [][]byte{spent})
	m.add(txB, [][]byte{spent})
	m.tick()
	iv := m.interval(100)
	if iv == nil || len(iv.txids) != 2 || iv.time != 1794461717 || m.nextSeq != 101 {
		t.Fatalf("first interval: %+v, next %d", iv, m.nextSeq)
	}
	wantHash := wire.MpIntervalHash([]chainhash.Hash{txA.TxHash(), txB.TxHash()})
	if iv.hash != wantHash {
		t.Errorf("interval hash %v, want %v", iv.hash, wantHash)
	}
	filter, err := builder.BuildTxScriptFilter(&wantHash, []*wire.MsgTx{txA, txB}, [][]byte{spent, spent})
	if err != nil {
		t.Fatal(err)
	}
	wantFilter, _ := filter.NBytes()
	if string(iv.filter) != string(wantFilter) {
		t.Errorf("filter %x, want %x", iv.filter, wantFilter)
	}
	key := builder.DeriveKey(&wantHash)
	for _, script := range [][]byte{paid, spent} {
		k := builder.ScriptKey(script)
		if match, _ := filter.Match(key, k[:]); !match {
			t.Errorf("script %x does not match", script)
		}
	}
	msg := iv.msg()
	if msg.Seq != 100 || msg.TxCount != 2 || msg.IntervalHash != wantHash || msg.FilterType != wire.GCSFilterScripts {
		t.Errorf("mpfilter message %+v", msg)
	}

	// A full interval closes without waiting for the tick.
	for i := 0; i < mpFilterMaxTxs; i++ {
		m.add(mpTestTx(3, paid), [][]byte{spent})
	}
	if iv := m.interval(101); iv == nil || len(iv.txids) != mpFilterMaxTxs || len(m.txs) != 0 {
		t.Fatalf("a full interval did not close")
	}

	// Only the latest mpFilterKeep intervals are kept, and since returns
	// those from a number on.
	for i := 0; i < mpFilterKeep; i++ {
		m.add(mpTestTx(4, paid), [][]byte{spent})
		m.tick()
	}
	if len(m.kept) != mpFilterKeep || m.interval(100) != nil || m.interval(101) != nil {
		t.Errorf("kept %d intervals, oldest %d", len(m.kept), m.kept[0].seq)
	}
	last := m.nextSeq - 1
	if got := m.since(last - 2); len(got) != 3 || got[0].seq != last-2 || got[2].seq != last {
		t.Errorf("since: %d intervals", len(got))
	}
	if got := m.since(last + 1); got != nil {
		t.Errorf("since past the last: %d intervals", len(got))
	}
	// A run of intervals comes oldest first, without those not kept.
	if got := m.run(last-1, last+5); len(got) != 2 || got[0].seq != last-1 || got[1].seq != last {
		t.Errorf("run: %d intervals", len(got))
	}
	if got := m.run(0, 101); got != nil {
		t.Errorf("run of intervals no longer kept: %d", len(got))
	}

	// Spent output scripts that don't line up give no interval, and the
	// next one still gets the next number. The loggers can't write before
	// the log file is set up, so the error is not logged here.
	oldLevel := peerLog.Level()
	peerLog.SetLevel(bchlog.LevelOff)
	t.Cleanup(func() { peerLog.SetLevel(oldLevel) })
	m.add(mpTestTx(5, paid), nil)
	m.tick()
	if m.nextSeq != last+1 || len(m.txs) != 0 {
		t.Errorf("a failed interval was numbered or kept open")
	}
}

// TestMpFiltersDropPeersThatLeft checks that a peer that asked for the
// filters and left is dropped at the next tick, with no transaction in the
// interval, and that a peer that is still connected is kept.
func TestMpFiltersDropPeersThatLeft(t *testing.T) {
	m := newMpFilters()

	newPeer := func() *serverPeer {
		p, err := peer.NewOutboundPeer(&peer.Config{
			ChainParams: &chaincfg.SimNetParams,
		}, "127.0.0.1:18555")
		if err != nil {
			t.Fatalf("new peer: %v", err)
		}
		return &serverPeer{Peer: p}
	}

	// A peer counts as connected from the time it has a connection. The
	// other end is never read, so the handshake doesn't go on.
	here := newPeer()
	near, far := net.Pipe()
	defer func() { _ = far.Close() }()
	here.AssociateConnection(near)
	defer here.Disconnect()
	if !here.Connected() {
		t.Fatal("the peer with a connection is not connected")
	}

	left := newPeer()
	m.subscribe(here)
	m.subscribe(left)
	if len(m.subscribers) != 2 {
		t.Fatalf("%d subscribers, want 2", len(m.subscribers))
	}

	m.tick()
	if _, ok := m.subscribers[left]; ok {
		t.Error("a peer that left is still a subscriber after a tick")
	}
	if _, ok := m.subscribers[here]; !ok {
		t.Error("a connected peer is no longer a subscriber after a tick")
	}
	if len(m.kept) != 0 {
		t.Errorf("an interval without transactions was kept")
	}
}

// TestMempoolScriptFilterOf checks that the script filter of the mempool
// leaves out a transaction whose spent outputs are gone, and that no filter
// is built when the outputs of one can't be looked up.
func TestMempoolScriptFilterOf(t *testing.T) {
	paid, spentScript, gonePaid := []byte{0x51}, []byte{0x52}, []byte{0x53}
	txA, txGone := mpTestTx(1, paid), mpTestTx(2, gonePaid)
	txDescs := []*mempool.TxDesc{{}, {}}
	txDescs[0].Tx = bchutil.NewTx(txA)
	txDescs[1].Tx = bchutil.NewTx(txGone)

	lookupFailed := errors.New("lookup failed")
	var failFor *chainhash.Hash
	spent := func(tx *bchutil.Tx) ([][]byte, error) {
		switch {
		case failFor != nil && tx.Hash().IsEqual(failFor):
			return nil, lookupFailed
		case tx.Hash().IsEqual(txDescs[1].Tx.Hash()):
			return nil, errSpentOutputGone
		}
		return [][]byte{spentScript}, nil
	}

	got, err := mempoolScriptFilterOf(txDescs, spent)
	if err != nil {
		t.Fatalf("filter of the mempool: %v", err)
	}
	filter, err := builder.BuildTxScriptFilter(&chainhash.Hash{},
		[]*wire.MsgTx{txA}, [][]byte{spentScript})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filter.NBytes()
	if string(got) != string(want) {
		t.Errorf("filter %x, want %x, of the one transaction still there", got, want)
	}

	failFor = txDescs[0].Tx.Hash()
	got, err = mempoolScriptFilterOf(txDescs, spent)
	if !errors.Is(err, lookupFailed) || got != nil {
		t.Errorf("after a failed lookup: filter %x, err %v", got, err)
	}
}
