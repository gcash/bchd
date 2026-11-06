// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

const (
	// MaxSHVProofLength is the most hashes an SHV proof can have: a proof
	// to the root of a commitment of 2^64 - 1 blocks has 64.
	MaxSHVProofLength = 64

	// SHVStatusFound is an shv entry that carries the header and the proof.
	// Every other status is that one byte alone.
	SHVStatusFound uint8 = 0

	// SHVStatusNotFound is an shv entry whose block is not on the main
	// chain.
	SHVStatusNotFound uint8 = 1

	// SHVStatusAboveCommitment is an shv entry whose block is above the
	// commitment height.
	SHVStatusAboveCommitment uint8 = 2

	// SHVStatusCommitmentAboveTip is an shv entry whose commitment height
	// is above the index's tip.
	SHVStatusCommitmentAboveTip uint8 = 3

	// SHVStatusReservedType is an shv entry whose request set a reserved
	// type bit, bits 2 to 7.
	SHVStatusReservedType uint8 = 4

	// minSHVResponsePayload is the size of the smallest SHV response, a
	// status other than found, which is that byte alone.  A found response
	// also carries the type, the heights, the header, the proof and the
	// target.
	minSHVResponsePayload = 1
)

// SHVResponse is one entry of an shv message.  Status says whether the entry
// carries the header of a block and the proof that its hash is the leaf at
// its height of the commitment at CommitmentHeight.  Any other status is the
// status byte alone.
type SHVResponse struct {
	// Status is the outcome of the request this entry answers.
	Status uint8

	// Type is the type of the request answered.  It is written only when
	// Status is SHVStatusFound.
	Type uint8

	// CommitmentHeight is the commitment height of the request answered.
	CommitmentHeight uint64

	// BlockHeight is the height of the block, also when the request
	// selected it by its hash.
	BlockHeight uint64

	// Header is the block's header, whose hash is the leaf proven.
	Header BlockHeader

	// Proof holds the siblings on the path from the leaf up to the target,
	// from the bottom up.  The bits of BlockHeight say which side each is
	// on: a 1 bit puts the sibling on the left.
	Proof []chainhash.Hash

	// Target is the peak of the block's mountain, or the root of the
	// commitment if the request's type has SHVProofToRoot set.
	Target chainhash.Hash
}

// SerializeSize returns the number of bytes the response takes in an shv
// message.
func (resp *SHVResponse) SerializeSize() int {
	if resp.Status != SHVStatusFound {
		return 1
	}
	return 1 + 1 + VarIntSerializeSize(resp.CommitmentHeight) +
		VarIntSerializeSize(resp.BlockHeight) + blockHeaderLen +
		VarIntSerializeSize(uint64(len(resp.Proof))) +
		len(resp.Proof)*chainhash.HashSize + chainhash.HashSize
}

// readSHVResponse reads an SHV response from r.
func readSHVResponse(r io.Reader, pver uint32, resp *SHVResponse) error {
	status, err := binarySerializer.Uint8(r)
	if err != nil {
		return err
	}
	if status > SHVStatusReservedType {
		str := fmt.Sprintf("unknown SHV status [status %d]", status)
		return messageError("MsgSHV.BchDecode", str)
	}
	resp.Status = status
	if status != SHVStatusFound {
		return nil
	}

	resp.Type, err = binarySerializer.Uint8(r)
	if err != nil {
		return err
	}
	resp.CommitmentHeight, err = ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	resp.BlockHeight, err = ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if err := readBlockHeader(r, pver, &resp.Header); err != nil {
		return err
	}

	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if count > MaxSHVProofLength {
		str := fmt.Sprintf("too many hashes in proof [count %v, max %v]",
			count, MaxSHVProofLength)
		return messageError("MsgSHV.BchDecode", str)
	}
	resp.Proof = make([]chainhash.Hash, 0, allocCount(r, count,
		chainhash.HashSize, MaxSHVProofLength))
	for range count {
		var hash chainhash.Hash
		if err := readElement(r, &hash); err != nil {
			return err
		}
		resp.Proof = append(resp.Proof, hash)
	}

	return readElement(r, &resp.Target)
}

