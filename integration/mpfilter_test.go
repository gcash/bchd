// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs"
	"github.com/gcash/bchutil/gcs/builder"
)

// mpFilterPeer is a raw peer connection to the harness, with a buffer for each
// kind of reply the mempool filter checks wait on. The buffers are large
// enough that the peer's read loop, which runs these listeners, does not block
// on the test.
type mpFilterPeer struct {
	peer     *peer.Peer
	t        *testing.T
	filters  chan *wire.MsgMpFilter
	txs      chan *wire.MsgMpTxs
	cfilters chan *wire.MsgCFilter
	pongs    chan *wire.MsgPong
	ready    chan struct{}
}

// newMpFilterPeer dials addr and associates the connection. The caller waits
// for the handshake and closes the peer.
func newMpFilterPeer(t *testing.T, addr string) *mpFilterPeer {
	t.Helper()
	c := &mpFilterPeer{
		t:        t,
		filters:  make(chan *wire.MsgMpFilter, 16),
		txs:      make(chan *wire.MsgMpTxs, 16),
		cfilters: make(chan *wire.MsgCFilter, 4),
		pongs:    make(chan *wire.MsgPong, 4),
		ready:    make(chan struct{}, 1),
	}
	p, err := peer.NewOutboundPeer(&peer.Config{
		UserAgentName:    "relay",
		UserAgentVersion: "1.0.0",
		ChainParams:      &chaincfg.SimNetParams,
		Listeners: peer.MessageListeners{
			// A message queued before the node has sent verack is read
			// and ignored while the node finishes its handshake.
			OnVerAck: func(*peer.Peer, *wire.MsgVerAck) {
				select {
				case c.ready <- struct{}{}:
				default:
				}
			},
			OnMpFilter: func(_ *peer.Peer, msg *wire.MsgMpFilter) {
				c.filters <- msg
			},
			OnMpTxs: func(_ *peer.Peer, msg *wire.MsgMpTxs) {
				c.txs <- msg
			},
			OnCFilter: func(_ *peer.Peer, msg *wire.MsgCFilter) {
				c.cfilters <- msg
			},
			OnPong: func(_ *peer.Peer, msg *wire.MsgPong) {
				c.pongs <- msg
			},
		},
	}, addr)
	if err != nil {
		t.Fatalf("peer: %v", err)
	}
	conn, err := net.Dial("tcp", p.Addr())
	if err != nil {
		t.Fatalf("dial %s: %v", p.Addr(), err)
	}
	c.peer = p
	p.AssociateConnection(conn)
	return c
}

// waitHandshake waits until the node has sent verack. Setup fails when that
// does not happen within 10 seconds.
func (c *mpFilterPeer) waitHandshake() {
	c.t.Helper()
	select {
	case <-c.ready:
	case <-time.After(10 * time.Second):
		c.t.Fatalf("peer %s did not finish the handshake", c.peer.Addr())
	}
}

// waitFilter waits up to timeout for an mpfilter. The bool is false when none
// arrives.
func (c *mpFilterPeer) waitFilter(timeout time.Duration) (*wire.MsgMpFilter, bool) {
	c.t.Helper()
	select {
	case msg := <-c.filters:
		return msg, true
	case <-time.After(timeout):
		return nil, false
	}
}

// waitTxs waits up to timeout for an mptxs message. The bool is false when
// none arrives.
func (c *mpFilterPeer) waitTxs(timeout time.Duration) (*wire.MsgMpTxs, bool) {
	c.t.Helper()
	select {
	case msg := <-c.txs:
		return msg, true
	case <-time.After(timeout):
		return nil, false
	}
}

// waitCFilter waits up to timeout for a cfilter. The bool is false when none
// arrives.
func (c *mpFilterPeer) waitCFilter(timeout time.Duration) (*wire.MsgCFilter, bool) {
	c.t.Helper()
	select {
	case msg := <-c.cfilters:
		return msg, true
	case <-time.After(timeout):
		return nil, false
	}
}

// waitPong waits up to timeout for a pong. The bool is false when none arrives.
func (c *mpFilterPeer) waitPong(timeout time.Duration) (*wire.MsgPong, bool) {
	c.t.Helper()
	select {
	case msg := <-c.pongs:
		return msg, true
	case <-time.After(timeout):
		return nil, false
	}
}

