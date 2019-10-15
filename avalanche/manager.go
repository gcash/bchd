package avalanche

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

const (
	// AvalancheFinalizationScore is the confidence score we consider to be final
	AvalancheFinalizationScore = 128

	// AvalancheTimeStep is the amount of time to wait between event ticks
	AvalancheTimeStep = 10 * time.Millisecond

	// AvalancheMaxElementPoll is the maximum number of invs to send in a single
	// query
	AvalancheMaxElementPoll = 4096

	// AvalancheRequestTimeout is the amount of time to wait for a response to a
	// query
	AvalancheRequestTimeout = 1 * time.Minute

	// AvalancheMaxInflightPoll is the max outstanding requests that we can have
	// for any inventory item.
	AvalancheMaxInflightPoll = 10

	// DeleteInventoryAfter is the maximum time we'll keep a transaction in memory
	// if it hasn't been finalized by avalanche.
	DeleteInventoryAfter = time.Hour * 6
)

// TxDesc wraps a mempool.TxDesc with a pointer to a reject code.
// A nil reject code means the transaction was accepted to the mempool.
type TxDesc struct {
	*mempool.TxDesc
	Code *wire.RejectCode
}

// newPeerMsg signifies a newly connected peer to the handler.
type newPeerMsg struct {
	peer *peer.Peer
}

// donePeerMsg signifies a disconnected peer to the handler.
type donePeerMsg struct {
	peer *peer.Peer
}

// newTxsMsg signifies new transactions to be processed.
type newTxsMsg struct {
	tx *TxDesc
}

type blockConnectedMsg struct {
	blk *bchutil.Block
}

// requestExpirationMsg signifies a request has expired and
// should be removed from the map.
type requestExpirationMsg struct {
	key string
}

// queryMsg signifies a query from another peer.
type queryMsg struct {
	request  *wire.MsgAvaRequest
	respChan chan *wire.MsgAvaResponse
}

// registerVotesMsg signifies a response to a query from another peer.
type registerVotesMsg struct {
	p    *peer.Peer
	resp *wire.MsgAvaResponse
}

type connectedPeerMsg struct {
	addr     net.Addr
	respChan chan bool
}

type AvalancheManager struct {
	peers   map[*peer.Peer]struct{}
	wg      sync.WaitGroup
	quit    chan struct{}
	msgChan chan interface{}

	voteRecords map[chainhash.Hash]*VoteRecord
	outpoints   map[wire.OutPoint][]*TxDesc
	rejectedTxs map[chainhash.Hash]struct{}
	round       int64
	queries     map[string]RequestRecord

	notificationCallback func(tx *bchutil.Tx, finalizationTime time.Duration)

	privKey *bchec.PrivateKey
}

func New() (*AvalancheManager, error) {
	avalanchePrivkey, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		return nil, err
	}
	return &AvalancheManager{
		peers:       make(map[*peer.Peer]struct{}),
		wg:          sync.WaitGroup{},
		quit:        make(chan struct{}),
		msgChan:     make(chan interface{}),
		voteRecords: make(map[chainhash.Hash]*VoteRecord),
		outpoints:   make(map[wire.OutPoint][]*TxDesc),
		rejectedTxs: make(map[chainhash.Hash]struct{}),
		queries:     make(map[string]RequestRecord),
		privKey:     avalanchePrivkey,
	}, nil
}

func (am *AvalancheManager) SetNotificationCallback(cb func(tx *bchutil.Tx, finalizationTime time.Duration)) {
	am.notificationCallback = cb
}

func (am *AvalancheManager) PrivateKey() *bchec.PrivateKey {
	return am.privKey
}

// Start begins the core handler which processes peers and avalanche messages.
func (am *AvalancheManager) Start() {
	am.wg.Add(1)
	go am.handler()
}

// Stop gracefully shuts down the avalanche manager by stopping all asynchronous
// handlers and waiting for them to finish.
func (am *AvalancheManager) Stop() {
	close(am.quit)
	am.wg.Wait()
}

