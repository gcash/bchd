// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"container/heap"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

const (
	// maxHeaderTreeNodes is the number of headers the header tree keeps.
	maxHeaderTreeNodes = 10000

	// maxPeerHeaderTreeNodes is the number of them that may come from one
	// peer, so that one peer can't crowd out the others.
	maxPeerHeaderTreeNodes = 2000
)

// errHeaderTreeFull is the error of a header that doesn't fit in the header
// tree, since every header it could replace is on a chain with at least the
// tip's work.
var errHeaderTreeFull = errors.New("the header tree is full")

// headerEntry is a header the header tree keeps.
type headerEntry struct {
	node *blockNode

	// peer is the ID of the peer that sent the header.
	peer int32

	// children is the number of the tree's headers that build on it.
	children int

	// seq orders the headers with the same work by when they were added.
	seq uint64

	// heapIdx and peerHeapIdx are the header's places in the heaps of the
	// tree's leaves and of its peer's, or -1 while it has children.
	heapIdx, peerHeapIdx int
}

// leafHeap is a heap of headers that nothing in the header tree builds on,
// the one with the least work first, and of those the one added first.
type leafHeap struct {
	entries []*headerEntry

	// peer tells that the heap is a peer's, whose headers keep their
	// place in it in peerHeapIdx.
	peer bool
}

func (h *leafHeap) Len() int { return len(h.entries) }

func (h *leafHeap) Less(i, j int) bool {
	a, b := h.entries[i], h.entries[j]
	if c := a.node.workSum.Cmp(b.node.workSum); c != 0 {
		return c < 0
	}
	return a.seq < b.seq
}

func (h *leafHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.setIndex(i, i)
	h.setIndex(j, j)
}

func (h *leafHeap) Push(x any) {
	h.entries = append(h.entries, x.(*headerEntry))
	h.setIndex(len(h.entries)-1, len(h.entries)-1)
}

func (h *leafHeap) Pop() any {
	last := len(h.entries) - 1
	e := h.entries[last]
	h.setIndex(last, -1)
	h.entries[last] = nil
	h.entries = h.entries[:last]
	return e
}

// setIndex records place as the place of the header at index i.
func (h *leafHeap) setIndex(i, place int) {
	if h.peer {
		h.entries[i].peerHeapIdx = place
	} else {
		h.entries[i].heapIdx = place
	}
}

// headerTree keeps the valid headers of blocks the chain doesn't have, which
// build on blocks it has or on other headers it keeps, so that the blocks of a
// chain of headers can be requested once the chain has enough work, as BCHN
// keeps headers in its block index.  Its nodes are not in the block index:
// there, a node means a stored block.  When a block arrives, its node moves
// into the block index, so that the nodes built on it keep their parent.
// When the tree is full, or a peer has its share, the header with the least
// work that nothing builds on makes room, but not one on a chain with at least
// the tip's work, whose blocks are about to be requested.
//
// The header tree is protected by the chain lock.
type headerTree struct {
	entries map[chainhash.Hash]*headerEntry

	// children holds the headers that build on each block or header.
	children map[chainhash.Hash]map[*headerEntry]struct{}

	// leaves are the headers nothing builds on, and peerLeaves and
	// peerCount each peer's of those and number of headers.
	leaves     leafHeap
	peerLeaves map[int32]*leafHeap
	peerCount  map[int32]int

	seq uint64

	// maxNodes and maxPeerNodes are the number of headers the tree keeps
	// and that may come from one peer.
	maxNodes, maxPeerNodes int

	// prunedAt is the finalized block the tree was last pruned for.
	prunedAt *blockNode
}

// newHeaderTree returns an empty header tree.
func newHeaderTree() *headerTree {
	return &headerTree{
		entries:      make(map[chainhash.Hash]*headerEntry),
		children:     make(map[chainhash.Hash]map[*headerEntry]struct{}),
		peerLeaves:   make(map[int32]*leafHeap),
		peerCount:    make(map[int32]int),
		maxNodes:     maxHeaderTreeNodes,
		maxPeerNodes: maxPeerHeaderTreeNodes,
	}
}

// lookup returns the node of the kept header with the passed hash, or nil.
func (t *headerTree) lookup(hash *chainhash.Hash) *blockNode {
	if e := t.entries[*hash]; e != nil {
		return e.node
	}
	return nil
}

// contains returns whether the node is the node of a kept header.
func (t *headerTree) contains(node *blockNode) bool {
	_, ok := t.entries[node.hash]
	return ok
}

