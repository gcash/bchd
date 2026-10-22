// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchest/siphash"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

const (
	// orphanTTL is the maximum amount of time an orphan is allowed to
	// stay in the orphan pool before it expires and is evicted during the
	// next scan.
	orphanTTL = time.Minute * 15

	// orphanExpireScanInterval is the minimum amount of time in between
	// scans of the orphan pool to evict expired transactions.
	orphanExpireScanInterval = time.Minute * 5

	// mempoolExpiry is how long a transaction can stay in the pool: BCHN's
	// default -mempoolexpiry of two weeks.
	mempoolExpiry = time.Hour * 336
)

// Tag represents an identifier to use for tagging orphan transactions.  The
// caller may choose any scheme it desires, however it is common to use peer IDs
// so that orphans can be identified by which peer first relayed them.
type Tag uint64

// Config is a descriptor containing the memory pool configuration.
type Config struct {
	// Policy defines the various mempool configuration options related
	// to policy.
	Policy Policy

	// ChainParams identifies which chain parameters the txpool is
	// associated with.
	ChainParams *chaincfg.Params

	// FetchUtxoView defines the function to use to fetch unspent
	// transaction output information.
	FetchUtxoView func(*bchutil.Tx) (*blockchain.UtxoViewpoint, error)

	// FetchUtxoEntryUncached looks up an output in the utxo set without
	// adding it, or its absence, to the utxo cache.  The coins named by
	// double spend proofs are looked up with it.  When it is nil,
	// FetchUtxoView is used.
	FetchUtxoEntryUncached func(wire.OutPoint) (*blockchain.UtxoEntry, error)

	// BestHeight defines the function to use to access the block height of
	// the current best chain.
	BestHeight func() int32

	// MedianTimePast defines the function to use in order to access the
	// median time past calculated from the point-of-view of the current
	// chain tip within the best chain.
	MedianTimePast func() time.Time

	// CalcSequenceLock defines the function to use in order to generate
	// the current sequence lock for the given transaction using the passed
	// utxo view.
	CalcSequenceLock func(*bchutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error)

	// IsDeploymentActive returns true if the target deploymentID is
	// active, and false otherwise. The mempool uses this function to gauge
	// if transactions using new to be soft-forked rules should be allowed
	// into the mempool or not.
	IsDeploymentActive func(deploymentID uint32) (bool, error)

	// SigCache defines a signature cache to use.
	SigCache *txscript.SigCache

	// HashCache defines the transaction hash mid-state cache to use.
	HashCache *txscript.HashCache

	// AddrIndex defines the optional address index instance to use for
	// indexing the unconfirmed transactions in the memory pool.
	// This can be nil if the address index is not enabled.
	AddrIndex *indexers.AddrIndex

	// FeeEstimatator provides a feeEstimator. If it is not nil, the mempool
	// records all new transactions it observes into the feeEstimator.
	FeeEstimator *FeeEstimator

	// DisableDSProofs turns off the creation and handling of double spend
	// proofs.
	DisableDSProofs bool

	// OnDSProof is called, without the mempool lock held, when a double
	// spend proof becomes associated with the memory pool transaction tx.
	OnDSProof func(proof *wire.MsgDSProof, tx *bchutil.Tx)

	// OnBadDSProofs is called, without the mempool lock held, with the
	// peers that sent orphan double spend proofs which turned out to be
	// invalid.
	OnBadDSProofs func(peers []Tag)
}

// Policy houses the policy (configuration parameters) which is used to
// control the mempool.
type Policy struct {
	// MaxTxVersion is the transaction version that the mempool should
	// accept.  All transactions above this version are rejected as
	// non-standard.
	MaxTxVersion int32

	// AcceptNonStd defines whether to accept non-standard transactions. If
	// true, non-standard transactions will be accepted into the mempool.
	// Otherwise, all non-standard transactions will be rejected.
	AcceptNonStd bool

	// MaxOrphanTxs is the maximum number of orphan transactions
	// that can be queued.
	MaxOrphanTxs int

	// MaxOrphanTxSize is the maximum size allowed for orphan transactions.
	// This helps prevent memory exhaustion attacks from sending a lot of
	// of big orphans.
	MaxOrphanTxSize int

	// LimitSigChecks applies an additional standardness limit to the number
	// of signature checks in each transaction.
	LimitSigChecks bool

	// MinRelayTxFee defines the minimum fee rate in Satoshi/kB a new
	// transaction must pay to be accepted.
	MinRelayTxFee bchutil.Amount
}

// TxDesc is a descriptor containing a transaction in the mempool along with
// additional metadata.
type TxDesc struct {
	mining.TxDesc

	// entrySeq orders the transactions by when they were added, with those
	// added back from disconnected blocks first.
	entrySeq int64

	// dsProofPossible and dsProofProtected are whether a double spend proof
	// is possible for the transaction and whether all of its signatures
	// cover all of it.  They depend only on the transaction and the coins
	// it spends, so they are found once, when it is added.
	dsProofPossible  bool
	dsProofProtected bool
}

// orphanTx is normal transaction that references an ancestor transaction
// that is not yet available.  It also contains additional information related
// to it such as an expiration time to help prevent caching the orphan forever.
type orphanTx struct {
	tx         *bchutil.Tx
	tag        Tag
	expiration time.Time
}

// TxPool is used as a source of transactions that need to be mined into blocks
// and relayed to other peers.  It is safe for concurrent access from multiple
// peers.
type TxPool struct {
	// The following variables must only be used atomically.
	lastUpdated atomic.Int64 // last time pool was updated

	mtx           sync.RWMutex
	cfg           Config
	pool          map[chainhash.Hash]*TxDesc
	orphans       map[chainhash.Hash]*orphanTx
	orphansByPrev map[wire.OutPoint]map[chainhash.Hash]*bchutil.Tx
	outpoints     map[wire.OutPoint]*bchutil.Tx
	dsProofs      *dsProofStorage
	txDSProofs    map[chainhash.Hash]chainhash.Hash
	nextEntrySeq  int64
	nextReaddSeq  int64

	// nextExpireScan is the time after which the orphan pool will be
	// scanned in order to evict orphans.  This is NOT a hard deadline as
	// the scan will only run when an orphan is added to the pool as opposed
	// to on an unconditional timer.
	nextExpireScan time.Time

	// Double spend proofs newly associated with transactions and peers that
	// sent invalid orphan proofs, passed to the callbacks once the write
	// lock is released.
	dsProofAnnouncements []dsProofAnnouncement
	badDSProofPeers      []Tag
}

// Ensure the TxPool type implements the mining.TxSource interface.
var _ mining.TxSource = (*TxPool)(nil)

