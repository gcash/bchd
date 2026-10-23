// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"fmt"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// RuleError identifies a rule violation.  It is used to indicate that
// processing of a transaction failed due to one of the many validation
// rules.  The caller can use type assertions to determine if a failure was
// specifically due to a rule violation and use the Err field to access the
// underlying error, which will be either a TxRuleError or a
// blockchain.RuleError.
type RuleError struct {
	Err error
}

// Error satisfies the error interface and prints human-readable errors.
func (e RuleError) Error() string {
	if e.Err == nil {
		return "<nil>"
	}
	return e.Err.Error()
}

// TxRuleError identifies a rule violation.  It is used to indicate that
// processing of a transaction failed due to one of the many validation
// rules.  The caller can use type assertions to determine if a failure was
// specifically due to a rule violation and access the ErrorCode field to
// ascertain the specific reason for the rule violation.
type TxRuleError struct {
	RejectCode  wire.RejectCode // The code to send with reject messages
	Reason      string          // BCHN's short reason, such as "dust"
	Description string          // Human readable description of the issue
}

// Error satisfies the error interface and prints human-readable errors.
func (e TxRuleError) Error() string {
	return e.Description
}

// HighFeeError identifies a transaction that pays more fees than the caller
// allows.
type HighFeeError struct {
	Fee    int64 // The fee paid by the transaction in satoshis
	MaxFee int64 // The maximum fee allowed in satoshis
}

// Error satisfies the error interface and prints human-readable errors.
func (e HighFeeError) Error() string {
	return fmt.Sprintf("absurdly-high-fee, %d > %d", e.Fee, e.MaxFee)
}

// txRuleError creates an underlying TxRuleError with the given a set of
// arguments and returns a RuleError that encapsulates it.
func txRuleError(c wire.RejectCode, reason, desc string) RuleError {
	return RuleError{Err: TxRuleError{
		RejectCode:  c,
		Reason:      reason,
		Description: desc,
	}}
}

// chainRuleError returns a RuleError that encapsulates the given
// blockchain.RuleError.
func chainRuleError(chainErr blockchain.RuleError) RuleError {
	return RuleError{
		Err: chainErr,
	}
}

// extractRejectCode attempts to return a relevant reject code for a given error
// by examining the error for known types.  It will return true if a code
// was successfully extracted.
func extractRejectCode(err error) (wire.RejectCode, bool) {
	// Pull the underlying error out of a RuleError, which the pool returns
	// for a transaction it refuses.  A transaction below the minimum size,
	// with a version outside the consensus range or above its sigchecks
	// limit is nonstandard there, as in BCHN's memory pool, which finds the
	// first two in its contextual checks and runs the scripts of the third
	// again without the limit, while a block holding one is invalid.
	if rerr, ok := err.(RuleError); ok {
		err = rerr.Err
		if cerr, ok := err.(blockchain.RuleError); ok {
			switch cerr.ErrorCode {
			case blockchain.ErrTxTooSmall, blockchain.ErrBadTxVersion,
				blockchain.ErrTxTooManySigChecks:
				return wire.RejectNonstandard, true
			}
		}
	}

	switch err := err.(type) {
	case blockchain.RuleError:
		// Convert the chain error to a reject code.
		var code wire.RejectCode
		switch err.ErrorCode {
		// Rejected due to duplicate.
		case blockchain.ErrDuplicateBlock:
			code = wire.RejectDuplicate

		// Rejected due to obsolete version.
		case blockchain.ErrBlockVersionTooOld:
			code = wire.RejectObsolete

		// Rejected due to checkpoint.
		case blockchain.ErrCheckpointTimeTooOld:
			fallthrough
		case blockchain.ErrDifficultyTooLow:
			fallthrough
		case blockchain.ErrBadCheckpoint:
			fallthrough
		case blockchain.ErrForkTooOld:
			code = wire.RejectCheckpoint

		// Everything else is due to the block or transaction being invalid.
		default:
			code = wire.RejectInvalid
		}

		return code, true

	case TxRuleError:
		return err.RejectCode, true

	case nil:
		return wire.RejectInvalid, false
	}

	return wire.RejectInvalid, false
}

