package avalanche

import (
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gcash/bchutil"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// Manager drives the Avalanche processes. The caller passes in peers and
// vertices and the manager returns finalization messages through callbacks.
type Manager struct {
	// identity is the staked identity of this peer.
	identity *SignedIdentity

	// map of peers and a mutex for writing to it
	peerWriteMu *sync.Mutex
	peers       peerMap

	// map of queries and a mutex for it
	queriesMu *sync.Mutex
	queries   queryMap

	// map of rejected vertex hashes and a mutex for it
	rejectedMu *sync.Mutex
	rejected   map[chainhash.Hash]struct{}

	// maps of vote records, blocks, and txs, and a mutex for them
	mapWriteMu     *sync.Mutex
	voteRecords    voteRecordMap
	conflictBlocks map[int32][]bchutil.Block // TODO: This should use cumulative work for collision points, not height.
	blocks         map[chainhash.Hash]bchutil.Block
	conflictTxs    map[wire.OutPoint][]bchutil.Tx
	txs            map[chainhash.Hash]wire.MsgTx

	// acceptedTip tracks last known accepted block.
	acceptedTip *bchutil.Block

	// receiver allows the caller access to Ava events.
	receiver Receiver

	// Concurrency control flow management.
	doneCh chan struct{}
	quitCh chan struct{}

	// rpcInfo contains the overview RPC request information.
	rpcInfo *pb.GetAvalancheInfoResponse
}

// New returns a new *Manager
func New(idKey bchec.PrivateKey, receivers ...Receiver) (*Manager, error) {
	ssi, err := NewSignedIdentity(NewIdentity(idKey, nil), nil)
	if err != nil {
		return nil, err
	}

	return &Manager{
		identity: ssi,

		doneCh: make(chan struct{}),
		quitCh: make(chan struct{}),

		receiver: compositeReceiver(receivers),
		rpcInfo:  &pb.GetAvalancheInfoResponse{},

		peerWriteMu: &sync.Mutex{},
		peers:       make(peerMap, 16),

		queriesMu: &sync.Mutex{},
		queries:   make(queryMap, 4096),

		rejectedMu: &sync.Mutex{},
		rejected:   make(map[chainhash.Hash]struct{}),

		mapWriteMu:     &sync.Mutex{},
		voteRecords:    make(voteRecordMap, 1024),
		conflictBlocks: make(map[int32][]bchutil.Block, 4),
		blocks:         make(map[chainhash.Hash]bchutil.Block, 16),
		conflictTxs:    make(map[wire.OutPoint][]bchutil.Tx, 64),
		txs:            make(map[chainhash.Hash]wire.MsgTx, 1024),
	}, nil
}

// Start tells the Manger to begin the Avalanche engine and start processing any
// items we know about.
func (m *Manager) Start() {
	ticker := time.NewTicker(queryLoopTimeStep)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.tick()
			case <-m.quitCh:
				m.doneCh <- struct{}{}
				break
			}
		}
	}()
}

// Stop halts the Avalanche process and waits for it to wrap up.
func (m *Manager) Stop() {
	m.quitCh <- struct{}{}
	<-m.doneCh
}

//
// Getters
//

// Identity returns the identity for the local Avalanche peer
func (m Manager) Identity() *SignedIdentity { return m.identity }

// IsAddrConnected returns whether this peer is in the pool or not.
func (m Manager) IsAddrConnected(addr *wire.NetAddress) bool {
	for otherPeer := range m.peers {
		if addr.IP.Equal(otherPeer.NA().IP) && addr.Port == otherPeer.NA().Port {
			return true
		}
	}
	return false
}

//
// Feeder routines. These are used to add peers and vertices into the engine.
//