// removeOrphan is the internal function which implements the public
// RemoveOrphan.  See the comment for RemoveOrphan for more details.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) removeOrphan(tx *bchutil.Tx, removeRedeemers bool) {
	// Nothing to do if passed tx is not an orphan.
	txHash := tx.Hash()
	otx, exists := mp.orphans[*txHash]
	if !exists {
		return
	}

	// Remove the reference from the previous orphan index.
	for _, txIn := range otx.tx.MsgTx().TxIn {
		orphans, exists := mp.orphansByPrev[txIn.PreviousOutPoint]
		if exists {
			delete(orphans, *txHash)

			// Remove the map entry altogether if there are no
			// longer any orphans which depend on it.
			if len(orphans) == 0 {
				delete(mp.orphansByPrev, txIn.PreviousOutPoint)
			}
		}
	}

	// Remove any orphans that redeem outputs from this one if requested.
	if removeRedeemers {
		prevOut := wire.OutPoint{Hash: *txHash}
		for txOutIdx := range tx.MsgTx().TxOut {
			prevOut.Index = uint32(txOutIdx)
			for _, orphan := range mp.orphansByPrev[prevOut] {
				mp.removeOrphan(orphan, true)
			}
		}
	}

	// Remove the transaction from the orphan pool.
	delete(mp.orphans, *txHash)
}

// RemoveOrphan removes the passed orphan transaction from the orphan pool and
// previous orphan index.
//
// This function is safe for concurrent access.
func (mp *TxPool) RemoveOrphan(tx *bchutil.Tx) {
	mp.mtx.Lock()
	mp.removeOrphan(tx, false)
	mp.mtx.Unlock()
}

// RemoveOrphansByTag removes all orphan transactions tagged with the provided
// identifier.
//
// This function is safe for concurrent access.
func (mp *TxPool) RemoveOrphansByTag(tag Tag) uint64 {
	var numEvicted uint64
	mp.mtx.Lock()
	for _, otx := range mp.orphans {
		if otx.tag == tag {
			mp.removeOrphan(otx.tx, true)
			numEvicted++
		}
	}
	mp.mtx.Unlock()
	return numEvicted
}

// limitNumOrphans limits the number of orphan transactions by evicting a random
// orphan if adding a new one would cause it to overflow the max allowed.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) limitNumOrphans() error {
	// Scan through the orphan pool and remove any expired orphans when it's
	// time.  This is done for efficiency so the scan only happens
	// periodically instead of on every orphan added to the pool.
	if now := time.Now(); now.After(mp.nextExpireScan) {
		origNumOrphans := len(mp.orphans)
		for _, otx := range mp.orphans {
			if now.After(otx.expiration) {
				// Remove redeemers too because the missing
				// parents are very unlikely to ever materialize
				// since the orphan has already been around more
				// than long enough for them to be delivered.
				mp.removeOrphan(otx.tx, true)
			}
		}

		// Set next expiration scan to occur after the scan interval.
		mp.nextExpireScan = now.Add(orphanExpireScanInterval)

		numOrphans := len(mp.orphans)
		if numExpired := origNumOrphans - numOrphans; numExpired > 0 {
			log.Debugf("Expired %d %s (remaining: %d)", numExpired,
				pickNoun(numExpired, "orphan", "orphans"),
				numOrphans)
		}
	}

	// Nothing to do if adding another orphan will not cause the pool to
	// exceed the limit.
	if len(mp.orphans)+1 <= mp.cfg.Policy.MaxOrphanTxs {
		return nil
	}

	// Remove a random entry from the map.  For most compilers, Go's
	// range statement iterates starting at a random item although
	// that is not 100% guaranteed by the spec.  The iteration order
	// is not important here because an adversary would have to be
	// able to pull off preimage attacks on the hashing function in
	// order to target eviction of specific entries anyways.
	for _, otx := range mp.orphans {
		// Don't remove redeemers in the case of a random eviction since
		// it is quite possible it might be needed again shortly.
		mp.removeOrphan(otx.tx, false)
		break
	}

	return nil
}

// addOrphan adds an orphan transaction to the orphan pool.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) addOrphan(tx *bchutil.Tx, tag Tag) {
	// Nothing to do if no orphans are allowed.
	if mp.cfg.Policy.MaxOrphanTxs <= 0 {
		return
	}

	// Limit the number orphan transactions to prevent memory exhaustion.
	// This will periodically remove any expired orphans and evict a random
	// orphan if space is still needed.
	_ = mp.limitNumOrphans()

	mp.orphans[*tx.Hash()] = &orphanTx{
		tx:         tx,
		tag:        tag,
		expiration: time.Now().Add(orphanTTL),
	}
	for _, txIn := range tx.MsgTx().TxIn {
		if _, exists := mp.orphansByPrev[txIn.PreviousOutPoint]; !exists {
			mp.orphansByPrev[txIn.PreviousOutPoint] =
				make(map[chainhash.Hash]*bchutil.Tx)
		}
		mp.orphansByPrev[txIn.PreviousOutPoint][*tx.Hash()] = tx
	}

	log.Debugf("Stored orphan transaction %v (total: %d)", tx.Hash(),
		len(mp.orphans))
}

// maybeAddOrphan potentially adds an orphan to the orphan pool.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) maybeAddOrphan(tx *bchutil.Tx, tag Tag) error {
	// Ignore orphan transactions that are too large.  This helps avoid
	// a memory exhaustion attack based on sending a lot of really large
	// orphans.  In the case there is a valid transaction larger than this,
	// it will ultimtely be rebroadcast after the parent transactions
	// have been mined or otherwise received.
	//
	// Note that the number of orphan transactions in the orphan pool is
	// also limited, so this equates to a maximum memory used of
	// mp.cfg.Policy.MaxOrphanTxSize * mp.cfg.Policy.MaxOrphanTxs (which is ~5MB
	// using the default values at the time this comment was written).
	serializedLen := tx.MsgTx().SerializeSize()
	if serializedLen > mp.cfg.Policy.MaxOrphanTxSize {
		str := fmt.Sprintf("orphan transaction size of %d bytes is "+
			"larger than max allowed size of %d bytes",
			serializedLen, mp.cfg.Policy.MaxOrphanTxSize)
		return txRuleError(wire.RejectNonstandard, str)
	}

	// Add the orphan if the none of the above disqualified it.
	mp.addOrphan(tx, tag)

	return nil
}

// removeOrphanDoubleSpends removes all orphans which spend outputs spent by the
// passed transaction from the orphan pool.  Removing those orphans then leads
// to removing all orphans which rely on them, recursively.  This is necessary
// when a transaction is added to the main pool because it may spend outputs
// that orphans also spend.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) removeOrphanDoubleSpends(tx *bchutil.Tx) {
	msgTx := tx.MsgTx()
	for _, txIn := range msgTx.TxIn {
		for _, orphan := range mp.orphansByPrev[txIn.PreviousOutPoint] {
			mp.removeOrphan(orphan, true)
		}
	}
}

// isTransactionInPool returns whether or not the passed transaction already
// exists in the main pool.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) isTransactionInPool(hash *chainhash.Hash) bool {
	if _, exists := mp.pool[*hash]; exists {
		return true
	}

	return false
}

