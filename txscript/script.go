// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// Bip16Activation is the timestamp where BIP0016 is valid to use in the
// blockchain.  To be used to determine if BIP0016 should be called for or not.
// This timestamp corresponds to Sun Apr 1 00:00:00 UTC 2012.
var Bip16Activation = time.Unix(1333238400, 0)

// SigHashType represents hash type bits at the end of a signature.
type SigHashType uint32

// Hash type bits from the end of a signature.
const (
	SigHashOld          SigHashType = 0x0
	SigHashAll          SigHashType = 0x1
	SigHashNone         SigHashType = 0x2
	SigHashSingle       SigHashType = 0x3
	SigHashAnyOneCanPay SigHashType = 0x80

	// SigHashForkID is the replay protected signature hash flag
	// used by the Uahf hardfork.
	SigHashForkID SigHashType = 0x40

	SigHashUTXO SigHashType = 0x20

	// sigHashMask defines the number of bits of the hash type which is used
	// to identify which outputs are signed.
	sigHashMask = 0x1f
)

// These are the constants specified for maximums in individual scripts.
const (
	MaxOpsPerScript            = 201                           // Max number of non-push operations.
	MaxPubKeysPerMultiSig      = 20                            // Multisig can't have more sigs than this.
	MaxScriptElementSizeLegacy = 520                           // Max bytes pushable to the stack.
	MaxScriptElementSize       = MaxScriptElementSizeUpgrade11 // Max bytes pushable to the stack after upgrade 11.
)

// isSmallInt returns whether or not the opcode is considered a small integer,
// which is an OP_0, or OP_1 through OP_16.
func isSmallInt(op *opcode) bool {
	if op.value == OP_0 || (op.value >= OP_1 && op.value <= OP_16) {
		return true
	}
	return false
}

// isScriptHash returns true if the script passed is a pay-to-script-hash
// transaction, false otherwise.
func isScriptHash(pops []parsedOpcode) bool {
	return len(pops) == 3 &&
		pops[0].opcode.value == OP_HASH160 &&
		pops[1].opcode.value == OP_DATA_20 &&
		pops[2].opcode.value == OP_EQUAL
}

// isScriptHash32 returns true if the script passed is a pay-to-script-hash-32
// transaction, false otherwise.
func isScriptHash32(pops []parsedOpcode) bool {
	return len(pops) == 3 &&
		pops[0].opcode.value == OP_HASH256 &&
		pops[1].opcode.value == OP_DATA_32 &&
		pops[2].opcode.value == OP_EQUAL
}

// IsPayToScriptHash returns true if the script is in the standard
// pay-to-script-hash (P2SH) format, false otherwise.
func IsPayToScriptHash(script []byte) bool {
	pops, err := parseScript(script)
	if err != nil {
		return false
	}
	return isScriptHash(pops)
}

// IsPayToScriptHash32 returns true if the script is in the standard
// pay-to-script-hash-32 (P2SH) format, false otherwise.
func IsPayToScriptHash32(script []byte) bool {
	pops, err := parseScript(script)
	if err != nil {
		return false
	}
	return isScriptHash32(pops)
}

// isPushOnly returns true if the script only pushes data, false otherwise.
func isPushOnly(pops []parsedOpcode) bool {
	// NOTE: This function does NOT verify opcodes directly since it is
	// internal and is only called with parsed opcodes for scripts that did
	// not have any parse errors.  Thus, consensus is properly maintained.

	for _, pop := range pops {
		// All opcodes up to OP_16 are data push instructions.
		// NOTE: This does consider OP_RESERVED to be a data push
		// instruction, but execution of OP_RESERVED will fail anyways
		// and matches the behavior required by consensus.
		if pop.opcode.value > OP_16 {
			return false
		}
	}
	return true
}

// IsPushOnlyScript returns whether or not the passed script only pushes data.
//
// False will be returned when the script does not parse.
func IsPushOnlyScript(script []byte) bool {
	pops, err := parseScript(script)
	if err != nil {
		return false
	}
	return isPushOnly(pops)
}

// ExtractDataElements returns a slice of all the data elements in the
// given script.
func ExtractDataElements(script []byte) ([][]byte, error) {
	var dataElements [][]byte
	pops, err := parseScript(script)
	if err != nil {
		return nil, err
	}
	for _, pop := range pops {
		// The only opcodes which carry data are OP_DATA_1 to OP_PUSHDATA4.
		// OP_0 and OP_1 - OP_16 are ignored for the purpose of this function
		// even though they push data to the stack.
		if pop.opcode.value > OP_0 && pop.opcode.value <= OP_PUSHDATA4 {
			dataElements = append(dataElements, pop.data)
		}
	}
	return dataElements, nil
}

