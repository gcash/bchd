// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
)

// TestBadPC sets the pc to a deliberately bad result then confirms that Step()
// and Disasm fail correctly.
func TestBadPC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		script, off int
	}{
		{script: 2, off: 0},
		{script: 0, off: 2},
	}

	// tx with almost empty scripts.
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash: chainhash.Hash([32]byte{
						0xc9, 0x97, 0xa5, 0xe5,
						0x6e, 0x10, 0x41, 0x02,
						0xfa, 0x20, 0x9c, 0x6a,
						0x85, 0x2d, 0xd9, 0x06,
						0x60, 0xa2, 0x0b, 0x2d,
						0x9c, 0x35, 0x24, 0x23,
						0xed, 0xce, 0x25, 0x85,
						0x7f, 0xcd, 0x37, 0x04,
					}),
					Index: 0,
				},
				SignatureScript: mustParseShortForm("NOP"),
				Sequence:        4294967295,
			},
		},
		TxOut: []*wire.TxOut{{
			Value:    1000000000,
			PkScript: nil,
		}},
		LockTime: 0,
	}
	pkScript := mustParseShortForm("NOP")

	for _, test := range tests {
		vm, err := NewEngine(pkScript, tx, 0, 0, nil, nil, nil, -1)
		if err != nil {
			t.Errorf("Failed to create script: %v", err)
		}

		// set to after all scripts
		vm.scriptIdx = test.script
		vm.scriptOff = test.off

		_, err = vm.Step()
		if err == nil {
			t.Errorf("Step with invalid pc (%v) succeeds!", test)
			continue
		}

		_, err = vm.DisasmPC()
		if err == nil {
			t.Errorf("DisasmPC with invalid pc (%v) succeeds!",
				test)
		}
	}
}

// TestCheckErrorCondition tests the execute early test in CheckErrorCondition()
// since most code paths are tested elsewhere.
func TestCheckErrorCondition(t *testing.T) {
	t.Parallel()

	// tx with almost empty scripts.
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash: chainhash.Hash([32]byte{
					0xc9, 0x97, 0xa5, 0xe5,
					0x6e, 0x10, 0x41, 0x02,
					0xfa, 0x20, 0x9c, 0x6a,
					0x85, 0x2d, 0xd9, 0x06,
					0x60, 0xa2, 0x0b, 0x2d,
					0x9c, 0x35, 0x24, 0x23,
					0xed, 0xce, 0x25, 0x85,
					0x7f, 0xcd, 0x37, 0x04,
				}),
				Index: 0,
			},
			SignatureScript: nil,
			Sequence:        4294967295,
		}},
		TxOut: []*wire.TxOut{{
			Value:    1000000000,
			PkScript: nil,
		}},
		LockTime: 0,
	}
	pkScript := mustParseShortForm("NOP NOP NOP NOP NOP NOP NOP NOP NOP" +
		" NOP TRUE")

	vm, err := NewEngine(pkScript, tx, 0, 0, nil, nil, nil, 0)
	if err != nil {
		t.Errorf("failed to create script: %v", err)
	}

	for i := range len(pkScript) - 1 {
		done, err := vm.Step()
		if err != nil {
			t.Fatalf("failed to step %dth time: %v", i, err)
		}
		if done {
			t.Fatalf("finished early on %dth time", i)
		}

		err = vm.CheckErrorCondition(false)
		if !IsErrorCode(err, ErrScriptUnfinished) {
			t.Fatalf("got unexpected error %v on %dth iteration",
				err, i)
		}
	}
	done, err := vm.Step()
	if err != nil {
		t.Fatalf("final step failed %v", err)
	}
	if !done {
		t.Fatalf("final step isn't done!")
	}

	err = vm.CheckErrorCondition(false)
	if err != nil {
		t.Errorf("unexpected error %v on final check", err)
	}
}

// TestExecuteTraceLog ensures Execute logs every step at the trace level and
// nothing at a higher level.  It must not run in parallel since it replaces
// the package logger.
func TestExecuteTraceLog(t *testing.T) {
	defer DisableLog()

	pkScript := mustParseShortForm("1 DUP DROP")
	tx := createSpendingTx(nil, pkScript, 0)
	run := func(level bchlog.Level) string {
		var buf bytes.Buffer
		logger := bchlog.NewBackend(&buf).Logger("SCRP")
		logger.SetLevel(level)
		UseLogger(logger)
		vm, err := NewEngine(pkScript, tx, 0, 0, nil, nil, nil, 0)
		if err != nil {
			t.Fatalf("NewEngine: %v", err)
		}
		if err := vm.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		return buf.String()
	}

	out := run(bchlog.LevelTrace)
	if n := strings.Count(out, "stepping"); n != 3 {
		t.Fatalf("got %d stepping lines, want 3:\n%s", n, out)
	}
	if !strings.Contains(out, "Stack:") {
		t.Fatalf("trace log has no stack dump:\n%s", out)
	}
	if out := run(bchlog.LevelDebug); out != "" {
		t.Fatalf("unexpected log output at debug level:\n%s", out)
	}
}

// TestSetStack ensures SetStack and SetAltStack replace the stack contents.
func TestSetStack(t *testing.T) {
	t.Parallel()

	pkScript := mustParseShortForm("1")
	tx := createSpendingTx(nil, pkScript, 0)
	vm, err := NewEngine(pkScript, tx, 0, 0, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	long := [][]byte{{1}, {2}, {3}}
	short := [][]byte{{4}}
	vm.SetStack(long)
	vm.SetAltStack(long)
	vm.SetStack(short)
	vm.SetAltStack(nil)
	if got := vm.GetStack(); !reflect.DeepEqual(got, short) {
		t.Fatalf("got stack %v, want %v", got, short)
	}
	if got := vm.GetAltStack(); len(got) != 0 {
		t.Fatalf("got alt stack %v, want empty", got)
	}
	if got := vm.dstack.stk[:3]; got[1] != nil || got[2] != nil {
		t.Fatalf("freed stack slots still hold %v", got[1:])
	}
}

// TestInvalidFlagCombinations ensures the script engine returns the expected
// error when disallowed flag combinations are specified.
func TestInvalidFlagCombinations(t *testing.T) {
	t.Parallel()

	tests := []ScriptFlags{
		ScriptVerifyCleanStack,
	}

	// tx with almost empty scripts.
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash: chainhash.Hash([32]byte{
						0xc9, 0x97, 0xa5, 0xe5,
						0x6e, 0x10, 0x41, 0x02,
						0xfa, 0x20, 0x9c, 0x6a,
						0x85, 0x2d, 0xd9, 0x06,
						0x60, 0xa2, 0x0b, 0x2d,
						0x9c, 0x35, 0x24, 0x23,
						0xed, 0xce, 0x25, 0x85,
						0x7f, 0xcd, 0x37, 0x04,
					}),
					Index: 0,
				},
				SignatureScript: []uint8{OP_NOP},
				Sequence:        4294967295,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    1000000000,
				PkScript: nil,
			},
		},
		LockTime: 0,
	}
	pkScript := []byte{OP_NOP}

	for i, test := range tests {
		_, err := NewEngine(pkScript, tx, 0, test, nil, nil, nil, -1)
		if !IsErrorCode(err, ErrInvalidFlags) {
			t.Fatalf("TestInvalidFlagCombinations #%d unexpected "+
				"error: %v", i, err)
		}
	}
}

