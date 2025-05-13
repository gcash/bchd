// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

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

func fatalf(t *testing.T, err string, test []interface{}, i int) {
	t.Fatalf(
		"Error: %s \n Test %d - '%s' failed to execute! Test description: %s. UTXO: %s, TX: %s",
		err,
		i,
		test[0].(string),
		test[1].(string),
		test[5].(string),
		test[4].(string))
}

func TestVMlimitsAndBigInt2023Standard(t *testing.T) {

	BCH2023Standard := []string{
		"bch_2023_standard/core.bigint.mod.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.baseline.vmb_tests.json",
		"bch_2023_standard/core.nop.vmb_tests.json",
		"bch_2023_standard/core.limits.vmb_tests.json",
		"bch_2023_standard/core.push.minimal.vmb_tests.json",
		"bch_2023_standard/core.bigint.lessthan.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.bitwise.vmb_tests.json",
		"bch_2023_standard/core.bigint.within.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.hashing.vmb_tests.json",
		"bch_2023_standard/core.bigint.sub.vmb_tests.json",
		"bch_2023_standard/core.signature-checking.multisig.signing-serialization.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.hashing-bytes.packed.vmb_tests.json",
		"bch_2023_standard/core.push.ops.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.signature-checking.p2pkh.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.signature-checking.p2pk.vmb_tests.json",
		"bch_2023_standard/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2023_standard/core.bigint.negate.vmb_tests.json",
		"bch_2023_standard/core.bigint.abs.vmb_tests.json",
		"bch_2023_standard/core.push.data.limits.vmb_tests.json",
		"bch_2023_standard/core.bigint.booland.vmb_tests.json",
		"bch_2023_standard/core.bigint.1sub.vmb_tests.json",
		"bch_2023_standard/core.bigint.numnotequal.vmb_tests.json",
		"bch_2023_standard/core.bigint.min.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.roll.vmb_tests.json",
		"bch_2023_standard/core.signature-checking.multisig.m-of-3.vmb_tests.json",
		"bch_2023_standard/core.formatting.vmb_tests.json",
		"bch_2023_standard/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2023_standard/core.bigint-basics.vmb_tests.json",
		"bch_2023_standard/core.bigint.max.vmb_tests.json",
		"bch_2023_standard/core.bigint.div.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.stack.vmb_tests.json",
		"bch_2023_standard/core.inspection.vmb_tests.json",
		"bch_2023_standard/core.bigint.num2bin.vmb_tests.json",
		"bch_2023_standard/core.bigint.0notequal.vmb_tests.json",
		"bch_2023_standard/core.bigint.mul.vmb_tests.json",
		"bch_2023_standard/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.signature-checking.bms-ecdsa.vmb_tests.json",
		"bch_2023_standard/core.bigint.1add.vmb_tests.json",
		"bch_2023_standard/core.bigint.boolor.vmb_tests.json",
		"bch_2023_standard/core.bigint.numequal.vmb_tests.json",
		"bch_2023_standard/core.cashtokens.vmb_tests.json",
		"bch_2023_standard/core.push.numbers.vmb_tests.json",
		"bch_2023_standard/core.data-signatures.vmb_tests.json",
		"bch_2023_standard/core.disabled.vmb_tests.json",
		"bch_2023_standard/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2023_standard/core.push.data.vmb_tests.json",
		"bch_2023_standard/core.hashing.vmb_tests.json",
		"bch_2023_standard/core.copy.vmb_tests.json",
		"bch_2023_standard/core.bigint.numequalverify.vmb_tests.json",
		"bch_2023_standard/core.push.bytes.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.arithmetic.add-sub.vmb_tests.json",
		"bch_2023_standard/core.conditionals.vmb_tests.json",
		"bch_2023_standard/core.bigint.not.vmb_tests.json",
		"bch_2023_standard/core.bigint.greaterthan.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.signature-checking.bms-schnorr.vmb_tests.json",
		"bch_2023_standard/core.signing-serialization.vmb_tests.json",
		"bch_2023_standard/core.bigint.add.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.hashing-iters.packed.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.arithmetic.div-mod.vmb_tests.json",
		"bch_2023_standard/core.benchmarks.arithmetic.mul.vmb_tests.json",
		"bch_2023_standard/core.bigint.bin2num.vmb_tests.json",
	}

	for _, testFileName := range BCH2023Standard {

		testFile, err := ioutil.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".standard_limits.json"

		testLimitsFile, err := ioutil.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			utxos := make([]wire.TxOut, utxoCount)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				wire.ReadTxOut(r, 0, 0, &utxos[i])

				entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)

			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens

			err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
			if err != nil { // Failed TX, move to the next one
				fatalf(t, err.Error(), test, i)
			}

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil { // Failed TX, move to the next one
				fatalf(t, err.Error(), test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
			if !valid || err != nil {
				fatalf(t, err.Error(), test, i)
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
				fatalf(t, err.Error(), test, i)
			}
		}
	}
}

