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

// SlpCache to manage slp index mempool txn and token metadata items and recently queried items
type SlpCache struct {
	sync.RWMutex
	maxEntries          int
	mempoolSlpTxEntries map[chainhash.Hash]*SlpTxEntry
	slpTxEntries        gcache.Cache
	tokenMetadata       gcache.Cache
}

// InitSlpCache creates a new instance of SlpCache
func InitSlpCache(maxEntries int) *SlpCache {
	return &SlpCache{
		maxEntries:          maxEntries,
		mempoolSlpTxEntries: make(map[chainhash.Hash]*SlpTxEntry),
		slpTxEntries:        gcache.New(maxEntries).LRU().Build(),
		tokenMetadata:       gcache.New(maxEntries).LRU().Build(),
	}
}

// AddSlpTxEntry puts new items in a temporary cache with limited size
func (s *SlpCache) AddSlpTxEntry(hash *chainhash.Hash, item SlpTxEntry) {
	s.slpTxEntries.Set(*hash, &item)
}

// AddMempoolSlpTxEntry puts new items in the mempool cache
func (s *SlpCache) AddMempoolSlpTxEntry(hash *chainhash.Hash, item SlpTxEntry) {
	if _, ok := s.GetSlpTxEntry(hash); ok {
		return
	}

	s.Lock()
	defer s.Unlock()
	s.mempoolSlpTxEntries[*hash] = &item
	s.slpTxEntries.Set(*hash, &item)
}

// GetSlpTxEntry gets tx entry items from the cache
func (s *SlpCache) GetSlpTxEntry(hash *chainhash.Hash) (SlpTxEntry, bool) {
	s.RLock()
	defer s.RUnlock()

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

// AddTempTokenMetadata puts token metadata into cache with a limited size
func (s *SlpCache) AddTempTokenMetadata(item TokenMetadata) {
	s.Lock()
	defer s.Unlock()

	s.tokenMetadata.Set(*item.TokenID, &item)
}

// GetTokenMetadata gets token metadata from the cache
func (s *SlpCache) GetTokenMetadata(hash *chainhash.Hash) (TokenMetadata, bool) {
	if entry, err := s.tokenMetadata.Get(*hash); err != nil {
		if val, ok := entry.(*TokenMetadata); ok {
			return *val, true
		}
	}
	return TokenMetadata{}, false
}

// RemoveTokenMetadata removes a token metadata item from cache
func (s *SlpCache) RemoveTokenMetadata(hash chainhash.Hash) {
	s.tokenMetadata.Remove(hash)
}

// RemoveMempoolSlpTxItems is called on block events to remove mempool transaction items and
// also we clear the tempTokenMetadata to avoid corrupt mint baton state from double spends
func (s *SlpCache) RemoveMempoolSlpTxItems(txs []*bchutil.Tx) {
	s.Lock()
	defer s.Unlock()

	for _, tx := range txs {
		hash := tx.MsgTx().TxHash()
		delete(s.mempoolSlpTxEntries, hash)
	}
}
