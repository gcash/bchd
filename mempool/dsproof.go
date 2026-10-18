// Copyright (c) 2020-2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// dsProofValidity is the outcome of checking a double spend proof against the
// memory pool and the utxo set.
type dsProofValidity int

const (
	dsProofValid dsProofValidity = iota
	dsProofMissingTransaction
	dsProofMissingUTXO
	dsProofInvalid
)

// String returns the validity in human-readable form.
func (v dsProofValidity) String() string {
	switch v {
	case dsProofValid:
		return "valid"
	case dsProofMissingTransaction:
		return "missing transaction"
	case dsProofMissingUTXO:
		return "missing utxo"
	}
	return "invalid"
}

// isPayToPubKeyHash returns whether the script is exactly a pay-to-pubkey-hash
// script.
func isPayToPubKeyHash(script []byte) bool {
	return len(script) == 25 &&
		script[0] == txscript.OP_DUP &&
		script[1] == txscript.OP_HASH160 &&
		script[2] == txscript.OP_DATA_20 &&
		script[23] == txscript.OP_EQUALVERIFY &&
		script[24] == txscript.OP_CHECKSIG
}

// isDSProofEmpty returns whether the proof lacks an outpoint or hash.
func isDSProofEmpty(proof *wire.MsgDSProof, hash *chainhash.Hash) bool {
	var zero chainhash.Hash
	return proof.PrevOut.Index > math.MaxInt32 ||
		proof.PrevOut.Hash == zero || *hash == zero
}

// spendersEqual returns whether the two spenders are identical.
func spendersEqual(a, b *wire.DSProofSpender) bool {
	if a.Version != b.Version || a.Sequence != b.Sequence ||
		a.LockTime != b.LockTime || a.HashPrevOutputs != b.HashPrevOutputs ||
		a.HashSequence != b.HashSequence || a.HashOutputs != b.HashOutputs ||
		len(a.PushData) != len(b.PushData) {
		return false
	}
	for i := range a.PushData {
		if !bytes.Equal(a.PushData[i], b.PushData[i]) {
			return false
		}
	}
	return true
}

// compareDSProofSpenders orders spenders by hashOutputs and then by
// hashPrevOutputs.
func compareDSProofSpenders(a, b *wire.DSProofSpender) int {
	if c := a.HashOutputs.Compare(&b.HashOutputs); c != 0 {
		return c
	}
	return a.HashPrevOutputs.Compare(&b.HashPrevOutputs)
}

// checkDSProofSanity checks the parts of a proof that do not depend on the
// memory pool: each spender must push exactly one validly encoded signature
// that does not sign the utxos, and the spenders must differ.
func checkDSProofSanity(proof *wire.MsgDSProof, hash *chainhash.Hash, flags txscript.ScriptFlags) error {
	if isDSProofEmpty(proof, hash) {
		return errors.New("proof is empty")
	}
	for _, s := range []*wire.DSProofSpender{&proof.FirstSpender, &proof.DoubleSpender} {
		if len(s.PushData) != 1 {
			return errors.New("proof must contain exactly one push per spender")
		}
		sig := s.PushData[0]
		if len(sig) == 0 {
			return errors.New("proof signature is empty")
		}
		if err := txscript.CheckTransactionSignatureEncoding(sig, flags); err != nil {
			return fmt.Errorf("proof contains an invalid signature: %v", err)
		}
		if txscript.SigHashType(sig[len(sig)-1])&txscript.SigHashUTXO != 0 {
			return errors.New("proof signature uses SIGHASH_UTXOS")
		}
	}
	if spendersEqual(&proof.FirstSpender, &proof.DoubleSpender) {
		return errors.New("proof spenders are the same")
	}
	return nil
}

