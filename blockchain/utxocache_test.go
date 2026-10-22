package blockchain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

var (
	// opTrueScript is simply a public key script that contains the OP_TRUE
	// opcode.  It is defined here to reduce garbage creation.
	opTrueScript = []byte{txscript.OP_TRUE}

	// lowFee is a single satoshi and exists to make the test code more
	// readable.
	lowFee = bchutil.Amount(1)
)

// uniqueOpReturnScript returns a standard provably-pruneable OP_RETURN script
// with a random uint64 encoded as the data.  The data is padded so that the
// spending transactions built by addBlock reach the minimum transaction size.
func uniqueOpReturnScript() []byte {
	rand, err := wire.RandomUint64()
	if err != nil {
		panic(err)
	}

	data := make([]byte, 28)
	binary.LittleEndian.PutUint64(data[0:8], rand)

	builder := txscript.NewScriptBuilder()
	script, err := builder.AddOp(txscript.OP_RETURN).AddData(data).Script()
	if err != nil {
		panic(err)
	}
	return script
}

// solveBlock attempts to find a nonce which makes the passed block header hash
// to a value less than the target difficulty.  When a successful solution is
// found true is returned and the nonce field of the passed header is updated
// with the solution.  False is returned if no solution exists.
func solveBlock(header *wire.BlockHeader) bool {
	// sbResult is used by the solver goroutines to send results.
	type sbResult struct {
		found bool
		nonce uint32
	}

	// solver accepts a block header and a nonce range to test. It is
	// intended to be run as a goroutine.
	targetDifficulty := CompactToBig(header.Bits)
	quit := make(chan bool)
	results := make(chan sbResult)
	solver := func(hdr wire.BlockHeader, startNonce, stopNonce uint32) {
		// We need to modify the nonce field of the header, so make sure
		// we work with a copy of the original header.
		for i := startNonce; i >= startNonce && i <= stopNonce; i++ {
			select {
			case <-quit:
				return
			default:
				hdr.Nonce = i
				hash := hdr.BlockHash()
				if HashToBig(&hash).Cmp(
					targetDifficulty) <= 0 {

					results <- sbResult{true, i}
					return
				}
			}
		}
		results <- sbResult{false, 0}
	}

	startNonce := uint32(1)
	stopNonce := uint32(math.MaxUint32)
	numCores := uint32(runtime.NumCPU())
	noncesPerCore := (stopNonce - startNonce) / numCores
	for i := range numCores {
		rangeStart := startNonce + (noncesPerCore * i)
		rangeStop := startNonce + (noncesPerCore * (i + 1)) - 1
		if i == numCores-1 {
			rangeStop = stopNonce
		}
		go solver(*header, rangeStart, rangeStop)
	}
	for range numCores {
		result := <-results
		if result.found {
			close(quit)
			header.Nonce = result.nonce
			return true
		}
	}

	return false
}

// spendableOut represents a transaction output that is spendable along with
// additional metadata such as the block its in and how much it pays.
type spendableOut struct {
	prevOut wire.OutPoint
	amount  bchutil.Amount
}

// makeSpendableOutForTx returns a spendable output for the given transaction
// and transaction output index within the transaction.
func makeSpendableOutForTx(tx *wire.MsgTx, txOutIndex uint32) spendableOut {
	return spendableOut{
		prevOut: wire.OutPoint{
			Hash:  tx.TxHash(),
			Index: txOutIndex,
		},
		amount: bchutil.Amount(tx.TxOut[txOutIndex].Value),
	}
}

