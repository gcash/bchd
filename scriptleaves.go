// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"sync"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs/builder"
)

// scriptLeavesOfBlock is a block with the outputs it spends and the leaves of
// its script filter entry tree. It also keeps the nodes of that tree above
// the leaves, about one hash for each leaf, and the block's Merkle tree, so
// that a range or a proof answered from it reads a few hashes for each level
// rather than hashing the whole tree or the whole block again.
type scriptLeavesOfBlock struct {
	block  *bchutil.Block
	stxos  []blockchain.SpentTxOut
	leaves []builder.ScriptLeaf

	// upper holds the levels of the entry tree above the leaves, from the
	// parents of the leaves up to the root. A node without a partner moves
	// up unchanged, as in the tree itself.
	upper [][]chainhash.Hash

	// merkles is the block's Merkle tree, as BuildMerkleTreeStore lays it
	// out.
	merkles []*chainhash.Hash
}

// errScriptRangeRun is returned for a run of leaves the tree doesn't have.
var errScriptRangeRun = errors.New("the run of leaves is not in the tree")

// node returns the hash of the node at index of a level of the entry tree,
// level 0 being the leaves.
func (e *scriptLeavesOfBlock) node(level, index int) chainhash.Hash {
	if level == 0 {
		return builder.ScriptLeafHash(&e.leaves[index])
	}
	return e.upper[level-1][index]
}

// rangeHashes returns the hashes that, with the run leaves[lo:end], rebuild
// the root of the entry tree, the same that builder.ScriptRangeHashes gives:
// level by level from the leaves up, the node just left of the run's first
// node, then the node just right of its last node, wherever the run doesn't
// already reach it.
func (e *scriptLeavesOfBlock) rangeHashes(lo, end int) ([]chainhash.Hash, error) {
	if lo < 0 || lo >= end || end > len(e.leaves) {
		return nil, errScriptRangeRun
	}
	var hashes []chainhash.Hash
	a, b, width := lo, end-1, len(e.leaves)
	for level := 0; width > 1; level++ {
		if a%2 == 1 {
			hashes = append(hashes, e.node(level, a-1))
		}
		if b%2 == 0 && b+1 < width {
			hashes = append(hashes, e.node(level, b+1))
		}
		a, b, width = a/2, b/2, (width+1)/2
	}
	return hashes, nil
}

// txBranch returns the Merkle branch of the block's transaction at index.
func (e *scriptLeavesOfBlock) txBranch(index int) []chainhash.Hash {
	return blockchain.MerkleBranch(e.merkles, index)
}

// scriptTreeUpper returns the levels of the entry tree over leaves above the
// leaves themselves, up to the root.
func scriptTreeUpper(leaves []builder.ScriptLeaf) [][]chainhash.Hash {
	if len(leaves) < 2 {
		return nil
	}
	level := make([]chainhash.Hash, len(leaves))
	for i := range leaves {
		level[i] = builder.ScriptLeafHash(&leaves[i])
	}
	var upper [][]chainhash.Hash
	for len(level) > 1 {
		next := make([]chainhash.Hash, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				next = append(next, level[i])
				continue
			}
			next = append(next, builder.ScriptNodeHash(&level[i], &level[i+1]))
		}
		upper = append(upper, next)
		level = next
	}
	return upper
}

// scriptLeavesKept is how long the block scriptFilterLeaves last rebuilt is
// kept.  A disagreement over a block is settled well within it, and a block
// nobody asks about any more is not held in memory.
const scriptLeavesKept = 2 * time.Minute

// scriptLeavesCall is one build of a block's leaves. Waiters block on done
// and then read got and err.
type scriptLeavesCall struct {
	done chan struct{}
	got  *scriptLeavesOfBlock
	err  error
}

// scriptLeavesCache keeps the leaves of the block last asked about. A build
// runs without the lock, and callers that ask for that same block while it
// is built share the one build. A caller that asks for another block, or for
// the block already kept, does not wait. One timer drops the kept block, and
// keeping a new block restarts that timer, so a replaced block is not held
// for the whole keep.
type scriptLeavesCache struct {
	mtx      sync.Mutex
	kept     *scriptLeavesOfBlock
	timer    *time.Timer
	building map[chainhash.Hash]*scriptLeavesCall
	keep     time.Duration
}

// newScriptLeavesCache returns a cache that drops its kept block after keep.
func newScriptLeavesCache(keep time.Duration) *scriptLeavesCache {
	return &scriptLeavesCache{
		building: make(map[chainhash.Hash]*scriptLeavesCall),
		keep:     keep,
	}
}

// get returns the kept entry when it is for hash. Otherwise it calls build
// without holding the cache lock, keeps a successful result in place of the
// entry kept before, and returns it. Callers that ask for hash while a build
// for that same hash is in flight wait for that build and get its result,
// error included. A build for another hash does not make them wait, and
// neither does a caller whose hash is the one kept. An error is not kept.
func (c *scriptLeavesCache) get(hash *chainhash.Hash, build func() (*scriptLeavesOfBlock, error)) (*scriptLeavesOfBlock, error) {
	// Copy the key before build runs. The caller keeps the pointer, and the
	// map entry has to be removed under the same key it was stored with.
	key := *hash
	c.mtx.Lock()
	if c.kept != nil && c.kept.block.Hash().IsEqual(&key) {
		kept := c.kept
		c.mtx.Unlock()
		return kept, nil
	}
	if call := c.building[key]; call != nil {
		c.mtx.Unlock()
		<-call.done
		return call.got, call.err
	}
	call := &scriptLeavesCall{done: make(chan struct{})}
	c.building[key] = call
	c.mtx.Unlock()

	// build runs without the lock, so a slow block read does not stall a
	// request for another block or for the one already kept. A caller that
	// found a build in flight waited on done above, also without the lock.
	got, err := build()

	c.mtx.Lock()
	delete(c.building, key)
	if err == nil {
		c.store(got)
	}
	call.got = got
	call.err = err
	c.mtx.Unlock()
	close(call.done)
	return got, err
}

// store keeps entry and restarts the one timer that drops the kept block.
// The caller holds c.mtx.
func (c *scriptLeavesCache) store(entry *scriptLeavesOfBlock) {
	if c.timer != nil {
		c.timer.Stop()
	}
	c.kept = entry
	// A stopped or replaced timer that has already fired may still run and
	// drop this newer entry. That only costs a rebuild, and is accepted.
	// The function closes over the cache, not the entry.
	c.timer = time.AfterFunc(c.keep, func() {
		c.mtx.Lock()
		c.kept = nil
		c.mtx.Unlock()
	})
}

// newScriptLeavesOfBlock returns an entry that several goroutines may read at
// once. Hash and Transactions fill the block's saved hash and wrapped
// transactions, and Hash fills each transaction's saved hash, because those
// methods write that state on first use and do not lock it. The levels of the
// entry tree above the leaves and the block's Merkle tree are built here,
// once.
func newScriptLeavesOfBlock(block *bchutil.Block, stxos []blockchain.SpentTxOut, leaves []builder.ScriptLeaf) *scriptLeavesOfBlock {
	block.Hash()
	for _, tx := range block.Transactions() {
		tx.Hash()
	}
	return &scriptLeavesOfBlock{
		block:   block,
		stxos:   stxos,
		leaves:  leaves,
		upper:   scriptTreeUpper(leaves),
		merkles: blockchain.BuildMerkleTreeStore(block.Transactions()),
	}
}
