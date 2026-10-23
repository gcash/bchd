// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// testTokenCategory is a token category in the byte order used to display
// transaction hashes.  It is deliberately not a palindrome so a byte order
// mistake changes the result.
const testTokenCategory = "00112233445566778899aabbccddeeff" +
	"0123456789abcdef0123456789abcdef"

// createRawTxTestAddr returns the encoding of a mainnet pay-to-pubkey-hash
// address built from a hash filled with the provided byte.
func createRawTxTestAddr(t *testing.T, fill byte) string {
	t.Helper()

	hash := bytes.Repeat([]byte{fill}, 20)
	addr, err := bchutil.NewAddressPubKeyHash(hash, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create test address: %v", err)
	}
	return addr.EncodeAddress()
}

// runCreateRawTx invokes the createrawtransaction handler with a single
// output paying to addr and the provided token entries, and returns the
// decoded transaction or the error.
func runCreateRawTx(t *testing.T, addr string,
	cashTokens *[]btcjson.TransactionOutputCashToken) (*wire.MsgTx, error) {

	t.Helper()

	s := &rpcServer{cfg: rpcserverConfig{ChainParams: &chaincfg.MainNetParams}}
	cmd := &btcjson.CreateRawTransactionCmd{
		Inputs: []btcjson.TransactionInput{{
			Txid: testTokenCategory,
			Vout: 0,
		}},
		Amounts:    map[string]float64{addr: 0.0001},
		CashTokens: cashTokens,
	}

	result, err := runCreateRawTxCmd(t, s, cmd)
	if err != nil {
		return nil, err
	}

	mtx := decodeCreateRawTxResult(t, result)
	if len(mtx.TxOut) != 1 {
		t.Fatalf("expected 1 output, got %d", len(mtx.TxOut))
	}
	return mtx, nil
}

// runCreateRawTxCmd runs cmd as the server runs a createrawtransaction
// request: the command is marshalled, and its params decoded by parseCmd for
// the handler.
func runCreateRawTxCmd(t *testing.T, s *rpcServer,
	cmd *btcjson.CreateRawTransactionCmd) (interface{}, error) {

	t.Helper()

	marshalled, err := btcjson.MarshalCmd("1.0", 1, cmd)
	if err != nil {
		t.Fatalf("unable to marshal command: %v", err)
	}
	return runCreateRawTxRequest(t, s, string(marshalled))
}

// runCreateRawTxRequest runs a marshalled createrawtransaction request through
// parseCmd and the handler, as the server does.
func runCreateRawTxRequest(t *testing.T, s *rpcServer, request string) (interface{}, error) {
	t.Helper()

	var req btcjson.Request
	if err := json.Unmarshal([]byte(request), &req); err != nil {
		t.Fatalf("unable to unmarshal request %s: %v", request, err)
	}
	parsed := parseCmd(&req)
	if parsed.err != nil {
		return nil, parsed.err
	}
	return s.standardCmdResult(parsed, nil)
}

// decodeCreateRawTxResult decodes the transaction createrawtransaction
// returned.
func decodeCreateRawTxResult(t *testing.T, result interface{}) *wire.MsgTx {
	t.Helper()

	serialized, err := hex.DecodeString(result.(string))
	if err != nil {
		t.Fatalf("unable to decode result hex: %v", err)
	}
	var mtx wire.MsgTx
	err = mtx.BchDecode(bytes.NewReader(serialized), maxProtocolVersion,
		wire.BaseEncoding)
	if err != nil {
		t.Fatalf("unable to decode result transaction: %v", err)
	}
	return &mtx
}

// TestHandleCreateRawTransactionNoTokens ensures createrawtransaction works
// when the optional cashtokens argument is omitted or empty and creates an
// output without token data.
func TestHandleCreateRawTransactionNoTokens(t *testing.T) {
	addr := createRawTxTestAddr(t, 0x01)

	tests := []struct {
		name       string
		cashTokens *[]btcjson.TransactionOutputCashToken
	}{
		{name: "omitted", cashTokens: nil},
		{name: "empty", cashTokens: &[]btcjson.TransactionOutputCashToken{}},
	}

	for _, test := range tests {
		mtx, err := runCreateRawTx(t, addr, test.cashTokens)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		tokenData := mtx.TxOut[0].TokenData
		if !tokenData.IsEmpty() || tokenData.BitField != 0 {
			t.Errorf("%s: expected no token data, got %+v", test.name,
				tokenData)
		}
	}
}