// ErrToRejectErr examines the underlying type of the error and returns a reject
// code and string appropriate to be sent in a wire.MsgReject message.
func ErrToRejectErr(err error) (wire.RejectCode, string) {
	// Return the reject code along with the error text if it can be
	// extracted from the error.
	rejectCode, found := extractRejectCode(err)
	if found {
		return rejectCode, err.Error()
	}

	// Return a generic rejected string if there is no error.  This really
	// should not happen unless the code elsewhere is not setting an error
	// as it should be, but it's best to be safe and simply return a generic
	// string rather than allowing the following code that dereferences the
	// err to panic.
	if err == nil {
		return wire.RejectInvalid, "rejected"
	}

	// When the underlying error is not one of the above cases, just return
	// wire.RejectInvalid with a generic rejected string plus the error
	// text.
	return wire.RejectInvalid, "rejected: " + err.Error()
}

// RejectReason returns the short reason Bitcoin Cash Node gives when it
// refuses a transaction for the rule violation err reports, such as
// "txn-mempool-conflict", or an empty string when err is not a rule
// violation.
func RejectReason(err error) string {
	// Pull the underlying error out of a RuleError.
	if rerr, ok := err.(RuleError); ok {
		err = rerr.Err
	}

	switch err := err.(type) {
	case TxRuleError:
		return err.Reason
	case blockchain.RuleError:
		return chainRejectReason(err)
	case HighFeeError:
		return "absurdly-high-fee"
	}
	return ""
}

// chainRejectReason returns the reason BCHN's mempool gives for a transaction
// that breaks the consensus rule err reports.  These are the reasons of BCHN's
// CheckRegularTransaction, ContextualCheckTransaction, CheckTxInputs and
// CheckInputs.
func chainRejectReason(err blockchain.RuleError) string {
	switch err.ErrorCode {
	case blockchain.ErrNoTxInputs:
		return "bad-txns-vin-empty"
	case blockchain.ErrNoTxOutputs:
		return "bad-txns-vout-empty"
	case blockchain.ErrTxTooBig:
		return "bad-txns-oversize"
	case blockchain.ErrTxTooSmall:
		return "bad-txns-undersize"
	case blockchain.ErrBadTxVersion:
		return "bad-txns-version"
	case blockchain.ErrTxOutValueNegative:
		return "bad-txns-vout-negative"
	case blockchain.ErrTxOutValueTooHigh:
		return "bad-txns-vout-toolarge"
	case blockchain.ErrTxOutTotalTooHigh:
		return "bad-txns-txouttotal-toolarge"
	case blockchain.ErrTxInValueOutOfRange:
		return "bad-txns-inputvalues-outofrange"
	case blockchain.ErrDuplicateTxInputs:
		return "bad-txns-inputs-duplicate"
	case blockchain.ErrBadTxInput:
		return "bad-txns-prevout-null"
	case blockchain.ErrBadCoinbaseScriptLen:
		// Only a coinbase is checked for it, which BCHN refuses first.
		return "bad-tx-coinbase"
	case blockchain.ErrMissingTxOut, blockchain.ErrSpentTxOut:
		// A spend of a token output made before the upgrade gets this
		// too, which BCHN refuses as
		// bad-txns-vin-token-created-pre-activation, or as
		// bad-txns-vin-tokenprefix when its token data does not parse.
		return "bad-txns-inputs-missingorspent"
	case blockchain.ErrImmatureSpend:
		return "bad-txns-premature-spend-of-coinbase"
	case blockchain.ErrSpendTooHigh:
		return "bad-txns-in-belowout"
	case blockchain.ErrCashTokensValidation:
		// BCHN has a reason for each token rule, which the code does
		// not identify.
		return "bad-txns-token-invalid"
	case blockchain.ErrTxTooManySigChecks:
		// BCHN runs the failing script again without the limit, and as
		// that passes, reports a policy failure.
		return "non-mandatory-script-verify-flag (Validation " +
			"resources exceeded (SigChecks))"
	case blockchain.ErrScriptMalformed, blockchain.ErrScriptValidation:
		// The pool refuses a script that fails only standard flags BCHN
		// does not count as mandatory with a TxRuleError and
		// non-mandatory-script-verify-flag, as BCHN does.
		return "mandatory-script-verify-flag-failed (" +
			scriptErrorString(err.ScriptErr) + ")"
	}
	return "rejected"
}

