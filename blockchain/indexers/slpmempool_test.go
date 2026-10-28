// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
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

// TestSlpConfirmedMintMetadata ensures a block confirming a MINT leaves the
// token metadata that a later MINT in the memory pool set, one spending the
// confirmed MINT's mint baton, so that the baton is reported where that MINT
// puts it rather than on the output it spends.  The token metadata of a MINT
// the block double-spends still goes at the block's event, once the memory
// pool has dropped that MINT, and that of the confirmed MINT itself goes at
// once, also while graph search loads, when the index keeps the memory pool
// entries of the block's transactions.
func TestSlpConfirmedMintMetadata(t *testing.T) {
	genesis := batonGenesis(t, 1)
	token := tokenIDOf(t, genesis)
	onGenesis := wire.OutPoint{Hash: genesis.TxHash(), Index: 2}
	mint1 := batonMint(t, token, onGenesis)
	onMint1 := wire.OutPoint{Hash: mint1.TxHash(), Index: 2}
	mint2 := batonMint(t, token, onMint1)
	onMint2 := wire.OutPoint{Hash: mint2.TxHash(), Index: 2}
	check := func(test *slpTokenIDTest, when string, want wire.OutPoint) {
		t.Helper()
		if got := test.baton(token); got == nil || *got != want {
			t.Errorf("%s: baton %v, want %v", when, got, want)
		}
	}

	// Both MINTs are in the memory pool, and a block confirms the first.
	test := newSlpTokenIDTest(t)
	test.connect(genesis)
	test.addPotential(mint1)
	test.addPotential(mint2)
	pool := map[chainhash.Hash]bool{mint2.TxHash(): true}
	inPool := func(hash *chainhash.Hash) bool { return pool[*hash] }
	test.connect(mint1)
	check(test, "first MINT confirmed, second in the memory pool", onMint2)
	test.idx.RemoveMempoolSlpTxsNotIn(inPool)
	check(test, "at the block's event", onMint2)
	delete(pool, mint2.TxHash())
	test.idx.RemoveMempoolSlpTxsNotIn(inPool)
	check(test, "second MINT gone", onMint1)

	// A block confirms a MINT that double-spends the one in the memory
	// pool, which the memory pool then drops.
	test = newSlpTokenIDTest(t)
	test.connect(genesis)
	test.addPotential(mint1)
	script, err := metadatamaker.TokenType1Mint(token[:],
		metadatamaker.NewMintBatonVout(2), 20)
	other := batonTx(t, onGenesis, script, err)
	test.connect(other)
	test.idx.RemoveMempoolSlpTxsNotIn(func(*chainhash.Hash) bool { return false })
	check(test, "double-spending MINT confirmed, at the block's event",
		wire.OutPoint{Hash: other.TxHash(), Index: 2})

	// Graph search is on but has no database yet, as while it loads, so
	// the index keeps the memory pool entries of a block's transactions
	// until the block's event.  A block confirms the MINT in the memory
	// pool and one spending its mint baton that the memory pool never had.
	test = newSlpTokenIDTest(t)
	test.idx.config.SlpGraphSearchEnabled = true
	test.connect(genesis)
	test.addPotential(mint1)
	test.connect(mint1, mint2)
	check(test, "both MINTs confirmed while graph search loads", onMint2)
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

// TestSlpEntryLeftPool ensures the slp entry of a transaction that left the
// memory pool unconfirmed goes from the slp cache at the next block's event,
// the cache's LRU as well, so that the transaction is no longer valid slp to
// the index, while the entry of one the block confirmed stays, also while
// graph search loads and the memory pool entry of a confirmed transaction
// stays until the block's event, and so does that of one still in the pool.
func TestSlpEntryLeftPool(t *testing.T) {
	test := newSlpTokenIDTest(t)
	test.idx.config.SlpGraphSearchEnabled = true
	left, confirmed, pooled := genesisTx(t, 1), genesisTx(t, 2), genesisTx(t, 3)
	for _, tx := range []*wire.MsgTx{left, confirmed, pooled} {
		test.addPotential(tx)
	}
	test.connect(confirmed)
	pooledHash := pooled.TxHash()
	test.idx.RemoveMempoolSlpTxsNotIn(func(hash *chainhash.Hash) bool {
		return *hash == pooledHash
	})

	for _, c := range []struct {
		name  string
		tx    *wire.MsgTx
		valid bool
	}{
		{"the GENESIS that left", left, false},
		{"the GENESIS confirmed", confirmed, true},
		{"the GENESIS in the memory pool", pooled, true},
	} {
		hash := c.tx.TxHash()
		if _, ok := test.idx.cache.GetSlpTxEntry(&hash); ok != c.valid {
			t.Errorf("%s: cached %v, want %v", c.name, ok, c.valid)
		}
		err := test.db.View(func(dbTx database.Tx) error {
			_, err := test.idx.GetSlpIndexEntry(dbTx, &hash)
			return err
		})
		if (err == nil) != c.valid {
			t.Errorf("%s: slp entry error %v, want valid %v", c.name, err,
				c.valid)
		}
	}

	// The entry of the confirmed GENESIS is the index's, with its token's
	// internal ID.
	hash := confirmed.TxHash()
	if entry, _ := test.idx.cache.GetSlpTxEntry(&hash); entry.TokenID == 0 {
		t.Error("the confirmed GENESIS has the memory pool's entry")
	}
}

// TestSlpEntryScript ensures the slp cache keeps of a transaction only its slp
// message, not the buffer that holds all the scripts of a deserialized
// transaction, as from a block or the network, since the cache holds up to
// 100,000 entries, of transactions the memory pool and blocks let go of.
func TestSlpEntryScript(t *testing.T) {
	const n, scripts = 500, 10000
	test := newSlpTokenIDTest(t)
	test.idx = NewSlpIndex(test.db, &SlpConfig{MaxCacheSize: 2 * n})
	if err := test.idx.Init(); err != nil {
		t.Fatal(err)
	}

	// decoded returns another GENESIS, deserialized, with an output of a
	// large script.
	next := 0
	decoded := func() *wire.MsgTx {
		t.Helper()
		tx := graphSearchGenesis(t, next)
		next++
		tx.AddTxOut(&wire.TxOut{Value: 546,
			PkScript: bytes.Repeat([]byte{0x51}, scripts)})
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		var decoded wire.MsgTx
		if err := decoded.Deserialize(&buf); err != nil {
			t.Fatal(err)
		}
		return &decoded
	}
	for _, c := range []struct {
		name string
		add  func(*wire.MsgTx)
	}{
		{"from the memory pool", test.addPotential},
		{"from blocks", func(tx *wire.MsgTx) { test.connect(tx) }},
	} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for i := 0; i < n; i++ {
			c.add(decoded())
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		held := (int64(after.HeapAlloc) - int64(before.HeapAlloc)) / n
		t.Logf("%s: %d bytes held for each transaction", c.name, held)
		if held > 2000 {
			t.Errorf("%s: %d bytes held for each transaction of %d bytes "+
				"of scripts", c.name, held, scripts)
		}
	}
}

// TestSlpDisconnectedEntries ensures the slp entries of the transactions of a
// block a reorg disconnects stay valid slp until the memory pool is next
// asked, and then only for those the memory pool took back, as the reorg puts
// them back in it without the check a new memory pool transaction gets, and
// for those a block confirms again, also when the entry of one in the memory
// pool before was kept while graph search loads.  An entry the index cannot
// read is left out and logged, without failing the disconnection.
func TestSlpDisconnectedEntries(t *testing.T) {
	test := newSlpTokenIDTest(t)
	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("INDX")
	logger.SetLevel(bchlog.LevelInfo)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)
	back, gone, again, pooled := genesisTx(t, 1), genesisTx(t, 2),
		genesisTx(t, 3), genesisTx(t, 4)
	backHash := back.TxHash()
	valid := func(tx *wire.MsgTx) bool {
		t.Helper()
		hash := tx.TxHash()
		err := test.db.View(func(dbTx database.Tx) error {
			_, err := test.idx.GetSlpIndexEntry(dbTx, &hash)
			return err
		})
		return err == nil
	}

	// The last was in the memory pool before a block confirmed it, while
	// graph search loads, so its memory pool entry stays.  The block also
	// holds a transaction that is not slp.
	test.idx.config.SlpGraphSearchEnabled = true
	test.addPotential(pooled)
	test.connect(back, gone, again, pooled, notSlpTx())
	test.disconnect(back, gone, again, pooled, notSlpTx())
	for i, tx := range []*wire.MsgTx{back, gone, again, pooled} {
		if !valid(tx) {
			t.Errorf("GENESIS %d is not valid once disconnected", i)
		}
	}
	if buf.Len() != 0 {
		t.Errorf("disconnecting logged %q", buf.String())
	}
	test.connect(again)
	test.idx.RemoveMempoolSlpTxsNotIn(func(hash *chainhash.Hash) bool {
		return *hash == backHash
	})
	for _, c := range []struct {
		name  string
		tx    *wire.MsgTx
		valid bool
	}{
		{"the GENESIS the memory pool took back", back, true},
		{"the GENESIS it did not", gone, false},
		{"the GENESIS confirmed again", again, true},
		{"the GENESIS pinned before", pooled, false},
	} {
		if got := valid(c.tx); got != c.valid {
			t.Errorf("%s: valid %v, want %v", c.name, got, c.valid)
		}
	}

	// The entry of a confirmed transaction is cut short.
	broken := genesisTx(t, 5)
	brokenHash := broken.TxHash()
	test.connect(broken)
	err := test.db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().Bucket(slpIndexKey).Put(brokenHash[:],
			[]byte{1})
	})
	if err != nil {
		t.Fatal(err)
	}
	test.disconnect(broken)
	if !strings.Contains(buf.String(), "[CRT] INDX: Failed to read the slp "+
		"entry of disconnected transaction "+brokenHash.String()) {

		t.Errorf("log %q, want the entry that could not be read", buf.String())
	}
}

