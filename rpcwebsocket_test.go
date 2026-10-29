// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
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

// TestParentsFirst ensures transactions are ordered after the ones whose
// outputs they spend, and otherwise kept in order.
func TestParentsFirst(t *testing.T) {
	t.Parallel()

	newTx := func(value int64, parents ...*bchutil.Tx) *bchutil.Tx {
		msgTx := wire.NewMsgTx(2)
		for _, parent := range parents {
			msgTx.AddTxIn(&wire.TxIn{
				PreviousOutPoint: wire.OutPoint{Hash: *parent.Hash()},
			})
		}
		if len(parents) == 0 {
			msgTx.AddTxIn(&wire.TxIn{
				PreviousOutPoint: wire.OutPoint{Index: uint32(value)},
			})
		}
		msgTx.AddTxOut(&wire.TxOut{Value: value})
		return bchutil.NewTx(msgTx)
	}
	a := newTx(1)
	b := newTx(2, a)
	c := newTx(3, b)
	d := newTx(4)
	e := newTx(5, a, d)
	names := map[*bchutil.Tx]string{a: "a", b: "b", c: "c", d: "d", e: "e"}
	nameAll := func(txs []*bchutil.Tx) []string {
		var s []string
		for _, tx := range txs {
			s = append(s, names[tx])
		}
		return s
	}

	tests := []struct {
		txs, want []*bchutil.Tx
	}{
		{nil, nil},
		{[]*bchutil.Tx{a, b, c, d, e}, []*bchutil.Tx{a, b, c, d, e}},
		{[]*bchutil.Tx{c, d, e, b, a}, []*bchutil.Tx{a, b, c, d, e}},
		{[]*bchutil.Tx{e, c}, []*bchutil.Tx{e, c}},
		{[]*bchutil.Tx{e, d, a}, []*bchutil.Tx{a, d, e}},
	}
	for _, test := range tests {
		got := nameAll(parentsFirst(test.txs))
		want := nameAll(test.want)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%v: got %v, want %v", nameAll(test.txs), got, want)
		}
	}
}

// spendBeforeParentBlock returns a block that lists a transaction before its
// parent, as the canonical transaction ordering can, and the outputs its
// inputs spend.  The parent pays the watched P2SH32 address of s, whose spend
// can't be told from its signature script, and the child spends that output
// and pays an address that isn't watched.  names gives the hex of each a name.
func spendBeforeParentBlock(t *testing.T, s *rescanTestSpends) (*bchutil.Block,
	[]blockchain.SpentTxOut, map[string]string) {

	t.Helper()

	coinbase := wire.NewMsgTx(2)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{txscript.OP_0, txscript.OP_0},
	})
	coinbase.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})

	parent := wire.NewMsgTx(2)
	parent.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{9}},
		SignatureScript:  s.otherSig,
	})
	parent.AddTxOut(&wire.TxOut{Value: 1000, PkScript: s.p2sh32Out.PkScript})

	child := wire.NewMsgTx(2)
	child.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: parent.TxHash()},
		SignatureScript:  s.p2sh32Sig,
	})
	child.AddTxOut(&wire.TxOut{Value: 900, PkScript: s.otherOut.PkScript})

	block := bchutil.NewBlock(&wire.MsgBlock{
		Transactions: []*wire.MsgTx{coinbase, child, parent},
	})
	block.SetHeight(100)

	// The spend journal lists the outputs the block's inputs spend in
	// block order, the coinbase's aside.
	stxos := []blockchain.SpentTxOut{s.p2sh32Out, s.otherOut}
	names := map[string]string{
		txHexString(parent): "parent",
		txHexString(child):  "child",
	}
	return block, stxos, names
}

// notified returns the notifications queued for a client, each as its method
// and the name of its transaction.
func notified(t *testing.T, wsc *wsClient, names map[string]string) []string {
	t.Helper()

	var got []string
	for {
		select {
		case marshalled := <-wsc.ntfnChan:
			var ntfn struct {
				Method string            `json:"method"`
				Params []json.RawMessage `json:"params"`
			}
			var txHex string
			if err := json.Unmarshal(marshalled, &ntfn); err != nil {
				t.Fatalf("unable to decode notification: %v", err)
			}
			if err := json.Unmarshal(ntfn.Params[0], &txHex); err != nil {
				t.Fatalf("unable to decode %s notification: %v",
					ntfn.Method, err)
			}
			got = append(got, ntfn.Method+" "+names[txHex])
		default:
			return got
		}
	}
}

