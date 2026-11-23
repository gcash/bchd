// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"container/list"
	"fmt"
	"math/big"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

const (
	// parkingLockInTicks is how far, in ticks, the tip may be past the
	// fork for a parked chain to need only an eighth more work than the
	// tip has since the fork.  From there on it needs twice that work.
	parkingLockInTicks = 4 * legacyBlockInterval

	// parkingSubCapShift divides the tip's work since the fork by eight
	// for the extra work a parked chain needs short of the lock-in.
	parkingSubCapShift = 3
)

// unparkRequiredWork returns the work a parked chain must exceed to be
// unparked once the tip is a block after the upgrade to 1-minute blocks.
// That is the tip's work plus an eighth of its work since the fork when the
// tip is less than parkingLockInTicks past the fork, and plus all of it from
// there on.
func unparkRequiredWork(tipWork, forkWork *big.Int, tickDepth int64) *big.Int {
	penalty := new(big.Int).Sub(tipWork, forkWork)
	if tickDepth < parkingLockInTicks {
		penalty.Rsh(penalty, parkingSubCapShift)
	}
	return penalty.Add(penalty, tipWork)
}

// legacyUnparkRequiredWork returns the work a parked chain must exceed to be
// unparked before the upgrade to 1-minute blocks applies to the tip, as in
// Bitcoin Cash Node.  A tip 1, 2 or 3 blocks past the fork asks for half a
// block's work more, half of the first block's at depth 2 and of the first
// two blocks' at depth 3, and a deeper tip for twice its work since the fork.
func legacyUnparkRequiredWork(tip, fork *blockNode) *big.Int {
	extra := tip
	switch tip.height - fork.height {
	case 2, 3:
		extra = tip.parent
		fallthrough
	case 1:
		penalty := new(big.Int).Sub(extra.workSum, fork.workSum)
		penalty.Rsh(penalty, 1)
		return penalty.Add(penalty, tip.workSum)
	default:
		penalty := new(big.Int).Sub(tip.workSum, fork.workSum)
		return penalty.Add(penalty, tip.workSum)
	}
}

// parkedChainHasEnoughWorkToUnpark returns whether the chain ending at
// candidate, which forks the main chain ending at tip at fork, has enough
// work to be unparked.  The rule after the upgrade applies once the tip is a
// block after it, whose parent's median time past has reached the activation
// time.  A tip whose own median time past first reaches the activation time
// still takes the legacy rule.
func parkedChainHasEnoughWorkToUnpark(tip, fork, candidate *blockNode) bool {
	var required *big.Int
	if isUpgrade2027Active(tip.parent) {
		required = unparkRequiredWork(tip.workSum, fork.workSum,
			tip.ticks-fork.ticks)
	} else {
		required = legacyUnparkRequiredWork(tip, fork)
	}
	return candidate.workSum.Cmp(required) > 0
}

// parkedChainUsable returns whether the main chain can switch to the chain
// ending at node, which forks it at fork and attaches attachNodes, as far as
// parking goes.  A chain with a parked block is unparked when automatic
// unparking is on and it has enough work, as in Bitcoin Cash Node, and is
// refused otherwise.  Nodes unparked here are not flushed.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) parkedChainUsable(node, fork *blockNode, attachNodes *list.List) bool {
	parked := false
	for e := attachNodes.Front(); e != nil && !parked; e = e.Next() {
		parked = b.index.NodeStatus(e.Value.(*blockNode)).Parked()
	}
	if !parked {
		return true
	}

	tip := b.bestChain.Tip()
	if !b.automaticUnparking ||
		!parkedChainHasEnoughWorkToUnpark(tip, fork, node) {

		log.Infof("Considered switching to better tip %v but that "+
			"chain contains a parked block", node.hash)
		b.noteParked(node)
		return false
	}

	log.Infof("Unpark chain up to block %v as it has accumulated enough "+
		"PoW", node.hash)
	b.unparkNodes(attachNodes)
	return true
}

// onParkedChain returns whether a block between the node and the main chain,
// the node included, is parked.
//
// This function MUST be called with the chain state lock held (for reads).
func (b *BlockChain) onParkedChain(node *blockNode) bool {
	for n := node; n != nil && !b.bestChain.Contains(n); n = n.parent {
		if b.index.NodeStatus(n).Parked() {
			return true
		}
	}
	return false
}

// noteParked makes the node the tip of the best parked chain if it is on a
// parked chain and has more work than the one before.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) noteParked(node *blockNode) {
	if !b.onParkedChain(node) {
		return
	}
	if b.bestParked == nil || node.workSum.Cmp(b.bestParked.workSum) > 0 {
		b.bestParked = node
	}
}

