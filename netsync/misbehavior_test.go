// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"errors"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// punishment is a ban score increase the sync manager asked for.
type punishment struct {
	peer   *peerpkg.Peer
	score  uint32
	reason string
}

// punishRecorder is a PeerNotifier that records the ban score increases the
// sync manager asks for.
type punishRecorder struct {
	nopNotifier
	punished []punishment
}

func (r *punishRecorder) Misbehaving(peer *peerpkg.Peer, score uint32, reason string) {
	r.punished = append(r.punished, punishment{peer, score, reason})
}

// expect fails the test unless the sync manager asked for one increase of
// the peer's ban score by score for reason since the last call, or for none
// when score is 0.
func (r *punishRecorder) expect(t *testing.T, what string, peer *peerpkg.Peer,
	score uint32, reason string) {

	t.Helper()
	got := r.punished
	r.punished = nil
	if score == 0 {
		if len(got) != 0 {
			t.Errorf("%s: punished %+v, want none", what, got)
		}
		return
	}
	want := punishment{peer, score, reason}
	if len(got) != 1 || got[0] != want {
		t.Errorf("%s: punished %+v, want %d for %q", what, got, score,
			reason)
	}
}

// TestTxMisbehavior ensures the peer that sent a transaction breaking a
// consensus rule gets BCHN's ban score for the rule, with BCHN's reason, while
// one refused by policy, such as a script failing only the standard flags or a
// transaction below the minimum size or with a version out of range, which
// BCHN's memory pool refuses as nonstandard, or for the node's own failure,
// gets none.  A script failing before it runs, for a malformed push, both
// scripts empty or an input script over 10,000 bytes, gets 100, as in BCHN.
// bchd punished no transaction.
func TestTxMisbehavior(t *testing.T) {
	sm, params, _ := unrequestedTestManager(t)
	recorder := &punishRecorder{}
	sm.peerNotifier = recorder
	peer, rejects := rejectTestPeer(t, sm, params)
	makeCurrent(t, sm, params)

	// Outputs paying to a script that fails, to an upgradable NOP, which
	// only the standard flags refuse, to a script that passes, to one with
	// a malformed push and to an empty one.
	funding := wire.NewMsgTx(1)
	funding.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{9}},
	})
	for _, script := range [][]byte{{txscript.OP_FALSE},
		{txscript.OP_NOP10, txscript.OP_TRUE}, {txscript.OP_TRUE},
		{txscript.OP_DATA_2, 1}, {}} {

		funding.AddTxOut(wire.NewTxOut(10000, script, wire.TokenData{}))
	}
	fetchFails := false
	sm.txMemPool = fundedPool(params, funding, &fetchFails)

	pad := wire.NewTxOut(0, append([]byte{txscript.OP_RETURN,
		txscript.OP_DATA_60}, make([]byte, 60)...), wire.TokenData{})
	tx := func(ins []wire.OutPoint, outs ...*wire.TxOut) *wire.MsgTx {
		msg := wire.NewMsgTx(1)
		for _, in := range ins {
			msg.AddTxIn(&wire.TxIn{PreviousOutPoint: in,
				Sequence: wire.MaxTxInSequenceNum})
		}
		for _, out := range outs {
			msg.AddTxOut(out)
		}
		return msg
	}
	spend := func(index uint32) []wire.OutPoint {
		return []wire.OutPoint{{Hash: funding.TxHash(), Index: index}}
	}
	null := wire.OutPoint{Index: wire.MaxPrevOutIndex}
	coinbase := tx([]wire.OutPoint{null}, pad)
	coinbase.TxIn[0].SignatureScript = []byte{txscript.OP_1, txscript.OP_1}
	small := wire.NewTxOut(0, []byte{txscript.OP_TRUE}, wire.TokenData{})
	bigInput := tx(spend(2), pad)
	bigInput.TxIn[0].SignatureScript = append([]byte{txscript.OP_PUSHDATA2,
		0x10, 0x27}, make([]byte, 10000)...)
	version3 := tx(spend(2), pad)
	version3.Version = 3

	for _, test := range []struct {
		name   string
		tx     *wire.MsgTx
		code   wire.RejectCode
		reason string
		score  uint32
	}{
		{"failing script", tx(spend(0), pad), wire.RejectInvalid,
			"mandatory-script-verify-flag-failed (Script evaluated " +
				"without error but finished with a false/empty top " +
				"stack element)", 100},
		{"upgradable NOP", tx(spend(1), pad), wire.RejectNonstandard,
			"non-mandatory-script-verify-flag (NOPx reserved for " +
				"soft-fork upgrades)", 0},
		{"negative output", tx(spend(2), pad, wire.NewTxOut(-1,
			[]byte{txscript.OP_TRUE}, wire.TokenData{})),
			wire.RejectInvalid, "bad-txns-vout-negative", 100},
		{"no inputs", tx(nil, pad), wire.RejectInvalid,
			"bad-txns-vin-empty", 10},
		{"null input", tx(append(spend(2), null), pad),
			wire.RejectInvalid, "bad-txns-prevout-null", 10},
		{"coinbase", coinbase, wire.RejectInvalid, "bad-tx-coinbase", 100},
		{"below the minimum size", tx(spend(2), small),
			wire.RejectNonstandard, "bad-txns-undersize", 0},
		{"malformed push", tx(spend(3), pad), wire.RejectInvalid,
			"mandatory-script-verify-flag-failed (Opcode missing or " +
				"not understood)", 100},
		{"both scripts empty", tx(spend(4), pad), wire.RejectInvalid,
			"mandatory-script-verify-flag-failed (Script evaluated " +
				"without error but finished with a false/empty top " +
				"stack element)", 100},
		{"input script over 10,000 bytes", bigInput, wire.RejectInvalid,
			"mandatory-script-verify-flag-failed (Script is too big)",
			100},
		{"version 3", version3, wire.RejectNonstandard,
			"bad-txns-version", 0},
	} {
		sm.handleTxMsg(&txMsg{tx: bchutil.NewTx(test.tx), peer: peer})
		expectReject(t, rejects, sentReject{test.code, test.tx.TxHash(),
			cutReason(test.reason)})
		recorder.expect(t, test.name, peer, test.score, test.reason)
	}

	fetchFails = true
	sm.handleTxMsg(&txMsg{tx: bchutil.NewTx(tx(spend(2), pad, pad)),
		peer: peer})
	recorder.expect(t, "the node's failure", peer, 0, "")
}

