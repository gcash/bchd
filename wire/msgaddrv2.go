// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"
)

// MaxV2AddrPerMsg is the maximum number of addresses that can be in a single
// addrv2 message (MsgAddrV2).
const MaxV2AddrPerMsg = 1000

// MsgAddrV2 implements the Message interface and represents a bitcoin cash
// addrv2 message as defined by BIP155.  It is the addr message for peers that
// sent sendaddrv2 and can carry addresses of networks other than IPv4 and
// IPv6, such as Tor v3.  Addresses of unknown networks are skipped when
// decoding.
type MsgAddrV2 struct {
	AddrList []*NetAddressV2
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgAddrV2) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Limit to max addresses per message.
	if count > MaxV2AddrPerMsg {
		str := fmt.Sprintf("too many addresses for message "+
			"[count %v, max %v]", count, MaxV2AddrPerMsg)
		return messageError("MsgAddrV2.BchDecode", str)
	}

	// Bound the allocation by the payload.
	n := allocCount(r, count, minNetAddressV2Payload, count)
	addrList := make([]NetAddressV2, n)
	msg.AddrList = make([]*NetAddressV2, 0, n)
	for i := range count {
		na := &addrList[i]
		err := readNetAddressV2(r, pver, na)
		if err == errSkipAddrV2 {
			continue
		}
		if err != nil {
			return err
		}
		msg.AddrList = append(msg.AddrList, na)
	}
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgAddrV2) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	count := len(msg.AddrList)
	if count > MaxV2AddrPerMsg {
		str := fmt.Sprintf("too many addresses for message "+
			"[count %v, max %v]", count, MaxV2AddrPerMsg)
		return messageError("MsgAddrV2.BchEncode", str)
	}

	err := WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}

	for _, na := range msg.AddrList {
		err = writeNetAddressV2(w, pver, na)
		if err != nil {
			return err
		}
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgAddrV2) Command() string {
	return CmdAddrV2
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgAddrV2) MaxPayloadLength(pver uint32) uint32 {
	return MaxVarIntPayload + MaxV2AddrPerMsg*maxNetAddressV2Payload
}

// NewMsgAddrV2 returns a new bitcoin addrv2 message that conforms to the
// Message interface.  See MsgAddrV2 for details.
func NewMsgAddrV2() *MsgAddrV2 {
	return &MsgAddrV2{
		AddrList: make([]*NetAddressV2, 0, MaxV2AddrPerMsg),
	}
}
