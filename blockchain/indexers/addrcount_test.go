// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// countsBucket is a mock of the bucket of stored counts.
type countsBucket map[[addrKeySize]byte][]byte

func (b countsBucket) Get(key []byte) []byte {
	if len(key) != addrKeySize {
		return nil
	}
	return b[[addrKeySize]byte(key)]
}

func (b countsBucket) Put(key []byte, value []byte) error {
	if len(key) != addrKeySize {
		return fmt.Errorf("count key of %d bytes", len(key))
	}
	b[[addrKeySize]byte(key)] = value
	return nil
}

func (b countsBucket) Delete(key []byte) error {
	if len(key) == addrKeySize {
		delete(b, [addrKeySize]byte(key))
	}
	return nil
}

// readCountingBucket counts the bytes read from an internalBucket.
type readCountingBucket struct {
	internalBucket
	read int
}

func (b *readCountingBucket) Get(key []byte) []byte {
	value := b.internalBucket.Get(key)
	b.read += len(value)
	return value
}

// addrLevels returns the number of entries of the address in the levels of
// bucket and its highest level, and checks that the levels follow the rules
// of the address index and that the entries are in the order added, as told
// by their block IDs and start offsets.
func addrLevels(bucket internalBucket, addrKey [addrKeySize]byte) (int, uint8, error) {
	var highest uint8
	for level := uint8(1); ; level++ {
		key := keyForLevel(addrKey, level)
		data := bucket.Get(key[:])
		if data == nil {
			break
		}
		full := maxEntriesForLevel(level)
		if n := len(data) / txEntrySize; n != full && n != full/2 {
			return 0, 0, fmt.Errorf("level %d has %d entries", level, n)
		}
		highest = level
	}
	key := keyForLevel(addrKey, 0)
	if data := bucket.Get(key[:]); len(data) > level0MaxEntries*txEntrySize ||
		highest > 0 && len(data) == 0 {

		return 0, 0, fmt.Errorf("level 0 has %d entries",
			len(data)/txEntrySize)
	}
	serialized := dbFetchAddrIndexLevels(bucket, addrKey, math.MaxUint64)
	n := len(serialized) / txEntrySize
	for i := 1; i < n; i++ {
		prev := serialized[(i-1)*txEntrySize:]
		entry := serialized[i*txEntrySize:]
		prevID, id := byteOrder.Uint32(prev), byteOrder.Uint32(entry)
		if id < prevID || id == prevID &&
			byteOrder.Uint32(entry[4:]) <= byteOrder.Uint32(prev[4:]) {

			return 0, 0, fmt.Errorf("entry %d is out of order", i)
		}
	}
	return n, highest, nil
}

// checkAddrCount checks that the address has a stored count exactly when its
// highest level is at least addrIndexCountLevel, which is exactly when it has
// more entries than the levels below hold, that the count is its number of
// entries, and that dbFetchAddrIndexCount gives that number, reading no level
// for an address with a stored count.
func checkAddrCount(levels, counts internalBucket, addrKey [addrKeySize]byte) error {
	n, highest, err := addrLevels(levels, addrKey)
	if err != nil {
		return err
	}
	threshold := minEntriesToReachLevel(addrIndexCountLevel)
	if (highest >= addrIndexCountLevel) != (n >= threshold) {
		return fmt.Errorf("%d entries up to level %d", n, highest)
	}
	stored, ok, err := dbFetchAddrIndexStoredCount(counts, addrKey)
	if err != nil {
		return err
	}
	if ok != (highest >= addrIndexCountLevel) || ok && int(stored) != n {
		return fmt.Errorf("%d entries up to level %d: stored count %d, %v",
			n, highest, stored, ok)
	}
	reads := &readCountingBucket{internalBucket: levels}
	count, err := dbFetchAddrIndexCount(reads, counts, addrKey)
	if err != nil {
		return err
	}
	if int(count) != n || ok && reads.read != 0 ||
		reads.read > (threshold-1)*txEntrySize {

		return fmt.Errorf("%d entries: counted %d, reading %d bytes of "+
			"levels", n, count, reads.read)
	}
	return nil
}

