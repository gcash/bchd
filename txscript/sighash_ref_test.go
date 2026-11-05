// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// The functions below are the buffer-based sighash code from before the
// preimage was streamed into sha256, with the script unparsing and the
// transaction serialization it used.  The tests check the streamed code
// against them.  They only use encoding/binary and bytes for the encoding, so
// they don't change with the code they are compared to.

// refWriteVarInt writes val to b as a bitcoin variable length integer.
func refWriteVarInt(b *bytes.Buffer, val uint64) {
	var buf [8]byte
	switch {
	case val < 0xfd:
		b.WriteByte(uint8(val))
	case val <= 0xffff:
		b.WriteByte(0xfd)
		binary.LittleEndian.PutUint16(buf[:], uint16(val))
		b.Write(buf[:2])
	case val <= 0xffffffff:
		b.WriteByte(0xfe)
		binary.LittleEndian.PutUint32(buf[:], uint32(val))
		b.Write(buf[:4])
	default:
		b.WriteByte(0xff)
		binary.LittleEndian.PutUint64(buf[:], val)
		b.Write(buf[:])
	}
}

// refTokenDataBuffer returns the serialized token prefix.
func refTokenDataBuffer(tokenData *wire.TokenData) bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte(0xef)
	buf.Write(tokenData.CategoryID[:])
	buf.WriteByte(tokenData.BitField)
	if tokenData.HasCommitmentLength() {
		refWriteVarInt(&buf, uint64(len(tokenData.Commitment)))
		buf.Write(tokenData.Commitment[:])
	}
	if tokenData.HasAmount() {
		refWriteVarInt(&buf, tokenData.Amount)
	}
	return buf
}

// refWriteTxOut writes the output to b as WriteTxOut serializes it.
func refWriteTxOut(b *bytes.Buffer, to *wire.TxOut) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(to.Value))
	b.Write(buf[:])
	script := to.PkScript
	if !to.TokenData.IsEmpty() {
		tb := refTokenDataBuffer(&to.TokenData)
		tb.Write(to.PkScript)
		script = tb.Bytes()
	}
	refWriteVarInt(b, uint64(len(script)))
	b.Write(script)
}

// refSerializeTx writes the transaction to b as Serialize does.
func refSerializeTx(b *bytes.Buffer, msg *wire.MsgTx) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(msg.Version))
	b.Write(buf[:])
	refWriteVarInt(b, uint64(len(msg.TxIn)))
	for _, ti := range msg.TxIn {
		b.Write(ti.PreviousOutPoint.Hash[:])
		binary.LittleEndian.PutUint32(buf[:], ti.PreviousOutPoint.Index)
		b.Write(buf[:])
		refWriteVarInt(b, uint64(len(ti.SignatureScript)))
		b.Write(ti.SignatureScript)
		binary.LittleEndian.PutUint32(buf[:], ti.Sequence)
		b.Write(buf[:])
	}
	refWriteVarInt(b, uint64(len(msg.TxOut)))
	for _, to := range msg.TxOut {
		refWriteTxOut(b, to)
	}
	binary.LittleEndian.PutUint32(buf[:], msg.LockTime)
	b.Write(buf[:])
}

// refOpcodeBytes returns any data associated with the opcode encoded as it
// would be in a script.
func refOpcodeBytes(pop *parsedOpcode) ([]byte, error) {
	var retbytes []byte
	if pop.opcode.length > 0 {
		retbytes = make([]byte, 1, pop.opcode.length)
	} else {
		retbytes = make([]byte, 1, 1+len(pop.data)-
			pop.opcode.length)
	}

	retbytes[0] = pop.opcode.value
	if pop.opcode.length == 1 {
		if len(pop.data) != 0 {
			str := fmt.Sprintf("internal consistency error - "+
				"parsed opcode %s has data length %d when %d "+
				"was expected", pop.opcode.name, len(pop.data),
				0)
			return nil, scriptError(ErrInternal, str)
		}
		return retbytes, nil
	}
	nbytes := pop.opcode.length
	if pop.opcode.length < 0 {
		l := len(pop.data)
		switch pop.opcode.length {
		case -1:
			retbytes = append(retbytes, byte(l))
			nbytes = int(retbytes[1]) + len(retbytes)
		case -2:
			retbytes = append(retbytes, byte(l&0xff),
				byte(l>>8&0xff))
			nbytes = int(binary.LittleEndian.Uint16(retbytes[1:])) +
				len(retbytes)
		case -4:
			retbytes = append(retbytes, byte(l&0xff),
				byte((l>>8)&0xff), byte((l>>16)&0xff),
				byte((l>>24)&0xff))
			nbytes = int(binary.LittleEndian.Uint32(retbytes[1:])) +
				len(retbytes)
		}
	}

	retbytes = append(retbytes, pop.data...)

	if len(retbytes) != nbytes {
		str := fmt.Sprintf("internal consistency error - "+
			"parsed opcode %s has data length %d when %d was "+
			"expected", pop.opcode.name, len(retbytes), nbytes)
		return nil, scriptError(ErrInternal, str)
	}

	return retbytes, nil
}

