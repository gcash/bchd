// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// txHexWithTrailingBytes returns the hex encoding of a minimal but otherwise
// valid transaction with an extra byte appended to it.  The result decodes to a
// complete transaction without consuming all of the provided input, which is
// exactly the case both raw transaction RPCs must reject.
func txHexWithTrailingBytes(t *testing.T) string {
	t.Helper()

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil))
	tx.AddTxOut(wire.NewTxOut(0, nil, wire.TokenData{}))

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatalf("unable to serialize test transaction: %v", err)
	}

	return hex.EncodeToString(append(buf.Bytes(), 0x00))
}

// assertDeserializationError ensures the provided error is an RPC
// deserialization error.
func assertDeserializationError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected trailing bytes to be rejected, got no error")
	}

	rpcErr, ok := err.(*btcjson.RPCError)
	if !ok {
		t.Fatalf("expected *btcjson.RPCError, got %T: %v", err, err)
	}

	if rpcErr.Code != btcjson.ErrRPCDeserialization {
		t.Fatalf("expected error code %v, got %v (%v)",
			btcjson.ErrRPCDeserialization, rpcErr.Code, rpcErr.Message)
	}
}

// TestHandleDecodeRawTransactionTrailingBytes ensures decoderawtransaction
// rejects a transaction that is followed by trailing bytes rather than
// silently ignoring them and reporting success.
func TestHandleDecodeRawTransactionTrailingBytes(t *testing.T) {
	cmd := &btcjson.DecodeRawTransactionCmd{
		HexTx: txHexWithTrailingBytes(t),
	}

	// The handler decodes its input before it touches the server, so a nil
	// server is sufficient to exercise the rejection path.
	_, err := handleDecodeRawTransaction(nil, cmd, nil)
	assertDeserializationError(t, err)
}

// TestHandleSendRawTransactionTrailingBytes ensures sendrawtransaction rejects
// a transaction that is followed by trailing bytes rather than accepting and
// relaying it.
func TestHandleSendRawTransactionTrailingBytes(t *testing.T) {
	allowHighFees := false
	cmd := &btcjson.SendRawTransactionCmd{
		HexTx:         txHexWithTrailingBytes(t),
		AllowHighFees: &allowHighFees,
	}

	// As above, the decode happens before the mempool is reached, so a nil
	// server is sufficient.  Without the trailing byte check this panics on
	// the nil server instead of returning, which is the symptom this test
	// guards against.
	_, err := handleSendRawTransaction(nil, cmd, nil)
	assertDeserializationError(t, err)
}
