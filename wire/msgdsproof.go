package wire

import (
	"bytes"
	"fmt"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"io"
)

// MsgDSProof implements the Message interface and represents a Bitcoin dsproof
// message.  It is relayed to all peers upon detecting a double spend.
type MsgDSProof struct {
	TxInPrevHash  chainhash.Hash
	TxInPrevIndex uint32
	FirstSpender  Spender
	DoubleSpender Spender
}

// Spender contains a proof that a given input was was signed, but it intentionally
// does not provide enough data to reconstruct the full transaction. Only enough
// to validate the signature.
type Spender struct {
	Version         int32
	Sequence        uint32
	LockTime        uint32
	HashPrevOutputs chainhash.Hash
	HashSequence    chainhash.Hash
	HashOutputs     chainhash.Hash
	PushData        [][]byte
}

// BchDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgDSProof) BchDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if pver < DoubleSpendProofVersion {
		str := fmt.Sprintf("dsproof message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgDSProof.BchDecode", str)
	}

	_, err := io.ReadFull(r, msg.TxInPrevHash[:])
	if err != nil {
		return err
	}

	msg.TxInPrevIndex, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}

	firstSpenderVersion, err := binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}
	msg.FirstSpender.Version = int32(firstSpenderVersion)

	firstSpenderSequence, err := binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}
	msg.FirstSpender.Sequence = firstSpenderSequence

	firstSpenderLocktime, err := binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}
	msg.FirstSpender.LockTime = firstSpenderLocktime

	_, err = io.ReadFull(r, msg.FirstSpender.HashPrevOutputs[:])
	if err != nil {
		return err
	}

	_, err = io.ReadFull(r, msg.FirstSpender.HashSequence[:])
	if err != nil {
		return err
	}

	_, err = io.ReadFull(r, msg.FirstSpender.HashOutputs[:])
	if err != nil {
		return err
	}

	numFirstSpenderPushDatas, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	msg.FirstSpender.PushData = make([][]byte, numFirstSpenderPushDatas)

	for i := 0; i < int(numFirstSpenderPushDatas); i++ {
		msg.FirstSpender.PushData[i], err = ReadVarBytes(r, pver, 512,
			"First spender push data")
		if err != nil {
			return err
		}
	}

	doubleSpenderVersion, err := binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}
	msg.FirstSpender.Version = int32(doubleSpenderVersion)

	doubleSpenderSequence, err := binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}
	msg.FirstSpender.Sequence = doubleSpenderSequence

	doubleSpenderLocktime, err := binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}
	msg.DoubleSpender.LockTime = doubleSpenderLocktime

	_, err = io.ReadFull(r, msg.DoubleSpender.HashPrevOutputs[:])
	if err != nil {
		return err
	}

	_, err = io.ReadFull(r, msg.DoubleSpender.HashSequence[:])
	if err != nil {
		return err
	}

	_, err = io.ReadFull(r, msg.DoubleSpender.HashOutputs[:])
	if err != nil {
		return err
	}

	numDoubleSpenderPushDatas, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	msg.DoubleSpender.PushData = make([][]byte, numDoubleSpenderPushDatas)

	for i := 0; i < int(numDoubleSpenderPushDatas); i++ {
		msg.DoubleSpender.PushData[i], err = ReadVarBytes(r, pver, 512,
			"Double spender push data")
		if err != nil {
			return err
		}
	}

	return nil
}

// BchEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgDSProof) BchEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if pver < DoubleSpendProofVersion {
		str := fmt.Sprintf("dsproof message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgDSProof.BchDecode", str)
	}

	err := writeElements(w,
		&msg.TxInPrevHash,
		msg.TxInPrevIndex,
		msg.FirstSpender.Version,
		msg.FirstSpender.Sequence,
		msg.FirstSpender.LockTime,
		&msg.FirstSpender.HashPrevOutputs,
		&msg.FirstSpender.HashSequence,
		&msg.FirstSpender.HashOutputs,
	)
	if err != nil {
		return err
	}

	if err := WriteVarInt(w, pver, uint64(len(msg.FirstSpender.PushData))); err != nil {
		return err
	}

	for _, elem := range msg.FirstSpender.PushData {
		if err := WriteVarBytes(w, pver, elem); err != nil {
			return err
		}
	}
	err = writeElements(w,
		msg.DoubleSpender.Version,
		msg.DoubleSpender.Sequence,
		msg.DoubleSpender.LockTime,
		&msg.DoubleSpender.HashPrevOutputs,
		&msg.DoubleSpender.HashSequence,
		&msg.DoubleSpender.HashOutputs,
	)
	if err != nil {
		return err
	}

	if err := WriteVarInt(w, pver, uint64(len(msg.DoubleSpender.PushData))); err != nil {
		return err
	}

	for _, elem := range msg.DoubleSpender.PushData {
		if err := WriteVarBytes(w, pver, elem); err != nil {
			return err
		}
	}
	return nil
}

// ProofHash generates the Hash for the message.
func (msg *MsgDSProof) ProofHash() chainhash.Hash {
	// Encode the proof and calculate double sha256 on the result.
	// Ignore the error returns since the only way the encode could fail
	// is being out of memory or due to nil pointers, both of which would
	// cause a run-time panic.
	var buf bytes.Buffer
	msg.BchEncode(&buf, ProtocolVersion, BaseEncoding)
	return chainhash.DoubleHashH(buf.Bytes())
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgDSProof) Command() string {
	return CmdDSProof
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgDSProof) MaxPayloadLength(pver uint32) uint32 {
	// 32 + 4 + (2 * (4 + 4 + 4 + 32 + 32 + 32 + 9 + txscript.MaxScriptSize))
	return 20270
}

// NewMsgDSProof returns a new bitcoin MsgDSProof message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgDSProof(outpoint OutPoint, firstSpender Spender, doubleSpender Spender) *MsgDSProof {
	return &MsgDSProof{
		TxInPrevHash:  outpoint.Hash,
		TxInPrevIndex: outpoint.Index,
		FirstSpender:  firstSpender,
		DoubleSpender: doubleSpender,
	}
}
