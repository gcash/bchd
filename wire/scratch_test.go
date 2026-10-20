// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"testing"
)

// scratchTokenTx returns a transaction with scripts and token outputs with
// commitments and amounts of several varint sizes.
func scratchTokenTx(seed byte) *MsgTx {
	tx := NewMsgTx(2)
	for i := range 3 {
		in := NewTxIn(&OutPoint{Hash: [32]byte{seed, byte(i)}, Index: uint32(i)},
			bytes.Repeat([]byte{seed + byte(i)}, 5+i*100))
		tx.AddTxIn(in)
	}
	tokens := []TokenData{
		{CategoryID: [32]byte{seed}, BitField: HAS_AMOUNT, Amount: 0xfd},
		{CategoryID: [32]byte{seed, 1}, BitField: HAS_NFT |
			HAS_COMMITMENT_LENGTH | MUTABLE, Commitment: bytes.Repeat(
			[]byte{seed}, 40)},
		{CategoryID: [32]byte{seed, 2}, BitField: HAS_NFT | HAS_AMOUNT |
			HAS_COMMITMENT_LENGTH | MINTING, Commitment: []byte{1, 2},
			Amount: MAX_FT_AMOUNT},
		{},
	}
	for i, td := range tokens {
		tx.AddTxOut(NewTxOut(int64(i)*1000, bytes.Repeat(
			[]byte{0x51 + byte(i)}, 25+i*50), td))
	}
	return tx
}

// poison overwrites the whole of buf, up to its capacity.
func poison(buf []byte) {
	buf = buf[:cap(buf)]
	for i := range buf {
		buf[i] = 0xa5
	}
}

// TestScratchBufferNotAliased decodes a block and a transaction with an
// explicit scratch buffer, overwrites the buffer and checks that they still
// encode to their input, so no decoded field shares memory with the buffer.
func TestScratchBufferNotAliased(t *testing.T) {
	blk := &MsgBlock{Header: blockOne.Header}
	for i := range 4 {
		blk.Transactions = append(blk.Transactions, scratchTokenTx(byte(i)))
	}
	var want bytes.Buffer
	if err := blk.BchEncode(&want, ProtocolVersion, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, scratchSize)
	var gotBlk MsgBlock
	err := gotBlk.bchDecode(bytes.NewReader(want.Bytes()), ProtocolVersion, buf)
	if err != nil {
		t.Fatal(err)
	}
	poison(buf)
	var got bytes.Buffer
	if err := gotBlk.BchEncode(&got, ProtocolVersion, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("decoded block shares memory with the scratch buffer")
	}

	tx := scratchTokenTx(9)
	want.Reset()
	if err := tx.Serialize(&want); err != nil {
		t.Fatal(err)
	}
	buf = make([]byte, scratchSize)
	var gotTx MsgTx
	err = gotTx.bchDecode(bytes.NewReader(want.Bytes()), ProtocolVersion, buf)
	if err != nil {
		t.Fatal(err)
	}
	poison(buf)
	got.Reset()
	if err := gotTx.Serialize(&got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("decoded transaction shares memory with the scratch buffer")
	}
}

// TestBlockHeaderPartialRead checks that a header cut short at every length
// is left as readElements leaves it, field by field.
func TestBlockHeaderPartialRead(t *testing.T) {
	var full bytes.Buffer
	if err := writeBlockHeader(&full, 0, &blockOne.Header); err != nil {
		t.Fatal(err)
	}
	for n := range blockHeaderLen {
		data := full.Bytes()[:n]
		var got, want BlockHeader
		gotErr := readBlockHeader(bytes.NewReader(data), 0, &got)
		wantErr := readElements(bytes.NewReader(data), &want.Version,
			&want.PrevBlock, &want.MerkleRoot,
			(*uint32Time)(&want.Timestamp), &want.Bits, &want.Nonce)
		if gotErr != wantErr {
			t.Fatalf("%d bytes: error %v, want %v", n, gotErr, wantErr)
		}
		if got.Version != want.Version || got.PrevBlock != want.PrevBlock ||
			got.MerkleRoot != want.MerkleRoot || got.Bits != want.Bits ||
			got.Nonce != want.Nonce ||
			!got.Timestamp.Equal(want.Timestamp) {
			t.Fatalf("%d bytes: header %+v, want %+v", n, got, want)
		}
	}
}
