package indexers

import (
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// SlpGraphSearchDb manages slp token graphs for graph search and TODO: recently queried items
type SlpGraphSearchDb struct {
	sync.RWMutex
	db map[chainhash.Hash]*SlpTokenGraph
}

// NewSlpGraphSearchDb creates a new instance of SlpCache
func NewSlpGraphSearchDb() *SlpGraphSearchDb {
	return &SlpGraphSearchDb{
		db: make(map[chainhash.Hash]*SlpTokenGraph),
	}
}

// AddTokenGraph puts new items in a temporary cache with limited size
func (s *SlpGraphSearchDb) AddTokenGraph(tokenID *chainhash.Hash, item *SlpTokenGraph) {
	s.Lock()
	defer s.Unlock()

	s.db[*tokenID] = item
}

// GetTokenGraph gets items from the cache allowing concurrent read access without
// holding a lock on other readers
func (s *SlpGraphSearchDb) GetTokenGraph(tokenID *chainhash.Hash) *SlpTokenGraph {
	s.RLock()
	defer s.RUnlock()

	return s.db[*tokenID]
}
