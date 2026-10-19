// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/netsync"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// nopPeerNotifier ignores everything the sync manager reports.
type nopPeerNotifier struct{}

func (nopPeerNotifier) AnnounceNewTransactions([]*mempool.TxDesc)            {}
func (nopPeerNotifier) UpdatePeerHeights(*chainhash.Hash, int32, *peer.Peer) {}
func (nopPeerNotifier) RelayInventory(*wire.InvVect, interface{})            {}
func (nopPeerNotifier) TransactionConfirmed(*bchutil.Tx)                     {}

// cmpctHarness is a server peer backed by a current regtest chain, an empty
// memory pool and a running sync manager, whose remote side the test drives.
type cmpctHarness struct {
	s      *server
	sp     *serverPeer
	remote net.Conn
	// getTxns receives each getblocktxns the server peer sends.
	getTxns chan *wire.MsgGetBlockTxns
	// getData receives the inventory of each getdata the server peer
	// sends.
	getData chan *wire.InvVect
}

// newCmpctHarness returns a harness whose peer has completed the handshake
// and is known to the sync manager.
func newCmpctHarness(t *testing.T) *cmpctHarness {
	t.Helper()
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	oldSyncLevel, oldTxmpLevel := syncLog.Level(), txmpLog.Level()
	syncLog.SetLevel(bchlog.LevelOff)
	txmpLog.SetLevel(bchlog.LevelOff)
	t.Cleanup(func() {
		syncLog.SetLevel(oldSyncLevel)
		txmpLog.SetLevel(oldTxmpLevel)
	})
	params := &chaincfg.RegressionNetParams
	chain, _ := newTestChainDB(t)
	// A recent tip makes the chain current, so blocks are requested as
	// compact blocks.
	tip := solvedBlock(t, chain, 0, nil)
	if _, _, err := chain.ProcessBlock(tip, blockchain.BFNone); err != nil {
		t.Fatal(err)
	}
	txPool := mempool.New(&mempool.Config{ChainParams: params})
	// The sync manager gets a copy of the regtest parameters, so that,
	// as on the other networks, it refuses blocks it did not request.
	syncParams := *params
	sm, err := netsync.New(&netsync.Config{
		PeerNotifier: nopPeerNotifier{},
		Chain:        chain,
		TxMemPool:    txPool,
		ChainParams:  &syncParams,
		MaxPeers:     8,
	})
	if err != nil {
		t.Fatal(err)
	}
	sm.Start()
	t.Cleanup(func() { _ = sm.Stop() })

	s := &server{
		chainParams:     params,
		chain:           chain,
		txMemPool:       txPool,
		syncManager:     sm,
		banPeers:        make(chan *serverPeer, 1),
		relayCmpctBlock: make(chan *wire.MsgCmpctBlock, 16),
	}
	sp := newServerPeer(s, false)
	// Only the listeners under test, as the server sets them up.
	listeners := newPeerConfig(sp).Listeners
	sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams:            params,
		ProtocolVersion:        peer.MaxProtocolVersion,
		TstAllowSelfConnection: true,
		Listeners: peer.MessageListeners{
			OnBlock:     listeners.OnBlock,
			OnBlockTxns: listeners.OnBlockTxns,
			OnInv:       listeners.OnInv,
			OnNotFound:  listeners.OnNotFound,
			OnRead:      listeners.OnRead,
		},
	})
	local, remote := net.Pipe()
	sp.AssociateConnection(&addrConn{Conn: local,
		remote: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 18444}})
	t.Cleanup(func() {
		sp.Disconnect()
		remote.Close()
		sp.WaitForDisconnect()
	})

	pver := wire.ProtocolVersion
	me := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.1"), 18444, 0)
	you := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.2"), 18444, 0)
	if err := wire.WriteMessage(remote, wire.NewMsgVersion(me, you, 1, 0),
		pver, params.Net); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, _, err := wire.ReadMessage(remote, pver, params.Net); err != nil {
			t.Fatal(err)
		}
	}
	if err := wire.WriteMessage(remote, wire.NewMsgVerAck(), pver,
		params.Net); err != nil {
		t.Fatal(err)
	}

	h := &cmpctHarness{s: s, sp: sp, remote: remote,
		getTxns: make(chan *wire.MsgGetBlockTxns, 16),
		getData: make(chan *wire.InvVect, 16)}
	go func() {
		for {
			msg, _, err := wire.ReadMessage(remote, pver, params.Net)
			if err != nil {
				return
			}
			switch m := msg.(type) {
			case *wire.MsgGetBlockTxns:
				h.getTxns <- m
			case *wire.MsgGetData:
				for _, iv := range m.InvList {
					h.getData <- iv
				}
			}
		}
	}()

	done := make(chan struct{}, 1)
	sm.NewPeer(sp.Peer, done)
	<-done
	return h
}

