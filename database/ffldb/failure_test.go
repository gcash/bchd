package ffldb

import (
	"errors"
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
// MANIFEST, as a disk that reports a write error does.
type failingStorage struct {
	storage.Storage
	fail     int32
	failSync int32
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
