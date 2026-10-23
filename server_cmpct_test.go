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

// expectFullBlockRequest waits for a getdata for the full block.
func (h *cmpctHarness) expectFullBlockRequest(t *testing.T,
	hash chainhash.Hash) {

	t.Helper()
	select {
	case iv := <-h.getData:
		if iv.Type != wire.InvTypeBlock || iv.Hash != hash {
			t.Fatalf("getdata for %v, want block %v", iv, hash)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("block %v was not requested in full", hash)
	}
}

// expectNotRelayed ensures no compact block has been relayed.
func (h *cmpctHarness) expectNotRelayed(t *testing.T) {
	t.Helper()
	select {
	case msg := <-h.s.relayCmpctBlock:
		t.Fatalf("compact block %v relayed", msg.BlockHash())
	default:
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
	waitDone(t, firstDone)
	waitDone(t, secondDone)
	// Both blocks are reconstructed and processed.  Only one is sure to be
	// relayed, as the other no longer extends the best chain once the
	// first is connected.
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

	// The relayed block answers getblocktxns from the peers it went to.
	if h.s.recentBlockByHash(block.Hash()) == nil {
		t.Fatal("the relayed block is not the recent block")
	}
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

// TestRemoveCmpctWaiter ensures ending a wait that already ended returns the
// reply that ended it and leaves a newer wait for the same block alone, and
// that ending a current wait removes it.
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
	if got := sp.removeCmpctWaiter(hash, first); got != wire.Message(reply) {
		t.Fatal("the reply that ended the wait was not returned")
	}
	if !sp.deliverCmpctReply(hash, reply) {
		t.Fatal("removing the ended wait removed the new one")
	}
	if got := <-second; got != wire.Message(reply) {
		t.Fatal("new wait got the wrong reply")
	}

	third, ok := sp.addCmpctWaiter(hash)
	if !ok {
		t.Fatal("third wait not added")
	}
	if sp.removeCmpctWaiter(hash, third) != nil || sp.cmpctWaiting(hash) {
		t.Fatal("ending a current wait returned a reply or left it")
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
	h.expectNotRelayed(t)
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

// TestCompactBlockWrongTransactions ensures a compact block whose
// reconstructed transactions don't match the merkle root is not relayed,
// and that the full block is requested and processed instead.  This covers
// wrong transactions from the compact block itself and from blocktxns.
func TestCompactBlockWrongTransactions(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	other := solvedBlock(t, h.s.chain, -1, nil)
	wrongTx := other.MsgBlock().Transactions[0]
	h.request(t, *block.Hash())

	// A prefilled transaction that is not the block's.
	prefilled := &wire.MsgCmpctBlock{
		Header:       block.MsgBlock().Header,
		PrefilledTxs: []*wire.PrefilledTx{{Index: 0, Tx: wrongTx}},
	}
	waitDone(t, h.process(prefilled))
	h.expectFullBlockRequest(t, *block.Hash())

	// A blocktxns with a transaction that is not the block's.
	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	h.send(t, wire.NewMsgBlockTxns(*block.Hash(), []*wire.MsgTx{wrongTx}))
	waitDone(t, done)
	h.expectFullBlockRequest(t, *block.Hash())
	h.expectNotRelayed(t)

	// The full block is accepted as the block requested from the peer.
	h.send(t, block.MsgBlock())
	h.expectHaveBlock(t, block.Hash())
}

// TestCompactBlockRepeatedTransaction ensures a reconstructed compact block
// that repeats its last transaction, which leaves the merkle root
// unchanged, is not relayed and the full block is requested instead.
func TestCompactBlockRepeatedTransaction(t *testing.T) {
	h := newCmpctHarness(t)
	msgBlock := *solvedBlock(t, h.s.chain, 0, nil).MsgBlock()
	var txs []*wire.MsgTx
	for i := byte(1); i <= 2; i++ {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{i}, 0),
			nil))
		tx.AddTxOut(wire.NewTxOut(1, []byte{0x51}, wire.TokenData{}))
		txs = append(txs, tx)
	}

	// Three transactions have the same merkle root as the same three
	// with the last one repeated.
	msgBlock.Transactions = append(msgBlock.Transactions[:1], txs...)
	merkles := blockchain.BuildMerkleTreeStore(
		bchutil.NewBlock(&msgBlock).Transactions())
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	for blockchain.CheckHeaderProofOfWork(&msgBlock.Header,
		h.s.chainParams.PowLimit) != nil {

		msgBlock.Header.Nonce++
	}
	hash := msgBlock.BlockHash()
	repeated := append(txs, txs[1])
	msgBlock.Transactions = append(msgBlock.Transactions[:1], repeated...)
	h.request(t, hash)

	done := h.process(shortIDsOnly(t, bchutil.NewBlock(&msgBlock)))
	h.expectGetTxns(t, hash)
	h.send(t, wire.NewMsgBlockTxns(hash, msgBlock.Transactions))
	waitDone(t, done)
	h.expectFullBlockRequest(t, hash)
	h.expectNotRelayed(t)
}

// TestCompactBlockSideChain ensures a compact block for a block on a side
// chain has its header checked against its own parent, not the tip, and is
// processed but not relayed.  Its timestamp is after its parent's median
// time but not the tip's, so a check against the tip would refuse it.
func TestCompactBlockSideChain(t *testing.T) {
	h := newCmpctHarness(t)
	best := h.s.chain.BestSnapshot()
	tip, err := h.s.chain.BlockByHash(&best.Hash)
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
	msgBlock.Header.Timestamp = best.MedianTime.Add(-time.Minute)
	for blockchain.CheckHeaderProofOfWork(&msgBlock.Header,
		h.s.chainParams.PowLimit) != nil {

		msgBlock.Header.Nonce++
	}
	block := bchutil.NewBlock(&msgBlock)
	h.request(t, *block.Hash())

	msg, err := wire.NewMsgCmpctBlockFromBlock(&msgBlock, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, h.process(msg))
	h.expectHaveBlock(t, block.Hash())
	h.expectNotRelayed(t)
}

// TestCompactBlockUnknownParent ensures a compact block whose parent is not
// known is fetched in full, so it is processed as an orphan and its parents
// are requested, and is not relayed.
func TestCompactBlockUnknownParent(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, &chainhash.Hash{0x55})
	h.request(t, *block.Hash())

	msg, err := wire.NewMsgCmpctBlockFromBlock(block.MsgBlock(), nil)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, h.process(msg))
	h.expectFullBlockRequest(t, *block.Hash())
	h.expectNotRelayed(t)
}

