// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"errors"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// failingIndex is an index whose connection and disconnection of a block fail
// with err.
type failingIndex struct {
	*TxIndex
	err error
}

func (idx *failingIndex) ConnectBlock(database.Tx, *bchutil.Block, []blockchain.SpentTxOut) error {
	return idx.err
}

func (idx *failingIndex) DisconnectBlock(database.Tx, *bchutil.Block, []blockchain.SpentTxOut) error {
	return idx.err
}

// TestManagerIndexFailureNamesIndex ensures the index manager's error for an
// index that fails to connect or disconnect a block names the index, for the
// chain to log, and keeps the index's error and its text.
func TestManagerIndexFailureNamesIndex(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	broke := errors.New("the index broke")
	idx := &failingIndex{TxIndex: NewTxIndex(db), err: broke}
	block := bchutil.NewBlock(&wire.MsgBlock{})
	block.SetHeight(1)
	tip := block.MsgBlock().Header.PrevBlock
	err = db.Update(func(dbTx database.Tx) error {
		if _, err := dbTx.Metadata().CreateBucket(indexTipsBucketName); err != nil {
			return err
		}
		return dbPutIndexerTip(dbTx, idx.Key(), &tip, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(db, []Indexer{idx})
	check := func(what string, err error) {
		t.Helper()
		var named interface{ IndexName() string }
		if !errors.As(err, &named) || named.IndexName() != idx.Name() ||
			!errors.Is(err, broke) || err.Error() != broke.Error() {

			t.Errorf("%s: got %v, want %v naming the %s", what, err, broke,
				idx.Name())
		}
	}
	check("connecting", db.View(func(dbTx database.Tx) error {
		return m.ConnectBlock(dbTx, block, nil)
	}))

	// The index's tip is the block to disconnect.
	err = db.Update(func(dbTx database.Tx) error {
		return dbPutIndexerTip(dbTx, idx.Key(), block.Hash(), 1)
	})
	if err != nil {
		t.Fatal(err)
	}
	check("disconnecting", db.View(func(dbTx database.Tx) error {
		return m.DisconnectBlock(dbTx, block, nil)
	}))
}

// TestManagerCatchUpRollbackNamesIndex ensures the index manager's catch-up at
// startup names an index that fails to disconnect a block of a fork the chain
// left while the index was off, as it takes the index back to the main chain,
// for the chain's creation, which fails, to name it.
func TestManagerCatchUpRollbackNamesIndex(t *testing.T) {
	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	broke := errors.New("the index broke")
	idx := &failingIndex{TxIndex: NewTxIndex(db), err: broke}

	// The index's tip is a block of a fork, which the database holds but the
	// main chain does not.
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{0x51, 0x51},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	coinbase.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{0x51}})
	fork := bchutil.NewBlock(&wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   4,
			PrevBlock: *params.GenesisHash,
			Timestamp: params.GenesisBlock.Header.Timestamp.Add(time.Second),
			Bits:      params.PowLimitBits,
		},
		Transactions: []*wire.MsgTx{coinbase},
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
	var named interface{ IndexName() string }
	if !errors.As(err, &named) || named.IndexName() != idx.Name() || !errors.Is(err, broke) {
		t.Errorf("got %v, want %v naming the %s", err, broke, idx.Name())
	}
}
