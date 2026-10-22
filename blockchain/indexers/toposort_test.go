// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// sortTxsByPasses is how TopologicallySortTxs used to sort: it passes over the
// list until every transaction is taken, each pass taking, in list order, the
// transactions with no parent in the list left to take.
func sortTxsByPasses(transactions []*bchutil.Tx) []*wire.MsgTx {
	sorted := make([]*wire.MsgTx, 0, len(transactions))
	txids := make(map[chainhash.Hash]struct{}, len(transactions))
	outpoints := make(map[wire.OutPoint]struct{})
	for _, tx := range transactions {
		for i := range tx.MsgTx().TxOut {
			outpoints[wire.OutPoint{Hash: *tx.Hash(), Index: uint32(i)}] = struct{}{}
		}
	}
	for len(sorted) < len(transactions) {
		for _, tx := range transactions {
			if _, ok := txids[*tx.Hash()]; ok {
				continue
			}
			foundParent := false
			for _, in := range tx.MsgTx().TxIn {
				if _, ok := outpoints[in.PreviousOutPoint]; ok {
					foundParent = true
					break
				}
			}
			if !foundParent {
				sorted = append(sorted, tx.MsgTx())
				for i := range tx.MsgTx().TxOut {
					delete(outpoints, wire.OutPoint{Hash: *tx.Hash(), Index: uint32(i)})
				}
				txids[*tx.Hash()] = struct{}{}
			}
		}
	}
	return sorted
}

// randTxList returns n transactions in random order that spend each other's
// outputs: in chains, with several parents, with several inputs from one
// parent, with inputs from transactions that are not in the list, and with
// inputs for outputs that their parent in the list does not have.
func randTxList(r *rand.Rand, n int) []*bchutil.Tx {
	txs := make([]*bchutil.Tx, 0, n)
	for i := range n {
		tx := wire.NewMsgTx(1)
		tx.LockTime = uint32(i)
		for k := r.Intn(4); k > 0 && len(txs) > 0; k-- {
			parent := txs[len(txs)-1]
			if r.Intn(2) == 0 {
				parent = txs[r.Intn(len(txs))]
			}
			index := uint32(r.Intn(len(parent.MsgTx().TxOut) + 1))
			tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(parent.Hash(), index), nil))
		}
		if r.Intn(2) == 0 {
			var hash chainhash.Hash
			r.Read(hash[:])
			tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&hash, 0), nil))
		}
		for k := 1 + r.Intn(3); k > 0; k-- {
			tx.AddTxOut(wire.NewTxOut(1, []byte{0x51}, wire.TokenData{}))
		}
		txs = append(txs, bchutil.NewTx(tx))
	}
	r.Shuffle(len(txs), func(i, j int) { txs[i], txs[j] = txs[j], txs[i] })
	return txs
}

// TestTopologicallySortTxs ensures TopologicallySortTxs sorts random lists of
// transactions into the order that passing over the list gives.
func TestTopologicallySortTxs(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(1))
	for i := range 500 {
		txs := randTxList(r, r.Intn(60))
		want := sortTxsByPasses(txs)
		if got := TopologicallySortTxs(txs); !slices.Equal(got, want) {
			t.Fatalf("list %d of %d transactions sorted out of order", i,
				len(txs))
		}
	}
}

// TestTopologicallySortTxsLongChain ensures a long chain of transactions that
// each spend the two before, in random order, is sorted into chain order in
// about linear time.  Passing over the list once for each link of the chain
// takes several seconds here.
func TestTopologicallySortTxsLongChain(t *testing.T) {
	chain := make([]*bchutil.Tx, 20000)
	prev, prev2 := chainhash.Hash{1}, chainhash.Hash{2}
	for i := range chain {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prev, 0), nil))
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prev2, 1), nil))
		tx.AddTxOut(wire.NewTxOut(1, []byte{0x51}, wire.TokenData{}))
		tx.AddTxOut(wire.NewTxOut(1, []byte{0x51}, wire.TokenData{}))
		chain[i] = bchutil.NewTx(tx)
		prev, prev2 = *chain[i].Hash(), prev
	}
	txs := slices.Clone(chain)
	r := rand.New(rand.NewSource(2))
	r.Shuffle(len(txs), func(i, j int) { txs[i], txs[j] = txs[j], txs[i] })

	start := time.Now()
	sorted := TopologicallySortTxs(txs)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("sorting a chain of %d transactions took %v", len(txs),
			elapsed)
	}
	for i, tx := range sorted {
		if tx != chain[i].MsgTx() {
			t.Fatalf("transaction %d is out of chain order", i)
		}
	}
}
