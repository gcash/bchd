// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// addrIndexBucket provides a mock address index database bucket by implementing
// the internalBucket interface.
type addrIndexBucket struct {
	levels map[[levelKeySize]byte][]byte
}

// Clone returns a deep copy of the mock address index bucket.
func (b *addrIndexBucket) Clone() *addrIndexBucket {
	levels := make(map[[levelKeySize]byte][]byte)
	for k, v := range b.levels {
		vCopy := make([]byte, len(v))
		copy(vCopy, v)
		levels[k] = vCopy
	}
	return &addrIndexBucket{levels: levels}
}

// Get returns the value associated with the key from the mock address index
// bucket.
//
// This is part of the internalBucket interface.
func (b *addrIndexBucket) Get(key []byte) []byte {
	var levelKey [levelKeySize]byte
	copy(levelKey[:], key)
	return b.levels[levelKey]
}

// Put stores the provided key/value pair to the mock address index bucket.
//
// This is part of the internalBucket interface.
func (b *addrIndexBucket) Put(key []byte, value []byte) error {
	var levelKey [levelKeySize]byte
	copy(levelKey[:], key)
	b.levels[levelKey] = value
	return nil
}

// Delete removes the provided key from the mock address index bucket.
//
// This is part of the internalBucket interface.
func (b *addrIndexBucket) Delete(key []byte) error {
	var levelKey [levelKeySize]byte
	copy(levelKey[:], key)
	delete(b.levels, levelKey)
	return nil
}

// printLevels returns a string with a visual representation of the provided
// address key taking into account the max size of each level.  It is useful
// when creating and debugging test cases.
func (b *addrIndexBucket) printLevels(addrKey [addrKeySize]byte) string {
	highestLevel := uint8(0)
	for k := range b.levels {
		if !bytes.Equal(k[:levelOffset], addrKey[:]) {
			continue
		}
		level := k[levelOffset]
		if level > highestLevel {
			highestLevel = level
		}
	}

	var levelBuf bytes.Buffer
	_, _ = levelBuf.WriteString("\n")
	maxEntries := level0MaxEntries
	for level := uint8(0); level <= highestLevel; level++ {
		data := b.levels[keyForLevel(addrKey, level)]
		numEntries := len(data) / txEntrySize
		for i := range numEntries {
			start := i * txEntrySize
			num := byteOrder.Uint32(data[start:])
			_, _ = levelBuf.WriteString(fmt.Sprintf("%02d ", num))
		}
		for i := numEntries; i < maxEntries; i++ {
			_, _ = levelBuf.WriteString("_  ")
		}
		_, _ = levelBuf.WriteString("\n")
		maxEntries *= 2
	}

	return levelBuf.String()
}

// sanityCheck ensures that all data stored in the bucket for the given address
// adheres to the level-based rules described by the address index
// documentation.
func (b *addrIndexBucket) sanityCheck(addrKey [addrKeySize]byte, expectedTotal int) error {
	// Find the highest level for the key.
	highestLevel := uint8(0)
	for k := range b.levels {
		if !bytes.Equal(k[:levelOffset], addrKey[:]) {
			continue
		}
		level := k[levelOffset]
		if level > highestLevel {
			highestLevel = level
		}
	}

	// Ensure the expected total number of entries are present and that
	// all levels adhere to the rules described in the address index
	// documentation.
	var totalEntries int
	maxEntries := level0MaxEntries
	for level := uint8(0); level <= highestLevel; level++ {
		// Level 0 can'have more entries than the max allowed if the
		// levels after it have data and it can't be empty.  All other
		// levels must either be half full or full.
		data := b.levels[keyForLevel(addrKey, level)]
		numEntries := len(data) / txEntrySize
		totalEntries += numEntries
		if level == 0 {
			if (highestLevel != 0 && numEntries == 0) ||
				numEntries > maxEntries {

				return fmt.Errorf("level %d has %d entries",
					level, numEntries)
			}
		} else if numEntries != maxEntries && numEntries != maxEntries/2 {
			return fmt.Errorf("level %d has %d entries", level,
				numEntries)
		}
		maxEntries *= 2
	}
	if totalEntries != expectedTotal {
		return fmt.Errorf("expected %d entries - got %d", expectedTotal,
			totalEntries)
	}

	// Ensure all of the numbers are in order starting from the highest
	// level moving to the lowest level.
	expectedNum := uint32(0)
	for level := highestLevel + 1; level > 0; level-- {
		data := b.levels[keyForLevel(addrKey, level)]
		numEntries := len(data) / txEntrySize
		for i := range numEntries {
			start := i * txEntrySize
			num := byteOrder.Uint32(data[start:])
			if num != expectedNum {
				return fmt.Errorf("level %d offset %d does "+
					"not contain the expected number of "+
					"%d - got %d", level, i, num,
					expectedNum)
			}
			expectedNum++
		}
	}

	return nil
}

