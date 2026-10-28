// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

// errReadFailed is the error of a readFailingDB's transaction whose read of a
// token metadata record failed.
var errReadFailed = errors.New("the read failed")

// readFailingDB is a database whose transactions, while failing is set, read
// no token metadata record and then fail with errReadFailed, as a database
// transaction fails after a read that failed.
type readFailingDB struct {
	database.DB
	failing atomic.Bool
}

func (db *readFailingDB) View(fn func(database.Tx) error) error {
	tx := &readFailingTx{failing: db.failing.Load()}
	err := db.DB.View(func(dbTx database.Tx) error {
		tx.Tx = dbTx
		return fn(tx)
	})
	if tx.failed {
		return errReadFailed
	}
	return err
}

// readFailingTx is a transaction of a readFailingDB.
type readFailingTx struct {
	database.Tx
	failing, failed bool
}

func (tx *readFailingTx) Metadata() database.Bucket {
	return &readFailingBucket{wrappedBucket: tx.Tx.Metadata(), tx: tx}
}

// readFailingBucket is a bucket of a readFailingTx.
type readFailingBucket struct {
	wrappedBucket
	tx     *readFailingTx
	failOn bool
}

func (b *readFailingBucket) Bucket(key []byte) database.Bucket {
	bucket := b.wrappedBucket.Bucket(key)
	if bucket == nil {
		return nil
	}
	return &readFailingBucket{wrappedBucket: bucket, tx: b.tx,
		failOn: bytes.Equal(key, tokenMetadataByIDIndexBucketName)}
}

func (b *readFailingBucket) Get(key []byte) []byte {
	if b.failOn && b.tx.failing {
		b.tx.failed = true
		return nil
	}
	return b.wrappedBucket.Get(key)
}

// TestSlpPotentialMintMetadataFailure ensures the check of a MINT of the
// memory pool that fails to read its token's metadata, which the index reads
// to record the MINT's mint baton, reports the failure rather than take the
// MINT for valid with the baton where it was: a read that fails as the check's
// own, which the caller checks again, and a corrupt record as the corrupt
// entry it is, leaving the MINT unjudged either way.  Once the read succeeds,
// the MINT is valid and its token's metadata has the baton on its output.  A
// MINT of a token the index holds no metadata for is valid.
func TestSlpPotentialMintMetadataFailure(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		test := newSlpTokenIDTest(t)
		db := &readFailingDB{DB: test.db}
		test.idx = NewSlpIndex(db, &SlpConfig{MaxCacheSize: 100})
		if err := test.idx.Init(); err != nil {
			t.Fatal(err)
		}
		genesis := batonGenesis(t, 1)
		test.connect(genesis)
		token := tokenIDOf(t, genesis)
		mint := batonMint(t, token, wire.OutPoint{Hash: genesis.TxHash(), Index: 2})
		mintHash := mint.TxHash()
		check := func() (bool, error) {
			var valid bool
			err := db.View(func(dbTx database.Tx) error {
				var err error
				valid, err = test.idx.AddPotentialSlpEntries(dbTx, mint)
				return err
			})
			return valid, err
		}

		what := "a read that fails"
		if corrupt {
			what = "a corrupt record"
			corruptSlpEntry(t, test.db, genesis.TxHash(), slpEntryCorruptions[2].corrupt)
		} else {
			db.failing.Store(true)
		}
		valid, err := check()
		if corrupt {
			if valid {
				t.Errorf("%s: the MINT is valid", what)
			}
			checkSlpEntryCorrupt(t, what, err, genesis.TxHash())
		}
		if !corrupt && (valid || !errors.Is(err, errReadFailed) || IsSlpEntryCorrupt(err)) {
			t.Errorf("%s: got %v and %v, want the read's failure", what, valid, err)
		}
		if test.idx.cache.GetMempoolItem(&mintHash) != nil {
			t.Errorf("%s: the MINT is cached", what)
		}
		if corrupt {
			continue
		}

		db.failing.Store(false)
		if valid, err := check(); !valid || err != nil {
			t.Fatalf("%s, then a read that succeeds: got %v and %v", what, valid, err)
		}
		if tm, ok := test.idx.cache.GetTokenMetadata(&token, nil); !ok ||
			tm.MintBatonHash == nil || *tm.MintBatonHash != mintHash {
			t.Errorf("%s, then a read that succeeds: the token's metadata is "+
				"%+v, %v, want the baton on the MINT", what, tm, ok)
		}
	}

	// A MINT of a token the index holds no metadata for, of a GENESIS of the
	// memory pool whose metadata the cache no longer holds, is valid, with
	// none to update.
	test := newSlpTokenIDTest(t)
	genesis := batonGenesis(t, 2)
	test.addPotential(genesis)
	token := tokenIDOf(t, genesis)
	test.idx.cache.RemoveTokenMetadata(token, genesis.TxHash())
	mint := batonMint(t, token, wire.OutPoint{Hash: genesis.TxHash(), Index: 2})
	test.addPotential(mint)
	if _, ok := test.idx.cache.GetTokenMetadata(&token, nil); ok {
		t.Error("a MINT of a token the index holds no metadata for: the token " +
			"has metadata")
	}
}
