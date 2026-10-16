// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// rescanTestSpends holds signature scripts and the outputs they spend for
// the rescan tests.
type rescanTestSpends struct {
	p2pkhAddr, p2sh32Addr, tokenAddr, otherAddr bchutil.Address

	// Schnorr signed P2PKH, P2SH32, token P2PKH and unwatched spends.
	p2pkhSig, p2sh32Sig, tokenSig, otherSig []byte
	p2pkhOut, p2sh32Out, tokenOut, otherOut blockchain.SpentTxOut
}

func newRescanTestSpends(t *testing.T, params *chaincfg.Params) *rescanTestSpends {
	t.Helper()

	pushes := func(data ...[]byte) []byte {
		b := txscript.NewScriptBuilder()
		for _, d := range data {
			b.AddData(d)
		}
		script, err := b.Script()
		if err != nil {
			t.Fatalf("unable to build script: %v", err)
		}
		return script
	}
	payTo := func(addr bchutil.Address) []byte {
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			t.Fatalf("unable to create script: %v", err)
		}
		return script
	}
	p2pkhAddr := func(seed byte) (bchutil.Address, []byte) {
		priv, _ := bchec.PrivKeyFromBytes(bchec.S256(),
			bytes.Repeat([]byte{seed}, 32))
		pubKey := priv.PubKey().SerializeCompressed()
		addr, err := bchutil.NewAddressPubKeyHash(
			bchutil.Hash160(pubKey), params)
		if err != nil {
			t.Fatalf("unable to create address: %v", err)
		}
		return addr, pubKey
	}
	schnorrSig := bytes.Repeat([]byte{0x01}, 65)

	var s rescanTestSpends
	var pubKey []byte
	s.p2pkhAddr, pubKey = p2pkhAddr(1)
	s.p2pkhSig = pushes(schnorrSig, pubKey)
	s.p2pkhOut = blockchain.SpentTxOut{PkScript: payTo(s.p2pkhAddr)}

	redeemScript := []byte{txscript.OP_TRUE}
	addr32, err := bchutil.NewAddressScriptHash32(redeemScript, params)
	if err != nil {
		t.Fatalf("unable to create address: %v", err)
	}
	s.p2sh32Addr = addr32
	s.p2sh32Sig = pushes(redeemScript)
	s.p2sh32Out = blockchain.SpentTxOut{PkScript: payTo(addr32)}

	// The spend journal stores token data in front of the script.
	amount := uint64(1)
	tokenData, err := wire.NewTokenData([32]byte{1}, &amount, nil, nil)
	if err != nil {
		t.Fatalf("unable to create token data: %v", err)
	}
	s.tokenAddr, pubKey = p2pkhAddr(2)
	buf := tokenData.TokenDataBuffer()
	buf.Write(payTo(s.tokenAddr))
	s.tokenSig = pushes(schnorrSig, pubKey)
	s.tokenOut = blockchain.SpentTxOut{PkScript: buf.Bytes()}

	s.otherAddr, pubKey = p2pkhAddr(3)
	s.otherSig = pushes(schnorrSig, pubKey)
	s.otherOut = blockchain.SpentTxOut{PkScript: payTo(s.otherAddr)}

	return &s
}

// TestSpendsWatchedAddr ensures spends are matched by the address of the
// output they spend, and by the signature script when that output isn't
// known.
func TestSpendsWatchedAddr(t *testing.T) {
	t.Parallel()

	params := &chaincfg.MainNetParams
	s := newRescanTestSpends(t, params)
	addrs := map[string]struct{}{
		s.p2pkhAddr.String():  {},
		s.p2sh32Addr.String(): {},
		s.tokenAddr.String():  {},
	}

	tests := []struct {
		name      string
		sigScript []byte
		stxo      *blockchain.SpentTxOut
		want      bool
	}{
		{"schnorr p2pkh", s.p2pkhSig, &s.p2pkhOut, true},
		{"schnorr p2pkh no stxo", s.p2pkhSig, nil, true},
		{"p2sh32", s.p2sh32Sig, &s.p2sh32Out, true},
		{"p2sh32 no stxo", s.p2sh32Sig, nil, false},
		{"token p2pkh", s.tokenSig, &s.tokenOut, true},
		{"unwatched", s.otherSig, &s.otherOut, false},
		{"unwatched no stxo", s.otherSig, nil, false},
		{"stxo wins over sigScript", s.p2pkhSig, &s.otherOut, false},
	}
	for _, test := range tests {
		txin := &wire.TxIn{SignatureScript: test.sigScript}
		got := spendsWatchedAddr(addrs, txin, test.stxo, params)
		if got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}

	if spendsWatchedAddr(nil, &wire.TxIn{SignatureScript: s.p2pkhSig},
		&s.p2pkhOut, params) {

		t.Error("matched with no watched addresses")
	}
}

