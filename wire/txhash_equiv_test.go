// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// The functions below are copies of the serialization, sizing and hashing
// code from before transactions were hashed field by field and token prefixes
// were sized without serializing them.  They only use encoding/binary and
// bytes, so they don't change with the code they are compared to.

// prevWriteVarInt writes val to b as a bitcoin variable length integer.
func prevWriteVarInt(b *bytes.Buffer, val uint64) {
	var buf [8]byte
	switch {
	case val < 0xfd:
		b.WriteByte(uint8(val))
	case val <= 0xffff:
		b.WriteByte(0xfd)
		binary.LittleEndian.PutUint16(buf[:], uint16(val))
		b.Write(buf[:2])
	case val <= 0xffffffff:
		b.WriteByte(0xfe)
		binary.LittleEndian.PutUint32(buf[:], uint32(val))
		b.Write(buf[:4])
	default:
		b.WriteByte(0xff)
		binary.LittleEndian.PutUint64(buf[:], val)
		b.Write(buf[:])
	}
}

// prevWriteUint32 writes v to b in little-endian order.
func prevWriteUint32(b *bytes.Buffer, v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	b.Write(buf[:])
}

// prevTokenDataBuffer returns the serialized token prefix.
func prevTokenDataBuffer(tokenData *TokenData) bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte(0xef)
	buf.Write(tokenData.CategoryID[:])
	buf.WriteByte(tokenData.BitField)
	if tokenData.HasCommitmentLength() {
		prevWriteVarInt(&buf, uint64(len(tokenData.Commitment)))
		buf.Write(tokenData.Commitment[:])
	}
	if tokenData.HasAmount() {
		prevWriteVarInt(&buf, tokenData.Amount)
	}
	return buf
}

// prevTxOutSerializeSize returns the number of bytes it would take to
// serialize the output.
func prevTxOutSerializeSize(t *TxOut) int {
	if t.TokenData.IsEmpty() {
		return 8 + VarIntSerializeSize(uint64(len(t.PkScript))) + len(t.PkScript)
	}
	b := prevTokenDataBuffer(&t.TokenData)
	PkScriptAndTDataSize := len(b.Bytes()) + len(t.PkScript)
	return 8 + VarIntSerializeSize(uint64(PkScriptAndTDataSize)) + PkScriptAndTDataSize
}

// prevWriteTxOut writes the output to b.
func prevWriteTxOut(b *bytes.Buffer, to *TxOut) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(to.Value))
	b.Write(buf[:])
	script := to.PkScript
	if !to.TokenData.IsEmpty() {
		tb := prevTokenDataBuffer(&to.TokenData)
		tb.Write(to.PkScript)
		script = tb.Bytes()
	}
	prevWriteVarInt(b, uint64(len(script)))
	b.Write(script)
}

// prevSerializeTx returns the serialized transaction.
func prevSerializeTx(msg *MsgTx) []byte {
	var b bytes.Buffer
	prevWriteUint32(&b, uint32(msg.Version))
	prevWriteVarInt(&b, uint64(len(msg.TxIn)))
	for _, ti := range msg.TxIn {
		b.Write(ti.PreviousOutPoint.Hash[:])
		prevWriteUint32(&b, ti.PreviousOutPoint.Index)
		prevWriteVarInt(&b, uint64(len(ti.SignatureScript)))
		b.Write(ti.SignatureScript)
		prevWriteUint32(&b, ti.Sequence)
	}
	prevWriteVarInt(&b, uint64(len(msg.TxOut)))
	for _, to := range msg.TxOut {
		prevWriteTxOut(&b, to)
	}
	prevWriteUint32(&b, msg.LockTime)
	return b.Bytes()
}

// prevBlockHash returns the hash of the serialized block header.
func prevBlockHash(h *BlockHeader) chainhash.Hash {
	var b bytes.Buffer
	prevWriteUint32(&b, uint32(h.Version))
	b.Write(h.PrevBlock[:])
	b.Write(h.MerkleRoot[:])
	prevWriteUint32(&b, uint32(h.Timestamp.Unix()))
	prevWriteUint32(&b, h.Bits)
	prevWriteUint32(&b, h.Nonce)
	return chainhash.DoubleHashH(b.Bytes())
}

// checkTxMatchesPrev checks the hash, size and serialization of tx and of
// each of its outputs against the copies above.
func checkTxMatchesPrev(t *testing.T, name string, tx *MsgTx) {
	t.Helper()
	want := prevSerializeTx(tx)
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("%s: serialized %x, want %x", name, buf.Bytes(), want)
	}
	if got := tx.SerializeSize(); got != len(want) {
		t.Fatalf("%s: SerializeSize %d, want %d", name, got, len(want))
	}
	if got, want := tx.TxHash(), chainhash.DoubleHashH(want); got != want {
		t.Fatalf("%s: TxHash %v, want %v", name, got, want)
	}
	for i, to := range tx.TxOut {
		var prev bytes.Buffer
		prevWriteTxOut(&prev, to)
		buf.Reset()
		if err := WriteTxOut(&buf, 0, 0, to); err != nil {
			t.Fatalf("%s out %d: %v", name, i, err)
		}
		if !bytes.Equal(buf.Bytes(), prev.Bytes()) {
			t.Fatalf("%s out %d: WriteTxOut %x, want %x", name, i,
				buf.Bytes(), prev.Bytes())
		}
		if got, want := to.SerializeSize(), prevTxOutSerializeSize(to); got != want {
			t.Fatalf("%s out %d: SerializeSize %d, want %d", name, i, got, want)
		}
		if !to.TokenData.IsEmpty() {
			got := to.TokenData.TokenDataBuffer()
			want := prevTokenDataBuffer(&to.TokenData)
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatalf("%s out %d: TokenDataBuffer %x, want %x", name, i,
					got.Bytes(), want.Bytes())
			}
		}
	}
}

