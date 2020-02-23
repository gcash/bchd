// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import "testing"

// TestInvalidHashStr ensures the newShaHashFromStr function panics when used to
// with an invalid hash string.
func TestInvalidHashStr(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for invalid hash, got nil")
		}
	}()
	newHashFromStr("banana")
}

// TestMustRegisterPanic ensures the mustRegister function panics when used to
// register an invalid network.
func TestMustRegisterPanic(t *testing.T) {
	t.Parallel()

	// Setup a defer to catch the expected panic to ensure it actually
	// paniced.
	defer func() {
		if err := recover(); err == nil {
			t.Error("mustRegister did not panic as expected")
		}
	}()

	// Intentionally try to register duplicate params to force a panic.
	mustRegister(&MainNetParams)
}

// TestSeeds ensures the right seeds are defined.
func TestSeeds(t *testing.T) {
	expectedSeeds := []DNSSeed{
		{"seed.bchd.cash", true},
		{"seed.bitcoinabc.org", true},
		{"seed-abc.bitcoinforks.org", true},
		{"btccash-seeder.bitcoinunlimited.info", true},
	}

	if MainNetParams.DNSSeeds == nil {
		t.Error("Seed values are not set")
		return
	}

	if len(MainNetParams.DNSSeeds) != len(expectedSeeds) {
		t.Error("Incorrect number of seed values")
		return
	}

	for i := range MainNetParams.DNSSeeds {
		if MainNetParams.DNSSeeds[i] != expectedSeeds[i] {
			t.Error("Seed values are incorrect")
			return
		}
	}
}
