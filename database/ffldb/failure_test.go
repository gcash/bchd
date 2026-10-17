package ffldb

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

var errInjected = errors.New("injected storage failure")

// failingStorage wraps a goleveldb storage and fails the creation of new
// table files while fail is set, which is what a full disk looks like to a
// leveldb transaction commit.
type failingStorage struct {
	storage.Storage
	fail int32
}

func (s *failingStorage) Create(fd storage.FileDesc) (storage.Writer, error) {
	if atomic.LoadInt32(&s.fail) != 0 && fd.Type == storage.TypeTable {
		return nil, errInjected
	}
	return s.Storage.Create(fd)
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
