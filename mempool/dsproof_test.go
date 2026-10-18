// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"math"
	"math/big"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// coinCounter makes the transactions added by addCoins unique.
var coinCounter atomic.Uint32

// addCoins adds a confirmed transaction with the given outputs to the fake
// chain and returns its outpoints.
func addCoins(p *poolHarness, outs ...*wire.TxOut) []wire.OutPoint {
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{0x01}, Index: coinCounter.Add(1)}})
	for _, out := range outs {
		tx.AddTxOut(out)
	}
	p.chain.utxos.AddTxOuts(bchutil.NewTx(tx), 1)
	ops := make([]wire.OutPoint, len(outs))
	for i := range outs {
		ops[i] = wire.OutPoint{Hash: tx.TxHash(), Index: uint32(i)}
	}
	return ops
}

// signP2PKH signs input idx of tx, which spends coin, and sets its signature
// script to the signature and the compressed public key.
func signP2PKH(t testing.TB, tx *wire.MsgTx, idx int, coin *wire.TxOut, key *bchec.PrivateKey,
	hashType txscript.SigHashType, schnorr bool) {

	t.Helper()
	sigHashes := txscript.NewTxSigHashes(tx)
	if !coin.TokenData.IsEmpty() {
		utxos := txscript.NewUtxoCache()
		utxos.AddEntry(idx, *coin)
		sigHashes = sigHashes.WithUtxos(tx, utxos)
	}
	hashType |= txscript.SigHashForkID
	hash, _, err := txscript.CalcSignatureHash(coin.PkScript, sigHashes, hashType, tx, idx, coin.Value, true)
	if err != nil {
		t.Fatal(err)
	}
	var sig []byte
	if schnorr {
		s, err := key.SignSchnorr(hash)
		if err != nil {
			t.Fatal(err)
		}
		sig = s.Serialize()
	} else {
		s, err := key.SignECDSA(hash)
		if err != nil {
			t.Fatal(err)
		}
		sig = s.Serialize()
	}
	sig = append(sig, byte(hashType))
	pub := (*bchec.PublicKey)(&key.PublicKey).SerializeCompressed()
	script, err := txscript.NewScriptBuilder().AddData(sig).AddData(pub).Script()
	if err != nil {
		t.Fatal(err)
	}
	tx.TxIn[idx].SignatureScript = script
}

// spend returns a transaction spending the outpoint op, which holds coin, to
// the harness script, keeping fee and sending the rest to nOut outputs.
func spend(t *testing.T, p *poolHarness, op wire.OutPoint, coin *wire.TxOut, nOut int, fee int64,
	hashType txscript.SigHashType, schnorr bool) *bchutil.Tx {

	t.Helper()
	return spendWithLockTime(t, p, op, coin, nOut, fee, hashType, schnorr, 0)
}

// spendWithLockTime is spend with a lock time.
func spendWithLockTime(t *testing.T, p *poolHarness, op wire.OutPoint, coin *wire.TxOut, nOut int, fee int64,
	hashType txscript.SigHashType, schnorr bool, lockTime uint32) *bchutil.Tx {

	t.Helper()
	tx := wire.NewMsgTx(1)
	tx.LockTime = lockTime
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op, Sequence: wire.MaxTxInSequenceNum})
	value := (coin.Value - fee) / int64(nOut)
	for range nOut {
		tx.AddTxOut(&wire.TxOut{Value: value, PkScript: p.payScript})
	}
	signP2PKH(t, tx, 0, coin, p.signKey, hashType, schnorr)
	return bchutil.NewTx(tx)
}

// doubleSpendPair returns a confirmed pay-to-pubkey-hash coin and two
// transactions spending it.
func doubleSpendPair(t *testing.T, p *poolHarness, hashType txscript.SigHashType, schnorr bool) (
	wire.OutPoint, *wire.TxOut, *bchutil.Tx, *bchutil.Tx) {

	t.Helper()
	coin := &wire.TxOut{Value: 100000000, PkScript: p.payScript}
	op := addCoins(p, coin)[0]
	tx1 := spend(t, p, op, coin, 1, 1000, hashType, schnorr)
	// With SIGHASH_NONE the signatures only differ when something else
	// signed does.
	tx2 := spendWithLockTime(t, p, op, coin, 1, 2000, hashType, schnorr, 1)
	return op, coin, tx1, tx2
}

