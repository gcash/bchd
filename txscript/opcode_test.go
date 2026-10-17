// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// TestOpcodeDisabled tests the opcodeDisabled function manually because all
// disabled opcodes result in a script execution failure when executed normally,
// so the function is not called under normal circumstances.
func TestOpcodeDisabled(t *testing.T) {
	t.Parallel()

	tests := []byte{OP_INVERT, OP_2MUL, OP_2DIV, OP_MUL, OP_LSHIFT, OP_RSHIFT}
	for _, opcodeVal := range tests {
		pop := parsedOpcode{opcode: &opcodeArray[opcodeVal], data: nil}
		err := opcodeDisabled(&pop, nil)
		if !IsErrorCode(err, ErrDisabledOpcode) {
			t.Errorf("opcodeDisabled: unexpected error - got %v, "+
				"want %v", err, ErrDisabledOpcode)
			continue
		}
	}
}

// TestOpcodeDisasm tests the print function for all opcodes in both the oneline
// and full modes to ensure it provides the expected disassembly.
func TestOpcodeDisasm(t *testing.T) {
	t.Parallel()

	// First, test the oneline disassembly.

	// The expected strings for the data push opcodes are replaced in the
	// test loops below since they involve repeating bytes.  Also, the
	// OP_NOP# and OP_UNKNOWN# are replaced below too, since it's easier
	// than manually listing them here.
	oneBytes := []byte{0x01}
	oneStr := "01"
	expectedStrings := [256]string{0x00: "0", 0x4f: "-1",
		0x50: "OP_RESERVED", 0x61: "OP_NOP", 0x62: "OP_VER",
		0x63: "OP_IF", 0x64: "OP_NOTIF", 0x65: "OP_BEGIN",
		0x66: "OP_UNTIL", 0x67: "OP_ELSE", 0x68: "OP_ENDIF",
		0x69: "OP_VERIFY", 0x6a: "OP_RETURN", 0x6b: "OP_TOALTSTACK",
		0x6c: "OP_FROMALTSTACK", 0x6d: "OP_2DROP", 0x6e: "OP_2DUP",
		0x6f: "OP_3DUP", 0x70: "OP_2OVER", 0x71: "OP_2ROT",
		0x72: "OP_2SWAP", 0x73: "OP_IFDUP", 0x74: "OP_DEPTH",
		0x75: "OP_DROP", 0x76: "OP_DUP", 0x77: "OP_NIP",
		0x78: "OP_OVER", 0x79: "OP_PICK", 0x7a: "OP_ROLL",
		0x7b: "OP_ROT", 0x7c: "OP_SWAP", 0x7d: "OP_TUCK",
		0x7e: "OP_CAT", 0x7f: "OP_SPLIT", 0x80: "OP_NUM2BIN",
		0x81: "OP_BIN2NUM", 0x82: "OP_SIZE", 0x83: "OP_INVERT",
		0x84: "OP_AND", 0x85: "OP_OR", 0x86: "OP_XOR",
		0x87: "OP_EQUAL", 0x88: "OP_EQUALVERIFY", 0x89: "OP_DEFINE",
		0x8a: "OP_INVOKE", 0x8b: "OP_1ADD", 0x8c: "OP_1SUB",
		0x8d: "OP_LSHIFTNUM", 0x8e: "OP_RSHIFTNUM", 0x8f: "OP_NEGATE",
		0x90: "OP_ABS", 0x91: "OP_NOT", 0x92: "OP_0NOTEQUAL",
		0x93: "OP_ADD", 0x94: "OP_SUB", 0x95: "OP_MUL", 0x96: "OP_DIV",
		0x97: "OP_MOD", 0x98: "OP_LSHIFTBIN", 0x99: "OP_RSHIFTBIN",
		0x9a: "OP_BOOLAND", 0x9b: "OP_BOOLOR", 0x9c: "OP_NUMEQUAL",
		0x9d: "OP_NUMEQUALVERIFY", 0x9e: "OP_NUMNOTEQUAL",
		0x9f: "OP_LESSTHAN", 0xa0: "OP_GREATERTHAN",
		0xa1: "OP_LESSTHANOREQUAL", 0xa2: "OP_GREATERTHANOREQUAL",
		0xa3: "OP_MIN", 0xa4: "OP_MAX", 0xa5: "OP_WITHIN",
		0xa6: "OP_RIPEMD160", 0xa7: "OP_SHA1", 0xa8: "OP_SHA256",
		0xa9: "OP_HASH160", 0xaa: "OP_HASH256", 0xab: "OP_CODESEPARATOR",
		0xac: "OP_CHECKSIG", 0xad: "OP_CHECKSIGVERIFY",
		0xae: "OP_CHECKMULTISIG", 0xaf: "OP_CHECKMULTISIGVERIFY",
		0xba: "OP_CHECKDATASIG", 0xbb: "OP_CHECKDATASIGVERIFY",
		0xbc: "OP_REVERSEBYTES", 0xfa: "OP_SMALLINTEGER",
		0xc0: "OP_INPUTINDEX", 0xc1: "OP_ACTIVEBYTECODE",
		0xc2: "OP_TXVERSION", 0xc3: "OP_TXINPUTCOUNT",
		0xc4: "OP_TXOUTPUTCOUNT", 0xc5: "OP_TXLOCKTIME",
		0xc6: "OP_UTXOVALUE", 0xc7: "OP_UTXOBYTECODE",
		0xc8: "OP_OUTPOINTTXHASH", 0xc9: "OP_OUTPOINTINDEX",
		0xca: "OP_INPUTBYTECODE", 0xcb: "OP_INPUTSEQUENCENUMBER",
		0xcc: "OP_OUTPUTVALUE", 0xcd: "OP_OUTPUTBYTECODE",
		0xfb: "OP_PUBKEYS", 0xfd: "OP_PUBKEYHASH",
		0xfe: "OP_PUBKEY", 0xff: "OP_INVALIDOPCODE",
		0xce: "OP_UTXOTOKENCATEGORY", 0xcf: "OP_UTXOTOKENCOMMITMENT",
		0xd0: "OP_UTXOTOKENAMOUNT", 0xd1: "OP_OUTPUTTOKENCATEGORY",
		0xd2: "OP_OUTPUTTOKENCOMMITMENT", 0xd3: "OP_OUTPUTTOKENAMOUNT",
		0xd4: "OP_RESERVED3", 0xd5: "OP_RESERVED4",
		0xef: "SPECIAL_TOKEN_PREFIX",
	}
	for opcodeVal, expectedStr := range expectedStrings {
		var data []byte
		switch {
		// OP_DATA_1 through OP_DATA_65 display the pushed data.
		case opcodeVal >= 0x01 && opcodeVal < 0x4c:
			data = bytes.Repeat(oneBytes, opcodeVal)
			expectedStr = strings.Repeat(oneStr, opcodeVal)

		// OP_PUSHDATA1.
		case opcodeVal == 0x4c:
			data = bytes.Repeat(oneBytes, 1)
			expectedStr = strings.Repeat(oneStr, 1)

		// OP_PUSHDATA2.
		case opcodeVal == 0x4d:
			data = bytes.Repeat(oneBytes, 2)
			expectedStr = strings.Repeat(oneStr, 2)

		// OP_PUSHDATA4.
		case opcodeVal == 0x4e:
			data = bytes.Repeat(oneBytes, 3)
			expectedStr = strings.Repeat(oneStr, 3)

		// OP_1 through OP_16 display the numbers themselves.
		case opcodeVal >= 0x51 && opcodeVal <= 0x60:
			val := byte(opcodeVal - (0x51 - 1))
			data = []byte{val}
			expectedStr = strconv.Itoa(int(val))

		// OP_NOP1 through OP_NOP10.
		case opcodeVal >= 0xb0 && opcodeVal <= 0xb9:
			switch opcodeVal {
			case 0xb1:
				// OP_NOP2 is an alias of OP_CHECKLOCKTIMEVERIFY
				expectedStr = "OP_CHECKLOCKTIMEVERIFY"
			case 0xb2:
				// OP_NOP3 is an alias of OP_CHECKSEQUENCEVERIFY
				expectedStr = "OP_CHECKSEQUENCEVERIFY"
			default:
				val := byte(opcodeVal - (0xb0 - 1))
				expectedStr = "OP_NOP" + strconv.Itoa(int(val))
			}

		// OP_UNKNOWN#.
		case (opcodeVal >= 0xbd && opcodeVal <= 0xbf) || (opcodeVal >= 0xd6 && opcodeVal <= 0xf9 && opcodeVal != 0xef) || opcodeVal == 0xfc:
			expectedStr = "OP_UNKNOWN" + strconv.Itoa(opcodeVal)
		}

		pop := parsedOpcode{opcode: &opcodeArray[opcodeVal], data: data}
		gotStr := pop.print(true)
		if gotStr != expectedStr {
			t.Errorf("pop.print (opcode %x): Unexpected disasm "+
				"string - got %v, want %v", opcodeVal, gotStr,
				expectedStr)
			continue
		}
	}

	// Now, replace the relevant fields and test the full disassembly.
	expectedStrings[0x00] = "OP_0"
	expectedStrings[0x4f] = "OP_1NEGATE"
	for opcodeVal, expectedStr := range expectedStrings {
		var data []byte
		switch {
		// OP_DATA_1 through OP_DATA_65 display the opcode followed by
		// the pushed data.
		case opcodeVal >= 0x01 && opcodeVal < 0x4c:
			data = bytes.Repeat(oneBytes, opcodeVal)
			expectedStr = fmt.Sprintf("OP_DATA_%d 0x%s", opcodeVal,
				strings.Repeat(oneStr, opcodeVal))

		// OP_PUSHDATA1.
		case opcodeVal == 0x4c:
			data = bytes.Repeat(oneBytes, 1)
			expectedStr = fmt.Sprintf("OP_PUSHDATA1 0x%02x 0x%s",
				len(data), strings.Repeat(oneStr, 1))

		// OP_PUSHDATA2.
		case opcodeVal == 0x4d:
			data = bytes.Repeat(oneBytes, 2)
			expectedStr = fmt.Sprintf("OP_PUSHDATA2 0x%04x 0x%s",
				len(data), strings.Repeat(oneStr, 2))

		// OP_PUSHDATA4.
		case opcodeVal == 0x4e:
			data = bytes.Repeat(oneBytes, 3)
			expectedStr = fmt.Sprintf("OP_PUSHDATA4 0x%08x 0x%s",
				len(data), strings.Repeat(oneStr, 3))

		// OP_1 through OP_16.
		case opcodeVal >= 0x51 && opcodeVal <= 0x60:
			val := byte(opcodeVal - (0x51 - 1))
			data = []byte{val}
			expectedStr = "OP_" + strconv.Itoa(int(val))

		// OP_NOP1 through OP_NOP10.
		case opcodeVal >= 0xb0 && opcodeVal <= 0xb9:
			switch opcodeVal {
			case 0xb1:
				// OP_NOP2 is an alias of OP_CHECKLOCKTIMEVERIFY
				expectedStr = "OP_CHECKLOCKTIMEVERIFY"
			case 0xb2:
				// OP_NOP3 is an alias of OP_CHECKSEQUENCEVERIFY
				expectedStr = "OP_CHECKSEQUENCEVERIFY"
			default:
				val := byte(opcodeVal - (0xb0 - 1))
				expectedStr = "OP_NOP" + strconv.Itoa(int(val))
			}

		// OP_UNKNOWN#.
		case (opcodeVal >= 0xbd && opcodeVal <= 0xbf) || (opcodeVal >= 0xd6 && opcodeVal <= 0xf9 && opcodeVal != 0xef) || opcodeVal == 0xfc:
			expectedStr = "OP_UNKNOWN" + strconv.Itoa(opcodeVal)
		}

		pop := parsedOpcode{opcode: &opcodeArray[opcodeVal], data: data}
		gotStr := pop.print(false)
		if gotStr != expectedStr {
			t.Errorf("pop.print (opcode %x): Unexpected disasm "+
				"string - got %v, want %v", opcodeVal, gotStr,
				expectedStr)
			continue
		}
	}
}

