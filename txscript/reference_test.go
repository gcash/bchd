// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// fixedExcessiveBlockSize should not be the default -we want to ensure it will work in all cases
const fixedExcessiveBlockSize uint32 = 42111000

func init() {
	wire.SetLimits(fixedExcessiveBlockSize)
}

// scriptTestName returns a descriptive test name for the given reference script
// test data.
func scriptTestName(test []interface{}) (string, error) {

	// The test must consist of at least a signature script, public key script,
	// flags, and expected error.  Finally, it may optionally contain a comment.
	if len(test) < 4 || len(test) > 5 {
		return "", fmt.Errorf("invalid test length %d", len(test))
	}

	// Use the comment for the test name if one is specified, otherwise,
	// construct the name based on the signature script, public key script,
	// and flags.
	var name string
	if len(test) == 5 {
		name = fmt.Sprintf("test (%s)", test[4])
	} else {
		name = fmt.Sprintf("test ([%s, %s, %s])", test[0],
			test[1], test[2])
	}
	return name, nil
}

func scriptTestAmount(test []interface{}) (bchutil.Amount, error) {
	iAmounts, ok := test[0].([]interface{})
	if !ok {
		return bchutil.Amount(0), errors.New("test index 0 is not type []interface")
	}
	amount, ok := iAmounts[0].(float64)
	if !ok {
		return bchutil.Amount(0), errors.New("test index 0 is not type []float64")
	}
	return bchutil.Amount(amount * 100000000), nil
}

// parse hex string into a []byte.
func parseHex(tok string) ([]byte, error) {
	if !strings.HasPrefix(tok, "0x") {
		return nil, errors.New("not a hex number")
	}
	return hex.DecodeString(tok[2:])
}

// shortFormOps holds a map of opcode names to values for use in short form
// parsing.  It is declared here so it only needs to be created once.
var shortFormOps map[string]byte

// parseShortForm parses a string as as used in the Bitcoin Core reference tests
// into the script it came from.
//
// The format used for these tests is pretty simple if ad-hoc:
//   - Opcodes other than the push opcodes and unknown are present as
//     either OP_NAME or just NAME
//   - Plain numbers are made into push operations
//   - Numbers beginning with 0x are inserted into the []byte as-is (so
//     0x14 is OP_DATA_20)
//   - Single quoted strings are pushed as data
//   - Anything else is an error
func parseShortForm(script string) ([]byte, error) {
	// Only create the short form opcode map once.
	if shortFormOps == nil {
		ops := make(map[string]byte)
		for opcodeName, opcodeValue := range OpcodeByName {
			if strings.Contains(opcodeName, "OP_UNKNOWN") {
				continue
			}
			ops[opcodeName] = opcodeValue

			// The opcodes named OP_# can't have the OP_ prefix
			// stripped or they would conflict with the plain
			// numbers.  Also, since OP_FALSE and OP_TRUE are
			// aliases for the OP_0, and OP_1, respectively, they
			// have the same value, so detect those by name and
			// allow them.
			if (opcodeName == "OP_FALSE" || opcodeName == "OP_TRUE") ||
				(opcodeValue != OP_0 && (opcodeValue < OP_1 ||
					opcodeValue > OP_16)) {

				ops[strings.TrimPrefix(opcodeName, "OP_")] = opcodeValue
			}
		}
		shortFormOps = ops
	}

	// Split only does one separator so convert all \n and tab into  space.
	script = strings.Replace(script, "\n", " ", -1)
	script = strings.Replace(script, "\t", " ", -1)
	tokens := strings.Split(script, " ")
	builder := NewScriptBuilder()

	for _, tok := range tokens {
		if len(tok) == 0 {
			continue
		}
		// if parses as a plain number
		if num, err := strconv.ParseInt(tok, 10, 64); err == nil {
			builder.AddInt64(num)
			continue
		} else if bts, err := parseHex(tok); err == nil {
			// Concatenate the bytes manually since the test code
			// intentionally creates scripts that are too large and
			// would cause the builder to error otherwise.
			if builder.err == nil {
				builder.script = append(builder.script, bts...)
			}
		} else if len(tok) >= 2 &&
			tok[0] == '\'' && tok[len(tok)-1] == '\'' {
			builder.AddFullData([]byte(tok[1 : len(tok)-1]))
		} else if opcode, ok := shortFormOps[tok]; ok {
			builder.AddOp(opcode)
		} else {
			return nil, fmt.Errorf("bad token %q", tok)
		}

	}
	return builder.Script()
}

