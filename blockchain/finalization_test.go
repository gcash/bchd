// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchutil"
)

// finalizationTestChain returns a regtest chain with the given finalization
// settings and five main chain blocks.
func finalizationTestChain(t *testing.T, depth int32, delay time.Duration) (*BlockChain, []*bchutil.Block) {
	t.Helper()
	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	chain, err := New(&Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(10),
		ExcessiveBlockSize: LegacyMaxBlockSize,
		MaxReorgDepth:      depth,
		FinalizationDelay:  delay,
	})
	if err != nil {
		t.Fatal(err)
	}
	blocks := []*bchutil.Block{bchutil.NewBlock(params.GenesisBlock)}
	for h := int32(1); h <= 5; h++ {
		block := procTestBlock(t, blocks[h-1].MsgBlock(), h-1,
			CalcBlockSubsidy(h, &params), params.PowLimitBits, nil)
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("block %d: %v", h, err)
		}
		blocks = append(blocks, block)
	}
	return chain, blocks
}

// TestFinalization ensures blocks deeper than the maximum reorg depth are
// finalized once known for the finalization delay, and that blocks forking
// the chain below the finalized block are rejected.
func TestFinalization(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Nanosecond)
	params := chain.chainParams
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[3].Hash() {
		t.Fatalf("finalized %v, want block 3", fin)
	}

	// A fork from block 2 is below the finalized block 3, one from block 3
	// is not.
	fork := procTestBlock(t, blocks[2].MsgBlock(), 2,
		CalcBlockSubsidy(3, params)-1, params.PowLimitBits, nil)
	_, _, err := chain.ProcessBlock(fork, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != ErrForkBeforeFinalized {
		t.Fatalf("fork below the finalized block: got %v, want "+
			"ErrForkBeforeFinalized", err)
	}
	fork = procTestBlock(t, blocks[3].MsgBlock(), 3,
		CalcBlockSubsidy(4, params)-1, params.PowLimitBits, nil)
	if _, _, err := chain.ProcessBlock(fork, BFNone); err != nil {
		t.Fatalf("fork above the finalized block: %v", err)
	}

	// Invalidating the finalized block moves finalization back.
	if err := chain.InvalidateBlock(blocks[3].Hash()); err != nil {
		t.Fatal(err)
	}
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v after invalidating it, want block 2", fin)
	}
}

// TestFinalizationDelay ensures nothing is finalized before the node has
// been running, and the block has been known, for the finalization delay.
func TestFinalizationDelay(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Hour)
	if chain.finalizedNode != nil {
		t.Fatal("finalized a block before the startup delay")
	}

	// Blocks received this session are not old enough, while blocks
	// loaded from disk, here only genesis, have no receive time.
	chain.startTime -= 2 * 3600
	tip := chain.bestChain.Tip()
	chain.maybeFinalize(tip)
	if fin := chain.finalizedNode; fin == nil || fin.height != 0 {
		t.Fatalf("finalized %v, want genesis", fin)
	}
	chain.index.LookupNode(blocks[1].Hash()).received = 0
	chain.index.LookupNode(blocks[2].Hash()).received = 0
	chain.maybeFinalize(tip)
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v, want block 2", fin)
	}

	// A negative depth disables finalization.
	chain.finalizedNode = nil
	chain.maxReorgDepth = -1
	chain.maybeFinalize(tip)
	if chain.finalizedNode != nil {
		t.Fatal("finalized a block with finalization disabled")
	}
}

// TestFinalizationMinimumChainWork ensures nothing is finalized while the
// chain has less than the minimum chain work.
func TestFinalizationMinimumChainWork(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Nanosecond)
	tip := chain.bestChain.Tip()

	chain.finalizedNode = nil
	chain.chainParams.MinimumChainWork = new(big.Int).Add(tip.workSum,
		big.NewInt(1))
	chain.maybeFinalize(tip)
	if chain.finalizedNode != nil {
		t.Fatalf("finalized %v below the minimum chain work",
			chain.finalizedNode)
	}

	chain.chainParams.MinimumChainWork = new(big.Int).Set(tip.workSum)
	chain.maybeFinalize(tip)
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[3].Hash() {
		t.Fatalf("finalized %v, want block 3", fin)
	}
}

// TestReconsiderBeforeFinalized ensures reconsidering a block that forks the
// chain below the finalized block moves finalization back to the fork point.
func TestReconsiderBeforeFinalized(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Nanosecond)
	params := chain.chainParams

	// Invalidate block 3 and finalize a fork from block 2 instead.
	if err := chain.InvalidateBlock(blocks[3].Hash()); err != nil {
		t.Fatal(err)
	}
	prev := blocks[2]
	for h := int32(3); h <= 6; h++ {
		fork := procTestBlock(t, prev.MsgBlock(), h-1,
			CalcBlockSubsidy(h, params)-1, params.PowLimitBits, nil)
		if _, _, err := chain.ProcessBlock(fork, BFNone); err != nil {
			t.Fatalf("fork block %d: %v", h, err)
		}
		prev = fork
	}
	if fin := chain.finalizedNode; fin == nil || fin.height != 4 {
		t.Fatalf("finalized %v, want the fork block at height 4", fin)
	}

	if err := chain.ReconsiderBlock(blocks[3].Hash()); err != nil {
		t.Fatalf("reconsider block 3: %v", err)
	}
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v after reconsidering, want block 2", fin)
	}
}