// TestHandleCreateRawTransactionTokens ensures createrawtransaction creates
// fungible-only, NFT-only and combined token outputs with the expected token
// data.
func TestHandleCreateRawTransactionTokens(t *testing.T) {
	addr := createRawTxTestAddr(t, 0x02)

	tests := []struct {
		name           string
		amount         uint64
		capability     string
		commitment     string
		wantBitField   byte
		wantCommitment []byte
	}{
		{
			name:         "fungible only",
			amount:       1000,
			wantBitField: wire.HAS_AMOUNT,
		},
		{
			name:         "fungible only, maximum amount",
			amount:       wire.MAX_FT_AMOUNT,
			wantBitField: wire.HAS_AMOUNT,
		},
		{
			name:         "immutable nft",
			capability:   "none",
			wantBitField: wire.HAS_NFT | wire.NONE,
		},
		{
			name:         "mutable nft",
			capability:   "mutable",
			wantBitField: wire.HAS_NFT | wire.MUTABLE,
		},
		{
			name:         "minting nft",
			capability:   "minting",
			wantBitField: wire.HAS_NFT | wire.MINTING,
		},
		{
			name:         "minting nft, capability in upper case",
			capability:   "MINTING",
			wantBitField: wire.HAS_NFT | wire.MINTING,
		},
		{
			name:       "immutable nft with commitment",
			capability: "none",
			commitment: "c0ffee",
			wantBitField: wire.HAS_NFT | wire.HAS_COMMITMENT_LENGTH |
				wire.NONE,
			wantCommitment: []byte{0xc0, 0xff, 0xee},
		},
		{
			name:       "mutable nft with commitment",
			capability: "mutable",
			commitment: "01",
			wantBitField: wire.HAS_NFT | wire.HAS_COMMITMENT_LENGTH |
				wire.MUTABLE,
			wantCommitment: []byte{0x01},
		},
		{
			name:       "minting nft with maximum length commitment",
			capability: "minting",
			commitment: strings.Repeat("ab", wire.MAX_COMMITMENT_LENGTH),
			wantBitField: wire.HAS_NFT | wire.HAS_COMMITMENT_LENGTH |
				wire.MINTING,
			wantCommitment: bytes.Repeat([]byte{0xab},
				wire.MAX_COMMITMENT_LENGTH),
		},
		{
			name:         "fungible and nft",
			amount:       5,
			capability:   "mutable",
			wantBitField: wire.HAS_AMOUNT | wire.HAS_NFT | wire.MUTABLE,
		},
		{
			name:       "fungible and nft with commitment",
			amount:     5,
			capability: "none",
			commitment: "beef",
			wantBitField: wire.HAS_AMOUNT | wire.HAS_NFT |
				wire.HAS_COMMITMENT_LENGTH | wire.NONE,
			wantCommitment: []byte{0xbe, 0xef},
		},
	}

	for _, test := range tests {
		cashTokens := &[]btcjson.TransactionOutputCashToken{{
			Address:    addr,
			Category:   testTokenCategory,
			Amount:     test.amount,
			Capability: test.capability,
			Commitment: test.commitment,
		}}
		mtx, err := runCreateRawTx(t, addr, cashTokens)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}

		tokenData := mtx.TxOut[0].TokenData
		if tokenData.BitField != test.wantBitField {
			t.Errorf("%s: bitfield %#x, want %#x", test.name,
				tokenData.BitField, test.wantBitField)
		}
		if tokenData.Amount != test.amount {
			t.Errorf("%s: amount %d, want %d", test.name,
				tokenData.Amount, test.amount)
		}
		if !bytes.Equal(tokenData.Commitment, test.wantCommitment) {
			t.Errorf("%s: commitment %x, want %x", test.name,
				tokenData.Commitment, test.wantCommitment)
		}

		// The category is given in the byte order used to display
		// transaction hashes, so it must display the same way.
		gotCategory := chainhash.Hash(tokenData.CategoryID).String()
		if gotCategory != testTokenCategory {
			t.Errorf("%s: category %s, want %s", test.name,
				gotCategory, testTokenCategory)
		}
	}
}

