// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/*
Package bcmr implements the on-chain rules of Bitcoin Cash Metadata Registries,
as CHIP-BCMR defines them: the publication outputs that announce a metadata
registry, the token geneses, which make their categories' IDs authbases, and
the burns that end an authchain.

An identity is anchored by a transaction, its authbase.  The identity's
authchain is the authbase, then each transaction that spends output 0, the
identity output, of the one before, up to the authhead, whose identity output
is unspent.  Of the publications along an authchain, CHIP-BCMR counts the
authhead's.  bchd's BCMR index goes further, and when the authhead has no
well-formed publication, it answers with the newest earlier one, marked as not
current.  A token category's ID is the hash of the transaction whose
identity output its genesis spends, so every category has an authchain from
that authbase on.  It is usually the identity a registry gives the token,
though CHIP-BCMR lets an identity move to a new category.

The package holds only these rules, so that bchd's BCMR index and wallets that
follow authchains themselves read every transaction the same way.
*/
package bcmr
