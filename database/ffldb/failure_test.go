package ffldb

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/database"
	"github.com/gcash/goleveldb/leveldb"
	"github.com/gcash/goleveldb/leveldb/opt"
	"github.com/gcash/goleveldb/leveldb/storage"
)

var errInjected = errors.New("injected storage failure")

// failingStorage wraps a goleveldb storage and fails the creation of new
// table files while fail is set, which is what a full disk looks like to a
// leveldb transaction commit. While failSync is set, it fails syncing the
// MANIFEST, as a disk that reports a write error does.  While failOpen is
// set, it fails opening table files, as an unreadable disk does.
type failingStorage struct {
	storage.Storage
	fail     int32
	failSync int32
	failOpen int32
}

func (s *failingStorage) Open(fd storage.FileDesc) (storage.Reader, error) {
	if atomic.LoadInt32(&s.failOpen) != 0 && fd.Type == storage.TypeTable {
		return nil, errInjected
	}
	return s.Storage.Open(fd)
}

func (s *failingStorage) Create(fd storage.FileDesc) (storage.Writer, error) {
	if atomic.LoadInt32(&s.fail) != 0 && fd.Type == storage.TypeTable {
		return nil, errInjected
	}
	w, err := s.Storage.Create(fd)
	if err != nil || fd.Type != storage.TypeManifest {
		return w, err
	}
	return &failingManifest{Writer: w, s: s}, nil
}

// failingManifest fails Sync while its storage's failSync is set.
type failingManifest struct {
	storage.Writer
	s *failingStorage
}

func (w *failingManifest) Sync() error {
	if atomic.LoadInt32(&w.s.failSync) != 0 {
		return errInjected
	}
	return w.Writer.Sync()
}

// writesStopped reports whether err, as updateDB returns it, is leveldb's
// error for writes it stopped until the database is reopened.
func writesStopped(err error) bool {
	var dbErr database.Error
	return errors.As(err, &dbErr) && errors.Is(dbErr.Err, leveldb.ErrWritesStopped)
}

// readFailed reports whether err is a database error for the injected
// storage failure.
func readFailed(err error) bool {
	var dbErr database.Error
	return errors.As(err, &dbErr) && errors.Is(dbErr.Err, errInjected)
}

// createTestDB creates a new database in a temp dir.
func createTestDB(t *testing.T) (*db, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ffldb")
	idb, err := openDB(dir, blockDataNet, true, 0, 0)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	return idb.(*db), dir
}

// swapFailingLdb reopens the metadata database of pdb through a storage
// that can be made to fail.
func swapFailingLdb(t *testing.T, pdb *db, dir string) *failingStorage {
	t.Helper()
	if err := pdb.cache.ldb.Close(); err != nil {
		t.Fatalf("close ldb: %v", err)
	}
	fstor, err := storage.OpenFile(filepath.Join(dir, metadataDbName), false)
	if err != nil {
		t.Fatalf("storage.OpenFile: %v", err)
	}
	fs := &failingStorage{Storage: fstor}
	ldb, err := leveldb.Open(fs, &opt.Options{Compression: opt.NoCompression})
	if err != nil {
		t.Fatalf("leveldb.Open: %v", err)
	}
	pdb.cache.ldb = ldb
	return fs
}

// TestLdbCommitFailureDoesNotWedgeDB ensures a failed leveldb commit does
// not leave the leveldb write lock held, which would block every later
// flush and Close.
func TestLdbCommitFailureDoesNotWedgeDB(t *testing.T) {
	pdb, dir := createTestDB(t)
	fs := swapFailingLdb(t, pdb, dir)
	defer pdb.cache.ldb.Close()

	put := func(k string) error {
		return pdb.cache.updateDB(func(tx *leveldb.Transaction) error {
			return tx.Put([]byte(k), []byte("v"), nil)
		})
	}

	atomic.StoreInt32(&fs.fail, 1)
	if err := put("a"); err == nil {
		t.Fatalf("expected injected commit failure")
	}
	atomic.StoreInt32(&fs.fail, 0)

	var err error
	runWithTimeout(t, 5*time.Second, "updateDB after a failed commit", func() {
		err = put("b")
	})
	if err != nil {
		t.Fatalf("updateDB after recovery: %v", err)
	}
}

