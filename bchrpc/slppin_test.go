// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"context"
	"testing"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/goslp/metadatamaker"
)

// TestSlpTxBackInPoolStaysValid ensures a transaction that leaves the memory
// pool and comes back stays valid slp to clients while it is in the pool,
// however many entries the slp cache's LRU takes meanwhile.  Its entry was not
// put with those of the memory pool's transactions again while the LRU still
// held it, so once the LRU evicted it, GetSlpTrustedValidation refused its
// outputs and the transaction and its outputs showed as not slp.
func TestSlpTxBackInPoolStaysValid(t *testing.T) {
	s := newDispatcherTestServer(t)
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.chainParams = &chaincfg.RegressionNetParams
	s.db, s.slpIndex = db, idx
	// The memory pool holds none of the transactions, so a block event
	// finds each of them gone.
	s.txMemPool = mempool.New(&mempool.Config{})
	go s.slpEventHandler()
	waitFor(t, "the slp event handler to subscribe", func() bool {
		return s.numSubscriptions() == 1
	})

	// genesisTx returns GENESIS n of a token of 1000.
	genesisTx := func(n byte) *wire.MsgTx {
		t.Helper()
		script, err := metadatamaker.TokenType1Genesis([]byte("T"),
			[]byte{n}, nil, nil, 0, nil, 1000)
		if err != nil {
			t.Fatal(err)
		}
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee, n}}, nil))
		tx.AddTxOut(&wire.TxOut{PkScript: script})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		return tx
	}
	accepted := func(tx *wire.MsgTx) {
		s.dispatchEvent(&rpcEventTxAccepted{TxDesc: &mempool.TxDesc{
			TxDesc: mining.TxDesc{Tx: bchutil.NewTx(tx)}}})
	}
	validate := func(tx *wire.MsgTx) (*pb.GetSlpTrustedValidationResponse, error) {
		hash := tx.TxHash()
		return s.GetSlpTrustedValidation(context.Background(),
			&pb.GetSlpTrustedValidationRequest{
				Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
					PrevOutHash: hash[:],
					PrevOutVout: 1,
				}},
			})
	}

	// The GENESIS comes to the memory pool, leaves it, as when it expires,
	// so the next block's event finds it gone, and comes back, as when its
	// wallet sends it again.  Then as many other transactions come to the
	// memory pool as the LRU holds.
	genesis := genesisTx(0)
	accepted(genesis)
	s.dispatchEvent(&rpcEventBlockConnected{Block: bchutil.NewBlock(&wire.MsgBlock{})})
	accepted(genesis)
	var last *wire.MsgTx
	for i := 1; i <= 100; i++ {
		last = genesisTx(byte(i))
		accepted(last)
	}
	waitFor(t, "the slp event handler to check the transactions", func() bool {
		_, err := validate(last)
		return err == nil
	})

	resp, err := validate(genesis)
	if err != nil {
		t.Errorf("GetSlpTrustedValidation of the GENESIS back in the memory "+
			"pool: %v", err)
	} else if got := resp.Results[0].GetV1TokenAmount(); got != 1000 {
		t.Errorf("GetSlpTrustedValidation: amount %d, want 1000", got)
	}
	tx := marshalTransaction(bchutil.NewTx(genesis), 0, nil, 0, s)
	if got := tx.SlpTransactionInfo.ValidityJudgement; got !=
		pb.SlpTransactionInfo_VALID {

		t.Errorf("the GENESIS shows as %v, want valid", got)
	}
	if token := tx.Outputs[1].SlpToken; token == nil || token.Amount != 1000 {
		t.Errorf("output 1 shows token %v, want 1000", token)
	}
}
