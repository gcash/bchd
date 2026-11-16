// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package shv

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// testVectors holds the CHIP-2026-02 test vectors, the file that BCHN's
// src/test/data/mmr_test_vectors.json and the mmr-accumulator libraries
// share.  Hashes are in display order, as RPCs and Electrum show them.
type testVectors struct {
	Blocks map[string]struct {
		MerkleRoot string   `json:"merkleroot"`
		Tx         []string `json:"tx"`
	} `json:"blocks"`
	HeaderSegments map[string]struct {
		StartHeight  int      `json:"start_height"`
		HeaderHashes []string `json:"header_hashes"`
	} `json:"header_segments"`
	ElectrumProofs map[string]struct {
		Height   uint64   `json:"height"`
		CPHeight uint64   `json:"cp_height"`
		Branch   []string `json:"branch"`
		Header   string   `json:"header"`
		Root     string   `json:"root"`
	} `json:"electrum_proofs"`
	CVE20122459 struct {
		RealLeafCount   uint64 `json:"real_leaf_count"`
		ForgedLeafCount uint64 `json:"forged_leaf_count"`
		Root            string `json:"root"`
		Mappings        []struct {
			ForgedIndex uint64 `json:"forged_index"`
			RealIndex   uint64 `json:"real_index"`
			ProofKey    string `json:"proof_key"`
		} `json:"forged_proof_mappings"`
	} `json:"cve_2012_2459"`
}

// loadTestVectors reads the CHIP-2026-02 test vectors.
func loadTestVectors(t *testing.T) *testVectors {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata",
		"mmr_test_vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vectors testVectors
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	return &vectors
}

// mustHash returns the hash shown by the display-order hex string.
func mustHash(t *testing.T, s string) chainhash.Hash {
	t.Helper()
	hash, err := chainhash.NewHashFromStr(s)
	if err != nil {
		t.Fatalf("%q: %v", s, err)
	}
	return *hash
}

// mustHashes returns the hashes shown by the display-order hex strings.
func mustHashes(t *testing.T, strs []string) []chainhash.Hash {
	t.Helper()
	hashes := make([]chainhash.Hash, len(strs))
	for i, s := range strs {
		hashes[i] = mustHash(t, s)
	}
	return hashes
}

// sliceLeaves returns a LeafFunc that reads the passed leaves.
func sliceLeaves(leaves []chainhash.Hash) LeafFunc {
	return func(index uint64) (chainhash.Hash, error) {
		if index >= uint64(len(leaves)) {
			return chainhash.Hash{}, ErrLeafIndex
		}
		return leaves[index], nil
	}
}

// buildMMR returns the MMR of the passed leaves.
func buildMMR(leaves []chainhash.Hash) *MMR {
	var m MMR
	m.Grow(uint64(len(leaves)))
	for i := range leaves {
		var prev chainhash.Hash
		if i > 0 {
			prev = leaves[i-1]
		}
		m.Append(&leaves[i], &prev)
	}
	return &m
}

// buildAccumulator returns the accumulator of the passed leaves.
func buildAccumulator(t *testing.T, leaves []chainhash.Hash) *Accumulator {
	t.Helper()
	var a Accumulator
	for i := range leaves {
		if err := a.Extend(&leaves[i]); err != nil {
			t.Fatal(err)
		}
	}
	return &a
}

// TestVectorMerkleRoots ensures the roots of an MMR and of an accumulator over
// the transaction hashes of the vectors' blocks, of 8, 10 and 11
// transactions, are the blocks' Merkle roots: one, two and three peaks.
func TestVectorMerkleRoots(t *testing.T) {
	vectors := loadTestVectors(t)
	if len(vectors.Blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(vectors.Blocks))
	}
	for height, block := range vectors.Blocks {
		leaves := mustHashes(t, block.Tx)
		want := mustHash(t, block.MerkleRoot)

		acc := buildAccumulator(t, leaves)
		if got := acc.Root(); got != want {
			t.Errorf("block %s: accumulator root %v, want %v", height,
				got, want)
		}
		if got, want := len(acc.Peaks()),
			popcount(uint64(len(leaves))); got != want {

			t.Errorf("block %s: %d peaks, want %d", height, got, want)
		}

		m := buildMMR(leaves)
		got, err := m.Root(uint64(len(leaves)), sliceLeaves(leaves))
		if err != nil {
			t.Fatalf("block %s: %v", height, err)
		}
		if got != want {
			t.Errorf("block %s: MMR root %v, want %v", height, got, want)
		}
	}
}

