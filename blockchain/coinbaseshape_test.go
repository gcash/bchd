// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// bip34VectorFile is the layout of the CHIP-2025-03 coinbase shape vectors.
type bip34VectorFile struct {
	Constants map[string]string `json:"constants"`
	Vectors   []struct {
		Name     string `json:"name"`
		Coinbase struct {
			Version  string `json:"version"`
			LockTime string `json:"locktime"`
			ValueOut string `json:"valueOut"`
			Sequence string `json:"sequence"`
		} `json:"coinbase"`
		Block struct {
			Height           string `json:"height"`
			LastExemptHeight string `json:"lastExemptHeight"`
		} `json:"block"`
		Expected struct {
			Invalid bool `json:"invalid"`
		} `json:"expected"`
	} `json:"vectors"`
	TxidVectors []struct {
		Name        string `json:"name"`
		Txid        string `json:"txid"`
		Height      string `json:"height"`
		DefaultBans bool   `json:"defaultBans"`
		Expected    struct {
			Invalid bool `json:"invalid"`
		} `json:"expected"`
	} `json:"txidVectors"`
	RawCoinbase struct {
		Hex        string `json:"hex"`
		Txid       string `json:"txid"`
		HeightPush string `json:"heightPush"`
		Parsed     struct {
			Version  string `json:"version"`
			LockTime string `json:"locktime"`
			ValueOut string `json:"valueOut"`
			Sequence string `json:"sequence"`
		} `json:"parsed"`
	} `json:"rawCoinbase"`
}

// shapeTestCoinbase returns a coinbase with the passed fields and one output
// paying value.
func shapeTestCoinbase(version int32, lockTime, sequence uint32,
	value int64) *bchutil.Tx {

	tx := wire.NewMsgTx(version)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{0x51, 0x51},
		Sequence:         sequence,
	})
	tx.AddTxOut(wire.NewTxOut(value, []byte{0x51}, wire.TokenData{}))
	tx.LockTime = lockTime
	return bchutil.NewTx(tx)
}

// isShapeError returns whether err is a rejection of the coinbase shape, and
// fails the test on any other error.
func isShapeError(t *testing.T, name string, err error) bool {
	t.Helper()
	if err == nil {
		return false
	}
	if rerr, ok := err.(RuleError); !ok || rerr.ErrorCode != ErrBadCoinbaseShape {
		t.Fatalf("%s: unexpected error %v", name, err)
	}
	return true
}

