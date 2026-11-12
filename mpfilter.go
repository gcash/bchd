// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"sync"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil/gcs/builder"
)

const (
	// mpFilterInterval is how long the node gathers the transactions it
	// admits to its mempool before it builds their filter, so a wallet
	// hears of a payment within about that long.
	mpFilterInterval = 5 * time.Second

	// mpFilterMaxTxs and mpFilterMaxBytes close an interval early, so that
	// its filter and its transactions stay small enough to send however
	// many transactions the node admits.
	mpFilterMaxTxs   = 10000
	mpFilterMaxBytes = 8000000

	// mpFilterKeep is how many closed intervals the node keeps for wallets
	// that reconnect: an hour of them at one every mpFilterInterval.
	mpFilterKeep = 720

	// mpFilterBanScore and mpTxsBanScore are added to the ban score of a
	// peer for a getmpfilters and a getmptxs message.
	mpFilterBanScore = 10
	mpTxsBanScore    = 5
)

// mpInterval is a closed interval of transactions the node admitted to its
// mempool. It keeps their IDs, not the transactions, which are read from the
// mempool when a peer asks for them.
type mpInterval struct {
	seq    uint32
	time   int64
	hash   chainhash.Hash
	txids  []chainhash.Hash
	filter []byte
}

// msg returns the mpfilter message of the interval.
func (iv *mpInterval) msg() *wire.MsgMpFilter {
	return &wire.MsgMpFilter{
		FilterType:   wire.GCSFilterScripts,
		Seq:          iv.seq,
		Time:         iv.time,
		IntervalHash: iv.hash,
		TxCount:      uint32(len(iv.txids)),
		Data:         iv.filter,
	}
}

// mpFilters gathers the transactions the node admits to its mempool into
// short intervals, builds the script filter (filter type 1, gcash/bchd#767)
// of each, sends it to the peers that asked with sendmpfilter, and keeps the
// latest for those that reconnect. A wallet matches each filter with its own
// scripts and token categories and asks for an interval's transactions only
// when it matches, so nothing it watches reaches the node.
type mpFilters struct {
	mtx sync.Mutex

	// The open interval: its transactions, the scripts of the outputs they
	// spend, in order, and their size.
	txs            []*wire.MsgTx
	prevOutScripts [][]byte
	bytes          int

	// nextSeq numbers the next interval that closes. It starts from the
	// time the node started, in seconds, so it keeps growing across
	// restarts while intervals close less than once a second on average.
	nextSeq uint32

	// kept holds the latest closed intervals, oldest first.
	kept []*mpInterval

	// subscribers are the peers that sent sendmpfilter.
	subscribers map[*serverPeer]struct{}

	now func() time.Time
}

// newMpFilters returns an empty mpFilters.
func newMpFilters() *mpFilters {
	return &mpFilters{
		nextSeq:     uint32(time.Now().Unix()),
		subscribers: make(map[*serverPeer]struct{}),
		now:         time.Now,
	}
}

// add puts a transaction the mempool admitted in the open interval, with the
// scripts of the outputs it spends, each with its token prefix in front. It
// closes the interval when that fills it.
func (m *mpFilters) add(tx *wire.MsgTx, prevOutScripts [][]byte) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.txs = append(m.txs, tx)
	m.prevOutScripts = append(m.prevOutScripts, prevOutScripts...)
	m.bytes += tx.SerializeSize()
	if len(m.txs) >= mpFilterMaxTxs || m.bytes >= mpFilterMaxBytes {
		m.closeInterval()
	}
}

// tick closes the open interval. It is called every mpFilterInterval.
func (m *mpFilters) tick() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.closeInterval()
}

// closeInterval builds the filter of the open interval, keeps it, sends it to
// the subscribers and opens the next one. An interval without transactions
// gets no filter and no number. It must be called with the mutex held.
func (m *mpFilters) closeInterval() {
	if len(m.txs) == 0 {
		return
	}
	iv := &mpInterval{
		seq:   m.nextSeq,
		time:  m.now().Unix(),
		txids: make([]chainhash.Hash, len(m.txs)),
	}
	for i, tx := range m.txs {
		iv.txids[i] = tx.TxHash()
	}
	iv.hash = wire.MpIntervalHash(iv.txids)
	filter, err := builder.BuildTxScriptFilter(&iv.hash, m.txs, m.prevOutScripts)
	m.txs, m.prevOutScripts, m.bytes = nil, nil, 0
	if err == nil {
		iv.filter, err = filter.NBytes()
	}
	if err != nil {
		peerLog.Errorf("Could not build the mempool filter of %d "+
			"transactions: %v", len(iv.txids), err)
		return
	}
	m.nextSeq++

	m.kept = append(m.kept, iv)
	if len(m.kept) > mpFilterKeep {
		m.kept = m.kept[len(m.kept)-mpFilterKeep:]
	}
	msg := iv.msg()
	for sp := range m.subscribers {
		if !sp.Connected() {
			delete(m.subscribers, sp)
			continue
		}
		sp.QueueMessage(msg, nil)
	}
}

