// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"math/rand/v2"
	"sort"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

const (
	// DefaultMaxDSProofOrphans is the default number of orphan double spend
	// proofs kept.  Up to a quarter more are kept before the oldest are
	// removed.
	DefaultMaxDSProofOrphans = 65535

	// DefaultDSProofOrphanTTL is how long an orphan double spend proof is
	// kept before it expires.
	DefaultDSProofOrphanTTL = 90 * time.Second

	// maxRejectedDSProofs is the number of ids of rejected double spend
	// proofs remembered.
	maxRejectedDSProofs = 120000

	// maxDSProofOrphansPerOutPoint is the number of orphan proofs kept for
	// one outpoint, all of which are checked when a transaction spending
	// it arrives.
	maxDSProofOrphansPerOutPoint = 32

	// maxDSProofOrphansPerPeerOutPoint is the number of orphan proofs one
	// peer can have for one outpoint, so a single peer cannot fill the
	// outpoint's slots.
	maxDSProofOrphansPerPeerOutPoint = 2

	// One peer's orphans may take at most 1/dsProofOrphanPeerShare of the
	// orphan limit.
	dsProofOrphanPeerShare = 8
)

// dsProofEntry is a double spend proof known to the memory pool.  Orphans
// are proofs that could not be checked yet because the output they spend or
// a memory pool transaction spending it is missing.
type dsProofEntry struct {
	proof  *wire.MsgDSProof
	hash   chainhash.Hash
	orphan bool

	// peer is the peer to blame when an orphan turns out to be invalid or
	// expires.  It is only set when hasPeer is.
	peer    Tag
	hasPeer bool

	// orphaned is when the proof last became an orphan and seq orders
	// entries with the same time.
	orphaned time.Time
	seq      uint64
}

// dsProofStorage holds the double spend proofs known to the memory pool,
// indexed by id and by the outpoint they prove was double spent.
//
// It is not safe for concurrent access; the memory pool lock protects it.
type dsProofStorage struct {
	proofs     map[chainhash.Hash]*dsProofEntry
	byOutPoint map[wire.OutPoint]map[chainhash.Hash]*dsProofEntry
	numOrphans int
	maxOrphans int
	orphanTTL  time.Duration
	nextSeq    uint64

	// peerOrphans counts the orphans of each peer.
	peerOrphans map[Tag]int

	// The rejected ids are kept in two generations so the oldest half can
	// be dropped at once.
	rejected, rejectedOld map[chainhash.Hash]struct{}

	now func() time.Time
}

// newDSProofStorage returns an empty proof storage.
func newDSProofStorage() *dsProofStorage {
	return &dsProofStorage{
		proofs:      make(map[chainhash.Hash]*dsProofEntry),
		byOutPoint:  make(map[wire.OutPoint]map[chainhash.Hash]*dsProofEntry),
		maxOrphans:  DefaultMaxDSProofOrphans,
		peerOrphans: make(map[Tag]int),
		orphanTTL:   DefaultDSProofOrphanTTL,
		rejected:    make(map[chainhash.Hash]struct{}),
		now:         time.Now,
	}
}

// add stores a proof that is not an orphan.  An existing orphan with the same
// id stops being an orphan.  It returns whether the proof was new.
func (s *dsProofStorage) add(proof *wire.MsgDSProof, hash chainhash.Hash) bool {
	if e, ok := s.proofs[hash]; ok {
		if e.orphan {
			s.uncountPeer(e)
			e.orphan = false
			e.hasPeer = false
			s.numOrphans--
		}
		return false
	}
	e := &dsProofEntry{proof: proof, hash: hash, seq: s.nextSeq}
	s.nextSeq++
	s.proofs[hash] = e
	m := s.byOutPoint[proof.PrevOut]
	if m == nil {
		m = make(map[chainhash.Hash]*dsProofEntry)
		s.byOutPoint[proof.PrevOut] = m
	}
	m[hash] = e
	return true
}

// addOrphan stores a proof as an orphan received from peer, if hasPeer is
// set.  An existing proof becomes an orphan.  It returns false when a new
// proof is dropped because the peer already has the most orphans it may
// have for the outpoint or in total, or the outpoint is full of proofs
// without a peer.
func (s *dsProofStorage) addOrphan(proof *wire.MsgDSProof, hash chainhash.Hash, peer Tag, hasPeer bool) bool {
	if _, ok := s.proofs[hash]; !ok {
		if hasPeer && s.peerOrphans[peer] >= s.maxOrphans/dsProofOrphanPeerShare {
			return false
		}
		if !s.makeOrphanRoom(proof.PrevOut, peer, hasPeer) {
			return false
		}
	}
	s.add(proof, hash)
	e := s.proofs[hash]
	if !e.hasPeer && hasPeer {
		e.peer, e.hasPeer = peer, true
	}
	if e.orphaned.IsZero() {
		e.orphaned = s.now()
	}
	if !e.orphan {
		e.orphan = true
		s.numOrphans++
	}
	if e.hasPeer {
		s.peerOrphans[e.peer]++
	}
	s.limitOrphans(hash)
	return true
}

// uncountPeer removes an orphan from the count of its peer.
func (s *dsProofStorage) uncountPeer(e *dsProofEntry) {
	if !e.orphan || !e.hasPeer {
		return
	}
	if s.peerOrphans[e.peer]--; s.peerOrphans[e.peer] <= 0 {
		delete(s.peerOrphans, e.peer)
	}
}

