// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"errors"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/mempool"
)

// txBanScores are the ban scores BCHN gives the peer that sent a transaction
// its memory pool refuses for breaking a consensus rule, by the bchd error
// code of the rule: its DoS levels in CheckRegularTransaction
// (consensus/tx_check.cpp:12-119), CheckTxInputs (consensus/tx_verify.cpp:
// 179-236), CheckTxTokens (consensus/tokens.cpp:109-303) and CheckInputs for a
// mandatory script failure (validation.cpp:1354-1366), which
// AcceptToMemoryPool returns (validation.cpp:487-877) and the transaction
// handler adds (net_processing.cpp:2999-3017).  The pool returns
// ErrScriptMalformed or ErrScriptValidation only for an input whose script
// also fails without the standard flags BCHN does not count as mandatory.
// ErrScriptMalformed is such a failure found before the scripts run: a
// malformed push in either script, both scripts empty, or an input script
// over 10,000 bytes, which BCHN refuses under any flags
// (script/interpreter.cpp:371-373, 423-425, 2741-2743).  The script engine
// also refuses a version out of range, but the pool checks the version before
// the scripts.  Codes BCHN gives no score for there are left out: a
// transaction below the minimum size or with an unknown version, which its
// memory pool refuses as nonstandard (validation.cpp:528-538), a premature
// coinbase spend, missing or spent inputs, and the transaction sigchecks
// limit, a non-mandatory failure.
var txBanScores = map[blockchain.ErrorCode]uint32{
	blockchain.ErrNoTxInputs:           10,  // bad-txns-vin-empty
	blockchain.ErrNoTxOutputs:          10,  // bad-txns-vout-empty
	blockchain.ErrTxTooBig:             100, // bad-txns-oversize
	blockchain.ErrTxOutValueNegative:   100, // bad-txns-vout-negative
	blockchain.ErrTxOutValueTooHigh:    100, // bad-txns-vout-toolarge
	blockchain.ErrTxOutTotalTooHigh:    100, // bad-txns-txouttotal-toolarge
	blockchain.ErrDuplicateTxInputs:    100, // bad-txns-inputs-duplicate
	blockchain.ErrBadTxInput:           10,  // bad-txns-prevout-null
	blockchain.ErrBadCoinbaseScriptLen: 100, // a coinbase, bad-tx-coinbase
	blockchain.ErrTxInValueOutOfRange:  100, // bad-txns-inputvalues-outofrange
	blockchain.ErrSpendTooHigh:         100, // bad-txns-in-belowout
	blockchain.ErrCashTokensValidation: 100, // bad-txns-token-...
	blockchain.ErrScriptMalformed:      100, // mandatory-script-verify-flag-failed
	blockchain.ErrScriptValidation:     100, // mandatory-script-verify-flag-failed
}

// txBanScore returns the ban score BCHN gives the peer that sent a
// transaction the memory pool refused with err, or 0 when it gives none.
func txBanScore(err error) uint32 {
	var rerr mempool.RuleError
	if !errors.As(err, &rerr) {
		return 0
	}
	switch e := rerr.Err.(type) {
	case blockchain.RuleError:
		return txBanScores[e.ErrorCode]
	case mempool.TxRuleError:
		// A coinbase is valid only in a block (consensus/tx_check.cpp:
		// 70-73).
		if e.Reason == "bad-tx-coinbase" {
			return 100
		}
	}
	return 0
}

