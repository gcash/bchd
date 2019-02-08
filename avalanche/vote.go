package avalanche

import "time"

// Status is the status of consensus on a particular target
type Status int

const (
	// StatusInvalid means the target is invalid
	StatusInvalid Status = iota

	// StatusRejected means the target is been deemed to be rejected
	StatusRejected

	// StatusAccepted means the target is been deemed to be accepted
	StatusAccepted

	// StatusFinalized means the consensus on the target is been finalized
	StatusFinalized
)

// VoteRecord keeps track of a series of votes for a target
type VoteRecord struct {
	txdesc           *TxDesc
	votes            uint8
	consider         uint8
	confidence       uint16
	inflightRequests uint8
	timestamp        time.Time
}

// NewVoteRecord instantiates a new base record for voting on a target
// `accepted` indicates whether or not the initial state should be acceptance
func NewVoteRecord(txdesc *TxDesc, accepted bool) *VoteRecord {
	return &VoteRecord{txdesc: txdesc, confidence: boolToUint16(accepted), timestamp: time.Now()}
}

// isAccepted returns whether or not the voted state is acceptance or not
func (vr VoteRecord) isAccepted() bool {
	return (vr.confidence & 0x01) == 1
}

// getConfidence returns the confidence in the current state's finalization
func (vr VoteRecord) getConfidence() uint16 {
	return vr.confidence >> 1
}

// hasFinalized returns whether or not the record has finalized a state
func (vr VoteRecord) hasFinalized() bool {
	return vr.getConfidence() >= AvalancheFinalizationScore
}

// regsiterVote adds a new vote for an item and update confidence accordingly.
// Returns true if the acceptance or finalization state changed.
func (vr *VoteRecord) regsiterVote(vote uint8) bool {
	vr.votes = (vr.votes << 1) | boolToUint8(vote == 1)
	vr.consider = (vr.consider << 1) | boolToUint8(int8(vote) >= 0)

	yes := countBits8(vr.votes&vr.consider&0xff) > 6

	// The round is inconclusive
	if !yes {
		no := countBits8((-vr.votes-1)&vr.consider&0xff) > 6
		if !no {
			return false
		}
	}

	// Vote is conclusive and agrees with our current state
	if vr.isAccepted() == yes {
		vr.confidence += 2
		return vr.getConfidence() == AvalancheFinalizationScore
	}

	// Vote is conclusive but does not agree with our current state
	vr.confidence = boolToUint16(yes)

	return true
}

func (vr *VoteRecord) status() (status Status) {
	finalized := vr.hasFinalized()
	accepted := vr.isAccepted()
	switch {
	case !finalized && accepted:
		status = StatusAccepted
	case !finalized && !accepted:
		status = StatusRejected
	case finalized && accepted:
		status = StatusFinalized
	case finalized && !accepted:
		status = StatusInvalid
	}
	return status
}

func countBits8(i uint8) (count int) {
	for ; i > 0; i &= (i - 1) {
		count++
	}
	return count
}

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func boolToUint16(b bool) uint16 {
	return uint16(boolToUint8(b))
}
