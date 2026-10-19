// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// openTestChain opens the chain in the database at dir, creating the database
// when it doesn't exist yet.
func openTestChain(t *testing.T, dir string, params *chaincfg.Params) (*BlockChain, database.DB, error) {
	t.Helper()
	return openTestChainConfig(t, dir, params, func(*Config) {})
}

// openTestChainConfig opens the chain like openTestChain with the chain
// configuration changed by modify.
func openTestChainConfig(t *testing.T, dir string, params *chaincfg.Params, modify func(*Config)) (*BlockChain, database.DB, error) {
	t.Helper()
	db, err := database.Open(testDbType, dir, params.Net)
	if err != nil {
		db, err = database.Create(testDbType, dir, params.Net)
		if err != nil {
			t.Fatalf("failed to create database: %v", err)
		}
	}
	config := &Config{
		DB:                 db,
		ChainParams:        params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(1000),
		UtxoCacheMaxSize:   250 * 1024 * 1024,
		ExcessiveBlockSize: 32000000,
	}
	modify(config)
	chain, err := New(config)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return chain, db, nil
}

// reorgTest is a chain with a branch of two blocks which was replaced by a
// branch of three blocks from genesis.  The second block of each branch spends
// the coinbase of the first.
type reorgTest struct {
	chain    *BlockChain
	db       database.DB
	replaced []*bchutil.Block
	branch   []*bchutil.Block
	unspent  []wire.OutPoint
	spent    []wire.OutPoint
}

// newReorgTest creates the reorg test chain at dir.  The utxo cache is
// flushed right before the reorg.
func newReorgTest(t *testing.T, dir string, params *chaincfg.Params) *reorgTest {
	t.Helper()
	chain, db, err := openTestChain(t, dir, params)
	if err != nil {
		t.Fatalf("failed to open chain: %v", err)
	}
	chain.TstSetCoinbaseMaturity(1)
	m := &ruleTestMiner{t: t, chain: chain, tip: params.GenesisBlock}
	mine := func(numBlocks int, tag byte) ([]*bchutil.Block, *wire.MsgTx) {
		var blocks []*bchutil.Block
		var spend *wire.MsgTx
		for range numBlocks {
			cb := m.coinbase(m.height + 1)
			cb.TxOut[1] = ruleTestPadding(tag)
			var txs []*wire.MsgTx
			if len(blocks) == 1 {
				prevCb := blocks[0].MsgBlock().Transactions[0]
				spend = ruleTestSpend(wire.OutPoint{Hash: prevCb.TxHash()},
					prevCb.TxOut[0].Value-1000, tag)
				txs = append(txs, spend)
			}
			block := m.build(cb, txs...)
			m.mustAccept(block)
			blocks = append(blocks, block)
		}
		return blocks, spend
	}
	coinbaseOut := func(block *bchutil.Block) wire.OutPoint {
		return wire.OutPoint{Hash: *block.Transactions()[0].Hash()}
	}

	r := &reorgTest{chain: chain, db: db}
	var replacedSpend, branchSpend *wire.MsgTx
	r.replaced, replacedSpend = mine(2, 1)
	m.tip, m.height = params.GenesisBlock, 0
	r.branch, branchSpend = mine(3, 2)
	if chain.BestSnapshot().Hash != *r.branch[2].Hash() {
		t.Fatal("chain did not reorganize")
	}

	r.unspent = []wire.OutPoint{coinbaseOut(r.branch[1]),
		coinbaseOut(r.branch[2]), {Hash: branchSpend.TxHash()}}
	r.spent = []wire.OutPoint{coinbaseOut(r.branch[0]),
		coinbaseOut(r.replaced[0]), coinbaseOut(r.replaced[1]),
		{Hash: replacedSpend.TxHash()}}
	return r
}

// requireUtxoSet fails the test unless the outputs of the test are spent or
// unspent in the chain as expected.
func (r *reorgTest) requireUtxoSet(t *testing.T, chain *BlockChain) {
	t.Helper()
	for i, outpoints := range [][]wire.OutPoint{r.spent, r.unspent} {
		for _, outpoint := range outpoints {
			entry, err := chain.FetchUtxoEntry(outpoint)
			if err != nil {
				t.Fatalf("FetchUtxoEntry: %v", err)
			}
			isUnspent := entry != nil && !entry.IsSpent()
			if isUnspent != (i == 1) {
				t.Fatalf("output %v unspent %v, want %v", outpoint,
					isUnspent, i == 1)
			}
		}
	}
}