// parseScriptFlags parses the provided flags string from the format used in the
// reference tests into ScriptFlags suitable for use in the script engine.
func parseScriptFlags(flagStr string) (ScriptFlags, error) {
	var flags ScriptFlags

	sFlags := strings.Split(flagStr, ",")
	for _, flag := range sFlags {
		switch flag {
		case "":
			// Nothing.
		case "CHECKLOCKTIMEVERIFY":
			flags |= ScriptVerifyCheckLockTimeVerify
		case "CHECKSEQUENCEVERIFY":
			flags |= ScriptVerifyCheckSequenceVerify
		case "CLEANSTACK":
			flags |= ScriptVerifyCleanStack
		case "DERSIG":
			flags |= ScriptVerifyDERSignatures
		case "DISCOURAGE_UPGRADABLE_NOPS":
			flags |= ScriptDiscourageUpgradableNops
		case "LOW_S":
			flags |= ScriptVerifyLowS
		case "MINIMALDATA":
			flags |= ScriptVerifyMinimalData
		case "NONE":
			// Nothing.
		case "NULLDUMMY":
			flags |= ScriptStrictMultiSig
		case "NULLFAIL":
			flags |= ScriptVerifyNullFail
		case "P2SH":
			flags |= ScriptBip16
		case "SIGPUSHONLY":
			flags |= ScriptVerifySigPushOnly
		case "STRICTENC":
			flags |= ScriptVerifyStrictEncoding
		case "CHECKDATASIG":
			flags |= ScriptVerifyCheckDataSig
		case "SCHNORR":
			flags |= ScriptVerifySchnorr
		case "MINIMALIF":
			flags |= ScriptVerifyMinimalIf
		case "SIGHASH_FORKID":
			flags |= ScriptVerifyBip143SigHash
		case "ALLOWSEGWITRECOVERY":
			flags |= ScriptVerifyAllowSegwitRecovery
		case "COMPRESSED_PUBKEYTYPE":
			flags |= ScriptVerifyCompressedPubkey
		case "SCHNORR_MULTISIG":
			flags |= ScriptVerifySchnorrMultisig
		case "REVERSEBYTES":
			flags |= ScriptVerifyReverseBytes
		default:
			return flags, fmt.Errorf("invalid flag: %s", flag)
		}
	}
	return flags, nil
}

