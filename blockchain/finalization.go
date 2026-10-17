// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"time"
)

const (
	// DefaultMaxReorgDepth is the default depth below the tip at which
	// main chain blocks are finalized, so they can no longer be reorged.
	DefaultMaxReorgDepth = 10

	// DefaultFinalizationDelay is the default time a block must have been
	// known, and the node running, before the block can be finalized.
	DefaultFinalizationDelay = 2 * time.Hour
)

// findBlockToFinalize returns the block to finalize now that tip is the main
// chain tip, or nil.  This is the newest block that is at least maxReorgDepth
// below the tip and has been known for the finalization delay.  Blocks loaded
// from disk count as known since startup.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) findBlockToFinalize(tip *blockNode) *blockNode {
	if b.maxReorgDepth < 0 {
		return nil
	}
	now := time.Now().Unix()
	delay := int64(b.finalizationDelay / time.Second)
	if now < b.startTime+delay {
		return nil
	}
	known := func(n *blockNode) bool {
		return now >= int64(n.received)+delay
	}

	candidate := b.bestChain.nodeByHeight(tip.height - b.maxReorgDepth)
	if candidate == nil || candidate == b.finalizedNode {
		return nil
	}
	if known(candidate) {
		return candidate
	}

	// A block is only accepted after its parent, so receive times don't
	// decrease along the chain.  Scan up from the finalized block rather
	// than down from the candidate, which can be far during a sync.
	height := int32(0)
	if b.finalizedNode != nil {
		height = b.finalizedNode.height + 1
	}
	var found *blockNode
	for ; height < candidate.height; height++ {
		node := b.bestChain.nodeByHeight(height)
		if !known(node) {
			break
		}
		found = node
	}
	return found
}

// maybeFinalize finalizes the block found by findBlockToFinalize for the new
// tip, if any.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) maybeFinalize(tip *blockNode) {
	node := b.findBlockToFinalize(tip)
	if node == nil {
		return
	}
	if b.finalizedNode != nil && node.height <= b.finalizedNode.height {
		return
	}
	log.Debugf("Finalized block %v (height %d)", node.hash, node.height)
	b.finalizedNode = node
}

// forksBeforeFinalized returns whether a block whose parent is prevNode would
// fork the chain below the finalized block.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) forksBeforeFinalized(prevNode *blockNode) bool {
	fin := b.finalizedNode
	return fin != nil && prevNode.Ancestor(fin.height) != fin
}

// lastCommonAncestor returns the last block that a and b both descend from.
func lastCommonAncestor(a, b *blockNode) *blockNode {
	if a.height > b.height {
		a = a.Ancestor(b.height)
	} else if b.height > a.height {
		b = b.Ancestor(a.height)
	}
	for a != b && a != nil && b != nil {
		a = a.parent
		b = b.parent
	}
	return a
}

// unfinalizeTo moves the finalized block back to the last block it shares
// with node, so that a block being reconsidered can be reorged to.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) unfinalizeTo(node *blockNode) {
	if b.finalizedNode != nil {
		b.finalizedNode = lastCommonAncestor(node, b.finalizedNode)
	}
}
