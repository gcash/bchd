// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// benchmarkP2PKHInput benchmarks validating a signed pay-to-pubkey-hash input
// with the standard verification flags and no signature cache.
func benchmarkP2PKHInput(b *testing.B, schnorr bool) {
	key := hexToBytes("9e0699c91ca1e3b7e3c9ba71eb71c89890872be97576010fe593fbf3fd57e66d")
	priv, pub := bchec.PrivKeyFromBytes(bchec.S256(), key)
	pubKey := pub.SerializeCompressed()
	addr, err := bchutil.NewAddressPubKeyHash(bchutil.Hash160(pubKey),
		&chaincfg.MainNetParams)
	if err != nil {
		b.Fatal(err)
	}
	pkScript, err := PayToAddrScript(addr)
	if err != nil {
		b.Fatal(err)
	}

	const amount = 100000
	tx := wire.NewMsgTx(2)
	prevHash := chainhash.DoubleHashH([]byte("prevout"))
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, 0), nil))
	tx.AddTxOut(wire.NewTxOut(amount-1000, pkScript, wire.TokenData{}))

	hashType := SigHashAll | SigHashForkID
	var sig []byte
	if schnorr {
		sig, err = RawTxInSchnorrSignature(tx, 0, pkScript, hashType, priv, amount)
	} else {
		sig, err = RawTxInECDSASignature(tx, 0, pkScript, hashType, priv, amount)
	}
	if err != nil {
		b.Fatal(err)
	}
	tx.TxIn[0].SignatureScript, err = NewScriptBuilder().AddData(sig).
		AddData(pubKey).Script()
	if err != nil {
		b.Fatal(err)
	}

	hashCache := NewTxSigHashes(tx)
	for b.Loop() {
		vm, err := NewEngine(pkScript, tx, 0, StandardVerifyFlags, nil,
			hashCache, nil, amount)
		if err != nil {
			b.Fatal(err)
		}
		if err := vm.Execute(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkP2PKHInputECDSA benchmarks validating a P2PKH input signed with
// ECDSA.
func BenchmarkP2PKHInputECDSA(b *testing.B) {
	benchmarkP2PKHInput(b, false)
}

// BenchmarkP2PKHInputSchnorr benchmarks validating a P2PKH input signed with
// Schnorr.
func BenchmarkP2PKHInputSchnorr(b *testing.B) {
	benchmarkP2PKHInput(b, true)
}

// benchmarkTokenTxInput benchmarks validating the first input of a
// transaction with the given number of inputs when tokens are enabled, using
// sighashes that include the spent outputs like blockchain does.  The inputs
// spend pay-to-pubkey-hash or 2-of-3 multisig outputs, and every fifth spent
// output carries fungible tokens.
func benchmarkTokenTxInput(b *testing.B, numInputs int, multisig, cached bool) {
	keys := make([]*bchec.PrivateKey, 3)
	pubKeys := make([][]byte, 3)
	for i := range keys {
		keys[i], _ = bchec.PrivKeyFromBytes(bchec.S256(),
			bytes.Repeat([]byte{byte(i + 1)}, 32))
		pubKeys[i] = keys[i].PubKey().SerializeCompressed()
	}

	builder := NewScriptBuilder()
	if multisig {
		builder.AddOp(OP_2).AddData(pubKeys[0]).AddData(pubKeys[1]).
			AddData(pubKeys[2]).AddOp(OP_3).AddOp(OP_CHECKMULTISIG)
	} else {
		builder.AddOp(OP_DUP).AddOp(OP_HASH160).
			AddData(bchutil.Hash160(pubKeys[0])).AddOp(OP_EQUALVERIFY).
			AddOp(OP_CHECKSIG)
	}
	pkScript, err := builder.Script()
	if err != nil {
		b.Fatal(err)
	}

	const amount = 100000
	tx := wire.NewMsgTx(2)
	utxos := NewUtxoCache()
	for i := range numInputs {
		prevHash := chainhash.DoubleHashH([]byte{byte(i)})
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, 0), nil))
		var tokenData wire.TokenData
		if i%5 == 0 {
			tokenData = wire.TokenData{
				CategoryID: prevHash,
				Amount:     1000,
				BitField:   wire.HAS_AMOUNT,
			}
		}
		utxos.AddEntry(i, *wire.NewTxOut(amount, pkScript, tokenData))
	}
	tx.AddTxOut(wire.NewTxOut(int64(numInputs)*amount-10000, pkScript,
		wire.TokenData{}))

	hashCache := NewTxSigHashes(tx)
	hashCache.AddTxSigHashUtxoFromUtxoCache(tx, utxos)
	hashType := SigHashAll | SigHashForkID
	hash, _, err := CalcSignatureHash(pkScript, hashCache, hashType, tx, 0,
		amount, true)
	if err != nil {
		b.Fatal(err)
	}
	builder = NewScriptBuilder()
	signers := keys[:1]
	if multisig {
		builder.AddOp(OP_0)
		signers = keys[:2]
	}
	for _, key := range signers {
		sig, err := key.SignECDSA(hash)
		if err != nil {
			b.Fatal(err)
		}
		builder.AddData(append(sig.Serialize(), byte(hashType)))
	}
	if !multisig {
		builder.AddData(pubKeys[0])
	}
	tx.TxIn[0].SignatureScript, err = builder.Script()
	if err != nil {
		b.Fatal(err)
	}

	flags := StandardVerifyFlags | ScriptAllowCashTokens
	var sigCache *SigCache
	if cached {
		sigCache = NewSigCache(10)
	}
	validate := func() {
		vm, err := NewEngine(pkScript, tx, 0, flags, sigCache, hashCache,
			utxos, amount)
		if err != nil {
			b.Fatal(err)
		}
		if err := vm.Execute(); err != nil {
			b.Fatal(err)
		}
	}
	validate()
	for b.Loop() {
		validate()
	}
}

