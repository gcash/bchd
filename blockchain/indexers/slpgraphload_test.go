// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/goslp/metadatamaker"
	"github.com/gcash/goslp/v1parser"
)

// graphSearchGenesis returns GENESIS n of a type 1 token, for n up to 65535.
func graphSearchGenesis(t *testing.T, n int) *wire.MsgTx {
	t.Helper()
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{
		Hash: chainhash.Hash{0xee, byte(n), byte(n >> 8)}}, nil))
	tx.AddTxOut(&wire.TxOut{PkScript: script})
	tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{0x51}})
	return tx
}

// graphSearchLoad is a load of slp graph search, from the memory pool entries
// of n GENESIS transactions, recording the batches it reads.
type graphSearchLoad struct {
	t       *testing.T
	test    *slpTokenIDTest
	txs     []*wire.MsgTx
	hashes  []chainhash.Hash
	raw     map[chainhash.Hash][]byte
	mtx     sync.Mutex
	batches [][]chainhash.Hash
}

func newGraphSearchLoad(t *testing.T, n int) *graphSearchLoad {
	t.Helper()
	load := &graphSearchLoad{t: t, test: newSlpTokenIDTest(t),
		raw: make(map[chainhash.Hash][]byte, n)}
	load.test.idx.config.SlpGraphSearchEnabled = true
	for i := 0; i < n; i++ {
		tx := graphSearchGenesis(t, i)
		hash := tx.TxHash()
		load.test.idx.cache.AddMempoolSlpTxEntry(&hash,
			SlpTxEntry{TokenIDHash: tokenIDOf(t, tx)})
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		load.raw[hash] = buf.Bytes()
		load.txs = append(load.txs, tx)
		load.hashes = append(load.hashes, hash)
	}
	return load
}

func (load *graphSearchLoad) fetch(txnHashes []chainhash.Hash) ([][]byte, error) {
	load.mtx.Lock()
	load.batches = append(load.batches, slices.Clone(txnHashes))
	load.mtx.Unlock()
	txns := make([][]byte, len(txnHashes))
	for i := range txnHashes {
		buf, ok := load.raw[txnHashes[i]]
		if !ok {
			return nil, errors.New("not found")
		}
		txns[i] = buf
	}
	return txns, nil
}

// run loads graph search, checks that it holds each transaction but those
// left out, and returns the sizes of the batches read whole, sorted, and the
// transactions read alone.
func (load *graphSearchLoad) run(left ...chainhash.Hash) ([]int, []chainhash.Hash) {
	t := load.t
	t.Helper()
	load.test.idx.LoadSlpGraphSearchDb(load.fetch, nil, nil)
	var sizes []int
	var singles []chainhash.Hash
	for _, batch := range load.batches {
		if len(batch) == 1 {
			singles = append(singles, batch[0])
			continue
		}
		sizes = append(sizes, len(batch))
	}
	slices.Sort(sizes)

	db := load.test.idx.graphSearchDb
	if !db.IsLoaded() {
		t.Fatal("graph search is not loaded")
	}
	for i, tx := range load.txs {
		token := tokenIDOf(t, tx)
		n, err := db.Count(context.Background(), &load.hashes[i], &token,
			&map[chainhash.Hash]struct{}{})
		if want := !slices.Contains(left, load.hashes[i]); (err == nil &&
			n == 1) != want {

			t.Errorf("GENESIS %d: counted %d and %v, want it loaded %v", i,
				n, err, want)
		}
	}
	return sizes, singles
}

// TestSlpGraphSearchLoadBatches ensures the load of slp graph search reads
// the transactions graphSearchLoadBatch at a time, and loads every transaction
// it can: one that cannot be read fails the read of its batch, which is then
// read a transaction at a time, and one that is not an slp transaction, or no
// transaction at all, is left out of its batch.
func TestSlpGraphSearchLoadBatches(t *testing.T) {
	const n = 2500

	// One GENESIS cannot be read, as one that has left the memory pool.
	// The log counts what the load reads.
	var logged bytes.Buffer
	logger := bchlog.NewBackend(&logged).Logger("INDX")
	logger.SetLevel(bchlog.LevelDebug)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)
	load := newGraphSearchLoad(t, n)
	missing := load.hashes[n/2]
	delete(load.raw, missing)
	sizes, singles := load.run(missing)
	for _, want := range []string{
		"fetching 2500 transactions for slp graph search db",
		"loading slp graph search transactions concurrently",
		"slp graph search finished fetching 2500 transactions",
	} {
		if strings.Count(logged.String(), want) != 1 {
			t.Errorf("log %q, want one line with %q", logged.String(), want)
		}
	}
	if want := []int{500, 1000, 1000}; !slices.Equal(sizes, want) {
		t.Errorf("read batches of %v, want %v", sizes, want)
	}
	var failed []chainhash.Hash
	for _, batch := range load.batches {
		if len(batch) > 1 && slices.Contains(batch, missing) {
			failed = batch
		}
	}
	if !slices.Equal(singles, failed) {
		t.Errorf("read %d transactions alone, want the %d of the batch "+
			"that failed", len(singles), len(failed))
	}

	// One is not an slp transaction, and another is not a transaction.
	load = newGraphSearchLoad(t, n)
	notSlp, garbled := load.hashes[n/3], load.hashes[2*n/3]
	var buf bytes.Buffer
	if err := notSlpTx().Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	load.raw[notSlp] = buf.Bytes()
	load.raw[garbled] = []byte{1, 2, 3}
	sizes, singles = load.run(notSlp, garbled)
	if want := []int{500, 1000, 1000}; !slices.Equal(sizes, want) ||
		singles != nil {

		t.Errorf("read batches of %v and %d transactions alone, want %v "+
			"and none", sizes, len(singles), want)
	}

	// A single GENESIS that cannot be read is read once, and not
	// concurrently.
	logged.Reset()
	load = newGraphSearchLoad(t, 1)
	delete(load.raw, load.hashes[0])
	load.run(load.hashes[0])
	if len(load.batches) != 1 {
		t.Errorf("read the transaction %d times, want once",
			len(load.batches))
	}
	if strings.Contains(logged.String(), "concurrently") {
		t.Errorf("log %q, loading one transaction concurrently",
			logged.String())
	}
}

