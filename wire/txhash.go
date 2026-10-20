// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"math"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// The functions below write the same bytes as the serializers in msgtx.go,
// straight into a DoubleHasher.

// WriteVarIntToHasher writes val to d as WriteVarInt encodes it.
func WriteVarIntToHasher(d *chainhash.DoubleHasher, val uint64) {
	switch {
	case val < 0xfd:
		_ = d.WriteByte(uint8(val))
	case val <= math.MaxUint16:
		_ = d.WriteByte(0xfd)
		_ = d.WriteByte(uint8(val))
		_ = d.WriteByte(uint8(val >> 8))
	case val <= math.MaxUint32:
		_ = d.WriteByte(0xfe)
		d.WriteUint32(uint32(val))
	default:
		_ = d.WriteByte(0xff)
		d.WriteUint64(val)
	}
}

// WriteVarBytesToHasher writes b to d as WriteVarBytes encodes it.
func WriteVarBytesToHasher(d *chainhash.DoubleHasher, b []byte) {
	WriteVarIntToHasher(d, uint64(len(b)))
	_, _ = d.Write(b)
}

// hashTokenPrefix writes the token prefix to d as TokenDataBuffer encodes it.
func hashTokenPrefix(d *chainhash.DoubleHasher, tokenData *TokenData) {
	_ = d.WriteByte(PREFIX_BYTE)
	_, _ = d.Write(tokenData.CategoryID[:])
	_ = d.WriteByte(tokenData.BitField)
	if tokenData.HasCommitmentLength() {
		WriteVarIntToHasher(d, uint64(len(tokenData.Commitment)))
		_, _ = d.Write(tokenData.Commitment)
	}
	if tokenData.HasAmount() {
		WriteVarIntToHasher(d, tokenData.Amount)
	}
}

// SerializeToHasher writes to to d as WriteTxOut serializes it.
func (to *TxOut) SerializeToHasher(d *chainhash.DoubleHasher) {
	d.WriteUint64(uint64(to.Value))
	if !to.TokenData.IsEmpty() {
		WriteVarIntToHasher(d, uint64(to.TokenData.serializeSize()+len(to.PkScript)))
		hashTokenPrefix(d, &to.TokenData)
	} else {
		WriteVarIntToHasher(d, uint64(len(to.PkScript)))
	}
	_, _ = d.Write(to.PkScript)
}

// hashTxIn writes ti to d as writeTxIn serializes it.
func hashTxIn(d *chainhash.DoubleHasher, ti *TxIn) {
	_, _ = d.Write(ti.PreviousOutPoint.Hash[:])
	d.WriteUint32(ti.PreviousOutPoint.Index)
	WriteVarBytesToHasher(d, ti.SignatureScript)
	d.WriteUint32(ti.Sequence)
}

// SerializeToHasher writes the transaction to d as Serialize does.
func (msg *MsgTx) SerializeToHasher(d *chainhash.DoubleHasher) {
	d.WriteUint32(uint32(msg.Version))
	WriteVarIntToHasher(d, uint64(len(msg.TxIn)))
	for _, ti := range msg.TxIn {
		hashTxIn(d, ti)
	}
	WriteVarIntToHasher(d, uint64(len(msg.TxOut)))
	for _, to := range msg.TxOut {
		to.SerializeToHasher(d)
	}
	d.WriteUint32(msg.LockTime)
}