// ExtractRedeemScript attempts to extract the redeem script from a signature
// script. In practice this amount to just returning the last popped element
// as we don't know for sure if it's a redeem script.
func ExtractRedeemScript(script []byte) ([]byte, error) {
	pops, err := parseScript(script)
	if err != nil {
		return nil, err
	}
	return pops[len(pops)-1].data, nil
}

// parseScriptTemplate is the same as parseScript but allows the passing of the
// template list for testing purposes.  When there are parse errors, it returns
// the list of parsed opcodes up to the point of failure along with the error.
func parseScriptTemplate(script []byte, opcodes *[256]opcode) ([]parsedOpcode, error) {
	retScript := make([]parsedOpcode, 0, len(script))
	for i := 0; i < len(script); {
		instr := script[i]
		op := &opcodes[instr]
		pop := parsedOpcode{opcode: op}

		// Parse data out of instruction.
		switch {
		// No additional data.  Note that some of the opcodes, notably
		// OP_1NEGATE, OP_0, and OP_[1-16] represent the data
		// themselves.
		case op.length == 1:
			i++

		// Data pushes of specific lengths -- OP_DATA_[1-75].
		case op.length > 1:
			if len(script[i:]) < op.length {
				str := fmt.Sprintf("opcode %s requires %d "+
					"bytes, but script only has %d remaining",
					op.name, op.length, len(script[i:]))
				return retScript, scriptError(ErrMalformedPush,
					str)
			}

			// Slice out the data.
			pop.data = script[i+1 : i+op.length]
			i += op.length

		// Data pushes with parsed lengths -- OP_PUSHDATAP{1,2,4}.
		case op.length < 0:
			var l uint
			off := i + 1

			if len(script[off:]) < -op.length {
				str := fmt.Sprintf("opcode %s requires %d "+
					"bytes, but script only has %d remaining",
					op.name, -op.length, len(script[off:]))
				return retScript, scriptError(ErrMalformedPush,
					str)
			}

			// Next -length bytes are little endian length of data.
			switch op.length {
			case -1:
				l = uint(script[off])
			case -2:
				l = ((uint(script[off+1]) << 8) |
					uint(script[off]))
			case -4:
				l = ((uint(script[off+3]) << 24) |
					(uint(script[off+2]) << 16) |
					(uint(script[off+1]) << 8) |
					uint(script[off]))
			default:
				str := fmt.Sprintf("invalid opcode length %d",
					op.length)
				return retScript, scriptError(ErrMalformedPush,
					str)
			}

			// Move offset to beginning of the data.
			off += -op.length

			// Disallow entries that do not fit script or were
			// sign extended.
			if int(l) > len(script[off:]) || int(l) < 0 {
				str := fmt.Sprintf("opcode %s pushes %d bytes, "+
					"but script only has %d remaining",
					op.name, int(l), len(script[off:]))
				return retScript, scriptError(ErrMalformedPush,
					str)
			}

			pop.data = script[off : off+int(l)]
			i += 1 - op.length + int(l)
		}

		retScript = append(retScript, pop)
	}

	return retScript, nil
}

// parseScript preparses the script in bytes into a list of parsedOpcodes while
// applying a number of sanity checks.
func parseScript(script []byte) ([]parsedOpcode, error) {
	return parseScriptTemplate(script, &opcodeArray)
}

// unparseScript reversed the action of parseScript and returns the
// parsedOpcodes as a list of bytes
func unparseScript(pops []parsedOpcode) ([]byte, error) {
	script := make([]byte, 0, len(pops))
	for _, pop := range pops {
		b, err := pop.bytes()
		if err != nil {
			return nil, err
		}
		script = append(script, b...)
	}
	return script, nil
}

// DisasmString formats a disassembled script for one line printing.  When the
// script fails to parse, the returned string will contain the disassembled
// script up to the point the failure occurred along with the string '[error]'
// appended.  In addition, the reason the script failed to parse is returned
// if the caller wants more information about the failure.
func DisasmString(buf []byte) (string, error) {
	var disbuf bytes.Buffer
	opcodes, err := parseScript(buf)
	for _, pop := range opcodes {
		disbuf.WriteString(pop.print(true))
		disbuf.WriteByte(' ')
	}
	if disbuf.Len() > 0 {
		disbuf.Truncate(disbuf.Len() - 1)
	}
	if err != nil {
		disbuf.WriteString("[error]")
	}
	return disbuf.String(), err
}

