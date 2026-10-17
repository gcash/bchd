// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/wire"
)

// sigOpTestKey returns a deterministic private key for the signature opcode
// tests.
func sigOpTestKey(t *testing.T, seed string) *bchec.PrivateKey {
	t.Helper()

	d := sha256.Sum256([]byte(seed))
	priv, _ := bchec.PrivKeyFromBytes(bchec.S256(), d[:])
	return priv
}

// offCurvePubKey returns a compressed public key that passes the strict
// encoding rules but whose x coordinate is not on the curve.
func offCurvePubKey(t *testing.T) []byte {
	t.Helper()

	for x := byte(1); x < 255; x++ {
		pubKey := make([]byte, 33)
		pubKey[0] = 0x02
		pubKey[32] = x
		if _, err := bchec.ParsePubKey(pubKey, bchec.S256()); err != nil {
			return pubKey
		}
	}
	t.Fatal("no off-curve public key found")
	return nil
}

// derSig returns a DER encoded signature with the passed R and S encodings.
func derSig(r, s []byte) []byte {
	sig := []byte{0x30, byte(4 + len(r) + len(s)), 0x02, byte(len(r))}
	sig = append(sig, r...)
	sig = append(sig, 0x02, byte(len(s)))
	return append(sig, s...)
}

// derInt returns the 33 byte DER integer encoding of v, which must have its
// top bit set.
func derInt(v *big.Int) []byte {
	b := make([]byte, 33)
	v.FillBytes(b[1:])
	return b
}

// buildScript returns the script built by the passed builder.
func buildScript(t *testing.T, b *ScriptBuilder) []byte {
	t.Helper()

	script, err := b.Script()
	if err != nil {
		t.Fatal(err)
	}
	return script
}

// runSigOpTest executes the passed scripts with the passed flags and
// signature cache.  The spent output holds 1000 satoshis.
func runSigOpTest(t *testing.T, sigScript, pkScript []byte, flags ScriptFlags,
	sigCache *SigCache) (*Engine, error) {

	t.Helper()

	tx := createSpendingTx(sigScript, pkScript, 1000)
	return runSigOpTestTx(t, tx, pkScript, flags, sigCache)
}

// runSigOpTestTx executes the first input of the passed transaction, which
// spends an output with the passed locking script and 1000 satoshis.
func runSigOpTestTx(t *testing.T, tx *wire.MsgTx, pkScript []byte,
	flags ScriptFlags, sigCache *SigCache) (*Engine, error) {

	t.Helper()

	utxos := NewUtxoCache()
	utxos.AddEntry(0, *wire.NewTxOut(1000, pkScript, wire.TokenData{}))
	vm, err := NewEngine(pkScript, tx, 0, flags, sigCache, nil, utxos, 1000)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return vm, vm.Execute()
}

// TestCheckSigNullFail ensures a non-empty signature that fails to verify
// fails OP_CHECKSIG and OP_CHECKDATASIG under NULLFAIL, also when the public
// key is not a point on the curve or the signature scalars are out of range.
func TestCheckSigNullFail(t *testing.T) {
	t.Parallel()

	priv := sigOpTestKey(t, "nullfail")
	pubKey := priv.PubKey().SerializeCompressed()
	msg := []byte("m")
	digest := sha256.Sum256(msg)
	ecdsaSig, err := priv.SignECDSA(digest[:])
	if err != nil {
		t.Fatal(err)
	}
	validDER := ecdsaSig.Serialize()
	schnorrSig := bytes.Repeat([]byte{0x11}, 64)
	n := bchec.S256().N

	tests := []struct {
		name   string
		sig    []byte
		pubKey []byte
	}{
		{"schnorr sig, off-curve key", schnorrSig, offCurvePubKey(t)},
		{"DER sig, off-curve key", validDER, offCurvePubKey(t)},
		{"DER sig, uncompressed off-curve key", validDER,
			append([]byte{0x04}, make([]byte, 64)...)},
		{"DER sig r=0", derSig([]byte{0x00}, []byte{0x01}), pubKey},
		{"DER sig s=0", derSig([]byte{0x01}, []byte{0x00}), pubKey},
		{"DER sig r=n", derSig(derInt(n), []byte{0x01}), pubKey},
		{"DER sig r=2^256-1", derSig(derInt(new(big.Int).Sub(
			new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))),
			[]byte{0x01}), pubKey},
	}

	hashType := byte(SigHashAll | SigHashForkID)
	for _, test := range tests {
		// OP_CHECKSIG.
		sigScript := buildScript(t, NewScriptBuilder().
			AddData(append(append([]byte{}, test.sig...), hashType)))
		pkScript := buildScript(t, NewScriptBuilder().AddData(test.pubKey).
			AddOp(OP_CHECKSIG).AddOp(OP_NOT))
		_, err := runSigOpTest(t, sigScript, pkScript, may2026TestFlags, nil)
		if !IsErrorCode(err, ErrNullFail) {
			t.Errorf("CHECKSIG %s: got error %v, want %v", test.name,
				err, ErrNullFail)
		}

		// OP_CHECKDATASIG.
		sigScript = buildScript(t, NewScriptBuilder().AddData(test.sig).
			AddData(msg))
		pkScript = buildScript(t, NewScriptBuilder().AddData(test.pubKey).
			AddOp(OP_CHECKDATASIG).AddOp(OP_NOT))
		_, err = runSigOpTest(t, sigScript, pkScript, may2026TestFlags, nil)
		if !IsErrorCode(err, ErrNullFail) {
			t.Errorf("CHECKDATASIG %s: got error %v, want %v", test.name,
				err, ErrNullFail)
		}
	}
}

