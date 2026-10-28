// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// TestCompetingChains relays a competing chain that overtakes a node's own to
// the node, from a peer that may relay blocks directly, and checks that the
// node follows it.  The peer announces the chain's first block, sends the
// others unrequested, and then the first, which the node asked for, as it
// would ask for an orphan's parents.  The chain up to the first orphans has
// less work than the node's tip, so the node must keep the orphans and store
// them once their parents arrive, or it only gets the next block each time it
// drops its stalled sync peer, every two minutes.  Two nodes mining competing
// chains at once showed this only when one mined its blocks faster than the
// other could request their parents.
func TestCompetingChains(t *testing.T) {
	h1, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h1.TearDown()
	if err := h1.SetUp(false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := h1.Node.Generate(20); err != nil {
		t.Fatal(err)
	}

	// The competing chain is mined by a node that is not connected to the
	// first.
	h2, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h2.TearDown()
	if err := h2.SetUp(false, 0); err != nil {
		t.Fatal(err)
	}
	hashes, err := h2.Node.Generate(30)
	if err != nil {
		t.Fatal(err)
	}
	blocks := make([]*wire.MsgBlock, len(hashes))
	for i, hash := range hashes {
		if blocks[i], err = h2.Node.GetBlock(hash); err != nil {
			t.Fatal(err)
		}
	}

	first := hashes[0]
	announce := make(chan bool, 1)
	requested := make(chan struct{}, 1)
	p, err := peer.NewOutboundPeer(&peer.Config{
		UserAgentName:    "relay",
		UserAgentVersion: "1.0.0",
		ChainParams:      &chaincfg.SimNetParams,
		Listeners: peer.MessageListeners{
			OnSendCmpct: func(_ *peer.Peer, msg *wire.MsgSendCmpct) {
				select {
				case announce <- msg.Announce:
				default:
				}
			},
			OnGetData: func(_ *peer.Peer, msg *wire.MsgGetData) {
				for _, iv := range msg.InvList {
					if iv.Hash != *first {
						continue
					}
					select {
					case requested <- struct{}{}:
					default:
					}
				}
			},
		},
	}, h1.P2PAddress())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", p.Addr())
	if err != nil {
		t.Fatal(err)
	}
	p.AssociateConnection(conn)
	defer p.Disconnect()

	select {
	case ok := <-announce:
		if !ok {
			t.Fatal("the node does not let the peer relay blocks directly")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the node sent no sendcmpct")
	}

	// The node only asks for an announced block once it knows the peer, so
	// the orphans are sent once it has asked for the first block.
	inv := wire.NewMsgInv()
	if err := inv.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, first)); err != nil {
		t.Fatal(err)
	}
	p.QueueMessage(inv, nil)
	select {
	case <-requested:
	case <-time.After(10 * time.Second):
		t.Fatal("the node did not ask for the first block")
	}
	for _, block := range blocks[1:] {
		p.QueueMessage(block, nil)
	}
	p.QueueMessage(blocks[0], nil)

	tip := hashes[len(hashes)-1]
	deadline := time.Now().Add(20 * time.Second)
	for {
		hash, height, err := h1.Node.GetBestBlock()
		if err != nil {
			t.Fatal(err)
		}
		if *hash == *tip {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the node did not follow the competing chain "+
				"within 20 seconds: its tip is at height %d", height)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