// TestAddrIndexStoredCountRandom ensures that across random sequences of
// blocks adding entries of an address and of disconnected blocks removing
// them, whose merges and backfills take its highest level across
// addrIndexCountLevel both ways, the address has a stored count exactly when
// its highest level is at least that level, holding its number of entries.
func TestAddrIndexStoredCountRandom(t *testing.T) {
	t.Parallel()

	threshold := minEntriesToReachLevel(addrIndexCountLevel)
	for seed := int64(1); seed <= 12; seed++ {
		rng := rand.New(rand.NewSource(seed))
		levels := &addrIndexBucket{levels: make(map[[levelKeySize]byte][]byte)}
		counts := make(countsBucket)
		var addrKey [addrKeySize]byte
		addrKey[1] = byte(seed)

		// The entries grow past the threshold by up to 600 and shrink
		// below it by up to 600, in steps whose size changes each time,
		// from a block that leaves them just below it.
		n, ups, downs := 0, 0, 0
		growing, target := true, threshold+rng.Intn(600)
		maxStep := []int{1, 3, 9, 40, 300, 1100}[rng.Intn(6)]
		for step := 0; step < 20000 && ups+downs < 12; step++ {
			k := 1 + rng.Intn(maxStep)
			if step == 0 {
				k = threshold - 1 - rng.Intn(50)
			}
			if growing {
				txLocs := make([]wire.TxLoc, k)
				txIdxs := make([]int, k)
				for i := range txLocs {
					txLocs[i] = wire.TxLoc{TxStart: n + i, TxLen: 1}
					txIdxs[i] = i
				}
				err := dbPutAddrIndexEntries(levels, counts, addrKey,
					uint32(step), txLocs, txIdxs)
				if err != nil {
					t.Fatalf("seed %d step %d: add %d: %v", seed,
						step, k, err)
				}
				if n < threshold && n+k >= threshold {
					ups++
				}
				n += k
			} else {
				k = min(k, n)
				err := dbRemoveAddrIndexEntries(levels, addrKey, k)
				if err != nil {
					t.Fatalf("seed %d step %d: remove %d: %v", seed,
						step, k, err)
				}
				err = dbRemoveAddrIndexCount(counts, addrKey, k)
				if err != nil {
					t.Fatalf("seed %d step %d: remove %d: %v", seed,
						step, k, err)
				}
				if n >= threshold && n-k < threshold {
					downs++
				}
				n -= k
			}
			if err := checkAddrCount(levels, counts, addrKey); err != nil {
				t.Fatalf("seed %d step %d, %d entries: %v", seed, step,
					n, err)
			}
			if total, _, _ := addrLevels(levels, addrKey); total != n {
				t.Fatalf("seed %d step %d: %d entries, want %d", seed,
					step, total, n)
			}
			if growing && n >= target || !growing && n <= target {
				growing = !growing
				target = threshold + rng.Intn(600)
				if !growing {
					target = threshold - 1 - rng.Intn(600)
				}
				maxStep = []int{1, 3, 9, 40, 300, 1100}[rng.Intn(6)]
			}
		}
		if ups < 5 || downs < 5 {
			t.Fatalf("seed %d: the entries crossed the threshold %d "+
				"times up and %d down", seed, ups, downs)
		}
	}
}

// TestAddrIndexPutNewLevel ensures dbPutAddrIndexEntry returns the level an
// entry's merges start, the address's new highest level, and 0 for an entry
// that starts none, also when the merges end in a half full level.
func TestAddrIndexPutNewLevel(t *testing.T) {
	t.Parallel()

	levels := &addrIndexBucket{levels: make(map[[levelKeySize]byte][]byte)}
	var addrKey [addrKeySize]byte
	var highest uint8
	for i := range minEntriesToReachLevel(addrIndexCountLevel + 1) {
		newLevel, err := dbPutAddrIndexEntry(levels, addrKey, 1,
			wire.TxLoc{TxStart: i})
		if err != nil {
			t.Fatal(err)
		}
		_, nowHighest, err := addrLevels(levels, addrKey)
		if err != nil {
			t.Fatal(err)
		}
		want := uint8(0)
		if nowHighest > highest {
			want = nowHighest
		}
		if newLevel != want {
			t.Fatalf("entry %d: new level %d, want %d", i, newLevel, want)
		}
		highest = nowHighest
	}
	if highest != addrIndexCountLevel+1 {
		t.Fatalf("highest level %d", highest)
	}
}

