// Copyright (c) 2019 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"io"
)

// MsgXVersion implements the Message interface and represents a bitcoin xversion
// message. This is a stub used to support BU clients.
type MsgXVersion struct {
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
//
// This is part of the Message interface implementation.
func (msg *MsgXVersion) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgXVersion) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgXVersion) Command() string {
	return CmdXVersion
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgXVersion) MaxPayloadLength(pver uint32) uint32 {
	// May payload size for xversion messages is defined as
	// 100000 in the spec.
	return 100000
}

// NewMsgXVersion returns a new bitcoin xversion message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgXVersion() *MsgXVersion {
	return &MsgXVersion{}
}
