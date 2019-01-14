package avalanche

import (
	"fmt"
	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"math/rand"
	"sync"
	"time"
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

	DeleteInventoryAfter = time.Minute * 30
)

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
	txs []*mempool.TxDesc
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
	p *peer.Peer
	resp *wire.MsgAvaResponse
}

type AvalancheManager struct {
	peers   map[*peer.Peer]struct{}
	wg      sync.WaitGroup
	quit    chan struct{}
	msgChan chan interface{}

	voteRecords map[chainhash.Hash]*VoteRecord
	outpoints map[wire.OutPoint][]*mempool.TxDesc
	round       int64
	queries     map[string]RequestRecord

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
		outpoints:   make(map[wire.OutPoint][]*mempool.TxDesc),
		queries:     make(map[string]RequestRecord),
		privKey:     avalanchePrivkey,
	}, nil
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
				am.handleNewTxs(msg.txs)
			case *requestExpirationMsg:
				am.handleRequestExpiration(msg.key)
			case *queryMsg:
				am.handleQuery(msg.request, msg.respChan)
			case *registerVotesMsg:
				am.handleRegisterVotes(msg.p, msg.resp)
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

// Query processes an avalanche request and returns the response.
func (am *AvalancheManager) Query(req *wire.MsgAvaRequest) *wire.MsgAvaResponse {
	respChan := make(chan *wire.MsgAvaResponse)
	am.msgChan <- &queryMsg{req, respChan}
	msg := <-respChan
	return msg
}

func (am *AvalancheManager) handleQuery(req *wire.MsgAvaRequest, respChan chan *wire.MsgAvaResponse) {
	resp := wire.NewMsgAvaResponse(req.RequestID, nil)
	for i := 0; i < len(req.InvList); i++ {
		txid := req.InvList[i].Hash
		record, ok := am.voteRecords[txid]
		if ok {
			// We're only going to vote for items we have a record for.
			vote := false
			if record.isAccepted() {
				vote = true
			}
			vr := wire.NewVoteRecord(vote, &txid)
			resp.AddVoteRecord(vr)
		} else {
			// TODO: we need to download this transaction from the peer and give it to
			// the mempool for processing. If it is a double spend of a transaction
			// we are currently processing it needs to be set aside to be re-processed
			// after avalanche finishes on the first transaction. This is going to add
			// some complexity as we don't want to allow an infinite number of double
			// spends into memory as we do this.
		}
	}
	sig, err := am.privKey.Sign(resp.SerializeForSignature())
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
		log.Warnf("Received done avalanche peer message for unknown peer %s", p)
		return
	}

	// Remove the peer from the list of peers.
	delete(am.peers, p)

	log.Infof("Lost avalanche peer %s", p)
}

// NewTransactions passes new unconfirmed transactions into the manager to be processed.
func (am *AvalancheManager) NewTransactions(txs []*mempool.TxDesc) {
	for _, tx := range txs {
		log.Infof("Starting avalanche for tx %s", tx.Tx.Hash().String())
	}
	am.msgChan <- &newTxsMsg{txs}
}

func (am *AvalancheManager) handleNewTxs(txs []*mempool.TxDesc) {
	for _, txdesc := range txs {
		txid := txdesc.Tx.Hash()
		// Add a new vote record
		_, ok := am.voteRecords[*txid]
		if !ok {
			//TODO: make sure we don't have any double spends currently in the accepted
			// state. If so then accepted should be set to false here.
			am.voteRecords[*txid] = NewVoteRecord(txdesc, true)
		}

		// Iterate over the inputs and add each outpoint to our outpoint map
		for _, in := range txdesc.Tx.MsgTx().TxIn {
			doubleSpends, ok := am.outpoints[in.PreviousOutPoint]
			if ok {
				contains := false
				for _, ds := range doubleSpends {
					if txid.IsEqual(ds.Tx.Hash()) {
						contains = true
						break
					}
				}
				if !contains {
					doubleSpends = append(doubleSpends, txdesc)
					am.outpoints[in.PreviousOutPoint] = doubleSpends
				}
			} else {
				am.outpoints[in.PreviousOutPoint] = []*mempool.TxDesc{txdesc}
			}
		}
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
	delete(am.queries, key)
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
	invs := make([]wire.InvVect, 0, len(am.voteRecords))
	for txid, r := range am.voteRecords {
		if r.hasFinalized() {
			// If this has finalized we can just skip.
			continue
		}

		// We don't have a decision, we need more votes.
		invs = append(invs, *wire.NewInvVect(wire.InvTypeTx, &txid))
	}

	if len(invs) >= AvalancheMaxElementPoll {
		invs = invs[:AvalancheMaxElementPoll]
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
		return
	}

	invs := r.GetInvs()
	votes := resp.VoteList

	for _, v := range votes {
		_, ok := invs[v.Hash]
		if !ok {
			log.Debugf("Received avalanche response from peer %s with an unrequested vote", p)
			return
		}
		vr, ok := am.voteRecords[v.Hash]
		if !ok {
			// We are not voting on this anymore
			continue
		}

		if !vr.regsiterVote(v.Vote) {
			// This vote did not provide any extra information
			continue
		}

		// This transaction was either finalized or moved to accepted from
		// a previously unaccepted state. Let's look up all the double spends
		// of this transaction and reset their confidence back to zero.
		for _, in := range vr.txdesc.Tx.MsgTx().TxIn {
			doublespends, ok := am.outpoints[in.PreviousOutPoint]
			if ok {
				for _, ds := range doublespends {
					dsid := ds.Tx.Hash()
					if !v.Hash.IsEqual(dsid) {
						dsvr, ok := am.voteRecords[*dsid]
						if ok {
							dsvr.confidence = 0
							am.voteRecords[*dsid] = dsvr
						}
					}
				}
			}
		}

		// When we finalize we want to remove our vote record, vote records of double spends and
		// outpoints.
		if vr.hasFinalized() {
			time.AfterFunc(DeleteInventoryAfter, func() {
				am.removeVoteRecords(vr.txdesc)
			})
			// TODO: the finalized transaction should be added to the mempool if it isn't already in there
			// TODO: double spends of the finalized transaction should be removed from the mempool.
		}

		switch vr.status() {
		case StatusFinalized:
			log.Infof("Avalanche finalized transaction %s in %s", v.Hash.String(), time.Since(time.Unix(r.timestamp, 0)))
		case StatusRejected:
			// TODO: remove tx and descendants from mempool and mark as a bad transaction
		}
	}
}

// removeVoteRecords recursively removes the vote record and any redeemers
func (am *AvalancheManager) removeVoteRecords(txdesc *mempool.TxDesc) {
	txHash := txdesc.Tx.Hash()
	// Remove any transactions which rely on this one.
	for i := uint32(0); i < uint32(len(txdesc.Tx.MsgTx().TxOut)); i++ {
		prevOut := wire.OutPoint{Hash: *txHash, Index: i}
		if txRedeemers, exists := am.outpoints[prevOut]; exists {
			for _, redeemer := range txRedeemers {
				am.removeVoteRecords(redeemer)
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
						am.removeVoteRecords(ds)
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