// testFlags are the script flags used for the proofs built in the tests.
const testFlags = txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens

// TestDSProofCreateAndValidate creates proofs for double spends signed with
// every hash type and both signature schemes and checks that they validate,
// that the spenders are in canonical order, and that the id does not depend
// on which transaction was seen first.
func TestDSProofCreateAndValidate(t *testing.T) {
	harness := newTestHarness(t)
	hashTypes := []txscript.SigHashType{
		txscript.SigHashAll,
		txscript.SigHashNone,
		txscript.SigHashSingle,
		txscript.SigHashAll | txscript.SigHashAnyOneCanPay,
		txscript.SigHashNone | txscript.SigHashAnyOneCanPay,
		txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
	}
	for _, hashType := range hashTypes {
		for _, schnorr := range []bool{false, true} {
			op, coin, tx1, tx2 := doubleSpendPair(t, harness, hashType, schnorr)
			proof, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags)
			if err != nil {
				t.Fatalf("hash type %v schnorr %v: %v", hashType, schnorr, err)
			}
			hash := proof.ProofHash()
			for _, spender := range []*bchutil.Tx{tx1, tx2} {
				v, err := validateDSProof(proof, &hash, coin, spender.MsgTx(), testFlags)
				if v != dsProofValid {
					t.Fatalf("hash type %v schnorr %v: %v %v", hashType, schnorr, v, err)
				}
			}
			if compareDSProofSpenders(&proof.FirstSpender, &proof.DoubleSpender) > 0 {
				t.Fatal("spenders not sorted")
			}
			other, err := newDSProof(tx2.MsgTx(), tx1.MsgTx(), op, coin, testFlags)
			if err != nil {
				t.Fatal(err)
			}
			// The order only decides when both sort keys are equal,
			// as they are with SIGHASH_NONE.
			tie := compareDSProofSpenders(&proof.FirstSpender, &proof.DoubleSpender) == 0
			if (other.ProofHash() == hash) == tie {
				t.Fatalf("hash type %v: order independence %v with tie %v",
					hashType, other.ProofHash() == hash, tie)
			}

			// The hashes a signature does not commit to are zero.
			var zero chainhash.Hash
			s := proof.FirstSpender
			acp := hashType&txscript.SigHashAnyOneCanPay != 0
			base := hashType & 0x1f
			if (s.HashPrevOutputs == zero) != acp {
				t.Fatalf("hash type %v: hashPrevOutputs %v", hashType, s.HashPrevOutputs)
			}
			if (s.HashSequence == zero) != (acp || base != txscript.SigHashAll) {
				t.Fatalf("hash type %v: hashSequence %v", hashType, s.HashSequence)
			}
			if (s.HashOutputs == zero) != (base == txscript.SigHashNone) {
				t.Fatalf("hash type %v: hashOutputs %v", hashType, s.HashOutputs)
			}

			// The proof must be the same for a stored copy.
			var buf bytes.Buffer
			if err := proof.BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
				t.Fatal(err)
			}
			var decoded wire.MsgDSProof
			if err := decoded.BchDecode(&buf, wire.ProtocolVersion, wire.BaseEncoding); err != nil {
				t.Fatal(err)
			}
			if v, err := validateDSProof(&decoded, &hash, coin, tx1.MsgTx(), testFlags); v != dsProofValid {
				t.Fatalf("decoded copy: %v %v", v, err)
			}
		}
	}
}