// TestSlpDisconnectedWhileAsked ensures the disconnection of a block confirming
// a transaction whose memory pool entry the slp cache kept, as while graph
// search loads, keeps that entry, so that RemoveMempoolSlpTxsNotIn still
// removes it once the memory pool reports the transaction gone, even when the
// disconnection comes while the pool is asked.
func TestSlpDisconnectedWhileAsked(t *testing.T) {
	test := newSlpTokenIDTest(t)
	test.idx.config.SlpGraphSearchEnabled = true
	genesis := genesisTx(t, 1)
	hash := genesis.TxHash()
	test.addPotential(genesis)
	test.connect(genesis)
	disconnect := sync.OnceFunc(func() { test.disconnect(genesis) })
	test.idx.RemoveMempoolSlpTxsNotIn(func(*chainhash.Hash) bool {
		disconnect()
		return false
	})
	if _, ok := test.idx.cache.GetSlpTxEntry(&hash); ok {
		t.Error("the GENESIS that left the memory pool is still valid")
	}
}

// clearingTx is a database transaction whose buckets return copies of their
// values, which clear overwrites, as a database may reuse the buffers of the
// values it returned once the transaction has ended.
type clearingTx struct {
	database.Tx
	values [][]byte
}

func (tx *clearingTx) Metadata() database.Bucket {
	return &clearingBucket{wrappedBucket: tx.Tx.Metadata(), tx: tx}
}

