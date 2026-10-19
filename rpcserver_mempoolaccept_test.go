// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
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
	sigScript, err := txscript.SignatureScript(tx, 0,
		mempoolTestOutputValue, h.pkScript, txscript.SigHashAll, h.key,
		true)
	if err != nil {
		t.Fatalf("unable to sign transaction: %v", err)
	}
	tx.TxIn[0].SignatureScript = sigScript
	return tx
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
	details := "already have transaction " + tx.TxHash().String()
	if result.Allowed || result.RejectReason != "18: "+details ||
		result.RejectDetails != details || result.Fees != nil {

		t.Fatalf("transaction in the pool: unexpected result %+v", result)
	}

	// A transaction with an invalid signature is rejected as invalid.
	tx = h.spend(t, h.outputs[1], 1000)
	tx.TxOut[0].Value--
	result = h.testMempoolAccept(t, tx, false)
	if result.Allowed || result.RejectReason[:4] != "16: " ||
		result.RejectReason[4:] != result.RejectDetails {

		t.Fatalf("invalid signature: unexpected result %+v", result)
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
// the expected error codes.
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

	// A fee above the maximum is rejected unless allowhighfees is set.
	tx = h.spend(t, h.outputs[1], int64(defaultMaxTxFee)+1)
	_, err = h.sendRawTransaction(t, tx, false)
	assertCode("high fee", err, btcjson.ErrRPCTxRejected)
	want := "TX rejected: absurdly-high-fee, 10000001 > 10000000"
	if err.(*btcjson.RPCError).Message != want {
		t.Fatalf("high fee: got message %q, want %q", err, want)
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

	// A transaction with unknown inputs is a transaction error.
	unknown := wire.OutPoint{Hash: h.outputs[0].Hash, Index: 100}
	_, err = h.sendRawTransaction(t, h.spend(t, unknown, 1000), false)
	assertCode("missing inputs", err, btcjson.ErrRPCTxError)

	// A transaction with unspent outputs in the chain is already in it.
	_, err = h.sendRawTransaction(t, h.funding, false)
	assertCode("in chain", err, btcjson.ErrRPCTxAlreadyInChain)
}