// TestCheckDataSigPubKeyLength ensures OP_CHECKDATASIG fails a valid
// signature for a public key whose length does not match its prefix, such as
// an uncompressed key with a 0x05 prefix, when strict encoding is off.
func TestCheckDataSigPubKeyLength(t *testing.T) {
	t.Parallel()

	priv := sigOpTestKey(t, "datasig key length")
	msg := []byte("m")
	digest := sha256.Sum256(msg)
	sig, err := priv.SignECDSA(digest[:])
	if err != nil {
		t.Fatal(err)
	}
	flags := may2026TestFlags &^ (ScriptVerifyStrictEncoding |
		ScriptVerifyNullFail)

	pubKey := priv.PubKey().SerializeUncompressed()
	for _, prefix := range []byte{0x04, 0x05} {
		pubKey[0] = prefix
		want := prefix == 0x04
		sigScript := buildScript(t, NewScriptBuilder().
			AddData(sig.Serialize()).AddData(msg))
		pkScript := buildScript(t, NewScriptBuilder().AddData(pubKey).
			AddOp(OP_CHECKDATASIG))
		_, err := runSigOpTest(t, sigScript, pkScript, flags, nil)
		if got := err == nil; got != want {
			t.Errorf("prefix %#x: got valid %v, want %v (error %v)",
				prefix, got, want, err)
		}
	}
}

// TestCheckSigFailedSigChecks ensures a non-empty signature that fails to
// verify is counted as a signature check, also when the public key or the
// signature does not parse.
func TestCheckSigFailedSigChecks(t *testing.T) {
	t.Parallel()

	priv := sigOpTestKey(t, "sigchecks")
	pubKey := priv.PubKey().SerializeCompressed()
	r0 := derSig([]byte{0x00}, []byte{0x01})
	hashType := byte(SigHashAll | SigHashForkID)
	flags := may2026TestFlags &^ ScriptVerifyNullFail

	tests := []struct {
		name      string
		sigScript []byte
		pkScript  []byte
	}{{
		name: "CHECKSIG off-curve key",
		sigScript: buildScript(t, NewScriptBuilder().AddData(
			append(bytes.Repeat([]byte{0x11}, 64), hashType))),
		pkScript: buildScript(t, NewScriptBuilder().
			AddData(offCurvePubKey(t)).AddOp(OP_CHECKSIG).AddOp(OP_NOT)),
	}, {
		name: "CHECKSIG r=0",
		sigScript: buildScript(t, NewScriptBuilder().AddData(
			append(append([]byte{}, r0...), hashType))),
		pkScript: buildScript(t, NewScriptBuilder().AddData(pubKey).
			AddOp(OP_CHECKSIG).AddOp(OP_NOT)),
	}, {
		name: "CHECKDATASIG off-curve key",
		sigScript: buildScript(t, NewScriptBuilder().
			AddData(bytes.Repeat([]byte{0x11}, 64)).AddData([]byte("m"))),
		pkScript: buildScript(t, NewScriptBuilder().
			AddData(offCurvePubKey(t)).AddOp(OP_CHECKDATASIG).
			AddOp(OP_NOT)),
	}, {
		name: "CHECKDATASIG r=0",
		sigScript: buildScript(t, NewScriptBuilder().AddData(r0).
			AddData([]byte("m"))),
		pkScript: buildScript(t, NewScriptBuilder().AddData(pubKey).
			AddOp(OP_CHECKDATASIG).AddOp(OP_NOT)),
	}}

	for _, test := range tests {
		vm, err := runSigOpTest(t, test.sigScript, test.pkScript, flags, nil)
		if err != nil {
			t.Errorf("%s: unexpected error %v", test.name, err)
			continue
		}
		if vm.SigChecks() != 1 {
			t.Errorf("%s: got %d sig checks, want 1", test.name,
				vm.SigChecks())
		}
		if vm.metrics.numSigChecks != 1 {
			t.Errorf("%s: got %d metric sig checks, want 1", test.name,
				vm.metrics.numSigChecks)
		}
	}
}

