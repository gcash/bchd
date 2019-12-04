package avalanche

import (
	"testing"
	"time"

	"github.com/tfactorapp/assert"

	"github.com/gcash/bchd/bchrpc/pb"
)

var (
	_negativeOne = -1
	neutral      = uint8(_negativeOne)
	yes          = uint8(0x01)
	no           = uint8(0x00)
)

func TestVoteRecord(t *testing.T) {
	var vr *VoteRecord
	registerVoteAndCheck := func(vote uint8, isAccepted, isFinalized bool, confidence uint16) {
		vr.registerVote(vote)
		assert.True(t, vr.isAccepted() == isAccepted)
		assert.True(t, vr.isFinalized() == isFinalized)
		assert.True(t, vr.getConfidence() == confidence)

		if isAccepted {
			assert.EqualString(t, "accepted", vr.state().String())
			return
		}
		assert.EqualString(t, "rejected", vr.state().String())
	}

	vr = newVoteRecord(typeTx, true)
	assert.True(t, vr.isAccepted())
	assert.False(t, vr.isFinalized())
	assert.EqualInt(t, 0, int(vr.getConfidence()))

	vr = newVoteRecord(typeTx, false)
	assert.False(t, vr.isAccepted())
	assert.False(t, vr.isFinalized())
	assert.EqualInt(t, 0, int(vr.getConfidence()))

	// We need to register 6 positive votes before we start counting.
	for i := uint16(0); i < 6; i++ {
		registerVoteAndCheck(yes, false, false, 0)
	}

	// Next vote will flip state, and confidence will increase as long as we
	// vote yes.
	registerVoteAndCheck(yes, true, false, 0)

	// A single neutral vote do not change anything.
	registerVoteAndCheck(neutral, true, false, 1)
	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(yes, true, false, i)
	}

	// Two neutral votes will stall progress.
	registerVoteAndCheck(neutral, true, false, 7)
	registerVoteAndCheck(neutral, true, false, 7)

	// Hundreds of neutral votes should not finalize it
	for i := 0; i < 400; i++ {
		registerVoteAndCheck(neutral, true, false, 7)
	}

	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(yes, true, false, 7)
	}

	// Now confidence will increase as long as we vote yes.
	for i := uint16(8); i < finalizationScore; i++ {
		registerVoteAndCheck(yes, true, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(no, true, true, finalizationScore)

	// Now that we have two no votes, confidence stop increasing.
	for i := uint16(0); i < 5; i++ {
		registerVoteAndCheck(no, true, true, finalizationScore)
	}

	// Next vote will flip state, and confidence will increase as long as we
	// vote no.
	registerVoteAndCheck(no, false, false, 0)

	// A single neutral vote do not change anything.
	registerVoteAndCheck(neutral, false, false, 1)
	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(no, false, false, i)
	}

	// Two neutral votes will stall progress.
	registerVoteAndCheck(neutral, false, false, 7)
	registerVoteAndCheck(neutral, false, false, 7)
	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(no, false, false, 7)
	}

	// Now confidence will increase as long as we vote no.
	for i := uint16(8); i < finalizationScore; i++ {
		registerVoteAndCheck(no, false, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(yes, false, true, finalizationScore)
}

func TestVoteRecordGetters(t *testing.T) {
	t0 := time.Now().Unix()
	vr := newVoteRecord(typeTx, true)
	t1 := time.Now().Unix()

	assert.True(t, vr.getStartTime() >= t0)
	assert.True(t, vr.getStartTime() <= t1)
	assert.True(t, vr.getStartState() == stateAccepted)
	assert.True(t, vr.getType() == typeTx)

	vr = newVoteRecord(typeTx, false)
	assert.True(t, vr.getStartState() == stateRejected)
	assert.True(t, vr.getType() == typeTx)

	vr = newVoteRecord(typeBlock, true)
	assert.True(t, vr.getStartState() == stateAccepted)
	assert.True(t, vr.getType() == typeBlock)

	vr = newVoteRecord(typeBlock, false)
	assert.True(t, vr.getStartState() == stateRejected)
	assert.True(t, vr.getType() == typeBlock)
}

func TestVoteRecordResetConfidence(t *testing.T) {
	var vr *VoteRecord
	assert.False(t, vr.resetConfidence())

	vr = newVoteRecord(typeTx, false)
	assert.False(t, vr.isFinalized())
	assert.True(t, vr.resetConfidence())

	vr.confidence = finalizationScore
	assert.False(t, vr.isFinalized())
	assert.True(t, vr.resetConfidence())

	vr.confidence = (finalizationScore << 1) - 1
	assert.False(t, vr.isFinalized())
	assert.True(t, vr.resetConfidence())

	vr.confidence = finalizationScore << 1
	assert.True(t, vr.isFinalized())
	assert.False(t, vr.resetConfidence())
}

func TestVoteRecordGetAge(t *testing.T) {
	for i := 0; i < 100; i++ {
		clock = fixedClock(time.Unix(int64(i), 0))
		globalTimeOffset = 0
		vr := newVoteRecord(typeTx, false)
		assert.EqualInt(t, i, int(vr.getStartTime()))
	}

	clock = fixedClock(time.Unix(100, 0))
	globalTimeOffset = 42
	vr := newVoteRecord(typeTx, false)
	assert.EqualInt(t, 100, int(vr.getStartTime()))
	assert.EqualInt(t, 0, int(vr.getAge()))
	clock = fixedClock(time.Unix(101, 0))
	assert.EqualInt(t, int(time.Second.Nanoseconds()), int(vr.getAge()))
	clock = fixedClock(time.Unix(110, 0))
	assert.EqualInt(t, int(10*time.Second.Nanoseconds()), int(vr.getAge()))
}

func TestVoteRecordInflight(t *testing.T) {
	// It's always false for nil pointers
	var vr *VoteRecord
	assert.False(t, vr.incInflight())
	assert.False(t, vr.decInflight())

	// Set all bits and count down
	vr = newVoteRecord(typeTx, true)

	vr.other = 0xFFFFFFFF
	for i := 15; i >= -15; i-- {
		if i >= 0 {
			assert.EqualInt(t, i, int(vr.getInflight()))
		}

		if i > 0 {
			assert.True(t, vr.decInflight())
			assert.EqualInt(t, i-1, int(vr.getInflight()))
		} else {
			assert.False(t, vr.decInflight())
			assert.EqualInt(t, 0, int(vr.getInflight()))
		}
	}

	// Increment back up in an unrolled loop, testing for the exact expected value
	// at each step
	assert.EqualInt(t, 0xFFFFFFF, int(vr.other))
	assert.EqualInt(t, 0, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x1FFFFFFF, int(vr.other))
	assert.EqualInt(t, 1, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x2FFFFFFF, int(vr.other))
	assert.EqualInt(t, 2, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x3FFFFFFF, int(vr.other))
	assert.EqualInt(t, 3, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x4FFFFFFF, int(vr.other))
	assert.EqualInt(t, 4, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x5FFFFFFF, int(vr.other))
	assert.EqualInt(t, 5, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x6FFFFFFF, int(vr.other))
	assert.EqualInt(t, 6, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x7FFFFFFF, int(vr.other))
	assert.EqualInt(t, 7, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x8FFFFFFF, int(vr.other))
	assert.EqualInt(t, 8, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0x9FFFFFFF, int(vr.other))
	assert.EqualInt(t, 9, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0xafffffff, int(vr.other))
	assert.EqualInt(t, 10, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0xbfffffff, int(vr.other))
	assert.EqualInt(t, 11, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0xcfffffff, int(vr.other))
	assert.EqualInt(t, 12, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0xdfffffff, int(vr.other))
	assert.EqualInt(t, 13, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0xefffffff, int(vr.other))
	assert.EqualInt(t, 14, int(vr.getInflight()))

	assert.True(t, vr.incInflight())
	assert.EqualInt(t, 0xffffffff, int(vr.other))
	assert.EqualInt(t, 15, int(vr.getInflight()))

	// Now do it without all non-inflight-counter bits set
	vr.other = packVoteRecordOtherParts(true, true, time.Now().Unix())

	for i := 0; i <= 30; i++ {
		if i <= 15 {
			assert.EqualInt(t, i, int(vr.getInflight()))
		}

		if i < 15 {
			assert.True(t, vr.incInflight())
			assert.EqualInt(t, i+1, int(vr.getInflight()))
		} else {
			assert.False(t, vr.incInflight())
			assert.EqualInt(t, 15, int(vr.getInflight()))
		}
	}

	for i := 15; i >= -15; i-- {
		if i >= 0 {
			assert.EqualInt(t, i, int(vr.getInflight()))
		}

		if i > 0 {
			assert.True(t, vr.decInflight())
			assert.EqualInt(t, i-1, int(vr.getInflight()))
		} else {
			assert.False(t, vr.decInflight())
			assert.EqualInt(t, 0, int(vr.getInflight()))
		}
	}
}

func TestState(t *testing.T) {
	for _, test := range []struct {
		s          state
		expected   string
		expectedPB pb.AvalancheState
	}{
		{stateRejected, "rejected", pb.AvalancheState_REJECTED},
		{stateAccepted, "accepted", pb.AvalancheState_ACCEPTED},
		{stateFinalizedRejected, "rejected", pb.AvalancheState_REJECTED},
		{stateFinalizedAccepted, "accepted", pb.AvalancheState_ACCEPTED},
	} {
		assert.EqualString(t, test.expected, test.s.String())
		assert.True(t, pb.AvalancheState(test.s&1) == test.expectedPB)
	}
}

func TestType(t *testing.T) {
	for _, test := range []struct {
		t          vertexType
		expected   string
		expectedPB pb.AvalancheType
	}{
		{typeBlock, "block", pb.AvalancheType_BLOCK},
		{typeTx, "tx", pb.AvalancheType_TRANSACTION},
	} {
		assert.EqualString(t, test.expected, test.t.String())
		assert.True(t, pb.AvalancheType(test.t) == test.expectedPB)
	}
}
