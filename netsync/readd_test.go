// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"bytes"
	"math"
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