// NewPeer notifies the manager of a new peer to add to the pool.
func (m *Manager) NewPeer(p peerer, ssi *SignedIdentity) {
	log.Debugf("Adding new avalanche peer %s", p)
	// TODO: ssi.Validate()
	// for otherPeer := range m.peers {
	// 	if otherPeer.AvalanchePubkey().IsEqual(p.AvalanchePubkey()) {
	// 		log.Debugf("Avalanche peer already known %s", hex.EncodeToString(p.AvalanchePubkey().SerializeCompressed()))
	// 		return
	// 	}
	// }

	m.peerWriteMu.Lock()
	m.peers[p] = ssi
	m.peerWriteMu.Unlock()

	atomic.AddInt64(&m.rpcInfo.SeenPeerCount, 1)

	// Send connection event
	// m.receiver.PeerConnect(*ssi)
}

// DonePeer notifies the manager of a peer to remove from the pool.
func (m *Manager) DonePeer(p peerer) {
	_, exists := m.peers[p]
	if !exists {
		log.Debugf("Received done avalanche peer message for unknown peer %s", p)
		return
	}

	// Remove the peer from the list of peers.
	m.peerWriteMu.Lock()
	delete(m.peers, p)
	m.peerWriteMu.Unlock()

	log.Debugf("Lost avalanche peer %s", p)

	// Send disconnection event
	// m.receiver.PeerDisconnect(*ssi)
}

// NewBlock adds new blocks to the processor.
func (m *Manager) NewBlock(block bchutil.Block, code wire.RejectCode) {
	// If we already have a VoteRecord for this block then we're done
	if _, ok := m.voteRecords[*block.Hash()]; ok {
		log.Debugf("Not starting avalanche for block %s", block.Hash().String())
		return
	}

	// Invalid blocks are blocks which violate the consensus rules and must be
	// permanently considered invalid.
	if code == wire.RejectInvalid {
		log.Debugf("avalanche rejecting block %s", block.Hash().String())
		m.rejectedMu.Lock()
		m.rejected[*block.Hash()] = struct{}{}
		m.rejectedMu.Unlock()
		return
	}

	// If a block reaches here we don't have a record for it and it has either:
	// - been accepted into the mempool
	// - been rejected due to a policy violation
	// TODO: Check all txs for FINALIZED accepted conflicts which causes a hard reject

	// Iterate over the inputs and add each outpoint to our conflict map. Also
	// check for any accepted conflicts.
	contains := false
	accepted := code == 0
	for _, conflictingBlk := range m.conflictBlocks[block.Height()] {
		vr := m.voteRecords[*block.Hash()]
		contains = contains || block.Hash().IsEqual(conflictingBlk.Hash())
		accepted = accepted && !vr.isAccepted()

		// If there's a known accepted, finalized, conflicting block then this one
		// must be hard rejected
		if vr.isFinalized() && vr.isAccepted() {
			m.rejectedMu.Lock()
			m.rejected[*block.Hash()] = struct{}{}
			m.rejectedMu.Unlock()
			return
		}
	}

	vr := newVoteRecord(typeBlock, accepted)

	// Get lock and then start writes to maps
	m.mapWriteMu.Lock()
	m.voteRecords[*block.Hash()] = vr
	m.blocks[*block.Hash()] = block
	if !contains {
		m.conflictBlocks[block.Height()] = append(m.conflictBlocks[block.Height()], block)
	}
	m.mapWriteMu.Unlock()

	m.receiver.NewVoteRecord(*block.Hash(), *vr)
	atomic.AddInt32(&m.rpcInfo.PendingBlockCount, 1)
	log.Debugf("Starting avalanche for block %s", block.Hash().String())
}

