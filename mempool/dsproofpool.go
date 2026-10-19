// Copyright (c) 2020-2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

const (
	// dsProofSearchMaxDepth and dsProofSearchMaxTxs limit the search of the
	// unconfirmed ancestors of a transaction for double spend proofs.
	dsProofSearchMaxDepth = 1000
	dsProofSearchMaxTxs   = 20000
)

// dsProofRelockHook is called by ProcessDSProof between releasing the read
// lock and taking the write lock, when set by tests.  It must only be set
// from tests that do not run in parallel.
var dsProofRelockHook func()

// ErrDSProofSearchLimit is returned when the search of a transaction's
// unconfirmed ancestors stops at its depth or size limit.
var ErrDSProofSearchLimit = errors.New("double spend proof search limit reached")

// DSProofDesc describes a double spend proof known to the memory pool.
type DSProofDesc struct {
	Proof *wire.MsgDSProof
	Hash  chainhash.Hash

	// TxHash is the memory pool transaction spending the proof's outpoint,
	// or nil for an orphan.
	TxHash *chainhash.Hash
}

// dsProofAnnouncement is a proof newly associated with a memory pool
// transaction, waiting to be passed to Config.OnDSProof.
type dsProofAnnouncement struct {
	proof *wire.MsgDSProof
	tx    *bchutil.Tx
}

// unlockAndNotify releases the write lock and then passes the proofs and the
// peers to blame queued while it was held to the configured callbacks.
func (mp *TxPool) unlockAndNotify() {
	announcements, bad := mp.dsProofAnnouncements, mp.badDSProofPeers
	mp.dsProofAnnouncements, mp.badDSProofPeers = nil, nil
	mp.mtx.Unlock()

	if len(bad) > 0 && mp.cfg.OnBadDSProofs != nil {
		mp.cfg.OnBadDSProofs(bad)
	}
	if mp.cfg.OnDSProof != nil {
		for _, a := range announcements {
			mp.cfg.OnDSProof(a.proof, a.tx)
		}
	}
}

// currentScriptFlags returns the script flags for the next block.
func (mp *TxPool) currentScriptFlags() txscript.ScriptFlags {
	return mp.standardScriptFlags(mp.cfg.BestHeight()+1, mp.cfg.MedianTimePast())
}

// dsProofCoin returns the unspent output op in the utxo set or the memory
// pool, regardless of whether a memory pool transaction spends it.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) dsProofCoin(op wire.OutPoint) *wire.TxOut {
	tx := bchutil.NewTx(&wire.MsgTx{TxIn: []*wire.TxIn{{PreviousOutPoint: op}}})
	view, err := mp.fetchInputUtxos(tx)
	if err != nil {
		return nil
	}
	entry := view.LookupEntry(op)
	if entry == nil || entry.IsSpent() {
		return nil
	}
	return &wire.TxOut{
		Value:     entry.Amount(),
		PkScript:  entry.PkScript(),
		TokenData: entry.TokenData(),
	}
}

// validateDSProofInPool checks a proof against the memory pool.  spendingTx
// is the memory pool transaction spending the proof's outpoint, or nil to
// look it up.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) validateDSProofInPool(proof *wire.MsgDSProof, hash *chainhash.Hash,
	spendingTx *bchutil.Tx, flags txscript.ScriptFlags) (dsProofValidity, error) {

	if err := checkDSProofSanity(proof, hash, flags); err != nil {
		return dsProofInvalid, err
	}
	coin := mp.dsProofCoin(proof.PrevOut)
	if spendingTx == nil {
		spendingTx = mp.outpoints[proof.PrevOut]
	}
	var msgTx *wire.MsgTx
	if spendingTx != nil {
		msgTx = spendingTx.MsgTx()
	}
	return validateDSProof(proof, hash, coin, msgTx, flags)
}

