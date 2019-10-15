// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"io"

	"github.com/gcash/bchd/bchec"
)

// MsgAvalanchePubkey implements the Message interface and represents a bitcoin
// avalanche pubkey message.
type MsgAvaPubkey struct {
	Pubkey    *bchec.PublicKey
	Signature *bchec.Signature
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgAvaPubkey) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	pubkeyBytes := make([]byte, 33)
	if _, err := io.ReadFull(r, pubkeyBytes); err != nil {
		return err
	}
	sigLen, err := ReadVarInt(r, ProtocolVersion)
	if err != nil {
		return err
	}
	sigBytes := make([]byte, sigLen)
	if _, err := io.ReadFull(r, sigBytes); err != nil {
		return err
	}
	msg.Pubkey, err = bchec.ParsePubKey(pubkeyBytes, bchec.S256())
	if err != nil {
		return err
	}
	msg.Signature, err = bchec.ParseSchnorrSignature(sigBytes)
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgAvaPubkey) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	_, err := w.Write(msg.Pubkey.SerializeCompressed())
	if err != nil {
		return err
	}
	sig := msg.Signature.Serialize()
	if err := WriteVarInt(w, ProtocolVersion, uint64(len(sig))); err != nil {
		return err
	}
	_, err = w.Write(sig)
	return err
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgAvaPubkey) Command() string {
	return CmdAvaPubkey
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgAvaPubkey) MaxPayloadLength(pver uint32) uint32 {
	return uint32(106)
}

// NewMsgAvaPubkey returns a new bitcoin avalanche pubkey message that conforms to the Message
// interface.  See NewMsgAvalanchePubkey for details.
func NewMsgAvaPubkey(pubkey *bchec.PublicKey, signature *bchec.Signature) *MsgAvaPubkey {
	return &MsgAvaPubkey{
		Pubkey:    pubkey,
		Signature: signature,
	}
}
