// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
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

	for i := 0; i < len(pkScript)-1; i++ {
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
