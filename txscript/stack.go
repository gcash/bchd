// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// asBool gets the boolean value of the byte array.
func asBool(t []byte) bool {
	for i := range t {
		if t[i] != 0 {
			// Negative 0 is also considered false.
			if i == len(t)-1 && t[i] == 0x80 {
				return false
			}
			return true
		}
	}
	return false
}

// fromBool converts a boolean into the appropriate byte array.
func fromBool(v bool) []byte {
	if v {
		return []byte{1}
	}
	return nil
}

// stack represents a stack of immutable objects to be used with bitcoin
// scripts.  Objects may be shared, therefore in usage if a value is to be
// changed it *must* be deep-copied first to avoid changing other values on the
// stack.
type stack struct {
	stk                 [][]byte
	verifyMinimalData   bool
	defaultScriptNumLen int
}

// Depth returns the number of items on the stack.
func (s *stack) Depth() int32 {
	return int32(len(s.stk))
}

// PushByteArray adds the given back array to the top of the stack.
//
// Stack transformation: [... x1 x2] -> [... x1 x2 data]
func (s *stack) PushByteArray(so []byte) {
	s.stk = append(s.stk, so)
}

// PushInt converts the provided scriptNum to a suitable byte array then pushes
// it onto the top of the stack.
//
// Stack transformation: [... x1 x2] -> [... x1 x2 int]
func (s *stack) PushInt(val scriptNum) {
	s.PushByteArray(val.Bytes())
}

// PushBool converts the provided boolean to a suitable byte array then pushes
// it onto the top of the stack.
//
// Stack transformation: [... x1 x2] -> [... x1 x2 bool]
func (s *stack) PushBool(val bool) {
	s.PushByteArray(fromBool(val))
}

// PopByteArray pops the value off the top of the stack and returns it.
//
// Stack transformation: [... x1 x2 x3] -> [... x1 x2]
func (s *stack) PopByteArray() ([]byte, error) {
	return s.nipN(0)
}

// PopInt pops the value off the top of the stack, converts it into a script
// num, and returns it.  The act of converting to a script num enforces the
// consensus rules imposed on data interpreted as numbers.
//
// Stack transformation: [... x1 x2 x3] -> [... x1 x2]
func (s *stack) PopInt() (scriptNum, error) {
	so, err := s.PopByteArray()
	if err != nil {
		return *makeScriptNumFromInt(0), err
	}

	return makeScriptNum(so, s.verifyMinimalData, s.defaultScriptNumLen)
}

// checkDepth returns an error unless the stack holds at least n items.  BCHN
// checks this before an opcode reads any of its operands, so that a failure
// is reported as BCHN reports it.
func (s *stack) checkDepth(n int32) error {
	if sz := s.Depth(); sz < n {
		str := fmt.Sprintf("index %d is invalid for stack size %d", n-1,
			sz)
		return scriptError(ErrInvalidStackOperation, str)
	}
	return nil
}

// popIntPair pops the top two items of the stack as script numbers and
// returns the second item and the top one.  It reads the second item first,
// as BCHN does, so a missing item fails before the top one is read.
//
// Stack transformation: [... x1 x2] -> [...]
func (s *stack) popIntPair() (scriptNum, scriptNum, error) {
	zero := *makeScriptNumFromInt(0)
	a, err := s.PeekInt(1)
	if err != nil {
		return zero, zero, err
	}
	b, err := s.PeekInt(0)
	if err != nil {
		return zero, zero, err
	}
	s.dropTop(2)
	return a, b, nil
}

// popIntTriple pops the top three items of the stack as script numbers and
// returns them deepest first.  It reads the deepest first, as BCHN does, so a
// missing item fails before the others are read.
//
// Stack transformation: [... x1 x2 x3] -> [...]
func (s *stack) popIntTriple() (scriptNum, scriptNum, scriptNum, error) {
	zero := *makeScriptNumFromInt(0)
	a, err := s.PeekInt(2)
	if err != nil {
		return zero, zero, zero, err
	}
	b, err := s.PeekInt(1)
	if err != nil {
		return zero, zero, zero, err
	}
	c, err := s.PeekInt(0)
	if err != nil {
		return zero, zero, zero, err
	}
	s.dropTop(3)
	return a, b, c, nil
}