// TestAddrIndexLevels ensures that adding and deleting entries to the address
// index creates multiple levels as described by the address index
// documentation.
func TestAddrIndexLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         [addrKeySize]byte
		numInsert   int
		printLevels bool // Set to help debug a specific test.
	}{
		{
			name:      "level 0 not full",
			numInsert: level0MaxEntries - 1,
		},
		{
			name:      "level 1 half",
			numInsert: level0MaxEntries + 1,
		},
		{
			name:      "level 1 full",
			numInsert: level0MaxEntries*2 + 1,
		},
		{
			name:      "level 2 half, level 1 half",
			numInsert: level0MaxEntries*3 + 1,
		},
		{
			name:      "level 2 half, level 1 full",
			numInsert: level0MaxEntries*4 + 1,
		},
		{
			name:      "level 2 full, level 1 half",
			numInsert: level0MaxEntries*5 + 1,
		},
		{
			name:      "level 2 full, level 1 full",
			numInsert: level0MaxEntries*6 + 1,
		},
		{
			name:      "level 3 half, level 2 half, level 1 half",
			numInsert: level0MaxEntries*7 + 1,
		},
		{
			name:      "level 3 full, level 2 half, level 1 full",
			numInsert: level0MaxEntries*12 + 1,
		},
	}

nextTest:
	for testNum, test := range tests {
		// Insert entries in order.
		populatedBucket := &addrIndexBucket{
			levels: make(map[[levelKeySize]byte][]byte),
		}
		for i := range test.numInsert {
			txLoc := wire.TxLoc{TxStart: i * 2}
			err := dbPutAddrIndexEntry(populatedBucket, test.key,
				uint32(i), txLoc)
			if err != nil {
				t.Errorf("dbPutAddrIndexEntry #%d (%s) - "+
					"unexpected error: %v", testNum,
					test.name, err)
				continue nextTest
			}
		}
		if test.printLevels {
			t.Log(populatedBucket.printLevels(test.key))
		}

		// Delete entries from the populated bucket until all entries
		// have been deleted.  The bucket is reset to the fully
		// populated bucket on each iteration so every combination is
		// tested.  Notice the upper limit purposes exceeds the number
		// of entries to ensure attempting to delete more entries than
		// there are works correctly.
		for numDelete := 0; numDelete <= test.numInsert+1; numDelete++ {
			// Clone populated bucket to run each delete against.
			bucket := populatedBucket.Clone()

			// Remove the number of entries for this iteration.
			err := dbRemoveAddrIndexEntries(bucket, test.key,
				numDelete)
			if err != nil {
				if numDelete <= test.numInsert {
					t.Errorf("dbRemoveAddrIndexEntries (%s) "+
						" delete %d - unexpected error: "+
						"%v", test.name, numDelete, err)
					continue nextTest
				}
			}
			if test.printLevels {
				t.Log(bucket.printLevels(test.key))
			}

			// Sanity check the levels to ensure the adhere to all
			// rules.
			numExpected := test.numInsert
			if numDelete <= test.numInsert {
				numExpected -= numDelete
			}
			err = bucket.sanityCheck(test.key, numExpected)
			if err != nil {
				t.Errorf("sanity check fail (%s) delete %d: %v",
					test.name, numDelete, err)
				continue nextTest
			}
		}
	}
}

