// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
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
	"github.com/gcash/bchutil"
)

// fetchCountingDB counts the blocks fetched through it.
type fetchCountingDB struct {
	database.DB
	fetches atomic.Int32
}

func (db *fetchCountingDB) View(fn func(database.Tx) error) error {
	return db.DB.View(func(tx database.Tx) error {
		return fn(&fetchCountingTx{Tx: tx, db: db})
	})
}

// fetchCountingTx counts the blocks fetched through it for its database.
type fetchCountingTx struct {
	database.Tx
	db *fetchCountingDB
}

func (tx *fetchCountingTx) FetchBlock(hash *chainhash.Hash) ([]byte, error) {
	tx.db.fetches.Add(1)
	return tx.Tx.FetchBlock(hash)
}

// getDataHarness is a server peer that serves two blocks from a regtest
// chain, whose remote side the test reads at its own pace.  The two blocks
// are the first of seven, so the first is more than maxCmpctBlockDepth below
// the tip and the second is not.
type getDataHarness struct {
	sp     *serverPeer
	db     *fetchCountingDB
	remote net.Conn
	blocks [2]chainhash.Hash
}

// newGetDataHarness returns a harness whose peer has completed the handshake
// and has nothing queued.
func newGetDataHarness(t *testing.T) *getDataHarness {
	t.Helper()
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	params := &chaincfg.RegressionNetParams
	chain, db := newTestChainDB(t)
	h := &getDataHarness{db: &fetchCountingDB{DB: db}}
	for i := range maxCmpctBlockDepth + 2 {
		block := solvedBlock(t, chain, 0, nil)
		if _, _, err := chain.ProcessBlock(block, blockchain.BFNone); err != nil {
			t.Fatal(err)
		}
		if i < len(h.blocks) {
			h.blocks[i] = *block.Hash()
		}
	}

	s := &server{chainParams: params, chain: chain, db: h.db}
	h.sp = newServerPeer(s, false)
	h.sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams:            params,
		ProtocolVersion:        peer.MaxProtocolVersion,
		TstAllowSelfConnection: true,
		MaxKnownInventory:      64000,
	})
	local, remote := net.Pipe()
	h.remote = remote
	h.sp.AssociateConnection(&addrConn{Conn: local,
		remote: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 18444}})
	t.Cleanup(func() {
		h.sp.Disconnect()
		remote.Close()
		h.sp.WaitForDisconnect()
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
	return h
}

// unsent returns items with nothing to send that produce no notfound: those
// of an unknown type.  A filtered block is sent whether or not a filter is
// loaded.
func (h *getDataHarness) unsent() []*wire.InvVect {
	return []*wire.InvVect{
		wire.NewInvVect(wire.InvType(0x7f), &h.blocks[0]),
		wire.NewInvVect(wire.InvType(0x7e), &h.blocks[1]),
	}
}

// getData returns a getdata that asks, rounds times, for both blocks and then
// for an item with nothing to send, taken in turn from nothing.  With
// endWithBlock, a last request for the first block follows.  It also returns
// how many blocks the getdata asks for.
func (h *getDataHarness) getData(rounds int, nothing []*wire.InvVect,
	endWithBlock bool) (*wire.MsgGetData, int) {

	msg := wire.NewMsgGetData()
	for r := range rounds {
		_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, &h.blocks[0]))
		_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, &h.blocks[1]))
		_ = msg.AddInvVect(nothing[r%len(nothing)])
	}
	blocks := 2 * rounds
	if endWithBlock {
		_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, &h.blocks[0]))
		blocks++
	}
	return msg, blocks
}

// serve handles msg in the background and returns a channel that is closed
// when the handler returns.
func (h *getDataHarness) serve(msg *wire.MsgGetData) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		h.sp.OnGetData(nil, msg)
		close(done)
	}()
	return done
}

// readAll reads from the remote side until it has the passed number of blocks
// and, with notFound, a notfound.
func (h *getDataHarness) readAll(t *testing.T, blocks int, notFound bool) {
	t.Helper()
	if err := h.remote.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for blocks > 0 || notFound {
		msg, _, err := wire.ReadMessage(h.remote, wire.ProtocolVersion,
			chaincfg.RegressionNetParams.Net)
		if err != nil {
			t.Fatalf("reading with %d blocks still to come: %v", blocks, err)
		}
		switch msg.(type) {
		case *wire.MsgBlock:
			blocks--
		case *wire.MsgNotFound:
			if blocks > 0 {
				t.Fatalf("notfound came before the last %d blocks", blocks)
			}
			notFound = false
		}
	}
}

