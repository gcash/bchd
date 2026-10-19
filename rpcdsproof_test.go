// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// dsProofTestHarness is an RPC server whose memory pool holds a transaction
// with a double spend proof and a child of that transaction.
type dsProofTestHarness struct {
	*mempoolTestHarness
	tx1, tx2, child *wire.MsgTx
	dspID           string
}

func newDSProofTestHarness(t *testing.T) *dsProofTestHarness {
	t.Helper()
	h := newMempoolTestHarness(t, 2)
	pool := h.server.cfg.TxMemPool
	tx1 := h.spend(t, h.outputs[0], 1000)
	tx2 := h.spend(t, h.outputs[0], 2000)
	if _, err := pool.ProcessTransaction(bchutil.NewTx(tx1), false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.ProcessTransaction(bchutil.NewTx(tx2), false, 0); err == nil {
		t.Fatal("double spend accepted")
	}
	child := wire.NewMsgTx(wire.TxVersion)
	child.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Hash: tx1.TxHash()},
		Sequence: wire.MaxTxInSequenceNum})
	child.AddTxOut(wire.NewTxOut(tx1.TxOut[0].Value-1000, h.pkScript, wire.TokenData{}))
	signTestInput(t, h, child, tx1.TxOut[0].Value)
	if _, err := pool.ProcessTransaction(bchutil.NewTx(child), false, 0); err != nil {
		t.Fatal(err)
	}
	list := pool.DSProofs(false)
	if len(list) != 1 {
		t.Fatalf("%d proofs", len(list))
	}
	return &dsProofTestHarness{h, tx1, tx2, child, list[0].Hash.String()}
}

func call(t *testing.T, h *dsProofTestHarness, method string, params ...interface{}) (string, error) {
	t.Helper()
	cmd, err := btcjson.NewCmd(method, params...)
	if err != nil {
		t.Fatal(err)
	}
	// Round trip through JSON as the server receives it.
	marshalled, err := btcjson.MarshalCmd("1.0", 1, cmd)
	if err != nil {
		t.Fatal(err)
	}
	var req btcjson.Request
	if err := json.Unmarshal(marshalled, &req); err != nil {
		t.Fatal(err)
	}
	cmd, err = btcjson.UnmarshalCmd(&req)
	if err != nil {
		t.Fatal(err)
	}
	handler := rpcHandlers[method]
	reply, err := handler(h.server, cmd, nil)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), nil
}