// addDSProofForTx associates a valid proof with tx, the memory pool
// transaction spending its outpoint, or when tx is nil, whichever
// transaction does.  It returns the transaction, or nil when there is none
// or it already has a proof.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) addDSProofForTx(proof *wire.MsgDSProof, hash chainhash.Hash, tx *bchutil.Tx) *bchutil.Tx {
	if tx == nil {
		tx = mp.outpoints[proof.PrevOut]
		if tx == nil {
			return nil
		}
	}
	if _, ok := mp.txDSProofs[*tx.Hash()]; ok {
		return nil
	}
	mp.dsProofs.add(proof, hash)
	mp.txDSProofs[*tx.Hash()] = hash
	mp.dsProofAnnouncements = append(mp.dsProofAnnouncements,
		dsProofAnnouncement{proof: proof, tx: tx})
	return tx
}

// hasDSProofForOutPoint returns whether the memory pool transaction spending
// op has a proof.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) hasDSProofForOutPoint(op wire.OutPoint) bool {
	tx, ok := mp.outpoints[op]
	if !ok {
		return false
	}
	_, ok = mp.txDSProofs[*tx.Hash()]
	return ok
}

// maybeCreateDSProof creates a proof when tx spends an output already spent
// by a memory pool transaction that has no proof yet.  Only the first input
// spending such an output is considered.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) maybeCreateDSProof(tx *bchutil.Tx, flags txscript.ScriptFlags) {
	for _, in := range tx.MsgTx().TxIn {
		op := in.PreviousOutPoint
		first, ok := mp.outpoints[op]
		if !ok {
			continue
		}
		if _, ok := mp.txDSProofs[*first.Hash()]; ok {
			return
		}
		err := mp.createDSProof(first, tx, op, flags)
		if err != nil {
			log.Debugf("No double spend proof for %v and %v: %v",
				first.Hash(), tx.Hash(), err)
		}
		return
	}
}

// createDSProof creates, checks and stores a proof that first, a memory pool
// transaction, and tx both spend op.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) createDSProof(first, tx *bchutil.Tx, op wire.OutPoint, flags txscript.ScriptFlags) error {
	coin := mp.dsProofCoin(op)
	if coin == nil {
		return fmt.Errorf("output %v not found", op)
	}
	proof, err := newDSProof(first.MsgTx(), tx.MsgTx(), op, coin, flags,
		mp.cfg.SigCache, mp.cfg.HashCache)
	if err != nil {
		return err
	}
	hash := proof.ProofHash()
	validity, err := validateDSProof(proof, &hash, coin, first.MsgTx(), flags)
	if validity != dsProofValid {
		return fmt.Errorf("proof is %v: %v", validity, err)
	}
	if mp.addDSProofForTx(proof, hash, first) == nil {
		return errors.New("transaction already has a proof")
	}
	log.Debugf("Created double spend proof %v for %v (double spend %v)",
		hash, first.Hash(), tx.Hash())
	return nil
}

// dsProofOrphansFor returns the orphan proofs for the outpoints tx spends.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) dsProofOrphansFor(tx *bchutil.Tx) []*dsProofEntry {
	var orphans []*dsProofEntry
	for _, in := range tx.MsgTx().TxIn {
		orphans = append(orphans, mp.dsProofs.findOrphans(in.PreviousOutPoint)...)
	}
	return orphans
}

// rescueDSProofOrphans checks the orphan proofs for the outpoints spent by
// tx, which was just added to the memory pool.  The first valid one is
// associated with tx and the rest are removed.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) rescueDSProofOrphans(tx *bchutil.Tx, orphans []*dsProofEntry, flags txscript.ScriptFlags) {
	for i, e := range orphans {
		validity, err := mp.validateDSProofInPool(e.proof, &e.hash, nil, flags)
		switch validity {
		case dsProofValid:
			if mp.addDSProofForTx(e.proof, e.hash, nil) == nil {
				mp.dsProofs.remove(e.hash)
				continue
			}
			log.Debugf("Orphan double spend proof %v is valid for %v",
				e.hash, tx.Hash())
			for _, rest := range orphans[i+1:] {
				mp.dsProofs.remove(rest.hash)
			}
			return

		case dsProofInvalid:
			log.Debugf("Orphan double spend proof %v is invalid: %v",
				e.hash, err)
			mp.dsProofs.remove(e.hash)
			if e.hasPeer {
				mp.badDSProofPeers = append(mp.badDSProofPeers, e.peer)
			}

		default:
			mp.dsProofs.remove(e.hash)
		}
	}
}

