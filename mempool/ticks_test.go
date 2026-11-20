// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestNextBlockTicks ensures the pool checks coinbase maturity and lock times
// for the ticks of the next block, on a chain whose block 1 is the last
// 600-tick block and on one that never activates 1-minute blocks.  The
// harness coinbase is at block 1, 600 ticks, so with the upgrade it matures
// for block 1001 at 60,600 ticks, the block after tip 1000, as in the
// CHIP-2025-03 test_MatureCurrentTip case.  A transaction locked to height
// 101 is final once the next block is past 60,600 ticks.
func TestNextBlockTicks(t *testing.T) {
	tests := []struct {
		name   string
		active bool
		mature int32
	}{
		{"active", true, 1000},
		{"never", false, 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness, outputs, err := newPoolHarness(
				&chaincfg.MainNetParams)
			if err != nil {
				t.Fatalf("unable to create test pool: %v", err)
			}
			if test.active {
				harness.chain.SetUpgrade2027Height(1)
			}

			tx := signedSpend(t, harness, outputs[0], 1, 0,
				wire.MaxTxInSequenceNum)
			harness.chain.SetHeight(test.mature - 1)
			_, err = harness.txPool.ProcessTransaction(tx, false, 0)
			var rerr RuleError
			var cerr blockchain.RuleError
			if !errors.As(err, &rerr) || !errors.As(rerr.Err, &cerr) ||
				cerr.ErrorCode != blockchain.ErrImmatureSpend {

				t.Fatalf("tip %d: got %v, want an immature spend",
					test.mature-1, err)
			}
			harness.chain.SetHeight(test.mature)
			if _, err := harness.txPool.ProcessTransaction(tx, false,
				0); err != nil {

				t.Fatalf("tip %d: spend rejected: %v", test.mature,
					err)
			}

			// With the upgrade the block after tip 1000 is at
			// 60,600 ticks, without it at 600,600.
			harness.chain.SetHeight(1000)
			out := txOutToSpendableOut(tx, 0)
			for _, lock := range []struct {
				lockTime uint32
				final    bool
			}{
				{100, true},
				{101, !test.active},
			} {
				locked := signedSpend(t, harness, out, 1,
					lock.lockTime, 0)
				_, err := harness.txPool.CheckMempoolAcceptance(
					locked, 0)
				if lock.final != (err == nil) {
					t.Errorf("lock time %d: got %v, want final "+
						"%v", lock.lockTime, err, lock.final)
				}
				if err != nil && RejectReason(err) !=
					"bad-txns-nonfinal" {

					t.Errorf("lock time %d: got %v, want "+
						"bad-txns-nonfinal", lock.lockTime, err)
				}
			}
		})
	}
}

// TestNextBlockSequenceLock ensures the pool checks height-based relative
// locks for the ticks of the next block.  The activation block is 1000 and
// the coins are in it and in block 1005.  With 1-minute blocks a lock of two
// units releases them twenty blocks later, without them two blocks later.
func TestNextBlockSequenceLock(t *testing.T) {
	tests := []struct {
		name       string
		active     bool
		coinHeight int32
		release    int32
	}{
		{"active seam coin", true, 1000, 1020},
		{"active coin", true, 1005, 1025},
		{"never seam coin", false, 1000, 1002},
		{"never coin", false, 1005, 1007},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness, outputs, err := newPoolHarness(
				&chaincfg.MainNetParams)
			if err != nil {
				t.Fatalf("unable to create test pool: %v", err)
			}
			if test.active {
				harness.chain.SetUpgrade2027Height(1000)
			}
			harness.txPool.cfg.Policy.MaxTxVersion = 2
			harness.txPool.cfg.CalcSequenceLock =
				harness.chain.heightSequenceLock

			fund := signedSpend(t, harness, outputs[0], 1, 0,
				wire.MaxTxInSequenceNum)
			harness.chain.utxos.AddTxOuts(fund, test.coinHeight)
			spend := signedSpend(t, harness,
				txOutToSpendableOut(fund, 0), 2, 0,
				blockchain.LockTimeToSequence(false, 2))

			for tip := test.coinHeight; tip < test.release-1; tip++ {
				harness.chain.SetHeight(tip)
				_, err := harness.txPool.CheckMempoolAcceptance(
					spend, 0)
				if RejectReason(err) != "non-BIP68-final" {
					t.Fatalf("tip %d: got %v, want "+
						"non-BIP68-final", tip, err)
				}
			}
			harness.chain.SetHeight(test.release - 1)
			_, err = harness.txPool.ProcessTransaction(spend, false, 0)
			if err != nil {
				t.Fatalf("tip %d: spend rejected: %v",
					test.release-1, err)
			}
		})
	}
}

// signedSpend returns a signed transaction spending out with the passed
// version, lock time and sequence.
func signedSpend(t *testing.T, harness *poolHarness, out spendableOutput,
	version int32, lockTime, sequence uint32) *bchutil.Tx {

	t.Helper()
	tx := wire.NewMsgTx(version)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: out.outPoint,
		Sequence:         sequence,
	})
	tx.AddTxOut(&wire.TxOut{
		PkScript: harness.payScript,
		Value:    int64(out.amount) - testTxFee,
	})
	tx.LockTime = lockTime
	sigScript, err := txscript.SignatureScript(tx, 0, int64(out.amount),
		harness.payScript, txscript.SigHashAll, harness.signKey, true)
	if err != nil {
		t.Fatalf("unable to sign: %v", err)
	}
	tx.TxIn[0].SignatureScript = sigScript
	return bchutil.NewTx(tx)
}
