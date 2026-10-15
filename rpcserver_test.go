// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
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

// getBlockTestBestHeight is the height of the best chain that holds the block
// used by the getblock tests.
const getBlockTestBestHeight = 305

// mustDecodeHex returns the bytes of the passed hex string and fails the test
// if it is not valid hex.
func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("unable to decode hex %q: %v", s, err)
	}
	return b
}

// getBlockTestBlock returns the block used by the getblock tests along with its
// spend journal.
func getBlockTestBlock(t *testing.T) (*bchutil.Block, []blockchain.SpentTxOut) {
	t.Helper()

	p2pkh := mustDecodeHex(t, "76a914123456789012345678901234567890123456789088ac")
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{},
		wire.MaxPrevOutIndex), mustDecodeHex(t, "022c0100")))
	coinbase.AddTxOut(wire.NewTxOut(5000020200, p2pkh, wire.TokenData{}))

	tx1 := wire.NewMsgTx(2)
	tx1.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{0x01}, 0),
		mustDecodeHex(t, "0151")))
	tx1.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{0x02}, 1),
		mustDecodeHex(t, "0152")))
	tx1.AddTxOut(wire.NewTxOut(4999990000, p2pkh, wire.TokenData{}))
	tx1.AddTxOut(decodeTokenTestTxOut(t, tokenDataTests[1].prefix))

	tx2 := wire.NewMsgTx(2)
	tx2.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0),
		mustDecodeHex(t, "0153")))
	tx2.TxIn[0].PreviousOutPoint.Hash = tx1.TxHash()
	tx2.AddTxOut(wire.NewTxOut(4999980000,
		mustDecodeHex(t, "a914b472a266d0bd89c13706a4132ccfb16f7c3b9fcb87"),
		wire.TokenData{}))

	prevHash, err := chainhash.NewHashFromStr(
		"00000000000000000123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("unable to decode previous block hash: %v", err)
	}
	msgBlock := wire.NewMsgBlock(&wire.BlockHeader{
		Version:    0x20000000,
		PrevBlock:  *prevHash,
		MerkleRoot: coinbase.TxHash(),
		Timestamp:  time.Unix(1700000000, 0),
		Bits:       0x1d00ffff,
		Nonce:      7,
	})
	msgBlock.Transactions = []*wire.MsgTx{coinbase, tx1, tx2}
	blk := bchutil.NewBlock(msgBlock)
	blk.SetHeight(300)

	// The last entry is the first output of tx1, which tx2 spends.
	tokenPrefix := tokenDataTests[5].prefix
	stxos := []blockchain.SpentTxOut{{
		Amount:     5000000000,
		PkScript:   mustDecodeHex(t, tokenTestP2PKHScript),
		Height:     100,
		IsCoinBase: true,
	}, {
		Amount:   decodeTokenTestTxOut(t, tokenPrefix).Value,
		PkScript: mustDecodeHex(t, tokenPrefix+tokenTestP2PKHScript),
		Height:   200,
	}, {
		Amount:   4999990000,
		PkScript: p2pkh,
		Height:   300,
	}}

	return blk, stxos
}

// getBlockTestReply returns the getblock result for the passed block, with the
// block fields set the same way handleGetBlock sets them.
func getBlockTestReply(blk *bchutil.Block) btcjson.GetBlockVerboseResult {
	blockHeader := &blk.MsgBlock().Header
	return btcjson.GetBlockVerboseResult{
		Hash:          blk.Hash().String(),
		Version:       blockHeader.Version,
		VersionHex:    fmt.Sprintf("%08x", blockHeader.Version),
		MerkleRoot:    blockHeader.MerkleRoot.String(),
		PreviousHash:  blockHeader.PrevBlock.String(),
		Nonce:         blockHeader.Nonce,
		Time:          blockHeader.Timestamp.Unix(),
		Confirmations: int64(1 + getBlockTestBestHeight - blk.Height()),
		Height:        int64(blk.Height()),
		Size:          int32(blk.MsgBlock().SerializeSize()),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits), 16),
		Difficulty: getDifficultyRatio(blockHeader.Bits,
			&chaincfg.MainNetParams),
	}
}

