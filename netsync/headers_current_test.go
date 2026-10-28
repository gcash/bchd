// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"slices"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// chainOn returns count blocks that build on prev, the first at the passed
// height, which differ from others alike when extra does.
func (h *requestTestHarness) chainOn(prev *wire.MsgBlock, height int32,
	count int, extra int64) []*bchutil.Block {

	h.t.Helper()
	blocks := make([]*bchutil.Block, 0, count)
	for i := range count {
		block := mineBlock(h.t, h.params, prev, height+int32(i), extra)
		blocks = append(blocks, block)
		prev = block.MsgBlock()
	}
	return blocks
}

// headersOf returns the headers of the blocks.
func headersOf(blocks []*bchutil.Block) []*wire.BlockHeader {
	headers := make([]*wire.BlockHeader, 0, len(blocks))
	for _, block := range blocks {
		headers = append(headers, &block.MsgBlock().Header)
	}
	return headers
}

// extendTip processes count blocks on the tip and makes the last the tip.
func (h *requestTestHarness) extendTip(count int) {
	h.t.Helper()
	height := h.sm.chain.BestSnapshot().Height + 1
	for _, block := range h.chainOn(h.tip, height, count, 0) {
		_, _, err := h.sm.chain.ProcessBlock(block, blockchain.BFNone)
		if err != nil {
			h.t.Fatal(err)
		}
		h.tip = block.MsgBlock()
	}
}

// requested returns whether the block is requested from any peer.
func (h *requestTestHarness) requested(hash chainhash.Hash) bool {
	_, ok := h.sm.requestedBlocks[hash]
	return ok
}

// expectScores fails the test unless the ban score increases asked for are
// want.
func (h *requestTestHarness) expectScores(want ...string) {
	h.t.Helper()
	if got := h.scores(); !slices.Equal(got, want) {
		h.t.Fatalf("ban score increases %q, want %q", got, want)
	}
}

// TestAnnouncedBlockGetsHeaders ensures a block a peer announces while the
// chain is current is not requested, but the headers leading to it from the
// tip are, as BCHN asks for them (net_processing.cpp:2573-2582, 2615-2618).  A
// peer that announces a block already requested from another peer is kept to
// request it from.
func TestAnnouncedBlockGetsHeaders(t *testing.T) {
	h := newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	announced := chainhash.Hash{0xbb}

	h.inv(a, wire.InvTypeBlock, announced)
	if h.requested(announced) {
		t.Fatal("the announced block was requested")
	}
	msg := h.waitGetHeaders(a, announced)
	best := h.sm.chain.BestSnapshot()
	if len(msg.BlockLocatorHashes) == 0 ||
		*msg.BlockLocatorHashes[0] != best.Hash {

		t.Fatal("the headers were not asked for from the tip")
	}

	header := &h.tipBlock(0).MsgBlock().Header
	hash := header.BlockHash()
	h.announce(a, header)
	h.inv(b, wire.InvTypeBlock, hash)
	if !slices.Contains(h.sm.blockAnnouncers[hash], b) {
		t.Fatal("the peer announcing a requested block is not kept to " +
			"request it from")
	}
}

// TestSyncPeerAnnouncingTipKept ensures a sync peer that announces, with its
// header, a block another peer delivered first is kept once it has gone a
// while without sending a block.  As an inv does, its header raises its
// height to the block's, so it isn't taken for a sync peer that is behind
// this node.  Peers announce blocks with headers since bchd asks for them,
// and a sync peer was replaced after every block it wasn't first to deliver.
func TestSyncPeerAnnouncingTipKept(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	setStalledSyncPeer(h.sm, a)

	h.extendTip(1)
	h.announce(a, &h.tip.Header)
	best := h.sm.chain.BestSnapshot().Height
	if got := a.LastBlock(); got != best {
		t.Fatalf("sync peer's height %d after it announced the tip, "+
			"want %d", got, best)
	}
	h.sm.handleCheckSyncPeer()
	if h.sm.syncPeer != a || isDisconnected(a) {
		t.Fatal("the sync peer that announced the tip was replaced")
	}
}