// collectFilters reads n mpfilter messages. It reports an error and returns
// what arrived when timeout passes first, so one slow reply does not wait out
// the timeout once per message.
func (c *mpFilterPeer) collectFilters(n int, timeout time.Duration) []*wire.MsgMpFilter {
	c.t.Helper()
	got := make([]*wire.MsgMpFilter, 0, n)
	deadline := time.After(timeout)
	for len(got) < n {
		select {
		case msg := <-c.filters:
			got = append(got, msg)
		case <-deadline:
			c.t.Errorf("got %d mpfilter messages within %s, want %d",
				len(got), timeout, n)
			return got
		}
	}
	return got
}

// collectTxs reads n mptxs messages. It reports an error and returns what
// arrived when timeout passes first.
func (c *mpFilterPeer) collectTxs(n int, timeout time.Duration) []*wire.MsgMpTxs {
	c.t.Helper()
	got := make([]*wire.MsgMpTxs, 0, n)
	deadline := time.After(timeout)
	for len(got) < n {
		select {
		case msg := <-c.txs:
			got = append(got, msg)
		case <-deadline:
			c.t.Errorf("got %d mptxs messages within %s, want %d",
				len(got), timeout, n)
			return got
		}
	}
	return got
}

// p2pkhScript is a pay-to-public-key-hash script whose 20-byte hash is fill
// repeated, so each payment has a script the filter check can name.
func p2pkhScript(t *testing.T, fill byte) []byte {
	t.Helper()
	addr, err := bchutil.NewAddressPubKeyHash(bytes.Repeat([]byte{fill}, 20),
		&chaincfg.SimNetParams)
	if err != nil {
		t.Fatalf("public key hash address: %v", err)
	}
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("p2pkh script: %v", err)
	}
	return script
}

// sendMempoolTx creates a transaction paying 10000 satoshis to a p2pkh script
// of fill and broadcasts it. The change output keeps the fee at the rate the
// harness is asked for, so the node admits the transaction. Each call spends
// a different mature coinbase, which the harness locks.
func sendMempoolTx(t *testing.T, h *rpctest.Harness, fill byte) *wire.MsgTx {
	t.Helper()
	tx, err := h.CreateTransaction([]*wire.TxOut{
		wire.NewTxOut(10000, p2pkhScript(t, fill), wire.TokenData{}),
	}, 10, true)
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	if _, err := h.Node.SendRawTransaction(tx, false); err != nil {
		t.Fatalf("sendrawtransaction: %v", err)
	}
	return tx
}

// spentScripts returns the locking scripts of the outputs txs spend, in order
// of the transactions and of their inputs. Simnet has no tokens, so that
// script is the output's PkScript with no token prefix in front of it.
func spentScripts(t *testing.T, h *rpctest.Harness, txs []*wire.MsgTx) [][]byte {
	t.Helper()
	var scripts [][]byte
	for _, tx := range txs {
		for _, in := range tx.TxIn {
			prev, err := h.Node.GetRawTransaction(&in.PreviousOutPoint.Hash)
			if err != nil {
				t.Fatalf("getrawtransaction %v: %v",
					in.PreviousOutPoint.Hash, err)
			}
			outs := prev.MsgTx().TxOut
			idx := in.PreviousOutPoint.Index
			if int(idx) >= len(outs) {
				t.Fatalf("transaction %v has no output %d",
					in.PreviousOutPoint.Hash, idx)
			}
			scripts = append(scripts, outs[idx].PkScript)
		}
	}
	return scripts
}

// intervalTxs is one mpfilter and the transactions the node put in that
// interval, in the order it admitted them.
type intervalTxs struct {
	filter *wire.MsgMpFilter
	txs    []*wire.MsgTx
}

// intervalsFor reads the mpfilter messages for txs, which were sent in order
// and back to back. They share one interval unless the node closed the
// interval between them, in which case each transaction has the next filter
// and the second is given the same 15 seconds as the first.
func intervalsFor(t *testing.T, sub *mpFilterPeer, txs []*wire.MsgTx) []intervalTxs {
	t.Helper()
	msg, ok := sub.waitFilter(15 * time.Second)
	if !ok {
		t.Errorf("no mpfilter within 15 seconds of sending the transactions")
		return nil
	}
	if msg.TxCount == uint32(len(txs)) {
		return []intervalTxs{{filter: msg, txs: txs}}
	}
	if msg.TxCount != 1 || len(txs) != 2 {
		t.Errorf("mpfilter has %d transactions, want %d", msg.TxCount, len(txs))
		return nil
	}
	next, ok := sub.waitFilter(15 * time.Second)
	if !ok {
		t.Errorf("no mpfilter for the second transaction within 15 seconds")
		return nil
	}
	if next.TxCount != 1 {
		t.Errorf("second mpfilter has %d transactions, want 1", next.TxCount)
		return nil
	}
	return []intervalTxs{
		{filter: msg, txs: txs[:1]},
		{filter: next, txs: txs[1:]},
	}
}

