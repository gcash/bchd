// Copyright (c) 2020-2021 Simple Ledger, Inc.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"fmt"
	"sync"

	"github.com/gcash/bchd/blockchain/slpgraphsearch"
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
	graphSearchDb     *slpgraphsearch.Db
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

// GetTxEntry gets tx entry items from the cache
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

// GetTokenMetadata gets token metadata from the cache
func (s *SlpCache) GetTokenMetadata(hash chainhash.Hash) *TokenMetadata {
	s.RLock()
	defer s.RUnlock()

	return s.tempTokenMetadata[hash]
}

// RemoveTokenMetadata removes a token metadata item from cache
func (s *SlpCache) RemoveTokenMetadata(hash chainhash.Hash) {
	s.Lock()
	defer s.Unlock()

	delete(s.tempTokenMetadata, hash)
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
}

// GetMempoolItem gets items from only mempool entries
func (s *SlpCache) GetMempoolItem(hash *chainhash.Hash) *SlpIndexEntry {
	s.RLock()
	defer s.RUnlock()

	return s.mempoolEntries[*hash]
}

// MempoolSize returns the size of the slp mempool cache
func (s *SlpCache) MempoolSize() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.mempoolEntries)
}

// ForEachMempoolItem provides thread-safe access to all mempool entries
func (s *SlpCache) ForEachMempoolItem(fnc func(hash *chainhash.Hash, entry *SlpIndexEntry) error) error {
	s.RLock()
	defer s.RUnlock()

	for k, v := range s.mempoolEntries {
		err := fnc(&k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetGraphSearchDb retrieves the graph search DB
func (s *SlpCache) GetGraphSearchDb() (*slpgraphsearch.Db, error) {

	s.RLock()
	if s.graphSearchDb != nil {
		dbState := s.graphSearchDb.State
		if dbState == 1 {
			s.RUnlock()
			return s.graphSearchDb, fmt.Errorf("graph search db is loaded but is not ready, waiting for the next block")
		} else if dbState == 0 {
			s.RUnlock()
			return s.graphSearchDb, fmt.Errorf("graph search db is loading, please try again in a few minutes")
		}
	}
	s.RUnlock()

	s.Lock()
	defer s.Unlock()

	if s.graphSearchDb != nil {
		dbState := s.graphSearchDb.State
		if dbState == 1 {
			s.RUnlock()
			return s.graphSearchDb, fmt.Errorf("graph search db is loaded but is not ready, waiting for the next block")
		} else if dbState == 0 {
			s.RUnlock()
			return s.graphSearchDb, fmt.Errorf("graph search db is loading, please try again in a few minutes")
		}
	}

	s.graphSearchDb = slpgraphsearch.NewDb()
	return s.graphSearchDb, nil
}
