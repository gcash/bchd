// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"fmt"
	"math"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

const (
	// maxStandardSigScriptSize is the maximum size allowed for a
	// transaction input signature script to be considered standard.  This
	// value allows for a 15-of-15 CHECKMULTISIG pay-to-script-hash with
	// compressed keys.
	//
	// The form of the overall script is: OP_0 <15 signatures> OP_PUSHDATA2
	// <2 bytes len> [OP_15 <15 pubkeys> OP_15 OP_CHECKMULTISIG]
	//
	// For the p2sh script portion, each of the 15 compressed pubkeys are
	// 33 bytes (plus one for the OP_DATA_33 opcode), and the thus it totals
	// to (15*34)+3 = 513 bytes.  Next, each of the 15 signatures is a max
	// of 73 bytes (plus one for the OP_DATA_73 opcode).  Also, there is one
	// extra byte for the initial extra OP_0 push and 3 bytes for the
	// OP_PUSHDATA2 needed to specify the 513 bytes for the script push.
	// That brings the total to 1+(15*74)+3+513 = 1627.  This value also
	// adds a few extra bytes to provide a little buffer.
	// (1 + 15*74 + 3) + (15*34 + 3) + 23 = 1650
	maxStandardSigScriptSize = 1650

	// DefaultMinRelayTxFee is the minimum fee rate a transaction must pay
	// to be relayed and mined.  This value is in Satoshi/1000 bytes.
	DefaultMinRelayTxFee = bchutil.Amount(1000)

	// DefaultDustRelayFee is the fee rate, in Satoshi/1000 bytes, that
	// outputs are judged dust by: BCHN's default -dustrelayfee.  It does
	// not follow the minimum relay fee, so raising that does not refuse
	// outputs BCHN relays.
	DefaultDustRelayFee = bchutil.Amount(1000)

	// maxStandardMultiSigKeys is the maximum number of public keys allowed
	// in a multi-signature transaction output script for it to be
	// considered standard.
	maxStandardMultiSigKeys = 3

	// maxStandardTxSize is the maximum size of a transaction
	maxStandardTxSize = 100000

	// maxStandardP2SScriptSize is the maximum length of currently
	// standard bare multi-signature (BMS) outputs.
	maxStandardP2SScriptSize = 201
)

// calcMinRequiredTxRelayFee returns the minimum transaction fee required for a
// transaction with the passed serialized size to be accepted into the memory
// pool and relayed.
func calcMinRequiredTxRelayFee(serializedSize int64, minRelayTxFee bchutil.Amount) int64 {
	// Calculate the minimum fee for a transaction to be allowed into the
	// mempool and relayed by scaling the base fee (which is the minimum
	// relay fee rate).  minRelayTxFee is in Satoshi/kB so
	// multiply by serializedSize (which is in bytes) and divide by 1000 to
	// get minimum Satoshis.  A product that overflows is far above the
	// valid range.
	if minRelayTxFee > 0 && serializedSize > math.MaxInt64/int64(minRelayTxFee) {
		return bchutil.MaxSatoshi
	}
	minFee := (serializedSize * int64(minRelayTxFee)) / 1000

	// A fee that rounds down to zero at a nonzero rate is one satoshi, as
	// BCHN's GetFee makes it.
	if minFee == 0 && minRelayTxFee > 0 {
		minFee = 1
	}

	// Set the minimum fee to the maximum possible value if the calculated
	// fee is not in the valid range for monetary amounts.
	if minFee < 0 || minFee > bchutil.MaxSatoshi {
		minFee = bchutil.MaxSatoshi
	}

	return minFee
}