// TestAnnouncedBlockKeptForHeight ensures the header of a block this node
// lacks makes the block the peer's last announced one, whose acceptance
// raises the peer's height, as with an inv, but leaves the height alone until
// then: a sync peer above this node's height makes the chain not current, and
// the block would not be fetched.
func TestAnnouncedBlockKeptForHeight(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	setStalledSyncPeer(h.sm, a)
	height := a.LastBlock()

	block := h.tipBlock(0)
	hash := *block.Hash()
	h.announce(a, &block.MsgBlock().Header)
	if got := a.LastAnnouncedBlock(); got == nil || *got != hash {
		t.Fatalf("last announced block %v, want %v", got, hash)
	}
	if got := a.LastBlock(); got != height {
		t.Fatalf("height %d before the block arrived, want %d", got,
			height)
	}
	if !h.requested(hash) {
		t.Fatal("the announced block was not requested")
	}
}

// TestAnnouncedLowWorkForkNotFetched ensures a block of a fork with less work
// than the tip that a peer announces is neither requested nor stored, even
// once the peer sends its header.
func TestAnnouncedLowWorkForkNotFetched(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	first := h.tip
	h.extendTip(2)
	fork := mineBlock(t, h.params, first, 2, 1)
	hash := *fork.Hash()

	h.inv(a, wire.InvTypeBlock, hash)
	if h.requested(hash) {
		t.Fatal("the announced block of a fork with less work was " +
			"requested")
	}
	h.waitGetHeaders(a, hash)
	h.announce(a, &fork.MsgBlock().Header)
	if h.requested(hash) {
		t.Fatal("the block of a fork with less work was requested once " +
			"its header arrived")
	}
	if known, _, _ := h.sm.chain.BlockValidity(&hash); known {
		t.Fatal("the block of a fork with less work was stored")
	}
	if isDisconnected(a) {
		t.Fatal("the peer was disconnected for the header")
	}
}

// TestAnnouncedChainFetched ensures the blocks of a chain of headers with more
// work than the tip are requested from the peer that sent the headers, in
// full, earliest first, and as blocks whose headers were validated, as BCHN's
// direct fetch requests them (net_processing.cpp:1765-1826).
func TestAnnouncedChainFetched(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	blocks := h.chainOn(h.params.GenesisBlock, 1, 3, 1)

	h.announce(a, headersOf(blocks)...)
	last := *blocks[len(blocks)-1].Hash()
	h.waitGetData(a, wire.InvTypeBlock, last)
	h.mtx.Lock()
	got := slices.Clone(h.got[a])
	h.mtx.Unlock()
	if len(got) != len(blocks) {
		t.Fatalf("%d blocks requested, want %d", len(got), len(blocks))
	}
	for i, iv := range got {
		if iv.Type != wire.InvTypeBlock || iv.Hash != *blocks[i].Hash() {
			t.Fatalf("request %d is for %v, want block %v", i, iv,
				blocks[i].Hash())
		}
		if _, ok := h.sm.validBlockRequests[iv.Hash]; !ok {
			t.Fatalf("block %v is not requested with a validated header",
				iv.Hash)
		}
	}
}

// TestAnnouncedBlockOnTipCompact ensures a single block on the tip, with no
// other block requested, is requested as a compact block, and one that does not
// build on the tip in full, as BCHN does (net_processing.cpp:1814-1820).
func TestAnnouncedBlockOnTipCompact(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	onTip := h.tipBlock(0)
	h.announce(a, &onTip.MsgBlock().Header)
	h.waitGetData(a, wire.InvTypeCmpctBlock, *onTip.Hash())

	h = newRequestTestHarness(t)
	a, b := h.peer(1), h.peer(2)
	sibling := mineBlock(t, h.params, h.params.GenesisBlock, 1, 1)
	h.announce(a, &sibling.MsgBlock().Header)
	h.waitGetData(a, wire.InvTypeBlock, *sibling.Hash())

	// Another block on the tip, while one is requested, is requested in
	// full too.
	onTip = h.tipBlock(0)
	h.announce(b, &onTip.MsgBlock().Header)
	h.waitGetData(b, wire.InvTypeBlock, *onTip.Hash())

	// So are the two blocks of a chain ending above the tip.
	h = newRequestTestHarness(t)
	a = h.peer(1)
	branch := h.chainOn(h.params.GenesisBlock, 1, 2, 2)
	h.announce(a, headersOf(branch)...)
	h.waitGetData(a, wire.InvTypeBlock, *branch[1].Hash())
	h.mtx.Lock()
	defer h.mtx.Unlock()
	for _, iv := range h.got[a] {
		if iv.Type != wire.InvTypeBlock {
			t.Fatalf("%v requested as %v", iv.Hash, iv.Type)
		}
	}
}