func TestVMlimitsAndBigInt2023NonStandard(t *testing.T) {

	BCH2023NonStandard := []string{
		"bch_2023_nonstandard/core.bigint.mod.vmb_tests.json",
		"bch_2023_nonstandard/core.nop.vmb_tests.json",
		"bch_2023_nonstandard/core.limits.vmb_tests.json",
		"bch_2023_nonstandard/core.push.minimal.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.lessthan.vmb_tests.json",
		"bch_2023_nonstandard/core.benchmarks.bitwise.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.within.vmb_tests.json",
		"bch_2023_nonstandard/core.benchmarks.hashing.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.sub.vmb_tests.json",
		"bch_2023_nonstandard/core.push.ops.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.negate.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.abs.vmb_tests.json",
		"bch_2023_nonstandard/core.push.data.limits.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.booland.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.1sub.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.numnotequal.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.min.vmb_tests.json",
		"bch_2023_nonstandard/core.formatting.vmb_tests.json",
		"bch_2023_nonstandard/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint-basics.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.max.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.div.vmb_tests.json",
		"bch_2023_nonstandard/core.benchmarks.stack.vmb_tests.json",
		"bch_2023_nonstandard/core.inspection.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.num2bin.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.0notequal.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.mul.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.1add.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.boolor.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.numequal.vmb_tests.json",
		"bch_2023_nonstandard/core.cashtokens.vmb_tests.json",
		"bch_2023_nonstandard/core.push.numbers.vmb_tests.json",
		"bch_2023_nonstandard/core.data-signatures.vmb_tests.json",
		"bch_2023_nonstandard/core.disabled.vmb_tests.json",
		"bch_2023_nonstandard/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2023_nonstandard/core.push.data.vmb_tests.json",
		"bch_2023_nonstandard/core.hashing.vmb_tests.json",
		"bch_2023_nonstandard/core.copy.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.numequalverify.vmb_tests.json",
		"bch_2023_nonstandard/core.push.bytes.vmb_tests.json",
		"bch_2023_nonstandard/core.benchmarks.arithmetic.add-sub.vmb_tests.json",
		"bch_2023_nonstandard/core.conditionals.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.not.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.greaterthan.vmb_tests.json",
		"bch_2023_nonstandard/core.signing-serialization.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.add.vmb_tests.json",
		"bch_2023_nonstandard/core.benchmarks.arithmetic.div-mod.vmb_tests.json",
		"bch_2023_nonstandard/core.benchmarks.arithmetic.mul.vmb_tests.json",
		"bch_2023_nonstandard/core.bigint.bin2num.vmb_tests.json",
	}

	for _, testFileName := range BCH2023NonStandard {

		testFile, err := ioutil.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".standard_limits.json"

		testLimitsFile, err := ioutil.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			utxos := make([]wire.TxOut, utxoCount)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				wire.ReadTxOut(r, 0, 0, &utxos[i])

				entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)

			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens
			flags ^= txscript.ScriptDiscourageUpgradableNops
			flags ^= txscript.ScriptVerifyInputSigChecks

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil { // Failed TX, move to the next one
				fatalf(t, err.Error(), test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
			if !valid || err != nil {
				fatalf(t, err.Error(), test, i)
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
				fatalf(t, err.Error(), test, i)
			}
		}
	}
}

