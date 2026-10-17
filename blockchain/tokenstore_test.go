// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// tokenTestPrefix returns a serialized token prefix with the given category,
// bitfield and remaining token fields.
func tokenTestPrefix(category byte, rest ...byte) []byte {
	prefix := append([]byte{wire.PREFIX_BYTE}, bytes.Repeat([]byte{category}, 32)...)
	return append(prefix, rest...)
}

// requireTokenData fails the test unless got matches want.
func requireTokenData(t *testing.T, got, want wire.TokenData) {
	t.Helper()
	if got.CategoryID != want.CategoryID || got.BitField != want.BitField ||
		got.Amount != want.Amount || !bytes.Equal(got.Commitment, want.Commitment) {
		t.Fatalf("token data %+v, want %+v", got, want)
	}
}

// TestUtxoEntryTokenStorage ensures utxo entries and spent outputs keep their
// token data and script through serialization, including scripts which start
// with the token prefix byte.
func TestUtxoEntryTokenStorage(t *testing.T) {
	var category [32]byte
	category[0] = 0x09
	ft := wire.TokenData{CategoryID: category, BitField: wire.HAS_AMOUNT, Amount: 5}
	nft := wire.TokenData{CategoryID: category, BitField: wire.HAS_NFT |
		wire.HAS_COMMITMENT_LENGTH | wire.MINTING, Commitment: []byte{0xab}}
	prefixedScript := append(tokenTestPrefix(0x02, 0x22), txscript.OP_TRUE)
	invalidPrefix := append(tokenTestPrefix(0x02, 0x60, 0x00), txscript.OP_TRUE)

	tests := []struct {
		name      string
		tokenData wire.TokenData
		pkScript  []byte
	}{
		{name: "plain script", pkScript: opTrueScript},
		{name: "token", tokenData: ft, pkScript: opTrueScript},
		{name: "token without script", tokenData: nft, pkScript: []byte{}},
		{name: "token with prefixed script", tokenData: ft, pkScript: prefixedScript},
		{name: "invalid prefix", pkScript: invalidPrefix},
		{name: "prefix byte only", pkScript: []byte{wire.PREFIX_BYTE}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := &UtxoEntry{
				amount:      1000,
				pkScript:    test.pkScript,
				tokenData:   test.tokenData,
				blockHeight: 100,
				packedFlags: tfModified,
			}
			serialized, err := serializeUtxoEntry(entry)
			if err != nil {
				t.Fatalf("serializeUtxoEntry: %v", err)
			}
			got, err := DeserializeUtxoEntry(serialized)
			if err != nil {
				t.Fatalf("DeserializeUtxoEntry: %v", err)
			}
			requireTokenData(t, got.TokenData(), test.tokenData)
			if !bytes.Equal(got.pkScript, test.pkScript) {
				t.Fatalf("script %x, want %x", got.pkScript, test.pkScript)
			}

			// Spend the entry and restore it from the spend journal.
			tx := wire.NewMsgTx(1)
			tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
			view := NewUtxoViewpoint()
			view.entries[tx.TxIn[0].PreviousOutPoint] = entry
			var stxos []SpentTxOut
			if err := spendTransactionInputs(view, bchutil.NewTx(tx), &stxos); err != nil {
				t.Fatalf("spendTransactionInputs: %v", err)
			}
			journal := serializeSpendJournalEntry(stxos)
			if _, err := decodeSpentTxOut(journal, &stxos[0]); err != nil {
				t.Fatalf("decodeSpentTxOut: %v", err)
			}
			block := bchutil.NewBlock(&wire.MsgBlock{
				Transactions: []*wire.MsgTx{wire.NewMsgTx(1), tx},
			})
			view = NewUtxoViewpoint()
			if err := disconnectTransactions(view, block, stxos); err != nil {
				t.Fatalf("disconnectTransactions: %v", err)
			}
			restored := view.LookupEntry(tx.TxIn[0].PreviousOutPoint)
			requireTokenData(t, restored.TokenData(), test.tokenData)
			if !bytes.Equal(restored.pkScript, test.pkScript) {
				t.Fatalf("restored script %x, want %x", restored.pkScript,
					test.pkScript)
			}
		})
	}
}
