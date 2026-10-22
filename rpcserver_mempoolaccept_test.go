// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// mempoolTestOutputValue is the value of each output the test chain holds.
const mempoolTestOutputValue = bchutil.SatoshiPerBitcoin

// relayRecorder is a connection manager that records the transactions it
// is asked to relay or rebroadcast.
type relayRecorder struct {
	rpcserverConnManager
	relayed     []*mempool.TxDesc
	rebroadcast []*wire.InvVect
}

// RelayTransactions records the relayed transactions.
func (r *relayRecorder) RelayTransactions(txns []*mempool.TxDesc) {
	r.relayed = append(r.relayed, txns...)
}

// AddRebroadcastInventory records the inventory to rebroadcast.
func (r *relayRecorder) AddRebroadcastInventory(iv *wire.InvVect, _ interface{}) {
	r.rebroadcast = append(r.rebroadcast, iv)
}

// mempoolTestHarness is an RPC server with a mempool on top of a chain that
// holds outputs paying to a single key.
type mempoolTestHarness struct {
	server   *rpcServer
	connMgr  *relayRecorder
	funding  *wire.MsgTx
	key      *bchec.PrivateKey
	pkScript []byte
	outputs  []wire.OutPoint
}

// newMempoolTestHarness returns a harness whose chain holds numOutputs
// outputs of mempoolTestOutputValue.  opts can change the mempool config.
func newMempoolTestHarness(t *testing.T, numOutputs int, opts ...func(*mempool.Config)) *mempoolTestHarness {
	t.Helper()

	params := &chaincfg.MainNetParams
	key, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatalf("unable to create key: %v", err)
	}
	addr, err := bchutil.NewAddressPubKeyHash(
		bchutil.Hash160(key.PubKey().SerializeCompressed()), params)
	if err != nil {
		t.Fatalf("unable to create address: %v", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("unable to create script: %v", err)
	}

	// The outputs come from a transaction mined at the current height.
	const height = 1000000
	fundingTx := wire.NewMsgTx(wire.TxVersion)
	fundingTx.AddTxIn(&wire.TxIn{})
	for range numOutputs {
		fundingTx.AddTxOut(wire.NewTxOut(mempoolTestOutputValue,
			pkScript, wire.TokenData{}))
	}
	utxos := blockchain.NewUtxoViewpoint()
	utxos.AddTxOuts(bchutil.NewTx(fundingTx), height)
	outputs := make([]wire.OutPoint, numOutputs)
	for i := range outputs {
		outputs[i] = wire.OutPoint{Hash: fundingTx.TxHash(), Index: uint32(i)}
	}

	fetchUtxoView := func(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
		view := blockchain.NewUtxoViewpoint()
		prevOut := wire.OutPoint{Hash: *tx.Hash()}
		for i := range tx.MsgTx().TxOut {
			prevOut.Index = uint32(i)
			view.Entries()[prevOut] = utxos.LookupEntry(prevOut).Clone()
		}
		for _, txIn := range tx.MsgTx().TxIn {
			entry := utxos.LookupEntry(txIn.PreviousOutPoint)
			view.Entries()[txIn.PreviousOutPoint] = entry.Clone()
		}
		return view, nil
	}
	poolCfg := &mempool.Config{
		Policy: mempool.Policy{
			MaxOrphanTxs:    5,
			MaxOrphanTxSize: 1000,
			LimitSigChecks:  true,
			MinRelayTxFee:   mempool.DefaultMinRelayTxFee,
			MaxTxVersion:    2,
		},
		ChainParams:    params,
		FetchUtxoView:  fetchUtxoView,
		BestHeight:     func() int32 { return height },
		MedianTimePast: time.Now,
		CalcSequenceLock: func(*bchutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return &blockchain.SequenceLock{Seconds: -1, BlockHeight: -1}, nil
		},
	}
	for _, opt := range opts {
		opt(poolCfg)
	}
	pool := mempool.New(poolCfg)

	connMgr := &relayRecorder{}
	s := &rpcServer{
		cfg: rpcserverConfig{
			ConnMgr:   connMgr,
			TxMemPool: pool,
			MaxTxFee:  defaultMaxTxFee,
		},
		gbtWorkState: newGbtWorkState(nil),
	}

	// Websocket notifications are dropped.
	s.ntfnMgr = newWsNotificationManager(s)
	close(s.ntfnMgr.quit)

	return &mempoolTestHarness{
		server:   s,
		connMgr:  connMgr,
		funding:  fundingTx,
		key:      key,
		pkScript: pkScript,
		outputs:  outputs,
	}
}

