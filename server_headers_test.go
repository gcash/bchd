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
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// wantHeaders has the remote side ask the server peer to announce blocks with
// headers, and waits for that to take effect.
func (h *cmpctHarness) wantHeaders(t *testing.T) {
	t.Helper()
	h.send(t, wire.NewMsgSendHeaders())
	deadline := time.Now().Add(5 * time.Second)
	for !h.sp.WantsHeaders() {
		if time.Now().After(deadline) {
			t.Fatal("the peer did not ask for headers")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// relay announces the block to the harness's peer as the server does.
func (h *cmpctHarness) relay(block *bchutil.Block) {
	state := newTestPeerState()
	state.inboundPeers[h.sp.ID()] = h.sp
	h.s.handleRelayInvMsg(state, relayMsg{
		invVect: wire.NewInvVect(wire.InvTypeBlock, block.Hash()),
		data:    block.MsgBlock(),
	})
}

// expectAnnouncement waits for the block to be announced to the remote side,
// with its header when withHeader is set and with an inv otherwise.
func (h *cmpctHarness) expectAnnouncement(t *testing.T, hash *chainhash.Hash,
	withHeader bool) {

	t.Helper()
	select {
	case msg := <-h.headers:
		if !withHeader {
			t.Fatalf("block %v announced with its header", hash)
		}
		if len(msg.Headers) != 1 || msg.Headers[0].BlockHash() != *hash {
			t.Fatalf("headers announcement for the wrong block, want %v",
				hash)
		}
	case iv := <-h.invs:
		if withHeader {
			t.Fatalf("block %v announced with an inv", hash)
		}
		if iv.Hash != *hash {
			t.Fatalf("inv for %v, want %v", iv.Hash, hash)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("block %v was not announced", hash)
	}
}

// sync waits for the server peer to answer a ping from the remote side, so it
// has handled the messages sent before.
func (h *cmpctHarness) sync(t *testing.T, nonce uint64) {
	t.Helper()
	h.send(t, wire.NewMsgPing(nonce))
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-h.pongs:
			if got == nonce {
				return
			}
		case <-deadline:
			t.Fatal("no pong")
		}
	}
}

// TestHeadersAnnouncedWhenParentKnown ensures a block is announced with its
// header to a peer that asked for headers only when the peer is known to have
// the header of the block's parent, as BCHN announces (net_processing.cpp:
// 4358-4371): the peer announced or sent the parent, or was sent its header.
// Otherwise the block is announced with an inv, as the peer could not connect
// the header.
func TestHeadersAnnouncedWhenParentKnown(t *testing.T) {
	h := newCmpctHarness(t)
	h.wantHeaders(t)
	tipHash := h.s.chain.BestSnapshot().Hash
	tip, err := h.s.chain.BlockByHash(&tipHash)
	if err != nil {
		t.Fatal(err)
	}

	first := solvedBlock(t, h.s.chain, 0, nil)
	h.relay(first)
	h.expectAnnouncement(t, first.Hash(), false)

	// The peer announces the tip with its header.
	msg := wire.NewMsgHeaders()
	if err := msg.AddBlockHeader(&tip.MsgBlock().Header); err != nil {
		t.Fatal(err)
	}
	h.send(t, msg)
	h.sync(t, 1)
	second := solvedBlock(t, h.s.chain, -1, nil)
	h.relay(second)
	h.expectAnnouncement(t, second.Hash(), true)

	// The header of the second was sent, so a block on it is announced
	// with its header.
	if _, _, err := h.s.chain.ProcessBlock(second, blockchain.BFNone); err != nil {
		t.Fatal(err)
	}
	third := solvedBlock(t, h.s.chain, 0, nil)
	h.relay(third)
	h.expectAnnouncement(t, third.Hash(), true)
}

// TestHeadersAnnouncedAfterGetHeaders ensures the headers sent in answer to a
// getheaders count as the peer's, so a block on the last is announced with its
// header, as BCHN records the best header it sent a peer.
func TestHeadersAnnouncedAfterGetHeaders(t *testing.T) {
	h := newCmpctHarness(t)
	h.wantHeaders(t)
	tipHash := h.s.chain.BestSnapshot().Hash

	getHeaders := wire.NewMsgGetHeaders()
	getHeaders.ProtocolVersion = wire.ProtocolVersion
	if err := getHeaders.AddBlockLocatorHash(
		h.s.chainParams.GenesisHash); err != nil {

		t.Fatal(err)
	}
	h.send(t, getHeaders)
	select {
	case msg := <-h.headers:
		if len(msg.Headers) == 0 ||
			msg.Headers[len(msg.Headers)-1].BlockHash() != tipHash {

			t.Fatal("the headers did not end with the tip")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no headers in answer to getheaders")
	}

	block := solvedBlock(t, h.s.chain, 0, nil)
	h.relay(block)
	h.expectAnnouncement(t, block.Hash(), true)
}

// handshake connects sp to a remote side whose version message has the passed
// protocol version, completes the handshake from the remote side, and returns
// the remote side.
func handshake(t *testing.T, sp *serverPeer, pver uint32) net.Conn {
	t.Helper()
	params := &chaincfg.RegressionNetParams
	local, remote := net.Pipe()
	sp.AssociateConnection(&addrConn{Conn: local,
		remote: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 18444}})
	t.Cleanup(func() {
		sp.Disconnect()
		remote.Close()
		sp.WaitForDisconnect()
	})
	me := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.1"), 18444, 0)
	you := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.2"), 18444, 0)
	version := wire.NewMsgVersion(me, you, 1, 0)
	version.ProtocolVersion = int32(pver)
	if err := wire.WriteMessage(remote, version, pver, params.Net); err != nil {
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
	return remote
}

// TestSendHeadersAfterVerAck ensures a peer whose protocol version has
// sendheaders is asked, once the handshake is done, to announce blocks with
// their headers, as BCHN asks (net_processing.cpp:2371-2377), and a peer
// whose version doesn't have it is not.
func TestSendHeadersAfterVerAck(t *testing.T) {
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	for _, test := range []struct {
		pver uint32
		want bool
	}{
		{wire.SendHeadersVersion, true},
		{wire.SendHeadersVersion - 1, false},
	} {
		s := &server{
			newPeers: make(chan *serverPeer, 1),
			quit:     make(chan struct{}),
		}
		sp := newServerPeer(s, false)
		sp.Peer = peer.NewInboundPeer(&peer.Config{
			ChainParams:            &chaincfg.RegressionNetParams,
			ProtocolVersion:        peer.MaxProtocolVersion,
			TstAllowSelfConnection: true,
		})
		remote := handshake(t, sp, test.pver)
		sp.OnVerAck(sp.Peer, wire.NewMsgVerAck())

		// A ping after the verack is answered after anything the
		// verack made the peer send.
		ping := wire.NewMsgPing(7)
		if err := wire.WriteMessage(remote, ping, test.pver,
			chaincfg.RegressionNetParams.Net); err != nil {

			t.Fatal(err)
		}
		got := false
		for {
			msg, _, err := wire.ReadMessage(remote, test.pver,
				chaincfg.RegressionNetParams.Net)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := msg.(*wire.MsgSendHeaders); ok {
				got = true
			}
			if pong, ok := msg.(*wire.MsgPong); ok && pong.Nonce == 7 {
				break
			}
		}
		if got != test.want {
			t.Fatalf("protocol version %d: sendheaders sent %v, want %v",
				test.pver, got, test.want)
		}
	}
}

// TestParentKnownFromPeer ensures the peer is known to have the parent of a
// block when it announced the parent last in an inv, or sent it in full or as
// a compact block, so the block is announced to it with its header.
func TestParentKnownFromPeer(t *testing.T) {
	tests := []struct {
		name string
		tell func(h *cmpctHarness, tip *bchutil.Block)
	}{{
		name: "inv",
		tell: func(h *cmpctHarness, tip *bchutil.Block) {
			inv := wire.NewMsgInv()
			for _, hash := range []*chainhash.Hash{{0x01}, tip.Hash()} {
				if err := inv.AddInvVect(wire.NewInvVect(
					wire.InvTypeBlock, hash)); err != nil {

					t.Fatal(err)
				}
			}
			h.send(t, inv)
		},
	}, {
		name: "block",
		tell: func(h *cmpctHarness, tip *bchutil.Block) {
			h.sp.SetAllowDirectBlockRelay(true)
			h.send(t, tip.MsgBlock())
		},
	}, {
		name: "compact block",
		tell: func(h *cmpctHarness, tip *bchutil.Block) {
			h.sp.OnCmpctBlock(nil, shortIDsOnly(t, tip))
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newCmpctHarness(t)
			h.wantHeaders(t)
			tipHash := h.s.chain.BestSnapshot().Hash
			tip, err := h.s.chain.BlockByHash(&tipHash)
			if err != nil {
				t.Fatal(err)
			}
			test.tell(h, tip)
			h.sync(t, 1)
			block := solvedBlock(t, h.s.chain, 0, nil)
			h.relay(block)
			h.expectAnnouncement(t, block.Hash(), true)
		})
	}
}

// TestKnownBlockNotAnnounced ensures a block the peer announced is not
// announced to it, though it has the block's parent, as BCHN announces only
// blocks a peer doesn't have, and that an empty answer to getheaders sends the
// peer no header.
func TestKnownBlockNotAnnounced(t *testing.T) {
	h := newCmpctHarness(t)
	h.wantHeaders(t)
	tipHash := h.s.chain.BestSnapshot().Hash

	// The peer gets the tip's header.
	getHeaders := wire.NewMsgGetHeaders()
	getHeaders.ProtocolVersion = wire.ProtocolVersion
	if err := getHeaders.AddBlockLocatorHash(
		h.s.chainParams.GenesisHash); err != nil {

		t.Fatal(err)
	}
	h.send(t, getHeaders)
	select {
	case <-h.headers:
	case <-time.After(5 * time.Second):
		t.Fatal("no headers in answer to getheaders")
	}

	block := solvedBlock(t, h.s.chain, 0, nil)
	msg := wire.NewMsgHeaders()
	if err := msg.AddBlockHeader(&block.MsgBlock().Header); err != nil {
		t.Fatal(err)
	}
	h.send(t, msg)
	h.sync(t, 1)
	h.relay(block)
	h.sync(t, 2)
	select {
	case msg := <-h.headers:
		t.Fatalf("headers for %v announced", msg.Headers[0].BlockHash())
	case iv := <-h.invs:
		t.Fatalf("inv for %v", iv.Hash)
	default:
	}

	// After an empty answer to getheaders, the tip's header is still the
	// last sent, and none other is taken for it.
	getHeaders = wire.NewMsgGetHeaders()
	getHeaders.ProtocolVersion = wire.ProtocolVersion
	if err := getHeaders.AddBlockLocatorHash(&tipHash); err != nil {
		t.Fatal(err)
	}
	h.send(t, getHeaders)
	select {
	case msg := <-h.headers:
		if len(msg.Headers) != 0 {
			t.Fatalf("%d headers after the tip", len(msg.Headers))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no headers in answer to getheaders")
	}
	if !h.sp.knowsBlock(&tipHash) {
		t.Fatal("an empty answer to getheaders replaced the tip's header")
	}
}

// TestHeadersAnnouncedOnce ensures a block announced to the peer with its
// header is not announced to it again when it is relayed again.
func TestHeadersAnnouncedOnce(t *testing.T) {
	h := newCmpctHarness(t)
	h.wantHeaders(t)
	tipHash := h.s.chain.BestSnapshot().Hash
	h.sp.sentHeaderTo(&tipHash)

	block := solvedBlock(t, h.s.chain, 0, nil)
	h.relay(block)
	h.expectAnnouncement(t, block.Hash(), true)
	h.relay(block)
	h.sync(t, 1)
	select {
	case msg := <-h.headers:
		t.Fatalf("headers for %v announced again",
			msg.Headers[0].BlockHash())
	case iv := <-h.invs:
		t.Fatalf("inv for %v", iv.Hash)
	default:
	}
}