// TestHandleCreateRawTransactionTokenErrors ensures createrawtransaction
// rejects token entries that do not describe a valid token output.
func TestHandleCreateRawTransactionTokenErrors(t *testing.T) {
	addr := createRawTxTestAddr(t, 0x04)
	otherAddr := createRawTxTestAddr(t, 0x05)

	// The same address with its network prefix, which must be treated
	// as a duplicate of the address without it.
	prefixedAddr := chaincfg.MainNetParams.CashAddressPrefix + ":" + addr

	// The same destination as addr in other forms and on another network.
	hash := bytes.Repeat([]byte{0x04}, 20)
	params := &chaincfg.MainNetParams
	tokenAwareAddr, err := bchutil.NewAddressPubKeyHash(hash, params)
	if err != nil {
		t.Fatalf("unable to create address: %v", err)
	}
	testnetAddr, err := bchutil.NewLegacyAddressPubKeyHash(hash,
		&chaincfg.TestNet3Params)
	if err != nil {
		t.Fatalf("unable to create testnet address: %v", err)
	}
	slpAddr, err := bchutil.NewSlpAddressPubKeyHash(hash, params)
	if err != nil {
		t.Fatalf("unable to create slp address: %v", err)
	}
	pubKey := "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959" +
		"f2815b16f81798"

	// Entries without an address or category use addr and
	// testTokenCategory so each case only lists the fields under test.
	tests := []struct {
		name       string
		cashTokens []btcjson.TransactionOutputCashToken
		wantCode   btcjson.RPCErrorCode
		wantMsg    string
	}{
		{
			name: "invalid capability",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Capability: "burnable",
			}},
			wantCode: btcjson.ErrRPCType,
		},
		{
			name: "commitment without nft",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Amount:     1,
				Commitment: "01",
			}},
			wantCode: btcjson.ErrRPCType,
		},
		{
			name: "commitment too long",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Capability: "none",
				Commitment: strings.Repeat("ab",
					wire.MAX_COMMITMENT_LENGTH+1),
			}},
			wantCode: btcjson.ErrRPCType,
		},
		{
			name: "commitment not hex",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Capability: "none",
				Commitment: "zz",
			}},
			wantCode: btcjson.ErrRPCDecodeHexString,
		},
		{
			name: "amount over maximum",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Amount: wire.MAX_FT_AMOUNT + 1,
			}},
			wantCode: btcjson.ErrRPCType,
		},
		{
			name:       "neither fungible amount nor nft",
			cashTokens: []btcjson.TransactionOutputCashToken{{}},
			wantCode:   btcjson.ErrRPCType,
		},
		{
			name: "category not hex",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Category: strings.Repeat("zz", chainhash.HashSize),
				Amount:   1,
			}},
			wantCode: btcjson.ErrRPCDecodeHexString,
		},
		{
			name: "category too short",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Category: testTokenCategory[:62],
				Amount:   1,
			}},
			wantCode: btcjson.ErrRPCType,
		},
		{
			name: "duplicate address",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Amount: 1,
			}, {
				Amount: 2,
			}},
			wantCode: btcjson.ErrRPCInvalidParameter,
		},
		{
			name: "duplicate address with prefix",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Amount: 1,
			}, {
				Address: prefixedAddr,
				Amount:  2,
			}},
			wantCode: btcjson.ErrRPCInvalidParameter,
		},
		{
			name: "duplicate address in token-aware form",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Amount: 1,
			}, {
				Address: tokenAwareAddr.EncodeCashAddress(true),
				Amount:  2,
			}},
			wantCode: btcjson.ErrRPCInvalidParameter,
			wantMsg:  "Invalid parameter, duplicated cashtokens address",
		},
		{
			name: "address for another network",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Address: testnetAddr.EncodeAddress(),
				Amount:  1,
			}},
			wantCode: btcjson.ErrRPCInvalidAddressOrKey,
		},
		{
			name: "slp address",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Address: params.SlpAddressPrefix + ":" +
					slpAddr.EncodeAddress(),
				Amount: 1,
			}},
			wantCode: btcjson.ErrRPCInvalidAddressOrKey,
		},
		{
			name: "public key",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Address: pubKey,
				Amount:  1,
			}},
			wantCode: btcjson.ErrRPCInvalidAddressOrKey,
		},
		{
			name: "address without output",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Address: otherAddr,
				Amount:  1,
			}},
			wantCode: btcjson.ErrRPCInvalidParameter,
		},
		{
			name: "invalid address",
			cashTokens: []btcjson.TransactionOutputCashToken{{
				Address: "notanaddress",
				Amount:  1,
			}},
			wantCode: btcjson.ErrRPCInvalidAddressOrKey,
		},
	}

	for _, test := range tests {
		for i := range test.cashTokens {
			ct := &test.cashTokens[i]
			if ct.Address == "" {
				ct.Address = addr
			}
			if ct.Category == "" {
				ct.Category = testTokenCategory
			}
		}

		_, err := runCreateRawTx(t, addr, &test.cashTokens)
		if err == nil {
			t.Errorf("%s: expected an error, got none", test.name)
			continue
		}
		rpcErr, ok := err.(*btcjson.RPCError)
		if !ok {
			t.Errorf("%s: expected *btcjson.RPCError, got %T: %v",
				test.name, err, err)
			continue
		}
		if rpcErr.Code != test.wantCode {
			t.Errorf("%s: error code %v, want %v (%v)", test.name,
				rpcErr.Code, test.wantCode, rpcErr.Message)
		}
		if !strings.HasPrefix(rpcErr.Message, test.wantMsg) {
			t.Errorf("%s: error message %q, want prefix %q",
				test.name, rpcErr.Message, test.wantMsg)
		}
	}
}

