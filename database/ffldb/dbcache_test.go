// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"testing"

	"github.com/gcash/bchd/database"
)

// TestCacheCommit ensures keys committed to the cache read the same before
// and after the cache is flushed: a key deleted over one in leveldb, a key
// deleted over one in the cache, and a key put over one deleted in the cache.
// It also ensures a transaction begun before a commit does not see it.
func TestCacheCommit(t *testing.T) {
	pdb, _ := createTestDB(t)
	defer pdb.Close()

	update := func(fn func(meta database.Bucket) error) {
		t.Helper()
		err := pdb.Update(func(tx database.Tx) error {
			return fn(tx.Metadata())
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
	}
	check := func(when string, tx database.Tx, want map[string]string) {
		t.Helper()
		for k, v := range want {
			got := tx.Metadata().Get([]byte(k))
			if (v == "" && got != nil) || (v != "" && string(got) != v) {
				t.Errorf("%s: %s is %q, want %q", when, k, got, v)
			}
		}
	}
	view := func(when string, want map[string]string) {
		t.Helper()
		err := pdb.View(func(tx database.Tx) error {
			check(when, tx, want)
			return nil
		})
		if err != nil {
			t.Fatalf("View: %v", err)
		}
	}

	// The key disk is in leveldb, and gone and readded only in the cache.
	update(func(meta database.Bucket) error {
		return meta.Put([]byte("disk"), []byte("1"))
	})
	if err := pdb.cache.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	update(func(meta database.Bucket) error {
		if err := meta.Put([]byte("gone"), []byte("1")); err != nil {
			return err
		}
		return meta.Put([]byte("readded"), []byte("1"))
	})
	update(func(meta database.Bucket) error {
		return meta.Delete([]byte("readded"))
	})

	before, err := pdb.Begin(false)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	update(func(meta database.Bucket) error {
		for _, k := range []string{"disk", "gone"} {
			if err := meta.Delete([]byte(k)); err != nil {
				return err
			}
		}
		return meta.Put([]byte("readded"), []byte("2"))
	})
	check("begun before the commit", before,
		map[string]string{"disk": "1", "gone": "1", "readded": ""})
	if err := before.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	want := map[string]string{"disk": "", "gone": "", "readded": "2"}
	view("in the cache", want)
	if err := pdb.cache.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	view("after the flush", want)
}