// TestCheckPubKeyEncoding ensures the internal checkPubKeyEncoding function
// works as expected.
func TestCheckPubKeyEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     []byte
		isValid bool
	}{
		{
			name: "uncompressed ok",
			key: hexToBytes("0411db93e1dcdb8a016b49840f8c53bc1eb68" +
				"a382e97b1482ecad7b148a6909a5cb2e0eaddfb84ccf" +
				"9744464f82e160bfa9b8b64f9d4c03f999b8643f656b" +
				"412a3"),
			isValid: true,
		},
		{
			name: "compressed ok",
			key: hexToBytes("02ce0b14fb842b1ba549fdd675c98075f12e9" +
				"c510f8ef52bd021a9a1f4809d3b4d"),
			isValid: true,
		},
		{
			name: "compressed ok",
			key: hexToBytes("032689c7c2dab13309fb143e0e8fe39634252" +
				"1887e976690b6b47f5b2a4b7d448e"),
			isValid: true,
		},
		{
			name: "hybrid",
			key: hexToBytes("0679be667ef9dcbbac55a06295ce870b07029" +
				"bfcdb2dce28d959f2815b16f81798483ada7726a3c46" +
				"55da4fbfc0e1108a8fd17b448a68554199c47d08ffb1" +
				"0d4b8"),
			isValid: false,
		},
		{
			name:    "empty",
			key:     nil,
			isValid: false,
		},
	}

	vm := Engine{flags: ScriptVerifyStrictEncoding}
	for _, test := range tests {
		err := vm.checkPubKeyEncoding(test.key)
		if err != nil && test.isValid {
			t.Errorf("checkSignatureEncoding test '%s' failed "+
				"when it should have succeeded: %v", test.name,
				err)
		} else if err == nil && !test.isValid {
			t.Errorf("checkSignatureEncooding test '%s' succeeded "+
				"when it should have failed", test.name)
		}
	}

}

