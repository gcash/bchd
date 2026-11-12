// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestMerkle tests the BuildMerkleTreeStore API.
func TestMerkle(t *testing.T) {
	block := bchutil.NewBlock(&Block100000)
	merkles := BuildMerkleTreeStore(block.Transactions())
	calculatedMerkleRoot := merkles[len(merkles)-1]
	wantMerkle := &Block100000.Header.MerkleRoot
	if !wantMerkle.IsEqual(calculatedMerkleRoot) {
		t.Errorf("BuildMerkleTreeStore: merkle root mismatch - "+
			"got %v, want %v", calculatedMerkleRoot, wantMerkle)
	}
}

// TestMerkleBranch checks that the branch of every transaction leads to the
// Merkle root, in block 100000 and in blocks of 1 to 20 transactions, where a
// node without a partner is hashed with itself.
func TestMerkleBranch(t *testing.T) {
	check := func(txs []*bchutil.Tx, want *chainhash.Hash) {
		t.Helper()
		merkles := BuildMerkleTreeStore(txs)
		for i, tx := range txs {
			branch := MerkleBranch(merkles, i)
			if got := MerkleBranchRoot(*tx.Hash(), uint32(i), branch); got != *want {
				t.Fatalf("%d transactions, transaction %d: root %v, want %v",
					len(txs), i, got, want)
			}
		}
	}
	block := bchutil.NewBlock(&Block100000)
	check(block.Transactions(), &Block100000.Header.MerkleRoot)

	for n := 1; n <= 20; n++ {
		txs := make([]*bchutil.Tx, n)
		for i := range txs {
			msgTx := wire.NewMsgTx(1)
			msgTx.LockTime = uint32(i)
			txs[i] = bchutil.NewTx(msgTx)
		}
		merkles := BuildMerkleTreeStore(txs)
		check(txs, merkles[len(merkles)-1])
	}
}
