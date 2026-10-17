// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/gcash/bchd/chaincfg"
)

// TestFastSyncCheckpoints ensures fast sync uses the checkpoints up to the
// last one with a UTXO set, and all of them on networks without one.
func TestFastSyncCheckpoints(t *testing.T) {
	mainnet := fastSyncCheckpoints(chaincfg.MainNetParams.Checkpoints)
	last := mainnet[len(mainnet)-1]
	if last.Height != 661648 || last.UtxoSetHash == nil {
		t.Fatalf("mainnet fast sync checkpoint at height %d", last.Height)
	}

	chip := chaincfg.ChipNetParams.Checkpoints
	if got := fastSyncCheckpoints(chip); len(got) != len(chip) {
		t.Fatalf("chipnet fast sync checkpoints %d, want %d", len(got),
			len(chip))
	}
}