func TestNativeIntrospectionOpcodes(t *testing.T) {
	newHashFromStr := func(hexStr string) chainhash.Hash {
		hash, err := chainhash.NewHashFromStr(hexStr)
		if err != nil {
			panic(err)
		}
		return *hash
	}
	newScript := func(builder *ScriptBuilder) []byte {
		script, err := builder.Script()
		if err != nil {
			panic(err)
		}
		return script
	}
	fromHex := func(s string) []byte {
		b, err := hex.DecodeString(s)
		if err != nil {
			panic(err)
		}
		return b
	}

	testVectors := [][]struct {
		name          string
		scriptPubkey  []byte
		scriptSig     []byte
		index         int
		flags         ScriptFlags
		expectedError ErrorCode
	}{
		// OP_INPUTINDEX (nullary)
		{
			{
				name: "OP_INPUTINDEX 0",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_INPUTINDEX).
					AddInt64(0).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_INPUTINDEX 1",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_INPUTINDEX).
					AddInt64(1).
					AddOp(OP_EQUAL)),
				index: 1,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_INPUTINDEX Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_INPUTINDEX).
					AddInt64(1).
					AddOp(OP_EQUAL)),
				index:         1,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_ACTIVEBYTECODE (nullary)
		{
			{
				name: "OP_ACTIVEBYTECODE OP_9",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_ACTIVEBYTECODE).
					AddOp(OP_9).
					AddOp(OP_DROP).
					AddOp(OP_EQUAL)),
				scriptSig: fromHex("04c1597587"),
				index:     0,
				flags:     ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_ACTIVEBYTECODE OP_10",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_ACTIVEBYTECODE).
					AddOp(OP_10).
					AddOp(OP_DROP).
					AddOp(OP_EQUAL)),
				scriptSig: fromHex("04c15a7587"),
				index:     0,
				flags:     ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_ACTIVEBYTECODE OP_CODESEPERATOR first",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_10).
					AddOp(OP_11).
					AddInt64(7654321).
					AddOp(OP_DROP).
					AddOp(OP_DROP).
					AddOp(OP_DROP).
					AddOp(OP_CODESEPARATOR).
					AddOp(OP_5).
					AddOp(OP_DROP).
					AddOp(OP_ACTIVEBYTECODE).
					AddOp(OP_EQUAL)),
				scriptSig: fromHex("045575c187"),
				index:     0,
				flags:     ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_ACTIVEBYTECODE OP_CODESEPERATOR second",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_10).
					AddOp(OP_DROP).
					AddOp(OP_ACTIVEBYTECODE).
					AddOp(OP_EQUALVERIFY).
					AddOp(OP_CODESEPARATOR).
					AddOp(OP_1)),
				scriptSig: fromHex("065a75c188ab51"),
				index:     0,
				flags:     ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_ACTIVEBYTECODE max",
				scriptPubkey: newScript(NewScriptBuilder().
					AddData(make([]byte, MaxScriptElementSizeLegacy-6)).
					AddOp(OP_DROP).
					AddOp(OP_ACTIVEBYTECODE).
					AddOp(OP_EQUAL)),
				scriptSig: fromHex("4d08024d02020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000075c187"),
				index:     0,
				flags:     ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_ACTIVEBYTECODE exceeds max",
				scriptPubkey: newScript(NewScriptBuilder().
					AddData(make([]byte, MaxScriptElementSizeLegacy-5)).
					AddOp(OP_DROP).
					AddOp(OP_ACTIVEBYTECODE).
					AddOp(OP_EQUAL)),
				scriptSig:     fromHex("4d09024d0302000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000075c187"),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrElementTooBig,
			},
			{
				name: "OP_ACTIVEBYTECODE not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_10).
					AddOp(OP_DROP).
					AddOp(OP_ACTIVEBYTECODE).
					AddOp(OP_EQUALVERIFY).
					AddOp(OP_CODESEPARATOR).
					AddOp(OP_1)),
				scriptSig:     fromHex("065a75c188ab51"),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_TXVERSION (nullary)
		{
			{
				name: "OP_TXVERSION",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_TXVERSION).
					AddInt64(1).
					AddOp(OP_EQUAL)),
				index: 1,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_TXVERSION Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_INPUTINDEX).
					AddInt64(1).
					AddOp(OP_EQUAL)),
				index:         1,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_TXINPUTCOUNT (nullary)
		{
			{
				name: "OP_TXINPUTCOUNT",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_TXINPUTCOUNT).
					AddInt64(2).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_TXINPUTCOUNT Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_TXINPUTCOUNT).
					AddInt64(1).
					AddOp(OP_EQUAL)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_TXOUTPUTCOUNT (nullary)
		{
			{
				name: "OP_TXOUTPUTCOUNT",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_TXOUTPUTCOUNT).
					AddInt64(2).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_TXOUTPUTCOUNT Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_TXOUTPUTCOUNT).
					AddInt64(1).
					AddOp(OP_EQUAL)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_TXLOCKTIME (nullary)
		{
			{
				name: "OP_TXLOCKTIME",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_TXLOCKTIME).
					AddInt64(10000).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_TXLOCKTIME Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_TXLOCKTIME).
					AddInt64(1).
					AddOp(OP_EQUAL)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_UTXOVALUE (unary)
		{
			{
				name: "OP_UTXOVALUE first input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(0).
					AddOp(OP_UTXOVALUE).
					AddInt64(2000).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_UTXOVALUE second input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_UTXOVALUE).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_UTXOVALUE out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_UTXOVALUE).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_UTXOVALUE missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_UTXOVALUE).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_UTXOVALUE Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_UTXOVALUE)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_UTXOBYTECODE (unary)
		{
			{
				name: "OP_UTXOBYTECODE first input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(0).
					AddOp(OP_UTXOBYTECODE).
					AddData(fromHex("0000000000000000")).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_UTXOBYTECODE exceeds max size",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_UTXOBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrElementTooBig,
			},
			{
				name: "OP_UTXOBYTECODE out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_UTXOBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_UTXOBYTECODE missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_UTXOBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_UTXOBYTECODE Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_UTXOBYTECODE)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_OUTPOINTTXHASH (unary)
		{
			{
				name: "OP_OUTPOINTHASH first input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(0).
					AddOp(OP_OUTPOINTTXHASH).
					AddData(fromHex("2f663b097fa3439dc2a637d350f410a969587750a99459104363526995ae89be")).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_OUTPOINTTXHASH second input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPOINTTXHASH).
					AddData(fromHex("48b36698efedb8c0a26282e46441948cfb15fae9b78193d3ce4f092b00fcd508")).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_OUTPOINTTXHASH out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_OUTPOINTTXHASH).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_OUTPOINTTXHASH missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_OUTPOINTTXHASH).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_OUTPOINTTXHASH Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPOINTTXHASH)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_OUTPOINTINDEX (unary)
		{
			{
				name: "OP_OUTPOINTINDEX first input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(0).
					AddOp(OP_OUTPOINTINDEX).
					AddInt64(5).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_OUTPOINTINDEX second input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPOINTINDEX).
					AddInt64(7).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_OUTPOINTINDEX out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_OUTPOINTINDEX).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_OUTPOINTINDEX missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_OUTPOINTINDEX).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_OUTPOINTINDEX Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPOINTINDEX)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_INPUTBYTECODE (unary)
		{
			{
				name: "OP_INPUTBYTECODE first input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_DROP).
					AddInt64(0).
					AddOp(OP_INPUTBYTECODE).
					AddData(fromHex("0000000000000000")).
					AddOp(OP_EQUAL)),
				index:     0,
				scriptSig: fromHex("0000000000000000"),
				flags:     ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_INPUTBYTECODE exceeds max size",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_DROP).
					AddInt64(0).
					AddOp(OP_INPUTBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				scriptSig:     make([]byte, MaxScriptElementSizeLegacy+1),
				expectedError: ErrElementTooBig,
			},
			{
				name: "OP_INPUTBYTECODE out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_INPUTBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_INPUTBYTECODE missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_INPUTBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_INPUTBYTECODE Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_INPUTBYTECODE)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_INPUTSEQUENCENUMBER (unary)
		{
			{
				name: "OP_INPUTSEQUENCENUMBER first input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(0).
					AddOp(OP_INPUTSEQUENCENUMBER).
					AddInt64(13).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_INPUTSEQUENCENUMBER second input",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_INPUTSEQUENCENUMBER).
					AddInt64(22).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_INPUTSEQUENCENUMBER out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_INPUTSEQUENCENUMBER).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_INPUTSEQUENCENUMBER missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_INPUTSEQUENCENUMBER).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_INPUTSEQUENCENUMBER Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_INPUTSEQUENCENUMBER)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_OUTPUTVALUE (unary)
		{
			{
				name: "OP_OUTPUTVALUE first output",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(0).
					AddOp(OP_OUTPUTVALUE).
					AddInt64(10000).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_OUTPUTVALUE second output",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPUTVALUE).
					AddInt64(20000).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_OUTPUTVALUE out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_OUTPUTVALUE).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_OUTPUTVALUE missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_OUTPUTVALUE).
					AddInt64(12000).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_OUTPUTVALUE Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPUTVALUE)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
		// OP_OUTPUTBYTECODE (unary)
		{
			{
				name: "OP_OUTPUTBYTECODE first output",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(0).
					AddOp(OP_OUTPUTBYTECODE).
					AddData(fromHex("76a914000000000000000000000000000000000000000088ac")).
					AddOp(OP_EQUAL)),
				index: 0,
				flags: ScriptVerifyNativeIntrospection,
			},
			{
				name: "OP_OUTPUTBYTECODE exceeds max size",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPUTBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrElementTooBig,
			},
			{
				name: "OP_OUTPUTBYTECODE out of range",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(2).
					AddOp(OP_OUTPUTBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidIndex,
			},
			{
				name: "OP_OUTPUTBYTECODE missing arg",
				scriptPubkey: newScript(NewScriptBuilder().
					AddOp(OP_OUTPUTBYTECODE).
					AddData(fromHex("0000000000000001")).
					AddOp(OP_EQUAL)),
				index:         0,
				flags:         ScriptVerifyNativeIntrospection,
				expectedError: ErrInvalidStackOperation,
			},
			{
				name: "OP_OUTPUTBYTECODE Not activated",
				scriptPubkey: newScript(NewScriptBuilder().
					AddInt64(1).
					AddOp(OP_OUTPUTBYTECODE)),
				index:         0,
				expectedError: ErrDisabledOpcode,
			},
		},
	}

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  newHashFromStr("be89ae9569526343105994a950775869a910f450d337a6c29d43a37f093b662f"),
					Index: 5,
				},
				SignatureScript: nil,
				Sequence:        13,
			},
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  newHashFromStr("08d5fc002b094fced39381b7e9fa15fb8c944164e48262a2c0b8edef9866b348"),
					Index: 7,
				},
				SignatureScript: nil,
				Sequence:        22,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value: 10000,
				PkScript: newScript(NewScriptBuilder().
					AddOp(OP_DUP).
					AddOp(OP_HASH160).
					addData(fromHex("0000000000000000000000000000000000000000")).
					AddOp(OP_EQUALVERIFY).
					AddOp(OP_CHECKSIG)),
			},
			{
				Value: 20000,
				PkScript: newScript(NewScriptBuilder().
					AddOp(OP_HASH160).
					addData(make([]byte, MaxScriptElementSizeLegacy)).
					AddOp(OP_EQUAL)),
			},
		},
		LockTime: 10000,
	}

	cache := NewUtxoCache()
	for i := range tx.TxIn {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(i))
		if i == 1 {
			b = make([]byte, MaxScriptElementSizeLegacy+1)
		}
		cache.AddEntry(i, wire.TxOut{
			Value:    int64(2000 + (i * 10000)),
			PkScript: b,
		})
	}

	for i, group := range testVectors {
		for x, test := range group {
			utxo, err := cache.GetEntry(test.index)
			if err != nil {
				t.Fatalf("Test %d - %s: Utxo not found", i*x, test.name)
			}

			if test.scriptSig != nil {
				tx.TxIn[test.index].SignatureScript = test.scriptSig
			}

			vm, err := NewEngine(test.scriptPubkey, tx, test.index, test.flags, nil, nil, cache, utxo.Value)
			if err == nil {
				err = vm.Execute()
			}
			if err != nil {
				if !IsErrorCode(err, test.expectedError) {
					t.Errorf("Test %d - %s: Expected error %v got %v", i*x, test.name, test.expectedError, err)
				}
			} else {
				if test.expectedError != ErrInternal {
					t.Errorf("Test %d - %s: Expected error %v got nil", i*x, test.name, test.expectedError)
				}
			}
			tx.TxIn[test.index].SignatureScript = nil
		}
	}
}

