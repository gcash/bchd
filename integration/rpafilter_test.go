// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil/gcs/builder"
)

// rpaReplyTimeout is how long the node is given to answer a filter request.
const rpaReplyTimeout = 10 * time.Second

// TestRPAFilters mines a block with a payment that spends several of the
// harness wallet's coins, rebuilds the input prefix filter of every block of
// the chain from the blocks themselves, and checks the filters and headers
// the node serves for filter type 2: through getcfilter and getcfilterheader,
// and to a raw peer through getcfilters, getcfheaders and getcfcheckpt.
func TestRPAFilters(t *testing.T) {
	h, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	defer func() { _ = h.TearDown() }()
	if err = h.SetUp(true, 25); err != nil {
		t.Fatalf("set up harness: %v", err)
	}

	// A payment large enough to spend several coins.
	script, err := txscript.NewScriptBuilder().AddOp(txscript.OP_TRUE).Script()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.CreateTransaction([]*wire.TxOut{wire.NewTxOut(120e8, script, wire.TokenData{})}, 10, true)
	if err != nil {
		t.Fatalf("create the payment: %v", err)
	}
	if len(tx.TxIn) < 2 {
		t.Fatalf("the payment spends %d coins, want several", len(tx.TxIn))
	}
	if _, err = h.Node.SendRawTransaction(tx, true); err != nil {
		t.Fatalf("send the payment: %v", err)
	}
	if _, err = h.Node.Generate(1); err != nil {
		t.Fatalf("mine the block: %v", err)
	}

	bestHash, tip, err := h.Node.GetBestBlock()
	if err != nil {
		t.Fatal(err)
	}
	hashes := make([]chainhash.Hash, tip+1)
	filters := make([][]byte, tip+1)
	headers := make([]chainhash.Hash, tip+1)
	var prev chainhash.Hash
	sawPayment := false
	for height := int32(0); height <= tip; height++ {
		hash, err := h.Node.GetBlockHash(int64(height))
		if err != nil {
			t.Fatal(err)
		}
		block, err := h.Node.GetBlock(hash)
		if err != nil {
			t.Fatal(err)
		}
		hashes[height] = *hash
		filters[height] = builder.BuildRPAFilter(block)
		headers[height] = builder.MakeHeaderForRPAFilter(filters[height], prev)
		prev = headers[height]

		served, err := h.Node.GetCFilter(hash, wire.GCSFilterInputPrefix)
		if err != nil || !bytes.Equal(served.Data, filters[height]) {
			t.Fatalf("getcfilter type 2 at height %d: %x, %v; want %x", height, served, err, filters[height])
		}
		header, err := h.Node.GetCFilterHeader(hash, wire.GCSFilterInputPrefix)
		if err != nil || header.PrevFilterHeader != headers[height] {
			t.Fatalf("getcfilterheader type 2 at height %d: %v, %v; want %v",
				height, header, err, headers[height])
		}

		for _, btx := range block.Transactions {
			if btx.TxHash() == tx.TxHash() {
				sawPayment = true
				values, err := builder.DecodeRPAFilter(filters[height])
				if err != nil {
					t.Fatal(err)
				}
				for i, txIn := range btx.TxIn {
					if !builder.RPAFilterMatch(values, builder.RPAInputValue(txIn), 16) {
						t.Errorf("input %d of the payment is not in its block's filter", i)
					}
				}
			}
		}
	}
	if !sawPayment {
		t.Fatal("no block holds the payment")
	}
	if hashes[tip] != *bestHash {
		t.Fatalf("tip %v, best block %v", hashes[tip], bestHash)
	}

	// A raw peer asks for the headers and filters of the whole chain.
	conn := dialRPAPeer(t, h.P2PAddress())
	defer conn.peer.Disconnect()

	conn.peer.QueueMessage(wire.NewMsgGetCFHeaders(wire.GCSFilterInputPrefix, 1, bestHash), nil)
	select {
	case msg := <-conn.headers:
		if msg.FilterType != wire.GCSFilterInputPrefix || msg.StopHash != *bestHash ||
			msg.PrevFilterHeader != headers[0] || len(msg.FilterHashes) != int(tip) {
			t.Fatalf("cfheaders: type %v, stop %v, previous %v, %d hashes",
				msg.FilterType, msg.StopHash, msg.PrevFilterHeader, len(msg.FilterHashes))
		}
		for i, hash := range msg.FilterHashes {
			if want := builder.GetRPAFilterHash(filters[i+1]); *hash != want {
				t.Errorf("cfheaders hash %d: %v, want %v", i+1, hash, want)
			}
		}
	case <-time.After(rpaReplyTimeout):
		t.Fatal("no cfheaders")
	}

	conn.peer.QueueMessage(wire.NewMsgGetCFilters(wire.GCSFilterInputPrefix, 0, bestHash), nil)
	for height := int32(0); height <= tip; height++ {
		select {
		case msg := <-conn.filters:
			if msg.FilterType != wire.GCSFilterInputPrefix || msg.BlockHash != hashes[height] ||
				!bytes.Equal(msg.Data, filters[height]) {
				t.Fatalf("cfilter %d: type %v, block %v, %x; want %x",
					height, msg.FilterType, msg.BlockHash, msg.Data, filters[height])
			}
		case <-time.After(rpaReplyTimeout):
			t.Fatalf("no cfilter for height %d", height)
		}
	}

	// The chain is shorter than the checkpoint interval, so the answer
	// lists no headers, for filter type 2.
	conn.peer.QueueMessage(wire.NewMsgGetCFCheckpt(wire.GCSFilterInputPrefix, bestHash), nil)
	select {
	case msg := <-conn.checkpts:
		if msg.FilterType != wire.GCSFilterInputPrefix || msg.StopHash != *bestHash ||
			len(msg.FilterHeaders) != int(tip)/wire.CFCheckptInterval {
			t.Fatalf("cfcheckpt: type %v, stop %v, %d headers",
				msg.FilterType, msg.StopHash, len(msg.FilterHeaders))
		}
	case <-time.After(rpaReplyTimeout):
		t.Fatal("no cfcheckpt")
	}
}

