// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bcmr

import (
	"bytes"
	"unicode/utf8"

	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// publicationPrefix is the locking bytecode prefix of a publication output:
// OP_RETURN and a push of the four bytes "BCMR".  Only these exact bytes mark
// a publication; the same script written with a longer push of "BCMR" does
// not.
var publicationPrefix = []byte{txscript.OP_RETURN, txscript.OP_DATA_4, 'B', 'C', 'M', 'R'}

// Publication is a transaction's publication output, the output that
// announces a metadata registry by its hash and the URIs it can be downloaded
// from.
type Publication struct {
	// Output is the index of the publication output.
	Output uint32

	// Malformed reports that the output starts with the publication prefix
	// but doesn't push a 32-byte hash followed only by URIs in UTF-8.  Hash
	// and URIs are then empty.
	Malformed bool

	// Hash is the SHA-256 of the registry, exactly as the output pushes it.
	// That is the byte order SHA-256 tools print, not the reversed order in
	// which transaction IDs are usually shown.
	Hash [32]byte

	// URIs are the URIs the output pushes after the hash, in order and as
	// they are.  CHIP-BCMR has a URI without a scheme mean HTTPS, and one
	// without a path the well-known path, but applying those rules is up to
	// the client that downloads the registry.
	URIs []string
}

// WellFormed reports whether pub is a publication, and a well-formed one.
func (pub *Publication) WellFormed() bool {
	return pub != nil && !pub.Malformed
}

// ParsePublication returns the publication of tx, or nil if it has none.  As
// CHIP-BCMR requires, a transaction has at most one: the first output, from
// output 0 up, whose locking bytecode starts with the publication prefix.
// Output 0 counts, so the transaction that burns an identity, which spends its
// identity output to a data-carrier output, can carry a final publication.  A
// first matching output that is malformed is still the publication, and later
// outputs stay ignored.
func ParsePublication(tx *wire.MsgTx) *Publication {
	for i, txOut := range tx.TxOut {
		if !bytes.HasPrefix(txOut.PkScript, publicationPrefix) {
			continue
		}

		pub := &Publication{Output: uint32(i)}
		hash, uris, ok := parsePushes(txOut.PkScript[len(publicationPrefix):])
		if !ok {
			pub.Malformed = true
			return pub
		}
		pub.Hash = hash
		pub.URIs = uris
		return pub
	}
	return nil
}

// parsePushes parses what follows the publication prefix: a data push of
// exactly 32 bytes, the hash, and then nothing but data pushes of valid UTF-8,
// the URIs.  Any data push opcode will do, since the CHIP asks only that they
// be pushed: OP_0, which pushes no bytes, so an empty URI, and those up to
// OP_PUSHDATA4.  OP_1NEGATE and OP_1 to OP_16 push numbers, not data.
func parsePushes(script []byte) (hash [32]byte, uris []string, ok bool) {
	tokenizer := txscript.MakeScriptTokenizer(script)
	haveHash := false
	for tokenizer.Next() {
		if tokenizer.Opcode() > txscript.OP_PUSHDATA4 {
			return hash, nil, false
		}
		data := tokenizer.Data()
		if !haveHash {
			if len(data) != len(hash) {
				return hash, nil, false
			}
			copy(hash[:], data)
			haveHash = true
			continue
		}
		if !utf8.Valid(data) {
			return hash, nil, false
		}
		uris = append(uris, string(data))
	}
	if tokenizer.Err() != nil || !haveHash {
		return hash, nil, false
	}
	return hash, uris, true
}
