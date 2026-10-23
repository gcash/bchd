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
	"github.com/gcash/bchutil"
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

// TestSlpRemoveNotInUnlocked ensures RemoveMempoolSlpTxsNotIn asks the memory
// pool about each transaction with no slp cache lock held, and then removes
// the entry and token metadata of a transaction that left it and keeps those
// of one still there.  Asking takes the memory pool's lock, whose holder may
// be waiting for the chain lock, which block connection holds while it takes
// the slp cache lock.
func TestSlpRemoveNotInUnlocked(t *testing.T) {
	test := newSlpTokenIDTest(t)
	left, pooled := batonGenesis(t, 1), batonGenesis(t, 2)
	test.addPotential(left)
	test.addPotential(pooled)
	pooledHash := pooled.TxHash()

	asked := 0
	test.idx.RemoveMempoolSlpTxsNotIn(func(hash *chainhash.Hash) bool {
		asked++
		if !test.idx.cache.TryLock() {
			t.Error("the slp cache lock is held while the memory pool is asked")
		} else {
			test.idx.cache.Unlock()
		}
		return *hash == pooledHash
	})
	if asked == 0 {
		t.Fatal("the memory pool was not asked")
	}
	for _, c := range []struct {
		name string
		tx   *wire.MsgTx
		want bool
	}{
		{"the GENESIS that left", left, false},
		{"the GENESIS in the memory pool", pooled, true},
	} {
		hash := c.tx.TxHash()
		token := tokenIDOf(t, c.tx)
		if got := test.idx.cache.GetMempoolItem(&hash) != nil; got != c.want {
			t.Errorf("%s: mempool entry %v, want %v", c.name, got, c.want)
		}
		if _, got := test.idx.cache.GetTokenMetadata(&token, nil); got != c.want {
			t.Errorf("%s: token metadata %v, want %v", c.name, got, c.want)
		}
		test.idx.cache.RLock()
		_, got := test.idx.cache.mempoolTokenMetadata[token]
		test.idx.cache.RUnlock()
		if got != c.want {
			t.Errorf("%s: the transaction that set the token metadata "+
				"recorded %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSlpRemoveNotInChangedMeanwhile ensures RemoveMempoolSlpTxsNotIn keeps an
// entry or token metadata put in the slp cache while the memory pool was
// asked, as it is not the one the memory pool was asked about: an entry
// removed and added again, and token metadata that a MINT set since.
func TestSlpRemoveNotInChangedMeanwhile(t *testing.T) {
	genesis := batonGenesis(t, 1)
	token := tokenIDOf(t, genesis)
	genesisHash := genesis.TxHash()
	mint := batonMint(t, token, wire.OutPoint{Hash: genesisHash, Index: 2})
	mintHash := mint.TxHash()

	// removeNotIn runs RemoveMempoolSlpTxsNotIn on cache, which runs
	// meanwhile the first time it asks the memory pool, and reports every
	// transaction gone.
	removeNotIn := func(cache *SlpCache, meanwhile func()) {
		t.Helper()
		cache.RemoveMempoolSlpTxsNotIn(func(*chainhash.Hash) bool {
			if !cache.TryLock() {
				t.Error("the slp cache lock is held while the memory pool is asked")
				return false
			}
			cache.Unlock()
			if meanwhile != nil {
				meanwhile()
				meanwhile = nil
			}
			return false
		})
	}

	// The cache holds one entry besides those of the memory pool, so the
	// GENESIS's leaves it when another is read.  A block confirms the
	// GENESIS, and once its entry is gone, the GENESIS is back in the
	// memory pool, as after a reorg, and is checked again.
	cache := InitSlpCache(1)
	cache.AddMempoolSlpTxEntry(&genesisHash, SlpTxEntry{TokenIDHash: token})
	removeNotIn(cache, func() {
		cache.RemoveMempoolSlpTxItems([]*bchutil.Tx{bchutil.NewTx(genesis)})
		other := chainhash.Hash{0xee}
		if err := cache.AddSlpTxEntry(&other, SlpTxEntry{}); err != nil {
			t.Fatal(err)
		}
		cache.AddMempoolSlpTxEntry(&genesisHash, SlpTxEntry{TokenIDHash: token})
	})
	if cache.GetMempoolItem(&genesisHash) == nil {
		t.Error("the entry added again while the memory pool was asked is gone")
	}

	// A MINT in the memory pool moves the mint baton that the GENESIS put
	// on its output 2.
	cache = InitSlpCache(10)
	err := cache.AddTempTokenMetadata(TokenMetadata{TokenID: &token,
		MintBatonHash: &genesisHash, MintBatonVout: 2}, genesisHash)
	if err != nil {
		t.Fatal(err)
	}
	removeNotIn(cache, func() {
		err := cache.AddTempTokenMetadata(TokenMetadata{TokenID: &token,
			MintBatonHash: &mintHash, MintBatonVout: 2}, mintHash)
		if err != nil {
			t.Fatal(err)
		}
	})
	if tm, ok := cache.GetTokenMetadata(&token, nil); !ok ||
		tm.MintBatonHash == nil || *tm.MintBatonHash != mintHash {

		t.Errorf("the token metadata the MINT set while the memory pool "+
			"was asked is %v, %v", tm, ok)
	}
}

// TestSlpMempoolEntryBackInPool ensures a transaction that comes back to the
// memory pool after its memory pool entry was removed is pinned in the slp
// cache again, even though the cache's LRU still holds its entry, so that the
// LRU cannot evict the entry while the transaction is in the pool.  The entry
// is all the index has of a transaction in the pool: once it is evicted, the
// transaction is not valid slp to the index, and neither is a transaction
// that spends its tokens.
func TestSlpMempoolEntryBackInPool(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis := genesisTx(t, 1)
	hash := genesis.TxHash()
	token := tokenIDOf(t, genesis)

	// The GENESIS is in the memory pool, then leaves it, as when it
	// expires, so the next block event removes its memory pool entry,
	// and then comes back, as when its wallet sends it again.
	test.addPotential(genesis)
	test.idx.RemoveMempoolSlpTxsNotIn(func(*chainhash.Hash) bool { return false })
	if test.idx.cache.GetMempoolItem(&hash) != nil {
		t.Fatal("the GENESIS that left the memory pool is still pinned")
	}
	test.addPotential(genesis)
	if test.idx.cache.GetMempoolItem(&hash) == nil {
		t.Error("the GENESIS back in the memory pool is not pinned")
	}

	// Other entries fill the LRU, which holds 100.
	for i := 0; i < 100; i++ {
		other := chainhash.Hash{0xaa, byte(i)}
		if err := test.idx.cache.AddSlpTxEntry(&other, SlpTxEntry{}); err != nil {
			t.Fatal(err)
		}
	}
	err := test.db.View(func(dbTx database.Tx) error {
		_, err := test.idx.GetSlpIndexEntry(dbTx, &hash)
		return err
	})
	if err != nil {
		t.Errorf("the GENESIS in the memory pool has no slp entry: %v", err)
	}

	// A SEND of its tokens.
	script, err := metadatamaker.TokenType1Send(token[:], []uint64{1000})
	if err != nil {
		t.Fatal(err)
	}
	send := wire.NewMsgTx(1)
	send.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: hash, Index: 1}, nil))
	send.AddTxOut(&wire.TxOut{PkScript: script})
	send.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{0x51}})
	var valid bool
	err = test.db.View(func(dbTx database.Tx) error {
		var err error
		valid, err = test.idx.AddPotentialSlpEntries(dbTx, send)
		return err
	})
	if err != nil || !valid {
		t.Errorf("a SEND of the tokens of the GENESIS in the memory pool is "+
			"%v, %v; want valid", valid, err)
	}
}

// TestSlpRemoveNotInCheckedAgain ensures checking a transaction again while
// its memory pool entry is in the slp cache, as each gRPC subscriber does for
// the transactions it sends, keeps that entry, so that RemoveMempoolSlpTxsNotIn
// still removes it once the memory pool reports the transaction gone, even
// when the check comes while the pool is asked.
func TestSlpRemoveNotInCheckedAgain(t *testing.T) {
	cache := InitSlpCache(10)
	genesis := genesisTx(t, 1)
	hash := genesis.TxHash()
	entry := SlpTxEntry{TokenIDHash: tokenIDOf(t, genesis)}
	cache.AddMempoolSlpTxEntry(&hash, entry)
	cache.RemoveMempoolSlpTxsNotIn(func(*chainhash.Hash) bool {
		cache.AddMempoolSlpTxEntry(&hash, entry)
		return false
	})
	if cache.GetMempoolItem(&hash) != nil {
		t.Error("the entry of the transaction that left the memory pool is " +
			"still there")
	}
}