// TestTxNotPunishedWhileSyncing ensures a transaction refused for breaking a
// consensus rule still gets its reject, but adds nothing to the ban score of
// the peer that sent it, while the sync manager is not current: while its
// chain is behind, and while its sync peer is ahead of it.  Until then the
// pool checks transactions under the rules of the old tip, so a valid one
// using a later upgrade, here a loop of the May 2026 upgrade, fails as
// breaking a consensus rule, and bchd fetches the transactions its sync peer
// announces, which BCHN does not during the initial sync.  Before, the sync
// peer got 100 for each, and was banned at the second.
func TestTxNotPunishedWhileSyncing(t *testing.T) {
	sm, params, _ := unrequestedTestManager(t)
	recorder := &punishRecorder{}
	sm.peerNotifier = recorder
	peer, rejects := rejectTestPeer(t, sm, params)

	// Outputs paying to a P2SH script whose redeem script runs a loop,
	// checked by a pool under the rules before the May 2026 upgrade, as at
	// the old tip of a node that is syncing.
	redeem := []byte{txscript.OP_BEGIN, txscript.OP_1, txscript.OP_UNTIL,
		txscript.OP_1}
	p2sh, err := txscript.NewScriptBuilder().AddOp(txscript.OP_HASH160).
		AddData(bchutil.Hash160(redeem)).AddOp(txscript.OP_EQUAL).Script()
	if err != nil {
		t.Fatal(err)
	}
	sigScript, err := txscript.NewScriptBuilder().AddData(redeem).Script()
	if err != nil {
		t.Fatal(err)
	}
	funding := wire.NewMsgTx(1)
	funding.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{9}},
	})
	for range 4 {
		funding.AddTxOut(wire.NewTxOut(10000, p2sh, wire.TokenData{}))
	}
	params.Upgrade12ActivationTime = uint64(time.Now().Add(time.Hour).Unix())
	sm.txMemPool = fundedPool(params, funding, new(bool))

	// spend returns the peer's transaction spending the funding output at
	// index.
	spend := func(index uint32) *bchutil.Tx {
		msg := wire.NewMsgTx(1)
		msg.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: funding.TxHash(),
				Index: index},
			SignatureScript: sigScript,
			Sequence:        wire.MaxTxInSequenceNum,
		})
		msg.AddTxOut(wire.NewTxOut(0, append([]byte{txscript.OP_RETURN,
			txscript.OP_DATA_60}, make([]byte, 60)...), wire.TokenData{}))
		return bchutil.NewTx(msg)
	}
	// refused fails the test unless the peer's spend of the output at index
	// gets the reject for breaking a consensus rule and adds score to the
	// peer's ban score.
	refused := func(what string, index uint32, score uint32) {
		t.Helper()
		tx := spend(index)
		sm.handleTxMsg(&txMsg{tx: tx, peer: peer})
		const reason = "mandatory-script-verify-flag-failed (Opcode " +
			"missing or not understood)"
		expectReject(t, rejects, sentReject{wire.RejectInvalid,
			*tx.Hash(), reason})
		recorder.expect(t, what, peer, score, reason)
	}

	// The chain is behind: its tip is the genesis block.
	refused("chain behind", 0, 0)

	// The chain is current, but its sync peer is ahead of it.
	makeCurrent(t, sm, params)
	sm.syncPeer = peer
	peer.UpdateLastBlockHeight(2)
	refused("sync peer ahead", 1, 0)

	// Once the sync manager is current, the same failure gets BCHN's
	// score.  This pool keeps the old rules, but a current node's pool
	// checks transactions under the rules of its tip, so a failure there
	// breaks the rules in force.
	sm.syncPeer = nil
	refused("current", 2, 100)

	// Under the May 2026 rules the spend is valid.
	params.Upgrade12ActivationTime = 0
	valid := spend(3)
	sm.handleTxMsg(&txMsg{tx: valid, peer: peer})
	if !sm.txMemPool.IsTransactionInPool(valid.Hash()) {
		t.Fatal("the spend was not accepted under the May 2026 rules")
	}
}

