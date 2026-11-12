// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// A script filter (filter type 1, gcash/bchd#767) is served with the root of
// a Merkle tree over its block's entries, its leaves, in front of it. The
// getcfrange and getcfproof messages ask what that tree holds, so a client
// can tell which of two peers that disagree serves a wrong filter without
// downloading the block.

// CFLeafSize is the size of a leaf of a script filter's entry tree: a 32-byte
// key, the transaction index and the input or output index as little-endian
// uint32, and the kind.
const CFLeafSize = 32 + 4 + 4 + 1

// CFLeaf is a serialized leaf of a script filter's entry tree.
type CFLeaf [CFLeafSize]byte

// Spent reports whether the leaf is the script or the token category of an
// output its block spends, kind 1 or 3. The cfproof of such a leaf carries
// the transaction of the spent output too.
func (l *CFLeaf) Spent() bool {
	return l[CFLeafSize-1] == 1 || l[CFLeafSize-1] == 3
}

// MaxCFTreeDepth is the most levels above the leaves of a tree of up to 2^32
// leaves: the most hashes in the branch of one leaf of a script filter's
// entry tree, or of one transaction in a block. A range has at most two per
// level.
const MaxCFTreeDepth = 32

// MaxCFRangePrefixBits is the longest key prefix getcfrange asks for: a whole
// 32-byte key.
const MaxCFRangePrefixBits = 256

// readCFHashes reads a varint count of hashes, at most max, then the hashes.
func readCFHashes(r io.Reader, pver uint32, max uint64, field string) ([]chainhash.Hash, error) {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return nil, err
	}
	if count > max {
		str := fmt.Sprintf("too many hashes in %s [count %v, max %v]", field, count, max)
		return nil, messageError(field, str)
	}
	hashes := make([]chainhash.Hash, 0, allocCount(r, count, chainhash.HashSize, count))
	for range count {
		var hash chainhash.Hash
		if err := readElement(r, &hash); err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

// writeCFHashes writes a varint count of hashes, then the hashes.
func writeCFHashes(w io.Writer, pver uint32, hashes []chainhash.Hash, max int, field string) error {
	if len(hashes) > max {
		str := fmt.Sprintf("too many hashes in %s [count %v, max %v]", field, len(hashes), max)
		return messageError(field, str)
	}
	if err := WriteVarInt(w, pver, uint64(len(hashes))); err != nil {
		return err
	}
	for i := range hashes {
		if err := writeElement(w, &hashes[i]); err != nil {
			return err
		}
	}
	return nil
}

// MsgGetCFRange implements the Message interface and represents a getcfrange
// message. It asks for the leaves of a block's script filter entry tree whose
// keys start with a prefix.
type MsgGetCFRange struct {
	FilterType FilterType
	BlockHash  chainhash.Hash

	// PrefixBits is the length of the prefix in bits, 0 to 256. 0 asks for
	// every leaf, and 256 for one exact key.
	PrefixBits uint16

	// Prefix holds the prefix, with the bits past its length zero.
	Prefix [32]byte
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetCFRange) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	if err := readElements(r, &msg.FilterType, &msg.BlockHash); err != nil {
		return err
	}
	bits, err := binarySerializer.Uint16(r, littleEndian)
	if err != nil {
		return err
	}
	if bits > MaxCFRangePrefixBits {
		str := fmt.Sprintf("prefix of %d bits is longer than a key", bits)
		return messageError("MsgGetCFRange.BchDecode", str)
	}
	msg.PrefixBits = bits
	_, err = io.ReadFull(r, msg.Prefix[:])
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetCFRange) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	if msg.PrefixBits > MaxCFRangePrefixBits {
		str := fmt.Sprintf("prefix of %d bits is longer than a key", msg.PrefixBits)
		return messageError("MsgGetCFRange.BchEncode", str)
	}
	if err := writeElements(w, msg.FilterType, &msg.BlockHash); err != nil {
		return err
	}
	if err := binarySerializer.PutUint16(w, littleEndian, msg.PrefixBits); err != nil {
		return err
	}
	_, err := w.Write(msg.Prefix[:])
	return err
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgGetCFRange) Command() string {
	return CmdGetCFRange
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgGetCFRange) MaxPayloadLength(pver uint32) uint32 {
	// Filter type + block hash + prefix length + prefix.
	return 1 + chainhash.HashSize + 2 + 32
}

// NewMsgGetCFRange returns a new getcfrange message for the leaves of a
// block's entry tree whose keys start with the first bits of prefix.
func NewMsgGetCFRange(filterType FilterType, blockHash *chainhash.Hash,
	bits uint16, prefix [32]byte) *MsgGetCFRange {

	return &MsgGetCFRange{
		FilterType: filterType,
		BlockHash:  *blockHash,
		PrefixBits: bits,
		Prefix:     prefix,
	}
}

