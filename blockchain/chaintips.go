// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// ChainTipStatus is the status of a tip of the block tree, as BCHN's
// getchaintips gives it.
type ChainTipStatus byte

// These constants are the statuses a tip of the block tree can have.
const (
	// ChainTipActive is the status of the tip of the main chain.
	ChainTipActive ChainTipStatus = iota

	// ChainTipValidFork is the status of the tip of a branch off the main
	// chain whose blocks were all fully validated, as when the branch was
	// the main chain once.
	ChainTipValidFork

	// ChainTipValidHeaders is the status of the tip of a branch whose
	// blocks the chain has, but has not all fully validated, as when the
	// branch never had the most work.
	ChainTipValidHeaders

	// ChainTipHeadersOnly is the status of the tip of a branch the chain
	// has the valid headers of, but not every block.
	ChainTipHeadersOnly

	// ChainTipInvalid is the status of the tip of a branch with a block
	// known to be invalid.
	ChainTipInvalid
)

// chainTipStatusStrings maps each ChainTipStatus to BCHN's name for it.
var chainTipStatusStrings = map[ChainTipStatus]string{
	ChainTipActive:       "active",
	ChainTipValidFork:    "valid-fork",
	ChainTipValidHeaders: "valid-headers",
	ChainTipHeadersOnly:  "headers-only",
	ChainTipInvalid:      "invalid",
}

// String returns BCHN's name for the status.
func (s ChainTipStatus) String() string {
	if str := chainTipStatusStrings[s]; str != "" {
		return str
	}
	return fmt.Sprintf("Unknown ChainTipStatus (%d)", int(s))
}

// ChainTip is a tip of the block tree: the tip of the main chain, or a block
// or kept header off the main chain that nothing the chain knows builds on.
type ChainTip struct {
	Hash   chainhash.Hash
	Height int32

	// BranchLen is the number of blocks from where the tip's branch forks
	// off the main chain to the tip, zero for the tip of the main chain.
	BranchLen int32

	Status ChainTipStatus
}

// ChainTips returns the tips of the block tree as BCHN's getchaintips does
// (rpc/blockchain.cpp): the tip of the main chain, and every block and kept
// header off the main chain that no other one builds on, the highest first,
// and at a height by hash.  Like BCHN, it reads every node of the block index
// and finds the fork of each tip by walking its branch.
//
// The status of a tip off the main chain is the first of these that fits, in
// the order BCHN checks them, with BCHD's block status for each of BCHN's:
//
//   - invalid: the block failed validation or has an invalid ancestor
//     (statusValidateFailed or statusInvalidAncestor), BCHN's
//     BLOCK_FAILED_MASK, or the tip is a kept header whose chain has such a
//     block.
//   - headers-only: the tip is a kept header, which BCHN keeps in its block
//     index without data and BCHD in the header tree, or a block without data
//     (no statusDataStored), as a fastsync header is; BCHN's
//     !HaveTxsDownloaded().
//   - valid-fork: the block was fully validated, scripts too (statusValid),
//     BCHN's BlockValidity::SCRIPTS.
//   - valid-headers: any other block, since a block is only added to the
//     block index once its header passed the checks BCHN's
//     BlockValidity::TREE stands for.
//
// BCHN's "parked" never applies, as BCHD doesn't park blocks, nor does its
// "unknown".
//
// This function is safe for concurrent access.
func (b *BlockChain) ChainTips() []ChainTip {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()
	b.bestChain.mtx.Lock()
	defer b.bestChain.mtx.Unlock()

	// As in BCHN, every node off the main chain is a tip unless another one
	// builds on it.  Every node of the main chain but its tip has the next
	// one built on it.
	side := make(map[*blockNode]blockStatus)
	builtOn := make(map[*blockNode]struct{})
	b.index.RLock()
	for _, node := range b.index.index {
		if !b.bestChain.contains(node) {
			side[node] = node.status
			builtOn[node.parent] = struct{}{}
		}
	}
	b.index.RUnlock()
	for _, e := range b.headers.entries {
		builtOn[e.node.parent] = struct{}{}
	}

	tip := b.bestChain.tip()
	tips := []ChainTip{{Hash: tip.hash, Height: tip.height,
		Status: ChainTipActive}}
	addTip := func(node *blockNode, status ChainTipStatus) {
		fork := b.bestChain.findFork(node)
		tips = append(tips, ChainTip{
			Hash:      node.hash,
			Height:    node.height,
			BranchLen: node.height - fork.height,
			Status:    status,
		})
	}
	for node, status := range side {
		if _, ok := builtOn[node]; ok {
			continue
		}
		switch {
		case status.KnownInvalid():
			addTip(node, ChainTipInvalid)
		case !status.HaveData():
			addTip(node, ChainTipHeadersOnly)
		case status.KnownValid():
			addTip(node, ChainTipValidFork)
		default:
			addTip(node, ChainTipValidHeaders)
		}
	}
	for _, e := range b.headers.entries {
		if _, ok := builtOn[e.node]; ok {
			continue
		}
		if b.headerChainInvalid(e.node) {
			addTip(e.node, ChainTipInvalid)
		} else {
			addTip(e.node, ChainTipHeadersOnly)
		}
	}

	slices.SortFunc(tips, func(x, y ChainTip) int {
		return cmp.Or(cmp.Compare(y.Height, x.Height),
			bytes.Compare(x.Hash[:], y.Hash[:]))
	})
	return tips
}