func (am *AvalancheManager) handler() {
	eventLoopTicker := time.NewTicker(AvalancheTimeStep)
out:
	for {
		select {
		case m := <-am.msgChan:
			switch msg := m.(type) {
			case *newPeerMsg:
				am.handleNewPeer(msg.peer)
			case *donePeerMsg:
				am.handleDonePeer(msg.peer)
			case *newTxsMsg:
				am.handleNewTx(msg.tx)
			case *blockConnectedMsg:
				am.handleBlockConnected(msg.blk)
			case *requestExpirationMsg:
				am.handleRequestExpiration(msg.key)
			case *queryMsg:
				am.handleQuery(msg.request, msg.respChan)
			case *registerVotesMsg:
				am.handleRegisterVotes(msg.p, msg.resp)
			case *connectedPeerMsg:
				am.handleConnectedPeer(msg.addr, msg.respChan)
			}
		case <-eventLoopTicker.C:
			am.eventLoop()
		case <-am.quit:
			break out
		}
	}
	eventLoopTicker.Stop()
	am.wg.Done()
}

func (am *AvalancheManager) Connected(addr net.Addr) bool {
	respChan := make(chan bool)
	am.msgChan <- &connectedPeerMsg{addr, respChan}
	connected := <-respChan
	return connected
}

func (am *AvalancheManager) handleConnectedPeer(addr net.Addr, respChan chan bool) {
	ip := strings.Split(addr.String(), ":")
	for peer := range am.peers {
		if peer.NA().IP.String() == ip[0] {
			respChan <- true
			close(respChan)
			return
		}
	}
	respChan <- false
	close(respChan)
}

// Query processes an avalanche request and returns the response.
func (am *AvalancheManager) Query(req *wire.MsgAvaRequest) *wire.MsgAvaResponse {
	respChan := make(chan *wire.MsgAvaResponse)
	am.msgChan <- &queryMsg{req, respChan}
	msg := <-respChan
	return msg
}

func (am *AvalancheManager) handleQuery(req *wire.MsgAvaRequest, respChan chan *wire.MsgAvaResponse) {
	votes := make([]byte, len(req.InvList))
	for i, inv := range req.InvList {
		txid := inv.Hash
		if _, exists := am.rejectedTxs[txid]; exists {
			votes[i] = 0x00 // No vote
			continue
		}
		record, ok := am.voteRecords[txid]
		if ok {
			// We're only going to vote for items we have a record for.
			vote := byte(0x00) // No vote
			if record.isAccepted() {
				vote = 0x01 // Yes vote
			}
			votes[i] = vote
		} else {
			// TODO: we need to download this transaction from the peer and give it to
			// the mempool for processing. If it is a double spend of a transaction
			// we are currently processing it needs to be set aside to be re-processed
			// after avalanche finishes on the first transaction. This is going to add
			// some complexity as we don't want to allow an infinite number of double
			// spends into memory as we do this.

			votes[i] = 0x80 // Neutral vote
		}
	}
	resp := wire.NewMsgAvaResponse(req.RequestID, votes, nil)
	sig, err := am.privKey.SignSchnorr(resp.SerializeForSignature())
	if err != nil {
		log.Error("Error signing response: %s", err.Error())
	}
	resp.Signature = sig
	respChan <- resp
}

// NewPeer adds a new peer to the manager
func (am *AvalancheManager) NewPeer(p *peer.Peer) {
	am.msgChan <- &newPeerMsg{p}
}

func (am *AvalancheManager) handleNewPeer(p *peer.Peer) {
	for peer := range am.peers {
		if peer.AvalanchePubkey().IsEqual(p.AvalanchePubkey()) {
			return
		}
	}
	log.Infof("New avalanche peer %s (%s)", p, p.UserAgent())
	am.peers[p] = struct{}{}
}

// DonePeer removes a peer from the manager
func (am *AvalancheManager) DonePeer(p *peer.Peer) {
	am.msgChan <- &donePeerMsg{p}
}

func (am *AvalancheManager) handleDonePeer(p *peer.Peer) {
	_, exists := am.peers[p]
	if !exists {
		log.Debugf("Received done avalanche peer message for unknown peer %s", p)
		return
	}

	// Remove the peer from the list of peers.
	delete(am.peers, p)

	log.Infof("Lost avalanche peer %s", p)
}

// NewTransactions passes new unconfirmed transactions into the manager to be processed.
func (am *AvalancheManager) NewTransaction(tx *TxDesc) {
	am.msgChan <- &newTxsMsg{tx}
}