// hasSpendJournal returns whether the spend journal holds an entry for the
// block.
func hasSpendJournal(t *testing.T, db database.DB, block *bchutil.Block) bool {
	t.Helper()
	var found bool
	err := db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(spendJournalBucketName)
		found = bucket.Get(block.Hash()[:]) != nil
		return nil
	})
	if err != nil {
		t.Fatalf("failed to read the spend journal: %v", err)
	}
	return found
}

// TestUtxoRecoveryAfterReorg ensures the utxo set is recovered on startup
// when its last flush names a block that a reorg removed from the best
// chain, whether the node stopped before the next flush, during it, or right
// before it finished.
func TestUtxoRecoveryAfterReorg(t *testing.T) {
	tests := []struct {
		name string
		stop func(t *testing.T, chain *BlockChain, db database.DB)
	}{
		{
			name: "before the next flush",
			stop: func(*testing.T, *BlockChain, database.DB) {},
		},
		{
			name: "when the next flush starts",
			stop: func(t *testing.T, chain *BlockChain, db database.DB) {
				err := db.Update(func(dbTx database.Tx) error {
					return dbPutUtxoStateConsistency(dbTx,
						ucsFlushOngoing, &chain.utxoCache.lastFlushHash)
				})
				if err != nil {
					t.Fatalf("failed to write the flush status: %v", err)
				}
			},
		},
		{
			name: "before the next flush finishes",
			stop: func(t *testing.T, chain *BlockChain, db database.DB) {
				// Only the final status update fails.
				chain.utxoCache.db = &failingCommitDB{DB: db, failOn: 2}
				if err := chain.FlushCachedState(FlushRequired); err == nil {
					t.Fatal("flush succeeded with a failing database")
				}
			},
		},
	}

	params := chaincfg.RegressionNetParams
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "db")
			r := newReorgTest(t, dir, &params)
			test.stop(t, r.chain, r.db)
			r.db.Close()

			for range 2 {
				chain, db, err := openTestChain(t, dir, &params)
				if err != nil {
					t.Fatalf("failed to restart: %v", err)
				}
				if chain.BestSnapshot().Hash != *r.branch[2].Hash() {
					t.Fatal("unexpected tip after restart")
				}
				r.requireUtxoSet(t, chain)
				if hasSpendJournal(t, db, r.replaced[1]) {
					t.Fatal("spend journal entry of a replaced block kept")
				}
				db.Close()
			}
		})
	}
}

// TestUtxoRecoveryFlushOngoingAtTip ensures startup recovers when the last
// flush was interrupted while the utxo set was consistent with the tip.
func TestUtxoRecoveryFlushOngoingAtTip(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	if err := r.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("flush: %v", err)
	}
	err := r.db.Update(func(dbTx database.Tx) error {
		return dbPutUtxoStateConsistency(dbTx, ucsFlushOngoing,
			r.branch[2].Hash())
	})
	if err != nil {
		t.Fatalf("failed to write the flush status: %v", err)
	}
	r.db.Close()

	chain, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to restart: %v", err)
	}
	defer db.Close()
	r.requireUtxoSet(t, chain)
}

// TestStaleSpendJournals ensures the spend journal entries of blocks removed
// from the best chain are kept until the next completed flush, and that the
// entries of blocks added back to it are kept after that.
func TestStaleSpendJournals(t *testing.T) {
	params := chaincfg.RegressionNetParams
	r := newReorgTest(t, filepath.Join(t.TempDir(), "db"), &params)
	defer r.db.Close()

	if !hasSpendJournal(t, r.db, r.replaced[1]) {
		t.Fatal("spend journal entry of a replaced block removed before a flush")
	}
	if err := r.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if hasSpendJournal(t, r.db, r.replaced[1]) {
		t.Fatal("spend journal entry of a replaced block kept after a flush")
	}
	for _, block := range r.branch {
		if !hasSpendJournal(t, r.db, block) {
			t.Fatalf("spend journal entry of block %d removed", block.Height())
		}
	}

	// Remove the last block and add it back.
	tip := r.chain.bestChain.Tip()
	if err := r.chain.InvalidateBlock(&tip.hash); err != nil {
		t.Fatalf("InvalidateBlock: %v", err)
	}
	if err := r.chain.ReconsiderBlock(&tip.hash); err != nil {
		t.Fatalf("ReconsiderBlock: %v", err)
	}
	if r.chain.BestSnapshot().Hash != tip.hash {
		t.Fatal("block not added back")
	}
	if err := r.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if !hasSpendJournal(t, r.db, r.branch[2]) {
		t.Fatal("spend journal entry of a block added back removed")
	}
}

