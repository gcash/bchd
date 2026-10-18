// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// quietRPCLog disables the RPC logger for the test, since the log rotator is
// not set up under test.
func quietRPCLog(t *testing.T) {
	t.Helper()
	old := rpcsLog
	rpcsLog = bchlog.Disabled
	t.Cleanup(func() { rpcsLog = old })
}

// submitSyncMgr is an rpcserverSyncManager whose SubmitBlock returns fixed
// results.
type submitSyncMgr struct {
	rpcserverSyncManager
	isOrphan bool
	err      error
	called   bool
}

func (s *submitSyncMgr) SubmitBlock(*bchutil.Block, blockchain.BehaviorFlags) (bool, error) {
	s.called = true
	return s.isOrphan, s.err
}

// submitBlockHex returns a hex encoded block with only a coinbase, optionally
// followed by trailing bytes.
func submitBlockHex(t *testing.T, trailing bool) string {
	t.Helper()
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex),
		SignatureScript: []byte{0x51, 0x51},
		Sequence:        wire.MaxTxInSequenceNum,
	})
	cb.AddTxOut(wire.NewTxOut(0, []byte{0x51}, wire.TokenData{}))
	var blk wire.MsgBlock
	if err := blk.AddTransaction(cb); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := blk.Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if trailing {
		b = append(b, 0xde, 0xad)
	}
	return hex.EncodeToString(b)
}

// noTxSource is a mining.TxSource without transactions.
type noTxSource struct{}

func (noTxSource) LastUpdated() time.Time               { return time.Time{} }
func (noTxSource) MiningDescs() []*mining.TxDesc        { return nil }
func (noTxSource) HaveTransaction(*chainhash.Hash) bool { return false }

// newTestChain returns a fresh regtest chain.  The chain and mining loggers
// are disabled for the test, since the log rotator is not set up.
func newTestChain(t *testing.T) *blockchain.BlockChain {
	t.Helper()
	blockchain.UseLogger(bchlog.Disabled)
	mining.UseLogger(bchlog.Disabled)
	t.Cleanup(func() {
		blockchain.UseLogger(chanLog)
		mining.UseLogger(minrLog)
	})
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &chaincfg.RegressionNetParams,
		TimeSource:         blockchain.NewMedianTime(),
		UtxoCacheMaxSize:   16 * 1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

// solvedBlock returns a solved block on the chain's tip whose coinbase pays
// delta satoshis more than allowed.  A non-nil prev replaces the parent.
func solvedBlock(t *testing.T, chain *blockchain.BlockChain, delta int64, prev *chainhash.Hash) *bchutil.Block {
	t.Helper()
	timeSource := blockchain.NewMedianTime()
	gen := mining.NewBlkTmplGenerator(&mining.Policy{BlockMaxSize: 1000000},
		&chaincfg.RegressionNetParams, noTxSource{}, chain, timeSource,
		txscript.NewSigCache(10), txscript.NewHashCache(10))
	tmpl, err := gen.NewBlockTemplate(nil)
	if err != nil {
		t.Fatal(err)
	}
	msg := tmpl.Block
	msg.Transactions[0].TxOut[0].Value += delta
	merkles := blockchain.BuildMerkleTreeStore(
		bchutil.NewBlock(msg).Transactions())
	msg.Header.MerkleRoot = *merkles[len(merkles)-1]
	if prev != nil {
		msg.Header.PrevBlock = *prev
	}
	target := blockchain.CompactToBig(msg.Header.Bits)
	for {
		hash := msg.Header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			break
		}
		msg.Header.Nonce++
	}
	return bchutil.NewBlock(msg)
}

