// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
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
