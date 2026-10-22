// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/gcash/bchd/chaincfg"
)

// TestBlockWriteFailureRequestsShutdown ensures a failure to store a block or
// its block index entry, or to write the chain state when a block is
// connected or disconnected, requests the process to shut down rather than
// leaving the node running on its tip.
func TestBlockWriteFailureRequestsShutdown(t *testing.T) {
	tests := []struct {
		name string
		// fail makes the write the test is about fail, after failOn
		// earlier writes to the same database succeed.
		fail  func(chain *BlockChain)
		reorg bool
	}{{
		name: "block",
		fail: func(chain *BlockChain) {
			chain.db = &failingCommitDB{DB: chain.db}
		},
	}, {
		name: "block index entry",
		fail: func(chain *BlockChain) {
			chain.index.db = &failingCommitDB{DB: chain.index.db}
		},
	}, {
		name: "block status before connecting",
		fail: func(chain *BlockChain) {
			chain.index.db = &failingCommitDB{DB: chain.index.db, failOn: 1}
		},
	}, {
		name: "connected block",
		fail: func(chain *BlockChain) {
			chain.db = &failingCommitDB{DB: chain.db, failOn: 1}
		},
	}, {
		name: "block status before disconnecting",
		fail: func(chain *BlockChain) {
			chain.index.db = &failingCommitDB{DB: chain.index.db, failOn: 1}
		},
		reorg: true,
	}, {
		name: "disconnected block",
		fail: func(chain *BlockChain) {
			chain.db = &failingCommitDB{DB: chain.db, failOn: 1}
		},
		reorg: true,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := newRuleTestMiner(t, &chaincfg.RegressionNetParams)
			m.mustAccept(m.build(nil))

			// A branch from genesis whose second block replaces the
			// first block of the chain.
			if test.reorg {
				m.tip, m.height = m.chain.chainParams.GenesisBlock, 0
				cb := m.coinbase(1)
				cb.TxOut[1] = ruleTestPadding(1)
				m.mustAccept(m.build(cb))
			}

			test.fail(m.chain)
			if err := m.accept(m.build(nil)); err == nil {
				t.Fatal("block accepted with a failing database")
			}
			select {
			case <-m.chain.RequestedProcessShutdown():
			default:
				t.Fatal("no shutdown requested after the write failed")
			}
		})
	}
}
