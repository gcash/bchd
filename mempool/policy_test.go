// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestCalcMinRequiredTxRelayFee tests the calcMinRequiredTxRelayFee API.
func TestCalcMinRequiredTxRelayFee(t *testing.T) {
	tests := []struct {
		name     string         // test description.
		size     int64          // Transaction size in bytes.
		relayFee bchutil.Amount // minimum relay transaction fee.
		want     int64          // Expected fee.
	}{
		{
			// Ensure combination of size and fee that are less than 1000
			// produce a fee of one satoshi, as in BCHN.
			"250 bytes with relay fee of 3",
			250,
			3,
			1,
		},
		{
			"99 bytes with relay fee of 10",
			99,
			10,
			1,
		},
		{
			// The product overflows int64 into a small positive fee.
			"9000 bytes with max satoshi relay fee",
			9000,
			bchutil.MaxSatoshi,
			bchutil.MaxSatoshi,
		},
		{
			"100 bytes with default minimum relay fee",
			100,
			DefaultMinRelayTxFee,
			100,
		},
		{
			"max standard tx size with default minimum relay fee",
			maxStandardTxSize,
			DefaultMinRelayTxFee,
			100000,
		},
		{
			"max standard tx size with max satoshi relay fee",
			maxStandardTxSize,
			bchutil.MaxSatoshi,
			bchutil.MaxSatoshi,
		},
		{
			"1500 bytes with 5000 relay fee",
			1500,
			5000,
			7500,
		},
		{
			"1500 bytes with 3000 relay fee",
			1500,
			3000,
			4500,
		},
		{
			"782 bytes with 5000 relay fee",
			782,
			5000,
			3910,
		},
		{
			"782 bytes with 3000 relay fee",
			782,
			3000,
			2346,
		},
		{
			"782 bytes with 2550 relay fee",
			782,
			2550,
			1994,
		},
	}

	for _, test := range tests {
		got := calcMinRequiredTxRelayFee(test.size, test.relayFee)
		if got != test.want {
			t.Errorf("TestCalcMinRequiredTxRelayFee test '%s' "+
				"failed: got %v want %v", test.name, got,
				test.want)
			continue
		}
	}
}

// TestCheckPkScriptStandard tests the checkPkScriptStandard API.
func TestCheckPkScriptStandard(t *testing.T) {
	var pubKeys [][]byte
	for range 4 {
		pk, err := bchec.NewPrivateKey(bchec.S256())
		if err != nil {
			t.Fatalf("TestCheckPkScriptStandard NewPrivateKey failed: %v",
				err)
			return
		}
		pubKeys = append(pubKeys, pk.PubKey().SerializeCompressed())
	}

	tests := []struct {
		name       string // test description.
		script     *txscript.ScriptBuilder
		isStandard bool
	}{
		{
			"key1 and key2",
			txscript.NewScriptBuilder().AddOp(txscript.OP_2).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddOp(txscript.OP_2).AddOp(txscript.OP_CHECKMULTISIG),
			true,
		},
		{
			"key1 or key2",
			txscript.NewScriptBuilder().AddOp(txscript.OP_1).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddOp(txscript.OP_2).AddOp(txscript.OP_CHECKMULTISIG),
			true,
		},
		{
			"escrow",
			txscript.NewScriptBuilder().AddOp(txscript.OP_2).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddData(pubKeys[2]).
				AddOp(txscript.OP_3).AddOp(txscript.OP_CHECKMULTISIG),
			true,
		},
		{
			"one of four",
			txscript.NewScriptBuilder().AddOp(txscript.OP_1).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddData(pubKeys[2]).AddData(pubKeys[3]).
				AddOp(txscript.OP_4).AddOp(txscript.OP_CHECKMULTISIG),
			false,
		},
		{
			"malformed1",
			txscript.NewScriptBuilder().AddOp(txscript.OP_3).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddOp(txscript.OP_2).AddOp(txscript.OP_CHECKMULTISIG),
			false,
		},
		{
			"malformed2",
			txscript.NewScriptBuilder().AddOp(txscript.OP_2).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddOp(txscript.OP_3).AddOp(txscript.OP_CHECKMULTISIG),
			false,
		},
		{
			"malformed3",
			txscript.NewScriptBuilder().AddOp(txscript.OP_0).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddOp(txscript.OP_2).AddOp(txscript.OP_CHECKMULTISIG),
			false,
		},
		{
			"malformed4",
			txscript.NewScriptBuilder().AddOp(txscript.OP_1).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddOp(txscript.OP_0).AddOp(txscript.OP_CHECKMULTISIG),
			false,
		},
		{
			"malformed5",
			txscript.NewScriptBuilder().AddOp(txscript.OP_1).
				AddData(pubKeys[0]).AddData(pubKeys[1]).
				AddOp(txscript.OP_CHECKMULTISIG),
			false,
		},
		{
			"malformed6",
			txscript.NewScriptBuilder().AddOp(txscript.OP_1).
				AddData(pubKeys[0]).AddData(pubKeys[1]),
			false,
		},
	}

	for _, test := range tests {
		script, err := test.script.Script()
		if err != nil {
			t.Fatalf("TestCheckPkScriptStandard test '%s' "+
				"failed: %v", test.name, err)
		}
		scriptClass := txscript.GetScriptClass(script)
		got := checkPkScriptStandard(script, scriptClass, false)
		if (test.isStandard && got != nil) ||
			(!test.isStandard && got == nil) {

			t.Fatalf("TestCheckPkScriptStandard test '%s' failed",
				test.name)
			return
		}
		if reason := RejectReason(got); !test.isStandard &&
			reason != "scriptpubkey" {

			t.Errorf("TestCheckPkScriptStandard test '%s': got "+
				"reason %q, want scriptpubkey", test.name, reason)
		}
	}
}