// NewTransactions adds new blocks to the processor.
func (m *Manager) NewTransaction(tx bchutil.Tx, code wire.RejectCode) {

	txID := *tx.Hash()
	// If we already have a record for this item just skip
	// We can ignore duplicates as we should have already processed it.
	if _, ok := m.voteRecords[*tx.Hash()]; ok {
		log.Debugf("Not starting avalanche for duplicate tx %s", tx.Hash().String())
		return
	}

	// Invalid transactions are transactions which violate the consensus  rules
	// and must be permanently considered invalid.
	if code == wire.RejectInvalid {
		log.Debugf("avalanche rejecting tx %s", tx.Hash().String())
		m.rejectedMu.Lock()
		m.rejected[*tx.Hash()] = struct{}{}
		m.rejectedMu.Unlock()
		return
	}

	// If a transaction reaches here it has either:
	// - been accepted into the mempool
	// - been rejected due to a policy violation
	// - been rejected due to being a double spend
	accepted := code != 0

	// Iterate over the inputs, add each outpoint to conflict map, and see if any
	// accepted conflicts exist.
	outpointsToAddTo := make([]wire.OutPoint, 0, len(tx.MsgTx().TxIn))
	for _, in := range tx.MsgTx().TxIn {
		contains := false
		for _, ds := range m.conflictTxs[in.PreviousOutPoint] {
			vr := m.voteRecords[*ds.Hash()]
			contains = contains || txID.IsEqual(ds.Hash())
			accepted = accepted && !vr.isAccepted()

			// If there's a known accepted, finalized, conflicting tx then this one
			// must be hard rejected
			if vr.isFinalized() && vr.isAccepted() {
				m.rejectedMu.Lock()
				m.rejected[*tx.Hash()] = struct{}{}
				m.rejectedMu.Unlock()
				return
			}
		}

		// If this tx wasn't in the map we add it now
		if !contains {
			outpointsToAddTo = append(outpointsToAddTo, in.PreviousOutPoint)
		}
	}

	// We don't have a VoteRecord for this vertex so we'll create one
	vr := newVoteRecord(typeTx, accepted)

	// Get lock and then start writes to maps
	m.mapWriteMu.Lock()
	m.voteRecords[txID] = vr
	m.txs[txID] = *tx.MsgTx()
	for _, op := range outpointsToAddTo {
		m.conflictTxs[op] = append(m.conflictTxs[op], tx)
	}
	m.mapWriteMu.Unlock()

	// Send new voterecord event
	m.receiver.NewVoteRecord(txID, *vr)
	atomic.AddInt64(&m.rpcInfo.PendingTransactionCount, 1)
	log.Debugf("Starting avalanche for tx %s (%d peers)", tx.Hash().String(), len(m.peers))
}

//
// Query request/response engine. These routines drive the Avalanche process
//

// ProcessQuery processes a query and sends a response back for valid queries.
func (m *Manager) ProcessQuery(p peerer, req *wire.MsgAvaQuery) error {
	log.Debug("ProcessQuery for %d invs (id %d)", len(req.InvList), req.QueryID)

	// Process each queried inv and set a vote for it. Also collect a list of
	// requested items we don't have so we can ask for them.
	missingItemsInvMsg := wire.NewMsgInv()
	votes := make([]byte, len(req.InvList))
	for i, inv := range req.InvList {
		// If we have a record for this item return our current vote
		if vr := m.voteRecords[inv.Hash]; vr != nil {
			votes[i] = vr.vote()
			continue
		}

		// Hard rejects are always a no vote. If we have a VoteRecord for this item
		// it should be returning a no vote. If we don't but it's in our hard reject
		// list we vote no too. Checking this after checking VoteRecords means an
		// extra map lookup in most cases for rejected items but saves an
		// unnecessary map lookup for the more common case of a non-rejected item.
		if _, vertexIsRejected := m.rejected[inv.Hash]; vertexIsRejected {
			votes[i] = voteNo
			continue
		}

		// Record not found. Download this vertex from the peer and give it to
		// the mempool for processing. Abstain from the vote.
		//
		// TODO: If it is a double spend of a transaction we are currently
		//  processing it needs to be set aside to be re-processed after avalanche
		//  finishes on the first transaction. This is going to add some complexity
		//  as we don't want to allow an infinite number of double pends into memory
		//  as we do this.
		//
		votes[i] = voteAbstain
		missingItemsInvMsg.AddInvVect(inv)
	}

	// Send the inv msg for the missing items
	p.QueueMessage(missingItemsInvMsg, nil)

	// Create response from votes, sign it, and send it back
	resp := wire.NewMsgAvaResponse(req.QueryID, votes, nil)
	sig, err := m.identity.Sign(resp.SerializeForSignature())
	if err != nil {
		log.Debug("Failed to process AvaQuery. Signature failed.")
		return err
	}
	resp.Signature = sig

	// Now send it to the peer
	log.Debug("ProcessQuery: Sending response")
	p.QueueMessage(resp, nil)

	return nil
}