// TestAddrIndexStoredCountEdges ensures the stored count starts with the
// entry that starts the count level, also within a block, and ends with the
// removal that leaves the levels below it full, also of all the entries, that
// removing more entries than a wrong count holds fails without changing it,
// and that a stored count of the wrong size is reported as corruption.
func TestAddrIndexStoredCountEdges(t *testing.T) {
	t.Parallel()

	threshold := minEntriesToReachLevel(addrIndexCountLevel)
	levels := &addrIndexBucket{levels: make(map[[levelKeySize]byte][]byte)}
	counts := make(countsBucket)
	var addrKey [addrKeySize]byte
	n := 0
	add := func(k int) {
		t.Helper()
		txLocs := make([]wire.TxLoc, k)
		txIdxs := make([]int, k)
		for i := range txLocs {
			txLocs[i] = wire.TxLoc{TxStart: n + i, TxLen: 1}
			txIdxs[i] = i
		}
		err := dbPutAddrIndexEntries(levels, counts, addrKey, 1, txLocs,
			txIdxs)
		if err != nil {
			t.Fatal(err)
		}
		n += k
	}
	remove := func(k int) {
		t.Helper()
		if err := dbRemoveAddrIndexEntries(levels, addrKey, k); err != nil {
			t.Fatal(err)
		}
		if err := dbRemoveAddrIndexCount(counts, addrKey, k); err != nil {
			t.Fatal(err)
		}
		n -= k
	}
	want := func(stored bool) {
		t.Helper()
		got, ok, err := dbFetchAddrIndexStoredCount(counts, addrKey)
		if err != nil || ok != stored || ok && int(got) != n {
			t.Fatalf("%d entries: stored count %d, %v, %v, want %v", n,
				got, ok, err, stored)
		}
		if err := checkAddrCount(levels, counts, addrKey); err != nil {
			t.Fatal(err)
		}
	}

	add(threshold - 1)
	want(false)
	add(1)
	want(true)
	remove(1)
	want(false)

	// A block whose fourth entry starts the level, and the removals of a
	// disconnected block that leave the address the levels below it full.
	remove(3)
	add(10)
	want(true)
	remove(6)
	want(true)
	remove(1)
	want(false)
	add(1)
	want(true)
	remove(n)
	want(false)
	add(threshold + 100)
	want(true)
	remove(n)
	want(false)
	add(threshold)
	want(true)

	// A wrong count lower than the entries removed fails the removal.
	err := counts.Put(addrKey[:], serializeAddrIndexCount(3))
	if err != nil {
		t.Fatal(err)
	}
	var assertErr AssertError
	err = dbRemoveAddrIndexCount(counts, addrKey, 4)
	if !errors.As(err, &assertErr) {
		t.Fatalf("removing 4 entries of a count of 3: %v", err)
	}
	if got, ok, _ := dbFetchAddrIndexStoredCount(counts, addrKey); !ok || got != 3 {
		t.Fatalf("stored count %d, %v after the failed removal", got, ok)
	}
	if err := counts.Put(addrKey[:], []byte{1, 2}); err != nil {
		t.Fatal(err)
	}
	var dbErr database.Error
	_, err = dbFetchAddrIndexCount(levels, counts, addrKey)
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrCorruption {
		t.Fatalf("count of 2 bytes: %v", err)
	}
	err = dbPutAddrIndexEntries(levels, counts, addrKey, 1,
		[]wire.TxLoc{{TxStart: n}}, []int{0})
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrCorruption {
		t.Fatalf("adding to a count of 2 bytes: %v", err)
	}
	err = dbRemoveAddrIndexCount(counts, addrKey, 1)
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrCorruption {
		t.Fatalf("removing from a count of 2 bytes: %v", err)
	}
}

