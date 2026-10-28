// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp/metadatamaker"
)

// TestSlpPoolMintEndsBaton ensures a MINT of the memory pool that ends its
// token's mint baton, with no baton output, shows the baton ended in the
// token's metadata, as the index records once the MINT is confirmed, rather
// than the baton the MINT spent, and that the baton is back where it was once
// the MINT leaves the memory pool unconfirmed.  A MINT that moves the baton
// keeps doing so, which other tests check.
func TestSlpPoolMintEndsBaton(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis := batonGenesis(t, 1)
	test.connect(genesis)
	token := tokenIDOf(t, genesis)
	baton := wire.OutPoint{Hash: genesis.TxHash(), Index: 2}
	script, err := metadatamaker.TokenType1Mint(token[:], nil, 10)
	ending := batonTx(t, baton, script, err)

	test.addPotential(ending)
	if got := test.baton(token); got != nil {
		t.Errorf("with the MINT in the memory pool, the mint baton is on %v, "+
			"want it ended", got)
	}
	pooled, ok := test.idx.cache.GetTokenMetadata(&token, nil)
	if !ok {
		t.Fatal("with the MINT in the memory pool, the token's metadata is not cached")
	}

	// The MINT leaves the memory pool unconfirmed.
	test.idx.RemoveMempoolSlpTxsNotIn(func(*chainhash.Hash) bool { return false })
	if got := test.baton(token); got == nil || *got != baton {
		t.Errorf("with the MINT gone from the memory pool, the mint baton is "+
			"on %v, want %v", got, baton)
	}

	// The MINT is back in the memory pool and then confirmed.
	test.addPotential(ending)
	test.connect(ending)
	if got := test.baton(token); got != nil {
		t.Errorf("with the MINT confirmed, the mint baton is on %v, want it "+
			"ended", got)
	}
	hash := genesis.TxHash()
	err = test.db.View(func(dbTx database.Tx) error {
		entry, err := dbFetchSlpIndexEntry(dbTx, &hash)
		if err != nil {
			return err
		}
		tm, err := dbFetchTokenMetadataByID(dbTx, entry.TokenID)
		if err != nil || tm.MintBatonHash != nil {
			t.Errorf("the index records the token's metadata as %+v (%v), "+
				"want the mint baton ended", tm, err)
			return nil
		}

		// The metadata the MINT had in the memory pool is what the index
		// records for it confirmed.
		if *pooled.TokenID != *tm.TokenID || pooled.SlpVersion != tm.SlpVersion ||
			pooled.MintBatonHash != nil || pooled.MintBatonVout != tm.MintBatonVout ||
			pooled.NftGroupID != nil || tm.NftGroupID != nil {

			t.Errorf("the token's metadata with the MINT in the memory pool is "+
				"%+v, want %+v, as confirmed", pooled, *tm)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