// TestCheckSignatureEncoding ensures the internal checkSignatureEncoding
// function works as expected.
func TestCheckSignatureEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sig     []byte
		isValid bool
	}{
		{
			name: "valid signature",
			sig: hexToBytes("304402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: true,
		},
		{
			name:    "empty.",
			sig:     nil,
			isValid: false,
		},
		{
			name: "bad magic",
			sig: hexToBytes("314402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "bad 1st int marker magic",
			sig: hexToBytes("304403204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "bad 2nd int marker",
			sig: hexToBytes("304402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41032018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "short len",
			sig: hexToBytes("304302204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "long len",
			sig: hexToBytes("304502204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "long X",
			sig: hexToBytes("304402424e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "long Y",
			sig: hexToBytes("304402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022118152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "short Y",
			sig: hexToBytes("304402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41021918152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "trailing crap",
			sig: hexToBytes("304402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d0901"),
			isValid: false,
		},
		{
			name: "X == N ",
			sig: hexToBytes("30440220fffffffffffffffffffffffffffff" +
				"ffebaaedce6af48a03bbfd25e8cd0364141022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "X == N ",
			sig: hexToBytes("30440220fffffffffffffffffffffffffffff" +
				"ffebaaedce6af48a03bbfd25e8cd0364142022018152" +
				"2ec8eca07de4860a4acdd12909d831cc56cbbac46220" +
				"82221a8768d1d09"),
			isValid: false,
		},
		{
			name: "Y == N",
			sig: hexToBytes("304402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd410220fffff" +
				"ffffffffffffffffffffffffffebaaedce6af48a03bb" +
				"fd25e8cd0364141"),
			isValid: false,
		},
		{
			name: "Y > N",
			sig: hexToBytes("304402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd410220fffff" +
				"ffffffffffffffffffffffffffebaaedce6af48a03bb" +
				"fd25e8cd0364142"),
			isValid: false,
		},
		{
			name: "0 len X",
			sig: hexToBytes("302402000220181522ec8eca07de4860a4acd" +
				"d12909d831cc56cbbac4622082221a8768d1d09"),
			isValid: false,
		},
		{
			name: "0 len Y",
			sig: hexToBytes("302402204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd410200"),
			isValid: false,
		},
		{
			name: "extra R padding",
			sig: hexToBytes("30450221004e45e16932b8af514961a1d3a1a" +
				"25fdf3f4f7732e9d624c6c61548ab5fb8cd410220181" +
				"522ec8eca07de4860a4acdd12909d831cc56cbbac462" +
				"2082221a8768d1d09"),
			isValid: false,
		},
		{
			name: "extra S padding",
			sig: hexToBytes("304502204e45e16932b8af514961a1d3a1a25" +
				"fdf3f4f7732e9d624c6c61548ab5fb8cd41022100181" +
				"522ec8eca07de4860a4acdd12909d831cc56cbbac462" +
				"2082221a8768d1d09"),
			isValid: false,
		},
	}

	vm := Engine{flags: ScriptVerifyStrictEncoding}
	for _, test := range tests {
		err := vm.checkSignatureEncoding(test.sig)
		if err != nil && test.isValid {
			t.Errorf("checkSignatureEncoding test '%s' failed "+
				"when it should have succeeded: %v", test.name,
				err)
		} else if err == nil && !test.isValid {
			t.Errorf("checkSignatureEncooding test '%s' succeeded "+
				"when it should have failed", test.name)
		}
	}
}

func TestCheckHashTypeEncoding(t *testing.T) {
	var SigHashBug SigHashType = 0x20
	encodingTests := []struct {
		SigHash     SigHashType
		EngineFlags ScriptFlags
		ShouldFail  bool
	}{
		{
			SigHashAll,
			ScriptVerifyStrictEncoding,
			false,
		},
		{
			SigHashNone,
			ScriptVerifyStrictEncoding,
			false,
		},
		{
			SigHashSingle,
			ScriptVerifyStrictEncoding,
			false,
		},
		{
			SigHashAll | SigHashAnyOneCanPay,
			ScriptVerifyStrictEncoding,
			false,
		},
		{
			SigHashNone | SigHashAnyOneCanPay,
			ScriptVerifyStrictEncoding,
			false,
		},
		{
			SigHashSingle | SigHashAnyOneCanPay,
			ScriptVerifyStrictEncoding,
			false,
		},

		{
			SigHashAll | SigHashForkID,
			ScriptVerifyStrictEncoding,
			true,
		},
		{
			SigHashNone | SigHashForkID,
			ScriptVerifyStrictEncoding,
			true,
		},
		{
			SigHashSingle | SigHashForkID,
			ScriptVerifyStrictEncoding,
			true,
		},
		{
			SigHashAll | SigHashAnyOneCanPay | SigHashForkID,
			ScriptVerifyStrictEncoding,
			true,
		},
		{
			SigHashNone | SigHashAnyOneCanPay | SigHashForkID,
			ScriptVerifyStrictEncoding,
			true,
		},
		{
			SigHashSingle | SigHashAnyOneCanPay | SigHashForkID,
			ScriptVerifyStrictEncoding,
			true,
		},

		{
			SigHashAll | SigHashForkID,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			false,
		},
		{
			SigHashNone | SigHashForkID,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			false,
		},
		{
			SigHashSingle | SigHashForkID,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			false,
		},
		{
			SigHashAll | SigHashAnyOneCanPay | SigHashForkID,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			false,
		},
		{
			SigHashNone | SigHashAnyOneCanPay | SigHashForkID,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			false,
		},
		{
			SigHashSingle | SigHashAnyOneCanPay | SigHashForkID,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			false,
		},

		{
			SigHashAll,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			true,
		},
		{
			SigHashNone,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			true,
		},
		{
			SigHashSingle,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			true,
		},
		{
			SigHashAll | SigHashAnyOneCanPay,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			true,
		},
		{
			SigHashNone | SigHashAnyOneCanPay,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			true,
		},
		{
			SigHashSingle | SigHashAnyOneCanPay,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			true,
		},
		{
			SigHashSingle | SigHashAnyOneCanPay | SigHashForkID | SigHashBug,
			ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash,
			true,
		},
	}

	for i, test := range encodingTests {
		e := Engine{flags: test.EngineFlags}
		err := e.checkHashTypeEncoding(test.SigHash)
		if test.ShouldFail && err == nil {
			t.Errorf("Expected test %d to fail", i)
		} else if !test.ShouldFail && err != nil {
			t.Errorf("Expected test %d not to fail", i)
		}
	}
}

func Test_isSegwitScript(t *testing.T) {
	t.Parallel()
	// Test vectors from https://github.com/bitcoincashorg/bitcoincash.org/blob/master/spec/2019-05-15-segwit-recovery.md
	tests := []struct {
		Name   string
		Script []byte
		Valid  bool
	}{
		{
			"recovering v0 P2SH-P2WPKH",
			hexToBytes("16001491b24bf9f5288532960ac687abb035127b1d28a5"),
			true,
		},
		{
			"recovering v0 P2SH-P2WSH",
			hexToBytes("2200205a0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			true,
		},
		{
			"max allowed version, v16",
			hexToBytes("2260205a0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			true,
		},
		{
			"max allowed length, 42 bytes",
			hexToBytes("2a00285a0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2021222324252627"),
			true,
		},
		{
			"non-minimal push of redeemscript",
			hexToBytes("4e16000000001491b24bf9f5288532960ac687abb035127b1d28a5"),
			true,
		},
		{
			"min allowed length, 4 bytes, valid in spite of a false boolean value being left on stack",
			hexToBytes("0451020000"),
			true,
		},
		{
			"min allowed length, 4 bytes, valid in spite of a false boolean value being left on stack",
			hexToBytes("0451020080"),
			true,
		},
		/*{
			"if not spending a P2SH coin",
			hexToBytes("16001491b24bf9f5288532960ac687abb035127b1d28a5"),
			false,
		},
		{
			"if hash does not match P2SH output",
			hexToBytes("16001491b24bf9f5288532960ac687abb035127b1d28a5"),
			false,
		},*/
		{
			"scriptSig pushes two items onto the stack",
			hexToBytes("0016001491b24bf9f5288532960ac687abb035127b1d28a5"),
			false,
		},
		{
			"invalid witness program, non-minimal push in version field",
			hexToBytes("1701001491b24bf9f5288532960ac687abb035127b1d28a5"),
			false,
		},
		{
			"invalid witness program, non-minimal push in program field",
			hexToBytes("05004c0245aa"),
			false,
		},
		{
			"invalid witness program, too short",
			hexToBytes("0300015a"),
			false,
		},
		{
			"invalid witness program, too long",
			hexToBytes("2b00295a0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728"),
			false,
		},
		{
			"invalid witness program, version -1",
			hexToBytes("224f205a0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			false,
		},
		{
			"invalid witness program, version 17",
			hexToBytes("230111205a0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			false,
		},
		{
			"invalid witness program, more than 2 stack items",
			hexToBytes("2300205a0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f51"),
			false,
		},
	}

	for _, test := range tests {
		parsedScript, err := parseScript(test.Script)
		if err != nil {
			t.Fatal(err)
		}
		valid := isSegwitScript(parsedScript)
		if valid != test.Valid {
			t.Errorf("%s: expected %t got %t", test.Name, test.Valid, valid)
		}
	}
}

func TestSegwitExemption(t *testing.T) {
	redeemScript := hexToBytes("0014fcf9969ce1c98a135ed293719721fb69f0b686cb")

	addr, err := bchutil.NewAddressScriptHash(redeemScript, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}

	pkScript, err := PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}

	// tx with almost empty scripts.
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash: chainhash.Hash([32]byte{
						0xc9, 0x97, 0xa5, 0xe5,
						0x6e, 0x10, 0x41, 0x02,
						0xfa, 0x20, 0x9c, 0x6a,
						0x85, 0x2d, 0xd9, 0x06,
						0x60, 0xa2, 0x0b, 0x2d,
						0x9c, 0x35, 0x24, 0x23,
						0xed, 0xce, 0x25, 0x85,
						0x7f, 0xcd, 0x37, 0x04,
					}),
					Index: 0,
				},
				SignatureScript: hexToBytes("160014fcf9969ce1c98a135ed293719721fb69f0b686cb"),
				Sequence:        4294967295,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    1000000000,
				PkScript: nil,
			},
		},
		LockTime: 0,
	}

	// This should fail the clean stack rule.
	vm, err := NewEngine(pkScript, tx, 0, ScriptVerifyCleanStack|ScriptBip16, nil, nil, nil, 0)
	if err != nil {
		t.Errorf("failed to create script: %v", err)
	}
	if err := vm.Execute(); !IsErrorCode(err, ErrCleanStack) {
		t.Errorf("TestSegwitExemption expected ErrCleanStack got "+
			"error: %v", err)
	}

	// We add the segwit exemption flag and now the same input should pass.
	vm, err = NewEngine(pkScript, tx, 0, ScriptVerifyCleanStack|ScriptBip16|ScriptVerifyAllowSegwitRecovery, nil, nil, nil, 0)
	if err != nil {
		t.Errorf("failed to create script: %v", err)
	}
	if err := vm.Execute(); err != nil {
		t.Errorf("TestSegwitExemption expected segwit exemption to pass")
	}
}

func TestScriptVerifyInputSigChecks(t *testing.T) {
	priv, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.PubKey()

	tests := []struct {
		buildRedeemScript func() ([]byte, error)
		tooManySigChecks  bool
	}{
		{
			buildRedeemScript: func() ([]byte, error) {
				builder := NewScriptBuilder().
					AddData(pub.SerializeCompressed()).
					AddOp(OP_2DUP).
					AddOp(OP_2DUP).
					AddOp(OP_2DUP).
					AddOp(OP_CHECKSIGVERIFY).
					AddOp(OP_CHECKSIGVERIFY).
					AddOp(OP_CHECKSIGVERIFY).
					AddOp(OP_CHECKSIG)

				return builder.Script()
			},
			tooManySigChecks: true,
		},
		{
			buildRedeemScript: func() ([]byte, error) {
				builder := NewScriptBuilder().
					AddData(pub.SerializeCompressed()).
					AddOp(OP_2DUP).
					AddOp(OP_2DUP).
					AddOp(OP_CHECKSIGVERIFY).
					AddOp(OP_CHECKSIGVERIFY).
					AddOp(OP_CHECKSIG)

				return builder.Script()
			},
			tooManySigChecks: false,
		},
	}

	for _, test := range tests {
		redeemScript, err := test.buildRedeemScript()
		if err != nil {
			t.Fatal(err)
		}

		addr, err := bchutil.NewAddressScriptHash(redeemScript, &chaincfg.TestNet3Params)
		if err != nil {
			t.Fatal(err)
		}

		pkScript, err := PayToAddrScript(addr)
		if err != nil {
			t.Fatal(err)
		}

		tx := &wire.MsgTx{
			Version: 1,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash: chainhash.Hash([32]byte{
							0xc9, 0x97, 0xa5, 0xe5,
							0x6e, 0x10, 0x41, 0x02,
							0xfa, 0x20, 0x9c, 0x6a,
							0x85, 0x2d, 0xd9, 0x06,
							0x60, 0xa2, 0x0b, 0x2d,
							0x9c, 0x35, 0x24, 0x23,
							0xed, 0xce, 0x25, 0x85,
							0x7f, 0xcd, 0x37, 0x04,
						}),
						Index: 0,
					},
					Sequence: 4294967295,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value:    1000000000,
					PkScript: nil,
				},
			},
			LockTime: 0,
		}

		sig, err := RawTxInSchnorrSignature(tx, 0, redeemScript, SigHashAll, priv, 0)
		if err != nil {
			t.Fatal(err)
		}

		sigBuilder := NewScriptBuilder().addData(sig).addData(redeemScript)
		sigBytes, err := sigBuilder.Script()
		if err != nil {
			t.Fatal(err)
		}

		tx.TxIn[0].SignatureScript = sigBytes

		vm, err := NewEngine(pkScript, tx, 0, StandardVerifyFlags, nil, nil, nil, 0)
		if err != nil {
			t.Errorf("failed to create script: %v", err)
		}
		err = vm.Execute()
		if test.tooManySigChecks && !IsErrorCode(err, ErrInputSigChecks) {
			t.Errorf("TestScriptVerifyInputSigChecks expected ErrInputSigChecks got "+
				"error: %v", err)
		} else if !test.tooManySigChecks && err != nil {
			t.Errorf("TestScriptVerifyInputSigChecks expected successful execution got "+
				"error: %v", err)
		}
	}
}

// multiSigTestKeys returns n deterministic private keys.
func multiSigTestKeys(n int) []*bchec.PrivateKey {
	keys := make([]*bchec.PrivateKey, n)
	for i := range keys {
		keys[i], _ = bchec.PrivKeyFromBytes(bchec.S256(),
			chainhash.HashB([]byte{byte(i)}))
	}
	return keys
}

// runLegacyMultiSig spends pkScript with a null dummy, an ECDSA signature
// over subScript by each signer or a null signature for each nil signer,
// and redeemScript when it is not nil, and returns the executed engine.
func runLegacyMultiSig(t *testing.T, pkScript, subScript, redeemScript []byte,
	signers []*bchec.PrivateKey, flags ScriptFlags) (*Engine, error) {

	t.Helper()

	tx := createSpendingTx(nil, pkScript, 1000)
	b := NewScriptBuilder().AddOp(OP_0)
	for _, signer := range signers {
		if signer == nil {
			b.AddOp(OP_0)
			continue
		}
		sig, err := RawTxInECDSASignature(tx, 0, subScript, SigHashAll,
			signer, 1000)
		if err != nil {
			t.Fatal(err)
		}
		b.AddData(sig)
	}
	if redeemScript != nil {
		b.AddData(redeemScript)
	}
	sigScript, err := b.Script()
	if err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].SignatureScript = sigScript

	utxos := NewUtxoCache()
	utxos.AddEntry(0, *wire.NewTxOut(1000, pkScript, wire.TokenData{}))
	vm, err := NewEngine(pkScript, tx, 0, flags, nil, nil, utxos, 1000)
	if err != nil {
		return nil, err
	}
	return vm, vm.Execute()
}

// TestLegacyCheckMultiSigSigChecks ensures a legacy (ECDSA) OP_CHECKMULTISIG
// counts every public key as a signature check when any signature is not
// null, and none when all signatures are null.
func TestLegacyCheckMultiSigSigChecks(t *testing.T) {
	t.Parallel()

	// The signature density and operation cost limits would reject the
	// short unlocking scripts used here.
	flags := (StandardVerifyFlags | ScriptAllowCashTokens) &^
		ScriptVerifyInputSigChecks
	other := multiSigTestKeys(21)[20]

	tests := []struct {
		name     string
		signers  func(keys []*bchec.PrivateKey) []*bchec.PrivateKey
		result   bool
		nullFail bool
		nonNull  bool
	}{{
		name: "no signatures",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return nil
		},
		result:   true,
		nullFail: true,
	}, {
		name: "one null signature",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return []*bchec.PrivateKey{nil}
		},
		nullFail: true,
	}, {
		name: "all null signatures",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return make([]*bchec.PrivateKey, len(keys))
		},
		nullFail: true,
	}, {
		name: "first key signs",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return keys[:1]
		},
		result:   true,
		nullFail: true,
		nonNull:  true,
	}, {
		name: "last key signs",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return keys[len(keys)-1:]
		},
		result:   true,
		nullFail: true,
		nonNull:  true,
	}, {
		name: "all keys sign",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return keys
		},
		result:   true,
		nullFail: true,
		nonNull:  true,
	}, {
		name: "other key signs",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return []*bchec.PrivateKey{other}
		},
		nonNull: true,
	}, {
		// The null signature is checked first and fails the
		// multisig before the other signature is reached.
		name: "other key signs before a null signature",
		signers: func(keys []*bchec.PrivateKey) []*bchec.PrivateKey {
			return []*bchec.PrivateKey{other, nil}
		},
		nonNull: true,
	}}

	for _, numKeys := range []int{1, 3, 15, 20} {
		keys := multiSigTestKeys(numKeys)
		for _, test := range tests {
			signers := test.signers(keys)
			if len(signers) > numKeys {
				continue
			}

			b := NewScriptBuilder().AddInt64(int64(len(signers)))
			for _, key := range keys {
				b.AddData(key.PubKey().SerializeCompressed())
			}
			b.AddInt64(int64(numKeys)).AddOp(OP_CHECKMULTISIG)
			if !test.result {
				b.AddOp(OP_NOT)
			}
			pkScript, err := b.Script()
			if err != nil {
				t.Fatal(err)
			}

			testFlags := flags
			if !test.nullFail {
				testFlags &^= ScriptVerifyNullFail
			}
			vm, err := runLegacyMultiSig(t, pkScript, pkScript, nil,
				signers, testFlags)
			if err != nil {
				t.Errorf("%d keys, %s: unexpected error: %v", numKeys,
					test.name, err)
				continue
			}

			want := 0
			if test.nonNull {
				want = numKeys
			}
			if vm.SigChecks() != want {
				t.Errorf("%d keys, %s: got %d sig checks, want %d",
					numKeys, test.name, vm.SigChecks(), want)
			}
		}
	}
}

