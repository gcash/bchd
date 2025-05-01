// Copyright (c) 2015-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// hexToBytes converts the passed hex string into bytes and will panic if there
// is an error.  This is only provided for the hard-coded constants so errors in
// the source code can be detected. It will only (and must only) be called with
// hard-coded values.
func hexToBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("invalid hex in source file: " + s)
	}
	return b
}

// TestScriptNumBytes ensures that converting from integral script numbers to
// byte representations works as expected.
func TestScriptNumBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		num        scriptNum
		serialized []byte
	}{
		{*makeScriptNumFromInt64(0), nil},
		{*makeScriptNumFromInt64(1), hexToBytes("01")},
		{*makeScriptNumFromInt64(-1), hexToBytes("81")},
		{*makeScriptNumFromInt64(127), hexToBytes("7f")},
		{*makeScriptNumFromInt64(-127), hexToBytes("ff")},
		{*makeScriptNumFromInt64(128), hexToBytes("8000")},
		{*makeScriptNumFromInt64(-128), hexToBytes("8080")},
		{*makeScriptNumFromInt64(129), hexToBytes("8100")},
		{*makeScriptNumFromInt64(-129), hexToBytes("8180")},
		{*makeScriptNumFromInt64(256), hexToBytes("0001")},
		{*makeScriptNumFromInt64(-256), hexToBytes("0081")},
		{*makeScriptNumFromInt64(32767), hexToBytes("ff7f")},
		{*makeScriptNumFromInt64(-32767), hexToBytes("ffff")},
		{*makeScriptNumFromInt64(32768), hexToBytes("008000")},
		{*makeScriptNumFromInt64(-32768), hexToBytes("008080")},
		{*makeScriptNumFromInt64(65535), hexToBytes("ffff00")},
		{*makeScriptNumFromInt64(-65535), hexToBytes("ffff80")},
		{*makeScriptNumFromInt64(524288), hexToBytes("000008")},
		{*makeScriptNumFromInt64(-524288), hexToBytes("000088")},
		{*makeScriptNumFromInt64(7340032), hexToBytes("000070")},
		{*makeScriptNumFromInt64(-7340032), hexToBytes("0000f0")},
		{*makeScriptNumFromInt64(8388608), hexToBytes("00008000")},
		{*makeScriptNumFromInt64(-8388608), hexToBytes("00008080")},
		{*makeScriptNumFromInt64(2147483647), hexToBytes("ffffff7f")},
		{*makeScriptNumFromInt64(-2147483647), hexToBytes("ffffffff")},

		// Values that are out of range for data that is interpreted as
		// numbers, but are allowed as the result of numeric operations.
		{*makeScriptNumFromInt64(2147483648), hexToBytes("0000008000")},
		{*makeScriptNumFromInt64(-2147483648), hexToBytes("0000008080")},
		{*makeScriptNumFromInt64(2415919104), hexToBytes("0000009000")},
		{*makeScriptNumFromInt64(-2415919104), hexToBytes("0000009080")},
		{*makeScriptNumFromInt64(4294967295), hexToBytes("ffffffff00")},
		{*makeScriptNumFromInt64(-4294967295), hexToBytes("ffffffff80")},
		{*makeScriptNumFromInt64(4294967296), hexToBytes("0000000001")},
		{*makeScriptNumFromInt64(-4294967296), hexToBytes("0000000081")},
		{*makeScriptNumFromInt64(281474976710655), hexToBytes("ffffffffffff00")},
		{*makeScriptNumFromInt64(-281474976710655), hexToBytes("ffffffffffff80")},
		{*makeScriptNumFromInt64(72057594037927935), hexToBytes("ffffffffffffff00")},
		{*makeScriptNumFromInt64(-72057594037927935), hexToBytes("ffffffffffffff80")},
		{*makeScriptNumFromInt64(9223372036854775807), hexToBytes("ffffffffffffff7f")},
		{*makeScriptNumFromInt64(-9223372036854775807), hexToBytes("ffffffffffffffff")},
	}

	for _, test := range tests {
		gotBytes := test.num.Bytes()
		if !bytes.Equal(gotBytes, test.serialized) {
			t.Errorf("Bytes: did not get expected bytes for %s - "+
				"got %x, want %x", test.num, gotBytes,
				test.serialized)
			continue
		}
	}
}

