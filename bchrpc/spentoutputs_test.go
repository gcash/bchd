// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc"
)

// txNotifyStream is a SubscribeTransactions or SubscribeTransactionStream
// stream that records the hashes of the serialized transactions it is sent.
// For SubscribeTransactionStream it sends req, closes delivered once the call
// has taken it, and then ends its client's side with the stream, as txStream
// does, closing ended as it does so, after which the call's request reader
// only returns.
type txNotifyStream struct {
	grpc.ServerStream
	ctx       context.Context
	sent      chan chainhash.Hash
	req       *pb.SubscribeTransactionsRequest
	delivered chan struct{}
	ended     chan struct{}
}

func (s *txNotifyStream) Context() context.Context { return s.ctx }

func (s *txNotifyStream) Recv() (*pb.SubscribeTransactionsRequest, error) {
	if req := s.req; req != nil {
		s.req = nil
		return req, nil
	}
	close(s.delivered)
	<-s.ctx.Done()
	close(s.ended)
	return nil, io.EOF
}

func (s *txNotifyStream) Send(n *pb.TransactionNotification) error {
	var tx wire.MsgTx
	err := tx.BchDecode(bytes.NewReader(n.GetSerializedTransaction()),
		wire.ProtocolVersion, wire.BaseEncoding)
	if err != nil {
		return err
	}
	s.sent <- tx.TxHash()
	return nil
}

// viewCountingDB counts the read transactions run on a database.
type viewCountingDB struct {
	database.DB
	views atomic.Int32
}

func (db *viewCountingDB) View(fn func(database.Tx) error) error {
	db.views.Add(1)
	return db.DB.View(fn)
}

// startDispatcher gives the harness's server a running event dispatcher.
func (h *addrUnspentHarness) startDispatcher() {
	s := h.s
	s.subscribe = make(chan *rpcEventSubscription)
	s.unsubscribe = make(chan *rpcEventSubscription)
	s.events = make(chan interface{})
	s.quit = make(chan struct{})
	s.maxSubscriptions = 10
	s.maxFilterEntries = maxFilterSize
	s.wg.Add(1)
	go s.runEventDispatcher()
	h.tb.Cleanup(func() {
		close(s.quit)
		s.wg.Wait()
	})
}