// ProcessQueryResponse processes the response to a query we made.
func (m *Manager) ProcessQueryResponse(p peerer, resp *wire.MsgAvaResponse) {
	log.Debug("ProcessQueryResponse from", p.NA().IP.String(), hex.EncodeToString(p.AvalanchePubkey().SerializeCompressed()))
	if !resp.Signature.Verify(resp.SerializeForSignature(), p.AvalanchePubkey()) {
		return
	}

	key := queryKey(p.ID(), resp.QueryID)
	m.queriesMu.Lock()
	r, ok := m.queries[key]
	if !ok {
		m.queriesMu.Unlock()
		return
	}
	delete(m.queries, key)
	m.queriesMu.Unlock()

	// Ignore responses to expired queries or that don't have the correct number of votes
	if len(resp.Votes) != len(r.invs) ||
		time.Unix(r.timestamp+globalTimeOffset, 0).Add(maxQueryAge).Before(clock.now()) {
		return
	}

	// Process each vote
	i := -1
	for _, inv := range r.invs {
		i++
		vr, ok := m.voteRecords[inv.Hash]

		// Ignore votes for records we're not voting on anymore, either because
		// they expired or were finalized
		if !ok {
			continue
		}
		if vr.isFinalized() {
			continue
		}

		// Register the vote
		vr.decInflight()
		stateChanged := vr.registerVote(resp.Votes[i])
		log.Debug("Registered avalanche vote", resp.Votes[i], vr.getConfidence())
		// This vote did not cause a state change so we can just continue to the
		// next vote
		if !stateChanged {
			continue
		}

		// We can't change from a finalized state to another state, so if the state
		// changed and we're finalized then we were just now finalized. We want to
		// run some post-finalization routines now.
		if vr.isFinalized() {
			m.finalizeVoteRecord(inv.Hash, *vr)
			m.receiver.FinalizedVoteRecord(inv.Hash, *vr)
		} else if !vr.isAccepted() {
			// We don't need to do anything if a vertex has changed to rejected but is
			// not yet finalized
			continue
		}

		// We either finalized or moved to accepted from a previously rejected
		// state. Let's look up all its conflicts and set their confidence to 0.
		for _, hash := range m.incomingEdgeHashes(vr.getType(), inv.Hash) {
			m.voteRecords[hash].resetConfidence()
		}
	}
}

// tick does the all the work for one cycle of the query engine.
func (m *Manager) tick() {
	invs := m.getInvsForTick()
	if len(invs) == 0 {
		return
	}

	log.Debugf("tick: have %d invs", len(invs))
	p := getRandomPeer(m.peers)
	if p == nil {
		return
	}

	log.Debug("sending to peer", p.NA().IP.String())

	queryID, err := wire.RandomUint64()
	if err != nil {
		log.Error(err)
		return
	}

	log.Debug("Sending avalanche query")

	// We have everything we need to create an Ava query.
	key := queryKey(p.ID(), queryID)
	m.queriesMu.Lock()
	m.queries[key] = query{clock.now().Unix() - globalTimeOffset, invs}
	m.queriesMu.Unlock()

	// Setup a timeout to remove the query after the expiration time.
	time.AfterFunc(maxQueryAge, func() { purgeQuery(m.queries, m.voteRecords, key) })

	// Create an AvaQuery
	// TODO: Don't preemptively send these in the future; only send when queried
	// for an unknown item.
	invMsg := wire.NewMsgInv()
	req := wire.NewMsgAvaQuery(queryID)

	for _, inv := range invs {
		err = req.AddInvVect(inv)
		if err != nil {
			log.Error(err)
			return
		}

		err = invMsg.AddInvVect(inv)
		if err != nil {
			log.Error(err)
			return
		}
	}

	// Send the messages
	p.QueueMessage(invMsg, nil)
	p.QueueMessage(req, nil)
}

