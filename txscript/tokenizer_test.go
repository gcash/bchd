// Copyright (c) 2019 The Decred developers
// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// repeatForm matches 0x<hex>{n}, which mustParseRepeatForm expands to the
// hex bytes repeated n times.
var repeatForm = regexp.MustCompile(`0x([0-9a-fA-F]+)\{(\d+)\}`)

// mustParseRepeatForm is mustParseShortForm with 0x<hex>{n} repetitions.
func mustParseRepeatForm(script string) []byte {
	script = repeatForm.ReplaceAllStringFunc(script, func(m string) string {
		sub := repeatForm.FindStringSubmatch(m)
		n, _ := strconv.Atoi(sub[2])
		return "0x" + strings.Repeat(sub[1], n)
	})
	return mustParseShortForm(script)
}

// TestScriptTokenizer ensures a wide variety of behavior provided by the script
// tokenizer performs as expected.
func TestScriptTokenizer(t *testing.T) {
	t.Parallel()

	type expectedResult struct {
		op    byte   // expected parsed opcode
		data  []byte // expected parsed data
		index int    // expected index into raw script after parsing token
	}

	type tokenizerTest struct {
		name     string           // test description
		script   []byte           // the script to tokenize
		expected []expectedResult // the expected info after parsing each token
		finalIdx int              // the expected final byte index
		err      error            // expected error
	}

	// Add both positive and negative tests for OP_DATA_1 through OP_DATA_75.
	const numTestsHint = 100 // Make prealloc linter happy.
	tests := make([]tokenizerTest, 0, numTestsHint)
	for op := byte(OP_DATA_1); op <= OP_DATA_75; op++ {
		data := bytes.Repeat([]byte{0x01}, int(op))
		tests = append(tests, tokenizerTest{
			name:     fmt.Sprintf("OP_DATA_%d", op),
			script:   append([]byte{op}, data...),
			expected: []expectedResult{{op, data, 1 + int(op)}},
			finalIdx: 1 + int(op),
			err:      nil,
		})

		// Create test that provides one less byte than the data push requires.
		tests = append(tests, tokenizerTest{
			name:     fmt.Sprintf("short OP_DATA_%d", op),
			script:   append([]byte{op}, data[1:]...),
			expected: nil,
			finalIdx: 0,
			err:      scriptError(ErrMalformedPush, ""),
		})
	}

	// Add both positive and negative tests for OP_PUSHDATA{1,2,4}.
	data := mustParseRepeatForm("0x01{76}")
	tests = append(tests, []tokenizerTest{{
		name:     "OP_PUSHDATA1",
		script:   mustParseRepeatForm("OP_PUSHDATA1 0x4c 0x01{76}"),
		expected: []expectedResult{{OP_PUSHDATA1, data, 2 + len(data)}},
		finalIdx: 2 + len(data),
		err:      nil,
	}, {
		name:     "OP_PUSHDATA1 no data length",
		script:   mustParseRepeatForm("OP_PUSHDATA1"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA1 short data by 1 byte",
		script:   mustParseRepeatForm("OP_PUSHDATA1 0x4c 0x01{75}"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA1 zero length",
		script:   mustParseRepeatForm("OP_PUSHDATA1 0x00"),
		expected: []expectedResult{{OP_PUSHDATA1, []byte{}, 2}},
		finalIdx: 2,
		err:      nil,
	}, {
		name:     "OP_PUSHDATA2",
		script:   mustParseRepeatForm("OP_PUSHDATA2 0x4c00 0x01{76}"),
		expected: []expectedResult{{OP_PUSHDATA2, data, 3 + len(data)}},
		finalIdx: 3 + len(data),
		err:      nil,
	}, {
		name:     "OP_PUSHDATA2 no data length",
		script:   mustParseRepeatForm("OP_PUSHDATA2"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA2 short data length",
		script:   mustParseRepeatForm("OP_PUSHDATA2 0x4c"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA2 short data by 1 byte",
		script:   mustParseRepeatForm("OP_PUSHDATA2 0x4c00 0x01{75}"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA4",
		script:   mustParseRepeatForm("OP_PUSHDATA4 0x4c000000 0x01{76}"),
		expected: []expectedResult{{OP_PUSHDATA4, data, 5 + len(data)}},
		finalIdx: 5 + len(data),
		err:      nil,
	}, {
		name:     "OP_PUSHDATA4 no data length",
		script:   mustParseRepeatForm("OP_PUSHDATA4"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA4 short data length",
		script:   mustParseRepeatForm("OP_PUSHDATA4 0x4c0000"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA4 short data by 1 byte",
		script:   mustParseRepeatForm("OP_PUSHDATA4 0x4c000000 0x01{75}"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:     "OP_PUSHDATA4 max length",
		script:   mustParseRepeatForm("OP_PUSHDATA4 0xffffffff 0x01"),
		expected: nil,
		finalIdx: 0,
		err:      scriptError(ErrMalformedPush, ""),
	}}...)

	// Add tests for OP_0, and OP_1 through OP_16 (small integers/true/false).
	opcodes := []byte{OP_0}
	for op := byte(OP_1); op <= OP_16; op++ {
		opcodes = append(opcodes, op)
	}
	for _, op := range opcodes {
		tests = append(tests, tokenizerTest{
			name:     opcodeArray[op].name,
			script:   []byte{op},
			expected: []expectedResult{{op, nil, 1}},
			finalIdx: 1,
			err:      nil,
		})
	}

	// Add various positive and negative tests for multi-opcode scripts.
	tests = append(tests, []tokenizerTest{{
		name:     "empty script",
		script:   nil,
		expected: nil,
		finalIdx: 0,
		err:      nil,
	}, {
		name:   "pay-to-pubkey-hash",
		script: mustParseRepeatForm("DUP HASH160 DATA_20 0x01{20} EQUALVERIFY CHECKSIG"),
		expected: []expectedResult{
			{OP_DUP, nil, 1}, {OP_HASH160, nil, 2},
			{OP_DATA_20, mustParseRepeatForm("0x01{20}"), 23},
			{OP_EQUALVERIFY, nil, 24}, {OP_CHECKSIG, nil, 25},
		},
		finalIdx: 25,
		err:      nil,
	}, {
		name:   "almost pay-to-pubkey-hash (short data)",
		script: mustParseRepeatForm("DUP HASH160 DATA_20 0x01{17} EQUALVERIFY CHECKSIG"),
		expected: []expectedResult{
			{OP_DUP, nil, 1}, {OP_HASH160, nil, 2},
		},
		finalIdx: 2,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:   "almost pay-to-pubkey-hash (overlapped data)",
		script: mustParseRepeatForm("DUP HASH160 DATA_20 0x01{19} EQUALVERIFY CHECKSIG"),
		expected: []expectedResult{
			{OP_DUP, nil, 1}, {OP_HASH160, nil, 2},
			{OP_DATA_20, mustParseRepeatForm("0x01{19} EQUALVERIFY"), 23},
			{OP_CHECKSIG, nil, 24},
		},
		finalIdx: 24,
		err:      nil,
	}, {
		name:   "pay-to-script-hash",
		script: mustParseRepeatForm("HASH160 DATA_20 0x01{20} EQUAL"),
		expected: []expectedResult{
			{OP_HASH160, nil, 1},
			{OP_DATA_20, mustParseRepeatForm("0x01{20}"), 22},
			{OP_EQUAL, nil, 23},
		},
		finalIdx: 23,
		err:      nil,
	}, {
		name:   "almost pay-to-script-hash (short data)",
		script: mustParseRepeatForm("HASH160 DATA_20 0x01{18} EQUAL"),
		expected: []expectedResult{
			{OP_HASH160, nil, 1},
		},
		finalIdx: 1,
		err:      scriptError(ErrMalformedPush, ""),
	}, {
		name:   "almost pay-to-script-hash (overlapped data)",
		script: mustParseRepeatForm("HASH160 DATA_20 0x01{19} EQUAL"),
		expected: []expectedResult{
			{OP_HASH160, nil, 1},
			{OP_DATA_20, mustParseRepeatForm("0x01{19} EQUAL"), 22},
		},
		finalIdx: 22,
		err:      nil,
	}, {
		name:   "pay-to-script-hash-32",
		script: mustParseRepeatForm("HASH256 DATA_32 0x01{32} EQUAL"),
		expected: []expectedResult{
			{OP_HASH256, nil, 1},
			{OP_DATA_32, mustParseRepeatForm("0x01{32}"), 34},
			{OP_EQUAL, nil, 35},
		},
		finalIdx: 35,
		err:      nil,
	}, {
		name:   "token prefix byte",
		script: mustParseRepeatForm("0xef HASH160 DATA_20 0x01{20} EQUAL"),
		expected: []expectedResult{
			{SPECIAL_TOKEN_PREFIX, nil, 1}, {OP_HASH160, nil, 2},
			{OP_DATA_20, mustParseRepeatForm("0x01{20}"), 23},
			{OP_EQUAL, nil, 24},
		},
		finalIdx: 24,
		err:      nil,
	}}...)

	for _, test := range tests {
		tokenizer := MakeScriptTokenizer(test.script)
		var opcodeNum int
		for tokenizer.Next() {
			// Ensure Next never returns true when there is an error set.
			if err := tokenizer.Err(); err != nil {
				t.Fatalf("%q: Next returned true when tokenizer has err: %v",
					test.name, err)
			}

			// Ensure the test data expects a token to be parsed.
			op := tokenizer.Opcode()
			data := tokenizer.Data()
			if opcodeNum >= len(test.expected) {
				t.Fatalf("%q: unexpected token '%d' (data: '%x')", test.name,
					op, data)
			}
			expected := &test.expected[opcodeNum]

			// Ensure the opcode and data are the expected values.
			if op != expected.op {
				t.Fatalf("%q: unexpected opcode -- got %v, want %v", test.name,
					op, expected.op)
			}
			if !bytes.Equal(data, expected.data) {
				t.Fatalf("%q: unexpected data -- got %x, want %x", test.name,
					data, expected.data)
			}
			if (data == nil) != (expected.data == nil) {
				t.Fatalf("%q: unexpected data nil-ness -- got %v, want %v",
					test.name, data == nil, expected.data == nil)
			}

			tokenizerIdx := tokenizer.ByteIndex()
			if tokenizerIdx != expected.index {
				t.Fatalf("%q: unexpected byte index -- got %d, want %d",
					test.name, tokenizerIdx, expected.index)
			}

			opcodeNum++
		}

		// Ensure the tokenizer claims it is done.  This should be the case
		// regardless of whether or not there was a parse error.
		if !tokenizer.Done() {
			t.Fatalf("%q: tokenizer claims it is not done", test.name)
		}

		// Ensure Next keeps returning false once done.
		if tokenizer.Next() {
			t.Fatalf("%q: Next returned true after done", test.name)
		}

		// Ensure the error is as expected.
		if test.err == nil && tokenizer.Err() != nil {
			t.Fatalf("%q: unexpected tokenizer err -- got %v, want nil",
				test.name, tokenizer.Err())
		} else if test.err != nil {
			wantCode := test.err.(Error).ErrorCode
			if !IsErrorCode(tokenizer.Err(), wantCode) {
				t.Fatalf("%q: unexpected tokenizer err -- got %v, want %v",
					test.name, tokenizer.Err(), wantCode)
			}
		}

		// Ensure the final index is the expected value.
		tokenizerIdx := tokenizer.ByteIndex()
		if tokenizerIdx != test.finalIdx {
			t.Fatalf("%q: unexpected final byte index -- got %d, want %d",
				test.name, tokenizerIdx, test.finalIdx)
		}

		// Ensure the full script is returned.
		if !bytes.Equal(tokenizer.Script(), test.script) {
			t.Fatalf("%q: unexpected script -- got %x, want %x", test.name,
				tokenizer.Script(), test.script)
		}
	}
}

// TestScriptTokenizerOpcodeLengths ensures no opcode has a length
// the tokenizer does not know how to parse.
func TestScriptTokenizerOpcodeLengths(t *testing.T) {
	t.Parallel()

	for i := range opcodeArray {
		switch l := opcodeArray[i].length; {
		case l >= 1, l == -1, l == -2, l == -4:
		default:
			t.Fatalf("opcode %#x has unexpected length %d", i, l)
		}
	}
}

// checkTokenizerMatchesParse ensures the tokenizer yields the same opcodes,
// data and error as parseScript for the script, and that parseScript
// matches parseScriptTemplate with a result no larger than needed.
func checkTokenizerMatchesParse(t testing.TB, script []byte) {
	pops, parseErr := parseScript(script)
	ref, refErr := parseScriptTemplate(script, &opcodeArray)
	if len(pops) != len(ref) || cap(pops) > max(len(script), len(pops)) ||
		(len(pops) < maxCountedOpcodes && cap(pops) != len(pops)) {

		t.Fatalf("%x: parseScript gave %d opcodes (cap %d), want %d",
			script, len(pops), cap(pops), len(ref))
	}
	for i := range ref {
		if pops[i].opcode != ref[i].opcode ||
			!sameSlice(pops[i].data, ref[i].data) {

			t.Fatalf("%x: opcode %d differs from parseScriptTemplate",
				script, i)
		}
	}
	if errString(parseErr) != errString(refErr) {
		t.Fatalf("%x: parseScript error %v, parseScriptTemplate error %v",
			script, parseErr, refErr)
	}
	tok := MakeScriptTokenizer(script)
	i := 0
	for tok.Next() {
		if i >= len(pops) {
			t.Fatalf("%x: tokenizer gave more opcodes than parseScript", script)
		}
		pop := &pops[i]
		if tok.op != pop.opcode || !bytes.Equal(tok.Data(), pop.data) ||
			(tok.Data() == nil) != (pop.data == nil) {

			t.Fatalf("%x: opcode %d is %x %x, parseScript gave %x %x",
				script, i, tok.Opcode(), tok.Data(), pop.opcode.value,
				pop.data)
		}
		i++
	}
	if i != len(pops) {
		t.Fatalf("%x: tokenizer gave %d opcodes, parseScript %d", script,
			i, len(pops))
	}
	if errString(tok.Err()) != errString(parseErr) {
		t.Fatalf("%x: tokenizer error %v, parseScript error %v", script,
			tok.Err(), parseErr)
	}
}

// sameSlice returns whether a and b are both nil or the same slice of the
// same array.
func sameSlice(a, b []byte) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) || cap(a) != cap(b) {
		return false
	}
	return cap(a) == 0 || &a[:cap(a)][cap(a)-1] == &b[:cap(b)][cap(b)-1]
}

// errString returns the text of err, or "" for a nil error.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestScriptTokenizerMatchesParse ensures the tokenizer agrees with
// parseScript on random scripts, including truncated pushes.
func TestScriptTokenizerMatchesParse(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	for range 50000 {
		script := randomScript(rng)
		checkTokenizerMatchesParse(t, script)
		for n := len(script) - 1; n >= 0 && n >= len(script)-6; n-- {
			checkTokenizerMatchesParse(t, script[:n])
		}
	}
}

// TestParseScriptLong ensures parseScript matches parseScriptTemplate on
// scripts with about as many opcodes as it counts to size its result,
// ending with or without a truncated push.
func TestParseScriptLong(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(2))
	for n := maxCountedOpcodes - 2; n <= maxCountedOpcodes+2; n++ {
		ops := bytes.Repeat([]byte{OP_NOP}, n)
		for _, tail := range [][]byte{nil, {OP_DATA_2, 1}, {OP_PUSHDATA1},
			{OP_PUSHDATA2, 0x00, 0x01, 1, 2}, randomScript(rng)} {

			checkTokenizerMatchesParse(t, append(ops, tail...))
		}
		pushes := bytes.Repeat([]byte{OP_DATA_3, 1, 2, 3}, n)
		checkTokenizerMatchesParse(t, append(pushes, OP_DATA_2, 1))
		checkTokenizerMatchesParse(t, append(pushes, randomScript(rng)...))
	}
}

// randomScript returns a script of random opcodes, mostly data pushes of
// every kind, whose final push may be truncated.
func randomScript(rng *rand.Rand) []byte {
	var script []byte
	for range rng.Intn(10) {
		switch rng.Intn(6) {
		case 0:
			n := rng.Intn(OP_PUSHDATA1)
			script = append(script, byte(n))
			script = append(script, make([]byte, n)...)
		case 1:
			n := rng.Intn(300)
			script = append(script, OP_PUSHDATA1, byte(n))
			script = append(script, make([]byte, n)...)
		case 2:
			n := rng.Intn(600)
			script = append(script, OP_PUSHDATA2, byte(n), byte(n>>8))
			script = append(script, make([]byte, n)...)
		case 3:
			var l [4]byte
			n := uint32(rng.Intn(600))
			if rng.Intn(20) == 0 {
				n = rng.Uint32()
			}
			binary.LittleEndian.PutUint32(l[:], n)
			script = append(script, OP_PUSHDATA4)
			script = append(script, l[:]...)
			if n < 600 {
				script = append(script, make([]byte, n)...)
			}
		default:
			script = append(script, byte(rng.Intn(256)))
		}
	}
	return script
}

// FuzzScriptTokenizer ensures the tokenizer, parseScript and
// parseScriptTemplate agree.
func FuzzScriptTokenizer(f *testing.F) {
	rng := rand.New(rand.NewSource(1))
	for range 200 {
		f.Add(randomScript(rng))
	}
	f.Add(bytes.Repeat([]byte{OP_NOP}, maxCountedOpcodes+1))
	f.Add(append(bytes.Repeat([]byte{OP_NOP}, maxCountedOpcodes), OP_DATA_1))
	f.Fuzz(func(t *testing.T, script []byte) {
		checkTokenizerMatchesParse(t, script)
	})
}

// TestScriptTokenizerAllocs ensures tokenizing a script does not allocate.
func TestScriptTokenizerAllocs(t *testing.T) {
	script := mustParseRepeatForm("0 DATA_72 0x01{72} PUSHDATA2 0x0001 0x02{256} " +
		"PUSHDATA4 0x05000000 0x03{5} 16 CHECKMULTISIG")
	allocs := testing.AllocsPerRun(100, func() {
		tok := MakeScriptTokenizer(script)
		for tok.Next() {
		}
		if tok.Err() != nil {
			t.Fatal(tok.Err())
		}
	})
	if allocs != 0 {
		t.Fatalf("tokenizing allocated %v times", allocs)
	}
}
