// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"math/big"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

var (
	// bigOne is 1 represented as a big.Int.  It is defined here to avoid
	// the overhead of creating it multiple times.
	bigOne = big.NewInt(1)

	// oneLsh256 is 1 shifted left 256 bits.  It is defined here to avoid
	// the overhead of creating it multiple times.
	oneLsh256 = new(big.Int).Lsh(bigOne, 256)
)

const (
	// difficultyAdjustmentWindow is the size of the window used by the DAA adjustment
	// algorithm when calculating the current difficulty. The algorithm requires fetching
	// a 'suitable' block out of blocks n-144, n-145, and n-146. We set this value equal
	// to n-144 as that is the first of the three candidate blocks and we will use it
	// to fetch the previous two.
	difficultyAdjustmentWindow = 144

	// idealBlockTime is used by the Asert difficulty adjustment algorithm. It equals
	// 10 minutes between blocks.
	idealBlockTime = 600

	// radix is used by the Asert difficulty adjustment algorithm.
	radix = 65536

	// rbits is the number of bits after the radix for fixed-point math
	rbits = 16
)

// The DifficultyAlgorithm specifies which algorithm to use and is passed into
// the calcNextRequiredDifficulty function.
//
// Bitcoin Cash has had three different difficulty adjustment algorithms during
// its life. What this means for us is our node needs to select which algorithm
// to use when calculating difficulty based on where it is in the chain.
type DifficultyAlgorithm uint32

const (
	// DifficultyLegacy was in effect from genesis through August 1st, 2017.
	DifficultyLegacy DifficultyAlgorithm = 0

	// DifficultyEDA (Emergency Difficulty Adjustment) was a short lived changed
	// right after the August 1st, 2017 hardfork and lasted until November 15th, 2017.
	DifficultyEDA DifficultyAlgorithm = 1

	// DifficultyDAA (Difficulty Adjustment Algorithm) is the Bitcoin Cash difficulty
	// algorithm in effect between November 15th, 2017 and November 15, 2020.
	DifficultyDAA DifficultyAlgorithm = 2

	// DifficultyAsert is the aserti3-2d algorithm in effect as of November 15, 2020.
	DifficultyAsert DifficultyAlgorithm = 3
)

// SelectDifficultyAdjustmentAlgorithm returns the difficulty adjustment algorithm that
// should be used when validating a block at the given height.
func (b *BlockChain) SelectDifficultyAdjustmentAlgorithm(prevNode *blockNode) DifficultyAlgorithm {
	height := prevNode.height + 1
	if height > b.chainParams.UahfForkHeight && height <= b.chainParams.DaaForkHeight {
		return DifficultyEDA
	} else if height > b.chainParams.DaaForkHeight && height <= b.chainParams.AxionActivationHeight {
		return DifficultyDAA
	} else if height > b.chainParams.AxionActivationHeight {
		return DifficultyAsert
	}
	return DifficultyLegacy
}

// HashToBig converts a chainhash.Hash into a big.Int that can be used to
// perform math comparisons.
func HashToBig(hash *chainhash.Hash) *big.Int {
	// A Hash is in little-endian, but the big package wants the bytes in
	// big-endian, so reverse them.
	buf := *hash
	blen := len(buf)
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}

	return new(big.Int).SetBytes(buf[:])
}

// CompactToBig converts a compact representation of a whole number N to an
// unsigned 32-bit number.  The representation is similar to IEEE754 floating
// point numbers.
//
// Like IEEE754 floating point, there are three basic components: the sign,
// the exponent, and the mantissa.  They are broken out as follows:
//
//   - the most significant 8 bits represent the unsigned base 256 exponent
//
//   - bit 23 (the 24th bit) represents the sign bit
//
//   - the least significant 23 bits represent the mantissa
//
//     -------------------------------------------------
//     |   Exponent     |    Sign    |    Mantissa     |
//     -------------------------------------------------
//     | 8 bits [31-24] | 1 bit [23] | 23 bits [22-00] |
//     -------------------------------------------------
//
// The formula to calculate N is:
//
//	N = (-1^sign) * mantissa * 256^(exponent-3)
//
// This compact form is only used in bitcoin to encode unsigned 256-bit numbers
// which represent difficulty targets, thus there really is not a need for a
// sign bit, but it is implemented here to stay consistent with bitcoind.
func CompactToBig(compact uint32) *big.Int {
	// Extract the mantissa, sign bit, and exponent.
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes to represent the full 256-bit number.  So,
	// treat the exponent as the number of bytes and shift the mantissa
	// right or left accordingly.  This is equivalent to:
	// N = mantissa * 256^(exponent-3)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}

	// Make it negative if the sign bit is set.
	if isNegative {
		bn = bn.Neg(bn)
	}

	return bn
}