// send writes msg to the server peer from the remote side.
func (h *cmpctHarness) send(t *testing.T, msg wire.Message) {
	t.Helper()
	err := wire.WriteMessage(h.remote, msg, wire.ProtocolVersion,
		h.s.chainParams.Net)
	if err != nil {
		t.Fatal(err)
	}
}

// request announces the block from the remote side and waits for the
// compact block to be requested, so the block is requested from the peer.
func (h *cmpctHarness) request(t *testing.T, hash chainhash.Hash) {
	t.Helper()
	inv := wire.NewMsgInv()
	if err := inv.AddInvVect(wire.NewInvVect(wire.InvTypeBlock,
		&hash)); err != nil {
		t.Fatal(err)
	}
	h.send(t, inv)
	select {
	case iv := <-h.getData:
		if iv.Type != wire.InvTypeCmpctBlock || iv.Hash != hash {
			t.Fatalf("getdata for %v, want compact block %v", iv, hash)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("compact block %v was not requested", hash)
	}
}

// process reconstructs msg in the background and returns a channel that is
// closed when that finishes.
func (h *cmpctHarness) process(msg *wire.MsgCmpctBlock) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		h.sp.processCompactBlock(msg)
		close(done)
	}()
	return done
}

// expectGetTxns waits for a getblocktxns for the block.
func (h *cmpctHarness) expectGetTxns(t *testing.T, hash chainhash.Hash) {
	t.Helper()
	select {
	case msg := <-h.getTxns:
		if msg.BlockHash != hash {
			t.Fatalf("getblocktxns for %v, want %v", msg.BlockHash, hash)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no getblocktxns for %v", hash)
	}
}

// expectRelayed waits for the compact blocks to be relayed, in any order.
func (h *cmpctHarness) expectRelayed(t *testing.T, hashes ...chainhash.Hash) {
	t.Helper()
	want := make(map[chainhash.Hash]bool)
	for _, hash := range hashes {
		want[hash] = true
	}
	for len(want) > 0 {
		select {
		case msg := <-h.s.relayCmpctBlock:
			hash := msg.BlockHash()
			if !want[hash] {
				t.Fatalf("unexpected relay of %v", hash)
			}
			delete(want, hash)
		case <-time.After(5 * time.Second):
			t.Fatalf("compact blocks %v not relayed", want)
		}
	}
}

// expectHaveBlock waits for the chain to have the block.
func (h *cmpctHarness) expectHaveBlock(t *testing.T, hash *chainhash.Hash) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		have, err := h.s.chain.HaveBlock(hash)
		if err != nil {
			t.Fatal(err)
		}
		if have {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("block %v was not processed", hash)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitDone waits for a reconstruction started with process to finish.
func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("compact block reconstruction did not finish")
	}
}