// TestRenamedOpcodes ensures the earlier names of renamed opcodes still name
// the same opcodes, while disassembly uses the current names.
func TestRenamedOpcodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		oldName string
		newName string
		value   byte
	}{
		{"OP_2MUL", "OP_LSHIFTNUM", OP_LSHIFTNUM},
		{"OP_2DIV", "OP_RSHIFTNUM", OP_RSHIFTNUM},
		{"OP_LSHIFT", "OP_LSHIFTBIN", OP_LSHIFTBIN},
		{"OP_RSHIFT", "OP_RSHIFTBIN", OP_RSHIFTBIN},
		{"OP_UNKNOWN212", "OP_RESERVED3", OP_RESERVED3},
		{"OP_UNKNOWN213", "OP_RESERVED4", OP_RESERVED4},
	}
	for _, test := range tests {
		if OpcodeByName[test.oldName] != test.value ||
			OpcodeByName[test.newName] != test.value {

			t.Errorf("%s: unexpected values %#x and %#x", test.newName,
				OpcodeByName[test.oldName], OpcodeByName[test.newName])
		}
		disasm, err := DisasmString([]byte{test.value})
		if err != nil || disasm != test.newName {
			t.Errorf("%s: unexpected disassembly %q (%v)", test.newName,
				disasm, err)
		}
	}

	// The short form used by the reference tests accepts both names.
	for _, short := range []string{"LSHIFT", "LSHIFTBIN", "2MUL", "LSHIFTNUM"} {
		if _, err := parseShortForm(short); err != nil {
			t.Errorf("%s: %v", short, err)
		}
	}
}