// subscribeAddress starts a SubscribeTransactions call, or a
// SubscribeTransactionStream call when bidi is set, watching the P2SH address
// of addrRedeem with serialized transactions, and waits until the dispatcher
// holds it and it has its filter.
func (h *addrUnspentHarness) subscribeAddress(t *testing.T, bidi, mempool,
	blocks bool) *txNotifyStream {

	t.Helper()
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}
	req := &pb.SubscribeTransactionsRequest{
		Subscribe:      &pb.TransactionFilter{Addresses: []string{addr.EncodeAddress()}},
		IncludeMempool: mempool,
		IncludeInBlock: blocks,
		SerializeTx:    true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	stream := &txNotifyStream{ctx: ctx, sent: make(chan chainhash.Hash, 100),
		delivered: make(chan struct{}), ended: make(chan struct{})}
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

// notified returns the transactions the stream was sent until the sentinel.
func notified(t *testing.T, stream *txNotifyStream,
	sentinel chainhash.Hash) map[chainhash.Hash]bool {

	t.Helper()
	got := make(map[chainhash.Hash]bool)
	for {
		select {
		case hash := <-stream.sent:
			if hash == sentinel {
				return got
			}
			got[hash] = true
		case <-time.After(5 * time.Second):
			t.Fatal("the sentinel was not notified")
		}
	}
}

// checkNotified ensures exactly the wanted transactions were notified.
func checkNotified(t *testing.T, what string, got map[chainhash.Hash]bool,
	want map[string]*wire.MsgTx) {

	t.Helper()
	for name, tx := range want {
		if !got[tx.TxHash()] {
			t.Errorf("%s: %s was not notified", what, name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%s: %d transactions notified, want %d", what, len(got),
			len(want))
	}
}

// spentOutputsChain is a chain in which the address of addrRedeem was paid
// and spent, and a memory pool with transactions spending its outputs.
type spentOutputsChain struct {
	cb1, a, s1, n1, m1, m2, m3 *wire.MsgTx
	block2                     *bchutil.Block
}

// newSpentOutputsChain mines and accepts the transactions of a
// spentOutputsChain.
func newSpentOutputsChain(t *testing.T, h *addrUnspentHarness) *spentOutputsChain {
	t.Helper()
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}
	c := &spentOutputsChain{}

	// Height 1 pays the address twice, and the other address twice.
	c.cb1 = h.mineBlock([]*wire.TxOut{pay(50e8, addrScript),
		pay(50e8, addrScript), pay(50e8, otherScript),
		pay(50e8, otherScript)}, nil)

	// Height 2: a pays the address from the other address's output, s1
	// spends an output of the address and pays nothing back to it, and n1
	// has nothing to do with the address.
	c.a = spendTx(t, outPoint(c.cb1, 2), otherRedeem,
		pay(20e8, addrScript), pay(29e8, otherScript))
	c.s1 = spendTx(t, outPoint(c.cb1, 0), addrRedeem, pay(49e8, otherScript))
	c.n1 = spendTx(t, outPoint(c.cb1, 3), otherRedeem, pay(49e8, otherScript))
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
		[]*wire.MsgTx{c.a, c.s1, c.n1})
	var err error
	c.block2, err = h.chain.BlockByHash(&h.prevHash)
	if err != nil {
		t.Fatalf("BlockByHash: %v", err)
	}

	// The memory pool: m1 spends a's confirmed output of the address, m2
	// pays the address, and m3 spends m2's output.
	c.m1 = spendTx(t, outPoint(c.a, 0), addrRedeem, pay(19e8, otherScript))
	c.m2 = spendTx(t, outPoint(c.a, 1), otherRedeem, pay(28e8, addrScript))
	c.m3 = spendTx(t, outPoint(c.m2, 0), addrRedeem, pay(27e8, otherScript))
	for _, tx := range []*wire.MsgTx{c.m1, c.m2, c.m3} {
		h.acceptTx(tx)
	}
	return c
}

// txEvent returns the event of the transaction's acceptance to the pool.  Its
// hash is cached first, as the memory pool does, since subscriptions read it
// at once.
func txEvent(tx *wire.MsgTx) *rpcEventTxAccepted {
	utx := bchutil.NewTx(tx)
	utx.Hash()
	return &rpcEventTxAccepted{TxDesc: &mempool.TxDesc{
		TxDesc: mining.TxDesc{Tx: utx}}}
}

// blockEvent returns the event of the block's connection.  Its transactions
// and hashes are cached first, as the chain does when it checks a block,
// since subscriptions read them at once.
func blockEvent(block *bchutil.Block) *rpcEventBlockConnected {
	block.Hash()
	for _, tx := range block.Transactions() {
		tx.Hash()
	}
	return &rpcEventBlockConnected{Block: block}
}

// sentinelTx returns a transaction paying the address, which every
// subscription watching it is sent.
func sentinelTx(t *testing.T) *wire.MsgTx {
	t.Helper()
	return spendTx(t, wire.OutPoint{Hash: chainhash.Hash{0xee}}, otherRedeem,
		wire.NewTxOut(1, p2shScript(t, addrRedeem), wire.TokenData{}))
}

