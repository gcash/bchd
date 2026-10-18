// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"errors"
	"slices"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
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

// TestTemplateBlockSizeAccounting ensures the starting block size is the
// header overhead plus the coinbase, so a small transaction fits in a small
// block.
func TestTemplateBlockSizeAccounting(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(105)

	h.gen.policy = &Policy{BlockMaxSize: 5000}
	h.src.add(spend(h.cbTxs[0], 1000), 1000)

	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}
	if n := len(tmpl.Block.Transactions); n != 2 {
		t.Fatalf("a small tx did not fit in a 5000 byte block (%d txs, "+
			"block size %d)", n, tmpl.Block.SerializeSize())
	}
}

// TestTemplateReservesCoinbaseRoom ensures at least 1000 bytes are kept for
// the header and coinbase, so a miner extending the coinbase stays within
// the configured size.
func TestTemplateReservesCoinbaseRoom(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(105)

	tx := spend(h.cbTxs[0], 1000)
	h.src.add(tx, 1000)
	for _, test := range []struct {
		maxSize uint32
		want    int
	}{
		{1000 + uint32(tx.SerializeSize()), 1},
		{1001 + uint32(tx.SerializeSize()), 2},
	} {
		h.gen.policy = &Policy{BlockMaxSize: test.maxSize}
		tmpl, err := h.gen.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate: %v", err)
		}
		if n := len(tmpl.Block.Transactions); n != test.want {
			t.Errorf("BlockMaxSize %d: template has %d txs, want %d",
				test.maxSize, n, test.want)
		}
	}
}

// ablaParams returns templateParams with the ABLA algorithm running from the
// first block above a 1 MB floor.
func ablaParams() chaincfg.Params {
	p := templateParams()
	p.ABLAForkHeight = 0
	p.ABLAConfig.FixedSize = false
	p.ABLAConfig.N0 = 0
	return p
}

// TestTemplateLimitsFollowABLA ensures the template's size and sigchecks
// limits are the ABLA limit for the next block, not the static floor.
func TestTemplateLimitsFollowABLA(t *testing.T) {
	const floor = 1000000
	h := newTemplateHarness(t, ablaParams(), floor, bigPolicy)
	h.mineN(105)

	// One large block grows the ABLA limit above the floor.
	big := spend(h.cbTxs[0], 1000)
	big.TxOut[1].PkScript = append([]byte{txscript.OP_RETURN},
		make([]byte, 600000)...)
	h.src.add(big, 1000)
	h.mine()

	limit := h.chain.MaxBlockSize(true, true)
	if limit <= floor {
		t.Fatalf("ABLA limit %d did not grow", limit)
	}
	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}
	if tmpl.MaxBlockSize != limit {
		t.Errorf("template MaxBlockSize %d, want the ABLA limit %d",
			tmpl.MaxBlockSize, limit)
	}
	want := limit / blockchain.BlockMaxBytesMaxSigChecksRatio
	if uint64(tmpl.MaxSigChecks) != want {
		t.Errorf("template MaxSigChecks %d, want %d", tmpl.MaxSigChecks,
			want)
	}
}

// TestTemplateSelectionCappedByLimit ensures transactions are selected up to
// the block size limit less 1000 bytes when the configured block max size is
// larger or zero, so the template stays valid.
func TestTemplateSelectionCappedByLimit(t *testing.T) {
	for _, pol := range []Policy{bigPolicy, {}} {
		const floor = 1000000
		h := newTemplateHarness(t, ablaParams(), floor, pol)
		h.mineN(105)

		// Three 400 KB transactions, more than the 1 MB limit.
		for i := range 3 {
			tx := spend(h.cbTxs[i], 1000)
			tx.TxOut[1].PkScript = append([]byte{txscript.OP_RETURN},
				make([]byte, 400000)...)
			h.src.add(tx, 1000)
		}
		tmpl, err := h.gen.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("BlockMaxSize %d: NewBlockTemplate: %v",
				pol.BlockMaxSize, err)
		}
		if n := len(tmpl.Block.Transactions); n != 3 {
			t.Errorf("BlockMaxSize %d: template has %d transactions, "+
				"want the coinbase and 2", pol.BlockMaxSize, n)
		}
		if size := tmpl.Block.SerializeSize(); size > floor-1000 {
			t.Errorf("BlockMaxSize %d: template size %d over %d",
				pol.BlockMaxSize, size, floor-1000)
		}
	}
}

