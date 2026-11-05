// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/gcash/bchd/wire"
)

// refCalcSignatureHash is CalcSignatureHash with the reference sighash code.
func refCalcSignatureHash(script []byte, sigHashes *TxSigHashes, hType SigHashType,
	tx *wire.MsgTx, idx int, amt int64, useBip143SigHashAlgo bool) ([]byte, int, error) {

	parsedScript, err := parseScript(script)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot parse output script: %v", err)
	}

	if useBip143SigHashAlgo {
		if sigHashes == nil {
			sigHashes = &TxSigHashes{
				HashPrevOuts: refCalcHashPrevOuts(tx),
				HashSequence: refCalcHashSequence(tx),
				HashOutputs:  refCalcHashOutputs(tx),
			}
		}
		if hType&SigHashUTXO != 0 && !sigHashes.hasAllUtxos() {
			return nil, 0, errors.New("a signature hash with " +
				"SigHashUTXO needs the outputs the transaction spends " +
				"in the sighashes")
		}
		return refCalcBip143SignatureHash(parsedScript, sigHashes, hType,
			tx, idx, amt, true)
	}
	return refCalcLegacySignatureHash(parsedScript, hType, tx, idx)
}

// equivLen returns a script or commitment length, often one at a varint or
// stack buffer boundary.
func equivLen(rng *rand.Rand, max int) int {
	lens := []int{0, 1, 0xfc, 0xfd, 0xfe, 0x100, 480, 511, 512, 513, 3000}
	if rng.Intn(4) == 0 {
		if l := lens[rng.Intn(len(lens))]; l <= max {
			return l
		}
	}
	return rng.Intn(min(max, 120) + 1)
}

// equivScript returns a script code like randSighashScript, which panics on
// a script past the size limit, so such scripts are drawn again.
func equivScript(rng *rand.Rand) []byte {
	for {
		script, err := func() (script []byte, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("%v", r)
				}
			}()
			return randSighashScript(rng), nil
		}()
		if err == nil {
			return script
		}
	}
}

// equivTokenData returns empty or random token data, with every bitfield
// form and amounts of every varint size.
func equivTokenData(rng *rand.Rand) wire.TokenData {
	var td wire.TokenData
	if rng.Intn(2) == 0 {
		return td
	}
	rng.Read(td.CategoryID[:])
	switch rng.Intn(3) {
	case 0:
		td.BitField = wire.HAS_AMOUNT
	case 1:
		td.BitField = wire.HAS_NFT | byte(rng.Intn(3))
	default:
		td.BitField = wire.HAS_NFT | wire.HAS_AMOUNT | byte(rng.Intn(3))
	}
	if td.HasNFT() && rng.Intn(2) == 0 {
		td.BitField |= wire.HAS_COMMITMENT_LENGTH
		td.Commitment = make([]byte, 1+rng.Intn(wire.MAX_COMMITMENT_LENGTH))
		rng.Read(td.Commitment)
	}
	if td.HasAmount() {
		amounts := []uint64{1, 0xfc, 0xfd, 0xffff, 0x10000, 0xffffffff,
			0x100000000, wire.MAX_FT_AMOUNT}
		td.Amount = amounts[rng.Intn(len(amounts))]
	}
	return td
}

// equivTxOut returns a random output.
func equivTxOut(rng *rand.Rand) *wire.TxOut {
	values := []int64{-1, 0, 1, 546, 21e14, 1<<63 - 1}
	out := &wire.TxOut{Value: rng.Int63n(1e12)}
	if rng.Intn(4) == 0 {
		out.Value = values[rng.Intn(len(values))]
	}
	out.PkScript = make([]byte, equivLen(rng, 600))
	rng.Read(out.PkScript)
	out.TokenData = equivTokenData(rng)
	return out
}

// equivCount returns an input or output count, rarely one past the 0xfd
// varint boundary.
func equivCount(rng *rand.Rand) int {
	if rng.Intn(40) == 0 {
		return 0xfd + rng.Intn(8)
	}
	counts := []int{0, 1, 1, 2, 3, 4, 5, 8, 20}
	return counts[rng.Intn(len(counts))]
}

// equivTx returns a random transaction and a utxo cache that lacks the spent
// output of some of its inputs.
func equivTx(rng *rand.Rand) (*wire.MsgTx, *UtxoCache) {
	return equivTxCounts(rng, equivCount(rng), equivCount(rng))
}

