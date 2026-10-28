// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

// TestSlpBatonBurnPoolEntry ensures a block that burns the mint baton of a
// GENESIS or MINT of the memory pool, which it or an earlier block confirms
// while the memory pool's entry of the transaction, of internal token ID 0, is
// still cached, records the baton burned in the token's own metadata, rather
// than in metadata under ID 0, which it would also make the token's ID.
func TestSlpBatonBurnPoolEntry(t *testing.T) {
	burnOf := func(prev wire.OutPoint) *wire.MsgTx {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&prev, nil))
		tx.AddTxOut(&wire.TxOut{Value: 500, PkScript: []byte{0x51}})
		return tx
	}
	check := func(what string, test *slpTokenIDTest, genesis *wire.MsgTx) {
		t.Helper()
		token, hash := tokenIDOf(t, genesis), genesis.TxHash()
		err := test.db.View(func(dbTx database.Tx) error {
			entry, err := dbFetchSlpIndexEntry(dbTx, &hash)
			if err != nil {
				return err
			}
			id, err := dbFetchTokenIDByHash(dbTx, &token)
			if err != nil || id != entry.TokenID {
				t.Errorf("%s: the token's internal ID is %d (%v), want the "+
					"GENESIS's %d", what, id, err, entry.TokenID)
			}
			tm, err := dbFetchTokenMetadataByID(dbTx, entry.TokenID)
			if err != nil || tm.MintBatonHash != nil {
				t.Errorf("%s: the token's metadata is %+v (%v), want the mint "+
					"baton burned", what, tm, err)
			}
			if _, err := dbFetchTokenMetadataByID(dbTx, 0); !errors.Is(err,
				errNoTokenMetadataEntry) {

				t.Errorf("%s: metadata under internal ID 0: %v", what, err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// A GENESIS of the memory pool and the burn of its mint baton in the
	// block that confirms it.
	test := newSlpTokenIDTest(t)
	genesis := batonGenesis(t, 1)
	test.addPotential(genesis)
	test.connect(genesis, burnOf(wire.OutPoint{Hash: genesis.TxHash(), Index: 2}))
	check("a GENESIS of the memory pool", test, genesis)

	// A MINT of the memory pool and the burn of the mint baton it moved, in
	// the block that confirms it.
	test = newSlpTokenIDTest(t)
	genesis = batonGenesis(t, 2)
	test.connect(genesis)
	mint := batonMint(t, tokenIDOf(t, genesis), wire.OutPoint{Hash: genesis.TxHash(),
		Index: 2})
	test.addPotential(mint)
	test.connect(mint, burnOf(wire.OutPoint{Hash: mint.TxHash(), Index: 2}))
	check("a MINT of the memory pool", test, genesis)

	// A GENESIS of the memory pool whose entry there the block confirming
	// it leaves, as while graph search loads, and the burn of its mint
	// baton in the next block.
	test = newSlpTokenIDTest(t)
	test.idx.config.SlpGraphSearchEnabled = true
	genesis = batonGenesis(t, 3)
	test.addPotential(genesis)
	test.connect(genesis)
	hash := genesis.TxHash()
	if entry := test.idx.cache.GetMempoolItem(&hash); entry == nil || entry.TokenID != 0 {
		t.Fatalf("the memory pool's entry of the GENESIS is %+v, want it kept", entry)
	}
	test.connect(burnOf(wire.OutPoint{Hash: hash, Index: 2}))
	check("a GENESIS whose memory pool entry stays", test, genesis)
}
