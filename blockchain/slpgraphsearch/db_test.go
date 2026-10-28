package slpgraphsearch

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
)

func TestSlpGraphSearch(t *testing.T) {
	inputTestsFile, err := os.Open("db_test.json")
	if err != nil {
		t.Fatal(err.Error())
	}
	data, err := io.ReadAll(inputTestsFile)
	defer inputTestsFile.Close()
	if err != nil {
		t.Fatal(err.Error())
	}

	type TestCase struct {
		Description         string
		TokenGraph          []string
		SearchTxid          string
		ClientValidityCache []string
		ExpectedResultTxids []string
	}
	var tests []TestCase
	err = json.Unmarshal(data, &tests)
	if err != nil {
		t.Fatal(err.Error())
	}

	// slpindex_test_graphsearch.json contains txids, they need to be reversed for BCHD
	reverseTxidFromString := func(txidHex string) (*chainhash.Hash, error) {
		txid, err := chainhash.NewHashFromStr(txidHex)
		if err != nil {
			return nil, err
		}
		return txid, nil
	}

	for _, test := range tests {
		hash, err := reverseTxidFromString(test.SearchTxid)
		if err != nil {
			t.Fatal(err.Error())
		}

		txns := make(map[chainhash.Hash]*wire.MsgTx)
		raw := make(map[chainhash.Hash][]byte)
		for _, txnHex := range test.TokenGraph { // TODO: rename this to txn DB
			txnBuf, err := hex.DecodeString(txnHex)
			if err != nil {
				t.Fatal(err.Error())
			}

			r := bytes.NewReader(txnBuf)
			msgTx := &wire.MsgTx{}
			if err := msgTx.Deserialize(r); err != nil {
				t.Fatal(err.Error())
			}
			txns[msgTx.TxHash()] = msgTx
			raw[msgTx.TxHash()] = txnBuf
		}

		tokenIDBuf, err := goslp.GetSlpTokenID(txns[*hash])
		if err != nil {
			t.Fatal(err.Error())
		}
		tokenID, err := chainhash.NewHashFromStr(hex.EncodeToString(tokenIDBuf))
		println(hex.EncodeToString(tokenID[:]))
		if err != nil {
			t.Fatal(err.Error())
		}
		tokenGraph := newTokenGraph(tokenID)
		if tx, ok := txns[*tokenID]; ok {
			err = tokenGraph.addTxn(tx)
			if err != nil {
				t.Fatal(err.Error())
			}
		} else {
			t.Fatalf("missing genesis transaction %s", hex.EncodeToString(tokenID[:]))
		}
		_ = tokenGraph.addTxn(txns[*tokenID])
		for _, txn := range txns {
			err = tokenGraph.addTxn(txn)
			if err != nil {
				t.Fatal(err.Error())
			}
		}
		if tokenGraph.size() != len(test.TokenGraph) {
			t.Fatal("token graph size does not match test inputs")
		}

		// load client's validity cache set (GS expects hashes, not txids)
		validityCacheSet := make(map[chainhash.Hash]struct{})
		for _, exTxid := range test.ClientValidityCache {
			hash, err := reverseTxidFromString(exTxid)
			if err != nil {
				t.Fatal(err.Error())
			}
			validityCacheSet[*hash] = struct{}{}
		}
		if len(validityCacheSet) != len(test.ClientValidityCache) {
			t.Fatal("exclude set size does not match test excludes")
		}

		// perform the graph search, reading the transactions from the test
		gsDb := NewDb(func(hashes []chainhash.Hash) ([][]byte, error) {
			txnBufs := make([][]byte, len(hashes))
			for i := range hashes {
				txnBuf, ok := raw[hashes[i]]
				if !ok {
					return nil, fmt.Errorf("txn %v is not in the test",
						hashes[i])
				}
				txnBufs[i] = txnBuf
			}
			return txnBufs, nil
		})
		gsDb.graphs[*tokenID] = tokenGraph
		gsRes, err := gsDb.Find(context.Background(), hash, tokenID, &validityCacheSet)
		if err != nil {
			t.Fatal(err.Error())
		}

		// check the graph search length matches the expected results length
		if len(test.ExpectedResultTxids) != len(gsRes) {
			t.Fatal("expected result has different size")
		}

		// create set of expected results
		expectedResults := make(map[chainhash.Hash]struct{})
		for _, resTxid := range test.ExpectedResultTxids {
			hash, err := reverseTxidFromString(resTxid)
			if err != nil {
				t.Fatal(err.Error())
			}
			expectedResults[*hash] = struct{}{}
		}

		// check each graph search results is part of the expected result
		for _, txnBuf := range gsRes {
			r := bytes.NewReader(txnBuf)
			msgTx := wire.MsgTx{}
			if err := msgTx.Deserialize(r); err != nil {
				t.Fatal(err.Error())
			}

			// check the expected txid is included
			if _, ok := expectedResults[msgTx.TxHash()]; ok != true {
				t.Fatalf("missing txid in graph search result: %v", msgTx.TxHash())
			}
		}
	}
}

