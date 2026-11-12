// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// MsgGetCFProof implements the Message interface and represents a getcfproof
// message. It asks for one leaf of a block's script filter entry tree and
// what the leaf points to.
type MsgGetCFProof struct {
	FilterType FilterType
	BlockHash  chainhash.Hash
	LeafIndex  uint32
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetCFProof) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	return readElements(r, &msg.FilterType, &msg.BlockHash, &msg.LeafIndex)
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetCFProof) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	return writeElements(w, msg.FilterType, &msg.BlockHash, msg.LeafIndex)
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgGetCFProof) Command() string {
	return CmdGetCFProof
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgGetCFProof) MaxPayloadLength(pver uint32) uint32 {
	// Filter type + block hash + leaf index.
	return 1 + chainhash.HashSize + 4
}

// NewMsgGetCFProof returns a new getcfproof message for one leaf of a block's
// entry tree.
func NewMsgGetCFProof(filterType FilterType, blockHash *chainhash.Hash, leafIndex uint32) *MsgGetCFProof {
	return &MsgGetCFProof{
		FilterType: filterType,
		BlockHash:  *blockHash,
		LeafIndex:  leafIndex,
	}
}

// MsgCFProof implements the Message interface and represents a cfproof
// message, the reply to getcfproof: a leaf with its branch in the entry tree,
// the transaction it names with that transaction's Merkle branch in the
// block, and, for the leaf of a spent output, the transaction of that output
// with its own block and Merkle branch. A client that has the block headers
// checks the leaf against them without downloading either block.
type MsgCFProof struct {
	FilterType FilterType
	BlockHash  chainhash.Hash

	// LeafCount is the number of leaves in the tree, and LeafIndex the
	// index of Leaf among them.
	LeafCount uint32
	LeafIndex uint32
	Leaf      CFLeaf

	// LeafBranch rebuilds the root of the tree from Leaf, level by level
	// from the leaves up.
	LeafBranch []chainhash.Hash

	// Tx is the transaction at the leaf's transaction index, and TxBranch
	// its Merkle branch to the block's Merkle root.
	Tx       *MsgTx
	TxBranch []chainhash.Hash

	// For a leaf of a spent output (Leaf.Spent), PrevTx is the transaction
	// of the output, at PrevTxIndex in block PrevBlockHash, and
	// PrevTxBranch its Merkle branch there. They are left out otherwise.
	PrevBlockHash chainhash.Hash
	PrevTxIndex   uint32
	PrevTx        *MsgTx
	PrevTxBranch  []chainhash.Hash
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgCFProof) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	err := readElements(r, &msg.FilterType, &msg.BlockHash, &msg.LeafCount, &msg.LeafIndex)
	if err != nil {
		return err
	}
	if msg.LeafIndex >= msg.LeafCount {
		str := fmt.Sprintf("leaf %d of a tree of %d leaves", msg.LeafIndex, msg.LeafCount)
		return messageError("MsgCFProof.BchDecode", str)
	}
	if _, err := io.ReadFull(r, msg.Leaf[:]); err != nil {
		return err
	}
	const field = "MsgCFProof.BchDecode"
	if msg.LeafBranch, err = readCFHashes(r, pver, MaxCFTreeDepth, field); err != nil {
		return err
	}
	msg.Tx = new(MsgTx)
	if err := msg.Tx.BchDecode(r, pver, enc); err != nil {
		return err
	}
	if msg.TxBranch, err = readCFHashes(r, pver, MaxCFTreeDepth, field); err != nil {
		return err
	}
	msg.PrevBlockHash, msg.PrevTxIndex, msg.PrevTx, msg.PrevTxBranch = chainhash.Hash{}, 0, nil, nil
	if !msg.Leaf.Spent() {
		return nil
	}
	if err := readElements(r, &msg.PrevBlockHash, &msg.PrevTxIndex); err != nil {
		return err
	}
	msg.PrevTx = new(MsgTx)
	if err := msg.PrevTx.BchDecode(r, pver, enc); err != nil {
		return err
	}
	msg.PrevTxBranch, err = readCFHashes(r, pver, MaxCFTreeDepth, field)
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgCFProof) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	const field = "MsgCFProof.BchEncode"
	if msg.LeafIndex >= msg.LeafCount || msg.Tx == nil || msg.Leaf.Spent() != (msg.PrevTx != nil) {
		str := fmt.Sprintf("leaf %d of %d, with a transaction %v and a spent "+
			"output's transaction %v, for a leaf of kind %d", msg.LeafIndex,
			msg.LeafCount, msg.Tx != nil, msg.PrevTx != nil, msg.Leaf[CFLeafSize-1])
		return messageError(field, str)
	}
	err := writeElements(w, msg.FilterType, &msg.BlockHash, msg.LeafCount, msg.LeafIndex)
	if err != nil {
		return err
	}
	if _, err := w.Write(msg.Leaf[:]); err != nil {
		return err
	}
	if err := writeCFHashes(w, pver, msg.LeafBranch, MaxCFTreeDepth, field); err != nil {
		return err
	}
	if err := msg.Tx.BchEncode(w, pver, enc); err != nil {
		return err
	}
	if err := writeCFHashes(w, pver, msg.TxBranch, MaxCFTreeDepth, field); err != nil {
		return err
	}
	if msg.PrevTx == nil {
		return nil
	}
	if err := writeElements(w, &msg.PrevBlockHash, msg.PrevTxIndex); err != nil {
		return err
	}
	if err := msg.PrevTx.BchEncode(w, pver, enc); err != nil {
		return err
	}
	return writeCFHashes(w, pver, msg.PrevTxBranch, MaxCFTreeDepth, field)
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgCFProof) Command() string {
	return CmdCFProof
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgCFProof) MaxPayloadLength(pver uint32) uint32 {
	return maxMessagePayload()
}
