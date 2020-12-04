package indexers

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/simpleledgerinc/goslp"
	"github.com/simpleledgerinc/goslp/v1parser"
)

// TestSlpInputUnitTests downloads SLP input unit tests and checks the input conditions for each test are met
func TestSlpInputUnitTests(t *testing.T) {
	inputTestsFile, err := os.Open("slpindex_test_inputs.json")
	if err != nil {
		t.Fatal(err.Error())
	}
	data, err := ioutil.ReadAll(inputTestsFile)
	defer inputTestsFile.Close()

	type TxItem struct {
		Txid  string
		Tx    string
		Valid bool
	}
	type TestCase struct {
		Description string
		When        []TxItem
		Should      []TxItem
	}
	var tests []TestCase
	err = json.Unmarshal(data, &tests)
	if err != nil {
		t.Fatal(err.Error())
	}

	for i, test := range tests {

		// create temporary db of input conditions
		entryDb := make(map[[32]byte]*SlpIndexEntry)

		for _, wen := range test.When {
			if !wen.Valid {
				continue
			}
			tx := wire.NewMsgTx(1)
			serializedTx, err := hex.DecodeString(wen.Tx)
			if err != nil {
				t.Fatal(err.Error())
			}

			// decode serialized transaction
			err = tx.BchDecode(bytes.NewReader(serializedTx), wire.ProtocolVersion, wire.LatestEncoding)
			if err != nil {
				t.Fatal(err.Error())
			}

			slpMsg, err := v1parser.ParseSLP(tx.TxOut[0].PkScript)
			if err != nil || slpMsg == nil {
				continue
			}
			tokenID, err := goslp.GetSlpTokenID(tx)
			tokenIDHash, err := chainhash.NewHash(tokenID[:])
			if err != nil {
				t.Fatal(err.Error())
			}
			entry := &SlpIndexEntry{
				TokenIDHash:    *tokenIDHash,
				TokenID:        0,
				SlpVersionType: slpMsg.TokenType(),
				SlpOpReturn:    tx.TxOut[0].PkScript,
			}
			hash := tx.TxHash()
			entryDb[hash] = entry
		}

		// add "When" and "Should" variables
		getSlpIndexEntry := func(txiHash *chainhash.Hash) (*SlpIndexEntry, error) {
			var hash [32]byte
			copy(hash[:], txiHash[:])
			slpEntry := entryDb[hash]
			if slpEntry == nil {
				return nil, errors.New("entry doesn't exist")
			}
			return slpEntry, nil
		}

		putTxIndexEntry := func(tx *wire.MsgTx, slpMsg v1parser.ParseResult, tokenIDHash *chainhash.Hash) error {
			return nil
		}

		// create transaction object
		tx := wire.NewMsgTx(1)
		serializedTx, err := hex.DecodeString(test.Should[0].Tx)
		if err != nil {
			t.Fatal(err.Error())
		}

		// decode serialized transaction
		err = tx.BchDecode(bytes.NewReader(serializedTx), wire.ProtocolVersion, wire.LatestEncoding)
		if err != nil {
			t.Fatal(err.Error())
		}

		// check the slp txns
		isValid, _, _ := CheckSlpTx(tx, getSlpIndexEntry, putTxIndexEntry)
		if isValid != test.Should[0].Valid {
			t.Errorf("Test %d: Expected valid = %t, got %t, \n%s", i, test.Should[0].Valid, isValid, test.Description)
		}
	}
}

func TestSlpGraphSearch(t *testing.T) {
	inputTestsFile, err := os.Open("slpindex_test_graphsearch.json")
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

		// load token graph db (GS expects hashes, not txids)
		tokenGraph := make(map[chainhash.Hash]*wire.MsgTx)
		for _, txnHex := range test.TokenGraph {
			txnBuf, err := hex.DecodeString(txnHex)
			if err != nil {
				t.Fatal(err.Error())
			}
			r := bytes.NewReader(txnBuf)
			msgTx := &wire.MsgTx{}
			msgTx.Deserialize(r)
			//msgTx := wire.NewMsgTx(1)
			//msgTx.BchDecode(r, wire.ProtocolVersion, wire.LatestEncoding)
			tokenGraph[msgTx.TxHash()] = msgTx
		}
		if len(tokenGraph) != len(test.TokenGraph) {
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
		gsRes, err := GraphSearchFor(*hash, &tokenGraph, &validityCacheSet)
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