// TestDSProofCreateRejects checks the double spends no proof is created for.
func TestDSProofCreateRejects(t *testing.T) {
	harness := newTestHarness(t)
	op, coin, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)

	if _, err := newDSProof(tx1.MsgTx(), tx1.MsgTx(), op, coin, testFlags); err == nil {
		t.Fatal("proof for a transaction with itself")
	}
	if _, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), wire.OutPoint{Index: 5}, coin, testFlags); err == nil {
		t.Fatal("proof for an outpoint neither spends")
	}

	// A badly signed double spend.
	bad := tx2.MsgTx().Copy()
	bad.TxOut[0].Value--
	if _, err := newDSProof(tx1.MsgTx(), bad, op, coin, testFlags); err == nil ||
		!strings.Contains(err.Error(), "not properly signed") {
		t.Fatalf("badly signed double spend: %v", err)
	}

	// A coin that is not pay-to-pubkey-hash.
	p2pk, err := txscript.NewScriptBuilder().
		AddData((*bchec.PublicKey)(&harness.signKey.PublicKey).SerializeCompressed()).
		AddOp(txscript.OP_CHECKSIG).Script()
	if err != nil {
		t.Fatal(err)
	}
	other := &wire.TxOut{Value: coin.Value, PkScript: p2pk}
	if _, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, other, testFlags); err == nil {
		t.Fatal("proof for a pay-to-pubkey coin")
	}

	// A signature with an extra push in the signature script.
	extra := tx2.MsgTx().Copy()
	extra.TxIn[0].SignatureScript = append(append([]byte{}, extra.TxIn[0].SignatureScript...), txscript.OP_1)
	if _, err := newDSProof(tx1.MsgTx(), extra, op, coin, testFlags); err == nil {
		t.Fatal("proof for a signature script with an extra push")
	}
}

// mutateProof returns a copy of proof with f applied.
func mutateProof(proof *wire.MsgDSProof, f func(*wire.MsgDSProof)) *wire.MsgDSProof {
	var buf bytes.Buffer
	_ = proof.BchEncode(&buf, 0, wire.BaseEncoding)
	var c wire.MsgDSProof
	_ = c.BchDecode(&buf, 0, wire.BaseEncoding)
	f(&c)
	return &c
}

