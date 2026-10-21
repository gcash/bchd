// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

const (
	// DefaultMaxReorgDepth is the default depth below the tip at which
	// main chain blocks are finalized, so they can no longer be reorged.
	DefaultMaxReorgDepth = 10

	// DefaultFinalizationDelay is the default time a block must have been
	// known, and the node running, before the block can be finalized.
	DefaultFinalizationDelay = 2 * time.Hour
)

// findBlockToFinalize returns the block to finalize at time now, in Unix
// seconds, with tip as the main chain tip, or nil.  This is the newest block
// that is at least maxReorgDepth below the tip and has been known for the
// finalization delay.  Blocks loaded from disk count as known since startup.
// Nothing is finalized while the tip has less than the minimum chain work.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) findBlockToFinalize(tip *blockNode, now int64) *blockNode {
	if b.maxReorgDepth < 0 {
		return nil
	}

	// Bitcoin Cash Node never downloads a chain with less than the minimum
	// chain work, but a sync peer can feed this node one.  Finalizing such a
	// chain would keep the node on it, so wait until the chain has that work.
	minWork := b.chainParams.MinimumChainWork
	if minWork != nil && tip.workSum.Cmp(minWork) < 0 {
		return nil
	}

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
// tip at time now, in Unix seconds, if any.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) maybeFinalize(tip *blockNode, now int64) {
	node := b.findBlockToFinalize(tip, now)
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
// fork the chain below the finalized block, checked as BCHN does.  Ancestor
// follows skip pointers, so this takes a logarithmic number of steps however
// long a side branch is.
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

// onSameFork returns whether one of a and b descends from the other.
func onSameFork(a, b *blockNode) bool {
	if a.height > b.height {
		a = a.Ancestor(b.height)
	} else {
		b = b.Ancestor(a.height)
	}
	return a == b
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

// FinalizedBlockHash returns the hash of the finalized block, or nil when no
// block is finalized.
//
// This function is safe for concurrent access.
func (b *BlockChain) FinalizedBlockHash() *chainhash.Hash {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()
	if b.finalizedNode == nil {
		return nil
	}
	hash := b.finalizedNode.hash
	return &hash
}

// FinalizeBlock finalizes the block with the given hash, so that blocks
// forking the chain below it are rejected.  When the block is not in the main
// chain, the main chain is disconnected down to the fork point, its first
// block there is marked invalid and the block's chain is connected up to the
// block.  A block that conflicts with the finalized block is refused.
//
// This function is safe for concurrent access.
func (b *BlockChain) FinalizeBlock(hash *chainhash.Hash) error {
	node := b.index.LookupNode(hash)
	if node == nil {
		return fmt.Errorf("block %s is not known", hash)
	}

	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	if b.index.NodeStatus(node).KnownInvalid() {
		return fmt.Errorf("block %s is invalid", hash)
	}
	fin := b.finalizedNode
	if fin != nil && !onSameFork(node, fin) {
		return fmt.Errorf("block %s conflicts with the finalized block %s",
			hash, fin.hash)
	}
	if fin != nil && node.height <= fin.height {
		return nil
	}

	if b.bestChain.Contains(node) {
		log.Infof("Finalized block %v (height %d)", node.hash, node.height)
		b.finalizedNode = node
		return nil
	}

	detachNodes, attachNodes := b.getReorganizeNodes(node)
	if attachNodes.Len() == 0 {
		return fmt.Errorf("block %s is invalid", hash)
	}

	// Finalize the block and invalidate the chain being detached before
	// the reorganization, which releases the chain lock while it sends
	// notifications, so blocks arriving meanwhile can't extend that chain.
	const invalid = statusValidateFailed | statusInvalidAncestor
	oldStatus := make([]blockStatus, 0, detachNodes.Len())
	for e := detachNodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(*blockNode)
		oldStatus = append(oldStatus, b.index.NodeStatus(n))
		if e == detachNodes.Back() {
			b.index.SetStatusFlags(n, statusValidateFailed)
		} else {
			b.index.SetStatusFlags(n, statusInvalidAncestor)
		}
		b.index.UnsetStatusFlags(n, statusValid)
	}
	b.finalizedNode = node

	if err := b.reorganizeChain(detachNodes, attachNodes); err != nil {
		b.finalizedNode = fin
		i := 0
		for e := detachNodes.Front(); e != nil; e, i = e.Next(), i+1 {
			n := e.Value.(*blockNode)
			b.index.UnsetStatusFlags(n, invalid&^oldStatus[i])
			b.index.SetStatusFlags(n, oldStatus[i]&statusValid)
		}
		return err
	}
	if err := b.index.flushToDB(); err != nil {
		log.Warnf("Error flushing block index changes to disk: %v", err)
	}

	log.Infof("Finalized block %v (height %d)", node.hash, node.height)
	return nil
}
