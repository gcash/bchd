// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// scriptLeavesTestBlock returns a block whose header nonce is part of its
// hash, so two nonces are two blocks.
func scriptLeavesTestBlock(t *testing.T, nonce uint32) *bchutil.Block {
	t.Helper()
	var prev chainhash.Hash
	msg := wire.NewMsgBlock(wire.NewBlockHeader(1, &prev, &prev, 0, nonce))
	tx := wire.NewMsgTx(1)
	outpoint := chainhash.Hash{byte(nonce)}
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&outpoint, nonce), nil))
	tx.AddTxOut(wire.NewTxOut(1, []byte{0x51}, wire.TokenData{}))
	if err := msg.AddTransaction(tx); err != nil {
		t.Fatalf("add transaction: %v", err)
	}
	return bchutil.NewBlock(msg)
}

// stopScriptLeavesCache stops the keep timer so it does not fire after the
// test has finished with the cache.
func stopScriptLeavesCache(c *scriptLeavesCache) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.timer != nil {
		c.timer.Stop()
	}
}

// TestScriptLeavesCacheBuildsOnce checks that callers asking for one block
// while it is built share that build and the same entry, and that a later
// ask for the block does not build it again.
func TestScriptLeavesCacheBuildsOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		block := scriptLeavesTestBlock(t, 1)
		hash := block.Hash()
		cache := newScriptLeavesCache(time.Hour)
		defer stopScriptLeavesCache(cache)

		release := make(chan struct{})
		var builds atomic.Int32
		const n = 8
		got := make([]*scriptLeavesOfBlock, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		wg.Add(n)
		for i := range n {
			go func() {
				defer wg.Done()
				got[i], errs[i] = cache.get(hash, func() (*scriptLeavesOfBlock, error) {
					builds.Add(1)
					<-release
					return newScriptLeavesOfBlock(block, nil, nil), nil
				})
			}()
		}
		// Wait until every caller is blocked in get: one inside build, the
		// rest on that build's done channel.
		synctest.Wait()
		if gotN := builds.Load(); gotN != 1 {
			close(release)
			wg.Wait()
			t.Fatalf("build ran %d times while callers were waiting, want 1", gotN)
		}
		close(release)
		wg.Wait()
		for i := range n {
			if errs[i] != nil {
				t.Fatalf("caller %d: %v", i, errs[i])
			}
			if got[i] != got[0] {
				t.Fatalf("caller %d got a different entry", i)
			}
		}

		var again atomic.Int32
		entry, err := cache.get(hash, func() (*scriptLeavesOfBlock, error) {
			again.Add(1)
			return nil, errors.New("built again")
		})
		if again.Load() != 0 {
			t.Fatal("get built the block that was already kept")
		}
		if err != nil {
			t.Fatalf("get of the kept block: %v", err)
		}
		if entry != got[0] {
			t.Fatal("get of the kept block returned a different entry")
		}
	})
}

// TestScriptLeavesCacheOtherBlock checks that a build for one block does not
// delay an ask for another, including when that other block is the one kept,
// and that the build which finishes last is the one kept.
func TestScriptLeavesCacheOtherBlock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		blockA := scriptLeavesTestBlock(t, 1)
		blockB := scriptLeavesTestBlock(t, 2)
		hashA := blockA.Hash()
		hashB := blockB.Hash()
		if hashA.IsEqual(hashB) {
			t.Fatal("the two test blocks hashed the same")
		}
		cache := newScriptLeavesCache(time.Hour)
		defer stopScriptLeavesCache(cache)

		release := make(chan struct{})
		var buildsA atomic.Int32
		errA := make(chan error, 1)
		gotA := make(chan *scriptLeavesOfBlock, 1)
		go func() {
			entry, err := cache.get(hashA, func() (*scriptLeavesOfBlock, error) {
				buildsA.Add(1)
				<-release
				return newScriptLeavesOfBlock(blockA, nil, nil), nil
			})
			errA <- err
			gotA <- entry
		}()
		synctest.Wait()
		if gotN := buildsA.Load(); gotN != 1 {
			close(release)
			t.Fatalf("build of A ran %d times, want 1", gotN)
		}

		var buildsB atomic.Int32
		entryB, err := cache.get(hashB, func() (*scriptLeavesOfBlock, error) {
			buildsB.Add(1)
			return newScriptLeavesOfBlock(blockB, nil, nil), nil
		})
		if err != nil {
			close(release)
			t.Fatalf("get B while A was building: %v", err)
		}
		if gotN := buildsB.Load(); gotN != 1 {
			close(release)
			t.Fatalf("build of B ran %d times, want 1", gotN)
		}
		var extraB atomic.Int32
		keptB, err := cache.get(hashB, func() (*scriptLeavesOfBlock, error) {
			extraB.Add(1)
			return nil, errors.New("rebuilt B")
		})
		if extraB.Load() != 0 {
			close(release)
			t.Fatal("get of kept block B built again while A was still building")
		}
		if err != nil {
			close(release)
			t.Fatalf("second get B: %v", err)
		}
		if keptB != entryB {
			close(release)
			t.Fatal("second get B returned a different entry")
		}

		close(release)
		if err := <-errA; err != nil {
			t.Fatalf("get A: %v", err)
		}
		entryA := <-gotA
		var extraA atomic.Int32
		keptA, err := cache.get(hashA, func() (*scriptLeavesOfBlock, error) {
			extraA.Add(1)
			return nil, errors.New("rebuilt A")
		})
		if extraA.Load() != 0 {
			t.Fatal("get A built again after A finished last")
		}
		if err != nil {
			t.Fatalf("get A after it finished: %v", err)
		}
		if keptA != entryA {
			t.Fatal("the entry kept after A finished was not A")
		}
		var rebuildB atomic.Int32
		if _, err := cache.get(hashB, func() (*scriptLeavesOfBlock, error) {
			rebuildB.Add(1)
			return newScriptLeavesOfBlock(blockB, nil, nil), nil
		}); err != nil {
			t.Fatalf("get B after A finished: %v", err)
		}
		if gotN := rebuildB.Load(); gotN != 1 {
			t.Fatalf("get B after A finished built %d times, want 1", gotN)
		}
	})
}

