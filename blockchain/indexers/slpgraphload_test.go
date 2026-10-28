// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp/metadatamaker"
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
	load := newGraphSearchLoad(t, n)
	missing := load.hashes[n/2]
	delete(load.raw, missing)
	sizes, singles := load.run(missing)
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

	// A single GENESIS that cannot be read is read once.
	load = newGraphSearchLoad(t, 1)
	delete(load.raw, load.hashes[0])
	load.run(load.hashes[0])
	if len(load.batches) != 1 {
		t.Errorf("read the transaction %d times, want once",
			len(load.batches))
	}
}
