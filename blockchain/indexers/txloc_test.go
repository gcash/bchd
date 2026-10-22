// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestBlockTxLocs ensures blockTxLocs returns the transaction locations that
// block.TxLoc returns, for a block that holds its serialized bytes and for one
// that has to serialize itself, and that it does not decode the block, which
// takes allocations for every transaction.
func TestBlockTxLocs(t *testing.T) {
	msgBlock := &wire.MsgBlock{}
	for i := range 5 {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Index: uint32(i)},
			make([]byte, 10*i)))
		for j := range i + 1 {
			tx.AddTxOut(wire.NewTxOut(int64(j), make([]byte, 25+j),
				wire.TokenData{}))
		}
		if err := msgBlock.AddTransaction(tx); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := msgBlock.Serialize(&buf); err != nil {
		t.Fatal(err)
	}

	blocks := []*bchutil.Block{
		bchutil.NewBlockFromBlockAndBytes(msgBlock, buf.Bytes()),
		bchutil.NewBlock(msgBlock),
	}
	for i, block := range blocks {
		want, err := block.TxLoc()
		if err != nil {
			t.Fatalf("block %d: TxLoc: %v", i, err)
		}
		got, err := blockTxLocs(block)
		if err != nil {
			t.Fatalf("block %d: blockTxLocs: %v", i, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("block %d: blockTxLocs = %v, want %v", i, got, want)
		}
	}

	allocs := testing.AllocsPerRun(10, func() {
		_, _ = blockTxLocs(blocks[0])
	})
	if n := len(msgBlock.Transactions); allocs >= float64(n) {
		t.Fatalf("blockTxLocs made %v allocations for %d transactions",
			allocs, n)
	}
}