// subscribe adds a peer to those that get each interval's filter.
func (m *mpFilters) subscribe(sp *serverPeer) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.subscribers[sp] = struct{}{}
}

// since returns the kept intervals numbered startSeq or above, oldest first.
func (m *mpFilters) since(startSeq uint32) []*mpInterval {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for i, iv := range m.kept {
		if iv.seq >= startSeq {
			return append([]*mpInterval(nil), m.kept[i:]...)
		}
	}
	return nil
}

// interval returns the kept interval numbered seq, or nil.
func (m *mpFilters) interval(seq uint32) *mpInterval {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for _, iv := range m.kept {
		if iv.seq == seq {
			return iv
		}
	}
	return nil
}

// run closes the open interval every mpFilterInterval until quit closes.
func (m *mpFilters) run(quit <-chan struct{}) {
	ticker := time.NewTicker(mpFilterInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.tick()
		case <-quit:
			return
		}
	}
}

// addMempoolTxs puts transactions the mempool just admitted in the open
// interval of the mempool filters. A transaction whose spent outputs can't
// all be found any more, as when a block took it meanwhile, is left out.
func (s *server) addMempoolTxs(txns []*mempool.TxDesc) {
	if s.mpFilters == nil {
		return
	}
	for _, txDesc := range txns {
		view, err := s.txMemPool.FetchInputUtxos(txDesc.Tx)
		if err != nil {
			continue
		}
		msgTx := txDesc.Tx.MsgTx()
		prevOutScripts := make([][]byte, 0, len(msgTx.TxIn))
		for _, txIn := range msgTx.TxIn {
			entry := view.LookupEntry(txIn.PreviousOutPoint)
			if entry == nil {
				break
			}
			// The builder takes the script of an output that
			// carries tokens with its token prefix in front, as
			// the spend journal keeps it.
			script := entry.PkScript()
			if tokenData := entry.TokenData(); !tokenData.IsEmpty() {
				prefix := tokenData.TokenDataBuffer()
				script = append(prefix.Bytes(), script...)
			}
			prevOutScripts = append(prevOutScripts, script)
		}
		if len(prevOutScripts) != len(msgTx.TxIn) {
			continue
		}
		s.mpFilters.add(msgTx, prevOutScripts)
	}
}

// mpFiltersFor returns the mempool filters when the node keeps them and the
// filter type is the script filter's, the only one they are built for.
func (sp *serverPeer) mpFiltersFor(filterType wire.FilterType, command string) *mpFilters {
	if sp.server.mpFilters == nil || filterType != wire.GCSFilterScripts {
		peerLog.Debugf("Ignoring %s from %s: no mempool filters of type %v",
			command, sp, filterType)
		return nil
	}
	return sp.server.mpFilters
}

// OnSendMpFilter is invoked when a peer receives a sendmpfilter bitcoin
// message. From then on the peer gets the filter of each interval of
// transactions the node admits to its mempool, as the interval closes.
func (sp *serverPeer) OnSendMpFilter(_ *peer.Peer, msg *wire.MsgSendMpFilter) {
	if m := sp.mpFiltersFor(msg.FilterType, "sendmpfilter"); m != nil {
		m.subscribe(sp)
	}
}

// OnGetMpFilters is invoked when a peer receives a getmpfilters bitcoin
// message. It sends the filters of the intervals the node still keeps from
// the sequence number asked for, so a peer that reconnects catches up.
func (sp *serverPeer) OnGetMpFilters(_ *peer.Peer, msg *wire.MsgGetMpFilters) {
	m := sp.mpFiltersFor(msg.FilterType, "getmpfilters")
	if m == nil {
		return
	}
	sp.addBanScore(0, mpFilterBanScore, "getmpfilters")
	for _, iv := range m.since(msg.StartSeq) {
		sp.QueueMessage(iv.msg(), nil)
	}
}

// OnGetMpTxs is invoked when a peer receives a getmptxs bitcoin message. It
// sends every transaction of the interval, so the node doesn't learn which
// one the peer wants: the transaction while the mempool has it, or only its
// ID once it was mined or dropped.
func (sp *serverPeer) OnGetMpTxs(_ *peer.Peer, msg *wire.MsgGetMpTxs) {
	m := sp.mpFiltersFor(msg.FilterType, "getmptxs")
	if m == nil {
		return
	}
	sp.addBanScore(0, mpTxsBanScore, "getmptxs")
	iv := m.interval(msg.Seq)
	if iv == nil {
		peerLog.Debugf("Ignoring getmptxs from %s: interval %d is not kept",
			sp, msg.Seq)
		return
	}
	reply := &wire.MsgMpTxs{
		FilterType: msg.FilterType,
		Seq:        iv.seq,
		Txs:        make([]wire.MpTx, len(iv.txids)),
	}
	for i := range iv.txids {
		reply.Txs[i].TxID = iv.txids[i]
		if tx, err := sp.server.txMemPool.FetchTransaction(&iv.txids[i]); err == nil {
			reply.Txs[i].Tx = tx.MsgTx()
		}
	}
	sp.QueueMessage(reply, nil)
}
