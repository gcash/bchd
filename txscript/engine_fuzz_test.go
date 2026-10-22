// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// maxEngineFuzzSeeds caps the VMB vectors added to the seed corpus, so the
// seeds run quickly without -fuzz.  Fewer are taken with -short.
const maxEngineFuzzSeeds = 3000

// engineFuzzPolicyFlags are the relay flags the fuzzer adds to the
// consensus flags.  Each only makes the rules stricter.
var engineFuzzPolicyFlags = [...]ScriptFlags{
	ScriptDiscourageUpgradableNops,
	ScriptVerifyInputSigChecks,
	ScriptAllowMay2025StandardOnly,
}

// engineFuzzStrictBits are the bits of engineFuzzFlags that make the rules
// stricter: the policy flags and the one that turns off segwit recovery.
const engineFuzzStrictBits = 1<<(len(engineFuzzPolicyFlags)+1) - 1

// engineFuzzFlags returns the flags picked by bits.  Bits 0 to 2 add the
// policy flags and bit 3 drops the segwit recovery exemption, as relay does.
// The higher bits pick the consensus flags of the May 2026, May 2025 or May
// 2023 upgrade.
func engineFuzzFlags(bits uint8) ScriptFlags {
	flags := ScriptFlags(may2026TestFlags)
	switch (bits >> 4) % 3 {
	case 1:
		flags &^= ScriptAllowMay2026
	case 2:
		flags &^= ScriptAllowMay2025 | ScriptAllowMay2026
	}
	for i, f := range engineFuzzPolicyFlags {
		if bits&(1<<i) != 0 {
			flags |= f
		}
	}
	if bits&(1<<len(engineFuzzPolicyFlags)) != 0 {
		flags &^= ScriptVerifyAllowSegwitRecovery
	}
	return flags
}

// engineFuzzTx returns a transaction whose first input has the passed
// signature script, and the outputs it spends.  The second input carries
// tokens, which the first output sends on, so the introspection and token
// opcodes have something to look at.
func engineFuzzTx(sigScript, rawPkScript []byte, amount int64, sequence,
	lockTime uint32) (*wire.MsgTx, *UtxoCache) {

	p2pkh := func(b byte) []byte {
		return catScripts([]byte{OP_DUP, OP_HASH160, OP_DATA_20},
			bytes.Repeat([]byte{b}, 20), []byte{OP_EQUALVERIFY, OP_CHECKSIG})
	}
	var category [32]byte
	copy(category[:], bytes.Repeat([]byte{0x55}, 32))

	tx := wire.NewMsgTx(2)
	tx.LockTime = lockTime
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{0x11}, Index: 0},
		SignatureScript:  sigScript,
		Sequence:         sequence,
	})
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{0x22}, Index: 1},
		SignatureScript: catScripts(pushData(0, bytes.Repeat([]byte{0x30}, 65)),
			pushData(0, bytes.Repeat([]byte{0x02}, 33))),
		Sequence: wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: 5000, PkScript: p2pkh(0x33),
		TokenData: wire.TokenData{CategoryID: category, Amount: 600,
			Commitment: []byte{1, 2, 3},
			BitField:   wire.HAS_AMOUNT | wire.HAS_NFT | wire.HAS_COMMITMENT_LENGTH | wire.MUTABLE}})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: []byte{OP_RETURN, OP_DATA_1, 7}})
	tx.AddTxOut(&wire.TxOut{Value: 2000, PkScript: catScripts([]byte{OP_HASH256, OP_DATA_32},
		bytes.Repeat([]byte{0x44}, 32), []byte{OP_EQUAL})})

	// The fuzzed output is split the way outputs are read from the wire.
	tokenData, pkScript := wire.SplitPkScript(rawPkScript)
	cache := NewUtxoCache()
	cache.AddEntry(0, wire.TxOut{Value: amount, PkScript: pkScript, TokenData: tokenData})
	cache.AddEntry(1, wire.TxOut{Value: 10000, PkScript: p2pkh(0x66),
		TokenData: wire.TokenData{CategoryID: category, Amount: 1000,
			Commitment: []byte{9},
			BitField:   wire.HAS_AMOUNT | wire.HAS_NFT | wire.HAS_COMMITMENT_LENGTH | wire.MINTING}})
	return tx, cache
}

// runFuzzEngine runs the first input of tx with Step, failing if it takes
// more steps than its operation cost budget allows.
func runFuzzEngine(t *testing.T, tx *wire.MsgTx, cache *UtxoCache,
	flags ScriptFlags) error {

	t.Helper()
	utxo, _ := cache.GetEntry(0)
	vm, err := NewEngine(utxo.PkScript, tx, 0, flags, nil, nil, cache, utxo.Value)
	if err != nil {
		return err
	}

	// Each step costs OpcodeBaseCost.  Without the May 2025 limits there
	// are no loops, so each opcode runs at most once.
	maxSteps := 3 * MaxScriptSize
	if flags.HasFlag(ScriptAllowMay2025) {
		maxSteps = int(vm.GetMetrics().GetMaxOpCostLimit()/OpcodeBaseCost) + 1
	}
	for steps := 1; ; steps++ {
		done, err := vm.Step()
		if err != nil {
			return err
		}
		if done {
			break
		}
		if steps >= maxSteps {
			t.Fatalf("no end after %d steps", steps)
		}
	}
	return vm.CheckErrorCondition(true)
}

