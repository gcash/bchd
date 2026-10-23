// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
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

// TestLoadTxFilterDecodesOutsideLocks ensures loadtxfilter decodes its
// addresses without holding the client's lock or its filter's, which the
// notification manager takes for every transaction it checks, whether it
// builds the client's first filter, adds to it or replaces it.
func TestLoadTxFilterDecodesOutsideLocks(t *testing.T) {
	params := &chaincfg.MainNetParams
	wsc := &wsClient{server: &rpcServer{cfg: rpcserverConfig{ChainParams: params}}}
	var decoded int
	loadTxFilterDecodeHook = func() {
		decoded++
		if !wsc.TryLock() {
			t.Error("the client's lock is held while an address is decoded")
			return
		}
		filter := wsc.filterData
		wsc.Unlock()
		if filter == nil {
			return
		}
		if !filter.mu.TryLock() {
			t.Error("the filter's lock is held while an address is decoded")
			return
		}
		filter.mu.Unlock()
	}
	t.Cleanup(func() { loadTxFilterDecodeHook = nil })

	addrs := make([]bchutil.Address, 6)
	for i := range addrs {
		addr, err := bchutil.NewAddressPubKeyHash(bytes.Repeat([]byte{byte(i)}, 20), params)
		if err != nil {
			t.Fatal(err)
		}
		addrs[i] = addr
	}
	hash, err := chainhash.NewHashFromStr(testTokenCategory)
	if err != nil {
		t.Fatal(err)
	}
	outPoint := func(i int) *wire.OutPoint {
		return &wire.OutPoint{Hash: *hash, Index: uint32(i)}
	}
	for i, reload := range []bool{false, false, true} {
		cmd := &btcjson.LoadTxFilterCmd{
			Reload: reload,
			Addresses: []string{addrs[2*i].EncodeAddress(), "not an address",
				addrs[2*i+1].EncodeAddress()},
			OutPoints: []btcjson.OutPoint{{Hash: testTokenCategory, Index: uint32(i)}},
		}
		if _, err := handleLoadTxFilter(wsc, cmd); err != nil {
			t.Fatal(err)
		}
		if decoded != 3*(i+1) {
			t.Fatalf("load %d: %d addresses decoded, want %d", i, decoded, 3*(i+1))
		}
		if !wsc.filterData.existsUnspentOutPoint(outPoint(i)) {
			t.Errorf("load %d: outpoint missing", i)
		}
	}

	// The last load replaced the filter holding the first four addresses
	// and the first two outpoints.
	for i, addr := range addrs {
		if got, want := wsc.filterData.existsAddress(addr), i >= 4; got != want {
			t.Errorf("address %d in the filter: %v, want %v", i, got, want)
		}
	}
	for i := range 2 {
		if wsc.filterData.existsUnspentOutPoint(outPoint(i)) {
			t.Errorf("outpoint %d kept by the reload", i)
		}
	}
	if t.Failed() {
		return
	}

	// A first load that another first load overtakes while it decodes
	// adds to the filter that one set.
	wsc.filterData = nil
	overtaken := false
	loadTxFilterDecodeHook = func() {
		if overtaken {
			return
		}
		overtaken = true
		cmd := &btcjson.LoadTxFilterCmd{Addresses: []string{addrs[0].EncodeAddress()}}
		if _, err := handleLoadTxFilter(wsc, cmd); err != nil {
			t.Fatal(err)
		}
	}
	cmd := &btcjson.LoadTxFilterCmd{Addresses: []string{addrs[1].EncodeAddress()}}
	if _, err := handleLoadTxFilter(wsc, cmd); err != nil {
		t.Fatal(err)
	}
	for i, addr := range addrs[:2] {
		if !wsc.filterData.existsAddress(addr) {
			t.Errorf("address %d of the overtaken loads missing", i)
		}
	}
}

// TestWsQueueNotificationQuit ensures QueueNotification gives up once the
// client quits, even when its queue is full and nothing takes from it any
// more, so the notification manager cannot wait on a client that is gone.
func TestWsQueueNotificationQuit(t *testing.T) {
	wsc := &wsClient{quit: make(chan struct{}), ntfnChan: make(chan []byte, 1)}
	wsc.ntfnChan <- nil
	close(wsc.quit)
	done := make(chan error, 1)
	go func() { done <- wsc.QueueNotification([]byte("{}")) }()
	select {
	case err := <-done:
		if err != ErrClientQuit {
			t.Fatalf("got %v, want ErrClientQuit", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("QueueNotification waited on a client that quit")
	}
}

// TestLimitedUserStopNotify checks that limited users can stop every
// notification they are allowed to start.
func TestLimitedUserStopNotify(t *testing.T) {
	for method := range wsHandlers {
		if !strings.HasPrefix(method, "notify") {
			continue
		}
		if _, ok := rpcLimited[method]; !ok {
			continue
		}
		stop := "stop" + method
		if _, ok := wsHandlers[stop]; !ok {
			t.Errorf("%s has no %s handler", method, stop)
		}
		if _, ok := rpcLimited[stop]; !ok {
			t.Errorf("%s is limited but %s is not", method, stop)
		}
	}
}

// TestRescanBlockFilterPanicUnlocks ensures a panic while a block is scanned
// for rescanblocks, which runRPCHandler recovers, leaves the client's filter
// unlocked, as the notification manager locks it for every transaction.  A
// nil transaction stands in for whatever could panic.
func TestRescanBlockFilterPanicUnlocks(t *testing.T) {
	filter := newWSClientFilter(nil, nil)
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{nil}})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("scanning a nil transaction did not panic")
			}
		}()
		rescanBlockFilter(filter, block, &chaincfg.MainNetParams)
	}()
	if !filter.mu.TryLock() {
		t.Fatal("filter left locked")
	}
	filter.mu.Unlock()
}