// refUnparseScript returns the parsed opcodes as a script.
func refUnparseScript(pops []parsedOpcode) ([]byte, error) {
	script := make([]byte, 0, len(pops))
	for i := range pops {
		b, err := refOpcodeBytes(&pops[i])
		if err != nil {
			return nil, err
		}
		script = append(script, b...)
	}
	return script, nil
}

// refCalcHashPrevOuts calculates a single hash of all the previous outputs
// (txid:index) referenced within the passed transaction. This calculated hash
// can be re-used when validating all inputs spending outputs, with a signature
// hash type of SigHashAll. This allows validation to re-use previous hashing
// computation, reducing the complexity of validating SigHashAll inputs from
// O(N^2) to O(N).
func refCalcHashPrevOuts(tx *wire.MsgTx) chainhash.Hash {
	var b bytes.Buffer
	for _, in := range tx.TxIn {
		// First write out the 32-byte transaction ID one of whose
		// outputs are being referenced by this input.
		b.Write(in.PreviousOutPoint.Hash[:])

		// Next, we'll encode the index of the referenced output as a
		// little endian integer.
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], in.PreviousOutPoint.Index)
		b.Write(buf[:])
	}

	return chainhash.DoubleHashH(b.Bytes())
}

func refCalcHashUtxos(tx *wire.MsgTx, utxoCache *UtxoCache) chainhash.Hash {
	var b bytes.Buffer

	for i := range tx.TxIn {
		if utxoCache != nil {
			utxo, err := utxoCache.GetEntry(i)
			if err == nil {
				refWriteTxOut(&b, &utxo)
			} else {
				log.Debugf("%v", err)
			}
		} else {
			log.Debugf("utxoCache is nil.") // It might not be a bad idea to fail the function at this point.
		}
	}

	if b.Len() > 0 {
		return chainhash.DoubleHashH(b.Bytes())
	}
	return chainhash.Hash{}
}

// refCalcHashSequence computes an aggregated hash of each of the sequence numbers
// within the inputs of the passed transaction. This single hash can be re-used
// when validating all inputs spending outputs, which include signatures using
// the SigHashAll sighash type. This allows validation to re-use previous hashing
// computation, reducing the complexity of validating SigHashAll inputs from
// O(N^2) to O(N).
func refCalcHashSequence(tx *wire.MsgTx) chainhash.Hash {
	var b bytes.Buffer
	for _, in := range tx.TxIn {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], in.Sequence)
		b.Write(buf[:])
	}

	return chainhash.DoubleHashH(b.Bytes())
}

// refCalcHashOutputs computes a hash digest of all outputs created by the
// transaction encoded using the wire format. This single hash can be re-used
// when validating all inputs spending witness programs, which include
// signatures using the SigHashAll sighash type. This allows computation to be
// cached, reducing the total hashing complexity from O(N^2) to O(N).
func refCalcHashOutputs(tx *wire.MsgTx) chainhash.Hash {
	var b bytes.Buffer
	for _, out := range tx.TxOut {
		refWriteTxOut(&b, out)
	}

	return chainhash.DoubleHashH(b.Bytes())
}

// refShallowCopyTx creates a shallow copy of the transaction for use when
// calculating the signature hash.  It is used over the Copy method on the
// transaction itself since that is a deep copy and therefore does more work and
// allocates much more space than needed.
func refShallowCopyTx(tx *wire.MsgTx) wire.MsgTx {
	// As an additional memory optimization, use contiguous backing arrays
	// for the copied inputs and outputs and point the final slice of
	// pointers into the contiguous arrays.  This avoids a lot of small
	// allocations.
	txCopy := wire.MsgTx{
		Version:  tx.Version,
		TxIn:     make([]*wire.TxIn, len(tx.TxIn)),
		TxOut:    make([]*wire.TxOut, len(tx.TxOut)),
		LockTime: tx.LockTime,
	}
	txIns := make([]wire.TxIn, len(tx.TxIn))
	for i, oldTxIn := range tx.TxIn {
		txIns[i] = *oldTxIn
		txCopy.TxIn[i] = &txIns[i]
	}
	txOuts := make([]wire.TxOut, len(tx.TxOut))
	for i, oldTxOut := range tx.TxOut {
		txOuts[i] = *oldTxOut
		txCopy.TxOut[i] = &txOuts[i]
	}
	return txCopy
}