// parseExpectedResult parses the provided expected result string into allowed
// script error codes.  An error is returned if the expected result string is
// not supported.
func parseExpectedResult(expected string) ([]ErrorCode, error) {
	switch expected {
	case "OK":
		return nil, nil
	case "UNKNOWN_ERROR", "OPERAND_SIZE":
		return []ErrorCode{ErrNumberTooBig, ErrNumberTooSmall, ErrMinimalData, ErrInvalidInputLength}, nil
	case "PUBKEYTYPE", "NONCOMPRESSED_PUBKEY":
		return []ErrorCode{ErrPubKeyType}, nil
	case "SIG_DER", "SIG_BADLENGTH", "MISSING_FORKID":
		return []ErrorCode{ErrSigTooShort, ErrSigTooLong,
			ErrSigInvalidSeqID, ErrSigInvalidDataLen, ErrSigMissingSTypeID,
			ErrSigMissingSLen, ErrSigInvalidSLen,
			ErrSigInvalidRIntID, ErrSigZeroRLen, ErrSigNegativeR,
			ErrSigTooMuchRPadding, ErrSigInvalidSIntID,
			ErrSigZeroSLen, ErrSigNegativeS, ErrSigTooMuchSPadding,
			ErrInvalidSigHashType}, nil
	case "ILLEGAL_FORKID":
		return []ErrorCode{ErrInvalidSigHashType}, nil
	case "EVAL_FALSE":
		return []ErrorCode{ErrEvalFalse, ErrEmptyStack, ErrCleanStack}, nil
	case "EQUALVERIFY":
		return []ErrorCode{ErrEqualVerify}, nil
	case "NULLFAIL":
		return []ErrorCode{ErrNullFail}, nil
	case "SIG_HIGH_S":
		return []ErrorCode{ErrSigHighS}, nil
	case "SIG_HASHTYPE":
		return []ErrorCode{ErrInvalidSigHashType}, nil
	case "SIG_NULLDUMMY":
		return []ErrorCode{ErrSigNullDummy}, nil
	case "SIG_PUSHONLY":
		return []ErrorCode{ErrNotPushOnly}, nil
	case "CLEANSTACK":
		return []ErrorCode{ErrCleanStack}, nil
	case "BAD_OPCODE":
		return []ErrorCode{ErrReservedOpcode, ErrMalformedPush}, nil
	case "UNBALANCED_CONDITIONAL":
		return []ErrorCode{ErrUnbalancedConditional,
			ErrInvalidStackOperation}, nil
	case "OP_RETURN":
		return []ErrorCode{ErrEarlyReturn}, nil
	case "VERIFY":
		return []ErrorCode{ErrVerify}, nil
	case "INVALID_STACK_OPERATION", "INVALID_ALTSTACK_OPERATION":
		return []ErrorCode{ErrInvalidStackOperation, ErrDisabledOpcode}, nil
	case "DISABLED_OPCODE":
		return []ErrorCode{ErrDisabledOpcode}, nil
	case "DISCOURAGE_UPGRADABLE_NOPS":
		return []ErrorCode{ErrDiscourageUpgradableNOPs}, nil
	case "PUSH_SIZE", "IMPOSSIBLE_ENCODING", "INVALID_NUMBER_RANGE", "DIV_BY_ZERO", "MOD_BY_ZERO":
		return []ErrorCode{ErrElementTooBig, ErrNumberTooBig, ErrNumberTooSmall}, nil
	case "OP_COUNT":
		return []ErrorCode{ErrTooManyOperations}, nil
	case "STACK_SIZE":
		return []ErrorCode{ErrStackOverflow}, nil
	case "SCRIPT_SIZE":
		return []ErrorCode{ErrScriptTooBig}, nil
	case "ELEMENT_SIZE":
		return []ErrorCode{ErrElementTooBig}, nil
	case "PUBKEY_COUNT":
		return []ErrorCode{ErrInvalidPubKeyCount}, nil
	case "SIG_COUNT":
		return []ErrorCode{ErrInvalidSignatureCount}, nil
	case "MINIMALDATA", "MINIMALIF":
		return []ErrorCode{ErrMinimalData, ErrMinimalIf}, nil
	case "NEGATIVE_LOCKTIME":
		return []ErrorCode{ErrNegativeLockTime}, nil
	case "UNSATISFIED_LOCKTIME":
		return []ErrorCode{ErrUnsatisfiedLockTime}, nil
	case "CHECKDATASIGVERIFY":
		return []ErrorCode{ErrCheckDataSigVerify}, nil
	case "CHECKSIGVERIFY":
		return []ErrorCode{ErrCheckSigVerify}, nil
	case "CHECKMULTISIGVERIFY":
		return []ErrorCode{ErrCheckMultiSigVerify}, nil
	case "SPLIT_RANGE":
		return []ErrorCode{ErrNumberTooBig, ErrNumberTooSmall}, nil
	case "BITFIELD_SIZE":
		return []ErrorCode{ErrInvalidDummy}, nil
	case "BIT_RANGE", "INVALID_BIT_COUNT":
		return []ErrorCode{ErrInvalidBitCount}, nil
	case "SIG_NONSCHNORR":
		return []ErrorCode{ErrSigInvalidDataLen}, nil
	}

	return nil, fmt.Errorf("unrecognized expected result in test data: %v",
		expected)
}

// createSpendTx generates a basic spending transaction given the passed
// signature, witness and public key scripts.
func createSpendingTx(sigScript, pkScript []byte,
	outputValue int64) *wire.MsgTx {

	coinbaseTx := wire.NewMsgTx(wire.TxVersion)

	outPoint := wire.NewOutPoint(&chainhash.Hash{}, ^uint32(0))
	txIn := wire.NewTxIn(outPoint, []byte{OP_0, OP_0})
	txOut := wire.NewTxOut(outputValue, pkScript)
	coinbaseTx.AddTxIn(txIn)
	coinbaseTx.AddTxOut(txOut)

	spendingTx := wire.NewMsgTx(wire.TxVersion)
	coinbaseTxSha := coinbaseTx.TxHash()
	outPoint = wire.NewOutPoint(&coinbaseTxSha, 0)
	txIn = wire.NewTxIn(outPoint, sigScript)
	txOut = wire.NewTxOut(outputValue, nil)

	spendingTx.AddTxIn(txIn)
	spendingTx.AddTxOut(txOut)

	return spendingTx
}