// TestTokenOpcodesBeforeTokens ensures the token opcodes fail as reserved
// opcodes when executed before tokens are enabled, and are skipped in
// branches that are not executed.
func TestTokenOpcodesBeforeTokens(t *testing.T) {
	t.Parallel()

	flags := ScriptVerifyNativeIntrospection
	ops := []byte{OP_UTXOTOKENCATEGORY, OP_UTXOTOKENCOMMITMENT,
		OP_UTXOTOKENAMOUNT, OP_OUTPUTTOKENCATEGORY,
		OP_OUTPUTTOKENCOMMITMENT, OP_OUTPUTTOKENAMOUNT}
	for _, op := range ops {
		name := opcodeArray[op].name
		for _, cond := range []byte{OP_0, OP_1} {
			pkScript := []byte{OP_0, cond, OP_IF, op, OP_ENDIF,
				OP_DROP, OP_1}
			tx := createSpendingTx(nil, pkScript, 0)
			vm, err := NewEngine(pkScript, tx, 0, flags, nil, nil,
				nil, 0)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			err = vm.Execute()
			if cond == OP_0 && err != nil {
				t.Errorf("%s in an unexecuted branch: %v", name, err)
			}
			if cond == OP_1 && !IsErrorCode(err, ErrReservedOpcode) {
				t.Errorf("%s: got %v, want %v", name, err,
					ErrReservedOpcode)
			}
		}
	}
}

