// Copyright (c) 2015-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"fmt"
)

const (
	maxInt32 = 1<<31 - 1
	minInt32 = -1 << 31

	maxInt64 = 1<<63 - 1
	minInt64 = -1 << 63

	// defaultSmallScriptNumLen is the default number of bytes
	// data being interpreted as a small integer may be.
	defaultSmallScriptNumLen = 4

	// defaultBigScriptNumLen is the default number of bytes
	// data being interpreted as a big integer may be.
	defaultBigScriptNumLen = 8
)

// scriptNum represents a numeric value used in the scripting engine with
// special handling to deal with the subtle semantics required by consensus.
//
// All numbers are stored on the data and alternate stacks encoded as little
// endian with a sign bit.  All numeric opcodes such as OP_ADD, OP_SUB,
// and OP_MUL, are only allowed to operate on n-byte integers where n=4 bytes
// until the CosmicInflation upgrade, and n=8 bytes after the upgrade.  That is,
// before CosmicInflation the range is [-2^31 + 1, 2^31 - 1], and after the
// range is [-2^63 + 1, 2^63 - 1].
//
// However the results of numeric operations may overflow. Before the
// CosmicInflation upgrade, overflows remain valid so long as they are not used
// as inputs to other numeric operations or otherwise interpreted as an integer.
// After the CosmicInflation upgrade, overflows result in an error and
// termination of script execution.
//
// For example, before the CosmicInflation upgrade:
// It is possible for OP_ADD to have 2^31 - 1 for its two operands resulting
// in 2^32 - 2, which overflows, but is still pushed to the stack as the
// result of the addition.  That value can then be used as input to OP_VERIFY
// which will succeed because the data is being interpreted as a boolean.
// However, if that same value were to be used as input to another numeric
// opcode, such as OP_SUB, it must fail.
//
// In a similar example after the CosmicInflation upgrade:
// It is possible for OP_ADD to have 2^63 - 1 for its two operands resulting
// in 2^64 - 2, which overflows, and results in an error and termination of
// script execution.
//
// This type handles the aforementioned requirements by storing all numeric
// operation results as an int64 and provides the Bytes method to get the
// serialized representation (including values that overflow).
//
// Then, whenever data is interpreted as an integer, it is converted to this
// type by using the makeScriptNum function which will return an error if the
// number is out of range or not minimally encoded depending on parameters.
// Since all numeric opcodes involve pulling data from the stack and
// interpreting it as an integer, it provides the required behavior.
type scriptNum int64

// checkMinimalDataEncoding returns whether or not the passed byte array adheres
// to the minimal encoding requirements.
func checkMinimalDataEncoding(v []byte) error {
	if len(v) == 0 {
		return nil
	}

	// Check that the number is encoded with the minimum possible
	// number of bytes.
	//
	// If the most-significant-byte - excluding the sign bit - is zero
	// then we're not minimal.  Note how this test also rejects the
	// negative-zero encoding, [0x80].
	if v[len(v)-1]&0x7f == 0 {
		// One exception: if there's more than one byte and the most
		// significant bit of the second-most-significant-byte is set
		// it would conflict with the sign bit.  An example of this case
		// is +-255, which encode to 0xff00 and 0xff80 respectively.
		// (big-endian).
		if len(v) == 1 || v[len(v)-2]&0x80 == 0 {
			str := fmt.Sprintf("numeric value encoded as %x is "+
				"not minimally encoded", v)
			return scriptError(ErrMinimalData, str)
		}
	}

	return nil
}

