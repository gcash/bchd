// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// MsgCFMempool implements the Message interface and represents a bitcoin cfmempool
// message. It is used to deliver a committed filter in response to a
// getcfmempool (MsgGetCFMempool) message.
type MsgCFMempool struct {
	FilterType FilterType
	Data       []byte
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgCFMempool) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	// Read filter type
	err := readElement(r, &msg.FilterType)
	if err != nil {
		return err
	}

	// Read filter data
	msg.Data, err = ReadVarBytes(r, pver, MaxCFilterDataSize,
		"cfmempool data")
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgCFMempool) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	size := len(msg.Data)
	if size > MaxCFilterDataSize {
		str := fmt.Sprintf("cfmempool size too large for message "+
			"[size %v, max %v]", size, MaxCFilterDataSize)
		return messageError("MsgCFMempool.BchEncode", str)
	}

	err := writeElement(w, msg.FilterType)
	if err != nil {
		return err
	}

	return WriteVarBytes(w, pver, msg.Data)
}

// Deserialize decodes a filter from r into the receiver using a format that is
// suitable for long-term storage such as a database. This function differs
// from BchDecode in that BchDecode decodes from the bitcoin wire protocol as
// it was sent across the network.  The wire encoding can technically differ
// depending on the protocol version and doesn't even really need to match the
// format of a stored filter at all. As of the time this comment was written,
// the encoded filter is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
func (msg *MsgCFMempool) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// and the stable long-term storage format.  As a result, make use of
	// BchDecode.
	return msg.BchDecode(r, 0, BaseEncoding)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgCFMempool) Command() string {
	return CmdCFMempool
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgCFMempool) MaxPayloadLength(pver uint32) uint32 {
	return uint32(VarIntSerializeSize(MaxCFilterDataSize)) +
		MaxCFilterDataSize + chainhash.HashSize + 1
}

// NewMsgCFMempool returns a new bitcoin cfmempool message that conforms to the
// Message interface. See MsgCFMempool for details.
func NewMsgCFMempool(filterType FilterType, data []byte) *MsgCFilter {
	return &MsgCFilter{
		FilterType: filterType,
		Data:       data,
	}
}
