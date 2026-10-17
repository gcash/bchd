// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"testing"
)

// TestTemplateSkipsTxNotFinalAtMTP ensures a transaction whose lock time is
// past the median time of the tip is left out instead of failing the whole
// template, since consensus checks lock times against that median time.
func TestTemplateSkipsTxNotFinalAtMTP(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(110)

	mtp := h.chain.BestSnapshot().MedianTime
	tx := spend(h.cbTxs[0], 1000)
	tx.TxIn[0].Sequence = 0 // enable the lock time
	tx.LockTime = uint32(mtp.Unix() + 600)
	h.src.add(tx, 1000)

	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}
	if len(tmpl.Block.Transactions) != 1 {
		t.Fatal("non-final tx was included")
	}
}

// TestTemplateSkipsSequenceLockedTx ensures a transaction whose BIP68
// relative lock is not yet met is left out instead of failing the whole
// template.
func TestTemplateSkipsSequenceLockedTx(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(110)

	tx := spend(h.cbTxs[4], 1000) // input confirmed at height 5
	tx.TxIn[0].Sequence = 1000    // relative lock of 1000 blocks
	h.src.add(tx, 1000)

	// A tx whose relative lock is met is still included.
	ok := spend(h.cbTxs[5], 1000)
	ok.TxIn[0].Sequence = 10
	h.src.add(ok, 1000)

	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}
	if len(tmpl.Block.Transactions) != 2 {
		t.Fatalf("template has %d txs, want the coinbase and the "+
			"unlocked tx", len(tmpl.Block.Transactions))
	}
}
