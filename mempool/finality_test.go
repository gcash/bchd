// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestAcceptRejectsNonFinal ensures the pool rejects transactions which are
// not final in the next block whether or not non-standard transactions are
// accepted.
func TestAcceptRejectsNonFinal(t *testing.T) {
	// A fixed median time past keeps the time lock cases deterministic.
	medianTime := time.Unix(1700000000, 0)

	tests := []struct {
		name     string
		lockTime func(h *poolHarness) uint32
		sequence uint32
		final    bool
	}{{
		name: "height lock at next block",
		lockTime: func(h *poolHarness) uint32 {
			return uint32(h.chain.BestHeight() + 1)
		},
		sequence: 0,
	}, {
		name: "time lock past median time",
		lockTime: func(h *poolHarness) uint32 {
			return uint32(h.chain.MedianTimePast().Unix() + 1)
		},
		sequence: 0,
	}, {
		name: "time lock at median time",
		lockTime: func(h *poolHarness) uint32 {
			return uint32(h.chain.MedianTimePast().Unix())
		},
		sequence: 0,
	}, {
		name: "height lock at best height",
		lockTime: func(h *poolHarness) uint32 {
			return uint32(h.chain.BestHeight())
		},
		sequence: 0,
		final:    true,
	}, {
		name: "time lock before median time",
		lockTime: func(h *poolHarness) uint32 {
			return uint32(h.chain.MedianTimePast().Unix() - 1)
		},
		sequence: 0,
		final:    true,
	}, {
		name: "future lock with final sequences",
		lockTime: func(h *poolHarness) uint32 {
			return uint32(h.chain.BestHeight() + 100)
		},
		sequence: wire.MaxTxInSequenceNum,
		final:    true,
	}}

	for _, test := range tests {
		for _, acceptNonStd := range []bool{false, true} {
			harness, outputs, err := newPoolHarness(&chaincfg.MainNetParams)
			if err != nil {
				t.Fatalf("unable to create test pool: %v", err)
			}
			harness.txPool.cfg.Policy.AcceptNonStd = acceptNonStd
			harness.chain.SetMedianTimePast(medianTime)

			out := outputs[0]
			tx := wire.NewMsgTx(1)
			tx.AddTxIn(&wire.TxIn{
				PreviousOutPoint: out.outPoint,
				Sequence:         test.sequence,
			})
			tx.AddTxOut(&wire.TxOut{
				PkScript: harness.payScript,
				Value:    int64(out.amount) - 1000,
			})
			tx.LockTime = test.lockTime(harness)
			sigScript, err := txscript.SignatureScript(tx, 0,
				int64(out.amount), harness.payScript,
				txscript.SigHashAll, harness.signKey, true)
			if err != nil {
				t.Fatalf("unable to sign: %v", err)
			}
			tx.TxIn[0].SignatureScript = sigScript

			_, err = harness.txPool.ProcessTransaction(bchutil.NewTx(tx),
				false, 0)
			if test.final {
				if err != nil {
					t.Errorf("%s (acceptNonStd=%v): final tx "+
						"rejected: %v", test.name, acceptNonStd, err)
				}
				continue
			}

			var rerr RuleError
			if !errors.As(err, &rerr) {
				t.Errorf("%s (acceptNonStd=%v): non-final tx not "+
					"rejected: %v", test.name, acceptNonStd, err)
				continue
			}
			if code, _ := extractRejectCode(err); code != wire.RejectNonstandard {
				t.Errorf("%s (acceptNonStd=%v): got reject code %v, "+
					"want %v", test.name, acceptNonStd, code,
					wire.RejectNonstandard)
			}
			if !strings.Contains(err.Error(), "not finalized") {
				t.Errorf("%s (acceptNonStd=%v): unexpected error: %v",
					test.name, acceptNonStd, err)
			}
			if acceptNonStd && !strings.Contains(err.Error(),
				"bad-txns-nonfinal") {
				t.Errorf("%s (acceptNonStd=%v): unexpected error: %v",
					test.name, acceptNonStd, err)
			}
		}
	}
}
