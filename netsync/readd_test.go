// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"bytes"
	"math"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// anyoneCanSpendTx returns a transaction spending the outpoint to one output
// anyone can spend.  The output is padded to the minimum transaction size.
func anyoneCanSpendTx(prevOut wire.OutPoint, value int64) *bchutil.Tx {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(wire.NewTxIn(&prevOut, nil))
	script := append(bytes.Repeat([]byte{txscript.OP_NOP}, 100), txscript.OP_TRUE)
	tx.AddTxOut(wire.NewTxOut(value, script, wire.TokenData{}))
	return bchutil.NewTx(tx)
}

// TestDisconnectedBlockReaddedParentsFirst ensures the transactions of a
// disconnected block go back into the memory pool even when a child sorts
// before its parent in the block, as it does in canonical order.
func TestDisconnectedBlockReaddedParentsFirst(t *testing.T) {
	params := &chaincfg.RegressionNetParams
	fund := bchutil.NewTx(&wire.MsgTx{Version: 1, TxOut: []*wire.TxOut{{
		Value: 100000000, PkScript: []byte{txscript.OP_TRUE},
	}}})
	coin := wire.OutPoint{Hash: *fund.Hash()}

	// A parent and a child whose txid sorts before the parent's.
	parent := anyoneCanSpendTx(coin, 90000000)
	var child *bchutil.Tx
	for value := int64(80000000); ; value-- {
		child = anyoneCanSpendTx(wire.OutPoint{Hash: *parent.Hash()}, value)
		if bytes.Compare(child.Hash()[:], parent.Hash()[:]) < 0 {
			break
		}
	}

	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			MaxTxVersion:  2,
			AcceptNonStd:  true,
			MinRelayTxFee: 1000,
		},
		ChainParams: params,
		FetchUtxoView: func(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			view := blockchain.NewUtxoViewpoint()
			for _, in := range tx.MsgTx().TxIn {
				if in.PreviousOutPoint == coin {
					view.AddTxOut(fund, 0, 1)
				}
			}
			return view, nil
		},
		BestHeight:     func() int32 { return 1000 },
		MedianTimePast: func() time.Time { return time.Now() },
		CalcSequenceLock: func(*bchutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return &blockchain.SequenceLock{Seconds: -1, BlockHeight: -1}, nil
		},
		IsDeploymentActive: func(uint32) (bool, error) { return true, nil },
	})
	sm := &SyncManager{txMemPool: pool}

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, math.MaxUint32), nil))
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{
		coinbase, child.MsgTx(), parent.MsgTx(),
	}})
	sm.handleBlockchainNotification(&blockchain.Notification{
		Type: blockchain.NTBlockDisconnected,
		Data: block,
	})

	for _, tx := range []*bchutil.Tx{parent, child} {
		if !pool.IsTransactionInPool(tx.Hash()) {
			t.Errorf("transaction %v was not added back", tx.Hash())
		}
	}
}

// readdTestChain holds the unspent outputs of a test chain, each with the
// transaction that created it.
type readdTestChain map[wire.OutPoint]*bchutil.Tx

// newReaddTestManager returns a sync manager whose memory pool sees the
// outputs of the chain, which the caller may change, and accepts nonstandard
// transactions and orphans.  As in the chain's view, an output that is not in
// the chain has a nil entry.
func newReaddTestManager(chain readdTestChain) *SyncManager {
	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			MaxTxVersion:    2,
			AcceptNonStd:    true,
			MinRelayTxFee:   1000,
			MaxOrphanTxs:    10,
			MaxOrphanTxSize: 100000,
		},
		ChainParams: &chaincfg.RegressionNetParams,
		FetchUtxoView: func(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			view := blockchain.NewUtxoViewpoint()
			for _, in := range tx.MsgTx().TxIn {
				prev, ok := chain[in.PreviousOutPoint]
				if !ok {
					view.Entries()[in.PreviousOutPoint] = nil
					continue
				}
				view.AddTxOut(prev, in.PreviousOutPoint.Index, 1)
			}
			return view, nil
		},
		BestHeight:     func() int32 { return 1000 },
		MedianTimePast: func() time.Time { return time.Now() },
		CalcSequenceLock: func(*bchutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return &blockchain.SequenceLock{Seconds: -1, BlockHeight: -1}, nil
		},
		IsDeploymentActive: func(uint32) (bool, error) { return true, nil },
	})
	return &SyncManager{txMemPool: pool}
}

