// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bcmr

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// hintHash returns a hash that starts with b.
func hintHash(b byte) chainhash.Hash {
	return chainhash.Hash{b}
}

// testHints returns hints of a plain token, a burned one, and one whose
// publication isn't on its head.
func testHints() []Hint {
	return []Hint{
		{Category: hintHash(3), Head: hintHash(30), HeadHeight: 300},
		{Category: hintHash(1), Head: hintHash(10), HeadHeight: 100, Burned: true},
		{Category: hintHash(2), Head: hintHash(20), HeadHeight: 200,
			Publication: &HintPublication{Txid: hintHash(21), Height: 150}},
	}
}

// TestHintsRoundTrip ensures hints read back as they were, and that a category
// the hints don't have isn't found, before, between and after the others.
func TestHintsRoundTrip(t *testing.T) {
	hints, err := ParseHints(EncodeHints(1000, testHints()))
	if err != nil {
		t.Fatal(err)
	}
	if hints.Height != 1000 || hints.Len() != 3 {
		t.Fatalf("height %d, %d hints", hints.Height, hints.Len())
	}
	for _, want := range testHints() {
		got, ok := hints.Lookup(want.Category)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, %v, want %+v", got, ok, want)
		}
	}
	missing := chainhash.Hash{2, 1}
	for _, category := range []chainhash.Hash{{}, missing, hintHash(9)} {
		if hint, ok := hints.Lookup(category); ok {
			t.Fatalf("found %+v", hint)
		}
	}
}

// TestHintsEmpty ensures a file with no hints reads as one.
func TestHintsEmpty(t *testing.T) {
	hints, err := ParseHints(EncodeHints(5, nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := hints.Lookup(hintHash(1)); ok || hints.Len() != 0 {
		t.Fatalf("%d hints", hints.Len())
	}
}

// TestHintsDeterministic ensures the order of the hints doesn't change the
// file, as two nodes must produce the same bytes.
func TestHintsDeterministic(t *testing.T) {
	want := EncodeHints(1000, testHints())
	h := testHints()
	h[0], h[1], h[2] = h[2], h[0], h[1]
	if !bytes.Equal(EncodeHints(1000, h), want) {
		t.Fatal("the order of the hints changed the file")
	}
}

// TestHintsDamaged ensures a damaged file is refused rather than read wrongly.
func TestHintsDamaged(t *testing.T) {
	good := EncodeHints(1000, testHints())
	damage := func(f func(b []byte) []byte) []byte {
		return f(bytes.Clone(good))
	}
	tests := map[string][]byte{
		"empty":      nil,
		"magic":      damage(func(b []byte) []byte { b[0] ^= 1; return b }),
		"short":      damage(func(b []byte) []byte { return b[:len(b)-1] }),
		"long":       damage(func(b []byte) []byte { return append(b, 0) }),
		"count":      damage(func(b []byte) []byte { b[12] = 200; return b }),
		"huge count": damage(func(b []byte) []byte { b[15] = 255; return b }),
		"height":     damage(func(b []byte) []byte { b[11] = 255; return b }),
		"order": damage(func(b []byte) []byte {
			b[hintsHeaderSize] = 200
			return b
		}),
		"publication": damage(func(b []byte) []byte {
			b[hintsHeaderSize+hintEntrySize-4] = 9
			return b
		}),
	}
	for name, data := range tests {
		if _, err := ParseHints(data); !errors.Is(err, ErrBadHints) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