// addBlock adds a block to the blockchain that succeeds prev.  The blocks spends
// all the provided spendable outputs.  The new block is returned, together with
// the new spendable outputs created in the block.
//
// Panics on errors.
func addBlock(chain *BlockChain, prev *bchutil.Block, spends []*spendableOut) (*bchutil.Block, []*spendableOut) {
	blockHeight := prev.Height() + 1
	txns := make([]*wire.MsgTx, 0, 1+len(spends))

	// Create and add coinbase tx.
	coinbaseScript, err := txscript.NewScriptBuilder().
		AddInt64(int64(blockHeight)).
		AddInt64(int64(0)).Script()
	if err != nil {
		panic(err)
	}
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex),
		Sequence:        wire.MaxTxInSequenceNum,
		SignatureScript: coinbaseScript,
	})
	cb.AddTxOut(&wire.TxOut{
		Value:    CalcBlockSubsidy(blockHeight, chain.chainParams),
		PkScript: opTrueScript,
	})
	if size := cb.SerializeSize(); size < MagneticAnomalyMinTransactionSize {
		cb.TxIn[0].SignatureScript = append(cb.TxIn[0].SignatureScript,
			make([]byte, MagneticAnomalyMinTransactionSize-size)...)
	}
	txns = append(txns, cb)

	// Spend all txs to be spent.
	for _, spend := range spends {
		spendTx := wire.NewMsgTx(1)
		spendTx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: spend.prevOut,
			Sequence:         wire.MaxTxInSequenceNum,
			SignatureScript:  nil,
		})
		spendTx.AddTxOut(wire.NewTxOut(int64(spend.amount-lowFee),
			opTrueScript, wire.TokenData{}))
		// Add a random (prunable) OP_RETURN output to make the txid unique.
		spendTx.AddTxOut(wire.NewTxOut(0, uniqueOpReturnScript(), wire.TokenData{}))

		cb.TxOut[0].Value += int64(lowFee)
		txns = append(txns, spendTx)
	}

	// Put the spends in canonical order.
	slices.SortFunc(txns[1:], func(a, b *wire.MsgTx) int {
		ha, hb := a.TxHash(), b.TxHash()
		return ha.Compare(&hb)
	})

	// Create spendable outs to return at the end.
	outs := make([]*spendableOut, len(txns))
	for i, tx := range txns {
		out := makeSpendableOutForTx(tx, 0)
		outs[i] = &out
	}

	// Build the block.

	// Use a timestamp that is one second after the previous block unless
	// this is the first block in which case the current time is used.
	var ts time.Time
	if blockHeight == 1 {
		ts = time.Unix(time.Now().Unix(), 0)
	} else {
		ts = prev.MsgBlock().Header.Timestamp.Add(time.Second)
	}

	// Calculate merkle root.
	utilTxns := make([]*bchutil.Tx, 0, len(txns))
	for _, tx := range txns {
		utilTxns = append(utilTxns, bchutil.NewTx(tx))
	}
	merkles := BuildMerkleTreeStore(utilTxns)

	block := bchutil.NewBlock(&wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    1,
			PrevBlock:  *prev.Hash(),
			MerkleRoot: *merkles[len(merkles)-1],
			Bits:       chain.chainParams.PowLimitBits,
			Timestamp:  ts,
			Nonce:      0, // To be solved.
		},
		Transactions: txns,
	})
	block.SetHeight(blockHeight)

	// Solve the block.
	if !solveBlock(&block.MsgBlock().Header) {
		panic(fmt.Sprintf("Unable to solve block at height %d", blockHeight))
	}

	_, _, err = chain.ProcessBlock(block, BFNone)
	if err != nil {
		panic(err)
	}

	return block, outs
}

// assertConsistencyState asserts the utxo consistency states of the blockchain.
func assertConsistencyState(t *testing.T, chain *BlockChain, code byte, hash *chainhash.Hash) {
	var actualCode byte
	var actualHash *chainhash.Hash
	err := chain.db.View(func(dbTx database.Tx) (err error) {
		actualCode, actualHash, err = dbFetchUtxoStateConsistency(dbTx)
		return
	})
	if err != nil {
		t.Fatalf("Error fetching utxo state consistency: %v", err)
	}
	if actualCode != code {
		t.Fatalf("Unexpected consistency code: %d instead of %d",
			actualCode, code)
	}
	if !actualHash.IsEqual(hash) {
		t.Fatalf("Unexpected consistency hash: %v instead of %v",
			actualHash, hash)
	}
}

