// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// FilterType is used to represent a filter type.
type FilterType uint8

const (
	// GCSFilterRegular is the regular filter type.
	GCSFilterRegular FilterType = iota
)

const (
	// MaxCFilterDataSize is the least maximum byte size of a committed
	// filter, 4MiB. This allows it to accomodate a large mempool.  The
	// maximum is larger for the filters of the largest blocks, see
	// maxCFilterDataSize.
	MaxCFilterDataSize = 4 * 1024 * 1024

	// gcsFilterP and gcsFilterM are the Golomb-Rice coding parameters of
	// the regular filter, those of BIP158's basic filter.
	gcsFilterP = 19
	gcsFilterM = 784931
)

// maxCFilterDataSize returns the maximum byte size of a committed filter: that
// of the regular filter of the largest block a block message may hold, or
// MaxCFilterDataSize when larger.  The filter holds the outpoints a block
// spends and its distinct non-empty output scripts, and each of them takes at
// least 10 bytes of the block, so a block of B bytes gives at most B/10
// entries.  Each entry takes P+1 bits and its quotient, and the quotients add
// up to less than M/2^P bits per entry, after the entry count's at most 5
// bytes.
func maxCFilterDataSize() uint32 {
	entries := uint64(MaxBlockPayload()) / 10
	bits := entries*(gcsFilterP+1) + entries*gcsFilterM>>gcsFilterP
	return uint32(max(MaxCFilterDataSize, 5+(bits+7)/8))
}

// MsgCFilter implements the Message interface and represents a bitcoin cfilter
// message. It is used to deliver a committed filter in response to a
// getcfilters (MsgGetCFilters) message.
type MsgCFilter struct {
	FilterType FilterType
	BlockHash  chainhash.Hash
	Data       []byte
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgCFilter) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	// Read filter type
	err := readElement(r, &msg.FilterType)
	if err != nil {
		return err
	}

	// Read the hash of the filter's block
	err = readElement(r, &msg.BlockHash)
	if err != nil {
		return err
	}

	// Read filter data
	msg.Data, err = ReadVarBytes(r, pver, maxCFilterDataSize(),
		"cfilter data")
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgCFilter) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	size := len(msg.Data)
	if maxSize := maxCFilterDataSize(); uint64(size) > uint64(maxSize) {
		str := fmt.Sprintf("cfilter size too large for message "+
			"[size %v, max %v]", size, maxSize)
		return messageError("MsgCFilter.BchEncode", str)
	}

	err := writeElement(w, msg.FilterType)
	if err != nil {
		return err
	}

	err = writeElement(w, msg.BlockHash)
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
func (msg *MsgCFilter) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// and the stable long-term storage format.  As a result, make use of
	// BchDecode.
	return msg.BchDecode(r, 0, BaseEncoding)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgCFilter) Command() string {
	return CmdCFilter
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgCFilter) MaxPayloadLength(pver uint32) uint32 {
	maxSize := maxCFilterDataSize()
	return uint32(VarIntSerializeSize(uint64(maxSize))) + maxSize +
		chainhash.HashSize + 1
}

// NewMsgCFilter returns a new bitcoin cfilter message that conforms to the
// Message interface. See MsgCFilter for details.
func NewMsgCFilter(filterType FilterType, blockHash *chainhash.Hash,
	data []byte) *MsgCFilter {
	return &MsgCFilter{
		FilterType: filterType,
		BlockHash:  *blockHash,
		Data:       data,
	}
}
