// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"reflect"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// addInputUtxosPerInput is a reference for addInputUtxos that adds the
// outputs of an in-block parent for every input spending it when the order is
// ignored, and leaves them out otherwise.
func addInputUtxosPerInput(view *UtxoViewpoint, source utxoView, block *bchutil.Block, ignoreOutOfOrder bool) {
	txInFlight := make(map[chainhash.Hash]int)
	transactions := block.Transactions()
	for i, tx := range transactions {
		txInFlight[*tx.Hash()] = i
	}
	for _, tx := range transactions[1:] {
		for _, txIn := range tx.MsgTx().TxIn {
			if idx, ok := txInFlight[txIn.PreviousOutPoint.Hash]; ok {
				if ignoreOutOfOrder {
					view.AddTxOuts(transactions[idx], block.Height())
				}
				continue
			}
			if _, ok := view.entries[txIn.PreviousOutPoint]; ok {
				continue
			}
			entry, err := source.getEntry(txIn.PreviousOutPoint)
			if err == nil && entry != nil {
				view.entries[txIn.PreviousOutPoint] = entry.Clone()
			}
		}
	}
}

// inBlockParentsBlock returns a block with a parent of n outputs, one of them
// unspendable, followed by n children that each spend one of them and an
// output from outside the block. The first child is spent by a later
// transaction, and a transaction placed before the parent spends it too.
func inBlockParentsBlock(n int) (*bchutil.Block, wire.OutPoint) {
	outside := wire.OutPoint{Hash: chainhash.Hash{0x01}, Index: 7}

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(wire.NewTxIn(&wire.OutPoint{Index: wire.MaxPrevOutIndex}, nil))
	coinbase.AddTxOut(wire.NewTxOut(50, []byte{0x51}, wire.TokenData{}))

	parent := wire.NewMsgTx(1)
	parent.AddTxIn(wire.NewTxIn(&outside, nil))
	for i := range n {
		script := []byte{0x76, 0xa9, byte(i), byte(i >> 8)}
		if i == 1 {
			script = []byte{0x6a, 0x01}
		}
		parent.AddTxOut(wire.NewTxOut(int64(1000+i), script, wire.TokenData{}))
	}
	parentHash := parent.TxHash()

	early := wire.NewMsgTx(1)
	early.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&parentHash, 0), nil))
	early.AddTxOut(wire.NewTxOut(1, []byte{0x51}, wire.TokenData{}))

	msg := &wire.MsgBlock{}
	msg.Transactions = append(msg.Transactions, coinbase, early, parent)
	for i := range n {
		child := wire.NewMsgTx(1)
		child.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&parentHash, uint32(i)), nil))
		child.AddTxIn(wire.NewTxIn(&outside, nil))
		child.AddTxOut(wire.NewTxOut(1, []byte{0x51, byte(i)}, wire.TokenData{}))
		child.AddTxOut(wire.NewTxOut(2, []byte{0x52, byte(i)}, wire.TokenData{}))
		msg.Transactions = append(msg.Transactions, child)
	}

	firstChild := msg.Transactions[3].TxHash()
	for i := range 2 {
		grandchild := wire.NewMsgTx(1)
		grandchild.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&firstChild, uint32(i)), nil))
		grandchild.AddTxOut(wire.NewTxOut(1, []byte{0x53, byte(i)}, wire.TokenData{}))
		msg.Transactions = append(msg.Transactions, grandchild)
	}

	block := bchutil.NewBlock(msg)
	block.SetHeight(100)
	return block, outside
}

// TestAddInputUtxosInBlockParents checks that addInputUtxos gives the same
// view as adding an in-block parent's outputs for every input spending it when
// the order is ignored, and leaves the outputs of the block's transactions out
// otherwise.
func TestAddInputUtxosInBlockParents(t *testing.T) {
	block, outside := inBlockParentsBlock(50)

	source := NewUtxoViewpoint()
	source.entries[outside] = &UtxoEntry{amount: 9, pkScript: []byte{0x51}, blockHeight: 3}
	parentHash := block.Transactions()[2].Hash()
	// An existing entry for a parent output is overwritten.
	stale := wire.OutPoint{Hash: *parentHash, Index: 3}

	for _, ignoreOutOfOrder := range []bool{false, true} {
		got := NewUtxoViewpoint()
		got.entries[stale] = &UtxoEntry{amount: 1, blockHeight: 1, packedFlags: tfSpent}
		if err := got.addInputUtxos(source, block, ignoreOutOfOrder); err != nil {
			t.Fatal(err)
		}
		want := NewUtxoViewpoint()
		want.entries[stale] = &UtxoEntry{amount: 1, blockHeight: 1, packedFlags: tfSpent}
		addInputUtxosPerInput(want, source, block, ignoreOutOfOrder)

		if !reflect.DeepEqual(got.entries, want.entries) {
			t.Fatalf("ignoreOutOfOrder=%v: view differs from reference", ignoreOutOfOrder)
		}
		firstChild := wire.OutPoint{Hash: *block.Transactions()[3].Hash(), Index: 1}
		if !ignoreOutOfOrder {
			// The caller adds them as it connects each transaction.
			if e := got.entries[stale]; !e.IsSpent() || e.amount != 1 {
				t.Fatalf("existing entry modified: %+v", e)
			}
			if got.entries[firstChild] != nil {
				t.Fatal("output of a transaction in the block added")
			}
			continue
		}
		if got.entries[wire.OutPoint{Hash: *parentHash, Index: 1}] != nil {
			t.Fatal("unspendable output added")
		}
		if e := got.entries[stale]; e.IsSpent() || e.amount != 1003 {
			t.Fatalf("existing entry not overwritten: %+v", e)
		}
		if e := got.entries[firstChild]; e == nil || e.amount != 2 {
			t.Fatalf("output of spent child not added: %+v", e)
		}
	}
}

// BenchmarkAddInputUtxosInBlockParent measures a parent with 2000 outputs
// spent by 2000 children in the same block.
func BenchmarkAddInputUtxosInBlockParent(b *testing.B) {
	block, outside := inBlockParentsBlock(2000)
	source := NewUtxoViewpoint()
	source.entries[outside] = &UtxoEntry{amount: 9, pkScript: []byte{0x51}}
	b.ReportAllocs()
	for b.Loop() {
		view := NewUtxoViewpoint()
		if err := view.addInputUtxos(source, block, true); err != nil {
			b.Fatal(err)
		}
	}
}