// IsTransactionInPool returns whether or not the passed transaction already
// exists in the main pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) IsTransactionInPool(hash *chainhash.Hash) bool {
	// Protect concurrent access.
	mp.mtx.RLock()
	inPool := mp.isTransactionInPool(hash)
	mp.mtx.RUnlock()

	return inPool
}

// isOrphanInPool returns whether or not the passed transaction already exists
// in the orphan pool.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) isOrphanInPool(hash *chainhash.Hash) bool {
	if _, exists := mp.orphans[*hash]; exists {
		return true
	}

	return false
}

// IsOrphanInPool returns whether or not the passed transaction already exists
// in the orphan pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) IsOrphanInPool(hash *chainhash.Hash) bool {
	// Protect concurrent access.
	mp.mtx.RLock()
	inPool := mp.isOrphanInPool(hash)
	mp.mtx.RUnlock()

	return inPool
}

// haveTransaction returns whether or not the passed transaction already exists
// in the main pool or in the orphan pool.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) haveTransaction(hash *chainhash.Hash) bool {
	return mp.isTransactionInPool(hash) || mp.isOrphanInPool(hash)
}

// HaveTransaction returns whether or not the passed transaction already exists
// in the main pool or in the orphan pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) HaveTransaction(hash *chainhash.Hash) bool {
	// Protect concurrent access.
	mp.mtx.RLock()
	haveTx := mp.haveTransaction(hash)
	mp.mtx.RUnlock()

	return haveTx
}

// removeTransaction is the internal function which implements the public
// RemoveTransaction.  See the comment for RemoveTransaction for more details.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) removeTransaction(tx *bchutil.Tx, removeRedeemers bool) {
	txHash := tx.Hash()
	if removeRedeemers {
		// Remove any transactions which rely on this one.
		for i := range uint32(len(tx.MsgTx().TxOut)) {
			prevOut := wire.OutPoint{Hash: *txHash, Index: i}
			if txRedeemer, exists := mp.outpoints[prevOut]; exists {
				mp.removeTransaction(txRedeemer, true)
			}
		}
	}

	// Remove the transaction if needed.
	if txDesc, exists := mp.pool[*txHash]; exists {
		// Remove unconfirmed address index entries associated with the
		// transaction if enabled.
		if mp.cfg.AddrIndex != nil {
			mp.cfg.AddrIndex.RemoveUnconfirmedTx(txHash)
		}

		// Mark the referenced outpoints as unspent by the pool.
		for _, txIn := range txDesc.Tx.MsgTx().TxIn {
			delete(mp.outpoints, txIn.PreviousOutPoint)
		}
		mp.removeTxDSProof(txHash)
		delete(mp.pool, *txHash)
		mp.lastUpdated.Store(time.Now().Unix())
	}
}

// RemoveTransaction removes the passed transaction from the mempool. When the
// removeRedeemers flag is set, any transactions that redeem outputs from the
// removed transaction will also be removed recursively from the mempool, as
// they would otherwise become orphans.
//
// This function is safe for concurrent access.
func (mp *TxPool) RemoveTransaction(tx *bchutil.Tx, removeRedeemers bool) {
	// Protect concurrent access.
	mp.mtx.Lock()
	mp.removeTransaction(tx, removeRedeemers)
	mp.mtx.Unlock()
}

// Expire removes the transactions that entered the pool longer ago than BCHN's
// default -mempoolexpiry of two weeks, with the transactions that spend them,
// and returns how many it removed.  Transactions a reorg put back below the
// minimum relay fee are never mined, so this is what removes them.
//
// This function is safe for concurrent access.
func (mp *TxPool) Expire() int {
	cutoff := time.Now().Add(-mempoolExpiry)

	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	n := len(mp.pool)
	for _, txDesc := range mp.pool {
		if txDesc.Added.Before(cutoff) {
			mp.removeTransaction(txDesc.Tx, true)
		}
	}
	return n - len(mp.pool)
}

// RemoveDoubleSpends removes all transactions which spend outputs spent by the
// passed transaction from the memory pool.  Removing those transactions then
// leads to removing all transactions which rely on them, recursively.  This is
// necessary when a block is connected to the main chain because the block may
// contain transactions which were previously unknown to the memory pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) RemoveDoubleSpends(tx *bchutil.Tx) {
	// Protect concurrent access.
	mp.mtx.Lock()
	for _, txIn := range tx.MsgTx().TxIn {
		if txRedeemer, ok := mp.outpoints[txIn.PreviousOutPoint]; ok {
			if !txRedeemer.Hash().IsEqual(tx.Hash()) {
				mp.removeTransaction(txRedeemer, true)
			}
		}
	}
	mp.mtx.Unlock()
}

// addTransaction adds the passed transaction to the memory pool.  It should
// not be called directly as it doesn't perform any validation.  This is a
// helper for maybeAcceptTransaction.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) addTransaction(utxoView *blockchain.UtxoViewpoint, tx *bchutil.Tx, height int32, fee int64, isNew bool) *TxDesc {
	// Add the transaction to the pool and mark the referenced outpoints
	// as spent by the pool.
	txD := &TxDesc{
		TxDesc: mining.TxDesc{
			Tx:       tx,
			Added:    time.Now(),
			Height:   height,
			Fee:      fee,
			FeePerKB: fee * 1000 / int64(tx.MsgTx().SerializeSize()),
		},
	}
	if isNew {
		txD.entrySeq = mp.nextEntrySeq
		mp.nextEntrySeq++
	} else {
		txD.entrySeq = mp.nextReaddSeq
		mp.nextReaddSeq++
	}
	txD.dsProofPossible, txD.dsProofProtected = proofPossibleForTx(tx.MsgTx(),
		func(op wire.OutPoint) *wire.TxOut {
			entry := utxoView.LookupEntry(op)
			if entry == nil {
				return nil
			}
			return &wire.TxOut{
				Value:     entry.Amount(),
				PkScript:  entry.PkScript(),
				TokenData: entry.TokenData(),
			}
		})

	mp.pool[*tx.Hash()] = txD
	for _, txIn := range tx.MsgTx().TxIn {
		mp.outpoints[txIn.PreviousOutPoint] = tx
	}
	mp.lastUpdated.Store(time.Now().Unix())

	// Add unconfirmed address index entries associated with the transaction
	// if enabled.
	if mp.cfg.AddrIndex != nil {
		mp.cfg.AddrIndex.AddUnconfirmedTx(tx, utxoView)
	}

	// Record this tx for fee estimation if enabled.
	if mp.cfg.FeeEstimator != nil {
		mp.cfg.FeeEstimator.ObserveTransaction(txD)
	}

	return txD
}

// checkPoolDoubleSpend checks whether or not the passed transaction is
// attempting to spend coins already spent by other transactions in the pool.
// Note it does not check for double spends against transactions already in the
// main chain.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) checkPoolDoubleSpend(tx *bchutil.Tx) error {
	for _, txIn := range tx.MsgTx().TxIn {
		if txR, exists := mp.outpoints[txIn.PreviousOutPoint]; exists {
			str := fmt.Sprintf("output %v already spent by "+
				"transaction %v in the memory pool",
				txIn.PreviousOutPoint, txR.Hash())
			return txRuleError(wire.RejectDuplicate, str)
		}
	}

	return nil
}

