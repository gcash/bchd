// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/txscript"
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

// TestFilterAddWithoutFilter ensures filteradd without a loaded filter keeps
// the peer, whose transactions still all match, as BCHN gives every peer a
// filter that matches everything until it loads one, and adding to that
// filter changes nothing.  The peer was disconnected.  On a server without
// bloom filtering, filteradd still disconnects, as in BCHN.
func TestFilterAddWithoutFilter(t *testing.T) {
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	sp := newBloomTestPeer()
	sp.OnFilterAdd(nil, wire.NewMsgFilterAdd([]byte{1, 2, 3}))
	if isDisconnected(sp) {
		t.Error("peer was disconnected")
	}
	if sp.filter.IsLoaded() {
		t.Error("filteradd loaded a filter")
	}

	sp = newBloomTestPeer()
	sp.server.services = 0
	sp.OnFilterAdd(nil, wire.NewMsgFilterAdd([]byte{1, 2, 3}))
	if !isDisconnected(sp) {
		t.Error("peer was not disconnected by a server without bloom " +
			"filtering")
	}
}

// TestFilterClear ensures filterclear leaves the peer without a filter and
// relaying transactions, whether or not it loaded a filter and whether or not
// the server offers bloom filtering, as in BCHN.  A peer without a filter was
// disconnected, and one on a server without bloom filtering banned.
func TestFilterClear(t *testing.T) {
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	tests := []struct {
		name     string
		services wire.ServiceFlag
		load     bool
	}{
		{"filter loaded", wire.SFNodeBloom, true},
		{"no filter", wire.SFNodeBloom, false},
		{"no bloom filtering", 0, false},
	}
	for _, test := range tests {
		sp := newBloomTestPeer()
		sp.server.services = test.services
		// The peer asked for no transactions in its version message.
		sp.setDisableRelayTx(true)
		if test.load {
			sp.OnFilterLoad(nil, wire.NewMsgFilterLoad(make([]byte, 8),
				5, 0, wire.BloomUpdateAll))
		}

		sp.OnFilterClear(nil, wire.NewMsgFilterClear())
		if isDisconnected(sp) {
			t.Errorf("%s: peer was disconnected", test.name)
			continue
		}
		if score := sp.banScore.Int(); score != 0 {
			t.Errorf("%s: ban score %d", test.name, score)
		}
		if sp.filter.IsLoaded() {
			t.Errorf("%s: a filter is still loaded", test.name)
		}
		if sp.relayTxDisabled() {
			t.Errorf("%s: transactions are not relayed", test.name)
		}
	}
}

// TestMemPoolReplyParentsFirst ensures the reply to a mempool message lists
// each transaction after the ones it spends, as BCHN lists its memory pool in
// the order the transactions entered it.  With a filter that adds the outputs
// it matches, a transaction spending a matched output then matches as well.
// In the memory pool's map order, such a transaction listed before the one it
// spends was left out.
func TestMemPoolReplyParentsFirst(t *testing.T) {
	const pairs = 20
	h := newMempoolTestHarness(t, pairs)
	pool := h.server.cfg.TxMemPool

	// Each parent pays to the key the filter holds, and its child to a key
	// it does not.
	other := append([]byte{txscript.OP_DUP, txscript.OP_HASH160,
		txscript.OP_DATA_20}, make([]byte, 20)...)
	other = append(other, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)
	parentOf := make(map[chainhash.Hash]chainhash.Hash)
	for _, out := range h.outputs {
		parent := h.spend(t, out, 1000)
		if _, err := pool.ProcessTransaction(bchutil.NewTx(parent), false,
			0); err != nil {

			t.Fatal(err)
		}
		child := wire.NewMsgTx(wire.TxVersion)
		child.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: parent.TxHash()},
			Sequence:         wire.MaxTxInSequenceNum,
		})
		child.AddTxOut(wire.NewTxOut(parent.TxOut[0].Value-1000, other,
			wire.TokenData{}))
		signTestInput(t, h, child, parent.TxOut[0].Value)
		if _, err := pool.ProcessTransaction(bchutil.NewTx(child), false,
			0); err != nil {

			t.Fatal(err)
		}
		parentOf[child.TxHash()] = parent.TxHash()
	}

	sp := newBloomTestPeer()
	sp.server.txMemPool = pool
	sp.filter = bloom.NewFilter(1000, 0, 1e-9, wire.BloomUpdateAll)
	sp.filter.Add(h.pkScript[3:23])
	listed := mempoolReplyHashes(sp)

	pos := make(map[chainhash.Hash]int)
	for i, hash := range listed {
		pos[hash] = i
	}
	if len(listed) != 2*pairs || len(pos) != 2*pairs {
		t.Fatalf("%d transactions listed, %d of them once, want %d",
			len(listed), len(pos), 2*pairs)
	}
	for child, parent := range parentOf {
		if pos[parent] > pos[child] {
			t.Fatalf("child %v listed before its parent", child)
		}
	}
}

