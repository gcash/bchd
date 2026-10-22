// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"github.com/gcash/goslp/v1parser"
)

// notSlpTx returns a transaction whose first output pays to a public key
// hash, as most transactions do, so it is not an slp transaction.
func notSlpTx() *wire.MsgTx {
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{9}}, nil))
	pkScript := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	pkScript = append(pkScript, 0x88, 0xac)
	tx.AddTxOut(wire.NewTxOut(1000, pkScript, wire.TokenData{}))
	return tx
}

// TestRemoveMempoolSlpTxItems ensures the mempool entries of the passed
// transactions are removed and others kept, using the hashes the transactions
// hold rather than hashing each one again.
func TestRemoveMempoolSlpTxItems(t *testing.T) {
	cache := InitSlpCache(10)
	tx := bchutil.NewTx(notSlpTx())
	other := chainhash.Hash{1}
	cache.AddMempoolSlpTxEntry(tx.Hash(), SlpTxEntry{})
	cache.AddMempoolSlpTxEntry(&other, SlpTxEntry{})

	txs := []*bchutil.Tx{tx}
	allocs := testing.AllocsPerRun(10, func() {
		cache.RemoveMempoolSlpTxItems(txs)
	})
	if cache.GetMempoolItem(tx.Hash()) != nil ||
		cache.GetMempoolItem(&other) == nil {

		t.Fatal("removed the wrong mempool entries")
	}
	if allocs != 0 {
		t.Fatalf("RemoveMempoolSlpTxItems made %v allocations", allocs)
	}
}

// TestCheckSlpTxNotSlp ensures CheckSlpTx finds a transaction that is not slp
// invalid, and allocates no more than parsing its first output does unless it
// logs at the debug level.  The slp index checks every transaction in every
// block, and most are not slp.
func TestCheckSlpTxNotSlp(t *testing.T) {
	tx := notSlpTx()
	getEntry := func(*chainhash.Hash) (*SlpTxEntry, error) { return nil, nil }
	putEntry := func(*wire.MsgTx, v1parser.ParseResult, *chainhash.Hash) error {
		return nil
	}
	isValid, burned, err := CheckSlpTx(tx, getEntry, putEntry)
	if isValid || burned != nil || err != nil {
		t.Fatalf("CheckSlpTx = %v, %v, %v, want false, nil, nil", isValid,
			burned, err)
	}

	parseAllocs := testing.AllocsPerRun(10, func() {
		_, _ = v1parser.ParseSLP(tx.TxOut[0].PkScript)
	})
	allocs := testing.AllocsPerRun(10, func() {
		_, _, _ = CheckSlpTx(tx, getEntry, putEntry)
	})
	if allocs > parseAllocs {
		t.Fatalf("CheckSlpTx made %v allocations, parsing its output %v",
			allocs, parseAllocs)
	}

	// At the debug level, the failed parse is logged with the transaction.
	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("INDX")
	logger.SetLevel(bchlog.LevelDebug)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)
	_, _, _ = CheckSlpTx(tx, getEntry, putEntry)
	if !strings.Contains(buf.String(), tx.TxHash().String()) {
		t.Fatalf("debug log %q does not name the transaction", buf.String())
	}
}

// TestSlpEntryNotFound ensures looking up a transaction the slp index has no
// entry for returns the error it always has, with at most one allocation
// more than reading the missing key takes.  Connecting a block looks up the
// inputs of every transaction in it, and most are not slp.
func TestSlpEntryNotFound(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	idx := NewSlpIndex(db, &SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatal(err)
	}

	hash := chainhash.Hash{7}
	want := fmt.Sprintf("slp entry does not exist %v", hash)
	err = db.View(func(dbTx database.Tx) error {
		_, err := dbFetchSlpIndexEntry(dbTx, &hash)
		if err == nil || err.Error() != want {
			t.Errorf("dbFetchSlpIndexEntry error %v, want %q", err, want)
		}
		_, err = idx.GetSlpIndexEntry(dbTx, &hash)
		if err == nil || err.Error() != want {
			t.Errorf("GetSlpIndexEntry error %v, want %q", err, want)
		}

		readAllocs := testing.AllocsPerRun(10, func() {
			_ = dbTx.Metadata().Bucket(slpIndexKey).Get(hash[:])
		})
		fetchAllocs := testing.AllocsPerRun(10, func() {
			_, _ = dbFetchSlpIndexEntry(dbTx, &hash)
		})
		if fetchAllocs > readAllocs+1 {
			t.Errorf("dbFetchSlpIndexEntry made %v allocations, reading "+
				"the key %v", fetchAllocs, readAllocs)
		}
		cacheAllocs := testing.AllocsPerRun(10, func() {
			_, _ = idx.cache.GetSlpTxEntry(&hash)
		})
		entryAllocs := testing.AllocsPerRun(10, func() {
			_, _ = idx.GetSlpIndexEntry(dbTx, &hash)
		})
		if entryAllocs > cacheAllocs+fetchAllocs {
			t.Errorf("GetSlpIndexEntry made %v allocations, the cache and "+
				"the database %v and %v", entryAllocs, cacheAllocs,
				fetchAllocs)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