// CheckSpend checks whether the passed outpoint is already spent by a
// transaction in the mempool. If that's the case the spending transaction will
// be returned, if not nil will be returned.
func (mp *TxPool) CheckSpend(op wire.OutPoint) *bchutil.Tx {
	mp.mtx.RLock()
	txR := mp.outpoints[op]
	mp.mtx.RUnlock()

	return txR
}

// FetchInputUtxos loads utxo details about the input transactions referenced by
// the passed transaction.  First, it loads the details form the viewpoint of
// the main chain, then it adjusts them based upon the contents of the
// transaction pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) FetchInputUtxos(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()

	return mp.fetchInputUtxos(tx)
}

// fetchInputUtxos loads utxo details about the input transactions referenced by
// the passed transaction.  First, it loads the details form the viewpoint of
// the main chain, then it adjusts them based upon the contents of the
// transaction pool.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) fetchInputUtxos(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
	utxoView, err := mp.cfg.FetchUtxoView(tx)
	if err != nil {
		return nil, err
	}

	// Attempt to populate any missing inputs from the transaction pool.
	for _, txIn := range tx.MsgTx().TxIn {
		prevOut := &txIn.PreviousOutPoint
		entry := utxoView.LookupEntry(*prevOut)
		if entry != nil && !entry.IsSpent() {
			continue
		}

		if poolTxDesc, exists := mp.pool[prevOut.Hash]; exists {
			// AddTxOut ignores out of range index values, so it is
			// safe to call without bounds checking here.
			utxoView.AddTxOut(poolTxDesc.Tx, prevOut.Index,
				mining.UnminedHeight)
		}
	}

	return utxoView, nil
}

// FetchUtxoView loads utxo details about the transactions inputs and outputs.
// First, it loads the details form the viewpoint of the main chain, then it
// adjusts them based upon the contents of the transaction pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) FetchUtxoView(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()

	return mp.fetchUtxoView(tx)
}

// fetchUtxoView loads utxo details about the transactions inputs and outputs.
// First, it loads the details form the viewpoint of the main chain, then it
// adjusts them based upon the contents of the transaction pool.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) fetchUtxoView(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
	utxoView, err := mp.fetchInputUtxos(tx)
	if err != nil {
		return nil, err
	}

	txHash := tx.Hash()
	_, existsInPool := mp.pool[*txHash]

	// Check to see if any of the outputs are spent in the pool.
	for i := range tx.MsgTx().TxOut {
		if existsInPool {
			utxoView.AddTxOut(tx, uint32(i), mining.UnminedHeight)
		}
		prevOut := wire.OutPoint{Hash: *txHash, Index: uint32(i)}
		if _, outpointExists := mp.outpoints[prevOut]; outpointExists {
			entry := utxoView.LookupEntry(prevOut)
			if entry != nil {
				entry.Spend()
			}
		}
	}

	return utxoView, nil
}

// FetchTransaction returns the requested transaction from the transaction pool.
// This only fetches from the main transaction pool and does not include
// orphans.
//
// This function is safe for concurrent access.
func (mp *TxPool) FetchTransaction(txHash *chainhash.Hash) (*bchutil.Tx, error) {
	// Protect concurrent access.
	mp.mtx.RLock()
	txDesc, exists := mp.pool[*txHash]
	mp.mtx.RUnlock()

	if exists {
		return txDesc.Tx, nil
	}

	return nil, fmt.Errorf("transaction is not in the pool")
}

// FetchTxDesc returns the requested transaction description from the
// transaction pool. This only fetches from the main transaction pool
// and does not include orphans.
//
// This function is safe for concurrent access.
func (mp *TxPool) FetchTxDesc(txHash *chainhash.Hash) (*TxDesc, error) {
	// Protect concurrent access.
	mp.mtx.RLock()
	txDesc, exists := mp.pool[*txHash]
	mp.mtx.RUnlock()

	if exists {
		return txDesc, nil
	}

	return nil, fmt.Errorf("transaction is not in the pool")
}

// maybeAcceptTransaction is the internal function which implements the public
// MaybeAcceptTransaction.  See the comment for MaybeAcceptTransaction for
// more details.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) maybeAcceptTransaction(tx *bchutil.Tx, isNew, rejectDupOrphans bool, maxFee bchutil.Amount) ([]*chainhash.Hash, *TxDesc, error) {
	result, err := mp.checkMempoolAcceptance(tx, isNew, rejectDupOrphans,
		maxFee, !mp.cfg.DisableDSProofs)
	if err != nil {
		return nil, nil, err
	}
	if len(result.MissingParents) > 0 {
		return result.MissingParents, nil, nil
	}

	// Add to transaction pool.
	txD := mp.addTransaction(result.utxoView, tx, result.bestHeight,
		result.Fee, isNew)

	// Check any orphan double spend proofs for the outputs it spends.
	if !mp.cfg.DisableDSProofs {
		if orphans := mp.dsProofOrphansFor(tx); len(orphans) > 0 {
			mp.rescueDSProofOrphans(tx, orphans, result.scriptFlags)
		}
	}

	log.Debugf("Accepted transaction %v (pool size: %v)", tx.Hash(),
		len(mp.pool))

	return nil, txD, nil
}

// MempoolAcceptResult describes a transaction that passed the mempool
// acceptance checks.
type MempoolAcceptResult struct {
	// MissingParents holds the unknown parents of an orphan transaction.
	// The other fields are only set when it is empty.
	MissingParents []*chainhash.Hash

	// Fee is the fee paid by the transaction in satoshis.
	Fee int64

	// SigChecks is the number of signature checks done by its inputs.
	SigChecks uint32

	utxoView    *blockchain.UtxoViewpoint
	bestHeight  int32
	scriptFlags txscript.ScriptFlags
}