func (am *AvalancheManager) handleNewTx(txd *TxDesc) {
	accepted := true
	if txd.Code != nil {
		switch *txd.Code {
		case wire.RejectDuplicate:
			// We can ignore duplicates as we should have already processed it.
			return
		case wire.RejectInvalid:
			// Invalid transactions are transactions which violate the consensus
			// rules and must be permanently considered invalid.
			am.rejectedTxs[*txd.Tx.Hash()] = struct{}{}
			return
		case wire.RejectDoubleSpend:
			fallthrough
		case wire.RejectInsufficientFee:
			fallthrough
		case wire.RejectDust:
			fallthrough
		case wire.RejectNonstandard:
			// In all of the above cases we don't want to actually vote for this
			// transaction.
			accepted = false
		}
	}

	// If a transaction reaches here it has either:
	// - been accepted into the mempool
	// - been rejected due to a policy violation
	// - been rejected due to being a double spend
	log.Debugf("Starting avalanche for tx %s", txd.Tx.Hash().String())

	txid := txd.Tx.Hash()

	// Iterate over the inputs and add each outpoint to our outpoint map
	for _, in := range txd.Tx.MsgTx().TxIn {
		doubleSpends, ok := am.outpoints[in.PreviousOutPoint]
		if ok {
			contains := false
			for _, ds := range doubleSpends {
				if txid.IsEqual(ds.Tx.Hash()) {
					contains = true
				}

				// If this double spend is in the accepted state then we need to set
				// the new transaction to accepted = false so we don't vote for it.
				dsTxid := ds.Tx.Hash()
				vr, ok := am.voteRecords[*dsTxid]
				if ok && vr.isAccepted() {
					accepted = false
				}
			}
			if !contains {
				doubleSpends = append(doubleSpends, txd)
				am.outpoints[in.PreviousOutPoint] = doubleSpends
			}
		} else {
			am.outpoints[in.PreviousOutPoint] = []*TxDesc{txd}
		}
	}

	// Add a new vote record
	_, ok := am.voteRecords[*txid]
	if !ok {
		am.voteRecords[*txid] = NewVoteRecord(txd, accepted)
	}
}

// BlockConnected fires whenever a new block is connected to the chain.
// When this happens we should go through the block and delete everything
// that has confirmed from memory.
func (am *AvalancheManager) BlockConnected(block *bchutil.Block) {
	am.msgChan <- &blockConnectedMsg{block}
}

func (am *AvalancheManager) handleBlockConnected(block *bchutil.Block) {
	for _, tx := range block.Transactions() {
		txid := tx.Hash()
		am.removeVoteRecords(tx)
		delete(am.rejectedTxs, *txid)
	}
}

func (am *AvalancheManager) eventLoop() {
	invs := am.getInvsForNextPoll()
	if len(invs) == 0 {
		return
	}

	p := am.getRandomPeerToQuery()
	if p == nil {
		return
	}
	requestID, err := wire.RandomUint64()
	if err != nil {
		log.Error(err)
		return
	}
	key := queryKey(requestID, p.ID())
	am.queries[key] = NewRequestRecord(time.Now().Unix(), invs)
	time.AfterFunc(AvalancheRequestTimeout, func() {
		am.msgChan <- &requestExpirationMsg{key}
	})

	req := wire.NewMsgAvaRequest(requestID)
	for _, inv := range invs {
		req.AddInvVect(&inv)
	}
	p.QueueMessage(req, nil)
}

func (am *AvalancheManager) handleRequestExpiration(key string) {
	r, ok := am.queries[key]
	if !ok {
		return
	}
	delete(am.queries, key)

	invs := r.GetInvs()
	for _, inv := range invs {
		vr, ok := am.voteRecords[inv.Hash]
		if ok {
			vr.inflightRequests--
		}
	}
}

func (am *AvalancheManager) getRandomPeerToQuery() *peer.Peer {
	i := 0
	if len(am.peers) > 0 {
		i = rand.Intn(len(am.peers))
	}
	for p := range am.peers {
		if i == 0 {
			return p
		}
		i--
	}
	return nil
}

func (am *AvalancheManager) getInvsForNextPoll() []wire.InvVect {
	var invs []wire.InvVect
	var toDelete []chainhash.Hash
	for txid, r := range am.voteRecords {
		// Delete very old inventory that hasn't finalized
		if time.Since(r.timestamp) > DeleteInventoryAfter {
			toDelete = append(toDelete, txid)
			continue
		}

		if r.hasFinalized() {
			// If this has finalized we can just skip.
			continue
		}
		if r.inflightRequests >= AvalancheMaxInflightPoll {
			// If we are already at the max inflight then continue
			continue
		}
		r.inflightRequests++

		// We don't have a decision, we need more votes.
		invs = append(invs, *wire.NewInvVect(wire.InvTypeTx, &txid))
	}

	if len(invs) >= AvalancheMaxElementPoll {
		invs = invs[:AvalancheMaxElementPoll]
	}

	for _, td := range toDelete {
		r := am.voteRecords[td]
		for _, in := range r.txdesc.Tx.MsgTx().TxIn {
			delete(am.outpoints, in.PreviousOutPoint)
		}
		delete(am.voteRecords, td)
	}

	return invs
}

