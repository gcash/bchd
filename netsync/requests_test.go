// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/mempool"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// viewCountingDB counts the read transactions run on it.
type viewCountingDB struct {
	database.DB
	views atomic.Int64
}

func (db *viewCountingDB) View(fn func(database.Tx) error) error {
	db.views.Add(1)
	return db.DB.View(fn)
}

// requestTestHarness is a sync manager whose regression test chain is
// current, so it requests what any peer announces, with peers whose getdata
// messages are recorded.  The reads of the chain's database are counted.
type requestTestHarness struct {
	t      *testing.T
	sm     *SyncManager
	db     *viewCountingDB
	params *chaincfg.Params
	tip    *wire.MsgBlock
	clock  time.Time

	mtx sync.Mutex
	got map[*peerpkg.Peer][]*wire.InvVect
}

// newRequestTestHarness returns a request test harness.
func newRequestTestHarness(t *testing.T) *requestTestHarness {
	t.Helper()

	params := chaincfg.RegressionNetParams
	created, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { created.Close() })
	db := &viewCountingDB{DB: created}
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatal(err)
	}

	// A block with the current time makes the chain current.
	tip := mineBlock(t, &params, params.GenesisBlock, 1, 0).MsgBlock()
	tip.Header.Timestamp = time.Unix(time.Now().Unix(), 0)
	solveBlock(tip)
	_, _, err = chain.ProcessBlock(bchutil.NewBlock(tip), blockchain.BFNone)
	if err != nil {
		t.Fatal(err)
	}

	sm, err := New(&Config{
		Chain:       chain,
		ChainParams: &params,
		TxMemPool: mempool.New(&mempool.Config{
			ChainParams:    &params,
			BestHeight:     func() int32 { return 1 },
			MedianTimePast: func() time.Time { return time.Now() },
		}),
		PeerNotifier: nopNotifier{},
		MaxPeers:     8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sm.current() {
		t.Fatal("sync manager is not current")
	}
	h := &requestTestHarness{
		t:      t,
		sm:     sm,
		db:     db,
		params: &params,
		tip:    tip,
		clock:  time.Now(),
		got:    make(map[*peerpkg.Peer][]*wire.InvVect),
	}
	sm.timeNow = func() time.Time { return h.clock }
	return h
}

// advance moves the sync manager's clock forward and checks its requests for
// timeouts.
func (h *requestTestHarness) advance(d time.Duration) {
	h.clock = h.clock.Add(d)
	h.sm.handleRequestTimeouts()
}

// solveBlock sets the nonce of the block so that its hash meets its target.
func solveBlock(block *wire.MsgBlock) {
	target := blockchain.CompactToBig(block.Header.Bits)
	for {
		hash := block.Header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			return
		}
		block.Header.Nonce++
	}
}

