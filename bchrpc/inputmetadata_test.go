// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"testing"
	"time"
	"unsafe"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/protobuf/proto"
)

// inputMetaChain is a chain whose last block holds transactions spending
// outputs of many kinds, with and without token data, several from one
// transaction, and outputs of one transaction that many spend from.
type inputMetaChain struct {
	spent  map[wire.OutPoint]*wire.TxOut // the outputs spent, as created
	shared *wire.MsgTx                   // the transaction many spend from
	block  *bchutil.Block                // the last block
}

// newInputMetaChain mines an inputMetaChain.  Every transaction but the
// coinbases of heights 2 and 3 pays the address of addrRedeem.
func newInputMetaChain(t *testing.T, h *addrUnspentHarness) *inputMetaChain {
	t.Helper()
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}
	c := &inputMetaChain{spent: make(map[wire.OutPoint]*wire.TxOut)}
	spend := func(outs []*wire.TxOut, prevs ...wire.OutPoint) *wire.MsgTx {
		tx := wire.NewMsgTx(2)
		for _, prev := range prevs {
			tx.AddTxIn(wire.NewTxIn(&prev, []byte{txscript.OP_TRUE}))
		}
		for _, out := range outs {
			tx.AddTxOut(out)
		}
		return tx
	}
	record := func(tx *wire.MsgTx) {
		for _, in := range tx.TxIn {
			c.spent[in.PreviousOutPoint] = nil
		}
	}

	key, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	pkh, err := bchutil.NewAddressPubKeyHash(
		bchutil.Hash160(key.PubKey().SerializeCompressed()), h.params)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: %v", err)
	}
	p2pkh, err := txscript.PayToAddrScript(pkh)
	if err != nil {
		t.Fatalf("PayToAddrScript: %v", err)
	}
	p2pk := func(pubKey []byte) []byte {
		script, err := txscript.NewScriptBuilder().AddData(pubKey).
			AddOp(txscript.OP_CHECKSIG).Script()
		if err != nil {
			t.Fatalf("pay to pubkey script: %v", err)
		}
		return script
	}

	// Height 1 pays the address and others: to a pubkey hash, to a
	// compressed and an uncompressed pubkey, a bare script and an empty
	// script.
	cb1 := h.mineBlock([]*wire.TxOut{
		pay(50e8, addrScript),
		pay(50e8, addrScript),
		pay(50e8, otherScript),
		pay(1e8, p2pkh),
		pay(2e8, p2pk(key.PubKey().SerializeCompressed())),
		pay(3e8, p2pk(key.PubKey().SerializeUncompressed())),
		pay(4e8, []byte{txscript.OP_TRUE}),
		pay(6e8, nil),
		pay(50e8, otherScript),
	}, nil)

	// Height 2: tok creates fungible tokens, an NFT with a commitment and a
	// minting NFT with tokens, and an output whose script starts with the
	// token prefix byte but holds no token prefix, and shared pays the
	// address ten times.
	amount := uint64(1000)
	ft, err := wire.NewTokenData(cb1.TxHash(), &amount, nil, nil)
	if err != nil {
		t.Fatalf("NewTokenData: %v", err)
	}
	commitment := []byte{1, 2, 3}
	none := byte(wire.NONE)
	nft, err := wire.NewTokenData(cb1.TxHash(), nil, &commitment, &none)
	if err != nil {
		t.Fatalf("NewTokenData: %v", err)
	}
	five := uint64(5)
	minting := byte(wire.MINTING)
	mint, err := wire.NewTokenData(cb1.TxHash(), &five, nil, &minting)
	if err != nil {
		t.Fatalf("NewTokenData: %v", err)
	}
	tok := spend([]*wire.TxOut{
		wire.NewTxOut(10e8, addrScript, *ft),
		wire.NewTxOut(11e8, addrScript, *nft),
		wire.NewTxOut(12e8, p2pkh, *mint),
		pay(13e8, otherScript),
		pay(5e8, []byte{wire.PREFIX_BYTE, 0x10, txscript.OP_TRUE}),
	}, outPoint(cb1, 2))
	shared := make([]*wire.TxOut, 10)
	for i := range shared {
		shared[i] = pay(int64(i+1)*1e6, addrScript)
	}
	c.shared = spend(shared, outPoint(cb1, 8))
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
		[]*wire.MsgTx{tok, c.shared})

	// Height 3: c1-c3 spend the outputs of tok and cb1, two of tok's in
	// one transaction, and ten transactions an output of shared each.
	txs := []*wire.MsgTx{
		spend([]*wire.TxOut{pay(1e8, addrScript)}, outPoint(tok, 0),
			outPoint(tok, 1), outPoint(cb1, 0)),
		spend([]*wire.TxOut{pay(1e8, addrScript)}, outPoint(tok, 2),
			outPoint(cb1, 3), outPoint(cb1, 4), outPoint(cb1, 5)),
		spend([]*wire.TxOut{pay(1e8, addrScript)}, outPoint(cb1, 6),
			outPoint(tok, 4), outPoint(cb1, 7), outPoint(cb1, 1)),
	}
	for i := range shared {
		txs = append(txs, spend([]*wire.TxOut{pay(1e5, addrScript)},
			outPoint(c.shared, uint32(i))))
	}
	for _, tx := range append([]*wire.MsgTx{tok, c.shared}, txs...) {
		record(tx)
	}
	for _, tx := range []*wire.MsgTx{cb1, tok, c.shared} {
		for i, out := range tx.TxOut {
			op := outPoint(tx, uint32(i))
			if _, ok := c.spent[op]; ok {
				c.spent[op] = out
			}
		}
	}
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, txs)
	c.block, err = h.chain.BlockByHash(&h.prevHash)
	if err != nil {
		t.Fatalf("BlockByHash: %v", err)
	}
	return c
}