// dsProofSigHash returns the signature hash a spender's signature commits to,
// built from the fields of the proof and the output being spent.
func dsProofSigHash(proof *wire.MsgDSProof, s *wire.DSProofSpender, coin *wire.TxOut,
	hashType txscript.SigHashType, flags txscript.ScriptFlags) []byte {

	var buf bytes.Buffer
	var b4 [4]byte
	var b8 [8]byte
	binary.LittleEndian.PutUint32(b4[:], s.Version)
	buf.Write(b4[:])
	buf.Write(s.HashPrevOutputs[:])
	buf.Write(s.HashSequence[:])
	buf.Write(proof.PrevOut.Hash[:])
	binary.LittleEndian.PutUint32(b4[:], proof.PrevOut.Index)
	buf.Write(b4[:])
	if !coin.TokenData.IsEmpty() && flags.HasFlag(txscript.ScriptAllowCashTokens) {
		tokenData := coin.TokenData.TokenDataBuffer()
		buf.Write(tokenData.Bytes())
	}
	_ = wire.WriteVarBytes(&buf, 0, coin.PkScript)
	binary.LittleEndian.PutUint64(b8[:], uint64(coin.Value))
	buf.Write(b8[:])
	binary.LittleEndian.PutUint32(b4[:], s.Sequence)
	buf.Write(b4[:])
	buf.Write(s.HashOutputs[:])
	binary.LittleEndian.PutUint32(b4[:], s.LockTime)
	buf.Write(b4[:])
	binary.LittleEndian.PutUint32(b4[:], uint32(hashType))
	buf.Write(b4[:])
	return chainhash.DoubleHashB(buf.Bytes())
}

// verifyDSProofSpender runs the pay-to-pubkey-hash script of coin with the
// spender's signature and pubKey, checking the signature against the hash
// built from the proof.
func verifyDSProofSpender(proof *wire.MsgDSProof, s *wire.DSProofSpender, coin *wire.TxOut,
	pubKey []byte, flags txscript.ScriptFlags) error {

	if !bytes.Equal(bchutil.Hash160(pubKey), coin.PkScript[3:23]) {
		return errors.New("public key does not match the output")
	}
	sig := s.PushData[0]
	if err := txscript.CheckTransactionSignatureEncoding(sig, flags); err != nil {
		return err
	}
	if err := txscript.CheckPubKeyEncoding(pubKey, flags); err != nil {
		return err
	}
	hashType := txscript.SigHashType(sig[len(sig)-1])
	sig = sig[:len(sig)-1]

	key, err := bchec.ParsePubKey(pubKey, bchec.S256())
	if err != nil {
		return err
	}
	var signature *bchec.Signature
	if len(sig) == 64 {
		signature, err = bchec.ParseSchnorrSignature(sig)
	} else {
		signature, err = bchec.ParseDERSignature(sig, bchec.S256())
	}
	if err != nil {
		return err
	}
	if !signature.Verify(dsProofSigHash(proof, s, coin, hashType, flags), key) {
		return errors.New("signature does not verify")
	}
	return nil
}

// dsProofPubKey returns the public key pushed by the input of spendingTx that
// spends the proof's outpoint, or nil.
func dsProofPubKey(proof *wire.MsgDSProof, spendingTx *wire.MsgTx) []byte {
	for _, in := range spendingTx.TxIn {
		if in.PreviousOutPoint != proof.PrevOut {
			continue
		}
		// The public key is the second push, after the signature.
		tokenizer := txscript.MakeScriptTokenizer(in.SignatureScript)
		for range 2 {
			if !tokenizer.Next() {
				return nil
			}
		}
		return tokenizer.Data()
	}
	return nil
}

