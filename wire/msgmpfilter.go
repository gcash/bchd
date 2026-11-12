// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// A node gathers the transactions it admits to its mempool into short
// intervals and builds a script filter (filter type 1, gcash/bchd#767) for
// each. A wallet that sent sendmpfilter gets each interval's filter in an
// mpfilter message as the interval closes, matches it with its own scripts
// and token categories, and asks for the interval's transactions with
// getmptxs only when it matches. Nothing the wallet watches reaches the node.

// MpIntervalHash returns the hash of an interval of mempool transactions: the
// double SHA-256 of their transaction IDs, in the order the node admitted
// them. The interval's filter is keyed by it, and a wallet checks the
// transactions it gets for the interval against it.
func MpIntervalHash(txids []chainhash.Hash) chainhash.Hash {
	var b bytes.Buffer
	b.Grow(chainhash.HashSize * len(txids))
	for i := range txids {
		b.Write(txids[i][:])
	}
	return chainhash.DoubleHashH(b.Bytes())
}

// MsgSendMpFilter implements the Message interface and represents a
// sendmpfilter message. It asks the node to send the filter of each interval
// of transactions it admits to its mempool, as the interval closes.
type MsgSendMpFilter struct {
	FilterType FilterType
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgSendMpFilter) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	return readElement(r, &msg.FilterType)
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgSendMpFilter) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	return writeElement(w, msg.FilterType)
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgSendMpFilter) Command() string {
	return CmdSendMpFilter
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgSendMpFilter) MaxPayloadLength(pver uint32) uint32 {
	return 1
}

// NewMsgSendMpFilter returns a new sendmpfilter message.
func NewMsgSendMpFilter(filterType FilterType) *MsgSendMpFilter {
	return &MsgSendMpFilter{FilterType: filterType}
}

// MsgGetMpFilters implements the Message interface and represents a
// getmpfilters message. It asks for the filters of the intervals the node
// still keeps from sequence number StartSeq on, as a wallet that reconnects
// does to catch up.
type MsgGetMpFilters struct {
	FilterType FilterType
	StartSeq   uint32
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetMpFilters) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	return readElements(r, &msg.FilterType, &msg.StartSeq)
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetMpFilters) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	return writeElements(w, msg.FilterType, msg.StartSeq)
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgGetMpFilters) Command() string {
	return CmdGetMpFilters
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgGetMpFilters) MaxPayloadLength(pver uint32) uint32 {
	return 1 + 4
}

// NewMsgGetMpFilters returns a new getmpfilters message.
func NewMsgGetMpFilters(filterType FilterType, startSeq uint32) *MsgGetMpFilters {
	return &MsgGetMpFilters{FilterType: filterType, StartSeq: startSeq}
}

// MsgMpFilter implements the Message interface and represents an mpfilter
// message: the filter of one interval of the transactions a node admitted to
// its mempool.
type MsgMpFilter struct {
	FilterType FilterType

	// Seq numbers the interval. It grows by one for each interval with
	// transactions, and starts from the time the node started, in seconds,
	// so it keeps growing across restarts.
	Seq uint32

	// Time is when the interval closed, in seconds since 1970.
	Time int64

	// IntervalHash is MpIntervalHash of the interval's transactions. The
	// filter is keyed by it.
	IntervalHash chainhash.Hash

	// TxCount is the number of transactions in the interval.
	TxCount uint32

	// Data is the serialized filter.
	Data []byte
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgMpFilter) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	err := readElements(r, &msg.FilterType, &msg.Seq, &msg.Time, &msg.IntervalHash, &msg.TxCount)
	if err != nil {
		return err
	}
	msg.Data, err = ReadVarBytes(r, pver, maxCFilterDataSize(), "mpfilter data")
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgMpFilter) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	if size := len(msg.Data); size > int(maxCFilterDataSize()) {
		str := fmt.Sprintf("mpfilter size too large for message [size %v, max %v]",
			size, maxCFilterDataSize())
		return messageError("MsgMpFilter.BchEncode", str)
	}
	err := writeElements(w, msg.FilterType, msg.Seq, msg.Time, &msg.IntervalHash, msg.TxCount)
	if err != nil {
		return err
	}
	return WriteVarBytes(w, pver, msg.Data)
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgMpFilter) Command() string {
	return CmdMpFilter
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgMpFilter) MaxPayloadLength(pver uint32) uint32 {
	// Filter type + sequence + time + interval hash + count + filter.
	return 1 + 4 + 8 + chainhash.HashSize + 4 +
		uint32(VarIntSerializeSize(uint64(maxCFilterDataSize()))) + maxCFilterDataSize()
}

