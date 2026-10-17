// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// ruleTestMiner builds and solves regtest blocks on top of a test chain so
// block-level consensus rules can be exercised through ProcessBlock.
type ruleTestMiner struct {
	t      *testing.T
	chain  *BlockChain
	tip    *wire.MsgBlock
	height int32
}

// newRuleTestMiner returns a chain using params along with a miner for it.
// The coinbase maturity is lowered to one block.
func newRuleTestMiner(t *testing.T, params *chaincfg.Params) *ruleTestMiner {
	t.Helper()
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	chain, teardown, err := chainSetup(dbName, params)
	if err != nil {
		t.Fatalf("failed to set up chain: %v", err)
	}
	t.Cleanup(teardown)
	chain.TstSetCoinbaseMaturity(1)
	return &ruleTestMiner{t: t, chain: chain, tip: params.GenesisBlock}
}

// ruleTestPadding returns a data carrier output which pads transactions above
// the minimum transaction size.
func ruleTestPadding(tag byte) *wire.TxOut {
	script := append([]byte{txscript.OP_RETURN, 60}, make([]byte, 60)...)
	script[2] = tag
	return wire.NewTxOut(0, script, wire.TokenData{})
}

// coinbase returns a coinbase transaction for the given height paying the
// block subsidy to an anyone-can-spend output.
func (m *ruleTestMiner) coinbase(height int32) *wire.MsgTx {
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
		SignatureScript: []byte{0x04, byte(height), byte(height >> 8),
			0, 0, txscript.OP_TRUE},
		Sequence: 0xffffffff,
	})
	cb.AddTxOut(wire.NewTxOut(CalcBlockSubsidy(height, m.chain.chainParams),
		[]byte{txscript.OP_TRUE}, wire.TokenData{}))
	cb.AddTxOut(ruleTestPadding(0))
	return cb
}

// solve recomputes the merkle root of the block and finds a nonce for it.
func (m *ruleTestMiner) solve(blk *wire.MsgBlock, height int32) *bchutil.Block {
	utxs := make([]*bchutil.Tx, 0, len(blk.Transactions))
	for _, tx := range blk.Transactions {
		utxs = append(utxs, bchutil.NewTx(tx))
	}
	merkles := BuildMerkleTreeStore(utxs)
	blk.Header.MerkleRoot = *merkles[len(merkles)-1]
	target := CompactToBig(blk.Header.Bits)
	for nonce := uint32(0); ; nonce++ {
		blk.Header.Nonce = nonce
		hash := blk.Header.BlockHash()
		if HashToBig(&hash).Cmp(target) <= 0 {
			break
		}
	}
	block := bchutil.NewBlock(blk)
	block.SetHeight(height)
	return block
}

// build returns a solved block on the current tip with the given coinbase
// and transactions, which are sorted in canonical order.
func (m *ruleTestMiner) build(cb *wire.MsgTx, txs ...*wire.MsgTx) *bchutil.Block {
	txs = slices.Clone(txs)
	slices.SortFunc(txs, func(a, b *wire.MsgTx) int {
		ha, hb := a.TxHash(), b.TxHash()
		return ha.Compare(&hb)
	})
	return m.buildInOrder(cb, txs...)
}

// buildInOrder returns a solved block on the current tip with the given
// coinbase and transactions in the given order.
func (m *ruleTestMiner) buildInOrder(cb *wire.MsgTx, txs ...*wire.MsgTx) *bchutil.Block {
	height := m.height + 1
	if cb == nil {
		cb = m.coinbase(height)
	}
	ts := m.tip.Header.Timestamp.Add(10 * time.Minute)
	if ts.Before(time.Unix(1700000000, 0)) {
		ts = time.Unix(1700000000, 0)
	}
	bits, err := m.chain.CalcNextRequiredDifficulty(ts)
	if err != nil {
		m.t.Fatalf("failed to calculate difficulty: %v", err)
	}
	blk := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   4,
			PrevBlock: m.tip.BlockHash(),
			Timestamp: ts,
			Bits:      bits,
		},
		Transactions: append([]*wire.MsgTx{cb}, txs...),
	}
	return m.solve(blk, height)
}

// accept processes the block and makes it the miner's tip when it connects.
func (m *ruleTestMiner) accept(block *bchutil.Block) error {
	_, _, err := m.chain.ProcessBlock(block, BFNone)
	if err == nil {
		m.tip = block.MsgBlock()
		m.height = block.Height()
	}
	return err
}

// mustAccept processes the block and fails the test when it is rejected.
func (m *ruleTestMiner) mustAccept(block *bchutil.Block) {
	m.t.Helper()
	if err := m.accept(block); err != nil {
		m.t.Fatalf("block at height %d rejected: %v", block.Height(), err)
	}
}

// ruleTestSpend returns a transaction spending prevOut to an
// anyone-can-spend output.
func ruleTestSpend(prevOut wire.OutPoint, value int64, tag byte) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut, Sequence: 0xffffffff})
	tx.AddTxOut(wire.NewTxOut(value, []byte{txscript.OP_TRUE}, wire.TokenData{}))
	tx.AddTxOut(ruleTestPadding(tag))
	return tx
}
