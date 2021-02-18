package slpgraphsearch

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/simpleledgerinc/goslp"
)

// Db manages slp token graphs for graph search and TODO: recently queried items
type Db struct {
	sync.RWMutex
	graphs map[chainhash.Hash]*tokenGraph
	state  uint32 // 0 = initial load incomplete, 1 = initial load complete, 2 = block found after load completed
}

// NewDb creates a new instance of SlpCache
func NewDb() *Db {
	return &Db{
		graphs: make(map[chainhash.Hash]*tokenGraph),
		state:  0,
	}
}

// IsLoaded indicates the db is initially loaded and can be used internally
func (gs *Db) IsLoaded() bool {
	gs.RLock()
	defer gs.RUnlock()

	return gs.state > 0
}

// IsReady indicates the db is loaded and ready for client queries
func (gs *Db) IsReady() bool {
	gs.RLock()
	defer gs.RUnlock()

	return gs.state > 1
}

// SetLoaded allows external callers to determine when all of the graph search db has been loaded
func (gs *Db) SetLoaded() error {
	gs.RLock()
	state := gs.state
	gs.RUnlock()

	if state == 1 {
		return nil
	}

	if state == 0 {
		gs.Lock()
		defer gs.Unlock()

		gs.state++
		return nil
	}
	return fmt.Errorf("slp gs db was not set to loaded with current state is %s", fmt.Sprint(gs.state))
}

// SetReady allows external callers to determine when the graph search db is ready for use
func (gs *Db) SetReady() error {
	gs.RLock()
	state := gs.state
	gs.RUnlock()

	if state == 2 {
		return nil
	}

	if state == 1 {
		gs.Lock()
		defer gs.Unlock()

		gs.state++
		return nil
	}
	return fmt.Errorf("slp gs db was not set to ready with current state is %s", fmt.Sprint(gs.state))
}

// AddTxn adds a transaction to the graph search database
func (gs *Db) AddTxn(msgTx *wire.MsgTx) error {
	tokenIDBuf, err := goslp.GetSlpTokenID(msgTx)
	if err != nil {
		return err
	}
	tokenID, err := chainhash.NewHash(tokenIDBuf)
	if err != nil {
		return err
	}

	tg := gs.getTokenGraph(tokenID)
	err = tg.addTxn(msgTx)
	if err != nil {
		return err
	}

	return nil

}

// Find performs a graph search for a given transaction hash
func (gs *Db) Find(hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}) ([][]byte, error) {

	// get token graph
	tokenGraph := gs.getTokenGraph(tokenID)
	if tokenGraph == nil {
		return nil, fmt.Errorf("graph search graph is missing for token ID %v", tokenID)
	}

	seen := make(map[chainhash.Hash]struct{})
	txdata := make([][]byte, tokenGraph.size())
	i := 0

	// check client validity cache transactions are valid
	for hash := range *validityCache {
		if txn := (*tokenGraph).getTxn(&hash); txn == nil {
			return nil, fmt.Errorf("client provided validity cache with hash %v that is not in the token graph", hash)
		}
	}

	// perform the recursive graph search
	err := gs.findInternal(hash, tokenGraph, &seen, validityCache, &txdata, &i)
	if err != nil {
		return nil, err
	}

	// TODO: Do an integrity check before returning results to client!

	return txdata[0:i], nil
}

func (gs *Db) findInternal(hash *chainhash.Hash, graph *tokenGraph, seen *map[chainhash.Hash]struct{}, validityCache *map[chainhash.Hash]struct{}, txdata *[][]byte, counter *int) error {

	// check if txn is valid slp
	txMsg := graph.getTxn(hash)
	if txMsg == nil {
		return fmt.Errorf("txn %v not in token graph, implies invalid slp", hash)
	}

	// check seen list
	if _, ok := (*seen)[*hash]; ok {
		return fmt.Errorf("txn %v already seen in graph search", hash)
	}
	(*seen)[*hash] = struct{}{}

	// add txn buffer to results
	txBuf := bytes.NewBuffer(make([]byte, 0, txMsg.SerializeSize()))
	if err := txMsg.Serialize(txBuf); err != nil {
		return err
	}
	(*txdata)[*counter] = txBuf.Bytes()
	(*counter)++

	// check exclude txids here, don't return with error
	if _, ok := (*validityCache)[*hash]; ok {
		//gs.logger.Debugf("skipping valid slp txn provided by client exclude list for %v", hash)
		return nil
	}

	// loop through inputs and recurse
	for _, txn := range txMsg.TxIn {
		err := gs.findInternal(&txn.PreviousOutPoint.Hash, graph, seen, validityCache, txdata, counter)
		if err != nil {
			//*logger.Debugf("%v", err)
			continue
		}
	}
	return nil
}

// getTokenGraph gets a token graph item from the db
func (gs *Db) getTokenGraph(tokenID *chainhash.Hash) *tokenGraph {

	gs.RLock()
	if tg, ok := gs.graphs[*tokenID]; ok {
		gs.RUnlock()
		return tg
	}
	gs.RUnlock()

	gs.Lock()
	defer gs.Unlock()

	if tg, ok := gs.graphs[*tokenID]; ok {
		return tg
	}

	item := newTokenGraph(tokenID)
	gs.graphs[*tokenID] = item
	return item
}
