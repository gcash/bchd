// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/wire"
)

// newDropTestDB returns a database holding an index under idxKey whose bucket
// holds the passed one-byte keys.
func newDropTestDB(t *testing.T, idxKey []byte, keys ...byte) database.DB {
	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	err = db.Update(func(tx database.Tx) error {
		meta := tx.Metadata()
		tips, err := meta.CreateBucketIfNotExists(indexTipsBucketName)
		if err != nil {
			return err
		}
		if err := tips.Put(idxKey, make([]byte, 36)); err != nil {
			return err
		}
		bucket, err := meta.CreateBucket(idxKey)
		if err != nil {
			return err
		}
		for _, k := range keys {
			if err := bucket.Put([]byte{k}, []byte{1}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// setDropIndexBatch sets dropIndexBatch for the test.
func setDropIndexBatch(t *testing.T, batch int) {
	saved := dropIndexBatch
	dropIndexBatch = batch
	t.Cleanup(func() { dropIndexBatch = saved })
}

// trapDB puts key in the index bucket once a read-write transaction has
// deleted first, and records whether key was still there after the next one.
type trapDB struct {
	database.DB
	idxKey, first, key     []byte
	put, checked, survived bool
}

// Update runs the transaction on the database, and then puts or checks the
// key.
func (db *trapDB) Update(fn func(tx database.Tx) error) error {
	if err := db.DB.Update(fn); err != nil {
		return err
	}
	if db.put {
		if !db.checked {
			db.checked = true
			return db.DB.View(func(tx database.Tx) error {
				bucket := tx.Metadata().Bucket(db.idxKey)
				db.survived = bucket != nil && bucket.Get(db.key) != nil
				return nil
			})
		}
		return nil
	}
	return db.DB.Update(func(tx database.Tx) error {
		bucket := tx.Metadata().Bucket(db.idxKey)
		if bucket == nil || bucket.Get(db.first) != nil {
			return nil
		}
		db.put = true
		return bucket.Put(db.key, []byte{1})
	})
}

// TestDropIndexResumes ensures each batch of an index drop continues after the
// last key the previous batch deleted instead of starting from the first key,
// which would step over every key deleted so far that the database has not
// yet compacted away.  So a key put before that point after the first batch
// is left alone by the next one.
func TestDropIndexResumes(t *testing.T) {
	setDropIndexBatch(t, 2)
	idxKey := []byte("testidx")
	db := newDropTestDB(t, idxKey, 0, 2, 4, 6, 8)

	// The first batch deletes 0 and 2, then 1 is put.
	trap := &trapDB{DB: db, idxKey: idxKey, first: []byte{0}, key: []byte{1}}
	if err := dropIndex(trap, idxKey, "test index", nil); err != nil {
		t.Fatal(err)
	}
	if !trap.checked || !trap.survived {
		t.Error("the second batch started from the first key")
	}
}

// TestDropIndexInterruptedBetweenBatches ensures an interrupted index drop
// stops after a batch rather than after the whole bucket, and that the next
// start finishes it.
func TestDropIndexInterruptedBetweenBatches(t *testing.T) {
	setDropIndexBatch(t, 2)
	idxKey := []byte("testidx")
	db := newDropTestDB(t, idxKey, 0, 2, 4, 6, 8)
	interrupt := make(chan struct{})
	close(interrupt)
	if err := dropIndex(db, idxKey, "test index", interrupt); err != errInterruptRequested {
		t.Fatalf("got %v, want %v", err, errInterruptRequested)
	}
	err := db.View(func(tx database.Tx) error {
		meta := tx.Metadata()
		var left int
		cursor := meta.Bucket(idxKey).Cursor()
		for ok := cursor.First(); ok; ok = cursor.Next() {
			left++
		}
		if left != 3 {
			t.Errorf("%d keys left, want the 3 after the first batch", left)
		}
		if meta.Bucket(indexTipsBucketName).Get(indexDropKey(idxKey)) == nil {
			t.Error("the drop is not marked to resume")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := dropIndex(db, idxKey, "test index", nil); err != nil {
		t.Fatal(err)
	}
	err = db.View(func(tx database.Tx) error {
		meta := tx.Metadata()
		tips := meta.Bucket(indexTipsBucketName)
		if meta.Bucket(idxKey) != nil || tips.Get(idxKey) != nil ||
			tips.Get(indexDropKey(idxKey)) != nil {

			t.Error("the resumed drop did not finish")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// errTestView is the error of a failing read-only transaction.
var errTestView = errors.New("test view failure")

// failingViewDB fails its n-th read-only transaction.
type failingViewDB struct {
	database.DB
	n, views int
}

// View fails the n-th transaction and runs the others on the database.
func (db *failingViewDB) View(fn func(tx database.Tx) error) error {
	db.views++
	if db.views == db.n {
		return errTestView
	}
	return db.DB.View(fn)
}

// TestDropIndexListError ensures an index drop that cannot list the index's
// buckets fails with the error rather than reporting success.
func TestDropIndexListError(t *testing.T) {
	idxKey := []byte("testidx")
	db := newDropTestDB(t, idxKey, 0)

	// The second read-only transaction is the one listing the buckets.
	failing := &failingViewDB{DB: db, n: 2}
	if err := dropIndex(failing, idxKey, "test index", nil); err != errTestView {
		t.Fatalf("got %v, want %v", err, errTestView)
	}
}

// newResumedDropTestDB returns a database whose drop of the index under idxKey
// stopped after the index bucket was deleted: the index tip and the marker of
// the drop are left, and so are the passed buckets.
func newResumedDropTestDB(t *testing.T, idxKey []byte,
	buckets ...[]byte) database.DB {

	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	err = db.Update(func(tx database.Tx) error {
		meta := tx.Metadata()
		tips, err := meta.CreateBucketIfNotExists(indexTipsBucketName)
		if err != nil {
			return err
		}
		if err := tips.Put(idxKey, make([]byte, 36)); err != nil {
			return err
		}
		if err := tips.Put(indexDropKey(idxKey), idxKey); err != nil {
			return err
		}
		for _, name := range buckets {
			if _, err := meta.CreateBucket(name); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// checkDropped fails the test unless the index tip, the marker of the drop
// and the passed buckets are gone.
func checkDropped(t *testing.T, db database.DB, idxKey []byte,
	buckets ...[]byte) {

	t.Helper()
	err := db.View(func(tx database.Tx) error {
		meta := tx.Metadata()
		tips := meta.Bucket(indexTipsBucketName)
		if tips.Get(idxKey) != nil || tips.Get(indexDropKey(idxKey)) != nil {
			t.Error("the index tip or the marker of the drop is left")
		}
		for _, name := range buckets {
			if meta.Bucket(name) != nil {
				t.Errorf("bucket %s is left", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDropIndexResumedAfterBucketDeleted ensures a drop that stopped after the
// index bucket was deleted, but before the index tip and the marker of the
// drop were, finishes when it is resumed.  It used to panic on every start.
func TestDropIndexResumedAfterBucketDeleted(t *testing.T) {
	idxKey := []byte("testidx")
	db := newResumedDropTestDB(t, idxKey)
	if err := dropIndex(db, idxKey, "test index", nil); err != nil {
		t.Fatal(err)
	}
	checkDropped(t, db, idxKey)
}

// TestDropIndexResumedOtherBuckets ensures a transaction or slp index drop
// that stopped after the index bucket was deleted finishes when it is
// resumed, deleting the index's other buckets if they are left and skipping
// them if they are gone too.
func TestDropIndexResumedOtherBuckets(t *testing.T) {
	tests := []struct {
		idxKey  []byte
		name    string
		buckets [][]byte
	}{
		{txIndexKey, txIndexName,
			[][]byte{idByHashIndexBucketName, hashByIDIndexBucketName}},
		{slpIndexKey, slpIndexName,
			[][]byte{tokenIDByHashIndexBucketName,
				tokenMetadataByIDIndexBucketName}},
	}
	for _, test := range tests {
		for _, left := range []bool{true, false} {
			var buckets [][]byte
			if left {
				buckets = test.buckets
			}
			db := newResumedDropTestDB(t, test.idxKey, buckets...)
			err := dropIndex(db, test.idxKey, test.name, nil)
			if err != nil {
				t.Fatalf("%s drop, other buckets left %v: %v", test.name,
					left, err)
			}
			checkDropped(t, db, test.idxKey, test.buckets...)
		}
	}
}

// TestDeleteBucketsIfExist ensures deleteBucketsIfExist deletes the passed
// buckets that exist and skips those that do not, wherever they are in the
// list.
func TestDeleteBucketsIfExist(t *testing.T) {
	db := newResumedDropTestDB(t, []byte("testidx"), []byte("b"))
	err := db.Update(func(tx database.Tx) error {
		return deleteBucketsIfExist(tx.Metadata(), []byte("a"), []byte("b"),
			[]byte("c"))
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.View(func(tx database.Tx) error {
		if tx.Metadata().Bucket([]byte("b")) != nil {
			t.Error("bucket b is left")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