// BigToCompact converts a whole number N to a compact representation using
// an unsigned 32-bit number.  The compact representation only provides 23 bits
// of precision, so values larger than (2^23 - 1) only encode the most
// significant digits of the number.  See CompactToBig for details.
func BigToCompact(n *big.Int) uint32 {
	// No need to do any work if it's zero.
	if n.Sign() == 0 {
		return 0
	}

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes.  So, shift the number right or left
	// accordingly.  This is equivalent to:
	// mantissa = mantissa / 256^(exponent-3)
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		// Use a copy to avoid modifying the caller's original number.
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}

	// When the mantissa already has the sign bit set, the number is too
	// large to fit into the available 23-bits, so divide the number by 256
	// and increment the exponent accordingly.
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}

	// Pack the exponent, sign bit, and mantissa into an unsigned 32-bit
	// int and return it.
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact
}

// CalcWork calculates a work value from difficulty bits.  Bitcoin increases
// the difficulty for generating a block by decreasing the value which the
// generated hash must be less than.  This difficulty target is stored in each
// block header using a compact representation as described in the documentation
// for CompactToBig.  The main chain is selected by choosing the chain that has
// the most proof of work (highest difficulty).  Since a lower target difficulty
// value equates to higher actual difficulty, the work value which will be
// accumulated must be the inverse of the difficulty.  Also, in order to avoid
// potential division by zero and really small floating point numbers, the
// result adds 1 to the denominator and multiplies the numerator by 2^256.
func CalcWork(bits uint32) *big.Int {
	// Return a work value of zero if the passed difficulty bits represent
	// a negative number. Note this should not happen in practice with valid
	// blocks, but an invalid block could trigger it.
	difficultyNum := CompactToBig(bits)
	if difficultyNum.Sign() <= 0 {
		return big.NewInt(0)
	}

	// (1 << 256) / (difficultyNum + 1)
	denominator := new(big.Int).Add(difficultyNum, bigOne)
	return new(big.Int).Div(oneLsh256, denominator)
}

// findPrevTestNetDifficulty returns the difficulty of the previous block which
// did not have the special testnet minimum difficulty rule applied.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) findPrevTestNetDifficulty(startNode *blockNode) uint32 {
	// Search backwards through the chain for the last block without
	// the special rule applied.
	iterNode := startNode
	for iterNode != nil && iterNode.height%b.blocksPerRetarget != 0 &&
		iterNode.bits == b.chainParams.PowLimitBits {

		iterNode = iterNode.parent
	}

	// Return the found difficulty or the minimum difficulty if no
	// appropriate block was found.
	lastBits := b.chainParams.PowLimitBits
	if iterNode != nil {
		lastBits = iterNode.bits
	}
	return lastBits
}

// getSuitableBlock locates the two parents of passed in block, sorts the three
// blocks by timestamp and returns the median.
func (b *BlockChain) getSuitableBlock(node0 *blockNode) (*blockNode, error) {
	node1 := node0.RelativeAncestor(1)
	if node1 == nil {
		return nil, AssertError("unable to obtain relative ancestor")
	}
	node2 := node1.RelativeAncestor(1)
	if node2 == nil {
		return nil, AssertError("unable to obtain relative ancestor")
	}
	blocks := []*blockNode{node2, node1, node0}
	if blocks[0].timestamp > blocks[2].timestamp {
		blocks[0], blocks[2] = blocks[2], blocks[0]
	}
	if blocks[0].timestamp > blocks[1].timestamp {
		blocks[0], blocks[1] = blocks[1], blocks[0]
	}
	if blocks[1].timestamp > blocks[2].timestamp {
		blocks[1], blocks[2] = blocks[2], blocks[1]
	}
	return blocks[1], nil
}