// TestMemPoolReplyFeeFilter ensures the reply to a mempool message leaves out
// the transactions paying less than the peer's fee filter, and keeps those
// paying as much, as BCHN does.
func TestMemPoolReplyFeeFilter(t *testing.T) {
	h := newMempoolTestHarness(t, 2)
	pool := h.server.cfg.TxMemPool
	low, err := pool.ProcessTransaction(bchutil.NewTx(
		h.spend(t, h.outputs[0], 1000)), false, 0)
	if err != nil {
		t.Fatal(err)
	}
	high, err := pool.ProcessTransaction(bchutil.NewTx(
		h.spend(t, h.outputs[1], 100000)), false, 0)
	if err != nil {
		t.Fatal(err)
	}

	if low[0].FeePerKB >= high[0].FeePerKB {
		t.Fatalf("fee rates %d and %d", low[0].FeePerKB, high[0].FeePerKB)
	}

	sp := newBloomTestPeer()
	sp.server.txMemPool = pool
	atomic.StoreInt64(&sp.feeFilter, high[0].FeePerKB)
	listed := mempoolReplyHashes(sp)
	if len(listed) != 1 || listed[0] != *high[0].Tx.Hash() {
		t.Fatalf("listed %v, want only %v", listed, high[0].Tx.Hash())
	}
}

// mempoolReplyHashes returns the transactions the reply to a mempool message
// from sp lists, in order, over all its inventory messages.
func mempoolReplyHashes(sp *serverPeer) []chainhash.Hash {
	var hashes []chainhash.Hash
	for _, inv := range sp.mempoolInvs() {
		for _, iv := range inv.InvList {
			hashes = append(hashes, iv.Hash)
		}
	}
	return hashes
}

// TestMemPoolReplyBloomFilter ensures the reply to a mempool message from a
// peer that loaded a bloom filter lists only the transactions that match it.
func TestMemPoolReplyBloomFilter(t *testing.T) {
	h := newMempoolTestHarness(t, 2)
	pool := h.server.cfg.TxMemPool

	// One transaction pays to the key the filter holds, and the other to a
	// key it does not.
	match := h.spend(t, h.outputs[0], 1000)
	other := append([]byte{txscript.OP_DUP, txscript.OP_HASH160,
		txscript.OP_DATA_20}, make([]byte, 20)...)
	other = append(other, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)
	noMatch := wire.NewMsgTx(wire.TxVersion)
	noMatch.AddTxIn(&wire.TxIn{
		PreviousOutPoint: h.outputs[1],
		Sequence:         wire.MaxTxInSequenceNum,
	})
	noMatch.AddTxOut(wire.NewTxOut(mempoolTestOutputValue-1000, other,
		wire.TokenData{}))
	h.sign(t, noMatch)
	for _, tx := range []*wire.MsgTx{match, noMatch} {
		if _, err := pool.ProcessTransaction(bchutil.NewTx(tx), false,
			0); err != nil {

			t.Fatal(err)
		}
	}

	sp := newBloomTestPeer()
	sp.server.txMemPool = pool
	sp.filter = bloom.NewFilter(1000, 0, 1e-9, wire.BloomUpdateNone)
	sp.filter.Add(h.pkScript[3:23])
	listed := mempoolReplyHashes(sp)
	if len(listed) != 1 || listed[0] != match.TxHash() {
		t.Fatalf("listed %v, want only %v", listed, match.TxHash())
	}
}

