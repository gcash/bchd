// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
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
		legacySubsidy(4, params)-1, params.PowLimitBits, nil)
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
		legacySubsidy(6, params), params.PowLimitBits, nil)
	if _, _, err := chain.ProcessBlock(next, BFUnrequested); !isUnrequested(err) {
		t.Fatalf("unrequested block below the minimum chain work: got "+
			"%v, want ErrUnrequestedBlock", err)
	}

	// An unrequested orphan is checked when its parent arrives.
	child := procTestBlock(t, next.MsgBlock(), 6,
		legacySubsidy(7, params), params.PowLimitBits, nil)
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

// TestUnrequestedBlockTicksAhead ensures an unrequested block may be 172,800
// ticks above the tip and no more: 288 10-minute blocks before the upgrade to
// 1-minute blocks, 2,880 1-minute blocks after it, and a mix of both when the
// tip is the last 10-minute block or the one before it.
func TestUnrequestedBlockTicksAhead(t *testing.T) {
	// The less-work and minimum-work rejections share the error code.
	isTicksAhead := func(err error) bool {
		var rerr RuleError
		return errors.As(err, &rerr) &&
			rerr.ErrorCode == ErrUnrequestedBlock &&
			strings.Contains(rerr.Description, "ticks ahead")
	}

	activation := uint64(1700000000 + 600*45)
	tests := []struct {
		name       string
		activation uint64
		tipHeight  func(activationHeight int32) int32
		ahead      int32
	}{{
		name:       "never active",
		activation: upgrade2027Never,
		tipHeight:  func(int32) int32 { return 100 },
		ahead:      288,
	}, {
		name:       "tip before the activation block",
		activation: activation,
		tipHeight:  func(ha int32) int32 { return ha - 1 },
		ahead:      1 + 2870,
	}, {
		name:       "tip is the activation block",
		activation: activation,
		tipHeight:  func(ha int32) int32 { return ha },
		ahead:      2880,
	}, {
		name:       "tip after the activation block",
		activation: activation,
		tipHeight:  func(ha int32) int32 { return ha + 1 },
		ahead:      2880,
	}}
	for _, test := range tests {
		nodes := tickTestChain(nil, regularGaps(3200), test.activation)
		activationHeight := int32(-1)
		for _, node := range nodes {
			if node.nextInterval == upgrade2027BlockInterval {
				activationHeight = node.height
				break
			}
		}
		if (activationHeight >= 0) != (test.activation != upgrade2027Never) {
			t.Fatalf("%s: activation block at height %d", test.name,
				activationHeight)
		}
		tip := nodes[test.tipHeight(activationHeight)]

		params := chaincfg.RegressionNetParams
		params.MinimumChainWork = nil
		chain := newFakeChain(&params)
		chain.bestChain = newChainView(tip)

		check := func(height int32) error {
			prev := nodes[height-1]
			header := wire.BlockHeader{PrevBlock: prev.hash}
			block := bchutil.NewBlock(&wire.MsgBlock{Header: header})
			return chain.checkUnrequestedBlock(block, prev)
		}
		last := tip.height + test.ahead
		if err := check(last); err != nil {
			t.Fatalf("%s: block %d ahead of the tip: %v", test.name,
				test.ahead, err)
		}
		if err := check(last + 1); !isTicksAhead(err) {
			t.Fatalf("%s: block %d ahead of the tip: got %v, want "+
				"ErrUnrequestedBlock for ticks ahead", test.name,
				test.ahead+1, err)
		}
	}
}
