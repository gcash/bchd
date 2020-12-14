package indexers

import (
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// SlpTokenGraph manages slp token graphs for graph search and TODO: recently queried items
type SlpTokenGraph struct {
	sync.RWMutex
	TokenID *chainhash.Hash
	graph   map[chainhash.Hash]*wire.MsgTx
}

// NewSlpTokenGraph creates a new instance of SlpCache
func NewSlpTokenGraph(tokenID *chainhash.Hash) *SlpTokenGraph {
	return &SlpTokenGraph{
		graph:   make(map[chainhash.Hash]*wire.MsgTx),
		TokenID: tokenID,
	}
}

// Size gets the current size of the token graph
func (s *SlpTokenGraph) Size() int {
	s.RLock()
	defer s.RUnlock()

	return len(s.graph)
}

// AddTxn puts new graph items in a temporary cache with limited size
func (s *SlpTokenGraph) AddTxn(hash *chainhash.Hash, item *wire.MsgTx) {
	s.Lock()
	defer s.Unlock()

	s.graph[*hash] = item
}

// GetTxn gets graph items allowing concurrent read access without
// holding a lock on other readers
func (s *SlpTokenGraph) GetTxn(hash *chainhash.Hash) *wire.MsgTx {
	s.RLock()
	defer s.RUnlock()

	return s.graph[*hash]
}
