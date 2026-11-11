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

	// Every coinbase pays 50 BCH, so it has to escape the forbidden shape.
	// TODO: once the subsidy follows the block interval these pay under
	// 25 BCH, so make one pay more to keep covering that case.
	for i, hash := range hashes {
		checkCoinbaseVersion(t, h, i+1, hash)
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