// RegisterVotes processes responses to queries
func (am *AvalancheManager) RegisterVotes(p *peer.Peer, resp *wire.MsgAvaResponse) {
	if !resp.Signature.Verify(resp.SerializeForSignature(), p.AvalanchePubkey()) {
		log.Errorf("Invalid signature on avalanche response from peer %s", p)
		return
	}
	am.msgChan <- &registerVotesMsg{p, resp}
}

func (am *AvalancheManager) handleRegisterVotes(p *peer.Peer, resp *wire.MsgAvaResponse) {
	key := queryKey(resp.RequestID, p.ID())

	r, ok := am.queries[key]
	if !ok {
		log.Debugf("Received avalanche response from peer %s with an unknown request ID", p)
		return
	}

	// Always delete the key if it's present
	delete(am.queries, key)

	if r.IsExpired() {
		log.Debugf("Received avalanche response from peer %s with an expired request", p)
		return
	}

	invs := r.GetInvs()
	if len(resp.Votes) != len(invs) {
		log.Debugf("Received avalanche response from peer %s with incorrect number of votes", p)
		return
	}

	i := -1
	for _, inv := range invs {
		i++
		vr, ok := am.voteRecords[inv.Hash]
		if !ok {
			// We are not voting on this anymore
			continue
		}
		vr.inflightRequests--

		if vr.hasFinalized() {
			continue
		}

		if !vr.regsiterVote(resp.Votes[i]) {
			// This vote did not provide any extra information
			continue
		}

		// This transaction was either finalized or moved to accepted from
		// a previously unaccepted state. Let's look up all the double spends
		// of this transaction and reset their confidence back to zero.
		if vr.isAccepted() {
			for _, in := range vr.txdesc.Tx.MsgTx().TxIn {
				doublespends, ok := am.outpoints[in.PreviousOutPoint]
				if ok {
					for _, ds := range doublespends {
						dsid := ds.Tx.Hash()
						if !inv.Hash.IsEqual(dsid) {
							dsvr, ok := am.voteRecords[*dsid]
							if ok {
								dsvr.confidence = 0
							}
						}
					}
				}
			}
		}

		switch vr.status() {
		case StatusFinalized:
			if am.notificationCallback != nil {
				go am.notificationCallback(vr.txdesc.Tx, time.Since(vr.timestamp))
			}
			log.Infof("Avalanche finalized transaction %s in %s", inv.Hash.String(), time.Since(vr.timestamp))
			// TODO: the finalized transaction should be added to the mempool if it isn't already in there
			// TODO: double spends of the finalized transaction should be removed from the mempool.
		case StatusInvalid:
			log.Infof("Avalanche rejected transaction %s", inv.Hash.String())
			am.rejectedTxs[inv.Hash] = struct{}{}
			// TODO: remove tx and descendants from mempool
		}
	}
}

// removeVoteRecords recursively removes the vote record and any redeemers
func (am *AvalancheManager) removeVoteRecords(tx *bchutil.Tx) {
	txHash := tx.Hash()
	// Remove any transactions which rely on this one.
	for i := uint32(0); i < uint32(len(tx.MsgTx().TxOut)); i++ {
		prevOut := wire.OutPoint{Hash: *txHash, Index: i}
		if txRedeemers, exists := am.outpoints[prevOut]; exists {
			for _, redeemer := range txRedeemers {
				am.removeVoteRecords(redeemer.Tx)
			}
		}
	}

	// Remove the transaction if needed.
	if vr, exists := am.voteRecords[*txHash]; exists {
		// Mark the referenced outpoints as unspent by the pool.
		for _, txIn := range vr.txdesc.Tx.MsgTx().TxIn {
			dstxs, ok := am.outpoints[txIn.PreviousOutPoint]
			delete(am.outpoints, txIn.PreviousOutPoint)
			if ok {
				for _, ds := range dstxs {
					if !txHash.IsEqual(ds.Tx.Hash()) {
						am.removeVoteRecords(ds.Tx)
					}
				}
			}
		}
		delete(am.voteRecords, *txHash)
	}
}

func queryKey(requestID uint64, peerID int32) string {
	return fmt.Sprintf("%d|%d", requestID, peerID)
}
