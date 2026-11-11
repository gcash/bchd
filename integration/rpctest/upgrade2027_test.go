// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpctest

import (
	"slices"
	"testing"
)

// TestUpgrade2027DefaultArg checks that a harness passes the never activating
// time unless the test sets its own.
func TestUpgrade2027DefaultArg(t *testing.T) {
	never := "--upgrade2027activationtime=281474976710655"
	if got := Upgrade2027Arg(Upgrade2027Never); got != never {
		t.Fatalf("Upgrade2027Arg(Upgrade2027Never) = %q, want %q", got,
			never)
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{{
		name: "no args",
		args: nil,
		want: []string{never},
	}, {
		name: "other args",
		args: []string{"--regtest"},
		want: []string{"--regtest", never},
	}, {
		name: "activation in the past",
		args: []string{Upgrade2027Arg(Upgrade2027InThePast), "--regtest"},
		want: []string{"--upgrade2027activationtime=1605451779",
			"--regtest"},
	}, {
		name: "similar option",
		args: []string{"--upgrade2027activationtimes=1"},
		want: []string{"--upgrade2027activationtimes=1", never},
	}}
	for _, test := range tests {
		got := withUpgrade2027Default(slices.Clone(test.args))
		if !slices.Equal(got, test.want) {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}

// TestUpgrade2027ArgTime checks that the harness reads the activation time
// bchd was started with.
func TestUpgrade2027ArgTime(t *testing.T) {
	tests := []struct {
		args []string
		want uint64
	}{
		{nil, Upgrade2027Never},
		{withUpgrade2027Default([]string{"--regtest"}), Upgrade2027Never},
		{[]string{Upgrade2027Arg(Upgrade2027InThePast)}, 1605451779},
		{[]string{"--upgrade2027activationtime", "1700000000"}, 1700000000},
		{[]string{"--upgrade2027activationtimes=1"}, Upgrade2027Never},
		{[]string{Upgrade2027Arg(5), Upgrade2027Arg(7)}, 7},
	}
	for _, test := range tests {
		if got := upgrade2027ArgTime(test.args); got != test.want {
			t.Errorf("%q: got %d, want %d", test.args, got, test.want)
		}
	}
}
