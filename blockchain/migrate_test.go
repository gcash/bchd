// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

// TestBlockIndexMigrationErrorIsReported ensures New returns a failed block
// index migration instead of carrying on without a chain tip.
func TestBlockIndexMigrationErrorIsReported(t *testing.T) {
	db, err := database.Create(testDbType, filepath.Join(t.TempDir(), "db"),
		wire.MainNet)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// A chain state without either block index bucket can't be migrated.
	if err := db.Update(func(tx database.Tx) error {
		return tx.Metadata().Put(chainStateKeyName, serializeBestChainState(
			bestChainState{hash: *chaincfg.MainNetParams.GenesisHash,
				workSum: big.NewInt(1)}))
	}); err != nil {
		t.Fatal(err)
	}

	var newErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("New panicked instead of returning the "+
					"migration error: %v", r)
			}
		}()
		_, newErr = New(&Config{
			DB:                 db,
			ChainParams:        &chaincfg.MainNetParams,
			TimeSource:         NewMedianTime(),
			ExcessiveBlockSize: 32000000,
		})
	}()
	if newErr == nil {
		t.Fatalf("New succeeded although the block index migration failed")
	}
}
