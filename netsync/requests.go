// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"slices"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

const (
	// txRequestTimeout is how long a transaction requested from a peer is
	// waited for before it is requested from the next peer that announced
	// it.  It is BCHN's GETDATA_TX_INTERVAL.
	txRequestTimeout = time.Minute

	// maxPeerTxAnnouncements is the most transactions announced by a peer
	// that are tracked, whether requested from it or kept to request from
	// it if a request to another peer fails.  Its other announcements are
	// ignored.  It is BCHN's MAX_PEER_TX_ANNOUNCEMENTS.
	maxPeerTxAnnouncements = 2 * wire.MaxInvPerMsg

	// maxBlockAnnouncers is the most peers kept for a requested block,
	// besides the one it is requested from, to request it from if that
	// request fails.
	maxBlockAnnouncers = 8

	// requestCheckInterval is how often requests are checked for timeouts.
	requestCheckInterval = 5 * time.Second
)

// txRequest is a request for a transaction from a peer.
type txRequest struct {
	peer   *peerpkg.Peer
	expiry time.Time

	// announcers are the other peers that announced the transaction, in
	// the order they announced it, to request it from if the request fails.
	announcers []*peerpkg.Peer
}

// txExpiry is the time a request for a transaction from a peer expires.  All
// requests have the same timeout, so these are queued in the order they
// expire.
type txExpiry struct {
	hash   chainhash.Hash
	peer   *peerpkg.Peer
	expiry time.Time
}

// getDataQueue collects the getdata messages for peers, so the requests made
// together go out in as few messages as possible.
type getDataQueue map[*peerpkg.Peer]*wire.MsgGetData

// add adds a request for the inventory to the peer's message, sending the
// message first when it is full.
func (q getDataQueue) add(peer *peerpkg.Peer, iv *wire.InvVect) {
	msg := q[peer]
	if msg == nil || len(msg.InvList) >= wire.MaxInvPerMsg {
		if msg != nil {
			peer.QueueMessage(msg, nil)
		}
		msg = wire.NewMsgGetData()
		q[peer] = msg
	}
	_ = msg.AddInvVect(iv)
}

// send sends the collected messages.
func (q getDataQueue) send() {
	for peer, msg := range q {
		if len(msg.InvList) > 0 {
			peer.QueueMessage(msg, nil)
		}
	}
}

// newPeerSyncState returns the state of a new peer.
func newPeerSyncState(syncCandidate bool) *peerSyncState {
	return &peerSyncState{
		syncCandidate:   syncCandidate,
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		announcedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}
}

// now returns the current time, from the clock a test set if there is one.
func (sm *SyncManager) now() time.Time {
	if sm.timeNow != nil {
		return sm.timeNow()
	}
	return time.Now()
}

// announceTx handles the peer's announcement of a transaction that is not
// known.  It requests the transaction from the peer when no other peer is
// asked for it, and otherwise keeps the peer to request it from if that
// request fails.  It returns whether it requested the transaction.
func (sm *SyncManager) announceTx(peer *peerpkg.Peer, state *peerSyncState,
	hash *chainhash.Hash, q getDataQueue) bool {

	if len(state.requestedTxns)+len(state.announcedTxns) >=
		maxPeerTxAnnouncements {

		return false
	}
	req, ok := sm.requestedTxns[*hash]
	if !ok {
		sm.requestTx(peer, state, hash, &txRequest{}, q)
		return true
	}
	if req.peer == peer {
		return false
	}
	if _, ok := state.announcedTxns[*hash]; ok {
		return false
	}
	if state.announcedTxns == nil {
		state.announcedTxns = make(map[chainhash.Hash]struct{})
	}
	state.announcedTxns[*hash] = struct{}{}
	req.announcers = append(req.announcers, peer)
	return false
}

// requestTx makes req the request for the transaction from the peer, and
// adds the request to q.
func (sm *SyncManager) requestTx(peer *peerpkg.Peer, state *peerSyncState,
	hash *chainhash.Hash, req *txRequest, q getDataQueue) {

	req.peer = peer
	req.expiry = sm.now().Add(txRequestTimeout)
	sm.requestedTxns[*hash] = req
	state.requestedTxns[*hash] = struct{}{}
	sm.txExpiries = append(sm.txExpiries, txExpiry{
		hash:   *hash,
		peer:   peer,
		expiry: req.expiry,
	})
	q.add(peer, wire.NewInvVect(wire.InvTypeTx, hash))
}

// retryTxRequest moves the request for the transaction to the next peer that
// announced it and is still connected, adding the request to q, or forgets it
// when there is none.
func (sm *SyncManager) retryTxRequest(hash *chainhash.Hash, q getDataQueue) {
	req, ok := sm.requestedTxns[*hash]
	if !ok {
		return
	}
	if state, ok := sm.peerStates[req.peer]; ok {
		delete(state.requestedTxns, *hash)
	}
	for len(req.announcers) > 0 {
		next := req.announcers[0]
		req.announcers = req.announcers[1:]
		state, ok := sm.peerStates[next]
		if !ok || !next.Connected() {
			continue
		}
		delete(state.announcedTxns, *hash)
		sm.requestTx(next, state, hash, req, q)
		return
	}
	delete(sm.requestedTxns, *hash)
}

