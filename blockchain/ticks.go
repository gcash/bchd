// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"math"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

const (
	// legacyBlockInterval is the target interval, in ticks (seconds), of
	// every block up to and including the upgrade 2027 activation block.
	legacyBlockInterval = 600

	// upgrade2027BlockInterval is the target interval, in ticks, of every
	// block after the upgrade 2027 activation block.
	upgrade2027BlockInterval = 60

	// mempoolHeight is the height of coins that are still in the mempool.
	mempoolHeight = 0x7fffffff
)

// upgrade2027Active returns whether the upgrade to 1-minute blocks applies to
// the children of a block with the passed median time past.
func upgrade2027Active(medianTime int64, activationTime uint64) bool {
	return activationTime <= math.MaxInt64 &&
		medianTime >= int64(activationTime)
}

// isUpgrade2027Active returns whether the upgrade to 1-minute blocks applies
// to the block after prevNode.
func isUpgrade2027Active(prevNode *blockNode) bool {
	return prevNode != nil && prevNode.nextInterval == upgrade2027BlockInterval
}

// blockInterval returns the target interval of the block itself in ticks.
func blockInterval(node *blockNode) int64 {
	if node.parent == nil {
		return legacyBlockInterval
	}
	return int64(node.parent.nextInterval)
}

// TickSchedule maps the heights of a branch to block ticks.  A block's ticks
// are the sum of the target intervals of the blocks up to and including it,
// which are 600 up to and including the upgrade 2027 activation block and 60
// after it.  The genesis block has zero ticks.
//
// A schedule is built for the block after a given block and holds for every
// height of that branch up to the next block.  The zero value never changes
// to 60 ticks, but has no next block, so NextBlockTicks and CoinTicks need a
// schedule from NewTickSchedule.
type TickSchedule struct {
	nextHeight       int32
	activationHeight int32
	active           bool
}

// NewTickSchedule returns the schedule for the block at nextHeight, given the
// height of the upgrade 2027 activation block of its branch, when the upgrade
// applies to it.
func NewTickSchedule(nextHeight, activationHeight int32, active bool) TickSchedule {
	return TickSchedule{
		nextHeight:       nextHeight,
		activationHeight: activationHeight,
		active:           active,
	}
}

// Active returns whether the blocks after the activation block of the branch
// are 60-tick blocks and, if so, the height of that activation block.
func (s TickSchedule) Active() (int32, bool) {
	return s.activationHeight, s.active
}

// NextHeight returns the height of the block the schedule was built for.
func (s TickSchedule) NextHeight() int32 {
	return s.nextHeight
}

// HeightToTicks returns the ticks of the block at the passed height.
func (s TickSchedule) HeightToTicks(height int32) int64 {
	if !s.active || height <= s.activationHeight {
		return int64(height) * legacyBlockInterval
	}
	return int64(s.activationHeight)*legacyBlockInterval +
		int64(height-s.activationHeight)*upgrade2027BlockInterval
}

// TicksToHeight returns the height of the last block whose ticks are not
// above the passed ticks, at most math.MaxInt32.
func (s TickSchedule) TicksToHeight(ticks int64) int32 {
	if ticks <= 0 {
		return 0
	}
	seam := int64(s.activationHeight) * legacyBlockInterval
	var height int64
	if !s.active || ticks <= seam {
		height = ticks / legacyBlockInterval
	} else {
		height = int64(s.activationHeight) +
			(ticks-seam)/upgrade2027BlockInterval
	}
	return int32(min(height, math.MaxInt32))
}

// BlockInterval returns the target interval of the block at the passed
// height.  The genesis block reports 600.
func (s TickSchedule) BlockInterval(height int32) int64 {
	if height == 0 {
		return legacyBlockInterval
	}
	return s.NextBlockInterval(height - 1)
}

// NextBlockInterval returns the target interval of the block after the one
// at the passed height.
func (s TickSchedule) NextBlockInterval(height int32) int64 {
	if s.active && height >= s.activationHeight {
		return upgrade2027BlockInterval
	}
	return legacyBlockInterval
}

// NextBlockTicks returns the ticks of the block the schedule was built for.
func (s TickSchedule) NextBlockTicks() int64 {
	return s.HeightToTicks(s.nextHeight)
}

