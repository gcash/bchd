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

	// minSHVResponsePayload is the size of the smallest SHV response: the
	// type, one-byte heights, the header, an empty proof and the target.
	minSHVResponsePayload = 1 + 1 + 1 + blockHeaderLen + 1 +
		chainhash.HashSize
)

// SHVResponse is a response of an shv message: the header of a block, and
// the proof that its hash is the leaf at its height of the commitment at
// CommitmentHeight, with the target the proof leads to.
type SHVResponse struct {
	// Type is the type of the request answered.
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
	return 1 + VarIntSerializeSize(resp.CommitmentHeight) +
		VarIntSerializeSize(resp.BlockHeight) + blockHeaderLen +
		VarIntSerializeSize(uint64(len(resp.Proof))) +
		len(resp.Proof)*chainhash.HashSize + chainhash.HashSize
}

// readSHVResponse reads an SHV response from r.
func readSHVResponse(r io.Reader, pver uint32, resp *SHVResponse) error {
	var err error
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

// MsgSHV implements the Message interface and represents an shv message,
// from CHIP-2026-02, Simplified Header Verification.  It answers a getshv
// message with a response for each valid request, leaving the invalid ones
// out.  A server may split its answer over more than one shv message, and the
// responses may come in any order, so a client matches them to its requests by
// their type, commitment height and block height, or for a request by hash,
// by the hash of the header.
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
	// The limit BCHN applies to every message but block, cmpctblock and
	// blocktxn.  An answer to a full getshv message with the longest
	// proofs, about 220 KB, fits well within it.
	return maxProtocolMessageLength
}

// NewMsgSHV returns a new shv message that conforms to the Message interface,
// with the passed responses.
func NewMsgSHV(responses ...SHVResponse) *MsgSHV {
	return &MsgSHV{Responses: responses}
}