// returnedWithin reports whether done is closed within d.
func returnedWithin(done <-chan struct{}, d time.Duration) bool {
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// TestOnGetDataReturnsOnceSent ensures the getdata handler returns only once
// the blocks it queued have been sent, when the last item asked for has
// nothing to send and nothing is unknown.  Such an item signaled completion at
// once, so the handler returned with the blocks before it still queued, and
// took the peer's next getdata while they were.
func TestOnGetDataReturnsOnceSent(t *testing.T) {
	h := newGetDataHarness(t)
	msg, blocks := h.getData(10, h.unsent(), false)

	done := h.serve(msg)
	if returnedWithin(done, 200*time.Millisecond) {
		t.Fatal("getdata handler returned before its blocks were sent")
	}
	h.readAll(t, blocks-1, false)
	if returnedWithin(done, 200*time.Millisecond) {
		t.Fatal("getdata handler returned before its last block was sent")
	}
	h.readAll(t, 1, false)
	if !returnedWithin(done, 10*time.Second) {
		t.Fatal("getdata handler did not return once its blocks were sent")
	}
}

// TestOnGetDataQueuesFewBlocks ensures the getdata handler fetches only a few
// blocks ahead of those sent, however many of the items asked for have
// nothing to send.  It waits on every third item, and such an item signaled
// at once that it was sent, so with one in every third place the handler
// never waited, and queued every block the getdata asked for.
func TestOnGetDataQueuesFewBlocks(t *testing.T) {
	// More than the handler may fetch before the first block is sent, and
	// far fewer than the getdata asks for.
	const maxAhead = 6

	h := newGetDataHarness(t)
	nothing := append(h.unsent(),
		wire.NewInvVect(wire.InvTypeBlock, &chainhash.Hash{1}))
	msg, blocks := h.getData(10, nothing, true)

	done := h.serve(msg)
	if returnedWithin(done, 200*time.Millisecond) {
		t.Fatal("getdata handler returned before its blocks were sent")
	}
	if n := h.db.fetches.Load(); n > maxAhead {
		t.Fatalf("getdata handler fetched %d blocks before sending any, "+
			"want at most %d", n, maxAhead)
	}
	h.readAll(t, blocks, true)
	if !returnedWithin(done, 10*time.Second) {
		t.Fatal("getdata handler did not return once its blocks were sent")
	}
}

// readMessage reads the next message from the remote side.
func (h *getDataHarness) readMessage(t *testing.T) wire.Message {
	t.Helper()
	if err := h.remote.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	msg, _, err := wire.ReadMessage(h.remote, wire.ProtocolVersion,
		chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// sideBlock stores a sibling of the chain's tip, on a side chain, and returns
// its hash.
func sideBlock(t *testing.T, chain *blockchain.BlockChain) chainhash.Hash {
	t.Helper()
	best := chain.BestSnapshot()
	tip, err := chain.BlockByHash(&best.Hash)
	if err != nil {
		t.Fatal(err)
	}
	// A sibling of the tip, paying one satoshi less.
	coinbase := tip.MsgBlock().Transactions[0].Copy()
	coinbase.TxOut[0].Value--
	msgBlock := wire.MsgBlock{
		Header:       tip.MsgBlock().Header,
		Transactions: []*wire.MsgTx{coinbase},
	}
	merkles := blockchain.BuildMerkleTreeStore(
		bchutil.NewBlock(&msgBlock).Transactions())
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	for blockchain.CheckHeaderProofOfWork(&msgBlock.Header,
		chaincfg.RegressionNetParams.PowLimit) != nil {

		msgBlock.Header.Nonce++
	}
	block := bchutil.NewBlock(&msgBlock)
	if _, _, err := chain.ProcessBlock(block, blockchain.BFNone); err != nil {
		t.Fatal(err)
	}
	if chain.MainChainHasBlock(block.Hash()) {
		t.Fatal("the sibling of the tip is in the main chain")
	}
	return *block.Hash()
}

// TestGetDataOldCmpctBlockSentInFull ensures a getdata for the compact block
// of a block more than maxCmpctBlockDepth below the tip, or not in the main
// chain, is answered with the full block, as in BCHN, and one for a block at
// that depth with its compact block, which sends the transactions the peer
// knows as short IDs.
func TestGetDataOldCmpctBlockSentInFull(t *testing.T) {
	h := newGetDataHarness(t)
	old, recent := h.blocks[0], h.blocks[1]
	side := sideBlock(t, h.sp.server.chain)
	block, err := h.sp.server.chain.BlockByHash(&recent)
	if err != nil {
		t.Fatal(err)
	}
	h.sp.AddKnownInventory(wire.NewInvVect(wire.InvTypeTx,
		block.Transactions()[0].Hash()))

	msg := wire.NewMsgGetData()
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeCmpctBlock, &old))
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeCmpctBlock, &side))
	_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeCmpctBlock, &recent))
	done := h.serve(msg)
	full, ok := h.readMessage(t).(*wire.MsgBlock)
	if !ok || full.BlockHash() != old {
		t.Fatalf("the block %d below the tip was not sent in full",
			maxCmpctBlockDepth+1)
	}
	full, ok = h.readMessage(t).(*wire.MsgBlock)
	if !ok || full.BlockHash() != side {
		t.Fatal("the block on a side chain was not sent in full")
	}
	cmpct, ok := h.readMessage(t).(*wire.MsgCmpctBlock)
	if !ok || cmpct.BlockHash() != recent {
		t.Fatalf("the block %d below the tip was not sent as a compact "+
			"block", maxCmpctBlockDepth)
	}
	if len(cmpct.ShortIDs) != 1 || len(cmpct.PrefilledTxs) != 0 {
		t.Fatalf("compact block with %d short IDs and %d prefilled "+
			"transactions, want the known coinbase as a short ID",
			len(cmpct.ShortIDs), len(cmpct.PrefilledTxs))
	}
	if !returnedWithin(done, 10*time.Second) {
		t.Fatal("getdata handler did not return")
	}
}

