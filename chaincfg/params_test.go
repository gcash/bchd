// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"testing"

	"github.com/gcash/bchd/wire"
)

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
		{&TempNetParams, 327989, "00000000e18af2c66d44a1bbfe165acc4b87a625d211a09e33d5ee61d35dcb66"},
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

// TestMinimumChainWork ensures the minimum chain work of each network is the
// reference implementation's.
func TestMinimumChainWork(t *testing.T) {
	tests := []struct {
		params *Params
		work   string
	}{
		{&MainNetParams, "30fd678912166474fe93f14"},
		{&TestNet3Params, "11fdbbe582349040762"},
		{&TestNet4Params, "bd5a1b5bda45b5da"},
		{&ChipNetParams, "7a3195391c4408f2"},
		{&TempNetParams, "7a3195391c4408f2"},
		{&RegressionNetParams, ""},
		{&SimNetParams, ""},
	}
	for _, test := range tests {
		work := test.params.MinimumChainWork
		got := ""
		if work != nil {
			got = work.Text(16)
		}
		if got != test.work {
			t.Errorf("%s: minimum chain work %q, want %q",
				test.params.Name, got, test.work)
		}
	}
}

// TestDefaultConsensusBlockSize checks the default block size of each network
// and, where Bitcoin Cash Node asserts the same, that ABLA starts at it.
func TestDefaultConsensusBlockSize(t *testing.T) {
	tests := []struct {
		params      *Params
		size        uint32
		matchesABLA bool
	}{
		{&MainNetParams, 32000000, true},
		{&TestNet3Params, 32000000, true},
		{&TestNet4Params, 2000000, true},
		{&ChipNetParams, 2000000, true},
		{&TempNetParams, 2000000, true},
		{&RegressionNetParams, 32000000, false},
		{&SimNetParams, 32000000, true},
	}
	for _, test := range tests {
		size := test.params.DefaultConsensusBlockSize
		if size != test.size {
			t.Errorf("%s: default block size %d, want %d",
				test.params.Name, size, test.size)
		}
		abla := test.params.ABLAConfig.Epsilon0 + test.params.ABLAConfig.Beta0
		if test.matchesABLA && abla != uint64(size) {
			t.Errorf("%s: ABLA starts at %d, not at the default block "+
				"size %d", test.params.Name, abla, size)
		}
	}
}

// TestUpgrade2027ActivationTime pins the activation time of the 1-minute
// block upgrade on each network.  Regtest and simnet keep the mainnet time
// rather than 0, so the upgrade only activates there when asked for.
func TestUpgrade2027ActivationTime(t *testing.T) {
	tests := []struct {
		params *Params
		time   uint64
	}{
		{&MainNetParams, 1810382400},
		{&TestNet3Params, 1810382400},
		{&TestNet4Params, 1810382400},
		{&ChipNetParams, 1794744000},
		{&TempNetParams, 1792065600},
		{&RegressionNetParams, 1810382400},
		{&SimNetParams, 1810382400},
	}
	for _, test := range tests {
		got := test.params.Upgrade2027ActivationTime
		if got != test.time {
			t.Errorf("%s: upgrade 2027 activation time %d, want %d",
				test.params.Name, got, test.time)
		}
	}
}

// TestTempNetParams ensures tempnet is chipnet with the May 2027 upgrade
// activated on 2026-10-15, its own DNS seeds and a checkpoint at the first
// block after the fork, and that deriving it leaves chipnet unchanged.
func TestTempNetParams(t *testing.T) {
	if TempNetParams.Name != "tempnet" || TempNetParams.Net != wire.ChipNet ||
		TempNetParams.DefaultPort != "48333" {

		t.Fatalf("got name %q, net %v, port %s", TempNetParams.Name,
			TempNetParams.Net, TempNetParams.DefaultPort)
	}
	if *TempNetParams.GenesisHash != *ChipNetParams.GenesisHash ||
		TempNetParams.AsertDifficultyAnchorHeight !=
			ChipNetParams.AsertDifficultyAnchorHeight ||
		TempNetParams.ABLAConfig != ChipNetParams.ABLAConfig ||
		TempNetParams.CashAddressPrefix != "bchtest" {

		t.Fatal("tempnet does not share chipnet's chain parameters")
	}

	wantSeeds := []DNSSeed{
		{"tempnet.bitjson.com", false},
		{"chipnet.bitjson.com", true},
	}
	if len(TempNetParams.DNSSeeds) != len(wantSeeds) {
		t.Fatalf("got DNS seeds %v, want %v", TempNetParams.DNSSeeds,
			wantSeeds)
	}
	for i, seed := range wantSeeds {
		if TempNetParams.DNSSeeds[i] != seed {
			t.Errorf("DNS seed %d: got %v, want %v", i,
				TempNetParams.DNSSeeds[i], seed)
		}
	}

	// The checkpoints are chipnet's and the fork block, none with a UTXO
	// set to fast sync from.
	chip, temp := ChipNetParams.Checkpoints, TempNetParams.Checkpoints
	if len(temp) != len(chip)+1 {
		t.Fatalf("got %d checkpoints, want %d", len(temp), len(chip)+1)
	}
	for i := range chip {
		if temp[i].Height != chip[i].Height ||
			*temp[i].Hash != *chip[i].Hash {

			t.Errorf("checkpoint %d: got %d %v, want chipnet's %d %v",
				i, temp[i].Height, temp[i].Hash, chip[i].Height,
				chip[i].Hash)
		}
	}
	for _, checkpoint := range temp {
		if checkpoint.UtxoSetHash != nil ||
			len(checkpoint.UtxoSetSources) != 0 ||
			checkpoint.UtxoSetSize != 0 {

			t.Errorf("checkpoint %d has a UTXO set", checkpoint.Height)
		}
	}
	if last := chip[len(chip)-1]; last.Height != 325627 {
		t.Errorf("chipnet's latest checkpoint is now %d", last.Height)
	}
	if len(ChipNetParams.DNSSeeds) != 2 ||
		ChipNetParams.DNSSeeds[0].Host != "chipnet-seed.bchd.cash" {

		t.Errorf("chipnet DNS seeds changed: %v", ChipNetParams.DNSSeeds)
	}
}
