// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"maps"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// headerTestChain returns a fake chain on a copy of the regression test
// parameters whose main chain has count blocks after the genesis block, ten
// minutes apart, and the nodes of its main chain, the genesis block's first.
func headerTestChain(t *testing.T, count int) (*BlockChain, []*blockNode) {
	t.Helper()
	params := chaincfg.RegressionNetParams
	chain := newFakeChain(&params)
	nodes := []*blockNode{chain.bestChain.Tip()}
	for range count {
		prev := nodes[len(nodes)-1]
		node := newFakeNode(prev, 4, params.PowLimitBits,
			time.Unix(prev.timestamp, 0).Add(10*time.Minute))
		node.status = statusDataStored | statusValid
		chain.index.AddNode(node)
		nodes = append(nodes, node)
	}
	chain.bestChain.SetTip(nodes[len(nodes)-1])
	return chain, nodes
}

// testHeader returns a header on prev with the passed timestamp and bits and
// valid proof of work.  Headers otherwise alike differ when n does.
func testHeader(prev chainhash.Hash, ts time.Time, bits uint32, n byte) *wire.BlockHeader {
	header := &wire.BlockHeader{
		Version:    4,
		PrevBlock:  prev,
		MerkleRoot: chainhash.Hash{n},
		Timestamp:  ts,
		Bits:       bits,
	}
	solveBlock(header)
	return header
}

// after returns the time d after the timestamp of node.
func after(node *blockNode, d time.Duration) time.Time {
	return time.Unix(node.timestamp, 0).Add(d)
}

// headerAfter returns the time d after the timestamp of header.
func headerAfter(header *wire.BlockHeader, d time.Duration) time.Time {
	return header.Timestamp.Add(d)
}

// hashOf returns the hash of header.
func hashOf(header *wire.BlockHeader) chainhash.Hash {
	return header.BlockHash()
}

// isKept returns whether the chain keeps the header in its header tree.
func isKept(chain *BlockChain, header *wire.BlockHeader) bool {
	hash := header.BlockHash()
	return chain.headers.lookup(&hash) != nil
}

// checkHeaderTree fails the test unless the header tree is consistent: each
// header's parent is a kept header or a block, each header counts the kept
// headers built on it and is among its parent's children, it is in the heaps
// of leaves exactly when nothing builds on it, and each peer's count is its
// number of headers.
func checkHeaderTree(t *testing.T, chain *BlockChain) {
	t.Helper()
	tree := chain.headers
	children := make(map[chainhash.Hash]int)
	peers := make(map[int32]int)
	linked := 0
	for _, set := range tree.children {
		linked += len(set)
	}
	if linked != len(tree.entries) {
		t.Fatalf("%d headers are linked to parents, want %d", linked,
			len(tree.entries))
	}
	for hash, e := range tree.entries {
		parent := e.node.parent
		if tree.entries[parent.hash] != nil {
			children[parent.hash]++
		} else if chain.index.LookupNode(&parent.hash) != parent {
			t.Fatalf("the parent of header %v is neither kept nor a "+
				"block", hash)
		}
		if _, ok := tree.children[parent.hash][e]; !ok {
			t.Fatalf("header %v is not among its parent's children", hash)
		}
		peers[e.peer]++
	}
	leaves := make(map[int32]int)
	for hash, e := range tree.entries {
		if e.children != children[hash] {
			t.Fatalf("header %v counts %d children, has %d", hash,
				e.children, children[hash])
		}
		leaf := e.children == 0
		if (e.heapIdx >= 0) != leaf || (e.peerHeapIdx >= 0) != leaf {
			t.Fatalf("header %v with %d children is in the heaps at %d "+
				"and %d", hash, e.children, e.heapIdx, e.peerHeapIdx)
		}
		if !leaf {
			continue
		}
		leaves[e.peer]++
		if tree.leaves.entries[e.heapIdx] != e ||
			tree.peerLeaves[e.peer].entries[e.peerHeapIdx] != e {

			t.Fatalf("header %v is not where the heaps say", hash)
		}
	}
	total := 0
	for peer, n := range leaves {
		total += n
		if tree.peerLeaves[peer].Len() != n {
			t.Fatalf("peer %d's heap has %d leaves, want %d", peer,
				tree.peerLeaves[peer].Len(), n)
		}
	}
	if tree.leaves.Len() != total {
		t.Fatalf("the heap has %d leaves, want %d", tree.leaves.Len(), total)
	}
	if !maps.Equal(peers, tree.peerCount) {
		t.Fatalf("peer counts %v, want %v", tree.peerCount, peers)
	}
}