// calcNextRequiredDifficulty calculates the required difficulty for the block
// after the passed previous block node based on the difficulty retarget rules.
// This function differs from the exported CalcNextRequiredDifficulty in that
// the exported version uses the current best chain as the previous block node
// while this function accepts any block node.
func (b *BlockChain) calcNextRequiredDifficulty(lastNode *blockNode, newBlockTime time.Time, algorithm DifficultyAlgorithm) (uint32, error) {
	// Genesis block.
	if lastNode == nil {
		return b.chainParams.PowLimitBits, nil
	}

	// If regest or simnet we don't adjust the difficulty
	if b.chainParams.NoDifficultyAdjustment {
		return lastNode.bits, nil
	}

	switch algorithm {
	case DifficultyLegacy, DifficultyEDA:
		return b.calcLegacyRequiredDifficulty(lastNode, newBlockTime, algorithm)
	case DifficultyDAA:
		return b.calcDAARequiredDifficulty(lastNode, newBlockTime)
	case DifficultyAsert:
		return b.calcAsertRequiredDifficulty(lastNode, b.chainParams.AsertDifficultyAnchorHeight, b.chainParams.AsertDifficultyAnchorParentTimestamp, b.chainParams.AsertDifficultyAnchorBits, newBlockTime)
	}
	return 0, errors.New("unknown difficulty algorithm")
}

func (b *BlockChain) calcAsertRequiredDifficulty(lastNode *blockNode, anchorBlockHeight int32, anchorBlockTime int64, anchorBlockBits uint32, evalTimestamp time.Time) (uint32, error) {
	// For networks that support it, allow special reduction of the
	// required difficulty once too much time has elapsed without
	// mining a block.
	if b.chainParams.ReduceMinDifficulty {
		// Return minimum difficulty when more than the desired
		// amount of time has elapsed without mining a block.
		reductionTime := int64(b.chainParams.MinDiffReductionTime /
			time.Second)
		allowMinTime := lastNode.timestamp + reductionTime
		if evalTimestamp.Unix() > allowMinTime {
			return b.chainParams.PowLimitBits, nil
		}
	}

	target := CompactToBig(anchorBlockBits)

	tDelta := lastNode.timestamp - anchorBlockTime
	hDelta := lastNode.height - anchorBlockHeight
	bigRadix := big.NewInt(radix)

	// exponent = int(((time_diff - IDEAL_BLOCK_TIME * (height_diff + 1)) * RADIX) / HALFLIFE)
	exponent := new(big.Int).Sub(big.NewInt(tDelta), new(big.Int).Mul(big.NewInt(int64(idealBlockTime)), new(big.Int).Add(big.NewInt(int64(hDelta)), big.NewInt(1))))
	exponent.Mul(exponent, bigRadix)
	exponent.Quo(exponent, big.NewInt(b.chainParams.AsertDifficultyHalflife))

	// shifts = exponent >> RBITS
	shifts := new(big.Int).Rsh(exponent, rbits)

	// exponent -= shifts * RADIX
	exponent.Sub(exponent, new(big.Int).Mul(shifts, bigRadix))

	//  target *= RADIX + ((195766423245049 * exponent + 971821376 * exponent**2 + 5127 * exponent**3 + 2**47) >> (RBITS * 3))
	factor := new(big.Int).Mul(big.NewInt(195766423245049), exponent)
	factor.Add(factor, new(big.Int).Mul(big.NewInt(971821376), new(big.Int).Exp(exponent, big.NewInt(2), nil)))
	factor.Add(factor, new(big.Int).Mul(big.NewInt(5127), new(big.Int).Exp(exponent, big.NewInt(3), nil)))
	factor.Add(factor, new(big.Int).Exp(big.NewInt(2), big.NewInt(47), nil))
	factor.Rsh(factor, rbits*3)

	target.Mul(target, new(big.Int).Add(bigRadix, factor))

	// if shifts < 0: target >>= -shifts else: target <<= shifts
	if shifts.Cmp(big.NewInt(0)) < 0 {
		target = target.Rsh(target, uint(-shifts.Int64()))
	} else {
		target = target.Lsh(target, uint(shifts.Int64()))
	}

	// target >>= RBITS
	target.Rsh(target, rbits)

	// If target is zero
	if target.Cmp(big.NewInt(0)) == 0 {
		return BigToCompact(big.NewInt(1)), nil
	}

	if target.Cmp(b.chainParams.PowLimit) > 0 {
		// Return softest target
		return b.chainParams.PowLimitBits, nil
	}

	return BigToCompact(target), nil
}