// checkInterval checks one pushed filter: its type, the hash of its
// transactions in the order they were sent, its close time, the bytes the
// script filter builder produces for those transactions, and that the filter
// matches each script they pay or spend and not a script of 20 bytes of 0x09.
func checkInterval(t *testing.T, h *rpctest.Harness, iv intervalTxs) {
	t.Helper()
	msg := iv.filter
	if msg.FilterType != wire.GCSFilterScripts {
		t.Errorf("interval %d filter type %v, want the script filter",
			msg.Seq, msg.FilterType)
	}
	if msg.TxCount != uint32(len(iv.txs)) {
		t.Errorf("interval %d has %d transactions, want %d",
			msg.Seq, msg.TxCount, len(iv.txs))
	}
	ids := make([]chainhash.Hash, len(iv.txs))
	for i, tx := range iv.txs {
		ids[i] = tx.TxHash()
	}
	intervalHash := wire.MpIntervalHash(ids)
	if msg.IntervalHash != intervalHash {
		t.Errorf("interval %d hash %v, want %v",
			msg.Seq, msg.IntervalHash, intervalHash)
	}
	age := time.Now().Unix() - msg.Time
	if age < -60 || age > 60 {
		t.Errorf("interval %d closed at %d, which is %d seconds from now",
			msg.Seq, msg.Time, age)
	}

	prev := spentScripts(t, h, iv.txs)
	built, err := builder.BuildTxScriptFilter(&intervalHash, iv.txs, prev)
	if err != nil {
		t.Errorf("interval %d: builder: %v", msg.Seq, err)
		return
	}
	want, err := built.NBytes()
	if err != nil {
		t.Errorf("interval %d: filter bytes: %v", msg.Seq, err)
		return
	}
	if !bytes.Equal(msg.Data, want) {
		t.Errorf("interval %d filter bytes differ from the builder", msg.Seq)
	}

	parsed, err := gcs.FromNBytes(builder.DefaultP, builder.DefaultM, msg.Data)
	if err != nil {
		t.Errorf("interval %d filter does not parse: %v", msg.Seq, err)
		return
	}
	key := builder.DeriveKey(&intervalHash)
	scripts := make([][]byte, 0, len(prev))
	scripts = append(scripts, prev...)
	for _, tx := range iv.txs {
		for _, out := range tx.TxOut {
			scripts = append(scripts, out.PkScript)
		}
	}
	for _, script := range scripts {
		scriptKey := builder.ScriptKey(script)
		match, err := parsed.Match(key, scriptKey[:])
		if err != nil {
			t.Errorf("interval %d: match: %v", msg.Seq, err)
			continue
		}
		if !match {
			t.Errorf("interval %d filter misses a script it pays or spends",
				msg.Seq)
		}
	}
	absent := builder.ScriptKey(bytes.Repeat([]byte{0x09}, 20))
	match, err := parsed.Match(key, absent[:])
	if err != nil {
		t.Errorf("interval %d: match: %v", msg.Seq, err)
	} else if match {
		t.Errorf("interval %d filter matches a script of 20 bytes of 0x09",
			msg.Seq)
	}
}

// checkMpTxs checks that msg is the mptxs for seq and lists txs in order.
// inPool is true while the node should still send each transaction, and false
// once it should send only the ID.
func checkMpTxs(t *testing.T, msg *wire.MsgMpTxs, seq uint32, txs []*wire.MsgTx, inPool bool) {
	t.Helper()
	if msg.Seq != seq {
		t.Errorf("mptxs sequence %d, want %d", msg.Seq, seq)
	}
	if len(msg.Txs) != len(txs) {
		t.Errorf("mptxs sequence %d lists %d transactions, want %d",
			msg.Seq, len(msg.Txs), len(txs))
		return
	}
	for i, tx := range txs {
		want := tx.TxHash()
		got := &msg.Txs[i]
		if got.TxID != want {
			t.Errorf("mptxs sequence %d transaction %d id %v, want %v",
				msg.Seq, i, got.TxID, want)
		}
		if !inPool {
			if got.Tx != nil {
				t.Errorf("mptxs sequence %d transaction %d still has the transaction",
					msg.Seq, i)
			}
			continue
		}
		if got.Tx == nil {
			t.Errorf("mptxs sequence %d transaction %d has no transaction",
				msg.Seq, i)
			continue
		}
		if h := got.Tx.TxHash(); h != got.TxID {
			t.Errorf("mptxs sequence %d transaction %d id %v, transaction hash %v",
				msg.Seq, i, got.TxID, h)
		}
	}
}

