// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// fixedExcessiveBlockSize should not be the default -we want to ensure it will work in all cases
const fixedExcessiveBlockSize uint32 = 42111000

func init() {
	wire.SetLimits(fixedExcessiveBlockSize)
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
		// if parses as a plain number, saturating out of range values
		// like strtoll
		if num, err := strconv.ParseInt(tok, 10, 64); err == nil ||
			errors.Is(err, strconv.ErrRange) {

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

// Script flags with the bit values BCHN uses, so that the random flag changes
// in testScripts match BCHN's script tests.
const (
	bchnP2SH                     uint32 = 1 << 0
	bchnStrictEnc                uint32 = 1 << 1
	bchnDERSig                   uint32 = 1 << 2
	bchnLowS                     uint32 = 1 << 3
	bchnNullDummy                uint32 = 1 << 4
	bchnSigPushOnly              uint32 = 1 << 5
	bchnMinimalData              uint32 = 1 << 6
	bchnDiscourageUpgradableNops uint32 = 1 << 7
	bchnCleanStack               uint32 = 1 << 8
	bchnCheckLockTimeVerify      uint32 = 1 << 9
	bchnCheckSequenceVerify      uint32 = 1 << 10
	bchnMinimalIf                uint32 = 1 << 13
	bchnNullFail                 uint32 = 1 << 14
	bchnSigHashForkID            uint32 = 1 << 16
	bchnDisallowSegwitRecovery   uint32 = 1 << 20
	bchnSchnorrMultisig          uint32 = 1 << 21
	bchnInputSigChecks           uint32 = 1 << 22
	bchnP2SH32                   uint32 = 1 << 24
	bchnTokens                   uint32 = 1 << 25
	bchnMay2025                  uint32 = 1 << 26
	bchnVMLimitsStandard         uint32 = 1 << 27
	bchnMay2026                  uint32 = 1 << 28

	// bchnNotRestrictive are the flags that enable features, so adding or
	// removing them can change the result either way.
	bchnNotRestrictive = bchnSigHashForkID | bchnSchnorrMultisig |
		bchnTokens | bchnP2SH32 | bchnMay2025 | bchnMay2026
)

// bchnFlagNames maps the flag names of the reference tests to their bits.
// NULLDUMMY is not a BCHN flag and is only used by BCHD's own rows.
var bchnFlagNames = map[string]uint32{
	"NONE":                       0,
	"P2SH":                       bchnP2SH,
	"STRICTENC":                  bchnStrictEnc,
	"DERSIG":                     bchnDERSig,
	"LOW_S":                      bchnLowS,
	"NULLDUMMY":                  bchnNullDummy,
	"SIGPUSHONLY":                bchnSigPushOnly,
	"MINIMALDATA":                bchnMinimalData,
	"DISCOURAGE_UPGRADABLE_NOPS": bchnDiscourageUpgradableNops,
	"CLEANSTACK":                 bchnCleanStack,
	"MINIMALIF":                  bchnMinimalIf,
	"NULLFAIL":                   bchnNullFail,
	"CHECKLOCKTIMEVERIFY":        bchnCheckLockTimeVerify,
	"CHECKSEQUENCEVERIFY":        bchnCheckSequenceVerify,
	"SIGHASH_FORKID":             bchnSigHashForkID,
	"DISALLOW_SEGWIT_RECOVERY":   bchnDisallowSegwitRecovery,
	"SCHNORR_MULTISIG":           bchnSchnorrMultisig,
	"INPUT_SIGCHECKS":            bchnInputSigChecks,
	"ENABLE_TOKENS":              bchnTokens,
	"P2SH_32":                    bchnP2SH32,
	"ENABLE_MAY2025":             bchnMay2025,
	"VM_LIMITS_STANDARD":         bchnVMLimitsStandard,
	"ENABLE_MAY2026":             bchnMay2026,
}

// parseBCHNFlags parses a comma separated list of flag names into flag bits.
func parseBCHNFlags(flagStr string) (uint32, error) {
	var bits uint32
	if flagStr == "" {
		return 0, nil
	}
	for name := range strings.SplitSeq(flagStr, ",") {
		bit, ok := bchnFlagNames[name]
		if !ok {
			return 0, fmt.Errorf("invalid flag: %s", name)
		}
		bits |= bit
	}
	return bits, nil
}

// bchnScriptFlags converts flag bits to engine flags.  Upgrades that have no
// flag in the reference tests are always enabled.
func bchnScriptFlags(bits uint32) ScriptFlags {
	flags := ScriptVerifyCheckDataSig | ScriptVerifySchnorr |
		ScriptVerifyReverseBytes | ScriptVerify64BitIntegers |
		ScriptVerifyNativeIntrospection
	if bits&bchnDisallowSegwitRecovery == 0 {
		flags |= ScriptVerifyAllowSegwitRecovery
	}
	// SIGHASH_FORKID also turns on STRICTENC.
	if bits&bchnSigHashForkID != 0 {
		flags |= ScriptVerifyBip143SigHash | ScriptVerifyStrictEncoding
	}
	// P2SH32 is enabled together with tokens.
	if bits&(bchnTokens|bchnP2SH32) != 0 {
		flags |= ScriptAllowCashTokens
	}
	conv := []struct {
		bit  uint32
		flag ScriptFlags
	}{
		{bchnP2SH, ScriptBip16},
		{bchnStrictEnc, ScriptVerifyStrictEncoding},
		{bchnDERSig, ScriptVerifyDERSignatures},
		{bchnLowS, ScriptVerifyLowS},
		{bchnNullDummy, ScriptStrictMultiSig},
		{bchnSigPushOnly, ScriptVerifySigPushOnly},
		{bchnMinimalData, ScriptVerifyMinimalData},
		{bchnDiscourageUpgradableNops, ScriptDiscourageUpgradableNops},
		{bchnCleanStack, ScriptVerifyCleanStack},
		{bchnCheckLockTimeVerify, ScriptVerifyCheckLockTimeVerify},
		{bchnCheckSequenceVerify, ScriptVerifyCheckSequenceVerify},
		{bchnMinimalIf, ScriptVerifyMinimalIf},
		{bchnNullFail, ScriptVerifyNullFail},
		{bchnSchnorrMultisig, ScriptVerifySchnorrMultisig},
		{bchnInputSigChecks, ScriptVerifyInputSigChecks},
		{bchnMay2025, ScriptAllowMay2025},
		{bchnVMLimitsStandard, ScriptAllowMay2025StandardOnly},
		{bchnMay2026, ScriptAllowMay2026},
	}
	for _, c := range conv {
		if bits&c.bit != 0 {
			flags |= c.flag
		}
	}
	return flags
}

// parseScriptFlags parses the flags string of the reference tests into
// ScriptFlags suitable for use in the script engine.
func parseScriptFlags(flagStr string) (ScriptFlags, error) {
	bits, err := parseBCHNFlags(flagStr)
	if err != nil {
		return 0, err
	}
	return bchnScriptFlags(bits), nil
}

// parseLegacyScriptFlags parses the flags string of script_tests_bchd.json,
// where each upgrade is only enabled when its flag is given.
func parseLegacyScriptFlags(flagStr string) (ScriptFlags, error) {
	var flags ScriptFlags

	sFlags := strings.SplitSeq(flagStr, ",")
	for flag := range sFlags {
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
		case "DUP":
			flags |= ScriptVerify64BitIntegers
		default:
			return flags, fmt.Errorf("invalid flag: %s", flag)
		}
	}
	return flags, nil
}

// bchnScriptErrorCodes maps the name of each of BCHN's script errors, as its
// script tests spell it (test/script_tests.cpp), to the codes txscript gives
// that error.  No code stands for two of these names, so the error of every
// failing test row is the one BCHN names, and the mempool reports it with
// BCHN's text.
var bchnScriptErrorCodes = map[string][]ErrorCode{
	"EVAL_FALSE":                   {ErrEvalFalse, ErrEmptyStack},
	"OP_RETURN":                    {ErrEarlyReturn},
	"SCRIPT_SIZE":                  {ErrScriptTooBig},
	"PUSH_SIZE":                    {ErrElementTooBig},
	"OP_COUNT":                     {ErrTooManyOperations},
	"STACK_SIZE":                   {ErrStackOverflow},
	"SIG_COUNT":                    {ErrInvalidSignatureCount},
	"PUBKEY_COUNT":                 {ErrInvalidPubKeyCount},
	"INPUT_SIGCHECKS":              {ErrInputSigChecks},
	"OPERAND_SIZE":                 {ErrInvalidInputLength},
	"INVALID_NUMBER_RANGE_32_BIT":  {ErrNumberRange32Bit},
	"INVALID_NUMBER_RANGE_64_BIT":  {ErrNumberRange64Bit},
	"INVALID_NUMBER_RANGE_BIG_INT": {ErrNumberRangeBigInt},
	"IMPOSSIBLE_ENCODING":          {ErrImpossibleEncoding},
	"SPLIT_RANGE":                  {ErrInvalidSplitRange},
	"INVALID_BIT_COUNT":            {ErrInvalidBitCount},
	"VERIFY":                       {ErrVerify},
	"EQUALVERIFY":                  {ErrEqualVerify},
	"CHECKMULTISIGVERIFY":          {ErrCheckMultiSigVerify},
	"CHECKSIGVERIFY":               {ErrCheckSigVerify},
	"CHECKDATASIGVERIFY":           {ErrCheckDataSigVerify},
	"NUMEQUALVERIFY":               {ErrNumEqualVerify},
	"BAD_OPCODE":                   {ErrReservedOpcode, ErrMalformedPush},
	"DISABLED_OPCODE":              {ErrDisabledOpcode},
	"INVALID_STACK_OPERATION":      {ErrInvalidStackOperation},
	"INVALID_ALTSTACK_OPERATION":   {ErrInvalidAltStackOperation},
	"UNBALANCED_CONDITIONAL":       {ErrUnbalancedConditional},
	"UNBALANCED_CONTROL_FLOW":      {ErrUnbalancedControlFlow},
	"NEGATIVE_LOCKTIME":            {ErrNegativeLockTime},
	"UNSATISFIED_LOCKTIME":         {ErrUnsatisfiedLockTime},
	"SIG_HASHTYPE":                 {ErrInvalidSigHashType},
	"SIG_DER": {ErrSigTooShort, ErrSigTooLong, ErrSigInvalidSeqID,
		ErrSigInvalidDataLen, ErrSigMissingSTypeID, ErrSigMissingSLen,
		ErrSigInvalidSLen, ErrSigInvalidRIntID, ErrSigZeroRLen,
		ErrSigNegativeR, ErrSigTooMuchRPadding, ErrSigInvalidSIntID,
		ErrSigZeroSLen, ErrSigNegativeS, ErrSigTooMuchSPadding},
	"MINIMALDATA":                {ErrMinimalData},
	"SIG_PUSHONLY":               {ErrNotPushOnly},
	"SIG_HIGH_S":                 {ErrSigHighS},
	"PUBKEYTYPE":                 {ErrPubKeyType},
	"CLEANSTACK":                 {ErrCleanStack},
	"MINIMALIF":                  {ErrMinimalIf},
	"NULLFAIL":                   {ErrNullFail},
	"MINIMALNUM":                 {ErrMinimalNumber},
	"SIG_BADLENGTH":              {ErrSigBadLength},
	"SIG_NONSCHNORR":             {ErrSigNonSchnorr},
	"DISCOURAGE_UPGRADABLE_NOPS": {ErrDiscourageUpgradableNOPs},
	"ILLEGAL_FORKID":             {ErrIllegalForkID},
	"MISSING_FORKID":             {ErrMustUseForkID},
	"DIV_BY_ZERO":                {ErrDivByZero},
	"MOD_BY_ZERO":                {ErrModByZero},
	"BITFIELD_SIZE":              {ErrInvalidDummy},
	"BIT_RANGE":                  {ErrInvalidBitRange},
	"OP_COST":                    {ErrOpCost},
	"HASH_ITERS":                 {ErrTooManyHashIters},
	"CONDITIONAL_STACK_DEPTH":    {ErrConditionalStackDepth},
	"CONTROL_STACK_DEPTH":        {ErrControlStackDepth},
}

// legacyScriptErrorCodes maps the names only script_tests_bchd.json uses,
// those of older BCHN releases and of bchd's rules before an upgrade, to the
// codes txscript gives them.
var legacyScriptErrorCodes = map[string][]ErrorCode{
	// Older releases named every number error UNKNOWN_ERROR.
	"UNKNOWN_ERROR": {ErrNumberRange32Bit, ErrNumberRange64Bit,
		ErrMinimalNumber},
	"INVALID_NUMBER_RANGE": {ErrNumberRange32Bit},
	"NONCOMPRESSED_PUBKEY": {ErrPubKeyType},
	"SIG_NULLDUMMY":        {ErrSigNullDummy},
}

// parseExpectedResult parses the provided expected result string into allowed
// script error codes.  An error is returned if the expected result string is
// not supported.
func parseExpectedResult(expected string) ([]ErrorCode, error) {
	if expected == "OK" {
		return nil, nil
	}
	if codes, ok := bchnScriptErrorCodes[expected]; ok {
		return codes, nil
	}
	if codes, ok := legacyScriptErrorCodes[expected]; ok {
		return codes, nil
	}

	return nil, fmt.Errorf("unrecognized expected result in test data: %v",
		expected)
}

// TestBCHNScriptErrorCodes ensures no error code stands for two of BCHN's
// script errors in the reference test mapping.
func TestBCHNScriptErrorCodes(t *testing.T) {
	names := make(map[ErrorCode]string)
	for name, codes := range bchnScriptErrorCodes {
		for _, code := range codes {
			if other, ok := names[code]; ok {
				t.Errorf("%v stands for %s and %s", code, other, name)
			}
			names[code] = name
		}
	}
}

// createSpendingTx generates a basic spending transaction given the passed
// signature, witness and public key scripts.
func createSpendingTx(sigScript, pkScript []byte,
	outputValue int64) *wire.MsgTx {

	coinbaseTx := wire.NewMsgTx(wire.TxVersion)

	outPoint := wire.NewOutPoint(&chainhash.Hash{}, ^uint32(0))
	txIn := wire.NewTxIn(outPoint, []byte{OP_0, OP_0})
	txOut := wire.NewTxOut(outputValue, pkScript, wire.TokenData{})
	coinbaseTx.AddTxIn(txIn)
	coinbaseTx.AddTxOut(txOut)

	spendingTx := wire.NewMsgTx(wire.TxVersion)
	coinbaseTxSha := coinbaseTx.TxHash()
	outPoint = wire.NewOutPoint(&coinbaseTxSha, 0)
	txIn = wire.NewTxIn(outPoint, sigScript)
	txOut = wire.NewTxOut(outputValue, nil, wire.TokenData{})

	spendingTx.AddTxIn(txIn)
	spendingTx.AddTxOut(txOut)

	return spendingTx
}

// scriptTestKey returns a short key that identifies a reference test row.
func scriptTestKey(test []interface{}) string {
	b, _ := json.Marshal(test)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:6])
}

