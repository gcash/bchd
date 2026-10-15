// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
)

// benchPubKey returns a compressed public key derived from seed.
func benchPubKey(seed byte) []byte {
	_, pub := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{seed}, 32))
	return pub.SerializeCompressed()
}

// benchScript builds a script with the builder, failing the benchmark on
// error.
func benchScript(b *testing.B, builder *ScriptBuilder) []byte {
	script, err := builder.Script()
	if err != nil {
		b.Fatal(err)
	}
	return script
}

// benchScripts holds the scripts the analysis benchmarks run on.
type benchScripts struct {
	p2pkh, p2sh, p2sh32, multisig       []byte
	p2pkhSig, multisigSig, largeP2SHSig []byte
	largeRedeem                         []byte
}

// newBenchScripts returns the scripts the analysis benchmarks run on: the
// standard output scripts, a P2PKH and a 2-of-3 P2SH multisig signature
// script, and a P2SH signature script whose redeem script is about 9 KB.
func newBenchScripts(b *testing.B) *benchScripts {
	hash20 := bytes.Repeat([]byte{0x11}, 20)
	hash32 := bytes.Repeat([]byte{0x22}, 32)
	sig := append(bytes.Repeat([]byte{0x30}, 71), 0x41)

	var s benchScripts
	s.p2pkh = benchScript(b, NewScriptBuilder().AddOp(OP_DUP).AddOp(OP_HASH160).
		AddData(hash20).AddOp(OP_EQUALVERIFY).AddOp(OP_CHECKSIG))
	s.p2sh = benchScript(b, NewScriptBuilder().AddOp(OP_HASH160).AddData(hash20).
		AddOp(OP_EQUAL))
	s.p2sh32 = benchScript(b, NewScriptBuilder().AddOp(OP_HASH256).AddData(hash32).
		AddOp(OP_EQUAL))
	s.multisig = benchScript(b, NewScriptBuilder().AddOp(OP_2).
		AddData(benchPubKey(1)).AddData(benchPubKey(2)).AddData(benchPubKey(3)).
		AddOp(OP_3).AddOp(OP_CHECKMULTISIG))

	s.p2pkhSig = benchScript(b, NewScriptBuilder().AddData(sig).AddData(benchPubKey(1)))
	s.multisigSig = benchScript(b, NewScriptBuilder().AddOp(OP_0).AddData(sig).
		AddData(sig).AddData(s.multisig))

	// A redeem script of 9000 bytes: hash and drop 128-byte pushes.
	redeem := NewScriptBuilder()
	chunk := bytes.Repeat([]byte{0x33}, 128)
	for range 68 {
		redeem.AddData(chunk).AddOp(OP_SHA256).AddOp(OP_DROP)
	}
	redeem.AddOp(OP_1)
	s.largeRedeem = benchScript(b, redeem)
	s.largeP2SHSig = benchScript(b, NewScriptBuilder().AddData(sig).
		AddData(sig).AddData(s.largeRedeem))
	return &s
}

// BenchmarkGetScriptClass benchmarks classifying standard output scripts and
// a large nonstandard script.
func BenchmarkGetScriptClass(b *testing.B) {
	s := newBenchScripts(b)
	benches := []struct {
		name   string
		script []byte
	}{
		{"p2pkh", s.p2pkh},
		{"p2sh", s.p2sh},
		{"p2sh32", s.p2sh32},
		{"multisig", s.multisig},
		{"large redeem", s.largeRedeem},
	}
	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = GetScriptClass(bench.script)
			}
		})
	}
}

// BenchmarkIsPushOnlyScript benchmarks checking signature scripts are push
// only.
func BenchmarkIsPushOnlyScript(b *testing.B) {
	s := newBenchScripts(b)
	benches := []struct {
		name   string
		script []byte
	}{
		{"p2pkh", s.p2pkhSig},
		{"p2sh multisig", s.multisigSig},
		{"large p2sh", s.largeP2SHSig},
	}
	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = IsPushOnlyScript(bench.script)
			}
		})
	}
}

// BenchmarkExtractPkScriptAddrs benchmarks extracting the addresses of
// standard output scripts.
func BenchmarkExtractPkScriptAddrs(b *testing.B) {
	s := newBenchScripts(b)
	benches := []struct {
		name   string
		script []byte
	}{
		{"p2pkh", s.p2pkh},
		{"p2sh", s.p2sh},
		{"p2sh32", s.p2sh32},
		{"multisig", s.multisig},
		{"large redeem", s.largeRedeem},
	}
	params := &chaincfg.MainNetParams
	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _, _, _ = ExtractPkScriptAddrs(bench.script, params)
			}
		})
	}
}

// BenchmarkPushedData benchmarks extracting the data pushes of scripts.
func BenchmarkPushedData(b *testing.B) {
	s := newBenchScripts(b)
	benches := []struct {
		name   string
		script []byte
	}{
		{"p2pkh", s.p2pkh},
		{"p2sh", s.p2sh},
		{"p2sh32", s.p2sh32},
		{"multisig", s.multisig},
		{"large p2sh", s.largeP2SHSig},
	}
	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = PushedData(bench.script)
			}
		})
	}
}

// BenchmarkNewEngine benchmarks creating an engine for P2PKH, P2SH multisig
// and large P2SH spends with the standard flags.
func BenchmarkNewEngine(b *testing.B) {
	s := newBenchScripts(b)
	p2shOf := func(redeem []byte) []byte {
		return benchScript(b, NewScriptBuilder().AddOp(OP_HASH160).
			AddData(hash160(redeem)).AddOp(OP_EQUAL))
	}
	benches := []struct {
		name                string
		sigScript, pkScript []byte
	}{
		{"p2pkh", s.p2pkhSig, s.p2pkh},
		{"p2sh multisig", s.multisigSig, p2shOf(s.multisig)},
		{"large p2sh", s.largeP2SHSig, p2shOf(s.largeRedeem)},
	}
	flags := StandardVerifyFlags | ScriptAllowCashTokens | ScriptAllowMay2025 |
		ScriptAllowMay2026
	for _, bench := range benches {
		tx := wire.NewMsgTx(2)
		tx.AddTxIn(&wire.TxIn{SignatureScript: bench.sigScript})
		tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: s.p2pkh})
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := NewEngine(bench.pkScript, tx, 0, flags, nil,
					nil, nil, 1000); err != nil {

					b.Fatal(err)
				}
			}
		})
	}
}