// TestSubscriptionMatchesSpendsWithoutTxIndex ensures a transaction
// subscription matches a spend of an output of a watched address that it did
// not see arrive, without the transaction index: in a block from the block's
// spend journal, or from the utxo set when the journal entry is gone, as for
// a block disconnected before the subscription reads its event, and in the
// memory pool from the utxo set or the memory pool.
func TestSubscriptionMatchesSpendsWithoutTxIndex(t *testing.T) {
	h := newAddrUnspentHarness(t)
	h.startDispatcher()
	c := newSpentOutputsChain(t, h)
	var blocks, pool []*txNotifyStream
	for _, bidi := range []bool{false, true} {
		blocks = append(blocks, h.subscribeAddress(t, bidi, false, true))
		pool = append(pool, h.subscribeAddress(t, bidi, true, false))
	}

	// s2 spends the address's other output of height 1 in a block that is
	// not connected, so has no spend journal entry.
	s2 := spendTx(t, outPoint(c.cb1, 1), addrRedeem,
		wire.NewTxOut(49e8, p2shScript(t, otherRedeem), wire.TokenData{}))
	unconnected := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{
		h.coinbaseTx(h.height+1, nil), s2}})
	sentinel := sentinelTx(t)
	sentinelBlock := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{
		sentinel}})

	h.s.dispatchEvent(blockEvent(c.block2))
	h.s.dispatchEvent(blockEvent(unconnected))
	h.s.dispatchEvent(blockEvent(sentinelBlock))
	for _, stream := range blocks {
		checkNotified(t, "blocks", notified(t, stream, sentinel.TxHash()),
			map[string]*wire.MsgTx{"a": c.a, "s1": c.s1, "s2": s2})
	}

	// m2's acceptance is not sent, so m3 matches on its spent output.
	h.s.dispatchEvent(txEvent(c.m1))
	h.s.dispatchEvent(txEvent(c.m3))
	h.s.dispatchEvent(txEvent(sentinel))
	for _, stream := range pool {
		checkNotified(t, "memory pool", notified(t, stream, sentinel.TxHash()),
			map[string]*wire.MsgTx{"m1": c.m1, "m3": c.m3})
	}
}

// TestSubscriptionSpendLookups ensures the outputs an event's transactions
// spend are not each read from the transaction index for every subscription
// with an address filter: a block's come from its spend journal, and a memory
// pool transaction's from the memory pool and the utxo set.
func TestSubscriptionSpendLookups(t *testing.T) {
	h := newAddrUnspentHarness(t)
	h.startDispatcher()
	c := newSpentOutputsChain(t, h)
	db := &viewCountingDB{DB: h.s.db}
	h.s.db = db
	h.s.txIndex = h.txIndex
	subs := []*txNotifyStream{h.subscribeAddress(t, false, true, true),
		h.subscribeAddress(t, true, true, true)}

	sentinel := sentinelTx(t)
	h.s.dispatchEvent(blockEvent(c.block2))
	h.s.dispatchEvent(txEvent(c.m1))
	h.s.dispatchEvent(txEvent(sentinel))
	for _, sub := range subs {
		checkNotified(t, "subscription", notified(t, sub, sentinel.TxHash()),
			map[string]*wire.MsgTx{"a": c.a, "s1": c.s1, "m1": c.m1})
	}
	if n := db.views.Load(); n != 0 {
		t.Errorf("%d transactions read from the database for %d "+
			"subscriptions", n, len(subs))
	}
}

// TestSubscriptionMatchesSpentOutputWithTxIndex ensures a memory pool
// transaction whose spent output a block spent before the subscriptions read
// its event still matches, from the transaction index, read once, whether the
// utxo cache dropped the output or holds it marked spent and without its
// script, as when it was flushed before.
func TestSubscriptionMatchesSpentOutputWithTxIndex(t *testing.T) {
	for _, flush := range []bool{false, true} {
		h := newAddrUnspentHarness(t)
		h.startDispatcher()
		c := newSpentOutputsChain(t, h)
		db := &viewCountingDB{DB: h.s.db}
		h.s.db = db
		h.s.txIndex = h.txIndex
		pool := []*txNotifyStream{h.subscribeAddress(t, false, true, false),
			h.subscribeAddress(t, true, true, false)}

		// A block confirms m1.  An output added since the last flush
		// leaves the cache when spent; one flushed before stays, marked
		// spent.
		if flush {
			err := h.chain.FlushCachedState(blockchain.FlushRequired)
			if err != nil {
				t.Fatalf("FlushCachedState: %v", err)
			}
		}
		h.mineBlock([]*wire.TxOut{wire.NewTxOut(50e8,
			p2shScript(t, otherRedeem), wire.TokenData{})},
			[]*wire.MsgTx{c.m1})
		entry, err := h.chain.FetchUtxoEntry(outPoint(c.a, 0))
		if err != nil || (entry != nil && !entry.IsSpent()) {
			t.Fatalf("flush %v: the spent output is unspent: %v", flush, err)
		}

		sentinel := sentinelTx(t)
		h.s.dispatchEvent(txEvent(c.m1))
		h.s.dispatchEvent(txEvent(sentinel))
		for _, stream := range pool {
			checkNotified(t, "memory pool", notified(t, stream,
				sentinel.TxHash()), map[string]*wire.MsgTx{"m1": c.m1})
		}
		if n := db.views.Load(); n != 1 {
			t.Errorf("flush %v: the spent output read %d times from the "+
				"transaction index, want once", flush, n)
		}
	}
}

