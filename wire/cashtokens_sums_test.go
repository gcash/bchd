// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire_test

import (
	"testing"

	"github.com/gcash/bchd/wire"
)

// TestCashTokensFungibleSumBounds ensures the fungible token amounts of a
// category are summed without wrapping.  Each amount comes from the
// transaction and may be as large as MAX_FT_AMOUNT, so a sum over several
// outputs can exceed the range of a uint64.  A sum which wrapped around would
// appear small enough to be covered by the inputs or to be created by a
// genesis category, so any sum above MAX_FT_AMOUNT must be rejected.
func TestCashTokensFungibleSumBounds(t *testing.T) {
	const maxAmount = wire.MAX_FT_AMOUNT
	category := tokenCategory(0x33)
	second := tokenCategory(0x44)
	genesis := genesisCategoryOfInput(0)

	tests := []struct {
		name    string
		inputs  []tokenInput
		outputs []wire.TokenData
		valid   bool
	}{
		{
			// The output sum is 2^64 + 1, which wraps to 1 and would
			// otherwise match the single unit available.
			name:   "output sum wrapping to a covered amount is rejected",
			inputs: []tokenInput{{tokenData: fungibleToken(category, 1)}},
			outputs: []wire.TokenData{
				fungibleToken(category, maxAmount),
				fungibleToken(category, maxAmount),
				fungibleToken(category, 3),
			},
			valid: false,
		},
		{
			// The output sum is exactly 2^64, which wraps to zero.
			name:   "output sum wrapping to zero is rejected",
			inputs: []tokenInput{{tokenData: fungibleToken(category, 1)}},
			outputs: []wire.TokenData{
				fungibleToken(category, maxAmount),
				fungibleToken(category, maxAmount),
				fungibleToken(category, 2),
			},
			valid: false,
		},
		{
			// The output sum is 3 * MAX_FT_AMOUNT, which wraps to a value
			// below MAX_FT_AMOUNT.
			name:   "genesis output sum wrapping below the maximum is rejected",
			inputs: []tokenInput{{isGenesis: true}},
			outputs: []wire.TokenData{
				fungibleToken(genesis, maxAmount),
				fungibleToken(genesis, maxAmount),
				fungibleToken(genesis, maxAmount),
			},
			valid: false,
		},
		{
			name:   "genesis output sum above the maximum is rejected",
			inputs: []tokenInput{{isGenesis: true}},
			outputs: []wire.TokenData{
				fungibleToken(genesis, maxAmount),
				fungibleToken(genesis, maxAmount),
			},
			valid: false,
		},
		{
			// The input sum is 2^64 + 1, which wraps to 1.
			name: "input sum above the maximum is rejected",
			inputs: []tokenInput{
				{tokenData: fungibleToken(category, maxAmount)},
				{tokenData: fungibleToken(category, maxAmount)},
				{tokenData: fungibleToken(category, 3)},
			},
			outputs: []wire.TokenData{fungibleToken(category, 1)},
			valid:   false,
		},
		{
			// Each category keeps its own sum, so a wrapped sum in one
			// category must be rejected even when another is balanced.
			name: "sum above the maximum in a second category is rejected",
			inputs: []tokenInput{
				{tokenData: fungibleToken(category, 5)},
				{tokenData: fungibleToken(second, 1)},
			},
			outputs: []wire.TokenData{
				fungibleToken(category, 5),
				fungibleToken(second, maxAmount),
				fungibleToken(second, maxAmount),
				fungibleToken(second, 3),
			},
			valid: false,
		},
		{
			name: "input sum of exactly the maximum is accepted",
			inputs: []tokenInput{
				{tokenData: fungibleToken(category, maxAmount-1)},
				{tokenData: fungibleToken(category, 1)},
			},
			outputs: []wire.TokenData{fungibleToken(category, maxAmount)},
			valid:   true,
		},
		{
			name:    "genesis output of the maximum amount is accepted",
			inputs:  []tokenInput{{isGenesis: true}},
			outputs: []wire.TokenData{fungibleToken(genesis, maxAmount)},
			valid:   true,
		},
		{
			name: "amounts conserved across several outputs are accepted",
			inputs: []tokenInput{
				{tokenData: fungibleToken(category, maxAmount-10)},
				{tokenData: fungibleToken(category, 10)},
			},
			outputs: []wire.TokenData{
				fungibleToken(category, maxAmount-100),
				fungibleToken(category, 60),
				fungibleToken(category, 40),
			},
			valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx, cache := buildTokenTx(test.inputs, test.outputs)
			valid, err := wire.RunCashTokensValidityAlgorithm(cache, tx, false)

			if valid != test.valid {
				t.Fatalf("unexpected validity - got %v, want %v (error %v)",
					valid, test.valid, err)
			}
			if test.valid && err != nil {
				t.Fatalf("unexpected error for a valid transaction: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected a validation error for an invalid transaction")
			}
		})
	}
}
