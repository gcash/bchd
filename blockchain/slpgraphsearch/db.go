package slpgraphsearch

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp"
)

// Db manages slp token graphs for graph search and TODO: recently queried items
type Db struct {
	sync.RWMutex
	graphs map[chainhash.Hash]*tokenGraph
	state  uint32 // 0 = initial load incomplete, 1 = initial load complete, 2 = block found after load completed

	// fetchTxns reads transactions of the graphs, serialized, in the order
	// of their hashes, as the graphs hold only what a search needs of each.
	fetchTxns func(txnHashes []chainhash.Hash) ([][]byte, error)
}

// NewDb creates a new instance of SlpCache, whose searches read the
// transactions they return with fetchTxns, which fails if it cannot read one
// of them.
func NewDb(fetchTxns func(txnHashes []chainhash.Hash) ([][]byte, error)) *Db {
	return &Db{
		graphs:    make(map[chainhash.Hash]*tokenGraph),
		state:     0,
		fetchTxns: fetchTxns,
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
	return fmt.Errorf("slp gs db was not set to loaded with current state is %s", fmt.Sprint(state))
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
	return fmt.Errorf("slp gs db was not set to ready with current state is %s", fmt.Sprint(state))
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

// graphSearchCheckInterval is how many transactions a graph search finds
// between looks at whether its context has ended, and how many it reads at a
// time.
const graphSearchCheckInterval = 1000

// Find performs a graph search for a given transaction hash.  The search can
// take the whole of a large token's graph, so it stops once ctx ends and
// returns the context's error.  It reads the transactions it returns, a batch
// of graphSearchCheckInterval at a time, as a node reads each batch in one
// database transaction, and fails if it cannot read one.
func (gs *Db) Find(ctx context.Context, hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}) ([][]byte, error) {

	// The result grows with what the search finds, not with the token's
	// graph, which can hold millions of transactions and can grow while the
	// search runs.
	var txdata [][]byte
	var batch []chainhash.Hash
	read := func() error {
		txBufs, err := gs.fetchTxns(batch)
		if err != nil {
			return fmt.Errorf("graph search failed to read txns: %w", err)
		}
		txdata = append(txdata, txBufs...)
		batch = batch[:0]
		return nil
	}
	err := gs.search(ctx, hash, tokenID, validityCache, func(hash *chainhash.Hash) error {
		// add txn to results, reading a full batch before the search
		// looks at its context again
		batch = append(batch, *hash)
		if len(batch) < graphSearchCheckInterval {
			return nil
		}
		return read()
	})
	if err == nil && len(batch) > 0 {
		err = read()
	}
	if err != nil {
		return nil, err
	}

	// TODO: Do an integrity check before returning results to client!

	return txdata, nil
}

// Count returns how many transactions Find returns for the same search,
// without reading them, for a client that asks how large the search is.
func (gs *Db) Count(ctx context.Context, hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}) (int, error) {
	n := 0
	err := gs.search(ctx, hash, tokenID, validityCache, func(*chainhash.Hash) error {
		n++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// search performs the graph search of Find and Count, calling found with the
// hash of each transaction it finds, in the order Find returns them.  An
// error from found ends the search.
func (gs *Db) search(ctx context.Context, hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}, found func(*chainhash.Hash) error) error {

	// get token graph
	tokenGraph := gs.getTokenGraph(tokenID)
	if tokenGraph == nil {
		return fmt.Errorf("graph search graph is missing for token ID %v", tokenID)
	}

	seen := make(map[chainhash.Hash]struct{})

	// check client validity cache transactions are valid
	for hash := range *validityCache {
		if _, ok := (*tokenGraph).getTxn(&hash); !ok {
			return fmt.Errorf("client provided validity cache with hash %v that is not in the token graph", hash)
		}
	}

	txn, ok := tokenGraph.getTxn(hash)
	if !ok {
		return fmt.Errorf("txn %v not in token graph, implies invalid slp", hash)
	}

	// perform the graph search
	return gs.findInternal(ctx, hash, txn, tokenGraph, &seen, validityCache, found)
}

// findInternal searches the token's graph depth first from txn, whose hash is
// hash, through each input that carried the token, in the order of the
// inputs, and calls found for each transaction the first time it reaches it.
// It keeps the transactions still to be searched on a stack of its own rather
// than recursing, as a token's graph can hold a chain of millions of sends,
// and a goroutine whose stack passes the runtime's 1 GB limit ends the
// process.  A transaction's inputs go on the stack in reverse, so that each,
// with all it leads to, is searched before the next, as a recursive search
// would.  Each goes with what the graph holds of it, so that the search does
// not look it up again.
func (gs *Db) findInternal(ctx context.Context, hash *chainhash.Hash, txn graphTxn, graph *tokenGraph, seen *map[chainhash.Hash]struct{}, validityCache *map[chainhash.Hash]struct{}, found func(*chainhash.Hash) error) error {
	type pending struct {
		hash *chainhash.Hash
		txn  graphTxn
	}
	root := *hash
	stack := []pending{{hash, txn}}
	n := 0
	for len(stack) > 0 {
		hash, txn := stack[len(stack)-1].hash, stack[len(stack)-1].txn
		stack = stack[:len(stack)-1]

		// check seen list
		if _, ok := (*seen)[*hash]; ok {
			continue
		}
		if n%graphSearchCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		(*seen)[*hash] = struct{}{}

		// add txn to results
		if err := found(hash); err != nil {
			return err
		}
		n++

		// check exclude txids here, don't return with error
		if _, ok := (*validityCache)[*hash]; ok {
			//gs.logger.Debugf("skipping valid slp txn provided by client exclude list for %v", hash)
			continue
		}

		// loop through inputs and push those that contributed, then turn
		// them around so the first is searched first
		pushed := len(stack)
	inputs:
		for i := range txn.inputs {
			prevTxHash := &txn.inputs[i].Hash
			prevTxVout := txn.inputs[i].Index

			inpTxn, ok := graph.getTxn(prevTxHash)
			if !ok {
				continue
			}

			// check to see if this parent contributed, by what its slp
			// msg says of the output this input spends
			switch out := inpTxn.slp; out.kind {
			case slpUnparsed:
				continue
			case slpGenesis, slpMint:
				if prevTxVout != 1 && prevTxVout != out.mintBatonVout {
					continue
				}
				if prevTxVout == 1 && !out.mintsTokens {
					continue
				}
			case slpSend:
				if int(prevTxVout) > out.sendAmounts {
					continue
				}
			default:
				// As when the search recursed, this leaves out the
				// input and those after it, or ends the search for
				// the transaction searched from.
				if *hash == root {
					return fmt.Errorf("txn %v was parsed as an unknown kind of slp transaction", prevTxHash)
				}
				break inputs
			}
			stack = append(stack, pending{prevTxHash, inpTxn})
		}
		slices.Reverse(stack[pushed:])
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