func parseOutpointKey(b []byte) wire.OutPoint {
	op := wire.OutPoint{}
	copy(op.Hash[:], b[:chainhash.HashSize])
	idx, _ := deserializeVLQ(b[chainhash.HashSize:])
	op.Index = uint32(idx)
	return op
}

// assertNbEntriesOnDisk asserts that the total number of utxo entries on the
// disk is equal to the given expected number.
func assertNbEntriesOnDisk(t *testing.T, chain *BlockChain, expectedNumber int) {
	t.Log("Asserting nb entries on disk...")
	var nb int
	err := chain.db.View(func(dbTx database.Tx) error {
		cursor := dbTx.Metadata().Bucket(utxoSetBucketName).Cursor()
		nb = 0
		for b := cursor.First(); b; b = cursor.Next() {
			nb++
			entry, err := DeserializeUtxoEntry(cursor.Value())
			if err != nil {
				t.Fatalf("Failed to deserialize entry: %v", err)
			}
			outpoint := parseOutpointKey(cursor.Key())
			t.Log(outpoint.String())
			t.Log(spew.Sdump(entry))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Error fetching utxo entries: %v", err)
	}
	if nb != expectedNumber {
		t.Fatalf("Expected %d elements in the UTXO set, but found %d",
			expectedNumber, nb)
	}
	t.Log("Assertion done")
}

// utxoCacheTestChain creates a test BlockChain to be used for utxo cache tests.
// It uses the regression test parameters, a coin maturity of 1 block and sets
// the cache size limit to 10 MiB.
func utxoCacheTestChain(testName string) (*BlockChain, *chaincfg.Params, func()) {
	params := chaincfg.RegressionNetParams
	chain, tearDown, err := chainSetup(testName, &params)
	if err != nil {
		panic(fmt.Sprintf("error loading blockchain with database: %v", err))
	}

	chain.TstSetCoinbaseMaturity(1)
	chain.utxoCache.maxTotalMemoryUsage = 10 * 1024 * 1024

	return chain, &params, tearDown
}

func TestUtxoCache_SimpleFlush(t *testing.T) {
	chain, params, tearDown := utxoCacheTestChain("TestUtxoCache_SimpleFlush")
	defer tearDown()
	cache := chain.utxoCache
	tip := bchutil.NewBlock(params.GenesisBlock)

	// The chainSetup init triggered write of consistency status of genesis.
	assertConsistencyState(t, chain, ucsConsistent, params.GenesisHash)
	assertNbEntriesOnDisk(t, chain, 0)

	// LastFlushHash starts with genesis.
	if cache.lastFlushHash != *params.GenesisHash {
		t.Fatalf("lastFlushHash before first flush expected to be "+
			"genesis block hash, instead was %v", cache.lastFlushHash)
	}

	// First, add 10 utxos without flushing.
	for range 10 {
		tip, _ = addBlock(chain, tip, nil)
	}
	if len(cache.cachedEntries) != 10 {
		t.Fatalf("Expected 10 entries, has %d instead", len(cache.cachedEntries))
	}

	// All elements should be fresh and modified.
	for outpoint, elem := range cache.cachedEntries {
		if elem == nil {
			t.Fatalf("Unexpected nil entry found for %v", outpoint)
		}
		if elem.packedFlags&tfModified == 0 {
			t.Fatal("Entry should be marked mofified")
		}
		if elem.packedFlags&tfFresh == 0 {
			t.Fatal("Entry should be marked fresh")
		}
	}

	// Not flushed yet.
	assertConsistencyState(t, chain, ucsConsistent, params.GenesisHash)
	assertNbEntriesOnDisk(t, chain, 0)

	// Flush.
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("unexpected error while flushing cache: %v", err)
	}
	if len(cache.cachedEntries) != 0 {
		t.Fatalf("Expected 0 entries, has %d instead", len(cache.cachedEntries))
	}
	assertConsistencyState(t, chain, ucsConsistent, tip.Hash())
	assertNbEntriesOnDisk(t, chain, 10)
}

func TestUtxoCache_ThresholdPeriodicFlush(t *testing.T) {
	chain, params, tearDown := utxoCacheTestChain("TestUtxoCache_ThresholdPeriodicFlush")
	defer tearDown()
	cache := chain.utxoCache
	tip := bchutil.NewBlock(params.GenesisBlock)

	// Set the limit to the size of 10 empty elements.  This will trigger
	// flushing when adding 10 non-empty elements.
	cache.maxTotalMemoryUsage = (*UtxoEntry)(nil).memoryUsage() * 10

	// Add 10 elems and let it exceed the threshold.
	var flushedAt *chainhash.Hash
	for range 10 {
		tip, _ = addBlock(chain, tip, nil)
		if len(cache.cachedEntries) == 0 {
			flushedAt = tip.Hash()
		}
	}

	// Should have flushed in the meantime.
	if flushedAt == nil {
		t.Fatal("should have flushed")
	}
	assertConsistencyState(t, chain, ucsConsistent, flushedAt)
	if len(cache.cachedEntries) >= 10 {
		t.Fatalf("Expected less than 10 entries, has %d instead",
			len(cache.cachedEntries))
	}

	// Make sure flushes on periodic.
	cache.maxTotalMemoryUsage = (90*cache.totalMemoryUsage())/100 - 1
	if err := chain.FlushCachedState(FlushPeriodic); err != nil {
		t.Fatalf("unexpected error while flushing cache: %v", err)
	}
	if len(cache.cachedEntries) != 0 {
		t.Fatalf("Expected 0 entries, has %d instead", len(cache.cachedEntries))
	}
	assertConsistencyState(t, chain, ucsConsistent, tip.Hash())
	assertNbEntriesOnDisk(t, chain, 10)
}

func TestUtxoCache_Reorg(t *testing.T) {
	chain, params, tearDown := utxoCacheTestChain("TestUtxoCache_Reorg")
	defer tearDown()
	tip := bchutil.NewBlock(params.GenesisBlock)

	// Create base blocks 1 and 2 that will not be reorged.
	// Spend the outputs of block 1.
	var emptySpendableOuts []*spendableOut
	b1, spendableOuts1 := addBlock(chain, tip, emptySpendableOuts)
	b2, spendableOuts2 := addBlock(chain, b1, spendableOuts1)
	t.Log(spew.Sdump(spendableOuts2))
	//                 db       cache
	// block 1:                  stxo
	// block 2:                  utxo

	// Commit the two base blocks to DB
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("unexpected error while flushing cache: %v", err)
	}
	//                 db       cache
	// block 1:       stxo
	// block 2:       utxo
	assertConsistencyState(t, chain, ucsConsistent, b2.Hash())
	assertNbEntriesOnDisk(t, chain, len(spendableOuts2))

	// Add blocks 3 and 4 that will be orphaned.
	// Spend the outputs of block 2 and 3a.
	b3a, spendableOuts3 := addBlock(chain, b2, spendableOuts2)
	addBlock(chain, b3a, spendableOuts3)
	//                 db       cache
	// block 1:       stxo
	// block 2:       utxo       stxo      << these are left spent without flush
	// ---
	// block 3a:                 stxo
	// block 4a:                 utxo

	// Build an alternative chain of blocks 3 and 4 + new 5th, spending none of the outputs
	b3b, altSpendableOuts3 := addBlock(chain, b2, nil)
	b4b, altSpendableOuts4 := addBlock(chain, b3b, nil)
	b5b, altSpendableOuts5 := addBlock(chain, b4b, nil)
	totalSpendableOuts := spendableOuts2[:]
	totalSpendableOuts = append(totalSpendableOuts, altSpendableOuts3...)
	totalSpendableOuts = append(totalSpendableOuts, altSpendableOuts4...)
	totalSpendableOuts = append(totalSpendableOuts, altSpendableOuts5...)
	t.Log(spew.Sdump(totalSpendableOuts))
	//                 db       cache
	// block 1:       stxo
	// block 2:       utxo       utxo     << now they should become utxo
	// ---
	// block 3b:                 utxo
	// block 4b:                 utxo
	// block 5b:                 utxo

	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("unexpected error while flushing cache: %v", err)
	}
	//                 db       cache
	// block 1:       stxo
	// block 2:       utxo
	// ---
	// block 3b:      utxo
	// block 4b:      utxo
	// block 5b:      utxo
	assertConsistencyState(t, chain, ucsConsistent, b5b.Hash())
	assertNbEntriesOnDisk(t, chain, len(totalSpendableOuts))
}