// dropTop removes the top n items, which the caller has checked are there.
func (s *stack) dropTop(n int) {
	sz := len(s.stk)
	clear(s.stk[sz-n:])
	s.stk = s.stk[:sz-n]
}

// popLegacyInt pops the top item of the stack as a script number, for the
// stack positions, sizes, counts and indexes opcodes take.  BCHN reads these
// with a limit of 8 bytes whatever the size of other numbers, so once numbers
// may be longer, an item over 8 bytes that does not read, or whose value is
// outside the 64-bit range, is reported as BCHN reports it, outside the
// 64-bit range.  Every such item fails here or in the opcode, as none of these
// values can be that large, so the error is the only change.  A longer item
// whose value is in the 64-bit range, which only an encoding that is not
// minimal can give, is read as before.
//
// Stack transformation: [... x1 x2] -> [... x1]
func (s *stack) popLegacyInt() (scriptNum, error) {
	so, err := s.PopByteArray()
	if err != nil {
		return *makeScriptNumFromInt(0), err
	}

	n, err := makeScriptNum(so, s.verifyMinimalData, s.defaultScriptNumLen)
	if s.defaultScriptNumLen > defaultBigScriptNumLen &&
		len(so) > defaultBigScriptNumLen && (err != nil ||
		n.IsGreaterThanInt(maxInt64) || n.IsLessThanInt(-maxInt64)) {

		str := fmt.Sprintf("numeric value encoded as %x is %d bytes "+
			"which exceeds the max allowed of %d", so, len(so),
			defaultBigScriptNumLen)
		return *makeScriptNumFromInt(0), scriptError(ErrNumberRange64Bit,
			str)
	}
	return n, err
}

// popLegacyInt32 returns popLegacyInt's number as Int32 gives it, without
// making a big.Int for an item of up to 8 bytes, which is never reported
// outside the 64-bit range.  It pops the item, and returns the same error,
// as popLegacyInt.
func (s *stack) popLegacyInt32() (int32, error) {
	so, err := s.PeekByteArray(0)
	if err != nil || len(so) > defaultBigScriptNumLen {
		n, err := s.popLegacyInt()
		if err != nil {
			return 0, err
		}
		return n.Int32(), nil
	}
	s.dropTop(1)
	if err := checkScriptNum(so, s.verifyMinimalData, s.defaultScriptNumLen); err != nil {
		return 0, err
	}
	v := decodeSmallScriptNum(so)
	if v > maxInt32 {
		return maxInt32, nil
	}
	if v < minInt32 {
		return minInt32, nil
	}
	return int32(v), nil
}

// PopBool pops the value off the top of the stack, converts it into a bool, and
// returns it.
//
// Stack transformation: [... x1 x2 x3] -> [... x1 x2]
func (s *stack) PopBool(requireMinimalIf bool) (bool, error) {
	so, err := s.PopByteArray()
	if err != nil {
		return false, err
	}
	if requireMinimalIf && (len(so) > 1 || len(so) == 1 && so[0] != 0x01) {
		return false, scriptError(ErrMinimalIf, "not minimal if encoding")
	}

	return asBool(so), nil
}

// PeekByteArray returns the Nth item on the stack without removing it.
func (s *stack) PeekByteArray(idx int32) ([]byte, error) {
	sz := int32(len(s.stk))
	if idx < 0 || idx >= sz {
		str := fmt.Sprintf("index %d is invalid for stack size %d", idx,
			sz)
		return nil, scriptError(ErrInvalidStackOperation, str)
	}

	return s.stk[sz-idx-1], nil
}

// PeekInt returns the Nth item on the stack as a script num without removing
// it.  The act of converting to a script num enforces the consensus rules
// imposed on data interpreted as numbers.
func (s *stack) PeekInt(idx int32) (scriptNum, error) {
	so, err := s.PeekByteArray(idx)
	if err != nil {
		return *makeScriptNumFromInt(0), err
	}

	return makeScriptNum(so, s.verifyMinimalData, s.defaultScriptNumLen)
}

