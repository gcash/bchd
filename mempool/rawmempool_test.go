// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestRawMempoolVerbose ensures the verbose getrawmempool result describes
// each transaction in the pool, with the transactions of the pool it spends
// as its depends, and no others.
func TestRawMempoolVerbose(t *testing.T) {
	t.Parallel()

	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create test pool: %v", err)
	}

	// spendOutput returns a transaction spending output idx of tx.
	spendOutput := func(tx *bchutil.Tx, idx uint32, nOut int) *bchutil.Tx {
		op := wire.OutPoint{Hash: *tx.Hash(), Index: idx}
		return spend(t, harness, op, tx.MsgTx().TxOut[idx], nOut, 1000,
			txscript.SigHashAll, true)
	}
	coin := &wire.TxOut{Value: 100000000, PkScript: harness.payScript}
	split := spend(t, harness, addCoins(harness, coin)[0], coin, 2, 1000,
		txscript.SigHashAll, true)
	chain := []*bchutil.Tx{spendOutput(split, 0, 1)}
	chain = append(chain, spendOutput(chain[0], 0, 1))
	other := spendOutput(split, 1, 1)
	for _, tx := range append([]*bchutil.Tx{split, other}, chain...) {
		if _, err := harness.txPool.ProcessTransaction(tx, false, 0); err != nil {
			t.Fatalf("ProcessTransaction: %v", err)
		}
	}

	result := harness.txPool.RawMempoolVerbose()
	wantDepends := map[*bchutil.Tx][]string{
		split:    {},
		chain[0]: {split.Hash().String()},
		chain[1]: {chain[0].Hash().String()},
		other:    {split.Hash().String()},
	}
	if len(result) != len(wantDepends) {
		t.Fatalf("%d entries, want %d", len(result), len(wantDepends))
	}
	for tx, depends := range wantDepends {
		entry := result[tx.Hash().String()]
		desc := harness.txPool.pool[*tx.Hash()]
		if entry == nil || desc == nil {
			t.Fatalf("%v: no entry", tx.Hash())
		}
		if !slices.Equal(entry.Depends, depends) {
			t.Errorf("%v: depends %v, want %v", tx.Hash(), entry.Depends,
				depends)
		}
		if entry.Size != int32(tx.MsgTx().SerializeSize()) ||
			entry.Fee != bchutil.Amount(desc.Fee).ToBCH() ||
			entry.Time != desc.Added.Unix() ||
			entry.Height != int64(desc.Height) {

			t.Errorf("%v: entry %+v, descriptor %+v", tx.Hash(), entry,
				desc.TxDesc)
		}
	}
}

// TestRawMempoolVerboseOutsideLock ensures the verbose getrawmempool result
// is built after the mempool lock is released, so the lock is held for no
// more than part of the call, which is long for a large pool.
func TestRawMempoolVerboseOutsideLock(t *testing.T) {
	// Fill the pool directly with transactions, every other one spending
	// the one before.
	mp := New(&Config{})
	var prev chainhash.Hash
	for i := range 100000 {
		tx := wire.NewMsgTx(1)
		var external chainhash.Hash
		external[0], external[1], external[2] = byte(i), byte(i>>8), byte(i>>16)
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&external, 0), nil))
		if i%2 == 1 {
			tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prev, 0), nil))
		}
		tx.AddTxOut(wire.NewTxOut(1000, make([]byte, 25), wire.TokenData{}))
		btx := bchutil.NewTx(tx)
		mp.pool[*btx.Hash()] = &TxDesc{TxDesc: mining.TxDesc{
			Tx:    btx,
			Added: time.Now(),
		}}
		prev = *btx.Hash()
	}

	// Try the write lock over and over during a call, and keep the longest
	// time it was held, from a try that failed to the next one that
	// succeeded. A writer that waits for the lock would also count the
	// time it isn't scheduled, which on a busy machine is longer than the
	// lock is held.
	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		result := mp.RawMempoolVerbose()
		if len(result) != len(mp.pool) {
			t.Errorf("%d entries, want %d", len(result), len(mp.pool))
		}
		done <- time.Since(start)
	}()
	var longest, call time.Duration
	var heldSince time.Time
	for call == 0 {
		now := time.Now()
		if mp.mtx.TryLock() {
			mp.mtx.Unlock()
			if !heldSince.IsZero() {
				longest = max(longest, now.Sub(heldSince))
				heldSince = time.Time{}
			}
		} else if heldSince.IsZero() {
			heldSince = now
		}
		select {
		case call = <-done:
		default:
		}
	}

	// The call has returned, so a lock that was still held is released.
	if !heldSince.IsZero() {
		longest = max(longest, time.Since(heldSince))
	}

	// Building the result under the lock holds it for all of the call,
	// while a slow or busy machine can hold it longer than the copy of the
	// pool's descriptors takes, when it doesn't run the call then.
	t.Logf("lock held for at most %v during a call of %v", longest, call)
	if longest > call/2 {
		t.Errorf("the mempool lock was held for %v during a call of %v",
			longest, call)
	}
}