// disconnectTestBlock tells the sync manager that a block holding a coinbase
// and the transactions was disconnected.
func disconnectTestBlock(sm *SyncManager, txs ...*bchutil.Tx) {
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, math.MaxUint32), nil))
	msgTxs := []*wire.MsgTx{coinbase}
	for _, tx := range txs {
		msgTxs = append(msgTxs, tx.MsgTx())
	}
	sm.handleBlockchainNotification(&blockchain.Notification{
		Type: blockchain.NTBlockDisconnected,
		Data: bchutil.NewBlock(&wire.MsgBlock{Transactions: msgTxs}),
	})
}

// TestDisconnectedBlockRefusedParent ensures that when the memory pool refuses
// a transaction of a disconnected block, the block's transaction spending it
// is kept out with the memory pool's transactions spending that one, so none
// is left spending an output that exists nowhere.
func TestDisconnectedBlockRefusedParent(t *testing.T) {
	// The parent spends an output whose script holds an upgradable NOP,
	// which blocks allow and the memory pool's policy refuses.
	fund := bchutil.NewTx(&wire.MsgTx{Version: 1, TxOut: []*wire.TxOut{{
		Value:    100000000,
		PkScript: []byte{txscript.OP_NOP10, txscript.OP_TRUE},
	}}})
	coin := wire.OutPoint{Hash: *fund.Hash()}
	parent := anyoneCanSpendTx(coin, 90000000)
	child := anyoneCanSpendTx(wire.OutPoint{Hash: *parent.Hash()}, 80000000)
	spender := anyoneCanSpendTx(wire.OutPoint{Hash: *child.Hash()}, 70000000)

	// The spender enters the memory pool while the block is connected.
	childOut := wire.OutPoint{Hash: *child.Hash()}
	chain := readdTestChain{childOut: child}
	sm := newReaddTestManager(chain)
	pool := sm.txMemPool
	if _, err := pool.ProcessTransaction(spender, false, 0); err != nil {
		t.Fatalf("spender not accepted: %v", err)
	}

	delete(chain, childOut)
	chain[coin] = fund
	disconnectTestBlock(sm, child, parent)
	for name, tx := range map[string]*bchutil.Tx{"parent": parent,
		"child": child, "spender": spender} {

		if pool.IsTransactionInPool(tx.Hash()) {
			t.Errorf("the %s is in the pool", name)
		}
	}
}

// TestDisconnectedBlockOrphan ensures a transaction of a disconnected block
// goes back into the memory pool when it is also in the orphan pool, as a
// confirmed transaction sent again once its outputs are spent can be, and
// that the memory pool's transactions spending it stay.
func TestDisconnectedBlockOrphan(t *testing.T) {
	fund := bchutil.NewTx(&wire.MsgTx{Version: 1, TxOut: []*wire.TxOut{{
		Value: 100000000, PkScript: []byte{txscript.OP_TRUE},
	}}})
	coin := wire.OutPoint{Hash: *fund.Hash()}
	tx := anyoneCanSpendTx(coin, 90000000)
	spend := anyoneCanSpendTx(wire.OutPoint{Hash: *tx.Hash()}, 80000000)

	// With the coin and the transaction's output spent, the transaction
	// sent again is an orphan.
	chain := readdTestChain{}
	sm := newReaddTestManager(chain)
	pool := sm.txMemPool
	_, err := pool.ProcessTransaction(tx, true, 0)
	if err != nil || !pool.IsOrphanInPool(tx.Hash()) {
		t.Fatalf("transaction not an orphan: %v", err)
	}

	// The block spending its output is disconnected, then its own.
	txOut := wire.OutPoint{Hash: *tx.Hash()}
	chain[txOut] = tx
	disconnectTestBlock(sm, spend)
	if !pool.IsTransactionInPool(spend.Hash()) {
		t.Fatal("the spend was not added back")
	}
	delete(chain, txOut)
	chain[coin] = fund
	disconnectTestBlock(sm, tx)
	for name, tx := range map[string]*bchutil.Tx{"transaction": tx,
		"spend": spend} {

		if !pool.IsTransactionInPool(tx.Hash()) {
			t.Errorf("the %s is not in the pool", name)
		}
	}
	if pool.IsOrphanInPool(tx.Hash()) {
		t.Error("the transaction is still an orphan")
	}
}