func TestVMlimitsAndBigInt2023Invalid(t *testing.T) {

	BCH2023Invalid := []string{
		"bch_2023_invalid/core.bigint.mod.vmb_tests.json",
		"bch_2023_invalid/core.nop.vmb_tests.json",
		"bch_2023_invalid/core.limits.vmb_tests.json",
		"bch_2023_invalid/core.push.minimal.vmb_tests.json",
		"bch_2023_invalid/core.bigint.lessthan.vmb_tests.json",
		"bch_2023_invalid/core.benchmarks.bitwise.vmb_tests.json",
		"bch_2023_invalid/core.bigint.within.vmb_tests.json",
		"bch_2023_invalid/core.benchmarks.hashing.vmb_tests.json",
		"bch_2023_invalid/core.bigint-limits.binary.vmb_tests.json",
		"bch_2023_invalid/core.bigint.sub.vmb_tests.json",
		"bch_2023_invalid/core.push.ops.vmb_tests.json",
		"bch_2023_invalid/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2023_invalid/core.bigint-limits.ternary.vmb_tests.json",
		"bch_2023_invalid/core.bigint.negate.vmb_tests.json",
		"bch_2023_invalid/core.push-only.vmb_tests.json",
		"bch_2023_invalid/core.bigint.abs.vmb_tests.json",
		"bch_2023_invalid/core.push.data.limits.vmb_tests.json",
		"bch_2023_invalid/core.bigint.booland.vmb_tests.json",
		"bch_2023_invalid/core.bigint.1sub.vmb_tests.json",
		"bch_2023_invalid/core.bigint.numnotequal.vmb_tests.json",
		"bch_2023_invalid/core.bigint.min.vmb_tests.json",
		"bch_2023_invalid/core.benchmarks.roll.vmb_tests.json",
		"bch_2023_invalid/core.signature-checking.multisig.m-of-3.vmb_tests.json",
		"bch_2023_invalid/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2023_invalid/core.bigint-basics.vmb_tests.json",
		"bch_2023_invalid/core.bigint-limits.unary.vmb_tests.json",
		"bch_2023_invalid/core.bigint.max.vmb_tests.json",
		"bch_2023_invalid/core.bigint.div.vmb_tests.json",
		"bch_2023_invalid/core.benchmarks.stack.vmb_tests.json",
		"bch_2023_invalid/core.inspection.vmb_tests.json",
		"bch_2023_invalid/core.bigint.num2bin.vmb_tests.json",
		"bch_2023_invalid/core.bigint.0notequal.vmb_tests.json",
		"bch_2023_invalid/core.bigint.mul.vmb_tests.json",
		"bch_2023_invalid/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2023_invalid/core.bigint.1add.vmb_tests.json",
		"bch_2023_invalid/core.bigint.boolor.vmb_tests.json",
		"bch_2023_invalid/core.bigint.numequal.vmb_tests.json",
		"bch_2023_invalid/core.cashtokens.vmb_tests.json",
		"bch_2023_invalid/core.push.numbers.vmb_tests.json",
		"bch_2023_invalid/core.data-signatures.vmb_tests.json",
		"bch_2023_invalid/core.disabled.vmb_tests.json",
		"bch_2023_invalid/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2023_invalid/core.push.data.vmb_tests.json",
		"bch_2023_invalid/core.hashing.vmb_tests.json",
		"bch_2023_invalid/core.bigint.numequalverify.vmb_tests.json",
		"bch_2023_invalid/core.push.bytes.vmb_tests.json",
		"bch_2023_invalid/core.benchmarks.arithmetic.add-sub.vmb_tests.json",
		"bch_2023_invalid/core.conditionals.vmb_tests.json",
		"bch_2023_invalid/core.bigint.not.vmb_tests.json",
		"bch_2023_invalid/core.bigint.greaterthan.vmb_tests.json",
		"bch_2023_invalid/core.signing-serialization.vmb_tests.json",
		"bch_2023_invalid/core.bigint.add.vmb_tests.json",
		"bch_2023_invalid/core.benchmarks.arithmetic.div-mod.vmb_tests.json",
		"bch_2023_invalid/core.benchmarks.arithmetic.mul.vmb_tests.json",
		"bch_2023_invalid/core.bigint.bin2num.vmb_tests.json",
	}

	for _, testFileName := range BCH2023Invalid {

		testFile, err := ioutil.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".standard_limits.json"

		testLimitsFile, err := ioutil.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			utxos := make([]wire.TxOut, utxoCount)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				wire.ReadTxOut(r, 0, 0, &utxos[i])

				entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)

			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil { // Failed TX, move to the next one
				continue
				// fatalf(t, err, test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
			if !valid || err != nil {
				continue
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
				continue
				// fatalf(t, err.Error(), test, i)
			}

			if err == nil {
				t.Fatalf("Error! Test %d - %s executed without error! Test description: %s", i, test[0].(string), test[1].(string))
			}
		}
	}
}

