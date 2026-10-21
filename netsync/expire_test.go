// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
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

// TestBlockConnectedExpiresMempool ensures a connected block expires the
// transactions that have been in the memory pool too long, such as one a
// reorg put back paying no fee, which is never mined.
func TestBlockConnectedExpiresMempool(t *testing.T) {
	fund := bchutil.NewTx(&wire.MsgTx{Version: 1, TxOut: []*wire.TxOut{{
		Value: 100000000, PkScript: []byte{txscript.OP_TRUE},
	}}})
	tx := anyoneCanSpendTx(wire.OutPoint{Hash: *fund.Hash()}, 100000000)
	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			MaxTxVersion:  2,
			AcceptNonStd:  true,
			MinRelayTxFee: 1000,
		},
		ChainParams: &chaincfg.RegressionNetParams,
		FetchUtxoView: func(*bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			view := blockchain.NewUtxoViewpoint()
			view.AddTxOut(fund, 0, 1)
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
	sm.handleBlockchainNotification(&blockchain.Notification{
		Type: blockchain.NTBlockDisconnected,
		Data: bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{
			coinbase, tx.MsgTx(),
		}}),
	})
	desc, err := pool.FetchTxDesc(tx.Hash())
	if err != nil {
		t.Fatalf("transaction was not added back: %v", err)
	}
	desc.Added = time.Now().Add(-15 * 24 * time.Hour)

	sm.handleBlockchainNotification(&blockchain.Notification{
		Type: blockchain.NTBlockConnected,
		Data: bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{coinbase}}),
	})
	if pool.IsTransactionInPool(tx.Hash()) {
		t.Fatal("transaction in the pool for 15 days was not expired")
	}
}
