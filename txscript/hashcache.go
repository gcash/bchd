// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"errors"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// TxSigHashes houses the partial set of sighashes introduced within BIP0143.
// This partial set of sighashes may be re-used within each input across a
// transaction when validating all inputs. As a result, validation complexity
// for SigHashAll can be reduced by a polynomial factor.
type TxSigHashes struct {
	HashPrevOuts  chainhash.Hash
	HashSequence  chainhash.Hash
	HashOutputs   chainhash.Hash
	HashUTXOS     chainhash.Hash
	tokenDataList [][]byte
	hasUtxos      bool

	// missingUtxos lists the inputs whose spent outputs were not in the
	// UtxoCache the spent outputs were added from, so signing can refuse
	// the signatures that would leave them out.
	missingUtxos []int
}

// NewTxSigHashes computes, and returns the cached sighashes of the given
// transaction.
func NewTxSigHashes(tx *wire.MsgTx) *TxSigHashes {
	return &TxSigHashes{
		HashPrevOuts: calcHashPrevOuts(tx),
		HashSequence: calcHashSequence(tx),
		HashOutputs:  calcHashOutputs(tx),
		// HashUTXOS:    calcHashUtxos(tx),
	}
}

// AddTxSigHashUtxoFromUtxoCache adds the hash of the outputs spent by the
// transaction and their token data to the sighashes.
func (txSighashes *TxSigHashes) AddTxSigHashUtxoFromUtxoCache(tx *wire.MsgTx, utxoCache *UtxoCache) {
	hash := calcHashUtxos(tx, utxoCache)
	txSighashes.HashUTXOS = hash
	txSighashes.tokenDataList, txSighashes.missingUtxos =
		calUtxoTokenData(tx, utxoCache)
	txSighashes.hasUtxos = true
}

// hasAllUtxos returns whether the sighashes include the outputs spent by all
// the inputs of the transaction, as a hash with SigHashUTXO covers them all.
func (txSighashes *TxSigHashes) hasAllUtxos() bool {
	return txSighashes.hasUtxos && len(txSighashes.missingUtxos) == 0
}

// WithUtxos returns a copy of the sighashes with the outputs spent by the
// transaction added.
func (txSighashes *TxSigHashes) WithUtxos(tx *wire.MsgTx, utxoCache *UtxoCache) *TxSigHashes {
	sigHashes := *txSighashes
	sigHashes.AddTxSigHashUtxoFromUtxoCache(tx, utxoCache)
	return &sigHashes
}

// HashCache houses a set of partial sighashes keyed by txid. The set of partial
// sighashes are those introduced within BIP0143 by the new more efficient
// sighash digest calculation algorithm. Using this threadsafe shared cache,
// multiple goroutines can safely re-use the pre-computed partial sighashes
// speeding up validation time amongst all inputs found within a block.
type HashCache struct {
	sigHashes  map[chainhash.Hash]*TxSigHashes
	maxEntries uint

	sync.RWMutex
}

// NewHashCache returns a new instance of the HashCache given a maximum number
// of entries which may exist within it at anytime.  Random entries are
// evicted to make room for new entries that would cause the number of
// entries in the cache to exceed the max.
func NewHashCache(maxSize uint) *HashCache {
	return &HashCache{
		sigHashes:  make(map[chainhash.Hash]*TxSigHashes, maxSize),
		maxEntries: maxSize,
	}
}

// AddSigHashes computes, then adds the partial sighashes for the passed
// transaction, and returns them.  When the cache is full, a random entry is
// evicted to make room, as in SigCache.  Entries are purged once a block
// holding their transactions passes validation, but those of transactions
// that fail validation or leave the memory pool unmined stay until evicted.
func (h *HashCache) AddSigHashes(tx *wire.MsgTx) *TxSigHashes {
	sigHashes := NewTxSigHashes(tx)
	if h.maxEntries == 0 {
		return sigHashes
	}
	txid := tx.TxHash()

	h.Lock()
	if uint(len(h.sigHashes)+1) > h.maxEntries {
		for evict := range h.sigHashes {
			delete(h.sigHashes, evict)
			break
		}
	}
	h.sigHashes[txid] = sigHashes
	h.Unlock()
	return sigHashes
}

// ContainsHashes returns true if the partial sighashes for the passed
// transaction currently exist within the HashCache, and false otherwise.
func (h *HashCache) ContainsHashes(txid *chainhash.Hash) bool {
	h.RLock()
	_, found := h.sigHashes[*txid]
	h.RUnlock()

	return found
}

// GetSigHashes possibly returns the previously cached partial sighashes for
// the passed transaction. This function also returns an additional boolean
// value indicating if the sighashes for the passed transaction were found to
// be present within the HashCache.
func (h *HashCache) GetSigHashes(txid *chainhash.Hash) (*TxSigHashes, bool) {
	h.RLock()
	item, found := h.sigHashes[*txid]
	h.RUnlock()

	return item, found
}

// PurgeSigHashes removes all partial sighashes from the HashCache belonging to
// the passed transaction.
func (h *HashCache) PurgeSigHashes(txid *chainhash.Hash) {
	h.Lock()
	delete(h.sigHashes, *txid)
	h.Unlock()
}

// UtxoCache houses the utxos (scriptPubkey and value) for each input index
// in a single transaction. We use this class for the native introspection
// opcodes instead of the UtxoViewpoint class from the blockchain package to
// avoid circular imports.
type UtxoCache struct {
	utxos map[int]wire.TxOut

	sync.RWMutex
}

// NewUtxoCache returns a new instance of the UtxoCache.
func NewUtxoCache() *UtxoCache {
	return &UtxoCache{
		utxos: make(map[int]wire.TxOut),
	}
}

// AddEntry adds a utxo entry for the given input index.
func (u *UtxoCache) AddEntry(i int, output wire.TxOut) {
	u.Lock()
	u.utxos[i] = output
	u.Unlock()
}

// GetEntry returns the utxo entry for the given input index.
func (u *UtxoCache) GetEntry(i int) (wire.TxOut, error) {
	u.RLock()
	utxo, ok := u.utxos[i]
	u.RUnlock()
	if !ok {
		return wire.TxOut{}, errors.New("not found")
	}
	return utxo, nil
}