// TestCoinbaseShapeVectors runs the CHIP-2025-03 coinbase shape vectors.
// Rows gated at the mainnet height run against mainnet, where the upgrade is
// active far below it, and the others against regtest with the activation
// block at their last exempt height.
func TestCoinbaseShapeVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "chip202503",
		"bip34", "bip34.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vectors bip34VectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	num := func(s string) int64 {
		v, err := strconv.ParseUint(s, 0, 63)
		if err != nil {
			t.Fatalf("bad number %q: %v", s, err)
		}
		return int64(v)
	}
	c := vectors.Constants
	if num(c["MIN_FORBIDDEN_VALUE_SATS"]) != forbiddenCoinbaseValue ||
		num(c["FORBIDDEN_SEQUENCE"]) != int64(wire.MaxTxInSequenceNum) ||
		num(c["MAINNET_SHAPE_LAST_EXEMPT"]) != coinbaseShapeMainnetHeight-1 ||
		num(c["MAINNET_NAMED_TXID_HEIGHT"]) != coinbaseBanHeight {

		t.Fatalf("constants %v", c)
	}

	if len(vectors.Vectors) != 61 || len(vectors.TxidVectors) != 6 {
		t.Fatalf("%d shape vectors and %d txid vectors, want 61 and 6",
			len(vectors.Vectors), len(vectors.TxidVectors))
	}
	for _, v := range vectors.Vectors {
		height := int32(num(v.Block.Height))
		lastExempt := int32(num(v.Block.LastExemptHeight))
		params := &chaincfg.RegressionNetParams
		active := height > lastExempt
		if lastExempt == coinbaseShapeMainnetHeight-1 {
			params = &chaincfg.MainNetParams
			active = true
		}
		cb := shapeTestCoinbase(int32(num(v.Coinbase.Version)),
			uint32(num(v.Coinbase.LockTime)),
			uint32(num(v.Coinbase.Sequence)), num(v.Coinbase.ValueOut))
		err := checkCoinbaseShape(params, cb, height, active)
		if got := isShapeError(t, v.Name, err); got != v.Expected.Invalid {
			t.Errorf("%s: invalid %v, want %v", v.Name, got,
				v.Expected.Invalid)
		}
	}

	// The coinbase of block 164384 has the forbidden shape, so whether it
	// is invalid on mainnet only depends on the exemption.  BCHD has no
	// configurable ban list, so rows with their own list do not apply.
	checked := 0
	for _, v := range vectors.TxidVectors {
		if !v.DefaultBans {
			continue
		}
		checked++
		hash, err := chainhash.NewHashFromStr(v.Txid)
		if err != nil {
			t.Fatalf("%s: bad txid: %v", v.Name, err)
		}
		height := int32(num(v.Height))
		exempt := coinbaseShapeExempt(&chaincfg.MainNetParams, hash, height)
		if exempt == v.Expected.Invalid {
			t.Errorf("%s: exempt %v, want invalid %v", v.Name, exempt,
				v.Expected.Invalid)
		}
	}
	if checked != 4 {
		t.Fatalf("checked %d txid vectors, want 4", checked)
	}

	raw := vectors.RawCoinbase
	b, err := hex.DecodeString(raw.Hex)
	if err != nil {
		t.Fatalf("bad raw coinbase: %v", err)
	}
	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(b)); err != nil {
		t.Fatalf("deserialize raw coinbase: %v", err)
	}
	cb := bchutil.NewTx(&msgTx)
	if cb.Hash().String() != raw.Txid || !cb.Hash().IsEqual(bannedCoinbaseHash) {
		t.Fatalf("raw coinbase hash %v, want %v", cb.Hash(), raw.Txid)
	}
	var value int64
	for _, txOut := range msgTx.TxOut {
		value += txOut.Value
	}
	if int64(msgTx.Version) != num(raw.Parsed.Version) ||
		int64(msgTx.LockTime) != num(raw.Parsed.LockTime) ||
		int64(msgTx.TxIn[0].Sequence) != num(raw.Parsed.Sequence) ||
		value != num(raw.Parsed.ValueOut) ||
		!hasForbiddenCoinbaseShape(&msgTx) {

		t.Fatalf("raw coinbase fields %v", raw.Parsed)
	}
	height, err := ExtractCoinbaseHeight(cb)
	if err != nil || int64(height) != num(raw.HeightPush) ||
		height != coinbaseBanHeight {

		t.Fatalf("raw coinbase height %d (%v), want %s", height, err,
			raw.HeightPush)
	}

	// The raw coinbase itself is only rejected at the ban height once the
	// upgrade is active.
	shapeTests := []struct {
		height  int32
		active  bool
		invalid bool
	}{
		{coinbaseBanHeight - 1, true, false},
		{coinbaseBanHeight, true, true},
		{coinbaseBanHeight + 1, true, false},
		{coinbaseBanHeight, false, false},
	}
	for _, test := range shapeTests {
		name := fmt.Sprintf("raw coinbase at %d, active %v", test.height,
			test.active)
		err := checkCoinbaseShape(&chaincfg.MainNetParams, cb, test.height,
			test.active)
		if got := isShapeError(t, name, err); got != test.invalid {
			t.Errorf("%s: rejected %v, want %v", name, got, test.invalid)
		}
	}
}

