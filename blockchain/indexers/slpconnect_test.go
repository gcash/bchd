// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
	"github.com/gcash/goslp/v1parser"
)

// slpEntryCorruptions are the ways the stored slp entry of a transaction can
// be corrupt: a value too short to read, one whose token has no metadata or a
// metadata record too short to read, and one whose slp message does not
// parse.
var slpEntryCorruptions = []struct {
	name    string
	corrupt func(dbTx database.Tx, hash chainhash.Hash, entry []byte) error
}{
	{"too short", func(dbTx database.Tx, hash chainhash.Hash, entry []byte) error {
		return dbTx.Metadata().Bucket(slpIndexKey).Put(hash[:], []byte{1})
	}},
	{"without token metadata", func(dbTx database.Tx, hash chainhash.Hash, entry []byte) error {
		return dbTx.Metadata().Bucket(tokenMetadataByIDIndexBucketName).Delete(entry[:4])
	}},
	{"with token metadata too short", func(dbTx database.Tx, hash chainhash.Hash, entry []byte) error {
		return dbTx.Metadata().Bucket(tokenMetadataByIDIndexBucketName).Put(entry[:4],
			make([]byte, 10))
	}},
	{"with a message that does not parse", func(dbTx database.Tx, hash chainhash.Hash, entry []byte) error {
		value := append(bytes.Clone(entry[:6]),
			bytes.Repeat([]byte{txscript.OP_RETURN}, 40)...)
		return dbTx.Metadata().Bucket(slpIndexKey).Put(hash[:], value)
	}},
}

// corruptSlpEntry corrupts the stored slp entry of the transaction hash the
// passed way.
func corruptSlpEntry(t *testing.T, db database.DB, hash chainhash.Hash,
	corrupt func(database.Tx, chainhash.Hash, []byte) error) {

	t.Helper()
	err := db.Update(func(dbTx database.Tx) error {
		entry := dbTx.Metadata().Bucket(slpIndexKey).Get(hash[:])
		if len(entry) < 6 {
			return errors.New("no entry")
		}
		return corrupt(dbTx, hash, bytes.Clone(entry))
	})
	if err != nil {
		t.Fatalf("corrupting the entry of %v: %v", hash, err)
	}
}

// spendingSlpTx returns a SEND of the token of the GENESIS genesis spending
// its output 1, when slp is set, and otherwise a transaction that is not slp
// spending it.
func spendingSlpTx(t *testing.T, genesis *wire.MsgTx, slp bool) *wire.MsgTx {
	t.Helper()
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: genesis.TxHash(), Index: 1},
		SignatureScript:  make([]byte, 60),
		Sequence:         wire.MaxTxInSequenceNum,
	})
	if slp {
		token, err := goslp.GetSlpTokenID(genesis)
		if err != nil {
			t.Fatal(err)
		}
		script, err := metadatamaker.TokenType1Send(token, []uint64{1000})
		if err != nil {
			t.Fatal(err)
		}
		tx.AddTxOut(&wire.TxOut{PkScript: script})
	}
	tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	return tx
}

// checkSlpEntryCorrupt checks that err is the error for the corrupt entry of
// the transaction hash, naming it and how to rebuild the index, and that it is
// no database.Error, which the chain answers with advice to rebuild the utxo
// set instead.
func checkSlpEntryCorrupt(t *testing.T, what string, err error, hash chainhash.Hash) {
	t.Helper()
	var corrupt errSlpEntryCorrupt
	if !errors.As(err, &corrupt) || corrupt.hash != hash {
		t.Errorf("%s: got %v, want the corrupt entry of %v", what, err, hash)
		return
	}
	for _, want := range []string{hash.String(), "--dropslpindex", "--slpindex"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q does not name %q", what, err, want)
		}
	}
	if _, ok := err.(database.Error); ok {
		t.Errorf("%s: error %q is a database.Error", what, err)
	}
}

