// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/*
Package shv implements the Merkle Mountain Range (MMR) of CHIP-2026-02,
Simplified Header Verification (SHV), over a sequence of block hashes.

A wallet that checks block headers with SHV keeps an Accumulator: the number
of leaves and the peak of each mountain, about 20 hashes for the whole chain,
instead of every header.  A node keeps an MMR, which can prove that a block
hash is the leaf at a given height of the commitment at any height up to its
tip.

# Leaves, nodes and roots

A leaf is a block hash, in the internal byte order that hashing a header gives.
An internal node is the double SHA-256 of its left child followed by its right
child, as in Bitcoin's Merkle trees.  Heights start at zero, so the commitment
at block height h has h + 1 leaves, genesis included.

The mountains are perfect binary trees, one for each bit set in the leaf
count, from the tallest to the shortest, and their roots are the peaks.  The
root of the MMR bags the peaks from the shortest up, duplicating the running
hash until its height matches the next peak's, which gives the same root as
Bitcoin's Merkle tree over the same leaves.  That is the root Electrum servers
serve for a cp_height.  The root of no leaves is the zero hash.

# Proofs

A proof lists the siblings on the path from a leaf up to its target, the leaf's
peak or the root, from the bottom up.  The bits of the leaf's index say which
side each sibling is on: a 1 bit puts the sibling on the left.

As Bitcoin's Merkle trees duplicate unpaired nodes, MMRs of different sizes can
have the same root (CVE-2012-2459).  A subtree duplicated when the peaks were
bagged could then be passed off as real leaves past the end.  Duplicates only
ever stand on the right, so proofs to a root, and bootstrapping from one, refuse
a sibling on the left that equals the hash it is merged with.  A proof to a peak
needs no such check, as the trusted peaks fix the MMR's shape.

# Server and client

MMR stores only internal nodes, n - popcount(n) hashes for n leaves, in the
order they are made, which lets it answer for every earlier size too, and
lose leaves from the end when a chain reorganization disconnects blocks.  The
block hashes themselves stay in the block index, so the caller passes the few
leaves an operation needs.

Accumulator keeps only the peaks.  It extends with new leaves, computes the
root, verifies proofs to a peak or to the root, and can be bootstrapped from a
proof to the root of the last leaf, which passes every peak.  Its root must
then be checked against a trusted root before the accumulator is trusted.
*/
package shv
