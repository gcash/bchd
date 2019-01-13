package avalanche

import (
	"testing"
)

var (
	_negativeOne = -1
	neutral  = uint8(_negativeOne)
	yes = uint8(0x01)
	no = uint8(0x00)
)

func TestVoteRecord(t *testing.T) {
	var vr *VoteRecord
	registerVoteAndCheck := func(vote uint8, state, finalized bool, confidence uint16) {
		vr.regsiterVote(vote)
		assertTrue(t, vr.isAccepted() == state)
		assertTrue(t, vr.hasFinalized() == finalized)
		assertTrue(t, vr.getConfidence() == confidence)
	}

	vr = NewVoteRecord(nil, true)
	assertTrue(t, vr.isAccepted())
	assertFalse(t, vr.hasFinalized())
	assertTrue(t, vr.getConfidence() == 0)

	vr = NewVoteRecord(nil, false)
	assertFalse(t, vr.isAccepted())
	assertFalse(t, vr.hasFinalized())
	assertTrue(t, vr.getConfidence() == 0)

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
	for i := 0; i<400; i++ {
		registerVoteAndCheck(neutral, true, false, 7)
	}

	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(yes, true, false, 7)
	}

	// Now confidence will increase as long as we vote yes.
	for i := uint16(8); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(yes, true, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(no, true, true, AvalancheFinalizationScore)

	// Now that we have two no votes, confidence stop increasing.
	for i := uint16(0); i < 5; i++ {
		registerVoteAndCheck(no, true, true,
			AvalancheFinalizationScore)
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
	for i := uint16(8); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(no, false, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(yes, false, true, AvalancheFinalizationScore)
}

func assertTrue(t *testing.T, actual bool) {
	if !actual {
		t.Fatal("Expected true; got false")
	}
}

func assertFalse(t *testing.T, actual bool) {
	if actual {
		t.Fatal("Expected false; got true")
	}
}