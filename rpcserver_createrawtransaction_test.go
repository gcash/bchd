// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
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

	result, err := handleCreateRawTransaction(s, cmd, nil)
	if err != nil {
		return nil, err
	}

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
	if len(mtx.TxOut) != 1 {
		t.Fatalf("expected 1 output, got %d", len(mtx.TxOut))
	}
	return &mtx, nil
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

		_, err := handleCreateRawTransaction(s, cmd, nil)
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

		result, err := handleCreateRawTransaction(s, cmd, nil)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		serialized, err := hex.DecodeString(result.(string))
		if err != nil {
			t.Fatalf("%s: unable to decode result hex: %v",
				test.name, err)
		}
		var mtx wire.MsgTx
		err = mtx.BchDecode(bytes.NewReader(serialized),
			maxProtocolVersion, wire.BaseEncoding)
		if err != nil {
			t.Fatalf("%s: unable to decode result: %v", test.name, err)
		}
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
		_, err := handleCreateRawTransaction(s, cmd, nil)
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
