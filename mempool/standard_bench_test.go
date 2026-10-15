// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// BenchmarkStandardness benchmarks the standardness checks the mempool runs
// on a transaction and the outputs it spends: one with P2PKH and P2SH
// multisig inputs paying to the standard output types, and one spending a
// P2SH output with a redeem script of about 9 KB.
func BenchmarkStandardness(b *testing.B) {
	mustScript := func(builder *txscript.ScriptBuilder) []byte {
		script, err := builder.Script()
		if err != nil {
			b.Fatal(err)
		}
		return script
	}
	pubKey := func(seed byte) []byte {
		_, pub := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{seed}, 32))
		return pub.SerializeCompressed()
	}
	p2sh := func(redeem []byte) []byte {
		return mustScript(txscript.NewScriptBuilder().AddOp(txscript.OP_HASH160).
			AddData(bchutil.Hash160(redeem)).AddOp(txscript.OP_EQUAL))
	}
	sig := append(bytes.Repeat([]byte{0x30}, 71), 0x41)
	hash20 := bytes.Repeat([]byte{0x11}, 20)
	hash32 := bytes.Repeat([]byte{0x22}, 32)

	p2pkh := mustScript(txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).
		AddOp(txscript.OP_HASH160).AddData(hash20).
		AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG))
	multisig := mustScript(txscript.NewScriptBuilder().AddOp(txscript.OP_2).
		AddData(pubKey(1)).AddData(pubKey(2)).AddData(pubKey(3)).
		AddOp(txscript.OP_3).AddOp(txscript.OP_CHECKMULTISIG))
	redeem := txscript.NewScriptBuilder()
	for range 68 {
		redeem.AddData(bytes.Repeat([]byte{0x33}, 128)).
			AddOp(txscript.OP_SHA256).AddOp(txscript.OP_DROP)
	}
	largeRedeem := mustScript(redeem.AddOp(txscript.OP_1))

	outputs := []*wire.TxOut{
		{Value: 10000, PkScript: p2pkh},
		{Value: 10000, PkScript: p2sh(multisig)},
		{Value: 10000, PkScript: mustScript(txscript.NewScriptBuilder().
			AddOp(txscript.OP_HASH256).AddData(hash32).AddOp(txscript.OP_EQUAL))},
		{Value: 0, PkScript: mustScript(txscript.NewScriptBuilder().
			AddOp(txscript.OP_RETURN).AddData(bytes.Repeat([]byte{0x44}, 40)))},
	}
	type spend struct {
		sigScript, pkScript []byte
	}
	benches := []struct {
		name   string
		spends []spend
	}{
		{"p2pkh and p2sh multisig", []spend{
			{mustScript(txscript.NewScriptBuilder().AddData(sig).AddData(pubKey(1))), p2pkh},
			{mustScript(txscript.NewScriptBuilder().AddOp(txscript.OP_0).AddData(sig).
				AddData(sig).AddData(multisig)), p2sh(multisig)},
		}},
		{"large p2sh", []spend{
			{mustScript(txscript.NewScriptBuilder().AddData(sig).AddData(sig).
				AddData(largeRedeem)), p2sh(largeRedeem)},
		}},
	}

	flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens |
		txscript.ScriptAllowMay2025 | txscript.ScriptAllowMay2026
	for _, bench := range benches {
		msgTx := wire.NewMsgTx(2)
		view := blockchain.NewUtxoViewpoint()
		for i, s := range bench.spends {
			op := wire.OutPoint{Hash: chainhash.Hash{byte(i + 1)}, Index: uint32(i)}
			msgTx.AddTxIn(&wire.TxIn{PreviousOutPoint: op,
				SignatureScript: s.sigScript, Sequence: wire.MaxTxInSequenceNum})
			view.Entries()[op] = blockchain.NewUtxoEntry(
				&wire.TxOut{Value: 100000, PkScript: s.pkScript}, 1, false)
		}
		for _, txOut := range outputs {
			msgTx.AddTxOut(txOut)
		}
		tx := bchutil.NewTx(msgTx)

		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				err := CheckTransactionStandard(tx, 1000, time.Unix(0, 0),
					DefaultMinRelayTxFee, 2, true, true)
				if err != nil {
					b.Fatal(err)
				}
				if err := CheckInputsStandard(tx, view, flags); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
