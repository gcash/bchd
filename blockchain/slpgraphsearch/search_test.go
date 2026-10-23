// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package slpgraphsearch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
	"github.com/gcash/goslp/v1parser"
)

// findRecursive is the graph search as Find did it before it kept a stack of
// its own, recursing once for each transaction it adds, which the tests
// compare Find with.
func findRecursive(gs *Db, ctx context.Context, hash *chainhash.Hash, tokenID *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}) ([][]byte, error) {

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

	txMsg := tokenGraph.getTxn(hash)
	if txMsg == nil {
		return nil, fmt.Errorf("txn %v not in token graph, implies invalid slp", hash)
	}

	// perform the recursive graph search
	err := findInternalRecursive(ctx, txMsg, tokenGraph, &seen, validityCache, &txdata, &i)
	if err != nil {
		return nil, err
	}

	// TODO: Do an integrity check before returning results to client!

	return txdata[0:i], nil
}

func findInternalRecursive(ctx context.Context, txMsg *wire.MsgTx, graph *tokenGraph, seen *map[chainhash.Hash]struct{}, validityCache *map[chainhash.Hash]struct{}, txdata *[][]byte, counter *int) error {

	hash := txMsg.TxHash()

	// check seen list
	if _, ok := (*seen)[hash]; ok {
		return fmt.Errorf("txn %v already seen in graph search", hash)
	}
	if *counter%graphSearchCheckInterval == 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	(*seen)[hash] = struct{}{}

	// add txn buffer to results
	txBuf := bytes.NewBuffer(make([]byte, 0, txMsg.SerializeSize()))
	if err := txMsg.Serialize(txBuf); err != nil {
		return err
	}
	(*txdata)[*counter] = txBuf.Bytes()
	(*counter)++

	// check exclude txids here, don't return with error
	if _, ok := (*validityCache)[hash]; ok {
		return nil
	}

	// loop through inputs and recurse
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
			return fmt.Errorf("txn %v was parsed as an unknown kind of slp transaction", prevTxHash)
		}

		// Only the end of the context stops the search; other errors
		// leave out the input.
		err = findInternalRecursive(ctx, inpTxMsg, graph, seen, validityCache, txdata, counter)
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {

			return err
		}
	}
	return nil
}

// randomGraph returns a database holding a token graph of n transactions, in
// the order made, each spending outputs of those before it at random, mostly
// of the last few, so that the graph has chains as well as transactions
// reached along more than one path.  It mixes GENESIS, MINT and SEND
// transactions, with and without tokens and mint batons, and transactions
// whose first output is no slp message, and their inputs spend outputs with
// and without a token or a mint baton, and outputs of transactions outside
// the graph.  In a deep graph each transaction also spends the first token
// output of the one before, which carries tokens, so that a search from the
// last goes back through the whole graph.
func randomGraph(t *testing.T, rng *rand.Rand, n int, deep bool) (*Db, *chainhash.Hash, []*wire.MsgTx) {
	t.Helper()
	tokenID := chainhash.Hash{0x51, byte(n), byte(rng.Intn(256))}
	baton := func() *metadatamaker.MintBatonVout {
		if rng.Intn(3) == 0 {
			return nil
		}
		return metadatamaker.NewMintBatonVout(2 + rng.Intn(2))
	}
	qty := func() uint64 {
		if !deep && rng.Intn(4) == 0 {
			return 0
		}
		return uint64(1 + rng.Intn(1000))
	}
	graph := newTokenGraph(&tokenID)
	txs := make([]*wire.MsgTx, 0, n)
	for i := 0; i < n; i++ {
		tx := wire.NewMsgTx(1)
		tx.LockTime = uint32(i)
		if deep && i > 0 {
			tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: txs[i-1].TxHash(),
				Index: 1}, nil))
		}
		for j := rng.Intn(4); j > 0; j-- {
			var prev wire.OutPoint
			switch {
			case len(txs) == 0 || rng.Intn(10) == 0:
				prev.Hash = chainhash.Hash{0xee, byte(i), byte(i >> 8), byte(j)}
			case rng.Intn(2) == 0:
				prev.Hash = txs[max(0, len(txs)-1-rng.Intn(3))].TxHash()
			default:
				prev.Hash = txs[rng.Intn(len(txs))].TxHash()
			}
			prev.Index = uint32(rng.Intn(5))
			tx.AddTxIn(wire.NewTxIn(&prev, nil))
		}
		var script []byte
		var err error
		switch k := rng.Intn(10); {
		case k == 0:
			script, err = metadatamaker.TokenType1Genesis([]byte("T"),
				[]byte("Token"), nil, nil, 0, baton(), qty())
		case k <= 2:
			script, err = metadatamaker.TokenType1Mint(tokenID[:], baton(),
				qty())
		case k == 3 && !deep:
			script = []byte{txscript.OP_TRUE}
		default:
			amounts := make([]uint64, 1+rng.Intn(4))
			for a := range amounts {
				amounts[a] = qty()
			}
			script, err = metadatamaker.TokenType1Send(tokenID[:], amounts)
		}
		if err != nil {
			t.Fatal(err)
		}
		tx.AddTxOut(&wire.TxOut{PkScript: script})
		for o := 0; o < 4; o++ {
			tx.AddTxOut(&wire.TxOut{Value: 546,
				PkScript: []byte{txscript.OP_TRUE}})
		}
		if err := graph.addTxn(tx); err != nil {
			t.Fatal(err)
		}
		txs = append(txs, tx)
	}
	db := NewDb()
	db.graphs[tokenID] = graph
	return db, &tokenID, txs
}