// TestCheckPkScriptStandardTemplates ensures that once the May 2026 upgrade
// is active, only the multisig forms BCHN matches are held to the multisig
// limits, and other forms, and spends of them, are checked as pay to script.
func TestCheckPkScriptStandardTemplates(t *testing.T) {
	compressed := append([]byte{0x02}, bytes.Repeat([]byte{0x11}, 32)...)
	uncompressed := append([]byte{0x04}, bytes.Repeat([]byte{0x22}, 64)...)
	push := func(key []byte) []byte {
		return append([]byte{byte(len(key))}, key...)
	}
	pushData1 := func(key []byte) []byte {
		return append([]byte{txscript.OP_PUSHDATA1, byte(len(key))}, key...)
	}
	multiSig := func(m, n byte, keys ...[]byte) []byte {
		script := []byte{m}
		for _, key := range keys {
			script = append(script, key...)
		}
		return append(script, n, txscript.OP_CHECKMULTISIG)
	}
	c, u, u1 := push(compressed), push(uncompressed), pushData1(uncompressed)

	tests := []struct {
		name          string
		script        []byte
		isStandard    bool
		spendStandard bool
	}{
		{"one of four",
			multiSig(txscript.OP_1, txscript.OP_4, c, c, c, c),
			false, true},
		{"one of four with a non-minimal push",
			multiSig(txscript.OP_1, txscript.OP_4, c, c, c,
				pushData1(compressed)),
			true, true},
		{"three of two", multiSig(txscript.OP_3, txscript.OP_2, c, c),
			true, true},
		{"zero of two", multiSig(txscript.OP_0, txscript.OP_2, c, c),
			true, true},
		{"one of three uncompressed in 201 bytes",
			multiSig(txscript.OP_1, txscript.OP_3, u, u, u),
			true, true},
		{"one of three uncompressed in 204 bytes of non-minimal pushes",
			multiSig(txscript.OP_1, txscript.OP_3, u1, u1, u1),
			false, false},
	}

	for _, test := range tests {
		class := txscript.GetScriptClass(test.script)
		err := checkPkScriptStandard(test.script, class, true)
		if (err == nil) != test.isStandard {
			t.Errorf("%s: output check returned %v, want standard %v",
				test.name, err, test.isStandard)
		}
		if reason := RejectReason(err); !test.isStandard &&
			reason != "scriptpubkey" {

			t.Errorf("%s: output check reason %q, want scriptpubkey",
				test.name, reason)
		}

		prevTx := wire.NewMsgTx(1)
		prevTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0),
			nil))
		prevTx.AddTxOut(wire.NewTxOut(1000, test.script, wire.TokenData{}))
		prev := bchutil.NewTx(prevTx)
		view := blockchain.NewUtxoViewpoint()
		view.AddTxOut(prev, 0, 1)

		spendTx := wire.NewMsgTx(1)
		spendTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(prev.Hash(), 0), nil))
		spendTx.AddTxOut(wire.NewTxOut(500, []byte{txscript.OP_TRUE},
			wire.TokenData{}))
		err = checkInputsStandard(bchutil.NewTx(spendTx), view,
			txscript.ScriptAllowMay2026)
		if (err == nil) != test.spendStandard {
			t.Errorf("%s: input check returned %v, want standard %v",
				test.name, err, test.spendStandard)
		}
		if reason := RejectReason(err); !test.spendStandard &&
			reason != "bad-txns-nonstandard-inputs" {

			t.Errorf("%s: input check reason %q, want "+
				"bad-txns-nonstandard-inputs", test.name, reason)
		}
	}
}

