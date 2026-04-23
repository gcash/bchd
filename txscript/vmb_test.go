// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

func TestMay2026Standard(t *testing.T) {

	BCH2025Standard := []string{
		"bch_2026_standard/chip.benchmarks.bitwise.vmb_tests.json",
		"bch_2026_standard/chip.bitwise.vmb_tests.json",
		"bch_2026_standard/chip.flow-control.vmb_tests.json",
		"bch_2026_standard/chip.functions.vmb_tests.json",
		"bch_2026_standard/chip.loops.vmb_tests.json",
		"bch_2026_standard/chip.p2s.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.arithmetic.add-sub.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.arithmetic.div-mod.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.arithmetic.mul.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.baseline.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.bitwise.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.hashing.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.roll.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.signature-checking.bms-ecdsa.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.signature-checking.bms-schnorr.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.signature-checking.p2pkh.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.signature-checking.p2pk.vmb_tests.json",
		"bch_2026_standard/core.benchmarks.stack.vmb_tests.json",
		"bch_2026_standard/core.bigint.0notequal.vmb_tests.json",
		"bch_2026_standard/core.bigint.1add.vmb_tests.json",
		"bch_2026_standard/core.bigint.1sub.vmb_tests.json",
		"bch_2026_standard/core.bigint.abs.vmb_tests.json",
		"bch_2026_standard/core.bigint.add.vmb_tests.json",
		"bch_2026_standard/core.bigint-basics.vmb_tests.json",
		"bch_2026_standard/core.bigint.bin2num.vmb_tests.json",
		"bch_2026_standard/core.bigint.booland.vmb_tests.json",
		"bch_2026_standard/core.bigint.boolor.vmb_tests.json",
		"bch_2026_standard/core.bigint.div.vmb_tests.json",
		"bch_2026_standard/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2026_standard/core.bigint.greaterthan.vmb_tests.json",
		"bch_2026_standard/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2026_standard/core.bigint.lessthan.vmb_tests.json",
		"bch_2026_standard/core.bigint-limits.binary.vmb_tests.json",
		"bch_2026_standard/core.bigint-limits.ternary.vmb_tests.json",
		"bch_2026_standard/core.bigint-limits.unary.vmb_tests.json",
		"bch_2026_standard/core.bigint.max.vmb_tests.json",
		"bch_2026_standard/core.bigint.min.vmb_tests.json",
		"bch_2026_standard/core.bigint.mod.vmb_tests.json",
		"bch_2026_standard/core.bigint.mul.vmb_tests.json",
		"bch_2026_standard/core.bigint.negate.vmb_tests.json",
		"bch_2026_standard/core.bigint.not.vmb_tests.json",
		"bch_2026_standard/core.bigint.num2bin.vmb_tests.json",
		"bch_2026_standard/core.bigint.numequalverify.vmb_tests.json",
		"bch_2026_standard/core.bigint.numequal.vmb_tests.json",
		"bch_2026_standard/core.bigint.numnotequal.vmb_tests.json",
		"bch_2026_standard/core.bigint.sub.vmb_tests.json",
		"bch_2026_standard/core.bigint.within.vmb_tests.json",
		"bch_2026_standard/core.cashtokens.vmb_tests.json",
		"bch_2026_standard/core.conditionals.vmb_tests.json",
		"bch_2026_standard/core.copy.vmb_tests.json",
		"bch_2026_standard/core.data-signatures.vmb_tests.json",
		"bch_2026_standard/core.disabled.vmb_tests.json",
		"bch_2026_standard/core.formatting.vmb_tests.json",
		"bch_2026_standard/core.hashing.vmb_tests.json",
		"bch_2026_standard/core.inspection.vmb_tests.json",
		"bch_2026_standard/core.limits.vmb_tests.json",
		"bch_2026_standard/core.nop.vmb_tests.json",
		"bch_2026_standard/core.push.bytes.vmb_tests.json",
		"bch_2026_standard/core.push.data.limits.vmb_tests.json",
		"bch_2026_standard/core.push.data.vmb_tests.json",
		"bch_2026_standard/core.push.minimal.vmb_tests.json",
		"bch_2026_standard/core.push.numbers.vmb_tests.json",
		"bch_2026_standard/core.push.ops.vmb_tests.json",
		"bch_2026_standard/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2026_standard/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2026_standard/core.signature-checking.multisig.m-of-3.vmb_tests.json",
		"bch_2026_standard/core.signature-checking.multisig.pubkey-validation.vmb_tests.json",
		"bch_2026_standard/core.signature-checking.multisig.signing-serialization.vmb_tests.json",
	}

	for _, testFileName := range BCH2025Standard {

		testFile, err := os.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("reading %s: %v", testFileName, err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("unmarshalling %s: %v", testFileName, err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".standard_limits.json"

		testLimitsFile, err := os.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("reading %s: %v", testLimitsFileName, err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("unmarshalling %s: %v", testLimitsFileName, err)
		}

		for i, test := range tests {
			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			if utxoCount != uint64(len(tx.TxIn)) {
				fatalf(t, fmt.Sprintf("utxoCount %d != len(tx.TxIn) %d",
					utxoCount, len(tx.TxIn)), test, i)
			}

			utxos := make([]wire.TxOut, utxoCount)
			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				if _, err := wire.ReadTxOut(r, 0, 0, &utxos[i]); err != nil {
					t.Fatal(err)
				}

				entry := blockchain.NewUtxoEntry(&utxos[i], 0, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)
			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptAllowMay2025 | txscript.ScriptAllowMay2025StandardOnly
			flags |= txscript.ScriptAllowMay2026

			err = mempool.CheckInputsStandard(bchutilTx, viewPoint, flags)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx, true)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}
			if !valid {
				fatalf(t, "cashtokens validity algorithm returned invalid", test, i)
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

			// Fixture schema: [effectiveInputSize (41+sigScriptSize), maxOpCost, opCost, description].
			// Field [0] is the input byte budget, not hash-iter count — verified by maxOpCost == [0]*800.
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

func TestMay2026NonStandard(t *testing.T) {

	BCH2025NonStandard := []string{
		"bch_2026_nonstandard/chip.bitwise.vmb_tests.json",
		"bch_2026_nonstandard/core.benchmarks.arithmetic.add-sub.vmb_tests.json",
		"bch_2026_nonstandard/core.benchmarks.arithmetic.div-mod.vmb_tests.json",
		"bch_2026_nonstandard/core.benchmarks.arithmetic.mul.vmb_tests.json",
		"bch_2026_nonstandard/core.benchmarks.hashing-bytes.packed.vmb_tests.json",
		"bch_2026_nonstandard/core.benchmarks.hashing-iters.packed.vmb_tests.json",
		"bch_2026_nonstandard/core.benchmarks.hashing.vmb_tests.json",
		"bch_2026_nonstandard/core.benchmarks.stack.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.0notequal.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.1add.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.1sub.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.abs.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.add.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.bin2num.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.booland.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.boolor.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.div.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.greaterthan.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.lessthan.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.max.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.min.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.mod.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.mul.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.negate.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.not.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.num2bin.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.numequalverify.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.numequal.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.numnotequal.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.sub.vmb_tests.json",
		"bch_2026_nonstandard/core.bigint.within.vmb_tests.json",
		"bch_2026_nonstandard/core.cashtokens.vmb_tests.json",
		"bch_2026_nonstandard/core.conditionals.vmb_tests.json",
		"bch_2026_nonstandard/core.inspection.vmb_tests.json",
		"bch_2026_nonstandard/core.limits.vmb_tests.json",
		"bch_2026_nonstandard/core.nop.vmb_tests.json",
		"bch_2026_nonstandard/core.push.data.vmb_tests.json",
		"bch_2026_nonstandard/core.signature-checking.multisig.m-of-15.vmb_tests.json",
	}

	for _, testFileName := range BCH2025NonStandard {

		testFile, err := os.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("reading %s: %v", testFileName, err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("unmarshalling %s: %v", testFileName, err)
		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".nonstandard_limits.json"

		testLimitsFile, err := os.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("reading %s: %v", testLimitsFileName, err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("unmarshalling %s: %v", testLimitsFileName, err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			if utxoCount != uint64(len(tx.TxIn)) {
				fatalf(t, fmt.Sprintf("utxoCount %d != len(tx.TxIn) %d",
					utxoCount, len(tx.TxIn)), test, i)
			}

			utxos := make([]wire.TxOut, utxoCount)
			viewPoint := blockchain.NewUtxoViewpoint()

			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				if _, err := wire.ReadTxOut(r, 0, 0, &utxos[i]); err != nil {
					t.Fatal(err)
				}
				entry := blockchain.NewUtxoEntry(&utxos[i], 0, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}

			bchutilTx := bchutil.NewTx(&tx)

			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptAllowMay2025
			flags &^= txscript.ScriptDiscourageUpgradableNops
			flags |= txscript.ScriptAllowMay2026

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx, true)
			if err != nil {
				fatalf(t, err.Error(), test, i)
			}
			if !valid {
				fatalf(t, "cashtokens validity algorithm returned invalid", test, i)
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

			// Fixture schema: [effectiveInputSize (41+sigScriptSize), maxOpCost, opCost, description].
			// Field [0] is the input byte budget, not hash-iter count — verified by maxOpCost == [0]*800.
			maximumOperationCost := int64(testLimits[test[0].(string)].([]interface{})[1].(float64))
			operationCost := int64(testLimits[test[0].(string)].([]interface{})[2].(float64))

			if vm.GetMetrics().GetCompositeOPCost(false) != operationCost {
				str := fmt.Sprintf("test opcost: %d, script opcost: %d",
					operationCost,
					vm.GetMetrics().GetCompositeOPCost(false))
				fatalf(t, "operation cost did not match. "+str, test, i)
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

func TestMay2026Invalid(t *testing.T) {

	BCH2025Invalid := []string{
		"bch_2026_invalid/chip.benchmarks.bitwise.vmb_tests.json",
		"bch_2026_invalid/chip.bitwise.vmb_tests.json",
		"bch_2026_invalid/chip.flow-control.vmb_tests.json",
		"bch_2026_invalid/chip.functions.vmb_tests.json",
		"bch_2026_invalid/chip.loops.vmb_tests.json",
		"bch_2026_invalid/core.benchmarks.bitwise.vmb_tests.json",
		"bch_2026_invalid/core.benchmarks.hashing.vmb_tests.json",
		"bch_2026_invalid/core.benchmarks.roll.vmb_tests.json",
		"bch_2026_invalid/core.benchmarks.stack.vmb_tests.json",
		"bch_2026_invalid/core.bigint.1add.vmb_tests.json",
		"bch_2026_invalid/core.bigint.1sub.vmb_tests.json",
		"bch_2026_invalid/core.bigint.abs.vmb_tests.json",
		"bch_2026_invalid/core.bigint.add.vmb_tests.json",
		"bch_2026_invalid/core.bigint-basics.vmb_tests.json",
		"bch_2026_invalid/core.bigint.bin2num.vmb_tests.json",
		"bch_2026_invalid/core.bigint.booland.vmb_tests.json",
		"bch_2026_invalid/core.bigint.boolor.vmb_tests.json",
		"bch_2026_invalid/core.bigint.div.vmb_tests.json",
		"bch_2026_invalid/core.bigint.greaterthanorequal.vmb_tests.json",
		"bch_2026_invalid/core.bigint.greaterthan.vmb_tests.json",
		"bch_2026_invalid/core.bigint.lessthanorequal.vmb_tests.json",
		"bch_2026_invalid/core.bigint.lessthan.vmb_tests.json",
		"bch_2026_invalid/core.bigint-limits.binary.vmb_tests.json",
		"bch_2026_invalid/core.bigint-limits.ternary.vmb_tests.json",
		"bch_2026_invalid/core.bigint-limits.unary.vmb_tests.json",
		"bch_2026_invalid/core.bigint.max.vmb_tests.json",
		"bch_2026_invalid/core.bigint.min.vmb_tests.json",
		"bch_2026_invalid/core.bigint.mod.vmb_tests.json",
		"bch_2026_invalid/core.bigint.mul.vmb_tests.json",
		"bch_2026_invalid/core.bigint.negate.vmb_tests.json",
		"bch_2026_invalid/core.bigint.num2bin.vmb_tests.json",
		"bch_2026_invalid/core.bigint.numequalverify.vmb_tests.json",
		"bch_2026_invalid/core.bigint.numequal.vmb_tests.json",
		"bch_2026_invalid/core.bigint.numnotequal.vmb_tests.json",
		"bch_2026_invalid/core.bigint.sub.vmb_tests.json",
		"bch_2026_invalid/core.bigint.within.vmb_tests.json",
		"bch_2026_invalid/core.cashtokens.vmb_tests.json",
		"bch_2026_invalid/core.conditionals.vmb_tests.json",
		"bch_2026_invalid/core.disabled.vmb_tests.json",
		"bch_2026_invalid/core.inspection.vmb_tests.json",
		"bch_2026_invalid/core.limits.vmb_tests.json",
		"bch_2026_invalid/core.nop.vmb_tests.json",
		"bch_2026_invalid/core.push.bytes.vmb_tests.json",
		"bch_2026_invalid/core.push.data.limits.vmb_tests.json",
		"bch_2026_invalid/core.push.data.vmb_tests.json",
		"bch_2026_invalid/core.push.minimal.vmb_tests.json",
		"bch_2026_invalid/core.push.numbers.vmb_tests.json",
		"bch_2026_invalid/core.push-only.vmb_tests.json",
		"bch_2026_invalid/core.push.ops.vmb_tests.json",
		"bch_2026_invalid/core.signature-checking.multisig.m-of-15.vmb_tests.json",
		"bch_2026_invalid/core.signature-checking.multisig.m-of-20.vmb_tests.json",
		"bch_2026_invalid/core.signature-checking.multisig.m-of-3.vmb_tests.json",
		"bch_2026_invalid/core.signature-checking.multisig.pubkey-validation.vmb_tests.json",
		"bch_2026_invalid/core.signing-serialization.vmb_tests.json",
	}

	for _, testFileName := range BCH2025Invalid {

		testFile, err := os.ReadFile("data/vmb_tests/" + testFileName)
		if err != nil {
			t.Fatalf("reading %s: %v", testFileName, err)
		}

		var tests [][]interface{}
		err = json.Unmarshal(testFile, &tests)
		if err != nil {
			t.Fatalf("unmarshalling %s: %v", testFileName, err)

		}

		testLimitsFileName := testFileName[:len(testFileName)-15] + ".standard_limits.json"

		testLimitsFile, err := os.ReadFile("data/vmb_tests/" + testLimitsFileName)
		if err != nil {
			t.Fatalf("TestScripts: %v\n", err)
		}

		var testLimits map[string]interface{}
		err = json.Unmarshal(testLimitsFile, &testLimits)
		if err != nil {
			t.Fatalf("unmarshalling %s: %v", testLimitsFileName, err)
		}

		for i, test := range tests {

			r := bytes.NewReader(fromHex(test[5].(string)))
			utxoCount, _ := wire.ReadVarInt(r, 0)

			tx := wire.MsgTx{}
			txr := bytes.NewReader(fromHex(test[4].(string)))

			err = tx.BchDecode(txr, 0, 0)
			if err != nil {
				// Invalid tx encoding is a valid rejection mode.
				continue
			}

			if utxoCount != uint64(len(tx.TxIn)) {
				// Malformed fixture; skip rather than allocate an unbounded slice.
				continue
			}

			utxos := make([]wire.TxOut, utxoCount)
			viewPoint := blockchain.NewUtxoViewpoint()

			readUtxosOK := true
			for i := uint64(0); i < utxoCount; i++ {
				utxos[i] = wire.TxOut{}
				if _, err := wire.ReadTxOut(r, 0, 0, &utxos[i]); err != nil {
					readUtxosOK = false
					break
				}
				entry := blockchain.NewUtxoEntry(&utxos[i], 0, false)
				viewPoint.Entries()[tx.TxIn[i].PreviousOutPoint] = entry
			}
			if !readUtxosOK {
				continue
			}

			bchutilTx := bchutil.NewTx(&tx)

			flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens | txscript.ScriptAllowMay2025 | txscript.ScriptAllowMay2025StandardOnly
			flags |= txscript.ScriptAllowMay2026

			err = blockchain.CheckTransactionSanity(bchutilTx, true, true, flags)
			if err != nil { // Failed TX, move to the next one
				continue
			}

			cache := txscript.NewUtxoCache()
			for i := range tx.TxIn {
				cache.AddEntry(i, utxos[i])
			}

			valid, err := wire.RunCashTokensValidityAlgorithm(cache, &tx, true)
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
			}

			t.Fatalf("Error! Test %d - %s executed without error! Test description: %s", i, test[0].(string), test[1].(string))
		}
	}
}