// add keeps the node, whose header the peer sent, if room can be made for it
// without dropping a header on a chain with at least tipWork, and returns
// whether it did.
func (t *headerTree) add(node *blockNode, peer int32, tipWork *big.Int) bool {
	// The parent counts the header as its child first, so it isn't dropped
	// to make room, even when its other children are.
	parent := t.entries[node.parent.hash]
	if parent != nil {
		if parent.children == 0 {
			t.removeLeaf(parent)
		}
		parent.children++
	}
	if !t.makeRoom(peer, tipWork) {
		if parent != nil {
			parent.children--
			if parent.children == 0 {
				t.addLeaf(parent)
			}
		}
		return false
	}

	t.seq++
	e := &headerEntry{node: node, peer: peer, seq: t.seq, heapIdx: -1,
		peerHeapIdx: -1}
	t.entries[node.hash] = e
	siblings := t.children[node.parent.hash]
	if siblings == nil {
		siblings = make(map[*headerEntry]struct{})
		t.children[node.parent.hash] = siblings
	}
	siblings[e] = struct{}{}
	t.peerCount[peer]++
	t.addLeaf(e)
	return true
}

// makeRoom drops headers until the tree has room for one more from the peer,
// the peer's first while it has its share, and returns whether it has.
func (t *headerTree) makeRoom(peer int32, tipWork *big.Int) bool {
	for t.peerCount[peer] >= t.maxPeerNodes {
		if !t.dropLeastWork(t.peerLeaves[peer], tipWork) {
			return false
		}
	}
	for len(t.entries) >= t.maxNodes {
		if !t.dropLeastWork(&t.leaves, tipWork) {
			return false
		}
	}
	return true
}

// dropLeastWork drops the header with the least work in the heap, unless its
// chain has at least tipWork, and returns whether it did.
func (t *headerTree) dropLeastWork(h *leafHeap, tipWork *big.Int) bool {
	if h == nil || h.Len() == 0 || h.entries[0].node.workSum.Cmp(tipWork) >= 0 {
		return false
	}
	t.remove(h.entries[0])
	return true
}

// addLeaf adds the header to the heaps of leaves.
func (t *headerTree) addLeaf(e *headerEntry) {
	heap.Push(&t.leaves, e)
	h := t.peerLeaves[e.peer]
	if h == nil {
		h = &leafHeap{peer: true}
		t.peerLeaves[e.peer] = h
	}
	heap.Push(h, e)
}

// removeLeaf removes the header from the heaps of leaves it is in.
func (t *headerTree) removeLeaf(e *headerEntry) {
	if e.heapIdx >= 0 {
		heap.Remove(&t.leaves, e.heapIdx)
	}
	if e.peerHeapIdx >= 0 {
		heap.Remove(t.peerLeaves[e.peer], e.peerHeapIdx)
	}
}

// unlink removes the header from the tree, but not the headers that build on
// it.
func (t *headerTree) unlink(e *headerEntry) {
	t.removeLeaf(e)
	delete(t.entries, e.node.hash)
	parentHash := e.node.parent.hash
	siblings := t.children[parentHash]
	delete(siblings, e)
	if len(siblings) == 0 {
		delete(t.children, parentHash)
	}
	t.peerCount[e.peer]--
	if t.peerCount[e.peer] == 0 {
		delete(t.peerCount, e.peer)
		delete(t.peerLeaves, e.peer)
	}
}

// remove removes the header, which nothing in the tree builds on.
func (t *headerTree) remove(e *headerEntry) {
	t.unlink(e)
	t.releaseParent(e)
}

// releaseParent makes the parent of the removed header a leaf if the header
// was the last one built on it.
func (t *headerTree) releaseParent(e *headerEntry) {
	if parent := t.entries[e.node.parent.hash]; parent != nil {
		parent.children--
		if parent.children == 0 {
			t.addLeaf(parent)
		}
	}
}

// removeSubtree removes the header and the headers that build on it.
func (t *headerTree) removeSubtree(e *headerEntry) {
	subtree := []*headerEntry{e}
	for i := 0; i < len(subtree); i++ {
		for child := range t.children[subtree[i].node.hash] {
			subtree = append(subtree, child)
		}
	}
	for _, e := range subtree {
		t.unlink(e)
	}
	t.releaseParent(e)
}

// take removes the kept header with the passed hash, whose block is being
// added to the block index with prevNode as its parent, and returns its node,
// for the block, so that the nodes built on it keep their parent.  It returns
// nil when the tree doesn't keep the header.
func (t *headerTree) take(hash *chainhash.Hash, prevNode *blockNode) *blockNode {
	e := t.entries[*hash]
	if e == nil {
		return nil
	}
	if e.node.parent != prevNode {
		t.removeSubtree(e)
		return nil
	}
	t.unlink(e)
	return e.node
}

// prune drops the headers whose chain forks below the finalized block, which
// can't be connected.
func (t *headerTree) prune(b *BlockChain) {
	if b.finalizedNode == t.prunedAt {
		return
	}
	t.prunedAt = b.finalizedNode
	var forks []*headerEntry
	for parentHash, children := range t.children {
		if t.entries[parentHash] != nil {
			continue
		}
		for e := range children {
			if b.forksBeforeFinalized(e.node.parent) {
				forks = append(forks, e)
			}
		}
	}
	for _, e := range forks {
		t.removeSubtree(e)
	}
}