// TestHandleCreateRawTransactionDuplicateDestination ensures
// createrawtransaction rejects two outputs to the same destination, also
// when its address is given in different forms.
func TestHandleCreateRawTransactionDuplicateDestination(t *testing.T) {
	params := &chaincfg.MainNetParams
	hash := bytes.Repeat([]byte{0x06}, 20)
	p2pkh, err := bchutil.NewAddressPubKeyHash(hash, params)
	if err != nil {
		t.Fatalf("unable to create p2pkh address: %v", err)
	}
	p2sh, err := bchutil.NewAddressScriptHashFromHash(hash, params)
	if err != nil {
		t.Fatalf("unable to create p2sh address: %v", err)
	}
	p2sh32, err := bchutil.NewAddressScriptHash32FromHash(
		bytes.Repeat([]byte{0x06}, 32), params)
	if err != nil {
		t.Fatalf("unable to create p2sh32 address: %v", err)
	}

	tests := []struct {
		name  string
		addrs []string
	}{
		{"p2pkh plain and token-aware", []string{
			p2pkh.EncodeCashAddress(false),
			p2pkh.EncodeCashAddress(true),
		}},
		{"p2sh plain and token-aware", []string{
			p2sh.EncodeCashAddress(false),
			p2sh.EncodeCashAddress(true),
		}},
		{"p2sh32 plain and token-aware", []string{
			p2sh32.EncodeCashAddress(false),
			p2sh32.EncodeCashAddress(true),
		}},
		{"with and without prefix", []string{
			p2pkh.EncodeCashAddress(true),
			params.CashAddressPrefix + ":" + p2pkh.EncodeCashAddress(true),
		}},
	}

	s := &rpcServer{cfg: rpcserverConfig{ChainParams: params}}
	for _, test := range tests {
		amounts := make(map[string]float64)
		for _, addr := range test.addrs {
			amounts[addr] = 0.0001
		}
		cmd := &btcjson.CreateRawTransactionCmd{
			Inputs: []btcjson.TransactionInput{{
				Txid: testTokenCategory,
				Vout: 0,
			}},
			Amounts: amounts,
		}

		_, err := runCreateRawTxCmd(t, s, cmd)
		rpcErr, ok := err.(*btcjson.RPCError)
		if !ok {
			t.Errorf("%s: expected *btcjson.RPCError, got %T: %v",
				test.name, err, err)
			continue
		}
		const wantMsg = "Invalid parameter, duplicated address: "
		if rpcErr.Code != btcjson.ErrRPCInvalidParameter ||
			!strings.HasPrefix(rpcErr.Message, wantMsg) {

			t.Errorf("%s: unexpected error %v", test.name, rpcErr)
		}
	}
}

// TestHandleCreateRawTransactionTokenAddressForms ensures a cashtokens entry
// attaches to the output paying to the same destination whichever form
// either address is given in.
func TestHandleCreateRawTransactionTokenAddressForms(t *testing.T) {
	params := &chaincfg.MainNetParams
	p2pkh, err := bchutil.NewAddressPubKeyHash(
		bytes.Repeat([]byte{0x08}, 20), params)
	if err != nil {
		t.Fatalf("unable to create p2pkh address: %v", err)
	}
	legacy, err := bchutil.NewLegacyAddressPubKeyHash(
		bytes.Repeat([]byte{0x08}, 20), params)
	if err != nil {
		t.Fatalf("unable to create legacy address: %v", err)
	}
	otherAddr := createRawTxTestAddr(t, 0x09)
	wantScript, err := txscript.PayToAddrScript(p2pkh)
	if err != nil {
		t.Fatalf("unable to create script: %v", err)
	}

	tests := []struct {
		name      string
		output    string
		tokenAddr string
	}{
		{"plain output, token-aware entry", p2pkh.EncodeCashAddress(false),
			p2pkh.EncodeCashAddress(true)},
		{"token-aware output, plain entry", p2pkh.EncodeCashAddress(true),
			p2pkh.EncodeCashAddress(false)},
		{"token-aware output and entry", p2pkh.EncodeCashAddress(true),
			p2pkh.EncodeCashAddress(true)},
		{"legacy output, token-aware entry", legacy.EncodeAddress(),
			p2pkh.EncodeCashAddress(true)},
		{"token-aware output, legacy entry", p2pkh.EncodeCashAddress(true),
			legacy.EncodeAddress()},
	}

	s := &rpcServer{cfg: rpcserverConfig{ChainParams: params}}
	for _, test := range tests {
		cmd := &btcjson.CreateRawTransactionCmd{
			Inputs: []btcjson.TransactionInput{{
				Txid: testTokenCategory,
				Vout: 0,
			}},
			Amounts: map[string]float64{
				test.output: 0.0001,
				otherAddr:   0.0001,
			},
			CashTokens: &[]btcjson.TransactionOutputCashToken{{
				Address:  test.tokenAddr,
				Category: testTokenCategory,
				Amount:   7,
			}},
		}

		result, err := runCreateRawTxCmd(t, s, cmd)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		mtx := decodeCreateRawTxResult(t, result)
		if len(mtx.TxOut) != 2 {
			t.Fatalf("%s: expected 2 outputs, got %d", test.name,
				len(mtx.TxOut))
		}
		for i, txOut := range mtx.TxOut {
			wantAmount := uint64(0)
			if bytes.Equal(txOut.PkScript, wantScript) {
				wantAmount = 7
			}
			if txOut.TokenData.Amount != wantAmount {
				t.Errorf("%s: output %d token amount %d, want %d",
					test.name, i, txOut.TokenData.Amount,
					wantAmount)
			}
		}
	}
}

