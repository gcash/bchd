// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp/metadatamaker"
)

// batonTx returns a transaction spending prev whose first output holds the
// slp script, with a token output and a mint baton output after it.
func batonTx(t *testing.T, prev wire.OutPoint, script []byte, err error) *wire.MsgTx {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(&prev, nil))
	tx.AddTxOut(&wire.TxOut{PkScript: script})
	tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{0x51}})
	tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{0x51}})
	return tx
}

// batonGenesis returns GENESIS n of a type 1 token with its mint baton on
// output 2.
func batonGenesis(t *testing.T, n byte) *wire.MsgTx {
	t.Helper()
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte{n},
		nil, nil, 0, metadatamaker.NewMintBatonVout(2), 1000)
	return batonTx(t, wire.OutPoint{Hash: chainhash.Hash{0xbb, n}}, script, err)
}

// batonMint returns a MINT of the token that spends the mint baton at prev
// and puts it on output 2.
func batonMint(t *testing.T, token chainhash.Hash, prev wire.OutPoint) *wire.MsgTx {
	t.Helper()
	script, err := metadatamaker.TokenType1Mint(token[:],
		metadatamaker.NewMintBatonVout(2), 10)
	return batonTx(t, prev, script, err)
}

// baton returns where the token metadata the index serves puts the mint
// baton of the token, or nil for none.
func (test *slpTokenIDTest) baton(token chainhash.Hash) *wire.OutPoint {
	test.t.Helper()
	var tm *TokenMetadata
	err := test.db.View(func(dbTx database.Tx) error {
		var err error
		tm, err = test.idx.GetTokenMetadata(dbTx, &SlpTxEntry{TokenIDHash: token})
		return err
	})
	if err != nil {
		test.t.Fatalf("GetTokenMetadata: %v", err)
	}
	if tm.MintBatonHash == nil {
		return nil
	}
	return &wire.OutPoint{Hash: *tm.MintBatonHash, Index: tm.MintBatonVout}
}

// addPotential checks tx as the gRPC server does for a transaction of the
// memory pool or of a block.
func (test *slpTokenIDTest) addPotential(tx *wire.MsgTx) {
	test.t.Helper()
	err := test.db.View(func(dbTx database.Tx) error {
		valid, err := test.idx.AddPotentialSlpEntries(dbTx, tx)
		if err == nil && !valid {
			err = errors.New("not valid")
		}
		return err
	})
	if err != nil {
		test.t.Fatalf("AddPotentialSlpEntries(%v): %v", tx.TxHash(), err)
	}
}

// TestSlpRecheckConfirmed ensures checking again a GENESIS or a MINT the
// index holds, as a gRPC subscriber does for the transactions of each block,
// in block order and perhaps after later blocks, leaves the token's metadata
// as the index has it, instead of putting back the mint baton where that
// transaction put it.
func TestSlpRecheckConfirmed(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis := batonGenesis(t, 1)
	token := tokenIDOf(t, genesis)
	mint1 := batonMint(t, token, wire.OutPoint{Hash: genesis.TxHash(), Index: 2})
	mint2 := batonMint(t, token, wire.OutPoint{Hash: mint1.TxHash(), Index: 2})
	test.connect(genesis)
	test.connect(mint1)
	test.connect(mint2)

	want := wire.OutPoint{Hash: mint2.TxHash(), Index: 2}
	for _, tx := range []*wire.MsgTx{mint1, genesis} {
		test.addPotential(tx)
		if got := test.baton(token); got == nil || *got != want {
			t.Errorf("after checking %v again, baton %v, want %v",
				tx.TxHash(), got, want)
		}
	}
}

// TestSlpConfirmedGenesisBurn ensures the token metadata a GENESIS set while
// in the memory pool is dropped once a block confirms the GENESIS, so the
// metadata shows a later block burning its mint baton.
func TestSlpConfirmedGenesisBurn(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis := batonGenesis(t, 1)
	token := tokenIDOf(t, genesis)
	baton := wire.OutPoint{Hash: genesis.TxHash(), Index: 2}
	test.addPotential(genesis)
	if got := test.baton(token); got == nil || *got != baton {
		t.Fatalf("GENESIS in the memory pool: baton %v, want %v", got, baton)
	}
	test.connect(genesis)

	// A transaction that is not slp spends the baton.
	burn := wire.NewMsgTx(1)
	burn.AddTxIn(wire.NewTxIn(&baton, nil))
	burn.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{0x51}})
	test.connect(burn)
	if got := test.baton(token); got != nil {
		t.Errorf("baton %v after it was burned", got)
	}
}

// TestSlpMempoolMetadataLifetime ensures the token metadata a memory pool
// transaction set stays while that transaction is in the memory pool, also
// when a block confirms the transaction whose metadata it replaced, and is
// removed with the transaction's entry once it is no longer in the pool.
func TestSlpMempoolMetadataLifetime(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis := batonGenesis(t, 1)
	token := tokenIDOf(t, genesis)
	mint := batonMint(t, token, wire.OutPoint{Hash: genesis.TxHash(), Index: 2})
	test.addPotential(genesis)
	test.addPotential(mint)
	onMint := wire.OutPoint{Hash: mint.TxHash(), Index: 2}

	pool := map[chainhash.Hash]bool{genesis.TxHash(): true, mint.TxHash(): true}
	inPool := func(hash *chainhash.Hash) bool { return pool[*hash] }
	check := func(when string, want wire.OutPoint) {
		t.Helper()
		if got := test.baton(token); got == nil || *got != want {
			t.Errorf("%s: baton %v, want %v", when, got, want)
		}
	}
	test.idx.RemoveMempoolSlpTxsNotIn(inPool)
	check("both in the memory pool", onMint)

	test.connect(genesis)
	delete(pool, genesis.TxHash())
	check("GENESIS confirmed", onMint)
	test.idx.RemoveMempoolSlpTxsNotIn(inPool)
	check("GENESIS confirmed, MINT in the memory pool", onMint)

	delete(pool, mint.TxHash())
	test.idx.RemoveMempoolSlpTxsNotIn(inPool)
	check("MINT gone", wire.OutPoint{Hash: genesis.TxHash(), Index: 2})
	mintHash := mint.TxHash()
	if test.idx.cache.GetMempoolItem(&mintHash) != nil {
		t.Error("the MINT's mempool entry is still there")
	}
}
