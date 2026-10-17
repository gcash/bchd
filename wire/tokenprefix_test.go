// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire_test

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// txWithOutputScript returns a serialized transaction with one output whose
// serialized script is the provided bytes.
func txWithOutputScript(t *testing.T, script []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{2, 0, 0, 0})
	buf.WriteByte(1)
	buf.Write(bytes.Repeat([]byte{0x01}, 32))
	buf.Write([]byte{0, 0, 0, 0})
	buf.WriteByte(0)
	buf.Write([]byte{0xff, 0xff, 0xff, 0xff})
	buf.WriteByte(1)
	buf.Write(make([]byte, 8))
	if err := wire.WriteVarBytes(&buf, 0, script); err != nil {
		t.Fatalf("WriteVarBytes: %v", err)
	}
	buf.Write([]byte{0, 0, 0, 0})
	return buf.Bytes()
}

// TestTokenPrefixParsing ensures output scripts are split into token data and
// a locking script only when they start with a valid token prefix, that
// invalid prefixes are kept whole with empty token data, and that every
// output re-serializes to the original bytes.
func TestTokenPrefixParsing(t *testing.T) {
	category := tokenCategory(0x01)
	var zeroCategory [32]byte
	lockingScript := []byte{0x51}
	prefix := func(category [32]byte, rest ...byte) []byte {
		return append(append([]byte{wire.PREFIX_BYTE}, category[:]...),
			rest...)
	}
	join := func(parts ...[]byte) []byte {
		return bytes.Join(parts, nil)
	}

	tests := []struct {
		name       string
		script     []byte
		wantToken  bool
		wantScript []byte
	}{
		{
			name:       "plain script",
			script:     lockingScript,
			wantScript: lockingScript,
		},
		{
			name:       "fungible token",
			script:     join(prefix(category, 0x10, 0x05), lockingScript),
			wantToken:  true,
			wantScript: lockingScript,
		},
		{
			name:       "token without a locking script",
			script:     prefix(category, 0x10, 0x05),
			wantToken:  true,
			wantScript: []byte{},
		},
		{
			name:       "zero category",
			script:     join(prefix(zeroCategory, 0x10, 0x05), lockingScript),
			wantToken:  true,
			wantScript: lockingScript,
		},
		{
			name: "locking script starting with the prefix byte",
			script: join(prefix(category, 0x10, 0x05),
				prefix(tokenCategory(0x02), 0x22), lockingScript),
			wantToken: true,
			wantScript: join(prefix(tokenCategory(0x02), 0x22),
				lockingScript),
		},
		{
			name:   "prefix byte only",
			script: []byte{wire.PREFIX_BYTE},
		},
		{
			name:   "prefix byte and a short script",
			script: []byte{wire.PREFIX_BYTE, 0x51},
		},
		{
			name:   "bitfield zero",
			script: join(prefix(category, 0x00), lockingScript),
		},
		{
			name:   "reserved bit",
			script: join(prefix(category, 0x90, 0x05), lockingScript),
		},
		{
			name:   "capability 3",
			script: join(prefix(category, 0x23), lockingScript),
		},
		{
			name:   "commitment without nft",
			script: join(prefix(category, 0x50, 0x01, 0xaa, 0x05), lockingScript),
		},
		{
			name:   "amount zero",
			script: join(prefix(category, 0x10, 0x00), lockingScript),
		},
		{
			name: "amount above the maximum",
			script: join(prefix(category, 0x10, 0xff, 0, 0, 0, 0, 0, 0,
				0, 0x80), lockingScript),
		},
		{
			name: "non-canonical amount",
			script: join(prefix(category, 0x10, 0xfd, 0x05, 0x00),
				lockingScript),
		},
		{
			name:   "commitment length zero",
			script: join(prefix(category, 0x60, 0x00), lockingScript),
		},
		{
			name:   "truncated commitment",
			script: prefix(category, 0x60, 0x05, 0x01),
		},
		{
			name:   "nft with commitment and amount, amount missing",
			script: prefix(category, 0x70, 0x01, 0xaa),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := txWithOutputScript(t, test.script)
			var tx wire.MsgTx
			if err := tx.Deserialize(bytes.NewReader(raw)); err != nil {
				t.Fatalf("Deserialize: %v", err)
			}
			txOut := tx.TxOut[0]
			if txOut.TokenData.IsEmpty() == test.wantToken {
				t.Fatalf("has token %v, want %v",
					!txOut.TokenData.IsEmpty(), test.wantToken)
			}
			wantScript := test.wantScript
			if !test.wantToken {
				wantScript = test.script
				if !bytes.Equal(txOut.TokenData.CategoryID[:],
					zeroCategory[:]) || txOut.TokenData.Commitment != nil ||
					txOut.TokenData.Amount != 0 {
					t.Fatalf("partial token data %+v", txOut.TokenData)
				}
			}
			if !bytes.Equal(txOut.PkScript, wantScript) {
				t.Fatalf("script %x, want %x", txOut.PkScript, wantScript)
			}

			var out bytes.Buffer
			if err := tx.Serialize(&out); err != nil {
				t.Fatalf("Serialize: %v", err)
			}
			if !bytes.Equal(out.Bytes(), raw) {
				t.Fatalf("re-serialized as %x, want %x", out.Bytes(), raw)
			}
			if tx.TxHash() != chainhash.DoubleHashH(raw) {
				t.Fatal("transaction hash differs from the hash of its bytes")
			}
			if tx.SerializeSize() != len(raw) {
				t.Fatalf("serialize size %d, want %d", tx.SerializeSize(),
					len(raw))
			}

			var tokenData wire.TokenData
			script, err := tokenData.SeparateTokenDataFromPKScriptIfExists(
				test.script, 0)
			isPrefixed := test.script[0] == wire.PREFIX_BYTE
			if (err != nil) != (isPrefixed && !test.wantToken) {
				t.Fatalf("unexpected error %v", err)
			}
			if err == nil && !bytes.Equal(script, wantScript) {
				t.Fatalf("separated script %x, want %x", script, wantScript)
			}
		})
	}
}