// num2binTestFlags returns the script flag sets used by the OP_NUM2BIN tests:
// the rules before the May 2022 upgrade (32-bit integers), the May 2022 rules
// (64-bit integers) and the current rules (big integers and larger stack
// elements).
func num2binTestFlags() map[string]ScriptFlags {
	legacy := ScriptBip16 | ScriptVerifyDERSignatures |
		ScriptVerifyCheckLockTimeVerify | ScriptVerifyCheckSequenceVerify |
		ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash |
		ScriptVerifyLowS | ScriptVerifyNullFail | ScriptVerifySigPushOnly |
		ScriptVerifyCleanStack | ScriptVerifyCheckDataSig |
		ScriptVerifySchnorr | ScriptVerifyAllowSegwitRecovery |
		ScriptVerifyMinimalData | ScriptVerifySchnorrMultisig |
		ScriptReportSigChecks | ScriptVerifyReverseBytes
	may2022 := legacy | ScriptVerify64BitIntegers |
		ScriptVerifyNativeIntrospection
	current := may2022 | ScriptAllowCashTokens | ScriptAllowMay2025 |
		ScriptAllowMay2026

	return map[string]ScriptFlags{
		"pre-May 2022": legacy,
		"May 2022":     may2022,
		"current":      current,
	}
}