// validateDSProof checks a proof against coin, the output it spends, and
// spendingTx, a memory pool transaction spending that output.  Either may be
// nil when it is not known.
func validateDSProof(proof *wire.MsgDSProof, hash *chainhash.Hash, coin *wire.TxOut,
	spendingTx *wire.MsgTx, flags txscript.ScriptFlags) (dsProofValidity, error) {

	if err := checkDSProofSanity(proof, hash, flags); err != nil {
		return dsProofInvalid, err
	}
	if compareDSProofSpenders(&proof.FirstSpender, &proof.DoubleSpender) > 0 {
		return dsProofInvalid, errors.New("spenders are not in canonical order")
	}
	if coin == nil {
		return dsProofMissingUTXO, nil
	}
	if !isPayToPubKeyHash(coin.PkScript) {
		return dsProofInvalid, errors.New("output is not pay-to-pubkey-hash")
	}
	if spendingTx == nil {
		return dsProofMissingTransaction, nil
	}
	pubKey := dsProofPubKey(proof, spendingTx)
	if len(pubKey) == 0 {
		return dsProofInvalid, errors.New("no public key in the spending transaction")
	}
	if err := verifyDSProofSpender(proof, &proof.FirstSpender, coin, pubKey, flags); err != nil {
		return dsProofInvalid, fmt.Errorf("first spender: %v", err)
	}
	if err := verifyDSProofSpender(proof, &proof.DoubleSpender, coin, pubKey, flags); err != nil {
		return dsProofInvalid, fmt.Errorf("second spender: %v", err)
	}
	return dsProofValid, nil
}

// p2pkhSignature verifies input idx of tx, which spends the pay-to-pubkey-hash
// output coin, and returns the signature it pushes.  sigHashes are the
// signature hashes of tx.
func p2pkhSignature(tx *wire.MsgTx, idx int, coin *wire.TxOut, flags txscript.ScriptFlags,
	sigHashes *txscript.TxSigHashes) ([]byte, error) {

	if !isPayToPubKeyHash(coin.PkScript) {
		return nil, errors.New("output is not pay-to-pubkey-hash")
	}
	utxos := txscript.NewUtxoCache()
	utxos.AddEntry(idx, *coin)
	vm, err := txscript.NewEngine(coin.PkScript, tx, idx, flags, nil,
		sigHashes, utxos, coin.Value)
	if err != nil {
		return nil, err
	}
	if err := vm.Execute(); err != nil {
		return nil, fmt.Errorf("input is not properly signed: %v", err)
	}
	pushes, err := txscript.PushedData(tx.TxIn[idx].SignatureScript)
	if err != nil || len(pushes) != 2 {
		return nil, errors.New("not a pay-to-pubkey-hash signature")
	}
	sig := pushes[0]
	if len(sig) == 0 {
		return nil, errors.New("input has no signature")
	}
	hashType := txscript.SigHashType(sig[len(sig)-1])
	if hashType&txscript.SigHashForkID == 0 {
		return nil, errors.New("signature does not use the fork id")
	}
	if hashType&txscript.SigHashUTXO != 0 {
		return nil, errors.New("signature uses SIGHASH_UTXOS")
	}
	return sig, nil
}

// hashDSProofSpender fills in the preimage hashes the spender's signature
// commits to, leaving the others zero.  sigHashes are the signature hashes
// of tx.
func hashDSProofSpender(s *wire.DSProofSpender, tx *wire.MsgTx, idx int, sigHashes *txscript.TxSigHashes) {
	hashType := txscript.SigHashType(s.PushData[0][len(s.PushData[0])-1])
	anyoneCanPay := hashType&txscript.SigHashAnyOneCanPay != 0
	base := hashType & 0x1f
	if !anyoneCanPay {
		s.HashPrevOutputs = sigHashes.HashPrevOuts
	}
	if !anyoneCanPay && base != txscript.SigHashSingle && base != txscript.SigHashNone {
		s.HashSequence = sigHashes.HashSequence
	}
	if base != txscript.SigHashSingle && base != txscript.SigHashNone {
		s.HashOutputs = sigHashes.HashOutputs
	} else if base == txscript.SigHashSingle && idx < len(tx.TxOut) {
		var b bytes.Buffer
		_ = wire.WriteTxOut(&b, 0, 0, tx.TxOut[idx])
		s.HashOutputs = chainhash.DoubleHashH(b.Bytes())
	}
}

