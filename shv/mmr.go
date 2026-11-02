// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package shv

import (
	"errors"
	"math/bits"
	"slices"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

var (
	// ErrLeafCount is returned when a leaf count is larger than the MMR's,
	// or when an accumulator or proof can't have the leaf count it claims.
	ErrLeafCount = errors.New("shv: leaf count out of range")

	// ErrLeafIndex is returned when a leaf index is not below the leaf
	// count.
	ErrLeafIndex = errors.New("shv: leaf index out of range")

	// ErrPeakCount is returned when the number of peaks does not match the
	// number of bits set in the leaf count.
	ErrPeakCount = errors.New("shv: peak count does not match leaf count")

	// ErrProofLength is returned when a proof does not have the number of
	// siblings its leaf count needs.
	ErrProofLength = errors.New("shv: proof has the wrong number of siblings")

	// ErrDuplicateSubtree is returned when a proof has a sibling on the left
	// that equals the hash it is merged with, which only a proof for a leaf
	// made up from a duplicated subtree has (CVE-2012-2459).
	ErrDuplicateSubtree = errors.New("shv: proof passes a duplicated " +
		"subtree off as leaves")
)

// LeafFunc returns the leaf at the passed index.  An MMR stores only its
// internal nodes, so its caller supplies the few leaves an operation needs,
// such as block hashes from the block index.  They must be the leaves the MMR
// was built from.
type LeafFunc func(index uint64) (chainhash.Hash, error)

// nodeHash returns the parent of the passed left and right nodes, the double
// SHA-256 of the two concatenated, as in Bitcoin's Merkle trees.
func nodeHash(left, right *chainhash.Hash) chainhash.Hash {
	var buf [chainhash.HashSize * 2]byte
	copy(buf[:chainhash.HashSize], left[:])
	copy(buf[chainhash.HashSize:], right[:])
	return chainhash.DoubleHashH(buf[:])
}

// internalNodes returns the number of internal nodes of an MMR of n leaves,
// n - popcount(n), as each merge of two subtrees makes one.
func internalNodes(n uint64) uint64 {
	return n - uint64(bits.OnesCount64(n))
}

// mountain describes the mountain of an MMR that holds a leaf.
type mountain struct {
	// start is the index of the mountain's first leaf.
	start uint64

	// height is the height of the mountain, which has 2^height leaves.
	height int

	// peak is the index of the mountain's peak among the peaks, which are
	// ordered from the tallest mountain's to the shortest's.
	peak int
}

// mountainOf returns the mountain that holds the leaf at index in an MMR of
// leafCount leaves.  The index must be below the leaf count.
func mountainOf(index, leafCount uint64) mountain {
	var m mountain
	for remaining := leafCount; ; m.peak++ {
		m.height = bits.Len64(remaining) - 1
		size := uint64(1) << m.height
		if index-m.start < size {
			return m
		}
		m.start += size
		remaining -= size
	}
}

// bagPeaks returns the root of an MMR of leafCount leaves with the passed
// peaks, ordered from the tallest mountain's to the shortest's.  It bags the
// peaks from the shortest up, duplicating the running hash until its height
// matches the next peak's, which gives Bitcoin's Merkle root over the leaves.
// The root of no leaves is the zero hash.
func bagPeaks(leafCount uint64, peaks []chainhash.Hash) chainhash.Hash {
	if leafCount == 0 {
		return chainhash.Hash{}
	}

	hash := peaks[len(peaks)-1]
	remaining := leafCount
	height := bits.TrailingZeros64(remaining)
	for i := len(peaks) - 1; i > 0; i-- {
		// Clearing the lowest set bit leaves the next peak's height as
		// the lowest.
		remaining &= remaining - 1
		next := bits.TrailingZeros64(remaining)
		for ; height < next; height++ {
			hash = nodeHash(&hash, &hash)
		}
		hash = nodeHash(&peaks[i-1], &hash)
		height++
	}
	return hash
}

// MMR is a Merkle Mountain Range over a sequence of leaves, such as the hashes
// of the main chain's blocks, that can prove any leaf to a peak or to the root
// of the MMR at any size up to its own.
//
// It stores only the internal nodes, n - popcount(n) hashes for n leaves, in
// the order appending the leaves makes them, so the nodes of the MMR of the
// first k leaves are the first k - popcount(k) of them.  The leaves themselves
// are not stored; the methods that need one take a LeafFunc for it.
//
// The zero value is an empty MMR.  An MMR is not safe for concurrent use
// while it is modified.
type MMR struct {
	leafCount uint64
	nodes     []chainhash.Hash
}

// LeafCount returns the number of leaves in the MMR.
func (m *MMR) LeafCount() uint64 {
	return m.leafCount
}

// NodeCount returns the number of internal nodes the MMR stores, which is
// its leaf count less the number of bits set in it.
func (m *MMR) NodeCount() uint64 {
	return uint64(len(m.nodes))
}

// Grow makes room for the internal nodes of leafCount leaves, so that
// appending up to that many does not allocate.
func (m *MMR) Grow(leafCount uint64) {
	if n := internalNodes(leafCount); n > uint64(len(m.nodes)) {
		m.nodes = slices.Grow(m.nodes, int(n)-len(m.nodes))
	}
}

// Append adds a leaf to the end of the MMR.  As the MMR does not store leaves,
// prev must be the leaf appended before, which for a block hash is the previous
// block hash of the block's header.  It is only read when the leaf count is
// odd, as the new leaf is then merged with it; the first leaf ignores it.
func (m *MMR) Append(leaf, prev *chainhash.Hash) {
	// Appending merges as many times as the leaf count has trailing one
	// bits, first with the previous leaf and then with the peak of each
	// next height, much as adding one carries in binary.
	merges := bits.TrailingZeros64(^m.leafCount)
	if merges > 0 {
		current := nodeHash(prev, leaf)
		m.nodes = append(m.nodes, current)
		for i := 1; i < merges; i++ {
			// The peak of height i, made when the leaf ending its
			// mountain was appended, is the last node made then.
			peak := internalNodes(m.leafCount+2-uint64(1)<<i) - 1
			current = nodeHash(&m.nodes[peak], &current)
			m.nodes = append(m.nodes, current)
		}
	}
	m.leafCount++
}

// Truncate removes leaves from the end of the MMR so that leafCount remain,
// as when a chain reorganization disconnects blocks.  The MMR is then exactly
// the MMR of the leaves that remain.  It returns ErrLeafCount if leafCount is
// larger than the MMR's.
func (m *MMR) Truncate(leafCount uint64) error {
	if leafCount > m.leafCount {
		return ErrLeafCount
	}
	m.leafCount = leafCount
	m.nodes = m.nodes[:internalNodes(leafCount)]
	return nil
}

// Peaks returns the peaks of the MMR of its first leafCount leaves, ordered
// from the tallest mountain's to the shortest's.  When leafCount is odd, the
// last peak is the last leaf, which leafAt is asked for.  It returns
// ErrLeafCount if leafCount is larger than the MMR's.
func (m *MMR) Peaks(leafCount uint64, leafAt LeafFunc) ([]chainhash.Hash, error) {
	if leafCount > m.leafCount {
		return nil, ErrLeafCount
	}

	peaks := make([]chainhash.Hash, 0, bits.OnesCount64(leafCount))
	var end uint64
	for remaining := leafCount; remaining > 1; {
		// The peak of a mountain of two or more leaves is the last node
		// made when its last leaf was appended.
		size := uint64(1) << (bits.Len64(remaining) - 1)
		end += size
		peaks = append(peaks, m.nodes[internalNodes(end)-1])
		remaining -= size
	}
	if leafCount&1 == 1 {
		last, err := leafAt(leafCount - 1)
		if err != nil {
			return nil, err
		}
		peaks = append(peaks, last)
	}
	return peaks, nil
}

// Root returns the root of the MMR of its first leafCount leaves, which is
// Bitcoin's Merkle root over them.  When leafCount is odd, leafAt is asked for
// the last leaf.  It returns ErrLeafCount if leafCount is larger than the
// MMR's.
func (m *MMR) Root(leafCount uint64, leafAt LeafFunc) (chainhash.Hash, error) {
	peaks, err := m.Peaks(leafCount, leafAt)
	if err != nil {
		return chainhash.Hash{}, err
	}
	return bagPeaks(leafCount, peaks), nil
}

// ProofToPeak returns the proof that the leaf at index is in the MMR of its
// first leafCount leaves, and the peak it leads to: the siblings from the
// leaf up to the peak of its mountain, one for each level of the mountain.  A
// leaf that is a peak by itself has an empty proof and is its own peak.
// leafAt is asked for the leaf next to the one proven, or for that leaf when
// it is its own peak.  It returns ErrLeafIndex if index is not below
// leafCount, and ErrLeafCount if leafCount is larger than the MMR's.
func (m *MMR) ProofToPeak(index, leafCount uint64, leafAt LeafFunc) ([]chainhash.Hash, chainhash.Hash, error) {
	if leafCount > m.leafCount {
		return nil, chainhash.Hash{}, ErrLeafCount
	}
	if index >= leafCount {
		return nil, chainhash.Hash{}, ErrLeafIndex
	}

	mtn := mountainOf(index, leafCount)
	proof, peak, err := m.proofToPeak(index, mtn, leafAt)
	if err != nil {
		return nil, chainhash.Hash{}, err
	}
	return proof, peak, nil
}

// proofToPeak returns the proof that the leaf at index is in the mountain mtn
// and the mountain's peak.  The leaf must be in the mountain.
func (m *MMR) proofToPeak(index uint64, mtn mountain, leafAt LeafFunc) ([]chainhash.Hash, chainhash.Hash, error) {
	// A mountain of one leaf is its own peak.
	if mtn.height == 0 {
		leaf, err := leafAt(index)
		if err != nil {
			return nil, chainhash.Hash{}, err
		}
		return []chainhash.Hash{}, leaf, nil
	}

	proof := make([]chainhash.Hash, 0, mtn.height)
	sibling, err := leafAt(index ^ 1)
	if err != nil {
		return nil, chainhash.Hash{}, err
	}
	proof = append(proof, sibling)

	// Each sibling above the leaves is the node covering the subtree next
	// to the path, made when that subtree's last leaf was appended.
	// Appending that leaf made one node for each trailing one bit of its
	// index, from height one up, so the node of this height is found from
	// the number of nodes there were once it was appended.
	local := (index - mtn.start) >> 1
	for level := 1; level < mtn.height; level++ {
		end := mtn.start + ((local^1)+1)<<level - 1
		merges := uint64(bits.TrailingZeros64(^end))
		node := internalNodes(end+1) - merges + uint64(level) - 1
		proof = append(proof, m.nodes[node])
		local >>= 1
	}

	// The peak is the last node made when the mountain's last leaf was
	// appended.
	peak := m.nodes[internalNodes(mtn.start+uint64(1)<<mtn.height)-1]
	return proof, peak, nil
}

// ProofToRoot returns the proof that the leaf at index is in the MMR of its
// first leafCount leaves, and that MMR's root: the siblings from the leaf up to
// the root, bit_width(leafCount - 1) of them, which is the branch Electrum
// servers give for a cp_height of leafCount - 1.  It passes the proof to the
// leaf's peak, then the hashes that bagging the peaks merges with it.  leafAt
// is asked for the leaf next to the one proven, for the last leaf when
// leafCount is odd, and for the leaf proven when it is a peak by itself.  It
// returns ErrLeafIndex if index is not below leafCount, and ErrLeafCount if
// leafCount is larger than the MMR's.
func (m *MMR) ProofToRoot(index, leafCount uint64, leafAt LeafFunc) ([]chainhash.Hash, chainhash.Hash, error) {
	if leafCount > m.leafCount {
		return nil, chainhash.Hash{}, ErrLeafCount
	}
	if index >= leafCount {
		return nil, chainhash.Hash{}, ErrLeafIndex
	}

	mtn := mountainOf(index, leafCount)
	proof, _, err := m.proofToPeak(index, mtn, leafAt)
	if err != nil {
		return nil, chainhash.Hash{}, err
	}
	peaks, err := m.Peaks(leafCount, leafAt)
	if err != nil {
		return nil, chainhash.Hash{}, err
	}
	if len(peaks) == 1 {
		return proof, peaks[0], nil
	}

	// Bag the peaks as Root does.  Below the leaf's peak, the running hash
	// is only computed; at its peak, the running hash, the shorter peaks
	// bagged, is the sibling; above it, each duplicate of the running hash
	// and each taller peak is.
	hash := peaks[len(peaks)-1]
	remaining := leafCount
	height := bits.TrailingZeros64(remaining)
	onPath := mtn.peak == len(peaks)-1
	for i := len(peaks) - 1; i > 0; i-- {
		remaining &= remaining - 1
		next := bits.TrailingZeros64(remaining)
		for ; height < next; height++ {
			if onPath {
				proof = append(proof, hash)
			}
			hash = nodeHash(&hash, &hash)
		}
		switch {
		case i-1 == mtn.peak:
			proof = append(proof, hash)
			onPath = true
		case onPath:
			proof = append(proof, peaks[i-1])
		}
		hash = nodeHash(&peaks[i-1], &hash)
		height++
	}
	return proof, hash, nil
}
