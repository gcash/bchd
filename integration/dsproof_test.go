// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// dsProofList calls getdsprooflist on a node.
func dsProofList(t *testing.T, h *rpctest.Harness, params ...json.RawMessage) []string {
	t.Helper()
	reply, err := h.Node.RawRequest("getdsprooflist", params)
	if err != nil {
		t.Fatalf("getdsprooflist: %v", err)
	}
	var ids []string
	if err := json.Unmarshal(reply, &ids); err != nil {
		t.Fatalf("getdsprooflist: %v", err)
	}
	return ids
}

// waitForProofs waits until getdsprooflist on h returns n proofs.
func waitForProofs(t *testing.T, h *rpctest.Harness, n int) []string {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		ids := dsProofList(t, h)
		if len(ids) == n {
			return ids
		}
		if time.Now().After(deadline) {
			t.Fatalf("got %d proofs, want %d", len(ids), n)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestDSProofRelay creates a double spend on one node and checks that the
// proof it creates reaches a second node, which validates it, stores it for
// its own copy of the transaction and notifies its websocket client.
func TestDSProofRelay(t *testing.T) {
	var mu sync.Mutex
	var notified []json.RawMessage
	handlers := &rpcclient.NotificationHandlers{
		OnUnknownNotification: func(method string, params []json.RawMessage) {
			if method == btcjson.DSProofNtfnMethod && len(params) == 1 {
				mu.Lock()
				notified = append(notified, params[0])
				mu.Unlock()
			}
		},
	}

	h1, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h1.TearDown()
	// A single mature coinbase makes both transactions spend it.
	if err := h1.SetUp(true, 1); err != nil {
		t.Fatal(err)
	}
	h2, err := rpctest.New(&chaincfg.SimNetParams, handlers, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer h2.TearDown()
	if err := h2.SetUp(false, 0); err != nil {
		t.Fatal(err)
	}
	if err := rpctest.ConnectNode(h2, h1); err != nil {
		t.Fatal(err)
	}
	if err := rpctest.JoinNodes([]*rpctest.Harness{h1, h2}, rpctest.Blocks); err != nil {
		t.Fatal(err)
	}
	if _, err := h2.Node.RawRequest("notifydsproofs", nil); err != nil {
		t.Fatalf("notifydsproofs: %v", err)
	}

	addr, err := h1.NewAddress()
	if err != nil {
		t.Fatal(err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}
	tx1, err := h1.CreateTransaction([]*wire.TxOut{wire.NewTxOut(1e8, pkScript, wire.TokenData{})}, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	h1.UnlockOutputs(tx1.TxIn)
	tx2, err := h1.CreateTransaction([]*wire.TxOut{wire.NewTxOut(2e8, pkScript, wire.TokenData{})}, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if tx1.TxIn[0].PreviousOutPoint != tx2.TxIn[0].PreviousOutPoint {
		t.Fatal("transactions do not spend the same output")
	}

	if _, err := h1.Node.SendRawTransaction(tx1, true); err != nil {
		t.Fatal(err)
	}
	if err := rpctest.JoinNodes([]*rpctest.Harness{h1, h2}, rpctest.Mempools); err != nil {
		t.Fatal(err)
	}
	if _, err := h1.Node.SendRawTransaction(tx2, true); err == nil {
		t.Fatal("double spend accepted")
	}

	ids1 := waitForProofs(t, h1, 1)
	ids2 := waitForProofs(t, h2, 1)
	if ids1[0] != ids2[0] {
		t.Fatalf("proof ids differ: %v %v", ids1, ids2)
	}

	// Both nodes return the same proof.
	param := json.RawMessage(`"` + ids1[0] + `"`)
	r1, err := h1.Node.RawRequest("getdsproof", []json.RawMessage{param, json.RawMessage("3")})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := h2.Node.RawRequest("getdsproof", []json.RawMessage{param, json.RawMessage("3")})
	if err != nil {
		t.Fatal(err)
	}
	if string(r1) != string(r2) {
		t.Fatalf("getdsproof differs:\n%s\n%s", r1, r2)
	}
	var proof btcjson.GetDSProofResult
	if err := json.Unmarshal(r2, &proof); err != nil {
		t.Fatal(err)
	}
	if *proof.TxID != tx1.TxHash().String() || len(proof.Spenders) != 2 {
		t.Fatalf("unexpected proof %s", r2)
	}

	// The second node's websocket client was notified.
	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		n := len(notified)
		mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no dsproof notification")
		}
		time.Sleep(100 * time.Millisecond)
	}
	mu.Lock()
	var ntfn btcjson.GetDSProofResult
	err = json.Unmarshal(notified[0], &ntfn)
	mu.Unlock()
	if err != nil || ntfn.DSPID != ids1[0] {
		t.Fatalf("notification %+v: %v", ntfn, err)
	}

	// Once mined, the proof is an orphan on both nodes.
	if _, err := h1.Node.Generate(1); err != nil {
		t.Fatal(err)
	}
	if err := rpctest.JoinNodes([]*rpctest.Harness{h1, h2}, rpctest.Blocks); err != nil {
		t.Fatal(err)
	}
	for _, h := range []*rpctest.Harness{h1, h2} {
		waitForProofs(t, h, 0)
		all := dsProofList(t, h, json.RawMessage("0"), json.RawMessage("true"))
		if len(all) != 1 || all[0] != ids1[0] {
			t.Fatalf("orphans %v", all)
		}
	}

	// The score of an unrelated transaction.
	tx3, err := h1.CreateTransaction([]*wire.TxOut{wire.NewTxOut(1e8, pkScript, wire.TokenData{})}, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := h1.Node.SendRawTransaction(tx3, true)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := h1.Node.RawRequest("getdsproofscore",
		[]json.RawMessage{json.RawMessage(`"` + hash.String() + `"`)})
	if err != nil || string(reply) != "1" {
		t.Fatalf("getdsproofscore: %s %v", reply, err)
	}
}