// TestHeaderChainFetchContinues ensures at most 16 blocks of a chain of headers
// are requested from a peer at a time, as BCHN requests at most
// MAX_BLOCKS_IN_TRANSIT_PER_PEER, and that the next is requested as one
// arrives.
func TestHeaderChainFetchContinues(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	blocks := h.chainOn(h.tip, 2, 20, 0)

	h.announce(a, headersOf(blocks)...)
	h.waitGetData(a, wire.InvTypeBlock, *blocks[15].Hash())
	if n := h.sm.peerStates[a].blocksInFlight; n != 16 {
		t.Fatalf("%d blocks requested at once, want 16", n)
	}
	if h.requested(*blocks[16].Hash()) {
		t.Fatal("a seventeenth block was requested")
	}

	h.sm.handleBlockMsg(&blockMsg{block: blocks[0], peer: a})
	h.waitGetData(a, wire.InvTypeBlock, *blocks[16].Hash())

	// Once the chain is not current, no more blocks are requested.
	syncPeer := h.peer(2)
	syncPeer.UpdateLastBlockHeight(100)
	h.sm.peerStates[syncPeer].syncCandidate = true
	h.sm.syncPeer = syncPeer
	h.sm.handleBlockMsg(&blockMsg{block: blocks[1], peer: a})
	if h.requested(*blocks[17].Hash()) {
		t.Fatal("a block was requested while the chain is not current")
	}
}

// unknownParentHeader returns a header whose parent is unknown, which differs
// for each n.
func (h *requestTestHarness) unknownParentHeader(n byte) *wire.BlockHeader {
	h.t.Helper()
	return mineHeaders(h.t, h.params, chainhash.Hash{0x55, n}, 1)[0]
}

// TestUnconnectedHeaders ensures a peer that sends fewer than 8 headers whose
// first one's parent is unknown is asked for the headers before them, and that
// every tenth such message in a row costs it 20, while a message whose headers
// connect starts the count again, as in BCHN (net_processing.cpp:1633-1658).
// Eight headers that don't connect cost it 10, BCHN's prev-blk-not-found.
func TestUnconnectedHeaders(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)

	for n := byte(1); n <= 9; n++ {
		h.announce(a, h.unknownParentHeader(n))
	}
	h.waitGetHeaders(a, chainhash.Hash{})
	h.expectScores()
	h.announce(a, h.unknownParentHeader(10))
	h.expectScores("20 too-many-unconnected-headers")

	// Headers that connect start the count again, after a run that cost
	// nothing too.
	h.announce(a, &h.tipBlock(0).MsgBlock().Header)
	for n := byte(11); n <= 13; n++ {
		h.announce(a, h.unknownParentHeader(n))
	}
	h.announce(a, &h.tipBlock(0).MsgBlock().Header)
	for n := byte(14); n <= 22; n++ {
		h.announce(a, h.unknownParentHeader(n))
	}
	h.expectScores("20 too-many-unconnected-headers")
	h.announce(a, h.unknownParentHeader(23))
	h.expectScores("20 too-many-unconnected-headers",
		"20 too-many-unconnected-headers")

	h.announce(a, mineHeaders(t, h.params, chainhash.Hash{0x66}, 8)...)
	h.expectScores("20 too-many-unconnected-headers",
		"20 too-many-unconnected-headers", "10 prev-blk-not-found")
	if isDisconnected(a) {
		t.Fatal("the peer was disconnected for its headers")
	}
}

// TestDisconnectedHeaders ensures a peer whose headers message has headers that
// don't build on the one before is punished with BCHN's 20, and none of the
// headers is processed.
func TestDisconnectedHeaders(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	header := &h.tipBlock(0).MsgBlock().Header
	h.announce(a, header, h.unknownParentHeader(1))
	h.expectScores("20 disconnected-header")
	if h.requested(header.BlockHash()) {
		t.Fatal("the block of a header in the message was requested")
	}
}