// shortIDsOnly returns a compact block for block that sends every
// transaction, including the coinbase, as a short ID.
func shortIDsOnly(t *testing.T, block *bchutil.Block) *wire.MsgCmpctBlock {
	t.Helper()
	known := make(map[chainhash.Hash]bool)
	for _, tx := range block.Transactions() {
		known[*tx.Hash()] = true
	}
	msg, err := wire.NewMsgCmpctBlockFromBlock(block.MsgBlock(), known)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// blockTxnsFor returns a blocktxns with every transaction of block.
func blockTxnsFor(block *bchutil.Block) *wire.MsgBlockTxns {
	return wire.NewMsgBlockTxns(*block.Hash(),
		block.MsgBlock().Transactions)
}

// TestCompactBlocksWaitingTogether ensures two compact blocks from one peer
// that wait for missing transactions at the same time each get the reply
// for their own block, in either order of the replies.
func TestCompactBlocksWaitingTogether(t *testing.T) {
	t.Run("in order", func(t *testing.T) {
		testCompactBlocksWaitingTogether(t, false)
	})
	t.Run("reversed", func(t *testing.T) {
		testCompactBlocksWaitingTogether(t, true)
	})
}

func testCompactBlocksWaitingTogether(t *testing.T, reverse bool) {
	h := newCmpctHarness(t)
	first := solvedBlock(t, h.s.chain, 0, nil)
	second := solvedBlock(t, h.s.chain, -1, nil)
	h.request(t, *first.Hash())
	h.request(t, *second.Hash())

	firstDone := h.process(shortIDsOnly(t, first))
	h.expectGetTxns(t, *first.Hash())
	secondDone := h.process(shortIDsOnly(t, second))
	h.expectGetTxns(t, *second.Hash())

	replies := []*bchutil.Block{first, second}
	if reverse {
		replies = []*bchutil.Block{second, first}
	}
	for _, block := range replies {
		h.send(t, blockTxnsFor(block))
	}
	h.expectRelayed(t, *first.Hash(), *second.Hash())
	waitDone(t, firstDone)
	waitDone(t, secondDone)
	h.expectHaveBlock(t, first.Hash())
	h.expectHaveBlock(t, second.Hash())
}

// TestCompactBlockWaitingTwice ensures a compact block the peer sends again
// while it waits for transactions is ignored, and that a blocktxns for a
// block nobody waits for is dropped.
func TestCompactBlockWaitingTwice(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	other := solvedBlock(t, h.s.chain, -1, nil)
	msg := shortIDsOnly(t, block)
	h.request(t, *block.Hash())

	done := h.process(msg)
	h.expectGetTxns(t, *block.Hash())
	waitDone(t, h.process(msg))
	select {
	case <-h.getTxns:
		t.Fatal("transactions requested twice")
	case <-time.After(100 * time.Millisecond):
	}

	h.send(t, blockTxnsFor(other))
	h.send(t, blockTxnsFor(block))
	h.expectRelayed(t, *block.Hash())
	waitDone(t, done)
	h.expectHaveBlock(t, block.Hash())
}

// TestCompactBlockWaitEndsOnDisconnect ensures a compact block stops waiting
// for transactions when the peer disconnects.
func TestCompactBlockWaitEndsOnDisconnect(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)

	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	close(h.sp.quit)
	waitDone(t, done)
	if h.sp.deliverCmpctReply(*block.Hash(), blockTxnsFor(block)) {
		t.Fatal("compact block still waiting")
	}
}

// TestRemoveCmpctWaiter ensures ending a wait that already ended leaves a
// newer wait for the same block alone.
func TestRemoveCmpctWaiter(t *testing.T) {
	sp := newServerPeer(&server{}, false)
	hash := chainhash.Hash{0x01}
	reply := &wire.MsgBlockTxns{BlockHash: hash}

	first, ok := sp.addCmpctWaiter(hash)
	if !ok {
		t.Fatal("first wait not added")
	}
	if _, ok := sp.addCmpctWaiter(hash); ok {
		t.Fatal("second wait added for the same block")
	}
	if !sp.deliverCmpctReply(hash, reply) {
		t.Fatal("reply not delivered")
	}
	second, ok := sp.addCmpctWaiter(hash)
	if !ok {
		t.Fatal("new wait not added")
	}
	sp.removeCmpctWaiter(hash, first)
	if !sp.deliverCmpctReply(hash, reply) {
		t.Fatal("removing the ended wait removed the new one")
	}
	if got := <-second; got != wire.Message(reply) {
		t.Fatal("new wait got the wrong reply")
	}
}

// TestCompactBlockFullBlockReply ensures a full block sent in reply to
// getblocktxns ends the wait of the compact block and is processed as the
// block requested from the peer.
func TestCompactBlockFullBlockReply(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())

	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	h.send(t, block.MsgBlock())
	waitDone(t, done)
	h.expectHaveBlock(t, block.Hash())

	// The compact block itself is not relayed, as it was not
	// reconstructed.
	select {
	case msg := <-h.s.relayCmpctBlock:
		t.Fatalf("compact block %v relayed", msg.BlockHash())
	default:
	}
}

// TestBlockNotFoundForgotten ensures a block the peer reports as not found
// is no longer requested from it, so a later announcement requests it
// again.
func TestBlockNotFoundForgotten(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())

	notFound := wire.NewMsgNotFound()
	if err := notFound.AddInvVect(wire.NewInvVect(wire.InvTypeCmpctBlock,
		block.Hash())); err != nil {
		t.Fatal(err)
	}
	h.send(t, notFound)
	h.request(t, *block.Hash())
}