// endingContext is a context whose Err returns err from its checks-th call
// on, counting the calls.
type endingContext struct {
	context.Context
	checks int
	err    error
	calls  int
}

func (c *endingContext) Err() error {
	c.calls++
	if c.calls >= c.checks {
		return c.err
	}
	return nil
}

// TestFindStopsWhenContextEnds ensures a graph search stops, returning the
// context's error, once its context ends, looking at it once per 1000
// transactions, so that a search through a large token's graph does not go
// on for a client that is gone.
func TestFindStopsWhenContextEnds(t *testing.T) {
	// A GENESIS and a chain of sends, each spending the token output of the
	// one before.
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
	const sends = 5000
	tip := genesis.TxHash()
	for i := 0; i < sends; i++ {
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

	// A search to the end looks at its context before the first
	// transaction and after every 1000: 6 times for 5001 transactions.
	live := &endingContext{Context: context.Background()}
	res, err := db.Find(live, &tip, tokenID, &map[chainhash.Hash]struct{}{})
	if err != nil || len(res) != sends+1 {
		t.Fatalf("got %d transactions and %v, want %d", len(res), err,
			sends+1)
	}
	if live.calls != 6 {
		t.Errorf("the context was looked at %d times, want 6", live.calls)
	}

	// The context ends at the first look, before anything is added, or at
	// the third, deep in the chain, from where the error comes back up.
	for _, test := range []struct {
		checks int
		err    error
	}{
		{1, context.Canceled},
		{3, context.Canceled},
		{3, context.DeadlineExceeded},
	} {
		ctx := &endingContext{Context: context.Background(),
			checks: test.checks, err: test.err}
		res, err := db.Find(ctx, &tip, tokenID, &map[chainhash.Hash]struct{}{})
		if !errors.Is(err, test.err) || res != nil {
			t.Errorf("%v at check %d: got %d transactions and %v",
				test.err, test.checks, len(res), err)
		}
		if ctx.calls != test.checks {
			t.Errorf("%v at check %d: the context was looked at %d times",
				test.err, test.checks, ctx.calls)
		}
	}
}

// TestDbStateRace ensures SetReady, which fails while the database loads, can
// run while SetLoaded ends the load, as the slp event handler adds the memory
// pool transactions that arrive during the load to graph search.  The error
// once read the state again without the lock.
func TestDbStateRace(t *testing.T) {
	for i := 0; i < 100; i++ {
		db := NewDb(nil)
		failed, ready := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(ready)
			for n := 0; db.SetReady() != nil; n++ {
				if n == 0 {
					close(failed)
				}
			}
		}()
		<-failed
		if err := db.SetLoaded(); err != nil {
			t.Fatal(err)
		}
		<-ready
		if !db.IsLoaded() || !db.IsReady() {
			t.Fatalf("loaded %v, ready %v", db.IsLoaded(), db.IsReady())
		}
	}
}
