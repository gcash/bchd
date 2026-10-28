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
	"slices"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
	"github.com/gcash/goslp/v1parser"
)

// errNotStored is the error testDb reads a transaction the test did not store
// with.
var errNotStored = errors.New("transaction not stored")

// testDb is a database that reads the transactions its searches return from
// those the test stored, as a node reads them from its transaction index, and
// counts the transactions read and the batches they were read in.
type testDb struct {
	*Db
	raw     map[chainhash.Hash][]byte
	reads   int
	batches []int
}

func newTestDb() *testDb {
	db := &testDb{raw: make(map[chainhash.Hash][]byte)}
	db.Db = NewDb(db.fetch)
	return db
}

func (db *testDb) fetch(hashes []chainhash.Hash) ([][]byte, error) {
	db.batches = append(db.batches, len(hashes))
	txns := make([][]byte, len(hashes))
	for i := range hashes {
		raw, ok := db.raw[hashes[i]]
		if !ok {
			return nil, fmt.Errorf("%v: %w", hashes[i], errNotStored)
		}
		txns[i] = raw
	}
	db.reads += len(hashes)
	return txns, nil
}

// checkBatches checks that the transactions found were read in batches of
// graphSearchCheckInterval, but for the last.
func checkBatches(t *testing.T, what string, batches []int, found int) {
	t.Helper()
	want := (found + graphSearchCheckInterval - 1) / graphSearchCheckInterval
	if len(batches) != want {
		t.Fatalf("%s: read %d transactions in %d batches, want %d", what,
			found, len(batches), want)
	}
	for i, n := range batches {
		if n != min(graphSearchCheckInterval,
			found-i*graphSearchCheckInterval) {

			t.Fatalf("%s: batches of %v for %d transactions", what,
				batches, found)
		}
	}
}

// store stores tx for the database to read.
func (db *testDb) store(t *testing.T, tx *wire.MsgTx) {
	t.Helper()
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	db.raw[tx.TxHash()] = buf.Bytes()
}

// addTxn stores tx and adds it to the database.
func (db *testDb) addTxn(t *testing.T, tx *wire.MsgTx) {
	t.Helper()
	db.store(t, tx)
	if err := db.AddTxn(tx); err != nil {
		t.Fatal(err)
	}
}

// findRecursive is the graph search as Find did it before it kept a stack of
// its own and before the graph held only what the search needs of each
// transaction, recursing once for each transaction it adds, through a graph
// of whole transactions, which the tests compare Find with.
func findRecursive(graph map[chainhash.Hash]*wire.MsgTx, ctx context.Context, hash *chainhash.Hash, validityCache *map[chainhash.Hash]struct{}) ([][]byte, error) {

	seen := make(map[chainhash.Hash]struct{})
	txdata := make([][]byte, len(graph))
	i := 0

	// check client validity cache transactions are valid
	for hash := range *validityCache {
		if txn := graph[hash]; txn == nil {
			return nil, fmt.Errorf("client provided validity cache with hash %v that is not in the token graph", hash)
		}
	}

	txMsg := graph[*hash]
	if txMsg == nil {
		return nil, fmt.Errorf("txn %v not in token graph, implies invalid slp", hash)
	}

	// perform the recursive graph search
	err := findInternalRecursive(ctx, txMsg, graph, &seen, validityCache, &txdata, &i)
	if err != nil {
		return nil, err
	}

	// TODO: Do an integrity check before returning results to client!

	return txdata[0:i], nil
}

func findInternalRecursive(ctx context.Context, txMsg *wire.MsgTx, graph map[chainhash.Hash]*wire.MsgTx, seen *map[chainhash.Hash]struct{}, validityCache *map[chainhash.Hash]struct{}, txdata *[][]byte, counter *int) error {

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

		inpTxMsg := graph[*prevTxHash]
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
func randomGraph(t *testing.T, rng *rand.Rand, n int, deep bool) (*testDb, *chainhash.Hash, []*wire.MsgTx) {
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
	db := newTestDb()
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
		db.store(t, tx)
		txs = append(txs, tx)
	}
	db.graphs[tokenID] = graph
	return db, &tokenID, txs
}

