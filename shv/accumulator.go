// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package shv

import (
	"math"
	"math/bits"
	"slices"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// Accumulator is the state an SHV client keeps of a Merkle Mountain Range: its
// leaf count and the peak of each mountain, ordered from the tallest
// mountain's to the shortest's.  That is one hash for each bit set in the leaf
// count, at most 64, and enough to extend the MMR, compute its root and verify
// proofs against either.
//
// The zero value is an accumulator of no leaves.  An Accumulator is not safe
// for concurrent use while it is modified.
type Accumulator struct {
	leafCount uint64
	peaks     []chainhash.Hash
}

// NewAccumulator returns an accumulator of leafCount leaves with the passed
// peaks, ordered from the tallest mountain's to the shortest's, as Peaks
// returns them.  It returns ErrPeakCount unless there is one peak for each bit
// set in leafCount.
func NewAccumulator(leafCount uint64, peaks []chainhash.Hash) (*Accumulator, error) {
	if len(peaks) != bits.OnesCount64(leafCount) {
		return nil, ErrPeakCount
	}
	return &Accumulator{leafCount: leafCount, peaks: slices.Clone(peaks)}, nil
}

// LeafCount returns the number of leaves the accumulator commits to.
func (a *Accumulator) LeafCount() uint64 {
	return a.leafCount
}

// Peaks returns a copy of the accumulator's peaks, ordered from the tallest
// mountain's to the shortest's.
func (a *Accumulator) Peaks() []chainhash.Hash {
	return slices.Clone(a.peaks)
}

// Equal reports whether the accumulators have the same leaf count and peaks.
func (a *Accumulator) Equal(other *Accumulator) bool {
	return a.leafCount == other.leafCount && slices.Equal(a.peaks, other.peaks)
}

// Extend appends a leaf to the accumulator.  The leaf merges with as many of
// the shortest peaks as the leaf count has trailing one bits.  It returns
// ErrLeafCount if the accumulator already has the most leaves a 64-bit count
// holds.
func (a *Accumulator) Extend(leaf *chainhash.Hash) error {
	if a.leafCount == math.MaxUint64 {
		return ErrLeafCount
	}

	current := *leaf
	merges := bits.TrailingZeros64(^a.leafCount)
	for range merges {
		last := len(a.peaks) - 1
		current = nodeHash(&a.peaks[last], &current)
		a.peaks = a.peaks[:last]
	}
	a.peaks = append(a.peaks, current)
	a.leafCount++
	return nil
}

// Root returns the root of the accumulator's MMR, which is Bitcoin's Merkle
// root over its leaves.  The root of no leaves is the zero hash.
func (a *Accumulator) Root() chainhash.Hash {
	return bagPeaks(a.leafCount, a.peaks)
}

// Rewind sets the accumulator to an earlier state, past, if extending past
// with the passed leaves, the leaves appended since, gives the accumulator's
// state, and reports whether it did.  A client that keeps only peaks can't
// remove leaves, so it rewinds from an earlier state it kept or was given.
func (a *Accumulator) Rewind(past *Accumulator, leaves []chainhash.Hash) bool {
	if past.leafCount >= a.leafCount ||
		uint64(len(leaves)) != a.leafCount-past.leafCount ||
		len(past.peaks) != bits.OnesCount64(past.leafCount) {

		return false
	}

	extended := Accumulator{leafCount: past.leafCount,
		peaks: slices.Clone(past.peaks)}
	for i := range leaves {
		if err := extended.Extend(&leaves[i]); err != nil {
			return false
		}
	}
	if !extended.Equal(a) {
		return false
	}
	a.peaks = slices.Clone(past.peaks)
	a.leafCount = past.leafCount
	return true
}

// VerifyProofToPeak reports whether proof shows that leaf is the leaf at index
// of the accumulator's MMR: whether climbing the proof from the leaf reaches
// the peak of the leaf's mountain.  The proof must have one sibling for each
// level of the mountain, and none when the leaf is a peak by itself.
//
// No duplicate subtree check is needed, as the trusted peaks fix the shape of
// the MMR.
func (a *Accumulator) VerifyProofToPeak(index uint64, leaf *chainhash.Hash, proof []chainhash.Hash) bool {
	if index >= a.leafCount {
		return false
	}

	// The highest bit where the index and the leaf count differ is set in
	// the leaf count, and it is the mountain that holds the leaf.  The
	// taller mountains are the bits set above it.
	diffWidth := bits.Len64(index ^ a.leafCount)
	height := diffWidth - 1
	peak := bits.OnesCount64(a.leafCount >> diffWidth)
	if len(proof) != height {
		return false
	}

	current := *leaf
	for i := range proof {
		if index&1 == 1 {
			current = nodeHash(&proof[i], &current)
		} else {
			current = nodeHash(&current, &proof[i])
		}
		index >>= 1
	}
	return current == a.peaks[peak]
}

// VerifyProofToRoot reports whether proof shows that leaf is the leaf at index
// of the accumulator's MMR: whether climbing the proof from the leaf reaches
// the accumulator's root.  See the package function VerifyProofToRoot.
func (a *Accumulator) VerifyProofToRoot(index uint64, leaf *chainhash.Hash, proof []chainhash.Hash) bool {
	root := a.Root()
	return VerifyProofToRoot(&root, a.leafCount, index, leaf, proof)
}

// VerifyProofToRoot reports whether proof shows that leaf is the leaf at index
// of an MMR of leafCount leaves with the passed root, as a client that trusts
// only a root checks it.  The proof must have bit_width(leafCount - 1)
// siblings, so none for a single leaf, which is then the root.
//
// A sibling on the left that equals the hash it is merged with fails the
// proof, as only a proof that passes a duplicated subtree off as leaves past
// the end has one (CVE-2012-2459).
func VerifyProofToRoot(root *chainhash.Hash, leafCount, index uint64, leaf *chainhash.Hash, proof []chainhash.Hash) bool {
	if index >= leafCount {
		return false
	}
	if leafCount == 1 {
		return len(proof) == 0 && *leaf == *root
	}
	if len(proof) != bits.Len64(leafCount-1) {
		return false
	}

	current := *leaf
	for i := range proof {
		if index&1 == 1 {
			if proof[i] == current {
				return false
			}
			current = nodeHash(&proof[i], &current)
		} else {
			current = nodeHash(&current, &proof[i])
		}
		index >>= 1
	}
	return current == *root
}

// Bootstrap returns the accumulator of leafCount leaves that a proof to the
// root for its last leaf, lastLeaf, passes through.  Such a proof holds every
// peak, between the duplicates that bagging the peaks makes, so a client that
// knows only a root can recover the peaks it needs to extend and verify proofs
// to a peak.
//
// Bootstrap only checks the proof's shape.  The caller must check that the
// accumulator's root is a root it trusts before trusting the accumulator.  As
// the leaf count it is given can't be trusted either, a sibling on the left
// that equals the hash it is merged with fails the proof anywhere on the path,
// which stops a server from claiming more leaves than there are by passing a
// duplicated subtree off as leaves (CVE-2012-2459).
//
// It returns ErrProofLength if the proof does not have bit_width(leafCount -
// 1) siblings, and ErrDuplicateSubtree if it fails that check.
func Bootstrap(leafCount uint64, lastLeaf *chainhash.Hash, proof []chainhash.Hash) (*Accumulator, error) {
	switch {
	case leafCount == 0:
		if len(proof) != 0 {
			return nil, ErrProofLength
		}
		return &Accumulator{}, nil

	case leafCount == 1:
		if len(proof) != 0 {
			return nil, ErrProofLength
		}
		return &Accumulator{leafCount: 1, peaks: []chainhash.Hash{*lastLeaf}}, nil
	}

	if len(proof) != bits.Len64(leafCount-1) {
		return nil, ErrProofLength
	}

	// merge climbs one level of the proof from the computed hash.
	computed := *lastLeaf
	index := leafCount - 1
	next := 0
	merge := func() error {
		sibling := &proof[next]
		if index&1 == 1 {
			if *sibling == computed {
				return ErrDuplicateSubtree
			}
			computed = nodeHash(sibling, &computed)
		} else {
			computed = nodeHash(&computed, sibling)
		}
		index >>= 1
		next++
		return nil
	}

	// Walk the peaks from the shortest up.  The path first climbs the last
	// leaf's own mountain and, past each peak, the duplicates that bring the
	// running hash up to the next peak's height; the sibling then is that
	// peak.  When the proof runs out, the running hash is the tallest peak,
	// which is the root when there is one mountain.
	peakCount := bits.OnesCount64(leafCount)
	peaks := make([]chainhash.Hash, peakCount)
	height := 0
	for remaining, p := leafCount, peakCount-1; remaining > 0; p-- {
		peakHeight := bits.TrailingZeros64(remaining)
		for ; height < peakHeight; height++ {
			if err := merge(); err != nil {
				return nil, err
			}
		}

		switch {
		case peakHeight == 0 && p == peakCount-1:
			// The shortest peak is the last leaf itself.
			peaks[p] = *lastLeaf

		case next < len(proof):
			peaks[p] = proof[next]
			if err := merge(); err != nil {
				return nil, err
			}
			height++

		default:
			peaks[p] = computed
		}
		remaining &= remaining - 1
	}

	return &Accumulator{leafCount: leafCount, peaks: peaks}, nil
}
