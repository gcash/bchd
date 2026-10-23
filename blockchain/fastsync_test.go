// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

// serveUtxoSet serves a utxo set of anyone-can-spend outputs over HTTP, in
// the format fast sync downloads, and returns its URL, hash and size.
func serveUtxoSet(t *testing.T, outpoints ...wire.OutPoint) (string, chainhash.Hash, uint32) {
	t.Helper()
	var file []byte
	m := bchec.NewMultiset(bchec.S256())
	for _, outpoint := range outpoints {
		utxo := make([]byte, 52, 52+len(opTrueScript))
		copy(utxo, outpoint.Hash[:])
		binary.LittleEndian.PutUint32(utxo[32:], outpoint.Index)
		binary.LittleEndian.PutUint32(utxo[36:], 1)
		binary.LittleEndian.PutUint64(utxo[40:], 5000)
		binary.LittleEndian.PutUint32(utxo[48:], uint32(len(opTrueScript)))
		utxo = append(utxo, opTrueScript...)
		m.Add(utxo)
		file = append(file, utxo...)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(file)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/utxoset", m.Hash(), uint32(len(file))
}

// fastSyncTo returns a chain configuration for a fast sync to the block at
// height with hash, downloading the utxo set at url with the given hash and
// size into a new directory.
func fastSyncTo(t *testing.T, height int32, hash *chainhash.Hash, url string, setHash chainhash.Hash, size uint32) func(*Config) {
	dataDir := t.TempDir()
	return func(config *Config) {
		config.FastSync = true
		config.FastSyncDataDir = dataDir
		config.Checkpoints = []chaincfg.Checkpoint{{
			Height:         height,
			Hash:           hash,
			UtxoSetHash:    &setHash,
			UtxoSetSources: []string{url},
			UtxoSetSize:    size,
		}}
	}
}

// waitFastSync waits until the fast sync of the chain has loaded the utxo set.
func waitFastSync(t *testing.T, chain *BlockChain) {
	t.Helper()
	select {
	case <-chain.FastSyncDoneChan():
	case <-time.After(30 * time.Second):
		t.Fatal("the fast sync did not finish")
	}
}

// requireFastSyncMarker fails the test unless the fast sync is marked as
// unfinished exactly when want is set.
func requireFastSyncMarker(t *testing.T, db database.DB, want bool) {
	t.Helper()
	var found bool
	err := db.View(func(dbTx database.Tx) error {
		found = dbTx.Metadata().Get(chainStateFastSyncKeyName) != nil
		return nil
	})
	if err != nil {
		t.Fatalf("failed to read the fast sync marker: %v", err)
	}
	if found != want {
		t.Fatalf("fast sync marked as unfinished %v, want %v", found, want)
	}
}

// requireUnspent fails the test unless the output is unspent in the chain
// exactly when want is set.
func requireUnspent(t *testing.T, chain *BlockChain, outpoint wire.OutPoint, want bool) {
	t.Helper()
	entry, err := chain.FetchUtxoEntry(outpoint)
	if err != nil {
		t.Fatalf("FetchUtxoEntry: %v", err)
	}
	if isUnspent := entry != nil && !entry.IsSpent(); isUnspent != want {
		t.Fatalf("output %v unspent %v, want %v", outpoint, isUnspent, want)
	}
}

// newFastSyncTest creates the reorg test chain at dir with its utxo set
// flushed, closes it, and returns it with its tip.
func newFastSyncTest(t *testing.T, dir string, params *chaincfg.Params) (*reorgTest, *BestState) {
	t.Helper()
	r := newReorgTest(t, dir, params)
	if err := r.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("failed to flush the utxo set: %v", err)
	}
	tip := r.chain.BestSnapshot()
	r.db.Close()
	return r, tip
}