// makeOrphanRoom checks whether a new orphan from peer fits the per outpoint
// limits.  When the outpoint is full, an orphan of a peer holding more than
// one is removed, the oldest of a peer picked at random among those holding
// the most.  If every peer holds one, the new orphan is refused so the first
// seen proofs are kept.  Proofs without a peer are never removed to make
// room.
func (s *dsProofStorage) makeOrphanRoom(op wire.OutPoint, peer Tag, hasPeer bool) bool {
	orphans := s.findOrphans(op)
	perPeer := make(map[Tag]int)
	most := 0
	for _, e := range orphans {
		if e.hasPeer {
			perPeer[e.peer]++
			most = max(most, perPeer[e.peer])
		}
	}
	if hasPeer && perPeer[peer] >= maxDSProofOrphansPerPeerOutPoint {
		return false
	}
	if len(orphans) < maxDSProofOrphansPerOutPoint {
		return true
	}
	if most <= 1 {
		return false
	}

	var candidates []Tag
	for p, n := range perPeer {
		if n == most {
			candidates = append(candidates, p)
		}
	}
	victimPeer := candidates[rand.IntN(len(candidates))]
	for _, e := range orphans {
		if e.hasPeer && e.peer == victimPeer {
			s.remove(e.hash)
			return true
		}
	}
	return false
}

// orphanExisting turns a proof that is not an orphan into one, so it expires
// unless a transaction claims it again.
func (s *dsProofStorage) orphanExisting(hash chainhash.Hash) {
	e, ok := s.proofs[hash]
	if !ok || e.orphan {
		return
	}
	e.orphan = true
	e.orphaned = s.now()
	s.numOrphans++
	s.limitOrphans(hash)
}

// remove deletes a proof and returns whether it was stored.
func (s *dsProofStorage) remove(hash chainhash.Hash) bool {
	e, ok := s.proofs[hash]
	if !ok {
		return false
	}
	if e.orphan {
		s.uncountPeer(e)
		s.numOrphans--
	}
	delete(s.proofs, hash)
	m := s.byOutPoint[e.proof.PrevOut]
	delete(m, hash)
	if len(m) == 0 {
		delete(s.byOutPoint, e.proof.PrevOut)
	}
	return true
}

// lookup returns the entry with the given id, or nil.
func (s *dsProofStorage) lookup(hash chainhash.Hash) *dsProofEntry {
	return s.proofs[hash]
}

// findOrphans returns the orphans for an outpoint, oldest first.
func (s *dsProofStorage) findOrphans(op wire.OutPoint) []*dsProofEntry {
	var orphans []*dsProofEntry
	for _, e := range s.byOutPoint[op] {
		if e.orphan {
			orphans = append(orphans, e)
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].seq < orphans[j].seq })
	return orphans
}

// sortedByOrphanTime returns the orphans, oldest first.
func (s *dsProofStorage) sortedByOrphanTime() []*dsProofEntry {
	orphans := make([]*dsProofEntry, 0, s.numOrphans)
	for _, e := range s.proofs {
		if e.orphan {
			orphans = append(orphans, e)
		}
	}
	sort.Slice(orphans, func(i, j int) bool {
		a, b := orphans[i], orphans[j]
		if !a.orphaned.Equal(b.orphaned) {
			return a.orphaned.Before(b.orphaned)
		}
		return a.seq < b.seq
	})
	return orphans
}

// limitOrphans removes the oldest orphans other than keep once there are a
// quarter more than the maximum, until the maximum is reached again.
func (s *dsProofStorage) limitOrphans(keep chainhash.Hash) {
	if s.numOrphans <= s.maxOrphans+s.maxOrphans/4 {
		return
	}
	removed := 0
	for _, e := range s.sortedByOrphanTime() {
		if s.numOrphans <= s.maxOrphans {
			break
		}
		if e.hash != keep {
			s.remove(e.hash)
			removed++
		}
	}
	log.Debugf("Removed %d orphan double spend proofs, %d left", removed,
		s.numOrphans)
}

// expireOrphans removes the orphans older than the orphan TTL and returns the
// peers that sent them.
func (s *dsProofStorage) expireOrphans() []Tag {
	expire := s.now().Add(-s.orphanTTL)
	var peers []Tag
	removed := 0
	for _, e := range s.sortedByOrphanTime() {
		if e.orphaned.After(expire) {
			break
		}
		if e.hasPeer {
			peers = append(peers, e.peer)
		}
		s.remove(e.hash)
		removed++
	}
	if removed > 0 {
		log.Debugf("Expired %d orphan double spend proofs, %d proofs left",
			removed, len(s.proofs))
	}
	return peers
}

// isRejected returns whether the proof id was recently rejected.
func (s *dsProofStorage) isRejected(hash chainhash.Hash) bool {
	_, ok := s.rejected[hash]
	if !ok {
		_, ok = s.rejectedOld[hash]
	}
	return ok
}

// markRejected remembers a rejected proof id.
func (s *dsProofStorage) markRejected(hash chainhash.Hash) {
	if len(s.rejected) >= maxRejectedDSProofs/2 {
		s.rejectedOld = s.rejected
		s.rejected = make(map[chainhash.Hash]struct{})
	}
	s.rejected[hash] = struct{}{}
}