func TestVMlimitsAndBigInt2025Standard(t *testing.T) {

	BCH2025Standard := []string{
		"bch_2025_standard/core.bigint.mod.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.baseline.vmb_tests.json",
		"bch_2025_standard/core.nop.vmb_tests.json",
		"bch_2025_standard/core.limits.vmb_tests.json",
		"bch_2025_standard/core.push.minimal.vmb_tests.json",
		"bch_2025_standard/core.bigint.lessthan.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.bitwise.vmb_tests.json",
		"bch_2025_standard/core.bigint.within.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.hashing.vmb_tests.json",
		"bch_2025_standard/core.bigint-limits.binary.vmb_tests.json",
		"bch_2025_standard/core.bigint.sub.vmb_tests.json",
		"bch_2025_standard/core.signature-checking.multisig.signing-serialization.vmb_tests.json",
		"bch_2025_standard/core.push.ops.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.signature-checking.p2pkh.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.signature-checking.p2pk.vmb_tests.json",
		"bch_2025_standard/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2025_standard/core.bigint-limits.ternary.vmb_tests.json",
		"bch_2025_standard/core.bigint.negate.vmb_tests.json",
		"bch_2025_standard/core.bigint.abs.vmb_tests.json",
		"bch_2025_standard/core.push.data.limits.vmb_tests.json",
		"bch_2025_standard/core.bigint.booland.vmb_tests.json",
		"bch_2025_standard/core.bigint.1sub.vmb_tests.json",
		"bch_2025_standard/core.bigint.numnotequal.vmb_tests.json",
		"bch_2025_standard/core.bigint.min.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.roll.vmb_tests.json",
		"bch_2025_standard/core.signature-checking.multisig.m-of-3.vmb_tests.json",
		"bch_2025_standard/core.formatting.vmb_tests.json",
		"bch_2025_standard/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2025_standard/core.bigint-basics.vmb_tests.json",
		"bch_2025_standard/core.bigint-limits.unary.vmb_tests.json",
		"bch_2025_standard/core.bigint.max.vmb_tests.json",
		"bch_2025_standard/core.bigint.div.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.stack.vmb_tests.json",
		"bch_2025_standard/core.inspection.vmb_tests.json",
		"bch_2025_standard/core.bigint.num2bin.vmb_tests.json",
		"bch_2025_standard/core.bigint.0notequal.vmb_tests.json",
		"bch_2025_standard/core.bigint.mul.vmb_tests.json",
		"bch_2025_standard/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.signature-checking.bms-ecdsa.vmb_tests.json",
		"bch_2025_standard/core.bigint.1add.vmb_tests.json",
		"bch_2025_standard/core.bigint.boolor.vmb_tests.json",
		"bch_2025_standard/core.bigint.numequal.vmb_tests.json",
		"bch_2025_standard/core.cashtokens.vmb_tests.json",
		"bch_2025_standard/core.push.numbers.vmb_tests.json",
		"bch_2025_standard/core.data-signatures.vmb_tests.json",
		"bch_2025_standard/core.disabled.vmb_tests.json",
		"bch_2025_standard/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2025_standard/core.push.data.vmb_tests.json",
		"bch_2025_standard/core.hashing.vmb_tests.json",
		"bch_2025_standard/core.copy.vmb_tests.json",
		"bch_2025_standard/core.bigint.numequalverify.vmb_tests.json",
		"bch_2025_standard/core.push.bytes.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.arithmetic.add-sub.vmb_tests.json",
		"bch_2025_standard/core.conditionals.vmb_tests.json",
		"bch_2025_standard/core.bigint.not.vmb_tests.json",
		"bch_2025_standard/core.bigint.greaterthan.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.signature-checking.bms-schnorr.vmb_tests.json",
		"bch_2025_standard/core.signing-serialization.vmb_tests.json",
		"bch_2025_standard/core.bigint.add.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.arithmetic.div-mod.vmb_tests.json",
		"bch_2025_standard/core.benchmarks.arithmetic.mul.vmb_tests.json",
		"bch_2025_standard/core.bigint.bin2num.vmb_tests.json",
	}

	for _, testFileName := range BCH2025Standard {

		testFile, err := ioutil.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".standard_limits.json"

		testLimitsFile, err := ioutil.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			utxos := make([]wire.TxOut, utxoCount)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				wire.ReadTxOut(r, 0, 0, &utxos[i])

				entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)
			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptAllowMay2025 | txscript.ScriptAllowMay2025StandardOnly

			err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
			if err != nil { // Failed TX, move to the next one
				fatalf(t, err.Error(), test, i)
			}

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil { // Failed TX, move to the next one
				fatalf(t, err.Error(), test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
			if !valid || err != nil {
				fatalf(t, err.Error(), test, i)
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
				fatalf(t, err.Error(), test, i)
			}

			densityControl := int64(testLimits[test[0].(string)].([]interface{})[0].(float64))
			maximumOperationCost := int64(testLimits[test[0].(string)].([]interface{})[1].(float64))
			operationCost := int64(testLimits[test[0].(string)].([]interface{})[2].(float64))

			if vm.GetMetrics().GetCompositeOPCost(true) != operationCost {
				str := fmt.Sprintf("test opcost: %d, script opcost: %d",
					operationCost,
					vm.GetMetrics().GetCompositeOPCost(true))
				fatalf(t, "operation cost did not match. "+str, test, i)
			}

			if vm.GetMetrics().GetHashDigestIterations() != densityControl {
				if err != nil {
					fatalf(t, "number of hash digest iterations did not match", test, i)
				}
			}

			if vm.GetMetrics().GetMaxOpCostLimit() != maximumOperationCost {
				str := fmt.Sprintf("test opcost limit: %d, script opcost limit: %d",
					maximumOperationCost,
					vm.GetMetrics().GetMaxOpCostLimit())
				fatalf(t, "max operation cost limit did not match. "+str, test, i)
			}
		}
	}
}