// TestScriptLeavesCacheError checks that a failed build is returned to every
// waiter, is not kept, and is tried again on the next ask.
func TestScriptLeavesCacheError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		block := scriptLeavesTestBlock(t, 3)
		hash := block.Hash()
		cache := newScriptLeavesCache(time.Hour)
		defer stopScriptLeavesCache(cache)

		release := make(chan struct{})
		wantErr := errors.New("script leaves build failed")
		var builds atomic.Int32
		const n = 8
		got := make([]*scriptLeavesOfBlock, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		wg.Add(n)
		for i := range n {
			go func() {
				defer wg.Done()
				got[i], errs[i] = cache.get(hash, func() (*scriptLeavesOfBlock, error) {
					builds.Add(1)
					<-release
					return nil, wantErr
				})
			}()
		}
		synctest.Wait()
		if gotN := builds.Load(); gotN != 1 {
			close(release)
			wg.Wait()
			t.Fatalf("build ran %d times while callers were waiting, want 1", gotN)
		}
		close(release)
		wg.Wait()
		for i := range n {
			if errs[i] != wantErr {
				t.Fatalf("caller %d error = %v, want %v", i, errs[i], wantErr)
			}
			if got[i] != nil {
				t.Fatalf("caller %d got an entry from a failed build", i)
			}
		}
		cache.mtx.Lock()
		kept := cache.kept
		cache.mtx.Unlock()
		if kept != nil {
			t.Fatal("a failed build was kept")
		}

		var again atomic.Int32
		if _, err := cache.get(hash, func() (*scriptLeavesOfBlock, error) {
			again.Add(1)
			return newScriptLeavesOfBlock(block, nil, nil), nil
		}); err != nil {
			t.Fatalf("get after the error: %v", err)
		}
		if gotN := again.Load(); gotN != 1 {
			t.Fatalf("get after the error built %d times, want 1", gotN)
		}
	})
}

