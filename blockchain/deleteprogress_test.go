// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
)

// captureInfoLog sends the package's log at the info level to the returned
// buffer until the test ends.
func captureInfoLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("CHAN")
	logger.SetLevel(bchlog.LevelInfo)
	saved := log
	UseLogger(logger)
	t.Cleanup(func() { UseLogger(saved) })
	return &buf
}

// setDeleteProgressInterval sets how often a deletion logs its progress
// until the test ends.
func setDeleteProgressInterval(t *testing.T, interval time.Duration) {
	t.Helper()
	saved := deleteProgressInterval
	deleteProgressInterval = interval
	t.Cleanup(func() { deleteProgressInterval = saved })
}

// createDeleteTestBucket creates a bucket holding four keys that start at a
// quarter, half and three quarters of the key space, and at its start.
func createDeleteTestBucket(t *testing.T) (database.DB, []byte) {
	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	name := []byte("test")
	err = db.Update(func(tx database.Tx) error {
		bucket, err := tx.Metadata().CreateBucket(name)
		if err != nil {
			return err
		}
		for _, first := range []byte{0x00, 0x40, 0x80, 0xc0} {
			key := make([]byte, 33)
			key[0] = first
			if err := bucket.Put(key, []byte{1}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, name
}

// TestDeleteBucketKeysProgress ensures the deletion of the old utxo set logs
// how many entries it deleted, and how far it got estimated from the
// position of the last deleted key, as its keys start with a hash.
func TestDeleteBucketKeysProgress(t *testing.T) {
	setDeleteProgressInterval(t, time.Nanosecond)
	db, name := createDeleteTestBucket(t)
	logged := captureInfoLog(t)
	if err := deleteBucketKeys(db, name, 1); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Deletion progress: 1 entries deleted (about 0%)",
		"Deletion progress: 2 entries deleted (about 25%)",
		"Deletion progress: 3 entries deleted (about 50%)",
		"Deletion progress: 4 entries deleted (about 75%)",
	} {
		if !strings.Contains(logged.String(), want) {
			t.Errorf("log %q does not say %q", logged.String(), want)
		}
	}
}

// TestDeleteBucketKeysNoWait ensures the deletion logs nothing before the
// interval has passed, and does not wait for it between batches.
func TestDeleteBucketKeysNoWait(t *testing.T) {
	setDeleteProgressInterval(t, time.Hour)
	db, name := createDeleteTestBucket(t)
	logged := captureInfoLog(t)
	done := make(chan error, 1)
	go func() { done <- deleteBucketKeys(db, name, 1) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the deletion waited for the progress interval")
	}
	if logged.Len() != 0 {
		t.Fatalf("logged %q before the interval passed", logged.String())
	}
}

// TestFastSyncStartLogsDeletion ensures the start of a fast sync logs the
// deletion of the old utxo set, as a re-index does.
func TestFastSyncStartLogsDeletion(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	_, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to open chain: %v", err)
	}
	defer db.Close()

	logged := captureInfoLog(t)
	cache := newUtxoCache(db, 1<<20)
	if err := cache.InitConsistentState(nil, nil, true, nil); err != nil {
		t.Fatalf("failed to start the fast sync: %v", err)
	}
	for _, want := range []string{"Deleting the UTXO set",
		"Deletion complete"} {
		if !strings.Contains(logged.String(), want) {
			t.Errorf("log %q does not say %q", logged.String(), want)
		}
	}
}
