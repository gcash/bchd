// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/fullblocktests"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
)

// TestUpgrade2027Seam runs the blocks around the activation block of the
// upgrade to 1-minute blocks (CHIP-2025-03) through ProcessBlock: the ASERT
// bits and minimum difficulty threshold, subsidy, coinbase shape, block size,
// coinbase maturity, lock time and relative lock of the first 60-tick blocks,
// and the same heights of a chain that never activates the upgrade.
//
// ABLA is inactive on the chain, so the block size limit is the excessive
// block size scaled to the interval.  TestABLASeam covers the limit with ABLA
// active.  Locks are only released on the 60-tick grid of the blocks, so only
// errors of a whole interval show.
func TestUpgrade2027Seam(t *testing.T) {
	const activationHeight = 110
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprintf("active=%v", active), func(t *testing.T) {
			tests, seam, err := fullblocktests.GenerateUpgrade2027Blocks(
				activationHeight, active)
			if err != nil {
				t.Fatalf("failed to generate tests: %v", err)
			}

			params := chaincfg.RegressionNetParams
			params.MagneticAnonomalyForkHeight = 100000000
			params.Upgrade9ForkHeight = 100000000
			params.ABLAForkHeight = 100000000
			params.CSVHeight = 1
			params.NoDifficultyAdjustment = false
			params.AsertDifficultyAnchorHeight = 0
			params.AsertDifficultyAnchorParentTimestamp =
				seam.AsertAnchorParentTime
			params.AsertDifficultyAnchorBits = seam.AsertAnchorBits
			params.Upgrade2027ActivationTime = seam.ActivationTime

			chain, teardown, err := chainSetup(
				fmt.Sprintf("upgrade2027seam%v", active), &params,
				seam.ExcessiveBlockSize)
			if err != nil {
				t.Fatalf("failed to set up chain: %v", err)
			}
			defer teardown()

			runFullBlockTests(t, chain, tests)

			// The activation block is the last 600-tick block.
			snap := chain.BestSnapshot()
			activation, ok := snap.TickSchedule.Active()
			if ok != active || (active &&
				activation != seam.ActivationHeight) {

				t.Fatalf("activation block %d (active %v), want "+
					"%d (active %v)", activation, ok,
					seam.ActivationHeight, active)
			}
		})
	}
}

// runFullBlockTests processes the blocks of the passed tests in order and
// checks each is accepted or rejected as the test expects.
func runFullBlockTests(t *testing.T, chain *blockchain.BlockChain,
	tests [][]fullblocktests.TestInstance) {

	t.Helper()
	for testNum, test := range tests {
		for itemNum, item := range test {
			switch item := item.(type) {
			case fullblocktests.AcceptedBlock:
				block := bchutil.NewBlock(item.Block)
				block.SetHeight(item.Height)
				isMainChain, isOrphan, err := chain.ProcessBlock(
					block, blockchain.BFNone)
				if err != nil {
					t.Fatalf("block %q (height %d) should "+
						"have been accepted: %v", item.Name,
						item.Height, err)
				}
				if isMainChain != item.IsMainChain ||
					isOrphan != item.IsOrphan {

					t.Fatalf("block %q (height %d): main "+
						"chain %v, orphan %v, want %v, %v",
						item.Name, item.Height, isMainChain,
						isOrphan, item.IsMainChain,
						item.IsOrphan)
				}

			case fullblocktests.RejectedBlock:
				block := bchutil.NewBlock(item.Block)
				block.SetHeight(item.Height)
				_, _, err := chain.ProcessBlock(block,
					blockchain.BFNone)
				var rerr blockchain.RuleError
				if !errors.As(err, &rerr) {
					t.Fatalf("block %q (height %d) returned "+
						"%v, want a RuleError", item.Name,
						item.Height, err)
				}
				if rerr.ErrorCode != item.RejectCode {
					t.Fatalf("block %q (height %d) rejected "+
						"with %v, want %v: %v", item.Name,
						item.Height, rerr.ErrorCode,
						item.RejectCode, err)
				}

			default:
				t.Fatalf("test #%d, item #%d is not one of the "+
					"supported test instance types -- got "+
					"type: %T", testNum, itemNum, item)
			}
		}
	}
}