// TestInvalidHeaderScores ensures the peer that sent an invalid header is
// punished with BCHN's score for the rule it breaks, and not for a header too
// far in the future.
func TestInvalidHeaderScores(t *testing.T) {
	tests := []struct {
		name   string
		change func(h *requestTestHarness, header *wire.BlockHeader)
		want   []string
	}{{
		name: "hash above the target",
		change: func(h *requestTestHarness, header *wire.BlockHeader) {
			for blockchain.CheckHeaderProofOfWork(header,
				h.params.PowLimit) == nil {

				header.Nonce++
			}
		},
		want: []string{"50 high-hash"},
	}, {
		name: "target above the limit",
		change: func(h *requestTestHarness, header *wire.BlockHeader) {
			header.Bits = 0x2100ffff
		},
		want: []string{"50 high-hash"},
	}, {
		name: "wrong difficulty",
		change: func(h *requestTestHarness, header *wire.BlockHeader) {
			header.Bits = 0x1f7fffff
			resolve(h, header)
		},
		want: []string{"100 bad-diffbits"},
	}, {
		name: "below the finalized block",
		change: func(h *requestTestHarness, header *wire.BlockHeader) {
			if err := h.sm.chain.FinalizeBlock(
				ptrHash(h.tip.BlockHash())); err != nil {

				h.t.Fatal(err)
			}
			header.PrevBlock = *h.params.GenesisHash
			resolve(h, header)
		},
		want: []string{"100 bad-header-finalization"},
	}, {
		name: "too far in the future",
		change: func(h *requestTestHarness, header *wire.BlockHeader) {
			header.Timestamp = time.Unix(time.Now().Add(3*time.Hour).
				Unix(), 0)
			resolve(h, header)
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newRequestTestHarness(t)
			a := h.peer(1)
			header := h.tipBlock(0).MsgBlock().Header
			test.change(h, &header)
			h.announce(a, &header)
			h.expectScores(test.want...)
			if h.requested(header.BlockHash()) {
				t.Fatal("the block of the invalid header was requested")
			}
		})
	}
}

// resolve finds a nonce that gives the header valid proof of work.
func resolve(h *requestTestHarness, header *wire.BlockHeader) {
	h.t.Helper()
	header.Nonce = 0
	for blockchain.CheckHeaderProofOfWork(header, h.params.PowLimit) != nil {
		header.Nonce++
	}
}

// ptrHash returns a pointer to a copy of hash.
func ptrHash(hash chainhash.Hash) *chainhash.Hash {
	return &hash
}

// TestHeadersWhenNotCurrent ensures that, while the chain is not current, the
// sync peer's headers announcing blocks are followed as its invs are, and
// another peer's are ignored, and that neither peer is disconnected for them.
func TestHeadersWhenNotCurrent(t *testing.T) {
	h := newRequestTestHarness(t)
	syncPeer, other := h.peer(1), h.peer(2)
	syncPeer.UpdateLastBlockHeight(10)
	h.sm.peerStates[syncPeer].syncCandidate = true
	h.sm.syncPeer = syncPeer
	if h.sm.current() {
		t.Fatal("the sync manager is current")
	}

	header := &h.tipBlock(0).MsgBlock().Header
	hash := header.BlockHash()
	h.announce(other, header)
	if h.requested(hash) {
		t.Fatal("the block another peer announced was requested")
	}
	h.announce(syncPeer, header)
	h.waitGetData(syncPeer, wire.InvTypeBlock, hash)
	if isDisconnected(syncPeer) || isDisconnected(other) {
		t.Fatal("a peer was disconnected for announcing a block")
	}
}

// TestOrphanParentsByHeaders ensures that, while the chain is current, the
// headers leading to an orphan a peer sends are asked for, so its chain is
// fetched once it has enough work, rather than its blocks with getblocks.
func TestOrphanParentsByHeaders(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	a.SetAllowDirectBlockRelay(true)
	orphan := h.chainOn(h.tip, 2, 2, 0)[1]

	h.sm.handleBlockMsg(&blockMsg{block: orphan, peer: a})
	if !h.sm.chain.IsKnownOrphan(orphan.Hash()) {
		t.Fatal("the block is not an orphan")
	}
	h.waitGetHeaders(a, *orphan.Hash())
	h.mtx.Lock()
	gotBlocks := len(h.gotBlocks[a])
	h.mtx.Unlock()
	if gotBlocks != 0 {
		t.Fatal("the orphan's parents were asked for with getblocks")
	}

	// With the headers, the parent is requested, but not the orphan.
	parent := h.chainOn(h.tip, 2, 2, 0)[0]
	h.announce(a, &parent.MsgBlock().Header, &orphan.MsgBlock().Header)
	h.waitGetData(a, wire.InvTypeBlock, *parent.Hash())
	if h.requested(*orphan.Hash()) {
		t.Fatal("the orphan was requested")
	}

	// An announced orphan has the headers leading to it requested too.
	other := h.chainOn(h.tip, 2, 2, 1)[1]
	if _, isOrphan, err := h.sm.chain.ProcessBlock(other,
		blockchain.BFNone); err != nil || !isOrphan {

		t.Fatalf("the block is not an orphan: %v", err)
	}
	h.inv(a, wire.InvTypeBlock, *other.Hash())
	h.waitGetHeaders(a, *other.Hash())
}

