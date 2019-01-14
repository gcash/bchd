// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"encoding/binary"
	"fmt"
	"github.com/gcash/bchd/bchec"
	"io"
)

// MsgAvaResponse implements the Message interface and represents a bitcoin avaresponse message.
// It is the signed response to the avarequest message.
type MsgAvaResponse struct {
	RequestID uint64
	VoteList  []*VoteRecord
	Signature *bchec.Signature
}

// AddVoteRecord adds an inventory vector to the message.
func (msg *MsgAvaResponse) AddVoteRecord(vr *VoteRecord) error {
	if len(msg.VoteList)+1 > MaxInvPerMsg {
		str := fmt.Sprintf("too many vote records in message [max %v]",
			MaxInvPerMsg)
		return messageError("MsgAvaResponse.AddVoteRecord", str)
	}

	msg.VoteList = append(msg.VoteList, vr)
	return nil
}

// SerializeForSignature returns the serialization of the vote records and
// the request ID that is suitable for signing.
func (msg *MsgAvaResponse) SerializeForSignature() []byte {
	var ser []byte
	binary.LittleEndian.PutUint64(ser, msg.RequestID)
	for _, vr := range msg.VoteList {
		ser = append(ser, vr.Serialize()...)
	}
	return ser
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgAvaResponse) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if err := readElement(r, msg.RequestID); err != nil {
		return err
	}
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Limit to max inventory vectors per message.
	if count > MaxInvPerMsg {
		str := fmt.Sprintf("too many voterecord in message [%v]", count)
		return messageError("MsgInv.BchDecode", str)
	}

	// Create a contiguous slice of inventory vectors to deserialize into in
	// order to reduce the number of allocations.
	voteList := make([]VoteRecord, count)
	msg.VoteList = make([]*VoteRecord, 0, count)
	for i := uint64(0); i < count; i++ {
		vr := &voteList[i]
		err := readVoteRecord(r, pver, vr)
		if err != nil {
			return err
		}
		msg.AddVoteRecord(vr)
	}

	sigLen, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	sigBytes := make([]byte, sigLen)
	_, err = io.ReadFull(r, sigBytes)
	if err != nil {
		return err
	}
	msg.Signature, err = bchec.ParseSignature(sigBytes, bchec.S256())
	if err != nil {
		return err
	}

	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgAvaResponse) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	// Limit to max inventory vectors per message.
	count := len(msg.VoteList)
	if count > MaxInvPerMsg {
		str := fmt.Sprintf("too many voterecord in message [%v]", count)
		return messageError("MsgInv.BchEncode", str)
	}

	if err := writeElement(w, msg.RequestID); err != nil {
		return err
	}

	err := WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}

	for _, vr := range msg.VoteList {
		err := writeVoteRecord(w, pver, vr)
		if err != nil {
			return err
		}
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
func NewMsgAvaResponse(requestID uint64, signature *bchec.Signature) *MsgAvaResponse {
	return &MsgAvaResponse{
		RequestID: requestID,
		VoteList:  make([]*VoteRecord, 0, defaultInvListAlloc),
		Signature: signature,
	}
}
