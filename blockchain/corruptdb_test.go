// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
)

// TestCorruptDatabaseRequestsShutdown ensures a block that finds the database
// corrupt, here the stored entry of an output it spends, requests the process
// to shut down, since no block that needs the data can be connected.  The
// block is not marked invalid, as it is not at fault.
func TestCorruptDatabaseRequestsShutdown(t *testing.T) {
	m := newRuleTestMiner(t, &chaincfg.RegressionNetParams)
	m.mustAccept(m.build(nil))
	prevOut := wire.OutPoint{Hash: m.tip.Transactions[0].TxHash()}
	value := m.tip.Transactions[0].TxOut[0].Value

	// The output is read from the database once the cache is flushed.
	if err := m.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("flush: %v", err)
	}
	err := m.chain.db.Update(func(dbTx database.Tx) error {
		key := outpointKey(prevOut)
		defer recycleOutpointKey(key)
		return dbTx.Metadata().Bucket(utxoSetBucketName).Put(*key,
			[]byte{0xff})
	})
	if err != nil {
		t.Fatalf("corrupting the entry: %v", err)
	}

	var logged bytes.Buffer
	logger := bchlog.NewBackend(&logged).Logger("CHAN")
	logger.SetLevel(bchlog.LevelCritical)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)

	block := m.build(nil, ruleTestSpend(prevOut, value, 1))
	err = m.accept(block)
	var dbErr database.Error
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrCorruption {
		t.Fatalf("got %v, want a corruption error", err)
	}
	select {
	case <-m.chain.RequestedProcessShutdown():
	default:
		t.Fatal("no shutdown requested for the corrupt database")
	}
	if m.chain.knownInvalid(block.Hash()) {
		t.Fatal("the block was marked invalid for the corrupt database")
	}
	for _, want := range []string{"The database is corrupt: corrupt utxo entry",
		"Restart with --reindexchainstate"} {
		if !strings.Contains(logged.String(), want) {
			t.Errorf("log %q does not say %q", logged.String(), want)
		}
	}
	if strings.Contains(logged.String(), "empty data directory") {
		t.Errorf("log %q advises a resync before a rebuild failed",
			logged.String())
	}
}

// TestReIndexFailureAdvisesResync ensures a re-index of the chain state that
// cannot read a stored block says to sync again from an empty data
// directory, the advice a corrupt database gets only once a rebuild failed.
func TestReIndexFailureAdvisesResync(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	block, err := r.chain.BlockByHeight(1)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := block.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	r.db.Close()

	// Flip a byte of the block in its block file, so reading it back fails
	// its checksum.
	path := filepath.Join(dir, fmt.Sprintf("%09d.fdb", 0))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(data, raw)
	if i < 0 {
		t.Fatal("block 1 is not in the first block file")
	}
	data[i+len(raw)/2] ^= 0xff
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	var logged bytes.Buffer
	logger := bchlog.NewBackend(&logged).Logger("CHAN")
	logger.SetLevel(bchlog.LevelCritical)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)

	_, _, err = openTestChainConfig(t, dir, &params, func(config *Config) {
		config.ReIndexChainState = true
	})
	var dbErr database.Error
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrCorruption {
		t.Fatalf("got %v, want a corruption error", err)
	}
	if !strings.Contains(logged.String(),
		"Sync again from an empty data directory") {

		t.Errorf("log %q does not advise a resync", logged.String())
	}
}

// TestRuleErrorNoShutdown ensures an invalid block does not request a
// shutdown.
func TestRuleErrorNoShutdown(t *testing.T) {
	m := newRuleTestMiner(t, &chaincfg.RegressionNetParams)
	block := m.build(nil)
	block.MsgBlock().Header.Bits = 0
	if err := m.accept(block); err == nil {
		t.Fatal("block with zero bits accepted")
	}
	select {
	case <-m.chain.RequestedProcessShutdown():
		t.Fatal("shutdown requested for an invalid block")
	default:
	}
}

// TestInterruptedReIndexNoResyncAdvice ensures a re-index of the chain state
// that is interrupted, which resumes on the next start, does not say to sync
// again from an empty data directory.
func TestInterruptedReIndexNoResyncAdvice(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	r.db.Close()

	var logged bytes.Buffer
	logger := bchlog.NewBackend(&logged).Logger("CHAN")
	logger.SetLevel(bchlog.LevelCritical)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)

	interrupt := make(chan struct{})
	close(interrupt)
	_, _, err := openTestChainConfig(t, dir, &params, func(config *Config) {
		config.ReIndexChainState = true
		config.Interrupt = interrupt
	})
	if err != errInterruptRequested {
		t.Fatalf("interrupted re-index returned %v", err)
	}
	if strings.Contains(logged.String(), "empty data directory") {
		t.Errorf("log %q advises a resync for an interrupted re-index",
			logged.String())
	}
}
