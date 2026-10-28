// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
	"github.com/gcash/goslp/v1parser"
)

// slpTokenIDTest is an slp index on a new database that connects and
// disconnects blocks of transactions and opens the index again, as a restart
// does.
type slpTokenIDTest struct {
	t   *testing.T
	db  database.DB
	idx *SlpIndex
}

func newSlpTokenIDTest(t *testing.T) *slpTokenIDTest {
	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	err = db.Update(func(dbTx database.Tx) error {
		idx := NewSlpIndex(db, &SlpConfig{MaxCacheSize: 100})
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		_, err := dbTx.Metadata().CreateBucketIfNotExists(indexTipsBucketName)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	test := &slpTokenIDTest{t: t, db: db}
	test.restart()
	return test
}

// restart opens the index again, with an empty cache.
func (test *slpTokenIDTest) restart() {
	test.t.Helper()
	test.idx = NewSlpIndex(test.db, &SlpConfig{MaxCacheSize: 100})
	if err := test.idx.Init(); err != nil {
		test.t.Fatalf("Init: %v", err)
	}
}

// connect connects a block of the passed transactions to the index.
func (test *slpTokenIDTest) connect(txs ...*wire.MsgTx) {
	test.t.Helper()
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: txs})
	err := test.db.Update(func(dbTx database.Tx) error {
		return test.idx.ConnectBlock(dbTx, block, nil)
	})
	if err != nil {
		test.t.Fatalf("ConnectBlock: %v", err)
	}
}

// disconnect disconnects a block of the passed transactions from the index.
func (test *slpTokenIDTest) disconnect(txs ...*wire.MsgTx) {
	test.t.Helper()
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: txs})
	err := test.db.Update(func(dbTx database.Tx) error {
		return test.idx.DisconnectBlock(dbTx, block, nil)
	})
	if err != nil {
		test.t.Fatalf("DisconnectBlock: %v", err)
	}
}

// tokenOf returns the token ID that the index database records for tx.
func (test *slpTokenIDTest) tokenOf(tx *wire.MsgTx) (chainhash.Hash, error) {
	var entry *SlpTxEntry
	hash := tx.TxHash()
	err := test.db.View(func(dbTx database.Tx) error {
		var err error
		entry, err = dbFetchSlpIndexEntry(dbTx, &hash)
		return err
	})
	if err != nil {
		return chainhash.Hash{}, err
	}
	return entry.TokenIDHash, nil
}

// slpTx returns a transaction whose first output holds the slp script, and
// that spends made-up output n, so that each n gives another transaction.
func slpTx(t *testing.T, n byte, script []byte, err error) *wire.MsgTx {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee, n}}, nil))
	tx.AddTxOut(&wire.TxOut{PkScript: script})
	tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{0x51}})
	return tx
}

// genesisTx returns GENESIS n of a type 1 token.
func genesisTx(t *testing.T, n byte) *wire.MsgTx {
	t.Helper()
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte{n},
		nil, nil, 0, nil, 1000)
	return slpTx(t, n, script, err)
}

// tokenIDOf returns the token ID of the slp transaction tx.
func tokenIDOf(t *testing.T, tx *wire.MsgTx) chainhash.Hash {
	t.Helper()
	id, err := goslp.GetSlpTokenID(tx)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := chainhash.NewHash(id)
	if err != nil {
		t.Fatal(err)
	}
	return *hash
}