// sendChain returns a database holding a GENESIS and a chain of n sends,
// each spending the token output of the one before, the hash of the last
// and the token ID.
func sendChain(t *testing.T, n int) (*Db, *chainhash.Hash, *chainhash.Hash) {
	t.Helper()
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	genesis := wire.NewMsgTx(1)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{1}}, nil))
	genesis.AddTxOut(&wire.TxOut{PkScript: script})
	genesis.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	tokenIDBuf, err := goslp.GetSlpTokenID(genesis)
	if err != nil {
		t.Fatal(err)
	}
	send, err := metadatamaker.TokenType1Send(tokenIDBuf, []uint64{1000})
	if err != nil {
		t.Fatal(err)
	}
	db := NewDb()
	if err := db.AddTxn(genesis); err != nil {
		t.Fatal(err)
	}
	tip := genesis.TxHash()
	for i := 0; i < n; i++ {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: tip, Index: 1}, nil))
		tx.AddTxOut(&wire.TxOut{PkScript: send})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		if err := db.AddTxn(tx); err != nil {
			t.Fatal(err)
		}
		tip = tx.TxHash()
	}
	tokenID, err := chainhash.NewHash(tokenIDBuf)
	if err != nil {
		t.Fatal(err)
	}
	return db, &tip, tokenID
}

// TestFindSameAsRecursive ensures Find returns the same transactions in the
// same order, or the same error, and looks at its context as often, as the
// recursive search it replaced, on random graphs, from random transactions,
// with random validity caches, and with a context that ends during the
// search.
func TestFindSameAsRecursive(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var searches, looks, found int
	for g := 0; g < 300; g++ {
		n, deep := 1+rng.Intn(80), false
		if g%60 == 0 {
			n, deep = 3000, true
		}
		db, tokenID, txs := randomGraph(t, rng, n, deep)
		for s := 0; s < 12; s++ {
			root := txs[rng.Intn(len(txs))].TxHash()
			if deep && s >= 6 {
				root = txs[len(txs)-1-rng.Intn(10)].TxHash()
			}
			valid := make(map[chainhash.Hash]struct{})
			for k := rng.Intn(4); k > 0; k-- {
				valid[txs[rng.Intn(len(txs))].TxHash()] = struct{}{}
			}
			switch s {
			case 0:
				// A validity cache naming a transaction outside the
				// graph.
				valid[chainhash.Hash{0xff}] = struct{}{}
			case 1:
				// A transaction outside the graph.
				root = chainhash.Hash{0xff}
			}
			checks, err := []int{0, 1, 2}[rng.Intn(3)], context.Canceled
			if checks == 2 && rng.Intn(2) == 0 {
				err = context.DeadlineExceeded
			}
			want := &endingContext{Context: context.Background(),
				checks: checks, err: err}
			if checks == 0 {
				want.checks = 1 << 30
			}
			got := *want
			wantRes, wantErr := findRecursive(db, want, &root, tokenID, &valid)
			gotRes, gotErr := db.Find(&got, &root, tokenID, &valid)

			what := fmt.Sprintf("graph %d of %d transactions, search %d "+
				"from %v with %d valid and the context ending at check %d",
				g, n, s, root, len(valid), checks)
			if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
				t.Fatalf("%s: got error %v, want %v", what, gotErr, wantErr)
			}
			if len(gotRes) != len(wantRes) || (gotRes == nil) != (wantRes == nil) {
				t.Fatalf("%s: got %d transactions, want %d", what,
					len(gotRes), len(wantRes))
			}
			for i := range wantRes {
				if !bytes.Equal(gotRes[i], wantRes[i]) {
					t.Fatalf("%s: transaction %d differs", what, i)
				}
			}
			if got.calls != want.calls {
				t.Fatalf("%s: the context was looked at %d times, want %d",
					what, got.calls, want.calls)
			}
			searches++
			if want.calls > 1 {
				looks++
			}
			if len(wantRes) > 1 {
				found++
			}
		}
	}

	// The searches must have gone past the transaction searched from, and
	// some past a check of the context after the first.
	t.Logf("%d searches, %d found more than one transaction, %d looked at "+
		"the context more than once", searches, found, looks)
	if found < searches/5 || looks == 0 {
		t.Fatalf("of %d searches, %d found more than one transaction and %d "+
			"looked at the context more than once", searches, found, looks)
	}
}

// stackContext is a context that records the most stack memory in use each
// time it is looked at.
type stackContext struct {
	context.Context
	most uint64
}

func (c *stackContext) Err() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	c.most = max(c.most, m.StackInuse)
	return nil
}

// TestFindDeepChainStack ensures a search through a long chain of sends does
// not grow its goroutine's stack with the chain.  Recursing once for each
// transaction, a search of 100,000 sends took about 34 MB of stack, and one
// of about 3.7 million would pass the runtime's 1 GB limit, which ends the
// process.
func TestFindDeepChainStack(t *testing.T) {
	const sends = 100000
	db, tip, tokenID := sendChain(t, sends)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	ctx := &stackContext{Context: context.Background(), most: before.StackInuse}
	var res [][]byte
	done := make(chan error)
	go func() {
		var err error
		res, err = db.Find(ctx, tip, tokenID, &map[chainhash.Hash]struct{}{})
		done <- err
	}()
	if err := <-done; err != nil || len(res) != sends+1 {
		t.Fatalf("got %d transactions and %v, want %d", len(res), err,
			sends+1)
	}
	if grown := int64(ctx.most) - int64(before.StackInuse); grown > 1<<20 {
		t.Errorf("the search of %d sends grew the stack by %.1f MB", sends,
			float64(grown)/1e6)
	}
}