// TestCompactBlockKnownOrphan ensures a compact block for a block the chain
// holds as an orphan is not fetched in full again.  A block whose parent is
// unknown is fetched in full, and the chain was only asked whether it has the
// block once the parent was known, so each later compact block for the block,
// one from every peer that relays it, fetched it again.
func TestCompactBlockKnownOrphan(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, &chainhash.Hash{0x55})
	h.request(t, *block.Hash())

	msg, err := wire.NewMsgCmpctBlockFromBlock(block.MsgBlock(), nil)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, h.process(msg))
	h.expectFullBlockRequest(t, *block.Hash())
	h.send(t, block.MsgBlock())
	h.expectHaveBlock(t, block.Hash())
	if !h.s.chain.IsKnownOrphan(block.Hash()) {
		t.Fatal("the block is not an orphan")
	}

	waitDone(t, h.process(msg))
	select {
	case iv := <-h.getData:
		t.Fatalf("getdata for %v, whose block is a known orphan", iv)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestCompactBlockKnownForgetsRequest ensures a compact block for a block the
// chain got otherwise forgets the block's request from the peer.  A full block
// the peer sends afterwards is then one it was not asked for, and the peer is
// disconnected for it.
func TestCompactBlockKnownForgetsRequest(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())
	if _, _, err := h.s.chain.ProcessBlock(block, blockchain.BFNone); err != nil {
		t.Fatal(err)
	}

	waitDone(t, h.process(shortIDsOnly(t, block)))
	h.send(t, block.MsgBlock())
	disconnected := make(chan struct{})
	go func() {
		h.sp.WaitForDisconnect()
		close(disconnected)
	}()
	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("the block was still requested from the peer")
	}
}

