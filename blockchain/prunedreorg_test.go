// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchutil"
)

// TestReorgPrunedSideChain ensures a pruned node does not reorganize onto a
// chain with more work holding a block whose data it pruned, and stays on its
// own chain without stopping, as Bitcoin Cash Node drops such a chain from the
// candidates for the best chain (validation.cpp:2736-2790).  Data the node must
// have still stops it, as in Bitcoin Cash Node: a side chain block missing on
// a node that is not pruned (validation.cpp:2537), a main chain block it has to
// disconnect (validation.cpp:2852-2862), and a block that fails to read for
// another reason.
func TestReorgPrunedSideChain(t *testing.T) {
	tests := []struct {
		name     string
		pruned   bool
		damage   string
		shutdown bool
	}{
		{"pruned side block", true, "side", false},
		{"missing side block, not pruned", false, "side", true},
		{"pruned main chain block", true, "main", true},
		{"unreadable side block", true, "location", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := ablaTestParams()
			db, err := database.Create("ffldb",
				filepath.Join(t.TempDir(), "db"), params.Net)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			chain, err := New(&Config{
				DB:                 db,
				ChainParams:        params,
				TimeSource:         NewMedianTime(),
				SigCache:           txscript.NewSigCache(1000),
				UtxoCacheMaxSize:   1 << 20,
				ExcessiveBlockSize: 1000000,
				Prune:              test.pruned,
				PruneDepth:         1000,
			})
			if err != nil {
				t.Fatal(err)
			}
			chain.TstSetCoinbaseMaturity(1)

			// The main chain has three blocks, and a side chain
			// forks after the first with two, as much work.
			blocks := ablaTestMine(t, chain, 3, 2000)
			var side []*bchutil.Block
			prev := blocks[0].MsgBlock()
			for h := int32(2); h <= 3; h++ {
				block := ablaTestBlock(t, prev, h, 2001, params)
				if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
					t.Fatalf("side block %d: %v", h, err)
				}
				side = append(side, block)
				prev = block.MsgBlock()
			}

			switch test.damage {
			case "side":
				pruneBlockData(t, db, side[1].Hash())
			case "main":
				pruneBlockData(t, db, blocks[1].Hash())
			case "location":
				// The block's location points past the end of
				// its block file, so reading it is a short read.
				err := db.Update(func(dbTx database.Tx) error {
					blockIdx := dbTx.Metadata().Bucket([]byte("ffldb-blockidx"))
					row := bytes.Clone(blockIdx.Get(side[1].Hash()[:]))
					binary.LittleEndian.PutUint32(row[4:8], 1<<30)
					return blockIdx.Put(side[1].Hash()[:], row)
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			// The next side block gives the side chain more work.
			next := ablaTestBlock(t, prev, 4, 2001, params)
			isMainChain, _, err := chain.ProcessBlock(next, BFNone)
			shutdown := false
			select {
			case <-chain.RequestedProcessShutdown():
				shutdown = true
			default:
			}
			if shutdown != test.shutdown {
				t.Fatalf("shutdown requested %v, want %v (error %v)",
					shutdown, test.shutdown, err)
			}
			tip := chain.BestSnapshot().Hash
			if test.shutdown {
				var dbErr database.Error
				if !errors.As(err, &dbErr) {
					t.Fatalf("got %v, want a database error", err)
				}
				return
			}
			if err != nil || isMainChain {
				t.Fatalf("block on the pruned side chain: main chain %v, "+
					"error %v", isMainChain, err)
			}
			if tip != *blocks[2].Hash() {
				t.Fatalf("tip %v, want the main chain tip %v", tip,
					blocks[2].Hash())
			}
			if chain.index.LookupNode(next.Hash()) == nil {
				t.Fatal("the block on the pruned side chain was not stored")
			}
		})
	}
}