// spend returns a signed transaction that spends prevOut, assumed to hold
// mempoolTestOutputValue, with the given fee.
func (h *mempoolTestHarness) spend(t *testing.T, prevOut wire.OutPoint, fee int64) *wire.MsgTx {
	t.Helper()

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: prevOut,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(wire.NewTxOut(mempoolTestOutputValue-fee, h.pkScript,
		wire.TokenData{}))
	h.sign(t, tx)
	return tx
}

// sign signs the first input of tx, assumed to spend an output of
// mempoolTestOutputValue paid to the harness key.
func (h *mempoolTestHarness) sign(t *testing.T, tx *wire.MsgTx) {
	t.Helper()

	sigScript, err := txscript.SignatureScript(tx, 0,
		mempoolTestOutputValue, h.pkScript, txscript.SigHashAll, h.key,
		true)
	if err != nil {
		t.Fatalf("unable to sign transaction: %v", err)
	}
	tx.TxIn[0].SignatureScript = sigScript
}

// txHex returns the serialized transaction in hex.
func txHex(t *testing.T, tx *wire.MsgTx) string {
	t.Helper()

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatalf("unable to serialize transaction: %v", err)
	}
	return hex.EncodeToString(buf.Bytes())
}

// testMempoolAccept calls testmempoolaccept for a single transaction.
func (h *mempoolTestHarness) testMempoolAccept(t *testing.T, tx *wire.MsgTx, allowHighFees bool) btcjson.TestMempoolAcceptResult {
	t.Helper()

	cmd := btcjson.NewTestMempoolAcceptCmd([]string{txHex(t, tx)},
		&allowHighFees)
	reply, err := handleTestMempoolAccept(h.server, cmd, nil)
	if err != nil {
		t.Fatalf("testmempoolaccept: unexpected error: %v", err)
	}
	results := reply.([]btcjson.TestMempoolAcceptResult)
	if len(results) != 1 || results[0].Txid != tx.TxHash().String() {
		t.Fatalf("testmempoolaccept: unexpected results %+v", results)
	}
	return results[0]
}