func TestVMlimitsAndBigInt2025NonStandard(t *testing.T) {

	BCH2025NonStandard := []string{
		"bch_2025_nonstandard/core.bigint.mod.vmb_tests.json",
		"bch_2025_nonstandard/core.nop.vmb_tests.json",
		"bch_2025_nonstandard/core.limits.vmb_tests.json",
		"bch_2025_nonstandard/core.push.minimal.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.lessthan.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.within.vmb_tests.json",
		"bch_2025_nonstandard/core.benchmarks.hashing.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint-limits.binary.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.sub.vmb_tests.json",
		"bch_2025_nonstandard/core.benchmarks.hashing-bytes.packed.vmb_tests.json",
		"bch_2025_nonstandard/core.push.ops.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint-limits.ternary.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.negate.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.abs.vmb_tests.json",
		"bch_2025_nonstandard/core.push.data.limits.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.booland.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.1sub.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.numnotequal.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.min.vmb_tests.json",
		"bch_2025_nonstandard/core.formatting.vmb_tests.json",
		"bch_2025_nonstandard/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint-basics.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint-limits.unary.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.max.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.div.vmb_tests.json",
		"bch_2025_nonstandard/core.benchmarks.stack.vmb_tests.json",
		"bch_2025_nonstandard/core.inspection.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.num2bin.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.0notequal.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.mul.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.1add.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.boolor.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.numequal.vmb_tests.json",
		"bch_2025_nonstandard/core.cashtokens.vmb_tests.json",
		"bch_2025_nonstandard/core.push.numbers.vmb_tests.json",
		"bch_2025_nonstandard/core.data-signatures.vmb_tests.json",
		"bch_2025_nonstandard/core.disabled.vmb_tests.json",
		"bch_2025_nonstandard/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2025_nonstandard/core.push.data.vmb_tests.json",
		"bch_2025_nonstandard/core.hashing.vmb_tests.json",
		"bch_2025_nonstandard/core.copy.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.numequalverify.vmb_tests.json",
		"bch_2025_nonstandard/core.push.bytes.vmb_tests.json",
		"bch_2025_nonstandard/core.benchmarks.arithmetic.add-sub.vmb_tests.json",
		"bch_2025_nonstandard/core.conditionals.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.not.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.greaterthan.vmb_tests.json",
		"bch_2025_nonstandard/core.signing-serialization.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.add.vmb_tests.json",
		"bch_2025_nonstandard/core.benchmarks.hashing-iters.packed.vmb_tests.json",
		"bch_2025_nonstandard/core.benchmarks.arithmetic.div-mod.vmb_tests.json",
		"bch_2025_nonstandard/core.benchmarks.arithmetic.mul.vmb_tests.json",
		"bch_2025_nonstandard/core.bigint.bin2num.vmb_tests.json",
	}

	for _, testFileName := range BCH2025NonStandard {

		testFile, err := ioutil.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".nonstandard_limits.json"

		testLimitsFile, err := ioutil.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			utxos := make([]wire.TxOut, utxoCount)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				wire.ReadTxOut(r, 0, 0, &utxos[i])

				entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)

			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptAllowMay2025
			flags ^= txscript.ScriptDiscourageUpgradableNops

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil { // Failed TX, move to the next one
				fatalf(t, err.Error(), test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
			if !valid || err != nil {
				fatalf(t, err.Error(), test, i)
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
				fatalf(t, err.Error(), test, i)
			}

			densityControl := int64(testLimits[test[0].(string)].([]interface{})[0].(float64))
			maximumOperationCost := int64(testLimits[test[0].(string)].([]interface{})[1].(float64))
			operationCost := int64(testLimits[test[0].(string)].([]interface{})[2].(float64))

			if vm.GetMetrics().GetCompositeOPCost(false) != operationCost {
				str := fmt.Sprintf("test opcost: %d, script opcost: %d",
					operationCost,
					vm.GetMetrics().GetCompositeOPCost(false))
				fatalf(t, "operation cost did not match. "+str, test, i)
			}

			if vm.GetMetrics().GetHashDigestIterations() != densityControl {
				if err != nil {
					fatalf(t, "number of hash digest iterations did not match", test, i)
				}
			}

			if vm.GetMetrics().GetMaxOpCostLimit() != maximumOperationCost {
				str := fmt.Sprintf("test opcost limit: %d, script opcost limit: %d",
					maximumOperationCost,
					vm.GetMetrics().GetMaxOpCostLimit())
				fatalf(t, "max operation cost limit did not match. "+str, test, i)
			}
		}
	}
}