// TestUtxoRecoveryAfterReorgSharedSpend ensures recovery after a reorg
// leaves an output spent when both the removed and the new branch spend it.
func TestUtxoRecoveryAfterReorgSharedSpend(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	chain, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to open chain: %v", err)
	}
	chain.TstSetCoinbaseMaturity(1)
	m := &ruleTestMiner{t: t, chain: chain, tip: params.GenesisBlock}

	// Both branches spend the coinbase of the common block.
	common := m.build(nil)
	m.mustAccept(common)
	shared := wire.OutPoint{Hash: *common.Transactions()[0].Hash()}
	value := common.MsgBlock().Transactions[0].TxOut[0].Value
	mine := func(tag byte, n int) (*wire.MsgTx, *bchutil.Block) {
		m.tip, m.height = common.MsgBlock(), 1
		spend := ruleTestSpend(shared, value-1000, tag)
		var block *bchutil.Block
		for i := range n {
			cb := m.coinbase(m.height + 1)
			cb.TxOut[1] = ruleTestPadding(tag)
			if i == 0 {
				block = m.build(cb, spend)
			} else {
				block = m.build(cb)
			}
			m.mustAccept(block)
		}
		return spend, block
	}
	oldSpend, _ := mine(1, 1)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("flush: %v", err)
	}
	newSpend, tip := mine(2, 2)
	if chain.BestSnapshot().Hash != *tip.Hash() {
		t.Fatal("chain did not reorganize")
	}
	db.Close()

	chain, db, err = openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to restart: %v", err)
	}
	defer db.Close()
	for _, test := range []struct {
		outpoint wire.OutPoint
		unspent  bool
	}{
		{shared, false},
		{wire.OutPoint{Hash: oldSpend.TxHash()}, false},
		{wire.OutPoint{Hash: newSpend.TxHash()}, true},
	} {
		entry, err := chain.FetchUtxoEntry(test.outpoint)
		if err != nil {
			t.Fatalf("FetchUtxoEntry: %v", err)
		}
		if unspent := entry != nil && !entry.IsSpent(); unspent != test.unspent {
			t.Fatalf("output %v unspent %v, want %v", test.outpoint,
				unspent, test.unspent)
		}
	}
}

// TestUtxoRecoveryAfterReorgPruned ensures a pruned node flushes the utxo
// set before pruning the spend journals that recovering from the last flush,
// on a branch a reorg removed, would need.
func TestUtxoRecoveryAfterReorgPruned(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	open := func() (*BlockChain, database.DB) {
		t.Helper()
		db, err := database.Open(testDbType, dir, params.Net)
		if err != nil {
			db, err = database.Create(testDbType, dir, params.Net)
			if err != nil {
				t.Fatalf("failed to create database: %v", err)
			}
		}
		chain, err := New(&Config{
			DB:                 db,
			ChainParams:        &params,
			TimeSource:         NewMedianTime(),
			SigCache:           txscript.NewSigCache(1000),
			UtxoCacheMaxSize:   250 * 1024 * 1024,
			ExcessiveBlockSize: 32000000,
			Prune:              true,
			PruneDepth:         2,
		})
		if err != nil {
			db.Close()
			t.Fatalf("failed to open chain: %v", err)
		}
		return chain, db
	}
	chain, db := open()
	chain.TstSetCoinbaseMaturity(1)
	m := &ruleTestMiner{t: t, chain: chain, tip: params.GenesisBlock}

	// The new branch spends the coinbase of the common block in its first
	// block, so rolling it back needs that block's spend journal.
	common := m.build(nil)
	m.mustAccept(common)
	shared := wire.OutPoint{Hash: *common.Transactions()[0].Hash()}
	value := common.MsgBlock().Transactions[0].TxOut[0].Value
	mine := func(tag byte, n int, txs ...*wire.MsgTx) {
		m.tip, m.height = common.MsgBlock(), 1
		for i := range n {
			cb := m.coinbase(m.height + 1)
			cb.TxOut[1] = ruleTestPadding(tag)
			if i == 0 {
				m.mustAccept(m.build(cb, txs...))
			} else {
				m.mustAccept(m.build(cb))
			}
		}
	}
	mine(1, 3)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("flush: %v", err)
	}
	newSpend := ruleTestSpend(shared, value-1000, 2)
	mine(2, 4, newSpend)
	if chain.BestSnapshot().Height != 5 {
		t.Fatal("chain did not reorganize")
	}
	db.Close()

	chain, db = open()
	defer db.Close()
	for _, test := range []struct {
		outpoint wire.OutPoint
		unspent  bool
	}{
		{shared, false},
		{wire.OutPoint{Hash: newSpend.TxHash()}, true},
	} {
		entry, err := chain.FetchUtxoEntry(test.outpoint)
		if err != nil {
			t.Fatalf("FetchUtxoEntry: %v", err)
		}
		if unspent := entry != nil && !entry.IsSpent(); unspent != test.unspent {
			t.Fatalf("output %v unspent %v, want %v", test.outpoint,
				unspent, test.unspent)
		}
	}
}

