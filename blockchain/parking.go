// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import "math/big"

const (
	// parkingLockInTicks is how far, in ticks, the tip may be past the
	// fork for a parked chain to need only an eighth more work than the
	// tip has since the fork.  From there on it needs twice that work.
	parkingLockInTicks = 4 * legacyBlockInterval

	// parkingSubCapShift divides the tip's work since the fork by eight
	// for the extra work a parked chain needs short of the lock-in.
	parkingSubCapShift = 3
)

// unparkRequiredWork returns the work a parked chain must exceed to be
// unparked once the tip is a block after the upgrade to 1-minute blocks.
// That is the tip's work plus an eighth of its work since the fork when the
// tip is less than parkingLockInTicks past the fork, and plus all of it from
// there on.
func unparkRequiredWork(tipWork, forkWork *big.Int, tickDepth int64) *big.Int {
	penalty := new(big.Int).Sub(tipWork, forkWork)
	if tickDepth < parkingLockInTicks {
		penalty.Rsh(penalty, parkingSubCapShift)
	}
	return penalty.Add(penalty, tipWork)
}

// legacyUnparkRequiredWork returns the work a parked chain must exceed to be
// unparked before the upgrade to 1-minute blocks applies to the tip, as in
// Bitcoin Cash Node.  A tip 1, 2 or 3 blocks past the fork asks for half a
// block's work more, half of the first block's at depth 2 and of the first
// two blocks' at depth 3, and a deeper tip for twice its work since the fork.
func legacyUnparkRequiredWork(tip, fork *blockNode) *big.Int {
	extra := tip
	switch tip.height - fork.height {
	case 2, 3:
		extra = tip.parent
		fallthrough
	case 1:
		penalty := new(big.Int).Sub(extra.workSum, fork.workSum)
		penalty.Rsh(penalty, 1)
		return penalty.Add(penalty, tip.workSum)
	default:
		penalty := new(big.Int).Sub(tip.workSum, fork.workSum)
		return penalty.Add(penalty, tip.workSum)
	}
}

// parkedChainHasEnoughWorkToUnpark returns whether the chain ending at
// candidate, which forks the main chain ending at tip at fork, has enough
// work to be unparked.  The rule after the upgrade applies once the tip is a
// block after it, whose parent's median time past has reached the activation
// time.  A tip whose own median time past first reaches the activation time
// still takes the legacy rule.
func parkedChainHasEnoughWorkToUnpark(tip, fork, candidate *blockNode) bool {
	var required *big.Int
	if isUpgrade2027Active(tip.parent) {
		required = unparkRequiredWork(tip.workSum, fork.workSum,
			tip.ticks-fork.ticks)
	} else {
		required = legacyUnparkRequiredWork(tip, fork)
	}
	return candidate.workSum.Cmp(required) > 0
}