// TestDSProofRPC checks the output of getdsproof, getdsprooflist and
// getdsproofscore, including the order of the keys.
func TestDSProofRPC(t *testing.T) {
	h := newDSProofTestHarness(t)
	tx1 := h.tx1.TxHash().String()
	child := h.child.TxHash().String()
	prev := h.outputs[0].Hash.String()
	descendants := []string{tx1, child}
	if descendants[0] > descendants[1] {
		descendants[0], descendants[1] = descendants[1], descendants[0]
	}
	descJSON := fmt.Sprintf(`["%s","%s"]`, descendants[0], descendants[1])

	out, err := call(t, h, "getdsprooflist")
	if err != nil || out != `["`+h.dspID+`"]` {
		t.Fatalf("getdsprooflist: %s %v", out, err)
	}

	// Verbosity 1: the serialized proof and the txid.
	out, _ = call(t, h, "getdsprooflist", 1)
	var list []struct {
		Hex  string `json:"hex"`
		TxID string `json:"txid"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil || len(list) != 1 || list[0].TxID != tx1 {
		t.Fatalf("getdsprooflist 1: %s", out)
	}
	if !regexp.MustCompile(`^\[\{"hex":"[0-9a-f]+","txid":"[0-9a-f]{64}"\}\]$`).MatchString(out) {
		t.Fatalf("getdsprooflist 1 keys: %s", out)
	}
	raw, _ := hex.DecodeString(list[0].Hex)
	var proof wire.MsgDSProof
	if err := proof.BchDecode(bytes.NewReader(raw), 0, wire.BaseEncoding); err != nil {
		t.Fatal(err)
	}
	if proof.ProofHash().String() != h.dspID {
		t.Fatal("hex does not hash to the dspid")
	}

	want := fmt.Sprintf(`{"dspid":"%s","txid":"%s","outpoint":{"txid":"%s","vout":0},"descendants":%s}`,
		h.dspID, tx1, prev, descJSON)
	for _, params := range [][]interface{}{{h.dspID}, {h.dspID, 2}, {tx1, 2, false}} {
		out, err = call(t, h, "getdsproof", params...)
		if err != nil || out != want {
			t.Fatalf("getdsproof %v:\n got %s\nwant %s (%v)", params, out, want, err)
		}
	}

	// A recursive lookup by txid includes the path, here just the
	// transaction itself.
	out, _ = call(t, h, "getdsproof", tx1)
	wantSelf := fmt.Sprintf(`{"dspid":"%s","txid":"%s","outpoint":{"txid":"%s","vout":0},"path":["%s"],"descendants":%s}`,
		h.dspID, tx1, prev, tx1, descJSON)
	if out != wantSelf {
		t.Fatalf("getdsproof by txid:\n got %s\nwant %s", out, wantSelf)
	}

	// By outpoint.
	out, _ = call(t, h, "getdsproof", fmt.Sprintf(`{"txid":"%s","vout":0}`, prev))
	if out != want {
		t.Fatalf("getdsproof outpoint: %s", out)
	}

	// Verbosity 0 is 1 without the descendants.
	want1 := fmt.Sprintf(`{"hex":"%s","txid":"%s","descendants":%s}`, list[0].Hex, tx1, descJSON)
	if out, _ = call(t, h, "getdsproof", h.dspID, 1); out != want1 {
		t.Fatalf("getdsproof 1: %s", out)
	}
	want0 := fmt.Sprintf(`{"hex":"%s","txid":"%s"}`, list[0].Hex, tx1)
	if out, _ = call(t, h, "getdsproof", h.dspID, 0); out != want0 {
		t.Fatalf("getdsproof 0: %s", out)
	}

	// A child finds its parent's proof, with the path, unless the search
	// is not recursive.
	out, _ = call(t, h, "getdsproof", child)
	wantPath := fmt.Sprintf(`{"dspid":"%s","txid":"%s","outpoint":{"txid":"%s","vout":0},"path":["%s","%s"],"descendants":%s}`,
		h.dspID, tx1, prev, child, tx1, descJSON)
	if out != wantPath {
		t.Fatalf("getdsproof child:\n got %s\nwant %s", out, wantPath)
	}
	if out, _ = call(t, h, "getdsproof", child, 2, false); out != "null" {
		t.Fatalf("getdsproof child, not recursive: %s", out)
	}
	if out, _ = call(t, h, "getdsproof", h.outputs[1].Hash.String()); out != "null" {
		t.Fatalf("getdsproof unknown: %s", out)
	}

	// Verbosity 3 adds the spenders.
	out, _ = call(t, h, "getdsproof", h.dspID, 3)
	var full struct {
		Spenders []btcjson.DSProofSpender `json:"spenders"`
	}
	if err := json.Unmarshal([]byte(out), &full); err != nil || len(full.Spenders) != 2 {
		t.Fatalf("getdsproof 3: %s", out)
	}
	spenderRe := regexp.MustCompile(`"spenders":\[\{"txversion":1,"sequence":4294967295,"locktime":0,` +
		`"hashprevoutputs":"[0-9a-f]{64}","hashsequence":"[0-9a-f]{64}","hashoutputs":"[0-9a-f]{64}",` +
		`"pushdata":\{"asm":"[0-9a-f]{128}\[ALL\|FORKID\]","hex":"41[0-9a-f]{128}41"\}\},`)
	if !spenderRe.MatchString(out) {
		t.Fatalf("getdsproof 3 spenders: %s", out)
	}
	for i, s := range []*wire.DSProofSpender{&proof.FirstSpender, &proof.DoubleSpender} {
		if full.Spenders[i].HashOutputs != s.HashOutputs.String() ||
			full.Spenders[i].PushData.Hex != "41"+hex.EncodeToString(s.PushData[0]) {
			t.Fatalf("spender %d differs", i)
		}
	}

	// The scores of the transaction and its child.
	for _, tx := range []string{tx1, child} {
		if out, _ = call(t, h, "getdsproofscore", tx); out != "0" {
			t.Fatalf("getdsproofscore %s: %s", tx, out)
		}
	}
	if _, err := call(t, h, "getdsproofscore", h.outputs[1].Hash.String()); err == nil ||
		err.(*btcjson.RPCError).Code != btcjson.ErrRPCInvalidAddressOrKey {
		t.Fatalf("getdsproofscore unknown: %v", err)
	}

	// Errors.
	if _, err := call(t, h, "getdsproof", h.dspID, 4); err == nil || err.(*btcjson.RPCError).Message != "Bad verbosity" {
		t.Fatalf("verbosity 4: %v", err)
	}
	if _, err := call(t, h, "getdsprooflist", -1); err == nil {
		t.Fatal("verbosity -1 accepted")
	}
	for _, bad := range []string{"00", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		for _, method := range []string{"getdsproof", "getdsproofscore"} {
			if _, err := call(t, h, method, bad); err == nil ||
				err.(*btcjson.RPCError).Code != btcjson.ErrRPCInvalidParameter {
				t.Fatalf("%s %s: %v", method, bad, err)
			}
		}
	}
	if _, err := call(t, h, "getdsproof", fmt.Sprintf(`{"txid":"%s","vout":-1}`, prev)); err == nil ||
		err.(*btcjson.RPCError).Message != "vout must be positive" {
		t.Fatalf("negative vout: %v", err)
	}

	// Once the transaction leaves the pool the proof is an orphan with a
	// null txid and no descendants.
	h.server.cfg.TxMemPool.RemoveTransaction(bchutil.NewTx(h.tx1), true)
	if out, _ = call(t, h, "getdsprooflist"); out != "[]" {
		t.Fatalf("getdsprooflist after removal: %s", out)
	}
	if out, _ = call(t, h, "getdsprooflist", 0, true); out != `["`+h.dspID+`"]` {
		t.Fatalf("getdsprooflist with orphans: %s", out)
	}
	wantOrphan := fmt.Sprintf(`{"dspid":"%s","txid":null,"outpoint":{"txid":"%s","vout":0}}`, h.dspID, prev)
	if out, _ = call(t, h, "getdsproof", h.dspID); out != wantOrphan {
		t.Fatalf("orphan:\n got %s\nwant %s", out, wantOrphan)
	}
	if out, _ = call(t, h, "getdsprooflist", 2, true); out != "["+wantOrphan+"]" {
		t.Fatalf("orphan list: %s", out)
	}
}

// signTestInput signs the only input of tx, which spends an output of the
// harness key holding value.
func signTestInput(t *testing.T, h *mempoolTestHarness, tx *wire.MsgTx, value int64) {
	t.Helper()
	sigScript, err := txscript.SignatureScript(tx, 0, value, h.pkScript,
		txscript.SigHashAll, h.key, true)
	if err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].SignatureScript = sigScript
}

// TestDSProofWebsocket checks the notifydsproofs subscription: the proofs
// already in the memory pool are sent on subscribing, new ones as they come,
// and none after stopnotifydsproofs.
func TestDSProofWebsocket(t *testing.T) {
	h := newDSProofTestHarness(t)
	s := h.server
	s.ntfnMgr = newWsNotificationManager(s)
	s.ntfnMgr.Start()
	defer func() {
		s.ntfnMgr.Shutdown()
		s.ntfnMgr.WaitForShutdown()
	}()
	wsc := &wsClient{
		server:   s,
		quit:     make(chan struct{}),
		ntfnChan: make(chan []byte, 10),
	}
	read := func() string {
		t.Helper()
		select {
		case msg := <-wsc.ntfnChan:
			return string(msg)
		case <-time.After(5 * time.Second):
			t.Fatal("no notification")
		}
		return ""
	}
	expectNone := func() {
		t.Helper()
		select {
		case msg := <-wsc.ntfnChan:
			t.Fatalf("unexpected notification %s", msg)
		case <-time.After(200 * time.Millisecond):
		}
	}

	if _, err := handleNotifyDSProofs(wsc, btcjson.NewNotifyDSProofsCmd(nil)); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`{"jsonrpc":"1.0","method":"dsproof","params":[{"dspid":"%s","txid":"%s","outpoint":{"txid":"%s","vout":0}}],"id":null}`,
		h.dspID, h.tx1.TxHash(), h.outputs[0].Hash)
	if msg := read(); msg != want {
		t.Fatalf("existing proof:\n got %s\nwant %s", msg, want)
	}

	// A new proof is sent as it comes.
	pool := h.server.cfg.TxMemPool
	tx3 := h.spend(t, h.outputs[1], 1000)
	tx4 := h.spend(t, h.outputs[1], 2000)
	if _, err := pool.ProcessTransaction(bchutil.NewTx(tx3), false, 0); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.ProcessTransaction(bchutil.NewTx(tx4), false, 0)
	desc, _, _, _ := pool.DSProofByTx(bchutil.NewTx(tx3).Hash(), false)
	if desc == nil {
		t.Fatal("no proof for the second double spend")
	}
	s.NotifyDSProof(desc.Proof, bchutil.NewTx(tx3))
	if msg := read(); !strings.Contains(msg, `"dspid":"`+desc.Hash.String()+`"`) {
		t.Fatalf("new proof notification: %s", msg)
	}

	// Nothing after stopping.
	if _, err := handleStopNotifyDSProofs(wsc, btcjson.NewStopNotifyDSProofsCmd()); err != nil {
		t.Fatal(err)
	}
	s.NotifyDSProof(desc.Proof, bchutil.NewTx(tx3))
	expectNone()

	// Verbosity 1 sends the serialized proofs.
	v := btcjson.DSProofVerbosity(5)
	if _, err := handleNotifyDSProofs(wsc, btcjson.NewNotifyDSProofsCmd(&v)); err == nil {
		t.Fatal("verbosity 5 accepted")
	}
	v = 1
	if _, err := handleNotifyDSProofs(wsc, btcjson.NewNotifyDSProofsCmd(&v)); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if msg := read(); !strings.Contains(msg, `"method":"dsproof","params":[{"hex":"`) {
			t.Fatalf("verbosity 1 notification %s", msg)
		}
	}

	// Limited users may subscribe and unsubscribe.
	for _, method := range []string{"notifydsproofs", "stopnotifydsproofs"} {
		if _, ok := rpcLimited[method]; !ok {
			t.Errorf("%s not available to limited users", method)
		}
	}
}

// TestDSProofRPCDisabled checks that the commands fail with -90 when proofs
// are disabled.
func TestDSProofRPCDisabled(t *testing.T) {
	s := &rpcServer{cfg: rpcserverConfig{
		TxMemPool: mempool.New(&mempool.Config{DisableDSProofs: true}),
	}}
	for _, cmd := range []interface{}{
		btcjson.NewGetDSProofCmd(btcjson.DSProofQuery("00"), nil, nil),
		btcjson.NewGetDSProofListCmd(nil, nil),
		btcjson.NewGetDSProofScoreCmd("00"),
	} {
		var err error
		switch c := cmd.(type) {
		case *btcjson.GetDSProofCmd:
			_, err = handleGetDSProof(s, c, nil)
		case *btcjson.GetDSProofListCmd:
			_, err = handleGetDSProofList(s, c, nil)
		case *btcjson.GetDSProofScoreCmd:
			_, err = handleGetDSProofScore(s, c, nil)
		}
		if rerr, ok := err.(*btcjson.RPCError); !ok || rerr.Code != -90 {
			t.Errorf("%T: got %v", cmd, err)
		}
	}
}

// TestDSProofWebsocketDisabled checks that notifydsproofs fails with -90
// when proofs are disabled.
func TestDSProofWebsocketDisabled(t *testing.T) {
	s := &rpcServer{cfg: rpcserverConfig{
		TxMemPool: mempool.New(&mempool.Config{DisableDSProofs: true}),
	}}
	wsc := &wsClient{server: s, quit: make(chan struct{})}
	_, err := handleNotifyDSProofs(wsc, btcjson.NewNotifyDSProofsCmd(nil))
	if rerr, ok := err.(*btcjson.RPCError); !ok || rerr.Code != -90 {
		t.Fatalf("got %v", err)
	}
}