// checkInputs ensures each input of tx has the value, previous script, token
// data and address of the output it spends, and a coinbase input none.
func (c *inputMetaChain) checkInputs(t *testing.T, h *addrUnspentHarness,
	tx *pb.Transaction) {

	t.Helper()
	for i, in := range tx.Inputs {
		var op wire.OutPoint
		copy(op.Hash[:], in.Outpoint.Hash)
		op.Index = in.Outpoint.Index
		out := c.spent[op]
		if out == nil {
			if in.Value != 0 || in.PreviousScript != nil ||
				in.CashToken != nil || in.Address != "" {

				t.Errorf("input %d of %x, a coinbase's: %v", i, tx.Hash, in)
			}
			continue
		}
		var token *pb.CashToken
		if !out.TokenData.IsEmpty() {
			token = getPbCashTokenDataFromTokenData(out.TokenData)
		}
		var addr string
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(out.PkScript,
			h.params)
		if err == nil && len(addrs) > 0 {
			addr = addrs[0].String()
		}
		if in.Value != out.Value || !bytes.Equal(in.PreviousScript,
			out.PkScript) || !proto.Equal(in.CashToken, token) ||
			in.Address != addr {

			t.Errorf("input %d of %x spending %v: value %d, script %x, "+
				"token %v, address %q; want %d, %x, %v, %q", i, tx.Hash,
				op, in.Value, in.PreviousScript, in.CashToken, in.Address,
				out.Value, out.PkScript, token, addr)
		}
	}
}

// TestGetAddressTransactionsLoadsInputsOnce ensures GetAddressTransactions
// gives each input of the transactions it returns the metadata of the output
// it spends, and loads each transaction spent from once for the call, though
// ten of the transactions spend from one and others from the same two, and
// that the reply keeps only the scripts of the outputs spent, not those of
// every output of the transactions loaded.
func TestGetAddressTransactionsLoadsInputsOnce(t *testing.T) {
	h := newAddrUnspentHarness(t)
	c := newInputMetaChain(t, h)
	db := &viewCountingDB{DB: h.s.db}
	h.s.db = db
	h.s.txIndex = h.txIndex
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}

	resp, err := h.s.GetAddressTransactions(context.Background(),
		&pb.GetAddressTransactionsRequest{Address: addr.EncodeAddress()})
	if err != nil {
		t.Fatalf("GetAddressTransactions: %v", err)
	}
	if n := len(resp.ConfirmedTransactions); n != 16 {
		t.Fatalf("%d confirmed transactions, want 16", n)
	}
	for _, tx := range resp.ConfirmedTransactions {
		c.checkInputs(t, h, tx)
	}

	// One read for the address's transactions, and one each for cb1, tok
	// and shared.
	if n := db.views.Load(); n != 4 {
		t.Errorf("%d reads from the database, want 4", n)
	}

	// Two outputs of shared that follow each other have their scripts next
	// to each other in its decoded scripts.
	scripts := make(map[uint32][]byte)
	sharedHash := c.shared.TxHash()
	for _, tx := range resp.ConfirmedTransactions {
		for _, in := range tx.Inputs {
			if bytes.Equal(in.Outpoint.Hash, sharedHash[:]) {
				scripts[in.Outpoint.Index] = in.PreviousScript
			}
		}
	}
	first, second := scripts[0], scripts[1]
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("no scripts for the outputs of shared")
	}
	if unsafe.Add(unsafe.Pointer(unsafe.SliceData(first)), len(first)) ==
		unsafe.Pointer(unsafe.SliceData(second)) {

		t.Error("the reply keeps the scripts of the transaction spent from")
	}

	// GetTransaction sets the metadata of its transaction's inputs too.
	hash := c.block.Transactions()[1].Hash()
	got, err := h.s.GetTransaction(context.Background(),
		&pb.GetTransactionRequest{Hash: hash[:]})
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	c.checkInputs(t, h, got.Transaction)
}