// TestCheckConnectBlockTemplateUsesTipHeight ensures a proposed block whose
// height was never set, as for a getblocktemplate proposal, is checked with
// the rules for the height after the tip, here the canonical order.
func TestCheckConnectBlockTemplateUsesTipHeight(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(105)

	h.src.add(spend(h.cbTxs[0], 1000), 1000)
	h.src.add(spend(h.cbTxs[1], 1000), 1000)
	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}

	// Swap the two transactions out of order and fix the merkle root.
	msg := tmpl.Block
	msg.Transactions[1], msg.Transactions[2] = msg.Transactions[2],
		msg.Transactions[1]
	merkles := blockchain.BuildMerkleTreeStore(
		bchutil.NewBlock(msg).Transactions())
	msg.Header.MerkleRoot = *merkles[len(merkles)-1]

	err = h.chain.CheckConnectBlockTemplate(bchutil.NewBlock(msg))
	var rerr blockchain.RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != blockchain.ErrInvalidTxOrder {
		t.Fatalf("expected ErrInvalidTxOrder, got %v", err)
	}
}

// templateHas returns whether the template contains tx.
func templateHas(tmpl *BlockTemplate, tx *wire.MsgTx) bool {
	hash := tx.TxHash()
	for _, blockTx := range tmpl.Block.Transactions {
		if blockTx.TxHash() == hash {
			return true
		}
	}
	return false
}

// TestTemplateIncludesChildAfterParent ensures a transaction spending another
// pool transaction is added once its parent is, and left out when the parent
// cannot be selected.
func TestTemplateIncludesChildAfterParent(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(105)

	parent := spend(h.cbTxs[0], 1000)
	child := spend(parent, 1000)
	h.src.add(child, 1000)
	h.src.add(parent, 1000)

	// A parent that is not final yet, and its child.
	lockedParent := spend(h.cbTxs[1], 1000)
	lockedParent.TxIn[0].Sequence = 0
	lockedParent.LockTime = uint32(h.chain.BestSnapshot().Height + 10)
	lockedChild := spend(lockedParent, 1000)
	h.src.add(lockedChild, 1000)
	h.src.add(lockedParent, 1000)

	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}
	if !templateHas(tmpl, parent) || !templateHas(tmpl, child) {
		t.Error("parent and child were not both included")
	}
	if templateHas(tmpl, lockedParent) || templateHas(tmpl, lockedChild) {
		t.Error("non-final parent or its child was included")
	}
	if n := len(tmpl.Block.Transactions); n != 3 {
		t.Errorf("template has %d txs, want the coinbase and 2", n)
	}
}

// TestTemplateSkipsImmatureCoinbaseSpend ensures a transaction spending an
// immature coinbase is left out and the template stays valid.
func TestTemplateSkipsImmatureCoinbaseSpend(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(105)

	good := spend(h.cbTxs[0], 1000)
	immature := spend(h.cbTxs[99], 1000) // coinbase of block 100
	h.src.add(immature, 1000)
	h.src.add(good, 1000)

	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}
	if templateHas(tmpl, immature) {
		t.Error("immature coinbase spend was included")
	}
	if !templateHas(tmpl, good) || len(tmpl.Block.Transactions) != 2 {
		t.Errorf("template has %d txs, want the coinbase and the "+
			"mature spend", len(tmpl.Block.Transactions))
	}
}

// TestTemplateCanonicalOrder ensures the transactions after the coinbase are
// sorted by txid, with a parent and child among them, and that each keeps its
// own fee.
func TestTemplateCanonicalOrder(t *testing.T) {
	h := newTemplateHarness(t, templateParams(), 32000000, bigPolicy)
	h.mineN(110)

	var txs []*wire.MsgTx
	for i := range 5 {
		txs = append(txs, spend(h.cbTxs[i], 10000))
	}
	txs = append(txs, spend(txs[0], 10000))

	// Pay the highest declared fee to the highest txid so selection order
	// is the reverse of the canonical order.
	sorted := slices.Clone(txs)
	slices.SortFunc(sorted, func(a, b *wire.MsgTx) int {
		ha, hb := a.TxHash(), b.TxHash()
		return ha.Compare(&hb)
	})
	for i, tx := range sorted {
		h.src.add(tx, int64(1000*(i+1)))
	}

	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate: %v", err)
	}
	got := tmpl.Block.Transactions[1:]
	if len(got) != len(sorted) {
		t.Fatalf("template has %d txs, want %d", len(got), len(sorted))
	}
	for i, tx := range got {
		if tx.TxHash() != sorted[i].TxHash() {
			t.Fatalf("tx %d is %v, want %v", i+1, tx.TxHash(),
				sorted[i].TxHash())
		}
	}

	// The coinbase entry is minus the total of the fees.
	numTxs := len(tmpl.Block.Transactions)
	if len(tmpl.Fees) != numTxs || len(tmpl.SigChecks) != numTxs {
		t.Fatalf("template has %d fees and %d sigchecks for %d txs",
			len(tmpl.Fees), len(tmpl.SigChecks), numTxs)
	}
	total := int64(0)
	for i := range got {
		want := int64(1000 * (i + 1))
		if tmpl.Fees[i+1] != want {
			t.Errorf("tx %d has fee %d, want %d", i+1, tmpl.Fees[i+1],
				want)
		}
		total += want
	}
	if tmpl.Fees[0] != -total {
		t.Errorf("coinbase fee entry is %d, want %d", tmpl.Fees[0], -total)
	}
}
