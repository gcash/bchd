// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bcmr

import (
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// Genesis is a token category that a transaction creates.
type Genesis struct {
	// Category is the category ID, the hash of the transaction whose
	// identity output the genesis spends, which is the authbase of the
	// category's own authchain.
	Category chainhash.Hash

	// Input is the input that spends the authbase's identity output.
	Input int

	// Supply is the number of fungible tokens the transaction creates of
	// the category, its genesis supply.
	Supply uint64
}

// Geneses returns the token categories a transaction creates, in the order
// of the outputs that first carry them.  A transaction creates a category when
// an output carries it and an input spends output 0 of the transaction whose
// ID is the category ID, the category's authbase.  That spend is also the
// first link of the authchain after the authbase.  Token data only counts in
// a block after CashTokens activated, so the caller passes no transaction of
// an earlier block.  It takes time in proportion to the transaction's inputs
// and outputs.
func Geneses(tx *wire.MsgTx) []Genesis {
	var geneses []Genesis

	// spenders holds the input that spends output 0 of each transaction,
	// and found where each category is in geneses, from the first output
	// that carries tokens on.
	var spenders, found map[chainhash.Hash]int
	for _, out := range tx.TxOut {
		if out.TokenData.IsEmpty() {
			continue
		}
		if spenders == nil {
			spenders = make(map[chainhash.Hash]int)
			for j, in := range tx.TxIn {
				if in.PreviousOutPoint.Index != 0 {
					continue
				}
				if _, ok := spenders[in.PreviousOutPoint.Hash]; !ok {
					spenders[in.PreviousOutPoint.Hash] = j
				}
			}
			found = make(map[chainhash.Hash]int)
		}
		category := chainhash.Hash(out.TokenData.CategoryID)
		k, ok := found[category]
		if !ok {
			input, ok := spenders[category]
			if !ok {
				continue
			}
			geneses = append(geneses, Genesis{Category: category, Input: input})
			k = len(geneses) - 1
			found[category] = k
		}
		if out.TokenData.HasAmount() {
			geneses[k].Supply += out.TokenData.Amount
		}
	}
	return geneses
}

// FindGenesis returns the genesis of the category among those of the
// transaction, if the transaction creates it.
func FindGenesis(tx *wire.MsgTx, category chainhash.Hash) (Genesis, bool) {
	for _, g := range Geneses(tx) {
		if g.Category == category {
			return g, true
		}
	}
	return Genesis{}, false
}

// IsBurn returns whether a transaction that spends an identity output burns
// the identity: its own identity output, output 0, is a data-carrier output,
// which ends the authchain.
func IsBurn(tx *wire.MsgTx) bool {
	if len(tx.TxOut) == 0 {
		return false
	}
	script := tx.TxOut[0].PkScript
	return len(script) > 0 && script[0] == txscript.OP_RETURN
}