// TestDust tests the isDust API.
func TestDust(t *testing.T) {
	pkScript := []byte{0x76, 0xa9, 0x21, 0x03, 0x2f, 0x7e, 0x43,
		0x0a, 0xa4, 0xc9, 0xd1, 0x59, 0x43, 0x7e, 0x84, 0xb9,
		0x75, 0xdc, 0x76, 0xd9, 0x00, 0x3b, 0xf0, 0x92, 0x2c,
		0xf3, 0xaa, 0x45, 0x28, 0x46, 0x4b, 0xab, 0x78, 0x0d,
		0xba, 0x5e, 0x88, 0xac}

	tests := []struct {
		name     string // test description
		txOut    wire.TxOut
		relayFee bchutil.Amount // minimum relay transaction fee.
		isDust   bool
	}{
		{
			// Any value is allowed with a zero relay fee.
			"zero value with zero relay fee",
			wire.TxOut{Value: 0, PkScript: pkScript},
			0,
			false,
		},
		{
			// Zero value is dust with any relay fee"
			"zero value with very small tx fee",
			wire.TxOut{Value: 0, PkScript: pkScript},
			1,
			true,
		},
		{
			"38 byte public key script with value 584",
			wire.TxOut{Value: 584, PkScript: pkScript},
			1000,
			true,
		},
		{
			"38 byte public key script with value 585",
			wire.TxOut{Value: 585, PkScript: pkScript},
			1000,
			false,
		},
		{
			// Maximum allowed value is never dust.
			"max satoshi amount is never dust",
			wire.TxOut{Value: bchutil.MaxSatoshi, PkScript: pkScript},
			bchutil.MaxSatoshi,
			false,
		},
		{
			// Maximum int64 value causes overflow.
			"maximum int64 value",
			wire.TxOut{Value: 1<<63 - 1, PkScript: pkScript},
			1<<63 - 1,
			true,
		},
	}
	for _, test := range tests {
		res := isDust(&test.txOut, test.relayFee)
		if res != test.isDust {
			t.Fatalf("Dust test '%s' failed: want %v got %v",
				test.name, test.isDust, res)
		}
	}
}