// checkMempoolAcceptance runs the checks done before a transaction is added
// to the memory pool.  It does not modify the pool.  A positive maxFee rejects
// transactions that pay more fees than that.
//
// When createDSProof is set and the transaction spends an output already
// spent in the pool, a double spend proof is created.
//
// This function MUST be called with the mempool lock held (for reads), or
// for writes when createDSProof is set.
func (mp *TxPool) checkMempoolAcceptance(tx *bchutil.Tx, isNew, rejectDupOrphans bool, maxFee bchutil.Amount, createDSProof bool) (*MempoolAcceptResult, error) {
	txHash := tx.Hash()

	// Don't accept the transaction if it already exists in the pool.  This
	// applies to orphan transactions as well when the reject duplicate
	// orphans flag is set.  This check is intended to be a quick check to
	// weed out duplicates.
	if mp.isTransactionInPool(txHash) || (rejectDupOrphans &&
		mp.isOrphanInPool(txHash)) {

		str := fmt.Sprintf("already have transaction %v", txHash)
		return nil, txRuleError(wire.RejectDuplicate, str)
	}

	medianTimePast := mp.cfg.MedianTimePast()

	// Get the current height of the main chain.  A standalone transaction
	// will be mined into the next block at best, so its height is at least
	// one more than the current height.
	bestHeight := mp.cfg.BestHeight()
	nextBlockHeight := bestHeight + 1

	magneticAnomalyActive := false
	if nextBlockHeight > mp.cfg.ChainParams.MagneticAnonomalyForkHeight {
		magneticAnomalyActive = true
	}
	upgrade9Active := false
	if nextBlockHeight > mp.cfg.ChainParams.Upgrade9ForkHeight {
		upgrade9Active = true
	}

	upgrade12Active := medianTimePast.Unix() >= int64(mp.cfg.ChainParams.Upgrade12ActivationTime)

	scriptFlags := mp.standardScriptFlags(nextBlockHeight, medianTimePast)

	// Perform preliminary sanity checks on the transaction.  This makes
	// use of blockchain which contains the invariant rules for what
	// transactions are allowed into blocks.
	err := blockchain.CheckTransactionSanity(tx, magneticAnomalyActive, upgrade9Active, scriptFlags)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, chainRuleError(cerr)
		}
		return nil, err
	}

	// A standalone transaction must not be a coinbase transaction.
	if blockchain.IsCoinBase(tx) {
		str := fmt.Sprintf("transaction %v is an individual coinbase",
			txHash)
		return nil, txRuleError(wire.RejectInvalid, str)
	}

	// Don't allow non-standard transactions if the network parameters
	// forbid their acceptance.
	if !mp.cfg.Policy.AcceptNonStd {
		err = checkTransactionStandard(tx, nextBlockHeight,
			medianTimePast, DefaultDustRelayFee,
			mp.cfg.Policy.MaxTxVersion, upgrade9Active, upgrade12Active)
		if err != nil {
			// Attempt to extract a reject code from the error so
			// it can be retained.  When not possible, fall back to
			// a non standard error.
			rejectCode, found := extractRejectCode(err)
			if !found {
				rejectCode = wire.RejectNonstandard
			}
			str := fmt.Sprintf("transaction %v is not standard: %v",
				txHash, err)
			return nil, txRuleError(rejectCode, str)
		}
	}

	// Only accept transactions that can be mined in the next block, even
	// when non-standard transactions are accepted.
	if !blockchain.IsFinalizedTransaction(tx, nextBlockHeight, medianTimePast) {
		str := fmt.Sprintf("transaction %v is not finalized in the "+
			"next block (bad-txns-nonfinal)", txHash)
		return nil, txRuleError(wire.RejectNonstandard, str)
	}

	// The transaction may not use any of the same outputs as other
	// transactions already in the pool as that would ultimately result in a
	// double spend.  This check is intended to be quick and therefore only
	// detects double spends within the transaction pool itself.  The
	// transaction could still be double spending coins from the main chain
	// at this point.  There is a more in-depth check that happens later
	// after fetching the referenced transaction inputs from the main chain
	// which examines the actual spend data and prevents double spends.
	err = mp.checkPoolDoubleSpend(tx)
	if err != nil {
		if createDSProof {
			mp.maybeCreateDSProof(tx, scriptFlags)
		}
		return nil, err
	}

	// Fetch all of the unspent transaction outputs referenced by the inputs
	// to this transaction.  This function also attempts to fetch the
	// transaction itself to be used for detecting a duplicate transaction
	// without needing to do a separate lookup.
	utxoView, err := mp.fetchInputUtxos(tx)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, chainRuleError(cerr)
		}
		return nil, err
	}

	// Don't allow the transaction if it exists in the main chain and is not
	// not already fully spent.
	prevOut := wire.OutPoint{Hash: *txHash}
	for txOutIdx := range tx.MsgTx().TxOut {
		prevOut.Index = uint32(txOutIdx)
		entry := utxoView.LookupEntry(prevOut)
		if entry != nil && !entry.IsSpent() {
			return nil, txRuleError(wire.RejectDuplicate,
				"transaction already exists")
		}
		utxoView.RemoveEntry(prevOut)
	}

	// Transaction is an orphan if any of the referenced transaction outputs
	// don't exist or are already spent.  Adding orphans to the orphan pool
	// is not handled by this function, and the caller should use
	// maybeAddOrphan if this behavior is desired.
	var missingParents []*chainhash.Hash
	for outpoint, entry := range utxoView.Entries() {
		if entry == nil || entry.IsSpent() {
			// Must make a copy of the hash here since the iterator
			// is replaced and taking its address directly would
			// result in all of the entries pointing to the same
			// memory location and thus all be the final hash.
			hashCopy := outpoint.Hash
			missingParents = append(missingParents, &hashCopy)
		}
	}
	if len(missingParents) > 0 {
		return &MempoolAcceptResult{MissingParents: missingParents}, nil
	}

	// Don't allow the transaction into the mempool unless its sequence
	// lock is active, meaning that it'll be allowed into the next block
	// with respect to its defined relative lock times.
	sequenceLock, err := mp.cfg.CalcSequenceLock(tx, utxoView)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, chainRuleError(cerr)
		}
		return nil, err
	}
	if !blockchain.SequenceLockActive(sequenceLock, nextBlockHeight,
		medianTimePast) {
		return nil, txRuleError(wire.RejectNonstandard,
			"transaction's sequence locks on inputs not met")
	}

	// Perform several checks on the transaction inputs using the invariant
	// rules in blockchain for what transactions are allowed into blocks.
	// Also returns the fees associated with the transaction which will be
	// used later.
	txFee, err := blockchain.CheckTransactionInputs(tx, nextBlockHeight,
		utxoView, mp.cfg.ChainParams)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, chainRuleError(cerr)
		}
		return nil, err
	}

	// Don't allow transactions with non-standard inputs if the network
	// parameters forbid their acceptance.
	if !mp.cfg.Policy.AcceptNonStd {
		err := checkInputsStandard(tx, utxoView, scriptFlags)
		if err != nil {
			// Attempt to extract a reject code from the error so
			// it can be retained.  When not possible, fall back to
			// a non standard error.
			rejectCode, found := extractRejectCode(err)
			if !found {
				rejectCode = wire.RejectNonstandard
			}
			str := fmt.Sprintf("transaction %v has a non-standard "+
				"input: %v", txHash, err)
			return nil, txRuleError(rejectCode, str)
		}
	}

	// Don't allow transactions which pay less than the minimum relay fee.
	// Transactions which are being added back to the memory pool from
	// blocks that have been disconnected during a reorg are exempted.
	serializedSize := int64(tx.MsgTx().SerializeSize())
	minFee := calcMinRequiredTxRelayFee(serializedSize,
		mp.cfg.Policy.MinRelayTxFee)
	if isNew && txFee < minFee {
		str := fmt.Sprintf("transaction %v has %d fees which is under "+
			"the required amount of %d (min relay fee not met)", txHash,
			txFee, minFee)
		return nil, txRuleError(wire.RejectInsufficientFee, str)
	}

	// Reject fees above the maximum the caller allows, if any.
	if maxFee > 0 && bchutil.Amount(txFee) > maxFee {
		return nil, RuleError{Err: HighFeeError{
			Fee:    txFee,
			MaxFee: int64(maxFee),
		}}
	}

	// Verify crypto signatures for each input and reject the transaction if
	// any don't verify.
	sigChecks, err := blockchain.ValidateTransactionScripts(tx, utxoView, scriptFlags,
		mp.cfg.SigCache, mp.cfg.HashCache, mp.cfg.ChainParams.Upgrade9ForkHeight)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, chainRuleError(cerr)
		}
		return nil, err
	}

	return &MempoolAcceptResult{
		Fee:         txFee,
		SigChecks:   sigChecks,
		utxoView:    utxoView,
		bestHeight:  bestHeight,
		scriptFlags: scriptFlags,
	}, nil
}

