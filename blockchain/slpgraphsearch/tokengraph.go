package slpgraphsearch

import (
	"errors"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// tokenGraph manages slp token graphs for graph search and TODO: recently queried items
type tokenGraph struct {
	sync.RWMutex
	TokenID *chainhash.Hash
	graph   map[chainhash.Hash]*wire.MsgTx
}

// newSlpTokenGraph creates a new instance of SlpCache
func newTokenGraph(tokenID *chainhash.Hash) *tokenGraph {
	return &tokenGraph{
		graph:   make(map[chainhash.Hash]*wire.MsgTx),
		TokenID: tokenID,
	}
}

// size gets the current size of the token graph
func (g *tokenGraph) size() int {
	return len(g.graph)
}

// getTxn gets graph items allowing concurrent read access without
func (g *tokenGraph) getTxn(hash *chainhash.Hash) *wire.MsgTx {
	g.RLock()
	defer g.RUnlock()

	return g.graph[*hash]
}

// addTxn puts new graph items in a token graph
func (g *tokenGraph) addTxn(tx *wire.MsgTx) error {
	g.Lock()
	defer g.Unlock()
	size0 := g.size()
	g.graph[tx.TxHash()] = tx
	if g.size() < size0 {
		return errors.New("token graph db should never get smaller")
	}
	return nil
}

// removeTxn removes a transaction from the graph
func (g *tokenGraph) removeTxn(tx *wire.MsgTx) error {
	g.Lock()
	defer g.Unlock()

	if _, ok := g.graph[tx.TxHash()]; !ok {
		return errors.New("transaction doesn't exist in graph")
	}
	delete(g.graph, tx.TxHash())
	return nil
}