// blockBanScores are the ban scores BCHN gives the peer that sent a block for
// breaking a rule it checks before connecting the block, by the bchd error
// code of the rule: its DoS levels in CheckBlock and the transaction checks it
// runs (validation.cpp:3567-3657, consensus/tx_check.cpp), AcceptBlockHeader
// with ContextualCheckBlockHeader (3685-3781, 3943-3946, 3989-3991) and
// ContextualCheckBlock (3828-3894), which BlockChecked adds when the reject
// code is a real one (net_processing.cpp:1094-1117).  bchd checks these rules
// for the block before it is connected, so a failure is the block's own.
// ErrUnexpectedDifficulty is a target out of range, high-hash (50) in BCHN,
// or the wrong one, bad-diffbits (100), so it gets the lower.  Codes BCHN
// gives no score for are left out: a time too old or too new and an old
// version (3756-3778), a block that forks below the finalized block, whose
// reject code BCHN keeps internal (REJECT_AGAINST_FINALIZED, validation.h:
// 753), and a premature coinbase spend.
var blockBanScores = map[blockchain.ErrorCode]uint32{
	blockchain.ErrHighHash:              50,  // high-hash
	blockchain.ErrUnexpectedDifficulty:  50,  // high-hash, bad-diffbits
	blockchain.ErrBadMerkleRoot:         100, // bad-txnmrklroot
	blockchain.ErrDuplicateTx:           100, // bad-txns-duplicate
	blockchain.ErrNoTransactions:        100, // bad-cb-missing
	blockchain.ErrFirstTxNotCoinbase:    100, // bad-cb-missing
	blockchain.ErrMultipleCoinbases:     100, // bad-tx-coinbase
	blockchain.ErrBadCoinbaseScriptLen:  100, // bad-cb-length
	blockchain.ErrNoTxInputs:            10,  // bad-txns-vin-empty
	blockchain.ErrNoTxOutputs:           10,  // bad-txns-vout-empty
	blockchain.ErrTxTooBig:              100, // bad-txns-oversize
	blockchain.ErrTxOutValueNegative:    100, // bad-txns-vout-negative
	blockchain.ErrTxOutValueTooHigh:     100, // bad-txns-vout-toolarge
	blockchain.ErrTxOutTotalTooHigh:     100, // bad-txns-txouttotal-toolarge
	blockchain.ErrDuplicateTxInputs:     100, // bad-txns-inputs-duplicate
	blockchain.ErrBadTxInput:            10,  // bad-txns-prevout-null
	blockchain.ErrInvalidTxOrder:        100, // tx-ordering
	blockchain.ErrTxTooSmall:            100, // bad-txns-undersize
	blockchain.ErrBadTxVersion:          100, // bad-txns-version
	blockchain.ErrMissingCoinbaseHeight: 100, // bad-cb-height
	blockchain.ErrBadCoinbaseHeight:     100, // bad-cb-height
	blockchain.ErrBadCheckpoint:         100, // checkpoint mismatch
	blockchain.ErrForkTooOld:            100, // bad-fork-prior-to-checkpoint
	blockchain.ErrInvalidAncestorBlock:  100, // bad-prevblk
}

// connectBanScores are the ban scores BCHN gives the peer that sent a block
// for breaking a rule it checks only when it connects the block, by the bchd
// error code of the rule: its DoS levels in ConnectBlock (validation.cpp:
// 1648-2023), CheckTxInputs and CheckTxTokens, which BlockChecked adds
// (2557).  bchd reports such a failure for whichever block of a
// reorganization had it, so these are given only for a block that extended
// the tip, which is connected alone.  ErrUnfinalizedTx is a lock time,
// bad-txns-nonfinal for 10 in ContextualCheckBlock, or a sequence lock, for
// 100 in ConnectBlock, so it gets the lower.  ErrBlockTooBig, which bchd
// checks before connecting, is bad-blk-length against the block size limit in
// ConnectBlock (1682-1688).  ErrCoinbaseHasTokens is checked in BCHN's
// ConnectBlock too.
var connectBanScores = map[blockchain.ErrorCode]uint32{
	blockchain.ErrMissingTxOut:         100, // bad-txns-inputs-missingorspent
	blockchain.ErrSpentTxOut:           100, // bad-txns-inputs-missingorspent
	blockchain.ErrTxInValueOutOfRange:  100, // bad-txns-inputvalues-outofrange
	blockchain.ErrSpendTooHigh:         100, // bad-txns-in-belowout
	blockchain.ErrBadFees:              100, // bad-txns-accumulated-fee-outofrange
	blockchain.ErrBadCoinbaseValue:     100, // bad-cb-amount
	blockchain.ErrOverwriteTx:          100, // bad-txns-BIP30
	blockchain.ErrUnfinalizedTx:        10,  // bad-txns-nonfinal
	blockchain.ErrScriptMalformed:      100, // blk-bad-inputs
	blockchain.ErrScriptValidation:     100, // blk-bad-inputs
	blockchain.ErrTxTooManySigChecks:   100, // blk-bad-inputs
	blockchain.ErrTooManySigChecks:     100, // blk-bad-inputs
	blockchain.ErrCashTokensValidation: 100, // bad-txns-token-...
	blockchain.ErrCoinbaseHasTokens:    100, // bad-txns-coinbase-has-tokens
	blockchain.ErrBlockTooBig:          100, // bad-blk-length
}

// blockBanScore returns the ban score BCHN gives the peer that sent a block
// that failed with the rule error code, or 0 when it gives none.  extendsTip
// tells whether the block's parent was the tip of the chain.
func blockBanScore(code blockchain.ErrorCode, extendsTip bool) uint32 {
	if score, ok := blockBanScores[code]; ok {
		return score
	}
	if extendsTip {
		return connectBanScores[code]
	}
	return 0
}