// requireConsistentAtTip fails the test unless the utxo state is marked as
// consistent with the tip of the chain.
func requireConsistentAtTip(t *testing.T, chain *BlockChain) {
	t.Helper()
	code, hash := fetchUtxoStatus(t, chain.db)
	if code != ucsConsistent || *hash != chain.BestSnapshot().Hash {
		t.Fatalf("utxo state status %d at %v, want consistent at the tip",
			code, hash)
	}
}

// fetchUtxoStatus returns the stored consistency status of the utxo state.
func fetchUtxoStatus(t *testing.T, db database.DB) (byte, *chainhash.Hash) {
	t.Helper()
	var code byte
	var hash *chainhash.Hash
	err := db.View(func(dbTx database.Tx) error {
		var err error
		code, hash, err = dbFetchUtxoStateConsistency(dbTx)
		return err
	})
	if err != nil {
		t.Fatalf("failed to read the utxo state status: %v", err)
	}
	return code, hash
}

// requireReIndexMarker fails the test unless the re-index of the chain state
// is marked as unfinished exactly when want is set.
func requireReIndexMarker(t *testing.T, db database.DB, want bool) {
	t.Helper()
	var found bool
	err := db.View(func(dbTx database.Tx) error {
		found = dbTx.Metadata().Get(chainStateReIndexKeyName) != nil
		return nil
	})
	if err != nil {
		t.Fatalf("failed to read the re-index marker: %v", err)
	}
	if found != want {
		t.Fatalf("re-index marked as unfinished %v, want %v", found, want)
	}
}