// TestCmpctBlockCostBoundedByBlock ensures building a peer's compact block,
// for a getdata or for direct relay, costs in proportion to the block rather
// than to the peer's known inventory.  Both used to copy every transaction of
// the known inventory, up to 64,000, under its lock.
func TestCmpctBlockCostBoundedByBlock(t *testing.T) {
	h := newGetDataHarness(t)
	s := h.sp.server
	hash := h.blocks[1]
	block, err := s.chain.BlockByHash(&hash)
	if err != nil {
		t.Fatal(err)
	}
	// The remote side asks for direct relay.
	err = wire.WriteMessage(h.remote, wire.NewMsgSendCmpct(true,
		wire.CompactBlocksProtocolVersion), wire.ProtocolVersion,
		chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !h.sp.WantsDirectBlockRelay() {
		if time.Now().After(deadline) {
			t.Fatal("the peer did not ask for direct relay")
		}
		time.Sleep(10 * time.Millisecond)
	}
	state := newTestPeerState()
	state.inboundPeers[h.sp.ID()] = h.sp
	relay := relayMsg{invVect: wire.NewInvVect(wire.InvTypeBlock, &hash),
		data: block.MsgBlock()}
	// The peer has the block's parent, so the block is relayed to it as a
	// compact block.
	h.sp.gotBlockFrom(&h.blocks[0])

	measure := func() (getData, relayed float64) {
		getData = testing.AllocsPerRun(5, func() {
			err := s.pushCmpctBlockMsg(h.sp, &hash, nil, nil,
				wire.BaseEncoding)
			if err != nil {
				t.Fatal(err)
			}
		})
		relayed = testing.AllocsPerRun(5, func() {
			s.handleRelayInvMsg(state, relay)
		})
		return getData, relayed
	}
	emptyGetData, emptyRelay := measure()
	for i := range 64000 {
		known := chainhash.Hash{byte(i), byte(i >> 8), byte(i >> 16), 9}
		h.sp.AddKnownInventory(wire.NewInvVect(wire.InvTypeTx, &known))
	}
	fullGetData, fullRelay := measure()
	t.Logf("allocations with no and 64000 known transactions: getdata %v "+
		"and %v, relay %v and %v", emptyGetData, fullGetData, emptyRelay,
		fullRelay)
	if fullGetData > emptyGetData+20 || fullRelay > emptyRelay+20 {
		t.Fatal("a full known inventory adds to the cost of a compact block")
	}
}