// boundaryTxs returns transactions with input and output counts, script
// lengths, commitment lengths and token amounts at the varint boundaries.
func boundaryTxs() []*MsgTx {
	counts := &MsgTx{Version: 2}
	for i := range 0xfd {
		counts.TxIn = append(counts.TxIn,
			&TxIn{PreviousOutPoint: OutPoint{Index: uint32(i)}})
		counts.TxOut = append(counts.TxOut, &TxOut{Value: int64(i)})
	}

	scripts := &MsgTx{Version: 1}
	for _, n := range []int{0, 1, 0xfc, 0xfd, 0xfe, 0xffff, 0x10000} {
		scripts.TxIn = append(scripts.TxIn,
			&TxIn{SignatureScript: make([]byte, n)})
		scripts.TxOut = append(scripts.TxOut, &TxOut{PkScript: make([]byte, n)})
	}

	tokens := &MsgTx{Version: 2}
	commitments := [][]byte{nil, {}, {1}, make([]byte, MAX_COMMITMENT_LENGTH)}
	amounts := []uint64{0, 1, 0xfc, 0xfd, 0xffff, 0x10000, 0xffffffff,
		0x100000000, MAX_FT_AMOUNT}
	for _, commitment := range commitments {
		for _, amount := range amounts {
			td := TokenData{BitField: HAS_NFT, Commitment: commitment,
				Amount: amount}
			td.CategoryID[0] = 1
			if commitment != nil {
				td.BitField |= HAS_COMMITMENT_LENGTH
			}
			if amount != 0 {
				td.BitField |= HAS_AMOUNT
				if commitment == nil {
					ft := td
					ft.BitField = HAS_AMOUNT
					tokens.TxOut = append(tokens.TxOut,
						&TxOut{PkScript: []byte{0x51}, TokenData: ft})
				}
			}
			tokens.TxOut = append(tokens.TxOut,
				&TxOut{PkScript: []byte{0x51}, TokenData: td})
		}
	}
	return []*MsgTx{counts, scripts, tokens}
}

// TestTxHashMatchesPrevious checks TxHash, SerializeSize, Serialize and
// WriteTxOut against copies of the code they replaced, for transactions at
// the varint boundaries, random transactions with every token prefix form,
// and BlockHash for random headers.
func TestTxHashMatchesPrevious(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(3))
	txs := []*MsgTx{&genesisCoinbaseTx, blockOne.Transactions[0], NewMsgTx(1),
		multiTx}
	txs = append(txs, boundaryTxs()...)
	for range 300 {
		txs = append(txs, randHashTx(rng))
	}
	for i, tx := range txs {
		checkTxMatchesPrev(t, fmt.Sprintf("tx %d", i), tx)
	}

	headers := []BlockHeader{blockOne.Header, {}}
	for range 1000 {
		h := BlockHeader{
			Version:   rng.Int31() - rng.Int31(),
			Timestamp: time.Unix(rng.Int63n(1<<33)-(1<<31), 0),
			Bits:      rng.Uint32(),
			Nonce:     rng.Uint32(),
		}
		rng.Read(h.PrevBlock[:])
		rng.Read(h.MerkleRoot[:])
		headers = append(headers, h)
	}
	for i := range headers {
		if got, want := headers[i].BlockHash(), prevBlockHash(&headers[i]); got != want {
			t.Fatalf("header %d: BlockHash %v, want %v", i, got, want)
		}
	}
}

// FuzzTxHashMatchesPrevious checks every transaction that decodes against the
// copies of the previous code, and the hash of the first 80 bytes as a block
// header.
func FuzzTxHashMatchesPrevious(f *testing.F) {
	rng := rand.New(rand.NewSource(4))
	for range 20 {
		f.Add(prevSerializeTx(randHashTx(rng)))
	}
	f.Add(prevSerializeTx(multiTx))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) >= blockHeaderLen {
			var h BlockHeader
			err := h.Deserialize(bytes.NewReader(data[:blockHeaderLen]))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := h.BlockHash(), prevBlockHash(&h); got != want {
				t.Fatalf("BlockHash %v, want %v", got, want)
			}
		}
		var tx MsgTx
		if tx.Deserialize(bytes.NewReader(data)) != nil {
			return
		}
		checkTxMatchesPrev(t, "fuzz", &tx)
	})
}
