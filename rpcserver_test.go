// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
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

// tokenTestP2PKHScript is the pay-to-pubkey-hash script that follows the token
// prefix in the outputs used by the CashTokens output tests.
const tokenTestP2PKHScript = "76a914ea132286328cfc819457b9dec386c4b5c84faa5c88ac"

// tokenDataTests are CashTokens outputs paired with the token data that Bitcoin
// Cash Node reports for them.  The prefixes and the expected JSON objects are
// taken verbatim from the token_tests_prefix_valid.json test vectors of Bitcoin
// Cash Node, except for the no-token output, which Bitcoin Cash Node reports
// without a tokenData object.
var tokenDataTests = []struct {
	name     string
	prefix   string
	expected string
}{
	{
		name:     "no token",
		prefix:   "",
		expected: "",
	},
	{
		name:     "fungible tokens only",
		prefix:   "ef21436587090000000000000000000000000000000000000000000090785634121001",
		expected: `{"amount": "1", "category": "1234567890000000000000000000000000000000000000000000000987654321"}`,
	},
	{
		name:     "immutable nft only",
		prefix:   "efbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb6001cc",
		expected: `{"amount": "0", "category": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "nft": {"commitment": "cc", "capability": "none"}}`,
	},
	{
		name:     "mutable nft only without commitment",
		prefix:   "efbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb21",
		expected: `{"amount": "0", "category": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "nft": {"commitment": "", "capability": "mutable"}}`,
	},
	{
		name:     "minting nft only without commitment",
		prefix:   "efbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb22",
		expected: `{"amount": "0", "category": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "nft": {"commitment": "", "capability": "minting"}}`,
	},
	{
		name:     "fungible tokens and mutable nft",
		prefix:   "efbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb7102ccccffffffffffffffff7f",
		expected: `{"amount": "9223372036854775807", "category": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "nft": {"commitment": "cccc", "capability": "mutable"}}`,
	},
	{
		name:     "fungible tokens and minting nft",
		prefix:   "efbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb3201",
		expected: `{"amount": "1", "category": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "nft": {"commitment": "", "capability": "minting"}}`,
	},
}

// decodeTokenTestTxOut returns the transaction output that results from
// decoding an output whose script is the passed token prefix followed by
// tokenTestP2PKHScript.
func decodeTokenTestTxOut(t *testing.T, prefix string) *wire.TxOut {
	t.Helper()

	script, err := hex.DecodeString(prefix + tokenTestP2PKHScript)
	if err != nil {
		t.Fatalf("unable to decode test script: %v", err)
	}

	var buf bytes.Buffer
	var value [8]byte
	binary.LittleEndian.PutUint64(value[:], 1000)
	buf.Write(value[:])
	if err := wire.WriteVarBytes(&buf, 0, script); err != nil {
		t.Fatalf("unable to serialize test output: %v", err)
	}

	var txOut wire.TxOut
	if _, err := wire.ReadTxOut(&buf, 0, 1, &txOut); err != nil {
		t.Fatalf("unable to deserialize test output: %v", err)
	}

	return &txOut
}

// assertTokenDataResult ensures the passed token data result matches the
// expected JSON object, which is empty when no token data is expected.
func assertTokenDataResult(t *testing.T, name string, result *btcjson.TokenDataResult, expected string) {
	t.Helper()

	if expected == "" {
		if result != nil {
			t.Errorf("%s: unexpected token data %+v", name, result)
		}
		return
	}
	if result == nil {
		t.Errorf("%s: missing token data", name)
		return
	}

	marshalled, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("%s: unable to marshal token data: %v", name, err)
	}
	var got, want interface{}
	if err := json.Unmarshal(marshalled, &got); err != nil {
		t.Fatalf("%s: unable to unmarshal token data: %v", name, err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatalf("%s: unable to unmarshal expected token data: %v", name, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: unexpected token data - got %s, want %s", name,
			marshalled, expected)
	}
}

// TestCreateVoutListTokenData ensures the outputs reported by the raw
// transaction RPCs include the CashTokens data of each output in the format
// used by Bitcoin Cash Node, and that outputs without a token are reported
// without a tokenData field.
func TestCreateVoutListTokenData(t *testing.T) {
	mtx := wire.NewMsgTx(2)
	mtx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil))
	for _, test := range tokenDataTests {
		mtx.AddTxOut(decodeTokenTestTxOut(t, test.prefix))
	}

	vouts := createVoutList(mtx, &chaincfg.MainNetParams, nil)
	if len(vouts) != len(tokenDataTests) {
		t.Fatalf("unexpected number of outputs - got %d, want %d",
			len(vouts), len(tokenDataTests))
	}
	for i, test := range tokenDataTests {
		vout := vouts[i]
		if vout.ScriptPubKey.Hex != tokenTestP2PKHScript {
			t.Errorf("%s: unexpected script - got %s, want %s",
				test.name, vout.ScriptPubKey.Hex, tokenTestP2PKHScript)
		}
		assertTokenDataResult(t, test.name, vout.TokenData, test.expected)

		marshalled, err := json.Marshal(vout)
		if err != nil {
			t.Fatalf("%s: unable to marshal output: %v", test.name, err)
		}
		hasTokenData := strings.Contains(string(marshalled), `"tokenData"`)
		if hasTokenData != (test.expected != "") {
			t.Errorf("%s: unexpected tokenData presence in %s",
				test.name, marshalled)
		}
	}

	// The tokenData object follows scriptPubKey, as it does in Bitcoin
	// Cash Node.
	marshalled, err := json.Marshal(vouts[5])
	if err != nil {
		t.Fatalf("unable to marshal output: %v", err)
	}
	const wantSuffix = `"type":"pubkeyhash","addresses":["qr4pxg5xx2x0eqv527uaasuxcj6usna2tsd4366h8k"]},` +
		`"tokenData":{"category":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",` +
		`"amount":"9223372036854775807","nft":{"capability":"mutable","commitment":"cccc"}}}`
	if !strings.HasSuffix(string(marshalled), wantSuffix) {
		t.Errorf("unexpected marshalled output - got %s, want suffix %s",
			marshalled, wantSuffix)
	}
}

// TestUtxoEntryTokenDataResult ensures the token data that gettxout reports
// for an output in the UTXO set matches what it reports for the same output in
// the mempool.
func TestUtxoEntryTokenDataResult(t *testing.T) {
	for _, test := range tokenDataTests {
		txOut := decodeTokenTestTxOut(t, test.prefix)

		// The mempool path of gettxout reads the token data from the
		// transaction output itself.
		assertTokenDataResult(t, test.name+" (mempool)",
			createTokenDataResult(&txOut.TokenData), test.expected)

		// The UTXO set path reads it from the UTXO entry.
		entryTxOut := *txOut
		entryTxOut.PkScript = append([]byte(nil), txOut.PkScript...)
		entry := blockchain.NewUtxoEntry(&entryTxOut, 100, false)
		tokenData := entry.TokenData()
		assertTokenDataResult(t, test.name+" (utxo set)",
			createTokenDataResult(&tokenData), test.expected)
	}
}
