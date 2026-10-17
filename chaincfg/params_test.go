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
	// panicked.
	defer func() {
		if err := recover(); err == nil {
			t.Error("mustRegister did not panic as expected")
		}
	}()

	// Intentionally try to register duplicate params to force a panic.
	mustRegister(&MainNetParams)
}

// TestMainNetSeeds ensures the right seeds are defined.
func TestMainNetSeeds(t *testing.T) {
	expectedSeeds := []DNSSeed{
		{"seed.bchd.cash", true},
		{"btccash-seeder.bitcoinunlimited.info", true},
		{"seed.bch.loping.net", true},
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

// TestTestNet3Seeds ensures the right seeds are defined.
func TestTestNet3Seeds(t *testing.T) {
	expectedSeeds := []DNSSeed{
		{"testnet-seed.bchd.cash", true},
		{"testnet-seed-bch.bitcoinforks.org", true},
		{"seed.tbch.loping.net", true},
	}

	if TestNet3Params.DNSSeeds == nil {
		t.Error("Seed values are not set")
		return
	}

	if len(TestNet3Params.DNSSeeds) != len(expectedSeeds) {
		t.Error("Incorrect number of seed values")
		return
	}

	for i := range TestNet3Params.DNSSeeds {
		if TestNet3Params.DNSSeeds[i] != expectedSeeds[i] {
			t.Error("Seed values are incorrect")
			return
		}
	}
}

// TestCheckpoints ensures the checkpoints are sorted and include the latest
// checkpoint of the reference implementation for each network.
func TestCheckpoints(t *testing.T) {
	tests := []struct {
		params *Params
		height int32
		hash   string
	}{
		{&MainNetParams, 970892, "000000000000000000e76270e57f4ebbd2b282db46a1695984935126c0a68cd8"},
		{&TestNet3Params, 1730264, "000000000001ce982fa29717a4ed233b0e5a7a8b9936a17fdeeccc5c1d0af795"},
		{&TestNet4Params, 325709, "00000000000097a6ca206aa88b45a2158eea1a9ab8a49aed8f4b193d558e74b4"},
		{&ChipNetParams, 325627, "000000002bb8a32bd6b1022e5370b10e7dc139770d626e1a0911e6b596cf3069"},
	}
	for _, test := range tests {
		checkpoints := test.params.Checkpoints
		for i := 1; i < len(checkpoints); i++ {
			if checkpoints[i].Height <= checkpoints[i-1].Height {
				t.Errorf("%s: checkpoints not sorted at height %d",
					test.params.Name, checkpoints[i].Height)
			}
		}
		if len(checkpoints) == 0 {
			t.Errorf("%s: no checkpoints", test.params.Name)
			continue
		}
		last := checkpoints[len(checkpoints)-1]
		if last.Height != test.height || last.Hash.String() != test.hash {
			t.Errorf("%s: latest checkpoint %d %v, want %d %s",
				test.params.Name, last.Height, last.Hash,
				test.height, test.hash)
		}
	}
}