// TestBlockMisbehavior ensures the peer that sent a block it was asked for,
// in a block message, gets BCHN's ban score for the rule the block breaks,
// with BCHN's reason: a rule checked before the block is connected for any
// block whose parent is known, and one checked when it is connected for a
// block that extended the tip.  None is given for a rule BCHN gives no score
// for, for a block rebuilt from a compact block, an unrequested one, an orphan,
// or a failure connecting a block that did not extend the tip, which can be an
// ancestor's.  bchd punished no block.
func TestBlockMisbehavior(t *testing.T) {
	sm, params, _ := unrequestedTestManager(t)
	params.BIP0065Height = 3
	recorder := &punishRecorder{}
	sm.peerNotifier = recorder
	peer, rejects := rejectTestPeer(t, sm, params)
	peer.SetAllowDirectBlockRelay(true)
	send := func(block *bchutil.Block, requested, compact bool) {
		if requested {
			sm.peerStates[peer].requestedBlocks[*block.Hash()] =
				struct{}{}
		}
		sm.handleBlockMsg(&blockMsg{block: block, peer: peer,
			compact: compact})
	}
	// expectRejected fails the test unless block got a reject with the
	// code and reason, and the peer the score for it.
	expectRejected := func(what string, block *bchutil.Block,
		code wire.RejectCode, reason string, score uint32) {

		t.Helper()
		expectReject(t, rejects, sentReject{code, *block.Hash(), reason})
		recorder.expect(t, what, peer, score, reason)
	}
	// rebuild returns block, which it changes, with its merkle root and
	// proof of work made right again.
	rebuild := func(block *bchutil.Block, change func(*wire.MsgBlock)) *bchutil.Block {
		msg := block.MsgBlock()
		change(msg)
		merkles := blockchain.BuildMerkleTreeStore(
			bchutil.NewBlock(msg).Transactions())
		msg.Header.MerkleRoot = *merkles[len(merkles)-1]
		solveBlock(msg)
		return bchutil.NewBlock(msg)
	}
	overpay := func(msg *wire.MsgBlock) { msg.Transactions[0].TxOut[0].Value++ }

	genesis := params.GenesisBlock
	a1 := mineBlock(t, params, genesis, 1, 0)
	send(a1, true, false)
	recorder.expect(t, "valid block", peer, 0, "")

	// Rules checked before connecting, for a block whose parent is known.
	badRoot := mineBlock(t, params, a1.MsgBlock(), 2, 0).MsgBlock()
	badRoot.Header.MerkleRoot = chainhash.Hash{1}
	solveBlock(badRoot)
	badRootBlock := bchutil.NewBlock(badRoot)
	send(badRootBlock, true, false)
	expectRejected("bad merkle root", badRootBlock, wire.RejectInvalid,
		"bad-txnmrklroot", 100)
	send(badRootBlock, true, true)
	expectRejected("compact block", badRootBlock, wire.RejectInvalid,
		"bad-txnmrklroot", 0)
	send(badRootBlock, false, false)
	expectRejected("unrequested block", badRootBlock, wire.RejectInvalid,
		"bad-txnmrklroot", 0)

	highHash := mineBlock(t, params, a1.MsgBlock(), 2, 1).MsgBlock()
	target := blockchain.CompactToBig(highHash.Header.Bits)
	for {
		hash := highHash.Header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) > 0 {
			break
		}
		highHash.Header.Nonce++
	}
	highHashBlock := bchutil.NewBlock(highHash)
	send(highHashBlock, true, false)
	expectRejected("high hash", highHashBlock, wire.RejectInvalid,
		"high-hash", 50)

	// An old version is refused with no score.
	a2 := mineBlock(t, params, a1.MsgBlock(), 2, 2)
	send(a2, true, false)
	old := rebuild(mineBlock(t, params, a2.MsgBlock(), 3, 0),
		func(msg *wire.MsgBlock) { msg.Header.Version = 3 })
	send(old, true, false)
	expectRejected("old version", old, wire.RejectObsolete,
		"bad-version(0x00000003)", 0)

	// An orphan is not punished.
	orphanRoot := mineBlock(t, params, old.MsgBlock(), 4, 0).MsgBlock()
	orphanRoot.Header.MerkleRoot = chainhash.Hash{1}
	solveBlock(orphanRoot)
	orphan := bchutil.NewBlock(orphanRoot)
	send(orphan, true, false)
	expectRejected("orphan", orphan, wire.RejectInvalid, "bad-txnmrklroot",
		0)

	// A failure connecting a block that extends the tip.
	overpaying := rebuild(mineBlock(t, params, a2.MsgBlock(), 3, 1), overpay)
	send(overpaying, true, false)
	expectRejected("coinbase paying too much", overpaying,
		wire.RejectInvalid, "bad-cb-amount", 100)

	// A child of a block known to be invalid.
	child := mineBlock(t, params, overpaying.MsgBlock(), 4, 0)
	send(child, true, false)
	expectRejected("child of an invalid block", child, wire.RejectInvalid,
		"bad-prevblk", 100)

	// A side chain block failing to connect only shows when the next
	// block makes it the best chain, and the failure is the side chain
	// block's, so neither is punished.
	b3 := rebuild(mineBlock(t, params, a2.MsgBlock(), 3, 2), overpay)
	a3 := mineBlock(t, params, a2.MsgBlock(), 3, 3)
	send(a3, true, false)
	send(b3, true, false)
	recorder.expect(t, "side chain block", peer, 0, "")
	b4 := mineBlock(t, params, b3.MsgBlock(), 4, 1)
	send(b4, true, false)
	expectRejected("block on the failing side chain", b4,
		wire.RejectInvalid, "bad-cb-amount", 0)
	if known, _, invalid := sm.chain.BlockValidity(b3.Hash()); !known ||
		!invalid {

		t.Fatal("the side chain block's failure did not show")
	}
}

