// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// gatedConn is a connection whose writes of messages with the gated command,
// blocktxns unless set, wait until the gate is opened.
type gatedConn struct {
	net.Conn
	started chan struct{}
	gate    chan struct{}
	cmd     atomic.Value
}

// Write signals started and waits for the gate before writing the header of a
// message with the gated command.
func (c *gatedConn) Write(b []byte) (int, error) {
	gated, _ := c.cmd.Load().(string)
	if gated == "" {
		gated = wire.CmdBlockTxns
	}
	cmd := []byte(gated + "\x00")
	if len(b) >= wire.MessageHeaderSize && bytes.HasPrefix(b[4:], cmd) {
		select {
		case c.started <- struct{}{}:
		default:
		}
		<-c.gate
	}
	return c.Conn.Write(b)
}

// blockTxnsHarness connects a server peer backed by a regtest chain to a
// remote peer which records the blocktxns messages it receives.
type blockTxnsHarness struct {
	chain    *blockchain.BlockChain
	sp       *serverPeer
	remote   *peer.Peer
	conn     *gatedConn
	handled  atomic.Int32
	replies  chan *wire.MsgBlockTxns
	blocks   chan *wire.MsgBlock
	cmpcts   chan *wire.MsgCmpctBlock
	notFound chan *wire.MsgNotFound
	getData  chan struct{}
	tipHash  *chainhash.Hash
	openGate func()
}

// newBlockTxnsHarness returns a connected harness whose chain has one block
// on top of the genesis block.  Writes of blocktxns messages wait for the
// gate, which starts open unless gated is set.
func newBlockTxnsHarness(t *testing.T, gated bool) *blockTxnsHarness {
	t.Helper()
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	chain, db := newTestChainDB(t)
	block := solvedBlock(t, chain, 0, nil)
	if _, _, err := chain.ProcessBlock(block, blockchain.BFNone); err != nil {
		t.Fatal(err)
	}

	h := &blockTxnsHarness{
		chain:    chain,
		replies:  make(chan *wire.MsgBlockTxns, 16),
		blocks:   make(chan *wire.MsgBlock, 16),
		cmpcts:   make(chan *wire.MsgCmpctBlock, 16),
		notFound: make(chan *wire.MsgNotFound, 16),
		getData:  make(chan struct{}, 1),
		tipHash:  block.Hash(),
	}
	h.sp = &serverPeer{
		server: &server{
			chain:    chain,
			db:       db,
			banPeers: make(chan *serverPeer, 1),
			quit:     make(chan struct{}),
		},
		quit: make(chan struct{}),
	}
	// The tip is the most recent block, as when it was just announced.
	h.sp.server.setRecentBlock(block.MsgBlock())
	h.sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams:            &chaincfg.RegressionNetParams,
		TstAllowSelfConnection: true,
		Listeners: peer.MessageListeners{
			OnGetBlockTxns: func(p *peer.Peer, msg *wire.MsgGetBlockTxns) {
				h.handled.Add(1)
				h.sp.OnGetBlockTxns(p, msg)
			},
			OnGetData: func(p *peer.Peer, msg *wire.MsgGetData) {
				h.sp.OnGetData(p, msg)
				h.getData <- struct{}{}
			},
		},
	})

	verack := make(chan struct{}, 1)
	var err error
	h.remote, err = peer.NewOutboundPeer(&peer.Config{
		ChainParams:            &chaincfg.RegressionNetParams,
		TstAllowSelfConnection: true,
		Listeners: peer.MessageListeners{
			OnVerAck: func(*peer.Peer, *wire.MsgVerAck) {
				verack <- struct{}{}
			},
			OnBlockTxns: func(_ *peer.Peer, msg *wire.MsgBlockTxns) {
				h.replies <- msg
			},
			OnBlock: func(_ *peer.Peer, msg *wire.MsgBlock, _ []byte) {
				h.blocks <- msg
			},
			OnCmpctBlock: func(_ *peer.Peer, msg *wire.MsgCmpctBlock) {
				h.cmpcts <- msg
			},
			OnNotFound: func(_ *peer.Peer, msg *wire.MsgNotFound) {
				h.notFound <- msg
			},
		},
	}, "127.0.0.1:18444")
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	outConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	inConn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	h.conn = &gatedConn{
		Conn:    inConn,
		started: make(chan struct{}, 1),
		gate:    make(chan struct{}),
	}
	var opened atomic.Bool
	h.openGate = func() {
		if opened.CompareAndSwap(false, true) {
			close(h.conn.gate)
		}
	}
	if !gated {
		h.openGate()
	}
	h.sp.AssociateConnection(h.conn)
	h.remote.AssociateConnection(outConn)
	t.Cleanup(func() {
		h.openGate()
		h.remote.Disconnect()
		h.sp.Disconnect()
	})

	select {
	case <-verack:
	case <-time.After(5 * time.Second):
		t.Fatal("peers did not complete the handshake")
	}
	return h
}