// TestSlpConnectCorruptEntry ensures connecting a block to the slp index fails
// when the index holds the entry of a transaction an input spends but cannot
// read it or parse its slp message, whether the spending transaction is slp or
// not, rather than take the input for one that is not slp, which would store a
// SEND spending it as invalid for good, or one valid without it as valid with
// the input's tokens burned.
func TestSlpConnectCorruptEntry(t *testing.T) {
	spenders := []struct {
		name  string
		spend func(genesis, other *wire.MsgTx) *wire.MsgTx
	}{
		{"a SEND", func(genesis, _ *wire.MsgTx) *wire.MsgTx {
			return spendingSlpTx(t, genesis, true)
		}},
		{"a SEND of another token, valid without it", func(genesis, other *wire.MsgTx) *wire.MsgTx {
			tx := spendingSlpTx(t, other, true)
			tx.AddTxIn(&wire.TxIn{
				PreviousOutPoint: wire.OutPoint{Hash: genesis.TxHash(), Index: 1},
				Sequence:         wire.MaxTxInSequenceNum,
			})
			return tx
		}},
		{"a transaction that is not slp", func(genesis, _ *wire.MsgTx) *wire.MsgTx {
			return spendingSlpTx(t, genesis, false)
		}},
	}
	for _, c := range slpEntryCorruptions {
		for _, spender := range spenders {
			test := newSlpTokenIDTest(t)
			genesis, other := genesisTx(t, 1), genesisTx(t, 2)
			test.connect(genesis, other)
			corruptSlpEntry(t, test.db, genesis.TxHash(), c.corrupt)

			// The index reads the entry from the database, as after a
			// restart.
			test.restart()
			block := bchutil.NewBlock(&wire.MsgBlock{
				Transactions: []*wire.MsgTx{spender.spend(genesis, other)}})
			err := test.db.Update(func(dbTx database.Tx) error {
				return test.idx.ConnectBlock(dbTx, block, nil)
			})
			checkSlpEntryCorrupt(t, c.name+", spent by "+spender.name, err,
				genesis.TxHash())
		}
	}
}

// TestSlpConnectCorruptEntryStopsNode ensures a block whose connection the slp
// index fails on a corrupt entry is not connected and requests the node to
// shut down, as a database error while processing a block does, logging the
// failure as the slp index's with the advice to rebuild it rather than the
// utxo set.
func TestSlpConnectCorruptEntryStopsNode(t *testing.T) {
	var logged bytes.Buffer
	blockchain.UseLogger(bchlog.NewBackend(&logged).Logger("CHAN"))
	t.Cleanup(blockchain.DisableLog)

	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := NewSlpIndex(db, &SlpConfig{StartHash: params.SlpIndexStartHash,
		StartHeight: params.SlpIndexStartHeight, MaxCacheSize: 100})
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		IndexManager:       NewManager(db, []Indexer{idx}),
		UtxoCacheMaxSize:   1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatalf("blockchain.New: %v", err)
	}

	// mine processes a block after prev at height holding a coinbase with
	// the passed outputs and the passed transaction, which is not validated.
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	mine := func(prev chainhash.Hash, height int32, outs []*wire.TxOut,
		txs ...*wire.MsgTx) (*wire.MsgBlock, error) {

		sigScript, err := txscript.NewScriptBuilder().AddInt64(int64(height)).
			AddData(make([]byte, 40)).Script()
		if err != nil {
			t.Fatal(err)
		}
		coinbase := wire.NewMsgTx(1)
		coinbase.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
			SignatureScript:  sigScript,
			Sequence:         wire.MaxTxInSequenceNum,
		})
		for _, out := range outs {
			coinbase.AddTxOut(out)
		}
		msgTxs := append([]*wire.MsgTx{coinbase}, txs...)
		utilTxs := make([]*bchutil.Tx, len(msgTxs))
		for i, tx := range msgTxs {
			utilTxs[i] = bchutil.NewTx(tx)
		}
		merkles := blockchain.BuildMerkleTreeStore(utilTxs)
		block := &wire.MsgBlock{
			Header: wire.BlockHeader{
				Version:    4,
				PrevBlock:  prev,
				MerkleRoot: *merkles[len(merkles)-1],
				Timestamp:  start.Add(time.Duration(height) * time.Second),
				Bits:       params.PowLimitBits,
			},
			Transactions: msgTxs,
		}
		_, _, err = chain.ProcessBlock(bchutil.NewBlock(block),
			blockchain.BFFastAdd|blockchain.BFNoPoWCheck)
		return block, err
	}

	// Block 1's coinbase is a GENESIS, whose entry is then corrupt.
	genesisScript, err := metadatamaker.TokenType1Genesis([]byte("T"),
		[]byte("Token"), nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	block1, err := mine(*params.GenesisHash, 1, []*wire.TxOut{
		{PkScript: genesisScript},
		{Value: 1000, PkScript: []byte{txscript.OP_TRUE}},
	})
	if err != nil {
		t.Fatalf("block 1: %v", err)
	}
	genesis := block1.Transactions[0]
	corruptSlpEntry(t, db, genesis.TxHash(), slpEntryCorruptions[0].corrupt)
	idx.cache = InitSlpCache(100)

	// Block 2 holds a SEND spending its tokens.
	_, err = mine(block1.BlockHash(), 2, []*wire.TxOut{
		{Value: 1000, PkScript: []byte{txscript.OP_TRUE}},
	}, spendingSlpTx(t, genesis, true))
	checkSlpEntryCorrupt(t, "block 2", err, genesis.TxHash())
	if height := chain.BestSnapshot().Height; height != 1 {
		t.Errorf("the chain is at height %d, want 1", height)
	}
	select {
	case <-chain.RequestedProcessShutdown():
	default:
		t.Error("no shutdown requested")
	}
	if !strings.Contains(logged.String(), "--dropslpindex") ||
		strings.Contains(logged.String(), "--reindexchainstate") {

		t.Errorf("log %q, want the advice to rebuild the slp index only",
			logged.String())
	}
	want := "[CRT] CHAN: Failed to update the slp index: the slp index entry " +
		"of transaction " + genesis.TxHash().String() + " is corrupt"
	if !strings.Contains(logged.String(), want) ||
		strings.Contains(logged.String(), "Failed to write the chain state") {

		t.Errorf("log %q, want a line with %q and none naming the chain state",
			logged.String(), want)
	}
}

