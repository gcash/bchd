// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

const (
	// MaxSHVRequestsPerMsg is the most requests a getshv message may hold.
	// CHIP-2026-02 has servers refuse a getshv message with more, and as
	// an shv message answers the requests of one getshv message, it holds
	// no more responses either.
	MaxSHVRequestsPerMsg = 100

	// SHVProofToRoot is the bit of an SHV request type that asks for a
	// proof to the root of the commitment, rather than to the peak of the
	// block's mountain.
	SHVProofToRoot uint8 = 1 << 0

	// SHVByHash is the bit of an SHV request type that selects the block
	// by its hash, rather than by its height.
	SHVByHash uint8 = 1 << 1

	// SHVReservedTypeBits are the bits of an SHV request type that
	// CHIP-2026-02 reserves.  A request with any of them set is invalid.
	SHVReservedTypeBits uint8 = 0xfc

	// minSHVRequestPayload is the size of the smallest SHV request: the
	// type, a one-byte commitment height and the block height or hash.
	minSHVRequestPayload = 1 + 1 + chainhash.HashSize

	// maxSHVRequestsDecode is the most requests a getshv message within its
	// payload limit can hold.  More than MaxSHVRequestsPerMsg decode, so
	// that the server can penalize the peer as BCHN does rather than
	// disconnect it as for a malformed message.
	maxSHVRequestsDecode = maxProtocolMessageLength / minSHVRequestPayload
)

// SHVRequest is a request of a getshv message for the proof that a block's
// header is in the main chain, from CHIP-2026-02, Simplified Header
// Verification.  The block is selected by its height or its hash, and the
// proof leads to the peak or the root of the commitment at CommitmentHeight,
// the Merkle Mountain Range of the block hashes up to that height.
type SHVRequest struct {
	// Type is the request type: SHVProofToRoot and SHVByHash, or neither.
	Type uint8

	// CommitmentHeight is the height of the last block of the commitment.
	CommitmentHeight uint64

	// BlockHeightOrHash is the block's hash for a request by hash, and
	// otherwise its height as a 32-byte little-endian number.
	BlockHeightOrHash chainhash.Hash
}

// NewSHVRequestByHeight returns a request for the proof of the block at
// blockHeight to the root of the commitment at commitmentHeight when toRoot is
// set, and to the block's peak otherwise.
func NewSHVRequestByHeight(toRoot bool, commitmentHeight, blockHeight uint64) SHVRequest {
	req := SHVRequest{CommitmentHeight: commitmentHeight}
	if toRoot {
		req.Type |= SHVProofToRoot
	}
	binary.LittleEndian.PutUint64(req.BlockHeightOrHash[:8], blockHeight)
	return req
}

// NewSHVRequestByHash returns a request for the proof of the block with the
// passed hash to the root of the commitment at commitmentHeight when toRoot is
// set, and to the block's peak otherwise.
func NewSHVRequestByHash(toRoot bool, commitmentHeight uint64, hash *chainhash.Hash) SHVRequest {
	req := SHVRequest{Type: SHVByHash, CommitmentHeight: commitmentHeight,
		BlockHeightOrHash: *hash}
	if toRoot {
		req.Type |= SHVProofToRoot
	}
	return req
}

// ProofToRoot reports whether the request asks for a proof to the root of the
// commitment rather than to the block's peak.
func (req *SHVRequest) ProofToRoot() bool {
	return req.Type&SHVProofToRoot != 0
}

// ByHash reports whether the request selects the block by its hash rather
// than by its height.
func (req *SHVRequest) ByHash() bool {
	return req.Type&SHVByHash != 0
}

// BlockHeight returns the height a request by height selects, from the first
// eight bytes of BlockHeightOrHash, and whether the other 24 are zero, as they
// must be.
func (req *SHVRequest) BlockHeight() (uint64, bool) {
	for _, b := range req.BlockHeightOrHash[8:] {
		if b != 0 {
			return 0, false
		}
	}
	return binary.LittleEndian.Uint64(req.BlockHeightOrHash[:8]), true
}

// IsValid reports whether the request is valid by itself: whether none of its
// reserved type bits are set and, for a request by height, the height is well
// formed.  Whether the heights and the hash fit the chain is up to the server.
func (req *SHVRequest) IsValid() bool {
	if req.Type&SHVReservedTypeBits != 0 {
		return false
	}
	if req.ByHash() {
		return true
	}
	_, ok := req.BlockHeight()
	return ok
}

// readSHVRequest reads an SHV request from r.
func readSHVRequest(r io.Reader, pver uint32, req *SHVRequest) error {
	var err error
	req.Type, err = binarySerializer.Uint8(r)
	if err != nil {
		return err
	}
	req.CommitmentHeight, err = ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	return readElement(r, &req.BlockHeightOrHash)
}

// writeSHVRequest writes an SHV request to w.
func writeSHVRequest(w io.Writer, pver uint32, req *SHVRequest) error {
	if err := binarySerializer.PutUint8(w, req.Type); err != nil {
		return err
	}
	if err := WriteVarInt(w, pver, req.CommitmentHeight); err != nil {
		return err
	}
	return writeElement(w, &req.BlockHeightOrHash)
}

// MsgGetSHV implements the Message interface and represents a getshv message,
// from CHIP-2026-02, Simplified Header Verification.  It asks a peer that
// advertises SFNodeSHV for the proofs that block headers are in its main
// chain, which it answers with an shv message.
//
// A server should answer no more than MaxSHVRequestsPerMsg requests, but more
// decode, up to what fits in the message, so that the server can tell that
// peer apart from one that sent a malformed message.
type MsgGetSHV struct {
	Requests []SHVRequest
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetSHV) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Limit to the number of requests that fit in the message.
	if count > maxSHVRequestsDecode {
		str := fmt.Sprintf("too many requests for message "+
			"[count %v, max %v]", count, maxSHVRequestsDecode)
		return messageError("MsgGetSHV.BchDecode", str)
	}

	// Bound the allocation by the payload.
	msg.Requests = make([]SHVRequest, 0, allocCount(r, count,
		minSHVRequestPayload, MaxSHVRequestsPerMsg))
	for range count {
		var req SHVRequest
		if err := readSHVRequest(r, pver, &req); err != nil {
			return err
		}
		msg.Requests = append(msg.Requests, req)
	}
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetSHV) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	count := len(msg.Requests)
	if count > maxSHVRequestsDecode {
		str := fmt.Sprintf("too many requests for message "+
			"[count %v, max %v]", count, maxSHVRequestsDecode)
		return messageError("MsgGetSHV.BchEncode", str)
	}

	if err := WriteVarInt(w, pver, uint64(count)); err != nil {
		return err
	}
	for i := range msg.Requests {
		if err := writeSHVRequest(w, pver, &msg.Requests[i]); err != nil {
			return err
		}
	}
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgGetSHV) Command() string {
	return CmdGetSHV
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetSHV) MaxPayloadLength(pver uint32) uint32 {
	// The limit BCHN applies to every message but block, cmpctblock and
	// blocktxn.
	return maxProtocolMessageLength
}

// NewMsgGetSHV returns a new getshv message that conforms to the Message
// interface, with the passed requests.
func NewMsgGetSHV(requests ...SHVRequest) *MsgGetSHV {
	return &MsgGetSHV{Requests: requests}
}