// request sends a getblocktxns message for the coinbase of the tip block.
func (h *blockTxnsHarness) request() {
	h.remote.QueueMessage(wire.NewMsgGetBlockTxns(*h.tipHash, []uint32{0}),
		nil)
}

// reply waits for the next blocktxns message sent to the remote peer.
func (h *blockTxnsHarness) reply(t *testing.T) {
	t.Helper()
	select {
	case msg := <-h.replies:
		if msg.BlockHash != *h.tipHash || len(msg.Txs) != 1 {
			t.Fatalf("got blocktxns for %v with %d txs, want %v with 1",
				msg.BlockHash, len(msg.Txs), h.tipHash)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no reply to getblocktxns")
	}
}

// TestGetBlockTxnsNoBanScore ensures valid getblocktxns requests are answered
// without raising the ban score, so a peer reconstructing several compact
// blocks in a short time is not banned.
func TestGetBlockTxnsNoBanScore(t *testing.T) {
	h := newBlockTxnsHarness(t, false)
	for range 6 {
		h.request()
		h.reply(t)
	}
	if score := h.sp.banScore.Int(); score != 0 {
		t.Fatalf("ban score is %d, want 0", score)
	}
	if !h.sp.Connected() {
		t.Fatal("peer was disconnected")
	}
}

// TestGetBlockTxnsOneReplyInFlight ensures a getblocktxns request is not
// handled until the reply to the previous one has been written.
func TestGetBlockTxnsOneReplyInFlight(t *testing.T) {
	h := newBlockTxnsHarness(t, true)
	h.request()
	h.request()

	select {
	case <-h.conn.started:
	case <-time.After(5 * time.Second):
		t.Fatal("first reply was not written")
	}
	time.Sleep(200 * time.Millisecond)
	if n := h.handled.Load(); n != 1 {
		t.Fatalf("%d requests handled while the first reply was "+
			"being written, want 1", n)
	}

	h.openGate()
	h.reply(t)
	h.reply(t)
	if n := h.handled.Load(); n != 2 {
		t.Fatalf("%d requests handled, want 2", n)
	}
}

// TestGetBlockTxnsSentInFull ensures a getblocktxns for a block other than the
// most recent one is answered with the full block, as BCHN does, so it costs
// the peer the whole block rather than a read and decode on this node.
func TestGetBlockTxnsSentInFull(t *testing.T) {
	h := newBlockTxnsHarness(t, false)
	h.sp.server.setRecentBlock(solvedBlock(t, h.chain, -1, nil).MsgBlock())
	h.request()
	select {
	case msg := <-h.blocks:
		if msg.BlockHash() != *h.tipHash {
			t.Fatalf("got block %v, want %v", msg.BlockHash(), h.tipHash)
		}
	case msg := <-h.replies:
		t.Fatalf("got blocktxns for %v, want the full block", msg.BlockHash)
	case <-time.After(5 * time.Second):
		t.Fatal("no reply to getblocktxns")
	}
}

// TestGetBlockTxnsNoIndexes ensures a peer asking for no transactions is
// disconnected, as BCHN does.
func TestGetBlockTxnsNoIndexes(t *testing.T) {
	h := newBlockTxnsHarness(t, false)
	h.remote.QueueMessage(wire.NewMsgGetBlockTxns(*h.tipHash, nil), nil)
	disconnected := make(chan struct{})
	go func() {
		h.sp.WaitForDisconnect()
		close(disconnected)
	}()
	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("peer asking for no transactions was not disconnected")
	}
}

