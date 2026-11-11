// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

//go:build replay

package blockchain

// TipABLAState returns the control block size and the elastic buffer size of
// the ABLA state after the chain tip.
func (b *BlockChain) TipABLAState() (epsilon, beta uint64) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()
	return b.ablaState.controlBlockSize, b.ablaState.elasticBufferSize
}
