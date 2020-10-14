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
