// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// requireInsufficientFee fails the test unless err rejects a transaction for
// paying too little fee.
func requireInsufficientFee(t *testing.T, what string, err error) {
	t.Helper()
	if code, _ := extractRejectCode(err); code != wire.RejectInsufficientFee {
		t.Fatalf("%s: expected an insufficient fee rejection, got %v",
			what, err)
	}
	// BCHN's reason, which clients look for.
	if !strings.Contains(err.Error(), "min relay fee not met") {
		t.Fatalf("%s: rejection %q lacks BCHN's reason", what, err)
	}
}

// TestMinRelayFeeRequired ensures every new transaction must pay the minimum
// relay fee, however old and large its inputs are, and that only transactions
// added back from disconnected blocks are exempt.
func TestMinRelayFeeRequired(t *testing.T) {
	t.Parallel()

	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create test pool: %v", err)
	}
	tc := &testContext{t, harness}
	pool := harness.txPool

	// Old coins of 1000 BCH, so the spends have a coin-age priority far
	// above what made free transactions relayable.
	harness.chain.SetHeight(100000)
	coin := &wire.TxOut{Value: 1000 * bchutil.SatoshiPerBitcoin,
		PkScript: harness.payScript}
	ops := addCoins(harness, coin, coin, coin, coin)
	spendFee := func(i int, fee int64) *bchutil.Tx {
		return spend(t, harness, ops[i], coin, 1, fee,
			txscript.SigHashAll, true)
	}

	// The spends all have the same size, so the same minimum fee.
	minFee := calcMinRequiredTxRelayFee(
		int64(spendFee(0, 0).MsgTx().SerializeSize()),
		pool.cfg.Policy.MinRelayTxFee)

	for _, fee := range []int64{0, minFee - 1} {
		tx := spendFee(0, fee)
		for _, allowOrphan := range []bool{false, true} {
			_, err := pool.ProcessTransaction(tx, allowOrphan, 0)
			requireInsufficientFee(t, "ProcessTransaction", err)
		}
		_, _, err := pool.MaybeAcceptTransaction(tx, true)
		requireInsufficientFee(t, "MaybeAcceptTransaction", err)
		_, err = pool.CheckMempoolAcceptance(tx, 0)
		requireInsufficientFee(t, "CheckMempoolAcceptance", err)
		testPoolMembership(tc, tx, false, false)
	}

	// Paying exactly the minimum is enough.
	paid := spendFee(0, minFee)
	if _, err := pool.ProcessTransaction(paid, false, 0); err != nil {
		t.Fatalf("transaction paying the minimum fee rejected: %v", err)
	}
	testPoolMembership(tc, paid, false, true)

	// A free orphan is rejected once its parent arrives.
	parent := spendFee(1, minFee)
	child := spend(t, harness, wire.OutPoint{Hash: *parent.Hash()},
		parent.MsgTx().TxOut[0], 1, 0, txscript.SigHashAll, true)
	if _, err := pool.ProcessTransaction(child, true, 0); err != nil {
		t.Fatalf("unable to add orphan: %v", err)
	}
	testPoolMembership(tc, child, true, false)
	accepted, err := pool.ProcessTransaction(parent, false, 0)
	if err != nil {
		t.Fatalf("unable to accept parent: %v", err)
	}
	if len(accepted) != 1 {
		t.Fatalf("accepted %d transactions, want only the parent",
			len(accepted))
	}
	testPoolMembership(tc, child, false, false)

	// A free transaction added back from a disconnected block is kept.
	readded := spendFee(2, 0)
	if _, _, err := pool.MaybeAcceptTransaction(readded, false); err != nil {
		t.Fatalf("transaction from a disconnected block rejected: %v",
			err)
	}
	testPoolMembership(tc, readded, false, true)
}

// TestRawMempoolVerbosePriority ensures the deprecated priority fields of
// RawMempoolVerbose are 0, even for a transaction spending old coins.
func TestRawMempoolVerbosePriority(t *testing.T) {
	t.Parallel()

	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create test pool: %v", err)
	}
	harness.chain.SetHeight(100000)
	coin := &wire.TxOut{Value: 1000 * bchutil.SatoshiPerBitcoin,
		PkScript: harness.payScript}
	tx := spend(t, harness, addCoins(harness, coin)[0], coin, 1, 1000,
		txscript.SigHashAll, true)
	if _, err := harness.txPool.ProcessTransaction(tx, false, 0); err != nil {
		t.Fatalf("ProcessTransaction: %v", err)
	}

	entry := harness.txPool.RawMempoolVerbose()[tx.Hash().String()]
	if entry == nil {
		t.Fatal("transaction missing from RawMempoolVerbose")
	}
	if entry.StartingPriority != 0 || entry.CurrentPriority != 0 {
		t.Fatalf("priorities %v and %v, want 0", entry.StartingPriority,
			entry.CurrentPriority)
	}
}

// TestDustNotFromMinRelayFee ensures dust is judged at BCHN's default dust
// relay fee whatever the minimum relay fee, so raising that does not refuse
// outputs BCHN relays: 600 satoshis to a P2PKH is above BCHN's 546 even with
// a minimum relay fee of 2000 satoshis per kilobyte, and 545 is still dust.
func TestDustNotFromMinRelayFee(t *testing.T) {
	t.Parallel()

	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create test pool: %v", err)
	}
	pool := harness.txPool
	pool.cfg.Policy.MinRelayTxFee = 2000
	coin := &wire.TxOut{Value: 100000, PkScript: harness.payScript}
	op := addCoins(harness, coin)[0]

	spend := func(value int64) error {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op, Sequence: wire.MaxTxInSequenceNum})
		tx.AddTxOut(&wire.TxOut{Value: value, PkScript: harness.payScript})
		tx.AddTxOut(&wire.TxOut{Value: 100000 - value - 2000, PkScript: harness.payScript})
		signP2PKH(t, tx, 0, coin, harness.signKey,
			txscript.SigHashAll|txscript.SigHashForkID, true)
		_, err := pool.ProcessTransaction(bchutil.NewTx(tx), false, 0)
		return err
	}
	err = spend(545)
	if code, _ := extractRejectCode(err); code != wire.RejectNonstandard {
		t.Fatalf("545 satoshi output: got %v, want dust", err)
	}
	if err := spend(600); err != nil {
		t.Fatalf("600 satoshi output refused: %v", err)
	}
}
