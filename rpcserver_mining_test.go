// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
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

func submitBlock(t *testing.T, sm *submitSyncMgr, hexBlock string) (interface{}, error) {
	t.Helper()
	quietRPCLog(t)
	s := &rpcServer{cfg: rpcserverConfig{SyncMgr: sm}}
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
	_, err := handleGetBlockTemplateProposal(&rpcServer{},
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