// TestMempoolScriptFilters checks the script filters of mempool intervals. A
// subscribed peer is pushed an mpfilter only for an interval that admitted
// transactions. The filter is the one built over the scripts those
// transactions pay and spend, keyed by the hash of their IDs. A peer that
// connects later can ask for the filters and the transactions, including only
// the IDs once a block has mined them, and for the script filter of the whole
// mempool.
func TestMempoolScriptFilters(t *testing.T) {
	h, err := rpctest.New(&chaincfg.SimNetParams, nil, []string{"--txindex"})
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	defer func() { _ = h.TearDown() }()
	if err := h.SetUp(true, 25); err != nil {
		t.Fatalf("start node: %v", err)
	}

	sub := newMpFilterPeer(t, h.P2PAddress())
	defer sub.peer.Disconnect()
	sub.waitHandshake()
	sub.peer.QueueMessage(wire.NewMsgSendMpFilter(wire.GCSFilterScripts), nil)

	// An interval with no transactions is not announced.
	if msg, ok := sub.waitFilter(7 * time.Second); ok {
		t.Errorf("mpfilter sequence %d pushed while the mempool had no transaction",
			msg.Seq)
		return
	}

	t1 := sendMempoolTx(t, h, 0x01)
	t2 := sendMempoolTx(t, h, 0x02)
	sent := []*wire.MsgTx{t1, t2}
	intervals := intervalsFor(t, sub, sent)
	if len(intervals) == 0 {
		return
	}
	for _, iv := range intervals {
		checkInterval(t, h, iv)
		sub.peer.QueueMessage(wire.NewMsgGetMpTxs(wire.GCSFilterScripts,
			iv.filter.Seq, iv.filter.Seq), nil)
		msg, ok := sub.waitTxs(10 * time.Second)
		if !ok {
			t.Errorf("no mptxs for interval %d within 10 seconds", iv.filter.Seq)
			return
		}
		checkMpTxs(t, msg, iv.filter.Seq, iv.txs, true)
	}

	t3 := sendMempoolTx(t, h, 0x03)
	sent = append(sent, t3)
	msg, ok := sub.waitFilter(15 * time.Second)
	if !ok {
		t.Errorf("no mpfilter within 15 seconds of the third transaction")
		return
	}
	prev := intervals[len(intervals)-1].filter.Seq
	if msg.Seq != prev+1 {
		t.Errorf("interval sequence %d, want %d", msg.Seq, prev+1)
	}
	third := intervalTxs{filter: msg, txs: []*wire.MsgTx{t3}}
	checkInterval(t, h, third)
	intervals = append(intervals, third)

	first := intervals[0].filter.Seq
	last := intervals[len(intervals)-1].filter.Seq
	sub.peer.QueueMessage(wire.NewMsgGetMpTxs(wire.GCSFilterScripts, first, last), nil)
	gotTxs := sub.collectTxs(len(intervals), 10*time.Second)
	for i := range gotTxs {
		checkMpTxs(t, gotTxs[i], intervals[i].filter.Seq, intervals[i].txs, true)
	}

	// This peer was not subscribed, so it only hears the intervals it asks for.
	late := newMpFilterPeer(t, h.P2PAddress())
	defer late.peer.Disconnect()
	late.waitHandshake()
	late.peer.QueueMessage(wire.NewMsgGetMpFilters(wire.GCSFilterScripts, 0), nil)
	got := late.collectFilters(len(intervals), 10*time.Second)
	if len(got) == len(intervals) {
		for i := range got {
			want := intervals[i].filter
			if got[i].Seq != want.Seq {
				t.Errorf("getmpfilters filter %d has sequence %d, want %d",
					i, got[i].Seq, want.Seq)
			}
			if got[i].IntervalHash != want.IntervalHash {
				t.Errorf("getmpfilters filter %d hash %v, want %v",
					i, got[i].IntervalHash, want.IntervalHash)
			}
			if got[i].TxCount != want.TxCount {
				t.Errorf("getmpfilters filter %d has %d transactions, want %d",
					i, got[i].TxCount, want.TxCount)
			}
			if !bytes.Equal(got[i].Data, want.Data) {
				t.Errorf("getmpfilters filter %d bytes differ from the subscriber's filter",
					i)
			}
		}
	}

	late.peer.QueueMessage(wire.NewMsgGetMpFilters(wire.GCSFilterScripts, last+1), nil)
	if msg, ok := late.waitFilter(3 * time.Second); ok {
		t.Errorf("getmpfilters from sequence %d returned interval %d",
			last+1, msg.Seq)
	}
	late.peer.QueueMessage(wire.NewMsgGetMpTxs(wire.GCSFilterScripts, last+5, last+5), nil)
	if msg, ok := late.waitTxs(3 * time.Second); ok {
		t.Errorf("getmptxs for sequence %d returned interval %d", last+5, msg.Seq)
	}
	late.peer.QueueMessage(wire.NewMsgGetMpFilters(wire.GCSFilterRegular, 0), nil)
	if msg, ok := late.waitFilter(3 * time.Second); ok {
		t.Errorf("getmpfilters for the regular filter type returned interval %d",
			msg.Seq)
	}

	// getcfmempool adds 10 to this peer's ban score each time. These two,
	// after the getmpfilters and getmptxs already sent here, stay under the
	// threshold of 100 at which the node disconnects a peer.
	late.peer.QueueMessage(wire.NewMsgGetCFMempool(wire.GCSFilterScripts), nil)
	cf, ok := late.waitCFilter(10 * time.Second)
	if !ok {
		t.Errorf("no cfilter for the mempool's script filter within 10 seconds")
	} else {
		if cf.FilterType != wire.GCSFilterScripts {
			t.Errorf("mempool cfilter type %v, want the script filter", cf.FilterType)
		}
		if cf.BlockHash != (chainhash.Hash{}) {
			t.Errorf("mempool cfilter block hash %v, want the zero hash", cf.BlockHash)
		}
		// The builder's result does not depend on the order of the
		// transactions: a filter holds a set.
		spent := spentScripts(t, h, sent)
		built, err := builder.BuildTxScriptFilter(&chainhash.Hash{}, sent, spent)
		if err != nil {
			t.Errorf("mempool script filter: builder: %v", err)
		} else if want, err := built.NBytes(); err != nil {
			t.Errorf("mempool script filter: filter bytes: %v", err)
		} else if !bytes.Equal(cf.Data, want) {
			t.Errorf("mempool script filter bytes differ from the builder")
		}
	}

	late.peer.QueueMessage(wire.NewMsgGetCFMempool(wire.GCSFilterRegular), nil)
	regular, ok := late.waitCFilter(10 * time.Second)
	if !ok {
		t.Errorf("no cfilter for the mempool's regular filter within 10 seconds")
	} else if regular.FilterType != wire.GCSFilterRegular {
		t.Errorf("mempool cfilter type %v, want the regular filter", regular.FilterType)
	}

	if _, err := h.Node.Generate(1); err != nil {
		t.Fatalf("generate: %v", err)
	}
	sub.peer.QueueMessage(wire.NewMsgGetMpTxs(wire.GCSFilterScripts, first, first), nil)
	mined, ok := sub.waitTxs(10 * time.Second)
	if !ok {
		t.Errorf("no mptxs for interval %d after the block within 10 seconds", first)
	} else {
		checkMpTxs(t, mined, first, intervals[0].txs, false)
	}

	sub.peer.QueueMessage(wire.NewMsgPing(1), nil)
	pong, ok := sub.waitPong(10 * time.Second)
	if !ok {
		t.Errorf("the subscriber sent no pong within 10 seconds")
	} else if pong.Nonce != 1 {
		t.Errorf("subscriber pong nonce %d, want 1", pong.Nonce)
	}
	late.peer.QueueMessage(wire.NewMsgPing(2), nil)
	pong, ok = late.waitPong(10 * time.Second)
	if !ok {
		t.Errorf("the later peer sent no pong within 10 seconds")
	} else if pong.Nonce != 2 {
		t.Errorf("later peer pong nonce %d, want 2", pong.Nonce)
	}
}