// refCalcLegacySignatureHash will, given a script and hash type for the current script
// engine instance, calculate the signature hash to be used for signing and
// verification.
func refCalcLegacySignatureHash(script []parsedOpcode, hashType SigHashType, tx *wire.MsgTx, idx int) ([]byte, int, error) {
	// This value is needed to calculate hash digest iterations after may 2025 upgrade.
	totalBytesHashedlength := 0

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx < 0 || idx > len(tx.TxIn)-1 {
		return nil, totalBytesHashedlength, fmt.Errorf("idx %d but %d txins", idx, len(tx.TxIn))
	}

	// The SigHashSingle signature type signs only the corresponding input
	// and output (the output with the same index number as the input).
	//
	// Since transactions can have more inputs than outputs, this means it
	// is improper to use SigHashSingle on input indices that don't have a
	// corresponding output.
	//
	// A bug in the original Satoshi client implementation means specifying
	// an index that is out of range results in a signature hash of 1 (as a
	// uint256 little endian).  The original intent appeared to be to
	// indicate failure, but unfortunately, it was never checked and thus is
	// treated as the actual signature hash.  This buggy behavior is now
	// part of the consensus and a hard fork would be required to fix it.
	//
	// Due to this, care must be taken by software that creates transactions
	// which make use of SigHashSingle because it can lead to an extremely
	// dangerous situation where the invalid inputs will end up signing a
	// hash of 1.  This in turn presents an opportunity for attackers to
	// cleverly construct transactions which can steal those coins provided
	// they can reuse signatures.
	if hashType&sigHashMask == SigHashSingle && idx >= len(tx.TxOut) {
		var hash chainhash.Hash
		hash[0] = 0x01
		return hash[:], totalBytesHashedlength, nil
	}

	// Remove all instances of OP_CODESEPARATOR from the script.
	script = removeOpcode(script, OP_CODESEPARATOR)

	// Make a shallow copy of the transaction, zeroing out the script for
	// all inputs that are not currently being processed.
	txCopy := refShallowCopyTx(tx)
	for i := range txCopy.TxIn {
		if i == idx {
			// UnparseScript cannot fail here because removeOpcode
			// above only returns a valid script.
			sigScript, _ := refUnparseScript(script)
			txCopy.TxIn[idx].SignatureScript = sigScript
		} else {
			txCopy.TxIn[i].SignatureScript = nil
		}
	}

	switch hashType & sigHashMask {
	case SigHashNone:
		txCopy.TxOut = txCopy.TxOut[0:0] // Empty slice.
		for i := range txCopy.TxIn {
			if i != idx {
				txCopy.TxIn[i].Sequence = 0
			}
		}

	case SigHashSingle:
		// Resize output array to up to and including requested index.
		txCopy.TxOut = txCopy.TxOut[:idx+1]

		// All but current output get zeroed out.
		for i := range idx {
			txCopy.TxOut[i].Value = -1
			txCopy.TxOut[i].PkScript = nil
		}

		// Sequence on all other inputs is 0, too.
		for i := range txCopy.TxIn {
			if i != idx {
				txCopy.TxIn[i].Sequence = 0
			}
		}

	default:
		// Consensus treats undefined hashtypes like normal SigHashAll
		// for purposes of hash generation.
		fallthrough
	case SigHashOld:
		fallthrough
	case SigHashAll:
		// Nothing special here.
	}
	if hashType&SigHashAnyOneCanPay != 0 {
		txCopy.TxIn = txCopy.TxIn[idx : idx+1]
	}

	// The final hash is the double sha256 of both the serialized modified
	// transaction and the hash type (encoded as a 4-byte little-endian
	// value) appended.
	wbuf := new(bytes.Buffer)
	refSerializeTx(wbuf, &txCopy)
	_ = binary.Write(wbuf, binary.LittleEndian, hashType)

	totalBytesHashedlength += wbuf.Len()

	return chainhash.DoubleHashB(wbuf.Bytes()), totalBytesHashedlength, nil
}

