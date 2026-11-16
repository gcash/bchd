// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// newMempoolEntryTestServer returns an RPC server on a regtest chain and a
// mempool holding a parent, which spends the first coinbase, and a child,
// which spends the first output of the parent.  Each pays a fee of 10000
// satoshis.
func newMempoolEntryTestServer(t *testing.T) (s *rpcServer, parent, child *wire.MsgTx) {
	t.Helper()
	chain, db := newRPCTestChain(t)
	params := &chaincfg.RegressionNetParams
	first := mineTestBlock(t, chain, nil)
	for range params.CoinbaseMaturity {
		mineTestBlock(t, chain, nil)
	}
	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			MaxTxVersion:    2,
			AcceptNonStd:    true,
			MinRelayTxFee:   mempool.DefaultMinRelayTxFee,
			MaxOrphanTxs:    5,
			MaxOrphanTxSize: 1000,
		},
		ChainParams:   params,
		FetchUtxoView: chain.FetchUtxoView,
		BestHeight:    func() int32 { return chain.BestSnapshot().Height },
		MedianTimePast: func() time.Time {
			return chain.BestSnapshot().MedianTime
		},
		CalcSequenceLock: func(tx *bchutil.Tx, view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return chain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: chain.IsDeploymentActive,
	})
	s = &rpcServer{cfg: rpcserverConfig{
		ChainParams: params,
		Chain:       chain,
		DB:          db,
		TxMemPool:   pool,
	}}

	// The scripts are long enough for the transactions to pass the minimum
	// size, and the first coinbase pays to OP_TRUE.
	script := append(bytes.Repeat([]byte{txscript.OP_NOP}, 10), txscript.OP_TRUE)
	coinbase := first.Transactions()[0]
	value := coinbase.MsgTx().TxOut[0].Value
	parent = wire.NewMsgTx(1)
	parent.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: *coinbase.Hash()}, nil))
	parent.AddTxOut(wire.NewTxOut(value/2, script, wire.TokenData{}))
	parent.AddTxOut(wire.NewTxOut(value/2-10000, script, wire.TokenData{}))
	child = wire.NewMsgTx(1)
	child.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: parent.TxHash()}, nil))
	child.AddTxOut(wire.NewTxOut(value/2-10000, script, wire.TokenData{}))
	for _, tx := range []*wire.MsgTx{parent, child} {
		if _, err := pool.ProcessTransaction(bchutil.NewTx(tx), false, 0); err != nil {
			t.Fatalf("transaction not accepted: %v", err)
		}
	}
	return s, parent, child
}

// TestGetMempoolEntry ensures getmempoolentry gives the entry of a mempool
// transaction that getrawmempool gives, with BCHN's fees and spentby beside
// bchd's own fields, that a limited user may call it, and that a transaction
// not in the mempool gets BCHN's error.
func TestGetMempoolEntry(t *testing.T) {
	s, parent, child := newMempoolEntryTestServer(t)
	parentID, childID := parent.TxHash().String(), child.TxHash().String()

	verbose := true
	all, err := handleGetRawMempool(s, btcjson.NewGetRawMempoolCmd(&verbose), nil)
	if err != nil {
		t.Fatal(err)
	}
	entries := all.(map[string]*btcjson.GetRawMempoolVerboseResult)
	fee := bchutil.Amount(10000).ToBCH()
	tests := []struct {
		txid    string
		depends []string
		spentBy []string
	}{
		{parentID, []string{}, []string{childID}},
		{childID, []string{parentID}, []string{}},
	}
	for _, test := range tests {
		result, err := handleGetMempoolEntry(s,
			btcjson.NewGetMempoolEntryCmd(test.txid), nil)
		if err != nil {
			t.Fatalf("%s: %v", test.txid, err)
		}
		entry := result.(*btcjson.GetRawMempoolVerboseResult)
		if !reflect.DeepEqual(entry, entries[test.txid]) {
			t.Errorf("%s: entry %+v, getrawmempool entry %+v",
				test.txid, entry, entries[test.txid])
		}
		if entry.Fees.Base != fee || entry.Fees.Modified != fee ||
			entry.Fee != fee ||
			!slices.Equal(entry.Depends, test.depends) ||
			!slices.Equal(entry.SpentBy, test.spentBy) {

			t.Errorf("%s: unexpected entry %+v", test.txid, entry)
		}
	}

	// Through the RPC server, as a limited user, the entry has BCHN's
	// fields in its JSON object, and an empty spentby is an empty array.
	reply := s.processRequest(&btcjson.Request{
		Jsonrpc: "1.0",
		Method:  "getmempoolentry",
		Params:  []json.RawMessage{json.RawMessage(`"` + childID + `"`)},
		ID:      json.RawMessage("1"),
	}, false, nil)
	for _, field := range []string{`"fees":{"base":0.0001,"modified":0.0001}`,
		`"spentby":[]`, `"depends":["` + parentID + `"]`, `"error":null`} {

		if !bytes.Contains(reply, []byte(field)) {
			t.Errorf("reply %s lacks %s", reply, field)
		}
	}

	_, err = handleGetMempoolEntry(s,
		btcjson.NewGetMempoolEntryCmd(chainhash.Hash{1}.String()), nil)
	assertRPCError(t, "not in the mempool", err,
		btcjson.ErrRPCInvalidAddressOrKey, "Transaction not in mempool")
	_, err = handleGetMempoolEntry(s, btcjson.NewGetMempoolEntryCmd("zz"), nil)
	if rpcErr, ok := err.(*btcjson.RPCError); !ok ||
		rpcErr.Code != btcjson.ErrRPCDecodeHexString {

		t.Errorf("txid not hex: got %v, want a decode error", err)
	}
}

// TestGetMempoolInfo ensures getmempoolinfo gives BCHN's fields, but for
// usage and maxmempool, which bchd does not have, with the minimum relay fee
// rate as the minimum fee rate of the mempool, which has no size limit.
func TestGetMempoolInfo(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })
	cfg = &config{minRelayTxFee: 2000}

	s, parent, child := newMempoolEntryTestServer(t)
	result, err := handleGetMempoolInfo(s, &btcjson.GetMempoolInfoCmd{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := result.(*btcjson.GetMempoolInfoResult)
	want := &btcjson.GetMempoolInfoResult{
		Loaded: true,
		Size:   2,
		Bytes: int64(parent.SerializeSize() +
			child.SerializeSize()),
		MempoolMinFee:      bchutil.Amount(2000).ToBCH(),
		MinRelayTxFee:      bchutil.Amount(2000).ToBCH(),
		PermitBareMultisig: true,
		MaxDataCarrierSize: 223,
		TotalFee:           bchutil.Amount(20000).ToBCH(),
	}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("got %+v, want %+v", info, want)
	}

	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"usage"`)) ||
		bytes.Contains(raw, []byte(`"maxmempool"`)) {

		t.Errorf("unexpected fields in %s", raw)
	}
}