func TestVMlimitsAndBigInt2025Invalid(t *testing.T) {

	BCH2025Invalid := []string{
		"bch_2025_invalid/core.bigint.mod.vmb_tests.json",
		"bch_2025_invalid/core.nop.vmb_tests.json",
		"bch_2025_invalid/core.limits.vmb_tests.json",
		"bch_2025_invalid/core.push.minimal.vmb_tests.json",
		"bch_2025_invalid/core.bigint.lessthan.vmb_tests.json",
		"bch_2025_invalid/core.benchmarks.bitwise.vmb_tests.json",
		"bch_2025_invalid/core.bigint.within.vmb_tests.json",
		"bch_2025_invalid/core.benchmarks.hashing.vmb_tests.json",
		"bch_2025_invalid/core.bigint-limits.binary.vmb_tests.json",
		"bch_2025_invalid/core.bigint.sub.vmb_tests.json",
		"bch_2025_invalid/core.push.ops.vmb_tests.json",
		"bch_2025_invalid/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2025_invalid/core.bigint-limits.ternary.vmb_tests.json",
		"bch_2025_invalid/core.bigint.negate.vmb_tests.json",
		"bch_2025_invalid/core.push-only.vmb_tests.json",
		"bch_2025_invalid/core.bigint.abs.vmb_tests.json",
		"bch_2025_invalid/core.push.data.limits.vmb_tests.json",
		"bch_2025_invalid/core.bigint.booland.vmb_tests.json",
		"bch_2025_invalid/core.bigint.1sub.vmb_tests.json",
		"bch_2025_invalid/core.bigint.numnotequal.vmb_tests.json",
		"bch_2025_invalid/core.bigint.min.vmb_tests.json",
		"bch_2025_invalid/core.benchmarks.roll.vmb_tests.json",
		"bch_2025_invalid/core.signature-checking.multisig.m-of-3.vmb_tests.json",
		"bch_2025_invalid/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2025_invalid/core.bigint-basics.vmb_tests.json",
		"bch_2025_invalid/core.bigint-limits.unary.vmb_tests.json",
		"bch_2025_invalid/core.bigint.max.vmb_tests.json",
		"bch_2025_invalid/core.bigint.div.vmb_tests.json",
		"bch_2025_invalid/core.benchmarks.stack.vmb_tests.json",
		"bch_2025_invalid/core.inspection.vmb_tests.json",
		"bch_2025_invalid/core.bigint.num2bin.vmb_tests.json",
		"bch_2025_invalid/core.bigint.mul.vmb_tests.json",
		"bch_2025_invalid/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2025_invalid/core.bigint.1add.vmb_tests.json",
		"bch_2025_invalid/core.bigint.boolor.vmb_tests.json",
		"bch_2025_invalid/core.bigint.numequal.vmb_tests.json",
		"bch_2025_invalid/core.cashtokens.vmb_tests.json",
		"bch_2025_invalid/core.push.numbers.vmb_tests.json",
		"bch_2025_invalid/core.disabled.vmb_tests.json",
		"bch_2025_invalid/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2025_invalid/core.push.data.vmb_tests.json",
		"bch_2025_invalid/core.bigint.numequalverify.vmb_tests.json",
		"bch_2025_invalid/core.push.bytes.vmb_tests.json",
		"bch_2025_invalid/core.conditionals.vmb_tests.json",
		"bch_2025_invalid/core.bigint.greaterthan.vmb_tests.json",
		"bch_2025_invalid/core.signing-serialization.vmb_tests.json",
		"bch_2025_invalid/core.bigint.add.vmb_tests.json",
		"bch_2025_invalid/core.bigint.bin2num.vmb_tests.json",
	}

	for _, testFileName := range BCH2025Invalid {

		testFile, err := ioutil.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".standard_limits.json"

		testLimitsFile, err := ioutil.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			utxos := make([]wire.TxOut, utxoCount)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				wire.ReadTxOut(r, 0, 0, &utxos[i])

				entry := blockchain.NewUtxoEntry(&utxos[i], 792771, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)

			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptAllowMay2025 | txscript.ScriptAllowMay2025StandardOnly

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil { // Failed TX, move to the next one
				continue
				// fatalf(t, err, test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx)
			if !valid || err != nil {
				continue
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
				continue
				// fatalf(t, err.Error(), test, i)
			}

			if testLimits[test[0].(string)] != nil {
				densityControl := int64(testLimits[test[0].(string)].([]interface{})[0].(float64))
				maximumOperationCost := int64(testLimits[test[0].(string)].([]interface{})[1].(float64))
				operationCost := int64(testLimits[test[0].(string)].([]interface{})[2].(float64))

				if vm.GetMetrics().GetCompositeOPCost(true) != operationCost {
					continue
					// fatalf(t, "operation cost did not match", test, i)
				}

				if vm.GetMetrics().GetHashDigestIterations() != densityControl {
					if err != nil {
						continue
						// fatalf(t, "number of hash digest iterations did not match", test, i)
					}
				}

				if vm.GetMetrics().GetMaxOpCostLimit() != maximumOperationCost {
					continue
					// fatalf(t, "max operation cost limit did not match", test, i)
				}
			}

			if err == nil {
				t.Fatalf("Error! Test %d - %s executed without error! Test description: %s", i, test[0].(string), test[1].(string))
			}
		}
	}
}
