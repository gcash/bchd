package indexers

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gcash/bchd/blockchain/slpgraphsearch"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchutil"
)

// SlpCache to manage slp index mempool items and recently queried items
type SlpCache struct {
	sync.RWMutex
	tempEntries    map[chainhash.Hash]*SlpIndexEntry
	mempoolEntries map[chainhash.Hash]*SlpIndexEntry
	maxTempEntries int
	graphSearchDb  *slpgraphsearch.Db
}

// NewSlpCache creates a new instance of SlpCache
func NewSlpCache(maxTempEntries int) *SlpCache {
	return &SlpCache{
		mempoolEntries: make(map[chainhash.Hash]*SlpIndexEntry),
		tempEntries:    make(map[chainhash.Hash]*SlpIndexEntry, maxTempEntries),
		maxTempEntries: maxTempEntries,
	}
}

// AddTemp puts new items in a temporary cache with limited size
func (s *SlpCache) AddTemp(hash *chainhash.Hash, item *SlpIndexEntry) {
	s.Lock()
	defer s.Unlock()

	// Remove a random entry from the map.  For most compilers, Go's
	// range statement iterates starting at a random item although
	// that is not 100% guaranteed by the spec.
	if len(s.tempEntries) > s.maxTempEntries {
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

// AddMempoolItem puts new items in the mempool cache
func (s *SlpCache) AddMempoolItem(hash *chainhash.Hash, item *SlpIndexEntry) {
	entry := s.GetMempoolItem(hash)
	if entry != nil {
		return
	}

	s.Lock()
	defer s.Unlock()
	s.mempoolEntries[*hash] = item
}

// RemoveMempoolItems a list of items from a list
func (s *SlpCache) RemoveMempoolItems(txs []*bchutil.Tx) {
	s.Lock()
	defer s.Unlock()
	for _, tx := range txs {
		delete(s.mempoolEntries, tx.MsgTx().TxHash())
	}
}

func (s *SlpCache) remove(hash *chainhash.Hash) {
	s.Lock()
	defer s.Unlock()
	delete(s.tempEntries, *hash)
	delete(s.mempoolEntries, *hash)
}

// Get gets items from the cache allowing concurrent read access without
func (s *SlpCache) Get(hash *chainhash.Hash) *SlpIndexEntry {
	s.RLock()
	defer s.RUnlock()

	entry := s.mempoolEntries[*hash]
	if entry == nil {
		entry = s.tempEntries[*hash]
	}

	return entry
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
	s.Lock()
	defer s.Unlock()

	if s.graphSearchDb == nil {
		s.graphSearchDb = slpgraphsearch.NewDb()
	}

	dbState := atomic.LoadUint32(&s.graphSearchDb.State)
	if dbState == 1 {
		return s.graphSearchDb, fmt.Errorf("graph search db is loaded but is not ready, waiting for the next block")
	} else if dbState == 0 {
		return s.graphSearchDb, fmt.Errorf("graph search db is loading, please try again in a few minutes")
	}

	return s.graphSearchDb, nil
}