// TestGetBlockTxnsRecentBlockNotStored ensures the most recent block, which is
// relayed early as a compact block before it is stored, answers getblocktxns.
func TestGetBlockTxnsRecentBlockNotStored(t *testing.T) {
	h := newBlockTxnsHarness(t, false)
	block := solvedBlock(t, h.chain, -1, nil)
	h.sp.server.setRecentBlock(block.MsgBlock())
	h.remote.QueueMessage(wire.NewMsgGetBlockTxns(*block.Hash(), []uint32{0}),
		nil)
	select {
	case msg := <-h.replies:
		if msg.BlockHash != *block.Hash() || len(msg.Txs) != 1 {
			t.Fatalf("got blocktxns for %v with %d txs, want %v with 1",
				msg.BlockHash, len(msg.Txs), block.Hash())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no reply to getblocktxns")
	}
}

// TestRelayedBlockIsRecent ensures announcing a block makes it the block that
// answers getblocktxns.
func TestRelayedBlockIsRecent(t *testing.T) {
	s := &server{}
	block := &wire.MsgBlock{Header: wire.BlockHeader{Nonce: 7}}
	hash := block.BlockHash()
	s.handleRelayInvMsg(&peerState{}, relayMsg{
		invVect: wire.NewInvVect(wire.InvTypeBlock, &hash),
		data:    block,
	})
	if s.recentBlockByHash(&hash) != block {
		t.Fatal("the announced block is not the recent block")
	}
}

// countingDB counts the read transactions of a database.
type countingDB struct {
	database.DB
	views atomic.Int32
}

// View counts the transaction and runs it on the database.
func (db *countingDB) View(fn func(tx database.Tx) error) error {
	db.views.Add(1)
	return db.DB.View(fn)
}

// TestGetDataStopsAfterDisconnect ensures a getdata stops fetching blocks once
// the peer has disconnected, rather than reading and decoding every block it
// asked for.
func TestGetDataStopsAfterDisconnect(t *testing.T) {
	h := newBlockTxnsHarness(t, true)
	h.conn.cmd.Store(wire.CmdBlock)
	db := &countingDB{DB: h.sp.server.db}
	h.sp.server.db = db

	const numBlocks = 50
	getData := wire.NewMsgGetData()
	for range numBlocks {
		if err := getData.AddInvVect(wire.NewInvVect(wire.InvTypeBlock,
			h.tipHash)); err != nil {
			t.Fatal(err)
		}
	}
	h.remote.QueueMessage(getData, nil)
	select {
	case <-h.conn.started:
	case <-time.After(5 * time.Second):
		t.Fatal("no block was sent")
	}
	h.sp.Disconnect()
	select {
	case <-h.getData:
	case <-time.After(5 * time.Second):
		t.Fatal("getdata was not handled")
	}
	h.openGate()
	if n := db.views.Load(); n > 4 {
		t.Fatalf("%d of %d blocks read for a disconnected peer, want "+
			"at most 4", n, numBlocks)
	}
}

// getDataFor sends a getdata for the block from the remote peer.
func (h *blockTxnsHarness) getDataFor(t *testing.T, invType wire.InvType,
	hash *chainhash.Hash) {

	t.Helper()
	getData := wire.NewMsgGetData()
	if err := getData.AddInvVect(wire.NewInvVect(invType, hash)); err != nil {
		t.Fatal(err)
	}
	h.remote.QueueMessage(getData, nil)
}

// expectNothingSent ensures no block, compact block or notfound arrives at
// the remote peer for a while.
func (h *blockTxnsHarness) expectNothingSent(t *testing.T) {
	t.Helper()
	select {
	case msg := <-h.blocks:
		t.Fatalf("block %v sent", msg.BlockHash())
	case msg := <-h.cmpcts:
		t.Fatalf("compact block %v sent", msg.BlockHash())
	case msg := <-h.notFound:
		t.Fatalf("notfound for %v sent", msg.InvList[0])
	case <-time.After(200 * time.Millisecond):
	}
}

// expectReply waits for the reply to a getdata of the given type for the
// block: the block in that form, or notfound if notFound is set.
func (h *blockTxnsHarness) expectReply(t *testing.T, invType wire.InvType,
	hash *chainhash.Hash, notFound bool) {

	t.Helper()
	var got string
	select {
	case msg := <-h.blocks:
		if msg.BlockHash() == *hash && !notFound &&
			invType == wire.InvTypeBlock {

			return
		}
		got = "block " + msg.BlockHash().String()
	case msg := <-h.cmpcts:
		if msg.BlockHash() == *hash && !notFound &&
			invType == wire.InvTypeCmpctBlock {

			return
		}
		got = "compact block " + msg.BlockHash().String()
	case msg := <-h.notFound:
		if len(msg.InvList) == 1 && msg.InvList[0].Hash == *hash &&
			notFound {

			return
		}
		got = "notfound for " + msg.InvList[0].Hash.String()
	case <-time.After(5 * time.Second):
		got = "nothing"
	}
	t.Fatalf("got %s for %v of %v, want notfound %v", got, invType, hash,
		notFound)
}

// TestGetDataWaitsForRelayedBlock ensures a getdata for a block relayed as a
// compact block before it was processed is answered once it has been, as BCHN
// finishes validating a block before it answers a getdata for it: with the
// block, or with notfound if it turned out invalid.  It was answered with
// notfound at once, which BCHN ignores for blocks: it waited for the block
// until its block download timeout and then disconnected this node.
func TestGetDataWaitsForRelayedBlock(t *testing.T) {
	tests := []struct {
		name    string
		invType wire.InvType
		invalid bool
	}{
		{"block", wire.InvTypeBlock, false},
		{"compact block", wire.InvTypeCmpctBlock, false},
		{"invalid block", wire.InvTypeBlock, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newBlockTxnsHarness(t, false)
			// A block paying one satoshi too much is stored, and fails
			// once it is connected.
			var delta int64
			if test.invalid {
				delta = 1
			}
			block := solvedBlock(t, h.chain, delta, nil)
			processed := h.sp.server.blockProcessing(*block.Hash())
			h.sp.server.setRecentBlock(block.MsgBlock())

			h.getDataFor(t, test.invType, block.Hash())
			h.expectNothingSent(t)
			_, _, err := h.chain.ProcessBlock(block, blockchain.BFNone)
			if (err != nil) != test.invalid {
				t.Fatalf("processing the block: %v", err)
			}
			processed()
			h.expectReply(t, test.invType, block.Hash(), test.invalid)
		})
	}
}