// createTestAddrIndex returns a new database with an empty address index and
// the buckets it needs from the transaction index.
func createTestAddrIndex(t *testing.T) (database.DB, *AddrIndex) {
	t.Helper()

	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		wire.SimNet)
	if err != nil {
		t.Fatalf("unable to create database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	idx := NewAddrIndex(db, &chaincfg.MainNetParams)
	err = db.Update(func(dbTx database.Tx) error {
		_, err := dbTx.Metadata().CreateBucket(indexTipsBucketName)
		if err != nil {
			return err
		}
		if err := NewTxIndex(db).Create(dbTx); err != nil {
			return err
		}
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		return dbPutIndexerTip(dbTx, addrIndexKey, &chainhash.Hash{}, -1)
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, idx
}

// TestAddrIndexTokenSpends ensures spends of token outputs are indexed under
// the address of the spent output and removed again on disconnect.
func TestAddrIndexTokenSpends(t *testing.T) {
	t.Parallel()

	db, idx := createTestAddrIndex(t)
	params := &chaincfg.MainNetParams

	p2pkh, err := bchutil.NewAddressPubKeyHash(bytes.Repeat([]byte{1}, 20),
		params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh32, err := bchutil.NewAddressScriptHash32FromHash(
		bytes.Repeat([]byte{2}, 32), params)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := bchutil.NewAddressPubKeyHash(bytes.Repeat([]byte{3}, 20),
		params)
	if err != nil {
		t.Fatal(err)
	}

	commitment := []byte{0xaa, 0xbb}
	capability := byte(wire.NONE)
	nft, err := wire.NewTokenData([32]byte{1}, nil, &commitment, &capability)
	if err != nil {
		t.Fatal(err)
	}
	amount := uint64(1000)
	ft, err := wire.NewTokenData([32]byte{2}, &amount, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		addr  bchutil.Address
		token *wire.TokenData
	}{
		{p2pkh, nft},
		{p2sh32, ft},
		{plain, nil},
	}

	// The first block pays to each address and the second spends those
	// outputs.  The spend journal keeps the token data in front of the
	// spent script.
	fund := wire.NewMsgTx(1)
	fund.AddTxIn(&wire.TxIn{})
	spend := wire.NewMsgTx(2)
	var stxos []blockchain.SpentTxOut
	for i, test := range tests {
		pkScript, err := txscript.PayToAddrScript(test.addr)
		if err != nil {
			t.Fatal(err)
		}
		txOut := &wire.TxOut{Value: 1000, PkScript: pkScript}
		journalScript := pkScript
		if test.token != nil {
			txOut.TokenData = *test.token
			buf := test.token.TokenDataBuffer()
			buf.Write(pkScript)
			journalScript = buf.Bytes()
		}
		fund.AddTxOut(txOut)

		spend.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: fund.TxHash(),
				Index: uint32(i)},
		})
		stxos = append(stxos, blockchain.SpentTxOut{
			Amount:   1000,
			PkScript: journalScript,
		})
	}
	spend.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{txscript.OP_TRUE}})
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{})
	coinbase.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{txscript.OP_TRUE}})

	block1 := bchutil.NewBlock(&wire.MsgBlock{
		Header:       wire.BlockHeader{Nonce: 1},
		Transactions: []*wire.MsgTx{fund},
	})
	block2 := bchutil.NewBlock(&wire.MsgBlock{
		Header:       wire.BlockHeader{Nonce: 2},
		Transactions: []*wire.MsgTx{coinbase, spend},
	})
	locs1, err := block1.TxLoc()
	if err != nil {
		t.Fatal(err)
	}
	locs2, err := block2.TxLoc()
	if err != nil {
		t.Fatal(err)
	}
	fundRegion := database.BlockRegion{Hash: block1.Hash(),
		Offset: uint32(locs1[0].TxStart), Len: uint32(locs1[0].TxLen)}
	spendRegion := database.BlockRegion{Hash: block2.Hash(),
		Offset: uint32(locs2[1].TxStart), Len: uint32(locs2[1].TxLen)}

	checkRegions := func(want ...database.BlockRegion) {
		t.Helper()
		err := db.View(func(dbTx database.Tx) error {
			for _, test := range tests {
				regions, _, err := idx.TxRegionsForAddress(dbTx,
					test.addr, 0, 10, false)
				if err != nil {
					return err
				}
				if len(regions) != len(want) {
					t.Errorf("%v: got %d entries, want %d",
						test.addr, len(regions), len(want))
					continue
				}
				for i := range want {
					if !regions[i].Hash.IsEqual(want[i].Hash) ||
						regions[i].Offset != want[i].Offset ||
						regions[i].Len != want[i].Len {
						t.Errorf("%v: entry %d is %v, want %v",
							test.addr, i, regions[i], want[i])
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	err = db.Update(func(dbTx database.Tx) error {
		if err := dbPutBlockIDIndexEntry(dbTx, block1.Hash(), 1); err != nil {
			return err
		}
		if err := dbPutBlockIDIndexEntry(dbTx, block2.Hash(), 2); err != nil {
			return err
		}
		if err := idx.ConnectBlock(dbTx, block1, nil); err != nil {
			return err
		}
		return idx.ConnectBlock(dbTx, block2, stxos)
	})
	if err != nil {
		t.Fatal(err)
	}
	checkRegions(fundRegion, spendRegion)

	err = db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block2, stxos)
	})
	if err != nil {
		t.Fatal(err)
	}
	checkRegions(fundRegion)
}
