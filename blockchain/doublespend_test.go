// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
)

// requireRuleError fails the test unless err is a RuleError with the given
// error code.
func requireRuleError(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != code {
		t.Fatalf("got error %v, want %v", err, code)
	}
}

// TestInBlockDoubleSpend ensures a block in which an output is spent twice
// is rejected both with and without canonical transaction ordering, while
// a block spending an output created in the same block once is accepted.
func TestInBlockDoubleSpend(t *testing.T) {
	tests := []struct {
		name        string
		magneticAct int32
	}{
		{name: "ctor", magneticAct: 0},
		{name: "ttor", magneticAct: 1000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := chaincfg.RegressionNetParams
			params.MagneticAnonomalyForkHeight = test.magneticAct
			m := newRuleTestMiner(t, &params)

			// Without canonical ordering the transactions are kept in
			// the given, topological, order.
			build := m.build
			if test.magneticAct != 0 {
				build = m.buildInOrder
			}

			b1 := build(nil)
			m.mustAccept(b1)
			cb := b1.MsgBlock().Transactions[0]
			prevOut := wire.OutPoint{Hash: cb.TxHash(), Index: 0}
			value := cb.TxOut[0].Value - 1000

			// Two transactions spending the same output.
			txA := ruleTestSpend(prevOut, value, 1)
			txB := ruleTestSpend(prevOut, value, 2)
			err := m.accept(build(nil, txA, txB))
			requireRuleError(t, err, ErrSpentTxOut)

			// One transaction spending the same output twice.
			txC := ruleTestSpend(prevOut, value, 3)
			txC.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut,
				Sequence: 0xffffffff})
			err = m.accept(build(nil, txC))
			requireRuleError(t, err, ErrDuplicateTxInputs)

			// An output created in the block and spent twice in it.
			parent := ruleTestSpend(prevOut, value, 4)
			parentOut := wire.OutPoint{Hash: parent.TxHash(), Index: 0}
			child1 := ruleTestSpend(parentOut, value-1000, 5)
			child2 := ruleTestSpend(parentOut, value-1000, 6)
			err = m.accept(build(nil, parent, child1, child2))
			requireRuleError(t, err, ErrSpentTxOut)

			// The output created in the block and spent once.
			b2 := build(nil, parent, child1)
			m.mustAccept(b2)

			// A side chain block with a double spend is rejected when
			// the side chain becomes the best chain.
			m.tip, m.height = b1.MsgBlock(), b1.Height()
			txD := ruleTestSpend(prevOut, value, 7)
			txE := ruleTestSpend(prevOut, value, 8)
			b2Side := build(nil, txD, txE)
			if _, _, err := m.chain.ProcessBlock(b2Side, BFNone); err != nil {
				t.Fatalf("side chain block rejected: %v", err)
			}
			m.tip, m.height = b2Side.MsgBlock(), b2Side.Height()
			err = m.accept(build(nil))
			requireRuleError(t, err, ErrSpentTxOut)
			if tip := m.chain.BestSnapshot().Hash; tip != *b2.Hash() {
				t.Fatalf("best chain tip %v, want %v", tip, b2.Hash())
			}
		})
	}
}

// txBefore reports whether a sorts before b in canonical order.
func txBefore(a, b *wire.MsgTx) bool {
	ha, hb := a.TxHash(), b.TxHash()
	return ha.Compare(&hb) < 0
}

// TestCanonicalSpendBeforeParent ensures that with canonical ordering a
// transaction sorted before the in-block transaction it spends from is
// accepted, its fees are counted, and the spent outputs are removed from the
// utxo set, while two such spends of one output are rejected.
func TestCanonicalSpendBeforeParent(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.MagneticAnonomalyForkHeight = 0
	m := newRuleTestMiner(t, &params)

	b1 := m.build(nil)
	m.mustAccept(b1)
	cb := b1.MsgBlock().Transactions[0]
	prevOut := wire.OutPoint{Hash: cb.TxHash(), Index: 0}
	value := cb.TxOut[0].Value - 1000

	// Find grandparent, parent and child sorted in reverse of the order
	// they spend each other.
	var grandparent, parent, child *wire.MsgTx
	found := false
	for tg := 10; tg < 250 && !found; tg++ {
		grandparent = ruleTestSpend(prevOut, value, byte(tg))
		gpOut := wire.OutPoint{Hash: grandparent.TxHash()}
		for tp := 10; tp < 250 && !found; tp++ {
			parent = ruleTestSpend(gpOut, value-1000, byte(tp))
			if !txBefore(parent, grandparent) {
				continue
			}
			pOut := wire.OutPoint{Hash: parent.TxHash()}
			for tc := 10; tc < 250; tc++ {
				child = ruleTestSpend(pOut, value-2000, byte(tc))
				if txBefore(child, parent) {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Fatal("no transactions in reverse order found")
	}
	gpOut := wire.OutPoint{Hash: grandparent.TxHash()}
	parentOut := wire.OutPoint{Hash: parent.TxHash()}
	childOut := wire.OutPoint{Hash: child.TxHash()}

	// Two transactions sorted before the parent spending its output.
	var child2 *wire.MsgTx
	for tc := 10; tc < 250; tc++ {
		child2 = ruleTestSpend(parentOut, value-2000, byte(tc))
		if txBefore(child2, parent) && child2.TxHash() != child.TxHash() {
			break
		}
	}
	err := m.accept(m.build(nil, grandparent, parent, child, child2))
	requireRuleError(t, err, ErrSpentTxOut)

	// The coinbase may claim the fees of all three transactions, but no
	// more.
	const fees = 3000
	height := m.height + 1
	cbOver := m.coinbase(height)
	cbOver.TxOut[0].Value += fees + 1
	over := m.build(cbOver, grandparent, parent, child)
	err = m.chain.CheckConnectBlockTemplate(over)
	requireRuleError(t, err, ErrBadCoinbaseValue)
	err = m.accept(over)
	requireRuleError(t, err, ErrBadCoinbaseValue)

	cbExact := m.coinbase(height)
	cbExact.TxOut[0].Value += fees
	b2 := m.build(cbExact, grandparent, parent, child)
	txs := b2.MsgBlock().Transactions
	if txs[1].TxHash() != child.TxHash() ||
		txs[3].TxHash() != grandparent.TxHash() {
		t.Fatal("transactions not in reverse order")
	}
	if err := m.chain.CheckConnectBlockTemplate(b2); err != nil {
		t.Fatalf("block template rejected: %v", err)
	}
	m.mustAccept(b2)

	// Only the child's output is left unspent.
	for _, op := range []wire.OutPoint{prevOut, gpOut, parentOut} {
		entry, err := m.chain.FetchUtxoEntry(op)
		if err != nil {
			t.Fatalf("FetchUtxoEntry(%v): %v", op, err)
		}
		if entry != nil && !entry.IsSpent() {
			t.Fatalf("output %v left unspent", op)
		}
	}
	entry, err := m.chain.FetchUtxoEntry(childOut)
	if err != nil || entry == nil || entry.IsSpent() {
		t.Fatalf("child output not unspent: %v, %v", entry, err)
	}

	// Spending the parent's output again in the next block is rejected.
	respend := ruleTestSpend(parentOut, value-2000, 1)
	err = m.accept(m.build(nil, respend))
	var rerr RuleError
	if !errors.As(err, &rerr) || (rerr.ErrorCode != ErrMissingTxOut &&
		rerr.ErrorCode != ErrSpentTxOut) {
		t.Fatalf("got error %v, want ErrMissingTxOut or ErrSpentTxOut", err)
	}
}