// sendChain returns a database holding a GENESIS and a chain of n sends,
// each spending the token output of the one before, the hash of the last
// and the token ID.
func sendChain(t *testing.T, n int) (*testDb, *chainhash.Hash, *chainhash.Hash) {
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
	db := newTestDb()
	db.addTxn(t, genesis)
	tip := genesis.TxHash()
	for i := 0; i < n; i++ {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: tip, Index: 1}, nil))
		tx.AddTxOut(&wire.TxOut{PkScript: send})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		db.addTxn(t, tx)
		tip = tx.TxHash()
	}
	tokenID, err := chainhash.NewHash(tokenIDBuf)
	if err != nil {
		t.Fatal(err)
	}
	return db, &tip, tokenID
}

// genesisHash returns the hash of the GENESIS of tokenID, which is the token
// ID in the byte order it is shown in.
func genesisHash(tokenID *chainhash.Hash) chainhash.Hash {
	var genesis chainhash.Hash
	for i := range genesis {
		genesis[i] = tokenID[chainhash.HashSize-1-i]
	}
	return genesis
}

// TestFindSameAsRecursive ensures Find returns the same transactions in the
// same order, or the same error, and looks at its context as often, as the
// recursive search through whole transactions it replaced, reading each
// transaction it returns once, and Count as many transactions, reading none,
// on random graphs, from random transactions, with random validity caches,
// and with a context that ends during the search.
func TestFindSameAsRecursive(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var searches, looks, found int
	for g := 0; g < 300; g++ {
		n, deep := 1+rng.Intn(80), false
		if g%60 == 0 {
			n, deep = 3000, true
		}
		db, tokenID, txs := randomGraph(t, rng, n, deep)
		whole := make(map[chainhash.Hash]*wire.MsgTx, len(txs))
		for _, tx := range txs {
			whole[tx.TxHash()] = tx
		}
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
			got, counted := *want, *want
			wantRes, wantErr := findRecursive(whole, want, &root, &valid)
			db.reads, db.batches = 0, nil
			gotRes, gotErr := db.Find(&got, &root, tokenID, &valid)
			reads, batches := db.reads, db.batches
			count, countErr := db.Count(&counted, &root, tokenID, &valid)

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
			if wantErr == nil {
				if reads != len(wantRes) {
					t.Fatalf("%s: read %d transactions for %d", what,
						reads, len(wantRes))
				}
				checkBatches(t, what, batches, len(wantRes))
			}
			if fmt.Sprint(countErr) != fmt.Sprint(wantErr) ||
				count != len(wantRes) || counted.calls != want.calls {

				t.Fatalf("%s: counted %d and %v, looking at the context %d "+
					"times, want %d and %v, %d times", what, count, countErr,
					counted.calls, len(wantRes), wantErr, want.calls)
			}
			if db.reads != reads || len(db.batches) != len(batches) {
				t.Fatalf("%s: counting read %d transactions", what,
					db.reads-reads)
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

// TestFindAllocatesForResult ensures a search allocates in proportion to what
// it finds, not to the token's graph.  A search of the GENESIS alone once
// allocated a slot for each transaction of the graph: 2.4 MB for a graph of
// 100,001 transactions, and 24 MB for one of a million.
func TestFindAllocatesForResult(t *testing.T) {
	const sends = 100000
	db, _, tokenID := sendChain(t, sends)

	genesis := genesisHash(tokenID)
	var res [][]byte
	var err error
	search := func() uint64 {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		res, err = db.Find(context.Background(), &genesis, tokenID,
			&map[chainhash.Hash]struct{}{})
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	least := search()
	for i := 0; i < 4; i++ {
		least = min(least, search())
	}
	if err != nil || len(res) != 1 {
		t.Fatalf("got %d transactions and %v, want the GENESIS", len(res), err)
	}
	if least > 64<<10 {
		t.Errorf("a search finding 1 of %d transactions allocated %d bytes",
			sends+1, least)
	}
}

// growingContext is a context that adds tx to db the second time it is
// looked at.
type growingContext struct {
	context.Context
	db    *Db
	tx    *wire.MsgTx
	calls int
	err   error
}

func (c *growingContext) Err() error {
	c.calls++
	if c.calls == 2 {
		c.err = c.db.AddTxn(c.tx)
	}
	return nil
}

// TestFindGraphGrows ensures a search that finds more transactions than the
// token's graph held when it began returns all of them.  Block connection and
// the memory pool add transactions to the graph in goroutines of their own, in
// no set order, so a search can find a parent added after it began; its
// result once had a slot for each transaction the graph held at the start, and
// the search indexed past the last.
func TestFindGraphGrows(t *testing.T) {
	// A GENESIS, added to the graph only once the search has begun, a SEND
	// spending its token output, a chain of 999 sends from outside the
	// graph, and the SEND searched from, which spends the token outputs of
	// the chain's last and of the first SEND, in that order.
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	genesis := wire.NewMsgTx(1)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{2}}, nil))
	genesis.AddTxOut(&wire.TxOut{PkScript: script})
	genesis.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	tokenIDBuf, err := goslp.GetSlpTokenID(genesis)
	if err != nil {
		t.Fatal(err)
	}
	tokenID, err := chainhash.NewHash(tokenIDBuf)
	if err != nil {
		t.Fatal(err)
	}
	send, err := metadatamaker.TokenType1Send(tokenIDBuf, []uint64{1000})
	if err != nil {
		t.Fatal(err)
	}
	db := newTestDb()
	db.store(t, genesis)
	sendFrom := func(prev ...wire.OutPoint) *wire.MsgTx {
		t.Helper()
		tx := wire.NewMsgTx(1)
		for i := range prev {
			tx.AddTxIn(wire.NewTxIn(&prev[i], nil))
		}
		tx.AddTxOut(&wire.TxOut{PkScript: send})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		db.addTxn(t, tx)
		return tx
	}
	first := sendFrom(wire.OutPoint{Hash: genesis.TxHash(), Index: 1})
	chain := sendFrom(wire.OutPoint{Hash: chainhash.Hash{3}, Index: 1})
	for i := 1; i < 999; i++ {
		chain = sendFrom(wire.OutPoint{Hash: chain.TxHash(), Index: 1})
	}
	root := sendFrom(wire.OutPoint{Hash: chain.TxHash(), Index: 1},
		wire.OutPoint{Hash: first.TxHash(), Index: 1})

	// The context is looked at before the search adds the 1001st
	// transaction, the first SEND, the last the graph held at the start.
	ctx := &growingContext{Context: context.Background(), db: db.Db, tx: genesis}
	var res [][]byte
	var panicked interface{}
	func() {
		defer func() { panicked = recover() }()
		hash := root.TxHash()
		res, err = db.Find(ctx, &hash, tokenID, &map[chainhash.Hash]struct{}{})
	}()
	if panicked != nil {
		t.Fatalf("the search panicked: %v", panicked)
	}
	if ctx.err != nil {
		t.Fatalf("adding the GENESIS: %v", ctx.err)
	}
	if err != nil || len(res) != 1002 {
		t.Fatalf("got %d transactions and %v, want 1002", len(res), err)
	}
	if !bytes.Equal(res[1001], db.raw[genesis.TxHash()]) {
		t.Error("the last transaction found is not the GENESIS")
	}
}

// TestCountReadsNothing ensures Count reads none of the transactions it
// counts, while Find reads each it returns once.  The graph search count of
// GetSlpTrustedValidation once serialized every transaction of the search
// only to count them.
func TestCountReadsNothing(t *testing.T) {
	const sends = 2000
	db, tip, tokenID := sendChain(t, sends)
	n, err := db.Count(context.Background(), tip, tokenID,
		&map[chainhash.Hash]struct{}{})
	if err != nil || n != sends+1 || db.reads != 0 || db.batches != nil {
		t.Fatalf("counted %d and %v, reading %d transactions, want %d, "+
			"reading none", n, err, db.reads, sends+1)
	}
	res, err := db.Find(context.Background(), tip, tokenID,
		&map[chainhash.Hash]struct{}{})
	if err != nil || len(res) != sends+1 || db.reads != sends+1 {
		t.Fatalf("found %d and %v, reading %d transactions, want %d, "+
			"reading each once", len(res), err, db.reads, sends+1)
	}
}

// readsContext is a context that records how many transactions db has read
// each time it is looked at, and whose Err returns err from its checks-th
// call on.
type readsContext struct {
	context.Context
	db     *testDb
	checks int
	err    error
	reads  []int
}

func (c *readsContext) Err() error {
	c.reads = append(c.reads, c.db.reads)
	if len(c.reads) == c.checks {
		return c.err
	}
	return nil
}

// TestFindReadsInBatches ensures a search reads the transactions it finds a
// batch of graphSearchCheckInterval at a time, each before it looks at its
// context again, so that a node reads them in one database transaction for
// each batch, and the search stops as soon as it would have read one at a
// time.
func TestFindReadsInBatches(t *testing.T) {
	const sends = 2500
	db, tip, tokenID := sendChain(t, sends)
	ctx := &readsContext{Context: context.Background(), db: db}
	res, err := db.Find(ctx, tip, tokenID, &map[chainhash.Hash]struct{}{})
	if err != nil || len(res) != sends+1 {
		t.Fatalf("got %d transactions and %v, want %d", len(res), err,
			sends+1)
	}
	checkBatches(t, "the search", db.batches, sends+1)
	if want := []int{0, 1000, 2000}; !slices.Equal(ctx.reads, want) {
		t.Errorf("the context was looked at with %v transactions read, "+
			"want %v", ctx.reads, want)
	}

	// A search finding a batch of transactions reads it once.
	db.batches = nil
	graph := db.getTokenGraph(tokenID)
	hash := *tip
	for i := 0; i < sends-graphSearchCheckInterval+1; i++ {
		txn, _ := graph.getTxn(&hash)
		hash = txn.inputs[0].Hash
	}
	res, err = db.Find(context.Background(), &hash, tokenID,
		&map[chainhash.Hash]struct{}{})
	if err != nil || len(res) != graphSearchCheckInterval {
		t.Fatalf("got %d transactions and %v, want %d", len(res), err,
			graphSearchCheckInterval)
	}
	checkBatches(t, "a search of a batch", db.batches,
		graphSearchCheckInterval)

	// The context ends at the third look, before the 2001st transaction
	// is found.
	db.reads, db.batches = 0, nil
	ctx = &readsContext{Context: context.Background(), db: db, checks: 3,
		err: context.Canceled}
	res, err = db.Find(ctx, tip, tokenID, &map[chainhash.Hash]struct{}{})
	if !errors.Is(err, context.Canceled) || res != nil || db.reads != 2000 {
		t.Errorf("the context ending: got %d transactions and %v, "+
			"reading %d, want %v, reading 2000", len(res), err, db.reads,
			context.Canceled)
	}
}

// TestFindReadError ensures a search that cannot read a transaction it finds
// fails with the error, which names the transaction, whether that is the
// transaction searched from or one it leads to, rather than returning the
// others, and reads no batch after the one that failed, while Count, which
// reads none, still counts it.
func TestFindReadError(t *testing.T) {
	const sends = 2500
	db, tip, tokenID := sendChain(t, sends)
	genesis := genesisHash(tokenID)

	// The search goes from the last send back to the GENESIS, which is
	// in the third batch, and the 600th transaction it finds is in the
	// first.
	graph := db.getTokenGraph(tokenID)
	middle := *tip
	for i := 1; i < 600; i++ {
		txn, _ := graph.getTxn(&middle)
		middle = txn.inputs[0].Hash
	}
	for _, test := range []struct {
		hash    *chainhash.Hash
		batches int
	}{
		{tip, 1},
		{&middle, 1},
		{&genesis, 3},
	} {
		hash := test.hash
		raw := db.raw[*hash]
		delete(db.raw, *hash)
		db.batches = nil
		res, err := db.Find(context.Background(), tip, tokenID,
			&map[chainhash.Hash]struct{}{})
		if !errors.Is(err, errNotStored) || res != nil ||
			!strings.Contains(err.Error(), hash.String()) {

			t.Errorf("without %v: got %d transactions and %v", hash,
				len(res), err)
		}
		if len(db.batches) != test.batches {
			t.Errorf("without %v: read %d batches, want %d", hash,
				len(db.batches), test.batches)
		}
		n, err := db.Count(context.Background(), tip, tokenID,
			&map[chainhash.Hash]struct{}{})
		if err != nil || n != sends+1 {
			t.Errorf("without %v: counted %d and %v, want %d", hash, n,
				err, sends+1)
		}
		db.raw[*hash] = raw
	}
}

// TestFindUnknownKind ensures a search that reaches an input spending a
// transaction whose slp message is of a kind it does not know fails, from the
// transaction searched from, and elsewhere leaves out that input and those
// after it, as the recursive search did.  The slp parser returns no such kind
// today.
func TestFindUnknownKind(t *testing.T) {
	db, _, tokenID := sendChain(t, 0)
	genesis := genesisHash(tokenID)
	send, err := metadatamaker.TokenType1Send(tokenID.CloneBytes(),
		[]uint64{1000})
	if err != nil {
		t.Fatal(err)
	}
	sendFrom := func(prev ...chainhash.Hash) chainhash.Hash {
		t.Helper()
		tx := wire.NewMsgTx(1)
		for i := range prev {
			tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: prev[i], Index: 1},
				nil))
		}
		tx.AddTxOut(&wire.TxOut{PkScript: send})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		db.addTxn(t, tx)
		return tx.TxHash()
	}

	// The SEND searched from spends one that spends the first of two sends,
	// then a transaction of an unknown kind, then the second send.
	unknown := chainhash.Hash{0x0f}
	db.getTokenGraph(tokenID).graph[unknown] = graphTxn{
		slp: slpOutputs{kind: slpOther}}
	first := sendFrom(genesis)
	second := sendFrom(first)
	mid := sendFrom(first, unknown, second)
	root := sendFrom(mid)

	res, err := db.Find(context.Background(), &root, tokenID,
		&map[chainhash.Hash]struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	want := []chainhash.Hash{root, mid, first, genesis}
	if len(res) != len(want) {
		t.Fatalf("got %d transactions, want %d", len(res), len(want))
	}
	for i := range want {
		if !bytes.Equal(res[i], db.raw[want[i]]) {
			t.Errorf("transaction %d is not %v", i, want[i])
		}
	}
	n, err := db.Count(context.Background(), &root, tokenID,
		&map[chainhash.Hash]struct{}{})
	if err != nil || n != len(want) {
		t.Errorf("counted %d and %v, want %d", n, err, len(want))
	}

	wantErr := fmt.Sprintf("txn %v was parsed as an unknown kind of slp "+
		"transaction", unknown)
	res, err = db.Find(context.Background(), &mid, tokenID,
		&map[chainhash.Hash]struct{}{})
	if fmt.Sprint(err) != wantErr || res != nil {
		t.Errorf("from the SEND spending it: got %d transactions and %v, "+
			"want %s", len(res), err, wantErr)
	}
	n, err = db.Count(context.Background(), &mid, tokenID,
		&map[chainhash.Hash]struct{}{})
	if fmt.Sprint(err) != wantErr || n != 0 {
		t.Errorf("counting from the SEND spending it: got %d and %v, want "+
			"%s", n, err, wantErr)
	}
}

