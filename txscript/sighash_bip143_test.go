// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/gcash/bchd/wire"
)

// TestBip143SigHashMatchesReference compares calcBip143SignatureHash with the
// reference on the BIP143 vectors, for every hash type, with and without
// token data and hashUtxos, and with script codes past the stack buffer.
func TestBip143SigHashMatchesReference(t *testing.T) {
	t.Parallel()

	file, err := os.ReadFile("data/sighash_bip143.json")
	if err != nil {
		t.Fatal(err)
	}
	var tests [][]interface{}
	if err := json.Unmarshal(file, &tests); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(1))
	bigPush := mustParseShortForm(fmt.Sprintf("0x4d 0xe803 0x%x", make([]byte, 1000)))
	for i, test := range tests[1:] {
		var tx wire.MsgTx
		rawTx, _ := hex.DecodeString(test[0].(string))
		if err := tx.Deserialize(bytes.NewReader(rawTx)); err != nil {
			t.Fatalf("vector %d: %v", i, err)
		}
		subScript, _ := hex.DecodeString(test[1].(string))
		if rng.Intn(4) == 0 {
			subScript = append(subScript, bigPush...)
		}
		pops, err := parseScript(subScript)
		if err != nil {
			t.Fatalf("vector %d: %v", i, err)
		}
		idx := int(test[2].(float64))

		sigHashes := NewTxSigHashes(&tx)
		if i%2 == 0 {
			rng.Read(sigHashes.HashUTXOS[:])
			sigHashes.tokenDataList = make([][]byte, len(tx.TxIn))
			for j := range sigHashes.tokenDataList {
				if rng.Intn(2) == 0 {
					sigHashes.tokenDataList[j] = make([]byte, 1+rng.Intn(80))
					rng.Read(sigHashes.tokenDataList[j])
				}
			}
		}
		amt := rng.Int63()
		for _, tokens := range []bool{false, true} {
			for ht := range 256 {
				hashType := SigHashType(ht)
				got, gotN, err := calcBip143SignatureHash(pops, sigHashes,
					hashType, &tx, idx, amt, tokens)
				want, wantN, wantErr := refCalcBip143SignatureHash(pops,
					sigHashes, hashType, &tx, idx, amt, tokens)
				if err != nil || wantErr != nil {
					t.Fatalf("vector %d: errors %v and %v", i, err, wantErr)
				}
				if !bytes.Equal(got, want) || gotN != wantN {
					t.Fatalf("vector %d hash type %#x tokens %v: got %x (%d bytes), want %x (%d bytes)",
						i, ht, tokens, got, gotN, want, wantN)
				}
			}
		}
	}

	// Out of range input index.
	if _, _, err := calcBip143SignatureHash(nil, &TxSigHashes{}, SigHashAll,
		wire.NewMsgTx(1), 0, 0, false); err == nil {
		t.Fatal("no error for an input index out of range")
	}

	// An opcode that does not serialize gives an empty script code.
	bad := []parsedOpcode{{opcode: &opcodeArray[OP_TRUE]},
		{opcode: &opcodeArray[OP_TRUE], data: []byte{1}}}
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, nil))
	got, gotN, err := calcBip143SignatureHash(bad, NewTxSigHashes(tx),
		SigHashAll, tx, 0, 0, false)
	want, wantN, _ := refCalcBip143SignatureHash(bad, NewTxSigHashes(tx),
		SigHashAll, tx, 0, 0, false)
	if err != nil || !bytes.Equal(got, want) || gotN != wantN {
		t.Fatalf("opcode that does not serialize: got %x (%d bytes) %v, "+
			"want %x (%d bytes)", got, gotN, err, want, wantN)
	}
}
