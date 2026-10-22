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

// chainSyncMgr is an rpcserverSyncManager whose SubmitBlock processes the
// block on a chain.
type chainSyncMgr struct {
	rpcserverSyncManager
	chain *blockchain.BlockChain
}

func (s *chainSyncMgr) SubmitBlock(block *bchutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
	_, isOrphan, err := s.chain.ProcessBlock(block, flags)
	return isOrphan, err
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
	chain, _ := newTestChainDB(t)
	return chain
}

// newTestChainDB returns a fresh regtest chain along with its database.
func newTestChainDB(t *testing.T) (*blockchain.BlockChain, database.DB) {
	t.Helper()
	return newTestChainDBParams(t, &chaincfg.RegressionNetParams)
}

// newTestChainDBParams returns a fresh chain with the given parameters, which
// must use the regtest network, along with its database.
func newTestChainDBParams(t *testing.T, params *chaincfg.Params) (*blockchain.BlockChain, database.DB) {
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
		ChainParams:        params,
		TimeSource:         blockchain.NewMedianTime(),
		UtxoCacheMaxSize:   16 * 1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	return chain, db
}

// solvedBlock returns a solved block on the chain's tip whose coinbase pays
// delta satoshis more than allowed.  A non-nil prev replaces the parent.
func solvedBlock(t *testing.T, chain *blockchain.BlockChain, delta int64, prev *chainhash.Hash) *bchutil.Block {
	t.Helper()
	return solvedBlockWith(t, chain, func(msg *wire.MsgBlock) {
		msg.Transactions[0].TxOut[0].Value += delta
		if prev != nil {
			msg.Header.PrevBlock = *prev
		}
	})
}