// scriptWithInputVal wraps a target pkScript with the value of the output in
// which it is contained. The inputVal is necessary in order to properly
// validate inputs which spend nested, or native witness programs.
type scriptWithInputVal struct {
	inputVal int64
	pkScript []byte
}

// testScripts ensures all of the passed script tests execute with the expected
// results with or without using a signature cache, as specified by the
// parameter.
func testScripts(t *testing.T, tests [][]interface{}, useSigCache bool) {
	// Create a signature cache to use only if requested.
	var sigCache *SigCache
	if useSigCache {
		sigCache = NewSigCache(10)
	}

	for i, test := range tests {
		// "Format is: [[wit..., amount]?, scriptSig, scriptPubKey,
		//    flags, expected_scripterror, ... comments]"

		// Skip single line comments.
		if len(test) == 1 {
			continue
		}

		var inputAmt bchutil.Amount

		if len(test) == 6 {
			var err error
			inputAmt, err = scriptTestAmount(test)
			if err != nil {
				t.Errorf("TestScripts: invalid test #%d: %v", i, err)
				continue
			}
			test = test[1:]
		}

		// Construct a name for the test based on the comment and test
		// data.
		name, err := scriptTestName(test)
		if err != nil {
			t.Errorf("TestScripts: invalid test #%d: %v", i, err)
			continue
		}

		// Extract and parse the signature script from the test fields.
		scriptSigStr, ok := test[0].(string)
		if !ok {
			t.Errorf("%s: signature script is not a string", name)
			continue
		}
		scriptSig, err := parseShortForm(scriptSigStr)
		if err != nil {
			t.Errorf("%s: can't parse signature script: %v", name,
				err)
			continue
		}

		// Extract and parse the public key script from the test fields.
		scriptPubKeyStr, ok := test[1].(string)
		if !ok {
			t.Errorf("%s: public key script is not a string", name)
			continue
		}
		scriptPubKey, err := parseShortForm(scriptPubKeyStr)
		if err != nil {
			t.Errorf("%s: can't parse public key script: %v", name,
				err)
			continue
		}

		// Extract and parse the script flags from the test fields.
		flagsStr, ok := test[2].(string)
		if !ok {
			t.Errorf("%s: flags field is not a string", name)
			continue
		}
		flags, err := parseScriptFlags(flagsStr)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		// Extract and parse the expected result from the test fields.
		//
		// Convert the expected result string into the allowed script
		// error codes.  This is necessary because txscript is more
		// fine grained with its errors than the reference test data, so
		// some of the reference test data errors map to more than one
		// possibility.
		resultStr, ok := test[3].(string)
		if !ok {
			t.Errorf("%s: result field is not a string", name)
			continue
		}
		allowedErrorCodes, err := parseExpectedResult(resultStr)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		// Generate a transaction pair such that one spends from the
		// other and the provided signature and public key scripts are
		// used, then create a new engine to execute the scripts.
		tx := createSpendingTx(scriptSig, scriptPubKey,
			int64(inputAmt))

		vm, err := NewEngine(scriptPubKey, tx, 0, flags, sigCache, nil, nil,
			int64(inputAmt))
		if err == nil {
			err = vm.Execute()
		}

		// Ensure there were no errors when the expected result is OK.
		if resultStr == "OK" {
			if err != nil {
				t.Errorf("%s failed to execute: %v", name, err)
			}
			continue
		}

		// At this point an error was expected so ensure the result of
		// the execution matches it.
		success := false
		for _, code := range allowedErrorCodes {
			if IsErrorCode(err, code) {
				success = true
				break
			}
		}
		if !success {
			if serr, ok := err.(Error); ok {
				t.Errorf("%s: want error codes %v, got %v", name,
					allowedErrorCodes, serr.ErrorCode)
				continue
			}
			t.Errorf("%s: want error codes %v, got err: %v (%T)",
				name, allowedErrorCodes, err, err)
			continue
		}
	}
}