// TestSlpZeroSendTokenID ensures a SEND of zero tokens of a token that has no
// GENESIS, which is valid, gets an internal token ID of its own, so that once
// the index is opened again its entry still names its token, the graph search
// map can be built, and a later GENESIS does not take the ID.  A SEND naming
// a GENESIS that comes after it shares the GENESIS's ID.
func TestSlpZeroSendTokenID(t *testing.T) {
	unknown := bytes.Repeat([]byte{0x77}, 32)
	for _, typ := range []struct {
		name string
		send func([]byte, []uint64) ([]byte, error)
	}{
		{"type 1", metadatamaker.TokenType1Send},
		{"nft1 child", metadatamaker.NFT1ChildSend},
	} {
		test := newSlpTokenIDTest(t)
		script, err := typ.send(unknown, []uint64{0})
		zero := slpTx(t, 1, script, err)
		test.connect(zero)
		test.restart()
		got, err := test.tokenOf(zero)
		if err != nil || !bytes.Equal(got[:], unknown) {
			t.Errorf("%s: SEND names token %v, %v; want %x", typ.name, got,
				err, unknown)
		}
		if _, err := test.idx.buildGraphSearchTxnList(); err != nil {
			t.Errorf("%s: graph search list: %v", typ.name, err)
		}

		genesis := genesisTx(t, 2)
		test.connect(genesis)
		test.restart()
		got, err = test.tokenOf(zero)
		if err != nil || !bytes.Equal(got[:], unknown) {
			t.Errorf("%s: after a GENESIS, SEND names token %v, %v; want %x",
				typ.name, got, err, unknown)
		}
		if got, err := test.tokenOf(genesis); err != nil ||
			got != tokenIDOf(t, genesis) {

			t.Errorf("%s: GENESIS names token %v, %v; want %v", typ.name,
				got, err, tokenIDOf(t, genesis))
		}

		// A SEND of zero tokens naming a GENESIS not yet indexed.
		later := genesisTx(t, 3)
		laterID := tokenIDOf(t, later)
		script, err = typ.send(laterID[:], []uint64{0})
		early := slpTx(t, 4, script, err)
		test.connect(early)
		test.connect(later)
		test.restart()
		for _, tx := range []*wire.MsgTx{early, later} {
			if got, err := test.tokenOf(tx); err != nil || got != laterID {
				t.Errorf("%s: %v names token %v, %v; want %v", typ.name,
					tx.TxHash(), got, err, laterID)
			}
		}
	}
}

// TestSlpGenesisReconnectTokenID ensures a GENESIS connected again, as after
// a reorg, keeps its internal token ID without using up another, so that after
// a restart no new token takes an ID already in use.
func TestSlpGenesisReconnectTokenID(t *testing.T) {
	test := newSlpTokenIDTest(t)
	g := []*wire.MsgTx{genesisTx(t, 1), genesisTx(t, 2), genesisTx(t, 3),
		genesisTx(t, 4)}
	test.connect(g[0])
	test.disconnect(g[0])
	test.connect(g[0])
	test.connect(g[1])
	if test.idx.curTokenID != 2 {
		t.Errorf("two tokens used up %d internal token IDs",
			test.idx.curTokenID)
	}
	test.restart()
	test.connect(g[2])
	test.connect(g[3])
	for i, tx := range g {
		if got, err := test.tokenOf(tx); err != nil || got != tokenIDOf(t, tx) {
			t.Errorf("GENESIS %d names token %v, %v; want %v", i, got, err,
				tokenIDOf(t, tx))
		}
	}
}

// TestSlpInitTokenIDGap ensures the internal token IDs given out after the
// index is opened are above every ID in use, even when an ID below them is
// unused, as in an index where an earlier version connected a GENESIS again.
// The keys of IDs 4 and 256 sort the other way round, as they are little
// endian.
func TestSlpInitTokenIDGap(t *testing.T) {
	test := newSlpTokenIDTest(t)
	inUse := []uint32{1, 2, 4, 256}
	err := test.db.Update(func(dbTx database.Tx) error {
		for _, id := range inUse {
			err := dbPutTokenIDIndexEntry(dbTx, id, &TokenMetadata{
				TokenID:    &chainhash.Hash{0xdd, byte(id)},
				SlpVersion: v1parser.TokenTypeFungible01,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	test.restart()
	if test.idx.curTokenID != 256 {
		t.Errorf("highest internal token ID %d, want 256", test.idx.curTokenID)
	}

	g := []*wire.MsgTx{genesisTx(t, 1), genesisTx(t, 2)}
	test.connect(g[0])
	test.connect(g[1])
	for i, tx := range g {
		if got, err := test.tokenOf(tx); err != nil || got != tokenIDOf(t, tx) {
			t.Errorf("GENESIS %d names token %v, %v; want %v", i, got, err,
				tokenIDOf(t, tx))
		}
	}
	err = test.db.View(func(dbTx database.Tx) error {
		for _, id := range inUse {
			tm, err := dbFetchTokenMetadataByID(dbTx, id)
			if err != nil {
				return err
			}
			if *tm.TokenID != (chainhash.Hash{0xdd, byte(id)}) {
				t.Errorf("token ID %d now names token %v", id, tm.TokenID)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