// solvedBlockWith returns a solved block made from a template on the chain's
// tip, which change modifies first.
func solvedBlockWith(t *testing.T, chain *blockchain.BlockChain, change func(*wire.MsgBlock)) *bchutil.Block {
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
	change(msg)
	merkles := blockchain.BuildMerkleTreeStore(
		bchutil.NewBlock(msg).Transactions())
	msg.Header.MerkleRoot = *merkles[len(merkles)-1]
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

// TestChainErrToGBTErrString ensures rule errors are reported with Bitcoin
// Cash Node's reason strings where it has one for the same check, and that
// an old block version is given as BCHN gives it.
func TestChainErrToGBTErrString(t *testing.T) {
	tests := []struct {
		code blockchain.ErrorCode
		want string
	}{
		{blockchain.ErrNoTransactions, "bad-cb-missing"},
		{blockchain.ErrFirstTxNotCoinbase, "bad-cb-missing"},
		{blockchain.ErrMultipleCoinbases, "bad-tx-coinbase"},
		{blockchain.ErrNoTxInputs, "bad-txns-vin-empty"},
		{blockchain.ErrNoTxOutputs, "bad-txns-vout-empty"},
		{blockchain.ErrTxTooBig, "bad-txns-oversize"},
		{blockchain.ErrTxTooSmall, "bad-txns-undersize"},
		{blockchain.ErrDuplicateTxInputs, "bad-txns-inputs-duplicate"},
		{blockchain.ErrBadTxInput, "bad-txns-prevout-null"},
		{blockchain.ErrMissingTxOut, "bad-txns-inputs-missingorspent"},
		{blockchain.ErrSpentTxOut, "bad-txns-inputs-missingorspent"},
		{blockchain.ErrUnfinalizedTx, "bad-txns-nonfinal"},
		{blockchain.ErrOverwriteTx, "bad-txns-BIP30"},
		{blockchain.ErrImmatureSpend, "bad-txns-premature-spend-of-coinbase"},
		{blockchain.ErrSpendTooHigh, "bad-txns-in-belowout"},
		{blockchain.ErrBadCoinbaseValue, "bad-cb-amount"},
		{blockchain.ErrBadFees, "bad-txns-accumulated-fee-outofrange"},
		{blockchain.ErrTooManySigChecks, "blk-bad-inputs"},
		{blockchain.ErrScriptMalformed, "blk-bad-inputs"},
		{blockchain.ErrScriptValidation, "blk-bad-inputs"},
		{blockchain.ErrInvalidTxOrder, "tx-ordering"},
		{blockchain.ErrBadCheckpoint, "checkpoint mismatch"},
		{blockchain.ErrForkTooOld, "bad-fork-prior-to-checkpoint"},
		{blockchain.ErrForkBeforeFinalized, "bad-fork-prior-finalized"},
		{blockchain.ErrTxTooManySigChecks, "blk-bad-inputs"},
		{blockchain.ErrTxOutValueNegative, "bad-txns-vout-negative"},
		{blockchain.ErrTxOutValueTooHigh, "bad-txns-vout-toolarge"},
		{blockchain.ErrTxOutTotalTooHigh, "bad-txns-txouttotal-toolarge"},
		{blockchain.ErrTxInValueOutOfRange, "bad-txns-inputvalues-outofrange"},
	}
	for _, test := range tests {
		err := blockchain.RuleError{ErrorCode: test.code, Description: "x"}
		if got := chainErrToGBTErrString(err, 4); got != test.want {
			t.Errorf("%v: got %q, want %q", test.code, got, test.want)
		}
	}

	for version, want := range map[int32]string{
		1:  "bad-version(0x00000001)",
		3:  "bad-version(0x00000003)",
		-1: "bad-version(0xffffffff)",
	} {
		err := blockchain.RuleError{ErrorCode: blockchain.ErrBlockVersionTooOld}
		if got := chainErrToGBTErrString(err, version); got != want {
			t.Errorf("version %d: got %q, want %q", version, got, want)
		}
	}
}

// TestSubmitBlockSideChainInconclusive ensures a valid block that ends up on a
// side chain is reported as inconclusive, as Bitcoin Cash Node does, rather
// than as accepted, while a block that becomes the tip is accepted.
func TestSubmitBlockSideChainInconclusive(t *testing.T) {
	quietRPCLog(t)
	chain := newTestChain(t)
	s := &rpcServer{cfg: rpcserverConfig{
		SyncMgr: &chainSyncMgr{chain: chain},
		Chain:   chain,
	}}
	submit := func(block *bchutil.Block) (interface{}, error) {
		return handleSubmitBlock(s, &btcjson.SubmitBlockCmd{
			HexBlock: blockHex(t, block),
		}, nil)
	}

	tip := solvedBlock(t, chain, 0, nil)
	if res, err := submit(tip); err != nil || res != nil {
		t.Fatalf("block on the tip: got %v, %v, want nil", res, err)
	}

	// A second block on the same parent loses to the first one.
	side := solvedBlock(t, chain, 0, chaincfg.RegressionNetParams.GenesisHash)
	if res, err := submit(side); err != nil || res != "inconclusive" {
		t.Fatalf("block on a side chain: got %v, %v, want \"inconclusive\"",
			res, err)
	}
}

// TestSubmitBlockOutputValueReasons ensures a block holding an output value
// out of range is refused with Bitcoin Cash Node's reason for the check it
// fails: an output below zero, one above the money supply, or outputs that
// together are above it.
func TestSubmitBlockOutputValueReasons(t *testing.T) {
	tests := []struct {
		name   string
		values []int64
		want   string
	}{
		{"negative", []int64{-1}, "bad-txns-vout-negative"},
		{"too large", []int64{bchutil.MaxSatoshi + 1}, "bad-txns-vout-toolarge"},
		{"total too large", []int64{bchutil.MaxSatoshi, 1},
			"bad-txns-txouttotal-toolarge"},
	}
	quietRPCLog(t)
	for _, test := range tests {
		chain := newTestChain(t)
		s := &rpcServer{cfg: rpcserverConfig{
			SyncMgr: &chainSyncMgr{chain: chain},
			Chain:   chain,
		}}
		block := solvedBlockWith(t, chain, func(msg *wire.MsgBlock) {
			coinbase := msg.Transactions[0]
			script := coinbase.TxOut[0].PkScript
			coinbase.TxOut = nil
			for _, value := range test.values {
				coinbase.AddTxOut(wire.NewTxOut(value, script,
					wire.TokenData{}))
			}
		})
		res, err := handleSubmitBlock(s, &btcjson.SubmitBlockCmd{
			HexBlock: blockHex(t, block),
		}, nil)
		if err != nil || res != test.want {
			t.Errorf("%s: got %v, %v, want %q", test.name, res, err,
				test.want)
		}
	}
}

// TestOldBlockVersionReason ensures submitblock and a block proposal refuse a
// block whose version is too old with Bitcoin Cash Node's reason, which gives
// the version.
func TestOldBlockVersionReason(t *testing.T) {
	quietRPCLog(t)
	params := chaincfg.RegressionNetParams
	params.BIP0066Height = 1
	chain, _ := newTestChainDBParams(t, &params)
	s := &rpcServer{cfg: rpcserverConfig{
		SyncMgr: &chainSyncMgr{chain: chain},
		Chain:   chain,
	}}
	block := solvedBlockWith(t, chain, func(msg *wire.MsgBlock) {
		msg.Header.Version = 2
	})
	data := blockHex(t, block)

	const want = "bad-version(0x00000002)"
	res, err := handleGetBlockTemplateProposal(s,
		&btcjson.TemplateRequest{Data: data})
	if err != nil || res != want {
		t.Errorf("proposal: got %v, %v, want %q", res, err, want)
	}
	res, err = handleSubmitBlock(s, &btcjson.SubmitBlockCmd{HexBlock: data},
		nil)
	if err != nil || res != want {
		t.Errorf("submitblock: got %v, %v, want %q", res, err, want)
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
		{"orphan", orphan, "prev-blk-not-found",
			"inconclusive-not-best-prevblk"},
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

// TestGBTReasonPrevBlockNotBest ensures the reason for a template that does
// not build on the tip uses the BIP22 spelling.
func TestGBTReasonPrevBlockNotBest(t *testing.T) {
	err := blockchain.RuleError{ErrorCode: blockchain.ErrPrevBlockNotBest}
	if got := chainErrToGBTErrString(err, 4); got != "inconclusive-not-best-prevblk" {
		t.Fatalf("got %q", got)
	}
}