// TestMsgTxCopyTokenCommitment ensures copying a transaction does not share
// token commitments with the original.
func TestMsgTxCopyTokenCommitment(t *testing.T) {
	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxOut(wire.NewTxOut(1000, []byte{0x51}, wire.TokenData{
		CategoryID: tokenCategory(0x01),
		BitField:   wire.HAS_NFT | wire.HAS_COMMITMENT_LENGTH,
		Commitment: []byte{0xaa},
	}))
	txCopy := tx.Copy()
	txCopy.TxOut[0].TokenData.Commitment[0] = 0xbb
	if tx.TxOut[0].TokenData.Commitment[0] != 0xaa {
		t.Fatal("copy shares the token commitment with the original")
	}
}

// TestCashTokensInvalidTokenPrefix ensures the token validity algorithm
// rejects outputs, and spends of outputs, whose script starts with the token
// prefix byte without holding a valid token prefix.
func TestCashTokensInvalidTokenPrefix(t *testing.T) {
	invalid := []byte{wire.PREFIX_BYTE, 0x51}

	tx, cache := buildTokenTx([]tokenInput{{}}, []wire.TokenData{{}})
	if _, err := wire.RunCashTokensValidityAlgorithm(cache, tx, true); err != nil {
		t.Fatalf("plain transaction rejected: %v", err)
	}

	tx.TxOut[0].PkScript = invalid
	if _, err := wire.RunCashTokensValidityAlgorithm(cache, tx, true); err == nil {
		t.Fatal("output with an invalid token prefix accepted")
	}

	tx, cache = buildTokenTx([]tokenInput{{}}, []wire.TokenData{{}})
	cache.AddEntry(0, wire.TxOut{Value: 1000, PkScript: invalid})
	if _, err := wire.RunCashTokensValidityAlgorithm(cache, tx, true); err == nil {
		t.Fatal("spend of an output with an invalid token prefix accepted")
	}
}
