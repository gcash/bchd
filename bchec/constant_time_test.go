// Copyright (c) 2017 The btcsuite developers
// Copyright (c) 2017 Brent Perreault
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchec

import "testing"

func TestLessThanUint32(t *testing.T) {
	tests := []struct {
		x uint32
		y uint32
		a uint32
	}{
		{0, 1, 1},
		{2, 2, 0},
		{1 << 30, 1 << 30, 0},
		{17, 1 << 30, 1},
		{1 << 30, 0, 0},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		answer := lessThanUint32(test.x, test.y)
		if test.a != answer {
			t.Errorf("lessThanUint32 #%d wrong result\ngot: %v\n"+
				"want: %v", i, answer, test.a)
			continue
		}
	}
}

func TestIsZeroUint32(t *testing.T) {
	tests := []struct {
		x uint32
		a uint32
	}{
		{1, 0},
		{0, 1},
		{1 << 30, 0},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		answer := isZeroUint32(test.x)
		if test.a != answer {
			t.Errorf("isZeroUint32 #%d wrong result\ngot: %v\n"+
				"want: %v", i, answer, test.a)
			continue
		}
	}
}

func TestNotZeroUint32(t *testing.T) {
	tests := []struct {
		x uint32
		a uint32
	}{
		{1, 1},
		{0, 0},
		{1 << 30, 1},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		answer := notZeroUint32(test.x)
		if test.a != answer {
			t.Errorf("notZeroUint32 #%d wrong result\ngot: %v\n"+
				"want: %v", i, answer, test.a)
			continue
		}
	}
}
