// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/integration/rpctest"
)

// TestCompetingChains mines competing chains on two connected nodes at the
// same time and checks that they soon agree on one.  The nodes relay their
// blocks to each other as compact blocks that were not requested, so the node
// whose chain has less work gets the other chain's blocks as orphans while it
// requests their parents.  It must keep those orphans, or it only gets the
// next block each time it drops its stalled sync peer, every two minutes.
func TestCompetingChains(t *testing.T) {
	h1, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h1.TearDown()
	if err := h1.SetUp(false, 0); err != nil {
		t.Fatal(err)
	}
	h2, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h2.TearDown()
	if err := h2.SetUp(false, 0); err != nil {
		t.Fatal(err)
	}
	if err := rpctest.ConnectNode(h1, h2); err != nil {
		t.Fatal(err)
	}
	if _, err := h1.Node.Generate(20); err != nil {
		t.Fatal(err)
	}
	if err := rpctest.JoinNodes([]*rpctest.Harness{h1, h2}, rpctest.Blocks); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 1)
	go func() {
		_, err := h2.Node.Generate(75)
		errs <- err
	}()
	if _, err := h1.Node.Generate(50); err != nil {
		t.Fatal(err)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		hash1, height1, err := h1.Node.GetBestBlock()
		if err != nil {
			t.Fatal(err)
		}
		hash2, height2, err := h2.Node.GetBestBlock()
		if err != nil {
			t.Fatal(err)
		}
		if *hash1 == *hash2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("nodes did not agree on a chain within 30 seconds: "+
				"heights %d and %d", height1, height2)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