// rpaConn is a raw peer connected to the harness, with the replies to its
// filter requests.
type rpaConn struct {
	peer     *peer.Peer
	headers  chan *wire.MsgCFHeaders
	filters  chan *wire.MsgCFilter
	checkpts chan *wire.MsgCFCheckpt
}

// dialRPAPeer connects a raw peer to the harness and waits for the node's
// verack.
func dialRPAPeer(t *testing.T, address string) *rpaConn {
	t.Helper()
	c := &rpaConn{
		headers:  make(chan *wire.MsgCFHeaders, 4),
		filters:  make(chan *wire.MsgCFilter, 64),
		checkpts: make(chan *wire.MsgCFCheckpt, 4),
	}
	verack := make(chan struct{}, 1)
	p, err := peer.NewOutboundPeer(&peer.Config{
		UserAgentName:    "rpa",
		UserAgentVersion: "1.0.0",
		ChainParams:      &chaincfg.SimNetParams,
		Listeners: peer.MessageListeners{
			OnVerAck: func(*peer.Peer, *wire.MsgVerAck) {
				select {
				case verack <- struct{}{}:
				default:
				}
			},
			OnCFHeaders: func(_ *peer.Peer, msg *wire.MsgCFHeaders) { c.headers <- msg },
			OnCFilter:   func(_ *peer.Peer, msg *wire.MsgCFilter) { c.filters <- msg },
			// The peer package hands a cfcheckpt to no listener, so it
			// is taken as it is read.
			OnRead: func(_ *peer.Peer, _ int, msg wire.Message, err error) {
				if checkpt, ok := msg.(*wire.MsgCFCheckpt); ok && err == nil {
					c.checkpts <- checkpt
				}
			},
		},
	}, address)
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	netConn, err := net.Dial("tcp", p.Addr())
	if err != nil {
		t.Fatalf("dial %s: %v", p.Addr(), err)
	}
	p.AssociateConnection(netConn)
	c.peer = p
	select {
	case <-verack:
	case <-time.After(rpaReplyTimeout):
		p.Disconnect()
		t.Fatal("the peer did not finish the handshake")
	}
	return c
}