// TestGetDataWaitEndsOnQuit ensures a getdata waiting for a block relayed
// before it was processed stops waiting when the peer or the server quits.
func TestGetDataWaitEndsOnQuit(t *testing.T) {
	for _, serverQuits := range []bool{false, true} {
		h := newBlockTxnsHarness(t, false)
		block := solvedBlock(t, h.chain, 0, nil)
		h.sp.server.blockProcessing(*block.Hash())
		h.getDataFor(t, wire.InvTypeBlock, block.Hash())
		select {
		case <-h.getData:
			t.Fatal("getdata was handled before the block was processed")
		case <-time.After(100 * time.Millisecond):
		}
		if serverQuits {
			close(h.sp.server.quit)
		} else {
			close(h.sp.quit)
		}
		select {
		case <-h.getData:
		case <-time.After(5 * time.Second):
			t.Fatalf("getdata still waits after the quit (server %v)",
				serverQuits)
		}
	}
}

// TestGetDataInvalidBlockNotSent ensures a block known to be invalid is not
// sent for a getdata, as BCHN sends only blocks it validated.  A block relayed
// early as a compact block is stored before its inputs are checked, and BCHN
// discourages a peer that sends it an invalid block in full.
func TestGetDataInvalidBlockNotSent(t *testing.T) {
	h := newBlockTxnsHarness(t, false)
	block := solvedBlock(t, h.chain, 1, nil)
	if _, _, err := h.chain.ProcessBlock(block, blockchain.BFNone); err == nil {
		t.Fatal("the block paying too much was accepted")
	}
	if known, _, invalid := h.chain.BlockValidity(block.Hash()); !known ||
		!invalid {

		t.Fatalf("block known %v, invalid %v, want both", known, invalid)
	}
	for _, invType := range []wire.InvType{wire.InvTypeBlock,
		wire.InvTypeCmpctBlock} {

		h.getDataFor(t, invType, block.Hash())
		h.expectReply(t, invType, block.Hash(), true)
	}
}