// TestHandleTestMempoolAccept ensures testmempoolaccept reports whether the
// mempool would accept a transaction, without adding it.
func TestHandleTestMempoolAccept(t *testing.T) {
	t.Parallel()

	h := newMempoolTestHarness(t, 5)
	pool := h.server.cfg.TxMemPool

	// A transaction paying 1000 satoshis is allowed.
	tx := h.spend(t, h.outputs[0], 1000)
	result := h.testMempoolAccept(t, tx, false)
	size := int64(tx.SerializeSize())
	want := btcjson.TestMempoolAcceptResult{
		Txid:    tx.TxHash().String(),
		Allowed: true,
		Size:    size,
		Vsize:   size,
		Fees: &btcjson.TestMempoolAcceptFees{
			Base:             0.00001,
			EffectiveFeeRate: bchutil.Amount(1000 * 1000 / size).ToBCH(),
		},
	}
	gotJSON, _ := json.Marshal(result)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("allowed transaction: got %s, want %s", gotJSON, wantJSON)
	}
	if pool.Count() != 0 {
		t.Fatalf("testmempoolaccept added the transaction to the pool")
	}

	// Once the transaction is in the pool, it is rejected as a duplicate.
	_, err := pool.ProcessTransaction(bchutil.NewTx(tx), false, 0)
	if err != nil {
		t.Fatalf("ProcessTransaction: unexpected error: %v", err)
	}
	result = h.testMempoolAccept(t, tx, false)
	details := "txn-already-in-mempool, already have transaction " +
		tx.TxHash().String()
	if result.Allowed || result.RejectReason != "18: txn-already-in-mempool" ||
		result.RejectDetails != details || result.Fees != nil {

		t.Errorf("transaction in the pool: unexpected result %+v", result)
	}

	// A transaction with an invalid signature is rejected as invalid.
	tx = h.spend(t, h.outputs[1], 1000)
	tx.TxOut[0].Value--
	result = h.testMempoolAccept(t, tx, false)
	reason := "mandatory-script-verify-flag-failed (Script failed)"
	details = reason + ", failed to validate input " +
		tx.TxHash().String() + ":0 which references output " +
		h.outputs[1].String() + " - signature not empty on failed checksig"
	if result.Allowed || result.RejectReason != "16: "+reason ||
		!strings.HasPrefix(result.RejectDetails, details) {

		t.Errorf("invalid signature: unexpected result %+v", result)
	}

	// A transaction with unknown inputs reports missing inputs.
	unknown := wire.OutPoint{Hash: h.outputs[0].Hash, Index: 100}
	result = h.testMempoolAccept(t, h.spend(t, unknown, 1000), false)
	if result.Allowed || result.RejectReason != "missing-inputs" ||
		result.RejectDetails != "" {

		t.Fatalf("missing inputs: unexpected result %+v", result)
	}

	// A fee above the maximum is only allowed with allowhighfees.
	tx = h.spend(t, h.outputs[2], int64(defaultMaxTxFee)+1)
	result = h.testMempoolAccept(t, tx, false)
	if result.Allowed || result.RejectReason != "256: absurdly-high-fee" ||
		result.RejectDetails != "absurdly-high-fee, 10000001 > 10000000" {

		t.Fatalf("high fee: unexpected result %+v", result)
	}
	result = h.testMempoolAccept(t, tx, true)
	if !result.Allowed || result.Fees.Base != 0.10000001 {
		t.Fatalf("high fee allowed: unexpected result %+v", result)
	}
	if pool.Count() != 1 {
		t.Fatalf("testmempoolaccept changed the pool")
	}
}