func (b *BlockChain) calcDAARequiredDifficulty(lastNode *blockNode, newBlockTime time.Time) (uint32, error) {
	// For networks that support it, allow special reduction of the
	// required difficulty once too much time has elapsed without
	// mining a block.
	if b.chainParams.ReduceMinDifficulty {
		// Return minimum difficulty when more than the desired
		// amount of time has elapsed without mining a block.
		reductionTime := int64(b.chainParams.MinDiffReductionTime /
			time.Second)
		allowMinTime := lastNode.timestamp + reductionTime
		if newBlockTime.Unix() > allowMinTime {
			return b.chainParams.PowLimitBits, nil
		}
	}

	// Get the block node at the beginning of the window (n-144)
	firstNode := lastNode.RelativeAncestor(difficultyAdjustmentWindow)
	if firstNode == nil {
		return 0, AssertError("unable to obtain previous retarget block")
	}

	// Find the suitable blocks to use as the first and last nodes for the
	// purpose of the difficulty calculation. A suitable block is the median
	// timestamp out of the three prior.
	suitableLastNode, err := b.getSuitableBlock(lastNode)
	if err != nil {
		return 0, err
	}
	suitableFirstNode, err := b.getSuitableBlock(firstNode)
	if err != nil {
		return 0, err
	}

	work := new(big.Int).Sub(suitableLastNode.workSum, suitableFirstNode.workSum)

	// In order to avoid difficulty cliffs, we bound the amplitude of the
	// adjustement we are going to do.
	duration := suitableLastNode.timestamp - suitableFirstNode.timestamp
	if duration > 288*int64(b.chainParams.TargetTimePerBlock.Seconds()) {
		duration = 288 * int64(b.chainParams.TargetTimePerBlock.Seconds())
	} else if duration < 72*int64(b.chainParams.TargetTimePerBlock.Seconds()) {
		duration = 72 * int64(b.chainParams.TargetTimePerBlock.Seconds())
	}

	projectedWork := new(big.Int).Mul(work, big.NewInt(int64(b.chainParams.TargetTimePerBlock.Seconds())))

	pw := new(big.Int).Div(projectedWork, big.NewInt(duration))

	e := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)

	nt := new(big.Int).Sub(e, pw)

	newTarget := new(big.Int).Div(nt, pw)

	// clip again if above minimum target (too easy)
	if newTarget.Cmp(b.chainParams.PowLimit) > 0 {
		newTarget.Set(b.chainParams.PowLimit)
	}
	return BigToCompact(newTarget), nil
}

