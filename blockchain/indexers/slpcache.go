// Copyright (c) 2020-2021 Simple Ledger, Inc.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"sync"

	"github.com/bluele/gcache"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchutil"
)

// SlpCache to manage slp index mempool inventory and recently queried slp index entries and
// token metadata items. The RWMutex lock is held for mempoolSlpTxEntries and
// mempoolTokenMetadata operations since gcache.Cache holds its own locks.
type SlpCache struct {
	sync.RWMutex
	maxEntries          int
	mempoolSlpTxEntries map[chainhash.Hash]*SlpTxEntry
	slpTxEntries        gcache.Cache
	tokenMetadata       gcache.Cache

	// mempoolTokenMetadata maps the token ID of each token whose cached
	// metadata a memory pool transaction set to that transaction.
	mempoolTokenMetadata map[chainhash.Hash]chainhash.Hash
}

// InitSlpCache creates a new instance of SlpCache
func InitSlpCache(maxEntries int) *SlpCache {
	return &SlpCache{
		maxEntries:           maxEntries,
		mempoolSlpTxEntries:  make(map[chainhash.Hash]*SlpTxEntry),
		slpTxEntries:         gcache.New(maxEntries).LRU().Build(),
		tokenMetadata:        gcache.New(maxEntries).LRU().Build(),
		mempoolTokenMetadata: make(map[chainhash.Hash]chainhash.Hash),
	}
}

// AddSlpTxEntry puts new items in a temporary cache with limited size
func (s *SlpCache) AddSlpTxEntry(hash *chainhash.Hash, item SlpTxEntry) error {
	return s.slpTxEntries.Set(*hash, &item)
}

// AddMempoolSlpTxEntry puts new items in the mempool cache
func (s *SlpCache) AddMempoolSlpTxEntry(hash *chainhash.Hash, item SlpTxEntry) {
	if _, ok := s.GetSlpTxEntry(hash); ok {
		return
	}

	s.Lock()
	defer s.Unlock()
	s.mempoolSlpTxEntries[*hash] = &item
	_ = s.slpTxEntries.Set(*hash, &item)
}

// GetSlpTxEntry gets tx entry items from the cache
func (s *SlpCache) GetSlpTxEntry(hash *chainhash.Hash) (SlpTxEntry, bool) {
	s.RLock()
	defer s.RUnlock()

	// The purpose of checking mempoolSlpTxEntries first is to guarantee
	// that a mempool txn is kept around in cache and isn't prematurely removed
	// by the LRU cache algorithm.
	if entry, ok := s.mempoolSlpTxEntries[*hash]; ok {
		return *entry, ok
	}

	if entry, err := s.slpTxEntries.Get(*hash); err == nil {
		if val, ok := entry.(*SlpTxEntry); ok {
			return *val, true
		}
	}
	return SlpTxEntry{}, false
}

// cachedTokenMetadata is token metadata in the cache.  Metadata read from the
// database holds the slp index tip of the snapshot it was read from, and is
// only served to readers of a snapshot with the same tip, as a block that
// moved a mint baton may have committed since.  Metadata from transactions in
// the memory pool holds none.
type cachedTokenMetadata struct {
	tm  TokenMetadata
	tip *chainhash.Hash
}

// AddTempTokenMetadata puts token metadata that the memory pool transaction
// from set into cache with a limited size.  It is removed once the
// transaction leaves the memory pool.
func (s *SlpCache) AddTempTokenMetadata(item TokenMetadata, from chainhash.Hash) error {
	s.Lock()
	defer s.Unlock()

	s.mempoolTokenMetadata[*item.TokenID] = from
	return s.tokenMetadata.Set(*item.TokenID, &cachedTokenMetadata{tm: item})
}

// AddTokenMetadataAt puts token metadata read from a database snapshot whose
// slp index tip is tip into the cache.
func (s *SlpCache) AddTokenMetadataAt(item TokenMetadata, tip chainhash.Hash) error {
	return s.tokenMetadata.Set(*item.TokenID,
		&cachedTokenMetadata{tm: item, tip: &tip})
}

// GetTokenMetadata gets token metadata from the cache for a reader of a
// database snapshot whose slp index tip is tip, or nil if not known.  Metadata
// read from a snapshot with another tip is not returned.
func (s *SlpCache) GetTokenMetadata(hash *chainhash.Hash, tip *chainhash.Hash) (TokenMetadata, bool) {
	if entry, err := s.tokenMetadata.Get(*hash); err == nil {
		if val, ok := entry.(*cachedTokenMetadata); ok &&
			(val.tip == nil || (tip != nil && *val.tip == *tip)) {

			return val.tm, true
		}
	}
	return TokenMetadata{}, false
}

// RemoveTokenMetadata removes a token metadata item from cache
func (s *SlpCache) RemoveTokenMetadata(hash chainhash.Hash) {
	s.Lock()
	defer s.Unlock()

	delete(s.mempoolTokenMetadata, hash)
	s.tokenMetadata.Remove(hash)
}

// RemoveMempoolSlpTxItems is called on block events to remove the mempool
// transaction items of the block's transactions, and the token metadata they
// set in the memory pool, which the index now has.
func (s *SlpCache) RemoveMempoolSlpTxItems(txs []*bchutil.Tx) {
	s.Lock()
	defer s.Unlock()

	for _, tx := range txs {
		hash := tx.Hash()
		entry, ok := s.mempoolSlpTxEntries[*hash]
		if !ok {
			continue
		}
		delete(s.mempoolSlpTxEntries, *hash)
		if from, ok := s.mempoolTokenMetadata[entry.TokenIDHash]; ok &&
			from == *hash {

			delete(s.mempoolTokenMetadata, entry.TokenIDHash)
			s.tokenMetadata.Remove(entry.TokenIDHash)
		}
	}
}

// RemoveMempoolSlpTxsNotIn removes the mempool transaction items, and the
// token metadata, of the transactions that inPool reports are no longer in the
// memory pool.  A transaction that left it without being confirmed, as when a
// conflicting one was, may have put a mint baton on an output that does not
// exist.
func (s *SlpCache) RemoveMempoolSlpTxsNotIn(inPool func(*chainhash.Hash) bool) {
	s.Lock()
	defer s.Unlock()

	for hash := range s.mempoolSlpTxEntries {
		if !inPool(&hash) {
			delete(s.mempoolSlpTxEntries, hash)
		}
	}
	for tokenID, from := range s.mempoolTokenMetadata {
		if !inPool(&from) {
			delete(s.mempoolTokenMetadata, tokenID)
			s.tokenMetadata.Remove(tokenID)
		}
	}
}

// GetMempoolItem gets items from only mempool entries
func (s *SlpCache) GetMempoolItem(hash *chainhash.Hash) *SlpTxEntry {
	s.RLock()
	defer s.RUnlock()

	return s.mempoolSlpTxEntries[*hash]
}

// MempoolSize returns the size of the slp mempool cache
func (s *SlpCache) MempoolSize() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.mempoolSlpTxEntries)
}

// ForEachMempoolItem provides thread-safe access to all mempool entries
func (s *SlpCache) ForEachMempoolItem(fnc func(hash *chainhash.Hash, entry *SlpTxEntry) error) error {
	s.RLock()
	defer s.RUnlock()

	for k, v := range s.mempoolSlpTxEntries {
		err := fnc(&k, v)
		if err != nil {
			return err
		}
	}

	return nil
}
