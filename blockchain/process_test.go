// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// procTestBlock builds a solved block on top of prev at height prevHeight+1.
// The transactions are kept in the given order and the coinbase pays
// cbValue.
func procTestBlock(t *testing.T, prev *wire.MsgBlock, prevHeight int32,
	cbValue int64, bits uint32, txs []*wire.MsgTx) *bchutil.Block {

	t.Helper()
	height := prevHeight + 1
	cbScript, err := txscript.NewScriptBuilder().AddInt64(int64(height)).
		AddInt64(int64(prev.Header.Nonce)).Script()
	if err != nil {
		t.Fatal(err)
	}
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		Sequence:         wire.MaxTxInSequenceNum,
		SignatureScript:  cbScript,
	})
	cb.AddTxOut(wire.NewTxOut(cbValue, opTrueScript, wire.TokenData{}))
	cb.AddTxOut(wire.NewTxOut(0, procTestPad(nil), wire.TokenData{}))

	all := append([]*wire.MsgTx{cb}, txs...)
	utxs := make([]*bchutil.Tx, 0, len(all))
	for _, tx := range all {
		utxs = append(utxs, bchutil.NewTx(tx))
	}
	merkles := BuildMerkleTreeStore(utxs)
	ts := prev.Header.Timestamp.Add(time.Minute)
	if ts.Before(time.Unix(1700000000, 0)) {
		ts = time.Unix(1700000000, 0)
	}
	msgBlock := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    4,
			PrevBlock:  prev.BlockHash(),
			MerkleRoot: *merkles[len(merkles)-1],
			Timestamp:  ts,
			Bits:       bits,
		},
		Transactions: all,
	}
	if !solveBlock(&msgBlock.Header) {
		t.Fatal("unable to solve block")
	}
	return bchutil.NewBlock(msgBlock)
}

// procTestPad returns an OP_RETURN script pushing data padded to 60 bytes.
func procTestPad(data []byte) []byte {
	pad := make([]byte, 60)
	copy(pad, data)
	return append([]byte{txscript.OP_RETURN, txscript.OP_DATA_60}, pad...)
}

// procTestSpend returns a transaction spending prev into an OP_TRUE output
// padded with an OP_RETURN output tagged with tag.
func procTestSpend(prev wire.OutPoint, value int64, tag byte) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: prev, Sequence: wire.MaxTxInSequenceNum})
	tx.AddTxOut(wire.NewTxOut(value, opTrueScript, wire.TokenData{}))
	tx.AddTxOut(wire.NewTxOut(0, procTestPad([]byte{tag}), wire.TokenData{}))
	return tx
}

// TestOrphanContextChecks ensures a block that arrives before its parent is
// held to the same height-dependent rules as one that arrives in order.
func TestOrphanContextChecks(t *testing.T) {
	unordered := func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx {
		a, b := procTestSpend(o1, v, 1), procTestSpend(o2, v, 2)
		ha, hb := a.TxHash(), b.TxHash()
		if ha.Compare(&hb) < 0 {
			return []*wire.MsgTx{b, a}
		}
		return []*wire.MsgTx{a, b}
	}
	ordered := func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx {
		txs := unordered(o1, o2, v)
		return []*wire.MsgTx{txs[1], txs[0]}
	}
	undersized := func(o1, _ wire.OutPoint, v int64) []*wire.MsgTx {
		tx := wire.NewMsgTx(2)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: o1, Sequence: wire.MaxTxInSequenceNum})
		tx.AddTxOut(wire.NewTxOut(v, opTrueScript, wire.TokenData{}))
		return []*wire.MsgTx{tx}
	}

	tests := []struct {
		name       string
		forkHeight int32
		txs        func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx
		wantErr    ErrorCode
		valid      bool
	}{
		{name: "not in CTOR order", txs: unordered, wantErr: ErrInvalidTxOrder},
		{name: "not in CTOR order, first block after activation",
			forkHeight: 2, txs: unordered, wantErr: ErrInvalidTxOrder},
		{name: "undersized tx", txs: undersized, wantErr: ErrTxTooSmall},
		{name: "undersized tx, first block after activation",
			forkHeight: 2, txs: undersized, wantErr: ErrTxTooSmall},
		{name: "in CTOR order", txs: ordered, valid: true},
		{name: "not in CTOR order before activation", forkHeight: 3,
			txs: unordered, valid: true},
		{name: "undersized tx before activation", forkHeight: 3,
			txs: undersized, valid: true},
	}

	for _, test := range tests {
		for _, asOrphan := range []bool{false, true} {
			name := fmt.Sprintf("%s/orphan=%v", test.name, asOrphan)
			t.Run(name, func(t *testing.T) {
				testOrphanContext(t, test.forkHeight, test.txs,
					asOrphan, test.valid, test.wantErr)
			})
		}
	}
}

