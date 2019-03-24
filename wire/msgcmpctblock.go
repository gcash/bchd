package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/dchest/siphash"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"io"
)

// ShortIDSize is the number of bytes in a short ID.
const ShortIDSize = 6

// PrefilledTx is a transaction that is sent along with the compact block.
// The index is included so we know where it the block it belongs.
type PrefilledTx struct {
	Index uint32
	Tx    *MsgTx
}

// MsgCmpctBlock implements the Message interface and represents a Bitcoin cmpctblock
// message.  When using protocol versions equal to or greater than BIP0152Version we
// save bandwidth on the wire by sending a compact block rather than a full block.
type MsgCmpctBlock struct {
	Header       BlockHeader
	Nonce        uint64
	ShortIDs     [][ShortIDSize]byte
	PrefilledTxs []*PrefilledTx
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgCmpctBlock) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if pver < BIP0152Version {
		str := fmt.Sprintf("cmpctblock message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgCmpctBlock.BchDecode", str)
	}

	if err := readBlockHeader(r, pver, &msg.Header); err != nil {
		return err
	}

	if err := readElement(r, &msg.Nonce); err != nil {
		return err
	}

	shortIDCount, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	for i := uint64(0); i < shortIDCount; i++ {
		shortIDBytes := make([]byte, ShortIDSize)
		_, err = io.ReadFull(r, shortIDBytes)
		if err != nil {
			return err
		}

		var shortID [ShortIDSize]byte
		copy(shortID[:], shortIDBytes[:ShortIDSize])
		msg.ShortIDs = append(msg.ShortIDs, shortID)
	}

	prefilledTxCount, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	for i := uint64(0); i < prefilledTxCount; i++ {
		index, err := ReadVarInt(r, pver)
		if err != nil {
			return err
		}
		tx := MsgTx{}
		err = tx.BchDecode(r, pver, enc)
		if err != nil {
			return err
		}
		ptx := &PrefilledTx{
			Index: uint32(index),
			Tx:    &tx,
		}
		msg.PrefilledTxs = append(msg.PrefilledTxs, ptx)
	}
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgCmpctBlock) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if pver < BIP0152Version {
		str := fmt.Sprintf("cmpctblock message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgCmpctBlock.BchDecode", str)
	}

	if err := writeBlockHeader(w, pver, &msg.Header); err != nil {
		return err
	}
	if err := writeElement(w, &msg.Nonce); err != nil {
		return err
	}

	if err := WriteVarInt(w, pver, uint64(len(msg.ShortIDs))); err != nil {
		return err
	}

	for _, shortID := range msg.ShortIDs {
		w.Write(shortID[:])
	}

	if err := WriteVarInt(w, pver, uint64(len(msg.PrefilledTxs))); err != nil {
		return err
	}

	for _, ptx := range msg.PrefilledTxs {
		if err := WriteVarInt(w, pver, uint64(ptx.Index)); err != nil {
			return err
		}
		if err := ptx.Tx.BchEncode(w, pver, enc); err != nil {
			return err
		}
	}
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgCmpctBlock) Command() string {
	return CmdCmpctBlock
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgCmpctBlock) MaxPayloadLength(pver uint32) uint32 {
	// This can take up to the max payload. The derived block
	// cannot be larger than the excessive block size.
	return maxMessagePayload()
}

// BlockHash computes the block identifier hash for this block.
func (msg *MsgCmpctBlock) BlockHash() chainhash.Hash {
	return msg.Header.BlockHash()
}

// TotalTransactions returns the total number of transactions in
// the block.
func (msg *MsgCmpctBlock) TotalTransactions() int {
	return len(msg.ShortIDs) + len(msg.PrefilledTxs)
}

// NewMsgCmpctBlockFromBlock builds a cmpctblock message from a block
// using a known inventory map. If a given transaction is not in the
// known inventory map, we will append it as a PrefilledTx. Otherwise
// we'll add the short ID of the transaction.
func NewMsgCmpctBlockFromBlock(block *MsgBlock, knownInventory map[chainhash.Hash]bool) (*MsgCmpctBlock, error) {
	nonce, err := RandomUint64()
	if err != nil {
		return nil, err
	}
	msg := &MsgCmpctBlock{
		Header: block.Header,
		Nonce:  nonce,
	}

	var buf bytes.Buffer
	if err := block.Header.Serialize(&buf); err != nil {
		return nil, err
	}

	// To calculate the siphash keys we need to append the little endian
	// nonce to the block header.
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, msg.Nonce)
	headerWithNonce := append(buf.Bytes(), nonceBytes...)

	// Hash the result once with sha256
	headerHash := sha256.Sum256(headerWithNonce)

	// The keys are the first two little endian uint64s in the resulting
	// byte array.
	key0 := binary.LittleEndian.Uint64(headerHash[0:8])
	key1 := binary.LittleEndian.Uint64(headerHash[8:16])

	lastIndex := 0
	for i, tx := range block.Transactions {
		if knownInventory[tx.TxHash()] { // The other peer knows the transaction so we can just send the short IDs.
			txHash := tx.TxHash()
			sum64 := siphash.Hash(key0, key1, txHash.CloneBytes())
			shortIDBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(shortIDBytes, sum64)
			var shortID [ShortIDSize]byte
			copy(shortID[:], shortIDBytes[:ShortIDSize])
			msg.ShortIDs = append(msg.ShortIDs, shortID)
		} else { // The other peer doesn't know the transaction so we just send the full tx.
			ptx := &PrefilledTx{
				Index: uint32(i - lastIndex),
				Tx:    tx,
			}
			lastIndex = i + 1
			msg.PrefilledTxs = append(msg.PrefilledTxs, ptx)
		}
	}
	return msg, nil
}

// NewMsgCmpctBlock returns a new bitcoin cmpctblock message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgCmpctBlock(blockHeader *BlockHeader) *MsgCmpctBlock {
	return &MsgCmpctBlock{
		Header:       *blockHeader,
		ShortIDs:     make([][ShortIDSize]byte, 0, defaultTransactionAlloc),
		PrefilledTxs: make([]*PrefilledTx, 0, defaultTransactionAlloc),
	}
}