// TestCheckTransactionStandard tests the checkTransactionStandard API.
func TestCheckTransactionStandard(t *testing.T) {
	// Create some dummy, but otherwise standard, data for transactions.
	prevOutHash, err := chainhash.NewHashFromStr("01")
	if err != nil {
		t.Fatalf("NewShaHashFromStr: unexpected error: %v", err)
	}
	dummyPrevOut := wire.OutPoint{Hash: *prevOutHash, Index: 1}
	dummySigScript := bytes.Repeat([]byte{0x00}, 65)
	dummyTxIn := wire.TxIn{
		PreviousOutPoint: dummyPrevOut,
		SignatureScript:  dummySigScript,
		Sequence:         wire.MaxTxInSequenceNum,
	}
	addrHash := [20]byte{0x01}
	addr, err := bchutil.NewAddressPubKeyHash(addrHash[:],
		&chaincfg.TestNet3Params)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: unexpected error: %v", err)
	}
	dummyPkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("PayToAddrScript: unexpected error: %v", err)
	}
	dummyTxOut := wire.TxOut{
		Value:    100000000, // 1 BCH
		PkScript: dummyPkScript,
	}

	tests := []struct {
		name       string
		tx         wire.MsgTx
		height     int32
		isStandard bool
		code       wire.RejectCode
		reason     string
	}{
		{
			name: "Typical pay-to-pubkey-hash transaction",
			tx: wire.MsgTx{
				Version:  1,
				TxIn:     []*wire.TxIn{&dummyTxIn},
				TxOut:    []*wire.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: true,
		},
		{
			name: "Transaction version too high",
			tx: wire.MsgTx{
				Version:  wire.TxVersion + 1,
				TxIn:     []*wire.TxIn{&dummyTxIn},
				TxOut:    []*wire.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "version",
		},
		{
			name: "Transaction is not finalized",
			tx: wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: dummyPrevOut,
					SignatureScript:  dummySigScript,
					Sequence:         0,
				}},
				TxOut:    []*wire.TxOut{&dummyTxOut},
				LockTime: 300001,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "bad-txns-nonfinal",
		},
		{
			name: "Transaction size is too large",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value: 0,
					PkScript: bytes.Repeat([]byte{0x00},
						maxStandardTxSize+1),
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "tx-size",
		},
		{
			name: "Signature script size is too large",
			tx: wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: dummyPrevOut,
					SignatureScript: bytes.Repeat([]byte{0x00},
						maxStandardSigScriptSize+1),
					Sequence: wire.MaxTxInSequenceNum,
				}},
				TxOut:    []*wire.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "scriptsig-size",
		},
		{
			name: "Signature script that does more than push data",
			tx: wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: dummyPrevOut,
					SignatureScript: []byte{
						txscript.OP_CHECKSIGVERIFY},
					Sequence: wire.MaxTxInSequenceNum,
				}},
				TxOut:    []*wire.TxOut{&dummyTxOut},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "scriptsig-not-pushonly",
		},
		{
			name: "Valid but non standard public key script",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					PkScript: []byte{txscript.OP_TRUE},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "scriptpubkey",
		},
		{
			name: "More than one nulldata output",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value:    0,
					PkScript: []byte{txscript.OP_RETURN},
				}, {
					Value:    0,
					PkScript: []byte{txscript.OP_RETURN},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: true,
			code:       wire.RejectNonstandard,
		},
		{
			name: "Dust output",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value:    0,
					PkScript: dummyPkScript,
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectDust,
			reason:     "dust",
		},
		{
			name: "Token output before the upgrade",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					PkScript: dummyPkScript,
					TokenData: wire.TokenData{
						Amount:   1,
						BitField: 0x10,
					},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "txn-tokens-before-activation",
		},
		{
			name: "One nulldata output with 0 amount (standard)",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value:    0,
					PkScript: []byte{txscript.OP_RETURN},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: true,
		},
		{
			name: "One nulldata output with multiple data pushes (standard)",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value: 0,
					PkScript: []byte{
						txscript.OP_RETURN,
						txscript.OP_DATA_2, 0xff, 0xff,
						txscript.OP_DATA_3, 0xff, 0xff, 0xff,
						txscript.OP_PUSHDATA4, 0x01, 0x00, 0x00, 0x00,
						0x01,
					},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: true,
		},
		{
			name: "One nulldata output exceeding MaxDataCarrierSize (non-standard)",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value: 0,
					PkScript: []byte{
						txscript.OP_RETURN,
						txscript.OP_PUSHDATA1, 0xdd,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00,
					},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "oversize-op-return",
		},
		{
			name: "Multiple nulldata outputs exceeds MaxDataCarrierSize (non-standard)",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value: 0,
					PkScript: []byte{
						txscript.OP_RETURN,
						txscript.OP_PUSHDATA1, 0xdc,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00,
					},
				}, {
					Value: 0,
					PkScript: []byte{
						txscript.OP_RETURN,
					},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "oversize-op-return",
		},
		{
			name: "Multiple nulldata outputs are below MaxDataCarrierSize (standard)",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value: 0,
					PkScript: []byte{
						txscript.OP_RETURN,
						txscript.OP_PUSHDATA1, 0xdb,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00,
					},
				}, {
					Value: 0,
					PkScript: []byte{
						txscript.OP_RETURN,
					},
				}},
				LockTime: 0,
			},
			height:     300000,
			isStandard: true,
			code:       wire.RejectNonstandard,
		},
		{
			// Unspendable pkScript due to an invalid public key
			// script.
			name: "unspendable pkScript",
			tx: wire.MsgTx{
				Version: 1,
				TxIn:    []*wire.TxIn{&dummyTxIn},
				TxOut: []*wire.TxOut{{
					Value:    5000,
					PkScript: []byte{0x01},
				}},
				LockTime: 0,
			},
			height:     0, // no relay fee
			isStandard: false,
			code:       wire.RejectNonstandard,
			reason:     "scriptpubkey",
		},
	}

	pastMedianTime := time.Now()
	for _, test := range tests {
		// Ensure standardness is as expected.
		err := checkTransactionStandard(bchutil.NewTx(&test.tx),
			test.height, pastMedianTime, DefaultDustRelayFee, 1, false, false)
		if err == nil && test.isStandard {
			// Test passes since function returned standard for a
			// transaction which is intended to be standard.
			continue
		}
		if err == nil && !test.isStandard {
			t.Errorf("checkTransactionStandard (%s): standard when "+
				"it should not be", test.name)
			continue
		}
		if err != nil && test.isStandard {
			t.Errorf("checkTransactionStandard (%s): nonstandard "+
				"when it should not be: %v", test.name, err)
			continue
		}

		// Ensure error type is a TxRuleError inside of a RuleError.
		rerr, ok := err.(RuleError)
		if !ok {
			t.Errorf("checkTransactionStandard (%s): unexpected "+
				"error type - got %T", test.name, err)
			continue
		}
		txrerr, ok := rerr.Err.(TxRuleError)
		if !ok {
			t.Errorf("checkTransactionStandard (%s): unexpected "+
				"error type - got %T", test.name, rerr.Err)
			continue
		}

		// Ensure the reject code is the expected one.
		if txrerr.RejectCode != test.code {
			t.Errorf("checkTransactionStandard (%s): unexpected "+
				"error code - got %v, want %v", test.name,
				txrerr.RejectCode, test.code)
			continue
		}

		// Ensure the reason is BCHN's.
		if txrerr.Reason != test.reason {
			t.Errorf("checkTransactionStandard (%s): unexpected "+
				"reason - got %q, want %q", test.name,
				txrerr.Reason, test.reason)
		}
	}
}