// TestLdbManifestFailureDoesNotWedgeDB ensures a leveldb commit whose MANIFEST
// sync fails, after which leveldb takes no writes until it is reopened, leaves
// later flushes returning that error rather than blocking on the leveldb write
// lock, and lets Close return.
func TestLdbManifestFailureDoesNotWedgeDB(t *testing.T) {
	pdb, dir := createTestDB(t)
	fs := swapFailingLdb(t, pdb, dir)

	put := func(k string) error {
		return pdb.cache.updateDB(func(tx *leveldb.Transaction) error {
			return tx.Put([]byte(k), []byte("v"), nil)
		})
	}

	atomic.StoreInt32(&fs.failSync, 1)
	if err := put("a"); !writesStopped(err) {
		t.Fatalf("updateDB whose MANIFEST sync fails = %v, want writes stopped", err)
	}
	atomic.StoreInt32(&fs.failSync, 0)

	var err error
	runWithTimeout(t, 5*time.Second, "updateDB after writes stopped", func() {
		err = put("b")
	})
	if !writesStopped(err) {
		t.Fatalf("updateDB after writes stopped = %v, want writes stopped", err)
	}
	runWithTimeout(t, 5*time.Second, "Close after writes stopped", func() {
		err = pdb.cache.ldb.Close()
	})
	if !errors.Is(err, leveldb.ErrWritesStopped) {
		t.Fatalf("Close after writes stopped = %v, want writes stopped", err)
	}
}

// TestCommitFailsAfterWritesStopped ensures that once leveldb stops taking
// writes, every later commit fails, including one that would not flush,
// rather than going to the cache, which could then never be written.
func TestCommitFailsAfterWritesStopped(t *testing.T) {
	pdb, dir := createTestDB(t)
	fs := swapFailingLdb(t, pdb, dir)
	defer pdb.cache.ldb.Close()

	put := func(k string) error {
		return pdb.Update(func(tx database.Tx) error {
			return tx.Metadata().Put([]byte(k), []byte("v"))
		})
	}

	// The commit flushes, and its MANIFEST sync fails.
	pdb.cache.flushInterval = 0
	atomic.StoreInt32(&fs.failSync, 1)
	if err := put("a"); !writesStopped(err) {
		t.Fatalf("commit whose MANIFEST sync fails = %v, want writes stopped", err)
	}
	atomic.StoreInt32(&fs.failSync, 0)

	pdb.cache.flushInterval = time.Hour
	if err := put("b"); !writesStopped(err) {
		t.Fatalf("commit after writes stopped = %v, want writes stopped", err)
	}
}

// TestFailedCommitRollsBackBlocks ensures a commit that fails after it wrote
// its blocks to the block files, as when leveldb fails a write or has stopped
// taking writes, removes them again, rather than leaving a copy of the block
// behind every time it is stored again.
func TestFailedCommitRollsBackBlocks(t *testing.T) {
	pdb, dir := createTestDB(t)
	fs := swapFailingLdb(t, pdb, dir)
	defer pdb.cache.ldb.Close()

	store := func(nonce uint32) error {
		return pdb.Update(func(tx database.Tx) error {
			return tx.StoreBlock(pruneTestBlock(nonce, int32(nonce)))
		})
	}
	if err := store(1); err != nil {
		t.Fatalf("store: %v", err)
	}
	wc := pdb.store.writeCursor
	fileNum, offset := wc.curFileNum, wc.curOffset
	requireRolledBack := func(what string) {
		t.Helper()
		info, err := os.Stat(blockFilePath(pdb.store.basePath, fileNum))
		if err != nil {
			t.Fatal(err)
		}
		if wc.curFileNum != fileNum || wc.curOffset != offset ||
			info.Size() != int64(offset) {

			t.Errorf("%s: write cursor at file %d offset %d, file size %d, "+
				"want file %d offset and size %d", what, wc.curFileNum,
				wc.curOffset, info.Size(), fileNum, offset)
		}
	}

	// Every commit flushes the cache, and the flush fails to write.
	pdb.cache.flushInterval = 0
	atomic.StoreInt32(&fs.fail, 1)
	if err := store(2); err == nil {
		t.Fatal("expected the injected flush failure")
	}
	atomic.StoreInt32(&fs.fail, 0)
	requireRolledBack("after a failed flush")

	atomic.StoreInt32(&fs.failSync, 1)
	if err := store(2); !writesStopped(err) {
		t.Fatalf("commit whose MANIFEST sync fails = %v, want writes stopped", err)
	}
	atomic.StoreInt32(&fs.failSync, 0)
	if err := store(2); !writesStopped(err) {
		t.Fatalf("commit after writes stopped = %v, want writes stopped", err)
	}
	requireRolledBack("after writes stopped")
}

