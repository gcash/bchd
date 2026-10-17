package wire

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// MsgGetBlockTxns implements the Message interface and represents a Bitcoin getblocktxn
// message.  It is used to request missing transactions as part of the compact block
// protocol.
type MsgGetBlockTxns struct {
	BlockHash chainhash.Hash
	Indexes   []uint32
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetBlockTxns) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if pver < BIP0152Version {
		str := fmt.Sprintf("getblocktxn message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgGetBlockTxns.BchDecode", str)
	}

	if err := readElement(r, &msg.BlockHash); err != nil {
		return err
	}

	indexCount, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Prevent more indexes than there could possibly be transactions in a
	// block.  The indexes are appended as they are read, so memory use
	// grows with the data actually received rather than with this count.
	if indexCount > uint64(maxTxPerBlock()) {
		str := fmt.Sprintf("too many transaction indexes for message "+
			"[count %d, max %d]", indexCount, maxTxPerBlock())
		return messageError("MsgGetBlockTxns.BchDecode", str)
	}

	// The indexes are differentially encoded.  Reject any whose absolute
	// index does not fit in 32 bits.
	var next uint64
	for range indexCount {
		index, err := ReadVarInt(r, pver)
		if err != nil {
			return err
		}
		if index > math.MaxUint32 || next+index > math.MaxUint32 {
			str := fmt.Sprintf("transaction index overflows 32 "+
				"bits [differential index %d, previous end %d]",
				index, next)
			return messageError("MsgGetBlockTxns.BchDecode", str)
		}
		msg.Indexes = append(msg.Indexes, uint32(index))
		next += index + 1
	}
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetBlockTxns) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if pver < BIP0152Version {
		str := fmt.Sprintf("sendcmpct message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgSendCmpct.BchDecode", str)
	}

	if err := writeElement(w, &msg.BlockHash); err != nil {
		return err
	}

	if err := WriteVarInt(w, pver, uint64(len(msg.Indexes))); err != nil {
		return err
	}

	for _, index := range msg.Indexes {
		if err := WriteVarInt(w, pver, uint64(index)); err != nil {
			return err
		}
	}
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgGetBlockTxns) Command() string {
	return CmdGetBlockTxns
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetBlockTxns) MaxPayloadLength(pver uint32) uint32 {
	// In practice this will always be less than the payload but the number
	// of txs in a block can vary so we really don't know the real max.
	return maxMessagePayload()
}

// RequestedTransactions extracts the transactions that were requested by this
// message from the given block and returns them.
func (msg *MsgGetBlockTxns) RequestedTransactions(block *MsgBlock) ([]*MsgTx, error) {
	var requestedTxs []*MsgTx
	var next uint64
	for _, i := range msg.Indexes {
		index := next + uint64(i)
		if index >= uint64(len(block.Transactions)) {
			return nil, errors.New("transaction index out of range")
		}
		requestedTxs = append(requestedTxs, block.Transactions[index])
		next = index + 1
	}
	return requestedTxs, nil
}

// NewMsgGetBlockTxnsFromBlock parses a block and for each nil transasction
// ads a index to the getblocktxn message that is returned.
func NewMsgGetBlockTxnsFromBlock(block *MsgBlock) *MsgGetBlockTxns {
	msg := &MsgGetBlockTxns{BlockHash: block.BlockHash()}
	lastIndex := 0
	for i, tx := range block.Transactions {
		if tx == nil {
			msg.Indexes = append(msg.Indexes, uint32(i-lastIndex))
			lastIndex = i + 1
		}
	}
	return msg
}

// NewMsgGetBlockTxns returns a new bitcoin getblocktxn message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgGetBlockTxns(blockHash chainhash.Hash, indexes []uint32) *MsgGetBlockTxns {
	return &MsgGetBlockTxns{BlockHash: blockHash, Indexes: indexes}
}
