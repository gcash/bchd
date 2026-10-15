// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
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