// removeOpcode will remove any opcode matching “opcode” from the opcode
// stream in pkscript
func removeOpcode(pkscript []parsedOpcode, opcode byte) []parsedOpcode {
	retScript := make([]parsedOpcode, 0, len(pkscript))
	for _, pop := range pkscript {
		if pop.opcode.value != opcode {
			retScript = append(retScript, pop)
		}
	}
	return retScript
}

// canonicalPush returns true if the object is either not a push instruction
// or the push instruction contained wherein is matches the canonical form
// or using the smallest instruction to do the job. False otherwise.
func canonicalPush(pop parsedOpcode) bool {
	opcode := pop.opcode.value
	data := pop.data
	dataLen := len(pop.data)
	if opcode > OP_16 {
		return true
	}

	if opcode < OP_PUSHDATA1 && opcode > OP_0 && (dataLen == 1 && data[0] <= 16) {
		return false
	}
	if opcode == OP_PUSHDATA1 && dataLen < OP_PUSHDATA1 {
		return false
	}
	if opcode == OP_PUSHDATA2 && dataLen <= 0xff {
		return false
	}
	if opcode == OP_PUSHDATA4 && dataLen <= 0xffff {
		return false
	}
	return true
}

// removeOpcodeByData will return the script minus any opcodes that would push
// the passed data to the stack.
func removeOpcodeByData(pkscript []parsedOpcode, data []byte) []parsedOpcode {
	retScript := make([]parsedOpcode, 0, len(pkscript))
	for _, pop := range pkscript {
		if !canonicalPush(pop) || !bytes.Contains(pop.data, data) {
			retScript = append(retScript, pop)
		}
	}
	return retScript

}