// The texts of BCHN's EVAL_FALSE and SIG_DER script errors, which several
// codes stand for.
const (
	bchnEvalFalse = "Script evaluated without error but finished with a " +
		"false/empty top stack element"
	bchnSigDER = "Non-canonical DER signature"
)

// bchnScriptErrors holds the text BCHN's ScriptErrorString gives the script
// error each of these codes stands for.  The other codes of txscript are not
// returned by the script engine for a script that fails, or stand for no
// error of BCHN's.
var bchnScriptErrors = map[txscript.ErrorCode]string{
	txscript.ErrEarlyReturn:               "OP_RETURN was encountered",
	txscript.ErrEmptyStack:                bchnEvalFalse,
	txscript.ErrEvalFalse:                 bchnEvalFalse,
	txscript.ErrScriptTooBig:              "Script is too big",
	txscript.ErrElementTooBig:             "Push value size limit exceeded",
	txscript.ErrTooManyOperations:         "Operation limit exceeded",
	txscript.ErrStackOverflow:             "Stack size limit exceeded",
	txscript.ErrInvalidPubKeyCount:        "Pubkey count negative or limit exceeded",
	txscript.ErrInvalidSignatureCount:     "Signature count negative or greater than pubkey count",
	txscript.ErrOpCost:                    "VM cost limit exceeded",
	txscript.ErrTooManyHashIters:          "Hash iteration limit exceeded",
	txscript.ErrConditionalStackDepth:     "Conditional depth limit exceeded",
	txscript.ErrControlStackDepth:         "Control stack depth limit exceeded",
	txscript.ErrVerify:                    "Script failed an OP_VERIFY operation",
	txscript.ErrEqualVerify:               "Script failed an OP_EQUALVERIFY operation",
	txscript.ErrNumEqualVerify:            "Script failed an OP_NUMEQUALVERIFY operation",
	txscript.ErrCheckSigVerify:            "Script failed an OP_CHECKSIGVERIFY operation",
	txscript.ErrCheckMultiSigVerify:       "Script failed an OP_CHECKMULTISIGVERIFY operation",
	txscript.ErrCheckDataSigVerify:        "Script failed an OP_CHECKDATASIGVERIFY operation",
	txscript.ErrDisabledOpcode:            "Attempted to use a disabled opcode",
	txscript.ErrReservedOpcode:            "Opcode missing or not understood",
	txscript.ErrMalformedPush:             "Opcode missing or not understood",
	txscript.ErrInvalidStackOperation:     "Operation not valid with the current stack size",
	txscript.ErrInvalidAltStackOperation:  "Operation not valid with the current altstack size",
	txscript.ErrUnbalancedConditional:     "Invalid OP_IF construction",
	txscript.ErrUnbalancedControlFlow:     "Invalid control flow construction",
	txscript.ErrInvalidInputLength:        "Invalid operand size",
	txscript.ErrMinimalData:               "Data push larger than necessary",
	txscript.ErrMinimalNumber:             "Number encoding must be minimal",
	txscript.ErrMinimalIf:                 "OP_IF/NOTIF argument must be minimal",
	txscript.ErrNumberRange32Bit:          "Given operand is not a number within the valid range [-2^31 + 1, 2^31 - 1]",
	txscript.ErrNumberRange64Bit:          "Given operand is not a number within the valid range [-2^63 + 1, 2^63 - 1]",
	txscript.ErrNumberRangeBigInt:         "Given operand is not a number within the valid range [-2^79,999 + 1, 2^79,999 - 1]",
	txscript.ErrImpossibleEncoding:        "The requested encoding is impossible to satisfy",
	txscript.ErrInvalidSplitRange:         "Invalid OP_SPLIT range",
	txscript.ErrDivByZero:                 "Division by zero error",
	txscript.ErrModByZero:                 "Modulo by zero error",
	txscript.ErrInvalidBitShift:           "Given bit shift amount is negative",
	txscript.ErrInvalidSigHashType:        "Signature hash type missing or not understood",
	txscript.ErrIllegalForkID:             "Illegal use of SIGHASH_FORKID",
	txscript.ErrMustUseForkID:             "Signature must use SIGHASH_FORKID",
	txscript.ErrSigTooShort:               bchnSigDER,
	txscript.ErrSigTooLong:                bchnSigDER,
	txscript.ErrSigInvalidSeqID:           bchnSigDER,
	txscript.ErrSigInvalidDataLen:         bchnSigDER,
	txscript.ErrSigMissingSTypeID:         bchnSigDER,
	txscript.ErrSigMissingSLen:            bchnSigDER,
	txscript.ErrSigInvalidSLen:            bchnSigDER,
	txscript.ErrSigInvalidRIntID:          bchnSigDER,
	txscript.ErrSigZeroRLen:               bchnSigDER,
	txscript.ErrSigNegativeR:              bchnSigDER,
	txscript.ErrSigTooMuchRPadding:        bchnSigDER,
	txscript.ErrSigInvalidSIntID:          bchnSigDER,
	txscript.ErrSigZeroSLen:               bchnSigDER,
	txscript.ErrSigNegativeS:              bchnSigDER,
	txscript.ErrSigTooMuchSPadding:        bchnSigDER,
	txscript.ErrSigHighS:                  "Non-canonical signature: S value is unnecessarily high",
	txscript.ErrSigBadLength:              "Signature cannot be 65 bytes in CHECKMULTISIG",
	txscript.ErrSigNonSchnorr:             "Only Schnorr signatures allowed in this operation",
	txscript.ErrNotPushOnly:               "Only push operators allowed in signatures",
	txscript.ErrPubKeyType:                "Public key is neither compressed or uncompressed",
	txscript.ErrCleanStack:                "Extra items left on stack after execution",
	txscript.ErrNullFail:                  "Signature must be zero for failed CHECK(MULTI)SIG operation",
	txscript.ErrDiscourageUpgradableNOPs:  "NOPx reserved for soft-fork upgrades",
	txscript.ErrNegativeLockTime:          "Negative locktime",
	txscript.ErrUnsatisfiedLockTime:       "Locktime requirement not satisfied",
	txscript.ErrInvalidDummy:              "Bitfield of unexpected size error",
	txscript.ErrInvalidBitCount:           "Invalid number of bit set in OP_CHECKMULTISIG",
	txscript.ErrInvalidBitRange:           "Bitfield's bit out of the expected range",
	txscript.ErrInputSigChecks:            "Input SigChecks limit exceeded",
	txscript.ErrInvalidTxInputIndex:       "The specified transaction input index is out of range",
	txscript.ErrInvalidTxOutputIndex:      "The specified transaction output index is out of range",
	txscript.ErrInvalidFunctionIdentifier: "Function identifier is not within the valid range [0, 999]",
	txscript.ErrFunctionAlreadyDefined:    "Redefinition of an existing function with the same identifier is disallowed",
	txscript.ErrUndefinedFunction:         "Attempted to invoke an undefined function",
}

