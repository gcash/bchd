// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
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

// TestAddrIndexMigrate ensures an index from before the current version is
// dropped for a rebuild and a current one is kept.
func TestAddrIndexMigrate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version uint32
		dropped bool
	}{
		{name: "no version", version: 0, dropped: true},
		{name: "version 1", version: 1, dropped: true},
		{name: "current version", version: addrIndexVersion,
			dropped: false},
	}
	for _, test := range tests {
		db, idx := createTestAddrIndex(t)
		entry := bytes.Repeat([]byte{1}, levelKeySize)
		tipHash := &chainhash.Hash{1}
		const tipHeight = 1000
		err := db.Update(func(dbTx database.Tx) error {
			bucket := dbTx.Metadata().Bucket(addrIndexKey)
			var err error
			if test.version == 0 {
				err = bucket.Delete(addrIndexVersionKey)
			} else {
				versionBytes := make([]byte, 4)
				binary.BigEndian.PutUint32(versionBytes,
					test.version)
				err = bucket.Put(addrIndexVersionKey,
					versionBytes)
			}
			if err != nil {
				return err
			}
			if err := bucket.Put(entry, entry); err != nil {
				return err
			}
			return dbPutIndexerTip(dbTx, addrIndexKey, tipHash,
				tipHeight)
		})
		if err != nil {
			t.Fatal(err)
		}

		// A second run must leave the index alone.
		for i := 0; i < 2; i++ {
			if err := idx.Migrate(db, nil); err != nil {
				t.Fatalf("%s: Migrate: %v", test.name, err)
			}
		}

		err = db.View(func(dbTx database.Tx) error {
			version, err := dbFetchAddrIndexVersion(dbTx)
			if err != nil {
				return err
			}
			if version != addrIndexVersion {
				t.Errorf("%s: version %d, want %d", test.name,
					version, addrIndexVersion)
			}
			hash, height, err := dbFetchIndexerTip(dbTx, addrIndexKey)
			if err != nil {
				return err
			}
			bucket := dbTx.Metadata().Bucket(addrIndexKey)
			kept := bucket.Get(entry) != nil
			wantHash, wantHeight := tipHash, int32(tipHeight)
			if test.dropped {
				wantHash, wantHeight = &chainhash.Hash{}, -1
			}
			if kept == test.dropped {
				t.Errorf("%s: entry kept %v, want %v", test.name,
					kept, !test.dropped)
			}
			if !hash.IsEqual(wantHash) || height != wantHeight {
				t.Errorf("%s: tip %v at %d, want %v at %d",
					test.name, hash, height, wantHash,
					wantHeight)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestAddrIndexP2SH32Key ensures a P2SH32 output whose hash ends in 12 zero
// bytes is indexed under its own address only and not under the P2SH address
// with the first 20 bytes of that hash.
func TestAddrIndexP2SH32Key(t *testing.T) {
	t.Parallel()

	db, idx := createTestAddrIndex(t)
	params := &chaincfg.MainNetParams

	hash := make([]byte, 32)
	copy(hash, bytes.Repeat([]byte{4}, 20))
	p2sh, err := bchutil.NewAddressScriptHashFromHash(hash[:20], params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh32, err := bchutil.NewAddressScriptHash32FromHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}

	// The coinbase pays to the P2SH address and the second transaction to
	// the P2SH32 address.
	var txns []*wire.MsgTx
	for i, addr := range []bchutil.Address{p2sh, p2sh32} {
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			t.Fatal(err)
		}
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Index: uint32(i)},
		})
		tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: pkScript})
		txns = append(txns, tx)
	}
	block := bchutil.NewBlock(&wire.MsgBlock{
		Header:       wire.BlockHeader{Nonce: 1},
		Transactions: txns,
	})
	locs, err := block.TxLoc()
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		if err := dbPutBlockIDIndexEntry(dbTx, block.Hash(), 1); err != nil {
			return err
		}
		stxos := []blockchain.SpentTxOut{
			{Amount: 1000, PkScript: []byte{txscript.OP_TRUE}},
		}
		return idx.ConnectBlock(dbTx, block, stxos)
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		addr bchutil.Address
		tx   int
	}{
		{p2sh, 0},
		{p2sh32, 1},
	}
	err = db.View(func(dbTx database.Tx) error {
		for _, test := range tests {
			regions, _, err := idx.TxRegionsForAddress(dbTx,
				test.addr, 0, 10, false)
			if err != nil {
				return err
			}
			want := locs[test.tx]
			if len(regions) != 1 {
				t.Errorf("%v: got %d entries, want 1", test.addr,
					len(regions))
				continue
			}
			if !regions[0].Hash.IsEqual(block.Hash()) ||
				regions[0].Offset != uint32(want.TxStart) ||
				regions[0].Len != uint32(want.TxLen) {
				t.Errorf("%v: entry is %v, want tx %d", test.addr,
					regions[0], test.tx)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// The unconfirmed index must keep the addresses apart too.
	for _, tx := range txns {
		idx.AddUnconfirmedTx(bchutil.NewTx(tx),
			blockchain.NewUtxoViewpoint())
	}
	for _, test := range tests {
		unconfirmed := idx.UnconfirmedTxnsForAddress(test.addr)
		wantHash := txns[test.tx].TxHash()
		if len(unconfirmed) != 1 ||
			!unconfirmed[0].Hash().IsEqual(&wantHash) {
			t.Errorf("%v: got %d unconfirmed transactions, want "+
				"only tx %d", test.addr, len(unconfirmed), test.tx)
		}
	}
}

// TestTxRegionPagesForAddress ensures TxRegionPagesForAddress gives the block
// regions of an address's transactions newest first, as
// dbFetchAddrIndexEntries does, in pages of the size asked for, that it stops
// at the first error of its callback and returns it, and that it does not
// call it for an address without transactions.  It also ensures
// dbFetchAddrIndexEntries, which shares the reading of the levels, still
// gives them all oldest first, and reads only the levels it needs newest
// first.
func TestTxRegionPagesForAddress(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	params := &chaincfg.MainNetParams
	idx := NewAddrIndex(db, params)
	addr, err := bchutil.NewAddressPubKeyHash(make([]byte, 20), params)
	if err != nil {
		t.Fatal(err)
	}
	addrKey, err := addrToKey(addr)
	if err != nil {
		t.Fatal(err)
	}

	// 100 transactions in blocks 1 to 10, ten in each.
	const n = 100
	levels := &addrIndexBucket{levels: make(map[[levelKeySize]byte][]byte)}
	for i := 0; i < n; i++ {
		err := dbPutAddrIndexEntry(levels, addrKey, uint32(i/10+1),
			wire.TxLoc{TxStart: i, TxLen: 1})
		if err != nil {
			t.Fatal(err)
		}
	}
	blockHash := func(id []byte) (*chainhash.Hash, error) {
		h := chainhash.DoubleHashH(id)
		return &h, nil
	}
	err = db.Update(func(dbTx database.Tx) error {
		bucket, err := dbTx.Metadata().CreateBucket(addrIndexKey)
		if err != nil {
			return err
		}
		for key, value := range levels.levels {
			if err := bucket.Put(key[:], value); err != nil {
				return err
			}
		}
		ids, err := dbTx.Metadata().CreateBucket(hashByIDIndexBucketName)
		if err != nil {
			return err
		}
		for id := uint32(1); id <= n/10; id++ {
			var serializedID [4]byte
			byteOrder.PutUint32(serializedID[:], id)
			hash, _ := blockHash(serializedID[:])
			if err := ids.Put(serializedID[:], hash[:]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := dbFetchAddrIndexEntries(levels, addrKey, 0, n, true,
		blockHash)
	if err != nil {
		t.Fatal(err)
	}

	// Read from the oldest, the same entries come in the other order, and
	// from the newest, only the levels needed are read.
	forward, _, err := dbFetchAddrIndexEntries(levels, addrKey, 0, n, false,
		blockHash)
	if err != nil {
		t.Fatal(err)
	}
	for i := range forward {
		if !reflect.DeepEqual(forward[i], want[n-1-i]) {
			t.Fatalf("oldest first, entry %d: got %v, want %v", i,
				forward[i], want[n-1-i])
		}
	}
	gets := &getCountingBucket{internalBucket: levels}
	_, _, err = dbFetchAddrIndexEntries(gets, addrKey, 0, 1, true, blockHash)
	if err != nil || gets.gets != 1 {
		t.Errorf("newest entry: %d levels read and %v, want 1", gets.gets, err)
	}

	for _, pageSize := range []uint32{1, 7, n, n + 1} {
		var got []database.BlockRegion
		pages := 0
		err := idx.TxRegionPagesForAddress(addr, pageSize,
			func(regions []database.BlockRegion) error {
				pages++
				if pages > n {
					return errors.New("too many pages")
				}
				if len(regions) > int(pageSize) {
					t.Errorf("pages of %d: a page of %d", pageSize,
						len(regions))
				}
				got = append(got, regions...)
				return nil
			})
		if err != nil {
			t.Fatalf("pages of %d: %v", pageSize, err)
		}
		if wantPages := (n + int(pageSize) - 1) / int(pageSize); pages != wantPages {
			t.Errorf("pages of %d: %d pages, want %d", pageSize, pages,
				wantPages)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("pages of %d: got %v, want %v", pageSize, got, want)
		}
	}

	stop := errors.New("stop")
	calls := 0
	err = idx.TxRegionPagesForAddress(addr, 10,
		func([]database.BlockRegion) error {
			calls++
			return stop
		})
	if err != stop || calls != 1 {
		t.Errorf("callback failing: got %v after %d calls, want its error "+
			"after 1", err, calls)
	}

	unused, err := bchutil.NewAddressPubKeyHash(append(make([]byte, 19), 1),
		params)
	if err != nil {
		t.Fatal(err)
	}
	err = idx.TxRegionPagesForAddress(unused, 10,
		func([]database.BlockRegion) error {
			return errors.New("called")
		})
	if err != nil {
		t.Errorf("address without transactions: %v", err)
	}
}

// getCountingBucket counts the reads of an internalBucket.
type getCountingBucket struct {
	internalBucket
	gets int
}

func (b *getCountingBucket) Get(key []byte) []byte {
	b.gets++
	return b.internalBucket.Get(key)
}

// TestTxRegionPagesForAddressReorg ensures that once a block of the history
// TxRegionPagesForAddress read with its first page has been disconnected, its
// later pages fail with ErrAddrHistoryChanged instead of naming the blocks
// that took over the IDs of those disconnected, or failing to name any, and
// that blocks connected meanwhile change nothing.
func TestTxRegionPagesForAddressReorg(t *testing.T) {
	params := &chaincfg.MainNetParams
	addr, err := bchutil.NewAddressPubKeyHash(make([]byte, 20), params)
	if err != nil {
		t.Fatal(err)
	}
	addrKey, err := addrToKey(addr)
	if err != nil {
		t.Fatal(err)
	}
	serializedID := func(id uint32) []byte {
		var serialized [4]byte
		byteOrder.PutUint32(serialized[:], id)
		return serialized[:]
	}
	blockHash := func(id []byte, branch byte) chainhash.Hash {
		return chainhash.DoubleHashH(append([]byte{branch}, id...))
	}

	for _, test := range []struct {
		name   string
		change func(ids database.Bucket) error
		want   error
	}{{
		// Blocks 9 and 10 are replaced.
		"replaced", func(ids database.Bucket) error {
			for id := uint32(9); id <= 10; id++ {
				hash := blockHash(serializedID(id), 1)
				if err := ids.Put(serializedID(id), hash[:]); err != nil {
					return err
				}
			}
			return nil
		}, ErrAddrHistoryChanged,
	}, {
		"disconnected", func(ids database.Bucket) error {
			return ids.Delete(serializedID(10))
		}, ErrAddrHistoryChanged,
	}, {
		"connected", func(ids database.Bucket) error {
			hash := blockHash(serializedID(11), 0)
			return ids.Put(serializedID(11), hash[:])
		}, nil,
	}} {
		db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		idx := NewAddrIndex(db, params)

		// 100 transactions in blocks 1 to 10, ten in each.
		const n = 100
		levels := &addrIndexBucket{levels: make(map[[levelKeySize]byte][]byte)}
		for i := 0; i < n; i++ {
			err := dbPutAddrIndexEntry(levels, addrKey, uint32(i/10+1),
				wire.TxLoc{TxStart: i, TxLen: 1})
			if err != nil {
				t.Fatal(err)
			}
		}
		err = db.Update(func(dbTx database.Tx) error {
			bucket, err := dbTx.Metadata().CreateBucket(addrIndexKey)
			if err != nil {
				return err
			}
			for key, value := range levels.levels {
				if err := bucket.Put(key[:], value); err != nil {
					return err
				}
			}
			ids, err := dbTx.Metadata().CreateBucket(hashByIDIndexBucketName)
			if err != nil {
				return err
			}
			for id := uint32(1); id <= n/10; id++ {
				hash := blockHash(serializedID(id), 0)
				if err := ids.Put(serializedID(id), hash[:]); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		// The chain changes after the first page of 7, all of block 10.
		var got []database.BlockRegion
		err = idx.TxRegionPagesForAddress(addr, 7,
			func(regions []database.BlockRegion) error {
				if len(got) == 0 {
					err := db.Update(func(dbTx database.Tx) error {
						return test.change(dbTx.Metadata().Bucket(
							hashByIDIndexBucketName))
					})
					if err != nil {
						return err
					}
				}
				got = append(got, regions...)
				return nil
			})
		if err != test.want {
			t.Errorf("%s: got %v, want %v", test.name, err, test.want)
		}
		for i, region := range got {
			id := serializedID(uint32((n-1-i)/10 + 1))
			if want := blockHash(id, 0); !region.Hash.IsEqual(&want) {
				t.Errorf("%s: entry %d names block %v, want %v", test.name,
					i, region.Hash, want)
			}
		}
		if test.want == nil && len(got) != n {
			t.Errorf("%s: %d entries, want %d", test.name, len(got), n)
		}
		if test.want != nil && len(got) != 7 {
			t.Errorf("%s: %d entries, want the first page's 7", test.name,
				len(got))
		}
	}
}