// getBlockTestTxns returns the raw transaction results for the passed block.
func getBlockTestTxns(t *testing.T, blk *bchutil.Block) []btcjson.TxRawResult {
	t.Helper()

	rawTxns, err := createBlockTxRawResults(&chaincfg.MainNetParams, blk,
		blk.Hash().String(), getBlockTestBestHeight)
	if err != nil {
		t.Fatalf("unable to create transaction results: %v", err)
	}
	return rawTxns
}

// assertGolden ensures the passed result marshals to the JSON in the named
// file in the testdata directory, byte for byte apart from indentation.
func assertGolden(t *testing.T, name string, result interface{}) {
	t.Helper()

	marshalled, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("%s: unable to marshal result: %v", name, err)
	}
	var got bytes.Buffer
	if err := json.Indent(&got, marshalled, "", "\t"); err != nil {
		t.Fatalf("%s: unable to indent result: %v", name, err)
	}
	got.WriteByte('\n')

	want, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("%s: unable to read golden file: %v", name, err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("%s: result does not match golden file - got:\n%s",
			name, got.Bytes())
	}
}

// TestGetBlockVerbosity1And2Unchanged ensures the getblock results for
// verbosity 1 and 2 are unchanged by the addition of verbosity 3.  The golden
// files were generated from the results before verbosity 3 was added.
func TestGetBlockVerbosity1And2Unchanged(t *testing.T) {
	blk, _ := getBlockTestBlock(t)

	reply := getBlockTestReply(blk)
	for _, tx := range blk.Transactions() {
		reply.Tx = append(reply.Tx, tx.Hash().String())
	}
	assertGolden(t, "getblock-verbosity-1.json", reply)

	reply = getBlockTestReply(blk)
	reply.RawTx = getBlockTestTxns(t, blk)
	assertGolden(t, "getblock-verbosity-2.json", reply)
}

// TestGetBlockVerbosity3 ensures getblock with verbosity 3 reports the prevout
// of each input and the fee of each transaction.
func TestGetBlockVerbosity3(t *testing.T) {
	blk, stxos := getBlockTestBlock(t)
	rawTxns := getBlockTestTxns(t, blk)
	err := addBlockPrevOuts(&chaincfg.MainNetParams, rawTxns, blk, stxos)
	if err != nil {
		t.Fatalf("unable to add prevouts: %v", err)
	}

	reply := getBlockTestReply(blk)
	assertGolden(t, "getblock-verbosity-3.json", btcjson.GetBlockVerboseTxResult{
		Hash:          reply.Hash,
		Confirmations: reply.Confirmations,
		Size:          reply.Size,
		Height:        reply.Height,
		Version:       reply.Version,
		VersionHex:    reply.VersionHex,
		MerkleRoot:    reply.MerkleRoot,
		Tx:            rawTxns,
		Time:          reply.Time,
		Nonce:         reply.Nonce,
		Bits:          reply.Bits,
		Difficulty:    reply.Difficulty,
		PreviousHash:  reply.PreviousHash,
	})
}

// TestAddBlockPrevOutsJournalMismatch ensures the transaction results are left
// unchanged, without prevouts or fees, when the spend journal does not hold
// exactly one entry for each input of the block.
func TestAddBlockPrevOutsJournalMismatch(t *testing.T) {
	blk, stxos := getBlockTestBlock(t)
	want, err := json.Marshal(getBlockTestTxns(t, blk))
	if err != nil {
		t.Fatalf("unable to marshal results: %v", err)
	}

	tests := map[string][]blockchain.SpentTxOut{
		"empty journal": nil,
		"missing entry": stxos[:len(stxos)-1],
		"extra entry":   append(stxos[:len(stxos):len(stxos)], stxos[0]),
	}
	for name, journal := range tests {
		rawTxns := getBlockTestTxns(t, blk)
		err := addBlockPrevOuts(&chaincfg.MainNetParams, rawTxns, blk,
			journal)
		if err == nil {
			t.Errorf("%s: expected an error", name)
		}
		got, err := json.Marshal(rawTxns)
		if err != nil {
			t.Fatalf("%s: unable to marshal results: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: results were changed: %s", name, got)
		}
	}
}