// scriptErrorString returns the text BCHN gives the script error err when its
// code stands for one of BCHN's errors, and the error's own text otherwise.
// Without an error it is BCHN's text for an error it cannot name.
func scriptErrorString(err error) string {
	if err == nil {
		return "unknown error"
	}
	if serr, ok := err.(txscript.Error); ok {
		if s, ok := bchnScriptErrors[serr.ErrorCode]; ok {
			return s
		}
	}
	return err.Error()
}

// BlockRejectReason returns the BIP0022 reason for a block that breaks the
// rule err reports, which submitblock returns and reject messages carry.
// Where Bitcoin Cash Node rejects a block for the same reason, its reason
// string is used.  blockVersion is the version of the block, which the reason
// for a version that is too old gives, as BCHN's does.  An error that is not
// a rule error, or a rule without a reason, gives "rejected: " and the error
// text.
func BlockRejectReason(err error, blockVersion int32) string {
	// When the passed error is not a RuleError, just return a generic
	// rejected string with the error text.
	ruleErr, ok := err.(blockchain.RuleError)
	if !ok {
		return "rejected: " + err.Error()
	}

	switch ruleErr.ErrorCode {
	case blockchain.ErrDuplicateBlock:
		return "duplicate"
	case blockchain.ErrBlockTooBig:
		return "bad-blk-length"
	case blockchain.ErrBlockVersionTooOld:
		return fmt.Sprintf("bad-version(0x%08x)", uint32(blockVersion))
	case blockchain.ErrInvalidTime:
		return "bad-time"
	case blockchain.ErrTimeTooOld:
		return "time-too-old"
	case blockchain.ErrTimeTooNew:
		return "time-too-new"
	case blockchain.ErrDifficultyTooLow:
		return "bad-diffbits"
	case blockchain.ErrUnexpectedDifficulty:
		return "bad-diffbits"
	case blockchain.ErrHighHash:
		return "high-hash"
	case blockchain.ErrBadMerkleRoot:
		return "bad-txnmrklroot"
	case blockchain.ErrBadCheckpoint:
		return "checkpoint mismatch"
	case blockchain.ErrForkTooOld:
		return "bad-fork-prior-to-checkpoint"
	case blockchain.ErrForkBeforeFinalized:
		return "bad-fork-prior-finalized"
	case blockchain.ErrCheckpointTimeTooOld:
		return "checkpoint-time-too-old"
	case blockchain.ErrNoTransactions:
		return "bad-cb-missing"
	case blockchain.ErrNoTxInputs:
		return "bad-txns-vin-empty"
	case blockchain.ErrNoTxOutputs:
		return "bad-txns-vout-empty"
	case blockchain.ErrTxTooBig:
		return "bad-txns-oversize"
	case blockchain.ErrTxTooSmall:
		return "bad-txns-undersize"
	case blockchain.ErrTxTooManySigChecks:
		// BCHN gives a transaction's sigchecks limit, like the block's,
		// the reason of its parallel script checks.
		return "blk-bad-inputs"
	case blockchain.ErrTxOutValueNegative:
		return "bad-txns-vout-negative"
	case blockchain.ErrTxOutValueTooHigh:
		return "bad-txns-vout-toolarge"
	case blockchain.ErrTxOutTotalTooHigh:
		return "bad-txns-txouttotal-toolarge"
	case blockchain.ErrTxInValueOutOfRange:
		return "bad-txns-inputvalues-outofrange"
	case blockchain.ErrDuplicateTxInputs:
		return "bad-txns-inputs-duplicate"
	case blockchain.ErrBadTxInput:
		return "bad-txns-prevout-null"
	case blockchain.ErrMissingTxOut:
		return "bad-txns-inputs-missingorspent"
	case blockchain.ErrSpentTxOut:
		return "bad-txns-inputs-missingorspent"
	case blockchain.ErrUnfinalizedTx:
		return "bad-txns-nonfinal"
	case blockchain.ErrDuplicateTx:
		return "bad-txns-duplicate"
	case blockchain.ErrOverwriteTx:
		return "bad-txns-BIP30"
	case blockchain.ErrImmatureSpend:
		return "bad-txns-premature-spend-of-coinbase"
	case blockchain.ErrSpendTooHigh:
		return "bad-txns-in-belowout"
	case blockchain.ErrBadFees:
		return "bad-txns-accumulated-fee-outofrange"
	case blockchain.ErrTooManySigChecks:
		// BCHN gives the block's sigchecks limit the reason of its
		// parallel script checks.
		return "blk-bad-inputs"
	case blockchain.ErrFirstTxNotCoinbase:
		return "bad-cb-missing"
	case blockchain.ErrMultipleCoinbases:
		return "bad-tx-coinbase"
	case blockchain.ErrBadCoinbaseScriptLen:
		return "bad-cb-length"
	case blockchain.ErrBadCoinbaseValue:
		return "bad-cb-amount"
	case blockchain.ErrMissingCoinbaseHeight:
		return "bad-cb-height"
	case blockchain.ErrBadCoinbaseHeight:
		return "bad-cb-height"
	case blockchain.ErrScriptMalformed:
		return "blk-bad-inputs"
	case blockchain.ErrScriptValidation:
		return "blk-bad-inputs"
	case blockchain.ErrPreviousBlockUnknown:
		return "prev-blk-not-found"
	case blockchain.ErrInvalidAncestorBlock:
		return "bad-prevblk"
	case blockchain.ErrPrevBlockNotBest:
		return "inconclusive-not-best-prevblk"
	case blockchain.ErrInvalidTxOrder:
		return "tx-ordering"
	case blockchain.ErrBadTxVersion:
		return "bad-txns-version"
	case blockchain.ErrCoinbaseHasTokens:
		return "bad-txns-coinbase-has-tokens"
	}

	return "rejected: " + err.Error()
}
