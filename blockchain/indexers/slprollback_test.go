// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain/slpgraphsearch"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestSlpConnectFailureLeavesNothing ensures a block whose connection to the
// slp index fails partway through, on a transaction after the others,
// changes nothing in the index's memory, as it changes nothing in the
// database: the next internal token ID, the cache's entries of the
// transactions the block stores and of those it reads, the memory pool's
// entries of its transactions, the token metadata a MINT of the memory pool
// set, and graph search.  The same block without the failing transaction then
// makes all those changes.
func TestSlpConnectFailureLeavesNothing(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis, other, corrupt := batonGenesis(t, 1), genesisTx(t, 2), genesisTx(t, 3)
	test.connect(genesis, other, corrupt)
	corruptSlpEntry(t, test.db, corrupt.TxHash(), slpEntryCorruptions[0].corrupt)
	token := tokenIDOf(t, genesis)

	// The memory pool holds a MINT of the GENESIS's token, which puts the
	// mint baton on its output 2, and a new GENESIS.  Checking the MINT
	// caches the entry of the GENESIS it spends.
	test.restart()
	test.idx.config.SlpGraphSearchEnabled = true
	gsDb := slpgraphsearch.NewDb(nil)
	test.idx.graphSearchDb.Store(gsDb)
	if err := gsDb.SetLoaded(); err != nil {
		t.Fatal(err)
	}
	inGraphSearch := func(tx *wire.MsgTx) bool {
		hash, token := tx.TxHash(), tokenIDOf(t, tx)
		_, err := gsDb.Count(context.Background(), &hash, &token,
			&map[chainhash.Hash]struct{}{})
		return err == nil
	}
	mint := batonMint(t, token, wire.OutPoint{Hash: genesis.TxHash(), Index: 2})
	newGenesis := genesisTx(t, 4)
	test.addPotential(mint)
	test.addPotential(newGenesis)

	// The block confirms them, a SEND of the other token, which reads the
	// other GENESIS's entry, and a GENESIS that only it holds, and lastly a
	// SEND spending an output of each of them and one whose entry is
	// corrupt, which fails the block.
	send := spendingSlpTx(t, other, true)
	extra := genesisTx(t, 5)
	failing := spendingSlpTx(t, corrupt, true)
	for _, tx := range []*wire.MsgTx{mint, newGenesis, send, extra} {
		failing.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: tx.TxHash(), Index: 1}, nil))
	}
	connect := func(txs ...*wire.MsgTx) error {
		block := bchutil.NewBlock(&wire.MsgBlock{Transactions: txs})
		return test.db.Update(func(dbTx database.Tx) error {
			return test.idx.ConnectBlock(dbTx, block, nil)
		})
	}
	checkSlpEntryCorrupt(t, "the block", connect(mint, newGenesis, send, extra,
		failing), corrupt.TxHash())

	mintHash, newHash, sendHash, otherHash := mint.TxHash(), newGenesis.TxHash(),
		send.TxHash(), other.TxHash()
	cached := func(hash *chainhash.Hash) bool {
		_, ok := test.idx.cache.GetSlpTxEntry(hash)
		return ok
	}
	if test.idx.curTokenID != 3 {
		t.Errorf("after the failed block, the highest internal token ID is %d, "+
			"want 3", test.idx.curTokenID)
	}
	if cached(&sendHash) || cached(&otherHash) {
		t.Errorf("after the failed block, the cache holds the entry of the SEND "+
			"%v or of the GENESIS it read %v", cached(&sendHash), cached(&otherHash))
	}
	if test.idx.cache.GetMempoolItem(&mintHash) == nil ||
		test.idx.cache.GetMempoolItem(&newHash) == nil {
		t.Error("after the failed block, the memory pool's entries are gone")
	}
	if tm, ok := test.idx.cache.GetTokenMetadata(&token, nil); !ok ||
		tm.MintBatonHash == nil || *tm.MintBatonHash != mintHash {
		t.Errorf("after the failed block, the token metadata the MINT set is "+
			"gone: %+v, %v", tm, ok)
	}

	if err := connect(mint, newGenesis, send); err != nil {
		t.Fatalf("the block without the failing SEND: %v", err)
	}
	if test.idx.curTokenID != 4 {
		t.Errorf("after the block, the highest internal token ID is %d, want 4",
			test.idx.curTokenID)
	}
	if !cached(&sendHash) || !cached(&otherHash) {
		t.Errorf("after the block, the cache lacks the entry of the SEND %v or "+
			"of the GENESIS it read %v", !cached(&sendHash), !cached(&otherHash))
	}
	if entry, _ := test.idx.cache.GetSlpTxEntry(&newHash); entry.TokenID != 4 ||
		test.idx.cache.GetMempoolItem(&mintHash) != nil {
		t.Errorf("after the block, the cache holds the new GENESIS as %+v, or "+
			"the MINT's memory pool entry", entry)
	}
	if _, ok := test.idx.cache.GetTokenMetadata(&token, nil); ok {
		t.Error("after the block, the token metadata the MINT set is cached")
	}
	if baton := test.baton(token); baton == nil || *baton != (wire.OutPoint{
		Hash: mintHash, Index: 2}) {
		t.Errorf("after the block, the mint baton is at %v, want the MINT's "+
			"output 2", baton)
	}

	// Graph search gets the block's transactions, which it adds in the
	// background, but not the GENESIS only the failed block held, which it
	// would have been given first.
	deadline := time.Now().Add(5 * time.Second)
	for !inGraphSearch(mint) || !inGraphSearch(newGenesis) || !inGraphSearch(send) {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the block's transactions in graph search")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if inGraphSearch(extra) {
		t.Error("graph search holds the GENESIS of the failed block")
	}
}

// failingDeleteTx is a database transaction whose buckets fail to delete.
type failingDeleteTx struct {
	database.Tx
}

func (tx *failingDeleteTx) Metadata() database.Bucket {
	return &failingDeleteBucket{wrappedBucket: tx.Tx.Metadata()}
}

// failingDeleteBucket is a bucket of a failingDeleteTx.
type failingDeleteBucket struct {
	wrappedBucket
}

func (b *failingDeleteBucket) Bucket(key []byte) database.Bucket {
	bucket := b.wrappedBucket.Bucket(key)
	if bucket == nil {
		return nil
	}
	return &failingDeleteBucket{wrappedBucket: bucket}
}

func (b *failingDeleteBucket) Delete([]byte) error {
	return errors.New("the delete failed")
}

// TestSlpDisconnectFailureLeavesNothing ensures a block whose disconnection
// from the slp index fails leaves the cache as it was, rather than holding the
// entries of the block's transactions with the memory pool's, as a reorg that
// puts them back in the memory pool has them, and that a disconnection that
// succeeds puts them there.
func TestSlpDisconnectFailureLeavesNothing(t *testing.T) {
	test := newSlpTokenIDTest(t)
	genesis := genesisTx(t, 1)
	test.connect(genesis)
	test.restart()
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{genesis}})
	err := test.db.Update(func(dbTx database.Tx) error {
		return test.idx.DisconnectBlock(&failingDeleteTx{Tx: dbTx}, block, nil)
	})
	if err == nil {
		t.Fatal("the disconnection did not fail")
	}
	hash := genesis.TxHash()
	if test.idx.cache.GetMempoolItem(&hash) != nil {
		t.Error("after the failed disconnection, the cache holds the GENESIS " +
			"with the memory pool's entries")
	}
	test.disconnect(genesis)
	if test.idx.cache.GetMempoolItem(&hash) == nil {
		t.Error("after the disconnection, the cache lacks the GENESIS")
	}
}