// TestHandleTestMempoolAcceptReasons ensures testmempoolaccept gives the
// reject code and BCHN's short reason of each rejection, and the reason
// followed by bchd's description as the details.
func TestHandleTestMempoolAcceptReasons(t *testing.T) {
	t.Parallel()

	h := newMempoolTestHarness(t, 10)
	pooled := h.spend(t, h.outputs[0], 1000)
	_, err := h.server.cfg.TxMemPool.ProcessTransaction(bchutil.NewTx(pooled),
		false, 0)
	if err != nil {
		t.Fatalf("ProcessTransaction: unexpected error: %v", err)
	}

	conflict := h.spend(t, h.outputs[0], 2000)
	unknown := h.spend(t, wire.OutPoint{Hash: h.outputs[0].Hash, Index: 100},
		1000)
	lowFee := h.spend(t, h.outputs[1], 100)
	dust := h.spend(t, h.outputs[2], 1000)
	dust.TxOut[0].Value -= 545
	dust.AddTxOut(wire.NewTxOut(545, h.pkScript, wire.TokenData{}))
	h.sign(t, dust)
	nonStd := h.spend(t, h.outputs[3], 1000)
	nonStd.TxOut[0].PkScript = bytes.Repeat([]byte{txscript.OP_1}, 202)
	h.sign(t, nonStd)
	badSig := h.spend(t, h.outputs[4], 1000)
	badSig.TxOut[0].Value--
	highFee := h.spend(t, h.outputs[5], int64(defaultMaxTxFee)+1)
	coinbase := wire.NewMsgTx(wire.TxVersion)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{txscript.OP_1, txscript.OP_1},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	coinbase.AddTxOut(wire.NewTxOut(1000, h.pkScript, wire.TokenData{}))
	nonFinal := h.spend(t, h.outputs[6], 1000)
	nonFinal.TxIn[0].Sequence = 0
	nonFinal.LockTime = 1000100
	h.sign(t, nonFinal)
	overspend := h.spend(t, h.outputs[7], -1)
	duplicate := h.spend(t, h.outputs[8], 1000)
	duplicate.AddTxIn(&wire.TxIn{
		PreviousOutPoint: h.outputs[8],
		Sequence:         wire.MaxTxInSequenceNum,
	})

	txid := func(tx *wire.MsgTx) string { return tx.TxHash().String() }
	scriptReason := "mandatory-script-verify-flag-failed (Script failed)"
	tests := []struct {
		name    string
		tx      *wire.MsgTx
		reason  string
		details string
	}{
		{"in the pool", pooled, "18: txn-already-in-mempool",
			"txn-already-in-mempool, already have transaction " +
				txid(pooled)},
		{"pool conflict", conflict, "18: txn-mempool-conflict",
			"txn-mempool-conflict, output " + h.outputs[0].String() +
				" already spent by transaction " + txid(pooled) +
				" in the memory pool"},
		{"in the chain", h.funding, "18: txn-already-known",
			"txn-already-known, transaction already exists"},
		{"missing inputs", unknown, "missing-inputs", ""},
		{"fee below the minimum", lowFee, "66: min relay fee not met",
			fmt.Sprintf("min relay fee not met, transaction %s has 100 "+
				"fees which is under the required amount of %d (min "+
				"relay fee not met)", txid(lowFee),
				lowFee.SerializeSize())},
		{"dust", dust, "65: dust",
			"dust, transaction " + txid(dust) + " is not standard: " +
				"transaction output 1: payment of 545 is dust"},
		{"nonstandard output script", nonStd, "64: scriptpubkey",
			"scriptpubkey, transaction " + txid(nonStd) + " is not " +
				"standard: transaction output 0: non-standard script form"},
		{"failed script", badSig, "16: " + scriptReason,
			fmt.Sprintf("%s, failed to validate input %s:0 which "+
				"references output %v - signature not empty on failed "+
				"checksig (input script bytes %x, prev output script "+
				"bytes %x)", scriptReason, txid(badSig), h.outputs[4],
				badSig.TxIn[0].SignatureScript, h.pkScript)},
		{"fee above the maximum", highFee, "256: absurdly-high-fee",
			"absurdly-high-fee, 10000001 > 10000000"},
		{"coinbase", coinbase, "16: bad-tx-coinbase",
			"bad-tx-coinbase, transaction " + txid(coinbase) +
				" is an individual coinbase"},
		{"not final", nonFinal, "64: bad-txns-nonfinal",
			"bad-txns-nonfinal, transaction " + txid(nonFinal) +
				" is not standard: transaction is not finalized " +
				"(bad-txns-nonfinal)"},
		{"outputs above the inputs", overspend, "16: bad-txns-in-belowout",
			"bad-txns-in-belowout, total value of all transaction " +
				"inputs for transaction " + txid(overspend) + " is " +
				"100000000 which is less than the amount spent of " +
				"100000001"},
		{"duplicate inputs", duplicate, "16: bad-txns-inputs-duplicate",
			"bad-txns-inputs-duplicate, transaction contains duplicate " +
				"inputs"},
	}
	for _, test := range tests {
		result := h.testMempoolAccept(t, test.tx, false)
		if result.Allowed || result.RejectReason != test.reason ||
			result.RejectDetails != test.details {

			t.Errorf("%s: got reject-reason %q, reject-details %q, "+
				"want %q, %q", test.name, result.RejectReason,
				result.RejectDetails, test.reason, test.details)
		}
	}

	// Sequence locks that the next block does not meet.
	h = newMempoolTestHarness(t, 1, func(cfg *mempool.Config) {
		cfg.CalcSequenceLock = func(*bchutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return &blockchain.SequenceLock{Seconds: -1,
				BlockHeight: 2000000}, nil
		}
	})
	result := h.testMempoolAccept(t, h.spend(t, h.outputs[0], 1000), false)
	details := "non-BIP68-final, transaction's sequence locks on inputs " +
		"not met"
	if result.Allowed || result.RejectReason != "64: non-BIP68-final" ||
		result.RejectDetails != details {

		t.Errorf("sequence locks: got reject-reason %q, reject-details "+
			"%q, want %q, %q", result.RejectReason, result.RejectDetails,
			"64: non-BIP68-final", details)
	}
}