// TestMempoolEntrySpentBy ensures the verbose getrawmempool entry of a
// transaction lists each transaction of the pool spending its outputs once in
// spentby, in the order they entered the pool, and each it spends from the
// pool once in depends, as BCHN lists them, gives its fee as BCHN's base and
// modified fees, and that MempoolEntry gives the same entries.
func TestMempoolEntrySpentBy(t *testing.T) {
	t.Parallel()

	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create test pool: %v", err)
	}

	// The parent has three outputs.  One child spends the first two and
	// the other the third.
	coin := &wire.TxOut{Value: 100000000, PkScript: harness.payScript}
	parent := spend(t, harness, addCoins(harness, coin)[0], coin, 3, 1000,
		txscript.SigHashAll, true)
	outs := parent.MsgTx().TxOut
	both := wire.NewMsgTx(1)
	for i := range 2 {
		both.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: *parent.Hash(), Index: uint32(i)},
			Sequence:         wire.MaxTxInSequenceNum,
		})
	}
	both.AddTxOut(&wire.TxOut{Value: outs[0].Value + outs[1].Value - 1000,
		PkScript: harness.payScript})
	for i := range 2 {
		signP2PKH(t, both, i, outs[i], harness.signKey, txscript.SigHashAll,
			true)
	}
	bothTx := bchutil.NewTx(both)
	last := spend(t, harness, wire.OutPoint{Hash: *parent.Hash(), Index: 2},
		outs[2], 1, 1000, txscript.SigHashAll, true)

	// The children enter the pool in the order their txids do not sort
	// in, so the order of spentby is not that of the txids.
	children := []*bchutil.Tx{bothTx, last}
	if bothTx.Hash().String() < last.Hash().String() {
		children = []*bchutil.Tx{last, bothTx}
	}
	for _, tx := range append([]*bchutil.Tx{parent}, children...) {
		if _, err := harness.txPool.ProcessTransaction(tx, false, 0); err != nil {
			t.Fatalf("ProcessTransaction: %v", err)
		}
	}

	result := harness.txPool.RawMempoolVerbose()
	tests := []struct {
		tx      *bchutil.Tx
		depends []string
		spentBy []string
	}{
		{parent, []string{}, []string{children[0].Hash().String(),
			children[1].Hash().String()}},
		{bothTx, []string{parent.Hash().String()}, []string{}},
		{last, []string{parent.Hash().String()}, []string{}},
	}
	for _, test := range tests {
		entry := result[test.tx.Hash().String()]
		if entry == nil {
			t.Fatalf("%v: no entry", test.tx.Hash())
		}
		if !slices.Equal(entry.Depends, test.depends) ||
			!slices.Equal(entry.SpentBy, test.spentBy) {

			t.Errorf("%v: depends %v and spentby %v, want %v and %v",
				test.tx.Hash(), entry.Depends, entry.SpentBy,
				test.depends, test.spentBy)
		}
		fee := bchutil.Amount(1000).ToBCH()
		if entry.Fees.Base != fee || entry.Fees.Modified != fee ||
			entry.Fee != fee {

			t.Errorf("%v: fees %+v and fee %v, want %v", test.tx.Hash(),
				entry.Fees, entry.Fee, fee)
		}

		got, err := harness.txPool.MempoolEntry(test.tx.Hash())
		if err != nil || !reflect.DeepEqual(got, entry) {
			t.Errorf("%v: MempoolEntry %+v, %v, want %+v",
				test.tx.Hash(), got, err, entry)
		}
	}

	// A transaction that is not in the main pool has no entry.
	if _, err := harness.txPool.MempoolEntry(&chainhash.Hash{1}); err == nil {
		t.Error("MempoolEntry found a transaction not in the pool")
	}
}