// testScripts ensures all of the passed script tests execute with the expected
// results with or without using a signature cache, as specified by the
// parameter.  Rows with BCHN's flag names are also run with random
// restrictive flags removed from passing rows and added to failing rows, as
// BCHN's script tests do.
func testScripts(t *testing.T, tests [][]interface{}, useSigCache bool,
	legacyFlags bool) {

	// Create a signature cache to use only if requested.
	var sigCache *SigCache
	if useSigCache {
		sigCache = NewSigCache(10)
	}

	for i, test := range tests {
		// Format is: [[amount]?, scriptSig, scriptPubKey, flags,
		// expected_scripterror, ... comments]
		key := scriptTestKey(test)
		row, _ := json.Marshal(test)
		name := fmt.Sprintf("test #%d %s %s", i, key, row)

		var inputAmt int64
		pos := 0
		if len(test) > 0 {
			if amounts, ok := test[0].([]interface{}); ok {
				amount, ok := amounts[0].(float64)
				if !ok {
					t.Errorf("%s: amount is not a number", name)
					continue
				}
				inputAmt = int64(math.Round(amount * 1e8))
				pos++
			}
		}

		// Extra fields are comments.
		if len(test) < 4+pos {
			if len(test) != 1 {
				t.Errorf("%s: bad test", name)
			}
			continue
		}

		strs := make([]string, 4)
		badField := false
		for j := range strs {
			s, ok := test[pos+j].(string)
			if !ok {
				t.Errorf("%s: field %d is not a string", name, pos+j)
				badField = true
				break
			}
			strs[j] = s
		}
		if badField {
			continue
		}
		scriptSig, err := parseShortForm(strs[0])
		if err != nil {
			t.Errorf("%s: can't parse signature script: %v", name, err)
			continue
		}
		scriptPubKey, err := parseShortForm(strs[1])
		if err != nil {
			t.Errorf("%s: can't parse public key script: %v", name, err)
			continue
		}

		var flags ScriptFlags
		var bits uint32
		if legacyFlags {
			flags, err = parseLegacyScriptFlags(strs[2])
		} else {
			bits, err = parseBCHNFlags(strs[2])
			if bits&bchnCleanStack != 0 {
				bits |= bchnP2SH
			}
			flags = bchnScriptFlags(bits)
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		// Convert the expected result string into the allowed script
		// error codes.  This is necessary because txscript is more
		// fine grained with its errors than the reference test data, so
		// some of the reference test data errors map to more than one
		// possibility.
		resultStr := strs[3]
		allowedErrorCodes, err := parseExpectedResult(resultStr)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		run := func(flags ScriptFlags) error {
			tx := createSpendingTx(scriptSig, scriptPubKey, inputAmt)
			utxos := NewUtxoCache()
			utxos.AddEntry(0, *wire.NewTxOut(inputAmt, scriptPubKey,
				wire.TokenData{}))
			vm, err := NewEngine(scriptPubKey, tx, 0, flags, sigCache,
				nil, utxos, inputAmt)
			if err == nil {
				err = vm.Execute()
			}
			return err
		}

		err = run(flags)
		ok := err == nil
		if resultStr != "OK" {
			ok = false
			for _, code := range allowedErrorCodes {
				if IsErrorCode(err, code) {
					ok = true
					break
				}
			}
		}

		if !ok {
			t.Errorf("%s: want %s, got %v", name, resultStr, err)
			continue
		}

		if legacyFlags {
			continue
		}
		expect := resultStr == "OK"
		rng := rand.New(rand.NewSource(int64(i)))
		for range 16 {
			extra := rng.Uint32() &^ bchnNotRestrictive
			combined := bits | extra
			if expect {
				combined = bits &^ extra
			}
			if combined&bchnCleanStack != 0 {
				combined |= bchnP2SH
			}
			err := run(bchnScriptFlags(combined))
			if (err == nil) != expect {
				t.Errorf("%s: with flags %08x %s: got %v", name,
					combined^bits, map[bool]string{
						true: "removed", false: "added",
					}[expect], err)
				break
			}
		}
	}
}

// TestScripts ensures all of the tests in script_tests.json execute with the
// expected results as defined in the test data.
func TestScripts(t *testing.T) {
	file, err := os.ReadFile("data/script_tests.json")
	if err != nil {
		t.Fatalf("TestScripts: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
	}

	// Run all script tests with and without the signature cache.
	testScripts(t, tests, true, false)
	testScripts(t, tests, false, false)
}

// TestScriptsBCHD runs the rows of script_tests_bchd.json, which check the
// rules before each upgrade activated.
func TestScriptsBCHD(t *testing.T) {
	file, err := os.ReadFile("data/script_tests_bchd.json")
	if err != nil {
		t.Fatalf("TestScriptsBCHD: %v\n", err)
	}

	var tests [][]interface{}
	err = json.Unmarshal(file, &tests)
	if err != nil {
		t.Fatalf("TestScriptsBCHD couldn't Unmarshal: %v", err)
	}

	testScripts(t, tests, true, true)
	testScripts(t, tests, false, true)
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

// TestCalcLegacySignatureHash runs the Bitcoin Core signature hash calculation tests
// in sighash.json.
// https://github.com/bitcoin/bitcoin/blob/master/src/test/data/sighash.json
func TestCalcLegacySignatureHash(t *testing.T) {
	file, err := os.ReadFile("data/sighash_legacy.json")
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
		hash, _, err = calcLegacySignatureHash(parsedScript, hashType, &tx,
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
	file, err := os.ReadFile("data/sighash_bip143.json")
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
		hash, _, err = calcBip143SignatureHash(parsedScript, sigHashes, hashType, &tx,
			int(test[2].(float64)), 0, false)
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