// TestFindBlockToFinalizeLongChain ensures the block to finalize is the
// newest block that is deep enough and known for the delay, also when many
// recent blocks lie between it and the tip, as during a sync.
func TestFindBlockToFinalizeLongChain(t *testing.T) {
	chain := newFakeChain(&chaincfg.RegressionNetParams)
	chain.maxReorgDepth = DefaultMaxReorgDepth
	chain.finalizationDelay = time.Hour
	now := time.Now().Unix()
	chain.startTime = now - 2*3600

	// Blocks received one second apart, the newest just now.
	const n = 20000
	node := chain.bestChain.Genesis()
	for i := 1; i <= n; i++ {
		node = newFakeNode(node, 1, chain.chainParams.PowLimitBits,
			time.Unix(int64(i), 0))
		node.received = uint32(now - n + int64(i))
		chain.bestChain.SetTip(node)
	}
	tip := chain.bestChain.Tip()

	// The reference walks down from the candidate.
	want := tip.Ancestor(tip.height - chain.maxReorgDepth)
	for now < int64(want.received)+3600 {
		want = want.parent
	}
	for i := 0; i < 2; i++ {
		chain.maybeFinalize(tip)
		if chain.finalizedNode != want {
			t.Fatalf("finalized %v, want height %d", chain.finalizedNode,
				want.height)
		}
	}

	// Nothing is finalized while disabled.
	chain.finalizedNode = nil
	chain.maxReorgDepth = -1
	chain.maybeFinalize(tip)
	if chain.finalizedNode != nil {
		t.Fatal("finalized a block with finalization disabled")
	}
}

// TestFinalizeBlock ensures blocks can be finalized by hand, also on a side
// chain, which then becomes the main chain.
func TestFinalizeBlock(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	params := chain.chainParams
	if hash := chain.FinalizedBlockHash(); hash != nil {
		t.Fatalf("finalized %v before finalizing anything", hash)
	}

	// A side chain from block 2 with less work than the main chain.
	prev := blocks[2]
	var fork []*bchutil.Block
	for h := int32(3); h <= 4; h++ {
		block := procTestBlock(t, prev.MsgBlock(), h-1,
			CalcBlockSubsidy(h, params)-1, params.PowLimitBits, nil)
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("fork block %d: %v", h, err)
		}
		fork = append(fork, block)
		prev = block
	}

	if err := chain.FinalizeBlock(blocks[2].Hash()); err != nil {
		t.Fatalf("finalize main chain block 2: %v", err)
	}
	if hash := chain.FinalizedBlockHash(); hash == nil || *hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v, want block 2", hash)
	}

	// Finalizing the side chain's tip switches to it and invalidates the
	// main chain after the fork point.  Both happen before the first
	// notification, while the chain lock is released.
	var checked bool
	chain.Subscribe(func(n *Notification) {
		if n.Type != NTBlockDisconnected || checked {
			return
		}
		checked = true
		if hash := chain.FinalizedBlockHash(); hash == nil || *hash != *fork[1].Hash() {
			t.Errorf("finalized %v during the reorganization, want the "+
				"side chain block", hash)
		}
		node := chain.index.LookupNode(blocks[4].Hash())
		if !chain.index.NodeStatus(node).KnownInvalid() {
			t.Error("old main chain is not invalid during the " +
				"reorganization")
		}
	})
	if err := chain.FinalizeBlock(fork[1].Hash()); err != nil {
		t.Fatalf("finalize side chain block: %v", err)
	}
	if tip := chain.bestChain.Tip(); tip.hash != *fork[1].Hash() {
		t.Fatalf("tip %v, want the finalized side chain block", tip.hash)
	}
	if hash := chain.FinalizedBlockHash(); hash == nil || *hash != *fork[1].Hash() {
		t.Fatalf("finalized %v, want the side chain block", hash)
	}
	if !checked {
		t.Fatal("no block was disconnected")
	}
	node := chain.index.LookupNode(blocks[3].Hash())
	if !chain.index.NodeStatus(node).KnownInvalid() {
		t.Fatal("old main chain block after the fork point is not invalid")
	}

	// Blocks that conflict with the finalized block are refused, and
	// ancestors of it are already final.
	if err := chain.FinalizeBlock(blocks[5].Hash()); err == nil {
		t.Fatal("finalized a block that conflicts with the finalized block")
	}
	if err := chain.FinalizeBlock(blocks[1].Hash()); err != nil {
		t.Fatalf("finalize an ancestor of the finalized block: %v", err)
	}
	if hash := chain.FinalizedBlockHash(); *hash != *fork[1].Hash() {
		t.Fatalf("finalized %v after finalizing an ancestor", hash)
	}
	child := procTestBlock(t, blocks[5].MsgBlock(), 5,
		CalcBlockSubsidy(6, params), params.PowLimitBits, nil)
	_, _, err := chain.ProcessBlock(child, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) {
		t.Fatalf("block on the old main chain: got %v, want a rule error",
			err)
	}
}
