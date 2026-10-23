// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchutil"
)

// disconnectCountIndexer is an index manager that records the chain's count
// of disconnections each time a block is disconnected, which it hears of in
// the database transaction of the disconnection.
type disconnectCountIndexer struct {
	chain  *BlockChain
	counts []uint64
}

func (m *disconnectCountIndexer) Init(chain *BlockChain, _ <-chan struct{}) error {
	m.chain = chain
	return nil
}

func (m *disconnectCountIndexer) ConnectBlock(database.Tx, *bchutil.Block, []SpentTxOut) error {
	return nil
}

func (m *disconnectCountIndexer) DisconnectBlock(database.Tx, *bchutil.Block, []SpentTxOut) error {
	m.counts = append(m.counts, m.chain.DisconnectCount())
	return nil
}

// TestDisconnectCount ensures DisconnectCount counts each block disconnected
// from the main chain, raised before the disconnection changes the database,
// and that connecting blocks leaves it alone.
func TestDisconnectCount(t *testing.T) {
	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	indexer := &disconnectCountIndexer{}
	chain, err := New(&Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(10),
		IndexManager:       indexer,
		ExcessiveBlockSize: LegacyMaxBlockSize,
		MaxReorgDepth:      -1,
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
	if n := chain.DisconnectCount(); n != 0 {
		t.Fatalf("%d disconnections counted after connecting blocks", n)
	}

	// Invalidating block 4 disconnects blocks 5 and 4, each counted before
	// the index manager hears of it.
	if err := chain.InvalidateBlock(blocks[4].Hash()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(indexer.counts, []uint64{1, 2}) {
		t.Errorf("counts seen by the index manager %v, want [1 2]",
			indexer.counts)
	}

	// A block replacing them is connected without being counted.
	block := procTestBlock(t, blocks[3].MsgBlock(), 3,
		CalcBlockSubsidy(4, &params)-1, params.PowLimitBits, nil)
	if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
		t.Fatal(err)
	}
	if tip := chain.BestSnapshot().Hash; tip != *block.Hash() {
		t.Fatalf("tip %v, want the replacement", tip)
	}
	if n := chain.DisconnectCount(); n != 2 {
		t.Errorf("%d disconnections counted, want 2", n)
	}
}
