// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// checkChainTips fails the test unless the chain's tips are those wanted, the
// highest first and at a height by hash.
func checkChainTips(t *testing.T, chain *BlockChain, want ...ChainTip) {
	t.Helper()
	wanted := make(map[chainhash.Hash]ChainTip)
	for _, tip := range want {
		wanted[tip.Hash] = tip
	}
	got := chain.ChainTips()
	if len(got) != len(want) {
		t.Fatalf("got %d tips %+v, want %d", len(got), got, len(want))
	}
	for i, tip := range got {
		if w, ok := wanted[tip.Hash]; !ok || tip != w {
			t.Fatalf("got tip %+v, want %+v", tip, w)
		}
		if i == 0 {
			continue
		}
		prev := got[i-1]
		if prev.Height < tip.Height || prev.Height == tip.Height &&
			bytes.Compare(prev.Hash[:], tip.Hash[:]) >= 0 {

			t.Fatalf("tip %+v is listed after %+v", tip, prev)
		}
	}
}

// TestChainTips ensures the tips of the block tree are the main chain's tip,
// even with a header kept on it, and every block or kept header off the main
// chain that nothing builds on, each with the length of its branch and BCHN's
// status for it.
func TestChainTips(t *testing.T) {
	chain, nodes := headerTestChain(t, 5)
	bits := chain.chainParams.PowLimitBits
	sideNode := func(parent *blockNode, status blockStatus) *blockNode {
		node := newFakeNode(parent, 4, bits, after(parent, 11*time.Minute))
		node.status = status
		chain.index.AddNode(node)
		return node
	}
	tip := func(node *blockNode, branchLen int32, status ChainTipStatus) ChainTip {
		return ChainTip{Hash: node.hash, Height: node.height,
			BranchLen: branchLen, Status: status}
	}
	checkChainTips(t, chain, tip(nodes[5], 0, ChainTipActive))

	// A branch that was the main chain once, one never validated, one with
	// an invalid block, and a header of fastsync without its block.
	fork := sideNode(sideNode(nodes[3], statusDataStored|statusValid),
		statusDataStored|statusValid)
	unvalidated := sideNode(nodes[4], statusDataStored)
	invalid := sideNode(sideNode(nodes[2], statusDataStored|statusValidateFailed),
		statusDataStored|statusInvalidAncestor)
	noData := sideNode(nodes[1], statusValid)

	// Kept headers on the main chain's tip, and on a block found invalid
	// after they were kept.
	onTip := testHeader(nodes[5].hash, after(nodes[5], 10*time.Minute),
		bits, 0)
	processHeaders(t, chain, 1, onTip)
	invalidated := newFakeNode(nodes[2], 4, bits, after(nodes[2], 12*time.Minute))
	invalidated.status = statusDataStored
	chain.index.AddNode(invalidated)
	kept1 := testHeader(invalidated.hash, after(invalidated, 10*time.Minute),
		bits, 0)
	kept2 := testHeader(hashOf(kept1), headerAfter(kept1, 10*time.Minute),
		bits, 0)
	processHeaders(t, chain, 1, kept1, kept2)
	chain.index.SetStatusFlags(invalidated, statusValidateFailed)

	checkChainTips(t, chain,
		tip(nodes[5], 0, ChainTipActive),
		tip(fork, 2, ChainTipValidFork),
		tip(unvalidated, 1, ChainTipValidHeaders),
		tip(invalid, 2, ChainTipInvalid),
		tip(noData, 1, ChainTipHeadersOnly),
		ChainTip{Hash: hashOf(onTip), Height: 6, BranchLen: 1,
			Status: ChainTipHeadersOnly},
		ChainTip{Hash: hashOf(kept2), Height: 5, BranchLen: 3,
			Status: ChainTipInvalid},
	)
}

// TestChainTipStatusString ensures the statuses are named as in BCHN.
func TestChainTipStatusString(t *testing.T) {
	tests := []struct {
		status ChainTipStatus
		want   string
	}{
		{ChainTipActive, "active"},
		{ChainTipValidFork, "valid-fork"},
		{ChainTipValidHeaders, "valid-headers"},
		{ChainTipHeadersOnly, "headers-only"},
		{ChainTipInvalid, "invalid"},
		{ChainTipInvalid + 1, "Unknown ChainTipStatus (5)"},
	}
	for _, test := range tests {
		if got := test.status.String(); got != test.want {
			t.Errorf("got %q, want %q", got, test.want)
		}
	}
}
