package slpgraphsearch

import (
	"errors"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// TokenGraph manages slp token graphs for graph search and TODO: recently queried items
type TokenGraph struct {
	sync.RWMutex
	TokenID *chainhash.Hash
	graph   map[chainhash.Hash]*wire.MsgTx
}

// newSlpTokenGraph creates a new instance of SlpCache
func newTokenGraph(tokenID *chainhash.Hash) *TokenGraph {
	return &TokenGraph{
		graph:   make(map[chainhash.Hash]*wire.MsgTx),
		TokenID: tokenID,
	}
}

// size gets the current size of the token graph
func (g *TokenGraph) size() int {
	return len(g.graph)
}

// addTxn puts new graph items in a token graph
func (g *TokenGraph) addTxn(tx *wire.MsgTx) error {
	g.Lock()
	defer g.Unlock()
	size0 := g.size()
	g.graph[tx.TxHash()] = tx
	if g.size() < size0 {
		return errors.New("token graph db should never get smaller")
	}
	return nil
}

// getTxn gets graph items allowing concurrent read access without
func (g *TokenGraph) getTxn(hash *chainhash.Hash) *wire.MsgTx {
	g.RLock()
	defer g.RUnlock()

	return g.graph[*hash]
}