// TestFlushRetriedAfterFailure ensures a commit after a failed flush flushes
// again, rather than waiting a whole flush interval with the cache unwritten.
func TestFlushRetriedAfterFailure(t *testing.T) {
	pdb, dir := createTestDB(t)
	fs := swapFailingLdb(t, pdb, dir)
	defer pdb.cache.ldb.Close()

	put := func(k string) error {
		return pdb.Update(func(tx database.Tx) error {
			return tx.Metadata().Put([]byte(k), []byte("v"))
		})
	}
	pdb.cache.flushInterval = time.Hour
	if err := put("x"); err != nil {
		t.Fatal(err)
	}

	// A flush is due, and it fails.
	pdb.cache.lastFlush = time.Now().Add(-2 * time.Hour)
	atomic.StoreInt32(&fs.fail, 1)
	if err := put("y"); err == nil {
		t.Fatal("expected the injected flush failure")
	}
	atomic.StoreInt32(&fs.fail, 0)

	if err := put("z"); err != nil {
		t.Fatal(err)
	}
	if _, err := pdb.cache.ldb.Get(bucketizedKey(metadataBucketID, []byte("x")), nil); err != nil {
		t.Fatalf("the cache was not flushed after the failure: %v", err)
	}
}

// TestReadErrorFailsTransaction ensures a key that cannot be read from disk
// fails its transaction rather than reading as missing: View returns the
// error, and Update does not commit.  Otherwise a utxo that cannot be read
// looks spent, and the valid block spending it is marked invalid.
func TestReadErrorFailsTransaction(t *testing.T) {
	pdb, dir := createTestDB(t)
	key := []byte("k")
	block := pruneTestBlock(1, 1)
	if err := pdb.Update(func(tx database.Tx) error {
		if err := tx.StoreBlock(block); err != nil {
			return err
		}
		return tx.Metadata().Put(key, []byte("v"))
	}); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := pdb.cache.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Reopening leveldb moves the key from its journal to a table file.
	fs := swapFailingLdb(t, pdb, dir)
	defer pdb.cache.ldb.Close()
	atomic.StoreInt32(&fs.failOpen, 1)

	err := pdb.View(func(tx database.Tx) error {
		if v := tx.Metadata().Get(key); v != nil {
			t.Errorf("read %q through a failing disk", v)
		}
		return nil
	})
	if !readFailed(err) {
		t.Fatalf("View: got %v, want the read error", err)
	}
	err = pdb.View(func(tx database.Tx) error {
		if has, _ := tx.HasBlock(block.Hash()); has {
			t.Error("found a block through a failing disk")
		}
		return nil
	})
	if !readFailed(err) {
		t.Fatalf("View with HasBlock: got %v, want the read error", err)
	}

	written := []byte("w")
	err = pdb.Update(func(tx database.Tx) error {
		tx.Metadata().Get(key)
		return tx.Metadata().Put(written, []byte("v"))
	})
	if !readFailed(err) {
		t.Fatalf("Update: got %v, want the read error", err)
	}

	atomic.StoreInt32(&fs.failOpen, 0)
	if err := pdb.View(func(tx database.Tx) error {
		if tx.Metadata().Get(key) == nil {
			t.Error("key missing once readable")
		}
		if tx.Metadata().Get(written) != nil {
			t.Error("Update with a failed read committed")
		}
		if has, _ := tx.HasBlock(block.Hash()); !has {
			t.Error("block missing once readable")
		}
		return nil
	}); err != nil {
		t.Fatalf("View once readable: %v", err)
	}
}