// TestFastSyncWithoutUtxoSetRefused ensures fast sync is refused before the
// database changes when the last checkpoint has no utxo set to download, as
// on testnet4 and chipnet, or misses any of its hash, sources and size.
func TestFastSyncWithoutUtxoSetRefused(t *testing.T) {
	params := chaincfg.RegressionNetParams
	tests := []struct {
		name    string
		hash    *chainhash.Hash
		sources []string
		size    uint32
	}{
		{name: "no utxo set"},
		{name: "no hash", sources: []string{"http://[::1"}, size: 1},
		{name: "no sources", hash: &chainhash.Hash{1}, size: 1},
		{name: "no size", hash: &chainhash.Hash{1},
			sources: []string{"http://[::1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "db")
			r := newReorgTest(t, dir, &params)
			if err := r.chain.FlushCachedState(FlushRequired); err != nil {
				t.Fatalf("failed to flush the utxo set: %v", err)
			}
			tip := r.chain.BestSnapshot()
			code, hash := fetchUtxoStatus(t, r.db)
			r.db.Close()

			_, _, err := openTestChainConfig(t, dir, &params, func(config *Config) {
				config.FastSync = true
				config.Checkpoints = []chaincfg.Checkpoint{{
					Height:         tip.Height,
					Hash:           &tip.Hash,
					UtxoSetHash:    test.hash,
					UtxoSetSources: test.sources,
					UtxoSetSize:    test.size,
				}}
			})
			if err == nil {
				t.Fatal("fast sync started without a utxo set to download")
			}

			chain, db, err := openTestChain(t, dir, &params)
			if err != nil {
				t.Fatalf("failed to restart: %v", err)
			}
			defer db.Close()
			if chain.IsPruned() {
				t.Error("the refused fast sync marked the database as pruned")
			}
			gotCode, gotHash := fetchUtxoStatus(t, db)
			if gotCode != code || *gotHash != *hash {
				t.Errorf("utxo state status %d at %v, want %d at %v",
					gotCode, gotHash, code, hash)
			}
			requireFastSyncMarker(t, db, false)
			r.requireUtxoSet(t, chain)
		})
	}
}

// TestFastSyncReplacesUtxoSet ensures a fast sync replaces the utxo set of an
// earlier start with the downloaded one, which a later start without fast
// sync then uses.
func TestFastSyncReplacesUtxoSet(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r, tip := newFastSyncTest(t, dir, &params)

	downloaded := wire.OutPoint{Hash: chainhash.Hash{0xaa}}
	url, setHash, size := serveUtxoSet(t, downloaded)
	chain, db, err := openTestChainConfig(t, dir, &params,
		fastSyncTo(t, tip.Height, &tip.Hash, url, setHash, size))
	if err != nil {
		t.Fatalf("failed to start the fast sync: %v", err)
	}
	waitFastSync(t, chain)
	requireFastSyncMarker(t, db, false)
	db.Close()

	chain, db, err = openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to start after the fast sync: %v", err)
	}
	defer db.Close()
	requireConsistentAtTip(t, chain)
	requireUnspent(t, chain, downloaded, true)
	for _, outpoint := range r.unspent {
		requireUnspent(t, chain, outpoint, false)
	}
}

// TestFastSyncUnfinishedRefusesNormalStart ensures that once a fast sync has
// started to replace the utxo set, a start without fast sync is refused until
// a fast sync has downloaded it, whether the tip is still the block it
// started from or a header the fast sync stored without its block.
func TestFastSyncUnfinishedRefusesNormalStart(t *testing.T) {
	params := chaincfg.RegressionNetParams
	for _, addHeader := range []bool{false, true} {
		dir := filepath.Join(t.TempDir(), "db")
		_, tip := newFastSyncTest(t, dir, &params)

		// The request for the download fails at once, so only the start
		// of the fast sync changes the database, and then the header if
		// the sync manager stores one.
		chain, db, err := openTestChainConfig(t, dir, &params,
			fastSyncTo(t, tip.Height+1, &chainhash.Hash{2}, "http://[::1",
				chainhash.Hash{1}, 1))
		if err != nil {
			t.Fatalf("failed to start the fast sync: %v", err)
		}
		if addHeader {
			header := wire.BlockHeader{
				Version:   1,
				PrevBlock: tip.Hash,
				Timestamp: time.Unix(1700000000, 0),
				Bits:      params.PowLimitBits,
			}
			if err := chain.AddHeader(&header); err != nil {
				t.Fatalf("failed to add a header: %v", err)
			}
		}
		db.Close()

		_, _, err = openTestChain(t, dir, &params)
		if err == nil || !strings.Contains(err.Error(), "--fastsync") {
			t.Fatalf("header %v: start without fast sync after an "+
				"unfinished one: got %v, want it refused", addHeader, err)
		}
		if addHeader {
			continue
		}

		downloaded := wire.OutPoint{Hash: chainhash.Hash{0xaa}}
		url, setHash, size := serveUtxoSet(t, downloaded)
		chain, db, err = openTestChainConfig(t, dir, &params,
			fastSyncTo(t, tip.Height, &tip.Hash, url, setHash, size))
		if err != nil {
			t.Fatalf("failed to restart the fast sync: %v", err)
		}
		waitFastSync(t, chain)
		db.Close()

		chain, db, err = openTestChain(t, dir, &params)
		if err != nil {
			t.Fatalf("failed to start after the fast sync: %v", err)
		}
		requireUnspent(t, chain, downloaded, true)
		db.Close()
	}
}

