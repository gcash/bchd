// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package rpctest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestUpgrade2027ActivatedHarness starts bchd with the 1-minute block upgrade
// activating in the past and mines with both the node's templates and the
// harness's own blocks.
func TestUpgrade2027ActivatedHarness(t *testing.T) {
	h, err := New(&chaincfg.RegressionNetParams, nil,
		[]string{Upgrade2027Arg(Upgrade2027InThePast)})
	if err != nil {
		t.Fatalf("unable to create harness: %v", err)
	}
	defer h.TearDown()
	if err := h.SetUp(false, 0); err != nil {
		t.Fatalf("unable to set up harness: %v", err)
	}

	hashes, err := h.Node.Generate(5)
	if err != nil {
		t.Fatalf("unable to generate blocks: %v", err)
	}
	for range 5 {
		block, err := h.GenerateAndSubmitBlock(nil, -1, time.Time{})
		if err != nil {
			t.Fatalf("unable to submit block: %v", err)
		}
		hashes = append(hashes, block.Hash())
	}

	info, err := h.Node.GetBlockChainInfo()
	if err != nil {
		t.Fatalf("unable to get chain info: %v", err)
	}
	if info.Blocks != 10 {
		t.Fatalf("chain height is %d, want 10", info.Blocks)
	}

	// Wall clock timestamps put the median time past above the activation
	// time either way, so check that bchd took the override instead.
	// TODO: check the activation state over RPC once bchd reports it.
	logFile := filepath.Join(h.node.config.logDir,
		chaincfg.RegressionNetParams.Name, "bchd.log")
	log, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("unable to read the node log: %v", err)
	}
	want := "Using upgrade 2027 activation time override: 1605451779"
	if !strings.Contains(string(log), want) {
		t.Fatalf("node log lacks %q", want)
	}

	// Every coinbase is version 2, including those up to the activation
	// block that pay 50 BCH and so must escape the forbidden shape.
	for i, hash := range hashes {
		checkCoinbaseVersion(t, h, i+1, hash)
	}

	// Blocks after the activation block pay a tenth of the subsidy, both
	// from the node's templates and the harness's own blocks.  The genesis
	// block is too old to activate, so block 1 at least pays 50 BCH.
	activation, err := h.upgrade2027Height(hashes[9], 10)
	if err != nil {
		t.Fatalf("unable to find the activation block: %v", err)
	}
	if activation < 1 || activation > 4 {
		t.Fatalf("activation block at %d, want one before block 5",
			activation)
	}
	for i, hash := range hashes {
		block, err := h.Node.GetBlock(hash)
		if err != nil {
			t.Fatalf("unable to get block %d: %v", i+1, err)
		}
		want := int64(500000000)
		if int32(i+1) <= activation {
			want = 5000000000
		}
		if v := block.Transactions[0].TxOut[0].Value; v != want {
			t.Errorf("coinbase of block %d pays %d, want %d", i+1, v,
				want)
		}
	}
}

func checkCoinbaseVersion(t *testing.T, h *Harness, height int,
	hash *chainhash.Hash) {

	t.Helper()
	block, err := h.Node.GetBlock(hash)
	if err != nil {
		t.Fatalf("unable to get block %d: %v", height, err)
	}
	if v := block.Transactions[0].Version; v != 2 {
		t.Errorf("coinbase of block %d has version %d, want 2", height, v)
	}
}
