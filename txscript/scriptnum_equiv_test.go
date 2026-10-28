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

// TestScriptNumArithmeticMatchesReference checks the scriptNum operations,
// which compute into their receiver, against big.Int results computed into
// fresh values, both with a fresh receiver and with the receiver aliasing
// the first operand, and the comparisons with an int64 against big.Int.Cmp.
func TestScriptNumArithmeticMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	vals := equivValues(r, 40)
	ints := []int64{0, 1, -1, 2, -2, 127, -128, maxInt32, minInt32, maxInt64, minInt64, minInt64 + 1}
	check := func(x, y *big.Int) {
		type op struct {
			name string
			do   func(n, a, b *scriptNum) *scriptNum
			ref  func(a, b *big.Int) *big.Int
		}
		ops := []op{
			{"Add", func(n, a, b *scriptNum) *scriptNum { return n.Add(a, b) }, func(a, b *big.Int) *big.Int { return new(big.Int).Add(a, b) }},
			{"Sub", func(n, a, b *scriptNum) *scriptNum { return n.Sub(a, b) }, func(a, b *big.Int) *big.Int { return new(big.Int).Sub(a, b) }},
			{"Mul", func(n, a, b *scriptNum) *scriptNum { return n.Mul(a, b) }, func(a, b *big.Int) *big.Int { return new(big.Int).Mul(a, b) }},
			{"Neg", func(n, a, b *scriptNum) *scriptNum { return n.Neg(a) }, func(a, b *big.Int) *big.Int { return new(big.Int).Neg(a) }},
			{"Abs", func(n, a, b *scriptNum) *scriptNum { return n.Abs(a) }, func(a, b *big.Int) *big.Int { return new(big.Int).Abs(a) }},
			{"LShift", func(n, a, b *scriptNum) *scriptNum { return n.LShift(a, 13) }, func(a, b *big.Int) *big.Int { return new(big.Int).Lsh(a, 13) }},
			{"RShift", func(n, a, b *scriptNum) *scriptNum { return n.RShift(a, 13) }, func(a, b *big.Int) *big.Int { return new(big.Int).Rsh(a, 13) }},
		}
		if y.Sign() != 0 {
			ops = append(ops,
				op{"Div", func(n, a, b *scriptNum) *scriptNum { return n.Div(a, b) }, func(a, b *big.Int) *big.Int { return new(big.Int).Quo(a, b) }},
				op{"Mod", func(n, a, b *scriptNum) *scriptNum { return n.Mod(a, b) }, func(a, b *big.Int) *big.Int { return new(big.Int).Rem(a, b) }})
		}
		for _, o := range ops {
			want := o.ref(x, y)
			a, b := scriptNum(*new(big.Int).Set(x)), scriptNum(*new(big.Int).Set(y))
			var n scriptNum
			if got := (*big.Int)(o.do(&n, &a, &b)); got.Cmp(want) != 0 {
				t.Fatalf("%s(%v, %v) = %v, want %v", o.name, x, y, got, want)
			}
			if (*big.Int)(&a).Cmp(x) != 0 || (*big.Int)(&b).Cmp(y) != 0 {
				t.Fatalf("%s(%v, %v) modified an operand", o.name, x, y)
			}
			if got := (*big.Int)(o.do(&a, &a, &b)); got.Cmp(want) != 0 {
				t.Fatalf("%s(%v, %v) in place = %v, want %v", o.name, x, y, got, want)
			}
		}
	}

	// Every value is an operand, on either side, with each of ints, which
	// hold the edges of the 32 and 64-bit ranges, and with two others
	// spread over vals, and is compared with each of ints, so boundaries
	// like -2^63 are always checked.
	for i, x := range vals {
		for _, v := range ints {
			check(x, big.NewInt(v))
			check(big.NewInt(v), x)
		}
		check(x, vals[len(vals)-1-i])
		check(x, vals[(i*7919+1)%len(vals)])
		n := scriptNum(*x)
		for _, v := range ints {
			c := x.Cmp(big.NewInt(v))
			if n.IsEqualeToInt(v) != (c == 0) || n.IsGreaterThanInt(v) != (c == 1) ||
				n.IsLessThanInt(v) != (c == -1) {
				t.Fatalf("comparing %v with %d disagrees with Cmp", x, v)
			}
		}
	}
}

// TestPopLegacyInt32MatchesPopLegacyInt checks popLegacyInt32 against
// popLegacyInt followed by Int32: the value, the error and the stack left.
func TestPopLegacyInt32MatchesPopLegacyInt(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	var items [][]byte
	for _, v := range equivValues(r, 12) {
		items = append(items, refScriptNumBytes(v))
	}
	for n := 0; n <= 12; n++ {
		for k := 0; k < 8; k++ {
			buf := make([]byte, n)
			r.Read(buf)
			items = append(items, buf)
			if n > 0 {
				b := append([]byte(nil), buf...)
				b[n-1] &= 0x80
				items = append(items, b)
			}
		}
	}
	for _, minimal := range []bool{false, true} {
		for _, numLen := range []int{defaultSmallScriptNumLen, defaultBigScriptNumLen, defaultBigIntScriptNumLen} {
			// An empty stack fails the same way.
			s1 := stack{verifyMinimalData: minimal, defaultScriptNumLen: numLen}
			s2 := s1
			_, err1 := s1.popLegacyInt()
			_, err2 := s2.popLegacyInt32()
			if err1 == nil || fmt.Sprint(err1) != fmt.Sprint(err2) {
				t.Fatalf("empty stack: errors %v and %v", err1, err2)
			}
			for _, item := range items {
				s1 := stack{stk: [][]byte{{1}, item}, verifyMinimalData: minimal, defaultScriptNumLen: numLen}
				s2 := stack{stk: [][]byte{{1}, item}, verifyMinimalData: minimal, defaultScriptNumLen: numLen}
				n1, err1 := s1.popLegacyInt()
				v2, err2 := s2.popLegacyInt32()
				if fmt.Sprint(err1) != fmt.Sprint(err2) {
					t.Fatalf("%x (minimal %v, len %d): errors %v and %v",
						item, minimal, numLen, err1, err2)
				}
				if err1 != nil && err1.(Error).ErrorCode != err2.(Error).ErrorCode {
					t.Fatalf("%x: codes %v and %v", item, err1, err2)
				}
				if err1 == nil && n1.Int32() != v2 {
					t.Fatalf("%x (minimal %v, len %d): %d and %d",
						item, minimal, numLen, n1.Int32(), v2)
				}
				if s1.Depth() != s2.Depth() {
					t.Fatalf("%x: depths %d and %d", item, s1.Depth(), s2.Depth())
				}
			}
		}
	}
}

// TestSmallIntBytes checks the shared encodings of true, -1 and 1 to 16
// against the encoder.
func TestSmallIntBytes(t *testing.T) {
	for n := 1; n <= 16; n++ {
		want := makeScriptNumFromInt(n).Bytes()
		if !bytes.Equal(smallIntBytes[n], want) || cap(smallIntBytes[n]) != len(want) {
			t.Fatalf("smallIntBytes[%d] = %x, want %x", n, smallIntBytes[n], want)
		}
	}
	if want := makeScriptNumFromInt(-1).Bytes(); !bytes.Equal(negOneBytes, want) || cap(negOneBytes) != 1 {
		t.Fatalf("negOneBytes = %x, want %x", negOneBytes, want)
	}
	if want := makeScriptNumFromInt(1).Bytes(); !bytes.Equal(trueBytes, want) || cap(trueBytes) != 1 {
		t.Fatalf("trueBytes = %x, want %x", trueBytes, want)
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