func blockHex(t *testing.T, block *bchutil.Block) string {
	t.Helper()
	b, err := block.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func submitBlock(t *testing.T, sm *submitSyncMgr, hexBlock string) (interface{}, error) {
	t.Helper()
	quietRPCLog(t)
	s := &rpcServer{cfg: rpcserverConfig{SyncMgr: sm, Chain: newTestChain(t)}}
	return handleSubmitBlock(s, &btcjson.SubmitBlockCmd{HexBlock: hexBlock},
		nil)
}

// TestSubmitBlockRejectsTrailingBytes ensures a block followed by extra data
// is not processed, since the extra data would be stored and served with it.
func TestSubmitBlockRejectsTrailingBytes(t *testing.T) {
	sm := &submitSyncMgr{}
	_, err := submitBlock(t, sm, submitBlockHex(t, true))
	if sm.called {
		t.Fatal("block with trailing bytes was processed")
	}
	var rpcErr *btcjson.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != btcjson.ErrRPCDeserialization {
		t.Fatalf("expected a deserialization error, got %v", err)
	}
}

// TestGetBlockTemplateProposalRejectsTrailingBytes ensures a proposal
// followed by extra data is rejected.
func TestGetBlockTemplateProposalRejectsTrailingBytes(t *testing.T) {
	quietRPCLog(t)
	s := &rpcServer{cfg: rpcserverConfig{Chain: newTestChain(t)}}
	_, err := handleGetBlockTemplateProposal(s,
		&btcjson.TemplateRequest{Data: submitBlockHex(t, true)})
	var rpcErr *btcjson.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != btcjson.ErrRPCDeserialization {
		t.Fatalf("expected a deserialization error, got %v", err)
	}
}

// TestSubmitBlockOrphanNotAccepted ensures a block with an unknown parent is
// not reported as accepted, which a null result means.
func TestSubmitBlockOrphanNotAccepted(t *testing.T) {
	res, err := submitBlock(t, &submitSyncMgr{isOrphan: true},
		submitBlockHex(t, false))
	if err != nil || res != "prev-blk-not-found" {
		t.Fatalf("got %v, %v, want \"prev-blk-not-found\"", res, err)
	}
}

// TestSubmitBlockUsesBIP22Reasons ensures rule violations are reported with
// BIP22 reason strings and other errors as RPC errors.
func TestSubmitBlockUsesBIP22Reasons(t *testing.T) {
	res, err := submitBlock(t, &submitSyncMgr{err: blockchain.RuleError{
		ErrorCode:   blockchain.ErrDuplicateBlock,
		Description: "already have block",
	}}, submitBlockHex(t, false))
	if err != nil || res != "duplicate" {
		t.Fatalf("got %v, %v, want \"duplicate\"", res, err)
	}

	_, err = submitBlock(t, &submitSyncMgr{err: errors.New("disk failure")},
		submitBlockHex(t, false))
	var rpcErr *btcjson.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != btcjson.ErrRPCVerify {
		t.Fatalf("expected a verify error, got %v", err)
	}
}

// TestGBTDependsCoversLaterParents ensures a transaction's depends list
// includes parents that sort after it in the block, as they can under the
// canonical transaction order.
func TestGBTDependsCoversLaterParents(t *testing.T) {
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex),
		SignatureScript: []byte{0x51, 0x51},
	})
	cb.AddTxOut(wire.NewTxOut(0, []byte{0x51}, wire.TokenData{}))

	// Find a parent and child where the child sorts first.
	var parent, child *wire.MsgTx
	for i := uint32(0); ; i++ {
		parent = wire.NewMsgTx(1)
		parent.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{1}, i), nil))
		parent.AddTxOut(wire.NewTxOut(1000, []byte{0x51}, wire.TokenData{}))
		child = wire.NewMsgTx(1)
		ph := parent.TxHash()
		child.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&ph, 0), nil))
		child.AddTxOut(wire.NewTxOut(900, []byte{0x51}, wire.TokenData{}))
		ch := child.TxHash()
		if ch.Compare(&ph) < 0 {
			break
		}
	}
	var blk wire.MsgBlock
	blk.Header.Timestamp = time.Unix(time.Now().Unix(), 0)
	for _, tx := range []*wire.MsgTx{cb, child, parent} {
		if err := blk.AddTransaction(tx); err != nil {
			t.Fatal(err)
		}
	}
	state := newGbtWorkState(blockchain.NewMedianTime())
	state.prevHash = &chainhash.Hash{}
	state.template = &mining.BlockTemplate{
		Block:     &blk,
		Fees:      []int64{-200, 100, 100},
		SigChecks: []int64{0, 0, 0},
	}
	res, err := state.blockTemplateResult(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if deps := res.Transactions[0].Depends; len(deps) != 1 || deps[0] != 2 {
		t.Fatalf("child depends on %v, want its parent at index 2", deps)
	}
	if deps := res.Transactions[1].Depends; len(deps) != 0 {
		t.Fatalf("parent depends on %v, want nothing", deps)
	}
}

// TestKnownBlockReasons ensures blocks the chain already has are reported by
// submitblock and proposals with the reasons Bitcoin Cash Node uses.
func TestKnownBlockReasons(t *testing.T) {
	quietRPCLog(t)
	chain := newTestChain(t)
	s := &rpcServer{cfg: rpcserverConfig{Chain: chain,
		SyncMgr: &submitSyncMgr{err: blockchain.RuleError{
			ErrorCode: blockchain.ErrDuplicateBlock,
		}}}}

	// Three blocks on genesis: one paying too much, one on the main
	// chain and one on a side chain.
	bad := solvedBlock(t, chain, 1, nil)
	main := solvedBlock(t, chain, 0, nil)
	side := solvedBlock(t, chain, -1, nil)
	if _, _, err := chain.ProcessBlock(bad, blockchain.BFNone); err == nil {
		t.Fatal("block paying too much was accepted")
	}
	for _, block := range []*bchutil.Block{main, side} {
		_, _, err := chain.ProcessBlock(block, blockchain.BFFastAdd)
		if err != nil {
			t.Fatal(err)
		}
	}
	orphan := solvedBlock(t, chain, 0, &chainhash.Hash{1})
	_, isOrphan, err := chain.ProcessBlock(orphan, blockchain.BFNone)
	if err != nil || !isOrphan {
		t.Fatalf("orphan: orphan %v, err %v", isOrphan, err)
	}

	for _, test := range []struct {
		name     string
		block    *bchutil.Block
		submit   string
		proposal string
	}{
		{"invalid", bad, "duplicate-invalid", "duplicate-invalid"},
		{"main chain", main, "duplicate", "duplicate"},
		{"side chain", side, "duplicate", "duplicate-inconclusive"},
		{"orphan", orphan, "prev-blk-not-found", "bad-prevblk"},
	} {
		data := blockHex(t, test.block)
		res, err := handleSubmitBlock(s,
			&btcjson.SubmitBlockCmd{HexBlock: data}, nil)
		if err != nil || res != test.submit {
			t.Errorf("%s: submitblock: got %v, %v, want %q", test.name,
				res, err, test.submit)
		}
		res, err = handleGetBlockTemplateProposal(s,
			&btcjson.TemplateRequest{Data: data})
		if err != nil || res != test.proposal {
			t.Errorf("%s: proposal: got %v, %v, want %q", test.name,
				res, err, test.proposal)
		}
	}
}
