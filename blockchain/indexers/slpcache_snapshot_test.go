// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp/v1parser"
)

// TestTokenMetadataCacheSnapshot ensures token metadata read from an older
// database snapshot, as a query running while a block commits reads it, is
// not served to readers of a later one, so a mint baton the block moved is
// not reported after it.
func TestTokenMetadataCacheSnapshot(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	idx := NewSlpIndex(db, &SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		_, err := dbTx.Metadata().CreateBucketIfNotExists(indexTipsBucketName)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenHash := chainhash.Hash{1}
	put := func(baton, tip chainhash.Hash) {
		t.Helper()
		err := db.Update(func(dbTx database.Tx) error {
			err := dbPutTokenIDIndexEntry(dbTx, 1, &TokenMetadata{
				TokenID:       &tokenHash,
				SlpVersion:    v1parser.TokenTypeFungible01,
				MintBatonHash: &baton,
				MintBatonVout: 2,
			})
			if err != nil {
				return err
			}
			return dbPutIndexerTip(dbTx, idx.Key(), &tip, 1)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	oldBaton, newBaton := chainhash.Hash{2}, chainhash.Hash{3}
	put(oldBaton, chainhash.Hash{0xa})
	old, err := db.Begin(false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.Rollback() }()

	// A block moves the baton while a query still reads the old snapshot.
	put(newBaton, chainhash.Hash{0xb})
	entry := &SlpTxEntry{TokenID: 1, TokenIDHash: tokenHash}
	tm, err := idx.GetTokenMetadata(old, entry)
	if err != nil || *tm.MintBatonHash != oldBaton {
		t.Fatalf("old snapshot: got %v, %v, want baton %v", tm, err, oldBaton)
	}

	err = db.View(func(dbTx database.Tx) error {
		tm, err = idx.GetTokenMetadata(dbTx, entry)
		return err
	})
	if err != nil || *tm.MintBatonHash != newBaton {
		t.Fatalf("later reader: got %v, %v, want baton %v", tm, err, newBaton)
	}
}

// TestTokenMetadataCacheMempool ensures token metadata from the memory pool,
// such as a genesis not yet in a block, is served whatever the reader's
// snapshot, as the database does not have it.
func TestTokenMetadataCacheMempool(t *testing.T) {
	cache := InitSlpCache(10)
	tokenHash := chainhash.Hash{1}
	if err := cache.AddTempTokenMetadata(TokenMetadata{TokenID: &tokenHash},
		chainhash.Hash{2}); err != nil {
		t.Fatal(err)
	}
	for _, tip := range []*chainhash.Hash{nil, {0xa}} {
		if _, ok := cache.GetTokenMetadata(&tokenHash, tip); !ok {
			t.Errorf("memory pool metadata not served at tip %v", tip)
		}
	}

	// Metadata read at a tip is served at that tip only.
	if err := cache.AddTokenMetadataAt(TokenMetadata{TokenID: &tokenHash},
		chainhash.Hash{0xa}); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.GetTokenMetadata(&tokenHash, &chainhash.Hash{0xa}); !ok {
		t.Error("metadata not served at its own tip")
	}
	for _, tip := range []*chainhash.Hash{nil, {0xb}} {
		if _, ok := cache.GetTokenMetadata(&tokenHash, tip); ok {
			t.Errorf("metadata read at another tip served at %v", tip)
		}
	}
}
