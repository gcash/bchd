// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestIsNonstandardTransaction ensures pay to script outputs only count as
// standard for checkpoint candidates once the May 2026 upgrade is active.
func TestIsNonstandardTransaction(t *testing.T) {
	p2pkh := append(append([]byte{txscript.OP_DUP, txscript.OP_HASH160,
		txscript.OP_DATA_20}, make([]byte, 20)...),
		txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)

	tests := []struct {
		name       string
		script     []byte
		wantBefore bool
		wantAfter  bool
	}{
		{"p2pkh", p2pkh, false, false},
		{"bare script", []byte{txscript.OP_1, txscript.OP_ADD,
			txscript.OP_2, txscript.OP_EQUAL}, true, false},
		{"max length", bytes.Repeat([]byte{txscript.OP_1},
			txscript.MaxP2SScriptSize), true, false},
		{"past max length", bytes.Repeat([]byte{txscript.OP_1},
			txscript.MaxP2SScriptSize+1), true, true},
	}

	for _, test := range tests {
		mtx := wire.NewMsgTx(2)
		mtx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0),
			nil))
		mtx.AddTxOut(wire.NewTxOut(1000, p2pkh, wire.TokenData{}))
		mtx.AddTxOut(wire.NewTxOut(1000, test.script, wire.TokenData{}))
		tx := bchutil.NewTx(mtx)

		if got := isNonstandardTransaction(tx, false); got != test.wantBefore {
			t.Errorf("%s: got %v before the upgrade, want %v",
				test.name, got, test.wantBefore)
		}
		if got := isNonstandardTransaction(tx, true); got != test.wantAfter {
			t.Errorf("%s: got %v after the upgrade, want %v",
				test.name, got, test.wantAfter)
		}
	}
}