// TestGraphTxnSize ensures a token's graph holds little of each transaction,
// and nothing that grows with its scripts, as it can hold millions: of a SEND
// spending 2 outputs, it holds the outpoints and what its slp message says,
// where it once held the whole transaction, scripts and all.  Each
// transaction is deserialized, as from a block or the network, so that its
// scripts share one buffer, which keeping any of them would hold.
func TestGraphTxnSize(t *testing.T) {
	const txns, scripts = 5000, 1000
	tokenID := chainhash.Hash{0x51}
	db := NewDb(nil)

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < txns; i++ {
		send, err := metadatamaker.TokenType1Send(tokenID[:],
			[]uint64{600 + uint64(i), 400})
		if err != nil {
			t.Fatal(err)
		}
		tx := wire.NewMsgTx(1)
		for j := 0; j < 2; j++ {
			prev := wire.OutPoint{Hash: chainhash.Hash{byte(i), byte(i >> 8),
				byte(j)}, Index: 1}
			tx.AddTxIn(wire.NewTxIn(&prev,
				bytes.Repeat([]byte{txscript.OP_TRUE}, scripts)))
		}
		tx.AddTxOut(&wire.TxOut{PkScript: send})
		for j := 0; j < 2; j++ {
			tx.AddTxOut(&wire.TxOut{Value: 546,
				PkScript: bytes.Repeat([]byte{txscript.OP_TRUE}, scripts)})
		}
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		var decoded wire.MsgTx
		if err := decoded.Deserialize(&buf); err != nil {
			t.Fatal(err)
		}
		if err := db.AddTxn(&decoded); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	held := (int64(after.HeapAlloc) - int64(before.HeapAlloc)) / txns
	t.Logf("the graph holds %d bytes for each transaction", held)
	if held > 400 {
		t.Errorf("the graph holds %d bytes for each of %d transactions of "+
			"%d bytes of scripts", held, txns, 4*scripts)
	}
	if size := db.getTokenGraph(&tokenID).size(); size != txns {
		t.Fatalf("the graph holds %d transactions, want %d", size, txns)
	}
}