// standardScriptFlags returns the script flags transactions must pass to be
// accepted into the memory pool for the block at nextBlockHeight.
func (mp *TxPool) standardScriptFlags(nextBlockHeight int32, medianTimePast time.Time) txscript.ScriptFlags {
	scriptFlags := txscript.StandardVerifyFlags
	if !mp.cfg.Policy.LimitSigChecks {
		scriptFlags ^= txscript.ScriptVerifyInputSigChecks
	}

	if nextBlockHeight > mp.cfg.ChainParams.Upgrade9ForkHeight {
		scriptFlags |= txscript.ScriptAllowCashTokens
	}

	if medianTimePast.Unix() >= int64(mp.cfg.ChainParams.Upgrade11ActivationTime) {
		scriptFlags |= txscript.ScriptAllowMay2025
		if !mp.cfg.Policy.AcceptNonStd {
			scriptFlags |= txscript.ScriptAllowMay2025StandardOnly
		}
	}

	if medianTimePast.Unix() >= int64(mp.cfg.ChainParams.Upgrade12ActivationTime) {
		scriptFlags |= txscript.ScriptAllowMay2026
	}
	return scriptFlags
}

// MaybeAcceptTransaction is the main workhorse for handling insertion of new
// free-standing transactions into a memory pool.  It includes functionality
// such as rejecting duplicate transactions, ensuring transactions follow all
// rules, detecting orphan transactions, and insertion into the memory pool.
//
// If the transaction is an orphan (missing parent transactions), the
// transaction is NOT added to the orphan pool, but each unknown referenced
// parent is returned.  Use ProcessTransaction instead if new orphans should
// be added to the orphan pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) MaybeAcceptTransaction(tx *bchutil.Tx, isNew bool) ([]*chainhash.Hash, *TxDesc, error) {
	// Protect concurrent access.
	mp.mtx.Lock()
	hashes, txD, err := mp.maybeAcceptTransaction(tx, isNew, true, 0)
	mp.unlockAndNotify()

	return hashes, txD, err
}

// CheckMempoolAcceptance runs the checks that ProcessTransaction runs for a
// transaction submitted without orphan handling, without adding the
// transaction to the pool.  A positive maxFee rejects transactions that pay
// more fees than that.
//
// This function is safe for concurrent access.
func (mp *TxPool) CheckMempoolAcceptance(tx *bchutil.Tx, maxFee bchutil.Amount) (*MempoolAcceptResult, error) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()

	// Leave the transaction out of the hash cache, since it is not added.
	if hc := mp.cfg.HashCache; hc != nil && !hc.ContainsHashes(tx.Hash()) {
		defer hc.PurgeSigHashes(tx.Hash())
	}

	return mp.checkMempoolAcceptance(tx, true, false, maxFee, false)
}

// processOrphans is the internal function which implements the public
// ProcessOrphans.  See the comment for ProcessOrphans for more details.
//
// This function MUST be called with the mempool lock held (for writes).
func (mp *TxPool) processOrphans(acceptedTx *bchutil.Tx) []*TxDesc {
	var acceptedTxns []*TxDesc

	// Start with processing at least the passed transaction.
	processList := list.New()
	processList.PushBack(acceptedTx)
	for processList.Len() > 0 {
		// Pop the transaction to process from the front of the list.
		firstElement := processList.Remove(processList.Front())
		processItem := firstElement.(*bchutil.Tx)

		prevOut := wire.OutPoint{Hash: *processItem.Hash()}
		for txOutIdx := range processItem.MsgTx().TxOut {
			// Look up all orphans that redeem the output that is
			// now available.  This will typically only be one, but
			// it could be multiple if the orphan pool contains
			// double spends.  While it may seem odd that the orphan
			// pool would allow this since there can only possibly
			// ultimately be a single redeemer, it's important to
			// track it this way to prevent malicious actors from
			// being able to purposely constructing orphans that
			// would otherwise make outputs unspendable.
			//
			// Skip to the next available output if there are none.
			prevOut.Index = uint32(txOutIdx)
			orphans, exists := mp.orphansByPrev[prevOut]
			if !exists {
				continue
			}

			// Potentially accept an orphan into the tx pool.
			for _, tx := range orphans {
				missing, txD, err := mp.maybeAcceptTransaction(
					tx, true, false, 0)
				if err != nil {
					// The orphan is now invalid, so there
					// is no way any other orphans which
					// redeem any of its outputs can be
					// accepted.  Remove them, and try the
					// other orphans redeeming this output,
					// which may still be valid.
					mp.removeOrphan(tx, true)
					continue
				}

				// Transaction is still an orphan.  Try the next
				// orphan which redeems this output.
				if len(missing) > 0 {
					continue
				}

				// Transaction was accepted into the main pool.
				//
				// Add it to the list of accepted transactions
				// that are no longer orphans, remove it from
				// the orphan pool, and add it to the list of
				// transactions to process so any orphans that
				// depend on it are handled too.
				acceptedTxns = append(acceptedTxns, txD)
				mp.removeOrphan(tx, false)
				processList.PushBack(tx)

				// Only one transaction for this outpoint can be
				// accepted, so the rest are now double spends
				// and are removed later.
				break
			}
		}
	}

	// Recursively remove any orphans that also redeem any outputs redeemed
	// by the accepted transactions since those are now definitive double
	// spends.
	mp.removeOrphanDoubleSpends(acceptedTx)
	for _, txD := range acceptedTxns {
		mp.removeOrphanDoubleSpends(txD.Tx)
	}

	return acceptedTxns
}