// TestNonFinalStandardReason ensures a transaction that is not final is
// refused as nonstandard with BCHN's reason, which the mempool's own finality
// check gives when nonstandard transactions are accepted.
func TestNonFinalStandardReason(t *testing.T) {
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{Sequence: 0})
	tx.AddTxOut(wire.NewTxOut(1000, []byte{txscript.OP_TRUE}, wire.TokenData{}))
	tx.LockTime = 300001
	err := checkTransactionStandard(bchutil.NewTx(tx), 300000, time.Now(),
		DefaultDustRelayFee, 2, true, true)
	if err == nil || !strings.Contains(err.Error(), "bad-txns-nonfinal") {
		t.Fatalf("got %v, want a rejection with bad-txns-nonfinal", err)
	}
}

// TestUnspendableScriptStandard ensures that once the May 2026 upgrade is
// active, an output whose script starts with OP_RETURN but is not data
// carrier is standard pay to script with any value, as in BCHN, where an
// unspendable output is never dust, and that it is not counted as data
// carrier bytes.
func TestUnspendableScriptStandard(t *testing.T) {
	out := func(value int64, script ...byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}
	opReturnScript := func(size int) []byte {
		return append([]byte{txscript.OP_RETURN},
			bytes.Repeat([]byte{txscript.OP_NOP}, size-1)...)
	}
	nullData := append([]byte{txscript.OP_RETURN, txscript.OP_PUSHDATA1,
		txscript.MaxDataCarrierSize - 3},
		make([]byte, txscript.MaxDataCarrierSize-3)...)

	tests := []struct {
		name      string
		outputs   []*wire.TxOut
		upgrade12 bool
		code      wire.RejectCode // 0 when standard
	}{
		{"OP_RETURN and a non-push opcode",
			[]*wire.TxOut{out(0, txscript.OP_RETURN, txscript.OP_ADD)},
			true, 0},
		{"OP_RETURN and a truncated push",
			[]*wire.TxOut{out(0, txscript.OP_RETURN, txscript.OP_DATA_5, 1)},
			true, 0},
		{"OP_RETURN with a value",
			[]*wire.TxOut{out(1000, txscript.OP_RETURN, txscript.OP_ADD)},
			true, 0},
		{"OP_RETURN in 201 bytes",
			[]*wire.TxOut{out(0, opReturnScript(201)...)}, true, 0},
		{"OP_RETURN in 202 bytes",
			[]*wire.TxOut{out(0, opReturnScript(202)...)}, true,
			wire.RejectNonstandard},
		{"beside a full data carrier output",
			[]*wire.TxOut{out(0, nullData...),
				out(0, txscript.OP_RETURN, txscript.OP_ADD)},
			true, 0},
		{"before the upgrade",
			[]*wire.TxOut{out(0, txscript.OP_RETURN, txscript.OP_ADD)},
			false, wire.RejectNonstandard},
		{"spendable script without a value",
			[]*wire.TxOut{out(0, txscript.OP_TRUE)}, true, wire.RejectDust},
		{"spendable script with a value",
			[]*wire.TxOut{out(1000, txscript.OP_TRUE)}, true, 0},
	}

	for _, test := range tests {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}},
			Sequence:         wire.MaxTxInSequenceNum,
		})
		tx.TxOut = test.outputs
		err := checkTransactionStandard(bchutil.NewTx(tx), 300000,
			time.Now(), DefaultDustRelayFee, 2, true, test.upgrade12)
		code, _ := extractRejectCode(err)
		if (err == nil) != (test.code == 0) || err != nil &&
			code != test.code {

			want := "standard"
			if test.code != 0 {
				want = "reject code " + test.code.String()
			}
			t.Errorf("%s: got %v, want %s", test.name, err, want)
		}
	}
}
