// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"encoding/hex"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// may2026TestFlags are the consensus flags of a block after the May 2026
// upgrade.
const may2026TestFlags = ScriptBip16 | ScriptVerifyDERSignatures |
	ScriptVerifyCheckLockTimeVerify | ScriptVerifyCheckSequenceVerify |
	ScriptVerifyStrictEncoding | ScriptVerifyBip143SigHash |
	ScriptVerifyLowS | ScriptVerifyNullFail | ScriptVerifySigPushOnly |
	ScriptVerifyCleanStack | ScriptVerifyCheckDataSig | ScriptVerifySchnorr |
	ScriptVerifyAllowSegwitRecovery | ScriptVerifyMinimalData |
	ScriptVerifySchnorrMultisig | ScriptReportSigChecks |
	ScriptVerifyReverseBytes | ScriptVerify64BitIntegers |
	ScriptVerifyNativeIntrospection | ScriptAllowCashTokens |
	ScriptAllowMay2025 | ScriptAllowMay2026

// runFunctionsTestScript executes the hex encoded locking script, spent by
// the passed unlocking script, with the passed flags.
func runFunctionsTestScript(t testing.TB, sigScript []byte, pkScriptHex string,
	flags ScriptFlags) (*Engine, error) {

	t.Helper()

	pkScript, err := hex.DecodeString(pkScriptHex)
	if err != nil {
		t.Fatalf("decode %q: %v", pkScriptHex, err)
	}
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}, Index: 1},
		SignatureScript:  sigScript,
		Sequence:         0xffffffff,
	})
	tx.AddTxOut(wire.NewTxOut(1000, []byte{OP_1}, wire.TokenData{}))
	utxos := NewUtxoCache()
	utxos.AddEntry(0, *wire.NewTxOut(2000, pkScript, wire.TokenData{}))
	vm, err := NewEngine(pkScript, tx, 0, flags, nil, NewTxSigHashes(tx),
		utxos, 2000)
	if err != nil {
		return nil, err
	}
	return vm, vm.Execute()
}

// TestInvokeUnbalancedControlStack ensures a frame that is restored when an
// invoked function returns is checked for open conditionals and loops when it
// has reached its end, too.
func TestInvokeUnbalancedControlStack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pkScript string
	}{{
		// <0x51> <1> OP_DEFINE OP_1 OP_IF <1> OP_INVOKE
		name:     "open if before a trailing invoke",
		pkScript: "0151" + "51" + "89" + "51" + "63" + "51" + "8a",
	}, {
		// <0x51> <1> OP_DEFINE OP_BEGIN <1> OP_INVOKE
		name:     "open begin before a trailing invoke",
		pkScript: "0151" + "51" + "89" + "65" + "51" + "8a",
	}, {
		// <0x51> <1> OP_DEFINE <OP_1 OP_IF <1> OP_INVOKE> <2> OP_DEFINE
		// <2> OP_INVOKE
		name:     "open if in a function before a trailing invoke",
		pkScript: "0151" + "51" + "89" + "04" + "5163518a" + "52" + "89" + "52" + "8a",
	}, {
		// <0x51> <1> OP_DEFINE OP_1 OP_IF <1> OP_INVOKE OP_NOP
		name:     "open if before an invoke and a nop",
		pkScript: "0151" + "51" + "89" + "51" + "63" + "51" + "8a" + "61",
	}}

	policyFlags := StandardVerifyFlags | ScriptAllowCashTokens |
		ScriptAllowMay2025 | ScriptAllowMay2025StandardOnly |
		ScriptAllowMay2026
	for _, test := range tests {
		for _, flags := range []ScriptFlags{may2026TestFlags, policyFlags} {
			_, err := runFunctionsTestScript(t, nil, test.pkScript, flags)
			if !IsErrorCode(err, ErrUnbalancedConditional) {
				t.Errorf("%s: flags %x: got error %v, want %v",
					test.name, uint64(flags), err,
					ErrUnbalancedConditional)
			}
		}
	}

	// Closing the conditional after the invoke makes the script valid.
	// <0x51> <1> OP_DEFINE OP_1 OP_IF <1> OP_INVOKE OP_ENDIF
	_, err := runFunctionsTestScript(t, nil, "015151895163518a68",
		may2026TestFlags)
	if err != nil {
		t.Errorf("balanced script: unexpected error %v", err)
	}
}