// TestScripts ensures all of the tests in script_tests.json execute with the
// expected results as defined in the test data.
func TestScripts(t *testing.T) {
	file, err := ioutil.ReadFile("data/script_tests.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	// Run all script tests with and without the signature cache.
	testScripts(t, tests, true)
	testScripts(t, tests, false)
}

// testVecF64ToUint32 properly handles conversion of float64s read from the JSON
// test data to unsigned 32-bit integers.  This is necessary because some of the
// test data uses -1 as a shortcut to mean max uint32 and direct conversion of a
// negative float to an unsigned int is implementation dependent and therefore
// doesn't result in the expected value on all platforms.  This function woks
// around that limitation by converting to a 32-bit signed integer first and
// then to a 32-bit unsigned integer which results in the expected behavior on
// all platforms.
func testVecF64ToUint32(f float64) uint32 {
	return uint32(int32(f))
}

// TestTxInvalidTests ensures all of the tests in tx_invalid.json fail as
// expected.
func TestTxInvalidTests(t *testing.T) {
	file, err := ioutil.ReadFile("data/tx_invalid.json")
	if err != nil {
		t.Fatalf("TestTxInvalidTests: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestTxInvalidTests couldn't Unmarshal: %v\n", err)
	}

	// form is either:
	//   ["this is a comment "]
	// or:
	//   [[[previous hash, previous index, previous scriptPubKey]...,]
	//	serializedTransaction, verifyFlags]
testloop:
	for i, test := range tests {
		inputs, ok := test[0].([]interface{})
		if !ok {
			continue
		}

		if len(test) != 3 {
			t.Errorf("bad test (bad length) %d: %v", i, test)
			continue

		}
		serializedhex, ok := test[1].(string)
		if !ok {
			t.Errorf("bad test (arg 2 not string) %d: %v", i, test)
			continue
		}
		serializedTx, err := hex.DecodeString(serializedhex)
		if err != nil {
			t.Errorf("bad test (arg 2 not hex %v) %d: %v", err, i,
				test)
			continue
		}

		tx, err := bchutil.NewTxFromBytes(serializedTx)
		if err != nil {
			t.Errorf("bad test (arg 2 not msgtx %v) %d: %v", err,
				i, test)
			continue
		}

		verifyFlags, ok := test[2].(string)
		if !ok {
			t.Errorf("bad test (arg 3 not string) %d: %v", i, test)
			continue
		}

		flags, err := parseScriptFlags(verifyFlags)
		if err != nil {
			t.Errorf("bad test %d: %v", i, err)
			continue
		}

		prevOuts := make(map[wire.OutPoint]scriptWithInputVal)
		for j, iinput := range inputs {
			input, ok := iinput.([]interface{})
			if !ok {
				t.Errorf("bad test (%dth input not array)"+
					"%d: %v", j, i, test)
				continue testloop
			}

			if len(input) < 3 || len(input) > 4 {
				t.Errorf("bad test (%dth input wrong length)"+
					"%d: %v", j, i, test)
				continue testloop
			}

			previoustx, ok := input[0].(string)
			if !ok {
				t.Errorf("bad test (%dth input hash not string)"+
					"%d: %v", j, i, test)
				continue testloop
			}

			prevhash, err := chainhash.NewHashFromStr(previoustx)
			if err != nil {
				t.Errorf("bad test (%dth input hash not hash %v)"+
					"%d: %v", j, err, i, test)
				continue testloop
			}

			idxf, ok := input[1].(float64)
			if !ok {
				t.Errorf("bad test (%dth input idx not number)"+
					"%d: %v", j, i, test)
				continue testloop
			}
			idx := testVecF64ToUint32(idxf)

			oscript, ok := input[2].(string)
			if !ok {
				t.Errorf("bad test (%dth input script not "+
					"string) %d: %v", j, i, test)
				continue testloop
			}

			script, err := parseShortForm(oscript)
			if err != nil {
				t.Errorf("bad test (%dth input script doesn't "+
					"parse %v) %d: %v", j, err, i, test)
				continue testloop
			}

			var inputValue float64
			if len(input) == 4 {
				inputValue, ok = input[3].(float64)
				if !ok {
					t.Errorf("bad test (%dth input value not int) "+
						"%d: %v", j, i, test)
					continue
				}
			}

			v := scriptWithInputVal{
				inputVal: int64(inputValue),
				pkScript: script,
			}
			prevOuts[*wire.NewOutPoint(prevhash, idx)] = v
		}

		for k, txin := range tx.MsgTx().TxIn {
			prevOut, ok := prevOuts[txin.PreviousOutPoint]
			if !ok {
				t.Errorf("bad test (missing %dth input) %d:%v",
					k, i, test)
				continue testloop
			}
			// These are meant to fail, so as soon as the first
			// input fails the transaction has failed. (some of the
			// test txns have good inputs, too..
			vm, err := NewEngine(prevOut.pkScript, tx.MsgTx(), k,
				flags, nil, nil, nil, prevOut.inputVal)
			if err != nil {
				continue testloop
			}

			err = vm.Execute()
			if err != nil {
				continue testloop
			}

		}
		t.Errorf("test (%d:%v) succeeded when should fail",
			i, test)
	}
}

// TestTxValidTests ensures all of the tests in tx_valid.json pass as expected.
func TestTxValidTests(t *testing.T) {
	file, err := ioutil.ReadFile("data/tx_valid.json")
	if err != nil {
		t.Fatalf("TestTxValidTests: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestTxValidTests couldn't Unmarshal: %v\n", err)
	}

	// form is either:
	//   ["this is a comment "]
	// or:
	//   [[[previous hash, previous index, previous scriptPubKey, input value]...,]
	//	serializedTransaction, verifyFlags]
testloop:
	for i, test := range tests {
		inputs, ok := test[0].([]interface{})
		if !ok {
			continue
		}

		if len(test) != 3 {
			t.Errorf("bad test (bad length) %d: %v", i, test)
			continue
		}
		serializedhex, ok := test[1].(string)
		if !ok {
			t.Errorf("bad test (arg 2 not string) %d: %v", i, test)
			continue
		}
		serializedTx, err := hex.DecodeString(serializedhex)
		if err != nil {
			t.Errorf("bad test (arg 2 not hex %v) %d: %v", err, i,
				test)
			continue
		}

		tx, err := bchutil.NewTxFromBytes(serializedTx)
		if err != nil {
			t.Errorf("bad test (arg 2 not msgtx %v) %d: %v", err,
				i, test)
			continue
		}

		verifyFlags, ok := test[2].(string)
		if !ok {
			t.Errorf("bad test (arg 3 not string) %d: %v", i, test)
			continue
		}

		flags, err := parseScriptFlags(verifyFlags)
		if err != nil {
			t.Errorf("bad test %d: %v", i, err)
			continue
		}

		prevOuts := make(map[wire.OutPoint]scriptWithInputVal)
		for j, iinput := range inputs {
			input, ok := iinput.([]interface{})
			if !ok {
				t.Errorf("bad test (%dth input not array)"+
					"%d: %v", j, i, test)
				continue
			}

			if len(input) < 3 || len(input) > 4 {
				t.Errorf("bad test (%dth input wrong length)"+
					"%d: %v", j, i, test)
				continue
			}

			previoustx, ok := input[0].(string)
			if !ok {
				t.Errorf("bad test (%dth input hash not string)"+
					"%d: %v", j, i, test)
				continue
			}

			prevhash, err := chainhash.NewHashFromStr(previoustx)
			if err != nil {
				t.Errorf("bad test (%dth input hash not hash %v)"+
					"%d: %v", j, err, i, test)
				continue
			}

			idxf, ok := input[1].(float64)
			if !ok {
				t.Errorf("bad test (%dth input idx not number)"+
					"%d: %v", j, i, test)
				continue
			}
			idx := testVecF64ToUint32(idxf)

			oscript, ok := input[2].(string)
			if !ok {
				t.Errorf("bad test (%dth input script not "+
					"string) %d: %v", j, i, test)
				continue
			}

			script, err := parseShortForm(oscript)
			if err != nil {
				t.Errorf("bad test (%dth input script doesn't "+
					"parse %v) %d: %v", j, err, i, test)
				continue
			}

			var inputValue float64
			if len(input) == 4 {
				inputValue, ok = input[3].(float64)
				if !ok {
					t.Errorf("bad test (%dth input value not int) "+
						"%d: %v", j, i, test)
					continue
				}
			}

			v := scriptWithInputVal{
				inputVal: int64(inputValue),
				pkScript: script,
			}
			prevOuts[*wire.NewOutPoint(prevhash, idx)] = v
		}

		for k, txin := range tx.MsgTx().TxIn {
			prevOut, ok := prevOuts[txin.PreviousOutPoint]
			if !ok {
				t.Errorf("bad test (missing %dth input) %d:%v",
					k, i, test)
				continue testloop
			}
			vm, err := NewEngine(prevOut.pkScript, tx.MsgTx(), k,
				flags, nil, nil, nil, prevOut.inputVal)
			if err != nil {
				t.Errorf("test (%d:%v:%d) failed to create "+
					"script: %v", i, test, k, err)
				continue
			}

			err = vm.Execute()
			if err != nil {
				t.Errorf("test (%d:%v:%d) failed to execute: "+
					"%v", i, test, k, err)
				continue
			}
		}
	}
}

// TestCalcSignatureHash runs the Bitcoin Core signature hash calculation tests
// in sighash.json.
// https://github.com/bitcoin/bitcoin/blob/master/src/test/data/sighash.json
func TestCalcLegacySignatureHash(t *testing.T) {
	file, err := ioutil.ReadFile("data/sighash_legacy.json")
	if err != nil {
		t.Fatalf("TestCalcSignatureHash: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestCalcSignatureHash couldn't Unmarshal: %v\n",
			err)
	}

	for i, test := range tests {
		if i == 0 {
			// Skip first line -- contains comments only.
			continue
		}
		if len(test) != 5 {
			t.Fatalf("TestCalcSignatureHash: Test #%d has "+
				"wrong length.", i)
		}
		var tx wire.MsgTx
		rawTx, _ := hex.DecodeString(test[0].(string))
		err := tx.Deserialize(bytes.NewReader(rawTx))
		if err != nil {
			t.Errorf("TestCalcSignatureHash failed test #%d: "+
				"Failed to parse transaction: %v", i, err)
			continue
		}

		subScript, _ := hex.DecodeString(test[1].(string))
		parsedScript, err := parseScript(subScript)
		if err != nil {
			t.Errorf("TestCalcSignatureHash failed test #%d: "+
				"Failed to parse sub-script: %v", i, err)
			continue
		}

		hashType := SigHashType(testVecF64ToUint32(test[3].(float64)))
		var hash []byte
		hash, err = calcLegacySignatureHash(parsedScript, hashType, &tx,
			int(test[2].(float64)))
		if err != nil {
			t.Errorf("TestCalcLegacySignatureHash failed test #%d: "+
				"calcLegacySignatureHash returned error: %v", i, err)
			continue
		}
		expectedHash, _ := chainhash.NewHashFromStr(test[4].(string))
		if !bytes.Equal(hash, expectedHash[:]) {
			t.Errorf("TestCalcLegacySignatureHash failed test #%d: "+
				"Signature hash mismatch.", i)
		}
	}
}

// This test data is pull out of the Bitcoin Cash mainnet blockchain
func TestCalcBip143SignatureHash(t *testing.T) {
	file, err := ioutil.ReadFile("data/sighash_bip143.json")
	if err != nil {
		t.Fatalf("TestCalcSignatureHash: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestCalcSignatureHash couldn't Unmarshal: %v\n",
			err)
	}
	fails := 0
	for i, test := range tests {
		if i == 0 {
			// Skip first line -- contains comments only.
			continue
		}
		if len(test) != 5 {
			t.Fatalf("TestCalcSignatureHash: Test #%d has "+
				"wrong length.", i)
		}
		var tx wire.MsgTx
		rawTx, _ := hex.DecodeString(test[0].(string))
		err := tx.Deserialize(bytes.NewReader(rawTx))
		if err != nil {
			t.Errorf("TestCalcSignatureHash failed test #%d: "+
				"Failed to parse transaction: %v", i, err)
			continue
		}

		subScript, _ := hex.DecodeString(test[1].(string))
		parsedScript, err := parseScript(subScript)
		if err != nil {
			t.Errorf("TestCalcSignatureHash failed test #%d: "+
				"Failed to parse sub-script: %v", i, err)
			continue
		}

		hashType := SigHashType(testVecF64ToUint32(test[3].(float64)))
		var hash []byte

		sigHashes := NewTxSigHashes(&tx)
		hash, err = calcBip143SignatureHash(parsedScript, sigHashes, hashType, &tx,
			int(test[2].(float64)), 0)
		if err != nil {
			t.Errorf("TestCalcBip143SignatureHash failed test #%d: "+
				"calcLegacySignatureHash returned error: %v", i, err)
			continue
		}
		expectedHash, _ := chainhash.NewHashFromStr(test[4].(string))
		if !bytes.Equal(hash, expectedHash[:]) {
			t.Errorf("TestCalcBip143SignatureHash failed test #%d: "+
				"Signature hash mismatch.", i)
			fails++
		}
	}
}