// BestParkedHeight returns the height of the tip of the parked chain with the
// most work the node has stored, while that chain has more work than the main
// chain and is still parked, and -1 otherwise.  A peer whose chain is no
// higher may only be on that chain, which the node has and doesn't follow.
//
// This function is safe for concurrent access.
func (b *BlockChain) BestParkedHeight() int32 {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	n := b.bestParked
	if n == nil || n.workSum.Cmp(b.bestChain.Tip().workSum) <= 0 ||
		b.index.NodeStatus(n).KnownInvalid() || !b.onParkedChain(n) {

		return -1
	}
	return n.height
}

// IsOnParkedChain returns whether the block with the given hash is off the
// main chain on a parked chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) IsOnParkedChain(hash *chainhash.Hash) bool {
	node := b.index.LookupNode(hash)
	if node == nil {
		return false
	}
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()
	return b.onParkedChain(node)
}

// unparkNodes unparks the parked nodes of the list and returns them.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) unparkNodes(nodes *list.List) []*blockNode {
	var unparked []*blockNode
	for e := nodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(*blockNode)
		if b.index.NodeStatus(n).Parked() {
			b.index.UnsetStatusFlags(n, statusParked)
			unparked = append(unparked, n)
		}
	}
	return unparked
}

// ParkBlock parks the block with the given hash, as Bitcoin Cash Node's
// parkblock does.  When the block is in the main chain, the main chain is
// disconnected down to its parent.  The chain with the most work the node
// has is then connected, if it has no parked block or can be unparked.  The
// block's own chain can be at once while unparking is automatic, since the
// main chain then has no work past the fork.  Parking a parked block is not
// an error.
//
// This function is safe for concurrent access.
func (b *BlockChain) ParkBlock(hash *chainhash.Hash) error {
	b.processLock.Lock()
	defer b.processLock.Unlock()

	node := b.index.LookupNode(hash)
	if node == nil {
		return fmt.Errorf("block %s is not known", hash)
	}
	if node.parent == nil {
		return fmt.Errorf("block %s has no parent", hash)
	}

	// The block is parked before the reorganization, which releases the
	// chain lock while it sends notifications, so that blocks arriving
	// meanwhile can't extend its chain, and a failure takes it back.
	b.chainLock.Lock()
	wasParked := b.index.NodeStatus(node).Parked()
	b.index.SetStatusFlags(node, statusParked)
	if b.bestChain.Contains(node) {
		detachNodes, attachNodes := b.getReorganizeNodes(node.parent)
		err := b.reorganizeChain(detachNodes, attachNodes)
		if err != nil {
			if !wasParked {
				b.index.UnsetStatusFlags(node, statusParked)
			}
			if writeErr := b.index.flushToDB(); writeErr != nil {
				log.Warnf("Error flushing block index changes to "+
					"disk: %v", writeErr)
			}
			b.chainLock.Unlock()
			return err
		}
	}
	b.chainLock.Unlock()
	log.Debugf("Parked block %v (height %d)", node.hash, node.height)

	err := b.reorganizeToBestKnown()

	if writeErr := b.index.flushToDB(); writeErr != nil {
		log.Warnf("Error flushing block index changes to disk: %v", writeErr)
	}
	return err
}

// UnparkBlock unparks the block with the given hash, the blocks between it
// and the main chain and the blocks built on it, as Bitcoin Cash Node's
// unparkblock does, and connects the best chain among them if it has more
// work than the main chain.  Unparking a block that is not parked is not an
// error.
//
// This function is safe for concurrent access.
func (b *BlockChain) UnparkBlock(hash *chainhash.Hash) error {
	b.processLock.Lock()
	defer b.processLock.Unlock()

	node := b.index.LookupNode(hash)
	if node == nil {
		return fmt.Errorf("block %s is not known", hash)
	}
	descendants := b.index.descendants(node)

	b.chainLock.Lock()
	unpark := func(n *blockNode) {
		if b.index.NodeStatus(n).Parked() {
			b.index.UnsetStatusFlags(n, statusParked)
		}
	}
	for n := node; n != nil && !b.bestChain.Contains(n); n = n.parent {
		unpark(n)
	}
	for _, n := range descendants {
		unpark(n)
	}
	b.chainLock.Unlock()
	log.Debugf("Unparked block %v (height %d) and the blocks built on it",
		node.hash, node.height)

	err := b.reorganizeToBest(append([]*blockNode{node}, descendants...))

	if writeErr := b.index.flushToDB(); writeErr != nil {
		log.Warnf("Error flushing block index changes to disk: %v", writeErr)
	}
	return err
}