// Bytes returns the number serialized as a little endian with a sign bit.
//
// Example encodings:
//
//	   127 -> [0x7f]
//	  -127 -> [0xff]
//	   128 -> [0x80 0x00]
//	  -128 -> [0x80 0x80]
//	   129 -> [0x81 0x00]
//	  -129 -> [0x81 0x80]
//	   256 -> [0x00 0x01]
//	  -256 -> [0x00 0x81]
//	 32767 -> [0xff 0x7f]
//	-32767 -> [0xff 0xff]
//	 32768 -> [0x00 0x80 0x00]
//	-32768 -> [0x00 0x80 0x80]
func (n scriptNum) Bytes() []byte {
	// Zero encodes as an empty byte slice.
	if n == 0 {
		return nil
	}

	// Take the absolute value and keep track of whether it was originally
	// negative.
	isNegative := n < 0
	if isNegative {
		n = -n
	}

	// Encode to little endian.  The maximum number of encoded bytes is 9
	// (8 bytes for max int64 plus a potential byte for sign extension).
	result := make([]byte, 0, 9)
	for n > 0 {
		result = append(result, byte(n&0xff))
		n >>= 8
	}

	// When the most significant byte already has the high bit set, an
	// additional high byte is required to indicate whether the number is
	// negative or positive.  The additional byte is removed when converting
	// back to an integral and its high bit is used to denote the sign.
	//
	// Otherwise, when the most significant byte does not already have the
	// high bit set, use it to indicate the value is negative, if needed.
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

// Int32 returns the script number clamped to a valid int32.  That is to say
// when the script number is higher than the max allowed int32, the max int32
// value is returned and vice versa for the minimum value.  Note that this
// behavior is different from a simple int32 cast because that truncates
// and the consensus rules dictate numbers which are directly cast to ints
// provide this behavior.
//
// In practice, for most opcodes, the number should never be out of range since
// it will have been created with makeScriptNum using the defaultScriptLen
// value, which rejects them.  In case something in the future ends up calling
// this function against the result of some arithmetic, which IS allowed to be
// out of range before being reinterpreted as an integer, this will provide the
// correct behavior.
func (n scriptNum) Int32() int32 {
	if n > maxInt32 {
		return maxInt32
	}

	if n < minInt32 {
		return minInt32
	}

	return int32(n)
}

// Int64 returns the script number clamped to a valid int64.  That is to say
// when the script number is higher than the max allowed int64, the max int64
// value is returned and vice versa for the minimum value.  Note that this
// behavior is different from a simple int64 cast because that truncates
// and the consensus rules dictate numbers which are directly cast to ints
// provide this behavior.
//
// In practice, for most opcodes, the number should never be out of range since
// it will have been created with makeScriptNum using the defaultScriptLen
// value, which rejects them.  In case something in the future ends up calling
// this function against the result of some arithmetic, which IS allowed to be
// out of range before being reinterpreted as an integer, this will provide the
// correct behavior.
func (n scriptNum) Int64() int64 {
	if n > maxInt64 {
		return maxInt64
	}

	if n < minInt64 {
		return minInt64
	}

	return int64(n)
}

// makeScriptNum interprets the passed serialized bytes as an encoded integer
// and returns the result as a script number.
//
// The requireMinimal flag causes an error to be returned if additional checks
// on the encoding determine it is not represented with the smallest possible
// number of bytes or is the negative 0 encoding, [0x80].  For example, consider
// the number 127.  It could be encoded as [0x7f], [0x7f 0x00],
// [0x7f 0x00 0x00 ...], etc.  All forms except [0x7f] will return an error with
// requireMinimal enabled.
//
// The scriptNumLen is the maximum number of bytes the encoded value can be
// before an ErrStackNumberTooBig is returned.  This effectively limits the
// range of allowed values and is important, for example, where script
// arguments are allowed only in a limited range on a per opcode basis.
//
// WARNING:  Great care should be taken if passing a value larger than
// defaultScriptNumLen, which could lead to undefined behavior.
//
// See the Bytes function documentation for example encodings.
func makeScriptNum(v []byte, requireMinimal bool, scriptNumLen int) (scriptNum, error) {
	// Interpreting data requires that it is not larger than
	// the the passed scriptNumLen value.
	if len(v) > scriptNumLen {
		str := fmt.Sprintf("numeric value encoded as %x is %d bytes "+
			"which exceeds the max allowed of %d", v, len(v),
			scriptNumLen)
		return 0, scriptError(ErrNumberTooBig, str)
	}

	// Enforce minimal encoded if requested.
	if requireMinimal {
		if err := checkMinimalDataEncoding(v); err != nil {
			return 0, err
		}
	}

	// Zero is encoded as an empty byte slice.
	if len(v) == 0 {
		return 0, nil
	}

	// Decode from little endian.
	var result int64
	for i, val := range v {
		result |= int64(val) << uint8(8*i)
	}

	// When the most significant byte of the input bytes has the sign bit
	// set, the result is negative.  So, remove the sign bit from the result
	// and make it negative.
	if v[len(v)-1]&0x80 != 0 {
		// The maximum length of v has already been determined to be 4
		// above, so uint8 is enough to cover the max possible shift
		// value of 24.
		result &= ^(int64(0x80) << uint8(8*(len(v)-1)))
		return scriptNum(-result), nil
	}

	return scriptNum(result), nil
}

// minimallyEncode takes in a byte slice and returns a slice that contains a
// minimally encoded version of the original.
func minimallyEncode(data []byte) (ret []byte) {
	if len(data) == 0 {
		return []byte{}
	}

	// If the last byte is not 0x00 or 0x80, we are minimally encoded.
	last := data[len(data)-1]
	if last&0x7f > 0 {
		ret = make([]byte, len(data))
		copy(ret, data)
		return
	}

	// If the script is one byte long, then we have a zero, which encodes as an
	// empty array.
	if len(data) == 1 {
		return []byte{}
	}

	// If the next byte has it sign bit set, then we are minimaly encoded.
	if data[len(data)-2]&0x80 > 0 {
		ret = make([]byte, len(data))
		copy(ret, data)
		return
	}

	// We are not minimally encoded, we need to figure out how much to trim.
	ret = make([]byte, len(data))
	copy(ret, data)
	for i := len(ret) - 1; i > 0; i-- {
		// We found a non zero byte, time to encode.
		if ret[i-1] != 0 {
			if ret[i-1]&0x80 > 0 {
				// We found a byte with it sign bit set so we need one more
				// byte.
				i++
				ret[i] = last
			} else {
				// the sign bit is clear, we can use it.
				ret[i-1] |= last
			}

			ret = ret[:i]
			return
		}
	}

	// If we the whole thing is zeros, then we have a zero.
	return []byte{}
}
