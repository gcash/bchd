// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

var errTestCommit = errors.New("injected commit failure")

// failingCommitDB wraps a database and makes its Update calls fail after
// running the callback, the way a failed commit looks to the caller, once
// failOn more calls have been made.  A negative failOn fails no calls.
type failingCommitDB struct {
	database.DB
	failOn int
}

func (d *failingCommitDB) Update(fn func(database.Tx) error) error {
	if d.failOn != 0 {
		d.failOn--
		return d.DB.Update(fn)
	}
	tx, err := d.DB.Begin(true)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	_ = tx.Rollback()
	return errTestCommit
}

// TestUtxoFlushFailureKeepsEntries ensures a utxo cache flush whose database
// commit fails keeps the entries it was flushing, so they are written by a
// later flush.
func TestUtxoFlushFailureKeepsEntries(t *testing.T) {
	chain, teardown, err := chainSetup(t.Name(), &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("failed to set up chain: %v", err)
	}
	defer teardown()

	// The first update marks the flush as ongoing and the second writes
	// the first batch of entries.
	db := &failingCommitDB{DB: chain.db, failOn: 1}
	cache := newUtxoCache(db, 1<<20)
	outpoint := wire.OutPoint{Hash: chainhash.Hash{1}}
	entry := NewUtxoEntry(wire.NewTxOut(5000, []byte{0x51},
		wire.TokenData{}), 100, false)
	if err := cache.addEntry(outpoint, entry, false); err != nil {
		t.Fatalf("addEntry: %v", err)
	}

	state := &BestState{Hash: chainhash.Hash{2}}
	if err := cache.flush(state); !errors.Is(err, errTestCommit) {
		t.Fatalf("flush: got %v, want the injected failure", err)
	}
	got, err := cache.FetchEntry(outpoint)
	if err != nil || got == nil || got.IsSpent() {
		t.Fatalf("entry lost after a failed flush: %v %v", got, err)
	}

	db.failOn = -1
	if err := cache.flush(state); err != nil {
		t.Fatalf("flush: %v", err)
	}
	cache = newUtxoCache(chain.db, 1<<20)
	got, err = cache.FetchEntry(outpoint)
	if err != nil || got == nil || got.Amount() != 5000 {
		t.Fatalf("entry not written by the next flush: %v %v", got, err)
	}
}

// TestUtxoFlushFailureRequestsShutdown ensures a failed utxo cache flush
// requests the process to shut down.
func TestUtxoFlushFailureRequestsShutdown(t *testing.T) {
	m := newRuleTestMiner(t, &chaincfg.RegressionNetParams)
	m.mustAccept(m.build(nil))

	select {
	case <-m.chain.RequestedProcessShutdown():
		t.Fatal("shutdown requested before any failure")
	default:
	}

	m.chain.utxoCache.db = &failingCommitDB{DB: m.chain.db}
	if err := m.chain.FlushCachedState(FlushRequired); err == nil {
		t.Fatal("flush succeeded with a failing database")
	}
	select {
	case <-m.chain.RequestedProcessShutdown():
	default:
		t.Fatal("no shutdown requested after a failed flush")
	}
}