// ProcessOrphans determines if there are any orphans which depend on the passed
// transaction hash (it is possible that they are no longer orphans) and
// potentially accepts them to the memory pool.  It repeats the process for the
// newly accepted transactions (to detect further orphans which may no longer be
// orphans) until there are no more.
//
// It returns a slice of transactions added to the mempool.  A nil slice means
// no transactions were moved from the orphan pool to the mempool.
//
// This function is safe for concurrent access.
func (mp *TxPool) ProcessOrphans(acceptedTx *bchutil.Tx) []*TxDesc {
	mp.mtx.Lock()
	acceptedTxns := mp.processOrphans(acceptedTx)
	mp.unlockAndNotify()

	return acceptedTxns
}

// ProcessTransaction is the main workhorse for handling insertion of new
// free-standing transactions into the memory pool.  It includes functionality
// such as rejecting duplicate transactions, ensuring transactions follow all
// rules, orphan transaction handling, and insertion into the memory pool.
//
// It returns a slice of transactions added to the mempool.  When the
// error is nil, the list will include the passed transaction itself along
// with any additional orphan transaactions that were added as a result of
// the passed one being accepted.
//
// The orphan pool is only treated as a source of duplicates when the caller
// allows orphans, which is to say for peer relay.  See the note on the
// maybeAcceptTransaction call below.
//
// This function is safe for concurrent access.
func (mp *TxPool) ProcessTransaction(tx *bchutil.Tx, allowOrphan bool, tag Tag) ([]*TxDesc, error) {
	return mp.ProcessTransactionWithMaxFee(tx, allowOrphan, tag, 0)
}

// ProcessTransactionWithMaxFee is like ProcessTransaction, but also rejects
// the transaction when it pays more fees than a positive maxFee.
//
// This function is safe for concurrent access.
func (mp *TxPool) ProcessTransactionWithMaxFee(tx *bchutil.Tx, allowOrphan bool, tag Tag, maxFee bchutil.Amount) ([]*TxDesc, error) {
	log.Tracef("Processing transaction %v", tx.Hash())

	// Protect concurrent access.
	mp.mtx.Lock()
	defer mp.unlockAndNotify()

	// Potentially accept the transaction to the memory pool.
	//
	// Duplicates are only rejected against the orphan pool for callers that
	// can themselves create orphans, meaning peer relay, where treating a
	// known orphan as a duplicate avoids re-requesting it.  Callers that do
	// not create orphans, the JSON-RPC and gRPC submit endpoints, must not
	// consult it: a transaction sitting in the orphan pool would otherwise be
	// reported as "already have transaction" even though it is absent from
	// the mempool, so a lookup for it immediately afterwards returns not
	// found.  Skipping the check lets those callers fall through to the
	// missing parent error below, which is both accurate and what BCHN
	// reports, since its mempool acceptance has no visibility into the orphan
	// map held by the P2P layer.
	missingParents, txD, err := mp.maybeAcceptTransaction(tx, true,
		allowOrphan, maxFee)
	if err != nil {
		return nil, err
	}

	if len(missingParents) == 0 {
		// Accept any orphan transactions that depend on this
		// transaction (they may no longer be orphans if all inputs
		// are now available) and repeat for those accepted
		// transactions until there are no more.
		newTxs := mp.processOrphans(tx)
		acceptedTxs := make([]*TxDesc, len(newTxs)+1)

		// Add the parent transaction first so remote nodes
		// do not add orphans.
		acceptedTxs[0] = txD
		copy(acceptedTxs[1:], newTxs)

		return acceptedTxs, nil
	}

	// The transaction is an orphan (has inputs missing).  Reject
	// it if the flag to allow orphans is not set.
	if !allowOrphan {
		// Only use the first missing parent transaction in
		// the error message.
		//
		// NOTE: RejectDuplicate is really not an accurate
		// reject code here, but it matches the reference
		// implementation and there isn't a better choice due
		// to the limited number of reject codes.  Missing
		// inputs is assumed to mean they are already spent
		// which is not really always the case.
		str := fmt.Sprintf("orphan transaction %v references "+
			"outputs of unknown or fully-spent "+
			"transaction %v", tx.Hash(), missingParents[0])
		return nil, txRuleError(wire.RejectDuplicate, str)
	}

	// Potentially add the orphan transaction to the orphan pool.
	err = mp.maybeAddOrphan(tx, tag)
	return nil, err
}

// Count returns the number of transactions in the main pool.  It does not
// include the orphan pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) Count() int {
	mp.mtx.RLock()
	count := len(mp.pool)
	mp.mtx.RUnlock()

	return count
}

// TxHashes returns a slice of hashes for all of the transactions in the memory
// pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) TxHashes() []*chainhash.Hash {
	mp.mtx.RLock()
	hashes := make([]*chainhash.Hash, len(mp.pool))
	i := 0
	for hash := range mp.pool {
		hashCopy := hash
		hashes[i] = &hashCopy
		i++
	}
	mp.mtx.RUnlock()

	return hashes
}

// txDescs returns a slice of descriptors for all the transactions in the pool.
// The descriptors are to be treated as read only.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) txDescs() []*TxDesc {
	descs := make([]*TxDesc, len(mp.pool))
	i := 0
	for _, desc := range mp.pool {
		descs[i] = desc
		i++
	}
	return descs
}

// TxDescs returns a slice of descriptors for all the transactions in the pool.
// The descriptors are to be treated as read only.
//
// This function is safe for concurrent access.
func (mp *TxPool) TxDescs() []*TxDesc {
	mp.mtx.RLock()
	descs := mp.txDescs()
	mp.mtx.RUnlock()

	return descs
}

// MiningDescs returns a slice of mining descriptors for all the transactions
// in the pool.
//
// This is part of the mining.TxSource interface implementation and is safe for
// concurrent access as required by the interface contract.
func (mp *TxPool) MiningDescs() []*mining.TxDesc {
	mp.mtx.RLock()
	descs := make([]*mining.TxDesc, len(mp.pool))
	i := 0
	for _, desc := range mp.pool {
		descs[i] = &desc.TxDesc
		i++
	}
	mp.mtx.RUnlock()

	return descs
}

// RawMempoolVerbose returns all of the entries in the mempool as a fully
// populated btcjson result.
//
// This function is safe for concurrent access.
func (mp *TxPool) RawMempoolVerbose() map[string]*btcjson.GetRawMempoolVerboseResult {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()

	result := make(map[string]*btcjson.GetRawMempoolVerboseResult,
		len(mp.pool))

	for _, desc := range mp.pool {
		// The priority fields are deprecated and left at 0.
		tx := desc.Tx
		mpd := &btcjson.GetRawMempoolVerboseResult{
			Size:    int32(tx.MsgTx().SerializeSize()),
			Fee:     bchutil.Amount(desc.Fee).ToBCH(),
			Time:    desc.Added.Unix(),
			Height:  int64(desc.Height),
			Depends: make([]string, 0),
		}
		for _, txIn := range tx.MsgTx().TxIn {
			hash := &txIn.PreviousOutPoint.Hash
			if mp.haveTransaction(hash) {
				mpd.Depends = append(mpd.Depends,
					hash.String())
			}
		}

		result[tx.Hash().String()] = mpd
	}

	return result
}

