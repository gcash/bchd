// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain_test

// TODO rename file?
import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

func fromHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func fatalf(t *testing.T, err error, test []interface{}, i int) {
	t.Fatalf(
		"Error: %s \n Test %d - '%s' failed to execute! Test description: %s. UTXO: %s, TX: %s",
		err,
		i,
		test[0].(string),
		test[1].(string),
		test[5].(string),
		test[4].(string))
}

func TestCashTokensStandardValidOPCodes(t *testing.T) {

	file, err := ioutil.ReadFile("testdata/bch_vmb_tests_chip_cashtokens_standard.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	for i, test := range tests {
		r := bytes.NewReader(fromHex(test[5].(string)))
		utxoCount, err := wire.ReadVarInt(r, 0)

		utxos := make([]wire.TxOut, utxoCount)

		tx := wire.MsgTx{}
		txr := bytes.NewReader(fromHex(test[4].(string)))

		err = tx.BchDecode(txr, 0, 0)
		if err != nil {
			fatalf(t, err, test, i)
		}

		viewPoint := blockchain.NewUtxoViewpoint()

		for i := uint64(0); i < utxoCount; i++ {
			utxos[i] = wire.TxOut{}
			wire.ReadTxOut(r, 0, 0, &utxos[i])

			entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
			viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
		}

		bchutilTx := bchutil.NewTx(&tx)

		err = mempool.CheckTransactionStandard(bchutilTx, 792772, time.Now(), bchutil.Amount(1000), 2, true)
		if err != nil {
			fatalf(t, err, test, i)
		}

		flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptBip16 | txscript.ScriptVerifyNativeIntrospection

		err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
		if err != nil { // Failed TX, move to the next one
			fatalf(t, err, test, i)
		}

		err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
		if err != nil { // Failed TX, move to the next one
			fatalf(t, err, test, i)
		}

		cache := txscript.NewUtxoCache()
		for i := range tx.TxIn {
			cache.AddEntry(i, utxos[i])
		}

		valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
		if !valid || err != nil {
			fatalf(t, err, test, i)
		}

		txIdx := 0
		if len(test) == 7 {
			txIdx = int(test[6].(float64))
		}
		inputAmount := utxos[txIdx].Value

		vm, err := txscript.NewEngine(utxos[txIdx].PkScript, &tx, txIdx, flags, nil, nil, cache, inputAmount)
		if err == nil {
			err = vm.Execute()
		}

		if err != nil {
			fatalf(t, err, test, i)
		}
	}
}

func TestCashTokensStandardInvalidOPCodes(t *testing.T) {

	file, err := ioutil.ReadFile("testdata/bch_vmb_tests_chip_cashtokens_invalid.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	for i, test := range tests {
		r := bytes.NewReader(fromHex(test[5].(string)))
		utxoCount, _ := wire.ReadVarInt(r, 0)

		tx := wire.MsgTx{}
		txr := bytes.NewReader(fromHex(test[4].(string)))

		err = tx.BchDecode(txr, 0, 0)
		if err != nil {
			continue
		}

		utxos := make([]wire.TxOut, utxoCount)
		viewPoint := blockchain.NewUtxoViewpoint()
		for i := uint64(0); i < utxoCount; i++ {
			utxos[i] = wire.TxOut{}
			_, err = wire.ReadTxOut(r, 0, 0, &utxos[i])

			entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
			viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry

			if err != nil { // Read script failed
				break
			}
		}
		if err != nil { // Read script failed
			continue
		}

		cache := txscript.NewUtxoCache()
		for i := range tx.TxIn {
			cache.AddEntry(i, utxos[i])
		}
		bchutilTx := bchutil.NewTx(&tx)

		err = mempool.CheckTransactionStandard(bchutilTx, 792772, time.Time{}, bchutil.Amount(1000), 2, true)
		if err != nil {
			continue
		}

		flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptBip16 | txscript.ScriptVerifyNativeIntrospection

		err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		valid, validationErr := wire.RunCashTokensValidityAlgorithm(cache, &tx)

		if valid || validationErr == nil {

			txIdx := 0
			if len(test) == 7 {
				txIdx = int(test[6].(float64))
			}
			inputAmount := utxos[txIdx].Value

			vm, err := txscript.NewEngine(utxos[txIdx].PkScript, &tx, txIdx, flags, nil, nil, cache, inputAmount)

			if err == nil {
				err = vm.Execute()
			}

			if err == nil {
				t.Fatalf("Error! Test %d - %s executed without error! Test description: %s", i, test[0].(string), test[1].(string))
			}
		}
	}
}

