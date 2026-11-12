// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"sync"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs/builder"
)

// scriptLeavesOfBlock is a block with the outputs it spends and the leaves of
// its script filter entry tree.
type scriptLeavesOfBlock struct {
	block  *bchutil.Block
	stxos  []blockchain.SpentTxOut
	leaves []builder.ScriptLeaf
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
// methods write that state on first use and do not lock it.
func newScriptLeavesOfBlock(block *bchutil.Block, stxos []blockchain.SpentTxOut, leaves []builder.ScriptLeaf) *scriptLeavesOfBlock {
	block.Hash()
	for _, tx := range block.Transactions() {
		tx.Hash()
	}
	return &scriptLeavesOfBlock{block: block, stxos: stxos, leaves: leaves}
}
