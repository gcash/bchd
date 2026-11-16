// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package shv

import (
	"errors"
	"math"
	"math/bits"
	"slices"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// testTxns returns n distinct transactions, whose hashes serve as leaves that
// blockchain.BuildMerkleTreeStore can build Bitcoin's Merkle tree over.
func testTxns(n int) []*bchutil.Tx {
	txns := make([]*bchutil.Tx, n)
	for i := range txns {
		tx := wire.NewMsgTx(wire.TxVersion)
		tx.LockTime = uint32(i)
		txns[i] = bchutil.NewTx(tx)
	}
	return txns
}

// txLeaves returns the hashes of the passed transactions.
func txLeaves(txns []*bchutil.Tx) []chainhash.Hash {
	leaves := make([]chainhash.Hash, len(txns))
	for i, tx := range txns {
		leaves[i] = *tx.Hash()
	}
	return leaves
}

// merkleBranch returns the branch of the leaf at index of the Merkle tree that
// blockchain.BuildMerkleTreeStore stored, the siblings from the leaf up to
// the root.  A node with no sibling is merged with itself.
func merkleBranch(store []*chainhash.Hash, index int) []chainhash.Hash {
	branch := []chainhash.Hash{}
	offset := 0
	width := (len(store) + 1) / 2
	for ; width > 1; width /= 2 {
		sibling := store[offset+(index^1)]
		if sibling == nil {
			sibling = store[offset+index]
		}
		branch = append(branch, *sibling)
		offset += width
		index >>= 1
	}
	return branch
}

// TestMMRMatchesMerkleTree ensures that, for every size up to 70 leaves, an MMR
// built once over all of them gives the root and branches of the Merkle tree
// blockchain.BuildMerkleTreeStore builds over the same hashes, that an
// accumulator of that size has the same root and the peaks the MMR gives, and
// that the accumulator verifies each proof to the root and to the peak.
func TestMMRMatchesMerkleTree(t *testing.T) {
	const maxLeaves = 70
	txns := testTxns(maxLeaves)
	leaves := txLeaves(txns)
	m := buildMMR(leaves)
	leafAt := sliceLeaves(leaves)
	if m.LeafCount() != maxLeaves {
		t.Fatalf("leaf count %d, want %d", m.LeafCount(), maxLeaves)
	}

	for n := 1; n <= maxLeaves; n++ {
		leafCount := uint64(n)
		store := blockchain.BuildMerkleTreeStore(txns[:n])
		merkleRoot := *store[len(store)-1]

		root, err := m.Root(leafCount, leafAt)
		if err != nil {
			t.Fatalf("%d leaves: %v", n, err)
		}
		if root != merkleRoot {
			t.Fatalf("%d leaves: root %v, want %v", n, root, merkleRoot)
		}

		acc := buildAccumulator(t, leaves[:n])
		peaks, err := m.Peaks(leafCount, leafAt)
		if err != nil {
			t.Fatalf("%d leaves: %v", n, err)
		}
		if !slices.Equal(peaks, acc.Peaks()) || acc.Root() != merkleRoot {
			t.Fatalf("%d leaves: accumulator peaks %v root %v, want %v %v",
				n, acc.Peaks(), acc.Root(), peaks, merkleRoot)
		}

		for i := 0; i < n; i++ {
			index := uint64(i)
			proof, proofRoot, err := m.ProofToRoot(index, leafCount, leafAt)
			if err != nil {
				t.Fatalf("%d leaves, leaf %d: %v", n, i, err)
			}
			want := merkleBranch(store, i)
			if !slices.Equal(proof, want) || proofRoot != merkleRoot {
				t.Fatalf("%d leaves, leaf %d: proof %v root %v, want "+
					"%v %v", n, i, proof, proofRoot, want, merkleRoot)
			}
			if len(proof) != bits.Len64(leafCount-1) {
				t.Fatalf("%d leaves, leaf %d: %d siblings", n, i,
					len(proof))
			}
			if !acc.VerifyProofToRoot(index, &leaves[i], proof) {
				t.Fatalf("%d leaves, leaf %d: proof to the root not "+
					"verified", n, i)
			}

			toPeak, peak, err := m.ProofToPeak(index, leafCount, leafAt)
			if err != nil {
				t.Fatalf("%d leaves, leaf %d: %v", n, i, err)
			}
			mtn := mountainOf(index, leafCount)
			if len(toPeak) != mtn.height || peak != peaks[mtn.peak] ||
				!slices.Equal(toPeak, proof[:len(toPeak)]) {

				t.Fatalf("%d leaves, leaf %d: proof to peak %v, peak %v",
					n, i, toPeak, peak)
			}
			if !acc.VerifyProofToPeak(index, &leaves[i], toPeak) {
				t.Fatalf("%d leaves, leaf %d: proof to the peak not "+
					"verified", n, i)
			}

			// A proof for the wrong leaf or index fails.
			other := leaves[(i+1)%n]
			if n > 1 && (acc.VerifyProofToRoot(index, &other, proof) ||
				acc.VerifyProofToPeak(index, &other, toPeak)) {

				t.Fatalf("%d leaves, leaf %d: proof verified for "+
					"another leaf", n, i)
			}
		}

		// The proof for the last leaf bootstraps the accumulator.
		proof, _, err := m.ProofToRoot(leafCount-1, leafCount, leafAt)
		if err != nil {
			t.Fatalf("%d leaves: %v", n, err)
		}
		booted, err := Bootstrap(leafCount, &leaves[n-1], proof)
		if err != nil || !booted.Equal(acc) {
			t.Fatalf("%d leaves: bootstrap gave %v, %v, want %v", n,
				booted, err, acc)
		}
	}
}

// TestMMRNodes ensures an MMR stores n - popcount(n) internal nodes for n
// leaves, has the CHIP's peak count for each size, and refuses out of range
// sizes and indexes.
func TestMMRNodes(t *testing.T) {
	var empty MMR
	root, err := empty.Root(0, nil)
	if err != nil || root != (chainhash.Hash{}) {
		t.Errorf("empty root %v, %v, want the zero hash", root, err)
	}
	peaks, err := empty.Peaks(0, nil)
	if err != nil || len(peaks) != 0 {
		t.Errorf("empty peaks %v, %v", peaks, err)
	}

	leaves := txLeaves(testTxns(40))
	m := buildMMR(leaves)
	leafAt := sliceLeaves(leaves)
	for n := uint64(0); n <= 40; n++ {
		var partial MMR
		for i := uint64(0); i < n; i++ {
			var prev chainhash.Hash
			if i > 0 {
				prev = leaves[i-1]
			}
			partial.Append(&leaves[i], &prev)
		}
		if want := n - uint64(popcount(n)); partial.NodeCount() != want {
			t.Errorf("%d leaves: %d nodes, want %d", n,
				partial.NodeCount(), want)
		}
		peaks, err := m.Peaks(n, leafAt)
		if err != nil || len(peaks) != popcount(n) {
			t.Errorf("%d leaves: %d peaks, %v", n, len(peaks), err)
		}
	}

	if _, err := m.Peaks(41, leafAt); err != ErrLeafCount {
		t.Errorf("peaks past the end: %v", err)
	}
	if _, _, err := m.ProofToRoot(0, 41, leafAt); err != ErrLeafCount {
		t.Errorf("proof past the end: %v", err)
	}
	if _, _, err := m.ProofToPeak(10, 10, leafAt); err != ErrLeafIndex {
		t.Errorf("proof of a leaf past the commitment: %v", err)
	}
	if _, _, err := m.ProofToRoot(10, 10, leafAt); err != ErrLeafIndex {
		t.Errorf("proof of a leaf past the commitment: %v", err)
	}
	if err := m.Truncate(41); err != ErrLeafCount {
		t.Errorf("truncating past the end: %v", err)
	}

	// A leaf function's error is returned.
	failure := errors.New("no leaf")
	failing := func(uint64) (chainhash.Hash, error) {
		return chainhash.Hash{}, failure
	}
	if _, err := m.Root(9, failing); err != failure {
		t.Errorf("root with a failing leaf function: %v", err)
	}
	if _, _, err := m.ProofToPeak(3, 9, failing); err != failure {
		t.Errorf("proof with a failing leaf function: %v", err)
	}
	if _, _, err := m.ProofToRoot(8, 9, failing); err != failure {
		t.Errorf("proof with a failing leaf function: %v", err)
	}
}

// TestMMRTruncate ensures truncating an MMR, as disconnecting blocks does,
// leaves exactly the MMR built from the leaves that remain, and that
// appending other leaves afterwards, as a reorganization connects a competing
// chain, gives exactly the MMR built from the new sequence.
func TestMMRTruncate(t *testing.T) {
	const total = 64
	leaves := txLeaves(testTxns(2 * total))
	original, competing := leaves[:total], leaves[total:]

	for keep := 0; keep <= total; keep++ {
		m := buildMMR(original)
		if err := m.Truncate(uint64(keep)); err != nil {
			t.Fatal(err)
		}
		want := buildMMR(original[:keep])
		if m.LeafCount() != want.LeafCount() ||
			!slices.Equal(m.nodes, want.nodes) {

			t.Fatalf("truncated to %d leaves: the MMR differs from the "+
				"one built from them", keep)
		}

		// Replace the removed leaves with others.
		sequence := slices.Concat(original[:keep], competing[:total-keep])
		for i := keep; i < total; i++ {
			prev := sequence[max(i, 1)-1]
			m.Append(&sequence[i], &prev)
		}
		want = buildMMR(sequence)
		if !slices.Equal(m.nodes, want.nodes) {
			t.Fatalf("reorganized at %d leaves: the MMR differs from "+
				"the one built from the new leaves", keep)
		}

		// And back again.
		if err := m.Truncate(uint64(keep)); err != nil {
			t.Fatal(err)
		}
		for i := keep; i < total; i++ {
			prev := original[max(i, 1)-1]
			m.Append(&original[i], &prev)
		}
		if !slices.Equal(m.nodes, buildMMR(original).nodes) {
			t.Fatalf("reorganized back at %d leaves: the MMR differs",
				keep)
		}
	}
}

// TestAccumulatorEdges ensures the accumulator handles no leaves, one leaf,
// powers of two and its largest leaf count, refuses a peak count that does
// not match the leaf count, and that proofs with the wrong length or index
// fail.
func TestAccumulatorEdges(t *testing.T) {
	leaves := txLeaves(testTxns(16))

	var empty Accumulator
	if empty.LeafCount() != 0 || len(empty.Peaks()) != 0 ||
		empty.Root() != (chainhash.Hash{}) {

		t.Errorf("empty accumulator: %d leaves, peaks %v, root %v",
			empty.LeafCount(), empty.Peaks(), empty.Root())
	}
	if empty.VerifyProofToPeak(0, &leaves[0], nil) ||
		empty.VerifyProofToRoot(0, &leaves[0], nil) {

		t.Error("the empty accumulator verified a proof")
	}

	one := buildAccumulator(t, leaves[:1])
	if one.Root() != leaves[0] || !one.VerifyProofToPeak(0, &leaves[0], nil) ||
		!one.VerifyProofToRoot(0, &leaves[0], nil) {

		t.Error("a single leaf is not its own root and peak")
	}
	if one.VerifyProofToRoot(0, &leaves[0], leaves[:1]) ||
		one.VerifyProofToRoot(1, &leaves[0], nil) {

		t.Error("a single leaf verified with a sibling or past the end")
	}

	for _, n := range []int{2, 4, 8, 16} {
		acc := buildAccumulator(t, leaves[:n])
		if peaks := acc.Peaks(); len(peaks) != 1 || peaks[0] != acc.Root() {
			t.Errorf("%d leaves: peaks %v, root %v", n, peaks, acc.Root())
		}
	}

	if _, err := NewAccumulator(11, leaves[:2]); err != ErrPeakCount {
		t.Errorf("11 leaves with 2 peaks: %v", err)
	}
	acc11 := buildAccumulator(t, leaves[:11])
	copied, err := NewAccumulator(11, acc11.Peaks())
	if err != nil || !copied.Equal(acc11) {
		t.Errorf("accumulator from 11 leaves' peaks: %v, %v", copied, err)
	}
	if got := len(acc11.Peaks()); got != 3 {
		t.Errorf("11 leaves: %d peaks, want 3", got)
	}

	// Proofs of the wrong length or past the end fail.
	m := buildMMR(leaves[:11])
	proof, _, err := m.ProofToRoot(3, 11, sliceLeaves(leaves))
	if err != nil {
		t.Fatal(err)
	}
	toPeak, _, err := m.ProofToPeak(3, 11, sliceLeaves(leaves))
	if err != nil {
		t.Fatal(err)
	}
	if !acc11.VerifyProofToRoot(3, &leaves[3], proof) ||
		!acc11.VerifyProofToPeak(3, &leaves[3], toPeak) {

		t.Fatal("proofs not verified")
	}
	for _, short := range [][]chainhash.Hash{nil, proof[:len(proof)-1]} {
		if acc11.VerifyProofToRoot(3, &leaves[3], short) {
			t.Errorf("proof to the root of %d siblings verified",
				len(short))
		}
	}
	if acc11.VerifyProofToRoot(3, &leaves[3], append(proof, proof[0])) {
		t.Error("proof to the root with an extra sibling verified")
	}
	if acc11.VerifyProofToPeak(3, &leaves[3], toPeak[1:]) ||
		acc11.VerifyProofToPeak(3, &leaves[3], append(toPeak, toPeak[0])) {

		t.Error("proof to the peak of the wrong length verified")
	}
	if acc11.VerifyProofToRoot(11, &leaves[3], proof) ||
		acc11.VerifyProofToPeak(11, &leaves[3], toPeak) {

		t.Error("proof past the end verified")
	}

	// The lone leaf peak has an empty proof to its peak.
	if !acc11.VerifyProofToPeak(10, &leaves[10], nil) {
		t.Error("the lone leaf peak not verified with an empty proof")
	}

	// The largest leaf count can't be extended.
	full, err := NewAccumulator(math.MaxUint64, make([]chainhash.Hash, 64))
	if err != nil {
		t.Fatal(err)
	}
	if err := full.Extend(&leaves[0]); err != ErrLeafCount {
		t.Errorf("extending the largest leaf count: %v", err)
	}
	if full.LeafCount() != math.MaxUint64 || len(full.Peaks()) != 64 {
		t.Error("a failed extension changed the accumulator")
	}
}

// TestAccumulatorRewind ensures an accumulator is rewound to an earlier state
// only when extending that state with the leaves since gives its own.
func TestAccumulatorRewind(t *testing.T) {
	leaves := txLeaves(testTxns(20))
	current := buildAccumulator(t, leaves)
	for n := 0; n < 20; n++ {
		past := buildAccumulator(t, leaves[:n])
		acc := buildAccumulator(t, leaves)
		if !acc.Rewind(past, leaves[n:]) || !acc.Equal(past) {
			t.Fatalf("not rewound to %d leaves", n)
		}

		// The leaves since must be the right ones, and all of them.
		acc = buildAccumulator(t, leaves)
		wrong := slices.Clone(leaves[n:])
		wrong[0] = leaves[(n+1)%20]
		if acc.Rewind(past, wrong) || acc.Rewind(past, leaves[n+1:]) ||
			!acc.Equal(current) {

			t.Fatalf("rewound to %d leaves with the wrong leaves", n)
		}
	}
	acc := buildAccumulator(t, leaves)
	if acc.Rewind(current, nil) || !acc.Equal(current) {
		t.Error("rewound to its own state")
	}
}

// TestBootstrapEdges ensures bootstrapping handles no leaves, one leaf and
// powers of two, refuses proofs of the wrong length, and never panics on a
// proof of the right length with any hashes.
func TestBootstrapEdges(t *testing.T) {
	leaves := txLeaves(testTxns(64))

	acc, err := Bootstrap(0, &leaves[0], nil)
	if err != nil || acc.LeafCount() != 0 {
		t.Errorf("bootstrap of no leaves: %v, %v", acc, err)
	}
	if _, err := Bootstrap(0, &leaves[0], leaves[:1]); err != ErrProofLength {
		t.Errorf("bootstrap of no leaves with a sibling: %v", err)
	}
	acc, err = Bootstrap(1, &leaves[0], nil)
	if err != nil || !acc.Equal(buildAccumulator(t, leaves[:1])) {
		t.Errorf("bootstrap of one leaf: %v, %v", acc, err)
	}
	if _, err := Bootstrap(1, &leaves[0], leaves[:1]); err != ErrProofLength {
		t.Errorf("bootstrap of one leaf with a sibling: %v", err)
	}

	m := buildMMR(leaves)
	for _, n := range []uint64{2, 3, 8, 11, 32, 63, 64} {
		proof, _, err := m.ProofToRoot(n-1, n, sliceLeaves(leaves))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Bootstrap(n, &leaves[n-1], proof[1:]); err != ErrProofLength {
			t.Errorf("%d leaves, short proof: %v", n, err)
		}
		if _, err := Bootstrap(n, &leaves[n-1], append(proof, proof[0])); err != ErrProofLength {
			t.Errorf("%d leaves, long proof: %v", n, err)
		}
	}

	// Proofs made of arbitrary hashes, including the leaf itself, of the
	// right length for every count up to 300 and for counts around powers
	// of two up to the largest, only ever fail.
	counts := []uint64{math.MaxUint64, math.MaxUint64 - 1}
	for i := uint64(1); i <= 300; i++ {
		counts = append(counts, i)
	}
	for shift := 9; shift < 64; shift++ {
		counts = append(counts, 1<<shift-1, 1<<shift, 1<<shift+1)
	}
	for _, n := range counts {
		length := bits.Len64(n - 1)
		for _, pattern := range []int{0, 1, 2} {
			proof := make([]chainhash.Hash, length)
			for i := range proof {
				switch pattern {
				case 0:
					proof[i] = leaves[i%len(leaves)]
				case 1:
					proof[i] = leaves[0]
				}
			}
			acc, err := Bootstrap(n, &leaves[0], proof)
			if err == nil && (acc.LeafCount() != n ||
				len(acc.Peaks()) != popcount(n)) {

				t.Fatalf("%d leaves: bootstrap gave %d leaves and %d "+
					"peaks", n, acc.LeafCount(), len(acc.Peaks()))
			}
		}
	}
}