// peer returns a peer of the sync manager whose handshake has completed, and
// records the getdata messages the sync manager sends it.
func (h *requestTestHarness) peer(index byte) *peerpkg.Peer {
	t := h.t
	t.Helper()

	conn, remoteConn := net.Pipe()
	local := peerpkg.NewInboundPeer(&peerpkg.Config{
		ChainParams:            h.params,
		TstAllowSelfConnection: true,
	})
	remote, err := peerpkg.NewOutboundPeer(&peerpkg.Config{
		ChainParams:            h.params,
		TstAllowSelfConnection: true,
		Listeners: peerpkg.MessageListeners{
			OnGetData: func(_ *peerpkg.Peer, msg *wire.MsgGetData) {
				h.mtx.Lock()
				h.got[local] = append(h.got[local], msg.InvList...)
				h.mtx.Unlock()
			},
		},
	}, fmt.Sprintf("10.0.0.%d:18444", index))
	if err != nil {
		t.Fatal(err)
	}
	local.AssociateConnection(tcpConn{conn})
	remote.AssociateConnection(tcpConn{remoteConn})
	t.Cleanup(func() {
		local.Disconnect()
		remote.Disconnect()
	})
	deadline := time.Now().Add(5 * time.Second)
	for !local.VerAckReceived() || !remote.VerAckReceived() {
		if time.Now().After(deadline) {
			t.Fatal("handshake did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}

	h.sm.peerStates[local] = newPeerSyncState(false)
	return local
}

// inv has the peer announce the passed inventory.
func (h *requestTestHarness) inv(p *peerpkg.Peer, typ wire.InvType,
	hashes ...chainhash.Hash) {

	h.t.Helper()

	msg := wire.NewMsgInv()
	for i := range hashes {
		if err := msg.AddInvVect(wire.NewInvVect(typ, &hashes[i])); err != nil {
			h.t.Fatal(err)
		}
	}
	h.sm.handleInvMsg(&invMsg{inv: msg, peer: p})
}

// notFound has the peer report that it does not have the passed inventory.
func (h *requestTestHarness) notFound(p *peerpkg.Peer, typ wire.InvType,
	hash chainhash.Hash) {

	h.t.Helper()

	msg := wire.NewMsgNotFound()
	if err := msg.AddInvVect(wire.NewInvVect(typ, &hash)); err != nil {
		h.t.Fatal(err)
	}
	h.sm.handleNotFoundMsg(&notFoundMsg{notFound: msg, peer: p})
}

// waitGetData fails the test unless the sync manager asks the peer for the
// passed inventory within a few seconds.
func (h *requestTestHarness) waitGetData(p *peerpkg.Peer, typ wire.InvType,
	hash chainhash.Hash) {

	h.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.mtx.Lock()
		for _, iv := range h.got[p] {
			if iv.Type == typ && iv.Hash == hash {
				h.mtx.Unlock()
				return
			}
		}
		h.mtx.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("%v %v was not requested from peer %v", typ, hash, p)
}

// requestedTxFrom fails the test unless the transaction is requested from the
// peer, and from it alone.
func (h *requestTestHarness) requestedTxFrom(p *peerpkg.Peer,
	hash chainhash.Hash) {

	h.t.Helper()

	for peer, state := range h.sm.peerStates {
		_, ok := state.requestedTxns[hash]
		if ok != (peer == p) {
			h.t.Fatalf("transaction %v requested from peer %v: %v", hash,
				peer, ok)
		}
	}
}

// TestTxNotFoundRequestsNextAnnouncer ensures a transaction the peer it was
// requested from reports it does not have is requested at once from another
// peer that announced it, with a timeout of its own.  A peer that was not
// asked cannot move the request, and the peer asked is not asked again for
// announcing it again.
func TestTxNotFoundRequestsNextAnnouncer(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	hash := chainhash.Hash{0xaa}

	h.inv(a, wire.InvTypeTx, hash)
	h.waitGetData(a, wire.InvTypeTx, hash)
	h.inv(a, wire.InvTypeTx, hash)
	h.inv(b, wire.InvTypeTx, hash)
	h.notFound(b, wire.InvTypeTx, hash)
	h.requestedTxFrom(a, hash)

	h.advance(txRequestTimeout / 2)
	h.notFound(a, wire.InvTypeTx, hash)
	h.waitGetData(b, wire.InvTypeTx, hash)
	h.requestedTxFrom(b, hash)

	// The request to a would have expired by now.
	h.advance(txRequestTimeout/2 + time.Second)
	h.requestedTxFrom(b, hash)
	h.advance(txRequestTimeout / 2)
	h.requestedTxFrom(nil, hash)
}

// TestTxNotFoundForgetsRequest ensures a transaction a peer reports it does
// not have, and no other peer announced, is requested from the next peer that
// announces it.
func TestTxNotFoundForgetsRequest(t *testing.T) {
	h := newRequestTestHarness(t)
	a, c := h.peer(1), h.peer(3)
	hash := chainhash.Hash{0xaa}

	h.inv(a, wire.InvTypeTx, hash)
	h.notFound(a, wire.InvTypeTx, hash)
	h.inv(c, wire.InvTypeTx, hash)
	h.waitGetData(c, wire.InvTypeTx, hash)
	h.requestedTxFrom(c, hash)
}

// TestDonePeerRequestsNextAnnouncer ensures the transactions and blocks
// requested from a peer that disconnects are requested from another peer that
// announced them.
func TestDonePeerRequestsNextAnnouncer(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	tx, block := chainhash.Hash{0xaa}, chainhash.Hash{0xbb}

	h.inv(a, wire.InvTypeTx, tx)
	h.inv(a, wire.InvTypeBlock, block)
	h.inv(b, wire.InvTypeTx, tx)
	h.inv(b, wire.InvTypeBlock, block)

	h.sm.handleDonePeerMsg(a)
	h.waitGetData(b, wire.InvTypeTx, tx)
	h.waitGetData(b, wire.InvTypeCmpctBlock, block)
	h.requestedTxFrom(b, tx)
	if _, ok := h.sm.peerStates[b].requestedBlocks[block]; !ok {
		t.Fatal("block is not requested from the other announcer")
	}
}

// TestTxFromOtherPeerForgetsRequest ensures a transaction that arrives from
// another peer than the one it was requested from is no longer requested from
// either.
func TestTxFromOtherPeerForgetsRequest(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)

	// A transaction without inputs is rejected.
	tx := wire.NewMsgTx(1)
	tx.AddTxOut(wire.NewTxOut(1, []byte{0x51}, wire.TokenData{}))
	btx := bchutil.NewTx(tx)

	h.inv(a, wire.InvTypeTx, *btx.Hash())
	h.inv(b, wire.InvTypeTx, *btx.Hash())
	h.sm.handleTxMsg(&txMsg{tx: btx, peer: b})
	h.requestedTxFrom(nil, *btx.Hash())
}

// TestPeerTxAnnouncementsCapped ensures no more transactions announced by a
// peer are tracked than BCHN tracks, twice the most an inv message holds.
func TestPeerTxAnnouncementsCapped(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	const limit = 2 * wire.MaxInvPerMsg

	// The first half are requested from b, so a only announced them.
	hashes := make([]chainhash.Hash, limit+1)
	for i := range hashes {
		hashes[i] = chainhash.Hash{byte(i), byte(i >> 8), byte(i >> 16), 1}
	}
	half := hashes[:wire.MaxInvPerMsg]
	h.inv(b, wire.InvTypeTx, half...)
	h.inv(a, wire.InvTypeTx, half...)
	h.inv(a, wire.InvTypeTx, hashes[wire.MaxInvPerMsg:limit]...)
	h.inv(a, wire.InvTypeTx, hashes[limit])

	if n := len(h.sm.peerStates[a].requestedTxns); n != limit-wire.MaxInvPerMsg {
		t.Fatalf("%d transactions requested from the peer, want %d", n,
			limit-wire.MaxInvPerMsg)
	}
	if _, ok := h.sm.peerStates[a].requestedTxns[hashes[limit]]; ok {
		t.Fatal("transaction over the cap was requested")
	}
}

// TestTxAnnouncementsLeaveUtxoCache ensures announcements of unknown
// transactions do not fill the UTXO cache with misses.
func TestTxAnnouncementsLeaveUtxoCache(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	hashes := make([]chainhash.Hash, 100)
	for i := range hashes {
		hashes[i] = chainhash.Hash{byte(i), 2}
	}

	before := h.sm.chain.CachedStateSize()
	h.inv(a, wire.InvTypeTx, hashes...)
	if after := h.sm.chain.CachedStateSize(); after != before {
		t.Fatalf("UTXO cache grew from %d to %d bytes", before, after)
	}
}

// TestTxAnnouncementsReadUtxoSetForRequests ensures the UTXO set is read for
// an announced transaction only when it is about to be requested: not when it
// is requested from another peer already, was rejected, or is announced by a
// peer that has as many transactions tracked as it may.
func TestTxAnnouncementsReadUtxoSetForRequests(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b, c := h.peer(1), h.peer(2), h.peer(3)
	requested, rejected, overCap := chainhash.Hash{0xaa}, chainhash.Hash{0xbb},
		chainhash.Hash{0xcc}
	h.sm.rejectedTxns[rejected] = struct{}{}
	for i := range maxPeerTxAnnouncements {
		hash := chainhash.Hash{byte(i), byte(i >> 8), byte(i >> 16), 3}
		h.sm.peerStates[c].requestedTxns[hash] = struct{}{}
	}

	steps := []struct {
		peer  *peerpkg.Peer
		hash  chainhash.Hash
		reads int64
	}{
		{a, requested, 2}, // the first two outputs
		{b, requested, 0},
		{a, rejected, 0},
		{c, overCap, 0},
	}
	for i, step := range steps {
		before := h.db.views.Load()
		h.inv(step.peer, wire.InvTypeTx, step.hash)
		if n := h.db.views.Load() - before; n != step.reads {
			t.Fatalf("step %d: %d reads of the UTXO set, want %d", i, n,
				step.reads)
		}
	}
	h.requestedTxFrom(a, requested)
}

// TestConfirmedTxNotRequested ensures an announced transaction whose first
// output is in the UTXO set is not requested, whether or not the output is in
// the UTXO cache.
func TestConfirmedTxNotRequested(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	coinbase := h.tip.Transactions[0].TxHash()

	h.inv(a, wire.InvTypeTx, coinbase)
	err := h.sm.chain.FlushCachedState(blockchain.FlushRequired)
	if err != nil {
		t.Fatal(err)
	}
	h.inv(a, wire.InvTypeTx, coinbase)
	if _, ok := h.sm.requestedTxns[coinbase]; ok {
		t.Fatal("confirmed transaction was requested")
	}
}

// TestBlockNotFoundRequestsNextAnnouncer ensures a block a peer reports it
// does not have is requested at once from the next peer that announced it,
// and not again from the peer asked for announcing it again.
func TestBlockNotFoundRequestsNextAnnouncer(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b, c := h.peer(1), h.peer(2), h.peer(3)
	block := chainhash.Hash{0xbb}

	h.inv(a, wire.InvTypeBlock, block)
	h.waitGetData(a, wire.InvTypeCmpctBlock, block)
	h.inv(a, wire.InvTypeBlock, block)
	h.inv(b, wire.InvTypeBlock, block)
	h.inv(c, wire.InvTypeBlock, block)

	for _, next := range []*peerpkg.Peer{b, c} {
		h.notFound(h.sm.requestedBlocks[block], wire.InvTypeCmpctBlock, block)
		h.waitGetData(next, wire.InvTypeCmpctBlock, block)
		if h.sm.requestedBlocks[block] != next {
			t.Fatal("block is not requested from the next announcer")
		}
	}
}

// TestRetrySkipsDisconnectedPeer ensures a transaction or block is not
// requested from a peer that announced it and has disconnected, though the
// sync manager has not heard it is done yet.
func TestRetrySkipsDisconnectedPeer(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b, c := h.peer(1), h.peer(2), h.peer(3)
	tx, block := chainhash.Hash{0xaa}, chainhash.Hash{0xbb}
	for _, p := range []*peerpkg.Peer{a, b, c} {
		h.inv(p, wire.InvTypeTx, tx)
		h.inv(p, wire.InvTypeBlock, block)
	}
	b.Disconnect()
	if !isDisconnected(b) {
		t.Fatal("peer did not disconnect")
	}

	h.notFound(a, wire.InvTypeTx, tx)
	h.notFound(a, wire.InvTypeCmpctBlock, block)
	h.requestedTxFrom(c, tx)
	if h.sm.requestedBlocks[block] != c {
		t.Fatal("block is not requested from the connected announcer")
	}
}

// TestBlockAnnouncersCapped ensures no more peers are kept to request a block
// from than maxBlockAnnouncers, besides the one it is requested from, and
// that peers that are gone make room for others.
func TestBlockAnnouncersCapped(t *testing.T) {
	h := newRequestTestHarness(t)
	block := chainhash.Hash{0xbb}
	h.inv(h.peer(1), wire.InvTypeBlock, block)

	var announcers []*peerpkg.Peer
	for i := range maxBlockAnnouncers + 1 {
		p := h.peer(byte(10 + i))
		h.inv(p, wire.InvTypeBlock, block)
		announcers = append(announcers, p)
	}
	if n := len(h.sm.blockAnnouncers[block]); n != maxBlockAnnouncers {
		t.Fatalf("%d announcers kept, want %d", n, maxBlockAnnouncers)
	}

	h.sm.handleDonePeerMsg(announcers[0])
	h.inv(announcers[maxBlockAnnouncers], wire.InvTypeBlock, block)
	got := h.sm.blockAnnouncers[block]
	if len(got) != maxBlockAnnouncers ||
		got[len(got)-1] != announcers[maxBlockAnnouncers] {

		t.Fatal("a peer that is gone did not make room for another")
	}
}

// TestStartSyncEndsBlockDownloads ensures the blocks a new sync forgets are no
// longer waited for, so their peers are not disconnected for them.
func TestStartSyncEndsBlockDownloads(t *testing.T) {
	h := newRequestTestHarness(t)
	a, syncPeer := h.peer(1), h.peer(2)
	h.inv(a, wire.InvTypeBlock, chainhash.Hash{0xbb})
	h.sm.requestBlock(a, h.sm.peerStates[a], &chainhash.Hash{0xcc}, true)

	h.sm.peerStates[syncPeer].syncCandidate = true
	syncPeer.UpdateLastBlockHeight(10)
	h.sm.startSync()
	if h.sm.syncPeer != syncPeer {
		t.Fatal("sync did not start")
	}
	if n := h.sm.peerStates[a].validBlocksInFlight; n != 0 {
		t.Fatalf("%d validated blocks in flight after the sync started", n)
	}
	h.advance(2 * h.params.TargetTimePerBlock)
	if isDisconnected(a) {
		t.Fatal("peer disconnected for a block the sync forgot")
	}
}

// TestTxRequestExpires ensures a transaction a peer does not send within a
// minute of its request is requested from the next peer that announced it,
// and forgotten when no peer is left to ask.
func TestTxRequestExpires(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	hash := chainhash.Hash{0xaa}

	h.inv(a, wire.InvTypeTx, hash)
	h.inv(b, wire.InvTypeTx, hash)
	h.advance(txRequestTimeout - time.Second)
	h.requestedTxFrom(a, hash)

	h.advance(time.Second)
	h.waitGetData(b, wire.InvTypeTx, hash)
	h.requestedTxFrom(b, hash)

	h.advance(txRequestTimeout)
	h.requestedTxFrom(nil, hash)
	if _, ok := h.sm.requestedTxns[hash]; ok {
		t.Fatal("transaction is still requested")
	}
}

// TestBlockDownloadTimeout ensures a peer that does not deliver a block it
// was asked for within a block interval is disconnected, though another peer
// is asked for a block it announced, and the block is requested from another
// peer that announced it once the peer is done.
func TestBlockDownloadTimeout(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b, d := h.peer(1), h.peer(2), h.peer(4)
	block := chainhash.Hash{0xbb}

	h.inv(a, wire.InvTypeBlock, block)
	h.inv(b, wire.InvTypeBlock, block)
	h.inv(d, wire.InvTypeBlock, chainhash.Hash{0xdd})
	h.advance(h.params.TargetTimePerBlock - time.Second)
	if isDisconnected(a) {
		t.Fatal("peer disconnected before a block interval")
	}
	h.advance(2 * time.Second)
	if !isDisconnected(a) {
		t.Fatal("peer still connected after the timeout")
	}

	h.sm.handleDonePeerMsg(a)
	h.waitGetData(b, wire.InvTypeCmpctBlock, block)
}

// TestBlockDownloadTimeoutValidatedPeers ensures each other peer downloading
// blocks whose headers were validated adds half a block interval to the
// timeout, as in BCHN.
func TestBlockDownloadTimeoutValidatedPeers(t *testing.T) {
	h := newRequestTestHarness(t)
	a, c := h.peer(1), h.peer(3)

	h.inv(a, wire.InvTypeBlock, chainhash.Hash{0xbb})
	h.advance(h.params.TargetTimePerBlock - time.Second)
	h.sm.requestBlock(c, h.sm.peerStates[c], &chainhash.Hash{0xcc}, true)
	h.advance(2 * time.Second)
	if isDisconnected(a) {
		t.Fatal("peer disconnected before the time a validated download " +
			"adds")
	}
	h.advance(h.params.TargetTimePerBlock / 2)
	if !isDisconnected(a) {
		t.Fatal("peer still connected after the timeout")
	}
	if isDisconnected(c) {
		t.Fatal("peer that started downloading later was disconnected")
	}

	// Its own validated download does not add to a peer's timeout.
	h.advance(h.params.TargetTimePerBlock / 2)
	if !isDisconnected(c) {
		t.Fatal("peer still connected a block interval after its request")
	}
}

// TestValidBlockRequestsCounted ensures a block request keeps whether the
// block's header was validated when it moves to another peer, and stops
// counting as one when it ends.
func TestValidBlockRequestsCounted(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	hash := chainhash.Hash{0xbb}

	h.sm.requestBlock(a, h.sm.peerStates[a], &hash, true)
	h.inv(b, wire.InvTypeBlock, hash)
	h.notFound(a, wire.InvTypeBlock, hash)
	if n := h.sm.peerStates[a].validBlocksInFlight; n != 0 {
		t.Fatalf("%d validated blocks in flight from the first peer", n)
	}
	if n := h.sm.peerStates[b].validBlocksInFlight; n != 1 {
		t.Fatalf("%d validated blocks in flight from the next peer, "+
			"want 1", n)
	}
	h.sm.endBlockRequest(&hash)
	if n := h.sm.peerStates[b].validBlocksInFlight; n != 0 {
		t.Fatalf("%d validated blocks in flight after the request ended", n)
	}
}

// TestBlockDownloadTimerRestarts ensures a peer that delivers a block it was
// asked for has a full timeout again for the rest.
func TestBlockDownloadTimerRestarts(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	block := mineBlock(t, h.params, h.tip, 2, 0)
	h.inv(a, wire.InvTypeBlock, chainhash.Hash{0xbb}, *block.Hash())

	h.advance(h.params.TargetTimePerBlock - time.Second)
	h.sm.handleBlockMsg(&blockMsg{block: block, peer: a})
	h.advance(h.params.TargetTimePerBlock - time.Second)
	if isDisconnected(a) {
		t.Fatal("peer disconnected though it delivered a block")
	}
	h.advance(2 * time.Second)
	if !isDisconnected(a) {
		t.Fatal("peer still connected a block interval after its last " +
			"block")
	}
}

// TestBlockFromOtherPeerEndsDownload ensures a block that arrives from another
// peer than the one it was requested from, such as a compact block relayed
// directly, is no longer waited for from the first, which may still send it.
func TestBlockFromOtherPeerEndsDownload(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	b.SetAllowDirectBlockRelay(true)
	block := mineBlock(t, h.params, h.tip, 2, 0)

	h.inv(a, wire.InvTypeBlock, *block.Hash())
	h.sm.handleBlockMsg(&blockMsg{block: block, peer: b})
	if _, ok := h.sm.requestedBlocks[*block.Hash()]; ok {
		t.Fatal("block still requested")
	}
	h.advance(2 * h.params.TargetTimePerBlock)
	if isDisconnected(a) {
		t.Fatal("peer disconnected for a block another peer delivered")
	}

	h.sm.handleBlockMsg(&blockMsg{block: block, peer: a})
	if isDisconnected(a) {
		t.Fatal("peer disconnected for sending the block it was asked for")
	}
}
