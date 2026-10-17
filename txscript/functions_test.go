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
