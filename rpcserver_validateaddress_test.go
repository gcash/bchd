// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchutil"
)

// TestValidateAddressScript ensures validateaddress reports the script an
// address pays to and whether it is a script, as BCHN does, a public key
// getting the script of the pay-to-pubkey-hash address it is shown as, and
// that an invalid address gets neither.
func TestValidateAddressScript(t *testing.T) {
	params := &chaincfg.MainNetParams
	hash := bytes.Repeat([]byte{0x03}, 20)
	p2pkh, err := bchutil.NewAddressPubKeyHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh, err := bchutil.NewAddressScriptHashFromHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh32, err := bchutil.NewAddressScriptHash32FromHash(
		bytes.Repeat([]byte{0x03}, 32), params)
	if err != nil {
		t.Fatal(err)
	}
	legacyP2SH, err := bchutil.NewLegacyAddressScriptHashFromHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}
	_, pub := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{0x01}, 32))
	pubKey, err := bchutil.NewAddressPubKey(pub.SerializeCompressed(), params)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		address  string
		pays     bchutil.Address
		isScript bool
	}{
		{"p2pkh", p2pkh.EncodeCashAddress(true), p2pkh, false},
		{"p2sh", p2sh.EncodeAddress(), p2sh, true},
		{"p2sh32", p2sh32.EncodeAddress(), p2sh32, true},
		{"legacy p2sh", legacyP2SH.EncodeAddress(), legacyP2SH, true},
		{"public key", hex.EncodeToString(pub.SerializeCompressed()),
			pubKey.AddressPubKeyHash(), false},
	}
	s := &rpcServer{cfg: rpcserverConfig{ChainParams: params}}
	for _, test := range tests {
		cmd := &btcjson.ValidateAddressCmd{Address: test.address}
		result, err := handleValidateAddress(s, cmd, nil)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		reply := result.(btcjson.ValidateAddressChainResult)
		want, err := txscript.PayToAddrScript(test.pays)
		if err != nil {
			t.Fatal(err)
		}
		if reply.ScriptPubKey != hex.EncodeToString(want) {
			t.Errorf("%s: scriptPubKey %s, want %x", test.name,
				reply.ScriptPubKey, want)
		}
		if reply.IsScript == nil || *reply.IsScript != test.isScript {
			t.Errorf("%s: isscript %v, want %v", test.name,
				reply.IsScript, test.isScript)
		}
	}

	cmd := &btcjson.ValidateAddressCmd{Address: "notanaddress"}
	result, err := handleValidateAddress(s, cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reply := result.(btcjson.ValidateAddressChainResult); reply.ScriptPubKey != "" ||
		reply.IsScript != nil {

		t.Errorf("invalid address: unexpected result %+v", reply)
	}
}

// TestValidateAddressShownAsCashAddr ensures validateaddress shows a legacy
// address as the cashaddr of the same destination, as BCHN does, and a
// public key as the cashaddr it pays to, while a cashaddr is shown in the
// form given, token-aware or not.
func TestValidateAddressShownAsCashAddr(t *testing.T) {
	params := &chaincfg.MainNetParams
	hash := bytes.Repeat([]byte{0x05}, 20)
	p2pkh, err := bchutil.NewAddressPubKeyHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh, err := bchutil.NewAddressScriptHashFromHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}
	legacyP2PKH, err := bchutil.NewLegacyAddressPubKeyHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}
	legacyP2SH, err := bchutil.NewLegacyAddressScriptHashFromHash(hash, params)
	if err != nil {
		t.Fatal(err)
	}
	testnetP2PKH, err := bchutil.NewLegacyAddressPubKeyHash(hash,
		&chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}
	testnetP2SH, err := bchutil.NewLegacyAddressScriptHashFromHash(hash,
		&chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}
	_, pub := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{0x01}, 32))
	pubKeyHash, err := bchutil.NewAddressPubKeyHash(
		bchutil.Hash160(pub.SerializeCompressed()), params)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		address    string
		want       string
		tokenAware bool
	}{
		{"legacy p2pkh", legacyP2PKH.EncodeAddress(), p2pkh.EncodeAddress(), false},
		{"legacy p2sh", legacyP2SH.EncodeAddress(), p2sh.EncodeAddress(), false},
		{"public key", hex.EncodeToString(pub.SerializeCompressed()),
			pubKeyHash.EncodeAddress(), false},
		{"cashaddr", p2pkh.EncodeAddress(), p2pkh.EncodeAddress(), false},
		{"token-aware cashaddr", p2pkh.EncodeCashAddress(true),
			p2pkh.EncodeCashAddress(true), true},
		{"cashaddr with prefix", params.CashAddressPrefix + ":" +
			p2sh.EncodeAddress(), p2sh.EncodeAddress(), false},
		{"legacy p2pkh for testnet", testnetP2PKH.EncodeAddress(), "", false},
		{"legacy p2sh for testnet", testnetP2SH.EncodeAddress(), "", false},
	}
	s := &rpcServer{cfg: rpcserverConfig{ChainParams: params}}
	for _, test := range tests {
		cmd := &btcjson.ValidateAddressCmd{Address: test.address}
		result, err := handleValidateAddress(s, cmd, nil)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		reply := result.(btcjson.ValidateAddressChainResult)
		if reply.IsValid != (test.want != "") || reply.Address != test.want {
			t.Errorf("%s: isvalid %v, address %q, want %q", test.name,
				reply.IsValid, reply.Address, test.want)
		}
		if test.want != "" && (reply.IsTokenAware == nil ||
			*reply.IsTokenAware != test.tokenAware) {

			t.Errorf("%s: istokenaware %v, want %v", test.name,
				reply.IsTokenAware, test.tokenAware)
		}
	}
}
