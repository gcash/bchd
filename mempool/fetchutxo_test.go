// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"testing"

	"github.com/gcash/bchd/chaincfg"
)

// TestFetchUtxoViewAllocs checks that loading the utxo view of a tx stays
// linear in the number of its outputs.
func TestFetchUtxoViewAllocs(t *testing.T) {
	harness, outputs, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := harness.CreateSignedTx(outputs[:1], 500)
	if err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(10, func() {
		if _, err := harness.txPool.fetchUtxoView(tx); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 100 {
		t.Fatalf("fetchUtxoView of a tx with 500 outputs made %v allocations",
			allocs)
	}
}

// BenchmarkFetchUtxoView measures loading the utxo view of a tx with many
// outputs.
func BenchmarkFetchUtxoView(b *testing.B) {
	harness, outputs, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		b.Fatal(err)
	}
	tx, err := harness.CreateSignedTx(outputs[:1], 500)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := harness.txPool.fetchUtxoView(tx); err != nil {
			b.Fatal(err)
		}
	}
}