// TestVirtualSize ensures each signature check counts as bytesPerSigCheck
// bytes once they outweigh the size.
func TestVirtualSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		size      int64
		sigChecks uint32
		want      int64
	}{
		{size: 200, sigChecks: 1, want: 200},
		{size: 200, sigChecks: 4, want: 200},
		{size: 200, sigChecks: 5, want: 250},
	}
	for _, test := range tests {
		got := virtualSize(test.size, test.sigChecks)
		if got != test.want {
			t.Errorf("virtualSize(%d, %d) = %d, want %d", test.size,
				test.sigChecks, got, test.want)
		}
	}
}

// TestHandleTestMempoolAcceptParams ensures testmempoolaccept rejects
// anything but a single valid transaction.
func TestHandleTestMempoolAcceptParams(t *testing.T) {
	t.Parallel()

	h := newMempoolTestHarness(t, 1)
	valid := txHex(t, h.spend(t, h.outputs[0], 1000))
	tests := []struct {
		name   string
		rawTxs []string
		code   btcjson.RPCErrorCode
	}{
		{"no transactions", []string{}, btcjson.ErrRPCInvalidParameter},
		{"two transactions", []string{valid, valid}, btcjson.ErrRPCInvalidParameter},
		{"invalid hex", []string{"zz"}, btcjson.ErrRPCDecodeHexString},
		{"truncated", []string{valid[:20]}, btcjson.ErrRPCDeserialization},
		{"trailing bytes", []string{valid + "00"}, btcjson.ErrRPCDeserialization},
	}
	for _, test := range tests {
		cmd := btcjson.NewTestMempoolAcceptCmd(test.rawTxs, nil)
		_, err := handleTestMempoolAccept(h.server, cmd, nil)
		rpcErr, ok := err.(*btcjson.RPCError)
		if !ok || rpcErr.Code != test.code {
			t.Errorf("%s: got error %v, want code %d", test.name, err,
				test.code)
		}
	}
}

// sendRawTransaction calls sendrawtransaction.
func (h *mempoolTestHarness) sendRawTransaction(t *testing.T, tx *wire.MsgTx, allowHighFees bool) (string, error) {
	t.Helper()

	cmd := btcjson.NewSendRawTransactionCmd(txHex(t, tx), &allowHighFees)
	reply, err := handleSendRawTransaction(h.server, cmd, nil)
	if err != nil {
		return "", err
	}
	return reply.(string), nil
}