// testOrphanContext connects a block spending two coinbases with the given
// transactions at height 3, before or after its parent.
func testOrphanContext(t *testing.T, forkHeight int32,
	txs func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx, asOrphan,
	valid bool, wantErr ErrorCode) {

	params := chaincfg.RegressionNetParams
	params.MagneticAnonomalyForkHeight = forkHeight
	params.Upgrade9ForkHeight = forkHeight
	chain, teardown, err := chainSetup("orphanctx", &params)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	chain.TstSetCoinbaseMaturity(1)

	bits := params.PowLimitBits
	subsidy := CalcBlockSubsidy(1, &params)
	b1 := procTestBlock(t, params.GenesisBlock, 0, subsidy, bits, nil)
	if _, _, err := chain.ProcessBlock(b1, BFNone); err != nil {
		t.Fatalf("block 1: %v", err)
	}
	b2 := procTestBlock(t, b1.MsgBlock(), 1, subsidy, bits, nil)
	o1 := wire.OutPoint{Hash: b1.MsgBlock().Transactions[0].TxHash()}
	o2 := wire.OutPoint{Hash: b2.MsgBlock().Transactions[0].TxHash()}
	b3 := procTestBlock(t, b2.MsgBlock(), 2, subsidy, bits,
		txs(o1, o2, subsidy-1000))

	first, second := b2, b3
	if asOrphan {
		first, second = b3, b2
	}
	_, isOrphan, err := chain.ProcessBlock(first, BFNone)
	if err != nil || isOrphan != asOrphan {
		t.Fatalf("first block: orphan=%v err=%v", isOrphan, err)
	}
	_, _, err = chain.ProcessBlock(second, BFNone)
	wantHeight := int32(3)
	if valid {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	} else {
		var rerr RuleError
		if !errors.As(err, &rerr) || rerr.ErrorCode != wantErr {
			t.Fatalf("got error %v, want %v", err, wantErr)
		}
		wantHeight = 2
	}
	if h := chain.BestSnapshot().Height; h != wantHeight {
		t.Fatalf("tip height %d, want %d", h, wantHeight)
	}
}

// TestOrphanNoFastAddInheritance ensures an orphan does not inherit BFFastAdd
// from the parent that made it connectable.
func TestOrphanNoFastAddInheritance(t *testing.T) {
	params := chaincfg.RegressionNetParams
	chain, teardown, err := chainSetup("orphanfastadd", &params)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	subsidy := CalcBlockSubsidy(1, &params)
	parent := procTestBlock(t, params.GenesisBlock, 0, subsidy,
		params.PowLimitBits, nil)

	// The child pays too much in its coinbase and uses the wrong
	// difficulty, which are only checked without BFFastAdd.
	child := procTestBlock(t, parent.MsgBlock(), 1, subsidy*1000,
		0x207ffffe, nil)

	_, isOrphan, err := chain.ProcessBlock(child, BFNone)
	if err != nil || !isOrphan {
		t.Fatalf("child not held as orphan: orphan=%v err=%v", isOrphan, err)
	}
	_, _, err = chain.ProcessBlock(parent, BFFastAdd)
	var rerr RuleError
	if !errors.As(err, &rerr) {
		t.Fatalf("child not rejected: %v", err)
	}
	best := chain.BestSnapshot()
	if best.Hash != *parent.Hash() {
		t.Fatalf("tip is %v at height %d, want the parent %v",
			best.Hash, best.Height, parent.Hash())
	}
	if chain.index.LookupNode(child.Hash()) != nil {
		t.Fatal("invalid orphan was added to the block index")
	}
}