// processHeaders processes the headers as sent by the peer, and fails the
// test on an error.
func processHeaders(t *testing.T, chain *BlockChain, peer int32,
	headers ...*wire.BlockHeader) *HeaderState {

	t.Helper()
	state, _, err := chain.ProcessBlockHeaders(headers, peer)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// expectHeaderError fails the test unless processing the headers fails at the
// header with the passed index with a rule error with the passed code.
func expectHeaderError(t *testing.T, chain *BlockChain, name string,
	want ErrorCode, wantIndex int, headers ...*wire.BlockHeader) {

	t.Helper()
	state, index, err := chain.ProcessBlockHeaders(headers, 1)
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != want ||
		index != wantIndex || state != nil {

		t.Fatalf("%s: got %v (%v) at index %d, want %v at index %d",
			name, err, state, index, want, wantIndex)
	}
	hash := headers[wantIndex].BlockHash()
	if chain.headers.lookup(&hash) != nil {
		t.Fatalf("%s: the invalid header was kept", name)
	}
}

// TestProcessBlockHeaders ensures valid headers that build on a block or on
// a kept header are kept, until their blocks arrive, and that a known header
// is not processed again.
func TestProcessBlockHeaders(t *testing.T) {
	chain, nodes := headerTestChain(t, 2)
	bits := chain.chainParams.PowLimitBits
	tip := nodes[2]

	h3 := testHeader(tip.hash, after(tip, 10*time.Minute), bits, 0)
	h4 := testHeader(hashOf(h3), headerAfter(h3, 10*time.Minute), bits, 0)
	h5 := testHeader(hashOf(h4), headerAfter(h4, 10*time.Minute), bits, 0)
	state := processHeaders(t, chain, 1, h3, h4, h5)
	if state.Hash != hashOf(h5) || state.Height != 5 || state.WorkCmp != 1 {
		t.Fatalf("got state %+v for the last of three headers on the tip",
			state)
	}
	want := new(big.Int).Add(tip.workSum, new(big.Int).Mul(big.NewInt(3),
		CalcWork(bits)))
	if state.Work.Cmp(want) != 0 {
		t.Fatalf("got work %v, want %v", state.Work, want)
	}
	for _, header := range []*wire.BlockHeader{h3, h4, h5} {
		hash := header.BlockHash()
		if !isKept(chain, header) || chain.index.LookupNode(&hash) != nil {
			t.Fatalf("header %v is not kept apart from the block index",
				hash)
		}
	}

	// Known headers are not added again, and the state of a header of the
	// main chain compares its work with the tip's.
	size := len(chain.headers.entries)
	got := processHeaders(t, chain, 2, h3, h4, h5)
	if got.Hash != state.Hash || got.Height != state.Height ||
		got.WorkCmp != 1 || got.Work.Cmp(state.Work) != 0 {

		t.Fatalf("got state %+v for known headers, want %+v", got, state)
	}
	if len(chain.headers.entries) != size || chain.headers.peerCount[2] != 0 {
		t.Fatal("known headers were added again")
	}
	tipHeader := tip.Header()
	if got := processHeaders(t, chain, 1, &tipHeader); got.Hash != tip.hash ||
		got.Height != 2 || got.WorkCmp != 0 {

		t.Fatalf("got state %+v for the tip", got)
	}
	parentHeader := nodes[1].Header()
	if got := processHeaders(t, chain, 1, &parentHeader); got.WorkCmp != -1 {
		t.Fatalf("got state %+v for the tip's parent", got)
	}

	// The headers before the first invalid one in a message are kept.
	h6 := testHeader(hashOf(h5), headerAfter(h5, 10*time.Minute), bits, 0)
	bad := testHeader(hashOf(h6), headerAfter(h6, 10*time.Minute), bits, 0)
	for CheckHeaderProofOfWork(bad, chain.chainParams.PowLimit) == nil {
		bad.Nonce++
	}
	expectHeaderError(t, chain, "high hash after a valid header",
		ErrHighHash, 1, h6, bad)
	if !isKept(chain, h6) {
		t.Fatal("the valid header before an invalid one was not kept")
	}
}

// TestProcessBlockHeadersRules ensures a header that breaks a rule BCHN checks
// for a header fails with the rule's error and is not kept.
func TestProcessBlockHeadersRules(t *testing.T) {
	chain, nodes := headerTestChain(t, 2)
	params := chain.chainParams
	bits := params.PowLimitBits
	tip := nodes[2]
	next := after(tip, 10*time.Minute)

	highHash := testHeader(tip.hash, next, bits, 1)
	for CheckHeaderProofOfWork(highHash, params.PowLimit) == nil {
		highHash.Nonce++
	}
	overLimit := testHeader(tip.hash, next, bits, 2)
	overLimit.Bits = 0x2100ffff
	tooOld := testHeader(tip.hash, tip.CalcPastMedianTime(), bits, 3)
	tooNew := testHeader(tip.hash,
		time.Unix(time.Now().Add(3*time.Hour).Unix(), 0), bits, 4)
	tests := []struct {
		name   string
		header *wire.BlockHeader
		want   ErrorCode
	}{
		{"unknown parent", testHeader(chainhash.Hash{1}, next, bits, 0),
			ErrPreviousBlockUnknown},
		{"high hash", highHash, ErrHighHash},
		{"target above the limit", overLimit, ErrUnexpectedDifficulty},
		{"wrong difficulty", testHeader(tip.hash, next, 0x1f7fffff, 5),
			ErrUnexpectedDifficulty},
		{"time too old", tooOld, ErrTimeTooOld},
		{"time too new", tooNew, ErrTimeTooNew},
	}
	for _, test := range tests {
		expectHeaderError(t, chain, test.name, test.want, 0, test.header)
	}

	params.BIP0034Height = 1
	old := testHeader(tip.hash, next, bits, 6)
	old.Version = 1
	solveBlock(old)
	expectHeaderError(t, chain, "old version", ErrBlockVersionTooOld, 0, old)
	params.BIP0034Height = chaincfg.RegressionNetParams.BIP0034Height

	// Below the finalized block.
	chain.finalizedNode = tip
	expectHeaderError(t, chain, "fork below the finalized block",
		ErrForkBeforeFinalized, 0,
		testHeader(nodes[1].hash, after(nodes[1], 11*time.Minute), bits, 7))
	chain.finalizedNode = nil

	// Against checkpoints, of which the chain has reached the one at
	// height 2.
	cp := chaincfg.Checkpoint{Height: 2, Hash: &tip.hash}
	chain.checkpoints = []chaincfg.Checkpoint{cp}
	chain.checkpointsByHeight = map[int32]*chaincfg.Checkpoint{2: &cp}
	expectHeaderError(t, chain, "checkpoint mismatch", ErrBadCheckpoint, 0,
		testHeader(nodes[1].hash, after(nodes[1], 11*time.Minute), bits, 8))
	expectHeaderError(t, chain, "fork before the checkpoint", ErrForkTooOld, 0,
		testHeader(nodes[0].hash, after(nodes[0], 11*time.Minute), bits, 9))
}

// TestProcessBlockHeadersInvalidChain ensures a header for a block known to be
// invalid fails, and one on such a block, or on a kept header whose chain has
// one, fails with an invalid ancestor.
func TestProcessBlockHeadersInvalidChain(t *testing.T) {
	chain, nodes := headerTestChain(t, 2)
	bits := chain.chainParams.PowLimitBits
	side := newFakeNode(nodes[1], 4, bits, after(nodes[1], 11*time.Minute))
	side.status = statusDataStored
	chain.index.AddNode(side)
	kept := testHeader(side.hash, after(side, 10*time.Minute), bits, 0)
	processHeaders(t, chain, 1, kept)

	chain.index.SetStatusFlags(side, statusValidateFailed)
	sideHeader := side.Header()
	expectHeaderError(t, chain, "invalid block", ErrDuplicateBlock, 0,
		&sideHeader)
	expectHeaderError(t, chain, "on an invalid block",
		ErrInvalidAncestorBlock, 0,
		testHeader(side.hash, after(side, 10*time.Minute), bits, 1))
	onKept := testHeader(hashOf(kept), headerAfter(kept, 10*time.Minute),
		bits, 0)
	expectHeaderError(t, chain, "on a kept header after an invalid block",
		ErrInvalidAncestorBlock, 0, onKept)
	expectHeaderError(t, chain, "after the kept header in the message",
		ErrInvalidAncestorBlock, 1, kept, onKept)
}

// TestProcessBlockHeadersDifficulty ensures a header is checked for the
// difficulty ASERT gives it after its parent when the parent is a kept header,
// including the minimum difficulty of a block 20 minutes after its parent.
func TestProcessBlockHeadersDifficulty(t *testing.T) {
	chain, nodes := headerTestChain(t, 2)
	params := chain.chainParams
	params.NoDifficultyAdjustment = false
	params.AsertDifficultyAnchorBits = 0x1f7fffff
	tip := nodes[2]

	// header returns a header on prev, a node of the chain, at ts, with
	// the bits ASERT gives it.
	header := func(prev *blockNode, ts time.Time, n byte) *wire.BlockHeader {
		t.Helper()
		bits, err := chain.calcNextRequiredDifficulty(prev, ts,
			DifficultyAsert)
		if err != nil {
			t.Fatal(err)
		}
		if bits == params.PowLimitBits {
			t.Fatalf("ASERT gives the minimum difficulty at %v", ts)
		}
		return testHeader(prev.hash, ts, bits, n)
	}
	h3 := header(tip, after(tip, 10*time.Minute), 0)
	processHeaders(t, chain, 1, h3)
	n3 := chain.headers.lookup(ptr(hashOf(h3)))
	h4 := header(n3, headerAfter(h3, 10*time.Minute), 0)
	processHeaders(t, chain, 1, h4)
	n4 := chain.headers.lookup(ptr(hashOf(h4)))

	early := testHeader(n4.hash, headerAfter(h4, 19*time.Minute),
		params.PowLimitBits, 1)
	expectHeaderError(t, chain, "minimum difficulty 19 minutes after the "+
		"parent", ErrUnexpectedDifficulty, 0, early)
	asert := header(n4, headerAfter(h4, 19*time.Minute), 2).Bits
	expectHeaderError(t, chain, "ASERT difficulty 21 minutes after the "+
		"parent", ErrUnexpectedDifficulty, 0,
		testHeader(n4.hash, headerAfter(h4, 21*time.Minute), asert, 2))
	late := testHeader(n4.hash, headerAfter(h4, 21*time.Minute),
		params.PowLimitBits, 3)
	processHeaders(t, chain, 1, late)
}

// ptr returns a pointer to a copy of hash.
func ptr(hash chainhash.Hash) *chainhash.Hash {
	return &hash
}

// TestHeaderTreeNodeMovesToIndex ensures a block whose header is kept gets the
// kept header's node, which the kept headers built on it have as their parent,
// so they are still found to descend from the block once it is finalized.
func TestHeaderTreeNodeMovesToIndex(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Hour)
	params := chain.chainParams

	prev := blocks[len(blocks)-1]
	var next []*bchutil.Block
	var headers []*wire.BlockHeader
	for height := int32(6); height <= 8; height++ {
		block := procTestBlock(t, prev.MsgBlock(), height-1,
			CalcBlockSubsidy(height, params), params.PowLimitBits, nil)
		next = append(next, block)
		headers = append(headers, &block.MsgBlock().Header)
		prev = block
	}
	processHeaders(t, chain, 1, headers...)
	node := chain.headers.lookup(next[0].Hash())

	if _, _, err := chain.ProcessBlock(next[0], BFNone); err != nil {
		t.Fatal(err)
	}
	if chain.index.LookupNode(next[0].Hash()) != node {
		t.Fatal("the block did not get the node of its kept header")
	}
	if isKept(chain, headers[0]) || !isKept(chain, headers[1]) {
		t.Fatal("the header tree did not let go of the block's node alone")
	}
	if !chain.index.NodeStatus(node).HaveData() {
		t.Fatal("the block's node has no data")
	}
	checkHeaderTree(t, chain)

	if err := chain.FinalizeBlock(next[0].Hash()); err != nil {
		t.Fatal(err)
	}
	last := next[len(next)-1].MsgBlock()
	processHeaders(t, chain, 1, testHeader(last.BlockHash(),
		last.Header.Timestamp.Add(time.Minute), params.PowLimitBits, 0))
}