// getInvsForTick returns a list of the items we need to query for this engine
// cycle
func (m *Manager) getInvsForTick() []*wire.InvVect {
	// log.Debugf("total voterecord count: %d", len(m.voteRecords))
	maxSize := len(m.voteRecords)
	if maxSize > maxQueriesPerRequest {
		maxSize = maxQueriesPerRequest
	}

	invs := make([]*wire.InvVect, 0, maxSize)
	i := 0
	for hash, vr := range m.voteRecords {
		// Delete expired or finalized vertices and skip to next iteration
		// if vr.getAge() > maxVoteRecordAge {
		// 	log.Debug("vote record too old", vr.getAge(), maxVoteRecordAge, globalTimeOffset, vr.other)
		// 	continue
		// }

		// Add this voterecord to the inv list unless we're at our inflight limit.
		if !vr.incInflight() {
			// log.Debug("incinflight returned false")
			continue
		}

		h := hash
		invs = append(invs, wire.NewInvVect(wire.InvType(vr.getType()+1), &h))

		// Stop if we're at our request limit
		i++
		if i >= maxSize {
			break
		}
	}

	return invs
}

//
// Finalization procedures
//

func (m *Manager) finalizeVoteRecord(hash chainhash.Hash, vr VoteRecord) {
	log.Debug("Finalizing avalanche vertex", hash.String())
	defer delete(m.voteRecords, hash)

	if vr.isAccepted() {
		// TODO: the finalized transaction should be added to the mempool if it isn't already in there
		// TODO: double spends of the finalized transaction should be removed from the mempool.
		//
		// for txs:
		//   sm.txMemPool.ProcessTransaction(tx, true, rateLimit, tag)
		//   sm.txMemPool.RemoveDoubleSpends(tx)
		//   foreach dsTX -> sm.txMemPool.RemoveTransaction(dsTX, false)
		//   foreach dsTX -> m.rejected[dsTx.Hash()] = struct{}{}
		//
		//
		// for blocks:
		// 	 foreach dsBlock -> s.chain.InvalidateBlock(dsBlock)
		//   foreach dsBlock -> m.rejected[dsBlock.Hash()] = struct{}{}
		//   s.chain.ReconsiderBlock(block)
	} else {
		// TODO: remove tx and descendants from mempool
		m.rejectedMu.Lock()
		m.rejected[hash] = struct{}{}
		m.rejectedMu.Unlock()
	}

	if vr.getType() == typeTx {
		m.finalizeTx(hash, vr)
	} else {
		m.finalizeBlock(hash, vr)
	}
}

func (m *Manager) finalizeTx(hash chainhash.Hash, vr VoteRecord) {
	// Delete this transaction when we're done.
	defer delete(m.txs, hash)

	// Get transaction. If it isn't found or has been rejected then we're done.
	tx, ok := m.txs[hash]
	if !ok || !vr.isAccepted() {
		return
	}

	// Remove ancestor transactions and conflict transactions.
	for _, txIn := range tx.TxIn {
		for _, redeemer := range m.conflictTxs[txIn.PreviousOutPoint] {
			delete(m.txs, *redeemer.Hash())
			delete(m.voteRecords, *redeemer.Hash())
		}
		delete(m.conflictTxs, txIn.PreviousOutPoint)
	}
}

