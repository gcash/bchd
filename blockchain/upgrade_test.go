// Copyright (c) 2015-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// tokenRepairTx returns a transaction paying 1000 satoshis to each of the
// passed locking scripts, decoded from its serialization as the transactions
// of a block are.
func tokenRepairTx(t *testing.T, scripts []string) *bchutil.Tx {
	t.Helper()

	var raw bytes.Buffer
	raw.Write(hexToBytes("02000000" + "01" + strings.Repeat("33", 32) +
		"00000000" + "00" + "ffffffff"))
	if err := wire.WriteVarInt(&raw, 0, uint64(len(scripts))); err != nil {
		t.Fatal(err)
	}
	for _, script := range scripts {
		raw.Write(hexToBytes("e803000000000000"))
		b := hexToBytes(script)
		if err := wire.WriteVarInt(&raw, 0, uint64(len(b))); err != nil {
			t.Fatal(err)
		}
		raw.Write(b)
	}
	raw.Write(hexToBytes("00000000"))
	tx, err := bchutil.NewTxFromBytes(raw.Bytes())
	if err != nil {
		t.Fatalf("NewTxFromBytes: %v", err)
	}
	return tx
}

// newTokenRepairChain returns a fake chain on a new database, with a utxo set
// at version 2 and a main chain tip at the passed height.
func newTokenRepairChain(t *testing.T, tipHeight int32) *BlockChain {
	t.Helper()

	params := &chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	chain := newFakeChain(params)
	chain.db = db
	nodes := chainedNodes(chain.bestChain.Tip(), int(tipHeight))
	chain.bestChain.SetTip(nodes[len(nodes)-1])

	err = db.Update(func(dbTx database.Tx) error {
		_, err := dbTx.Metadata().CreateBucketIfNotExists(utxoSetBucketName)
		if err != nil {
			return err
		}
		return dbPutVersion(dbTx, utxoSetVersionKeyName, 2)
	})
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

// putTokenRepairUtxo stores a utxo of 1000 satoshis whose locking script,
// with any token prefix in front, is stored.
func putTokenRepairUtxo(t *testing.T, db database.DB, outpoint wire.OutPoint,
	stored []byte, height int32) {

	t.Helper()

	serialized, err := serializeUtxoEntry(&UtxoEntry{
		amount:      1000,
		pkScript:    stored,
		blockHeight: height,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(utxoSetBucketName)
		return bucket.Put(*outpointKey(outpoint), serialized)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// fetchTokenRepairState returns the stored utxo for each outpoint and the
// utxo set version.
func fetchTokenRepairState(t *testing.T, db database.DB,
	outpoints []wire.OutPoint) ([][]byte, uint32) {

	t.Helper()

	var (
		stored  [][]byte
		version uint32
	)
	err := db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(utxoSetBucketName)
		for _, outpoint := range outpoints {
			value := bucket.Get(*outpointKey(outpoint))
			stored = append(stored, append([]byte(nil), value...))
		}
		version = dbFetchVersion(dbTx, utxoSetVersionKeyName)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return stored, version
}

// TestRepairUtxoTokenPrefixes ensures the utxo set upgrade to version 3
// rebuilds the entries bchd 0.22.2 stored with their token prefix split
// twice, leaves the others alone, survives an interrupt, and asks for a
// resync when the block of a possibly damaged entry is gone.
func TestRepairUtxoTokenPrefixes(t *testing.T) {
	c := strings.Repeat("11", 32)
	d := strings.Repeat("22", 32)
	z := strings.Repeat("00", 32)
	p2pkh := "76a914" + strings.Repeat("44", 20) + "88ac"
	prefix := "ef" + c + "10fde803" // a fungible token of category c

	// The stored scripts are what bchd 0.22.2 stored, produced with its
	// wire package: decoding split off the token prefix and addTxOuts split
	// the remaining script again.  An invalid prefix with a nonzero
	// category is left out because 0.22.2 hashed its transaction wrongly
	// and so rejected any block holding one.
	tests := []struct {
		name    string
		script  string // the locking script in the transaction
		stored  string // what 0.22.2 stored for it
		damaged bool
	}{
		{"p2pkh", p2pkh, p2pkh, false},
		{"token", prefix + p2pkh, prefix + p2pkh, false},
		{"token script read as a token prefix",
			prefix + "ef" + d + "1005" + "51", "ef" + d + "1005" + "51", true},
		{"token script failing as a token prefix",
			prefix + "ef" + d + "00" + "51", "ef" + d + "00", true},
		{"invalid prefix with a zero category",
			"ef" + z + "00" + "51", "", true},
	}

	t.Run("repair", func(t *testing.T) {
		chain := newTokenRepairChain(t, 99)
		scripts := make([]string, len(tests))
		for i, test := range tests {
			scripts[i] = test.script
		}
		tx := tokenRepairTx(t, scripts)
		block := bchutil.NewBlock(&wire.MsgBlock{
			Header:       wire.BlockHeader{PrevBlock: chain.bestChain.Tip().hash},
			Transactions: []*wire.MsgTx{tx.MsgTx()},
		})
		chain.bestChain.SetTip(newBlockNode(&block.MsgBlock().Header,
			chain.bestChain.Tip()))
		err := chain.db.Update(func(dbTx database.Tx) error {
			return dbTx.StoreBlock(block)
		})
		if err != nil {
			t.Fatal(err)
		}

		var outpoints []wire.OutPoint
		for i, test := range tests {
			outpoint := wire.OutPoint{Hash: *tx.Hash(), Index: uint32(i)}
			putTokenRepairUtxo(t, chain.db, outpoint, hexToBytes(test.stored), 100)
			outpoints = append(outpoints, outpoint)
		}
		before, _ := fetchTokenRepairState(t, chain.db, outpoints)

		// Candidates only need their own blocks: this p2pkh utxo's
		// block is not stored.
		other := wire.OutPoint{Hash: *tx.Hash(), Index: 99}
		putTokenRepairUtxo(t, chain.db, other, hexToBytes(p2pkh), 50)

		// An interrupted repair keeps the repairs done so far and leaves
		// the version, and a second run completes it.
		interrupt := make(chan struct{})
		close(interrupt)
		if err := chain.maybeUpgradeDbBuckets(interrupt); err != errInterruptRequested {
			t.Fatalf("interrupted repair: got %v, want %v", err, errInterruptRequested)
		}
		if _, version := fetchTokenRepairState(t, chain.db, nil); version != 2 {
			t.Fatalf("interrupted repair set version %d, want 2", version)
		}
		if err := chain.maybeUpgradeDbBuckets(nil); err != nil {
			t.Fatalf("repair: %v", err)
		}

		after, version := fetchTokenRepairState(t, chain.db, outpoints)
		if version != 3 {
			t.Errorf("utxo set version %d, want 3", version)
		}
		for i, test := range tests {
			txOut := tx.MsgTx().TxOut[i]
			want, err := serializeUtxoEntry(&UtxoEntry{
				amount:      txOut.Value,
				pkScript:    txOut.PkScript,
				tokenData:   txOut.TokenData,
				blockHeight: 100,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after[i], want) {
				t.Errorf("%s: stored %x, want %x", test.name, after[i], want)
			}
			if changed := !bytes.Equal(before[i], after[i]); changed != test.damaged {
				t.Errorf("%s: changed %v, want %v", test.name, changed, test.damaged)
			}
		}
	})

	// A pruned or fast synced node skips the check, which needs blocks it
	// no longer has.
	t.Run("pruned", func(t *testing.T) {
		chain := newTokenRepairChain(t, 200)
		err := chain.db.Update(func(dbTx database.Tx) error {
			return dbPutBlockchainType(dbTx, prunedBlockchainEntryValue)
		})
		if err != nil {
			t.Fatal(err)
		}
		outpoint := wire.OutPoint{Hash: *tokenRepairTx(t, []string{p2pkh}).Hash()}
		putTokenRepairUtxo(t, chain.db, outpoint, nil, 200)
		before, _ := fetchTokenRepairState(t, chain.db, []wire.OutPoint{outpoint})

		if err := chain.maybeUpgradeDbBuckets(nil); err != nil {
			t.Fatalf("pruned node: %v", err)
		}
		after, version := fetchTokenRepairState(t, chain.db, []wire.OutPoint{outpoint})
		if version != 3 {
			t.Errorf("utxo set version %d, want 3", version)
		}
		if !bytes.Equal(before[0], after[0]) {
			t.Errorf("utxo changed from %x to %x", before[0], after[0])
		}
	})

	// A node that is not pruned has every block, so a missing one fails
	// the check.
	t.Run("missing block", func(t *testing.T) {
		chain := newTokenRepairChain(t, 200)
		outpoint := wire.OutPoint{Hash: *tokenRepairTx(t, []string{p2pkh}).Hash()}
		putTokenRepairUtxo(t, chain.db, outpoint, nil, 200)

		if err := chain.maybeUpgradeDbBuckets(nil); err == nil {
			t.Fatal("got no error for a missing block")
		}
		if _, version := fetchTokenRepairState(t, chain.db, nil); version != 2 {
			t.Errorf("utxo set version %d, want 2", version)
		}
	})
}

// TestDeserializeUtxoEntryV0 ensures deserializing unspent trasaction output
// entries from the legacy version 0 format works as expected.
func TestDeserializeUtxoEntryV0(t *testing.T) {
	tests := []struct {
		name       string
		entries    map[uint32]*UtxoEntry
		serialized []byte
	}{
		// From tx in main blockchain:
		// 0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098
		{
			name: "Only output 0, coinbase",
			entries: map[uint32]*UtxoEntry{
				0: {
					amount:      5000000000,
					pkScript:    hexToBytes("410496b538e853519c726a2c91e61ec11600ae1390813a627c66fb8be7947be63c52da7589379515d4e0a604f8141781e62294721166bf621e73a82cbf2342c858eeac"),
					blockHeight: 1,
					packedFlags: tfCoinBase,
				},
			},
			serialized: hexToBytes("010103320496b538e853519c726a2c91e61ec11600ae1390813a627c66fb8be7947be63c52"),
		},
		// From tx in main blockchain:
		// 8131ffb0a2c945ecaf9b9063e59558784f9c3a74741ce6ae2a18d0571dac15bb
		{
			name: "Only output 1, not coinbase",
			entries: map[uint32]*UtxoEntry{
				1: {
					amount:      1000000,
					pkScript:    hexToBytes("76a914ee8bd501094a7d5ca318da2506de35e1cb025ddc88ac"),
					blockHeight: 100001,
					packedFlags: 0,
				},
			},
			serialized: hexToBytes("01858c21040700ee8bd501094a7d5ca318da2506de35e1cb025ddc"),
		},
		// Adapted from tx in main blockchain:
		// df3f3f442d9699857f7f49de4ff0b5d0f3448bec31cdc7b5bf6d25f2abd637d5
		{
			name: "Only output 2, coinbase",
			entries: map[uint32]*UtxoEntry{
				2: {
					amount:      100937281,
					pkScript:    hexToBytes("76a914da33f77cee27c2a975ed5124d7e4f7f97513510188ac"),
					blockHeight: 99004,
					packedFlags: tfCoinBase,
				},
			},
			serialized: hexToBytes("0185843c010182b095bf4100da33f77cee27c2a975ed5124d7e4f7f975135101"),
		},
		// Adapted from tx in main blockchain:
		// 4a16969aa4764dd7507fc1de7f0baa4850a246de90c45e59a3207f9a26b5036f
		{
			name: "outputs 0 and 2 not coinbase",
			entries: map[uint32]*UtxoEntry{
				0: {
					amount:      20000000,
					pkScript:    hexToBytes("76a914e2ccd6ec7c6e2e581349c77e067385fa8236bf8a88ac"),
					blockHeight: 113931,
					packedFlags: 0,
				},
				2: {
					amount:      15000000,
					pkScript:    hexToBytes("76a914b8025be1b3efc63b0ad48e7f9f10e87544528d5888ac"),
					blockHeight: 113931,
					packedFlags: 0,
				},
			},
			serialized: hexToBytes("0185f90b0a011200e2ccd6ec7c6e2e581349c77e067385fa8236bf8a800900b8025be1b3efc63b0ad48e7f9f10e87544528d58"),
		},
		// Adapted from tx in main blockchain:
		// 1b02d1c8cfef60a189017b9a420c682cf4a0028175f2f563209e4ff61c8c3620
		{
			name: "Only output 22, not coinbase",
			entries: map[uint32]*UtxoEntry{
				22: {
					amount:      366875659,
					pkScript:    hexToBytes("a9141dd46a006572d820e448e12d2bbb38640bc718e687"),
					blockHeight: 338156,
					packedFlags: 0,
				},
			},
			serialized: hexToBytes("0193d06c100000108ba5b9e763011dd46a006572d820e448e12d2bbb38640bc718e6"),
		},
	}

	for i, test := range tests {
		// Deserialize to map of utxos keyed by the output index.
		entries, err := deserializeUtxoEntryV0(test.serialized)
		if err != nil {
			t.Errorf("deserializeUtxoEntryV0 #%d (%s) unexpected "+
				"error: %v", i, test.name, err)
			continue
		}

		// Ensure the deserialized entry has the same properties as the
		// ones in the test entry.
		if !reflect.DeepEqual(entries, test.entries) {
			t.Errorf("deserializeUtxoEntryV0 #%d (%s) unexpected "+
				"entries: got %v, want %v", i, test.name,
				entries, test.entries)
			continue
		}
	}
}