// TestCoinbaseShapeExemption checks the mainnet exemption at its boundaries,
// and that no other network has it.
func TestCoinbaseShapeExemption(t *testing.T) {
	cb := shapeTestCoinbase(1, 0, wire.MaxTxInSequenceNum,
		forbiddenCoinbaseValue)
	if !hasForbiddenCoinbaseShape(cb.MsgTx()) {
		t.Fatal("test coinbase lacks the forbidden shape")
	}

	mainnet := &chaincfg.MainNetParams
	tests := []struct {
		name    string
		params  *chaincfg.Params
		height  int32
		banned  bool
		active  bool
		invalid bool
	}{
		{"below ban", mainnet, 1983701, true, true, false},
		{"ban", mainnet, 1983702, true, true, true},
		{"ban, not active", mainnet, 1983702, true, false, false},
		{"ban height, other coinbase", mainnet, 1983702, false, true, false},
		{"above ban", mainnet, 1983703, true, true, false},
		{"last exempt", mainnet, 3708178, false, true, false},
		{"first governed", mainnet, 3708179, false, true, true},
		{"first governed, banned", mainnet, 3708179, true, true, true},
		{"first governed, not active", mainnet, 3708179, false, false, false},
		{"chipnet", &chaincfg.ChipNetParams, 1983701, false, true, true},
		{"testnet3", &chaincfg.TestNet3Params, 1983701, false, true, true},
		{"testnet4", &chaincfg.TestNet4Params, 1983701, false, true, true},
		{"regtest", &chaincfg.RegressionNetParams, 1, false, true, true},
		{"simnet", &chaincfg.SimNetParams, 1, false, true, true},
	}
	for _, test := range tests {
		if !test.banned {
			err := checkCoinbaseShape(test.params, cb, test.height,
				test.active)
			if got := isShapeError(t, test.name, err); got != test.invalid {
				t.Errorf("%s: rejected %v, want %v", test.name,
					got, test.invalid)
			}
			continue
		}

		// The banned coinbase has the forbidden shape, see the
		// vectors, so only the exemption decides.
		exempt := coinbaseShapeExempt(test.params, bannedCoinbaseHash,
			test.height)
		if invalid := test.active && !exempt; invalid != test.invalid {
			t.Errorf("%s: invalid %v, want %v", test.name, invalid,
				test.invalid)
		}
	}
}

// TestUpgrade2027CoinbaseShape connects regtest blocks across the activation
// block, 140, with coinbases that collect a 20 BCH fee.  A coinbase of the
// forbidden shape is accepted in the activation block and rejected after it,
// where each escape is accepted.  The coinbase after the activation block
// pays 5 + 20 BCH, exactly the least a forbidden coinbase pays.  Without the
// upgrade the forbidden shape is accepted.
func TestUpgrade2027CoinbaseShape(t *testing.T) {
	tests := []struct {
		name       string
		activation uint64
	}{
		// Block 140 is the first whose median time past reaches
		// this time.
		{"active", 1700000000 + 600*134},
		{"never", upgrade2027Never},
	}
	const fee = 20 * bchutil.SatoshiPerBitcoin
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := chaincfg.RegressionNetParams
			params.Upgrade2027ActivationTime = test.activation
			m := newRuleTestMiner(t, &params)
			coinbases := make(map[int32]*wire.MsgTx)
			for m.height < 139 {
				block := m.build(nil)
				m.mustAccept(block)
				coinbases[m.height] = block.MsgBlock().Transactions[0]
			}

			// feeTx spends the coinbase of block height-100 and
			// pays the fee.
			feeTx := func(height int32) *wire.MsgTx {
				cb := coinbases[height-100]
				return ruleTestSpend(wire.OutPoint{Hash: cb.TxHash()},
					cb.TxOut[0].Value-fee, byte(height))
			}
			// shaped returns the coinbase of the next block with
			// the forbidden shape, collecting the fee.
			shaped := func() *wire.MsgTx {
				cb := m.coinbase(m.height + 1)
				cb.TxOut[0].Value += fee
				if !hasForbiddenCoinbaseShape(cb) {
					t.Fatalf("block %d: coinbase lacks the "+
						"forbidden shape", m.height+1)
				}
				return cb
			}

			m.mustAccept(m.build(shaped(), feeTx(140)))
			if test.activation == upgrade2027Never {
				m.mustAccept(m.build(shaped(), feeTx(141)))
				return
			}
			escapes := []struct {
				name   string
				escape func(cb *wire.MsgTx)
			}{
				{"version", func(cb *wire.MsgTx) { cb.Version = 2 }},
				{"sequence", func(cb *wire.MsgTx) {
					cb.TxIn[0].Sequence = wire.MaxTxInSequenceNum - 1
				}},
				{"lock time", func(cb *wire.MsgTx) { cb.LockTime = 1 }},
				{"value", func(cb *wire.MsgTx) { cb.TxOut[0].Value-- }},
			}
			for _, e := range escapes {
				height := m.height + 1
				err := m.accept(m.build(shaped(), feeTx(height)))
				requireRuleError(t, err, ErrBadCoinbaseShape)
				cb := shaped()
				e.escape(cb)
				if err := m.accept(m.build(cb, feeTx(height))); err != nil {
					t.Fatalf("%s escape at block %d: %v", e.name,
						height, err)
				}
			}
		})
	}
}
