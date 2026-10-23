package ffldb

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

// TestPruneDeleteFailureReleasesLock ensures a failed block file removal
// does not leave the file heights lock held.
func TestPruneDeleteFailureReleasesLock(t *testing.T) {
	pdb := createPruneTestDB(t, filepath.Join(t.TempDir(), "db"))
	for i := 1; i <= 4; i++ {
		storeTestBlock(t, pdb, pruneTestBlock(uint32(i), int32(i)))
	}

	s := pdb.store
	deleteFile := s.deleteFileFunc
	s.deleteFileFunc = func(uint32) error { return errors.New("delete failed") }
	prune := func(h uint32) (err error) {
		runWithTimeout(t, 5*time.Second, "DeleteBlocks", func() {
			err = pdb.Update(func(tx database.Tx) error {
				return tx.DeleteBlocks(h)
			})
		})
		return err
	}
	if err := prune(3); err == nil {
		t.Fatal("expected DeleteBlocks to fail")
	}

	s.deleteFileFunc = deleteFile
	if err := prune(3); err != nil {
		t.Fatalf("DeleteBlocks after a failed removal: %v", err)
	}
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestPruneReadDoesNotReopenFile ensures a read of a block in a block file
// being pruned can't open the file again between the prune closing the file's
// handle and removing the file.  On Windows a file open for reading can't be
// removed, so such a read failed the prune, and the block being connected with
// it.  The read is made from the removal: were the open files lock released
// by then, it would open the file again there and then, which the test sees in
// the open files cache; held, the read waits and finds the file removed.
func TestPruneReadDoesNotReopenFile(t *testing.T) {
	t.Run("open file", func(t *testing.T) {
		testPruneReadDoesNotReopenFile(t, true)
	})
	t.Run("closed file", func(t *testing.T) {
		testPruneReadDoesNotReopenFile(t, false)
	})
}

// testPruneReadDoesNotReopenFile is TestPruneReadDoesNotReopenFile for a block
// file whose handle is open when the prune starts, as for a block just served,
// or not.
func testPruneReadDoesNotReopenFile(t *testing.T, open bool) {
	pdb := createPruneTestDB(t, filepath.Join(t.TempDir(), "db"))
	defer pdb.Close()
	var blocks []*bchutil.Block
	for i := 1; i <= 4; i++ {
		block := pruneTestBlock(uint32(i), int32(i))
		storeTestBlock(t, pdb, block)
		blocks = append(blocks, block)
	}

	// The first block is in file 0.
	read := func() error {
		return pdb.View(func(tx database.Tx) error {
			_, err := tx.FetchBlock(blocks[0].Hash())
			return err
		})
	}
	s := pdb.store
	if open {
		if err := read(); err != nil {
			t.Fatal(err)
		}
	}
	s.obfMutex.RLock()
	_, cached := s.openBlockFiles[0]
	s.obfMutex.RUnlock()
	if cached != open {
		t.Fatalf("block file 0 open %v before the prune, want %v", cached,
			open)
	}

	deleteFile := s.deleteFileFunc
	readErr := make(chan error, 1)
	s.deleteFileFunc = func(fileNum uint32) error {
		if fileNum == 0 {
			if s.obfMutex.TryLock() {
				s.obfMutex.Unlock()
				readErr <- read()
			} else {
				go func() { readErr <- read() }()
			}
		}
		return deleteFile(fileNum)
	}
	runWithTimeout(t, 5*time.Second, "DeleteBlocks", func() {
		prunePruneTestDB(t, pdb, 3)
	})
	s.deleteFileFunc = deleteFile
	pruneTestFiles(t, pdb, 4, 2, 3)

	var err error
	runWithTimeout(t, 5*time.Second, "the read", func() { err = <-readErr })
	var dbErr database.Error
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrDriverSpecific ||
		!errors.Is(dbErr.Err, os.ErrNotExist) {

		t.Errorf("read of the block during its file's removal: got %v, "+
			"want a missing block file", err)
	}
	s.obfMutex.RLock()
	_, cached = s.openBlockFiles[0]
	s.obfMutex.RUnlock()
	if cached {
		t.Error("block file 0 was opened again between its close and " +
			"its removal")
	}
}

// reopenPruneTestDB closes the database at dir and opens it again.
func reopenPruneTestDB(t *testing.T, pdb *db, dir string) *db {
	t.Helper()
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	idb, err := database.Open(dbType, dir, blockDataNet)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return idb.(*db)
}

// pruneTestFiles fails the test unless exactly the block files in want exist
// among files 0 to n-1.
func pruneTestFiles(t *testing.T, pdb *db, n uint32, want ...uint32) {
	t.Helper()
	for i := range n {
		_, err := os.Stat(blockFilePath(pdb.store.basePath, i))
		if exists := err == nil; exists != slices.Contains(want, i) {
			t.Errorf("block file %d exists %v, want %v", i, exists, !exists)
		}
	}
}

// pruneTestHeights fails the test unless the last block heights the store
// knows are want.
func pruneTestHeights(t *testing.T, pdb *db, want map[uint32]uint32) {
	t.Helper()
	pdb.store.fbhMutex.RLock()
	defer pdb.store.fbhMutex.RUnlock()
	if !maps.Equal(pdb.store.fileBlockHeights, want) {
		t.Errorf("last block heights %v, want %v",
			pdb.store.fileBlockHeights, want)
	}
}

// prunePruneTestDB deletes the block files whose blocks are all below height.
func prunePruneTestDB(t *testing.T, pdb *db, height uint32) {
	t.Helper()
	err := pdb.Update(func(tx database.Tx) error {
		return tx.DeleteBlocks(height)
	})
	if err != nil {
		t.Fatalf("DeleteBlocks(%d): %v", height, err)
	}
}

// TestPruneAfterReopen ensures block files filled before the database was
// opened are pruned too, since their last block heights are read back when
// it opens.  Each test block takes a file, whose last height is written when
// the next block moves writes to the next file.
func TestPruneAfterReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	pdb := createPruneTestDB(t, dir)
	for i := 1; i <= 3; i++ {
		storeTestBlock(t, pdb, pruneTestBlock(uint32(i), int32(i)))
	}

	pdb = reopenPruneTestDB(t, pdb, dir)
	pruneTestHeights(t, pdb, map[uint32]uint32{0: 1, 1: 2})
	prunePruneTestDB(t, pdb, 3)
	pruneTestFiles(t, pdb, 3, 2)
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestPruneAfterReopenPruned ensures the last block heights are read back
// from the first block file on disk when the files before it were pruned.
func TestPruneAfterReopenPruned(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	pdb := createPruneTestDB(t, dir)
	for i := 1; i <= 5; i++ {
		storeTestBlock(t, pdb, pruneTestBlock(uint32(i), int32(i)))
	}
	prunePruneTestDB(t, pdb, 3)
	pruneTestFiles(t, pdb, 5, 2, 3, 4)

	pdb = reopenPruneTestDB(t, pdb, dir)
	pruneTestHeights(t, pdb, map[uint32]uint32{2: 3, 3: 4})
	prunePruneTestDB(t, pdb, 5)
	pruneTestFiles(t, pdb, 5, 4)
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestPruneAfterReopenShortFile ensures a full block file too short to hold
// its last block height does not fail the open and is kept.
func TestPruneAfterReopenShortFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	pdb := createPruneTestDB(t, dir)
	for i := 1; i <= 3; i++ {
		storeTestBlock(t, pdb, pruneTestBlock(uint32(i), int32(i)))
	}
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := os.Truncate(blockFilePath(pdb.store.basePath, 0), 3); err != nil {
		t.Fatal(err)
	}

	idb, err := database.Open(dbType, dir, blockDataNet)
	if err != nil {
		t.Fatalf("Open with a short block file: %v", err)
	}
	pdb = idb.(*db)
	pruneTestHeights(t, pdb, map[uint32]uint32{1: 2})
	prunePruneTestDB(t, pdb, 3)
	pruneTestFiles(t, pdb, 3, 0, 2)
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestPruneAfterReopenRollback ensures the open forgets the last block heights
// of the block files it removes because the metadata was not flushed after
// their blocks were written, as after a crash.
func TestPruneAfterReopenRollback(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	pdb := createPruneTestDB(t, dir)
	for i := 1; i <= 3; i++ {
		storeTestBlock(t, pdb, pruneTestBlock(uint32(i), int32(i)))
	}
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The data a crash leaves: the last height of the current file and
	// two more files, which the metadata does not know about.
	path := func(n uint32) string { return blockFilePath(pdb.store.basePath, n) }
	file, err := os.OpenFile(path(2), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte{3, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for n := uint32(3); n <= 4; n++ {
		if err := os.WriteFile(path(n), make([]byte, 8), 0600); err != nil {
			t.Fatal(err)
		}
	}

	idb, err := database.Open(dbType, dir, blockDataNet)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	pdb = idb.(*db)
	pruneTestFiles(t, pdb, 5, 0, 1, 2)
	pruneTestHeights(t, pdb, map[uint32]uint32{0: 1, 1: 2})
	prunePruneTestDB(t, pdb, 3)
	pruneTestFiles(t, pdb, 5, 2)
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