// TestScriptLeavesCacheDrops checks that a kept block is served again without
// a build and is gone once its keep timer has fired, and that keeping a
// second block replaces the timer started for the first.
func TestScriptLeavesCacheDrops(t *testing.T) {
	cache := newScriptLeavesCache(20 * time.Millisecond)
	defer func() { stopScriptLeavesCache(cache) }()
	blockA := scriptLeavesTestBlock(t, 1)
	hashA := blockA.Hash()
	var builds atomic.Int32
	if _, err := cache.get(hashA, func() (*scriptLeavesOfBlock, error) {
		builds.Add(1)
		return newScriptLeavesOfBlock(blockA, nil, nil), nil
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, err := cache.get(hashA, func() (*scriptLeavesOfBlock, error) {
		builds.Add(1)
		return nil, errors.New("rebuilt")
	}); err != nil {
		t.Fatalf("second get: %v", err)
	}
	if gotN := builds.Load(); gotN != 1 {
		t.Fatalf("build ran %d times, want 1", gotN)
	}
	cache.mtx.Lock()
	if cache.kept == nil {
		cache.mtx.Unlock()
		t.Fatal("block was not kept after get")
	}
	cache.mtx.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	for {
		cache.mtx.Lock()
		gone := cache.kept == nil
		cache.mtx.Unlock()
		if gone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("kept block was still present 5s after the keep timer was started")
		}
	}

	cache = newScriptLeavesCache(20 * time.Millisecond)
	blockA = scriptLeavesTestBlock(t, 4)
	hashA = blockA.Hash()
	if _, err := cache.get(hashA, func() (*scriptLeavesOfBlock, error) {
		return newScriptLeavesOfBlock(blockA, nil, nil), nil
	}); err != nil {
		t.Fatalf("get A: %v", err)
	}
	cache.mtx.Lock()
	timerA := cache.timer
	cache.mtx.Unlock()
	if timerA == nil {
		t.Fatal("keeping block A did not start a timer")
	}
	blockB := scriptLeavesTestBlock(t, 5)
	hashB := blockB.Hash()
	if _, err := cache.get(hashB, func() (*scriptLeavesOfBlock, error) {
		return newScriptLeavesOfBlock(blockB, nil, nil), nil
	}); err != nil {
		t.Fatalf("get B: %v", err)
	}
	cache.mtx.Lock()
	timerB := cache.timer
	cache.mtx.Unlock()
	if timerB == nil {
		t.Fatal("keeping block B did not start a timer")
	}
	if timerA == timerB {
		t.Fatal("keeping block B left the timer started for block A")
	}
}

// TestNewScriptLeavesOfBlock checks that the block hash and every transaction
// hash are already saved, so several goroutines can read them together.
func TestNewScriptLeavesOfBlock(t *testing.T) {
	const n = 3
	var prev chainhash.Hash
	msg := wire.NewMsgBlock(wire.NewBlockHeader(1, &prev, &prev, 0, 9))
	for i := range n {
		tx := wire.NewMsgTx(1)
		outpoint := chainhash.Hash{byte(i + 1)}
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&outpoint, uint32(i)), []byte{byte(i)}))
		tx.AddTxOut(wire.NewTxOut(int64(i+1), []byte{0x51, byte(i)}, wire.TokenData{}))
		if err := msg.AddTransaction(tx); err != nil {
			t.Fatalf("add transaction: %v", err)
		}
	}
	var buf bytes.Buffer
	if err := msg.Serialize(&buf); err != nil {
		t.Fatalf("serialize block: %v", err)
	}
	block, err := bchutil.NewBlockFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("read block: %v", err)
	}
	wantBlock := block.MsgBlock().BlockHash()
	wireTxs := block.MsgBlock().Transactions
	if len(wireTxs) < 3 {
		t.Fatalf("block has %d transactions, want at least 3", len(wireTxs))
	}
	wantTx := make([]chainhash.Hash, len(wireTxs))
	for i, tx := range wireTxs {
		wantTx[i] = tx.TxHash()
	}
	entry := newScriptLeavesOfBlock(block, nil, nil)

	const readers = 4
	errc := make(chan error, readers)
	for range readers {
		go func() {
			errc <- checkScriptLeavesBlock(entry, wantBlock, wantTx)
		}()
	}
	for range readers {
		if err := <-errc; err != nil {
			t.Error(err)
		}
	}
}

// checkScriptLeavesBlock compares the saved block and transaction hashes with
// hashes taken from the wire block.
func checkScriptLeavesBlock(entry *scriptLeavesOfBlock, wantBlock chainhash.Hash, wantTx []chainhash.Hash) error {
	if got := entry.block.Hash(); *got != wantBlock {
		return fmt.Errorf("block hash %v, want %v", got, wantBlock)
	}
	txs := entry.block.Transactions()
	if len(txs) != len(wantTx) {
		return fmt.Errorf("got %d transactions, want %d", len(txs), len(wantTx))
	}
	for i, tx := range txs {
		if got := tx.Hash(); *got != wantTx[i] {
			return fmt.Errorf("transaction %d hash %v, want %v", i, got, wantTx[i])
		}
	}
	return nil
}

// TestScriptLeavesCacheErrorKeepsOther checks that a failed build for one
// block leaves the block that was kept before in place.
func TestScriptLeavesCacheErrorKeepsOther(t *testing.T) {
	keptBlock := scriptLeavesTestBlock(t, 11)
	failing := scriptLeavesTestBlock(t, 12)
	cache := newScriptLeavesCache(time.Hour)
	defer stopScriptLeavesCache(cache)

	kept, err := cache.get(keptBlock.Hash(), func() (*scriptLeavesOfBlock, error) {
		return newScriptLeavesOfBlock(keptBlock, nil, nil), nil
	})
	if err != nil {
		t.Fatalf("build of the first block: %v", err)
	}
	wantErr := errors.New("script leaves build failed")
	if _, err := cache.get(failing.Hash(), func() (*scriptLeavesOfBlock, error) {
		return nil, wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("failed build returned %v, want %v", err, wantErr)
	}
	again, err := cache.get(keptBlock.Hash(), func() (*scriptLeavesOfBlock, error) {
		t.Error("the kept block was built again after another block failed")
		return newScriptLeavesOfBlock(keptBlock, nil, nil), nil
	})
	if err != nil || again != kept {
		t.Fatalf("kept block after a failed build: entry %p, want %p, err %v", again, kept, err)
	}
}
