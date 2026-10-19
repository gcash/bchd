// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// TestBitcoinNet ensures GetBlockchainInfo reports a distinct network type
// for each network in chaincfg.
func TestBitcoinNet(t *testing.T) {
	tests := []struct {
		params *chaincfg.Params
		want   pb.GetBlockchainInfoResponse_BitcoinNet
	}{
		{&chaincfg.MainNetParams, pb.GetBlockchainInfoResponse_MAINNET},
		{&chaincfg.RegressionNetParams, pb.GetBlockchainInfoResponse_REGTEST},
		{&chaincfg.TestNet3Params, pb.GetBlockchainInfoResponse_TESTNET3},
		{&chaincfg.SimNetParams, pb.GetBlockchainInfoResponse_SIMNET},
		{&chaincfg.TestNet4Params, pb.GetBlockchainInfoResponse_TESTNET4},
		{&chaincfg.ChipNetParams, pb.GetBlockchainInfoResponse_CHIPNET},
	}
	wants := make(map[pb.GetBlockchainInfoResponse_BitcoinNet]struct{})
	for _, test := range tests {
		wants[test.want] = struct{}{}
	}
	if len(wants) != len(pb.GetBlockchainInfoResponse_BitcoinNet_name) {
		t.Fatalf("%d network types tested, %d defined", len(wants),
			len(pb.GetBlockchainInfoResponse_BitcoinNet_name))
	}
	for _, test := range tests {
		got, err := bitcoinNet(test.params)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.params.Name, err)
			continue
		}
		if got != test.want {
			t.Errorf("%s: got %v, want %v", test.params.Name, got, test.want)
		}
	}

	unknown := chaincfg.MainNetParams
	if _, err := bitcoinNet(&unknown); status.Code(err) != codes.Internal {
		t.Errorf("unknown params: got %v, want an Internal error", err)
	}
}