// runNum2binTestScript executes the given short form signature and public key
// scripts with the passed flags.
func runNum2binTestScript(t *testing.T, sigScript, pkScript string,
	flags ScriptFlags) error {

	t.Helper()

	sig, err := parseShortForm(sigScript)
	if err != nil {
		t.Fatalf("parse %q: %v", sigScript, err)
	}
	pk, err := parseShortForm(pkScript)
	if err != nil {
		t.Fatalf("parse %q: %v", pkScript, err)
	}

	tx := createSpendingTx(sig, pk, 1000)
	utxos := NewUtxoCache()
	utxos.AddEntry(0, *wire.NewTxOut(1000, pk, wire.TokenData{}))
	vm, err := NewEngine(pk, tx, 0, flags, nil, nil, utxos, 1000)
	if err != nil {
		return err
	}
	return vm.Execute()
}

// TestNum2binNonMinimalNegative ensures OP_NUM2BIN keeps the sign and value of
// negative operands that are not minimally encoded, matching Bitcoin Cash
// Node.  OP_BIN2NUM is checked against the same operands for comparison.
func TestNum2binNonMinimalNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sigScript string
		pkScript  string
		err       ErrorCode
		wantErr   bool
	}{
		// -255 with one padding byte.
		{sigScript: "0x03 0xff0080 2", pkScript: "NUM2BIN 0x02 0xff80 EQUAL"},
		{sigScript: "0x03 0xff0080 3", pkScript: "NUM2BIN 0x03 0xff0080 EQUAL"},
		{sigScript: "0x03 0xff0080 4", pkScript: "NUM2BIN 0x04 0xff000080 EQUAL"},
		{
			sigScript: "0x03 0xff0080 1",
			pkScript:  "NUM2BIN DROP 1",
			err:       ErrNumberTooSmall,
			wantErr:   true,
		},
		{sigScript: "0x03 0xff0080", pkScript: "BIN2NUM 0x02 0xff80 EQUAL"},

		// -128 with two padding bytes.
		{sigScript: "0x04 0x80000080 2", pkScript: "NUM2BIN 0x02 0x8080 EQUAL"},
		{sigScript: "0x04 0x80000080 3", pkScript: "NUM2BIN 0x03 0x800080 EQUAL"},
		{sigScript: "0x04 0x80000080", pkScript: "BIN2NUM 0x02 0x8080 EQUAL"},

		// -32768 with one padding byte.
		{sigScript: "0x04 0x00800080 3", pkScript: "NUM2BIN 0x03 0x008080 EQUAL"},

		// Positive values whose most significant byte uses the sign bit.
		{sigScript: "0x03 0xff0000 2", pkScript: "NUM2BIN 0x02 0xff00 EQUAL"},
		{sigScript: "0x04 0x80000000 2", pkScript: "NUM2BIN 0x02 0x8000 EQUAL"},
	}

	for name, flags := range num2binTestFlags() {
		for _, test := range tests {
			err := runNum2binTestScript(t, test.sigScript, test.pkScript,
				flags)
			switch {
			case test.wantErr && !IsErrorCode(err, test.err):
				t.Errorf("%s: [%s] [%s]: got error %v, want %v", name,
					test.sigScript, test.pkScript, err, test.err)
			case !test.wantErr && err != nil:
				t.Errorf("%s: [%s] [%s]: unexpected error %v", name,
					test.sigScript, test.pkScript, err)
			}
		}
	}
}

