// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"io"
	"testing"
)

// benchTokenTxOut returns an output with a non-fungible token, a fungible
// amount and a pay-to-pubkey-hash locking script.
func benchTokenTxOut() *TxOut {
	return NewTxOut(1000, append([]byte{0x76, 0xa9, 0x14},
		append(make([]byte, 20), 0x88, 0xac)...), TokenData{
		CategoryID: [32]byte{0x01},
		BitField:   HAS_AMOUNT | HAS_NFT | HAS_COMMITMENT_LENGTH,
		Commitment: bytes.Repeat([]byte{0xaa}, 40),
		Amount:     1000000,
	})
}

// BenchmarkTxOutSerializeSizeToken benchmarks computing the serialized size
// of an output with a token.
func BenchmarkTxOutSerializeSizeToken(b *testing.B) {
	txOut := benchTokenTxOut()
	b.ReportAllocs()
	for b.Loop() {
		txOut.SerializeSize()
	}
}

// BenchmarkWriteTxOutToken benchmarks serializing an output with a token.
func BenchmarkWriteTxOutToken(b *testing.B) {
	txOut := benchTokenTxOut()
	b.ReportAllocs()
	for b.Loop() {
		_ = WriteTxOut(io.Discard, 0, 0, txOut)
	}
}

// TestTxOutSerializeSizeToken ensures the serialized size of an output with a
// token is computed without allocating and matches its serialization.
func TestTxOutSerializeSizeToken(t *testing.T) {
	txOut := benchTokenTxOut()
	var buf bytes.Buffer
	if err := WriteTxOut(&buf, 0, 0, txOut); err != nil {
		t.Fatalf("WriteTxOut: %v", err)
	}
	if txOut.SerializeSize() != buf.Len() {
		t.Fatalf("serialize size %d, want %d", txOut.SerializeSize(),
			buf.Len())
	}
	allocs := testing.AllocsPerRun(100, func() { txOut.SerializeSize() })
	if allocs != 0 {
		t.Fatalf("SerializeSize allocated %v times", allocs)
	}
}
