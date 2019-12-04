package avalanche

import (
	"sync/atomic"
	"time"
)

const (
	voteNo byte = iota
	voteYes
	voteAbstain

	// finalizationScore is the confidence score we consider to be final.
	finalizationScore uint16 = 128

	// maxInflightQueries is the max number of queries to maintain
	// simultaneously for a vote record. It's stored in packed format as a uin32.
	maxInflightQueries uint32 = 10 << 28 // 10 requests in packed format
)

const (
	typeTx vertexType = iota
	typeBlock
)

const (
	// Each state carries 2 data points, acceptance and finalization, encoded as
	// a bitmap in the lowest 2 bits of a byte. bit 0 is acceptance and bit 1 is
	// finalization
	stateRejected state = iota
	stateAccepted
	stateFinalizedRejected
	stateFinalizedAccepted
)

var (
	typeStrings  = [2]string{"tx", "block"}
	stateStrings = [2]string{"rejected", "accepted"}
)

type (
	state      uint8
	vertexType int
)

func (vt vertexType) String() string { return typeStrings[vt] }
func (s state) String() string       { return stateStrings[s&1] }

// VoteRecord keeps track of a series of votes for a target
type VoteRecord struct {
	// votes is a bitmap of the last 8 votes. 'yes' votes are 1 and 'no' votes are
	// 0. Abstaining votes are undefined.
	votes uint8

	// consider is a bitmap of the whether or not the last 8 votes abstained. A 0
	// is an abstention and a 1 is a real vote which is stored in the same the bit
	// place of the votes bitmap.
	consider uint8

	// confidence is a compound property of 2 values. The lowest bit is a bool
	// meaning accepted or rejected. The high 15 bits are treated as a uint16.
	confidence uint16

	// other is a compound property of 4 values.
	// bit 0:       Vertex type
	// bit 1:       Initial state, 1 for accepted and 0 for rejected
	// bits 2-27:   26 bits of started at time minus the global time offset
	// bits 28-31:  4 bits of inflight request count
	//
	// It is structured this way because the inflight request count must be 32bits
	// in order to be safely mutated without a mutex, even though it is <= 4 bits
	// of information. This leaves 28 bits wasted. We recycle this space to store
	// the other 3 data fields which are immutable after being set during init.
	other uint32
}

// newVoteRecord creates a new Snowball with the given initial acceptance.
func newVoteRecord(t vertexType, isAccepted bool) *VoteRecord {
	return &VoteRecord{
		confidence: boolToUint16(isAccepted),
		other:      packVoteRecordOtherParts(t == typeBlock, isAccepted, clock.now().Unix()),
	}
}

// Getters for confidence field
func (vr VoteRecord) getConfidence() uint16 { return vr.confidence >> 1 }
func (vr VoteRecord) isAccepted() bool      { return vr.confidence&0x01 == 1 }
func (vr VoteRecord) vote() byte            { return byte(vr.confidence & 0x01) }
func (vr VoteRecord) isFinalized() bool     { return vr.getConfidence() >= finalizationScore }
func (vr VoteRecord) state() state {
	return state(boolToUint16(vr.isFinalized())<<1 | vr.confidence&0x01)
}

// Getters for other field
func (vr VoteRecord) getType() vertexType  { return vertexType(vr.other & 1) }
func (vr VoteRecord) getStartState() state { return state(vr.other >> 1 & 1) }
func (vr VoteRecord) getStartTime() int64  { return globalTimeOffset + int64(vr.other>>2&0x3ffffff) }
func (vr VoteRecord) getInflight() uint8   { return uint8(vr.other >> 28) }
func (vr VoteRecord) getAge() int64 {
	return int64(clock.now().Sub(time.Unix(vr.getStartTime(), 0)).Seconds())
}

//
// Mutators
//

func (vr *VoteRecord) resetConfidence() bool {
	if vr != nil && !vr.isFinalized() {
		vr.confidence = 0
		return true
	}
	return false
}

func (vr *VoteRecord) incInflight() bool {
	if vr == nil {
		return false
	}

	count := vr.other             // freeze the count in this var
	newCount := count + (1 << 28) // add 1

	// Ensure new value won't overflow or breach the limit
	if count > 0xEFFFFFFF || count>>28 >= maxInflightQueries {
		return false
	}

	return atomic.CompareAndSwapUint32(&vr.other, count, newCount)
}

func (vr *VoteRecord) decInflight() bool {
	if vr == nil || (vr.other < (1<<28)+1) {
		return false
	}

	return atomic.CompareAndSwapUint32(&vr.other, vr.other, vr.other-(1<<28))
}

// registerVote adds a new vote for an item and update confidence accordingly.
// Returns true if the acceptance or finalization state changed.
func (vr *VoteRecord) registerVote(vote uint8) bool {
	vr.votes = (vr.votes << 1) | vote&1
	vr.consider = (vr.consider << 1) | vote>>1&1 ^ 1

	yes := countBits8(vr.votes&vr.consider&0xff) > 6

	if !yes {
		// The round is inconclusive if there is no quorum for either yes or no
		if !(countBits8((-vr.votes-1)&vr.consider&0xff) > 6) {
			return false
		}
	}

	// Vote is conclusive and agrees with our current state
	if vr.isAccepted() == yes {
		vr.confidence += 2
		return vr.getConfidence() == finalizationScore
	}

	// Vote is conclusive but does not agree with our current state
	vr.confidence = boolToUint16(yes)

	return true
}

//
// Utilities
//

func packVoteRecordOtherParts(isBlock bool, isAccepted bool, startTime int64) (other uint32) {
	other = uint32(startTime - globalTimeOffset)
	other = other<<1 | uint32(boolToUint16(isAccepted))
	other = other<<1 | uint32(boolToUint16(isBlock))
	return other & 0x0FFFFFFF
}

func countBits8(i uint8) (count int) {
	for ; i > 0; i &= (i - 1) {
		count++
	}
	return count
}

func boolToUint16(b bool) uint16 {
	if b {
		return 1
	}
	return 0
}
