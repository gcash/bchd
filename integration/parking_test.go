// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/integration/rpctest"
)

// TestParkBlock ensures parkblock takes a block and the blocks after it off
// the main chain only while automatic unparking is off, and unparkblock
// brings them back.
func TestParkBlock(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		stay bool
	}{
		{"automatic unparking", nil, false},
		{"no automatic unparking", []string{"--noautomaticunparking"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, err := rpctest.New(&chaincfg.SimNetParams, nil, test.args)
			if err != nil {
				t.Fatal(err)
			}
			defer h.TearDown()
			if err := h.SetUp(false, 0); err != nil {
				t.Fatal(err)
			}
			hashes, err := h.Node.Generate(10)
			if err != nil {
				t.Fatal(err)
			}
			tip := hashes[9]

			// Parking the block before the tip disconnects both,
			// and with automatic unparking connects them again.
			if err := h.Node.ParkBlock(hashes[8]); err != nil {
				t.Fatalf("parkblock: %v", err)
			}
			best, err := h.Node.GetBestBlockHash()
			if err != nil {
				t.Fatal(err)
			}
			want := tip
			if test.stay {
				want = hashes[7]
			}
			if *best != *want {
				t.Fatalf("best block %v after parkblock, want %v",
					best, want)
			}

			if err := h.Node.UnparkBlock(hashes[8]); err != nil {
				t.Fatalf("unparkblock: %v", err)
			}
			best, err = h.Node.GetBestBlockHash()
			if err != nil {
				t.Fatal(err)
			}
			if *best != *tip {
				t.Fatalf("best block %v after unparkblock, want %v",
					best, tip)
			}
		})
	}
}