// calcHashPrevOuts calculates a single hash of all the previous outputs
// (txid:index) referenced within the passed transaction. This calculated hash
// can be re-used when validating all inputs spending outputs, with a signature
// hash type of SigHashAll. This allows validation to re-use previous hashing
// computation, reducing the complexity of validating SigHashAll inputs from
// O(N^2) to O(N).
func calcHashPrevOuts(tx *wire.MsgTx) chainhash.Hash {
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

func calcHashUtxos(tx *wire.MsgTx, utxoCache *UtxoCache) chainhash.Hash {
	var b bytes.Buffer

	for i := range tx.TxIn {
		if utxoCache != nil {
			utxo, err := utxoCache.GetEntry(i)
			if err == nil {
				wire.WriteTxOut(&b, 0, 0, &utxo)
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

func calUtxoTokenData(tx *wire.MsgTx, utxoCache *UtxoCache) [][]byte {
	tokenDataList := make([][]byte, len(tx.TxIn))
	for i := range tx.TxIn {
		if utxoCache != nil {
			utxo, err := utxoCache.GetEntry(i)
			if err == nil {
				if !utxo.TokenData.IsEmpty() {
					b := utxo.TokenData.TokenDataBuffer()
					tokenDataList[i] = b.Bytes()
				}
			} else {
				log.Debugf("%v", err)
			}
		} else {
			log.Debugf("utxoCache is nil.") // It might not be a bad idea to fail the function at this point.
		}
	}
	return tokenDataList
}

// calcHashSequence computes an aggregated hash of each of the sequence numbers
// within the inputs of the passed transaction. This single hash can be re-used
// when validating all inputs spending outputs, which include signatures using
// the SigHashAll sighash type. This allows validation to re-use previous hashing
// computation, reducing the complexity of validating SigHashAll inputs from
// O(N^2) to O(N).
func calcHashSequence(tx *wire.MsgTx) chainhash.Hash {
	var b bytes.Buffer
	for _, in := range tx.TxIn {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], in.Sequence)
		b.Write(buf[:])
	}

	return chainhash.DoubleHashH(b.Bytes())
}

// calcHashOutputs computes a hash digest of all outputs created by the
// transaction encoded using the wire format. This single hash can be re-used
// when validating all inputs spending witness programs, which include
// signatures using the SigHashAll sighash type. This allows computation to be
// cached, reducing the total hashing complexity from O(N^2) to O(N).
func calcHashOutputs(tx *wire.MsgTx) chainhash.Hash {
	var b bytes.Buffer
	for _, out := range tx.TxOut {
		wire.WriteTxOut(&b, 0, 0, out)
	}

	return chainhash.DoubleHashH(b.Bytes())
}

// CalcSignatureHash returns a signature hash which can then be signed by the
// input. Since Bitcoin Cash uses a different signature hashing algorithm
// before and after the Uahf fork, the 'useBip143SigHashAlgo' bool is used
// to specify which algorithm to use.
func CalcSignatureHash(script []byte, sigHashes *TxSigHashes, hType SigHashType,
	tx *wire.MsgTx, idx int, amt int64, useBip143SigHashAlgo bool) ([]byte, int, error) {

	parsedScript, err := parseScript(script)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot parse output script: %v", err)
	}
	return calcSignatureHash(parsedScript, sigHashes, hType, tx, idx, amt, useBip143SigHashAlgo)
}

// CalcSignatureHash will, given a script and hash type for the current script
// engine instance, calculate the signature hash to be used for signing and
// verification using the given signature hashing algorithm.
func calcSignatureHash(script []parsedOpcode, sigHashes *TxSigHashes, hType SigHashType,
	tx *wire.MsgTx, idx int, amt int64, useBip143SigHashAlgo bool) ([]byte, int, error) {
	if !useBip143SigHashAlgo {
		return calcLegacySignatureHash(script, hType, tx, idx)
	}
	return calcBip143SignatureHash(script, sigHashes, hType, tx, idx, amt, true)
}

// shallowCopyTx creates a shallow copy of the transaction for use when
// calculating the signature hash.  It is used over the Copy method on the
// transaction itself since that is a deep copy and therefore does more work and
// allocates much more space than needed.
func shallowCopyTx(tx *wire.MsgTx) wire.MsgTx {
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

// calcLegacySignatureHash will, given a script and hash type for the current script
// engine instance, calculate the signature hash to be used for signing and
// verification.
func calcLegacySignatureHash(script []parsedOpcode, hashType SigHashType, tx *wire.MsgTx, idx int) ([]byte, int, error) {
	// This value is needed to calculate hash digest iterations after may 2025 upgrade.
	totalBytesHashedlength := 0

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx > len(tx.TxIn)-1 {
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
	txCopy := shallowCopyTx(tx)
	for i := range txCopy.TxIn {
		if i == idx {
			// UnparseScript cannot fail here because removeOpcode
			// above only returns a valid script.
			sigScript, _ := unparseScript(script)
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
		for i := 0; i < idx; i++ {
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
	wbuf := bytes.NewBuffer(make([]byte, 0, txCopy.SerializeSize()+4))
	txCopy.Serialize(wbuf)
	binary.Write(wbuf, binary.LittleEndian, hashType)

	totalBytesHashedlength += wbuf.Len()

	return chainhash.DoubleHashB(wbuf.Bytes()), totalBytesHashedlength, nil
}

// calcBip143SignatureHash computes the sighash digest of a transaction's
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
func calcBip143SignatureHash(subScript []parsedOpcode, sigHashes *TxSigHashes,
	hashType SigHashType, tx *wire.MsgTx, idx int, amt int64, scriptAllowCashTokens bool) ([]byte, int, error) {

	// This value is needed to calculate hash digest iterations after may 2025 upgrade.
	totalBytesHashedlength := 0

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx > len(tx.TxIn)-1 {
		return nil, totalBytesHashedlength, fmt.Errorf("idx %d but %d txins", idx, len(tx.TxIn))
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

	scriptBytes, _ := unparseScript(subScript)

	wire.WriteVarBytes(&sigHash, 0, scriptBytes)

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
		wire.WriteTxOut(&b, 0, 0, tx.TxOut[idx])
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

// asSmallInt returns the passed opcode, which must be true according to
// isSmallInt(), as an integer.
func asSmallInt(op *opcode) int {
	if op.value == OP_0 {
		return 0
	}

	return int(op.value - (OP_1 - 1))
}

// IsUnspendable returns true if the passed public key script is provably
// unspendable. Scripts may still be otherwise unspendable due to script
// validation rules which this function intentionally does not account for
// due to compatibility with other implementations. Provably unspendable
// outputs are pruned instantly when entering the UTXO set.
func IsUnspendable(pkScript []byte) bool {
	scriptSize := len(pkScript)
	return (scriptSize > 0 && pkScript[0] == OP_RETURN) ||
		(scriptSize > MaxScriptSize)
}
