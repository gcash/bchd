// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"encoding/binary"
	"io"

	"github.com/gcash/bchd/bchec"
)

// "github.com/gcash/bchd/txscript"
// "github.com/gcash/bchutil"

// MsgAvalanchePubkey implements the Message interface and represents a bitcoin
// avalanche pubkey message.
type MsgAvaPubkey struct {
	version  int8
	sequence int64

	pubKey    *bchec.PublicKey
	outPoints []OutPoint

	identitySignature  *bchec.Signature
	outPointSignatures []*bchec.Signature
	// Message
}

func NewMsgAvaPubkey(pubKey *bchec.PublicKey) *MsgAvaPubkey {
	return &MsgAvaPubkey{pubKey: pubKey}
}

func (msg *MsgAvaPubkey) PubKey() *bchec.PublicKey {
	return msg.pubKey
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgAvaPubkey) Command() string { return CmdAvaPubkey }

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgAvaPubkey) MaxPayloadLength(pver uint32) uint32 { return uint32(2046) }

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgAvaPubkey) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	// Ready version and sequence
	version, err := binarySerializer.Uint8(r)
	if err != nil {
		return err
	}

	sequence, err := binarySerializer.Uint64(r, binary.LittleEndian)
	if err != nil {
		return err
	}

	msg.version = int8(version)
	msg.sequence = int64(sequence)

	pubKeyBytes, err := ReadVarBytes(r, ProtocolVersion, 33, "identity_pubkey")
	if err != nil {
		return err
	}

	msg.pubKey, err = bchec.ParsePubKey(pubKeyBytes, bchec.S256())
	if err != nil {
		return err
	}

	return nil

	// Read outpoints
	outPointCount, err := binarySerializer.Uint8(r)
	if err != nil {
		return err
	}

	msg.outPoints = make([]OutPoint, outPointCount)
	for i := uint8(0); i < outPointCount; i++ {
		msg.outPoints[i] = OutPoint{}
		if err := (&msg.outPoints[i]).Deserialize(r); err != nil {
			return err
		}
	}

	// // Read identity signature
	// sig, err := ReadVarBytes(r, ProtocolVersion, 65, "identity_signature")
	// if err != nil {
	// 	return err
	// }

	// msg.identitySignature, err = bchec.ParseBERSignature(sig, bchec.S256())

	// Read outpoint signatures
	outPointSigCount, err := binarySerializer.Uint8(r)
	if err != nil {
		return err
	}

	msg.outPointSignatures = make([]*bchec.Signature, outPointSigCount)
	for i := uint8(0); i < outPointSigCount; i++ {
		sig, err := ReadVarBytes(r, ProtocolVersion, 65, "outpoint_signature")
		if err != nil {
			return err
		}

		msg.outPointSignatures[i], err = bchec.ParseBERSignature(sig, bchec.S256())
		if err != nil {
			return err
		}
	}
	return err
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgAvaPubkey) BchEncode(w io.Writer, _ uint32, _ MessageEncoding) error {
	// Write version and sequence
	if err := binarySerializer.PutUint8(w, uint8(msg.version)); err != nil {
		return err
	}
	if err := binarySerializer.PutUint64(w, binary.LittleEndian, uint64(msg.sequence)); err != nil {
		return err
	}

	// Write identity pubkey
	if err := WriteVarBytes(w, ProtocolVersion, msg.pubKey.SerializeCompressed()); err != nil {
		return err
	}
	return nil

	// Write outpoints
	if err := binarySerializer.PutUint8(w, uint8(len(msg.outPoints))); err != nil {
		return err
	}
	for _, outpoint := range msg.outPoints {
		if err := outpoint.Serialize(w); err != nil {
			return err
		}
	}

	// // Write identity key signature
	// if err := WriteVarBytes(w, ProtocolVersion, msg.identitySignature.Serialize()); err != nil {
	// 	return err
	// }

	// Write outpoint signatures
	if err := binarySerializer.PutUint8(w, uint8(len(msg.outPointSignatures))); err != nil {
		return err
	}
	for _, sig := range msg.outPointSignatures {
		if err := WriteVarBytes(w, ProtocolVersion, sig.Serialize()); err != nil {
			return err
		}
	}

	return nil
}