// TestMakeScriptNum ensures that converting from byte representations to
// integral script numbers works as expected.
func TestMakeScriptNum(t *testing.T) {
	t.Parallel()

	// Errors used in the tests below defined here for convenience and to
	// keep the horizontal test size shorter.
	errNumTooBig := scriptError(ErrNumberTooBig, "")
	errMinimalData := scriptError(ErrMinimalData, "")

	tests := []struct {
		serialized      []byte
		num             scriptNum
		numLen          int
		minimalEncoding bool
		err             error
	}{
		// Minimal encoding must reject negative 0.
		{hexToBytes("80"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},

		// Minimally encoded valid values with minimal encoding flag.
		// Should not error and return expected integral number.
		{nil, *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("01"), *makeScriptNumFromInt64(1), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("81"), *makeScriptNumFromInt64(-1), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("7f"), *makeScriptNumFromInt64(127), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("ff"), *makeScriptNumFromInt64(-127), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("8000"), *makeScriptNumFromInt64(128), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("8080"), *makeScriptNumFromInt64(-128), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("8100"), *makeScriptNumFromInt64(129), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("8180"), *makeScriptNumFromInt64(-129), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("0001"), *makeScriptNumFromInt64(256), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("0081"), *makeScriptNumFromInt64(-256), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("ff7f"), *makeScriptNumFromInt64(32767), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("ffff"), *makeScriptNumFromInt64(-32767), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("008000"), *makeScriptNumFromInt64(32768), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("008080"), *makeScriptNumFromInt64(-32768), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("ffff00"), *makeScriptNumFromInt64(65535), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("ffff80"), *makeScriptNumFromInt64(-65535), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("000008"), *makeScriptNumFromInt64(524288), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("000088"), *makeScriptNumFromInt64(-524288), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("000070"), *makeScriptNumFromInt64(7340032), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("0000f0"), *makeScriptNumFromInt64(-7340032), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("00008000"), *makeScriptNumFromInt64(8388608), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("00008080"), *makeScriptNumFromInt64(-8388608), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("ffffff7f"), *makeScriptNumFromInt64(2147483647), defaultSmallScriptNumLen, true, nil},
		{hexToBytes("ffffffff"), *makeScriptNumFromInt64(-2147483647), defaultSmallScriptNumLen, true, nil},

		{nil, *makeScriptNumFromInt64(0), defaultBigScriptNumLen, true, nil},
		{hexToBytes("01"), *makeScriptNumFromInt64(1), defaultBigScriptNumLen, true, nil},
		{hexToBytes("81"), *makeScriptNumFromInt64(-1), defaultBigScriptNumLen, true, nil},
		{hexToBytes("7f"), *makeScriptNumFromInt64(127), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ff"), *makeScriptNumFromInt64(-127), defaultBigScriptNumLen, true, nil},
		{hexToBytes("8000"), *makeScriptNumFromInt64(128), defaultBigScriptNumLen, true, nil},
		{hexToBytes("8080"), *makeScriptNumFromInt64(-128), defaultBigScriptNumLen, true, nil},
		{hexToBytes("8100"), *makeScriptNumFromInt64(129), defaultBigScriptNumLen, true, nil},
		{hexToBytes("8180"), *makeScriptNumFromInt64(-129), defaultBigScriptNumLen, true, nil},
		{hexToBytes("0001"), *makeScriptNumFromInt64(256), defaultBigScriptNumLen, true, nil},
		{hexToBytes("0081"), *makeScriptNumFromInt64(-256), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ff7f"), *makeScriptNumFromInt64(32767), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ffff"), *makeScriptNumFromInt64(-32767), defaultBigScriptNumLen, true, nil},
		{hexToBytes("008000"), *makeScriptNumFromInt64(32768), defaultBigScriptNumLen, true, nil},
		{hexToBytes("008080"), *makeScriptNumFromInt64(-32768), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ffff00"), *makeScriptNumFromInt64(65535), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ffff80"), *makeScriptNumFromInt64(-65535), defaultBigScriptNumLen, true, nil},
		{hexToBytes("000008"), *makeScriptNumFromInt64(524288), defaultBigScriptNumLen, true, nil},
		{hexToBytes("000088"), *makeScriptNumFromInt64(-524288), defaultBigScriptNumLen, true, nil},
		{hexToBytes("000070"), *makeScriptNumFromInt64(7340032), defaultBigScriptNumLen, true, nil},
		{hexToBytes("0000f0"), *makeScriptNumFromInt64(-7340032), defaultBigScriptNumLen, true, nil},
		{hexToBytes("00008000"), *makeScriptNumFromInt64(8388608), defaultBigScriptNumLen, true, nil},
		{hexToBytes("00008080"), *makeScriptNumFromInt64(-8388608), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ffffff7f"), *makeScriptNumFromInt64(2147483647), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ffffffff"), *makeScriptNumFromInt64(-2147483647), defaultBigScriptNumLen, true, nil},
		{hexToBytes("0000000002"), *makeScriptNumFromInt64(8589934592), defaultBigScriptNumLen, true, nil},
		{hexToBytes("0000000082"), *makeScriptNumFromInt64(-8589934592), defaultBigScriptNumLen, true, nil},
		{hexToBytes("0000000000000040"), *makeScriptNumFromInt64(4611686018427387904), defaultBigScriptNumLen, true, nil},
		{hexToBytes("00000000000000c0"), *makeScriptNumFromInt64(-4611686018427387904), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ffffffffffffff7f"), *makeScriptNumFromInt64(9223372036854775807), defaultBigScriptNumLen, true, nil},
		{hexToBytes("ffffffffffffffff"), *makeScriptNumFromInt64(-9223372036854775807), defaultBigScriptNumLen, true, nil},

		{hexToBytes("ffffffff7f"), *makeScriptNumFromInt64(549755813887), 5, true, nil},
		{hexToBytes("ffffffffff"), *makeScriptNumFromInt64(-549755813887), 5, true, nil},
		{hexToBytes("ffffffffffffff7f"), *makeScriptNumFromInt64(9223372036854775807), 8, true, nil},
		{hexToBytes("ffffffffffffffff"), *makeScriptNumFromInt64(-9223372036854775807), 8, true, nil},

		// Minimally encoded values that are out of range for data that
		// is interpreted as script numbers with the minimal encoding
		// flag set.  Should error and return 0.
		{hexToBytes("0000008000"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("0000008080"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("0000009000"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("0000009080"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffff00"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffff80"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("0000000001"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("0000000081"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffffffff00"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffffffff80"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffffffffff00"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffffffffff80"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffffffffff7f"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffffffffffff"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errNumTooBig},

		{hexToBytes("ffffffffffffff7fff"), *makeScriptNumFromInt64(0), defaultBigScriptNumLen, true, errNumTooBig},
		{hexToBytes("ffffffffffffffffff"), *makeScriptNumFromInt64(0), defaultBigScriptNumLen, true, errNumTooBig},

		// Non-minimally encoded, but otherwise valid values with
		// minimal encoding flag.  Should error and return 0.
		{hexToBytes("00"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},       // 0
		{hexToBytes("0100"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},     // 1
		{hexToBytes("7f00"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},     // 127
		{hexToBytes("800000"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},   // 128
		{hexToBytes("810000"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},   // 129
		{hexToBytes("000100"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},   // 256
		{hexToBytes("ff7f00"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData},   // 32767
		{hexToBytes("00800000"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData}, // 32768
		{hexToBytes("ffff0000"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData}, // 65535
		{hexToBytes("00000800"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData}, // 524288
		{hexToBytes("00007000"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, true, errMinimalData}, // 7340032
		{hexToBytes("0009000100"), *makeScriptNumFromInt64(0), 5, true, errMinimalData},                      // 16779520

		// Non-minimally encoded, but otherwise valid values without
		// minimal encoding flag.  Should not error and return expected
		// integral number.
		{hexToBytes("00"), *makeScriptNumFromInt64(0), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("0100"), *makeScriptNumFromInt64(1), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("7f00"), *makeScriptNumFromInt64(127), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("800000"), *makeScriptNumFromInt64(128), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("810000"), *makeScriptNumFromInt64(129), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("000100"), *makeScriptNumFromInt64(256), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("ff7f00"), *makeScriptNumFromInt64(32767), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("00800000"), *makeScriptNumFromInt64(32768), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("ffff0000"), *makeScriptNumFromInt64(65535), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("00000800"), *makeScriptNumFromInt64(524288), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("00007000"), *makeScriptNumFromInt64(7340032), defaultSmallScriptNumLen, false, nil},
		{hexToBytes("0009000100"), *makeScriptNumFromInt64(16779520), 5, false, nil},
	}

	for _, test := range tests {
		// Ensure the error code is of the expected type and the error
		// code matches the value specified in the test instance.
		gotNum, err := makeScriptNum(test.serialized, test.minimalEncoding,
			test.numLen)
		if e := tstCheckScriptError(err, test.err); e != nil {
			t.Errorf("makeScriptNum(%#x): %v", test.serialized, e)
			continue
		}

		if !gotNum.IsEqualeTo(&test.num) {
			t.Errorf("makeScriptNum(%#x): did not get expected "+
				"number - got %s, want %s", test.serialized,
				gotNum, test.num)
			continue
		}
	}
}

// TestScriptNumInt32 ensures that the Int32 function on script number behaves
// as expected.
func TestScriptNumInt32(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   scriptNum
		want int32
	}{
		// Values inside the valid int32 range are just the values
		// themselves cast to an int32.
		{*makeScriptNumFromInt64(0), 0},
		{*makeScriptNumFromInt64(1), 1},
		{*makeScriptNumFromInt64(-1), -1},
		{*makeScriptNumFromInt64(127), 127},
		{*makeScriptNumFromInt64(-127), -127},
		{*makeScriptNumFromInt64(128), 128},
		{*makeScriptNumFromInt64(-128), -128},
		{*makeScriptNumFromInt64(129), 129},
		{*makeScriptNumFromInt64(-129), -129},
		{*makeScriptNumFromInt64(256), 256},
		{*makeScriptNumFromInt64(-256), -256},
		{*makeScriptNumFromInt64(32767), 32767},
		{*makeScriptNumFromInt64(-32767), -32767},
		{*makeScriptNumFromInt64(32768), 32768},
		{*makeScriptNumFromInt64(-32768), -32768},
		{*makeScriptNumFromInt64(65535), 65535},
		{*makeScriptNumFromInt64(-65535), -65535},
		{*makeScriptNumFromInt64(524288), 524288},
		{*makeScriptNumFromInt64(-524288), -524288},
		{*makeScriptNumFromInt64(7340032), 7340032},
		{*makeScriptNumFromInt64(-7340032), -7340032},
		{*makeScriptNumFromInt64(8388608), 8388608},
		{*makeScriptNumFromInt64(-8388608), -8388608},
		{*makeScriptNumFromInt64(2147483647), 2147483647},
		{*makeScriptNumFromInt64(-2147483647), -2147483647},
		{*makeScriptNumFromInt64(-2147483648), -2147483648},

		// Values outside of the valid int32 range are limited to int32.
		{*makeScriptNumFromInt64(2147483648), 2147483647},
		{*makeScriptNumFromInt64(-2147483649), -2147483648},
		{*makeScriptNumFromInt64(1152921504606846975), 2147483647},
		{*makeScriptNumFromInt64(-1152921504606846975), -2147483648},
		{*makeScriptNumFromInt64(2305843009213693951), 2147483647},
		{*makeScriptNumFromInt64(-2305843009213693951), -2147483648},
		{*makeScriptNumFromInt64(4611686018427387903), 2147483647},
		{*makeScriptNumFromInt64(-4611686018427387903), -2147483648},
		{*makeScriptNumFromInt64(9223372036854775807), 2147483647},
		{*makeScriptNumFromInt64(-9223372036854775808), -2147483648},
	}

	for _, test := range tests {
		got := test.in.Int32()
		if got != test.want {
			t.Errorf("Int32: did not get expected value for %s - "+
				"got %d, want %d", test.in, got, test.want)
			continue
		}
	}
}

// TestScriptNumInt64 ensures that the Int64 function on script number behaves
// as expected.
func TestScriptNumInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   scriptNum
		want int64
	}{
		// Values inside the valid int64 range are just the values
		// themselves cast to an int64.
		{*makeScriptNumFromInt64(0), 0},
		{*makeScriptNumFromInt64(1), 1},
		{*makeScriptNumFromInt64(-1), -1},
		{*makeScriptNumFromInt64(127), 127},
		{*makeScriptNumFromInt64(-127), -127},
		{*makeScriptNumFromInt64(128), 128},
		{*makeScriptNumFromInt64(-128), -128},
		{*makeScriptNumFromInt64(129), 129},
		{*makeScriptNumFromInt64(-129), -129},
		{*makeScriptNumFromInt64(256), 256},
		{*makeScriptNumFromInt64(-256), -256},
		{*makeScriptNumFromInt64(32767), 32767},
		{*makeScriptNumFromInt64(-32767), -32767},
		{*makeScriptNumFromInt64(32768), 32768},
		{*makeScriptNumFromInt64(-32768), -32768},
		{*makeScriptNumFromInt64(65535), 65535},
		{*makeScriptNumFromInt64(-65535), -65535},
		{*makeScriptNumFromInt64(524288), 524288},
		{*makeScriptNumFromInt64(-524288), -524288},
		{*makeScriptNumFromInt64(7340032), 7340032},
		{*makeScriptNumFromInt64(-7340032), -7340032},
		{*makeScriptNumFromInt64(8388608), 8388608},
		{*makeScriptNumFromInt64(-8388608), -8388608},
		{*makeScriptNumFromInt64(2147483647), 2147483647},
		{*makeScriptNumFromInt64(-2147483647), -2147483647},
		{*makeScriptNumFromInt64(-2147483648), -2147483648},
		{*makeScriptNumFromInt64(2147483648), 2147483648},
		{*makeScriptNumFromInt64(-2147483649), -2147483649},
		{*makeScriptNumFromInt64(1152921504606846975), 1152921504606846975},
		{*makeScriptNumFromInt64(-1152921504606846975), -1152921504606846975},
		{*makeScriptNumFromInt64(2305843009213693951), 2305843009213693951},
		{*makeScriptNumFromInt64(-2305843009213693951), -2305843009213693951},
		{*makeScriptNumFromInt64(4611686018427387903), 4611686018427387903},
		{*makeScriptNumFromInt64(-4611686018427387903), -4611686018427387903},
		{*makeScriptNumFromInt64(9223372036854775807), 9223372036854775807},
		{*makeScriptNumFromInt64(-9223372036854775808), -9223372036854775808},
	}

	for _, test := range tests {
		got := test.in.Int64()
		if got != test.want {
			t.Errorf("Int64: did not get expected value for %s - "+
				"got %d, want %d", test.in, got, test.want)
			continue
		}
	}
}

func Test_minimallyEncode(t *testing.T) {
	tests := []struct {
		data     []byte
		expected []byte
	}{
		{
			bytes.Repeat([]byte{0x00}, MaxScriptElementSize),
			[]byte{},
		},
		{
			[]byte{0x80},
			[]byte{},
		},
		{
			[]byte{0x00, 0x80},
			[]byte{},
		},
		{
			[]byte{0x01, 0x00},
			[]byte{0x01},
		},
		{
			[]byte{0x01, 0x80},
			[]byte{0x81},
		},
	}

	for i, test := range tests {
		result := minimallyEncode(test.data)
		if !bytes.Equal(result, test.expected) {
			t.Errorf("Test %d: Expected %v, got %v", i, test.expected, result)
		}

		if checkMinimalDataEncoding(result) != nil {
			t.Errorf("Test %d: Result not minimally encoded", i)
		}
	}
}
