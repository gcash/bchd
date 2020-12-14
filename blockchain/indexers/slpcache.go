package indexers

import (
	"errors"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// SlpCache to manage slp index mempool items and recently queried items
type SlpCache struct {
	sync.RWMutex
	tempEntries      map[chainhash.Hash]*SlpIndexEntry
	mempoolEntries   map[chainhash.Hash]*SlpIndexEntry
	maxTempEntries   int
	graphSearchDb    *SlpGraphSearchDb
	tmpTxnCacheForGs map[chainhash.Hash]*wire.MsgTx
}

// NewSlpCache creates a new instance of SlpCache
func NewSlpCache(maxTempEntries int) *SlpCache {
	return &SlpCache{
		mempoolEntries:   make(map[chainhash.Hash]*SlpIndexEntry),
		tempEntries:      make(map[chainhash.Hash]*SlpIndexEntry, maxTempEntries),
		maxTempEntries:   maxTempEntries,
		tmpTxnCacheForGs: make(map[chainhash.Hash]*wire.MsgTx),
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

// GetMempoolItem gets items from only mempool entries
func (s *SlpCache) GetMempoolItem(hash *chainhash.Hash) *SlpIndexEntry {
	s.RLock()
	defer s.RUnlock()

	return s.mempoolEntries[*hash]
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

// SetGraphSearchDb provides a safe way to only have the db set only one time
func (s *SlpCache) SetGraphSearchDb(db *SlpGraphSearchDb) error {
	s.Lock()
	defer s.Unlock()

	if s.graphSearchDb != nil {
		return errors.New("slp graph search db is already set")
	}
	s.tmpTxnCacheForGs = nil
	s.graphSearchDb = db
	return nil
}

// GetGraphSearchDb retrieves the graph search DB
func (s *SlpCache) GetGraphSearchDb() *SlpGraphSearchDb {
	s.RLock()
	defer s.RUnlock()

	return s.graphSearchDb
}

// AddCachedTransactionForGs adds MsgTx items during graph search startup phase
func (s *SlpCache) AddCachedTransactionForGs(tx *wire.MsgTx) error {
	s.Lock()
	defer s.Unlock()

	if s.graphSearchDb != nil {
		return errors.New("this cache is not available after slp graph search has been loaded")
	}
	s.tmpTxnCacheForGs[tx.TxHash()] = tx
	return nil
}

// GetCachedTransactionForGs from temporary transaction cache during slp graph search
func (s *SlpCache) GetCachedTransactionForGs(hash *chainhash.Hash) (*wire.MsgTx, error) {
	s.RLock()
	defer s.RUnlock()

	if s.graphSearchDb != nil {
		return nil, errors.New("this cache is not available after slp graph search has been loaded")
	}
	return s.tmpTxnCacheForGs[*hash], nil
}
