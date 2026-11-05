BCMR hints
==========

A hints file says where the authchain of every token stood at a height. A wallet
that can only scan block filters starts from there rather than from a token's
genesis, so that it reads weeks of filters rather than years.

A wallet can't verify what comes before the height of its hints, so it trusts
whoever gave it the file. The file is built so that nobody has to take that on
faith: **the same height gives the same bytes on every node.** A published file
can be checked by anyone with a node.

## Generate or check a file

You need a node with `--bcmrindex` that has finished catching up, and `bcmrhints`
from the same format version:

```sh
go install github.com/gcash/bchd/cmd/bcmrhints@latest
BCMRHINTS_RPCUSER=user BCMRHINTS_RPCPASS=pass bcmrhints --height 975821 -o hints.bin
```

It prints the height, the number of tokens, the size and the SHA-256:

```text
height 975821, 21438 tokens, 1624772 bytes
sha256 1b00f7bc609aa5c18a32dc445192879f91d32a8d0527606390b905f5dc5f05d6
```

To check a published file, run it with the published height and compare the
SHA-256. A different hash means a different file, whoever made it.

The height must be at least 100 blocks below the tip, so that a reorganization
can't make two nodes disagree. The tip doesn't otherwise matter: only what the
blocks up to that height say goes in the file.

Reading a token whose authchain moved past the height takes that authchain's
transactions from the node, so a height far below the tip takes longer.

## What's in it

For each token: its category, the authhead at that height and the head's height,
whether the identity is burned, and the newest publication if that isn't on the
head. It holds no names or other metadata: the wallet reads those from the
publication and its registry, and follows the authchain past the height itself.

A bad file can only show wrong metadata for a token. It can't touch funds.

The format is in `bcmr/hints.go`. Its magic includes a version, and a change to
the format must change the version, so that files stay comparable.