// barrier waits for a getdata queued to the peer now to reach it, so the
// messages queued to it before have too.
func (h *requestTestHarness) barrier(p *peerpkg.Peer) {
	h.t.Helper()
	hash := chainhash.Hash{0xfe}
	msg := wire.NewMsgGetData()
	if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeTx, &hash)); err != nil {
		h.t.Fatal(err)
	}
	p.QueueMessage(msg, nil)
	h.waitGetData(p, wire.InvTypeTx, hash)
}

// headersAsked returns how many getheaders the peer got.
func (h *requestTestHarness) headersAsked(p *peerpkg.Peer) int {
	h.mtx.Lock()
	defer h.mtx.Unlock()
	return len(h.gotHeaders[p])
}

// TestNewPeerGetsHeaders ensures a full node peer that connects while the chain
// is current is asked for the headers after the tip, so that a chain of its
// with more work is fetched, as BCHN asks every peer once its best header is
// recent (net_processing.cpp:4259-4290), and that a peer that is no full
// node, or one that connects while the chain is not current, is not.
func TestNewPeerGetsHeaders(t *testing.T) {
	h := newRequestTestHarness(t)
	p := h.peer(1)
	delete(h.sm.peerStates, p)
	h.sm.handleNewPeerMsg(p)
	if !h.sm.peerStates[p].syncCandidate {
		t.Fatal("the peer is not a full node")
	}
	msg := h.waitGetHeaders(p, chainhash.Hash{})
	best := h.sm.chain.BestSnapshot()
	if *msg.BlockLocatorHashes[0] != best.Hash {
		t.Fatal("the headers were not asked for from the tip")
	}

	light := h.peerWithServices(2, 0)
	delete(h.sm.peerStates, light)
	h.sm.handleNewPeerMsg(light)
	h.barrier(light)
	if h.headersAsked(light) != 0 {
		t.Fatal("a peer that is no full node was asked for headers")
	}

	p.UpdateLastBlockHeight(10)
	h.sm.peerStates[p].syncCandidate = true
	h.sm.syncPeer = p
	late := h.peer(3)
	delete(h.sm.peerStates, late)
	h.sm.handleNewPeerMsg(late)
	h.barrier(late)
	if h.headersAsked(late) != 0 {
		t.Fatal("a peer was asked for headers while the chain is not " +
			"current")
	}
}

// TestFullHeadersMessageContinues ensures a peer that sends a full headers
// message is asked for the headers after the last of them, as BCHN asks
// (net_processing.cpp:1749-1761).
func TestFullHeadersMessageContinues(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	headers := headersOf(h.chainOn(h.params.GenesisBlock, 1,
		wire.MaxBlockHeadersPerMsg, 3))
	h.announce(a, headers...)
	msg := h.waitGetHeaders(a, chainhash.Hash{})
	last := headers[len(headers)-1].BlockHash()
	if len(msg.BlockLocatorHashes) != 1 || *msg.BlockLocatorHashes[0] != last {
		t.Fatal("the headers after the last were not asked for")
	}
}

