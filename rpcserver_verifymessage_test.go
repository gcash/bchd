// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// signTestMessage signs a message the way signmessage does.
func signTestMessage(t *testing.T, key *bchec.PrivateKey, msg string,
	compressed bool) string {

	t.Helper()
	var buf bytes.Buffer
	_ = wire.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
	_ = wire.WriteVarString(&buf, 0, msg)
	sig, err := bchec.SignCompact(bchec.S256(), key,
		chainhash.DoubleHashB(buf.Bytes()), compressed)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// TestHandleVerifyMessage ensures verifymessage accepts cashaddr and legacy
// P2PKH addresses and returns Bitcoin Cash Node's errors.
func TestHandleVerifyMessage(t *testing.T) {
	params := &chaincfg.MainNetParams
	s := &rpcServer{cfg: rpcserverConfig{ChainParams: params}}

	key, _ := bchec.PrivKeyFromBytes(bchec.S256(), big.NewInt(1).Bytes())
	otherKey, _ := bchec.PrivKeyFromBytes(bchec.S256(), big.NewInt(2).Bytes())
	hash := bchutil.Hash160(key.PubKey().SerializeCompressed())
	uncompressedHash := bchutil.Hash160(key.PubKey().SerializeUncompressed())
	otherHash := bchutil.Hash160(otherKey.PubKey().SerializeCompressed())

	cashAddr, _ := bchutil.NewAddressPubKeyHash(hash, params)
	legacyAddr, _ := bchutil.NewLegacyAddressPubKeyHash(hash, params)
	uncompressedAddr, _ := bchutil.NewLegacyAddressPubKeyHash(
		uncompressedHash, params)
	otherAddr, _ := bchutil.NewAddressPubKeyHash(otherHash, params)
	testnetAddr, _ := bchutil.NewLegacyAddressPubKeyHash(hash,
		&chaincfg.TestNet3Params)
	scriptAddr, _ := bchutil.NewAddressScriptHashFromHash(hash, params)
	script32Addr, _ := bchutil.NewAddressScriptHash32FromHash(
		make([]byte, 32), params)
	legacyScriptAddr, _ := bchutil.NewLegacyAddressScriptHashFromHash(hash,
		params)
	slpAddr, _ := bchutil.NewSlpAddressPubKeyHash(hash, params)

	full := params.CashAddressPrefix + ":" + cashAddr.EncodeAddress()
	if full != "bitcoincash:qp63uahgrxged4z5jswyt5dn5v3lzsem6cy4spdc2h" {
		t.Fatalf("unexpected address %s", full)
	}

	const msg = "verifymessage test"
	sig := signTestMessage(t, key, msg, true)
	uncompressedSig := signTestMessage(t, key, msg, false)

	// Set an unused bit in the last character before the padding.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" +
		"0123456789+/"
	padBitsSig := []byte(sig)
	last := len(padBitsSig) - 2
	padBitsSig[last] = alphabet[strings.IndexByte(alphabet,
		padBitsSig[last])|1]

	tests := []struct {
		name    string
		address string
		sig     string
		msg     string
		want    bool
	}{
		{"cashaddr", full, sig, msg, true},
		{"cashaddr without prefix", cashAddr.EncodeAddress(), sig, msg, true},
		{"upper case cashaddr", strings.ToUpper(full), sig, msg, true},
		{"upper case cashaddr without prefix",
			strings.ToUpper(cashAddr.EncodeAddress()), sig, msg, true},
		{"token-aware cashaddr",
			"bitcoincash:zp63uahgrxged4z5jswyt5dn5v3lzsem6crlrlr74y", sig,
			msg, true},
		{"legacy", legacyAddr.EncodeAddress(), sig, msg, true},
		{"uncompressed key", uncompressedAddr.EncodeAddress(),
			uncompressedSig, msg, true},
		{"uncompressed key against compressed address", full,
			uncompressedSig, msg, false},
		{"compressed key against uncompressed address",
			uncompressedAddr.EncodeAddress(), sig, msg, false},
		{"wrong message", full, sig, "other message", false},
		{"wrong address", otherAddr.EncodeAddress(), sig, msg, false},
		{"bad signature", full, base64.StdEncoding.EncodeToString(
			[]byte("not a signature")), msg, false},
	}
	for _, test := range tests {
		cmd := btcjson.NewVerifyMessageCmd(test.address, test.sig, test.msg)
		result, err := handleVerifyMessage(s, cmd, nil)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		if result != test.want {
			t.Errorf("%s: got %v, want %v", test.name, result, test.want)
		}
	}

	errTests := []struct {
		name    string
		address string
		sig     string
		code    btcjson.RPCErrorCode
		message string
	}{
		{"bad address", "bitcoincash:qqqq", sig, btcjson.ErrRPCType,
			"Invalid address"},
		{"other network", testnetAddr.EncodeAddress(), sig,
			btcjson.ErrRPCType, "Invalid address"},
		{"slp", params.SlpAddressPrefix + ":" + slpAddr.EncodeAddress(), sig,
			btcjson.ErrRPCType, "Invalid address"},
		{"p2sh", scriptAddr.EncodeAddress(), sig, btcjson.ErrRPCType,
			"Address does not refer to key"},
		{"p2sh32", script32Addr.EncodeAddress(), sig, btcjson.ErrRPCType,
			"Address does not refer to key"},
		{"legacy p2sh", legacyScriptAddr.EncodeAddress(), sig,
			btcjson.ErrRPCType, "Address does not refer to key"},
		{"hex public key", "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce" +
			"28d959f2815b16f81798", sig, btcjson.ErrRPCType, "Invalid address"},
		{"malformed base64", full, "@@@", btcjson.ErrRPCInvalidAddressOrKey,
			"Malformed base64 encoding"},
		{"line break in base64", full, sig[:44] + "\n" + sig[44:],
			btcjson.ErrRPCInvalidAddressOrKey, "Malformed base64 encoding"},
		{"carriage return in base64", full, sig + "\r",
			btcjson.ErrRPCInvalidAddressOrKey, "Malformed base64 encoding"},
		{"non-zero padding bits", full, string(padBitsSig),
			btcjson.ErrRPCInvalidAddressOrKey, "Malformed base64 encoding"},
	}
	for _, test := range errTests {
		cmd := btcjson.NewVerifyMessageCmd(test.address, test.sig, msg)
		_, err := handleVerifyMessage(s, cmd, nil)
		var rpcErr *btcjson.RPCError
		if !errors.As(err, &rpcErr) {
			t.Errorf("%s: got %v, want an RPC error", test.name, err)
			continue
		}
		if rpcErr.Code != test.code || rpcErr.Message != test.message {
			t.Errorf("%s: got %d %q, want %d %q", test.name, rpcErr.Code,
				rpcErr.Message, test.code, test.message)
		}
	}
}

// TestHandleVerifyMessageBCHNVector checks a signature made by Bitcoin Cash
// Node's signmessage on regtest.
func TestHandleVerifyMessageBCHNVector(t *testing.T) {
	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: &chaincfg.RegressionNetParams,
	}}

	const (
		address = "bchreg:qr7agl23pve2a07sarvg9k0me2rrqqkvjsedqtt6gd"
		sig     = "IONd6ajn7k+e0I9e8zoXqyMwGxdyNH7NHCwRtBHE8DPtM2rWQdPn97qK" +
			"LQ7lFDostEy1XA6+/ANhK5sRF/ZRwyU="
		msg = "BCHD verifymessage vector"
	)
	tests := []struct {
		name    string
		address string
		msg     string
		want    bool
	}{
		{"cashaddr", address, msg, true},
		{"cashaddr without prefix", strings.TrimPrefix(address, "bchreg:"),
			msg, true},
		{"token-aware cashaddr",
			"bchreg:zr7agl23pve2a07sarvg9k0me2rrqqkvjs78n49uh7", msg, true},
		{"wrong message", address, "other message", false},
	}
	for _, test := range tests {
		cmd := btcjson.NewVerifyMessageCmd(test.address, sig, test.msg)
		result, err := handleVerifyMessage(s, cmd, nil)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		if result != test.want {
			t.Errorf("%s: got %v, want %v", test.name, result, test.want)
		}
	}
}