// LastUpdated returns the last time a transaction was added to or removed from
// the main pool.  It does not include the orphan pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) LastUpdated() time.Time {
	return time.Unix(mp.lastUpdated.Load(), 0)
}

// maxCompactBlockTxs returns the maximum number of transactions a compact
// block may describe.  It is the maximum block payload divided by the minimum
// transaction size, which bounds the number of transactions any block can
// hold.
func maxCompactBlockTxs() uint64 {
	return uint64(wire.MaxBlockPayload()) / blockchain.MinTransactionSize
}

// placePrefilledTxs validates the transaction count and the prefilled
// transactions of the passed compact block.  It returns a transaction slice
// sized to the block with each prefilled transaction at its position and nil
// in every other position.  Those positions are left for the short IDs, in
// order, and there is exactly one per short ID.
//
// The prefilled indexes are differentially encoded as described in BIP0152:
// the first index is absolute and each following index counts the positions
// between the previous prefilled transaction and this one.  An error is
// returned when the block describes no transactions or more than
// maxCompactBlockTxs, when a prefilled transaction is missing or empty, or
// when a decoded index exceeds 32 bits or points past the positions that the
// earlier prefilled transactions and the short IDs can fill.
func placePrefilledTxs(block *wire.MsgCmpctBlock) ([]*wire.MsgTx, error) {
	numShortIDs := uint64(len(block.ShortIDs))
	numTxs := numShortIDs + uint64(len(block.PrefilledTxs))
	if numTxs == 0 {
		return nil, errors.New("cmpctblock has no transactions")
	}
	if maxTxs := maxCompactBlockTxs(); numTxs > maxTxs {
		return nil, fmt.Errorf("cmpctblock has %d transactions which "+
			"exceeds the max of %d", numTxs, maxTxs)
	}

	txs := make([]*wire.MsgTx, numTxs)
	lastIndex := int64(-1)
	for i, ptx := range block.PrefilledTxs {
		if ptx == nil || ptx.Tx == nil ||
			(len(ptx.Tx.TxIn) == 0 && len(ptx.Tx.TxOut) == 0) {

			return nil, fmt.Errorf("cmpctblock prefilled tx %d is "+
				"empty", i)
		}

		// The previous index is at most math.MaxUint32, so adding a
		// uint32 plus one cannot overflow an int64.
		lastIndex += int64(ptx.Index) + 1
		if lastIndex > math.MaxUint32 {
			return nil, fmt.Errorf("cmpctblock prefilled tx %d index "+
				"overflows 32 bits", i)
		}

		// Each position before this one must be filled by one of the
		// i previous prefilled transactions or by a short ID.  Since
		// the decoded indexes strictly increase, this also keeps them
		// unique and within the transaction count.
		if uint64(lastIndex) > numShortIDs+uint64(i) {
			return nil, fmt.Errorf("cmpctblock prefilled tx %d index "+
				"%d is out of range", i, lastIndex)
		}
		txs[lastIndex] = ptx.Tx
	}
	return txs, nil
}

// DecodeCompressedBlock takes in a block interface and attempts to decode it
// according to know block compressing algorithms. If successful it returns
// the complete block.
func (mp *TxPool) DecodeCompressedBlock(iBlock interface{}) (*wire.MsgBlock, error) {
	mp.mtx.RLock()
	defer mp.mtx.RUnlock()

	switch block := iBlock.(type) {
	case *wire.MsgCmpctBlock:
		// First add all the prefilled txs.
		txs, err := placePrefilledTxs(block)
		if err != nil {
			return nil, err
		}
		msgBlock := wire.NewMsgBlock(&block.Header)
		msgBlock.Transactions = txs

		// Next we'll decode the short IDs.
		shortIDMap := make(map[[wire.ShortIDSize]byte]int)
		for i, sid := range block.ShortIDs {
			shortIDMap[sid] = i
		}
		recoveredTxs := make([]*wire.MsgTx, len(block.ShortIDs))

		// To calculate the siphash keys we need to append the little endian
		// nonce to the block header.
		var buf bytes.Buffer
		if err := block.Header.Serialize(&buf); err != nil {
			return nil, err
		}
		nonceBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(nonceBytes, block.Nonce)
		headerWithNonce := append(buf.Bytes(), nonceBytes...)

		// Hash the result once with sha256
		headerHash := sha256.Sum256(headerWithNonce)

		// The keys are the first two little endian uint64s in the resulting
		// byte array.
		key0 := binary.LittleEndian.Uint64(headerHash[0:8])
		key1 := binary.LittleEndian.Uint64(headerHash[8:16])

		calcShortID := func(txid chainhash.Hash) [wire.ShortIDSize]byte {
			sum64 := siphash.Hash(key0, key1, txid.CloneBytes())
			shortIDBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(shortIDBytes, sum64)
			var shortID [wire.ShortIDSize]byte
			copy(shortID[:], shortIDBytes[:wire.ShortIDSize])
			return shortID
		}

		// Iterate over the mempool and see if any txs match
		for txid, txdesc := range mp.pool {
			shortID := calcShortID(txid)
			index, ok := shortIDMap[shortID]
			if ok {
				recoveredTxs[index] = txdesc.Tx.MsgTx()
			}
		}

		// Iterate over the orphan map and see if any txs match
		for txid, orphan := range mp.orphans {
			shortID := calcShortID(txid)
			index, ok := shortIDMap[shortID]
			if ok {
				recoveredTxs[index] = orphan.tx.MsgTx()
			}
		}

		var pop *wire.MsgTx
		for i, tx := range msgBlock.Transactions {
			// This was a prefilled tx so we can continue
			if tx != nil {
				continue
			}
			// placePrefilledTxs leaves exactly one empty slot per short
			// ID, so this only guards that invariant.
			if len(recoveredTxs) == 0 {
				return nil, errors.New("cmpctblock has more empty " +
					"slots than short ids")
			}
			// Pop the first item of the recoveredTxs list
			pop, recoveredTxs = recoveredTxs[0], recoveredTxs[1:]
			msgBlock.Transactions[i] = pop
		}
		return msgBlock, nil
	default:
		return nil, errors.New("unknown block type")
	}
}

// New returns a new memory pool for validating and storing standalone
// transactions until they are mined into a block.
func New(cfg *Config) *TxPool {
	return &TxPool{
		cfg:            *cfg,
		pool:           make(map[chainhash.Hash]*TxDesc),
		orphans:        make(map[chainhash.Hash]*orphanTx),
		orphansByPrev:  make(map[wire.OutPoint]map[chainhash.Hash]*bchutil.Tx),
		nextExpireScan: time.Now().Add(orphanExpireScanInterval),
		outpoints:      make(map[wire.OutPoint]*bchutil.Tx),
		dsProofs:       newDSProofStorage(),
		txDSProofs:     make(map[chainhash.Hash]chainhash.Hash),
		nextReaddSeq:   math.MinInt64,
	}
}
