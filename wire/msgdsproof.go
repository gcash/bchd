// Copyright (c) 2020-2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// MaxDSProofPayload is the maximum number of bytes a dsproof message can be.
// It matches the limit other nodes apply to all messages other than blocks.
const MaxDSProofPayload = 2 * 1024 * 1024

// maxDSProofPushes is the maximum number of pushes a dsproof spender can
// have.  A valid proof has exactly one, so this only bounds the memory used
// to decode invalid ones.
const maxDSProofPushes = 1000

// DSProofSpender holds the parts of the signature preimage of one of the two
// inputs that spend the same output, along with the data that input pushed.
// It is enough to check the signature without the rest of the transaction.
type DSProofSpender struct {
	Version         uint32
	Sequence        uint32
	LockTime        uint32
	HashPrevOutputs chainhash.Hash
	HashSequence    chainhash.Hash
	HashOutputs     chainhash.Hash
	PushData        [][]byte
}

// MsgDSProof implements the Message interface and represents a bitcoin
// dsproof-beta message.  It proves that the output PrevOut was spent by two
// different signed inputs.
type MsgDSProof struct {
	PrevOut       OutPoint
	FirstSpender  DSProofSpender
	DoubleSpender DSProofSpender
}

// readDSProofSpender reads one spender of a dsproof message.
func readDSProofSpender(r io.Reader, pver uint32, s *DSProofSpender) error {
	err := readElements(r, &s.Version, &s.Sequence, &s.LockTime,
		&s.HashPrevOutputs, &s.HashSequence, &s.HashOutputs)
	if err != nil {
		return err
	}

	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	if count > maxDSProofPushes {
		str := fmt.Sprintf("too many dsproof pushes [count %d, max %d]",
			count, maxDSProofPushes)
		return messageError("MsgDSProof.BchDecode", str)
	}

	// Each push takes at least one byte for its length, so the count is
	// bounded by the remaining payload before anything is allocated.
	s.PushData = make([][]byte, 0, allocCount(r, count, 1, 16))
	for i := uint64(0); i < count; i++ {
		data, err := ReadVarBytes(r, pver, MaxDSProofPayload,
			"dsproof push data")
		if err != nil {
			return err
		}
		s.PushData = append(s.PushData, data)
	}
	return nil
}

// writeDSProofSpender writes one spender of a dsproof message.
func writeDSProofSpender(w io.Writer, pver uint32, s *DSProofSpender) error {
	err := writeElements(w, s.Version, s.Sequence, s.LockTime,
		&s.HashPrevOutputs, &s.HashSequence, &s.HashOutputs)
	if err != nil {
		return err
	}

	err = WriteVarInt(w, pver, uint64(len(s.PushData)))
	if err != nil {
		return err
	}
	for _, data := range s.PushData {
		if err := WriteVarBytes(w, pver, data); err != nil {
			return err
		}
	}
	return nil
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// Bytes after the second spender are ignored.
// This is part of the Message interface implementation.
func (msg *MsgDSProof) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	err := readElements(r, &msg.PrevOut.Hash, &msg.PrevOut.Index)
	if err != nil {
		return err
	}
	if err := readDSProofSpender(r, pver, &msg.FirstSpender); err != nil {
		return err
	}
	return readDSProofSpender(r, pver, &msg.DoubleSpender)
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgDSProof) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	err := writeElements(w, &msg.PrevOut.Hash, msg.PrevOut.Index)
	if err != nil {
		return err
	}
	if err := writeDSProofSpender(w, pver, &msg.FirstSpender); err != nil {
		return err
	}
	return writeDSProofSpender(w, pver, &msg.DoubleSpender)
}

// SerializeSize returns the number of bytes it would take to serialize the
// message.
func (msg *MsgDSProof) SerializeSize() int {
	n := chainhash.HashSize + 4
	for _, s := range []*DSProofSpender{&msg.FirstSpender, &msg.DoubleSpender} {
		n += 12 + 3*chainhash.HashSize + VarIntSerializeSize(uint64(len(s.PushData)))
		for _, data := range s.PushData {
			n += VarIntSerializeSize(uint64(len(data))) + len(data)
		}
	}
	return n
}

// ProofHash returns the double sha256 of the serialized proof, which
// identifies the proof in inventory vectors.
func (msg *MsgDSProof) ProofHash() chainhash.Hash {
	// Encoding to a buffer only fails when out of memory, which panics.
	buf := bytes.NewBuffer(make([]byte, 0, msg.SerializeSize()))
	_ = msg.BchEncode(buf, 0, BaseEncoding)
	return chainhash.DoubleHashH(buf.Bytes())
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgDSProof) Command() string {
	return CmdDSProof
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgDSProof) MaxPayloadLength(pver uint32) uint32 {
	return MaxDSProofPayload
}

// String returns a short description of the proof.
func (msg *MsgDSProof) String() string {
	return fmt.Sprintf("dsproof %v (outpoint %v)", msg.ProofHash(),
		msg.PrevOut)
}

// NewMsgDSProof returns a new bitcoin dsproof-beta message that conforms to
// the Message interface.
func NewMsgDSProof(prevOut OutPoint, firstSpender, doubleSpender DSProofSpender) *MsgDSProof {
	return &MsgDSProof{
		PrevOut:       prevOut,
		FirstSpender:  firstSpender,
		DoubleSpender: doubleSpender,
	}
}