// popcount returns the number of bits set in n.
func popcount(n uint64) int {
	count := 0
	for ; n != 0; n &= n - 1 {
		count++
	}
	return count
}

// TestVectorElectrumProofs ensures an MMR over the first 16 block hashes gives
// the branches and roots of the vectors' Electrum proofs, that each header
// hashes to its block hash, and that the accumulator at the commitment
// verifies each proof.
func TestVectorElectrumProofs(t *testing.T) {
	vectors := loadTestVectors(t)
	hashes := mustHashes(t,
		vectors.HeaderSegments["first_16"].HeaderHashes)
	if len(hashes) != 16 {
		t.Fatalf("got %d header hashes, want 16", len(hashes))
	}
	m := buildMMR(hashes)
	leafAt := sliceLeaves(hashes)

	if len(vectors.ElectrumProofs) != 9 {
		t.Fatalf("got %d Electrum proofs, want 9",
			len(vectors.ElectrumProofs))
	}
	for name, vector := range vectors.ElectrumProofs {
		rawHeader, err := hex.DecodeString(vector.Header)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var header wire.BlockHeader
		if err := header.Deserialize(bytes.NewReader(rawHeader)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		leaf := header.BlockHash()
		if leaf != hashes[vector.Height] {
			t.Errorf("%s: header hashes to %v, want %v", name, leaf,
				hashes[vector.Height])
		}

		leafCount := vector.CPHeight + 1
		wantBranch := mustHashes(t, vector.Branch)
		wantRoot := mustHash(t, vector.Root)
		branch, root, err := m.ProofToRoot(vector.Height, leafCount, leafAt)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Equal(branch, wantBranch) || root != wantRoot {
			t.Errorf("%s: got branch %v root %v, want %v %v", name,
				branch, root, wantBranch, wantRoot)
		}

		acc := buildAccumulator(t, hashes[:leafCount])
		if acc.Root() != wantRoot {
			t.Errorf("%s: accumulator root %v, want %v", name,
				acc.Root(), wantRoot)
		}
		if !acc.VerifyProofToRoot(vector.Height, &leaf, wantBranch) {
			t.Errorf("%s: proof to the root not verified", name)
		}
		if !VerifyProofToRoot(&wantRoot, leafCount, vector.Height, &leaf,
			wantBranch) {

			t.Errorf("%s: proof not verified against the root", name)
		}

		// The proof to the leaf's peak is the start of the proof to
		// the root.
		toPeak, peak, err := m.ProofToPeak(vector.Height, leafCount, leafAt)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Equal(toPeak, wantBranch[:len(toPeak)]) {
			t.Errorf("%s: proof to the peak %v is not the start of %v",
				name, toPeak, wantBranch)
		}
		if !slices.Contains(acc.Peaks(), peak) {
			t.Errorf("%s: peak %v is not one of %v", name, peak,
				acc.Peaks())
		}
		if !acc.VerifyProofToPeak(vector.Height, &leaf, toPeak) {
			t.Errorf("%s: proof to the peak not verified", name)
		}
	}
}

// TestVectorDuplicateSubtree ensures the CVE-2012-2459 vectors' forged MMR of
// 16 leaves, whose last five repeat earlier leaves, has the root of the real
// MMR of 11 leaves, so that its proofs would pass without the duplicate
// subtree check, and that verifying proofs to the root and bootstrapping
// refuse them.
func TestVectorDuplicateSubtree(t *testing.T) {
	vectors := loadTestVectors(t)
	cve := vectors.CVE20122459
	hashes := mustHashes(t,
		vectors.HeaderSegments["first_16"].HeaderHashes)
	forged := mustHashes(t,
		vectors.HeaderSegments["first_16_forged"].HeaderHashes)
	root := mustHash(t, cve.Root)
	if cve.RealLeafCount != 11 || cve.ForgedLeafCount != 16 {
		t.Fatalf("leaf counts %d and %d, want 11 and 16",
			cve.RealLeafCount, cve.ForgedLeafCount)
	}

	real11 := buildAccumulator(t, hashes[:11])
	forged16 := buildAccumulator(t, forged)
	forged15 := buildAccumulator(t, forged[:15])
	if real11.Root() != root || forged16.Root() != root ||
		forged15.Root() != root {

		t.Fatalf("roots %v, %v and %v, want all %v", real11.Root(),
			forged16.Root(), forged15.Root(), root)
	}

	// The roots match, but the peaks don't: proofs to a peak are not
	// open to the attack.
	if slices.Equal(real11.Peaks(), forged15.Peaks()) {
		t.Error("the forged MMR of 15 leaves has the real one's peaks")
	}

	for _, mapping := range cve.Mappings {
		if forged[mapping.ForgedIndex] != forged[mapping.RealIndex] {
			t.Fatalf("forged leaf %d is not leaf %d",
				mapping.ForgedIndex, mapping.RealIndex)
		}
		proofVector := vectors.ElectrumProofs[mapping.ProofKey]
		proof := mustHashes(t, proofVector.Branch)
		leaf := forged[mapping.ForgedIndex]

		// Without the check, the real proof climbs from the phantom
		// leaf to the root.
		if climb(mapping.ForgedIndex, leaf, proof) != root {
			t.Errorf("forged leaf %d: proof %s does not reach the root",
				mapping.ForgedIndex, mapping.ProofKey)
		}
		if VerifyProofToRoot(&root, cve.ForgedLeafCount,
			mapping.ForgedIndex, &leaf, proof) {

			t.Errorf("forged leaf %d: proof %s verified",
				mapping.ForgedIndex, mapping.ProofKey)
		}
		if forged16.VerifyProofToRoot(mapping.ForgedIndex, &leaf, proof) {
			t.Errorf("forged leaf %d: proof %s verified by the forged "+
				"accumulator", mapping.ForgedIndex, mapping.ProofKey)
		}

		// The same proof is good for the real leaf.
		realCount := proofVector.CPHeight + 1
		if !VerifyProofToRoot(&root, realCount, mapping.RealIndex,
			&hashes[mapping.RealIndex], proof) {

			t.Errorf("real leaf %d: proof %s not verified",
				mapping.RealIndex, mapping.ProofKey)
		}
	}

	// Bootstrapping from the proof of the real last leaf gives the real
	// accumulator, but from the same proof as the proof of the last leaf of
	// the forged MMR of 16 leaves fails.
	proof10 := mustHashes(t, vectors.ElectrumProofs["proof_10_10"].Branch)
	acc, err := Bootstrap(11, &hashes[10], proof10)
	if err != nil || !acc.Equal(real11) {
		t.Errorf("bootstrap of 11 leaves: %v, %v", acc, err)
	}
	if _, err := Bootstrap(16, &forged[15], proof10); err != ErrDuplicateSubtree {
		t.Errorf("forged bootstrap of 16 leaves: got %v, want %v", err,
			ErrDuplicateSubtree)
	}
	proof15 := mustHashes(t, vectors.ElectrumProofs["proof_15_15"].Branch)
	acc, err = Bootstrap(16, &hashes[15], proof15)
	if err != nil || !acc.Equal(buildAccumulator(t, hashes)) {
		t.Errorf("bootstrap of 16 leaves: %v, %v", acc, err)
	}
}

// climb returns the hash a proof climbs to from the leaf at index, without
// the duplicate subtree check.
func climb(index uint64, leaf chainhash.Hash, proof []chainhash.Hash) chainhash.Hash {
	for i := range proof {
		if index&1 == 1 {
			leaf = nodeHash(&proof[i], &leaf)
		} else {
			leaf = nodeHash(&leaf, &proof[i])
		}
		index >>= 1
	}
	return leaf
}