// forgetTxRequest forgets the request for the transaction and the peers that
// announced it.
func (sm *SyncManager) forgetTxRequest(hash *chainhash.Hash) {
	req, ok := sm.requestedTxns[*hash]
	if !ok {
		return
	}
	delete(sm.requestedTxns, *hash)
	if state, ok := sm.peerStates[req.peer]; ok {
		delete(state.requestedTxns, *hash)
	}
	for _, peer := range req.announcers {
		if state, ok := sm.peerStates[peer]; ok {
			delete(state.announcedTxns, *hash)
		}
	}
}

// requestBlock records the request for the block from the peer, which the
// caller sends.  The peer may then send the block.  validHeader says whether
// the block's header was validated, as headers-first sync validates them.
func (sm *SyncManager) requestBlock(peer *peerpkg.Peer, state *peerSyncState,
	hash *chainhash.Hash, validHeader bool) {

	if requester, ok := sm.requestedBlocks[*hash]; ok {
		if requester == peer {
			return
		}
		sm.endBlockRequest(hash)
	}
	if state.blocksInFlight == 0 {
		state.downloadingSince = sm.now()
	}
	state.blocksInFlight++
	if validHeader {
		state.validBlocksInFlight++
		sm.validBlockRequests[*hash] = struct{}{}
	}
	sm.requestedBlocks[*hash] = peer
	state.requestedBlocks[*hash] = struct{}{}
}

// endBlockRequest stops waiting for the block from the peer it was requested
// from, which may still send it, and forgets the peers that announced it.  The
// peer's download timer restarts.
func (sm *SyncManager) endBlockRequest(hash *chainhash.Hash) {
	peer, ok := sm.requestedBlocks[*hash]
	if !ok {
		return
	}
	_, valid := sm.validBlockRequests[*hash]
	delete(sm.requestedBlocks, *hash)
	delete(sm.validBlockRequests, *hash)
	delete(sm.blockAnnouncers, *hash)
	state, ok := sm.peerStates[peer]
	if !ok {
		return
	}
	if valid && state.validBlocksInFlight > 0 {
		state.validBlocksInFlight--
	}
	if state.blocksInFlight > 0 {
		state.blocksInFlight--
		state.downloadingSince = sm.now()
	}
}

// addBlockAnnouncer keeps the peer to request the block from if the request
// to another peer fails, unless enough connected peers are kept already.
func (sm *SyncManager) addBlockAnnouncer(peer *peerpkg.Peer,
	hash *chainhash.Hash) {

	announcers := sm.blockAnnouncers[*hash]
	if slices.Contains(announcers, peer) {
		return
	}
	announcers = slices.DeleteFunc(announcers, func(p *peerpkg.Peer) bool {
		_, ok := sm.peerStates[p]
		return !ok
	})
	if len(announcers) < maxBlockAnnouncers {
		announcers = append(announcers, peer)
	}
	sm.blockAnnouncers[*hash] = announcers
}

// retryBlockRequest moves the request for the block to the next peer that
// announced it and is still connected, adding the request to q, or ends it
// when there is none.
func (sm *SyncManager) retryBlockRequest(hash *chainhash.Hash,
	q getDataQueue) {

	announcers := sm.blockAnnouncers[*hash]
	_, valid := sm.validBlockRequests[*hash]
	sm.endBlockRequest(hash)
	for i, next := range announcers {
		state, ok := sm.peerStates[next]
		if !ok || !next.Connected() {
			continue
		}
		sm.requestBlock(next, state, hash, valid)
		if rest := announcers[i+1:]; len(rest) > 0 {
			sm.blockAnnouncers[*hash] = rest
		}
		iv := wire.NewInvVect(wire.InvTypeBlock, hash)
		if sm.current() && next.ProtocolVersion() >= wire.BIP0152Version {
			iv.Type = wire.InvTypeCmpctBlock
		}
		q.add(next, iv)
		return
	}
}

// blockTimeout returns how long a peer may go without delivering a block it
// was asked for while the passed number of other peers are asked for blocks
// whose headers were validated: as in BCHN, a block interval, and half of one
// more for each of them.  Only blocks with validated headers count, so that
// peers announcing blocks that do not exist cannot stretch the timeout.
func (sm *SyncManager) blockTimeout(others int) time.Duration {
	return sm.chainParams.TargetTimePerBlock * time.Duration(2+others) / 2
}

// handleRequestTimeouts requests the transactions whose requests expired from
// the next peers that announced them, and disconnects the peers that have
// gone too long without delivering a block they were asked for, as BCHN does.
// Their blocks are requested from other peers that announced them once the
// peers are done.
func (sm *SyncManager) handleRequestTimeouts() {
	now := sm.now()
	q := make(getDataQueue)
	for len(sm.txExpiries) > 0 && !sm.txExpiries[0].expiry.After(now) {
		e := sm.txExpiries[0]
		sm.txExpiries = sm.txExpiries[1:]
		req, ok := sm.requestedTxns[e.hash]
		if ok && req.peer == e.peer && req.expiry.Equal(e.expiry) {
			sm.retryTxRequest(&e.hash, q)
		}
	}
	q.send()

	validDownloads := 0
	for _, state := range sm.peerStates {
		if state.validBlocksInFlight > 0 {
			validDownloads++
		}
	}
	for peer, state := range sm.peerStates {
		if state.blocksInFlight == 0 || !peer.Connected() {
			continue
		}
		others := validDownloads
		if state.validBlocksInFlight > 0 {
			others--
		}
		if now.Sub(state.downloadingSince) > sm.blockTimeout(others) {
			log.Infof("Timeout downloading blocks from peer %s -- "+
				"disconnecting", peer)
			peer.Disconnect()
		}
	}
}