// addrCountChain is a chain of blocks whose transactions pay to the
// addresses of a test address index.
type addrCountChain struct {
	t      *testing.T
	db     database.DB
	idx    *AddrIndex
	addrs  []bchutil.Address
	blocks []*bchutil.Block
	stxos  [][]blockchain.SpentTxOut
	ids    uint32
}

// newAddrCountChain returns a chain of no blocks over an empty test address
// index, and numAddrs addresses.
func newAddrCountChain(t *testing.T, numAddrs int) *addrCountChain {
	t.Helper()
	db, idx := createTestAddrIndex(t)
	c := &addrCountChain{t: t, db: db, idx: idx}
	for i := range numAddrs {
		addr, err := bchutil.NewAddressPubKeyHash(
			bytes.Repeat([]byte{byte(i + 1)}, 20), &chaincfg.MainNetParams)
		if err != nil {
			t.Fatal(err)
		}
		c.addrs = append(c.addrs, addr)
	}
	return c
}

// next returns the next block, with pays[i] transactions paying to the i-th
// address after its coinbase, and the outputs they spend.
func (c *addrCountChain) next(pays ...int) (*bchutil.Block, []blockchain.SpentTxOut) {
	c.t.Helper()
	c.ids++
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: c.ids}})
	coinbase.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{txscript.OP_TRUE}})
	txns := []*wire.MsgTx{coinbase}
	var stxos []blockchain.SpentTxOut
	for i, k := range pays {
		pkScript, err := txscript.PayToAddrScript(c.addrs[i])
		if err != nil {
			c.t.Fatal(err)
		}
		for range k {
			tx := wire.NewMsgTx(1)
			tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{byte(c.ids), byte(c.ids >> 8)},
				Index: uint32(len(txns))}})
			tx.AddTxOut(&wire.TxOut{Value: 1, PkScript: pkScript})
			txns = append(txns, tx)
			stxos = append(stxos, blockchain.SpentTxOut{Amount: 1,
				PkScript: []byte{txscript.OP_TRUE}})
		}
	}
	var prev chainhash.Hash
	if len(c.blocks) > 0 {
		prev = *c.blocks[len(c.blocks)-1].Hash()
	}
	block := bchutil.NewBlock(&wire.MsgBlock{
		Header:       wire.BlockHeader{PrevBlock: prev, Nonce: c.ids},
		Transactions: txns,
	})
	block.SetHeight(int32(len(c.blocks)))
	return block, stxos
}

// connect connects the next block, paying pays[i] transactions to the i-th
// address, as the index manager does when olderRelease is false, and as a
// release that does not keep the stored counts does when it is true.
func (c *addrCountChain) connect(olderRelease bool, pays ...int) {
	c.t.Helper()
	block, stxos := c.next(pays...)
	err := c.db.Update(func(dbTx database.Tx) error {
		err := dbPutBlockIDIndexEntry(dbTx, block.Hash(), c.ids)
		if err != nil {
			return err
		}
		if !olderRelease {
			return dbIndexConnectBlock(dbTx, c.idx, block, stxos)
		}
		addrsToTxns := make(writeIndexData)
		c.idx.indexBlock(addrsToTxns, block, stxos)
		txLocs, err := block.TxLoc()
		if err != nil {
			return err
		}
		bucket := dbTx.Metadata().Bucket(addrIndexKey)
		for addrKey, txIdxs := range addrsToTxns {
			for _, txIdx := range txIdxs {
				_, err := dbPutAddrIndexEntry(bucket, addrKey, c.ids,
					txLocs[txIdx])
				if err != nil {
					return err
				}
			}
		}
		return dbPutIndexerTip(dbTx, addrIndexKey, block.Hash(),
			block.Height())
	})
	if err != nil {
		c.t.Fatal(err)
	}
	c.blocks = append(c.blocks, block)
	c.stxos = append(c.stxos, stxos)
}