// MsgCFRange implements the Message interface and represents a cfrange
// message, the reply to getcfrange. With the hashes on its edges, the run of
// leaves from Before to After rebuilds the root of the tree, which shows that
// the tree holds no other leaf with the prefix asked for.
type MsgCFRange struct {
	FilterType FilterType
	BlockHash  chainhash.Hash

	// LeafCount is the number of leaves in the tree.
	LeafCount uint32

	// FirstIndex is the index of the first leaf with the prefix or, when
	// no leaf has it, of the place where one would sit.
	FirstIndex uint32

	// Leaves are the leaves with the prefix.
	Leaves []CFLeaf

	// Before is the leaf just before them. It is there exactly when
	// FirstIndex is above 0.
	Before *CFLeaf

	// After is the leaf just after them. It is there exactly when they end
	// before the last leaf.
	After *CFLeaf

	// Hashes are the hashes on the edges of the run from Before to After,
	// level by level from the leaves up, the left one before the right one.
	Hashes []chainhash.Hash
}

// maxCFRangeLeaves is the most leaves a cfrange message can hold.
func maxCFRangeLeaves() uint64 {
	return uint64(maxMessagePayload()) / CFLeafSize
}

// hasNeighbours returns whether a range from first with n leaves, in a tree
// of count leaves, has a leaf before it and a leaf after it.
func hasNeighbours(count, first uint32, n int) (before, after bool) {
	return first > 0, uint64(first)+uint64(n) < uint64(count)
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgCFRange) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	err := readElements(r, &msg.FilterType, &msg.BlockHash, &msg.LeafCount, &msg.FirstIndex)
	if err != nil {
		return err
	}
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if count > maxCFRangeLeaves() || uint64(msg.FirstIndex)+count > uint64(msg.LeafCount) {
		str := fmt.Sprintf("%d leaves from index %d do not fit a tree of %d "+
			"leaves or a message", count, msg.FirstIndex, msg.LeafCount)
		return messageError("MsgCFRange.BchDecode", str)
	}
	msg.Leaves = make([]CFLeaf, 0, allocCount(r, count, CFLeafSize, count))
	for range count {
		var leaf CFLeaf
		if _, err := io.ReadFull(r, leaf[:]); err != nil {
			return err
		}
		msg.Leaves = append(msg.Leaves, leaf)
	}
	before, after := hasNeighbours(msg.LeafCount, msg.FirstIndex, len(msg.Leaves))
	msg.Before, msg.After = nil, nil
	if before {
		msg.Before = new(CFLeaf)
		if _, err := io.ReadFull(r, msg.Before[:]); err != nil {
			return err
		}
	}
	if after {
		msg.After = new(CFLeaf)
		if _, err := io.ReadFull(r, msg.After[:]); err != nil {
			return err
		}
	}
	msg.Hashes, err = readCFHashes(r, pver, 2*MaxCFTreeDepth, "MsgCFRange.BchDecode")
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgCFRange) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	before, after := hasNeighbours(msg.LeafCount, msg.FirstIndex, len(msg.Leaves))
	if uint64(len(msg.Leaves)) > maxCFRangeLeaves() ||
		uint64(msg.FirstIndex)+uint64(len(msg.Leaves)) > uint64(msg.LeafCount) ||
		before != (msg.Before != nil) || after != (msg.After != nil) {

		str := fmt.Sprintf("%d leaves from index %d, with neighbours %v and "+
			"%v, do not fit a tree of %d leaves or a message", len(msg.Leaves),
			msg.FirstIndex, msg.Before != nil, msg.After != nil, msg.LeafCount)
		return messageError("MsgCFRange.BchEncode", str)
	}
	err := writeElements(w, msg.FilterType, &msg.BlockHash, msg.LeafCount, msg.FirstIndex)
	if err != nil {
		return err
	}
	if err := WriteVarInt(w, pver, uint64(len(msg.Leaves))); err != nil {
		return err
	}
	for i := range msg.Leaves {
		if _, err := w.Write(msg.Leaves[i][:]); err != nil {
			return err
		}
	}
	for _, neighbour := range []*CFLeaf{msg.Before, msg.After} {
		if neighbour != nil {
			if _, err := w.Write(neighbour[:]); err != nil {
				return err
			}
		}
	}
	return writeCFHashes(w, pver, msg.Hashes, 2*MaxCFTreeDepth, "MsgCFRange.BchEncode")
}

// SerializeSize returns the number of bytes the message takes on the wire.
func (msg *MsgCFRange) SerializeSize() int {
	n := 1 + chainhash.HashSize + 4 + 4 + VarIntSerializeSize(uint64(len(msg.Leaves))) +
		CFLeafSize*len(msg.Leaves) + VarIntSerializeSize(uint64(len(msg.Hashes))) +
		chainhash.HashSize*len(msg.Hashes)
	if msg.Before != nil {
		n += CFLeafSize
	}
	if msg.After != nil {
		n += CFLeafSize
	}
	return n
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgCFRange) Command() string {
	return CmdCFRange
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgCFRange) MaxPayloadLength(pver uint32) uint32 {
	return maxMessagePayload()
}