// TestHandleSendRawTransaction ensures sendrawtransaction returns the hash of
// a transaction already in the mempool, enforces the maximum fee and returns
// the expected error codes and messages.
func TestHandleSendRawTransaction(t *testing.T) {
	t.Parallel()

	h := newMempoolTestHarness(t, 5)
	pool := h.server.cfg.TxMemPool

	assertCode := func(name string, err error, code btcjson.RPCErrorCode) {
		t.Helper()
		rpcErr, ok := err.(*btcjson.RPCError)
		if !ok || rpcErr.Code != code {
			t.Fatalf("%s: got error %v, want code %d", name, err, code)
		}
	}

	// A new transaction is accepted, relayed and kept for rebroadcast.
	tx := h.spend(t, h.outputs[0], 1000)
	txid, err := h.sendRawTransaction(t, tx, false)
	if err != nil || txid != tx.TxHash().String() {
		t.Fatalf("new transaction: got %q, error %v", txid, err)
	}
	if len(h.connMgr.relayed) != 1 || len(h.connMgr.rebroadcast) != 1 {
		t.Fatalf("new transaction: relayed %d, rebroadcast %d",
			len(h.connMgr.relayed), len(h.connMgr.rebroadcast))
	}

	// Sending it again returns its hash and relays it again.
	txid, err = h.sendRawTransaction(t, tx, false)
	if err != nil || txid != tx.TxHash().String() {
		t.Fatalf("resubmitted transaction: got %q, error %v", txid, err)
	}
	if len(h.connMgr.relayed) != 2 ||
		*h.connMgr.relayed[1].Tx.Hash() != tx.TxHash() {

		t.Fatalf("resubmitted transaction was not relayed")
	}

	// A rejected transaction gets BCHN's reason, the description and the
	// reject code.
	_, err = h.sendRawTransaction(t, h.spend(t, h.outputs[0], 2000), false)
	assertCode("conflict", err, btcjson.ErrRPCTxRejected)
	want := "txn-mempool-conflict, output " + h.outputs[0].String() +
		" already spent by transaction " + tx.TxHash().String() +
		" in the memory pool (code 18)"
	if err.(*btcjson.RPCError).Message != want {
		t.Errorf("conflict: got message %q, want %q", err, want)
	}

	// A fee above the maximum is rejected unless allowhighfees is set.
	tx = h.spend(t, h.outputs[1], int64(defaultMaxTxFee)+1)
	_, err = h.sendRawTransaction(t, tx, false)
	assertCode("high fee", err, btcjson.ErrRPCTxRejected)
	want = "absurdly-high-fee, 10000001 > 10000000 (code 256)"
	if err.(*btcjson.RPCError).Message != want {
		t.Errorf("high fee: got message %q, want %q", err, want)
	}
	if pool.IsTransactionInPool(bchutil.NewTx(tx).Hash()) {
		t.Fatalf("high fee: transaction added to the pool")
	}
	txid, err = h.sendRawTransaction(t, tx, true)
	if err != nil || txid != tx.TxHash().String() {
		t.Fatalf("high fee allowed: got %q, error %v", txid, err)
	}

	// A transaction failing a consensus rule is rejected.
	tx = h.spend(t, h.outputs[2], 1000)
	tx.TxOut[0].Value--
	_, err = h.sendRawTransaction(t, tx, false)
	assertCode("invalid signature", err, btcjson.ErrRPCTxRejected)
	msg := err.(*btcjson.RPCError).Message
	prefix := "mandatory-script-verify-flag-failed (Script failed), " +
		"failed to validate input "
	if !strings.HasPrefix(msg, prefix) ||
		!strings.HasSuffix(msg, ") (code 16)") {

		t.Errorf("invalid signature: unexpected message %q", msg)
	}

	// A transaction with unknown inputs is a transaction error.
	unknown := wire.OutPoint{Hash: h.outputs[0].Hash, Index: 100}
	_, err = h.sendRawTransaction(t, h.spend(t, unknown, 1000), false)
	assertCode("missing inputs", err, btcjson.ErrRPCTxError)
	msg = err.(*btcjson.RPCError).Message
	if !strings.HasPrefix(msg, "TX rejected: orphan transaction ") {
		t.Errorf("missing inputs: unexpected message %q", msg)
	}

	// A transaction with unspent outputs in the chain is already in it.
	_, err = h.sendRawTransaction(t, h.funding, false)
	assertCode("in chain", err, btcjson.ErrRPCTxAlreadyInChain)
	msg = err.(*btcjson.RPCError).Message
	if msg != "TX rejected: transaction already exists" {
		t.Errorf("in chain: unexpected message %q", msg)
	}
}
