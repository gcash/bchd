// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/gcash/bchd/bchec"
)

// MsgAvaResponse implements the Message interface and represents a bitcoin avaresponse message.
// It is the signed response to the avarequest message.
type MsgAvaResponse struct {
	RequestID uint64
	Votes     []byte
	Signature *bchec.Signature
}

// SerializeForSignature returns the serialization of the vote records and
// the request ID that is suitable for signing.
func (msg *MsgAvaResponse) SerializeForSignature() []byte {
	var buf bytes.Buffer
	reqIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(reqIDBytes, msg.RequestID)

	buf.Write(reqIDBytes)
	buf.Write(msg.Votes)
	return buf.Bytes()
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgAvaResponse) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if err := readElement(r, &msg.RequestID); err != nil {
		return err
	}

	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	votes := make([]byte, count)
	_, err = io.ReadFull(r, votes)
	if err != nil {
		return err
	}
	msg.Votes = votes

	sigLen, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	sigBytes := make([]byte, sigLen)
	_, err = io.ReadFull(r, sigBytes)
	if err != nil {
		return err
	}
	msg.Signature, err = bchec.ParseSchnorrSignature(sigBytes)
	if err != nil {
		return err
	}

	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgAvaResponse) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if err := writeElement(w, msg.RequestID); err != nil {
		return err
	}

	if err := WriteVarInt(w, pver, uint64(len(msg.Votes))); err != nil {
		return err
	}
	if _, err := w.Write(msg.Votes); err != nil {
		return err
	}

	if err := WriteVarInt(w, pver, uint64(len(msg.Signature.Serialize()))); err != nil {
		return err
	}
	if _, err := w.Write(msg.Signature.Serialize()); err != nil {
		return err
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgAvaResponse) Command() string {
	return CmdAvaResponse
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgAvaResponse) MaxPayloadLength(pver uint32) uint32 {
	// Num inventory vectors (varInt) + max allowed inventory vectors.
	return MaxVarIntPayload + (MaxInvPerMsg*(maxInvVectPayload+1) + 73)
}

// NewMsgAvaResponse returns a new bitcoin avaresponse message that conforms to the Message
// interface.  See MsgInv for details.
func NewMsgAvaResponse(requestID uint64, votes []byte, signature *bchec.Signature) *MsgAvaResponse {
	return &MsgAvaResponse{
		RequestID: requestID,
		Votes:     votes,
		Signature: signature,
	}
}