// inputIndex returns the index of the input of tx spending op, or -1.
func inputIndex(tx *wire.MsgTx, op wire.OutPoint) int {
	for i, in := range tx.TxIn {
		if in.PreviousOutPoint == op {
			return i
		}
	}
	return -1
}

// newDSProof creates a proof that tx1 and tx2 both spend prevOut, the output
// coin.  Both inputs must be properly signed pay-to-pubkey-hash spends.
func newDSProof(tx1, tx2 *wire.MsgTx, prevOut wire.OutPoint, coin *wire.TxOut,
	flags txscript.ScriptFlags) (*wire.MsgDSProof, error) {

	if tx1.TxHash() == tx2.TxHash() {
		return nil, errors.New("transactions are the same")
	}
	idx1, idx2 := inputIndex(tx1, prevOut), inputIndex(tx2, prevOut)
	if idx1 < 0 || idx2 < 0 {
		return nil, errors.New("transactions do not both spend the outpoint")
	}

	spenders := make([]wire.DSProofSpender, 2)
	for i, side := range []struct {
		tx  *wire.MsgTx
		idx int
	}{{tx1, idx1}, {tx2, idx2}} {
		sigHashes := txscript.NewTxSigHashes(side.tx)
		sig, err := p2pkhSignature(side.tx, side.idx, coin, flags, sigHashes)
		if err != nil {
			return nil, err
		}
		s := &spenders[i]
		s.Version = uint32(side.tx.Version)
		s.Sequence = side.tx.TxIn[side.idx].Sequence
		s.LockTime = side.tx.LockTime
		s.PushData = [][]byte{sig}
		hashDSProofSpender(s, side.tx, side.idx, sigHashes)
	}

	// The order does not depend on which transaction was seen first.
	if compareDSProofSpenders(&spenders[0], &spenders[1]) > 0 {
		spenders[0], spenders[1] = spenders[1], spenders[0]
	}
	proof := wire.NewMsgDSProof(prevOut, spenders[0], spenders[1])
	hash := proof.ProofHash()
	if err := checkDSProofSanity(proof, &hash, flags); err != nil {
		return nil, err
	}
	return proof, nil
}

// signatureHashType returns the hash type of the signature pushed first by a
// signature script, or false when there is none or it does not use the fork
// id or signs the utxos.
func signatureHashType(sigScript []byte) (txscript.SigHashType, bool) {
	tokenizer := txscript.MakeScriptTokenizer(sigScript)
	if !tokenizer.Next() || len(tokenizer.Data()) == 0 {
		return 0, false
	}
	sig := tokenizer.Data()
	hashType := txscript.SigHashType(sig[len(sig)-1])
	if hashType&txscript.SigHashForkID == 0 || hashType&txscript.SigHashUTXO != 0 {
		return 0, false
	}
	return hashType, true
}

// proofPossibleForTx returns whether a proof could be made for every input of
// tx, a memory pool transaction, which needs every input to spend a known
// pay-to-pubkey-hash output with a fork id signature.  protected is also
// true when every input signs all inputs and outputs.  The signatures were
// verified when tx was accepted, so only their hash types are read.
func proofPossibleForTx(tx *wire.MsgTx, coin func(wire.OutPoint) *wire.TxOut) (possible, protected bool) {
	if len(tx.TxIn) == 0 || blockchain.IsCoinBaseTx(tx) {
		return false, false
	}
	unprotected := false
	for _, in := range tx.TxIn {
		c := coin(in.PreviousOutPoint)
		if c == nil || !isPayToPubKeyHash(c.PkScript) {
			return false, false
		}
		hashType, ok := signatureHashType(in.SignatureScript)
		if !ok {
			return false, false
		}
		if hashType&txscript.SigHashAnyOneCanPay != 0 ||
			hashType&0x1f != txscript.SigHashAll {
			unprotected = true
		}
	}
	return true, !unprotected
}