// TestDirectRelaySlotNotForGonePeer ensures a direct relay slot is not given
// to a peer that is already gone, as one turned away at the peer limit can
// ask for it after its disconnect was handled, and the slot would then never
// be freed.
func TestDirectRelaySlotNotForGonePeer(t *testing.T) {
	h := newCmpctHarness(t)
	s := &server{}
	state := &peerState{directRelayPeers: make(map[int32]*serverPeer)}
	resp := make(chan bool, 1)

	gone := newServerPeer(s, false)
	gone.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams: &chaincfg.RegressionNetParams,
	})
	s.handleMaybeAddDirectRelayPeer(state,
		&maybeAddDirectRelayPeerMsg{peer: gone, response: resp})
	if <-resp || len(state.directRelayPeers) != 0 {
		t.Fatal("a peer that is gone got a direct relay slot")
	}

	s.handleMaybeAddDirectRelayPeer(state,
		&maybeAddDirectRelayPeerMsg{peer: h.sp, response: resp})
	if !<-resp || len(state.directRelayPeers) != 1 {
		t.Fatal("a connected peer got no direct relay slot")
	}
}

// TestCompactBlockTooManyTxs ensures a compact block describing more
// transactions than fit in the next block's size limit is dropped before it
// is decoded, and its request is forgotten so the block can be requested
// again.
func TestCompactBlockTooManyTxs(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())
	header := block.MsgBlock().Header
	limit, err := h.s.chain.NextBlockSizeLimit(&header.PrevBlock)
	if err != nil {
		t.Fatal(err)
	}
	msg := &wire.MsgCmpctBlock{
		Header:   header,
		ShortIDs: make([][wire.ShortIDSize]byte, limit/blockchain.MinTransactionSize+1),
	}

	waitDone(t, h.process(msg))
	select {
	case <-h.getTxns:
		t.Fatal("transactions requested for an oversized compact block")
	case <-time.After(100 * time.Millisecond):
	}
	h.request(t, *block.Hash())
}

// TestCompactBlockAlreadyKnown ensures a compact block for a block the chain
// already has is not reconstructed.
func TestCompactBlockAlreadyKnown(t *testing.T) {
	h := newCmpctHarness(t)
	best := h.s.chain.BestSnapshot()
	tip, err := h.s.chain.BlockByHash(&best.Hash)
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, h.process(shortIDsOnly(t, tip)))
	select {
	case <-h.getTxns:
		t.Fatal("transactions requested for a known block")
	case <-time.After(100 * time.Millisecond):
	}
	h.expectNotRelayed(t)
}

// TestCompactBlocksInFlightLimit ensures at most two compact blocks from a
// peer are reconstructed at a time, which bounds the memory a peer can make
// waiting compact blocks hold.  A compact block dropped at the limit has its
// full block requested, unless it is known.
func TestCompactBlocksInFlightLimit(t *testing.T) {
	h := newCmpctHarness(t)
	best := h.s.chain.BestSnapshot()
	tip, err := h.s.chain.BlockByHash(&best.Hash)
	if err != nil {
		t.Fatal(err)
	}
	blocks := make(map[chainhash.Hash]*bchutil.Block)
	var msgs []*wire.MsgCmpctBlock
	for i := range 3 {
		block := solvedBlock(t, h.s.chain, -int64(i), nil)
		blocks[*block.Hash()] = block
		h.request(t, *block.Hash())
		msgs = append(msgs, shortIDsOnly(t, block))
	}
	for _, msg := range msgs {
		h.sp.OnCmpctBlock(nil, msg)
	}
	h.sp.OnCmpctBlock(nil, shortIDsOnly(t, tip))

	var waiting []chainhash.Hash
	for done := false; !done; {
		select {
		case msg := <-h.getTxns:
			waiting = append(waiting, msg.BlockHash)
		case <-time.After(500 * time.Millisecond):
			done = true
		}
	}
	for _, hash := range waiting {
		h.send(t, blockTxnsFor(blocks[hash]))
	}
	if len(waiting) != 2 {
		t.Fatalf("%d compact blocks waited for transactions at once, "+
			"want 2", len(waiting))
	}

	// The dropped block is requested in full, and processed as the block
	// requested from the peer.  The known tip is not requested.
	for _, hash := range waiting {
		h.expectHaveBlock(t, &hash)
		delete(blocks, hash)
	}
	for hash, block := range blocks {
		h.expectFullBlockRequest(t, hash)
		select {
		case iv := <-h.getData:
			t.Fatalf("getdata for %v", iv)
		case <-time.After(100 * time.Millisecond):
		}
		h.send(t, block.MsgBlock())
		h.expectHaveBlock(t, &hash)
	}
}

