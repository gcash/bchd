package slpgraphsearch

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/v1parser"
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

// graphSearchCheckInterval is how many transactions a graph search finds
// between looks at whether its context has ended.
const graphSearchCheckInterval = 1000

// Find performs a graph search for a given transaction hash.  The search can
// take the whole of a large token's graph, so it stops once ctx ends and
// returns the context's error.
func (gs *Db) Find(ctx context.Context, hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}) ([][]byte, error) {

	// The result grows with what the search finds, not with the token's
	// graph, which can hold millions of transactions and can grow while the
	// search runs.
	var txdata [][]byte
	err := gs.search(ctx, hash, tokenID, validityCache, func(txMsg *wire.MsgTx) error {
		// add txn buffer to results
		txBuf := bytes.NewBuffer(make([]byte, 0, txMsg.SerializeSize()))
		if err := txMsg.Serialize(txBuf); err != nil {
			return err
		}
		txdata = append(txdata, txBuf.Bytes())
		return nil
	})
	if err != nil {
		return nil, err
	}

	// TODO: Do an integrity check before returning results to client!

	return txdata, nil
}

// Count returns how many transactions Find returns for the same search,
// without serializing them, for a client that asks how large the search is.
func (gs *Db) Count(ctx context.Context, hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}) (int, error) {
	n := 0
	err := gs.search(ctx, hash, tokenID, validityCache, func(*wire.MsgTx) error {
		n++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// search performs the graph search of Find and Count, calling found for each
// transaction it finds, in the order Find returns them.
func (gs *Db) search(ctx context.Context, hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}, found func(*wire.MsgTx) error) error {

	// get token graph
	tokenGraph := gs.getTokenGraph(tokenID)
	if tokenGraph == nil {
		return fmt.Errorf("graph search graph is missing for token ID %v", tokenID)
	}

	seen := make(map[chainhash.Hash]struct{})

	// check client validity cache transactions are valid
	for hash := range *validityCache {
		if txn := (*tokenGraph).getTxn(&hash); txn == nil {
			return fmt.Errorf("client provided validity cache with hash %v that is not in the token graph", hash)
		}
	}

	txMsg := tokenGraph.getTxn(hash)
	if txMsg == nil {
		return fmt.Errorf("txn %v not in token graph, implies invalid slp", hash)
	}

	// perform the graph search
	return gs.findInternal(ctx, hash, txMsg, tokenGraph, &seen, validityCache, found)
}

// findInternal searches the token's graph depth first from txMsg, whose hash
// is hash, through each input that carried the token, in the order of the
// inputs, and calls found for each transaction the first time it reaches it.
// It keeps the transactions still to be searched on a stack of its own rather
// than recursing, as a token's graph can hold a chain of millions of sends,
// and a goroutine whose stack passes the runtime's 1 GB limit ends the
// process.  A transaction's inputs go on the stack in reverse, so that each,
// with all it leads to, is searched before the next, as a recursive search
// would.  Each goes with the hash the graph holds it by, so that the search
// does not serialize it to hash it again.
func (gs *Db) findInternal(ctx context.Context, hash *chainhash.Hash, txMsg *wire.MsgTx, graph *tokenGraph, seen *map[chainhash.Hash]struct{}, validityCache *map[chainhash.Hash]struct{}, found func(*wire.MsgTx) error) error {
	type pending struct {
		hash  *chainhash.Hash
		txMsg *wire.MsgTx
	}
	root := txMsg
	stack := []pending{{hash, txMsg}}
	n := 0
	for len(stack) > 0 {
		hash, txMsg := stack[len(stack)-1].hash, stack[len(stack)-1].txMsg
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

		// add txn to results; as when the search recursed, an error leaves
		// out the transaction, or ends the search if it is the one
		// searched from
		if err := found(txMsg); err != nil {
			if txMsg == root {
				return err
			}
			continue
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
		for _, txn := range txMsg.TxIn {
			prevTxHash := &txn.PreviousOutPoint.Hash
			prevTxVout := txn.PreviousOutPoint.Index

			inpTxMsg := graph.getTxn(prevTxHash)
			if inpTxMsg == nil {
				continue
			}

			// try to parse slp msg to check if this input contributes to this transaction
			inpSlpMsg, err := v1parser.ParseSLP(inpTxMsg.TxOut[0].PkScript)
			if err != nil {
				continue
			}

			// check to see if this parent contributed
			switch msg := inpSlpMsg.(type) {
			case *v1parser.SlpGenesis:
				if prevTxVout != 1 && prevTxVout != uint32(msg.MintBatonVout) {
					continue
				}
				if prevTxVout == 1 && msg.Qty == 0 {
					continue
				}
			case *v1parser.SlpMint:
				if prevTxVout != 1 && prevTxVout != uint32(msg.MintBatonVout) {
					continue
				}
				if prevTxVout == 1 && msg.Qty == 0 {
					continue
				}
			case *v1parser.SlpSend:
				if int(prevTxVout) > len(msg.Amounts) {
					continue
				}
			default:
				// As when the search recursed, this leaves out the
				// input and those after it, or ends the search for
				// the transaction searched from.
				if txMsg == root {
					return fmt.Errorf("txn %v was parsed as an unknown kind of slp transaction", prevTxHash)
				}
				break inputs
			}
			stack = append(stack, pending{prevTxHash, inpTxMsg})
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