// TestHeaderTreeCaps ensures the header tree keeps at most its number of
// headers, and of headers from one peer, by dropping the header with the least
// work that has no header built on it, the first added of those with the same
// work, but neither a header on a chain with at least the tip's work nor the
// parent of the header being added.
func TestHeaderTreeCaps(t *testing.T) {
	chain, nodes := headerTestChain(t, 4)
	chain.headers.maxNodes = 4
	chain.headers.maxPeerNodes = 2
	bits := chain.chainParams.PowLimitBits
	genesis := nodes[0]
	at := after(genesis, 10*time.Minute)

	s1 := testHeader(genesis.hash, at, bits, 1)
	s2 := testHeader(genesis.hash, at, bits, 2)
	s3 := testHeader(genesis.hash, at, bits, 3)
	processHeaders(t, chain, 1, s1, s2)
	processHeaders(t, chain, 1, s3)
	if isKept(chain, s1) || !isKept(chain, s2) || !isKept(chain, s3) {
		t.Fatal("the peer's third header did not drop its first")
	}

	// The parent of the header being added is not dropped for it.
	c3 := testHeader(hashOf(s3), headerAfter(s3, 10*time.Minute), bits, 0)
	processHeaders(t, chain, 1, c3)
	if isKept(chain, s2) || !isKept(chain, s3) || !isKept(chain, c3) ||
		chain.headers.peerCount[1] != 2 {

		t.Fatal("a header on the peer's header dropped the wrong one")
	}

	// Headers on a chain with the tip's work are not dropped, so the
	// peer's third header doesn't fit.
	g3 := testHeader(nodes[2].hash, after(nodes[2], 11*time.Minute), bits, 0)
	g4 := testHeader(hashOf(g3), headerAfter(g3, 10*time.Minute), bits, 0)
	processHeaders(t, chain, 2, g3, g4)
	g4b := testHeader(hashOf(g3), headerAfter(g3, 10*time.Minute), bits, 1)
	_, index, err := chain.ProcessBlockHeaders([]*wire.BlockHeader{g4b}, 2)
	var rerr RuleError
	if err == nil || errors.As(err, &rerr) || index != 0 || isKept(chain, g4b) {
		t.Fatalf("got %v at index %d for a header from a peer whose "+
			"headers all have the tip's work", err, index)
	}
	if !isKept(chain, g4) {
		t.Fatal("a header with the tip's work was dropped")
	}
	checkHeaderTree(t, chain)

	// The tree is full, so another peer's header drops the tree's header
	// with the least work that nothing builds on.
	x := testHeader(genesis.hash, at, bits, 4)
	processHeaders(t, chain, 3, x)
	if isKept(chain, c3) || !isKept(chain, s3) || !isKept(chain, x) ||
		len(chain.headers.entries) != 4 || chain.headers.peerCount[1] != 1 {

		t.Fatal("a header added to a full tree dropped the wrong one")
	}

	// The header that made room had s3 as its parent, which nothing
	// builds on now, so it is the next to go, before the later x.
	y := testHeader(genesis.hash, at, bits, 5)
	processHeaders(t, chain, 4, y)
	if isKept(chain, s3) || !isKept(chain, x) || !isKept(chain, y) {
		t.Fatal("the header nothing builds on any more was not dropped")
	}
	if _, ok := chain.headers.peerCount[1]; ok ||
		chain.headers.peerLeaves[1] != nil {

		t.Fatal("a peer whose headers were all dropped is still counted")
	}
	checkHeaderTree(t, chain)
}

