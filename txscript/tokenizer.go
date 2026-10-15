// Copyright (c) 2019 The Decred developers
// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import "fmt"

// ScriptTokenizer provides a facility for easily and efficiently tokenizing
// scripts without allocating.  Each successive opcode is parsed with the Next
// function, which returns false when iteration is complete, either due to
// successfully tokenizing the entire script or encountering a parse error.  In
// the case of failure, the Err function may be used to obtain the specific
// parse error.
//
// Upon successfully parsing an opcode, the opcode and data associated with it
// may be obtained via the Opcode and Data functions, respectively.
type ScriptTokenizer struct {
	script []byte
	offset int
	op     *opcode
	data   []byte
	err    error
}

// MakeScriptTokenizer returns a new instance of a script tokenizer.  See the
// ScriptTokenizer type for more details.
func MakeScriptTokenizer(script []byte) ScriptTokenizer {
	return ScriptTokenizer{script: script}
}

// Done returns true when either all opcodes have been exhausted or a parse
// failure was encountered and therefore the state has an associated error.
func (t *ScriptTokenizer) Done() bool {
	return t.err != nil || t.offset >= len(t.script)
}

// Next attempts to parse the next opcode and returns whether or not it was
// successful.  It will not be successful if invoked when already at the end of
// the script, a parse failure is encountered, or an associated error already
// exists due to a previous parse failure.
//
// In the case of a true return, the parsed opcode and data can be obtained
// with the associated functions and the offset into the script will either
// point to the next opcode or the end of the script if the final opcode was
// parsed.
//
// In the case of a false return, the parsed opcode and data will be the last
// successfully parsed values (if any) and the offset into the script will
// either point to the failing opcode or the end of the script if the function
// was invoked when already at the end of the script.
//
// Invoking this function when already at the end of the script is not
// considered an error and will simply return false.
func (t *ScriptTokenizer) Next() bool {
	if t.Done() {
		return false
	}

	// The parsing below matches parseScript, including its error messages.
	script := t.script
	i := t.offset
	op := &opcodeArray[script[i]]
	switch {
	// No additional data.  Note that some of the opcodes, notably
	// OP_1NEGATE, OP_0, and OP_[1-16] represent the data themselves.
	case op.length == 1:
		t.offset++
		t.op = op
		t.data = nil
		return true

	// Data pushes of specific lengths -- OP_DATA_[1-75].
	case op.length > 1:
		if len(script[i:]) < op.length {
			str := fmt.Sprintf("opcode %s requires %d "+
				"bytes, but script only has %d remaining",
				op.name, op.length, len(script[i:]))
			t.err = scriptError(ErrMalformedPush, str)
			return false
		}

		t.offset += op.length
		t.op = op
		t.data = script[i+1 : i+op.length]
		return true

	// Data pushes with parsed lengths -- OP_PUSHDATA{1,2,4}.
	case op.length < 0:
		var l uint
		off := i + 1

		if len(script[off:]) < -op.length {
			str := fmt.Sprintf("opcode %s requires %d "+
				"bytes, but script only has %d remaining",
				op.name, -op.length, len(script[off:]))
			t.err = scriptError(ErrMalformedPush, str)
			return false
		}

		// Next -length bytes are little endian length of data.
		switch op.length {
		case -1:
			l = uint(script[off])
		case -2:
			l = ((uint(script[off+1]) << 8) |
				uint(script[off]))
		case -4:
			l = ((uint(script[off+3]) << 24) |
				(uint(script[off+2]) << 16) |
				(uint(script[off+1]) << 8) |
				uint(script[off]))
		default:
			str := fmt.Sprintf("invalid opcode length %d",
				op.length)
			t.err = scriptError(ErrMalformedPush, str)
			return false
		}

		// Move offset to beginning of the data.
		off += -op.length

		// Disallow entries that do not fit script or were
		// sign extended.
		if int(l) > len(script[off:]) || int(l) < 0 {
			str := fmt.Sprintf("opcode %s pushes %d bytes, "+
				"but script only has %d remaining",
				op.name, int(l), len(script[off:]))
			t.err = scriptError(ErrMalformedPush, str)
			return false
		}

		t.offset = off + int(l)
		t.op = op
		t.data = script[off : off+int(l)]
		return true
	}

	// No opcode has a length of zero.
	str := fmt.Sprintf("invalid opcode length %d", op.length)
	t.err = scriptError(ErrMalformedPush, str)
	return false
}

// Script returns the full script associated with the tokenizer.
func (t *ScriptTokenizer) Script() []byte {
	return t.script
}

// ByteIndex returns the current offset into the full script that will be
// parsed next and therefore also implies everything before it has already
// been parsed.
func (t *ScriptTokenizer) ByteIndex() int {
	return t.offset
}

// Opcode returns the current opcode associated with the tokenizer.
func (t *ScriptTokenizer) Opcode() byte {
	return t.op.value
}

// Data returns the data associated with the most recently successfully parsed
// opcode.
func (t *ScriptTokenizer) Data() []byte {
	return t.data
}

// Err returns any errors currently associated with the tokenizer.  This will
// only be non-nil in the case a parsing error was encountered.
func (t *ScriptTokenizer) Err() error {
	return t.err
}
