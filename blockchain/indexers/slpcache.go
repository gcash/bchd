// Copyright (c) 2020 Simple Ledger, Inc.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchutil"
)

// SlpCache to manage slp index mempool txn and token metadata items and recently queried items
type SlpCache struct {
	sync.RWMutex
	maxEntries        int
	tempEntries       map[chainhash.Hash]*SlpIndexEntry
	mempoolEntries    map[chainhash.Hash]*SlpIndexEntry
	tempTokenMetadata map[chainhash.Hash]*TokenMetadata
}

// NewSlpCache creates a new instance of SlpCache
func NewSlpCache(maxEntries int) *SlpCache {
	return &SlpCache{
		maxEntries:        maxEntries,
		tempEntries:       make(map[chainhash.Hash]*SlpIndexEntry, maxEntries),
		mempoolEntries:    make(map[chainhash.Hash]*SlpIndexEntry),
		tempTokenMetadata: make(map[chainhash.Hash]*TokenMetadata, maxEntries),
	}
}

// AddTempEntry puts new items in a temporary cache with limited size
func (s *SlpCache) AddTempEntry(hash *chainhash.Hash, item *SlpIndexEntry) {
	s.Lock()
	defer s.Unlock()

	// Remove a random entry from the map.  For most compilers, Go's
	// range statement iterates starting at a random item although
	// that is not 100% guaranteed by the spec.
	if len(s.tempEntries) > s.maxEntries {
		for txHash := range s.tempEntries {
			delete(s.tempEntries, txHash)
			break
		}
	}
	_, ok := s.mempoolEntries[*hash]
	if !ok {
		s.tempEntries[*hash] = item
	}
}

// AddMempoolEntry puts new items in the mempool cache
func (s *SlpCache) AddMempoolEntry(hash *chainhash.Hash, item *SlpIndexEntry) {
	entry := s.GetTxEntry(hash)
	if entry != nil {
		return
	}

	s.Lock()
	defer s.Unlock()
	s.mempoolEntries[*hash] = item
}

// GetTxEntry gets tx entry items from the cache allowing concurrent read access without
// holding a lock on other readers
func (s *SlpCache) GetTxEntry(hash *chainhash.Hash) *SlpIndexEntry {
	s.RLock()
	defer s.RUnlock()

	entry := s.mempoolEntries[*hash]
	if entry == nil {
		entry = s.tempEntries[*hash]
	}

	return entry
}

// AddTempTokenMetadata puts token metadata into cache with a limited size
func (s *SlpCache) AddTempTokenMetadata(item *TokenMetadata) {
	s.Lock()
	defer s.Unlock()

	// Remove a random entry from the map.  For most compilers, Go's
	// range statement iterates starting at a random item although
	// that is not 100% guaranteed by the spec.
	if len(s.tempTokenMetadata) > s.maxEntries {
		for txHash := range s.tempTokenMetadata {
			delete(s.tempTokenMetadata, txHash)
			break
		}
	}
	s.tempTokenMetadata[*item.TokenID] = item
}

// GetTokenMetadata gets token metadata from the cache allowing concurrent read access
// without holding a lock on other readers
func (s *SlpCache) GetTokenMetadata(hash *chainhash.Hash) *TokenMetadata {
	s.RLock()
	defer s.RUnlock()

	return s.tempTokenMetadata[*hash]
}

// RemoveMempoolItems is called on block events to remove mempool transaction items and
// also we clear the tempTokenMetadata to avoid corrupt mint baton state from double spends
func (s *SlpCache) RemoveMempoolItems(txs []*bchutil.Tx) {
	s.Lock()
	defer s.Unlock()
	for _, tx := range txs {
		hash := tx.MsgTx().TxHash()
		delete(s.mempoolEntries, hash)
	}
	s.tempTokenMetadata = make(map[chainhash.Hash]*TokenMetadata, s.maxEntries)
}