// TestHeaderTreeParentKept ensures a header that gets a child while its only
// other child is dropped to make room for it is not then dropped itself while
// its new child is kept.
func TestHeaderTreeParentKept(t *testing.T) {
	chain, nodes := headerTestChain(t, 4)
	chain.headers.maxPeerNodes = 2
	bits := chain.chainParams.PowLimitBits
	genesis := nodes[0]

	x := testHeader(genesis.hash, after(genesis, 10*time.Minute), bits, 0)
	c := testHeader(hashOf(x), headerAfter(x, 10*time.Minute), bits, 0)
	processHeaders(t, chain, 1, x, c)
	d := testHeader(hashOf(x), headerAfter(x, 10*time.Minute), bits, 1)
	processHeaders(t, chain, 1, d)
	checkHeaderTree(t, chain)
	e := testHeader(genesis.hash, after(genesis, 10*time.Minute), bits, 1)
	processHeaders(t, chain, 1, e)
	checkHeaderTree(t, chain)
	if !isKept(chain, x) || isKept(chain, c) || isKept(chain, d) ||
		!isKept(chain, e) {

		t.Fatal("the peer's headers that made room were the wrong ones")
	}
}

// TestHeaderTreePrune ensures the headers whose chain forks below the
// finalized block are dropped when headers are processed or blocks to fetch
// are looked for.
func TestHeaderTreePrune(t *testing.T) {
	chain, nodes := headerTestChain(t, 3)
	bits := chain.chainParams.PowLimitBits
	first := testHeader(nodes[0].hash, after(nodes[0], 11*time.Minute), bits, 0)
	below := testHeader(nodes[1].hash, after(nodes[1], 11*time.Minute), bits, 0)
	onBelow := testHeader(hashOf(below), headerAfter(below, 10*time.Minute),
		bits, 0)
	above := testHeader(nodes[2].hash, after(nodes[2], 11*time.Minute), bits, 0)
	processHeaders(t, chain, 2, first)
	processHeaders(t, chain, 2, below, onBelow)
	processHeaders(t, chain, 1, above)

	chain.finalizedNode = nodes[1]
	processHeaders(t, chain, 1, above)
	if isKept(chain, first) || !isKept(chain, below) {
		t.Fatal("processing headers did not drop just the header forking " +
			"below the finalized block")
	}

	chain.finalizedNode = nodes[2]
	chain.BlocksToFetch(&nodes[3].hash)
	if isKept(chain, below) || isKept(chain, onBelow) || !isKept(chain, above) {
		t.Fatal("the headers forking below the finalized block were not " +
			"the ones dropped")
	}
	if len(chain.headers.entries) != 1 || chain.headers.peerCount[1] != 1 {
		t.Fatal("dropped headers are still counted")
	}
	if _, ok := chain.headers.peerCount[2]; ok ||
		chain.headers.peerLeaves[2] != nil {

		t.Fatal("a peer whose headers were all dropped is still counted")
	}
	checkHeaderTree(t, chain)
}