// TestRescanBlockFilterSpendBeforeParent ensures rescanblocks finds a
// transaction listed before the parent whose watched output it spends, and
// returns the parent first.
func TestRescanBlockFilterSpendBeforeParent(t *testing.T) {
	t.Parallel()

	params := &chaincfg.MainNetParams
	s := newRescanTestSpends(t, params)
	block, _, names := spendBeforeParentBlock(t, s)

	filter := newWSClientFilter([]bchutil.Address{s.p2sh32Addr}, nil)
	var got []string
	for _, txHex := range rescanBlockFilter(filter, block, params) {
		got = append(got, names[txHex])
	}
	if want := "parent child"; strings.Join(got, " ") != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestRescanBlockSpendBeforeParent ensures rescan notifies a transaction
// listed before the parent whose watched output it spends, after the parent,
// whether or not the spend journal lines up with the block.
func TestRescanBlockSpendBeforeParent(t *testing.T) {
	t.Parallel()

	params := &chaincfg.MainNetParams
	s := newRescanTestSpends(t, params)
	block, stxos, names := spendBeforeParentBlock(t, s)

	tests := []struct {
		name  string
		stxos []blockchain.SpentTxOut
	}{
		{"spent outputs", stxos},
		{"no spent outputs", nil},
	}
	for _, test := range tests {
		wsc := &wsClient{
			server:   &rpcServer{cfg: rpcserverConfig{ChainParams: params}},
			ntfnChan: make(chan []byte, 10),
		}
		lookups := &rescanKeys{
			addrs:   map[string]struct{}{s.p2sh32Addr.String(): {}},
			unspent: map[wire.OutPoint]struct{}{},
		}
		rescanBlock(wsc, lookups, block, test.stxos)

		got := notified(t, wsc, names)
		want := "recvtx parent,redeemingtx child"
		if strings.Join(got, ",") != want {
			t.Errorf("%s: got %v, want %v", test.name, got, want)
		}
		if len(lookups.unspent) != 0 {
			t.Errorf("%s: still watching %v, which the child spent",
				test.name, lookups.unspent)
		}
	}
}

// TestNotifyForBlockTxsSpendBeforeParent ensures a connected block's
// transaction listed before the parent whose watched output it spends is
// notified, after the parent.
func TestNotifyForBlockTxsSpendBeforeParent(t *testing.T) {
	t.Parallel()

	params := &chaincfg.MainNetParams
	s := newRescanTestSpends(t, params)
	block, _, names := spendBeforeParentBlock(t, s)

	m := &wsNotificationManager{server: &rpcServer{cfg: rpcserverConfig{
		ChainParams: params,
		TxMemPool:   &mempool.TxPool{},
	}}}
	wsc := &wsClient{
		quit:          make(chan struct{}),
		ntfnChan:      make(chan []byte, 10),
		spentRequests: make(map[wire.OutPoint]struct{}),
	}
	ops := make(map[wire.OutPoint]map[chan struct{}]*wsClient)
	addrs := map[string]map[chan struct{}]*wsClient{
		s.p2sh32Addr.EncodeAddress(): {wsc.quit: wsc},
	}
	m.notifyForBlockTxs(ops, addrs, block)

	got := notified(t, wsc, names)
	if want := "recvtx parent,redeemingtx child"; strings.Join(got, ",") != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(ops) != 0 || len(wsc.spentRequests) != 0 {
		t.Errorf("still watching %v, which the child spent", ops)
	}
}

// TestNotifyFilteredBlockConnectedSpendBeforeParent ensures a filtered block
// lists a transaction that comes before the parent whose watched output it
// spends, after the parent.
func TestNotifyFilteredBlockConnectedSpendBeforeParent(t *testing.T) {
	t.Parallel()

	params := &chaincfg.MainNetParams
	s := newRescanTestSpends(t, params)
	block, _, names := spendBeforeParentBlock(t, s)

	m := &wsNotificationManager{server: &rpcServer{cfg: rpcserverConfig{
		ChainParams: params,
	}}}
	wsc := &wsClient{
		quit:       make(chan struct{}),
		ntfnChan:   make(chan []byte, 1),
		filterData: newWSClientFilter([]bchutil.Address{s.p2sh32Addr}, nil),
	}
	m.notifyFilteredBlockConnected(map[chan struct{}]*wsClient{wsc.quit: wsc},
		block)

	var ntfn struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(<-wsc.ntfnChan, &ntfn); err != nil {
		t.Fatalf("unable to decode notification: %v", err)
	}
	var txHexes []string
	if err := json.Unmarshal(ntfn.Params[2], &txHexes); err != nil {
		t.Fatalf("unable to decode the subscribed transactions: %v", err)
	}
	var got []string
	for _, txHex := range txHexes {
		got = append(got, names[txHex])
	}
	if want := "parent child"; strings.Join(got, " ") != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