// TestMemPoolReplyWholePool ensures the reply to a mempool message lists the
// whole memory pool, as BCHN's does, in as many inventory messages of at most
// maxMempoolInvs transactions as it takes, rather than as many as fit in one.
func TestMemPoolReplyWholePool(t *testing.T) {
	saved := maxMempoolInvs
	maxMempoolInvs = 3
	t.Cleanup(func() { maxMempoolInvs = saved })

	const n = 7
	h := newMempoolTestHarness(t, n)
	pool := h.server.cfg.TxMemPool
	want := make(map[chainhash.Hash]struct{})
	for _, out := range h.outputs {
		tx := h.spend(t, out, 1000)
		if _, err := pool.ProcessTransaction(bchutil.NewTx(tx), false,
			0); err != nil {

			t.Fatal(err)
		}
		want[tx.TxHash()] = struct{}{}
	}

	sp := newBloomTestPeer()
	sp.server.txMemPool = pool
	var sizes []int
	listed := make(map[chainhash.Hash]struct{})
	for _, inv := range sp.mempoolInvs() {
		sizes = append(sizes, len(inv.InvList))
		for _, iv := range inv.InvList {
			listed[iv.Hash] = struct{}{}
		}
	}
	if len(sizes) != 3 || sizes[0] != 3 || sizes[1] != 3 || sizes[2] != 1 {
		t.Fatalf("inventory messages of %v transactions, want [3 3 1]",
			sizes)
	}
	for hash := range want {
		if _, ok := listed[hash]; !ok || len(listed) != n {
			t.Fatalf("listed %d transactions, not %v, want all %d",
				len(listed), hash, n)
		}
	}
}

// TestParentsFirstDescs ensures the memory pool's transactions are each put
// after those of the list it spends, in the order of its inputs, and once.
func TestParentsFirstDescs(t *testing.T) {
	desc := func(tag byte, prevs ...chainhash.Hash) *mempool.TxDesc {
		msg := wire.NewMsgTx(2)
		for _, prev := range prevs {
			msg.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: prev},
				[]byte{tag}))
		}
		return &mempool.TxDesc{TxDesc: mining.TxDesc{Tx: bchutil.NewTx(msg)}}
	}
	hash := func(d *mempool.TxDesc) chainhash.Hash { return *d.Tx.Hash() }

	// c spends b, an outside output and a, in that order.
	a := desc(1, chainhash.Hash{1})
	b := desc(2, chainhash.Hash{2})
	c := desc(3, hash(b), chainhash.Hash{3}, hash(a))
	// e spends d, listed before it, and f spends e and d.
	d := desc(4, chainhash.Hash{4})
	e := desc(5, hash(d))
	f := desc(6, hash(e), hash(d))

	tests := []struct {
		name       string
		list, want []*mempool.TxDesc
	}{
		{"input order", []*mempool.TxDesc{c, a, b},
			[]*mempool.TxDesc{b, a, c}},
		{"shared parent", []*mempool.TxDesc{d, f, e},
			[]*mempool.TxDesc{d, e, f}},
	}
	for _, test := range tests {
		got := parentsFirstDescs(test.list)
		if len(got) != len(test.want) {
			t.Errorf("%s: %d transactions, want %d", test.name, len(got),
				len(test.want))
			continue
		}
		for i := range test.want {
			if got[i] != test.want[i] {
				t.Errorf("%s: position %d holds %v, want %v", test.name,
					i, got[i].Tx.Hash(), test.want[i].Tx.Hash())
			}
		}
	}
}