// disconnect disconnects the last block, as the index manager does when
// olderRelease is false, and as a release that does not keep the stored
// counts does when it is true.
func (c *addrCountChain) disconnect(olderRelease bool) {
	c.t.Helper()
	block := c.blocks[len(c.blocks)-1]
	stxos := c.stxos[len(c.stxos)-1]
	err := c.db.Update(func(dbTx database.Tx) error {
		if !olderRelease {
			return dbIndexDisconnectBlock(dbTx, c.idx, block, stxos)
		}
		addrsToTxns := make(writeIndexData)
		c.idx.indexBlock(addrsToTxns, block, stxos)
		bucket := dbTx.Metadata().Bucket(addrIndexKey)
		for addrKey, txIdxs := range addrsToTxns {
			err := dbRemoveAddrIndexEntries(bucket, addrKey, len(txIdxs))
			if err != nil {
				return err
			}
		}
		return dbPutIndexerTip(dbTx, addrIndexKey,
			&block.MsgBlock().Header.PrevBlock, block.Height()-1)
	})
	if err != nil {
		c.t.Fatal(err)
	}
	c.blocks = c.blocks[:len(c.blocks)-1]
	c.stxos = c.stxos[:len(c.stxos)-1]
}

// check checks the stored counts of all the addresses against their levels as
// checkAddrCount does, that the counts bucket holds no others, that the
// counts were kept at the index's tip, and that TxRegionsAndCountForAddress
// gives each address's number of entries.
func (c *addrCountChain) check(wantCounts ...int) {
	c.t.Helper()
	err := c.db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(addrIndexKey)
		counts := bucket.Bucket(addrIndexCountsKey)
		if counts == nil {
			return errors.New("no counts bucket")
		}
		stored := make(map[[addrKeySize]byte]bool)
		for i, addr := range c.addrs {
			addrKey, err := addrToKey(addr)
			if err != nil {
				return err
			}
			if err := checkAddrCount(bucket, counts, addrKey); err != nil {
				return fmt.Errorf("address %d: %v", i, err)
			}
			_, _, count, err := c.idx.TxRegionsAndCountForAddress(dbTx,
				addr, 0, 10)
			if err != nil {
				return err
			}
			if int(count) != wantCounts[i] {
				return fmt.Errorf("address %d: count %d, want %d", i,
					count, wantCounts[i])
			}
			_, stored[addrKey], _ = dbFetchAddrIndexStoredCount(counts,
				addrKey)
		}
		err := counts.ForEach(func(k, _ []byte) error {
			if len(k) != addrKeySize || !stored[[addrKeySize]byte(k)] {
				return fmt.Errorf("stored count for key %x", k)
			}
			return nil
		})
		if err != nil {
			return err
		}
		kept, err := dbAddrIndexCountsKept(dbTx)
		if err != nil {
			return err
		}
		if !kept {
			return errors.New("the counts were not kept at the tip")
		}
		version, err := dbFetchAddrIndexVersion(dbTx)
		if err != nil || version != addrIndexVersion {
			return fmt.Errorf("version %d, %v", version, err)
		}
		return nil
	})
	if err != nil {
		c.t.Fatal(err)
	}
}