// removeSpendJournals removes the spend journal entries of the blocks, which
// rolling back the utxo state needs.
func removeSpendJournals(t *testing.T, db database.DB, blocks []*bchutil.Block) {
	t.Helper()
	err := db.Update(func(dbTx database.Tx) error {
		for _, block := range blocks {
			if err := dbRemoveSpendJournalEntry(dbTx, block.Hash()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to remove spend journal entries: %v", err)
	}
}

// TestReIndexChainStateSkipsConsistencyCheck ensures re-indexing the chain
// state rebuilds a utxo state whose status can't be recovered from.
func TestReIndexChainStateSkipsConsistencyCheck(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	err := r.db.Update(func(dbTx database.Tx) error {
		return dbPutUtxoStateConsistency(dbTx, ucsFlushOngoing,
			&chainhash.Hash{0x01})
	})
	if err != nil {
		t.Fatalf("failed to write the flush status: %v", err)
	}
	r.db.Close()

	if _, _, err := openTestChain(t, dir, &params); err == nil {
		t.Fatal("chain opened with an unknown utxo state status")
	}
	chain, db, err := openTestChainConfig(t, dir, &params, func(config *Config) {
		config.ReIndexChainState = true
	})
	if err != nil {
		t.Fatalf("failed to re-index: %v", err)
	}
	defer db.Close()
	r.requireUtxoSet(t, chain)
	requireConsistentAtTip(t, chain)
	requireReIndexMarker(t, db, false)
}

// TestReIndexChainStateRestartsBeforeFirstFlush ensures a re-index stopped
// before its first flush completed is started over on the next start instead
// of trusting the incomplete utxo set or rolling the whole chain back.
func TestReIndexChainStateRestartsBeforeFirstFlush(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)

	// Fail the final flush of the re-index.
	r.chain.utxoCache.db = &failingCommitDB{DB: r.db, failOn: 1}
	if err := r.chain.ReIndexChainState(); err == nil {
		t.Fatal("re-index succeeded with a failing database")
	}
	requireReIndexMarker(t, r.db, true)

	// Rolling back from the tip would need these.
	removeSpendJournals(t, r.db, r.branch)
	r.db.Close()

	chain, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to restart: %v", err)
	}
	defer db.Close()
	r.requireUtxoSet(t, chain)
	requireConsistentAtTip(t, chain)
	requireReIndexMarker(t, db, false)
}

// TestReIndexChainStateResume ensures an interrupted re-index flushes the
// utxo set and is resumed from there on the next start.
func TestReIndexChainStateResume(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	r.db.Close()

	interrupt := make(chan struct{})
	close(interrupt)
	_, _, err := openTestChainConfig(t, dir, &params, func(config *Config) {
		config.ReIndexChainState = true
		config.Interrupt = interrupt
	})
	if err != errInterruptRequested {
		t.Fatalf("interrupted re-index returned %v", err)
	}

	// The re-index stopped after the first block.  An output added to
	// the utxo set now is kept only when the re-index is resumed rather
	// than started over.
	db, err := database.Open(testDbType, dir, params.Net)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	requireReIndexMarker(t, db, true)
	var code byte
	var hash *chainhash.Hash
	sentinel := wire.OutPoint{Hash: chainhash.Hash{0x01}}
	err = db.Update(func(dbTx database.Tx) error {
		var err error
		code, hash, err = dbFetchUtxoStateConsistency(dbTx)
		if err != nil {
			return err
		}
		entry := NewUtxoEntry(wire.NewTxOut(1, []byte{0x51},
			wire.TokenData{}), 1, false)
		return dbPutUtxoEntries(dbTx, map[wire.OutPoint]*UtxoEntry{
			sentinel: entry})
	})
	if err != nil {
		t.Fatalf("failed to update the database: %v", err)
	}
	if code != ucsConsistent || *hash != *r.branch[0].Hash() {
		t.Fatalf("utxo state status %d at %v after the interrupt, want "+
			"consistent at the first block", code, hash)
	}
	removeSpendJournals(t, db, r.branch)
	db.Close()

	chain, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}
	defer db.Close()
	r.requireUtxoSet(t, chain)
	requireConsistentAtTip(t, chain)
	requireReIndexMarker(t, db, false)
	entry, err := chain.FetchUtxoEntry(sentinel)
	if err != nil || entry == nil {
		t.Fatalf("re-index started over: %v", err)
	}
}

// TestReIndexChainStateFinishedBeforeMarkerDelete ensures a re-index that
// stopped after its final flush but before it was marked as finished is
// finished on the next start without the spend journals.
func TestReIndexChainStateFinishedBeforeMarkerDelete(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	if err := r.chain.ReIndexChainState(); err != nil {
		t.Fatalf("failed to re-index: %v", err)
	}
	err := r.db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().Put(chainStateReIndexKeyName, []byte{1})
	})
	if err != nil {
		t.Fatalf("failed to write the re-index marker: %v", err)
	}
	removeSpendJournals(t, r.db, r.branch)
	r.db.Close()

	chain, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to restart: %v", err)
	}
	defer db.Close()
	r.requireUtxoSet(t, chain)
	requireConsistentAtTip(t, chain)
	requireReIndexMarker(t, db, false)
}