// equivTxCounts returns a random transaction with the given input and output
// counts, and a utxo cache that lacks the spent output of some of its inputs.
func equivTxCounts(rng *rand.Rand, nIn, nOut int) (*wire.MsgTx, *UtxoCache) {
	tx := &wire.MsgTx{Version: rng.Int31() - rng.Int31(), LockTime: rng.Uint32()}
	utxos := NewUtxoCache()
	for i := range nIn {
		in := &wire.TxIn{Sequence: rng.Uint32()}
		if rng.Intn(2) == 0 {
			in.Sequence = 0xffffffff
		}
		rng.Read(in.PreviousOutPoint.Hash[:])
		in.PreviousOutPoint.Index = rng.Uint32()
		in.SignatureScript = make([]byte, equivLen(rng, 600))
		rng.Read(in.SignatureScript)
		tx.TxIn = append(tx.TxIn, in)
		if rng.Intn(10) != 0 {
			utxos.AddEntry(i, *equivTxOut(rng))
		}
	}
	for range nOut {
		tx.TxOut = append(tx.TxOut, equivTxOut(rng))
	}
	return tx, utxos
}

// equivIndexes returns the input indexes a transaction is checked with: every
// one from -1 to one past the last input, or for many inputs those around
// the 0xfd boundary and the ends.
func equivIndexes(tx *wire.MsgTx) []int {
	n := len(tx.TxIn)
	var idxs []int
	for idx := -1; idx <= n; idx++ {
		if n <= 50 || idx < 2 || idx >= n-2 ||
			(idx >= 0xfa && idx <= 0xfe) {
			idxs = append(idxs, idx)
		}
	}
	return idxs
}

// equivHashTypes are the hash types every transaction is checked with: each
// base type with and without ForkID, SigHashUTXO and AnyOneCanPay, and some
// undefined ones.
var equivHashTypes = func() []SigHashType {
	var types []SigHashType
	for _, base := range []SigHashType{0, SigHashAll, SigHashNone,
		SigHashSingle, 4, 0x1f} {

		for flags := range SigHashType(8) {
			types = append(types, base|(flags<<5))
		}
	}
	return types
}()

// checkSighashMatches checks the sighash midstates of tx and the sighash of
// the equivIndexes input indexes against the reference code, for the given
// hash types.
func checkSighashMatches(t *testing.T, name string, tx *wire.MsgTx,
	utxos *UtxoCache, script []byte, hashTypes []SigHashType, amt int64) {

	t.Helper()
	sigHashes := NewTxSigHashes(tx)
	if sigHashes.HashPrevOuts != refCalcHashPrevOuts(tx) ||
		sigHashes.HashSequence != refCalcHashSequence(tx) ||
		sigHashes.HashOutputs != refCalcHashOutputs(tx) {
		t.Fatalf("%s: tx sighashes differ", name)
	}
	withUtxos := sigHashes.WithUtxos(tx, utxos)
	if withUtxos.HashUTXOS != refCalcHashUtxos(tx, utxos) {
		t.Fatalf("%s: utxos hash differs", name)
	}
	for i := range tx.TxIn {
		var want []byte
		if utxo, err := utxos.GetEntry(i); err == nil && !utxo.TokenData.IsEmpty() {
			b := refTokenDataBuffer(&utxo.TokenData)
			want = b.Bytes()
		}
		if !bytes.Equal(withUtxos.tokenDataList[i], want) {
			t.Fatalf("%s: token data of input %d differs", name, i)
		}
	}
	pops, parseErr := parseScript(script)

	check := func(what string, idx int, hashType SigHashType, got []byte,
		gotN int, gotErr error, want []byte, wantN int, wantErr error) {

		t.Helper()
		if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) ||
			!bytes.Equal(got, want) || gotN != wantN {
			t.Fatalf("%s idx %d type %#x %s: got %x %d %v, want %x %d %v",
				name, idx, uint32(hashType), what, got, gotN, gotErr,
				want, wantN, wantErr)
		}
	}
	for _, idx := range equivIndexes(tx) {
		for _, hashType := range hashTypes {
			for _, bip143 := range []bool{false, true} {
				for _, sh := range []*TxSigHashes{nil, withUtxos} {
					if sh == nil && !bip143 {
						continue
					}
					got, gotN, gotErr := CalcSignatureHash(script, sh,
						hashType, tx, idx, amt, bip143)
					want, wantN, wantErr := refCalcSignatureHash(script, sh,
						hashType, tx, idx, amt, bip143)
					check("public", idx, hashType, got, gotN, gotErr,
						want, wantN, wantErr)
				}
			}
			if parseErr != nil {
				continue
			}
			got, gotN, gotErr := calcLegacySignatureHash(pops, hashType, tx, idx)
			want, wantN, wantErr := refCalcLegacySignatureHash(pops, hashType, tx, idx)
			check("legacy", idx, hashType, got, gotN, gotErr, want, wantN, wantErr)
			for _, sh := range []*TxSigHashes{sigHashes, withUtxos} {
				for _, tokens := range []bool{false, true} {
					got, gotN, gotErr := calcBip143SignatureHash(pops, sh,
						hashType, tx, idx, amt, tokens)
					want, wantN, wantErr := refCalcBip143SignatureHash(pops,
						sh, hashType, tx, idx, amt, tokens)
					check("bip143", idx, hashType, got, gotN, gotErr,
						want, wantN, wantErr)
				}
			}
		}
	}
}