func (b *BlockChain) calcLegacyRequiredDifficulty(lastNode *blockNode, newBlockTime time.Time, algorithm DifficultyAlgorithm) (uint32, error) {
	// Return the previous block's difficulty requirements if this block
	// is not at a difficulty retarget interval.
	if (lastNode.height+1)%b.blocksPerRetarget != 0 {
		// For networks that support it, allow special reduction of the
		// required difficulty once too much time has elapsed without
		// mining a block.
		if b.chainParams.ReduceMinDifficulty {
			// Return minimum difficulty when more than the desired
			// amount of time has elapsed without mining a block.
			reductionTime := int64(b.chainParams.MinDiffReductionTime /
				time.Second)
			allowMinTime := lastNode.timestamp + reductionTime
			if newBlockTime.Unix() > allowMinTime {
				return b.chainParams.PowLimitBits, nil
			}

			// The block was mined within the desired timeframe, so
			// return the difficulty for the last block which did
			// not have the special minimum difficulty rule applied.
			return b.findPrevTestNetDifficulty(lastNode), nil
		}

		// If we're using the EDA check if we need to perform an emergency
		// difficulty adjustment
		if algorithm == DifficultyEDA {
			// We can't go bellow the minimum, so early bail.
			oldTarget := CompactToBig(lastNode.bits)
			if oldTarget.Cmp(b.chainParams.PowLimit) == 0 {
				return BigToCompact(b.chainParams.PowLimit), nil
			}
			// If producing the last 6 block took less than 12h, we keep the same
			// difficulty.
			firstNode := lastNode.RelativeAncestor(6)
			if firstNode == nil {
				return 0, AssertError("unable to obtain previous retarget block")
			}
			mtp6Blocks := lastNode.CalcPastMedianTime().Sub(firstNode.CalcPastMedianTime())
			if mtp6Blocks >= 12*time.Hour {
				// If producing the last 6 block took more than 12h, increase the difficulty
				// target by 1/4 (which reduces the difficulty by 20%). This ensure the
				// chain do not get stuck in case we lose hashrate abruptly.
				nPow := CompactToBig(lastNode.bits)
				shft := new(big.Int).Rsh(nPow, 2)
				nPow.Add(nPow, shft)

				// Make sure it doesn't go over limit
				if nPow.Cmp(b.chainParams.PowLimit) > 0 {
					return BigToCompact(b.chainParams.PowLimit), nil
				}

				newTargetBits := BigToCompact(nPow)
				log.Debugf("Emergency difficulty retarget at block height %d", lastNode.height+1)
				log.Debugf("Old target %08x (%064x)", lastNode.bits, oldTarget)
				log.Debugf("New target %08x (%064x)", newTargetBits, CompactToBig(newTargetBits))
				log.Debugf("Actual mtp time passed %s", mtp6Blocks)
				return newTargetBits, nil
			}
		}

		// For the main network (or any unrecognized networks), simply
		// return the previous block's difficulty requirements.
		return lastNode.bits, nil
	}

	// Get the block node at the previous retarget (targetTimespan days
	// worth of blocks).
	firstNode := lastNode.RelativeAncestor(b.blocksPerRetarget - 1)
	if firstNode == nil {
		return 0, AssertError("unable to obtain previous retarget block")
	}

	// Limit the amount of adjustment that can occur to the previous
	// difficulty.
	actualTimespan := lastNode.timestamp - firstNode.timestamp
	adjustedTimespan := actualTimespan
	if actualTimespan < b.minRetargetTimespan {
		adjustedTimespan = b.minRetargetTimespan
	} else if actualTimespan > b.maxRetargetTimespan {
		adjustedTimespan = b.maxRetargetTimespan
	}

	// Calculate new target difficulty as:
	//  currentDifficulty * (adjustedTimespan / targetTimespan)
	// The result uses integer division which means it will be slightly
	// rounded down.  Bitcoind also uses integer division to calculate this
	// result.
	oldTarget := CompactToBig(lastNode.bits)
	newTarget := new(big.Int).Mul(oldTarget, big.NewInt(adjustedTimespan))
	targetTimeSpan := int64(b.chainParams.TargetTimespan / time.Second)
	newTarget.Div(newTarget, big.NewInt(targetTimeSpan))

	// Limit new value to the proof of work limit.
	if newTarget.Cmp(b.chainParams.PowLimit) > 0 {
		newTarget.Set(b.chainParams.PowLimit)
	}

	// Log new target difficulty and return it.  The new target logging is
	// intentionally converting the bits back to a number instead of using
	// newTarget since conversion to the compact representation loses
	// precision.
	newTargetBits := BigToCompact(newTarget)
	log.Debugf("Difficulty retarget at block height %d", lastNode.height+1)
	log.Debugf("Old target %08x (%064x)", lastNode.bits, oldTarget)
	log.Debugf("New target %08x (%064x)", newTargetBits, CompactToBig(newTargetBits))
	log.Debugf("Actual timespan %v, adjusted timespan %v, target timespan %v",
		time.Duration(actualTimespan)*time.Second,
		time.Duration(adjustedTimespan)*time.Second,
		b.chainParams.TargetTimespan)

	return newTargetBits, nil
}

// CalcNextRequiredDifficulty calculates the required difficulty for the block
// after the end of the current best chain based on the difficulty retarget
// rules.
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcNextRequiredDifficulty(timestamp time.Time) (uint32, error) {
	b.chainLock.Lock()
	tip := b.bestChain.Tip()
	difficulty, err := b.calcNextRequiredDifficulty(tip, timestamp,
		b.SelectDifficultyAdjustmentAlgorithm(tip))
	b.chainLock.Unlock()
	return difficulty, err
}
