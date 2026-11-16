// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs/builder"
)

func TestRPAFilterIndex(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	idx := NewRPAFilterIndex(db, &chaincfg.RegressionNetParams)
	if err := db.Update(idx.Create); err != nil {
		t.Fatal(err)
	}

	payTo := []byte{0x51}

	// Block 0 holds a coinbase alone. Block 1 holds a transaction with 31
	// inputs, of which the first 30 are in its filter, and one with two.
	block0 := scriptFilterTestBlock(t, chainhash.Hash{}, 0, payTo)
	many := wire.NewMsgTx(2)
	for i := uint32(0); i < 31; i++ {
		many.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{0x55}, i), nil))
	}
	many.AddTxOut(wire.NewTxOut(1000, payTo, wire.TokenData{}))
	two := wire.NewMsgTx(2)
	two.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{0x66}, 0), []byte{0x51}))
	two.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{0x66}, 1), []byte{0x52}))
	two.AddTxOut(wire.NewTxOut(1000, payTo, wire.TokenData{}))
	block1 := scriptFilterTestBlock(t, *block0.Hash(), 1, payTo, many, two)

	connect := func(block *bchutil.Block) error {
		return db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, block, nil)
		})
	}
	for _, block := range []*bchutil.Block{block0, block1} {
		if err := connect(block); err != nil {
			t.Fatal(err)
		}
	}

	// Each block's filter, hash and header are what bchutil builds from the
	// block alone, and the headers chain from zero.
	var prevHeader chainhash.Hash
	for _, block := range []*bchutil.Block{block0, block1} {
		filter := builder.BuildRPAFilter(block.MsgBlock())
		hash := builder.GetRPAFilterHash(filter)
		header := builder.MakeHeaderForRPAFilter(filter, prevHeader)
		prevHeader = header

		got, err := idx.FilterByBlockHash(block.Hash(), wire.GCSFilterInputPrefix)
		if err != nil || !bytes.Equal(got, filter) {
			t.Fatalf("block %v filter: %x, %v; want %x", block.Hash(), got, err, filter)
		}
		gotHashes, err := idx.FilterHashesByBlockHashes([]*chainhash.Hash{block.Hash()}, wire.GCSFilterInputPrefix)
		if err != nil || !bytes.Equal(gotHashes[0], hash[:]) {
			t.Fatalf("block %v filter hash: %x, %v; want %x", block.Hash(), gotHashes, err, hash)
		}
		gotHeader, err := idx.FilterHeaderByBlockHash(block.Hash(), wire.GCSFilterInputPrefix)
		if err != nil || !bytes.Equal(gotHeader, header[:]) {
			t.Fatalf("block %v filter header: %x, %v; want %x", block.Hash(), gotHeader, err, header)
		}
	}

	// Block 0's filter has no values. Block 1's holds the values of the
	// first 30 inputs of its first transaction and both of the second's,
	// and the 31st input's value only if another input has it too.
	stored, _ := idx.FilterByBlockHash(block0.Hash(), wire.GCSFilterInputPrefix)
	if values, err := builder.DecodeRPAFilter(stored); err != nil || len(values) != 0 {
		t.Fatalf("block 0's filter: %x, %v", values, err)
	}
	stored, _ = idx.FilterByBlockHash(block1.Hash(), wire.GCSFilterInputPrefix)
	values, err := builder.DecodeRPAFilter(stored)
	if err != nil {
		t.Fatal(err)
	}
	want := builder.RPAValues([]*wire.MsgTx{many, two})
	if len(values) != len(want) || len(values) < 2 {
		t.Fatalf("block 1's filter holds %d values, want %d", len(values), len(want))
	}
	for i, txIn := range append(many.TxIn[:30:30], two.TxIn...) {
		v := builder.RPAInputValue(txIn)
		if !builder.RPAFilterMatch(values, v, 16) {
			t.Errorf("input %d's value %04x is not in block 1's filter", i, v)
		}
	}

	// Only the input prefix filter type is served from this index.
	if _, err := idx.FilterByBlockHash(block1.Hash(), wire.GCSFilterScripts); !errors.Is(err, errNotRPAFilter) {
		t.Fatalf("script filter type: got %v", err)
	}

	// Disconnecting block 1 removes its entries and leaves block 0's.
	err = db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block1, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := idx.FiltersByBlockHashes([]*chainhash.Hash{block0.Hash(), block1.Hash()}, wire.GCSFilterInputPrefix)
	if err != nil || len(entries[0]) == 0 || len(entries[1]) != 0 {
		t.Fatalf("after disconnecting block 1: %d and %d bytes, %v", len(entries[0]), len(entries[1]), err)
	}

	// A block whose parent has no header in the index is refused.
	orphan := scriptFilterTestBlock(t, chainhash.Hash{0x55}, 2, payTo)
	if err := connect(orphan); err == nil {
		t.Fatal("connected a block whose parent has no input prefix filter header")
	}
}
