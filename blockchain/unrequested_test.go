// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"math/big"
	"testing"
	"time"
)

// TestUnrequestedBlock ensures a block that was not requested is only stored
// when it has at least the work of the tip and the minimum chain work, also
// when it arrives as an orphan.
func TestUnrequestedBlock(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	params := chain.chainParams

	isUnrequested := func(err error) bool {
		var rerr RuleError
		return errors.As(err, &rerr) && rerr.ErrorCode == ErrUnrequestedBlock
	}

	// A fork with less work than the tip.
	fork := procTestBlock(t, blocks[3].MsgBlock(), 3,
		CalcBlockSubsidy(4, params)-1, params.PowLimitBits, nil)
	if _, _, err := chain.ProcessBlock(fork, BFUnrequested); !isUnrequested(err) {
		t.Fatalf("unrequested low-work fork: got %v, want "+
			"ErrUnrequestedBlock", err)
	}
	if chain.index.LookupNode(fork.Hash()) != nil {
		t.Fatal("unrequested low-work fork was stored")
	}
	if _, _, err := chain.ProcessBlock(fork, BFNone); err != nil {
		t.Fatalf("requested low-work fork: %v", err)
	}

	// A block extending the tip while the chain is below the minimum
	// chain work.
	tip := chain.bestChain.Tip()
	params.MinimumChainWork = new(big.Int).Lsh(tip.workSum, 1)
	next := procTestBlock(t, blocks[5].MsgBlock(), 5,
		CalcBlockSubsidy(6, params), params.PowLimitBits, nil)
	if _, _, err := chain.ProcessBlock(next, BFUnrequested); !isUnrequested(err) {
		t.Fatalf("unrequested block below the minimum chain work: got "+
			"%v, want ErrUnrequestedBlock", err)
	}

	// An unrequested orphan is checked when its parent arrives.
	child := procTestBlock(t, next.MsgBlock(), 6,
		CalcBlockSubsidy(7, params), params.PowLimitBits, nil)
	_, isOrphan, err := chain.ProcessBlock(child, BFUnrequested)
	if err != nil || !isOrphan {
		t.Fatalf("unrequested orphan: orphan %v, err %v", isOrphan, err)
	}
	if _, _, err := chain.ProcessBlock(next, BFNone); err != nil {
		t.Fatalf("requested block: %v", err)
	}
	if chain.index.LookupNode(child.Hash()) != nil {
		t.Fatal("unrequested orphan below the minimum chain work was " +
			"stored")
	}

	// With enough work it is stored.
	params.MinimumChainWork = nil
	if _, _, err := chain.ProcessBlock(child, BFUnrequested); err != nil {
		t.Fatalf("unrequested block extending the tip: %v", err)
	}
	if chain.bestChain.Tip().hash != *child.Hash() {
		t.Fatal("unrequested block extending the tip is not the tip")
	}
}
