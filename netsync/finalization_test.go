// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// mineBlock returns a block with only a coinbase that builds on prev at the
// passed height and has valid proof of work for the regression test network.
// Blocks with a different extra value differ.
func mineBlock(t *testing.T, params *chaincfg.Params, prev *wire.MsgBlock,
	height int32, extra int64) *bchutil.Block {

	t.Helper()

	cbScript, err := txscript.NewScriptBuilder().AddInt64(int64(height)).
		AddInt64(extra).Script()
	if err != nil {
		t.Fatal(err)
	}
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex),
		Sequence:        wire.MaxTxInSequenceNum,
		SignatureScript: cbScript,
	})
	cb.AddTxOut(wire.NewTxOut(blockchain.CalcBlockSubsidy(height, params),
		[]byte{txscript.OP_TRUE}, wire.TokenData{}))
	pad := append([]byte{txscript.OP_RETURN, txscript.OP_DATA_60},
		make([]byte, 60)...)
	cb.AddTxOut(wire.NewTxOut(0, pad, wire.TokenData{}))

	merkles := blockchain.BuildMerkleTreeStore([]*bchutil.Tx{bchutil.NewTx(cb)})
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    4,
			PrevBlock:  prev.BlockHash(),
			MerkleRoot: *merkles[len(merkles)-1],
			Timestamp:  prev.Header.Timestamp.Add(time.Minute),
			Bits:       params.PowLimitBits,
		},
		Transactions: []*wire.MsgTx{cb},
	}
	target := blockchain.CompactToBig(block.Header.Bits)
	for {
		hash := block.Header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			break
		}
		block.Header.Nonce++
	}
	return bchutil.NewBlock(block)
}

// TestForkBeforeFinalizedDisconnects ensures a peer that sends a block forking
// the chain below the finalized block is disconnected.
func TestForkBeforeFinalizedDisconnects(t *testing.T) {
	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		ExcessiveBlockSize: 32000000,
		MaxReorgDepth:      1,
		FinalizationDelay:  time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	sm, err := New(&Config{Chain: chain, ChainParams: &params, MaxPeers: 8})
	if err != nil {
		t.Fatal(err)
	}

	// Blocks 1 to 3 finalize block 2.
	blocks := []*wire.MsgBlock{params.GenesisBlock}
	for h := int32(1); h <= 3; h++ {
		block := mineBlock(t, &params, blocks[h-1], h, 0)
		if _, _, err := chain.ProcessBlock(block, blockchain.BFNone); err != nil {
			t.Fatalf("block %d: %v", h, err)
		}
		blocks = append(blocks, block.MsgBlock())
	}

	peer := peerpkg.NewInboundPeer(&peerpkg.Config{ChainParams: &params})
	t.Cleanup(peer.Disconnect)
	state := &peerSyncState{
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}
	sm.peerStates[peer] = state

	// A requested block forking from block 1.
	fork := mineBlock(t, &params, blocks[1], 2, 1)
	state.requestedBlocks[*fork.Hash()] = struct{}{}
	sm.handleBlockMsg(&blockMsg{block: fork, peer: peer})
	if have, _ := chain.HaveBlock(fork.Hash()); have {
		t.Fatal("block forking below the finalized block was stored")
	}
	if !isDisconnected(peer) {
		t.Fatal("peer sending a block forking below the finalized " +
			"block was not disconnected")
	}
}
