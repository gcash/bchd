package wire

import (
	"github.com/gcash/bchd/chaincfg/chainhash"
	"io"
)

// VoteRecord defines an avalanche vote record. The vote field is true
// if this is an affirmative vote.
type VoteRecord struct {
	Vote bool
	Hash chainhash.Hash
}

// Serialize returns a serialization of the VoteRecord
func (vr *VoteRecord) Serialize() []byte {
	var ser []byte
	if vr.Vote {
		ser = []byte{0x01}
	} else {
		ser = []byte{0x00}
	}
	ser = append(ser, vr.Hash.CloneBytes()...)
	return ser
}

// NewVoteRecord returns a new VoteRecord using the provided vote and hash.
func NewVoteRecord(vote bool, hash *chainhash.Hash) *VoteRecord {
	return &VoteRecord{
		Vote: vote,
		Hash: *hash,
	}
}

// readVoteRecord reads an encoded VoteRecord from r depending on the protocol
// version.
func readVoteRecord(r io.Reader, pver uint32, vr *VoteRecord) error {
	return readElements(r, &vr.Vote, &vr.Hash)
}

// writeVoteRecord serializes a VoteRecord to w depending on the protocol version.
func writeVoteRecord(w io.Writer, pver uint32, vr *VoteRecord) error {
	return writeElements(w, vr.Vote, &vr.Hash)
}
