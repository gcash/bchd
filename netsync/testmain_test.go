// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"os"
	"testing"

	"github.com/gcash/bchd/chaincfg"
)

// TestMain runs the tests with the upgrade to 1-minute blocks never active
// on every network, since the test chains stamp their blocks with the wall
// clock.
func TestMain(m *testing.M) {
	for _, params := range []*chaincfg.Params{&chaincfg.MainNetParams,
		&chaincfg.TestNet3Params, &chaincfg.TestNet4Params,
		&chaincfg.ChipNetParams, &chaincfg.RegressionNetParams,
		&chaincfg.SimNetParams} {

		params.Upgrade2027ActivationTime = 1<<48 - 1
	}
	os.Exit(m.Run())
}
