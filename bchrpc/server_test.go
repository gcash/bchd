// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestMarshalTransactionScriptClass ensures the script class of the outputs
// of a transaction is named as in the RPC results.
func TestMarshalTransactionScriptClass(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"p2pkh", "76a914" + hex.EncodeToString(bytes.Repeat([]byte{0xab}, 20)) +
			"88ac", "pubkeyhash"},
		{"p2sh32", "aa20" + hex.EncodeToString(bytes.Repeat([]byte{0xab}, 32)) +
			"87", "scripthash"},
		{"bare script", "51935287", "script"},
		{"does not parse", "0501", "script"},
		{"nulldata", "6a0401020304", "datacarrier"},
		{"long nulldata", "6a4d2c01" +
			hex.EncodeToString(make([]byte, 300)), "datacarrier"},
		{"long script", hex.EncodeToString(bytes.Repeat([]byte{0x51}, 202)),
			"nonstandard"},
	}

	mtx := wire.NewMsgTx(2)
	mtx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil))
	for _, test := range tests {
		script, err := hex.DecodeString(test.script)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		mtx.AddTxOut(wire.NewTxOut(1000, script, wire.TokenData{}))
	}

	s := &GrpcServer{chainParams: &chaincfg.MainNetParams}
	tx := marshalTransaction(bchutil.NewTx(mtx), 0, nil, 0, s)
	for i, test := range tests {
		if got := tx.Outputs[i].ScriptClass; got != test.want {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}