// TestBlocksToFetch ensures the blocks to fetch for a header are those the
// chain lacks between the main chain and it, earliest first, when its chain
// has at least the tip's work and no invalid block, and that the kept headers
// of a chain with an invalid block are dropped.
func TestBlocksToFetch(t *testing.T) {
	chain, nodes := headerTestChain(t, 3)
	bits := chain.chainParams.PowLimitBits
	side := newFakeNode(nodes[1], 4, bits, after(nodes[1], 11*time.Minute))
	side.status = statusDataStored
	chain.index.AddNode(side)
	h3 := testHeader(side.hash, after(side, 10*time.Minute), bits, 0)
	h4 := testHeader(hashOf(h3), headerAfter(h3, 10*time.Minute), bits, 0)
	short := testHeader(nodes[0].hash, after(nodes[0], 11*time.Minute), bits, 0)
	processHeaders(t, chain, 1, h3, h4)
	processHeaders(t, chain, 1, short)

	tests := []struct {
		name string
		hash chainhash.Hash
		want []chainhash.Hash
	}{
		{"more work", hashOf(h4), []chainhash.Hash{hashOf(h3), hashOf(h4)}},
		{"same work", hashOf(h3), []chainhash.Hash{hashOf(h3)}},
		{"less work", hashOf(short), nil},
		{"the tip", nodes[3].hash, nil},
		{"unknown", chainhash.Hash{1}, nil},
	}
	for _, test := range tests {
		got := chain.BlocksToFetch(&test.hash)
		if !slices.Equal(got, test.want) {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}

	chain.index.SetStatusFlags(side, statusValidateFailed)
	if got := chain.BlocksToFetch(ptr(hashOf(h4))); got != nil {
		t.Fatalf("got %v for a chain with an invalid block", got)
	}
	if isKept(chain, h3) || isKept(chain, h4) || !isKept(chain, short) {
		t.Fatal("the kept headers of the chain with an invalid block were " +
			"not the ones dropped")
	}
	checkHeaderTree(t, chain)
}