// TestSlpTokenMetadataRecordLength ensures a token metadata record of a length
// the format does not have is read as corrupt, rather than sliced, which
// panics for one under 32 bytes, while the records the index writes, of a token
// with neither a mint baton nor a group, of one with a mint baton, and of an
// NFT1 child token with its group, are read back, with their token type.
func TestSlpTokenMetadataRecordLength(t *testing.T) {
	test := newSlpTokenIDTest(t)
	token, baton, group := chainhash.Hash{1}, chainhash.Hash{2}, chainhash.Hash{3}
	for i, tm := range []TokenMetadata{
		{TokenID: &token, SlpVersion: v1parser.TokenTypeFungible01},
		{TokenID: &token, SlpVersion: v1parser.TokenTypeFungible01,
			MintBatonHash: &baton, MintBatonVout: 2},
		{TokenID: &token, SlpVersion: v1parser.TokenTypeNft1Child41,
			NftGroupID: &group},
	} {
		var got *TokenMetadata
		err := test.db.Update(func(dbTx database.Tx) error {
			if err := dbPutTokenIDIndexEntry(dbTx, uint32(i+1), &tm); err != nil {
				return err
			}
			var err error
			got, err = dbFetchTokenMetadataByID(dbTx, uint32(i+1))
			return err
		})
		if err != nil || *got.TokenID != token || got.SlpVersion != tm.SlpVersion ||
			(got.MintBatonHash == nil) != (tm.MintBatonHash == nil) ||
			got.MintBatonVout != tm.MintBatonVout ||
			(got.NftGroupID == nil) != (tm.NftGroupID == nil) {

			t.Errorf("record %d: got %+v and %v, want %+v", i, got, err, tm)
		}
	}

	for _, n := range []int{1, 31, 32, 33, 35, 65, 67, 69, 71} {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			err = test.db.Update(func(dbTx database.Tx) error {
				err := dbTx.Metadata().Bucket(tokenMetadataByIDIndexBucketName).Put(
					[]byte{9, 0, 0, 0}, make([]byte, n))
				if err != nil {
					return err
				}
				_, err = dbFetchTokenMetadataByID(dbTx, 9)
				return err
			})
		}()
		var dbErr database.Error
		if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrCorruption {
			t.Errorf("a record of %d bytes: got %v, want it corrupt", n, err)
		}
	}
}