// checkInputsStandard performs a series of checks on a transaction's inputs
// to ensure they are "standard".  A standard transaction input within the
// context of this function is one whose referenced public key script is of a
// standard form. However, it should also be noted that standard inputs also are
// those which have a clean stack after execution and only contain pushed data
// in their signature scripts.  This function does not perform those checks
// because the script engine already does this more accurately and concisely
// via the txscript.ScriptVerifyCleanStack and txscript.ScriptVerifySigPushOnly
// flags.
func checkInputsStandard(tx *bchutil.Tx, utxoView *blockchain.UtxoViewpoint, scriptFlags txscript.ScriptFlags) error {
	// NOTE: The reference implementation also does a coinbase check here,
	// but coinbases have already been rejected prior to calling this
	// function so no need to recheck.

	for i, txIn := range tx.MsgTx().TxIn {
		// It is safe to elide existence and index checks here since
		// they have already been checked prior to calling this
		// function.
		entry := utxoView.LookupEntry(txIn.PreviousOutPoint)
		originPkScript := entry.PkScript()
		switch txscript.GetScriptClass(originPkScript) {
		case txscript.ScriptHash32Ty:
			// P2SH32 was enabled together with tokens.
			if !scriptFlags.HasFlag(txscript.ScriptAllowCashTokens) {
				str := fmt.Sprintf("transaction input #%d has a "+
					"non-standard script form", i)
				return txRuleError(wire.RejectNonstandard, str)
			}
		case txscript.NonStandardTy:
			// After the May 2026 upgrade, any script that does not match any of the
			// P2PK, P2PKH, P2SH20, P2SH32, OP_RETURN, or BMS types, is considered
			// standard if it has a length less or equal to maxStandardP2SScriptSize.
			if !scriptFlags.HasFlag(txscript.ScriptAllowMay2026) || len(originPkScript) > maxStandardP2SScriptSize {
				str := fmt.Sprintf("transaction input #%d has a "+
					"non-standard script form", i)
				return txRuleError(wire.RejectNonstandard, str)
			}
		}
	}

	return nil
}

// checkPkScriptStandard performs a series of checks on a transaction output
// script (public key script) to ensure it is a "standard" public key script.
// A standard public key script is one that is a recognized form, and for
// multi-signature scripts, only contains from 1 to maxStandardMultiSigKeys
// public keys.
func checkPkScriptStandard(pkScript []byte, scriptClass txscript.ScriptClass, upgrade12Active bool) error {
	switch scriptClass {
	case txscript.MultiSigTy:
		numPubKeys, numSigs, err := txscript.CalcMultiSigStats(pkScript)
		if err != nil {
			str := fmt.Sprintf("multi-signature script parse "+
				"failure: %v", err)
			return txRuleError(wire.RejectNonstandard, str)
		}

		// A standard multi-signature public key script must contain
		// from 1 to maxStandardMultiSigKeys public keys.
		if numPubKeys < 1 {
			str := "multi-signature script with no pubkeys"
			return txRuleError(wire.RejectNonstandard, str)
		}
		if numPubKeys > maxStandardMultiSigKeys {
			str := fmt.Sprintf("multi-signature script with %d "+
				"public keys which is more than the allowed "+
				"max of %d", numPubKeys, maxStandardMultiSigKeys)
			return txRuleError(wire.RejectNonstandard, str)
		}

		// A standard multi-signature public key script must have at
		// least 1 signature and no more signatures than available
		// public keys.
		if numSigs < 1 {
			return txRuleError(wire.RejectNonstandard,
				"multi-signature script with no signatures")
		}
		if numSigs > numPubKeys {
			str := fmt.Sprintf("multi-signature script with %d "+
				"signatures which is more than the available "+
				"%d public keys", numSigs, numPubKeys)
			return txRuleError(wire.RejectNonstandard, str)
		}

	case txscript.NonStandardTy:
		// After the May 2026 upgrade, any script that does not match any of the
		// P2PK, P2PKH, P2SH20, P2SH32, OP_RETURN, or BMS types, is considered
		// standard if it has a length less or equal to maxStandardP2SScriptSize.
		if !upgrade12Active || len(pkScript) > maxStandardP2SScriptSize {
			return txRuleError(wire.RejectNonstandard,
				"non-standard script form")
		}
	}

	return nil
}