// TestHeaderBanScore ensures each rule a header breaks gets BCHN's score and
// reason.
func TestHeaderBanScore(t *testing.T) {
	params := chaincfg.RegressionNetParams
	inRange := mineHeaders(t, &params, chainhash.Hash{}, 1)[0]
	outOfRange := *inRange
	outOfRange.Bits = 0x2100ffff
	tests := []struct {
		code   blockchain.ErrorCode
		header *wire.BlockHeader
		score  uint32
		reason string
	}{
		{blockchain.ErrHighHash, inRange, 50, "high-hash"},
		{blockchain.ErrUnexpectedDifficulty, &outOfRange, 50, "high-hash"},
		{blockchain.ErrUnexpectedDifficulty, inRange, 100, "bad-diffbits"},
		{blockchain.ErrBadCheckpoint, inRange, 100, "checkpoint mismatch"},
		{blockchain.ErrForkTooOld, inRange, 100,
			"bad-fork-prior-to-checkpoint"},
		{blockchain.ErrForkBeforeFinalized, inRange, 100,
			"bad-header-finalization"},
		{blockchain.ErrInvalidAncestorBlock, inRange, 100, "bad-prevblk"},
		{blockchain.ErrPreviousBlockUnknown, inRange, 10,
			"prev-blk-not-found"},
		{blockchain.ErrTimeTooNew, inRange, 0, ""},
		{blockchain.ErrTimeTooOld, inRange, 0, ""},
		{blockchain.ErrBlockVersionTooOld, inRange, 0, ""},
		{blockchain.ErrDuplicateBlock, inRange, 0, ""},
	}
	for _, test := range tests {
		score, reason := HeaderBanScore(test.code, test.header,
			params.PowLimit)
		if score != test.score || reason != test.reason {
			t.Errorf("%v: got %d %q, want %d %q", test.code, score,
				reason, test.score, test.reason)
		}
	}
}

// TestCmpctHeaders ensures the header of a compact block that is not
// reconstructed costs the peer nothing when it doesn't connect, but has the
// headers before it requested without counting against the peer, costs it
// BCHN's score when it is invalid, has its block fetched in full if it was
// requested from the peer, and has its chain fetched only when that has more
// work than the tip, as BCHN does (net_processing.cpp:3039-3075, 3115-3129).
func TestCmpctHeaders(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	cmpct := func(header *wire.BlockHeader) {
		t.Helper()
		h.sm.handleHeadersMsg(&headersMsg{
			headers: headersMessage(t, []*wire.BlockHeader{header}),
			peer:    a,
			cmpct:   true,
		})
	}

	for n := byte(1); n <= maxUnconnectingHeaders; n++ {
		cmpct(h.unknownParentHeader(n))
	}
	h.waitGetHeaders(a, chainhash.Hash{})
	if n := h.sm.peerStates[a].unconnectedHeaders; n != 0 {
		t.Fatalf("%d compact blocks counted as unconnected headers", n)
	}
	invalid := h.tipBlock(0).MsgBlock().Header
	invalid.Bits = 0x2100ffff
	cmpct(&invalid)
	h.expectScores("50 high-hash")

	sibling := mineBlock(t, h.params, h.params.GenesisBlock, 1, 1)
	cmpct(&sibling.MsgBlock().Header)
	if h.requested(*sibling.Hash()) {
		t.Fatal("the block of a compact block with no more work than " +
			"the tip was requested")
	}
	h.sm.requestBlock(a, h.sm.peerStates[a], sibling.Hash(), true)
	cmpct(&sibling.MsgBlock().Header)
	h.waitGetData(a, wire.InvTypeBlock, *sibling.Hash())

	onTip := h.tipBlock(0)
	cmpct(&onTip.MsgBlock().Header)
	h.waitGetData(a, wire.InvTypeBlock, *onTip.Hash())
}

// TestBestHeaderFollowed ensures the blocks of the chain of a peer's best
// header, the last it sent with the most work as in BCHN, are requested as
// blocks arrive, here those of the second of two chains with the same work.
func TestBestHeaderFollowed(t *testing.T) {
	h := newRequestTestHarness(t)
	a := h.peer(1)
	blocks := h.chainOn(h.tip, 2, 20, 0)
	h.announce(a, headersOf(blocks)...)
	last := mineBlock(t, h.params, blocks[18].MsgBlock(), 20, 1)
	h.announce(a, &last.MsgBlock().Header)
	for _, block := range blocks[:4] {
		h.sm.handleBlockMsg(&blockMsg{block: block, peer: a})
	}
	h.waitGetData(a, wire.InvTypeBlock, *last.Hash())
	if h.requested(*blocks[19].Hash()) {
		t.Fatal("the block of the first chain was requested")
	}
}