// removeTxDSProof turns the proof of a transaction leaving the memory pool
// into an orphan, so it applies again should the transaction come back.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) removeTxDSProof(txHash *chainhash.Hash) {
	if hash, ok := mp.txDSProofs[*txHash]; ok {
		delete(mp.txDSProofs, *txHash)
		mp.dsProofs.orphanExisting(hash)
	}
}

// DSProofResult is the outcome of processing a double spend proof received
// from a peer.
type DSProofResult int

const (
	// DSProofAccepted means the proof was valid and associated with a
	// memory pool transaction.
	DSProofAccepted DSProofResult = iota

	// DSProofOrphan means the proof was stored as an orphan because the
	// output it spends or a transaction spending that output is missing.
	DSProofOrphan

	// DSProofIgnored means the proof was dropped because the transaction
	// spending its outpoint already has a proof, or because double spend
	// proofs are disabled.
	DSProofIgnored
)

// String returns the result in human-readable form.
func (r DSProofResult) String() string {
	switch r {
	case DSProofAccepted:
		return "accepted"
	case DSProofOrphan:
		return "orphan"
	}
	return "ignored"
}

// ProcessDSProof checks a double spend proof received from peer and stores
// it when it is valid or cannot be checked yet.  An error means the proof is
// invalid; its id is then remembered as rejected, and the proof is not
// checked again while it is.
//
// This function is safe for concurrent access.
func (mp *TxPool) ProcessDSProof(proof *wire.MsgDSProof, peer Tag, hasPeer bool) (DSProofResult, error) {
	if mp.cfg.DisableDSProofs {
		return DSProofIgnored, nil
	}
	hash := proof.ProofHash()
	flags := mp.currentScriptFlags()

	// The signatures are checked under the read lock, and again under the
	// write lock only if the transaction spending the outpoint changed in
	// between.
	mp.mtx.RLock()
	if mp.hasDSProofForOutPoint(proof.PrevOut) {
		mp.mtx.RUnlock()
		return DSProofIgnored, nil
	}
	if mp.dsProofs.isRejected(hash) {
		mp.mtx.RUnlock()
		return DSProofIgnored, txRuleError(wire.RejectInvalid,
			fmt.Sprintf("double spend proof %v was already rejected", hash))
	}
	spender := mp.outpoints[proof.PrevOut]
	validity, err := mp.validateDSProofInPool(proof, &hash, spender, flags)
	mp.mtx.RUnlock()

	if dsProofRelockHook != nil {
		dsProofRelockHook()
	}
	mp.mtx.Lock()
	defer mp.unlockAndNotify()

	if mp.hasDSProofForOutPoint(proof.PrevOut) {
		return DSProofIgnored, nil
	}
	if mp.outpoints[proof.PrevOut] != spender {
		validity, err = mp.validateDSProofInPool(proof, &hash, nil, flags)
	}
	switch validity {
	case dsProofValid:
		if mp.addDSProofForTx(proof, hash, nil) == nil {
			return DSProofIgnored, nil
		}
		return DSProofAccepted, nil

	case dsProofMissingUTXO, dsProofMissingTransaction:
		if !mp.dsProofs.addOrphan(proof, hash, peer, hasPeer) {
			return DSProofIgnored, nil
		}
		return DSProofOrphan, nil
	}

	mp.dsProofs.markRejected(hash)
	return DSProofIgnored, txRuleError(wire.RejectInvalid,
		fmt.Sprintf("invalid double spend proof %v: %v", hash, err))
}

// DSProofsEnabled returns whether double spend proofs are enabled.
func (mp *TxPool) DSProofsEnabled() bool {
	return !mp.cfg.DisableDSProofs
}

// HaveDSProof returns whether the proof is known, as an orphan or not, or was
// recently rejected.  It always returns true when proofs are disabled, so
// they are never requested.
//
// This function is safe for concurrent access.
func (mp *TxPool) HaveDSProof(hash *chainhash.Hash) bool {
	if mp.cfg.DisableDSProofs {
		return true
	}
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	return mp.dsProofs.lookup(*hash) != nil || mp.dsProofs.isRejected(*hash)
}

