package ffldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// pruneTestBlock returns a unique block of roughly 600 bytes with the given
// height set.
func pruneTestBlock(nonce uint32, height int32) *bchutil.Block {
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  make([]byte, 500),
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{0x51}})
	b := bchutil.NewBlock(&wire.MsgBlock{
		Header:       wire.BlockHeader{Nonce: nonce, Timestamp: time.Unix(1, 0)},
		Transactions: []*wire.MsgTx{tx},
	})
	b.SetHeight(height)
	return b
}

// createPruneTestDB creates a database that stores one test block per
// block file.
func createPruneTestDB(t *testing.T, dir string) *db {
	t.Helper()
	idb, err := database.Create(dbType, dir, blockDataNet)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pdb := idb.(*db)
	pdb.store.maxBlockFileSize = 1024
	return pdb
}

func storeTestBlock(t *testing.T, pdb *db, b *bchutil.Block) {
	t.Helper()
	err := pdb.Update(func(tx database.Tx) error {
		return tx.StoreBlock(b)
	})
	if err != nil {
		t.Fatalf("store block %d: %v", b.Height(), err)
	}
}

// runWithTimeout fails the test when fn does not return within d.
func runWithTimeout(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { fn(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not complete within %v", what, d)
	}
}

// TestPruneManyFilesDoesNotDeadlock ensures pruning more block files than
// the open file limit does not leave read locks behind that deadlock the
// next file eviction.
func TestPruneManyFilesDoesNotDeadlock(t *testing.T) {
	pdb := createPruneTestDB(t, filepath.Join(t.TempDir(), "db"))

	const numBlocks = 2 * maxOpenFiles
	for i := 1; i <= numBlocks; i++ {
		storeTestBlock(t, pdb, pruneTestBlock(uint32(i), int32(i)))
	}

	// Read the blocks so their files are in the open file cache.
	err := pdb.View(func(tx database.Tx) error {
		for i := 1; i <= numBlocks; i++ {
			b := pruneTestBlock(uint32(i), int32(i))
			if _, err := tx.FetchBlock(b.Hash()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("FetchBlock: %v", err)
	}

	// Prune one file at a time like the chain does on each new block.
	for h := uint32(2); h <= numBlocks; h++ {
		runWithTimeout(t, 5*time.Second, "DeleteBlocks", func() {
			err = pdb.Update(func(tx database.Tx) error {
				return tx.DeleteBlocks(h)
			})
		})
		if err != nil {
			t.Fatalf("DeleteBlocks(%d): %v", h, err)
		}
	}

	// Pruned files must be gone from the open file cache as well.
	s := pdb.store
	s.obfMutex.RLock()
	s.fbhMutex.RLock()
	for n := range s.openBlockFiles {
		if _, ok := s.fileBlockHeights[n]; !ok {
			t.Errorf("pruned block file %d is still cached as open", n)
		}
	}
	s.fbhMutex.RUnlock()
	s.obfMutex.RUnlock()

	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
