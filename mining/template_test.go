// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
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