// FetchDSProof returns the proof with the given id, including orphans.
//
// This function is safe for concurrent access.
func (mp *TxPool) FetchDSProof(hash *chainhash.Hash) (*wire.MsgDSProof, error) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	if e := mp.dsProofs.lookup(*hash); e != nil {
		return e.proof, nil
	}
	return nil, fmt.Errorf("double spend proof %v is not known", hash)
}

// ExpireDSProofOrphans removes the orphan proofs that have been waiting for
// longer than the orphan TTL and returns the peers that sent them.
//
// This function is safe for concurrent access.
func (mp *TxPool) ExpireDSProofOrphans() []Tag {
	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	return mp.dsProofs.expireOrphans()
}

// dsProofDesc returns the description of a stored proof.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) dsProofDesc(e *dsProofEntry) *DSProofDesc {
	desc := &DSProofDesc{Proof: e.proof, Hash: e.hash}
	if tx, ok := mp.outpoints[e.proof.PrevOut]; ok {
		desc.TxHash = tx.Hash()
	}
	return desc
}

// DSProofs returns the proofs associated with memory pool transactions, and
// the orphans too when includeOrphans is set.
//
// This function is safe for concurrent access.
func (mp *TxPool) DSProofs(includeOrphans bool) []*DSProofDesc {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	descs := make([]*DSProofDesc, 0, len(mp.dsProofs.proofs))
	for _, e := range mp.dsProofs.proofs {
		if e.orphan && !includeOrphans {
			continue
		}
		desc := &DSProofDesc{Proof: e.proof, Hash: e.hash}
		if tx, ok := mp.outpoints[e.proof.PrevOut]; ok && !e.orphan {
			desc.TxHash = tx.Hash()
		}
		descs = append(descs, desc)
	}
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Hash.Compare(&descs[j].Hash) < 0
	})
	return descs
}

// descendants returns the memory pool transactions spending the outputs of
// txHash, directly or not, including txHash itself, ordered by hash.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) descendants(txHash chainhash.Hash) []chainhash.Hash {
	seen := map[chainhash.Hash]struct{}{txHash: {}}
	queue := []chainhash.Hash{txHash}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		desc, ok := mp.pool[h]
		if !ok {
			continue
		}
		for i := range desc.Tx.MsgTx().TxOut {
			child, ok := mp.outpoints[wire.OutPoint{Hash: h, Index: uint32(i)}]
			if !ok {
				continue
			}
			if _, ok := seen[*child.Hash()]; !ok {
				seen[*child.Hash()] = struct{}{}
				queue = append(queue, *child.Hash())
			}
		}
	}
	hashes := make([]chainhash.Hash, 0, len(seen))
	for h := range seen {
		hashes = append(hashes, h)
	}
	sort.Slice(hashes, func(i, j int) bool {
		return hashes[i].Compare(&hashes[j]) < 0
	})
	return hashes
}

// DSProofByHash returns the proof with the given id, including orphans, and
// the descendants of the transaction it is for.
//
// This function is safe for concurrent access.
func (mp *TxPool) DSProofByHash(hash *chainhash.Hash) (*DSProofDesc, []chainhash.Hash) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	e := mp.dsProofs.lookup(*hash)
	if e == nil {
		return nil, nil
	}
	desc := mp.dsProofDesc(e)
	if desc.TxHash == nil {
		return desc, nil
	}
	return desc, mp.descendants(*desc.TxHash)
}

// DSProofByOutPoint returns the proof of the memory pool transaction
// spending op, or when no transaction spends it, an orphan proof for it.
//
// This function is safe for concurrent access.
func (mp *TxPool) DSProofByOutPoint(op wire.OutPoint) (*DSProofDesc, []chainhash.Hash) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	if tx, ok := mp.outpoints[op]; ok {
		hash, ok := mp.txDSProofs[*tx.Hash()]
		if !ok {
			return nil, nil
		}
		e := mp.dsProofs.lookup(hash)
		return &DSProofDesc{Proof: e.proof, Hash: hash, TxHash: tx.Hash()},
			mp.descendants(*tx.Hash())
	}
	if orphans := mp.dsProofs.findOrphans(op); len(orphans) > 0 {
		return &DSProofDesc{Proof: orphans[0].proof, Hash: orphans[0].hash}, nil
	}
	return nil, nil
}

