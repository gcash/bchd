package indexers

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/simpleledgerinc/goslp"
	"github.com/simpleledgerinc/goslp/v1parser"
)

// TestSlpMesesageUnitTests downloads SLP parser unit tests and checks the parser throws for each test where code != nil
func TestSlpMessageUnitTests(t *testing.T) {
	resp, err := http.Get("https://raw.githubusercontent.com/simpleledger/slp-unit-test-data/master/script_tests.json")
	if err != nil {
		t.Fatal("cannot download unit tests")
	}
	data, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	type TestCase struct {
		Msg    string
		Script string
		Code   *float64
	}
	var tests []TestCase
	err = json.Unmarshal(data, &tests)
	if err != nil {
		t.Fatal(err.Error())
	}
	for _, test := range tests {
		slpbuf, _ := hex.DecodeString(test.Script)
		_, err := v1parser.ParseSLP(slpbuf)
		if err != nil {
			if test.Code != nil {
				continue
			}
			t.Fatal("goslp parser did not throw an error")
		}
	}
}

// TestSlpInputUnitTests downloads SLP input unit tests and checks the input conditions for each test are met
func TestSlpInputUnitTests(t *testing.T) {
	resp, err := http.Get("https://raw.githubusercontent.com/simpleledger/slp-unit-test-data/master/tx_input_tests.json")
	if err != nil {
		t.Fatal("cannot download unit tests")
	}
	data, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

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

	for _, test := range tests {

		// create temporary db of input conditions
		_entryDb := make(map[[32]byte]*SlpIndexEntry)

		for _, wen := range test.When {
			if !wen.Valid {
				continue
			}
			tx := wire.NewMsgTx(1)
			serializedTx, err := hex.DecodeString(wen.Tx)
			if err != nil {
				panic(err.Error())
			}
			tx.Deserialize(bytes.NewReader(serializedTx))
			slpMsg, err := v1parser.ParseSLP(tx.TxOut[0].PkScript)
			if err != nil || slpMsg == nil {
				continue
			}
			_tokenid, err := goslp.GetSlpTokenID(tx)
			tokenIDHash, _ := chainhash.NewHash(_tokenid[:])
			entry := &SlpIndexEntry{
				TokenIDHash:    *tokenIDHash,
				TokenID:        0,
				SlpVersionType: uint16(slpMsg.TokenType),
				SlpOpReturn:    tx.TxOut[0].PkScript,
			}
			_hash := tx.TxHash()
			_entryDb[_hash] = entry
		}

		// add "When" and "Should" variables
		_getSlpIndexEntry := func(txiHash *chainhash.Hash) (*SlpIndexEntry, error) {
			var _hash [32]byte
			copy(_hash[:], txiHash[:])
			slpEntry := _entryDb[_hash]
			if slpEntry == nil {
				return nil, errors.New("entry doesn't exist")
			}
			return slpEntry, nil
		}

		_putTxIndexEntry := func(tx *wire.MsgTx, slpMsg *v1parser.ParseResult, tokenIDHash *chainhash.Hash) error {
			return nil
		}

		// create transaction object
		tx := wire.NewMsgTx(1)
		serializedTx, err := hex.DecodeString(test.Should[0].Tx)
		if err != nil {
			panic(err.Error())
		}
		tx.Deserialize(bytes.NewReader(serializedTx))

		// check the slp txns
		isValid := CheckSlpTx(tx, _getSlpIndexEntry, _putTxIndexEntry)
		if !isValid && !test.Should[0].Valid {
			continue
		} else if isValid && test.Should[0].Valid {
			continue
		} else {
			t.Fatal("input unit test failed")
		}
	}
}
