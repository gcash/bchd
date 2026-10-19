// Copyright (c) 2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// BenchmarkIsCoinBase performs a simple benchmark against the IsCoinBase
// function.
func BenchmarkIsCoinBase(b *testing.B) {
	tx, _ := bchutil.NewBlock(&Block100000).Tx(1)

	for b.Loop() {
		IsCoinBase(tx)
	}
}

// BenchmarkIsCoinBaseTx performs a simple benchmark against the IsCoinBaseTx
// function.
func BenchmarkIsCoinBaseTx(b *testing.B) {
	tx := Block100000.Transactions[1]

	for b.Loop() {
		IsCoinBaseTx(tx)
	}
}

// BenchmarkValidateTransactionScripts validates 100 transactions that each
// spend a P2PKH output and a 2-of-3 P2SH multisig output with ECDSA
// signatures.
func BenchmarkValidateTransactionScripts(b *testing.B) {
	const (
		numTxs = 100
		amt    = 100000
		flags  = (txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens |
			txscript.ScriptAllowMay2025 | txscript.ScriptAllowMay2026) &^
			(txscript.ScriptDiscourageUpgradableNops | txscript.ScriptVerifyInputSigChecks)
		hashType = txscript.SigHashAll | txscript.SigHashForkID
	)

	var keys [3]*bchec.PrivateKey
	for i := range keys {
		keys[i], _ = bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{byte(i + 1)}, 32))
	}
	pubKey := keys[0].PubKey().SerializeCompressed()
	p2pkh, err := txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).
		AddOp(txscript.OP_HASH160).AddData(bchutil.Hash160(pubKey)).
		AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).Script()
	if err != nil {
		b.Fatal(err)
	}
	builder := txscript.NewScriptBuilder().AddOp(txscript.OP_2)
	for _, key := range keys {
		builder.AddData(key.PubKey().SerializeCompressed())
	}
	redeemScript, err := builder.AddOp(txscript.OP_3).
		AddOp(txscript.OP_CHECKMULTISIG).Script()
	if err != nil {
		b.Fatal(err)
	}
	p2sh, err := txscript.NewScriptBuilder().AddOp(txscript.OP_HASH160).
		AddData(bchutil.Hash160(redeemScript)).AddOp(txscript.OP_EQUAL).Script()
	if err != nil {
		b.Fatal(err)
	}

	fundTx := wire.NewMsgTx(wire.TxVersion)
	fundTx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Index: 0}, nil))
	for range numTxs {
		fundTx.AddTxOut(wire.NewTxOut(amt, p2pkh, wire.TokenData{}))
		fundTx.AddTxOut(wire.NewTxOut(amt, p2sh, wire.TokenData{}))
	}
	view := NewUtxoViewpoint()
	view.AddTxOuts(bchutil.NewTx(fundTx), 1)

	sign := func(tx *wire.MsgTx, idx int, script []byte, key *bchec.PrivateKey) []byte {
		sig, err := txscript.RawTxInECDSASignature(tx, idx, script, hashType, key, amt)
		if err != nil {
			b.Fatal(err)
		}
		return sig
	}
	fundHash := fundTx.TxHash()
	txs := make([]*bchutil.Tx, numTxs)
	for i := range txs {
		tx := wire.NewMsgTx(wire.TxVersion)
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&fundHash, uint32(2*i)), nil))
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&fundHash, uint32(2*i+1)), nil))
		tx.AddTxOut(wire.NewTxOut(2*amt-1000, p2pkh, wire.TokenData{}))
		tx.TxIn[0].SignatureScript, err = txscript.NewScriptBuilder().
			AddData(sign(tx, 0, p2pkh, keys[0])).AddData(pubKey).Script()
		if err != nil {
			b.Fatal(err)
		}
		tx.TxIn[1].SignatureScript, err = txscript.NewScriptBuilder().
			AddOp(txscript.OP_0).AddData(sign(tx, 1, redeemScript, keys[0])).
			AddData(sign(tx, 1, redeemScript, keys[1])).
			AddData(redeemScript).Script()
		if err != nil {
			b.Fatal(err)
		}
		txs[i] = bchutil.NewTx(tx)
	}

	run := func() error {
		for _, tx := range txs {
			if _, err := ValidateTransactionScripts(tx, view, flags, nil, nil, 0); err != nil {
				return err
			}
		}
		return nil
	}
	if err := run(); err != nil {
		b.Fatalf("scripts fail: %v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = run()
	}
}