// metaStream is the stream of a transaction subscription that passes on the
// confirmed transactions it is sent.
type metaStream struct {
	txNotifyStream
	txs chan *pb.Transaction
}

func (s *metaStream) Send(n *pb.TransactionNotification) error {
	s.txs <- n.GetConfirmedTransaction()
	return nil
}

// subscribeConfirmed starts a SubscribeTransactions call, or a
// SubscribeTransactionStream call when bidi is set, sent every confirmed
// transaction in full, and waits until the dispatcher holds it and it has its
// filter.
func (h *addrUnspentHarness) subscribeConfirmed(t *testing.T,
	bidi bool) *metaStream {

	t.Helper()
	req := &pb.SubscribeTransactionsRequest{
		Subscribe:      &pb.TransactionFilter{AllTransactions: true},
		IncludeInBlock: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	stream := &metaStream{
		txNotifyStream: txNotifyStream{ctx: ctx,
			delivered: make(chan struct{}), ended: make(chan struct{})},
		txs: make(chan *pb.Transaction, 100),
	}
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		<-done
		// A call that took its request has its request reader in the
		// Recv that closed delivered, which closes ended as it returns
		// the end of the stream.
		if bidi {
			select {
			case <-stream.delivered:
				<-stream.ended
			default:
			}
		}
	})
	n := h.s.numSubscriptions()
	go func() {
		defer close(done)
		if bidi {
			stream.req = req
			_ = h.s.SubscribeTransactionStream(stream)
			return
		}
		close(stream.delivered)
		_ = h.s.SubscribeTransactions(req, stream)
	}()
	waitSubscriptions(t, h.s, n+1)
	select {
	case <-stream.delivered:
	case <-time.After(5 * time.Second):
		t.Fatal("the call did not take its filter")
	}
	return stream
}

// received returns the n transactions the stream is sent next.
func received(t *testing.T, stream *metaStream, n int) []*pb.Transaction {
	t.Helper()
	var txs []*pb.Transaction
	for range n {
		select {
		case tx := <-stream.txs:
			txs = append(txs, tx)
		case <-time.After(5 * time.Second):
			t.Fatalf("%d transactions sent, want %d", len(txs), n)
		}
	}
	return txs
}

// TestSubscriptionInputMetadataFromSpendJournal ensures the confirmed
// transactions that subscriptions are sent take the metadata of their inputs
// from the block's spend journal, so no transaction is loaded from the
// transaction index for them, and that it is the metadata the transaction
// index gives, token data included.
func TestSubscriptionInputMetadataFromSpendJournal(t *testing.T) {
	h := newAddrUnspentHarness(t)
	h.startDispatcher()
	c := newInputMetaChain(t, h)
	db := &viewCountingDB{DB: h.s.db}
	h.s.db = db
	h.s.txIndex = h.txIndex
	subs := []*metaStream{h.subscribeConfirmed(t, false),
		h.subscribeConfirmed(t, true)}

	h.s.dispatchEvent(blockEvent(c.block))
	txs := c.block.Transactions()
	got := make([][]*pb.Transaction, len(subs))
	for i, sub := range subs {
		got[i] = received(t, sub, len(txs))
	}
	if n := db.views.Load(); n != 0 {
		t.Errorf("%d transactions loaded from the transaction index for "+
			"%d subscriptions", n, len(subs))
	}

	for _, sent := range got {
		for i, tx := range sent {
			c.checkInputs(t, h, tx)
			header := c.block.MsgBlock().Header
			want := marshalTransaction(txs[i], tx.Confirmations, &header,
				c.block.Height(), h.s)
			if err := h.s.setInputMetadata(want); err != nil {
				t.Fatalf("setInputMetadata: %v", err)
			}
			if !proto.Equal(tx, want) {
				t.Errorf("transaction %d: sent\n%v\nthe transaction index "+
					"gives\n%v", i, tx, want)
			}
		}
	}
}

// TestSubscriptionInputMetadataWithoutSpendJournal ensures the confirmed
// transactions of a block without a spend journal entry, as one disconnected
// before the subscriptions read its event, take the metadata of their inputs
// from the transaction index.
func TestSubscriptionInputMetadataWithoutSpendJournal(t *testing.T) {
	h := newAddrUnspentHarness(t)
	h.startDispatcher()
	c := newInputMetaChain(t, h)
	h.s.txIndex = h.txIndex
	subs := []*metaStream{h.subscribeConfirmed(t, false),
		h.subscribeConfirmed(t, true)}

	unconnected := bchutil.NewBlock(&wire.MsgBlock{
		Transactions: c.block.MsgBlock().Transactions})
	h.s.dispatchEvent(blockEvent(unconnected))
	for _, sub := range subs {
		sent := received(t, sub, len(unconnected.Transactions()))
		for _, tx := range sent {
			c.checkInputs(t, h, tx)
		}
	}
}
