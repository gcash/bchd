// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestRollbackWithoutSpendJournal ensures that an index that needs no spent
// outputs can roll back from a block of a fork that the chain left while the
// index was off, whose spend journal the flush after the reorganization
// removed.  It stopped the node from starting.
func TestRollbackWithoutSpendJournal(t *testing.T) {
	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	idx := NewBcmrIndex(db, &params)

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{0x51, 0x51},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	coinbase.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{0x51}})
	spend := wire.NewMsgTx(1)
	spend.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{1}, 1), nil))
	spend.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{0x51}})
	fork := bchutil.NewBlock(&wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   4,
			PrevBlock: *params.GenesisHash,
			Timestamp: params.GenesisBlock.Header.Timestamp.Add(time.Second),
			Bits:      params.PowLimitBits,
		},
		Transactions: []*wire.MsgTx{coinbase, spend},
	})
	err = db.Update(func(dbTx database.Tx) error {
		if err := dbTx.StoreBlock(fork); err != nil {
			return err
		}
		if _, err := dbTx.Metadata().CreateBucket(indexTipsBucketName); err != nil {
			return err
		}
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		return dbPutIndexerTip(dbTx, idx.Key(), fork.Hash(), 1)
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		IndexManager:       NewManager(db, []Indexer{idx}),
		UtxoCacheMaxSize:   1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatalf("starting with the index: %v", err)
	}
}