// writeSHVResponse writes an SHV response to w.
func writeSHVResponse(w io.Writer, pver uint32, resp *SHVResponse) error {
	if resp.Status > SHVStatusReservedType {
		str := fmt.Sprintf("unknown SHV status [status %d]", resp.Status)
		return messageError("MsgSHV.BchEncode", str)
	}
	if err := binarySerializer.PutUint8(w, resp.Status); err != nil {
		return err
	}
	if resp.Status != SHVStatusFound {
		return nil
	}

	if err := binarySerializer.PutUint8(w, resp.Type); err != nil {
		return err
	}
	if err := WriteVarInt(w, pver, resp.CommitmentHeight); err != nil {
		return err
	}
	if err := WriteVarInt(w, pver, resp.BlockHeight); err != nil {
		return err
	}
	if err := writeBlockHeader(w, pver, &resp.Header); err != nil {
		return err
	}

	count := len(resp.Proof)
	if count > MaxSHVProofLength {
		str := fmt.Sprintf("too many hashes in proof [count %v, max %v]",
			count, MaxSHVProofLength)
		return messageError("MsgSHV.BchEncode", str)
	}
	if err := WriteVarInt(w, pver, uint64(count)); err != nil {
		return err
	}
	for i := range resp.Proof {
		if err := writeElement(w, &resp.Proof[i]); err != nil {
			return err
		}
	}

	return writeElement(w, &resp.Target)
}

// MsgSHV implements the Message interface and represents an shv message, from
// CHIP-2026-02, Simplified Header Verification.  It answers one getshv message
// with one entry per request, in the same order.  An entry other than
// SHVStatusFound is a status byte alone.
type MsgSHV struct {
	Responses []SHVResponse
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgSHV) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// An shv message answers no more requests than a getshv message may
	// hold.
	if count > MaxSHVRequestsPerMsg {
		str := fmt.Sprintf("too many responses for message "+
			"[count %v, max %v]", count, MaxSHVRequestsPerMsg)
		return messageError("MsgSHV.BchDecode", str)
	}

	// Bound the allocation by the payload.
	msg.Responses = make([]SHVResponse, 0, allocCount(r, count,
		minSHVResponsePayload, MaxSHVRequestsPerMsg))
	for range count {
		var resp SHVResponse
		if err := readSHVResponse(r, pver, &resp); err != nil {
			return err
		}
		msg.Responses = append(msg.Responses, resp)
	}
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgSHV) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	count := len(msg.Responses)
	if count > MaxSHVRequestsPerMsg {
		str := fmt.Sprintf("too many responses for message "+
			"[count %v, max %v]", count, MaxSHVRequestsPerMsg)
		return messageError("MsgSHV.BchEncode", str)
	}

	if err := WriteVarInt(w, pver, uint64(count)); err != nil {
		return err
	}
	for i := range msg.Responses {
		if err := writeSHVResponse(w, pver, &msg.Responses[i]); err != nil {
			return err
		}
	}
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgSHV) Command() string {
	return CmdSHV
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgSHV) MaxPayloadLength(pver uint32) uint32 {
	// One shv message answers one getshv message in full: up to
	// MaxSHVRequestsPerMsg found responses, each with a proof of
	// MaxSHVProofLength hashes and heights at their longest compact size.
	// The limit is the larger of that size and maxProtocolMessageLength,
	// which BCHN applies to every message but block, cmpctblock and
	// blocktxn, so the whole answer fits in one message.
	entry := 1 + 1 + 2*VarIntSerializeSize(^uint64(0)) + blockHeaderLen +
		VarIntSerializeSize(MaxSHVProofLength) +
		MaxSHVProofLength*chainhash.HashSize + chainhash.HashSize
	full := VarIntSerializeSize(MaxSHVRequestsPerMsg) +
		MaxSHVRequestsPerMsg*entry
	if full > int(maxProtocolMessageLength) {
		return uint32(full)
	}
	return maxProtocolMessageLength
}

// NewMsgSHV returns a new shv message that conforms to the Message interface,
// with the passed responses.
func NewMsgSHV(responses ...SHVResponse) *MsgSHV {
	return &MsgSHV{Responses: responses}
}