// TestDSProofValidationRules checks one invalid proof per validation rule.
func TestDSProofValidationRules(t *testing.T) {
	harness := newTestHarness(t)
	op, coin, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, false)
	proof, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags)
	if err != nil {
		t.Fatal(err)
	}
	acpOp, acpCoin, acp1, acp2 := doubleSpendPair(t, harness, txscript.SigHashAll|txscript.SigHashAnyOneCanPay, true)
	acpProof, err := newDSProof(acp1.MsgTx(), acp2.MsgTx(), acpOp, acpCoin, testFlags)
	if err != nil {
		t.Fatal(err)
	}

	p2pk := &wire.TxOut{Value: coin.Value, PkScript: append([]byte{txscript.OP_DATA_33},
		append((*bchec.PublicKey)(&harness.signKey.PublicKey).SerializeCompressed(), txscript.OP_CHECKSIG)...)}
	noPubKey := tx1.MsgTx().Copy()
	noPubKey.TxIn[0].SignatureScript = noPubKey.TxIn[0].SignatureScript[:1+len(proof.FirstSpender.PushData[0])]
	otherKey, _ := bchec.NewPrivateKey(bchec.S256())
	otherPub := tx1.MsgTx().Copy()
	sigPush := otherPub.TxIn[0].SignatureScript[:1+len(proof.FirstSpender.PushData[0])]
	otherPub.TxIn[0].SignatureScript = append(append([]byte{}, sigPush...),
		append([]byte{txscript.OP_DATA_33}, (*bchec.PublicKey)(&otherKey.PublicKey).SerializeCompressed()...)...)
	highS := func(sig []byte) []byte {
		parsed, err := bchec.ParseDERSignature(sig[:len(sig)-1], bchec.S256())
		if err != nil {
			t.Fatal(err)
		}
		der := func(b []byte) []byte {
			if b[0]&0x80 != 0 {
				b = append([]byte{0}, b...)
			}
			return append([]byte{0x02, byte(len(b))}, b...)
		}
		s := new(big.Int).Sub(bchec.S256().N, parsed.S)
		body := append(der(parsed.R.Bytes()), der(s.Bytes())...)
		return append(append([]byte{0x30, byte(len(body))}, body...), sig[len(sig)-1])
	}

	tests := []struct {
		name   string
		proof  *wire.MsgDSProof
		coin   *wire.TxOut
		spends *wire.MsgTx
		want   dsProofValidity
		reason string
	}{
		{"valid", proof, coin, tx1.MsgTx(), dsProofValid, ""},
		{"valid anyonecanpay", acpProof, acpCoin, acp1.MsgTx(), dsProofValid, ""},
		{"missing utxo", proof, nil, tx1.MsgTx(), dsProofMissingUTXO, ""},
		{"missing transaction", proof, coin, nil, dsProofMissingTransaction, ""},
		{"null prevout hash", mutateProof(proof, func(p *wire.MsgDSProof) { p.PrevOut.Hash = chainhash.Hash{} }),
			coin, tx1.MsgTx(), dsProofInvalid, "empty"},
		{"prevout index above int32", mutateProof(proof, func(p *wire.MsgDSProof) { p.PrevOut.Index = math.MaxInt32 + 1 }),
			coin, tx1.MsgTx(), dsProofInvalid, "empty"},
		{"no push", mutateProof(proof, func(p *wire.MsgDSProof) { p.FirstSpender.PushData = nil }),
			coin, tx1.MsgTx(), dsProofInvalid, "exactly one push"},
		{"two pushes", mutateProof(proof, func(p *wire.MsgDSProof) {
			p.DoubleSpender.PushData = append(p.DoubleSpender.PushData, []byte{1})
		}), coin, tx1.MsgTx(), dsProofInvalid, "exactly one push"},
		{"empty signature", mutateProof(proof, func(p *wire.MsgDSProof) { p.FirstSpender.PushData[0] = nil }),
			coin, tx1.MsgTx(), dsProofInvalid, "empty"},
		{"bad encoding", mutateProof(proof, func(p *wire.MsgDSProof) { p.FirstSpender.PushData[0][0] = 0x31 }),
			coin, tx1.MsgTx(), dsProofInvalid, "invalid signature"},
		{"high s", mutateProof(proof, func(p *wire.MsgDSProof) { p.FirstSpender.PushData[0] = highS(p.FirstSpender.PushData[0]) }),
			coin, tx1.MsgTx(), dsProofInvalid, "invalid signature"},
		{"no fork id", mutateProof(proof, func(p *wire.MsgDSProof) {
			s := p.FirstSpender.PushData[0]
			s[len(s)-1] = byte(txscript.SigHashAll)
		}), coin, tx1.MsgTx(), dsProofInvalid, "invalid signature"},
		{"undefined hash type", mutateProof(proof, func(p *wire.MsgDSProof) {
			s := p.FirstSpender.PushData[0]
			s[len(s)-1] = byte(txscript.SigHashForkID | 4)
		}), coin, tx1.MsgTx(), dsProofInvalid, "invalid signature"},
		{"sighash utxos", mutateProof(proof, func(p *wire.MsgDSProof) {
			s := p.FirstSpender.PushData[0]
			s[len(s)-1] |= byte(txscript.SigHashUTXO)
		}), coin, tx1.MsgTx(), dsProofInvalid, "SIGHASH_UTXOS"},
		{"same spenders", mutateProof(proof, func(p *wire.MsgDSProof) { p.DoubleSpender = p.FirstSpender }),
			coin, tx1.MsgTx(), dsProofInvalid, "the same"},
		{"wrong order", mutateProof(proof, func(p *wire.MsgDSProof) {
			p.FirstSpender, p.DoubleSpender = p.DoubleSpender, p.FirstSpender
		}), coin, tx1.MsgTx(), dsProofInvalid, "canonical order"},
		{"not pay-to-pubkey-hash", proof, p2pk, tx1.MsgTx(), dsProofInvalid, "pay-to-pubkey-hash"},
		{"no public key", proof, coin, noPubKey, dsProofInvalid, "no public key"},
		{"other public key", proof, coin, otherPub, dsProofInvalid, "does not match"},
		{"wrong amount", proof, &wire.TxOut{Value: coin.Value + 1, PkScript: coin.PkScript},
			tx1.MsgTx(), dsProofInvalid, "does not verify"},
		{"wrong version", mutateProof(proof, func(p *wire.MsgDSProof) { p.DoubleSpender.Version = 2 }),
			coin, tx1.MsgTx(), dsProofInvalid, "second spender"},
		{"wrong sequence", mutateProof(proof, func(p *wire.MsgDSProof) { p.FirstSpender.Sequence-- }),
			coin, tx1.MsgTx(), dsProofInvalid, "first spender"},
		{"wrong locktime", mutateProof(proof, func(p *wire.MsgDSProof) { p.DoubleSpender.LockTime = 7 }),
			coin, tx1.MsgTx(), dsProofInvalid, "second spender"},
		{"wrong hash sequence", mutateProof(proof, func(p *wire.MsgDSProof) { p.FirstSpender.HashSequence[0] ^= 1 }),
			coin, tx1.MsgTx(), dsProofInvalid, "first spender"},
		{"wrong outpoint index", mutateProof(proof, func(p *wire.MsgDSProof) { p.PrevOut.Index = 1 }),
			coin, tx1.MsgTx(), dsProofInvalid, "no public key"},
		// The proof's hashPrevOutputs is used even when the signature
		// does not commit to it.
		{"anyonecanpay with hashPrevOutputs", mutateProof(acpProof, func(p *wire.MsgDSProof) {
			p.FirstSpender.HashPrevOutputs[0] = 1
		}), acpCoin, acp1.MsgTx(), dsProofInvalid, "first spender"},
	}
	for _, test := range tests {
		hash := test.proof.ProofHash()
		v, err := validateDSProof(test.proof, &hash, test.coin, test.spends, testFlags)
		if v != test.want {
			t.Errorf("%s: got %v (%v), want %v", test.name, v, err, test.want)
			continue
		}
		if test.reason != "" && (err == nil || !strings.Contains(err.Error(), test.reason)) {
			t.Errorf("%s: error %v, want %q", test.name, err, test.reason)
		}
	}
}

