// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// openTestChain opens the chain in the database at dir, creating the database
// when it doesn't exist yet.
func openTestChain(t *testing.T, dir string, params *chaincfg.Params) (*BlockChain, database.DB, error) {
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
		ChainParams:        params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(1000),
		UtxoCacheMaxSize:   250 * 1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
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