// isDust returns whether or not the passed transaction output amount is
// considered dust or not based on the passed dust relay fee.  In particular,
// if the cost to the network to spend coins is more than 1/3 of the dust
// relay fee, it is considered dust.
func isDust(txOut *wire.TxOut, dustRelayFee bchutil.Amount) bool {
	// The total serialized size consists of the output and the associated
	// input script to redeem it.  Since there is no input script
	// to redeem it yet, use the minimum size of a typical input script.
	//
	// Pay-to-pubkey-hash bytes breakdown:
	//
	//  Output to hash (34 bytes):
	//   8 value, 1 script len, 25 script [1 OP_DUP, 1 OP_HASH_160,
	//   1 OP_DATA_20, 20 hash, 1 OP_EQUALVERIFY, 1 OP_CHECKSIG]
	//
	//  Input with compressed pubkey (148 bytes):
	//   36 prev outpoint, 1 script len, 107 script [1 OP_DATA_72, 72 sig,
	//   1 OP_DATA_33, 33 compressed pubkey], 4 sequence
	//
	//  Input with uncompressed pubkey (180 bytes):
	//   36 prev outpoint, 1 script len, 139 script [1 OP_DATA_72, 72 sig,
	//   1 OP_DATA_65, 65 compressed pubkey], 4 sequence
	//
	// Pay-to-pubkey bytes breakdown:
	//
	//  Output to compressed pubkey (44 bytes):
	//   8 value, 1 script len, 35 script [1 OP_DATA_33,
	//   33 compressed pubkey, 1 OP_CHECKSIG]
	//
	//  Output to uncompressed pubkey (76 bytes):
	//   8 value, 1 script len, 67 script [1 OP_DATA_65, 65 pubkey,
	//   1 OP_CHECKSIG]
	//
	//  Input (114 bytes):
	//   36 prev outpoint, 1 script len, 73 script [1 OP_DATA_72,
	//   72 sig], 4 sequence
	//
	//
	// Theoretically this could examine the script type of the output script
	// and use a different size for the typical input script size for
	// pay-to-pubkey vs pay-to-pubkey-hash inputs per the above breakdowns,
	// but the only combination which is less than the value chosen is
	// a pay-to-pubkey script with a compressed pubkey, which is not very
	// common.
	//
	// The most common scripts are pay-to-pubkey-hash, and as per the above
	// breakdown, the minimum size of a p2pkh input script is 148 bytes.  So
	// that figure is used.
	//
	// Both cases share a 41 byte preamble required to reference the input
	// being spent and the sequence number of the input.
	totalSize := txOut.SerializeSize() + 41 + 107

	// The output is considered dust if the cost to the network to spend the
	// coins is more than 1/3 of the dust relay fee.  dustRelayFee is in
	// Satoshi/KB, so multiply by 1000 to convert to bytes.
	//
	// Using the typical values for a pay-to-pubkey-hash transaction from
	// the breakdown above and the default dust relay fee of 1000, this
	// equates to values less than 546 satoshi being considered dust.
	//
	// The following is equivalent to (value/totalSize) * (1/3) * 1000
	// without needing to do floating point math.
	return txOut.Value*1000/(3*int64(totalSize)) < int64(dustRelayFee)
}

