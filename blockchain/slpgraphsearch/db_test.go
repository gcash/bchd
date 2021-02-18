package slpgraphsearch

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/simpleledgerinc/goslp"
)

func TestSlpGraphSearch(t *testing.T) {
	inputTestsFile, err := os.Open("db_test.json")
	if err != nil {
		t.Fatal(err.Error())
	}
	data, err := ioutil.ReadAll(inputTestsFile)
	defer inputTestsFile.Close()

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
		for _, txnHex := range test.TokenGraph { // TODO: rename this to txn DB
			txnBuf, err := hex.DecodeString(txnHex)
			if err != nil {
				t.Fatal(err.Error())
			}

			r := bytes.NewReader(txnBuf)
			msgTx := &wire.MsgTx{}
			msgTx.Deserialize(r)
			txns[msgTx.TxHash()] = msgTx
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
		tokenGraph.addTxn(txns[*tokenID])
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

		// perform the graph search
		gsDb := NewDb()
		gsDb.graphs[*tokenID] = tokenGraph
		gsRes, err := gsDb.Find(hash, tokenID, &validityCacheSet)
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
			msgTx.Deserialize(r)

			// check the expected txid is included
			if _, ok := expectedResults[msgTx.TxHash()]; ok != true {
				t.Fatalf("missing txid in graph search result: %v", msgTx.TxHash())
			}
		}
	}
}