// refCalcBip143SignatureHash computes the sighash digest of a transaction's
// input using the new, optimized digest calculation algorithm defined
// in BIP0143: https://github.com/bitcoin/bips/blob/master/bip-0143.mediawiki.
// This function makes use of pre-calculated sighash fragments stored within
// the passed HashCache to eliminate duplicate hashing computations when
// calculating the final digest, reducing the complexity from O(N^2) to O(N).
// Additionally, signatures now cover the input value of the referenced unspent
// output. This allows offline, or hardware wallets to compute the exact amount
// being spent, in addition to the final transaction fee. In the case the
// wallet if fed an invalid input amount, the real sighash will differ causing
// the produced signature to be invalid.
func refCalcBip143SignatureHash(subScript []parsedOpcode, sigHashes *TxSigHashes,
	hashType SigHashType, tx *wire.MsgTx, idx int, amt int64, scriptAllowCashTokens bool) ([]byte, int, error) {

	// This value is needed to calculate hash digest iterations after may 2025 upgrade.
	totalBytesHashedlength := 0

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx < 0 || idx > len(tx.TxIn)-1 {
		return nil, totalBytesHashedlength, fmt.Errorf("idx %d but %d txins", idx, len(tx.TxIn))
	}

	// Sighashes with the spent outputs hold the token data of the output
	// each input spends, so they must be for as many inputs as tx has.
	if sigHashes.hasUtxos && len(sigHashes.tokenDataList) != len(tx.TxIn) {
		return nil, totalBytesHashedlength, fmt.Errorf("sighashes for %d "+
			"inputs but %d txins", len(sigHashes.tokenDataList), len(tx.TxIn))
	}

	// We'll utilize this buffer throughout to incrementally calculate
	// the signature hash for this transaction.
	var sigHash bytes.Buffer

	// First write out, then encode the transaction's version number.
	var bVersion [4]byte
	binary.LittleEndian.PutUint32(bVersion[:], uint32(tx.Version))
	sigHash.Write(bVersion[:])

	// Next write out the possibly pre-calculated hashes for the sequence
	// numbers of all inputs, and the hashes of the previous outs for all
	// outputs.
	var zeroHash chainhash.Hash

	// If anyone can pay isn't active, then we can use the cached
	// hashPrevOuts, otherwise we just write zeroes for the prev outs.
	if hashType&SigHashAnyOneCanPay == 0 {
		sigHash.Write(sigHashes.HashPrevOuts[:])
	} else {
		sigHash.Write(zeroHash[:])
	}

	// add CashTokens data here hashUtxos is a 32-byte double SHA256
	// of the serialization of all UTXOs spent by the transaction's inputs,
	// concatenated in input order, excluding output count.
	if scriptAllowCashTokens && hashType&SigHashUTXO > 0 {
		sigHash.Write(sigHashes.HashUTXOS[:])
	}

	// If the sighash isn't anyone can pay, single, or none, the use the
	// cached hash sequences, otherwise write all zeroes for the
	// hashSequence.
	if hashType&SigHashAnyOneCanPay == 0 &&
		hashType&sigHashMask != SigHashSingle &&
		hashType&sigHashMask != SigHashNone {
		sigHash.Write(sigHashes.HashSequence[:])
	} else {
		sigHash.Write(zeroHash[:])
	}

	// Next, write the outpoint being spent.
	sigHash.Write(tx.TxIn[idx].PreviousOutPoint.Hash[:])
	var bIndex [4]byte
	binary.LittleEndian.PutUint32(bIndex[:], tx.TxIn[idx].PreviousOutPoint.Index)
	sigHash.Write(bIndex[:])

	if len(sigHashes.tokenDataList) > 0 && len(sigHashes.tokenDataList[idx]) > 0 {
		sigHash.Write(sigHashes.tokenDataList[idx])
	}

	scriptBytes, _ := refUnparseScript(subScript)

	refWriteVarInt(&sigHash, uint64(len(scriptBytes)))
	sigHash.Write(scriptBytes)

	// Next, add the input amount, and sequence number of the input being
	// signed.
	var bAmount [8]byte
	binary.LittleEndian.PutUint64(bAmount[:], uint64(amt))
	sigHash.Write(bAmount[:])
	var bSequence [4]byte
	binary.LittleEndian.PutUint32(bSequence[:], tx.TxIn[idx].Sequence)
	sigHash.Write(bSequence[:])

	// If the current signature mode isn't single, or none, then we can
	// re-use the pre-generated hashoutputs sighash fragment. Otherwise,
	// we'll serialize and add only the target output index to the signature
	// pre-image.
	if hashType&sigHashMask != SigHashSingle &&
		hashType&sigHashMask != SigHashNone {
		sigHash.Write(sigHashes.HashOutputs[:])
	} else if hashType&sigHashMask == SigHashSingle && idx < len(tx.TxOut) {
		var b bytes.Buffer
		refWriteTxOut(&b, tx.TxOut[idx])
		sigHash.Write(chainhash.DoubleHashB(b.Bytes()))
	} else {
		sigHash.Write(zeroHash[:])
	}

	// Finally, write out the transaction's locktime, and the sig hash
	// type.
	var bLockTime [4]byte
	binary.LittleEndian.PutUint32(bLockTime[:], tx.LockTime)
	sigHash.Write(bLockTime[:])
	var bHashType [4]byte
	binary.LittleEndian.PutUint32(bHashType[:], uint32(hashType))
	sigHash.Write(bHashType[:])

	totalBytesHashedlength += sigHash.Len()

	return chainhash.DoubleHashB(sigHash.Bytes()), totalBytesHashedlength, nil
}