// TestInvokeParsesBodyOnce ensures a function body is parsed only once, no
// matter how often the function is invoked.
func TestInvokeParsesBodyOnce(t *testing.T) {
	t.Parallel()

	// <OP_1 OP_DROP> <1> OP_DEFINE 3 OP_BEGIN <1> OP_INVOKE OP_1SUB OP_DUP
	// OP_NOT OP_UNTIL OP_DROP OP_1
	pkScript, err := hex.DecodeString("025175" + "51" + "89" + "53" + "65" +
		"51" + "8a" + "8c" + "76" + "91" + "66" + "75" + "51")
	if err != nil {
		t.Fatal(err)
	}
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}, Index: 1},
		Sequence:         0xffffffff,
	})
	tx.AddTxOut(wire.NewTxOut(1000, []byte{OP_1}, wire.TokenData{}))
	utxos := NewUtxoCache()
	utxos.AddEntry(0, *wire.NewTxOut(2000, pkScript, wire.TokenData{}))
	vm, err := NewEngine(pkScript, tx, 0, may2026TestFlags, nil,
		NewTxSigHashes(tx), utxos, 2000)
	if err != nil {
		t.Fatal(err)
	}

	var bodies []*parsedOpcode
	for done := false; !done; {
		done, err = vm.Step()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if !done && len(vm.frameStack) == 1 && vm.scriptOff == 0 {
			bodies = append(bodies, &vm.scripts[vm.scriptIdx][0])
		}
	}
	if err := vm.CheckErrorCondition(true); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(bodies) != 3 {
		t.Fatalf("got %d invocations, want 3", len(bodies))
	}
	for i := 1; i < len(bodies); i++ {
		if bodies[i] != bodies[0] {
			t.Fatalf("invocation %d parsed the body again", i)
		}
	}
}

// TestInvokeMalformedBody ensures a malformed function body can be defined
// and only fails the script when the function is invoked.
func TestInvokeMalformedBody(t *testing.T) {
	t.Parallel()

	// <0x4c> <1> OP_DEFINE OP_1
	if _, err := runFunctionsTestScript(t, nil, "014c"+"51"+"89"+"51",
		may2026TestFlags); err != nil {
		t.Errorf("defined only: unexpected error %v", err)
	}

	// <0x4c> <1> OP_DEFINE <1> OP_INVOKE OP_1
	_, err := runFunctionsTestScript(t, nil, "014c"+"51"+"89"+"51"+"8a"+"51",
		may2026TestFlags)
	if !IsErrorCode(err, ErrMalformedPush) {
		t.Errorf("invoked: got error %v, want %v", err, ErrMalformedPush)
	}
}

// TestFunctionTablePerScript ensures functions defined by the unlocking script
// are gone in the locking script, and that a script can still define the
// same identifier again.
func TestFunctionTablePerScript(t *testing.T) {
	t.Parallel()

	// Without SIGPUSHONLY the unlocking script may define functions.
	flags := may2026TestFlags &^ (ScriptVerifySigPushOnly | ScriptVerifyCleanStack)

	// <0x51> <1> OP_DEFINE
	sigScript := []byte{0x01, 0x51, OP_1, OP_DEFINE}

	// <1> OP_INVOKE
	_, err := runFunctionsTestScript(t, sigScript, "518a", flags)
	if !IsErrorCode(err, ErrUndefinedFunction) {
		t.Fatalf("invoke in the locking script: got %v, want %v", err,
			ErrUndefinedFunction)
	}

	// <0x51> <1> OP_DEFINE <1> OP_INVOKE
	if _, err := runFunctionsTestScript(t, sigScript, "01515189518a", flags); err != nil {
		t.Fatalf("redefine in the locking script: %v", err)
	}
}
