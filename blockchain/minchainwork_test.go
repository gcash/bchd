// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
)

// TestMinimumChainWork ensures the chain is not current while the best chain
// has less than the network's minimum chain work.
func TestMinimumChainWork(t *testing.T) {
	params := chaincfg.RegressionNetParams
	chain := newFakeChain(&params)
	node := newFakeNode(chain.bestChain.Genesis(), 1, params.PowLimitBits, time.Now())
	chain.index.AddNode(node)
	chain.bestChain.SetTip(node)
	if !chain.IsCurrent() {
		t.Fatal("chain with a recent tip and no minimum work is not current")
	}
	params.MinimumChainWork = new(big.Int).Add(node.workSum, big.NewInt(1))
	if chain.IsCurrent() {
		t.Fatal("chain below the minimum chain work is current")
	}
}