// DSProofByTx returns the proof of a memory pool transaction.  When
// recursive is set and the transaction has none, its unconfirmed ancestors
// are searched as well, and path lists the transactions from txHash to the
// one with the proof.
//
// This function is safe for concurrent access.
func (mp *TxPool) DSProofByTx(txHash *chainhash.Hash, recursive bool) (desc *DSProofDesc,
	descendants []chainhash.Hash, path []chainhash.Hash, err error) {

	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	if !recursive {
		hash, ok := mp.txDSProofs[*txHash]
		if !ok {
			return nil, nil, nil, nil
		}
		e := mp.dsProofs.lookup(hash)
		return &DSProofDesc{Proof: e.proof, Hash: hash, TxHash: txHash},
			mp.descendants(*txHash), nil, nil
	}

	path, hash, _, err := mp.searchDSProofs(*txHash, false)
	if err != nil || path == nil {
		return nil, nil, nil, err
	}
	dsTx := path[len(path)-1]
	e := mp.dsProofs.lookup(hash)
	return &DSProofDesc{Proof: e.proof, Hash: hash, TxHash: &dsTx},
		mp.descendants(dsTx), path, nil
}

// DSProofScore returns how far a memory pool transaction can be trusted not
// to be double spent: 1 when neither it nor an unconfirmed ancestor has a
// proof and proofs are possible for all of them, 0 when one has a proof or
// one cannot have a proof, and 0.25 when an input does not sign all inputs
// and outputs or the ancestors are too many to check.
//
// This function is safe for concurrent access.
func (mp *TxPool) DSProofScore(txHash *chainhash.Hash) (float64, error) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	if _, ok := mp.pool[*txHash]; !ok {
		return 0, fmt.Errorf("transaction %v is not in the pool", txHash)
	}
	_, _, score, err := mp.searchDSProofs(*txHash, true)
	if err == ErrDSProofSearchLimit {
		err = nil
	}
	return score, err
}

// searchDSProofs searches a memory pool transaction and its unconfirmed
// ancestors, depth first, for a proof.  It returns the path from txHash to
// the transaction with the proof and the proof id.  With scoring set, the
// search also stops at a transaction that cannot have a proof and the score
// is computed as described for DSProofScore.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) searchDSProofs(txHash chainhash.Hash, scoring bool) ([]chainhash.Hash,
	chainhash.Hash, float64, error) {

	score := 1.0
	var path []chainhash.Hash
	var found chainhash.Hash
	var done bool
	seen := make(map[chainhash.Hash]struct{})

	var search func(txD *TxDesc) error
	search = func(txD *TxDesc) error {
		hash := *txD.Tx.Hash()
		path = append(path, hash)
		if len(path) > dsProofSearchMaxDepth || len(seen) > dsProofSearchMaxTxs {
			score = min(score, 0.25)
			return ErrDSProofSearchLimit
		}
		if proof, ok := mp.txDSProofs[hash]; ok {
			found, done, score = proof, true, 0
			return nil
		}
		if scoring {
			possible, protected := proofPossibleForTx(txD.Tx.MsgTx(), mp.dsProofCoin)
			if !possible {
				score = 0
				return nil
			}
			if !protected {
				score = min(score, 0.25)
			}
		}
		// Parents are searched in the order they entered the pool.
		var parents []*TxDesc
		for _, in := range txD.Tx.MsgTx().TxIn {
			if parent, ok := mp.pool[in.PreviousOutPoint.Hash]; ok {
				parents = append(parents, parent)
			}
		}
		sort.Slice(parents, func(i, j int) bool {
			return parents[i].entrySeq < parents[j].entrySeq
		})
		for _, parent := range parents {
			if _, ok := seen[*parent.Tx.Hash()]; ok {
				continue
			}
			seen[*parent.Tx.Hash()] = struct{}{}
			if err := search(parent); err != nil {
				return err
			}
			if done || (scoring && score < 0.001) {
				return nil
			}
		}
		path = path[:len(path)-1]
		return nil
	}

	txD, ok := mp.pool[txHash]
	if !ok {
		return nil, found, -1, nil
	}
	err := search(txD)
	if err != nil || !done {
		return nil, found, score, err
	}
	return path, found, score, nil
}
