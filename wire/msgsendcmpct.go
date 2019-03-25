package wire

import (
	"fmt"
	"io"
)

// CompactBlocksProtocolVersion is the current version of the compact blocks protocol
const CompactBlocksProtocolVersion = 1

// MsgSendCmpct implements the Message interface and represents a Bitcoin sendcmpct
// message.  It is sent to the remove peer immediately after receiving a a version
// message to signal that they wish to receive compact blocks.
type MsgSendCmpct struct {
	// If announce is set to false the receive node must announce new blocks
	// via the standard inv relay. If announce is true, a new Compact Block
	// can be pushed directly to the peer.
	Announce bool

	// The version of this protocol is currently 1.
	Version uint64
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgSendCmpct) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if pver < BIP0152Version {
		str := fmt.Sprintf("sendcmpct message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgSendCmpct.BchDecode", str)
	}

	if err := readElement(r, &msg.Announce); err != nil {
		return err
	}

	if err := readElement(r, &msg.Version); err != nil {
		return err
	}
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgSendCmpct) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if pver < BIP0152Version {
		str := fmt.Sprintf("sendcmpct message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgSendCmpct.BchDecode", str)
	}

	if err := writeElement(w, msg.Announce); err != nil {
		return err
	}

	if err := writeElement(w, msg.Version); err != nil {
		return err
	}
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgSendCmpct) Command() string {
	return CmdSendCmpct
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgSendCmpct) MaxPayloadLength(pver uint32) uint32 {
	// One byte bool and eight byte uint64
	return 9
}

// NewMsgSendCmpct returns a new bitcoin sendcmpct message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgSendCmpct(announce bool, version uint64) *MsgSendCmpct {
	return &MsgSendCmpct{Announce: announce, Version: version}
}
