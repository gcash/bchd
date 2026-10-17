// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import "github.com/gcash/bchd/wire"

// Exported for the reference tests of package txscript_test.
var (
	ParseShortForm   = parseShortForm
	ParseScriptFlags = parseScriptFlags
)

// SignatureHashes returns the signature hash of input idx with the legacy
// algorithm, and with the replay protected algorithm when hashType has
// SigHashForkID and the legacy one otherwise.
func SignatureHashes(script []byte, hashType SigHashType, tx *wire.MsgTx,
	idx int) (legacy, forkID []byte, err error) {

	pops, err := parseScript(script)
	if err != nil {
		return nil, nil, err
	}
	legacy, _, err = calcLegacySignatureHash(pops, hashType, tx, idx)
	if err != nil {
		return nil, nil, err
	}
	if hashType&SigHashForkID == 0 {
		return legacy, legacy, nil
	}
	forkID, _, err = calcBip143SignatureHash(pops, NewTxSigHashes(tx),
		hashType, tx, idx, 0, false)
	return legacy, forkID, err
}