// cutReason returns reason cut to the 111 bytes a reject carries.
func cutReason(reason string) string {
	if len(reason) > 111 {
		return reason[:111]
	}
	return reason
}

// fundedPool returns a memory pool accepting nonstandard transactions, at
// height 1000 and the time now, whose transactions can spend only the outputs
// of funding, and whose output lookups fail while fetchFails is true.
func fundedPool(params *chaincfg.Params, funding *wire.MsgTx,
	fetchFails *bool) *mempool.TxPool {

	fundingTx := bchutil.NewTx(funding)
	return mempool.New(&mempool.Config{
		Policy:      mempool.Policy{MaxTxVersion: 2, AcceptNonStd: true},
		ChainParams: params,
		FetchUtxoView: func(*bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			if *fetchFails {
				return nil, errors.New("injected failure")
			}
			view := blockchain.NewUtxoViewpoint()
			view.AddTxOuts(fundingTx, 1)
			return view, nil
		},
		BestHeight:     func() int32 { return 1000 },
		MedianTimePast: time.Now,
		CalcSequenceLock: func(*bchutil.Tx,
			*blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {

			return &blockchain.SequenceLock{Seconds: -1,
				BlockHeight: -1}, nil
		},
	})
}

// makeCurrent connects a block made now to the genesis block of the sync
// manager's chain, so that, with no sync peer ahead of it, the sync manager
// is current.
func makeCurrent(t *testing.T, sm *SyncManager, params *chaincfg.Params) {
	t.Helper()
	block := mineBlock(t, params, params.GenesisBlock, 1, 0).MsgBlock()
	block.Header.Timestamp = time.Unix(time.Now().Unix(), 0)
	solveBlock(block)
	_, _, err := sm.chain.ProcessBlock(bchutil.NewBlock(block),
		blockchain.BFNone)
	if err != nil {
		t.Fatal(err)
	}
	if !sm.current() {
		t.Fatal("the sync manager is not current")
	}
}