// TestParentsFirst ensures each transaction is put after those of the list it
// spends, across generations, and that the order is kept otherwise.
func TestParentsFirst(t *testing.T) {
	tx := func(prev chainhash.Hash, tag byte) *bchutil.Tx {
		msg := wire.NewMsgTx(2)
		msg.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: prev}, []byte{tag}))
		return bchutil.NewTx(msg)
	}
	a := tx(chainhash.Hash{1}, 1)
	b := tx(*a.Hash(), 2)
	c := tx(*b.Hash(), 3)
	other := tx(chainhash.Hash{2}, 4)

	got := parentsFirst([]*bchutil.Tx{c, other, b, a})
	want := []*bchutil.Tx{a, b, c, other}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d holds %v, want %v", i, got[i].Hash(),
				want[i].Hash())
		}
	}
}

// TestParentsFirstInputOrder ensures the parents of a transaction are put in
// the order of its inputs.
func TestParentsFirstInputOrder(t *testing.T) {
	tx := func(tag byte, prevs ...chainhash.Hash) *bchutil.Tx {
		msg := wire.NewMsgTx(2)
		for _, prev := range prevs {
			msg.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: prev},
				[]byte{tag}))
		}
		return bchutil.NewTx(msg)
	}
	a := tx(1, chainhash.Hash{1})
	b := tx(2, chainhash.Hash{2})
	c := tx(3, *b.Hash(), chainhash.Hash{3}, *a.Hash())

	got := parentsFirst([]*bchutil.Tx{c, a, b})
	want := []*bchutil.Tx{b, a, c}
	if len(got) != len(want) {
		t.Fatalf("%d transactions, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d holds %v, want %v", i, got[i].Hash(),
				want[i].Hash())
		}
	}
}

// TestParentsFirstSharedParent ensures a transaction spent by two others, and
// one listed before the transactions spending it, are each put once.
func TestParentsFirstSharedParent(t *testing.T) {
	tx := func(tag byte, prevs ...chainhash.Hash) *bchutil.Tx {
		msg := wire.NewMsgTx(2)
		for _, prev := range prevs {
			msg.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: prev},
				[]byte{tag}))
		}
		return bchutil.NewTx(msg)
	}
	a := tx(1, chainhash.Hash{1})
	b := tx(2, *a.Hash())
	c := tx(3, *b.Hash(), *a.Hash())

	got := parentsFirst([]*bchutil.Tx{a, c, b})
	want := []*bchutil.Tx{a, b, c}
	if len(got) != len(want) {
		t.Fatalf("%d transactions, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d holds %v, want %v", i, got[i].Hash(),
				want[i].Hash())
		}
	}
}

// TestParentsFirstLongChain ensures putting the transactions of a block that
// form one long chain in order takes no stack for each transaction.  The order
// was found recursively, with a stack frame for each transaction of the chain,
// so the longest chain a 32 MB block holds grew the stack to 128 MiB, and one
// in a block of about 240 MB passed Go's stack limit, which ends the process.
func TestParentsFirstLongChain(t *testing.T) {
	const n = 100000
	chain := make([]*bchutil.Tx, n)
	prev := chainhash.Hash{1}
	for i := range chain {
		msg := wire.NewMsgTx(2)
		msg.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: prev}, nil))
		chain[i] = bchutil.NewTx(msg)
		prev = *chain[i].Hash()
	}
	// The block holds the chain children first.
	txs := make([]*bchutil.Tx, n)
	for i, tx := range chain {
		txs[n-1-i] = tx
	}

	// No collection runs meanwhile, which could shrink the stack.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var sorted []*bchutil.Tx
	var grown int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		sorted = parentsFirst(txs)
		runtime.ReadMemStats(&after)
		grown = int64(after.StackInuse) - int64(before.StackInuse)
	}()
	<-done

	if grown > 1<<20 {
		t.Errorf("the stack grew by %d bytes for a chain of %d "+
			"transactions", grown, n)
	}
	if len(sorted) != n {
		t.Fatalf("%d transactions, want %d", len(sorted), n)
	}
	for i, tx := range sorted {
		if tx != chain[i] {
			t.Fatalf("position %d holds %v, want %v", i, tx.Hash(),
				chain[i].Hash())
		}
	}
}