// TestLegacyCheckMultiSigInputSigChecks ensures the signature density limit
// counts all 15 public keys of a P2SH 1-of-15 legacy multisig.  The keys
// after the signing key are one-byte placeholders.
func TestLegacyCheckMultiSigInputSigChecks(t *testing.T) {
	t.Parallel()

	flags := StandardVerifyFlags | ScriptAllowCashTokens |
		ScriptAllowMay2025 | ScriptAllowMay2025StandardOnly |
		ScriptAllowMay2026
	keys := multiSigTestKeys(15)

	tests := []struct {
		numShort int
		wantErr  bool
	}{
		{0, false},
		{1, true},
		{3, true},
	}

	for _, test := range tests {
		// The signing key comes after numShort one byte public keys.
		b := NewScriptBuilder().AddOp(OP_1)
		for i, key := range keys {
			if i < test.numShort {
				b.AddOp(OP_0)
				continue
			}
			b.AddData(key.PubKey().SerializeCompressed())
		}
		redeemScript, err := b.AddOp(OP_15).AddOp(OP_CHECKMULTISIG).Script()
		if err != nil {
			t.Fatal(err)
		}
		addr, err := bchutil.NewAddressScriptHash(redeemScript,
			&chaincfg.MainNetParams)
		if err != nil {
			t.Fatal(err)
		}
		pkScript, err := PayToAddrScript(addr)
		if err != nil {
			t.Fatal(err)
		}

		signer := keys[test.numShort : test.numShort+1]
		_, err = runLegacyMultiSig(t, pkScript, redeemScript, redeemScript,
			signer, flags)
		switch {
		case test.wantErr && !IsErrorCode(err, ErrInputSigChecks):
			t.Errorf("%d short keys: got error %v, want %v",
				test.numShort, err, ErrInputSigChecks)
		case !test.wantErr && err != nil:
			t.Errorf("%d short keys: unexpected error: %v",
				test.numShort, err)
		}
	}
}