// BenchmarkTokenTxP2PKHInput benchmarks validating a P2PKH input of a
// transaction when tokens are enabled, with and without the signature in the
// signature cache.
func BenchmarkTokenTxP2PKHInput(b *testing.B) {
	for _, numInputs := range []int{10, 50} {
		for _, cached := range []bool{false, true} {
			name := fmt.Sprintf("inputs=%d/sigcache=%v", numInputs, cached)
			b.Run(name, func(b *testing.B) {
				benchmarkTokenTxInput(b, numInputs, false, cached)
			})
		}
	}
}

// BenchmarkTokenTxMultisigInput benchmarks validating a 2-of-3 multisig input
// of a transaction when tokens are enabled.
func BenchmarkTokenTxMultisigInput(b *testing.B) {
	for _, numInputs := range []int{10, 50} {
		b.Run(fmt.Sprintf("inputs=%d", numInputs), func(b *testing.B) {
			benchmarkTokenTxInput(b, numInputs, true, false)
		})
	}
}

// BenchmarkParseScript benchmarks parsing a P2PKH script, a token covenant
// script, a P2SH input with a 9 KB redeem script of one-byte opcodes, that
// redeem script and a single 9990-byte push.
func BenchmarkParseScript(b *testing.B) {
	hash := bytes.Repeat([]byte{0x11}, 20)
	category := bytes.Repeat([]byte{0x22}, 32)
	p2pkh := benchScript(b, NewScriptBuilder().AddOp(OP_DUP).AddOp(OP_HASH160).
		AddData(hash).AddOp(OP_EQUALVERIFY).AddOp(OP_CHECKSIG))
	covenant := benchScript(b, NewScriptBuilder().
		AddOp(OP_INPUTINDEX).AddOp(OP_UTXOTOKENCATEGORY).AddData(category).
		AddOp(OP_EQUALVERIFY).
		AddOp(OP_INPUTINDEX).AddOp(OP_OUTPUTTOKENCATEGORY).AddData(category).
		AddOp(OP_EQUALVERIFY).
		AddOp(OP_INPUTINDEX).AddOp(OP_UTXOBYTECODE).
		AddOp(OP_INPUTINDEX).AddOp(OP_OUTPUTBYTECODE).AddOp(OP_EQUALVERIFY).
		AddOp(OP_INPUTINDEX).AddOp(OP_OUTPUTTOKENCOMMITMENT).AddOp(OP_BIN2NUM).
		AddOp(OP_INPUTINDEX).AddOp(OP_UTXOTOKENCOMMITMENT).AddOp(OP_BIN2NUM).
		AddOp(OP_1ADD).AddOp(OP_NUMEQUALVERIFY).
		AddOp(OP_INPUTINDEX).AddOp(OP_OUTPUTVALUE).AddInt64(1000).
		AddOp(OP_GREATERTHANOREQUAL).AddOp(OP_VERIFY).
		AddOp(OP_TXOUTPUTCOUNT).AddOp(OP_2).AddOp(OP_NUMEQUALVERIFY).
		AddData(benchPubKey(1)).AddOp(OP_CHECKSIG))
	redeem := bytes.Repeat([]byte{OP_DUP, OP_DROP}, 4500)
	p2sh := benchScript(b, NewScriptBuilder().AddOp(OP_1).AddData(redeem))
	bigPush := append([]byte{OP_PUSHDATA2, 0x06, 0x27}, make([]byte, 9990)...)

	benches := []struct {
		name   string
		script []byte
	}{
		{"p2pkh", p2pkh},
		{"covenant", covenant},
		{"p2sh-input", p2sh},
		{"redeem-9k", redeem},
		{"push-9990", bigPush},
	}
	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := parseScript(bench.script); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// benchBlockFlags approximates the script flags of block validation after the
// May 2026 upgrade.
const benchBlockFlags = (StandardVerifyFlags | ScriptAllowCashTokens |
	ScriptAllowMay2025 | ScriptAllowMay2026) &^
	(ScriptDiscourageUpgradableNops | ScriptVerifyInputSigChecks)

// benchExecute validates input 0 of tx spending pkScript with amt.
func benchExecute(b *testing.B, tx *wire.MsgTx, pkScript []byte, amt int64, flags ScriptFlags) {
	hashes := NewTxSigHashes(tx)
	run := func() error {
		vm, err := NewEngine(pkScript, tx, 0, flags, nil, hashes, nil, amt)
		if err != nil {
			return err
		}
		return vm.Execute()
	}
	if err := run(); err != nil {
		b.Fatalf("script fails: %v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = run()
	}
}

// benchKey returns a fixed private key derived from seed.
func benchKey(seed byte) *bchec.PrivateKey {
	key, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{seed}, 32))
	return key
}

