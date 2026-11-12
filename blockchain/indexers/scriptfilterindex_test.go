// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs/builder"
)

// scriptFilterTestBlock returns a block on prev: a coinbase paying payTo, and
// the given transactions.
func scriptFilterTestBlock(t *testing.T, prev chainhash.Hash, nonce uint32, payTo []byte,
	txs ...*wire.MsgTx) *bchutil.Block {

	t.Helper()
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{txscript.OP_DATA_4, byte(nonce), 0, 0, 0},
	})
	coinbase.AddTxOut(wire.NewTxOut(5000000000, payTo, wire.TokenData{}))
	msgBlock := wire.NewMsgBlock(&wire.BlockHeader{Version: 1, PrevBlock: prev, Nonce: nonce})
	msgBlock.Transactions = append([]*wire.MsgTx{coinbase}, txs...)
	merkles := blockchain.BuildMerkleTreeStore(bchutil.NewBlock(msgBlock).Transactions())
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	return bchutil.NewBlock(msgBlock)
}

func TestScriptFilterIndex(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	idx := NewScriptFilterIndex(db, &chaincfg.RegressionNetParams)
	if err := db.Update(idx.Create); err != nil {
		t.Fatal(err)
	}

	scriptA := append(append([]byte{0x76, 0xa9, 0x14}, bytes.Repeat([]byte{0x11}, 20)...), 0x88, 0xac)
	scriptB := append(append([]byte{0x76, 0xa9, 0x14}, bytes.Repeat([]byte{0x33}, 20)...), 0x88, 0xac)
	scriptD := append(append([]byte{0xa9, 0x14}, bytes.Repeat([]byte{0x44}, 20)...), 0x87)
	var category [32]byte
	for i := range category {
		category[i] = 0x22
	}
	token := wire.TokenData{CategoryID: category, Amount: 100, BitField: wire.HAS_AMOUNT}

	// Block 0 pays A. Block 1 spends a token coin locked by A, as the spend
	// journal keeps it (token prefix in front), and pays tokens to D.
	block0 := scriptFilterTestBlock(t, chainhash.Hash{}, 0, scriptA)
	spend := wire.NewMsgTx(2)
	spend.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{0x99}, 0), nil))
	spend.AddTxOut(wire.NewTxOut(1000, scriptD, token))
	block1 := scriptFilterTestBlock(t, *block0.Hash(), 1, scriptB, spend)
	prefix := token.TokenDataBuffer()
	stxos1 := []blockchain.SpentTxOut{{Amount: 2000, PkScript: append(prefix.Bytes(), scriptA...)}}

	connect := func(block *bchutil.Block, stxos []blockchain.SpentTxOut) error {
		return db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, block, stxos)
		})
	}
	if err := connect(block0, nil); err != nil {
		t.Fatal(err)
	}
	if err := connect(block1, stxos1); err != nil {
		t.Fatal(err)
	}

	// Each block's served filter, hash and header are what bchutil builds,
	// and the headers chain from zero.
	var prevHeader chainhash.Hash
	for _, c := range []struct {
		block   *bchutil.Block
		scripts [][]byte
	}{
		{block0, nil},
		{block1, [][]byte{stxos1[0].PkScript}},
	} {
		filter, root, err := builder.BuildScriptFilter(c.block.MsgBlock(), c.scripts)
		if err != nil {
			t.Fatal(err)
		}
		served, _ := builder.ScriptFilterBytes(filter, root)
		hash, _ := builder.GetScriptFilterHash(filter, root)
		header, _ := builder.MakeHeaderForScriptFilter(filter, root, prevHeader)
		prevHeader = header

		got, err := idx.FilterByBlockHash(c.block.Hash(), wire.GCSFilterScripts)
		if err != nil || !bytes.Equal(got, served) {
			t.Fatalf("block %v filter: %x, %v; want %x", c.block.Hash(), got, err, served)
		}
		gotHashes, err := idx.FilterHashesByBlockHashes([]*chainhash.Hash{c.block.Hash()}, wire.GCSFilterScripts)
		if err != nil || !bytes.Equal(gotHashes[0], hash[:]) {
			t.Fatalf("block %v filter hash: %x, %v; want %x", c.block.Hash(), gotHashes, err, hash)
		}
		gotHeader, err := idx.FilterHeaderByBlockHash(c.block.Hash(), wire.GCSFilterScripts)
		if err != nil || !bytes.Equal(gotHeader, header[:]) {
			t.Fatalf("block %v filter header: %x, %v; want %x", c.block.Hash(), gotHeader, err, header)
		}
	}

	// A wallet watching A finds block 1, which spends A's coin, and an app
	// following the category finds it too.
	served, _ := idx.FilterByBlockHash(block1.Hash(), wire.GCSFilterScripts)
	filter, _, err := builder.ParseScriptFilter(served)
	if err != nil {
		t.Fatal(err)
	}
	key := builder.DeriveKey(block1.Hash())
	keyA, keyC := builder.ScriptKey(scriptA), builder.CategoryKey(category)
	for _, item := range [][32]byte{keyA, keyC} {
		if match, err := filter.Match(key, item[:]); err != nil || !match {
			t.Fatalf("block 1's filter does not match %x: %v", item, err)
		}
	}

	// Only the script filter type is served from this index.
	if _, err := idx.FilterByBlockHash(block1.Hash(), wire.GCSFilterRegular); !errors.Is(err, errNotScriptFilter) {
		t.Fatalf("basic filter type: got %v", err)
	}

	// Disconnecting block 1 removes its entries and leaves block 0's.
	err = db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block1, stxos1)
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := idx.FiltersByBlockHashes([]*chainhash.Hash{block0.Hash(), block1.Hash()}, wire.GCSFilterScripts)
	if err != nil || len(entries[0]) == 0 || len(entries[1]) != 0 {
		t.Fatalf("after disconnecting block 1: %d and %d bytes, %v", len(entries[0]), len(entries[1]), err)
	}

	// A block whose spent outputs do not line up with its inputs is refused,
	// and so is one whose parent has no header in the index.
	if err := connect(block1, nil); !errors.Is(err, builder.ErrPrevOutScripts) {
		t.Fatalf("block 1 without its spent outputs: got %v", err)
	}
	orphan := scriptFilterTestBlock(t, chainhash.Hash{0x55}, 2, scriptB)
	if err := connect(orphan, nil); err == nil {
		t.Fatal("connected a block whose parent has no script filter header")
	}
}
