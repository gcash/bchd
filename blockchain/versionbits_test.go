// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
)

// TestWarnUnknownVersionsIgnoresVersionRolling ensures blocks whose versions
// set only the bits BIP320 leaves to miners' version rolling don't make the
// chain warn about unknown versions, while an unknown bit outside them still
// does.  Miners roll those bits for ASICBoost, so most blocks set some of
// them, and the warning fired on every network although no new rules were in
// effect.
func TestWarnUnknownVersionsIgnoresVersionRolling(t *testing.T) {
	for _, test := range []struct {
		name    string
		version int32
		warn    bool
	}{
		{"version rolling bits", vbTopBits | 0x1fffe000, false},
		{"unknown bit 1", vbTopBits | 1<<1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			params := chaincfg.RegressionNetParams
			chain := newFakeChain(&params)
			node := chain.bestChain.Tip()
			for range unknownVerNumToCheck {
				node = newFakeNode(node, test.version, params.PowLimitBits,
					time.Unix(node.timestamp, 0).Add(time.Minute))
			}

			if err := chain.warnUnknownVersions(node); err != nil {
				t.Fatal(err)
			}
			if chain.unknownVersionsWarned != test.warn {
				t.Fatalf("warned about unknown versions: %v, want %v",
					chain.unknownVersionsWarned, test.warn)
			}
		})
	}
}

// TestUnknownRuleWarningIgnoresVersionRolling ensures a bit BIP320 leaves to
// version rolling does not count as new rules activating, however many blocks
// set it, while another unknown bit still does.
func TestUnknownRuleWarningIgnoresVersionRolling(t *testing.T) {
	params := chaincfg.RegressionNetParams
	for _, test := range []struct {
		bit    uint32
		warned bool
	}{
		{bit: 13, warned: false},
		{bit: 20, warned: false},
		{bit: 12, warned: true},
	} {
		chain := newFakeChain(&params)
		tip := chain.bestChain.Tip()
		version := int32(vbTopBits | 1<<test.bit)
		for range 4 * params.MinerConfirmationWindow {
			tip = newFakeNode(tip, version, params.PowLimitBits,
				time.Unix(tip.timestamp+600, 0))
			chain.index.AddNode(tip)
		}
		chain.bestChain.SetTip(tip)
		if err := chain.warnUnknownRuleActivations(tip); err != nil {
			t.Fatal(err)
		}
		if chain.unknownRulesWarned != test.warned {
			t.Errorf("bit %d: warned %v, want %v", test.bit,
				chain.unknownRulesWarned, test.warned)
		}
	}
}