// TestNum2binSizeBounds ensures the size operand of OP_NUM2BIN is bounds
// checked on its full 64-bit value, so that sizes which do not fit in a
// 32-bit int are rejected the same way on every platform.
func TestNum2binSizeBounds(t *testing.T) {
	t.Parallel()

	type num2binSizeTest struct {
		name      string
		sigScript string
		err       ErrorCode
		wantErr   bool
	}

	common := []num2binSizeTest{
		{name: "negative", sigScript: "1 -3", err: ErrNumberTooSmall, wantErr: true},
		{name: "max legacy element", sigScript: "1 520"},
	}
	wide := []num2binSizeTest{
		{
			name:      "2^31",
			sigScript: "1 0x05 0x0000008000",
			err:       ErrNumberTooBig,
			wantErr:   true,
		},
		{
			name:      "2^32+2",
			sigScript: "1 0x05 0x0200000001",
			err:       ErrNumberTooBig,
			wantErr:   true,
		},
		{
			name:      "-(2^32-3)",
			sigScript: "1 0x05 0xfdffffff80",
			err:       ErrNumberTooSmall,
			wantErr:   true,
		},
		{
			name:      "2^63-1",
			sigScript: "1 0x08 0xffffffffffffff7f",
			err:       ErrNumberTooBig,
			wantErr:   true,
		},
	}

	tests := map[string][]num2binSizeTest{
		"pre-May 2022": append(common, num2binSizeTest{
			name:      "max legacy element + 1",
			sigScript: "1 521",
			err:       ErrNumberTooBig,
			wantErr:   true,
		}),
		"May 2022": append(append(common, wide...), num2binSizeTest{
			name:      "max legacy element + 1",
			sigScript: "1 521",
			err:       ErrNumberTooBig,
			wantErr:   true,
		}),
		"current": append(append(common, wide...),
			num2binSizeTest{name: "max element", sigScript: "1 10000"},
			num2binSizeTest{
				name:      "max element + 1",
				sigScript: "1 10001",
				err:       ErrNumberTooBig,
				wantErr:   true,
			},
			num2binSizeTest{
				name:      "2^64+2",
				sigScript: "1 0x09 0x020000000000000001",
				err:       ErrNumberTooBig,
				wantErr:   true,
			},
		),
	}

	flagSets := num2binTestFlags()
	for name, flagTests := range tests {
		for _, test := range flagTests {
			err := runNum2binTestScript(t, test.sigScript,
				"NUM2BIN DROP 1", flagSets[name])
			switch {
			case test.wantErr && !IsErrorCode(err, test.err):
				t.Errorf("%s: %s: got error %v, want %v", name, test.name,
					err, test.err)
			case !test.wantErr && err != nil:
				t.Errorf("%s: %s: unexpected error %v", name, test.name,
					err)
			}
		}
	}
}