// FuzzDSProofValidate checks that validating arbitrary proofs against a
// real double spend never panics.
func FuzzDSProofValidate(f *testing.F) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		f.Fatal(err)
	}
	coin := &wire.TxOut{Value: 100000000, PkScript: harness.payScript}
	op := addCoins(harness, coin)[0]
	mk := func(fee int64) *wire.MsgTx {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op, Sequence: wire.MaxTxInSequenceNum})
		tx.AddTxOut(&wire.TxOut{Value: coin.Value - fee, PkScript: harness.payScript})
		signP2PKH(f, tx, 0, coin, harness.signKey, txscript.SigHashAll, true)
		return tx
	}
	tx1, tx2 := mk(1000), mk(2000)
	proof, err := newDSProof(tx1, tx2, op, coin, testFlags)
	if err != nil {
		f.Fatal(err)
	}
	var buf bytes.Buffer
	_ = proof.BchEncode(&buf, 0, wire.BaseEncoding)
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		var p wire.MsgDSProof
		if p.BchDecode(bytes.NewReader(data), 0, wire.BaseEncoding) != nil {
			return
		}
		hash := p.ProofHash()
		v, _ := validateDSProof(&p, &hash, coin, tx1, testFlags)
		if v == dsProofValid && p.PrevOut != op {
			t.Fatal("valid proof for another outpoint")
		}
	})
}

// newTestHarness returns a pool harness on the main network parameters.
func newTestHarness(t *testing.T) *poolHarness {
	t.Helper()
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	return harness
}

// TestSignatureHashType checks the hash types the score accepts.
func TestSignatureHashType(t *testing.T) {
	sig := make([]byte, 65)
	script := func(hashType byte) []byte {
		s := append([]byte{}, sig...)
		s[64] = hashType
		b, _ := txscript.NewScriptBuilder().AddData(s).AddData(make([]byte, 33)).Script()
		return b
	}
	tests := []struct {
		script []byte
		ok     bool
	}{
		{script(0x41), true},
		{script(0xc3), true},
		{script(0x01), false},
		{script(0x61), false},
		{[]byte{txscript.OP_0}, false},
		{nil, false},
	}
	for i, test := range tests {
		if _, ok := signatureHashType(test.script); ok != test.ok {
			t.Errorf("#%d: got %v", i, ok)
		}
	}
}