// PeekBool returns the Nth item on the stack as a bool without removing it.
func (s *stack) PeekBool(idx int32) (bool, error) {
	so, err := s.PeekByteArray(idx)
	if err != nil {
		return false, err
	}

	return asBool(so), nil
}

// nipN is an internal function that removes the nth item on the stack and
// returns it.
//
// Stack transformation:
// nipN(0): [... x1 x2 x3] -> [... x1 x2]
// nipN(1): [... x1 x2 x3] -> [... x1 x3]
// nipN(2): [... x1 x2 x3] -> [... x2 x3]
func (s *stack) nipN(idx int32) ([]byte, error) {
	sz := int32(len(s.stk))
	if idx < 0 || idx > sz-1 {
		str := fmt.Sprintf("index %d is invalid for stack size %d", idx,
			sz)
		return nil, scriptError(ErrInvalidStackOperation, str)
	}

	so := s.stk[sz-idx-1]
	copy(s.stk[sz-idx-1:], s.stk[sz-idx:])
	s.stk[sz-1] = nil
	s.stk = s.stk[:sz-1]
	return so, nil
}

// NipN removes the Nth object on the stack
//
// Stack transformation:
// NipN(0): [... x1 x2 x3] -> [... x1 x2]
// NipN(1): [... x1 x2 x3] -> [... x1 x3]
// NipN(2): [... x1 x2 x3] -> [... x2 x3]
func (s *stack) NipN(idx int32) error {
	_, err := s.nipN(idx)
	return err
}

// Tuck copies the item at the top of the stack and inserts it before the 2nd
// to top item.
//
// Stack transformation: [... x1 x2] -> [... x2 x1 x2]
func (s *stack) Tuck() (int, error) {
	so2, err := s.PopByteArray()
	if err != nil {
		return 0, err
	}
	so1, err := s.PopByteArray()
	if err != nil {
		return 0, err
	}
	s.PushByteArray(so2) // stack [... x2]
	s.PushByteArray(so1) // stack [... x2 x1]
	s.PushByteArray(so2) // stack [... x2 x1 x2]

	tuckedItemLength := len(so2)

	return tuckedItemLength, nil
}

// DropN removes the top N items from the stack.
//
// Stack transformation:
// DropN(1): [... x1 x2] -> [... x1]
// DropN(2): [... x1 x2] -> [...]
func (s *stack) DropN(n int32) error {
	if n < 1 {
		str := fmt.Sprintf("attempt to drop %d items from stack", n)
		return scriptError(ErrInvalidStackOperation, str)
	}

	for ; n > 0; n-- {
		_, err := s.PopByteArray()
		if err != nil {
			return err
		}
	}
	return nil
}

// clear removes all items from the stack.
func (s *stack) clear() {
	clear(s.stk)
	s.stk = s.stk[:0]
}

// DupN duplicates the top N items on the stack.
//
// Stack transformation:
// DupN(1): [... x1 x2] -> [... x1 x2 x2]
// DupN(2): [... x1 x2] -> [... x1 x2 x1 x2]
func (s *stack) DupN(n int32) (int, error) {
	// This value is needed to calculate opsCost after may 2025 upgrade.
	totalDuplicatedItemsLength := 0

	if n < 1 {
		str := fmt.Sprintf("attempt to dup %d stack items", n)
		return totalDuplicatedItemsLength, scriptError(ErrInvalidStackOperation, str)
	}

	// Iteratively duplicate the value n-1 down the stack n times.
	// This leaves an in-order duplicate of the top n items on the stack.
	for i := n; i > 0; i-- {
		so, err := s.PeekByteArray(n - 1)
		if err != nil {
			return totalDuplicatedItemsLength, err
		}
		s.PushByteArray(so)
		totalDuplicatedItemsLength += len(so)
	}
	return totalDuplicatedItemsLength, nil
}

