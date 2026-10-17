// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// txTest is a parsed row of tx_valid.json or tx_invalid.json.
type txTest struct {
	name  string
	tx    *bchutil.Tx
	flags txscript.ScriptFlags
	prev  map[wire.OutPoint]wire.TxOut
}

// readTxTests reads tx_valid.json or tx_invalid.json.  Rows are either
// ["comment"] or [[[prevout hash, prevout index, prevout scriptPubKey,
// amount?]...], serializedTransaction, verifyFlags].
func readTxTests(t *testing.T, file string) []txTest {
	data, err := os.ReadFile("data/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]interface{}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}

	var tests []txTest
	for i, row := range rows {
		inputs, ok := row[0].([]interface{})
		if !ok {
			continue
		}
		str, _ := json.Marshal(row)
		name := fmt.Sprintf("%s #%d %s", file, i, str)
		test, err := parseTxTest(inputs, row)
		if err != nil {
			t.Errorf("%s: bad test: %v", name, err)
			continue
		}
		test.name = name
		tests = append(tests, test)
	}
	return tests
}

// parseTxTest parses one row of the transaction tests.
func parseTxTest(inputs []interface{}, row []interface{}) (txTest, error) {
	var test txTest
	if len(row) != 3 {
		return test, fmt.Errorf("bad length")
	}
	txHex, ok1 := row[1].(string)
	flagStr, ok2 := row[2].(string)
	if !ok1 || !ok2 {
		return test, fmt.Errorf("transaction or flags not a string")
	}
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return test, err
	}
	test.tx, err = bchutil.NewTxFromBytes(txBytes)
	if err != nil {
		return test, err
	}
	test.flags, err = txscript.ParseScriptFlags(flagStr)
	if err != nil {
		return test, err
	}

	test.prev = make(map[wire.OutPoint]wire.TxOut)
	for _, iinput := range inputs {
		input, ok := iinput.([]interface{})
		if !ok || len(input) < 3 || len(input) > 4 {
			return test, fmt.Errorf("bad input %v", iinput)
		}
		hashStr, ok1 := input[0].(string)
		idx, ok2 := input[1].(float64)
		scriptStr, ok3 := input[2].(string)
		if !ok1 || !ok2 || !ok3 {
			return test, fmt.Errorf("bad input %v", input)
		}
		hash, err := chainhash.NewHashFromStr(hashStr)
		if err != nil {
			return test, err
		}
		pkScript, err := txscript.ParseShortForm(scriptStr)
		if err != nil {
			return test, err
		}
		var amount float64
		if len(input) == 4 {
			if amount, ok = input[3].(float64); !ok {
				return test, fmt.Errorf("bad amount %v", input[3])
			}
		}
		op := wire.OutPoint{Hash: *hash, Index: uint32(int32(idx))}
		test.prev[op] = wire.TxOut{Value: int64(amount), PkScript: pkScript}
	}
	for i, txIn := range test.tx.MsgTx().TxIn {
		if _, ok := test.prev[txIn.PreviousOutPoint]; !ok {
			return test, fmt.Errorf("missing input %d", i)
		}
	}
	return test, nil
}

// checkRegularTx runs the context free checks of a transaction that is not a
// coinbase.
func checkRegularTx(tx *bchutil.Tx) error {
	if blockchain.IsCoinBase(tx) {
		return fmt.Errorf("coinbase")
	}
	return blockchain.CheckTransactionSanity(tx, false, false, 0)
}

// checkCoinbaseTx runs the context free checks of a coinbase transaction.
func checkCoinbaseTx(tx *bchutil.Tx) error {
	if !blockchain.IsCoinBase(tx) {
		return fmt.Errorf("not a coinbase")
	}
	return blockchain.CheckTransactionSanity(tx, false, false, 0)
}

// verifyInput runs the scripts of input i of a transaction test.
func verifyInput(test txTest, i int) error {
	msgTx := test.tx.MsgTx()
	utxos := txscript.NewUtxoCache()
	for j, txIn := range msgTx.TxIn {
		if prev, ok := test.prev[txIn.PreviousOutPoint]; ok {
			utxos.AddEntry(j, prev)
		}
	}
	prev := test.prev[msgTx.TxIn[i].PreviousOutPoint]
	vm, err := txscript.NewEngine(prev.PkScript, msgTx, i, test.flags,
		nil, nil, utxos, prev.Value)
	if err != nil {
		return err
	}
	return vm.Execute()
}

// TestTxValidTests ensures all of the tests in tx_valid.json pass as expected.
func TestTxValidTests(t *testing.T) {
	for _, test := range readTxTests(t, "tx_valid.json") {
		check, other := checkRegularTx, checkCoinbaseTx
		if blockchain.IsCoinBase(test.tx) {
			check, other = checkCoinbaseTx, checkRegularTx
		}
		if err := check(test.tx); err != nil {
			t.Errorf("%s: transaction checks failed: %v", test.name, err)
		}
		if other(test.tx) == nil {
			t.Errorf("%s: passes the checks of the other "+
				"transaction kind", test.name)
		}
		for i := range test.tx.MsgTx().TxIn {
			if err := verifyInput(test, i); err != nil {
				t.Errorf("%s: input %d failed: %v", test.name, i,
					err)
			}
		}
	}
}

// TestTxInvalidTests ensures all of the tests in tx_invalid.json fail as
// expected, either in the context free transaction checks or in an input.
func TestTxInvalidTests(t *testing.T) {
	for _, test := range readTxTests(t, "tx_invalid.json") {
		valid := checkRegularTx(test.tx) == nil
		for i := range test.tx.MsgTx().TxIn {
			if !valid {
				break
			}
			valid = verifyInput(test, i) == nil
		}
		if valid {
			t.Errorf("%s: succeeded when it should fail", test.name)
		}
	}
}