func TestCashTokensNonStandardOPCodes(t *testing.T) {

	file, err := ioutil.ReadFile("testdata/bch_vmb_tests_chip_cashtokens_nonstandard.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	for i, test := range tests {
		r := bytes.NewReader(fromHex(test[5].(string)))
		utxoCount, _ := wire.ReadVarInt(r, 0)

		tx := wire.MsgTx{}
		txr := bytes.NewReader(fromHex(test[4].(string)))

		err = tx.BchDecode(txr, 0, 0)
		if err != nil {
			continue
		}

		utxos := make([]wire.TxOut, utxoCount)
		viewPoint := blockchain.NewUtxoViewpoint()
		for i := uint64(0); i < utxoCount; i++ {
			utxos[i] = wire.TxOut{}
			_, err = wire.ReadTxOut(r, 0, 0, &utxos[i])

			entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
			viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry

			if err != nil { // Read script failed
				break
			}
		}
		if err != nil { // Read script failed
			continue
		}

		cache := txscript.NewUtxoCache()
		for i := range tx.TxIn {
			cache.AddEntry(i, utxos[i])
		}
		bchutilTx := bchutil.NewTx(&tx)

		err = mempool.CheckTransactionStandard(bchutilTx, 792772, time.Time{}, bchutil.Amount(1000), 2, true)
		if err != nil {
			continue
		}

		flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptBip16 | txscript.ScriptVerifyNativeIntrospection

		err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		valid, validationErr := wire.RunCashTokensValidityAlgorithm(cache, &tx)

		if valid || validationErr == nil {

			txIdx := 0
			if len(test) == 7 {
				txIdx = int(test[6].(float64))
			}
			inputAmount := utxos[txIdx].Value

			vm, err := txscript.NewEngine(utxos[txIdx].PkScript, &tx, txIdx, flags, nil, nil, cache, inputAmount)

			if err == nil {
				err = vm.Execute()
			}

			if err == nil {
				t.Fatalf("Error! Test %d - %s executed without error! Test description: %s", i, test[0].(string), test[1].(string))
			}
		}
	}
}

func TestCashTokensBeforeActivationStandardValidOPCodes(t *testing.T) {

	file, err := ioutil.ReadFile("testdata/bch_vmb_tests_before_chip_cashtokens_standard.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	for i, test := range tests {
		r := bytes.NewReader(fromHex(test[5].(string)))
		utxoCount, err := wire.ReadVarInt(r, 0)

		utxos := make([]wire.TxOut, utxoCount)

		tx := wire.MsgTx{}
		txr := bytes.NewReader(fromHex(test[4].(string)))

		err = tx.BchDecode(txr, 0, 0)
		if err != nil {
			fatalf(t, err, test, i)
		}

		viewPoint := blockchain.NewUtxoViewpoint()

		for i := uint64(0); i < utxoCount; i++ {
			utxos[i] = wire.TxOut{}
			wire.ReadTxOut(r, 0, 0, &utxos[i])

			entry := blockchain.NewUtxoEntry(&utxos[i], 782771, false)
			viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
		}

		bchutilTx := bchutil.NewTx(&tx)

		err = mempool.CheckTransactionStandard(bchutilTx, 782772, time.Now(), bchutil.Amount(1000), 2, false)
		if err != nil {
			fatalf(t, err, test, i)
		}

		flags := txscript.StandardVerifyFlags | txscript.ScriptBip16 | txscript.ScriptVerifyNativeIntrospection

		err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
		if err != nil { // Failed TX, move to the next one
			fatalf(t, err, test, i)
		}

		err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
		if err != nil { // Failed TX, move to the next one
			fatalf(t, err, test, i)
		}

		cache := txscript.NewUtxoCache()
		for i := range tx.TxIn {
			cache.AddEntry(i, utxos[i])
		}

		valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
		if !valid || err != nil {
			fatalf(t, err, test, i)
		}

		txIdx := 0
		if len(test) == 7 {
			txIdx = int(test[6].(float64))
		}
		inputAmount := utxos[txIdx].Value

		vm, err := txscript.NewEngine(utxos[txIdx].PkScript, &tx, txIdx, flags, nil, nil, cache, inputAmount)
		if err == nil {
			err = vm.Execute()
		}

		if err != nil {
			fatalf(t, err, test, i)
		}
	}
}