// TestSpentScriptsLoadedOnce ensures the scripts of an event's spent outputs
// are loaded once, however many subscriptions look them up at once.
func TestSpentScriptsLoadedOnce(t *testing.T) {
	var spent spentScripts
	op := wire.OutPoint{Index: 1}
	var loads atomic.Int32
	load := func() map[wire.OutPoint][]byte {
		loads.Add(1)
		time.Sleep(10 * time.Millisecond)
		return map[wire.OutPoint][]byte{op: {txscript.OP_TRUE}}
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			lookup := spent.lookup(load)
			if got := lookup(op); !bytes.Equal(got, []byte{txscript.OP_TRUE}) {
				t.Errorf("got script %x", got)
			}
			if got := lookup(wire.OutPoint{}); got != nil {
				t.Errorf("got script %x for an output not spent", got)
			}
		})
	}
	wg.Wait()
	if n := loads.Load(); n != 1 {
		t.Errorf("loaded %d times", n)
	}
}

// TestSubscriptionMatchesTokenSpendInBlock ensures a spend in a block of a
// token output of a watched address matches, although the block's spend
// journal holds the output's script with its token prefix.
func TestSubscriptionMatchesTokenSpendInBlock(t *testing.T) {
	h := newAddrUnspentHarness(t)
	h.startDispatcher()
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	cb1 := h.mineBlock([]*wire.TxOut{
		wire.NewTxOut(50e8, otherScript, wire.TokenData{})}, nil)
	amount := uint64(1000)
	tokenData, err := wire.NewTokenData(cb1.TxHash(), &amount, nil, nil)
	if err != nil {
		t.Fatalf("NewTokenData: %v", err)
	}
	coinbaseOuts := []*wire.TxOut{
		wire.NewTxOut(50e8, otherScript, wire.TokenData{})}
	t1 := spendTx(t, outPoint(cb1, 0), otherRedeem,
		wire.NewTxOut(49e8, addrScript, *tokenData))
	h.mineBlock(coinbaseOuts, []*wire.MsgTx{t1})
	t2 := spendTx(t, outPoint(t1, 0), addrRedeem,
		wire.NewTxOut(48e8, otherScript, *tokenData))
	h.mineBlock(coinbaseOuts, []*wire.MsgTx{t2})
	block3, err := h.chain.BlockByHash(&h.prevHash)
	if err != nil {
		t.Fatalf("BlockByHash: %v", err)
	}
	blocks := []*txNotifyStream{h.subscribeAddress(t, false, false, true),
		h.subscribeAddress(t, true, false, true)}

	sentinel := sentinelTx(t)
	h.s.dispatchEvent(blockEvent(block3))
	h.s.dispatchEvent(blockEvent(bchutil.NewBlock(
		&wire.MsgBlock{Transactions: []*wire.MsgTx{sentinel}})))
	for _, stream := range blocks {
		checkNotified(t, "blocks", notified(t, stream, sentinel.TxHash()),
			map[string]*wire.MsgTx{"t2": t2})
	}
}
