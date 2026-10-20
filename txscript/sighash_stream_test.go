// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/gcash/bchd/wire"
)

// randSighashScript returns a script with pushes of several sizes,
// signature opcodes and code separators.
func randSighashScript(rng *rand.Rand) []byte {
	b := NewScriptBuilder()
	for range rng.Intn(8) {
		switch rng.Intn(5) {
		case 0:
			b.AddOp(OP_CODESEPARATOR)
		case 1:
			b.AddOp(OP_CHECKSIG)
		case 2:
			b.AddData(bytes.Repeat([]byte{byte(rng.Intn(256))}, 1+rng.Intn(75)))
		case 3:
			b.AddData(bytes.Repeat([]byte{0x42}, 300+rng.Intn(3000)))
		default:
			b.AddOp(OP_DUP)
		}
	}
	script, err := b.Script()
	if err != nil {
		panic(err)
	}
	return script
}

// randSighashTx returns a random tx and a utxo cache for its inputs, with
// token data on some outputs.
func randSighashTx(rng *rand.Rand) (*wire.MsgTx, *UtxoCache) {
	randOut := func() *wire.TxOut {
		out := &wire.TxOut{Value: rng.Int63n(1e12)}
		out.PkScript = make([]byte, rng.Intn(60))
		rng.Read(out.PkScript)
		if rng.Intn(3) == 0 {
			rng.Read(out.TokenData.CategoryID[:])
			out.TokenData.BitField = wire.HAS_NFT | wire.HAS_AMOUNT |
				wire.HAS_COMMITMENT_LENGTH | byte(rng.Intn(3))
			out.TokenData.Commitment = []byte{1, 2, 3}
			out.TokenData.Amount = 1 + rng.Uint64()%wire.MAX_FT_AMOUNT
		}
		return out
	}
	tx := &wire.MsgTx{Version: 2, LockTime: rng.Uint32()}
	utxos := NewUtxoCache()
	for i := range 1 + rng.Intn(4) {
		in := &wire.TxIn{Sequence: rng.Uint32()}
		rng.Read(in.PreviousOutPoint.Hash[:])
		in.PreviousOutPoint.Index = rng.Uint32()
		in.SignatureScript = randSighashScript(rng)
		tx.TxIn = append(tx.TxIn, in)
		utxos.AddEntry(i, *randOut())
	}
	for range rng.Intn(5) {
		tx.TxOut = append(tx.TxOut, randOut())
	}
	return tx, utxos
}

// TestSighashStreamed checks the streamed sighash code against the
// buffer-based reference for every hash type, including the number of
// bytes hashed that the VM limits charge.
func TestSighashStreamed(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for n := range 40 {
		tx, utxos := randSighashTx(rng)

		sigHashes := NewTxSigHashes(tx)
		if sigHashes.HashPrevOuts != refCalcHashPrevOuts(tx) ||
			sigHashes.HashSequence != refCalcHashSequence(tx) ||
			sigHashes.HashOutputs != refCalcHashOutputs(tx) {
			t.Fatalf("tx %d: tx sighashes differ", n)
		}
		if n%2 == 0 {
			sigHashes.AddTxSigHashUtxoFromUtxoCache(tx, utxos)
			if sigHashes.HashUTXOS != refCalcHashUtxos(tx, utxos) {
				t.Fatalf("tx %d: utxos hash differs", n)
			}
		}
		if got, want := calcHashUtxos(tx, nil), refCalcHashUtxos(tx, nil); got != want {
			t.Fatalf("tx %d: utxos hash without cache differs", n)
		}

		script, err := parseScript(randSighashScript(rng))
		if err != nil {
			t.Fatal(err)
		}
		// A short token data list has to fail as before.
		var short *TxSigHashes
		if sigHashes.hasUtxos {
			c := *sigHashes
			c.tokenDataList = c.tokenDataList[:len(c.tokenDataList)-1]
			short = &c
		}
		for idx := -1; idx <= len(tx.TxIn); idx++ {
			for ht := range 256 {
				hashType := SigHashType(ht)
				amt := rng.Int63()
				check := func(name string, got []byte, gotN int, gotErr error,
					want []byte, wantN int, wantErr error) {

					if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) ||
						!bytes.Equal(got, want) || gotN != wantN {
						t.Fatalf("tx %d idx %d type %#x %s: got %x %d %v, "+
							"want %x %d %v", n, idx, ht, name, got, gotN,
							gotErr, want, wantN, wantErr)
					}
				}
				got, gotN, gotErr := calcLegacySignatureHash(script, hashType, tx, idx)
				want, wantN, wantErr := refCalcLegacySignatureHash(script, hashType, tx, idx)
				check("legacy", got, gotN, gotErr, want, wantN, wantErr)

				for _, tokens := range []bool{false, true} {
					got, gotN, gotErr := calcBip143SignatureHash(script,
						sigHashes, hashType, tx, idx, amt, tokens)
					want, wantN, wantErr := refCalcBip143SignatureHash(script,
						sigHashes, hashType, tx, idx, amt, tokens)
					check("bip143", got, gotN, gotErr, want, wantN, wantErr)
					if short != nil {
						got, gotN, gotErr := calcBip143SignatureHash(script,
							short, hashType, tx, idx, amt, tokens)
						want, wantN, wantErr := refCalcBip143SignatureHash(script,
							short, hashType, tx, idx, amt, tokens)
						if wantErr == nil && idx >= 0 && idx < len(tx.TxIn) {
							t.Fatalf("tx %d: short token data list accepted", n)
						}
						check("bip143 short", got, gotN, gotErr, want, wantN, wantErr)
					}
				}
			}
		}
	}
}

// benchSighashTx returns a tx with n p2pkh-sized inputs and outputs.
func benchSighashTx(n int) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	for i := range n {
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Index: uint32(i)},
			SignatureScript:  bytes.Repeat([]byte{0x30}, 107),
			Sequence:         0xffffffff,
		})
		tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: bytes.Repeat([]byte{0x76}, 25)})
	}
	return tx
}

// BenchmarkCalcSignatureHash measures the sighash of one input of a tx with
// 20 inputs and outputs and a p2pkh script code.
func BenchmarkCalcSignatureHash(b *testing.B) {
	tx := benchSighashTx(20)
	sigHashes := NewTxSigHashes(tx)
	script, err := parseScript(bytes.Repeat([]byte{OP_DUP}, 25))
	if err != nil {
		b.Fatal(err)
	}
	for _, c := range []struct {
		name     string
		hashType SigHashType
		bip143   bool
	}{
		{"forkid-all", SigHashAll | SigHashForkID, true},
		{"forkid-single", SigHashSingle | SigHashForkID, true},
		{"legacy-all", SigHashAll, false},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _, err := calcSignatureHash(script, sigHashes, c.hashType,
					tx, 3, 1000, c.bip143)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkNewTxSigHashes measures the per-tx sighash midstates of a tx with
// 20 inputs and outputs.
func BenchmarkNewTxSigHashes(b *testing.B) {
	tx := benchSighashTx(20)
	b.ReportAllocs()
	for b.Loop() {
		_ = NewTxSigHashes(tx)
	}
}