func TestCashTokensBeforeActivationStandardInvalidOPCodes(t *testing.T) {

	file, err := ioutil.ReadFile("testdata/bch_vmb_tests_before_chip_cashtokens_invalid.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	for i, test := range tests {
		r := bytes.NewReader(fromHex(test[5].(string)))
		utxoCount, _ := wire.ReadVarInt(r, 0)

		tx := wire.MsgTx{}
		txr := bytes.NewReader(fromHex(test[4].(string)))

		err = tx.BchDecode(txr, 0, 0)
		if err != nil {
			continue
		}

		utxos := make([]wire.TxOut, utxoCount)
		viewPoint := blockchain.NewUtxoViewpoint()
		for i := uint64(0); i < utxoCount; i++ {
			utxos[i] = wire.TxOut{}
			_, err = wire.ReadTxOut(r, 0, 0, &utxos[i])

			entry := blockchain.NewUtxoEntry(&utxos[i], 782771, false)
			viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry

			if err != nil { // Read script failed
				break
			}
		}
		if err != nil { // Read script failed
			continue
		}

		cache := txscript.NewUtxoCache()
		for i := range tx.TxIn {
			cache.AddEntry(i, utxos[i])
		}
		bchutilTx := bchutil.NewTx(&tx)

		err = mempool.CheckTransactionStandard(bchutilTx, 782772, time.Time{}, bchutil.Amount(1000), 2, false)
		if err != nil {
			continue
		}

		flags := txscript.StandardVerifyFlags | txscript.ScriptBip16 | txscript.ScriptVerifyNativeIntrospection

		err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		valid, validationErr := wire.RunCashTokensValidityAlgorithm(cache, &tx)

		if valid || validationErr == nil {

			txIdx := 0
			if len(test) == 7 {
				txIdx = int(test[6].(float64))
			}

			if blockchain.IsPATFO(utxos[txIdx].TokenData, utxos[txIdx].PkScript, 100, 782772) {
				continue
			}

			inputAmount := utxos[txIdx].Value

			vm, err := txscript.NewEngine(utxos[txIdx].PkScript, &tx, txIdx, flags, nil, nil, cache, inputAmount)

			if err == nil {
				err = vm.Execute()
			}

			if err == nil {
				t.Fatalf("Error! Test %d - %s executed without error! Test description: %s", i, test[0].(string), test[1].(string))
			}
		}
	}
}

func TestCashTokensBeforeActivationNonStandardOPCodes(t *testing.T) {

	file, err := ioutil.ReadFile("testdata/bch_vmb_tests_before_chip_cashtokens_nonstandard.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	for i, test := range tests {
		r := bytes.NewReader(fromHex(test[5].(string)))
		utxoCount, _ := wire.ReadVarInt(r, 0)

		tx := wire.MsgTx{}
		txr := bytes.NewReader(fromHex(test[4].(string)))

		err = tx.BchDecode(txr, 0, 0)
		if err != nil {
			continue
		}

		utxos := make([]wire.TxOut, utxoCount)
		viewPoint := blockchain.NewUtxoViewpoint()
		for i := uint64(0); i < utxoCount; i++ {
			utxos[i] = wire.TxOut{}
			_, err = wire.ReadTxOut(r, 0, 0, &utxos[i])

			entry := blockchain.NewUtxoEntry(&utxos[i], 782771, false)
			viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry

			if err != nil { // Read script failed
				break
			}
		}
		if err != nil { // Read script failed
			continue
		}

		cache := txscript.NewUtxoCache()
		for i := range tx.TxIn {
			cache.AddEntry(i, utxos[i])
		}
		bchutilTx := bchutil.NewTx(&tx)

		err = mempool.CheckTransactionStandard(bchutilTx, 782772, time.Time{}, bchutil.Amount(1000), 2, false)
		if err != nil {
			continue
		}

		flags := txscript.StandardVerifyFlags | txscript.ScriptBip16 | txscript.ScriptVerifyNativeIntrospection

		err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
		if err != nil { // Failed TX, move to the next one
			continue
		}

		valid, validationErr := wire.RunCashTokensValidityAlgorithm(cache, &tx)

		if valid || validationErr == nil {

			txIdx := 0
			if len(test) == 7 {
				txIdx = int(test[6].(float64))
			}
			inputAmount := utxos[txIdx].Value

			vm, err := txscript.NewEngine(utxos[txIdx].PkScript, &tx, txIdx, flags, nil, nil, cache, inputAmount)

			if err == nil {
				err = vm.Execute()
			}

			if err == nil {
				t.Fatalf("Error! Test %d - %s executed without error! Test description: %s", i, test[0].(string), test[1].(string))
			}
		}
	}
}