// signLegacyMultiSig returns a transaction that spends an output with the
// passed m-of-n multisig locking script with an ECDSA signature by the passed
// key, along with the number of bytes hashed for the signature hash.
func signLegacyMultiSig(t *testing.T, priv *bchec.PrivateKey,
	pkScript []byte) (*wire.MsgTx, int) {

	t.Helper()

	tx := createSpendingTx(nil, pkScript, 1000)
	utxos := NewUtxoCache()
	utxos.AddEntry(0, *wire.NewTxOut(1000, pkScript, wire.TokenData{}))
	pops, err := parseScript(pkScript)
	if err != nil {
		t.Fatal(err)
	}
	hashType := SigHashAll | SigHashForkID
	hash, bytesHashed, err := calcSignatureHash(pops,
		NewTxSigHashes(tx).WithUtxos(tx, utxos), hashType, tx, 0, 1000,
		true)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := priv.SignECDSA(hash)
	if err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].SignatureScript = buildScript(t, NewScriptBuilder().
		AddOp(OP_0).AddData(append(sig.Serialize(), byte(hashType))))
	return tx, bytesHashed
}

// TestCheckMultiSigHashCost ensures legacy OP_CHECKMULTISIG charges the
// signature hash for every public key it evaluates against a non-null
// signature, also when that public key is not a point on the curve.
func TestCheckMultiSigHashCost(t *testing.T) {
	t.Parallel()

	signer := sigOpTestKey(t, "multisig signer")
	pkScript := buildScript(t, NewScriptBuilder().AddOp(OP_1).
		AddData(signer.PubKey().SerializeCompressed()).
		AddData(offCurvePubKey(t)).AddOp(OP_2).AddOp(OP_CHECKMULTISIG))
	tx, bytesHashed := signLegacyMultiSig(t, signer, pkScript)
	vm, err := runSigOpTestTx(t, tx, pkScript, may2026TestFlags, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	want := int64(2 * GetDigestIterationCount(bytesHashed, true))
	if got := vm.metrics.GetHashDigestIterations(); got != want {
		t.Fatalf("got %d hash digest iterations, want %d", got, want)
	}
}

// TestSigCacheSignatureType ensures a cached valid ECDSA signature does not
// make a Schnorr signature with the same R and S valid, and the reverse, so
// the result of a signature check does not depend on the cache contents.
func TestSigCacheSignatureType(t *testing.T) {
	t.Parallel()

	priv := sigOpTestKey(t, "sigcache type")
	pkScript := buildScript(t, NewScriptBuilder().
		AddData(priv.PubKey().SerializeCompressed()).
		AddOp(OP_CHECKDATASIG))

	// Pick a message whose Schnorr signature has a low S and an R below
	// the curve order, so that its DER twin keeps the same R and S.
	n := bchec.S256().N
	halfN := new(big.Int).Rsh(n, 1)
	var msg []byte
	var digest [32]byte
	var schnorrSig *bchec.Signature
	for i := 0; ; i++ {
		msg = fmt.Appendf(nil, "sigcache %d", i)
		digest = sha256.Sum256(msg)
		sig, err := priv.SignSchnorr(digest[:])
		if err != nil {
			t.Fatal(err)
		}
		if sig.S.Cmp(halfN) <= 0 && sig.R.Cmp(n) < 0 {
			schnorrSig = sig
			break
		}
	}

	run := func(sig []byte, sigCache *SigCache) error {
		sigScript := buildScript(t, NewScriptBuilder().AddData(sig).
			AddData(msg))
		_, err := runSigOpTest(t, sigScript, pkScript,
			may2026TestFlags&^ScriptVerifyNullFail, sigCache)
		return err
	}
	pad32 := func(v *big.Int) []byte {
		b := make([]byte, 32)
		v.FillBytes(b)
		return b
	}

	// A valid ECDSA signature and the Schnorr encoding of its R and S.
	ecdsaSig, err := priv.SignECDSA(digest[:])
	if err != nil {
		t.Fatal(err)
	}
	ecdsaTwin := append(pad32(ecdsaSig.R), pad32(ecdsaSig.S)...)

	// The DER encoding of the Schnorr signature's R and S.
	schnorrTwin := (&bchec.Signature{R: schnorrSig.R, S: schnorrSig.S}).
		Serialize()
	parsedTwin, err := bchec.ParseDERSignature(schnorrTwin, bchec.S256())
	if err != nil {
		t.Fatal(err)
	}
	if parsedTwin.R.Cmp(schnorrSig.R) != 0 ||
		parsedTwin.S.Cmp(schnorrSig.S) != 0 {

		t.Fatal("DER twin does not keep the Schnorr R and S")
	}

	tests := []struct {
		name  string
		valid []byte
		twin  []byte
	}{
		{"ECDSA cached", ecdsaSig.Serialize(), ecdsaTwin},
		{"Schnorr cached", schnorrSig.Serialize(), schnorrTwin},
	}
	for _, test := range tests {
		if err := run(test.twin, nil); err == nil {
			t.Fatalf("%s: twin accepted without a cache", test.name)
		}
		sigCache := NewSigCache(10)
		if err := run(test.valid, sigCache); err != nil {
			t.Fatalf("%s: unexpected error %v", test.name, err)
		}
		if err := run(test.twin, sigCache); err == nil {
			t.Errorf("%s: twin accepted from the cache", test.name)
		}
	}
}