// putSlpIndexEntries puts in the slp index of test n entries of transactions
// of tokens tokens, each with token metadata, as an index of n slp
// transactions holds, and returns the hashes of the transactions.
func putSlpIndexEntries(t *testing.T, test *slpTokenIDTest, n, tokens int) []chainhash.Hash {
	t.Helper()
	hashes := make([]chainhash.Hash, n)
	err := test.db.Update(func(dbTx database.Tx) error {
		for id := 1; id <= tokens; id++ {
			err := dbPutTokenIDIndexEntry(dbTx, uint32(id), &TokenMetadata{
				TokenID:    &chainhash.Hash{0xdd, byte(id), byte(id >> 8)},
				SlpVersion: v1parser.TokenTypeFungible01,
			})
			if err != nil {
				return err
			}
		}
		bucket := dbTx.Metadata().Bucket(slpIndexKey)
		for i := range hashes {
			hashes[i] = chainhash.Hash{0xcc, byte(i), byte(i >> 8), byte(i >> 16)}
			entry := make([]byte, 43)
			byteOrder.PutUint32(entry, uint32(1+i%tokens))
			byteOrder.PutUint16(entry[4:], uint16(v1parser.TokenTypeFungible01))
			if err := bucket.Put(hashes[i][:], entry); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hashes
}

// TestSlpGraphSearchLoadList ensures the load of slp graph search reads each
// transaction of the index and of the memory pool once, graphSearchLoadBatch
// at a time, and keeps no more of them than their hashes while it loads them,
// as an index can hold millions, which the graph holds besides.  It once kept
// a pointer to the token ID of each in a map, and read and allocated the
// token's metadata for each.
func TestSlpGraphSearchLoadList(t *testing.T) {
	const n = 20000
	test := newSlpTokenIDTest(t)
	hashes := putSlpIndexEntries(t, test, n, 100)

	// The memory pool entries of ten of the index's transactions, as the
	// index keeps those of the transactions a block confirms while graph
	// search loads, and of ten others.
	for i := 0; i < 20; i++ {
		hash := chainhash.Hash{0xbb, byte(i)}
		if i < 10 {
			hash = hashes[i*1000]
		} else {
			hashes = append(hashes, hash)
		}
		test.idx.cache.AddMempoolSlpTxEntry(&hash, SlpTxEntry{})
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	list, err := test.idx.buildGraphSearchTxnList()
	runtime.GC()
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	held := (int64(after.HeapAlloc) - int64(before.HeapAlloc)) / int64(len(hashes))
	t.Logf("the load keeps %d bytes for each transaction", held)
	if held > 40 {
		t.Errorf("the load keeps %d bytes for each of %d transactions", held,
			len(hashes))
	}

	var got []chainhash.Hash
	for i, batch := range list {
		if len(batch) != graphSearchLoadBatch && i != len(list)-1 {
			t.Errorf("batch %d of %d holds %d transactions", i, len(list),
				len(batch))
		}
		got = append(got, batch...)
	}
	slices.SortFunc(got, func(a, b chainhash.Hash) int { return a.Compare(&b) })
	slices.SortFunc(hashes, func(a, b chainhash.Hash) int { return a.Compare(&b) })
	if !slices.Equal(got, hashes) {
		t.Errorf("the load reads %d transactions, want the %d of the index "+
			"and the memory pool, each once", len(got), len(hashes))
	}

	// Without its bucket the index has nothing to load.
	err = test.db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().DeleteBucket(slpIndexKey)
	})
	if err != nil {
		t.Fatal(err)
	}
	if list, err := test.idx.buildGraphSearchTxnList(); err == nil || list != nil {
		t.Errorf("without the index's bucket: got %d batches and %v",
			len(list), err)
	}
}