// TestRescanBlockSpentOutputs ensures rescanBlock notifies spends of watched
// addresses that are only known by address, using the spent outputs when
// they line up with the block.
func TestRescanBlockSpentOutputs(t *testing.T) {
	t.Parallel()

	params := &chaincfg.MainNetParams
	s := newRescanTestSpends(t, params)

	newTx := func(sigScript []byte) *wire.MsgTx {
		tx := wire.NewMsgTx(2)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Index: uint32(len(sigScript))},
			SignatureScript:  sigScript,
		})
		tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})
		return tx
	}
	coinbase := wire.NewMsgTx(2)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  s.p2pkhSig,
	})
	coinbase.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})

	p2pkhTx := newTx(s.p2pkhSig)
	p2sh32Tx := newTx(s.p2sh32Sig)
	otherTx := newTx(s.otherSig)
	stxos := []blockchain.SpentTxOut{s.p2pkhOut, s.p2sh32Out, s.otherOut}

	// A transaction with two inputs matched by outpoint, so the second is
	// skipped once the first is notified.
	watched := []wire.OutPoint{{Index: 1000}, {Index: 1001}}
	outpointTx := wire.NewMsgTx(2)
	for _, op := range watched {
		outpointTx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: op,
			SignatureScript:  s.otherSig,
		})
	}
	outpointTx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})

	tests := []struct {
		name    string
		txs     []*wire.MsgTx
		stxos   []blockchain.SpentTxOut
		unspent []wire.OutPoint
		want    []*wire.MsgTx
	}{{
		name:  "spent outputs",
		txs:   []*wire.MsgTx{p2pkhTx, p2sh32Tx, otherTx},
		stxos: stxos,
		want:  []*wire.MsgTx{p2pkhTx, p2sh32Tx},
	}, {
		name: "no spent outputs",
		txs:  []*wire.MsgTx{p2pkhTx, p2sh32Tx, otherTx},
		want: []*wire.MsgTx{p2pkhTx},
	}, {
		name:  "mismatched spent outputs",
		txs:   []*wire.MsgTx{p2pkhTx, p2sh32Tx, otherTx},
		stxos: stxos[:2],
		want:  []*wire.MsgTx{p2pkhTx},
	}, {
		name:    "outpoint match before address match",
		txs:     []*wire.MsgTx{outpointTx, p2sh32Tx},
		stxos:   []blockchain.SpentTxOut{s.otherOut, s.otherOut, s.p2sh32Out},
		unspent: watched,
		want:    []*wire.MsgTx{outpointTx, p2sh32Tx},
	}}
	for _, test := range tests {
		msgBlock := &wire.MsgBlock{
			Transactions: append([]*wire.MsgTx{coinbase}, test.txs...),
		}
		blk := bchutil.NewBlock(msgBlock)
		blk.SetHeight(100)

		wsc := &wsClient{
			server:   &rpcServer{cfg: rpcserverConfig{ChainParams: params}},
			ntfnChan: make(chan []byte, len(msgBlock.Transactions)),
		}
		lookups := &rescanKeys{
			addrs: map[string]struct{}{
				s.p2pkhAddr.String():  {},
				s.p2sh32Addr.String(): {},
			},
			unspent: map[wire.OutPoint]struct{}{},
		}
		for _, op := range test.unspent {
			lookups.unspent[op] = struct{}{}
		}
		rescanBlock(wsc, lookups, blk, test.stxos)

		close(wsc.ntfnChan)
		var got [][]byte
		for ntfn := range wsc.ntfnChan {
			got = append(got, ntfn)
		}
		if len(got) != len(test.want) {
			t.Errorf("%s: got %d notifications, want %d", test.name,
				len(got), len(test.want))
			continue
		}
		for i, tx := range test.want {
			if !bytes.Contains(got[i], []byte(txHexString(tx))) {
				t.Errorf("%s: notification %d is not for tx %v",
					test.name, i, tx.TxHash())
			}
		}
	}
}
