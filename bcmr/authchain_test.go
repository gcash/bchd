// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bcmr

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// tokenOut returns an output with token data.
func tokenOut(category chainhash.Hash, amount uint64, bitField byte) *wire.TxOut {
	return wire.NewTxOut(1000, []byte{txscript.OP_TRUE}, wire.TokenData{
		CategoryID: category, Amount: amount, BitField: bitField,
	})
}

// spending returns a transaction that spends the passed outpoints and has the
// passed outputs.
func spending(ins []wire.OutPoint, outs ...*wire.TxOut) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	for i := range ins {
		tx.AddTxIn(wire.NewTxIn(&ins[i], nil))
	}
	for _, out := range outs {
		tx.AddTxOut(out)
	}
	return tx
}

// TestGeneses checks the token categories that transactions create, with their
// genesis supply and the input that spends the authbase's identity output.
func TestGeneses(t *testing.T) {
	a, b, c := chainhash.Hash{1}, chainhash.Hash{2}, chainhash.Hash{3}
	ft := byte(wire.HAS_AMOUNT)
	nft := byte(wire.HAS_NFT | wire.MINTING)

	tests := []struct {
		name string
		tx   *wire.MsgTx
		want []Genesis
	}{{
		name: "no token",
		tx:   spending([]wire.OutPoint{{Hash: a}}, wire.NewTxOut(1, nil, wire.TokenData{})),
	}, {
		name: "a transfer, which spends no output 0 of the category's ID",
		tx:   spending([]wire.OutPoint{{Hash: a, Index: 1}}, tokenOut(a, 5, ft)),
	}, {
		name: "fungible tokens over two outputs",
		tx: spending([]wire.OutPoint{{Hash: c, Index: 1}, {Hash: a}},
			tokenOut(a, 5, ft), tokenOut(a, 7, ft)),
		want: []Genesis{{Category: a, Input: 1, Supply: 12}},
	}, {
		name: "an NFT only",
		tx:   spending([]wire.OutPoint{{Hash: a}}, tokenOut(a, 0, nft)),
		want: []Genesis{{Category: a, Input: 0}},
	}, {
		name: "two categories, and a transfer of a third",
		tx: spending([]wire.OutPoint{{Hash: b}, {Hash: c, Index: 2}, {Hash: a}},
			tokenOut(c, 1, ft), tokenOut(b, 3, ft), tokenOut(a, 4, ft|nft)),
		want: []Genesis{
			{Category: b, Input: 0, Supply: 3},
			{Category: a, Input: 2, Supply: 4},
		},
	}, {
		name: "a transfer between the outputs of a genesis",
		tx: spending([]wire.OutPoint{{Hash: c, Index: 1}, {Hash: a}},
			tokenOut(a, 2, ft), tokenOut(c, 1, ft), tokenOut(a, 3, ft), tokenOut(c, 1, ft)),
		want: []Genesis{{Category: a, Input: 1, Supply: 5}},
	}}
	for _, test := range tests {
		if got := Geneses(test.tx); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %+v, want %+v", test.name, got, test.want)
		}
	}
}

// TestFindGenesis checks that the genesis of a category is found among a
// transaction's, and that a category it only transfers isn't.
func TestFindGenesis(t *testing.T) {
	a, b, c := chainhash.Hash{1}, chainhash.Hash{2}, chainhash.Hash{3}
	ft := byte(wire.HAS_AMOUNT)
	tx := spending([]wire.OutPoint{{Hash: b}, {Hash: c, Index: 2}, {Hash: a}},
		tokenOut(c, 1, ft), tokenOut(b, 3, ft), tokenOut(a, 4, ft))
	if g, ok := FindGenesis(tx, a); !ok || g != (Genesis{Category: a, Input: 2, Supply: 4}) {
		t.Errorf("a: %+v, %v", g, ok)
	}
	if g, ok := FindGenesis(tx, b); !ok || g != (Genesis{Category: b, Input: 0, Supply: 3}) {
		t.Errorf("b: %+v, %v", g, ok)
	}
	if g, ok := FindGenesis(tx, c); ok {
		t.Errorf("a transfer of c: %+v", g)
	}
}

// TestIsBurn checks which identity outputs burn an identity.
func TestIsBurn(t *testing.T) {
	tests := []struct {
		name   string
		script []byte
		want   bool
	}{
		{"a data-carrier output", []byte{txscript.OP_RETURN, txscript.OP_DATA_1, 1}, true},
		{"OP_RETURN alone", []byte{txscript.OP_RETURN}, true},
		{"an empty locking bytecode", nil, false},
		{"anyone can spend", []byte{txscript.OP_TRUE}, false},
		{"OP_RETURN after another opcode", []byte{txscript.OP_FALSE, txscript.OP_RETURN}, false},
	}
	for _, test := range tests {
		tx := spending(nil, wire.NewTxOut(0, test.script, wire.TokenData{}))
		if got := IsBurn(tx); got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
	if IsBurn(wire.NewMsgTx(2)) {
		t.Error("a transaction without outputs burns")
	}
}

// TestTokenPrefixedOutputs checks the rules on a transaction read from its
// serialized form, whose outputs carry token prefixes, which the wire format
// splits off before the locking bytecode: a genesis of ten tokens on each of
// two outputs, whose identity output is a publication that burns the identity.
func TestTokenPrefixedOutputs(t *testing.T) {
	authbase := strings.Repeat("11", 32)
	tokens := "ef" + authbase + "10" + "0a"
	raw := "02000000" + "01" + authbase + "00000000" + "00" + "ffffffff" + "02" +
		"0000000000000000" + "4a" + tokens + "6a0442434d52" + "20" + testHash +
		"0000000000000000" + "3c" + tokens + p2pkh +
		"00000000"
	serialized, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	var tx wire.MsgTx
	if err := tx.Deserialize(bytes.NewReader(serialized)); err != nil {
		t.Fatal(err)
	}

	if !IsBurn(&tx) {
		t.Error("the identity output doesn't burn")
	}
	pub := ParsePublication(&tx)
	if pub == nil || pub.Output != 0 || pub.Malformed || hex.EncodeToString(pub.Hash[:]) != testHash {
		t.Errorf("publication %+v, want a well-formed one at output 0", pub)
	}
	var category chainhash.Hash
	for i := range category {
		category[i] = 0x11
	}
	want := []Genesis{{Category: category, Input: 0, Supply: 20}}
	if got := Geneses(&tx); !reflect.DeepEqual(got, want) {
		t.Errorf("geneses %+v, want %+v", got, want)
	}
}
