package ffldb

import (
	"bytes"
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

// TestReopenAfterFailedCommit ensures the database opens after a commit whose
// leveldb MANIFEST commit failed, the cache flush's or the commit's own, both
// when the record reached the disk anyway, as leveldb says it may, and when it
// did not, and that the database then holds the blocks the record stored.
// Removing the blocks of a commit whose own record reached the disk left the
// stored write cursor past the end of the block files, which the next open
// refuses as corruption.
func TestReopenAfterFailedCommit(t *testing.T) {
	tests := []struct {
		name string

		// cached is whether block 1 waits in the cache, so the MANIFEST
		// commit that fails is the flush's rather than block 2's own.
		cached bool

		// stored is whether the failed MANIFEST record reaches the disk.
		stored bool

		// blocks is the number of blocks the reopened database holds.
		blocks int
	}{
		{name: "flush record stored", cached: true, stored: true, blocks: 1},
		{name: "flush record lost", cached: true, stored: false, blocks: 0},
		{name: "own record stored", cached: false, stored: true, blocks: 2},
		{name: "own record lost", cached: false, stored: false, blocks: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pdb, dir := createTestDB(t)
			fs := swapFailingLdb(t, pdb, dir)
			store := func(pdb *db, nonce uint32) error {
				return pdb.Update(func(tx database.Tx) error {
					return tx.StoreBlock(pruneTestBlock(nonce, int32(nonce)))
				})
			}
			fileSize := func(pdb *db) int64 {
				t.Helper()
				info, err := os.Stat(blockFilePath(pdb.store.basePath, 0))
				if err != nil {
					t.Fatal(err)
				}
				return info.Size()
			}

			// With a flush interval of 0, every commit flushes the cache
			// and then writes its own keys to leveldb.
			if !test.cached {
				pdb.cache.flushInterval = 0
			}
			if err := store(pdb, 1); err != nil {
				t.Fatalf("store block 1: %v", err)
			}
			pdb.cache.flushInterval = 0

			// The test blocks are all the same size.
			blockSize := pdb.store.writeCursor.curOffset

			// A write that fails without stopping leveldb's writes stores
			// nothing, so the commit's block goes.
			atomic.StoreInt32(&fs.fail, 1)
			if err := store(pdb, 2); err == nil || writesStopped(err) {
				t.Fatalf("commit whose write fails = %v, want the injected failure", err)
			}
			atomic.StoreInt32(&fs.fail, 0)
			if got := fileSize(pdb); got != int64(blockSize) {
				t.Fatalf("failed commit left the block file at %d bytes, want %d",
					got, blockSize)
			}

			// The MANIFEST sync of block 2's commit fails, after the record
			// was written to the file.
			manifest, err := fs.GetMeta()
			if err != nil {
				t.Fatal(err)
			}
			manifestPath := filepath.Join(dir, metadataDbName, manifest.String())
			info, err := os.Stat(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			atomic.StoreInt32(&fs.failSync, 1)
			if err := store(pdb, 2); !writesStopped(err) {
				t.Fatalf("commit whose MANIFEST sync fails = %v, want writes stopped", err)
			}
			atomic.StoreInt32(&fs.failSync, 0)

			// A commit refused once writes stopped leaves the block files as
			// they were.
			size := fileSize(pdb)
			if err := store(pdb, 3); !writesStopped(err) {
				t.Fatalf("commit after writes stopped = %v, want writes stopped", err)
			}
			if got := fileSize(pdb); got != size {
				t.Errorf("refused commit changed the block file from %d to %d bytes",
					size, got)
			}

			if err := pdb.Close(); !writesStopped(err) {
				t.Fatalf("close after writes stopped = %v, want writes stopped", err)
			}
			if err := fs.Close(); err != nil {
				t.Fatalf("close storage: %v", err)
			}
			if !test.stored {
				// The write of the record is lost, as it can be when its
				// sync fails.
				if err := os.Truncate(manifestPath, info.Size()); err != nil {
					t.Fatal(err)
				}
			}

			idb, err := openDB(dir, blockDataNet, false, 0, 0)
			if err != nil {
				t.Fatalf("open after the failed commit: %v", err)
			}
			pdb = idb.(*db)
			defer pdb.Close()

			// The block files end after the blocks the database holds,
			// where its write cursor is.
			want := uint32(test.blocks) * blockSize
			wc := pdb.store.writeCursor
			if size := fileSize(pdb); wc.curFileNum != 0 || wc.curOffset != want ||
				size != int64(want) {

				t.Errorf("write cursor at file %d offset %d, block file size %d, "+
					"want file 0 offset and size %d", wc.curFileNum,
					wc.curOffset, size, want)
			}
			err = pdb.View(func(tx database.Tx) error {
				for nonce := uint32(1); nonce <= 2; nonce++ {
					block := pruneTestBlock(nonce, int32(nonce))
					has, err := tx.HasBlock(block.Hash())
					if err != nil {
						return err
					}
					if want := int(nonce) <= test.blocks; has != want {
						t.Errorf("has block %d = %v, want %v", nonce, has, want)
					}
					if !has {
						continue
					}
					got, err := tx.FetchBlock(block.Hash())
					if err != nil {
						return err
					}
					if want, _ := block.Bytes(); !bytes.Equal(got, want) {
						t.Errorf("block %d read back differs", nonce)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("read the blocks: %v", err)
			}

			// The database takes blocks again.
			if err := store(pdb, 3); err != nil {
				t.Fatalf("store block 3 after reopening: %v", err)
			}
		})
	}
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

// TestReadErrorOverFnError ensures View and Update return the error of a read
// that failed rather than the error their function made of what it read: a
// block whose index entry can't be read is not found, and a key that can't
// be read is missing.  Otherwise a caller acts on "not found", as a rescan
// does by looking for a reorg.
func TestReadErrorOverFnError(t *testing.T) {
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

	errMissing := errors.New("key missing")
	fns := []struct {
		name string
		fn   func(tx database.Tx) error
	}{{
		name: "FetchBlock",
		fn: func(tx database.Tx) error {
			_, err := tx.FetchBlock(block.Hash())
			return err
		},
	}, {
		name: "Get",
		fn: func(tx database.Tx) error {
			if tx.Metadata().Get(key) == nil {
				return errMissing
			}
			return nil
		},
	}}
	atomic.StoreInt32(&fs.failOpen, 1)
	for _, test := range fns {
		if err := pdb.View(test.fn); !readFailed(err) {
			t.Errorf("View with %s: got %v, want the read error",
				test.name, err)
		}
		if err := pdb.Update(test.fn); !readFailed(err) {
			t.Errorf("Update with %s: got %v, want the read error",
				test.name, err)
		}
	}

	// Without a failed read, the function's own error is returned.
	atomic.StoreInt32(&fs.failOpen, 0)
	missing := func(tx database.Tx) error {
		if tx.Metadata().Get([]byte("absent")) == nil {
			return errMissing
		}
		return nil
	}
	if err := pdb.View(missing); err != errMissing {
		t.Errorf("View of a missing key: got %v, want %v", err, errMissing)
	}
	if err := pdb.Update(missing); err != errMissing {
		t.Errorf("Update of a missing key: got %v, want %v", err,
			errMissing)
	}
	for _, test := range fns {
		if err := pdb.View(test.fn); err != nil {
			t.Errorf("View with %s once readable: %v", test.name, err)
		}
	}
}