// checkTransactionStandard performs a series of checks on a transaction to
// ensure it is a "standard" transaction.  A standard transaction is one that
// conforms to several additional limiting cases over what is considered a
// "sane" transaction such as having a version in the supported range, being
// finalized, conforming to more stringent size constraints, having scripts
// of recognized forms, and not containing "dust" outputs (those that are
// so small it costs more to process them than they are worth, judged at
// dustRelayFee).
func checkTransactionStandard(tx *bchutil.Tx, height int32,
	medianTimePast time.Time, dustRelayFee bchutil.Amount,
	maxTxVersion int32, upgrade9Active bool, upgrade12Active bool) error {

	// The transaction must be a currently supported version.
	msgTx := tx.MsgTx()
	if msgTx.Version > maxTxVersion || msgTx.Version < 1 {
		str := fmt.Sprintf("transaction version %d is not in the "+
			"valid range of %d-%d", msgTx.Version, 1,
			maxTxVersion)
		return txRuleError(wire.RejectNonstandard, str)
	}

	// The transaction must be finalized to be standard and therefore
	// considered for inclusion in a block.
	if !blockchain.IsFinalizedTransaction(tx, height, medianTimePast) {
		return txRuleError(wire.RejectNonstandard,
			"transaction is not finalized (bad-txns-nonfinal)")
	}

	// Since extremely large transactions with a lot of inputs can cost
	// almost as much to process as the sender fees, limit the maximum
	// size of a transaction.  This also helps mitigate CPU exhaustion
	// attacks.
	txSize := tx.MsgTx().SerializeSize()
	if txSize > maxStandardTxSize {
		str := fmt.Sprintf("size of transaction %v is larger than max "+
			"allowed size of %v", txSize, maxStandardTxSize)
		return txRuleError(wire.RejectNonstandard, str)
	}

	for i, txIn := range msgTx.TxIn {
		// Each transaction input signature script must not exceed the
		// maximum size allowed for a standard transaction.  See
		// the comment on maxStandardSigScriptSize for more details.
		sigScriptLen := len(txIn.SignatureScript)
		maxStandardSizeAllowed := maxStandardSigScriptSize
		if upgrade12Active {
			// After the May 2026 upgrade, the limit on maximum standard
			// input bytecode length (A.K.A. MAX_TX_IN_SCRIPT_SIG_SIZE –
			// 1,650 bytes) is removed such that the maximum unlocking
			// bytecode length is equal for both standard and consensus
			// validation: 10,000 bytes (A.K.A. MAX_SCRIPT_SIZE).
			maxStandardSizeAllowed = txscript.MaxScriptSize
		}
		if sigScriptLen > maxStandardSizeAllowed {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script size of %d bytes is large than max "+
				"allowed size of %d bytes", i, sigScriptLen,
				maxStandardSizeAllowed)
			return txRuleError(wire.RejectNonstandard, str)
		}

		// Each transaction input signature script must only contain
		// opcodes which push data onto the stack.
		if !txscript.IsPushOnlyScript(txIn.SignatureScript) {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script is not push only", i)
			return txRuleError(wire.RejectNonstandard, str)
		}
	}

	// None of the output public key scripts can be a non-standard script or
	// be "dust" (except when the script is a null data script).
	dataCarrierSize := 0
	for i, txOut := range msgTx.TxOut {

		if !upgrade9Active && !txOut.TokenData.IsEmpty() {
			rejectCode := wire.RejectNonstandard
			str := "txn-tokens-before-activation"
			return txRuleError(rejectCode, str)
		}

		scriptClass := txscript.GetScriptClass(txOut.PkScript)
		err := checkPkScriptStandard(txOut.PkScript, scriptClass, upgrade12Active)
		if err != nil {
			// Attempt to extract a reject code from the error so
			// it can be retained.  When not possible, fall back to
			// a non standard error.
			rejectCode := wire.RejectNonstandard
			if rejCode, found := extractRejectCode(err); found {
				rejectCode = rejCode
			}
			str := fmt.Sprintf("transaction output %d: %v", i, err)
			return txRuleError(rejectCode, str)
		}

		// Accumulate the script size of outputs which only carry data.  For
		// all other script types, ensure the output value is not
		// "dust".  An unspendable output, such as a pay to script output
		// starting with OP_RETURN, is never dust, as in BCHN.
		if scriptClass == txscript.NullDataTy {
			dataCarrierSize += len(txOut.PkScript)
		} else if !txscript.IsUnspendable(txOut.PkScript) && isDust(txOut, dustRelayFee) {
			str := fmt.Sprintf("transaction output %d: payment "+
				"of %d is dust", i, txOut.Value)
			return txRuleError(wire.RejectDust, str)
		}
	}

	// A standard transaction cannot have null data exceeding txscript.MaxDataCarrierSize
	if dataCarrierSize > txscript.MaxDataCarrierSize {
		str := fmt.Sprintf("transaction nulldata exceeds %d bytes", txscript.MaxDataCarrierSize)
		return txRuleError(wire.RejectNonstandard, str)
	}

	return nil
}

func CheckTransactionStandard(tx *bchutil.Tx, height int32, medianTimePast time.Time, dustRelayFee bchutil.Amount,
	maxTxVersion int32, upgrade9Active bool, upgrade12Active bool) error {
	return checkTransactionStandard(tx, height, medianTimePast, dustRelayFee, maxTxVersion, upgrade9Active, upgrade12Active)
}

func CheckInputsStandard(tx *bchutil.Tx, utxoView *blockchain.UtxoViewpoint, scriptFlags txscript.ScriptFlags) error {
	return checkInputsStandard(tx, utxoView, scriptFlags)
}
