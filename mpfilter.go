// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"sync"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
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
	// peer for a getmpfilters and a getmptxs message.  A wallet that
	// downloads every interval, as one that hides what it watches does
	// while intervals are small, sends a getmptxs every mpFilterInterval,
	// which at 2 each settles at a score near 35.
	mpFilterBanScore = 10
	mpTxsBanScore    = 2

	// mpTxsMaxBytes is how many bytes of transactions one getmptxs is
	// answered with before the intervals left in its run are left out. The
	// peer sees which intervals came, and asks for the rest.
	mpTxsMaxBytes = 32000000
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

	// An interval without transactions sends nothing, so the peers that
	// left are dropped here, or a quiet node would keep every peer that
	// ever asked.
	for sp := range m.subscribers {
		if !sp.Connected() {
			delete(m.subscribers, sp)
		}
	}
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
	if ivs := m.run(seq, seq); len(ivs) == 1 {
		return ivs[0]
	}
	return nil
}

// run returns the kept intervals numbered firstSeq to lastSeq, oldest first.
func (m *mpFilters) run(firstSeq, lastSeq uint32) []*mpInterval {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	var ivs []*mpInterval
	for _, iv := range m.kept {
		if iv.seq >= firstSeq && iv.seq <= lastSeq {
			ivs = append(ivs, iv)
		}
	}
	return ivs
}

// serve closes the open interval every mpFilterInterval until quit closes.
func (m *mpFilters) serve(quit <-chan struct{}) {
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
		prevOutScripts, err := s.spentScripts(txDesc.Tx)
		if err != nil {
			// Any other failure leaves a transaction that is still in
			// the mempool out of every interval, which a wallet can't
			// tell, so it is logged.
			if !errors.Is(err, errSpentOutputGone) {
				peerLog.Warnf("Transaction %v is in no mempool filter: %v",
					txDesc.Tx.Hash(), err)
			}
			continue
		}
		s.mpFilters.add(txDesc.Tx.MsgTx(), prevOutScripts)
	}
}

// errSpentOutputGone is returned by spentScripts for a transaction that spends
// an output found in neither the UTXO set nor the mempool, as when a block
// took the transaction meanwhile.
var errSpentOutputGone = errors.New("an output the transaction spends is gone")

// spentScripts returns the scripts of the outputs a mempool transaction
// spends, in order, from the UTXO set and the mempool, as the script filter
// builder takes them: the script of an output that carries tokens has its
// token prefix in front, as the spend journal keeps it. It returns
// errSpentOutputGone when an output can't be found any more, and the error of
// the lookup when that fails.
func (s *server) spentScripts(tx *bchutil.Tx) ([][]byte, error) {
	view, err := s.txMemPool.FetchInputUtxos(tx)
	if err != nil {
		return nil, err
	}
	msgTx := tx.MsgTx()
	prevOutScripts := make([][]byte, 0, len(msgTx.TxIn))
	for _, txIn := range msgTx.TxIn {
		entry := view.LookupEntry(txIn.PreviousOutPoint)
		if entry == nil {
			return nil, errSpentOutputGone
		}
		script := entry.PkScript()
		if tokenData := entry.TokenData(); !tokenData.IsEmpty() {
			prefix := tokenData.TokenDataBuffer()
			script = append(prefix.Bytes(), script...)
		}
		prevOutScripts = append(prevOutScripts, script)
	}
	return prevOutScripts, nil
}

// mempoolScriptFilter returns the script filter of the whole mempool, keyed
// by the zero hash as the regular filter of the mempool is. A wallet asks for
// it when it starts, for the transactions admitted before the intervals the
// node still keeps, and to compare what two peers show. It is built again
// only when the mempool changed.
func (s *server) mempoolScriptFilter() ([]byte, error) {
	s.mempoolFilterMtx.Lock()
	defer s.mempoolFilterMtx.Unlock()

	changes := s.txMemPool.Changes()
	if s.mempoolScriptFilterBytes != nil && s.mempoolScriptFilterChanges == changes {
		return s.mempoolScriptFilterBytes, nil
	}
	filterBytes, err := mempoolScriptFilterOf(s.txMemPool.TxDescs(), s.spentScripts)
	if err != nil {
		return nil, err
	}
	s.mempoolScriptFilterBytes = filterBytes
	s.mempoolScriptFilterChanges = changes
	return filterBytes, nil
}

// mempoolScriptFilterOf builds the script filter of the given mempool
// transactions, keyed by the zero hash. spent returns the scripts of the
// outputs a transaction spends, as spentScripts does. A transaction whose
// spent outputs are gone has left the mempool, and is left out. Any other
// failure gives no filter: one without a transaction that is still in the
// mempool would be taken for the whole of it, and kept until the mempool
// changed.
func mempoolScriptFilterOf(txDescs []*mempool.TxDesc,
	spent func(*bchutil.Tx) ([][]byte, error)) ([]byte, error) {

	var txs []*wire.MsgTx
	var prevOutScripts [][]byte
	for _, txDesc := range txDescs {
		scripts, err := spent(txDesc.Tx)
		if errors.Is(err, errSpentOutputGone) {
			continue
		}
		if err != nil {
			return nil, err
		}
		txs = append(txs, txDesc.Tx.MsgTx())
		prevOutScripts = append(prevOutScripts, scripts...)
	}
	filter, err := builder.BuildTxScriptFilter(&chainhash.Hash{}, txs, prevOutScripts)
	if err != nil {
		return nil, err
	}
	return filter.NBytes()
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
// sends every transaction of each interval in the run asked for, so the node
// doesn't learn which one the peer wants: the transaction while the mempool
// has it, or only its ID once it was mined or dropped.
func (sp *serverPeer) OnGetMpTxs(_ *peer.Peer, msg *wire.MsgGetMpTxs) {
	m := sp.mpFiltersFor(msg.FilterType, "getmptxs")
	if m == nil {
		return
	}
	sp.addBanScore(0, mpTxsBanScore, "getmptxs")
	sent := 0
	for _, iv := range m.run(msg.FirstSeq, msg.LastSeq) {
		if sent >= mpTxsMaxBytes {
			break
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
				sent += tx.MsgTx().SerializeSize()
			}
		}
		sp.QueueMessage(reply, nil)
	}
}