// benchSign returns an ECDSA SIGHASH_ALL|FORKID signature for input 0.
func benchSign(b *testing.B, tx *wire.MsgTx, script []byte, amt int64, key *bchec.PrivateKey) []byte {
	sig, err := RawTxInECDSASignature(tx, 0, script, SigHashAll|SigHashForkID, key, amt)
	if err != nil {
		b.Fatal(err)
	}
	return sig
}

// BenchmarkExecuteP2PKH validates a P2PKH input with an ECDSA signature.
func BenchmarkExecuteP2PKH(b *testing.B) {
	const amt = 100000
	key := benchKey(1)
	pubKey := key.PubKey().SerializeCompressed()
	pkScript, err := NewScriptBuilder().AddOp(OP_DUP).AddOp(OP_HASH160).
		AddData(bchutil.Hash160(pubKey)).AddOp(OP_EQUALVERIFY).
		AddOp(OP_CHECKSIG).Script()
	if err != nil {
		b.Fatal(err)
	}
	tx := createSpendingTx(nil, pkScript, amt)
	sig := benchSign(b, tx, pkScript, amt, key)
	tx.TxIn[0].SignatureScript, err = NewScriptBuilder().AddData(sig).
		AddData(pubKey).Script()
	if err != nil {
		b.Fatal(err)
	}
	benchExecute(b, tx, pkScript, amt, benchBlockFlags)
}

// BenchmarkExecuteP2SHMultisig validates a 2-of-3 P2SH multisig input with
// ECDSA signatures.
func BenchmarkExecuteP2SHMultisig(b *testing.B) {
	const amt = 100000
	keys := []*bchec.PrivateKey{benchKey(1), benchKey(2), benchKey(3)}
	builder := NewScriptBuilder().AddOp(OP_2)
	for _, key := range keys {
		builder.AddData(key.PubKey().SerializeCompressed())
	}
	redeemScript, err := builder.AddOp(OP_3).AddOp(OP_CHECKMULTISIG).Script()
	if err != nil {
		b.Fatal(err)
	}
	pkScript, err := NewScriptBuilder().AddOp(OP_HASH160).
		AddData(bchutil.Hash160(redeemScript)).AddOp(OP_EQUAL).Script()
	if err != nil {
		b.Fatal(err)
	}
	tx := createSpendingTx(nil, pkScript, amt)
	tx.TxIn[0].SignatureScript, err = NewScriptBuilder().AddOp(OP_0).
		AddData(benchSign(b, tx, redeemScript, amt, keys[0])).
		AddData(benchSign(b, tx, redeemScript, amt, keys[1])).
		AddData(redeemScript).Script()
	if err != nil {
		b.Fatal(err)
	}
	benchExecute(b, tx, pkScript, amt, benchBlockFlags)
}

// BenchmarkExecuteOpcodes runs a script of 3000 cheap opcodes.
func BenchmarkExecuteOpcodes(b *testing.B) {
	sigScript, err := NewScriptBuilder().AddData(bytes.Repeat([]byte{1}, 1000)).Script()
	if err != nil {
		b.Fatal(err)
	}
	pkScript := append([]byte{OP_DROP}, bytes.Repeat([]byte{OP_NOP}, 3000)...)
	pkScript = append(pkScript, OP_1)
	benchExecute(b, createSpendingTx(sigScript, pkScript, 0), pkScript, 0, benchBlockFlags)
}