// TestReIndexChainStateInterruptedFlush ensures a re-index stopped during a
// flush after an earlier completed one is started over, dropping the entries
// the interrupted flush already wrote.
func TestReIndexChainStateInterruptedFlush(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	r.db.Close()

	interrupt := make(chan struct{})
	close(interrupt)
	_, _, err := openTestChainConfig(t, dir, &params, func(config *Config) {
		config.ReIndexChainState = true
		config.Interrupt = interrupt
	})
	if err != errInterruptRequested {
		t.Fatalf("interrupted re-index returned %v", err)
	}

	// Leave the next flush half done, with an entry of a later block
	// written.
	db, err := database.Open(testDbType, dir, params.Net)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	stray := wire.OutPoint{Hash: chainhash.Hash{0x02}}
	err = db.Update(func(dbTx database.Tx) error {
		entry := NewUtxoEntry(wire.NewTxOut(1, []byte{0x51},
			wire.TokenData{}), 2, false)
		err := dbPutUtxoEntries(dbTx, map[wire.OutPoint]*UtxoEntry{
			stray: entry})
		if err != nil {
			return err
		}
		return dbPutUtxoStateConsistency(dbTx, ucsFlushOngoing,
			r.branch[0].Hash())
	})
	if err != nil {
		t.Fatalf("failed to update the database: %v", err)
	}
	removeSpendJournals(t, db, r.branch)
	db.Close()

	chain, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to restart: %v", err)
	}
	defer db.Close()
	r.requireUtxoSet(t, chain)
	requireConsistentAtTip(t, chain)
	requireReIndexMarker(t, db, false)
	entry, err := chain.FetchUtxoEntry(stray)
	if err != nil || entry != nil {
		t.Fatalf("entry of the interrupted flush kept: %v %v", entry, err)
	}
}

// TestReIndexChainStateGenesisOnly ensures re-indexing a chain of only the
// genesis block leaves the utxo state consistent at it.
func TestReIndexChainStateGenesisOnly(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	_, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to open chain: %v", err)
	}
	db.Close()

	chain, db, err := openTestChainConfig(t, dir, &params, func(config *Config) {
		config.ReIndexChainState = true
	})
	if err != nil {
		t.Fatalf("failed to re-index: %v", err)
	}
	defer db.Close()
	requireConsistentAtTip(t, chain)
	requireReIndexMarker(t, db, false)
}

// TestReIndexChainStateNeedsAllBlocks ensures a re-index is refused before
// it replaces the utxo set when blocks it needs are or would be removed by
// pruning or fast sync.
func TestReIndexChainStateNeedsAllBlocks(t *testing.T) {
	params := chaincfg.RegressionNetParams
	tests := []struct {
		name       string
		wasPruned  bool
		unfinished bool
		modify     func(*Config)
	}{{
		name:      "pruned or fast synced database",
		wasPruned: true,
		modify: func(config *Config) {
			config.ReIndexChainState = true
		},
	}, {
		name:       "unfinished in a pruned or fast synced database",
		wasPruned:  true,
		unfinished: true,
		modify:     func(*Config) {},
	}, {
		name:       "unfinished with pruning",
		unfinished: true,
		modify: func(config *Config) {
			config.Prune = true
		},
	}, {
		name:       "unfinished with fast sync",
		unfinished: true,
		modify: func(config *Config) {
			config.FastSync = true
			config.Checkpoints = []chaincfg.Checkpoint{{
				Height: 100, Hash: &chainhash.Hash{0x01}}}
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "db")
			r := newReorgTest(t, dir, &params)
			err := r.db.Update(func(dbTx database.Tx) error {
				if test.wasPruned {
					err := dbPutBlockchainType(dbTx,
						prunedBlockchainEntryValue)
					if err != nil {
						return err
					}
				}
				if test.unfinished {
					return dbTx.Metadata().Put(
						chainStateReIndexKeyName, []byte{1})
				}
				return nil
			})
			if err != nil {
				t.Fatalf("failed to update the database: %v", err)
			}
			code, hash := fetchUtxoStatus(t, r.db)
			r.db.Close()

			_, _, err = openTestChainConfig(t, dir, &params, test.modify)
			if err == nil {
				t.Fatal("re-index started without all blocks")
			}

			// The utxo set is left as it was.
			db, err := database.Open(testDbType, dir, params.Net)
			if err != nil {
				t.Fatalf("failed to open database: %v", err)
			}
			defer db.Close()
			requireReIndexMarker(t, db, test.unfinished)
			gotCode, gotHash := fetchUtxoStatus(t, db)
			if gotCode != code || *gotHash != *hash {
				t.Fatalf("utxo state status %d at %v, want %d at %v",
					gotCode, gotHash, code, hash)
			}
		})
	}
}
