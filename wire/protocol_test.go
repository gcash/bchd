// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import "testing"

// TestServiceFlagStringer tests the stringized output for service flag types.
func TestServiceFlagStringer(t *testing.T) {
	tests := []struct {
		in   ServiceFlag
		want string
	}{
		{0, "0x0"},
		{SFNodeNetwork, "SFNodeNetwork"},
		{SFNodeGetUTXO, "SFNodeGetUTXO"},
		{SFNodeBloom, "SFNodeBloom"},
		{SFNodeWitness, "SFNodeWitness"},
		{SFNodeXthin, "SFNodeXthin"},
		{SFNodeBitcoinCash, "SFNodeBitcoinCash"},
		{SFNodeGraphene, "SFNodeGraphene"},
		{SFNodeWeakBlocks, "SFNodeWeakBlocks"},
		{SFNodeCF, "SFNodeCF"},
		{SFNodeSHV, "SFNodeSHV"},
		{SFNodeNetworkLimited, "SFNodeNetworkLimited"},
		{0xffffffff, "SFNodeNetwork|SFNodeGetUTXO|SFNodeBloom|SFNodeWitness|SFNodeXthin|SFNodeBitcoinCash|SFNodeGraphene|SFNodeWeakBlocks|SFNodeCF|SFNodeSHV|SFNodeNetworkLimited|0xfffff800"},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		result := test.in.String()
		if result != test.want {
			t.Errorf("String #%d\n got: %s want: %s", i, result,
				test.want)
			continue
		}
	}
}

// TestSFNodeSHV ensures SFNodeSHV is bit 9, CHIP-2026-02's NODE_SHV, and that
// the deprecated SFNodeXThinner, which named the bit before, still names it.
func TestSFNodeSHV(t *testing.T) {
	if SFNodeSHV != 1<<9 {
		t.Errorf("SFNodeSHV is %#x, want %#x", uint64(SFNodeSHV), 1<<9)
	}
	if SFNodeXThinner != SFNodeSHV {
		t.Errorf("SFNodeXThinner is %#x, want SFNodeSHV",
			uint64(SFNodeXThinner))
	}
	if SFNodeNetworkLimited != 1<<10 {
		t.Errorf("SFNodeNetworkLimited is %#x, want %#x",
			uint64(SFNodeNetworkLimited), 1<<10)
	}
}

// TestBitcoinNetStringer tests the stringized output for bitcoin net types.
func TestBitcoinNetStringer(t *testing.T) {
	tests := []struct {
		in   BitcoinNet
		want string
	}{
		{MainNet, "MainNet"},
		{TestNet, "TestNet"},
		{TestNet3, "TestNet3"},
		{SimNet, "SimNet"},
		{0xffffffff, "Unknown BitcoinNet (4294967295)"},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		result := test.in.String()
		if result != test.want {
			t.Errorf("String #%d\n got: %s want: %s", i, result,
				test.want)
			continue
		}
	}
}