func (m *Manager) finalizeBlock(hash chainhash.Hash, vr VoteRecord) {
	// Delete this block when we're done.
	defer delete(m.blocks, hash)

	// Get block. If it isn't found or has been rejected then we're done.
	block, ok := m.blocks[hash]
	if !ok || !vr.isAccepted() {
		return
	}

	if m.acceptedTip == nil || block.Height() > m.acceptedTip.Height() {
		m.acceptedTip = &block
	}

	// Delete any ancestor transactions
	for _, tx := range block.MsgBlock().Transactions {
		delete(m.txs, tx.TxHash())
		delete(m.voteRecords, tx.TxHash())
	}

	// Delete parent block
	delete(m.blocks, block.MsgBlock().Header.PrevBlock)
	delete(m.voteRecords, block.MsgBlock().Header.PrevBlock)

	// Delete conflicting blocks
	for _, block := range m.conflictBlocks[block.Height()] {
		delete(m.blocks, *block.Hash())
		delete(m.voteRecords, *block.Hash())
	}
	delete(m.conflictBlocks, block.Height())
}

func (m *Manager) incomingEdgeHashes(t vertexType, hash chainhash.Hash) []chainhash.Hash {
	if t == typeBlock {
		block := m.blocks[hash]
		hashes := make([]chainhash.Hash, 1+len(block.Transactions()))
		hashes[0] = block.MsgBlock().Header.PrevBlock

		for i, tx := range block.Transactions() {
			hashes[i+1] = *tx.Hash()
		}
		return hashes
	}

	inputs := m.txs[hash].TxIn
	hashes := make([]chainhash.Hash, len(inputs))
	for i, input := range inputs {
		hashes[i] = input.PreviousOutPoint.Hash
	}
	return hashes
}

//
// Protobuf presenters
//

func (m Manager) GetInfoPB() pb.GetAvalancheInfoResponse {
	m.rpcInfo.CurrentPeerCount = int32(len(m.peers))
	if m.acceptedTip != nil {
		m.rpcInfo.AcceptedTipHash = m.acceptedTip.Hash().CloneBytes()
	}
	return *m.rpcInfo
}

func GetPeerNotificationPB(ssi SignedIdentity, isConnected bool) pb.AvalanchePeerNotification {
	ntfn := pb.AvalanchePeerNotification{
		IsConnected:        isConnected,
		PublicKey:          ssi.PubKey.SerializeCompressed(),
		Version:            int32(ssi.Version),
		Sequence:           ssi.Sequence,
		OutPoints:          make([]*pb.Transaction_Input_Outpoint, len(ssi.OutPoints)),
		IdentitySignature:  ssi.IdentitySignature.Serialize(),
		OutPointSignatures: make([][]byte, len(ssi.OutPointSignatures)),
	}

	for i, outPoint := range ssi.OutPoints {
		ntfn.OutPoints[i] = &pb.Transaction_Input_Outpoint{
			Index: outPoint.Index,
			Hash:  outPoint.Hash.CloneBytes(),
		}
		ntfn.OutPointSignatures[i] = ssi.OutPointSignatures[i].Serialize()
	}

	return ntfn
}

func GetFinalizationNotificationPB(h chainhash.Hash, vr VoteRecord) pb.AvalancheFinalizationNotification {
	return pb.AvalancheFinalizationNotification{
		Vertex: &pb.AvalancheVertex{
			Hash: h.CloneBytes(),
			Type: pb.AvalancheType(vr.getType()),
		},
		StartedAt:  vr.getStartTime(),
		FinalState: pb.AvalancheState(vr.state() & 1),
		StartState: pb.AvalancheState(vr.getStartState() & 1),
	}
}

//
// Utilities
//

func queryKey(peerID int32, queryID uint64) string {
	return fmt.Sprintf("%d|%d", peerID, queryID)
}

func getRandomPeer(peers peerMap) (p peerer) {
	i := len(peers)
	if i == 0 {
		return nil
	}

	j := 0
	t := randomGen.Intn(i)
	for p = range peers {
		if j >= t {
			break
		}
		j++
	}
	return p
}

func purgeQuery(queries queryMap, vrs voteRecordMap, key string) {
	for _, inv := range queries[key].invs {
		vrs[inv.Hash].decInflight()
	}
	delete(queries, key)
}
