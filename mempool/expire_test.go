// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
)

// TestExpire ensures the transactions that entered the pool longer ago than
// BCHN's default -mempoolexpiry leave it with the transactions spending them,
// and the others stay.
func TestExpire(t *testing.T) {
	t.Parallel()

	harness, spendableOuts, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create test pool: %v", err)
	}
	tc := &testContext{t, harness}
	chain, err := harness.CreateTxChain(spendableOuts[0], 3)
	if err != nil {
		t.Fatalf("unable to create transaction chain: %v", err)
	}
	for _, tx := range chain {
		if _, err := harness.txPool.ProcessTransaction(tx, false, 0); err != nil {
			t.Fatalf("ProcessTransaction: %v", err)
		}
	}

	// The first entered the pool an hour short of two weeks ago, the second
	// an hour more than two weeks ago, and the third, which spends the
	// second, now.
	pool := harness.txPool
	pool.pool[*chain[0].Hash()].Added = time.Now().Add(-335 * time.Hour)
	pool.pool[*chain[1].Hash()].Added = time.Now().Add(-337 * time.Hour)
	if n := pool.Expire(); n != 2 {
		t.Errorf("expired %d transactions, want 2", n)
	}
	testPoolMembership(tc, chain[0], false, true)
	testPoolMembership(tc, chain[1], false, false)
	testPoolMembership(tc, chain[2], false, false)
}
