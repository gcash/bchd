// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bcmr

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/gcash/bchd/wire"
)

// testHash is the hash in CHIP-BCMR's example publication output.
const testHash = "6fe28c0ab6f1b372c1a6a246ae63f74f931e8365e15a089c68d6190000000000"

// p2pkh is the locking bytecode of a pay-to-public-key-hash output.
const p2pkh = "76a914000000000000000000000000000000000000000088ac"

// txWithOutputs returns a transaction with an output for each hex-encoded
// locking bytecode in scripts.
func txWithOutputs(t *testing.T, scripts ...string) *wire.MsgTx {
	t.Helper()
	tx := wire.NewMsgTx(2)
	for _, script := range scripts {
		pkScript, err := hex.DecodeString(script)
		if err != nil {
			t.Fatalf("bad test script %q: %v", script, err)
		}
		tx.AddTxOut(wire.NewTxOut(0, pkScript, wire.TokenData{}))
	}
	return tx
}

// push returns the hex of a direct push of the hex-encoded data.
func push(data string) string {
	return hex.EncodeToString([]byte{byte(len(data) / 2)}) + data
}

// TestParsePublication checks that ParsePublication finds a
// transaction's publication output as CHIP-BCMR defines it, and tells a
// well-formed one from a malformed one.
func TestParsePublication(t *testing.T) {
	var chipHash [32]byte
	hashBytes, _ := hex.DecodeString(testHash)
	copy(chipHash[:], hashBytes)

	prefix := "6a0442434d52"
	exampleCom := push(hex.EncodeToString([]byte("example.com")))
	ipfs := push(hex.EncodeToString([]byte("ipfs://bafkreiexample")))
	root := push(hex.EncodeToString([]byte("test.example.com/")))

	tests := []struct {
		name    string
		scripts []string
		want    *Publication
	}{{
		// The locking bytecode is the one the CHIP gives.
		name: "the CHIP's example",
		scripts: []string{p2pkh,
			"6a0442434d52206fe28c0ab6f1b372c1a6a246ae63f74f931e83" +
				"65e15a089c68d61900000000000b6578616d706c652e636f6d"},
		want: &Publication{Output: 1, Hash: chipHash,
			URIs: []string{"example.com"}},
	}, {
		name:    "no publication",
		scripts: []string{p2pkh, "6a0454455354" + exampleCom},
	}, {
		name:    "a burn's identity output",
		scripts: []string{prefix + push(testHash) + exampleCom, p2pkh},
		want: &Publication{Output: 0, Hash: chipHash,
			URIs: []string{"example.com"}},
	}, {
		name:    "a hash without URIs",
		scripts: []string{p2pkh, prefix + push(testHash)},
		want:    &Publication{Output: 1, Hash: chipHash},
	}, {
		name:    "URIs in order",
		scripts: []string{p2pkh, prefix + push(testHash) + ipfs + exampleCom + root},
		want: &Publication{Output: 1, Hash: chipHash,
			URIs: []string{"ipfs://bafkreiexample", "example.com", "test.example.com/"}},
	}, {
		name:    "a hash pushed with OP_PUSHDATA1",
		scripts: []string{p2pkh, prefix + "4c20" + testHash + exampleCom},
		want: &Publication{Output: 1, Hash: chipHash,
			URIs: []string{"example.com"}},
	}, {
		name:    "a URI pushed with OP_PUSHDATA2",
		scripts: []string{p2pkh, prefix + push(testHash) + "4d0b00" + strings.TrimPrefix(exampleCom, "0b")},
		want: &Publication{Output: 1, Hash: chipHash,
			URIs: []string{"example.com"}},
	}, {
		name:    "BCMR pushed with OP_PUSHDATA1",
		scripts: []string{p2pkh, "6a4c0442434d52" + push(testHash) + exampleCom},
	}, {
		name:    "the prefix alone",
		scripts: []string{p2pkh, prefix},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "a 31-byte hash",
		scripts: []string{p2pkh, prefix + push(testHash[2:]) + exampleCom},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "a 33-byte hash",
		scripts: []string{p2pkh, prefix + push(testHash+"00") + exampleCom},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "a URI that isn't pushed",
		scripts: []string{p2pkh, prefix + push(testHash) + "76"},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "OP_RESERVED after the hash",
		scripts: []string{p2pkh, prefix + push(testHash) + "50"},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "a URI pushed with OP_1",
		scripts: []string{p2pkh, prefix + push(testHash) + "51"},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "a URI pushed with OP_1NEGATE",
		scripts: []string{p2pkh, prefix + push(testHash) + "4f"},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "an empty URI, pushed with OP_0",
		scripts: []string{p2pkh, prefix + push(testHash) + "00"},
		want: &Publication{Output: 1, Hash: chipHash,
			URIs: []string{""}},
	}, {
		name:    "a push cut short",
		scripts: []string{p2pkh, prefix + push(testHash) + "0b6578616d706c65"},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name:    "a URI that isn't UTF-8",
		scripts: []string{p2pkh, prefix + push(testHash) + "02c328"},
		want:    &Publication{Output: 1, Malformed: true},
	}, {
		name: "a malformed first publication",
		scripts: []string{p2pkh, prefix + push(testHash[2:]),
			prefix + push(testHash) + exampleCom},
		want: &Publication{Output: 1, Malformed: true},
	}, {
		name: "two publications",
		scripts: []string{prefix + push(testHash) + exampleCom,
			prefix + push(testHash) + ipfs},
		want: &Publication{Output: 0, Hash: chipHash,
			URIs: []string{"example.com"}},
	}, {
		name: "another data-carrier output first",
		scripts: []string{"6a0454455354", p2pkh,
			prefix + push(testHash) + exampleCom},
		want: &Publication{Output: 2, Hash: chipHash,
			URIs: []string{"example.com"}},
	}}

	for _, test := range tests {
		got := ParsePublication(txWithOutputs(t, test.scripts...))
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %+v, want %+v", test.name, got, test.want)
		}
	}
}

// TestWellFormed checks WellFormed on no publication, a malformed one and a
// well-formed one.
func TestWellFormed(t *testing.T) {
	var none *Publication
	if none.WellFormed() || (&Publication{Malformed: true}).WellFormed() ||
		!(&Publication{}).WellFormed() {

		t.Error("WellFormed is wrong")
	}
}