// MsgGetMpTxs implements the Message interface and represents a getmptxs
// message. It asks for the transactions of one interval, as a wallet does
// when the interval's filter matches.
type MsgGetMpTxs struct {
	FilterType FilterType
	Seq        uint32
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetMpTxs) BchDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	return readElements(r, &msg.FilterType, &msg.Seq)
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetMpTxs) BchEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	return writeElements(w, msg.FilterType, msg.Seq)
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgGetMpTxs) Command() string {
	return CmdGetMpTxs
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgGetMpTxs) MaxPayloadLength(pver uint32) uint32 {
	return 1 + 4
}

// NewMsgGetMpTxs returns a new getmptxs message.
func NewMsgGetMpTxs(filterType FilterType, seq uint32) *MsgGetMpTxs {
	return &MsgGetMpTxs{FilterType: filterType, Seq: seq}
}

// MpTx is one transaction of an interval, in an mptxs message: the
// transaction while it is in the node's mempool, or only its ID once it left,
// mined or dropped.
type MpTx struct {
	Tx   *MsgTx
	TxID chainhash.Hash
}

// MsgMpTxs implements the Message interface and represents an mptxs message,
// the reply to getmptxs: every transaction of the interval, in the order the
// node admitted them, so that their IDs give the interval's hash.
type MsgMpTxs struct {
	FilterType FilterType
	Seq        uint32
	Txs        []MpTx
}

// mpTxMinSize is the fewest bytes a transaction takes in an mptxs message:
// a flag and a transaction ID.
const mpTxMinSize = 1 + chainhash.HashSize

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgMpTxs) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if err := readElements(r, &msg.FilterType, &msg.Seq); err != nil {
		return err
	}
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if max := uint64(maxMessagePayload()) / mpTxMinSize; count > max {
		str := fmt.Sprintf("too many transactions in mptxs [count %v, max %v]", count, max)
		return messageError("MsgMpTxs.BchDecode", str)
	}
	msg.Txs = make([]MpTx, 0, allocCount(r, count, mpTxMinSize, count))
	for range count {
		var inPool bool
		if err := readElement(r, &inPool); err != nil {
			return err
		}
		var entry MpTx
		if inPool {
			entry.Tx = new(MsgTx)
			if err := entry.Tx.BchDecode(r, pver, enc); err != nil {
				return err
			}
			entry.TxID = entry.Tx.TxHash()
		} else if err := readElement(r, &entry.TxID); err != nil {
			return err
		}
		msg.Txs = append(msg.Txs, entry)
	}
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgMpTxs) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if err := writeElements(w, msg.FilterType, msg.Seq); err != nil {
		return err
	}
	if err := WriteVarInt(w, pver, uint64(len(msg.Txs))); err != nil {
		return err
	}
	for i := range msg.Txs {
		entry := &msg.Txs[i]
		if err := writeElement(w, entry.Tx != nil); err != nil {
			return err
		}
		if entry.Tx != nil {
			if err := entry.Tx.BchEncode(w, pver, enc); err != nil {
				return err
			}
		} else if err := writeElement(w, &entry.TxID); err != nil {
			return err
		}
	}
	return nil
}

// Command returns the protocol command string for the message. This is part
// of the Message interface implementation.
func (msg *MsgMpTxs) Command() string {
	return CmdMpTxs
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgMpTxs) MaxPayloadLength(pver uint32) uint32 {
	return maxMessagePayload()
}