// TestFastSyncFailedVerification ensures a downloaded utxo set whose hash does
// not match the checkpoint is not marked as the state at the checkpoint, a
// start without fast sync refuses it even after the flush on shutdown, and
// the next fast sync replaces it.
func TestFastSyncFailedVerification(t *testing.T) {
	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	r := newReorgTest(t, dir, &params)
	if err := r.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("failed to flush the utxo set: %v", err)
	}
	tip := r.chain.BestSnapshot()

	// Start a fast sync on the chain, with the empty utxo cache of a new
	// start, and download a set that does not match the checkpoint.
	r.chain.utxoCache = newUtxoCache(r.db, 250*1024*1024)
	if err := r.chain.utxoCache.InitConsistentState(nil, nil, true, nil); err != nil {
		t.Fatalf("failed to start the fast sync: %v", err)
	}
	bad := wire.OutPoint{Hash: chainhash.Hash{0xbb}}
	url, _, size := serveUtxoSet(t, bad)
	r.chain.fastSyncDataDir = t.TempDir()
	err := r.chain.fastSyncUtxoSet(&chaincfg.Checkpoint{
		Height:         tip.Height,
		Hash:           &tip.Hash,
		UtxoSetHash:    &chainhash.Hash{1},
		UtxoSetSources: []string{url},
		UtxoSetSize:    size,
	}, "")
	if err == nil {
		t.Fatal("a utxo set that does not match the checkpoint was loaded")
	}
	if code, hash := fetchUtxoStatus(t, r.db); code == ucsConsistent && *hash == tip.Hash {
		t.Fatal("the utxo set that failed verification is marked as " +
			"consistent at the checkpoint")
	}
	requireFastSyncMarker(t, r.db, true)

	// The flush on shutdown writes the downloaded outputs.
	if err := r.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("failed to flush the utxo set: %v", err)
	}
	r.db.Close()
	if _, _, err := openTestChain(t, dir, &params); err == nil {
		t.Fatal("start without fast sync accepted the utxo set that " +
			"failed verification")
	}

	downloaded := wire.OutPoint{Hash: chainhash.Hash{0xaa}}
	url, setHash, size := serveUtxoSet(t, downloaded)
	chain, db, err := openTestChainConfig(t, dir, &params,
		fastSyncTo(t, tip.Height, &tip.Hash, url, setHash, size))
	if err != nil {
		t.Fatalf("failed to restart the fast sync: %v", err)
	}
	waitFastSync(t, chain)
	db.Close()

	chain, db, err = openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to start after the fast sync: %v", err)
	}
	defer db.Close()
	requireUnspent(t, chain, downloaded, true)
	requireUnspent(t, chain, bad, false)
}

