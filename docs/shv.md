# Simplified Header Verification

[CHIP-2026-02](https://gitlab.com/0353F40E/mmr), Simplified Header
Verification (SHV), lets a light wallet check that a block header is in the
main chain without storing every header.  The wallet keeps a Merkle Mountain
Range (MMR) accumulator over the block hashes, about 20 hashes for the whole
chain, and asks a node for a short proof when it needs to check a header.  SHV
covers headers only, so BIP37 and Neutrino wallets use it the same way.  It
needs no consensus change.

bchd serves SHV proofs by default.  `--noshv` turns it off.

## The index

At startup bchd builds an MMR over the hashes of the main chain's blocks, from
genesis to the tip, from the block index, and keeps it in step as blocks are
connected and disconnected, so reorganizations are followed.  It keeps only
the MMR's internal nodes, n - popcount(n) hashes of 32 bytes for n blocks,
about 30 MB in memory for 930,000 blocks; the block hashes themselves are read
from the block index.  The block index holds every header from genesis also
with `--prune` and `--fastsync`, so both work with SHV.  The headers fast sync
adds are taken in at the next connected block.  If the index can't be built at
startup, bchd logs an error and runs without SHV.

The root of the MMR over n block hashes is Bitcoin's Merkle root over the same
hashes, which is the root Electrum servers give for a `cp_height` of n - 1.
Heights start at zero, so the commitment at height h has h + 1 blocks.

## P2P

A node with SHV advertises `NODE_SHV`, service bit 9, which bchd's `wire`
package names `SFNodeSHV`.  The bit was the never used `SFNodeXThinner`, which
remains as a deprecated alias.

`getshv` holds up to 100 requests.  Each has a type byte (bit 0: proof to the
root rather than to the block's peak; bit 1: block selected by hash rather
than by height; bits 2 to 7 reserved), a compact size commitment height, and 32
bytes holding the block's hash or its height as a little-endian number.  `shv`
answers with, for each valid request, the type, commitment height and block
height, the 80-byte header, the proof's sibling hashes from the bottom up, and
the target peak or root.  Hashes are in internal byte order.

As the CHIP has it, a request is left out of the answer when a reserved bit is
set, a height does not fit in eight bytes, the main chain has no such block,
the block is above the commitment, or the commitment is above the index's tip.
A `getshv` message whose requests are all left out gets an empty `shv`
message.  One of more than 100 requests gets no answer and, as in Bitcoin Cash
Node, adds 20 to the peer's ban score.  A node without SHV ignores `getshv`.

## RPC

`getshv "root"|"peak" commitment_height block_height` returns the header and
the proof as `branch`, `header` and `root` or `peak`; see the
[JSON-RPC reference](json_rpc_api.md#getshv) for its errors.  Verbose `getblock`
and `getblockheader` give `shvroot`, the root of the commitment at the block's
height, when SHV is on.

## Verifying proofs in Go

The `github.com/gcash/bchd/shv` package holds the MMR the node keeps and the
`Accumulator` a wallet keeps.  An accumulator extends with new block hashes,
gives its root, verifies proofs to a peak or to the root, and can be
bootstrapped from the proof to the root of the commitment's last block, which
passes every peak; its root must then be checked against a root the wallet
trusts.  Proofs to a root, and bootstrapping, refuse a left sibling equal to
the hash it is merged with, as only a proof that passes a duplicated subtree
off as blocks past the end has one (CVE-2012-2459).
