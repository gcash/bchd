package slpgraphsearch

import (
	"errors"
	"sync"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp/v1parser"
)

// tokenGraph manages slp token graphs for graph search and TODO: recently queried items
type tokenGraph struct {
	sync.RWMutex
	TokenID *chainhash.Hash
	graph   map[chainhash.Hash]graphTxn
}

// graphTxn is what a token's graph holds of a transaction: the outpoints its
// inputs spend, to search back through, and what its slp message says of the
// outputs a child spends, to tell whether an input carried the token.  A
// token's graph can hold millions of transactions, so it holds no more of
// them; a search reads back each transaction it returns.
type graphTxn struct {
	inputs []wire.OutPoint
	slp    slpOutputs
}

// slpKind is the kind of slp message in a transaction's first output.
type slpKind uint8

const (
	// slpUnparsed is a first output the slp parser does not take.
	slpUnparsed slpKind = iota
	slpGenesis
	slpMint
	slpSend

	// slpOther is an slp message of a kind the search does not know.
	slpOther
)

// slpOutputs is what the slp message of a transaction says of its outputs.
type slpOutputs struct {
	kind slpKind

	// mintsTokens is whether a GENESIS or MINT puts tokens on output 1.
	mintsTokens bool

	// mintBatonVout is the output a GENESIS or MINT puts its mint baton
	// on.
	mintBatonVout uint32

	// sendAmounts is how many outputs a SEND puts tokens on.
	sendAmounts int
}

// newGraphTxn returns what a token's graph holds of tx.
func newGraphTxn(tx *wire.MsgTx) graphTxn {
	inputs := make([]wire.OutPoint, len(tx.TxIn))
	for i, txIn := range tx.TxIn {
		inputs[i] = txIn.PreviousOutPoint
	}
	return graphTxn{inputs: inputs, slp: parseSlpOutputs(tx)}
}

// parseSlpOutputs returns what the slp message in the first output of tx
// says of its outputs.
func parseSlpOutputs(tx *wire.MsgTx) slpOutputs {
	if len(tx.TxOut) == 0 {
		return slpOutputs{}
	}
	slpMsg, err := v1parser.ParseSLP(tx.TxOut[0].PkScript)
	if err != nil {
		return slpOutputs{}
	}
	switch msg := slpMsg.(type) {
	case *v1parser.SlpGenesis:
		return slpOutputs{kind: slpGenesis, mintsTokens: msg.Qty != 0,
			mintBatonVout: uint32(msg.MintBatonVout)}
	case *v1parser.SlpMint:
		return slpOutputs{kind: slpMint, mintsTokens: msg.Qty != 0,
			mintBatonVout: uint32(msg.MintBatonVout)}
	case *v1parser.SlpSend:
		return slpOutputs{kind: slpSend, sendAmounts: len(msg.Amounts)}
	default:
		return slpOutputs{kind: slpOther}
	}
}

// newTokenGraph creates a new instance of tokenGraph
func newTokenGraph(tokenID *chainhash.Hash) *tokenGraph {
	return &tokenGraph{
		graph:   make(map[chainhash.Hash]graphTxn),
		TokenID: tokenID,
	}
}

// size gets the current size of the token graph
func (g *tokenGraph) size() int {
	return len(g.graph)
}

// getTxn gets graph items allowing concurrent read access without
func (g *tokenGraph) getTxn(hash *chainhash.Hash) (graphTxn, bool) {
	g.RLock()
	defer g.RUnlock()

	txn, ok := g.graph[*hash]
	return txn, ok
}

// addTxn puts new graph items in a token graph
func (g *tokenGraph) addTxn(tx *wire.MsgTx) error {
	hash, txn := tx.TxHash(), newGraphTxn(tx)
	g.Lock()
	defer g.Unlock()
	size0 := g.size()
	g.graph[hash] = txn
	if g.size() < size0 {
		return errors.New("token graph db should never get smaller")
	}
	return nil
}

// removeTxn removes a transaction from the graph
//
//nolint:unused // retained for future callers
func (g *tokenGraph) removeTxn(tx *wire.MsgTx) error {
	g.Lock()
	defer g.Unlock()

	if _, ok := g.graph[tx.TxHash()]; !ok {
		return errors.New("transaction doesn't exist in graph")
	}
	delete(g.graph, tx.TxHash())
	return nil
}