// TestHandleCreateRawTransactionLegacyAddress ensures createrawtransaction
// pays to legacy addresses, as BCHN does, and refuses one for another
// network.
func TestHandleCreateRawTransactionLegacyAddress(t *testing.T) {
	hash := bytes.Repeat([]byte{0x01}, 20)
	p2pkh, err := bchutil.NewLegacyAddressPubKeyHash(hash, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	p2sh, err := bchutil.NewLegacyAddressScriptHashFromHash(hash, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	for _, addr := range []bchutil.Address{p2pkh, p2sh} {
		mtx, err := runCreateRawTx(t, addr.EncodeAddress(), nil)
		if err != nil {
			t.Errorf("%v: %v", addr, err)
			continue
		}
		want, err := txscript.PayToAddrScript(addr)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(mtx.TxOut[0].PkScript, want) {
			t.Errorf("%v: script %x, want %x", addr, mtx.TxOut[0].PkScript, want)
		}
	}

	testnet, err := bchutil.NewLegacyAddressPubKeyHash(hash, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCreateRawTx(t, testnet.EncodeAddress(), nil); err == nil {
		t.Error("testnet legacy address accepted on mainnet")
	}
}

// TestHandleCreateRawTransactionAmountRange ensures createrawtransaction
// accepts an amount of up to 21 million BCH, all there can be, and refuses a
// larger one, as the bound is in BCH like the amounts.
func TestHandleCreateRawTransactionAmountRange(t *testing.T) {
	s := &rpcServer{cfg: rpcserverConfig{ChainParams: &chaincfg.MainNetParams}}
	addr := createRawTxTestAddr(t, 1)
	for _, test := range []struct {
		amount float64
		ok     bool
	}{
		{21e6, true},
		{21e6 + 1, false},
		{1e12, false},
	} {
		cmd := &btcjson.CreateRawTransactionCmd{
			Inputs:  []btcjson.TransactionInput{{Txid: testTokenCategory}},
			Amounts: map[string]float64{addr: test.amount},
		}
		_, err := runCreateRawTxCmd(t, s, cmd)
		if test.ok {
			if err != nil {
				t.Errorf("%v BCH: %v", test.amount, err)
			}
			continue
		}
		if rerr, ok := err.(*btcjson.RPCError); !ok || rerr.Code != btcjson.ErrRPCType ||
			rerr.Message != "Invalid amount" {

			t.Errorf("%v BCH: got error %v, want Invalid amount", test.amount, err)
		}
	}
}

// runCreateRawTxParams runs a mainnet createrawtransaction request with the
// params given in JSON, as the server does, and returns the transaction made
// or the error.
func runCreateRawTxParams(t *testing.T, params string) (*wire.MsgTx, error) {
	t.Helper()

	s := &rpcServer{cfg: rpcserverConfig{ChainParams: &chaincfg.MainNetParams}}
	request := `{"jsonrpc":"1.0","id":1,"method":"createrawtransaction",` +
		`"params":` + params + `}`
	result, err := runCreateRawTxRequest(t, s, request)
	if err != nil {
		return nil, err
	}
	return decodeCreateRawTxResult(t, result), nil
}

// createRawTxTestInput is the input the createrawtransaction tests spend,
// in JSON, with extra members such as a sequence number.
func createRawTxTestInput(extra string) string {
	return `[{"txid":"` + testTokenCategory + `","vout":3` + extra + `}]`
}

// TestCreateRawTransactionOutputOrder ensures createrawtransaction makes the
// outputs in the order given, in an object of outputs as in an array of
// single-key objects, as BCHN does.
func TestCreateRawTransactionOutputOrder(t *testing.T) {
	var addrs []string
	var wantScripts [][]byte
	for _, fill := range []byte{0x37, 0x05, 0x91, 0x22, 0x10, 0x6c, 0x48, 0x03} {
		addr := createRawTxTestAddr(t, fill)
		decoded, err := bchutil.DecodeAddress(addr, &chaincfg.MainNetParams)
		if err != nil {
			t.Fatal(err)
		}
		script, err := txscript.PayToAddrScript(decoded)
		if err != nil {
			t.Fatal(err)
		}
		addrs = append(addrs, addr)
		wantScripts = append(wantScripts, script)
	}

	var object, array []string
	for i, addr := range addrs {
		member := `"` + addr + `":0.000` + string(rune('1'+i))
		object = append(object, member)
		array = append(array, "{"+member+"}")
	}
	for name, outputs := range map[string]string{
		"object": "{" + strings.Join(object, ",") + "}",
		"array":  "[" + strings.Join(array, ",") + "]",
	} {
		mtx, err := runCreateRawTxParams(t, "["+createRawTxTestInput("")+
			","+outputs+"]")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(mtx.TxOut) != len(addrs) {
			t.Errorf("%s: %d outputs, want %d", name, len(mtx.TxOut),
				len(addrs))
			continue
		}
		for i, txOut := range mtx.TxOut {
			if !bytes.Equal(txOut.PkScript, wantScripts[i]) ||
				txOut.Value != int64(i+1)*10000 {

				t.Errorf("%s: output %d pays %d to %x, want %d to %x",
					name, i, txOut.Value, txOut.PkScript,
					(i+1)*10000, wantScripts[i])
			}
		}
	}
}

// TestCreateRawTransactionBCHNForms ensures createrawtransaction takes the
// params in BCHN's forms: the locktime as the third param, an input's
// sequence number, an output given as an object with its amount and token
// data, and "data" outputs, and makes a version 2 transaction as BCHN does,
// while bchd's forms keep working.
func TestCreateRawTransactionBCHNForms(t *testing.T) {
	addr := createRawTxTestAddr(t, 0x0a)
	decoded, err := bchutil.DecodeAddress(addr, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	addrScript, err := txscript.PayToAddrScript(decoded)
	if err != nil {
		t.Fatal(err)
	}
	category, err := chainhash.NewHashFromStr(testTokenCategory)
	if err != nil {
		t.Fatal(err)
	}
	output := `{"` + addr + `":0.0001}`
	tokenEntry := `[{"address":"` + addr + `","category":"` +
		testTokenCategory + `","amount":5}]`
	push76 := strings.Repeat("ab", 76)

	// pushes returns a null data script with pushes of each length given,
	// with the push opcodes BCHN uses.
	pushes := func(lengths ...int) (string, []byte) {
		var hexes []string
		script := []byte{txscript.OP_RETURN}
		for _, n := range lengths {
			data := bytes.Repeat([]byte{0xcd}, n)
			hexes = append(hexes, `"`+hex.EncodeToString(data)+`"`)
			switch {
			case n < 76:
				script = append(script, byte(n))
			case n < 256:
				script = append(script, 0x4c, byte(n))
			case n < 65536:
				script = append(script, 0x4d, byte(n), byte(n>>8))
			default:
				script = append(script, 0x4e, byte(n), byte(n>>8),
					byte(n>>16), byte(n>>24))
			}
			script = append(script, data...)
		}
		return "[" + strings.Join(hexes, ",") + "]", script
	}
	longPushes, longScript := pushes(75, 255, 256, 65535, 65536)

	// tokenData returns the token data of the first output.
	tokenData := func(mtx *wire.MsgTx) *wire.TokenData { return &mtx.TxOut[0].TokenData }

	tests := []struct {
		name     string
		params   string
		lockTime uint32
		sequence uint32
		check    func(*wire.MsgTx) bool
	}{{
		name:     "locktime as the third param",
		params:   "[" + createRawTxTestInput("") + "," + output + ",500000]",
		lockTime: 500000,
		sequence: wire.MaxTxInSequenceNum - 1,
	}, {
		name:     "sequence",
		params:   "[" + createRawTxTestInput(`,"sequence":7`) + "," + output + "]",
		sequence: 7,
	}, {
		name: "sequence with a locktime",
		params: "[" + createRawTxTestInput(`,"sequence":4294967295`) + "," +
			output + ",500000]",
		lockTime: 500000,
		sequence: wire.MaxTxInSequenceNum,
	}, {
		name: "output object with token data",
		params: "[" + createRawTxTestInput("") + `,{"` + addr +
			`":{"amount":0.0001,"tokenData":{"category":"` +
			testTokenCategory + `","amount":"1000","nft":{` +
			`"capability":"mutable","commitment":"beef"}}}}]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			td := tokenData(mtx)
			return mtx.TxOut[0].Value == 10000 &&
				bytes.Equal(mtx.TxOut[0].PkScript, addrScript) &&
				td.CategoryID == [32]byte(*category) &&
				td.Amount == 1000 && td.IsMutableNFT() &&
				bytes.Equal(td.Commitment, []byte{0xbe, 0xef})
		},
	}, {
		name: "output object with an nft of no capability",
		params: "[" + createRawTxTestInput("") + `,[{"` + addr +
			`":{"amount":0.0001,"tokenData":{"category":"` +
			testTokenCategory + `","nft":{}}}}]]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			td := tokenData(mtx)
			return td.HasNFT() && !td.IsMutableNFT() &&
				!td.IsMintingNFT() && td.Amount == 0 &&
				len(td.Commitment) == 0
		},
	}, {
		// The capability and commitment of a cashtokens entry are not
		// members of BCHN's tokenData, which ignores them.
		name: "token data with members of a cashtokens entry",
		params: "[" + createRawTxTestInput("") + `,{"` + addr +
			`":{"amount":0.0001,"tokenData":{"category":"` +
			testTokenCategory + `","amount":5,"capability":"minting",` +
			`"commitment":"beef"}}}]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			td := tokenData(mtx)
			return td.Amount == 5 && !td.HasNFT()
		},
	}, {
		name: "output object with the amount twice",
		params: "[" + createRawTxTestInput("") + `,{"` + addr +
			`":{"amount":0.0001,"amount":0.0002}}]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			return mtx.TxOut[0].Value == 10000
		},
	}, {
		name:     "null outputs",
		params:   "[" + createRawTxTestInput("") + ",null]",
		sequence: wire.MaxTxInSequenceNum,
		check:    func(mtx *wire.MsgTx) bool { return len(mtx.TxOut) == 0 },
	}, {
		name: "output object without token data",
		params: "[" + createRawTxTestInput("") + `,{"` + addr +
			`":{"amount":0.0001}}]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			return mtx.TxOut[0].Value == 10000 && tokenData(mtx).IsEmpty()
		},
	}, {
		name: "data",
		params: "[" + createRawTxTestInput("") + `,{"data":"00ff","` + addr +
			`":0.0001}]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			return len(mtx.TxOut) == 2 && mtx.TxOut[0].Value == 0 &&
				bytes.Equal(mtx.TxOut[0].PkScript,
					[]byte{0x6a, 0x02, 0x00, 0xff}) &&
				bytes.Equal(mtx.TxOut[1].PkScript, addrScript)
		},
	}, {
		// Single bytes are pushed with a length byte, as BCHN pushes
		// them, not as small integer opcodes.
		name: "data array",
		params: "[" + createRawTxTestInput("") + `,[{"data":["01","10","81","` +
			push76 + `"]}]]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			want := append([]byte{0x6a, 0x01, 0x01, 0x01, 0x10, 0x01,
				0x81, 0x4c, 76}, bytes.Repeat([]byte{0xab}, 76)...)
			return len(mtx.TxOut) == 1 && mtx.TxOut[0].Value == 0 &&
				bytes.Equal(mtx.TxOut[0].PkScript, want)
		},
	}, {
		name: "data pushes at each length boundary",
		params: "[" + createRawTxTestInput("") + `,{"data":` + longPushes +
			`}]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			return len(mtx.TxOut) == 1 &&
				bytes.Equal(mtx.TxOut[0].PkScript, longScript)
		},
	}, {
		name:     "two data outputs",
		params:   "[" + createRawTxTestInput("") + `,{"data":"01","data":"02"}]`,
		sequence: wire.MaxTxInSequenceNum,
		check: func(mtx *wire.MsgTx) bool {
			return len(mtx.TxOut) == 2 &&
				bytes.Equal(mtx.TxOut[0].PkScript, []byte{0x6a, 0x01, 0x01}) &&
				bytes.Equal(mtx.TxOut[1].PkScript, []byte{0x6a, 0x01, 0x02})
		},
	}, {
		name:     "bchd form with tokens and locktime",
		params:   "[" + createRawTxTestInput("") + "," + output + "," + tokenEntry + ",7]",
		lockTime: 7,
		sequence: wire.MaxTxInSequenceNum - 1,
		check:    func(mtx *wire.MsgTx) bool { return tokenData(mtx).Amount == 5 },
	}, {
		name:     "bchd form with tokens",
		params:   "[" + createRawTxTestInput("") + "," + output + "," + tokenEntry + "]",
		sequence: wire.MaxTxInSequenceNum,
		check:    func(mtx *wire.MsgTx) bool { return tokenData(mtx).Amount == 5 },
	}, {
		name:     "bchd form without tokens, with locktime",
		params:   "[" + createRawTxTestInput("") + "," + output + ",null,8]",
		lockTime: 8,
		sequence: wire.MaxTxInSequenceNum - 1,
	}, {
		name:     "null third param",
		params:   "[" + createRawTxTestInput("") + "," + output + ",null]",
		sequence: wire.MaxTxInSequenceNum,
	}}

	for _, test := range tests {
		mtx, err := runCreateRawTxParams(t, test.params)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		if mtx.Version != 2 {
			t.Errorf("%s: version %d, want 2", test.name, mtx.Version)
		}
		if mtx.LockTime != test.lockTime {
			t.Errorf("%s: locktime %d, want %d", test.name,
				mtx.LockTime, test.lockTime)
		}
		if len(mtx.TxIn) != 1 || mtx.TxIn[0].Sequence != test.sequence ||
			mtx.TxIn[0].PreviousOutPoint.Index != 3 {

			t.Errorf("%s: %d inputs, the first %+v, want one spending "+
				"output 3 with sequence %d", test.name, len(mtx.TxIn),
				*mtx.TxIn[0], test.sequence)
		}
		if test.check != nil && !test.check(mtx) {
			t.Errorf("%s: unexpected outputs %+v", test.name, mtx.TxOut)
		}
	}
}

// TestCreateRawTransactionBCHNFormErrors ensures createrawtransaction refuses
// the outputs BCHN refuses, with its messages, and an output given token data
// both with it and in the cashtokens param.
func TestCreateRawTransactionBCHNFormErrors(t *testing.T) {
	addr := createRawTxTestAddr(t, 0x0b)
	otherAddr := createRawTxTestAddr(t, 0x0c)
	input := createRawTxTestInput("")
	tokenObject := `{"amount":0.0001,"tokenData":{"category":"` +
		testTokenCategory + `","amount":1}}`

	tests := []struct {
		name     string
		params   string
		wantCode btcjson.RPCErrorCode
		wantMsg  string
	}{{
		name:     "array member not an object",
		params:   "[" + input + `,[1]]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Invalid parameter, key-value pair not an object as expected",
	}, {
		name:     "array member with two keys",
		params:   "[" + input + `,[{"` + addr + `":0.1,"` + otherAddr + `":0.1}]]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Invalid parameter, key-value pair must contain exactly one key",
	}, {
		name:     "array member with no key",
		params:   "[" + input + `,[{}]]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Invalid parameter, key-value pair must contain exactly one key",
	}, {
		name:     "address twice in an array",
		params:   "[" + input + `,[{"` + addr + `":0.1},{"` + addr + `":0.2}]]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Invalid parameter, duplicated address: " + addr,
	}, {
		name:     "address twice in an object",
		params:   "[" + input + `,{"` + addr + `":0.1,"` + addr + `":0.2}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Invalid parameter, duplicated address: " + addr,
	}, {
		name:     "data not hex",
		params:   "[" + input + `,{"data":"zz"}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "data must be hexadecimal string (not 'zz')",
	}, {
		name:     "data of odd length",
		params:   "[" + input + `,{"data":"abc"}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "data must be hexadecimal string (not 'abc')",
	}, {
		name:     "data empty",
		params:   "[" + input + `,{"data":""}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "data must be hexadecimal string (not '')",
	}, {
		name:     "data array empty",
		params:   "[" + input + `,{"data":[]}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "data array must contain at least one element",
	}, {
		name:     "data array element not a string",
		params:   "[" + input + `,{"data":[1]}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "data array element must be hexadecimal string",
	}, {
		name:     "data array element not hex",
		params:   "[" + input + `,{"data":["0g"]}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "data array element must be hexadecimal string (not '0g')",
	}, {
		name:     "data neither string nor array",
		params:   "[" + input + `,{"data":5}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "data must be either a hexadecimal string or an array of hexadecimal strings",
	}, {
		name:     "output object without amount",
		params:   "[" + input + `,{"` + addr + `":{"tokenData":{}}}]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Invalid parameter, missing amount key",
	}, {
		name: "nft capability empty",
		params: "[" + input + `,{"` + addr + `":{"amount":0.0001,"tokenData":{` +
			`"category":"` + testTokenCategory + `","nft":{"capability":""}}}}]`,
		wantCode: btcjson.ErrRPCType,
		wantMsg:  "Invalid token capability",
	}, {
		name: "token data twice",
		params: "[" + input + `,{"` + addr + `":` + tokenObject + `},[{"address":"` +
			addr + `","category":"` + testTokenCategory + `","amount":2}]]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Invalid parameter, duplicated cashtokens address: " + addr,
	}, {
		name:     "numeric third param and a fourth",
		params:   "[" + input + `,{"` + addr + `":0.1},5,6]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg: "Failed to parse request: parameter #3 'cashtokens' must " +
			"be type []btcjson.TransactionOutputCashToken (got number)",
	}, {
		name:     "five params",
		params:   "[" + input + `,{"` + addr + `":0.1},[],5,6]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg: "Failed to parse request: wrong number of params " +
			"(expected between 2 and 4, received 5)",
	}, {
		name:     "one param",
		params:   "[" + input + `]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg: "Failed to parse request: wrong number of params " +
			"(expected between 2 and 4, received 1)",
	}, {
		name:     "locktime not an integer",
		params:   "[" + input + `,{"` + addr + `":0.1},1.5]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg: "Failed to parse request: parameter #3 'locktime' must " +
			"be type int64 (got number 1.5)",
	}, {
		name:     "outputs not an object or array",
		params:   "[" + input + `,"` + addr + `"]`,
		wantCode: btcjson.ErrRPCInvalidParameter,
		wantMsg:  "Failed to parse request: parameter #2 'amounts'",
	}}

	for _, test := range tests {
		_, err := runCreateRawTxParams(t, test.params)
		rpcErr, ok := err.(*btcjson.RPCError)
		if !ok {
			t.Errorf("%s: got %v, want an RPC error", test.name, err)
			continue
		}
		if rpcErr.Code != test.wantCode ||
			!strings.HasPrefix(rpcErr.Message, test.wantMsg) {

			t.Errorf("%s: got error %d %q, want %d %q", test.name,
				rpcErr.Code, rpcErr.Message, test.wantCode, test.wantMsg)
		}
	}
}

// TestCreateRawTxDataScriptLongPush ensures a push of 16 MiB or more gets all
// four bytes of its OP_PUSHDATA4 length, as no JSON test request is that
// large.
func TestCreateRawTxDataScriptLongPush(t *testing.T) {
	push := make([]byte, 1<<24+2)
	script := createRawTxDataScript([][]byte{push})
	want := []byte{txscript.OP_RETURN, txscript.OP_PUSHDATA4, 2, 0, 0, 1}
	if len(script) != len(want)+len(push) || !bytes.Equal(script[:len(want)], want) {
		t.Errorf("script of %d bytes starts %x, want %d bytes starting %x",
			len(script), script[:len(want)], len(want)+len(push), want)
	}
}
