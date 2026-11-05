// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"
	"time"

	"github.com/gcash/bchutil"
)

// TestFailedInvalidateKeepsBlockValid ensures a block that can't be taken off
// the main chain isn't left marked invalid while it is still there, as a block
// the chain has connected can't be invalid, and its marks would make
// invalidateblock refuse a second try.
func TestFailedInvalidateKeepsBlockValid(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	tip := blocks[len(blocks)-1]
	pruneBlockData(t, chain.db, tip.Hash())

	if err := chain.InvalidateBlock(tip.Hash()); err == nil {
		t.Fatal("invalidating a block whose data is gone succeeded")
	}
	if got := chain.bestChain.Tip(); got.hash != *tip.Hash() {
		t.Fatalf("tip %v, want the block still connected", got.hash)
	}
	node := chain.index.LookupNode(tip.Hash())
	status := chain.index.NodeStatus(node)
	if status.KnownInvalid() || !status.KnownValid() {
		t.Fatalf("the connected block has status %v", status)
	}

	// The mark taken back is written, in case the reorganization wrote it.
	chain.index.RLock()
	dirty := len(chain.index.dirty)
	chain.index.RUnlock()
	if dirty != 0 {
		t.Fatalf("%d block index entries aren't written", dirty)
	}
}

// TestFailedFinalizeWritesStatus ensures the marks a failed finalization takes
// back are written, in case the reorganization wrote them.
func TestFailedFinalizeWritesStatus(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	params := chain.chainParams

	// A side chain from block 4 whose block can't be read back.
	side := procTestBlock(t, blocks[4].MsgBlock(), 4,
		CalcBlockSubsidy(5, params)-1, params.PowLimitBits, nil)
	if _, _, err := chain.ProcessBlock(side, BFNone); err != nil {
		t.Fatal(err)
	}
	pruneBlockData(t, chain.db, side.Hash())

	if err := chain.FinalizeBlock(side.Hash()); err == nil {
		t.Fatal("finalizing a block whose data is gone succeeded")
	}
	node := chain.index.LookupNode(blocks[5].Hash())
	if chain.index.NodeStatus(node).KnownInvalid() {
		t.Fatal("the connected block was left marked invalid")
	}
	chain.index.RLock()
	dirty := len(chain.index.dirty)
	chain.index.RUnlock()
	if dirty != 0 {
		t.Fatalf("%d block index entries aren't written", dirty)
	}
}

// TestInvalidatedBlockMarkedDuringReorganize ensures the block is invalid while
// the reorganization sends its notifications, with the chain lock released, so
// that headers arriving meanwhile can't extend it.
func TestInvalidatedBlockMarkedDuringReorganize(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	tip := blocks[len(blocks)-1]
	notified := false
	chain.Subscribe(func(n *Notification) {
		if n.Type != NTBlockDisconnected {
			return
		}
		notified = true
		node := chain.index.LookupNode(tip.Hash())
		if !chain.index.NodeStatus(node).KnownInvalid() {
			t.Error("the block isn't marked during the reorganization")
		}
	})
	if err := chain.InvalidateBlock(tip.Hash()); err != nil {
		t.Fatal(err)
	}
	if !notified {
		t.Fatal("no notification of the disconnected block")
	}
}

// TestFailedReconsiderKeepsMarks ensures a block that can't be read back keeps
// its marks, with the blocks built on it, rather than a chain being marked
// valid that nothing validated again.
func TestFailedReconsiderKeepsMarks(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	first, next := blocks[len(blocks)-2], blocks[len(blocks)-1]
	if err := chain.InvalidateBlock(first.Hash()); err != nil {
		t.Fatal(err)
	}
	pruneBlockData(t, chain.db, first.Hash())

	if err := chain.ReconsiderBlock(first.Hash()); err == nil {
		t.Fatal("reconsidering a block whose data is gone succeeded")
	}
	if !chain.index.NodeStatus(chain.index.LookupNode(first.Hash())).KnownInvalid() {
		t.Fatal("the block lost its mark")
	}
	if !chain.index.NodeStatus(chain.index.LookupNode(next.Hash())).KnownInvalid() {
		t.Fatal("the block built on it lost its mark")
	}
}

// TestInvalidChainNotMainChain ensures a block with more work on a chain with
// an invalid ancestor is reported as off the main chain, since the chain can't
// reorganize to it.
func TestInvalidChainNotMainChain(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	params := chain.chainParams
	tip := chain.bestChain.Tip()

	// A side chain from block 3, the first of whose blocks alone is marked
	// invalid, so that the block on top of it is accepted.
	prev := blocks[3]
	var first *bchutil.Block
	for h := int32(4); h <= 5; h++ {
		block := procTestBlock(t, prev.MsgBlock(), h-1,
			CalcBlockSubsidy(h, params)-1, params.PowLimitBits, nil)
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("side chain block %d: %v", h, err)
		}
		if first == nil {
			first = block
		}
		prev = block
	}
	chain.index.SetStatusFlags(chain.index.LookupNode(first.Hash()), statusValidateFailed)

	block := procTestBlock(t, prev.MsgBlock(), 5, CalcBlockSubsidy(6, params)-1,
		params.PowLimitBits, nil)
	isMainChain, _, err := chain.ProcessBlock(block, BFNone)
	if err != nil {
		t.Fatal(err)
	}
	if isMainChain {
		t.Fatal("the block was reported on the main chain")
	}
	if got := chain.bestChain.Tip(); got != tip {
		t.Fatalf("tip %v, want the old tip", got.hash)
	}
	status := chain.index.NodeStatus(chain.index.LookupNode(block.Hash()))
	if !status.KnownInvalid() {
		t.Fatalf("the block has status %v", status)
	}
}
