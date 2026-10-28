// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestCompetingChains has a peer announce two chains to a node: a fork of one
// block with less work than the node's tip, and a competing chain with more
// work, whose blocks the node doesn't have.  The node must ask for the headers
// leading to each, fetch the blocks of the competing chain and follow it, and
// neither fetch nor store the fork's block, which it fetched when announced and
// stored though its chain had less work.
func TestCompetingChains(t *testing.T) {
	h1, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h1.TearDown()
	if err := h1.SetUp(false, 0); err != nil {
		t.Fatal(err)
	}
	mined, err := h1.Node.Generate(20)
	if err != nil {
		t.Fatal(err)
	}

	// The fork's block builds on the node's block 18.
	parent, err := h1.Node.GetBlock(mined[17])
	if err != nil {
		t.Fatal(err)
	}
	parentBlock := bchutil.NewBlock(parent)
	parentBlock.SetHeight(18)
	addr, err := h1.NewAddress()
	if err != nil {
		t.Fatal(err)
	}
	fork, err := rpctest.CreateBlock(parentBlock, nil, parent.Header.Version,
		parent.Header.Timestamp.Add(time.Second), addr, nil,
		&chaincfg.SimNetParams)
	if err != nil {
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
	blocks := make(map[chainhash.Hash]*wire.MsgBlock)
	blocks[*fork.Hash()] = fork.MsgBlock()
	var chain []*wire.BlockHeader
	for _, hash := range hashes {
		block, err := h2.Node.GetBlock(hash)
		if err != nil {
			t.Fatal(err)
		}
		blocks[*hash] = block
		chain = append(chain, &block.Header)
	}

	// The peer answers getheaders with the fork's header or the competing
	// chain's headers up to the one asked for, and getdata with blocks.
	var mtx sync.Mutex
	asked := make(map[chainhash.Hash]bool)
	p, err := peer.NewOutboundPeer(&peer.Config{
		UserAgentName:    "relay",
		UserAgentVersion: "1.0.0",
		ChainParams:      &chaincfg.SimNetParams,
		Listeners: peer.MessageListeners{
			OnGetHeaders: func(p *peer.Peer, msg *wire.MsgGetHeaders) {
				mtx.Lock()
				asked[msg.HashStop] = true
				mtx.Unlock()
				reply := wire.NewMsgHeaders()
				if msg.HashStop == *fork.Hash() {
					_ = reply.AddBlockHeader(&fork.MsgBlock().Header)
				} else {
					for _, header := range chain {
						_ = reply.AddBlockHeader(header)
						if header.BlockHash() == msg.HashStop {
							break
						}
					}
				}
				p.QueueMessage(reply, nil)
			},
			OnGetData: func(p *peer.Peer, msg *wire.MsgGetData) {
				for _, iv := range msg.InvList {
					if block, ok := blocks[iv.Hash]; ok {
						p.QueueMessage(block, nil)
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

	// announce has the peer announce the block with an inv until the node
	// asks for the headers leading to it, which it only does once it knows
	// the peer.
	announce := func(hash *chainhash.Hash) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			inv := wire.NewMsgInv()
			if err := inv.AddInvVect(wire.NewInvVect(wire.InvTypeBlock,
				hash)); err != nil {

				t.Fatal(err)
			}
			p.QueueMessage(inv, nil)
			time.Sleep(200 * time.Millisecond)
			mtx.Lock()
			done := asked[*hash]
			mtx.Unlock()
			if done {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("the node did not ask for the headers leading "+
					"to block %v", hash)
			}
		}
	}
	announce(fork.Hash())
	announce(hashes[len(hashes)-1])

	tip := hashes[len(hashes)-1]
	deadline := time.Now().Add(20 * time.Second)
	for {
		hash, height, err := h1.Node.GetBestBlock()
		if err != nil {
			t.Fatal(err)
		}
		if *hash == *tip {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the node did not follow the competing chain "+
				"within 20 seconds: its tip is at height %d", height)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := h1.Node.GetBlock(fork.Hash()); err == nil {
		t.Fatal("the node stored the block of the fork with less work")
	}
}