// TestSighashMatchesPrevious checks the sighash code against the reference
// code for transactions with input and output counts past the 0xfd varint
// boundary, and for random transactions with token data, missing spent
// outputs, script codes past the stack buffer and input indexes including out
// of range ones (SigHashSingle with an index past the outputs among them).
// Each transaction is checked with equivHashTypes, the random ones with random
// hash types too, and every 20th with all 256 low bytes.
func TestSighashMatchesPrevious(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(7))
	for _, c := range [][2]int{{0x101, 0x101}, {0xfd, 2}, {2, 0xfd}} {
		tx, utxos := equivTxCounts(rng, c[0], c[1])
		checkSighashMatches(t, fmt.Sprintf("%d in %d out", c[0], c[1]),
			tx, utxos, equivScript(rng), equivHashTypes, rng.Int63())
	}

	n := 100
	if testing.Short() {
		n = 50
	}
	all := make([]SigHashType, 256)
	for i := range all {
		all[i] = SigHashType(i)
	}
	for i := range n {
		tx, utxos := equivTx(rng)
		hashTypes := equivHashTypes
		if i%20 == 0 && len(tx.TxIn) <= 50 {
			hashTypes = all
		} else {
			hashTypes = append(hashTypes[:len(hashTypes):len(hashTypes)],
				SigHashType(rng.Uint32()), SigHashType(rng.Intn(256)))
		}
		checkSighashMatches(t, fmt.Sprintf("tx %d", i), tx, utxos,
			equivScript(rng), hashTypes, rng.Int63()-rng.Int63())
	}
}

// FuzzSighashMatchesPrevious checks the sighash code against the reference
// code for every transaction that decodes, with the outputs of the
// transaction itself as the spent outputs.
func FuzzSighashMatchesPrevious(f *testing.F) {
	rng := rand.New(rand.NewSource(8))
	for range 20 {
		tx, _ := equivTx(rng)
		var buf bytes.Buffer
		refSerializeTx(&buf, tx)
		f.Add(buf.Bytes(), equivScript(rng), uint32(rng.Intn(256)),
			rng.Int63(), uint8(rng.Intn(4)))
	}
	f.Fuzz(func(t *testing.T, txBytes, script []byte, hashType uint32,
		amt int64, missing uint8) {

		var tx wire.MsgTx
		if tx.Deserialize(bytes.NewReader(txBytes)) != nil || len(tx.TxIn) > 50 {
			return
		}
		utxos := NewUtxoCache()
		for i := range tx.TxIn {
			if len(tx.TxOut) > 0 && (missing == 0 || i%int(missing+1) != 0) {
				utxos.AddEntry(i, *tx.TxOut[i%len(tx.TxOut)])
			}
		}
		hashTypes := []SigHashType{SigHashType(hashType),
			SigHashType(hashType & 0xff), SigHashType(hashType&0xff) | SigHashForkID}
		checkSighashMatches(t, "fuzz", &tx, utxos, script, hashTypes, amt)
	})
}
