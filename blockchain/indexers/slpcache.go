package indexers

import (
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchutil"
)

// SlpCache to manage slp index mempool items and recently queried items
type SlpCache struct {
	sync.RWMutex
	tempEntries    map[chainhash.Hash]*SlpIndexEntry
	mempoolEntries map[chainhash.Hash]*SlpIndexEntry
	maxEntries     int
}

// NewSlpCache creates a new instance of SlpCache
func NewSlpCache(maxEntries int) *SlpCache {
	return &SlpCache{
		mempoolEntries: make(map[chainhash.Hash]*SlpIndexEntry),
		tempEntries:    make(map[chainhash.Hash]*SlpIndexEntry, maxEntries),
		maxEntries:     maxEntries,
	}
}

// AddTemp puts new items in a temporary cache with limited size
func (s *SlpCache) AddTemp(hash *chainhash.Hash, item *SlpIndexEntry) {
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

// AddMempoolItem puts new items in the mempool cache
func (s *SlpCache) AddMempoolItem(hash *chainhash.Hash, item *SlpIndexEntry) {
	entry := s.Get(hash)
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
// holding a lock on other readers
func (s *SlpCache) Get(hash *chainhash.Hash) *SlpIndexEntry {
	s.RLock()
	defer s.RUnlock()

	entry := s.mempoolEntries[*hash]
	if entry == nil {
		entry = s.tempEntries[*hash]
	}

	return entry
}
