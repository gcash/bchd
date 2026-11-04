// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"errors"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/wire"
)

// TestIndexVersions checks that the transaction and slp indexes store their
// versions when they are created, that one built before indexes stored their
// versions is marked version 1, that one a later bchd built is refused before
// it is read, and that dropping an index removes its version.
func TestIndexVersions(t *testing.T) {
	tests := []struct {
		name     string
		newIndex func(db database.DB) Indexer
		drop     func(db database.DB, interrupt <-chan struct{}) error
		flag     string
	}{
		{
			name:     "transaction index",
			newIndex: func(db database.DB) Indexer { return NewTxIndex(db) },
			drop:     DropTxIndex,
			flag:     "--droptxindex",
		},
		{
			name: "slp index",
			newIndex: func(db database.DB) Indexer {
				return NewSlpIndex(db, &SlpConfig{MaxCacheSize: 100})
			},
			drop: DropSlpIndex,
			flag: "--dropslpindex",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			idx := test.newIndex(db)
			version := func() uint32 {
				t.Helper()
				var version uint32
				err := db.View(func(dbTx database.Tx) error {
					var err error
					version, err = dbFetchIndexVersion(dbTx, idx.Key())
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
				return version
			}
			setVersion := func(version uint32) {
				t.Helper()
				err := db.Update(func(dbTx database.Tx) error {
					if version == 0 {
						tips := dbTx.Metadata().Bucket(indexTipsBucketName)
						return tips.Delete(indexVersionKey(idx.Key()))
					}
					return dbPutIndexVersion(dbTx, idx.Key(), version)
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			err = db.Update(func(dbTx database.Tx) error {
				_, err := dbTx.Metadata().CreateBucket(indexTipsBucketName)
				if err != nil {
					return err
				}
				if err := idx.Create(dbTx); err != nil {
					return err
				}
				return dbPutIndexerTip(dbTx, idx.Key(), &chainhash.Hash{}, -1)
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := version(); got != 1 {
				t.Errorf("created at version %d, want 1", got)
			}

			// An index built before indexes stored their versions.
			setVersion(0)
			if err := idx.Init(); err != nil {
				t.Fatalf("initializing an index without a version: %v", err)
			}
			if err := idx.Migrate(db, nil); err != nil {
				t.Fatal(err)
			}
			if got := version(); got != 1 {
				t.Errorf("an index without a version is marked version %d, want 1", got)
			}

			// An index that a later bchd built.
			setVersion(2)
			if err := idx.Init(); err == nil || !strings.Contains(err.Error(), test.flag) {
				t.Errorf("initializing an index of version 2: %v", err)
			}

			// Dropping the index removes its version.
			if err := test.drop(db, nil); err != nil {
				t.Fatal(err)
			}
			err = db.View(func(dbTx database.Tx) error {
				tips := dbTx.Metadata().Bucket(indexTipsBucketName)
				if tips.Get(indexVersionKey(idx.Key())) != nil {
					t.Error("the dropped index's version is still stored")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestIndexVersionRecord checks that an index has no version in a database
// without the bucket of index tips, which storing one creates, and that a
// version of the wrong size is reported as corrupt.
func TestIndexVersionRecord(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	checkCode := func(what string, err error, code database.ErrorCode) {
		t.Helper()
		var dbErr database.Error
		if !errors.As(err, &dbErr) || dbErr.ErrorCode != code {
			t.Errorf("%s: got %v, want %v", what, err, code)
		}
	}
	err = db.Update(func(dbTx database.Tx) error {
		if version, err := dbFetchIndexVersion(dbTx, txIndexKey); err != nil || version != 0 {
			t.Errorf("version without the bucket: %d, %v", version, err)
		}
		if err := dbPutIndexVersion(dbTx, txIndexKey, 1); err != nil {
			return err
		}
		if version, err := dbFetchIndexVersion(dbTx, txIndexKey); err != nil || version != 1 {
			t.Errorf("version stored without the bucket: %d, %v", version, err)
		}

		tips := dbTx.Metadata().Bucket(indexTipsBucketName)
		if err := tips.Put(indexVersionKey(txIndexKey), []byte{1, 0, 0}); err != nil {
			return err
		}
		_, err = dbFetchIndexVersion(dbTx, txIndexKey)
		checkCode("reading a version of 3 bytes", err, database.ErrCorruption)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
