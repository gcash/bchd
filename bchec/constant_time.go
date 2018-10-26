// Copyright (c) 2017 The btcsuite developers
// Copyright (c) 2017 Brent Perreault
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchec

// constant_time.go provides constant time implementations of useful
// mathematical operations. In addition, these functions return integers,
// using 0 or 1 to represent false or true respectively, which is useful
// for writing logic in terms of bitwise operators

// References
// These functions are based on the sample implementation in
// golang.org/src/crypto/subtle/constant_time.go
// Here we have refactored these functions for uint32 arithmetic and
// to avoid extra shifts and casts

// Note - these use the sign bit of int32 internally. For that reason all
// of the inputs need to be less than 2^31 to avoid overflowing int32.
// These are intended for use internal to btcec.

// lessThanUint32 returns 1 if x < y and 0 otherwise.
// It works by checking the most significant bit, and then testing the
// rest of the bits by casting to int32
func lessThanUint32(x, y uint32) uint32 {
	diff := int32(x) - int32(y)
	return uint32((diff >> 31) & 1)
}

// isZeroUint32 returns 1 if x == y and 0 otherwise.
func isZeroUint32(x uint32) uint32 {
	x32 := int32(x)
	return uint32((((x32 - 1) ^ x32) >> 31) & 1)
}

// notZeroUint32 returns 1 if x != y and 0 otherwise.
func notZeroUint32(x uint32) uint32 {
	x32 := int32(x)
	return uint32((((-x32) | x32) >> 31) & 1)
}
