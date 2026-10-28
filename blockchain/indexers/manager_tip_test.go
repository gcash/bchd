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
	"github.com/gcash/bchutil"
)

// TestIndexerTipMissingBucket ensures reading or writing the tip of an index in
// a database without the bucket of index tips, which the index manager
// creates, fails with an error rather than a panic, so that GetTokenMetadata,
// which reads the slp index's tip, still serves a token's metadata.
func TestIndexerTipMissingBucket(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	idx := NewSlpIndex(db, &SlpConfig{MaxCacheSize: 100})
	if err := db.Update(idx.Create); err != nil {
		t.Fatal(err)
	}
	genesis := genesisTx(t, 1)
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{genesis}})
	err = db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	})
	if err != nil {
		t.Fatal(err)
	}

	hash := genesis.TxHash()
	err = db.Update(func(dbTx database.Tx) error {
		err := dbPutIndexerTip(dbTx, idx.Key(), &hash, 1)
		var dbErr database.Error
		if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrBucketNotFound {
			t.Errorf("writing the tip without its bucket: got %v, want %v",
				err, database.ErrBucketNotFound)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.View(func(dbTx database.Tx) error {
		_, _, err := dbFetchIndexerTip(dbTx, idx.Key())
		var dbErr database.Error
		if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrBucketNotFound {
			t.Errorf("the tip without its bucket: got %v, want %v", err,
				database.ErrBucketNotFound)
		}
		entry, err := idx.GetSlpIndexEntry(dbTx, &hash)
		if err != nil {
			return err
		}
		tm, err := idx.GetTokenMetadata(dbTx, entry)
		if err != nil || *tm.TokenID != tokenIDOf(t, genesis) {
			t.Errorf("the GENESIS's token metadata: got %v and %v", tm, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
