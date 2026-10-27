// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
)

// refScriptNumBytes is the encoder scriptNum.Bytes had before it wrote the
// words directly.  The tests below check the current encoder against it.
func refScriptNumBytes(bigN *big.Int) []byte {
	if bigN.Sign() == 0 {
		return nil
	}
	absBytes := bigN.Bytes()
	isNegative := bigN.Sign() == -1
	result := make([]byte, len(absBytes))
	for i := range absBytes {
		result[i] = absBytes[len(absBytes)-1-i]
	}
	if result[len(result)-1]&0x80 != 0 {
		extraByte := byte(0x00)
		if isNegative {
			extraByte = 0x80
		}
		result = append(result, extraByte)
	} else if isNegative {
		result[len(result)-1] |= 0x80
	}
	return result
}

// refMakeScriptNum is the decoder makeScriptNum had before it decoded into
// words directly.
func refMakeScriptNum(v []byte, requireMinimal bool, scriptNumLen int) (*big.Int, error) {
	if len(v) > scriptNumLen {
		str := fmt.Sprintf("numeric value encoded as %x is %d bytes "+
			"which exceeds the max allowed of %d", v, len(v),
			scriptNumLen)
		return big.NewInt(0), scriptError(
			numberRangeErrorCode(scriptNumLen), str)
	}
	if requireMinimal {
		if err := checkMinimalDataEncoding(v); err != nil {
			return big.NewInt(0), err
		}
	}
	if len(v) == 0 {
		return big.NewInt(0), nil
	}
	bigEndian := make([]byte, len(v))
	for i := range v {
		bigEndian[i] = v[len(v)-1-i]
	}
	isNegative := false
	if bigEndian[0]&0x80 != 0 {
		bigEndian[0] = bigEndian[0] & ^byte(0x80)
		isNegative = true
	}
	result := new(big.Int).SetBytes(bigEndian)
	if isNegative {
		result.Neg(result)
	}
	return result, nil
}

// equivValues returns numbers around every byte and word boundary up to
// maxBytes bytes, both signs, and random numbers of every length.
func equivValues(r *rand.Rand, maxBytes int) []*big.Int {
	var vals []*big.Int
	one := big.NewInt(1)
	for bitLen := 0; bitLen <= maxBytes*8; bitLen++ {
		p := new(big.Int).Lsh(one, uint(bitLen))
		for _, d := range []int64{-2, -1, 0, 1, 2} {
			v := new(big.Int).Add(p, big.NewInt(d))
			vals = append(vals, v, new(big.Int).Neg(v))
		}
		buf := make([]byte, (bitLen+7)/8)
		for k := 0; k < 4; k++ {
			r.Read(buf)
			v := new(big.Int).SetBytes(buf)
			vals = append(vals, v, new(big.Int).Neg(v))
		}
	}
	return vals
}

func TestScriptNumBytesMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for _, v := range equivValues(r, 300) {
		got := scriptNum(*v).Bytes()
		want := refScriptNumBytes(v)
		if !bytes.Equal(got, want) {
			t.Fatalf("Bytes(%v) = %x, want %x", v, got, want)
		}
		if l := (*scriptNum)(v).byteLen(); l != len(want) {
			t.Fatalf("byteLen(%v) = %d, want %d", v, l, len(want))
		}
	}
}

func TestMakeScriptNumMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	var inputs [][]byte
	for _, v := range equivValues(r, 300) {
		inputs = append(inputs, refScriptNumBytes(v))
	}
	// Non-minimal encodings: padded with zero bytes, with the sign moved
	// into the padding, and negative zero.
	for n := 0; n <= 300; n++ {
		for k := 0; k < 4; k++ {
			buf := make([]byte, n)
			r.Read(buf)
			inputs = append(inputs, buf)
			if n > 0 {
				b := append([]byte(nil), buf...)
				b[n-1] = 0x00
				inputs = append(inputs, b)
				b = append([]byte(nil), buf...)
				b[n-1] = 0x80
				inputs = append(inputs, b)
				b = append([]byte(nil), buf...)
				b[n-1] &= 0x7f
				inputs = append(inputs, append(b, 0x80))
			}
		}
		inputs = append(inputs, make([]byte, n))
		neg0 := make([]byte, n)
		if n > 0 {
			neg0[n-1] = 0x80
		}
		inputs = append(inputs, neg0)
	}
	for _, in := range inputs {
		orig := append([]byte(nil), in...)
		for _, minimal := range []bool{false, true} {
			for _, limit := range []int{4, 8, 9, 64, 300, 10000} {
				want, wantErr := refMakeScriptNum(in, minimal, limit)
				got, gotErr := makeScriptNum(in, minimal, limit)
				if !bytes.Equal(in, orig) {
					t.Fatalf("makeScriptNum(%x) modified its input", orig)
				}
				if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
					t.Fatalf("makeScriptNum(%x, %v, %d) error %v, want %v",
						in, minimal, limit, gotErr, wantErr)
				}
				if wantErr != nil {
					if gotErr.(Error).ErrorCode != wantErr.(Error).ErrorCode {
						t.Fatalf("makeScriptNum(%x, %v, %d) code %v, want %v",
							in, minimal, limit, gotErr, wantErr)
					}
					continue
				}
				if (*big.Int)(&got).Cmp(want) != 0 {
					t.Fatalf("makeScriptNum(%x, %v, %d) = %v, want %v",
						in, minimal, limit, (*big.Int)(&got), want)
				}
			}
		}
	}
}

// TestOpMulProductRange checks OP_MUL at the edges of the 64-bit range with
// both signs, and with a zero operand, before and after the May 2025
// upgrade.  Before it, a product outside -(2^63-1) to 2^63-1 fails with
// "integer overflow".
func TestOpMulProductRange(t *testing.T) {
	may2025 := ScriptFlags(may2026TestFlags) &^ ScriptAllowMay2026
	bits64 := may2025 &^ ScriptAllowMay2025
	tests := []struct {
		script   string
		overflow bool // before May 2025
	}{
		{"-5 0 MUL 0 NUMEQUAL", false},
		{"0 -5 MUL 0 NUMEQUAL", false},
		{"-2 -3 MUL 6 NUMEQUAL", false},
		{"-1 -9223372036854775807 MUL 9223372036854775807 NUMEQUAL", false},
		{"9223372036854775807 -1 MUL -9223372036854775807 NUMEQUAL", false},
		{"-4611686018427387903 2 MUL -9223372036854775806 NUMEQUAL", false},
		{"3037000499 -3037000499 MUL -9223372030926249001 NUMEQUAL", false},
		{"4611686018427387904 2 MUL DROP 1", true},
		{"-4611686018427387904 2 MUL DROP 1", true},
		{"2 -4611686018427387904 MUL DROP 1", true},
		{"-3037000500 -3037000500 MUL DROP 1", true},
		{"3037000500 -3037000500 MUL DROP 1", true},
		{"9223372036854775807 9223372036854775807 MUL DROP 1", true},
	}
	for _, test := range tests {
		pkScript := hex.EncodeToString(mustParseShortForm(test.script))
		_, err := runFunctionsTestScript(t, nil, pkScript, bits64)
		if !test.overflow && err != nil {
			t.Errorf("%s: %v", test.script, err)
		}
		if test.overflow && (!IsErrorCode(err, ErrNumberRange64Bit) ||
			err.Error() != "integer overflow") {
			t.Errorf("%s: got %v, want integer overflow", test.script, err)
		}
		if _, err := runFunctionsTestScript(t, nil, pkScript, may2025); err != nil {
			t.Errorf("%s after May 2025: %v", test.script, err)
		}
	}
}