// TestCompactBlockInvalidNotRelayed ensures a compact block that fails the
// checks needing no inputs, here for holding two coinbases, is not relayed,
// as BCHN relays a block before storing it only once its CheckBlock and
// ContextualCheckBlock pass.
func TestCompactBlockInvalidNotRelayed(t *testing.T) {
	h := newCmpctHarness(t)
	msgBlock := *solvedBlock(t, h.s.chain, 0, nil).MsgBlock()
	second := msgBlock.Transactions[0].Copy()
	second.TxOut[0].Value--
	msgBlock.Transactions = append(msgBlock.Transactions, second)
	merkles := blockchain.BuildMerkleTreeStore(
		bchutil.NewBlock(&msgBlock).Transactions())
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	for blockchain.CheckHeaderProofOfWork(&msgBlock.Header,
		h.s.chainParams.PowLimit) != nil {

		msgBlock.Header.Nonce++
	}
	block := bchutil.NewBlock(&msgBlock)
	h.request(t, *block.Hash())

	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	h.send(t, blockTxnsFor(block))
	waitDone(t, done)
	h.expectNotRelayed(t)
}

// shortenCmpctTxnsTimeout makes compact blocks wait briefly for missing
// transactions for the rest of the test.
func shortenCmpctTxnsTimeout(t *testing.T) {
	old := cmpctTxnsTimeout
	cmpctTxnsTimeout = 100 * time.Millisecond
	t.Cleanup(func() { cmpctTxnsTimeout = old })
}

// TestCompactBlockLateFullBlock ensures a full block that answers a
// getblocktxns after the compact block stopped waiting, as BCHN sends for a
// block that is not its most recent one, is still taken as requested, so the
// peer is not disconnected for it.
func TestCompactBlockLateFullBlock(t *testing.T) {
	shortenCmpctTxnsTimeout(t)
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())

	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	waitDone(t, done)
	h.send(t, block.MsgBlock())
	h.expectHaveBlock(t, block.Hash())
	if !h.sp.Connected() {
		t.Fatal("peer was disconnected for the late block")
	}
}

// TestCompactBlockLateBlockTxns ensures a compact block whose wait for
// transactions times out has its full block requested, keeping the request,
// and that a blocktxns arriving later is ignored, as BCHN ignores a blocktxn
// it does not expect.  The late blocktxns used to forget the request, and
// nothing asked for the block again.
func TestCompactBlockLateBlockTxns(t *testing.T) {
	shortenCmpctTxnsTimeout(t)
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())

	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	waitDone(t, done)
	h.expectFullBlockRequest(t, *block.Hash())
	h.send(t, blockTxnsFor(block))
	h.send(t, block.MsgBlock())
	h.expectHaveBlock(t, block.Hash())
	if !h.sp.Connected() {
		t.Fatal("peer was disconnected for the block it was asked for")
	}
}

// TestCompactBlockReplyAsWaitTimesOut ensures a blocktxns delivered just as
// the compact block's wait for it times out is used.  It was lost, with the
// request kept, so the block was not processed and the peer was eventually
// disconnected for not sending it.
func TestCompactBlockReplyAsWaitTimesOut(t *testing.T) {
	shortenCmpctTxnsTimeout(t)
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())

	cmpctTxnsTimeoutHook = func() {
		cmpctTxnsTimeoutHook = nil
		h.sp.deliverCmpctReply(*block.Hash(), blockTxnsFor(block))
	}
	t.Cleanup(func() { cmpctTxnsTimeoutHook = nil })
	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	waitDone(t, done)
	h.expectHaveBlock(t, block.Hash())
}

// TestCompactBlockTimeoutKnownBlock ensures a compact block whose wait for
// transactions times out after the block arrived otherwise does not have its
// full block requested.
func TestCompactBlockTimeoutKnownBlock(t *testing.T) {
	shortenCmpctTxnsTimeout(t)
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	h.request(t, *block.Hash())

	done := h.process(shortIDsOnly(t, block))
	h.expectGetTxns(t, *block.Hash())
	if _, _, err := h.s.chain.ProcessBlock(block, blockchain.BFNone); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)
	select {
	case iv := <-h.getData:
		t.Fatalf("getdata for %v", iv)
	case <-time.After(100 * time.Millisecond):
	}
}
