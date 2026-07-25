package wire_test

import (
	"testing"

	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// tokenCategory returns a distinct category ID for the provided seed.  The seed
// must not be zero since a zero category ID marks token data as empty.
func tokenCategory(seed byte) [32]byte {
	var category [32]byte
	category[0] = seed
	return category
}

// genesisCategoryOfInput returns the category which the input at the provided
// index is permitted to create when it spends output index 0.
func genesisCategoryOfInput(i int) [32]byte {
	return tokenCategory(byte(0xa0 + i))
}

func mintingNFT(category [32]byte) wire.TokenData {
	return wire.TokenData{CategoryID: category, BitField: wire.HAS_NFT | wire.MINTING}
}

func fungibleToken(category [32]byte, amount uint64) wire.TokenData {
	return wire.TokenData{CategoryID: category, BitField: wire.HAS_AMOUNT, Amount: amount}
}

// tokenInput describes an input along with the token data of the previous
// output it spends.  When isGenesis is set the input spends output index 0,
// which makes its outpoint hash a genesis category permitted to mint.
type tokenInput struct {
	tokenData wire.TokenData
	isGenesis bool
}

// buildTokenTx assembles a transaction along with the utxo cache describing the
// previous outputs its inputs spend.
func buildTokenTx(inputs []tokenInput, outputs []wire.TokenData) (*wire.MsgTx, *txscript.UtxoCache) {
	tx := wire.NewMsgTx(wire.TxVersion)
	cache := txscript.NewUtxoCache()

	for i, input := range inputs {
		index := uint32(1)
		if input.isGenesis {
			index = 0
		}
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  genesisCategoryOfInput(i),
				Index: index,
			},
		})
		cache.AddEntry(i, wire.TxOut{
			Value:     1000,
			PkScript:  []byte{0x51},
			TokenData: input.tokenData,
		})
	}

	for _, output := range outputs {
		tx.AddTxOut(&wire.TxOut{
			Value:     500,
			PkScript:  []byte{0x51},
			TokenData: output,
		})
	}

	return tx, cache
}

// TestCashTokensMintingAuthorizationPerCategory ensures every minting NFT
// created by a transaction is authorized for its own category.  A transaction
// which mints one authorized category must not thereby become authorized to
// mint any other category.
func TestCashTokensMintingAuthorizationPerCategory(t *testing.T) {
	owned := tokenCategory(0x11)
	other := tokenCategory(0x22)
	victim := tokenCategory(0x99)

	// The attacker holds a minting NFT for a category it created plus a single
	// fungible unit of the victim category.  Holding fungible tokens of a
	// category confers no right to mint that category.  The fungible unit is
	// what allows the forged output to clear the output sum check, so the
	// minting authorization check is the only rule standing in the way.
	attackerInputs := []tokenInput{
		{tokenData: mintingNFT(owned)},
		{tokenData: fungibleToken(victim, 1)},
	}

	tests := []struct {
		name    string
		inputs  []tokenInput
		outputs []wire.TokenData
		valid   bool
	}{
		{
			name:    "forged minting NFT alone is rejected",
			inputs:  attackerInputs,
			outputs: []wire.TokenData{mintingNFT(victim)},
			valid:   false,
		},
		{
			// Regression test.  The authorized output must not clear the
			// authorization requirement for the forged one.
			name:    "forged minting NFT paired with an authorized one is rejected",
			inputs:  attackerInputs,
			outputs: []wire.TokenData{mintingNFT(owned), mintingNFT(victim)},
			valid:   false,
		},
		{
			name:    "authorized minting NFT is accepted",
			inputs:  []tokenInput{{tokenData: mintingNFT(owned)}},
			outputs: []wire.TokenData{mintingNFT(owned)},
			valid:   true,
		},
		{
			// Guards against over-tightening: several minting categories in
			// one transaction remain valid so long as each is authorized.
			name: "multiple authorized minting categories are accepted",
			inputs: []tokenInput{
				{tokenData: mintingNFT(owned)},
				{tokenData: mintingNFT(other)},
			},
			outputs: []wire.TokenData{mintingNFT(owned), mintingNFT(other)},
			valid:   true,
		},
		{
			name:    "minting NFT authorized by a genesis category is accepted",
			inputs:  []tokenInput{{isGenesis: true}},
			outputs: []wire.TokenData{mintingNFT(genesisCategoryOfInput(0))},
			valid:   true,
		},
		{
			name:    "transaction without minting outputs is unaffected",
			inputs:  []tokenInput{{tokenData: fungibleToken(owned, 5)}},
			outputs: []wire.TokenData{fungibleToken(owned, 5)},
			valid:   true,
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