// TestCheckSigForkIDScriptCode ensures a signature that uses SIGHASH_FORKID is
// not removed from the script code it signs.  A function body comes from the
// unlocking script, so it can contain the signature it checks.
func TestCheckSigForkIDScriptCode(t *testing.T) {
	t.Parallel()

	may2026Flags := StandardVerifyFlags | ScriptAllowCashTokens |
		ScriptAllowMay2025 | ScriptAllowMay2025StandardOnly |
		ScriptAllowMay2026
	priv := multiSigTestKeys(1)[0]
	pubKey := priv.PubKey().SerializeCompressed()
	hashType := SigHashAll | SigHashForkID
	// OP_1 OP_DEFINE OP_1 OP_INVOKE
	invokeScript := []byte{OP_1, OP_DEFINE, OP_1, OP_INVOKE}

	mustScript := func(b *ScriptBuilder) []byte {
		script, err := b.Script()
		if err != nil {
			t.Fatal(err)
		}
		return script
	}

	// sign returns a Schnorr or ECDSA signature of the passed script code.
	sign := func(tx *wire.MsgTx, scriptCode []byte, schnorr,
		bip143 bool) []byte {

		utxos := NewUtxoCache()
		utxos.AddEntry(0, *wire.NewTxOut(1000, invokeScript,
			wire.TokenData{}))
		pops, err := parseScript(scriptCode)
		if err != nil {
			t.Fatal(err)
		}
		hash, _, err := calcSignatureHash(pops,
			NewTxSigHashes(tx).WithUtxos(tx, utxos), hashType, tx, 0,
			1000, bip143)
		if err != nil {
			t.Fatal(err)
		}
		var sig *bchec.Signature
		if schnorr {
			sig, err = priv.SignSchnorr(hash)
		} else {
			sig, err = priv.SignECDSA(hash)
		}
		if err != nil {
			t.Fatal(err)
		}
		return append(sig.Serialize(), byte(hashType))
	}

	tests := []struct {
		name    string
		schnorr bool
		// legacy runs without SIGHASH_FORKID enabled, with the body
		// as the locking script instead of a function body.
		legacy bool
		// signed is the script code the signature covers and body
		// returns the script that contains the signature.
		signed  []byte
		body    func(sig []byte) []byte
		wantErr ErrorCode
		valid   bool
	}{{
		name:    "checksig",
		schnorr: true,
		signed: mustScript(NewScriptBuilder().AddData(pubKey).
			AddOp(OP_CHECKSIG)),
		body: func(sig []byte) []byte {
			return mustScript(NewScriptBuilder().AddData(sig).
				AddData(pubKey).AddOp(OP_CHECKSIG))
		},
		wantErr: ErrNullFail,
	}, {
		name:    "checksigverify",
		schnorr: true,
		signed: mustScript(NewScriptBuilder().AddData(pubKey).
			AddOp(OP_CHECKSIGVERIFY).AddOp(OP_1)),
		body: func(sig []byte) []byte {
			return mustScript(NewScriptBuilder().AddData(sig).
				AddData(pubKey).AddOp(OP_CHECKSIGVERIFY).
				AddOp(OP_1))
		},
		wantErr: ErrNullFail,
	}, {
		name: "ECDSA multisig",
		signed: mustScript(NewScriptBuilder().AddOp(OP_0).AddOp(OP_1).
			AddData(pubKey).AddOp(OP_1).AddOp(OP_CHECKMULTISIG)),
		body: func(sig []byte) []byte {
			return mustScript(NewScriptBuilder().AddOp(OP_0).
				AddData(sig).AddOp(OP_1).AddData(pubKey).
				AddOp(OP_1).AddOp(OP_CHECKMULTISIG))
		},
		wantErr: ErrNullFail,
	}, {
		name:    "Schnorr multisig",
		schnorr: true,
		signed: mustScript(NewScriptBuilder().AddOp(OP_1).AddOp(OP_1).
			AddData(pubKey).AddOp(OP_1).AddOp(OP_CHECKMULTISIG)),
		body: func(sig []byte) []byte {
			return mustScript(NewScriptBuilder().AddOp(OP_1).
				AddData(sig).AddOp(OP_1).AddData(pubKey).
				AddOp(OP_1).AddOp(OP_CHECKMULTISIG))
		},
		wantErr: ErrNullFail,
	}, {
		// The signature is not in the script code after an
		// OP_CODESEPARATOR, so it can sign it.
		name:    "checksig after OP_CODESEPARATOR",
		schnorr: true,
		signed: mustScript(NewScriptBuilder().AddData(pubKey).
			AddOp(OP_CHECKSIG)),
		body: func(sig []byte) []byte {
			return mustScript(NewScriptBuilder().AddData(sig).
				AddOp(OP_CODESEPARATOR).AddData(pubKey).
				AddOp(OP_CHECKSIG))
		},
		valid: true,
	}, {
		// Without SIGHASH_FORKID enabled the fork bit does not select
		// the fork id algorithm, so the signature is removed.
		name:   "checksig without SIGHASH_FORKID enabled",
		legacy: true,
		signed: mustScript(NewScriptBuilder().AddData(pubKey).
			AddOp(OP_CHECKSIG)),
		body: func(sig []byte) []byte {
			return mustScript(NewScriptBuilder().AddData(sig).
				AddData(pubKey).AddOp(OP_CHECKSIG))
		},
		valid: true,
	}}

	for _, test := range tests {
		flags := may2026Flags
		tx := createSpendingTx(nil, invokeScript, 1000)
		sig := sign(tx, test.signed, test.schnorr, !test.legacy)
		pkScript := invokeScript
		if test.legacy {
			flags = ScriptVerifyDERSignatures
			pkScript = test.body(sig)
		} else {
			tx.TxIn[0].SignatureScript = mustScript(
				NewScriptBuilder().AddData(test.body(sig)))
		}

		utxos := NewUtxoCache()
		utxos.AddEntry(0, *wire.NewTxOut(1000, pkScript, wire.TokenData{}))
		vm, err := NewEngine(pkScript, tx, 0, flags, nil, nil, utxos, 1000)
		if err != nil {
			t.Fatalf("%s: NewEngine: %v", test.name, err)
		}
		err = vm.Execute()
		switch {
		case test.valid && err != nil:
			t.Errorf("%s: unexpected error: %v", test.name, err)
		case !test.valid && !IsErrorCode(err, test.wantErr):
			t.Errorf("%s: got error %v, want %v", test.name, err,
				test.wantErr)
		}
	}
}