// CoinTicks returns the ticks of the block a coin at the passed height was
// created in.  Coins in the mempool are taken to be in the next block.
func (s TickSchedule) CoinTicks(height int32) int64 {
	if height == mempoolHeight {
		height = s.nextHeight
	}
	return s.HeightToTicks(height)
}

// upgrade2027ActivationNode returns the activation block of the branch of
// prevNode: the first block whose children are 60-tick blocks.  It returns
// nil when the upgrade does not apply to the block after prevNode.
//
// The last result is cached, as most calls are for the same branch.
//
// This function is safe for concurrent access.
func (b *BlockChain) upgrade2027ActivationNode(prevNode *blockNode) *blockNode {
	if !isUpgrade2027Active(prevNode) {
		return nil
	}
	cached := b.upgrade2027Node.Load()
	if cached != nil && prevNode.Ancestor(cached.height) == cached {
		return cached
	}

	// The median time past never decreases along a branch, so every block
	// from the activation block on is active.  Skip back while the skip
	// target is still active.
	n := prevNode
	for n.parent != nil {
		if isUpgrade2027Active(n.skip) {
			n = n.skip
			continue
		}
		if !isUpgrade2027Active(n.parent) {
			break
		}
		n = n.parent
	}
	b.upgrade2027Node.Store(n)
	return n
}

// tickSchedule returns the tick schedule for the block after prevNode.
//
// This function is safe for concurrent access.
func (b *BlockChain) tickSchedule(prevNode *blockNode) TickSchedule {
	s := TickSchedule{nextHeight: prevNode.height + 1}
	if n := b.upgrade2027ActivationNode(prevNode); n != nil {
		s.activationHeight = n.height
		s.active = true
	}
	return s
}

// BlockTicks describes where a block lies on the tick schedule of its branch.
type BlockTicks struct {
	// Ticks is the sum of the target intervals of the blocks up to and
	// including this one.
	Ticks int64

	// Interval is the target interval of the block itself.
	Interval int64

	// NextInterval is the target interval of the block's children.
	NextInterval int64
}

// BlockTicksByHash returns the ticks of the block or kept header with the
// passed hash.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockTicksByHash(hash *chainhash.Hash) (BlockTicks, error) {
	// The ticks of a node never change once it is in the block index, so
	// only the header tree needs the chain lock.  Look in the index again
	// under the lock, since a kept header may have moved there meanwhile.
	node := b.index.LookupNode(hash)
	if node == nil {
		b.chainLock.RLock()
		node = b.index.LookupNode(hash)
		if node == nil {
			node = b.headers.lookup(hash)
		}
		b.chainLock.RUnlock()
	}
	if node == nil {
		return BlockTicks{}, fmt.Errorf("block %s is not known", hash)
	}
	return BlockTicks{
		Ticks:        node.ticks,
		Interval:     blockInterval(node),
		NextInterval: int64(node.nextInterval),
	}, nil
}

// Upgrade2027ActivationBlock returns the hash and height of the upgrade 2027
// activation block of the main chain: the last 600-tick block.  It returns
// false when the upgrade has not activated on the main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) Upgrade2027ActivationBlock() (chainhash.Hash, int32, bool) {
	n := b.upgrade2027ActivationNode(b.bestChain.Tip())
	if n == nil {
		return chainhash.Hash{}, 0, false
	}
	return n.hash, n.height, true
}

// Upgrade2027ActivationBlocks returns the height and hash of the upgrade 2027
// activation block of the branch that ends at the block with the passed hash,
// and the hash of the block after it on that branch, which is nil when the
// branch ends at the activation block.  The activation hash is nil when the
// upgrade has not activated on the branch or the block is not known.
//
// This function is safe for concurrent access.
func (b *BlockChain) Upgrade2027ActivationBlocks(tip *chainhash.Hash) (int32, *chainhash.Hash, *chainhash.Hash) {
	node := b.index.LookupNode(tip)
	if node == nil {
		return 0, nil, nil
	}
	n := b.upgrade2027ActivationNode(node)
	if n == nil {
		return 0, nil, nil
	}
	pre := n.hash
	var post *chainhash.Hash
	if next := node.Ancestor(n.height + 1); next != nil {
		post = &next.hash
	}
	return n.height, &pre, post
}