// TestCreatePrevOutResultScripts ensures the script and token data of a prevout
// are reported correctly.
func TestCreatePrevOutResultScripts(t *testing.T) {
	multisig := "51" + "21" + strings.Repeat("02", 33) + "21" +
		strings.Repeat("03", 33) + "52ae"
	p2pk := "21" + "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959" +
		"f2815b16f81798" + "ac"

	tests := []struct {
		name     string
		pkScript string
		wantHex  string
		wantType string
		wantAddr string
	}{
		{"p2pkh", tokenTestP2PKHScript, tokenTestP2PKHScript, "pubkeyhash",
			"qr4pxg5xx2x0eqv527uaasuxcj6usna2tsd4366h8k"},
		{"p2pk", p2pk, p2pk, "pubkey",
			"qp63uahgrxged4z5jswyt5dn5v3lzsem6cy4spdc2h"},
		{"bare multisig", multisig, multisig, "multisig", ""},
		{"token", tokenDataTests[1].prefix + tokenTestP2PKHScript,
			tokenTestP2PKHScript, "pubkeyhash",
			"qr4pxg5xx2x0eqv527uaasuxcj6usna2tsd4366h8k"},
	}

	for _, test := range tests {
		stxo := blockchain.SpentTxOut{
			Amount:   1000,
			PkScript: mustDecodeHex(t, test.pkScript),
			Height:   10,
		}
		result := createPrevOutResult(&chaincfg.MainNetParams, &stxo)
		script := result.ScriptPubKey
		if script.Hex != test.wantHex || script.Type != test.wantType ||
			script.Address != test.wantAddr {

			t.Errorf("%s: unexpected script %+v", test.name, script)
		}
		wantToken := test.name == "token"
		if (result.TokenData != nil) != wantToken {
			t.Errorf("%s: unexpected token data %+v", test.name,
				result.TokenData)
		}
	}
}

// TestCreatePrevOutResultP2SH32Type ensures a P2SH32 prevout is reported with
// the scripthash type.
func TestCreatePrevOutResultP2SH32Type(t *testing.T) {
	stxo := blockchain.SpentTxOut{
		Amount:   1000,
		PkScript: mustDecodeHex(t, "aa20"+strings.Repeat("ab", 32)+"87"),
		Height:   10,
	}
	result := createPrevOutResult(&chaincfg.MainNetParams, &stxo)
	if result.ScriptPubKey.Type != "scripthash" {
		t.Errorf("unexpected type %q", result.ScriptPubKey.Type)
	}
}

// TestHandleDecodeScriptP2SH ensures decodescript doesn't wrap a script that is
// already pay-to-script-hash in another pay-to-script-hash address.
func TestHandleDecodeScriptP2SH(t *testing.T) {
	s := &rpcServer{cfg: rpcserverConfig{ChainParams: &chaincfg.MainNetParams}}
	tests := []struct {
		name     string
		script   string
		wantP2sh bool
	}{
		{"p2pkh", tokenTestP2PKHScript, true},
		{"p2sh", "a914" + strings.Repeat("ab", 20) + "87", false},
		{"p2sh32", "aa20" + strings.Repeat("ab", 32) + "87", false},
	}

	for _, test := range tests {
		cmd := &btcjson.DecodeScriptCmd{HexScript: test.script}
		result, err := handleDecodeScript(s, cmd, nil)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		reply := result.(btcjson.DecodeScriptResult)
		if (reply.P2sh != "") != test.wantP2sh {
			t.Errorf("%s: unexpected p2sh %q", test.name, reply.P2sh)
		}
	}
}