// RotN rotates the top 3N items on the stack to the left N times.
//
// Stack transformation:
// RotN(1): [... x1 x2 x3] -> [... x2 x3 x1]
// RotN(2): [... x1 x2 x3 x4 x5 x6] -> [... x3 x4 x5 x6 x1 x2]
func (s *stack) RotN(n int32) (int, error) {
	// This value is needed to calculate opsCost after may 2025 upgrade.
	totalRotatedItemsLength := 0

	if n < 1 {
		str := fmt.Sprintf("attempt to rotate %d stack items", n)
		return totalRotatedItemsLength, scriptError(ErrInvalidStackOperation, str)
	}

	// Nip the 3n-1th item from the stack to the top n times to rotate
	// them up to the head of the stack.
	entry := 3*n - 1
	for i := n; i > 0; i-- {
		so, err := s.nipN(entry)
		if err != nil {
			return totalRotatedItemsLength, err
		}

		s.PushByteArray(so)
		totalRotatedItemsLength += len(so)
	}
	return totalRotatedItemsLength, nil
}

// SwapN swaps the top N items on the stack with those below them.
//
// Stack transformation:
// SwapN(1): [... x1 x2] -> [... x2 x1]
// SwapN(2): [... x1 x2 x3 x4] -> [... x3 x4 x1 x2]
func (s *stack) SwapN(n int32) error {
	if n < 1 {
		str := fmt.Sprintf("attempt to swap %d stack items", n)
		return scriptError(ErrInvalidStackOperation, str)
	}

	entry := 2*n - 1
	for i := n; i > 0; i-- {
		// Swap 2n-1th entry to top.
		so, err := s.nipN(entry)
		if err != nil {
			return err
		}

		s.PushByteArray(so)
	}
	return nil
}

// OverN copies N items N items back to the top of the stack.
//
// Stack transformation:
// OverN(1): [... x1 x2 x3] -> [... x1 x2 x3 x2]
// OverN(2): [... x1 x2 x3 x4] -> [... x1 x2 x3 x4 x1 x2]
func (s *stack) OverN(n int32) (int, error) {

	totalCopiedItemsLength := 0

	if n < 1 {
		str := fmt.Sprintf("attempt to perform over on %d stack items",
			n)
		return totalCopiedItemsLength, scriptError(ErrInvalidStackOperation, str)
	}

	// Copy 2n-1th entry to top of the stack.
	entry := 2*n - 1
	for ; n > 0; n-- {
		so, err := s.PeekByteArray(entry)
		if err != nil {
			return totalCopiedItemsLength, err
		}
		s.PushByteArray(so)
		totalCopiedItemsLength += len(so)
	}

	return totalCopiedItemsLength, nil
}

// PickN copies the item N items back in the stack to the top.
//
// Stack transformation:
// PickN(0): [x1 x2 x3] -> [x1 x2 x3 x3]
// PickN(1): [x1 x2 x3] -> [x1 x2 x3 x2]
// PickN(2): [x1 x2 x3] -> [x1 x2 x3 x1]
func (s *stack) PickN(n int32) (int, error) {
	so, err := s.PeekByteArray(n)
	if err != nil {
		return 0, err
	}
	s.PushByteArray(so)

	pickedItemLength := len(so)

	return pickedItemLength, nil
}

// RollN moves the item N items back in the stack to the top.
//
// Stack transformation:
// RollN(0): [x1 x2 x3] -> [x1 x2 x3]
// RollN(1): [x1 x2 x3] -> [x1 x3 x2]
// RollN(2): [x1 x2 x3] -> [x2 x3 x1]
func (s *stack) RollN(n int32) (int, error) {
	so, err := s.nipN(n)
	if err != nil {
		return 0, err
	}

	s.PushByteArray(so)

	rolledItemLength := len(so)

	return rolledItemLength, nil
}

// String returns the stack in a readable format.
func (s *stack) String() string {
	var result strings.Builder
	for _, stack := range s.stk {
		if len(stack) == 0 {
			result.WriteString("00000000  <empty>\n")
		}
		result.WriteString(hex.Dump(stack))
	}

	return result.String()
}