// executeFuzzEngine runs the first input of tx with Execute.
func executeFuzzEngine(tx *wire.MsgTx, cache *UtxoCache, flags ScriptFlags) error {
	utxo, _ := cache.GetEntry(0)
	vm, err := NewEngine(utxo.PkScript, tx, 0, flags, nil, nil, cache, utxo.Value)
	if err != nil {
		return err
	}
	return vm.Execute()
}

// addEngineVMBSeeds adds the tested inputs of the May 2026 VMB vectors,
// leaving out the large bigint and benchmark files and taking an even
// spread of at most maxEngineFuzzSeeds.
func addEngineVMBSeeds(f *testing.F) {
	files, err := filepath.Glob("data/vmb_tests/bch_2026_*/*.vmb_tests.json")
	if err != nil {
		f.Fatal(err)
	}
	type seed struct {
		sigScript, pkScript []byte
		amount              int64
		sequence, lockTime  uint32
		bits                uint8
	}
	var seeds []seed
	for _, name := range files {
		base := filepath.Base(name)
		if strings.HasPrefix(base, "core.bigint.") ||
			strings.Contains(base, "benchmarks") {
			continue
		}
		file, err := os.ReadFile(name)
		if err != nil {
			f.Fatal(err)
		}
		var tests [][]interface{}
		if err := json.Unmarshal(file, &tests); err != nil {
			f.Fatalf("%s: %v", name, err)
		}

		// Standard vectors run with the relay flags.
		var bits uint8
		if strings.Contains(name, "_standard") {
			bits = engineFuzzStrictBits
		}
		for _, test := range tests {
			if len(test) < 6 {
				continue
			}
			txHex, ok1 := test[4].(string)
			utxoHex, ok2 := test[5].(string)
			if !ok1 || !ok2 {
				continue
			}
			txBytes, err1 := hex.DecodeString(txHex)
			utxoBytes, err2 := hex.DecodeString(utxoHex)
			if err1 != nil || err2 != nil {
				continue
			}
			var tx wire.MsgTx
			if err := tx.BchDecode(bytes.NewReader(txBytes), 0, 0); err != nil {
				continue
			}
			idx := 0
			if len(test) > 6 {
				if i, ok := test[6].(float64); ok {
					idx = int(i)
				}
			}
			values, scripts := readRawUtxoScripts(utxoBytes)
			if idx >= len(tx.TxIn) || idx >= len(scripts) {
				continue
			}
			in := tx.TxIn[idx]
			seeds = append(seeds, seed{in.SignatureScript, scripts[idx],
				values[idx], in.Sequence, tx.LockTime, bits})
		}
	}
	if len(seeds) == 0 {
		f.Fatal("no VMB test vectors found")
	}
	maxSeeds := maxEngineFuzzSeeds
	if testing.Short() {
		maxSeeds /= 10
	}
	step := (len(seeds) + maxSeeds - 1) / maxSeeds
	for i := 0; i < len(seeds); i += step {
		s := seeds[i]
		f.Add(s.sigScript, s.pkScript, s.amount, s.sequence, s.lockTime,
			s.bits, uint8(engineFuzzStrictBits))
	}
}

// FuzzScriptEngine ensures the engine does not panic or run past its
// operation cost budget, that Step and Execute agree, and that dropping
// relay flags from a passing input or adding them to a failing one keeps
// the result.
func FuzzScriptEngine(f *testing.F) {
	addEngineVMBSeeds(f)

	c := &analysisCorpus{seen: make(map[string]struct{})}
	c.addVectors(f)
	for i, pair := range c.pairs {
		f.Add(pair.sigScript, pair.pkScript, int64(i%3)*1000,
			uint32(wire.MaxTxInSequenceNum), uint32(0), uint8(i),
			uint8(engineFuzzStrictBits))
	}

	f.Fuzz(func(t *testing.T, sigScript, pkScript []byte, amount int64,
		sequence, lockTime uint32, bits, fuzzedBits uint8) {

		if amount < 0 || amount > MaxSatoshi {
			return
		}
		tx, cache := engineFuzzTx(sigScript, pkScript, amount, sequence, lockTime)
		flags := engineFuzzFlags(bits)

		// Inputs whose tokens could not be spent never reach the engine.
		valid, err := wire.RunCashTokensValidityAlgorithm(cache, tx,
			flags.HasFlag(ScriptAllowMay2026))
		if err != nil || !valid {
			return
		}

		err = runFuzzEngine(t, tx, cache, flags)
		if err2 := executeFuzzEngine(tx, cache, flags); errString(err2) != errString(err) {
			t.Fatalf("Step gave %v, Execute gave %v", err, err2)
		}

		other := bits | fuzzedBits&engineFuzzStrictBits
		if err == nil {
			other = bits &^ (fuzzedBits & engineFuzzStrictBits)
		}
		if other == bits {
			return
		}
		err2 := runFuzzEngine(t, tx, cache, engineFuzzFlags(other))
		if (err == nil) != (err2 == nil) {
			t.Fatalf("flags %v gave %v, flags %v gave %v", flags, err,
				engineFuzzFlags(other), err2)
		}
	})
}
