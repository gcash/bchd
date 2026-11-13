// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchutil"
)

// TestPruneWindowTicks ensures a pruned node keeps the blocks less than
// prune depth * 600 ticks below the tip, the newest block at least that deep
// included, serves blocks from that block on and headers below it, and
// flushes the utxo set before its last flush leaves that window.  The prune
// depth is 3, so the window is 3 blocks before the upgrade to 1-minute blocks
// and 30 blocks after it.  CHIP-2025-03 sets the NODE_NETWORK_LIMITED and
// prune window to 288 * 600 ticks.
func TestPruneWindowTicks(t *testing.T) {
	// Blocks are a minute apart from 1700000000, so with the activation
	// time below block 20 is the activation block and block 21 the first
	// 1-minute block.
	tests := []struct {
		name       string
		activation uint64
		keep       map[int32]int32 // tip height -> deepest kept height
	}{
		{"never active", upgrade2027Never, map[int32]int32{
			2: 0, 3: 0, 4: 1, 20: 17, 30: 27, 150: 147,
		}},
		{"active", 1700000000 + 60*14, map[int32]int32{
			2: 0, 3: 0, 4: 1, 20: 17, 21: 17, 25: 17, 30: 18,
			50: 20, 51: 21, 150: 120,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := chaincfg.RegressionNetParams
			params.Upgrade2027ActivationTime = test.activation
			db, err := database.Create("ffldb",
				filepath.Join(t.TempDir(), "db"), params.Net)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			chain, err := New(&Config{
				DB:                 db,
				ChainParams:        &params,
				TimeSource:         NewMedianTime(),
				SigCache:           txscript.NewSigCache(10),
				UtxoCacheMaxSize:   250 * 1024 * 1024,
				ExcessiveBlockSize: LegacyMaxBlockSize,
				Prune:              true,
				PruneDepth:         3,
			})
			if err != nil {
				t.Fatal(err)
			}

			blocks := []*bchutil.Block{bchutil.NewBlock(params.GenesisBlock)}
			for h := int32(1); h <= 150; h++ {
				block := procTestBlock(t, blocks[h-1].MsgBlock(), h-1,
					childSubsidy(chain, blocks[h-1]),
					params.PowLimitBits, nil)
				if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
					t.Fatalf("block %d: %v", h, err)
				}
				blocks = append(blocks, block)
				checkPruneWindow(t, chain, blocks, test.keep)
			}

			n := chain.index.LookupNode(blocks[20].Hash())
			active := n.nextInterval == 60 && n.parent.nextInterval == 600
			if active != (test.activation != upgrade2027Never) {
				t.Fatalf("block 20 activation block %v", active)
			}
		})
	}
}

// checkPruneWindow checks the prune height, the kept block data and spend
// journals, the served blocks and headers, and the last utxo flush of a pruned chain against the deepest block keep
// says it keeps at the tip height, if listed.
func checkPruneWindow(t *testing.T, chain *BlockChain, blocks []*bchutil.Block,
	keep map[int32]int32) {

	t.Helper()
	tip := chain.bestChain.Tip()

	// The last flush is never on or below the deepest kept block, so the
	// spend journals needed to recover from it are kept.
	if kept := chain.bestChain.NodeByTickDepth(chain.pruneTicks()); kept != nil {
		flushed := chain.bestChain.FindFork(
			chain.index.LookupNode(&chain.utxoCache.lastFlushHash))
		if flushed.height <= kept.height {
			t.Fatalf("tip %d: last flush at %d, at or below the "+
				"deepest kept block %d", tip.height, flushed.height,
				kept.height)
		}
	}

	want, ok := keep[tip.height]
	if !ok {
		return
	}
	var pruneHeight uint32
	err := chain.db.View(func(tx database.Tx) error {
		pruneHeight = dbFetchPruneHeight(tx)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pruneHeight != uint32(want) {
		t.Fatalf("tip %d: prune height %d, want %d", tip.height,
			pruneHeight, want)
	}

	// The block data and spend journal of the deepest kept block stay and
	// the spend journal below it goes.  Block data goes by whole block
	// file, so the test chain, which fits in one file, keeps all of it.
	err = chain.db.View(func(tx database.Tx) error {
		for _, h := range []int32{want, tip.height} {
			if _, err := tx.FetchBlock(blocks[h].Hash()); err != nil {
				t.Errorf("tip %d: block %d: %v", tip.height, h, err)
			}
		}
		if want == 0 {
			return nil
		}
		spend := tx.Metadata().Bucket(spendJournalBucketName)
		if spend.Get(blocks[want].Hash()[:]) == nil {
			t.Errorf("tip %d: no spend journal for the deepest kept "+
				"block %d", tip.height, want)
		}
		if spend.Get(blocks[want-1].Hash()[:]) != nil {
			t.Errorf("tip %d: spend journal for %d below the window "+
				"kept", tip.height, want-1)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if t.Failed() {
		t.FailNow()
	}

	// Blocks are served after a locator block in the window and not after
	// one below it, while headers are served after both.
	locate := func(h int32) ([]chainhash.Hash, []chainhash.Hash) {
		locator := BlockLocator{blocks[h].Hash()}
		hashes := chain.LocateBlocks(locator, &chainhash.Hash{}, 500)
		var headers []chainhash.Hash
		for _, hdr := range chain.LocateHeaders(locator, &chainhash.Hash{}) {
			headers = append(headers, hdr.BlockHash())
		}
		return hashes, headers
	}
	hashes, headers := locate(want)
	if want < tip.height && (len(hashes) == 0 || hashes[0] != *blocks[want+1].Hash() ||
		len(headers) == 0) {

		t.Fatalf("tip %d: locator at the deepest kept block %d got %d "+
			"blocks and %d headers", tip.height, want, len(hashes),
			len(headers))
	}
	if want > 0 {
		hashes, headers = locate(want - 1)
		if len(hashes) != 0 || len(headers) == 0 {
			t.Fatalf("tip %d: locator below the window at %d got %d "+
				"blocks and %d headers, want none and some",
				tip.height, want-1, len(hashes), len(headers))
		}
	}
}