func TestUtxoCache_InitConsistentState(t *testing.T) {
	// First we are going to simulate a hard shutdown where the utxo cache has
	// entries (spends plus new utxos) that are not persisted to the database.
	//
	// On next startup the chain height should be ahead of the utxo set height
	// and this should trigger the utxo set on disk to be rolled forward using
	// the blocks from the chain.
	t.Run("Hard shutdown before flush", func(t *testing.T) {
		chain, params, tearDown := utxoCacheTestChain("TestUtxoCache_InitConsistentState")
		defer tearDown()
		tip := bchutil.NewBlock(params.GenesisBlock)

		// Create base blocks 1 and 2 and flush to disk.
		var emptySpendableOuts []*spendableOut
		b1, spendableOuts1 := addBlock(chain, tip, emptySpendableOuts)
		b2, spendableOuts2 := addBlock(chain, b1, spendableOuts1)
		t.Log(spew.Sdump(spendableOuts2))
		//                 db       cache
		// block 1:                  stxo
		// block 2:                  utxo

		if err := chain.FlushCachedState(FlushRequired); err != nil {
			t.Fatalf("unexpected error while flushing cache: %v", err)
		}

		//                 db       cache
		// block 1:       stxo
		// block 2:       utxo
		assertConsistencyState(t, chain, ucsConsistent, b2.Hash())
		assertNbEntriesOnDisk(t, chain, len(spendableOuts2))

		// Add blocks 3 and 4.
		// Spend the outputs of block 2 and 3.
		b3, spendableOuts3 := addBlock(chain, b2, spendableOuts2)
		b4, spendableOuts4 := addBlock(chain, b3, spendableOuts3)
		//                 db       cache
		// block 1:       stxo
		// block 2:       utxo       stxo      << these are left spent without flush
		// ---
		// block 3:                  stxo
		// block 4:                  utxo

		// At this point the chain should have blocks 1-4 saved on disk, but the utxo set
		// should only be persisted through block 2. Calling InitConsistentState at this point
		// should roll the utxo set forward to block 4.

		// Reset the utxo cache
		chain.utxoCache = newUtxoCache(chain.db, 10*1024*1024)

		err := chain.utxoCache.InitConsistentState(chain.bestChain.Tip(), chain.index, false, nil)
		if err != nil {
			t.Fatalf("failed to init utxo cache: %v", err)
		}

		//                 db       cache
		// block 1:       stxo
		// block 2:       stxo
		// block 3:       stxo
		// block 4:       utxo

		assertConsistencyState(t, chain, ucsConsistent, b4.Hash())
		assertNbEntriesOnDisk(t, chain, len(spendableOuts4))
	})

	// Next we are going to simulate a crash mid-flush where the state is left as ucsFlushOngoing.
	t.Run("Hard shutdown mid flush", func(t *testing.T) {
		chain, params, tearDown := utxoCacheTestChain("TestUtxoCache_InitConsistentState")
		defer tearDown()
		tip := bchutil.NewBlock(params.GenesisBlock)

		// Create base blocks 1 and 2 and flush to disk.
		var emptySpendableOuts []*spendableOut
		b1, spendableOuts1 := addBlock(chain, tip, emptySpendableOuts)
		b2, spendableOuts2 := addBlock(chain, b1, spendableOuts1)
		t.Log(spew.Sdump(spendableOuts2))
		//                 db       cache
		// block 1:                  stxo
		// block 2:                  utxo

		if err := chain.FlushCachedState(FlushRequired); err != nil {
			t.Fatalf("unexpected error while flushing cache: %v", err)
		}

		//                 db       cache
		// block 1:       stxo
		// block 2:       utxo
		assertConsistencyState(t, chain, ucsConsistent, b2.Hash())
		assertNbEntriesOnDisk(t, chain, len(spendableOuts2))

		// Add blocks 3 and 4.
		// Spend the outputs of block 2 and 3.
		b3, spendableOuts3 := addBlock(chain, b2, spendableOuts2)
		b4, spendableOuts4 := addBlock(chain, b3, spendableOuts3)
		//                 db       cache
		// block 1:       stxo
		// block 2:       utxo       stxo      << these are left spent without flush
		// ---
		// block 3:                  stxo
		// block 4:                  utxo

		// At this point the chain should have blocks 1-4 saved on disk, but the utxo set
		// should only be persisted through block 2.

		// Now we're going to manually save one utxo and delete one spend and then stop,
		// simulating a hard shutdown.

		addedUtxo, deletedSpend := false, false
		err := chain.db.Update(func(dbTx database.Tx) error {
			for outpoint, entry := range chain.utxoCache.cachedEntries {
				if entry == nil {
					continue
				}
				if entry.IsSpent() && !deletedSpend {
					if err := dbDeleteUtxoEntries(dbTx, []wire.OutPoint{outpoint}); err != nil {
						return err
					}
					deletedSpend = true
				}
				if !entry.IsSpent() && !addedUtxo {
					if err := dbPutUtxoEntries(dbTx, map[wire.OutPoint]*UtxoEntry{outpoint: entry}); err != nil {
						return err
					}
					addedUtxo = true
				}
				if deletedSpend && addedUtxo {
					break
				}
			}

			return dbPutUtxoStateConsistency(dbTx, ucsFlushOngoing, &chain.utxoCache.lastFlushHash)
		})
		if err != nil {
			t.Fatalf("unexpected error while puting utxo state to db: %v", err)
		}

		// Reset the utxo cache
		chain.utxoCache = newUtxoCache(chain.db, 10*1024*1024)

		err = chain.utxoCache.InitConsistentState(chain.bestChain.Tip(), chain.index, false, nil)
		if err != nil {
			t.Fatalf("failed to init utxo cache: %v", err)
		}

		//                 db       cache
		// block 1:       stxo
		// block 2:       stxo
		// block 3:       stxo
		// block 4:       utxo

		assertConsistencyState(t, chain, ucsConsistent, b4.Hash())
		assertNbEntriesOnDisk(t, chain, len(spendableOuts4))
	})

	// A status hash that is not in the best chain must be reported as an
	// error rather than walked past the genesis block.
	t.Run("Status hash not in best chain", func(t *testing.T) {
		chain, params, tearDown := utxoCacheTestChain("TestUtxoCache_InitConsistentState")
		defer tearDown()
		tip := bchutil.NewBlock(params.GenesisBlock)

		var emptySpendableOuts []*spendableOut
		addBlock(chain, tip, emptySpendableOuts)

		unknownHash := chainhash.Hash{0x01}
		err := chain.db.Update(func(dbTx database.Tx) error {
			return dbPutUtxoStateConsistency(dbTx, ucsConsistent, &unknownHash)
		})
		if err != nil {
			t.Fatalf("unexpected error while puting utxo state to db: %v", err)
		}

		chain.utxoCache = newUtxoCache(chain.db, 10*1024*1024)
		err = chain.utxoCache.InitConsistentState(chain.bestChain.Tip(), chain.index, false, nil)
		if _, ok := err.(AssertError); !ok {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestUtxoCache_FlushReleasesMap checks that a full flush does not keep the
// memory of the cache's map once the map is empty.
func TestUtxoCache_FlushReleasesMap(t *testing.T) {
	chain, _, tearDown := utxoCacheTestChain("TestUtxoCache_FlushReleasesMap")
	defer tearDown()
	cache := chain.utxoCache

	heapAlloc := func() uint64 {
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.HeapAlloc
	}
	before := heapAlloc()

	const n = 200000
	pkScript := []byte{0x51}
	for i := range n {
		var op wire.OutPoint
		binary.LittleEndian.PutUint64(op.Hash[:], uint64(i))
		entry := NewUtxoEntry(&wire.TxOut{Value: 1, PkScript: pkScript}, 1, false)
		if err := cache.addEntry(op, entry, false); err != nil {
			t.Fatal(err)
		}
	}
	filled := heapAlloc()

	// Any hash other than the last flushed one makes flush run.
	if err := cache.flush(&BestState{Hash: chainhash.Hash{1}}); err != nil {
		t.Fatal(err)
	}
	if len(cache.cachedEntries) != 0 {
		t.Fatalf("expected empty cache, got %d entries", len(cache.cachedEntries))
	}
	after := heapAlloc()

	// The flushed entries stay in the database write cache until it is
	// flushed, so measure only what dropping the cache's map would free.
	cache.cachedEntries = make(map[wire.OutPoint]*UtxoEntry)
	mapKept := int64(after) - int64(heapAlloc())
	grown := filled - before
	t.Logf("cache grew by %d KiB, its emptied map still held %d KiB",
		grown/1024, mapKept/1024)
	if mapKept > int64(grown/10) {
		t.Fatalf("emptied map still held %d KiB after a full flush",
			mapKept/1024)
	}
}

// TestUtxoCache_FetchEntryUncached ensures FetchUtxoEntryUncached returns
// the same entries as FetchUtxoEntry without adding to the cache.
func TestUtxoCache_FetchEntryUncached(t *testing.T) {
	chain, params, tearDown := utxoCacheTestChain("TestUtxoCache_FetchEntryUncached")
	defer tearDown()
	cache := chain.utxoCache
	tip := bchutil.NewBlock(params.GenesisBlock)

	tip, flushedOuts := addBlock(chain, tip, nil)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("FlushCachedState: %v", err)
	}
	_, cachedOuts := addBlock(chain, tip, nil)
	if len(cache.cachedEntries) != 1 {
		t.Fatalf("got %d cached entries, want 1", len(cache.cachedEntries))
	}

	missing := wire.OutPoint{Hash: chainhash.Hash{1}}
	outpoints := []wire.OutPoint{
		flushedOuts[0].prevOut,
		cachedOuts[0].prevOut,
		missing,
	}
	entries := make([]*UtxoEntry, len(outpoints))
	for i, op := range outpoints {
		entry, err := chain.FetchUtxoEntryUncached(op)
		if err != nil {
			t.Fatalf("FetchUtxoEntryUncached(%v): %v", op, err)
		}
		entries[i] = entry
	}
	if len(cache.cachedEntries) != 1 {
		t.Fatalf("got %d cached entries after FetchUtxoEntryUncached, "+
			"want 1", len(cache.cachedEntries))
	}

	for i, op := range outpoints {
		entry := entries[i]
		want, err := chain.FetchUtxoEntry(op)
		if err != nil {
			t.Fatalf("FetchUtxoEntry(%v): %v", op, err)
		}
		if (entry == nil) != (op == missing) || (want == nil) != (op == missing) {
			t.Fatalf("%v: got %v, want %v", op, entry, want)
		}
		if entry != nil && (entry.Amount() != want.Amount() ||
			entry.BlockHeight() != want.BlockHeight() ||
			entry.IsCoinBase() != want.IsCoinBase() ||
			!bytes.Equal(entry.PkScript(), want.PkScript())) {

			t.Fatalf("%v: got %+v, want %+v", op, entry, want)
		}
	}
}

// viewCountingDB is a database that counts its read transactions.
type viewCountingDB struct {
	database.DB
	views int
}

func (db *viewCountingDB) View(fn func(database.Tx) error) error {
	db.views++
	return db.DB.View(fn)
}

// TestUtxoCache_FetchUnspentOutputOfTx ensures FetchUnspentOutputOfTx finds an
// unspent output of a transaction at any index, in the cache or in the
// database, where the cache's state overrides the database's, that it reads
// the database once, and that it adds nothing to the cache.
func TestUtxoCache_FetchUnspentOutputOfTx(t *testing.T) {
	chain, _, tearDown := utxoCacheTestChain("TestUtxoCache_FetchUnspentOutputOfTx")
	defer tearDown()
	cache := chain.utxoCache
	db := &viewCountingDB{DB: cache.db}
	cache.db = db
	defer func() { cache.db = db.DB }()

	newEntry := func(height int32) *UtxoEntry {
		return NewUtxoEntry(wire.NewTxOut(1000, opTrueScript, wire.TokenData{}),
			height, false)
	}
	outpoint := func(hash byte, index uint32) wire.OutPoint {
		return wire.OutPoint{Hash: chainhash.Hash{hash}, Index: index}
	}
	spent := newEntry(20)
	spent.Spend()
	next := chainhash.Hash{4}
	next[chainhash.HashSize-1] = 1

	// 1: in the database only.  2: in the database, its first output
	// spent in the cache.  3: in the database and loaded into the cache
	// during the call.  4: none, with outputs of the next hash in the
	// database.  5 and 6: in the cache only, past the first chunk and at
	// the last index a transaction can have.  7: spent in the cache.
	stored := map[wire.OutPoint]*UtxoEntry{
		outpoint(1, 5): newEntry(10),
		outpoint(2, 0): newEntry(20),
		outpoint(2, 2): newEntry(21),
		outpoint(3, 1): newEntry(30),
		{Hash: next}:   newEntry(40),
	}
	err := cache.db.Update(func(dbTx database.Tx) error {
		return dbPutUtxoEntries(dbTx, stored)
	})
	if err != nil {
		t.Fatal(err)
	}
	cache.cachedEntries[outpoint(2, 0)] = spent
	cache.cachedEntries[outpoint(5, utxoScanChunk+44)] = newEntry(50)
	cache.cachedEntries[outpoint(6, maxTxOutputs-1)] = newEntry(60)
	cache.cachedEntries[outpoint(7, 0)] = nil
	numCached := len(cache.cachedEntries)

	for _, test := range []struct {
		hash   byte
		height int32
		load   bool
	}{
		{1, 10, false},
		{2, 21, false},
		{3, 30, true},
		{4, 0, false},
		{5, 50, false},
		{6, 60, false},
		{7, 0, false},
	} {
		if test.load {
			unspentOutputScanHook = func() {
				cache.cachedEntries[outpoint(3, 1)] = stored[outpoint(3, 1)].Clone()
				numCached++
			}
		}
		db.views = 0
		entry, err := chain.FetchUnspentOutputOfTx(&chainhash.Hash{test.hash})
		unspentOutputScanHook = nil
		if err != nil {
			t.Fatalf("%d: %v", test.hash, err)
		}
		if (entry == nil) != (test.height == 0) || entry != nil && entry.BlockHeight() != test.height {
			t.Fatalf("%d: got %+v, want height %d", test.hash, entry, test.height)
		}
		if db.views > 1 {
			t.Fatalf("%d: %d database reads", test.hash, db.views)
		}
		if len(cache.cachedEntries) != numCached {
			t.Fatalf("%d: %d cached entries, want %d", test.hash,
				len(cache.cachedEntries), numCached)
		}
	}
}