// clear overwrites the values the transaction's buckets returned.
func (tx *clearingTx) clear() {
	for _, value := range tx.values {
		clear(value)
	}
}

// wrappedBucket names the bucket a clearingBucket wraps, whose Bucket method
// it replaces.
type wrappedBucket = database.Bucket

// clearingBucket is a bucket of a clearingTx.
type clearingBucket struct {
	wrappedBucket
	tx *clearingTx
}

func (b *clearingBucket) Bucket(key []byte) database.Bucket {
	bucket := b.wrappedBucket.Bucket(key)
	if bucket == nil {
		return nil
	}
	return &clearingBucket{wrappedBucket: bucket, tx: b.tx}
}

func (b *clearingBucket) Get(key []byte) []byte {
	value := b.wrappedBucket.Get(key)
	if value == nil {
		return nil
	}
	value = bytes.Clone(value)
	b.tx.values = append(b.tx.values, value)
	return value
}

// TestSlpEntryCopied ensures the slp entry the index reads from the database,
// which it caches and returns, holds a copy of the transaction's slp message,
// as a value the database returns is valid only during its transaction.
func TestSlpEntryCopied(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis := genesisTx(t, 1)
	test.connect(genesis)

	// The index is opened again, with an empty cache, so that it reads the
	// entry from the database.
	test.restart()
	hash := genesis.TxHash()
	var entry *SlpTxEntry
	err := test.db.View(func(dbTx database.Tx) error {
		tx := &clearingTx{Tx: dbTx}
		defer tx.clear()
		var err error
		entry, err = test.idx.GetSlpIndexEntry(tx, &hash)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := genesis.TxOut[0].PkScript
	if !bytes.Equal(entry.SlpOpReturn, want) {
		t.Errorf("the entry read holds %x, want %x", entry.SlpOpReturn, want)
	}
	cached, ok := test.idx.cache.GetSlpTxEntry(&hash)
	if !ok || !bytes.Equal(cached.SlpOpReturn, want) {
		t.Errorf("the entry cached holds %x, %v, want %x", cached.SlpOpReturn,
			ok, want)
	}
}

// TestSlpPotentialEntryIndexFailure ensures AddPotentialSlpEntries returns the
// error for the corrupt entry of a transaction an input spends, which
// IsSlpEntryCorrupt tells, whether the index cannot read the entry or parse
// its slp message, rather than take the input for one that is not slp, which
// could find the transaction invalid, and leaves a transaction valid without
// the input unjudged too, out of the cache, while an input of a transaction
// the index does not hold is still not slp.
func TestSlpPotentialEntryIndexFailure(t *testing.T) {
	check := func(what string, test *slpTokenIDTest, tx *wire.MsgTx, corrupt bool) {
		t.Helper()
		var valid bool
		err := test.db.View(func(dbTx database.Tx) error {
			var err error
			valid, err = test.idx.AddPotentialSlpEntries(dbTx, tx)
			return err
		})
		if IsSlpEntryCorrupt(err) != corrupt || valid || (err != nil) != corrupt {
			t.Errorf("%s: got %v and %v, want a corrupt entry %v", what, valid,
				err, corrupt)
		}
		hash := tx.TxHash()
		if _, ok := test.idx.cache.GetSlpTxEntry(&hash); ok {
			t.Errorf("%s: the transaction is in the cache", what)
		}
	}

	script, err := metadatamaker.TokenType1Send(bytes.Repeat([]byte{0x08}, 32),
		[]uint64{1000})
	send := slpTx(t, 0x22, script, err)
	send.TxIn[0].PreviousOutPoint = wire.OutPoint{Hash: chainhash.Hash{0xee, 0x21}, Index: 1}
	check("an input the index does not hold", newSlpTokenIDTest(t), send, false)

	for _, c := range slpEntryCorruptions {
		for _, validWithout := range []bool{false, true} {
			test := newSlpTokenIDTest(t)
			genesis, other := genesisTx(t, 1), genesisTx(t, 2)
			test.connect(genesis, other)
			corruptSlpEntry(t, test.db, genesis.TxHash(), c.corrupt)
			test.restart()
			what := c.name + ", spent by a SEND"
			spend := spendingSlpTx(t, genesis, true)
			if validWithout {
				what = c.name + ", spent by a SEND of another token, valid without it"
				spend = spendingSlpTx(t, other, true)
				spend.AddTxIn(&wire.TxIn{
					PreviousOutPoint: wire.OutPoint{Hash: genesis.TxHash(), Index: 1},
					Sequence:         wire.MaxTxInSequenceNum,
				})
			}
			check(what, test, spend, true)
		}
	}
}