// TestAddrIndexCountBlocks ensures the address index keeps the stored counts
// of the addresses of connected and disconnected blocks, also of an address
// that gets several entries and crosses the threshold in one block, and that
// TxRegionsAndCountForAddress reads the stored count of an address with one,
// and only the levels a page needs.
func TestAddrIndexCountBlocks(t *testing.T) {
	t.Parallel()

	c := newAddrCountChain(t, 3)
	c.check(0, 0, 0)
	c.connect(false, 2000, 30, 1)
	c.check(2000, 30, 1)
	c.connect(false, 1000, 20, 2040)
	c.check(3000, 50, 2041)

	// A page of the newest 10 of the 3000 transactions reads the levels it
	// needs, the hashes of its blocks and the stored count.
	err := c.db.View(func(dbTx database.Tx) error {
		reads := &readCountingTx{Tx: dbTx}
		regions, _, count, err := c.idx.TxRegionsAndCountForAddress(reads,
			c.addrs[0], 0, 10)
		if err != nil {
			return err
		}
		if len(regions) != 10 || count != 3000 || reads.read > 1024 {
			return fmt.Errorf("%d regions, count %d, %d bytes read",
				len(regions), count, reads.read)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	c.disconnect(false)
	c.check(2000, 30, 1)
	c.connect(false, 41, 0, 0)
	c.check(2041, 30, 1)
	c.disconnect(false)
	c.disconnect(false)
	c.check(0, 0, 0)
}

// readCountingTx counts the bytes read from the buckets of a database
// transaction.
type readCountingTx struct {
	database.Tx
	read int
}

func (tx *readCountingTx) Metadata() database.Bucket {
	return &readCountingDBBucket{dbBucket: tx.Tx.Metadata(), tx: tx}
}

// dbBucket names the database.Bucket embedded in readCountingDBBucket apart
// from its Bucket method.
type dbBucket = database.Bucket

// readCountingDBBucket counts the bytes read from a bucket, and its nested
// buckets, for a readCountingTx.
type readCountingDBBucket struct {
	dbBucket
	tx *readCountingTx
}

func (b *readCountingDBBucket) Bucket(key []byte) database.Bucket {
	nested := b.dbBucket.Bucket(key)
	if nested == nil {
		return nil
	}
	return &readCountingDBBucket{dbBucket: nested, tx: b.tx}
}

func (b *readCountingDBBucket) Get(key []byte) []byte {
	value := b.dbBucket.Get(key)
	b.tx.read += len(value)
	return value
}

// setAddrIndexCountBatch sets addrIndexCountBatch for the test.
func setAddrIndexCountBatch(t *testing.T, batch int) {
	saved := addrIndexCountBatch
	addrIndexCountBatch = batch
	t.Cleanup(func() { addrIndexCountBatch = saved })
}

// TestAddrIndexStoreCounts ensures Migrate gives a version 2 index, as 0.22.3
// leaves it, the stored counts and marks it the current version, reading its
// keys in batches whose last address may go on in the next, that when
// interrupted it leaves the index as an unfinished pass, and that it does the
// pass again on the next start, as it does for missing counts.
func TestAddrIndexStoreCounts(t *testing.T) {
	setAddrIndexCountBatch(t, 3)

	c := newAddrCountChain(t, 4)
	c.connect(true, 2100, 7, 2041, 2040)
	c.connect(true, 5, 0, 0, 0)
	wantCounts := []int{2105, 7, 2041, 2040}
	err := c.db.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(addrIndexKey)
		if err := bucket.DeleteBucket(addrIndexCountsKey); err != nil {
			return err
		}
		if err := bucket.Delete(addrIndexCountsTipKey); err != nil {
			return err
		}
		return bucket.Put(addrIndexVersionKey, []byte{0, 0, 0, 2})
	})
	if err != nil {
		t.Fatal(err)
	}

	interrupt := make(chan struct{})
	close(interrupt)
	if err := c.idx.Migrate(c.db, interrupt); err != errInterruptRequested {
		t.Fatalf("interrupted Migrate: %v", err)
	}
	err = c.db.View(func(dbTx database.Tx) error {
		version, err := dbFetchAddrIndexVersion(dbTx)
		if err != nil {
			return err
		}
		kept, err := dbAddrIndexCountsKept(dbTx)
		if err != nil {
			return err
		}
		if version != 2 || kept {
			return fmt.Errorf("interrupted: version %d, counts kept %v",
				version, kept)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := c.idx.Migrate(c.db, nil); err != nil {
			t.Fatal(err)
		}
		c.check(wantCounts...)
	}

	// The index keeps the counts from now on.
	c.connect(false, 0, 2034, 0, 1)
	c.check(2105, 2041, 2041, 2041)
	c.disconnect(false)
	c.check(wantCounts...)

	// Counts kept at the tip but missing are counted again, also after an
	// interrupted pass.
	err = c.db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().Bucket(addrIndexKey).DeleteBucket(
			addrIndexCountsKey)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.idx.Migrate(c.db, interrupt); err != errInterruptRequested {
		t.Fatalf("interrupted Migrate: %v", err)
	}
	if err := c.idx.Migrate(c.db, nil); err != nil {
		t.Fatal(err)
	}
	c.check(wantCounts...)
}

// TestAddrIndexCountsRecount ensures the stored counts are counted again when
// a release that does not keep them has connected or disconnected blocks
// since they were kept, as told by the tip they were kept at, replacing those
// of the addresses whose entries changed and deleting those of the addresses
// that left the count level, and that they are left alone when they were kept
// at the index's tip.
func TestAddrIndexCountsRecount(t *testing.T) {
	t.Parallel()

	c := newAddrCountChain(t, 3)
	c.connect(false, 2040, 2045, 2000)
	c.connect(false, 1, 0, 0)
	c.check(2041, 2045, 2000)

	// An older release disconnects a block and connects two others.
	c.disconnect(true)
	c.connect(true, 0, 0, 1)
	c.connect(true, 0, 0, 40)
	if err := c.idx.Migrate(c.db, nil); err != nil {
		t.Fatal(err)
	}
	c.check(2040, 2045, 2041)
	c.disconnect(true)
	c.disconnect(true)
	c.connect(true, 0, 5, 0)
	if err := c.idx.Migrate(c.db, nil); err != nil {
		t.Fatal(err)
	}
	c.check(2040, 2050, 2000)

	// An older release that leaves the index at the tip the counts were
	// kept at leaves each address the entries it had.
	c.connect(true, 0, 0, 41)
	c.disconnect(true)
	if err := c.idx.Migrate(c.db, nil); err != nil {
		t.Fatal(err)
	}
	c.check(2040, 2050, 2000)

	// An interrupt during the last batch of keys leaves the pass done.
	c.connect(true, 1, 0, 0)
	interrupt := make(chan struct{})
	close(interrupt)
	if err := c.idx.Migrate(c.db, interrupt); err != nil {
		t.Fatal(err)
	}
	c.check(2041, 2050, 2000)

	// Counts kept at the tip are not counted again.
	addrKey, err := addrToKey(c.addrs[1])
	if err != nil {
		t.Fatal(err)
	}
	err = c.db.Update(func(dbTx database.Tx) error {
		counts := dbTx.Metadata().Bucket(addrIndexKey).Bucket(
			addrIndexCountsKey)
		return counts.Put(addrKey[:], serializeAddrIndexCount(1))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.idx.Migrate(c.db, nil); err != nil {
		t.Fatal(err)
	}
	err = c.db.View(func(dbTx database.Tx) error {
		counts := dbTx.Metadata().Bucket(addrIndexKey).Bucket(
			addrIndexCountsKey)
		count, ok, err := dbFetchAddrIndexStoredCount(counts, addrKey)
		if err != nil || !ok || count != 1 {
			return fmt.Errorf("stored count %d, %v, %v, want the 1 put",
				count, ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestAddrIndexDropCounts ensures dropping the address index drops the stored
// counts and the tip they were kept at with it, so an index created again
// starts with none, kept at its new tip.
func TestAddrIndexDropCounts(t *testing.T) {
	t.Parallel()

	c := newAddrCountChain(t, 1)
	c.connect(false, 2100)
	c.check(2100)
	if err := DropAddrIndex(c.db, nil); err != nil {
		t.Fatal(err)
	}
	err := c.db.View(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		if meta.Bucket(addrIndexKey) != nil ||
			meta.Bucket(indexTipsBucketName).Get(addrIndexKey) != nil {

			return errors.New("the address index is still there")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = c.db.Update(func(dbTx database.Tx) error {
		return NewManager(c.db, []Indexer{c.idx}).maybeCreateIndexes(dbTx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.idx.Migrate(c.db, nil); err != nil {
		t.Fatal(err)
	}
	c.blocks, c.stxos = nil, nil
	c.check(0)
	c.connect(false, 3)
	c.check(3)
}
