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
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// gatedConn is a connection whose writes of blocktxns messages wait until
// the gate is opened.
type gatedConn struct {
	net.Conn
	started chan struct{}
	gate    chan struct{}
}

// Write signals started and waits for the gate before writing a blocktxns
// message header.
func (c *gatedConn) Write(b []byte) (int, error) {
	cmd := []byte(wire.CmdBlockTxns + "\x00")
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
		chain:   chain,
		replies: make(chan *wire.MsgBlockTxns, 16),
		blocks:  make(chan *wire.MsgBlock, 16),
		tipHash: block.Hash(),
	}
	h.sp = &serverPeer{
		server: &server{
			chain:    chain,
			db:       db,
			banPeers: make(chan *serverPeer, 1),
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