// TestExecuteAllocs ensures Execute does not allocate per opcode when trace
// logging is off.  AllocsPerRun does not allow it to run in parallel.
func TestExecuteAllocs(t *testing.T) {
	pkScript := append(bytes.Repeat([]byte{OP_NOP}, 300), OP_1)
	tx := createSpendingTx(nil, pkScript, 0)
	flags := ScriptAllowMay2025 | ScriptAllowMay2026
	allocs := testing.AllocsPerRun(10, func() {
		vm, err := NewEngine(pkScript, tx, 0, flags, nil, nil, nil, 0)
		if err != nil {
			t.Fatalf("NewEngine: %v", err)
		}
		if err := vm.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if allocs > 100 {
		t.Fatalf("executing 300 opcodes allocates %v times", allocs)
	}
}

// TestExportedEncodingChecks ensures the exported signature and public key
// encoding checks apply the hash type, signature and key rules of the flags.
func TestExportedEncodingChecks(t *testing.T) {
	t.Parallel()

	der := hexToBytes("304402204e45e16932b8af514961a1d3a1a25fdf3f4f7732e9d624c6c61548ab5fb8cd41022018152" +
		"2ec8eca07de4860a4acdd12909d831cc56cbbac4622082221a8768d1d09")
	schnorr := make([]byte, 64)
	flags := StandardVerifyFlags | ScriptAllowCashTokens
	tests := []struct {
		name    string
		sig     []byte
		flags   ScriptFlags
		isValid bool
	}{
		{"empty", nil, flags, true},
		{"der all forkid", append(der, 0x41), flags, true},
		{"schnorr all forkid", append(schnorr, 0x41), flags, true},
		{"no forkid", append(der, 0x01), flags, false},
		{"undefined base type", append(der, 0x44), flags, false},
		{"utxos with tokens", append(der, 0x61), flags, true},
		{"utxos without tokens", append(der, 0x61), StandardVerifyFlags, false},
		{"utxos with anyonecanpay", append(der, 0xe1), flags, false},
		{"bad der", append(append([]byte{0x31}, der[1:]...), 0x41), flags, false},
	}
	for _, test := range tests {
		err := CheckTransactionSignatureEncoding(test.sig, test.flags)
		if (err == nil) != test.isValid {
			t.Errorf("%s: got %v, want valid %v", test.name, err, test.isValid)
		}
	}

	compressed := hexToBytes("02ce0b14fb842b1ba549fdd675c98075f12e9c510f8ef52bd021a9a1f4809d3b4d")
	if err := CheckPubKeyEncoding(compressed, flags); err != nil {
		t.Errorf("compressed key: %v", err)
	}
	if err := CheckPubKeyEncoding(compressed[:32], flags); err == nil {
		t.Error("short key accepted")
	}
}

// TestBCHNErrorOrder ensures a script that breaks several rules fails with the
// error BCHN reports, where txscript used to check another rule first or give
// several of BCHN's errors one code.
func TestBCHNErrorOrder(t *testing.T) {
	t.Parallel()

	may2025 := may2026TestFlags &^ ScriptAllowMay2026
	bits64 := may2025 &^ ScriptAllowMay2025
	noPushOnly := may2026TestFlags &^ ScriptVerifySigPushOnly
	der := "0x30 0x06 0x02 0x01 0x01 0x02 0x01 0x01"
	pubKey := "0x21 0x02ce0b14fb842b1ba549fdd675c98075f12e9c510f8ef52bd021a9a1f4809d3b4d"
	dataSig := "0x40 0x" + strings.Repeat("11", 64) + " 0 " + pubKey +
		" CHECKDATASIG DROP"
	redeem := mustParseShortForm("0")
	p2sh := "HASH160 0x14 0x" + hex.EncodeToString(bchutil.Hash160(redeem)) +
		" EQUAL"
	wrongP2SH := "HASH160 0x14 0x" + strings.Repeat("00", 20) + " EQUAL"
	wrongP2SH32 := "HASH256 0x20 0x" + strings.Repeat("00", 32) + " EQUAL"

	// An input script that is not push only, has more signature checks
	// than its size allows and ends with the redeem script.
	denseSig := "0x40 0x" + strings.Repeat("11", 64) + " 0 " + pubKey +
		strings.Repeat(" 3DUP CHECKDATASIG DROP", 6) +
		" DROP DROP DROP 0x01 0x00"
	tests := []struct {
		name      string
		sigScript string
		pkScript  string
		flags     ScriptFlags
		want      ErrorCode
	}{
		{"second operand read first", "0x09 0x010000000000000001 0x02 0x0100",
			"ADD", bits64, ErrNumberRange64Bit},
		{"deepest operand read first",
			"0x09 0x010000000000000001 0x02 0x0100 1", "WITHIN", bits64,
			ErrNumberRange64Bit},
		{"stack size before the operands", "0x02 0x0100 1", "WITHIN",
			bits64, ErrInvalidStackOperation},
		{"result out of the 64-bit range",
			"0x08 0xffffffffffffff7f", "1ADD", bits64, ErrNumberRange64Bit},
		{"lock time over 5 bytes", "0x06 0x010000000000",
			"CHECKLOCKTIMEVERIFY", may2026TestFlags, ErrNumberRange32Bit},
		{"until without a loop", "", "UNTIL", may2026TestFlags,
			ErrUnbalancedControlFlow},
		{"until on a conditional", "", "1 IF UNTIL", may2026TestFlags,
			ErrUnbalancedControlFlow},
		{"else on a loop in a conditional", "", "1 IF BEGIN ELSE",
			may2026TestFlags, ErrUnbalancedControlFlow},
		{"endif on a loop", "", "BEGIN ENDIF", may2026TestFlags,
			ErrUnbalancedConditional},
		{"end of script in a loop", "", "1 BEGIN", may2026TestFlags,
			ErrUnbalancedControlFlow},
		{"end of script in a conditional in a loop", "", "BEGIN 1 IF",
			may2026TestFlags, ErrUnbalancedConditional},
		{"conditional depth before May 2026",
			strings.Repeat("1 ", 101), strings.Repeat("IF ", 101), may2025,
			ErrConditionalStackDepth},
		{"invoke of a long identifier", "",
			"0x08 0x0102030405060708 INVOKE", may2026TestFlags,
			ErrInvalidFunctionIdentifier},
		{"define with one item", "", "0x08 0x0102030405060708 DEFINE",
			may2026TestFlags, ErrInvalidStackOperation},
		{"fork id where it is illegal", "0x09 " + der + " 0x41",
			pubKey + " CHECKSIG", may2026TestFlags &^ ScriptVerifyBip143SigHash,
			ErrIllegalForkID},
		{"missing fork id", "0x09 " + der + " 0x01", pubKey + " CHECKSIG",
			may2026TestFlags, ErrMustUseForkID},
		{"undefined base type before the fork id", "0x09 " + der + " 0x04",
			pubKey + " CHECKSIG", may2026TestFlags, ErrInvalidSigHashType},
		{"false result before the signature density", "",
			dataSig + " " + dataSig + " 0",
			bits64&^ScriptVerifyNullFail | ScriptVerifyInputSigChecks,
			ErrEvalFalse},
		{"clean stack before the signature density", "",
			dataSig + " " + dataSig + " 1 1",
			bits64&^ScriptVerifyNullFail | ScriptVerifyInputSigChecks,
			ErrCleanStack},
		{"pay to script hash scripts before push only",
			"1 DROP 0x01 0x00", wrongP2SH, noPushOnly, ErrEvalFalse},
		{"pay to script hash push only before the redeem script",
			"1 DROP 0x01 0x00", p2sh, noPushOnly, ErrNotPushOnly},
		{"pay to script hash 32 scripts before push only",
			"1 DROP 0x01 0x00", wrongP2SH32, noPushOnly, ErrEvalFalse},
		{"pay to script hash push only before the signature density",
			denseSig, p2sh,
			bits64&^(ScriptVerifyNullFail|ScriptVerifySigPushOnly) |
				ScriptVerifyInputSigChecks, ErrNotPushOnly},
		{"stack size before the roll position", "0x02 0x0100", "ROLL",
			may2026TestFlags, ErrInvalidStackOperation},
		{"stack size before the split position", "0x02 0x0100", "SPLIT",
			may2026TestFlags, ErrInvalidStackOperation},
		{"negative split position", "0x02 0x0102 -1", "SPLIT",
			may2026TestFlags, ErrInvalidSplitRange},
		{"stack size before the size", "0x02 0x0100", "NUM2BIN",
			may2026TestFlags, ErrInvalidStackOperation},
		{"bin2num out of the 64-bit range", "0x09 0x010000000000000001",
			"BIN2NUM", bits64, ErrNumberRange64Bit},
		{"shift result out of the big integer range", "1 0x03 0x803801",
			"LSHIFTNUM", may2026TestFlags, ErrNumberRangeBigInt},
		{"negative right shift", "1 -1", "RSHIFTNUM", may2026TestFlags,
			ErrInvalidBitShift},
		{"negative left binary shift", "0x02 0x0102 -1", "LSHIFTBIN",
			may2026TestFlags, ErrInvalidBitShift},
		{"negative right binary shift", "0x02 0x0102 -1", "RSHIFTBIN",
			may2026TestFlags, ErrInvalidBitShift},
		{"stack size before the shift", "-1", "LSHIFTNUM",
			may2026TestFlags, ErrInvalidStackOperation},
		{"stack size before the right shift", "-1", "RSHIFTNUM",
			may2026TestFlags, ErrInvalidStackOperation},
		{"stack size before the binary shift", "-1", "LSHIFTBIN",
			may2026TestFlags, ErrInvalidStackOperation},
		{"stack size before the right binary shift", "-1", "RSHIFTBIN",
			may2026TestFlags, ErrInvalidStackOperation},
		{"invoke past the control stack depth", strings.Repeat("1 ", 100),
			"0x01 0x51 0x01 0x11 DEFINE " + strings.Repeat("IF ", 100) +
				"0x01 0x11 INVOKE", may2026TestFlags, ErrControlStackDepth},
		{"key count over 8 bytes", "",
			"0x09 0x010000000000000001 CHECKMULTISIG", may2026TestFlags,
			ErrNumberRange64Bit},
		{"signature count over 8 bytes", "",
			"0x09 0x010000000000000001 0 CHECKMULTISIG", may2026TestFlags,
			ErrNumberRange64Bit},
		{"input index over 8 bytes", "",
			"0x09 0x010000000000000001 UTXOVALUE", may2026TestFlags,
			ErrNumberRange64Bit},
		{"output index over 8 bytes", "",
			"0x09 0x010000000000000001 OUTPUTVALUE", may2026TestFlags,
			ErrNumberRange64Bit},
		{"negative token output index", "", "-1 OUTPUTTOKENCATEGORY",
			may2026TestFlags, ErrInvalidTxOutputIndex},
		{"negative token output index for the commitment", "",
			"-1 OUTPUTTOKENCOMMITMENT", may2026TestFlags,
			ErrInvalidTxOutputIndex},
		{"negative token output index for the amount", "",
			"-1 OUTPUTTOKENAMOUNT", may2026TestFlags,
			ErrInvalidTxOutputIndex},
		{"token output index", "", "5 OUTPUTTOKENCATEGORY",
			may2026TestFlags, ErrInvalidTxOutputIndex},
		{"token input index", "", "5 UTXOTOKENCATEGORY", may2026TestFlags,
			ErrInvalidTxInputIndex},
		{"token input index for the commitment", "",
			"5 UTXOTOKENCOMMITMENT", may2026TestFlags,
			ErrInvalidTxInputIndex},
		{"token input index for the amount", "", "5 UTXOTOKENAMOUNT",
			may2026TestFlags, ErrInvalidTxInputIndex},
	}
	for _, test := range tests {
		sigScript := mustParseShortForm(test.sigScript)
		pkScript := mustParseShortForm(test.pkScript)
		_, err := runFunctionsTestScript(t, sigScript,
			hex.EncodeToString(pkScript), test.flags)
		if !IsErrorCode(err, test.want) {
			t.Errorf("%s: got error %v, want %v", test.name, err,
				test.want)
		}
	}

	// A script that passes leaves its stack without the top item, as
	// before the checks moved.
	vm, err := runFunctionsTestScript(t, nil, "5251", ScriptBip16)
	if err != nil {
		t.Fatalf("2 1: %v", err)
	}
	if got := vm.GetStack(); !reflect.DeepEqual(got, [][]byte{{2}}) {
		t.Errorf("2 1: stack %x after the checks, want [02]", got)
	}

	// The stack size is checked before the operation cost, as BCHN does.
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{})
	tx.AddTxOut(wire.NewTxOut(0, []byte{OP_1}, wire.TokenData{}))
	vm, err = NewEngine([]byte{OP_1}, tx, 0, may2026TestFlags, nil, nil,
		nil, 0)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	for range MaxStackSize {
		vm.dstack.PushByteArray(nil)
	}
	vm.metrics.AddOPCost(int(vm.metrics.GetMaxOpCostLimit()))
	if _, err := vm.Step(); !IsErrorCode(err, ErrStackOverflow) {
		t.Errorf("stack size and operation cost: got error %v, want %v",
			err, ErrStackOverflow)
	}
}

// TestLegacyIntReads ensures the stack positions, sizes, counts and indexes
// opcodes take are reported outside the 64-bit range when they are longer
// than 8 bytes, as BCHN reports them, without failing an item that passed
// before: an encoding of a value in the 64-bit range that is not minimal.
func TestLegacyIntReads(t *testing.T) {
	t.Parallel()

	may2025 := may2026TestFlags &^ ScriptAllowMay2026
	notMinimal := may2025 &^ ScriptVerifyMinimalData
	before2022 := may2025 &^ (ScriptAllowMay2025 | ScriptVerify64BitIntegers)
	tests := []struct {
		name      string
		sigScript string
		pkScript  string
		flags     ScriptFlags
		want      ErrorCode // ErrInternal when the script passes
	}{
		{"zero over 8 bytes without minimal data",
			"1 0x09 0x000000000000000000", "PICK NIP", notMinimal,
			ErrInternal},
		{"zero over 8 bytes", "1 0x09 0x000000000000000000", "PICK NIP",
			may2025, ErrNumberRange64Bit},
		{"position over 8 bytes", "1 0x09 0x010000000000000001",
			"PICK NIP", notMinimal, ErrNumberRange64Bit},
		{"negative position over 8 bytes", "1 0x09 0x010000000000000081",
			"PICK NIP", notMinimal, ErrNumberRange64Bit},
		{"position over 4 bytes before 64-bit integers", "1 0x05 0x0100000001",
			"PICK NIP", before2022, ErrNumberRange32Bit},
		{"position over 8 bytes before 64-bit integers",
			"1 0x09 0x010000000000000001", "PICK NIP", before2022,
			ErrNumberRange32Bit},
	}
	for _, test := range tests {
		_, err := runFunctionsTestScript(t,
			mustParseShortForm(test.sigScript),
			hex.EncodeToString(mustParseShortForm(test.pkScript)),
			test.flags)
		if test.want == ErrInternal {
			if err != nil {
				t.Errorf("%s: got error %v, want none", test.name, err)
			}
			continue
		}
		if !IsErrorCode(err, test.want) {
			t.Errorf("%s: got error %v, want %v", test.name, err,
				test.want)
		}
	}
}