// TestFastSyncStartDeletesUtxoSet ensures the start of a fast sync deletes
// whatever an earlier start left in the utxo set, here part of an earlier
// download, after it marks the download as unfinished, so a start stopped
// while it deletes, here in batches of one, is finished by the next one.
func TestFastSyncStartDeletesUtxoSet(t *testing.T) {
	saved := reIndexDeleteBatch
	reIndexDeleteBatch = 1
	t.Cleanup(func() { reIndexDeleteBatch = saved })

	params := chaincfg.RegressionNetParams
	dir := filepath.Join(t.TempDir(), "db")
	_, db, err := openTestChain(t, dir, &params)
	if err != nil {
		t.Fatalf("failed to open chain: %v", err)
	}
	defer db.Close()

	// The outputs and the status that the flushes of an earlier download
	// write.
	entries := make(map[wire.OutPoint]*UtxoEntry)
	for i := range 3 {
		entries[wire.OutPoint{Hash: chainhash.Hash{byte(i + 1)}}] = NewUtxoEntry(
			wire.NewTxOut(5000, opTrueScript, wire.TokenData{}), 1, false)
	}
	err = db.Update(func(dbTx database.Tx) error {
		if err := dbPutUtxoEntries(dbTx, entries); err != nil {
			return err
		}
		return dbPutUtxoStateConsistency(dbTx, ucsConsistent,
			&chainhash.Hash{})
	})
	if err != nil {
		t.Fatalf("failed to write the utxo set: %v", err)
	}

	// The marker and one entry's deletion commit, then the next fails.
	cache := newUtxoCache(&failingCommitDB{DB: db, failOn: 2}, 1<<20)
	if err := cache.InitConsistentState(nil, nil, true, nil); err != errTestCommit {
		t.Fatalf("got %v, want %v", err, errTestCommit)
	}
	requireFastSyncMarker(t, db, true)
	if code, hash := fetchUtxoStatus(t, db); code != ucsFlushOngoing || *hash != (chainhash.Hash{}) {
		t.Fatalf("utxo state status %d at %v, want a flush from no block",
			code, hash)
	}

	cache = newUtxoCache(db, 1<<20)
	if err := cache.InitConsistentState(nil, nil, true, nil); err != nil {
		t.Fatalf("failed to start the fast sync: %v", err)
	}
	err = db.View(func(dbTx database.Tx) error {
		if dbTx.Metadata().Bucket(utxoSetBucketName).Cursor().First() {
			t.Error("outputs left in the utxo set")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// progressGoroutines returns the number of goroutines that fastSyncUtxoSet
// started and are still running.
func progressGoroutines() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]),
				"blockchain.(*BlockChain).fastSyncUtxoSet.func")
		}
		buf = make([]byte, 2*len(buf))
	}
}

// TestFastSyncProgress ensures the goroutine that logs the progress of the
// verification of the downloaded utxo set reads the count of bytes read
// safely, which the race detector checks, and ends with the verification.
// It read the count without synchronization, and as stopping a ticker does
// not close its channel, it never ended.
func TestFastSyncProgress(t *testing.T) {
	saved := fastSyncProgressInterval
	fastSyncProgressInterval = time.Millisecond
	t.Cleanup(func() { fastSyncProgressInterval = saved })

	params := chaincfg.RegressionNetParams
	r := newReorgTest(t, filepath.Join(t.TempDir(), "db"), &params)
	defer r.db.Close()
	if err := r.chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("failed to flush the utxo set: %v", err)
	}
	tip := r.chain.BestSnapshot()
	r.chain.utxoCache = newUtxoCache(r.db, 250*1024*1024)
	if err := r.chain.utxoCache.InitConsistentState(nil, nil, true, nil); err != nil {
		t.Fatalf("failed to start the fast sync: %v", err)
	}
	r.chain.fastSyncDataDir = t.TempDir()

	// Enough outputs to take many intervals to verify.  The set does not
	// match the checkpoint, so the sync ends after the verification.
	outpoints := make([]wire.OutPoint, 5000)
	for i := range outpoints {
		outpoints[i].Hash[0], outpoints[i].Hash[1] = byte(i), byte(i>>8)
	}
	url, _, size := serveUtxoSet(t, outpoints...)
	logged := captureInfoLog(t)
	before := progressGoroutines()
	err := r.chain.fastSyncUtxoSet(&chaincfg.Checkpoint{
		Height:         tip.Height,
		Hash:           &tip.Hash,
		UtxoSetHash:    &chainhash.Hash{1},
		UtxoSetSources: []string{url},
		UtxoSetSize:    size,
	}, "")
	if err == nil {
		t.Fatal("a utxo set that does not match the checkpoint was loaded")
	}
	if !strings.Contains(logged.String(), "UTXO verification progress") {
		t.Fatal("no progress logged")
	}

	deadline := time.Now().Add(5 * time.Second)
	for progressGoroutines() > before {
		if time.Now().After(deadline) {
			t.Fatal("the progress goroutine still runs after the " +
				"verification")
		}
		time.Sleep(time.Millisecond)
	}
}