// HeaderState describes a block header the chain knows, as that of a block it
// has or as one it keeps until the block arrives.
type HeaderState struct {
	Hash   chainhash.Hash
	Height int32

	// Work is the work of the chain up to and including the block, and
	// WorkCmp compares it with the tip's: it is -1 when it is less, 0 when
	// it is the same and 1 when it is more.
	Work    *big.Int
	WorkCmp int
}

// ProcessBlockHeaders checks the headers, each of which must build on the one
// before it, against the block or kept header it builds on, with the rules
// BCHN checks a header with before it keeps it (AcceptBlockHeader and
// ContextualCheckBlockHeader, validation.cpp:3685-3781, 3901-4007), and keeps
// the headers of the blocks the chain doesn't have, as sent by the peer with
// the passed ID, until their blocks arrive.  It returns the state of the last
// header or, for the first one that fails, the error and the header's index.
// A header that doesn't fit in the header tree fails with an error that is not
// a RuleError.
//
// This function is safe for concurrent access.
func (b *BlockChain) ProcessBlockHeaders(headers []*wire.BlockHeader, peerID int32) (*HeaderState, int, error) {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	b.headers.prune(b)
	var node, added *blockNode
	for i, header := range headers {
		var isNew bool
		var err error
		node, isNew, err = b.processBlockHeader(header, peerID, added)
		if err != nil {
			return nil, i, err
		}
		added = nil
		if isNew {
			added = node
		}
	}
	if node == nil {
		return nil, -1, nil
	}
	return &HeaderState{
		Hash:    node.hash,
		Height:  node.height,
		Work:    new(big.Int).Set(node.workSum),
		WorkCmp: node.workSum.Cmp(b.bestChain.Tip().workSum),
	}, -1, nil
}

// processBlockHeader processes the header for ProcessBlockHeaders and returns
// its node and whether it was added.  added is the node of the header before
// it if that was added, whose chain was then found to have no invalid block.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) processBlockHeader(header *wire.BlockHeader, peerID int32, added *blockNode) (*blockNode, bool, error) {
	hash := header.BlockHash()
	if node := b.index.LookupNode(&hash); node != nil {
		if b.index.NodeStatus(node).KnownInvalid() {
			str := fmt.Sprintf("block %v is known to be invalid", hash)
			return nil, false, ruleError(ErrDuplicateBlock, str)
		}
		return node, false, nil
	}
	if node := b.headers.lookup(&hash); node != nil {
		return node, false, nil
	}

	err := checkBlockHeaderSanity(header, b.chainParams.PowLimit,
		b.timeSource, BFNone)
	if err != nil {
		return nil, false, err
	}
	prevNode := b.index.LookupNode(&header.PrevBlock)
	if prevNode == nil {
		prevNode = b.headers.lookup(&header.PrevBlock)
	}
	if prevNode == nil {
		str := fmt.Sprintf("previous block %s is unknown", header.PrevBlock)
		return nil, false, ruleError(ErrPreviousBlockUnknown, str)
	}
	if prevNode != added && b.headerChainInvalid(prevNode) {
		str := fmt.Sprintf("previous block %s has an invalid ancestor",
			header.PrevBlock)
		return nil, false, ruleError(ErrInvalidAncestorBlock, str)
	}
	err = b.checkBlockHeaderContext(header, prevNode, BFNone)
	if err != nil {
		return nil, false, err
	}

	node := newBlockNode(header, prevNode)
	if !b.headers.add(node, peerID, b.bestChain.Tip().workSum) {
		return nil, false, errHeaderTreeFull
	}
	return node, true, nil
}

// headerChainInvalid returns whether the first block in node's chain that is
// not a kept header is known to be invalid.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) headerChainInvalid(node *blockNode) bool {
	for b.headers.contains(node) {
		node = node.parent
	}
	return b.index.NodeStatus(node).KnownInvalid()
}

// BlocksToFetch returns the hashes of the blocks of the kept headers between
// the main chain and the block or kept header with the passed hash, earliest
// first, when the chain up to it has at least the tip's work and none of its
// blocks is known to be invalid, and none otherwise.  The kept headers of a
// chain with an invalid block are dropped.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlocksToFetch(hash *chainhash.Hash) []chainhash.Hash {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	b.headers.prune(b)
	node := b.index.LookupNode(hash)
	if node == nil {
		node = b.headers.lookup(hash)
	}
	if node == nil || node.workSum.Cmp(b.bestChain.Tip().workSum) < 0 {
		return nil
	}

	var hashes []chainhash.Hash
	var kept *blockNode
	for n := node; !b.bestChain.Contains(n); n = n.parent {
		if b.headers.contains(n) {
			hashes = append(hashes, n.hash)
			kept = n
			continue
		}
		if b.index.NodeStatus(n).KnownInvalid() {
			if kept != nil {
				b.headers.removeSubtree(b.headers.entries[kept.hash])
			}
			return nil
		}
	}
	slices.Reverse(hashes)
	return hashes
}
