// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"fmt"

	"github.com/gcash/bchd/blockchain"
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
	// for a transaction it refuses.  A transaction below the minimum size or
	// above its sigchecks limit is nonstandard there, as in BCHN's memory
	// pool, which finds the first in its contextual checks and runs the
	// scripts of the second again without the limit, while a block holding
	// one is invalid.
	if rerr, ok := err.(RuleError); ok {
		err = rerr.Err
		if cerr, ok := err.(blockchain.RuleError); ok {
			switch cerr.ErrorCode {
			case blockchain.ErrTxTooSmall, blockchain.ErrTxTooManySigChecks:
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
		return chainRejectReason(err.ErrorCode)
	case HighFeeError:
		return "absurdly-high-fee"
	}
	return ""
}

// chainRejectReason returns the reason BCHN's mempool gives for a transaction
// that breaks the consensus rule identified by code.  These are the reasons
// of BCHN's CheckRegularTransaction, ContextualCheckTransaction, CheckTxInputs
// and CheckInputs.
func chainRejectReason(code blockchain.ErrorCode) string {
	switch code {
	case blockchain.ErrNoTxInputs:
		return "bad-txns-vin-empty"
	case blockchain.ErrNoTxOutputs:
		return "bad-txns-vout-empty"
	case blockchain.ErrTxTooBig:
		return "bad-txns-oversize"
	case blockchain.ErrTxTooSmall:
		return "bad-txns-undersize"
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
		// BCHN names the script error, and reports a failure that only
		// the policy flags cause as non-mandatory-script-verify-flag.
		// The code does neither.
		return "mandatory-script-verify-flag-failed (Script failed)"
	}
	return "rejected"
}
