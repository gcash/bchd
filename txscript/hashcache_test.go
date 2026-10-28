// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// genTestTx creates a random transaction for uses within test cases.
func genTestTx() (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(2)
	tx.Version = rand.Int31()

	numTxins := rand.Intn(11)
	for range numTxins {
		randTxIn := wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Index: uint32(rand.Int31()),
			},
			Sequence: uint32(rand.Int31()),
		}
		_, err := crand.Read(randTxIn.PreviousOutPoint.Hash[:])
		if err != nil {
			return nil, err
		}

		tx.TxIn = append(tx.TxIn, &randTxIn)
	}

	numTxouts := rand.Intn(11)
	for range numTxouts {
		randTxOut := wire.TxOut{
			Value:    rand.Int63(),
			PkScript: make([]byte, rand.Intn(30)),
		}
		if _, err := crand.Read(randTxOut.PkScript); err != nil {
			return nil, err
		}
		tx.TxOut = append(tx.TxOut, &randTxOut)
	}

	return tx, nil
}

// TestHashCacheAddContainsHashes tests that after items have been added to the
// hash cache, the ContainsHashes method returns true for all the items
// inserted.  Conversely, ContainsHashes should return false for any items
// _not_ in the hash cache.
func TestHashCacheAddContainsHashes(t *testing.T) {
	t.Parallel()

	cache := NewHashCache(10)

	var err error

	// First, we'll generate 10 random transactions for use within our
	// tests.
	const numTxns = 10
	txns := make([]*wire.MsgTx, numTxns)
	for i := range numTxns {
		txns[i], err = genTestTx()
		if err != nil {
			t.Fatalf("unable to generate test tx: %v", err)
		}
	}

	// With the transactions generated, we'll add each of them to the hash
	// cache.
	for _, tx := range txns {
		cache.AddSigHashes(tx)
	}

	// Next, we'll ensure that each of the transactions inserted into the
	// cache are properly located by the ContainsHashes method.
	for _, tx := range txns {
		txid := tx.TxHash()
		if ok := cache.ContainsHashes(&txid); !ok {
			t.Fatalf("txid %v not found in cache but should be: ",
				txid)
		}
	}

	randTx, err := genTestTx()
	if err != nil {
		t.Fatalf("unable to generate tx: %v", err)
	}

	// Finally, we'll assert that a transaction that wasn't added to the
	// cache won't be reported as being present by the ContainsHashes
	// method.
	randTxid := randTx.TxHash()
	if ok := cache.ContainsHashes(&randTxid); ok {
		t.Fatalf("txid %v wasn't inserted into cache but was found",
			randTxid)
	}
}

// TestHashCacheAddGet tests that the sighashes for a particular transaction
// are properly retrieved by the GetSigHashes function.
func TestHashCacheAddGet(t *testing.T) {
	t.Parallel()

	cache := NewHashCache(10)

	// To start, we'll generate a random transaction and compute the set of
	// sighashes for the transaction.
	randTx, err := genTestTx()
	if err != nil {
		t.Fatalf("unable to generate tx: %v", err)
	}
	sigHashes := NewTxSigHashes(randTx)

	// Next, add the transaction to the hash cache.
	cache.AddSigHashes(randTx)

	// The transaction inserted into the cache above should be found.
	txid := randTx.TxHash()
	cacheHashes, ok := cache.GetSigHashes(&txid)
	if !ok {
		t.Fatalf("tx %v wasn't found in cache", txid)
	}

	// Finally, the sighashes retrieved should exactly match the sighash
	// originally inserted into the cache.
	// TODO TODO
	if sigHashes.HashOutputs != cacheHashes.HashOutputs ||
		sigHashes.HashPrevOuts != cacheHashes.HashPrevOuts ||
		sigHashes.HashSequence != cacheHashes.HashSequence {
		t.Fatalf("sighashes don't match: expected %v, got %v",
			spew.Sdump(sigHashes), spew.Sdump(cacheHashes))
	}
}

// TestHashCachePurge tests that items are able to be properly removed from the
// hash cache.
func TestHashCachePurge(t *testing.T) {
	t.Parallel()

	cache := NewHashCache(10)

	var err error

	// First we'll start by inserting numTxns transactions into the hash cache.
	const numTxns = 10
	txns := make([]*wire.MsgTx, numTxns)
	for i := range numTxns {
		txns[i], err = genTestTx()
		if err != nil {
			t.Fatalf("unable to generate test tx: %v", err)
		}
	}
	for _, tx := range txns {
		cache.AddSigHashes(tx)
	}

	// Once all the transactions have been inserted, we'll purge them from
	// the hash cache.
	for _, tx := range txns {
		txid := tx.TxHash()
		cache.PurgeSigHashes(&txid)
	}

	// At this point, none of the transactions inserted into the hash cache
	// should be found within the cache.
	for _, tx := range txns {
		txid := tx.TxHash()
		if ok := cache.ContainsHashes(&txid); ok {
			t.Fatalf("tx %v found in cache but should have "+
				"been purged: ", txid)
		}
	}
}

// TestHashCacheMaxEntries ensures the hash cache holds at most its maximum
// number of entries, evicting others to make room for each new one, and that
// AddSigHashes returns the sighashes it computed, whether it keeps them or
// not.
func TestHashCacheMaxEntries(t *testing.T) {
	t.Parallel()

	for _, maxEntries := range []uint{5, 1, 0} {
		cache := NewHashCache(maxEntries)
		for i := range 20 {
			tx, err := genTestTx()
			if err != nil {
				t.Fatalf("unable to generate test tx: %v", err)
			}
			sigHashes := cache.AddSigHashes(tx)
			if !reflect.DeepEqual(sigHashes, NewTxSigHashes(tx)) {
				t.Fatalf("max %d: tx %d: AddSigHashes returned %v",
					maxEntries, i, spew.Sdump(sigHashes))
			}
			txid := tx.TxHash()
			cached, ok := cache.GetSigHashes(&txid)
			if ok != (maxEntries > 0) || ok && cached != sigHashes {
				t.Fatalf("max %d: tx %d: cached %v, as returned %v",
					maxEntries, i, ok, cached == sigHashes)
			}
			want := min(uint(i+1), maxEntries)
			if n := uint(len(cache.sigHashes)); n != want {
				t.Fatalf("max %d: %d entries after %d adds, want %d",
					maxEntries, n, i+1, want)
			}
		}
	}
}

// TestTxSigHashesWithUtxos ensures WithUtxos returns the same sighashes as
// AddTxSigHashUtxoFromUtxoCache without modifying the original ones.
func TestTxSigHashesWithUtxos(t *testing.T) {
	t.Parallel()

	tx, utxos := spentOutputsTestTx(nil)
	sigHashes := NewTxSigHashes(tx)
	orig := *sigHashes

	want := NewTxSigHashes(tx)
	want.AddTxSigHashUtxoFromUtxoCache(tx, utxos)
	got := sigHashes.WithUtxos(tx, utxos)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithUtxos: got %v, want %v", spew.Sdump(got),
			spew.Sdump(want))
	}
	if !reflect.DeepEqual(*sigHashes, orig) {
		t.Fatal("WithUtxos modified the original sighashes")
	}
}

// spentOutputsTestTx returns a transaction spending one output for each of
// the passed scripts, the first of them with fungible tokens, along with the
// spent outputs.
func spentOutputsTestTx(pkScripts [][]byte) (*wire.MsgTx, *UtxoCache) {
	if pkScripts == nil {
		pkScripts = [][]byte{{OP_TRUE}, {OP_TRUE}}
	}
	tx := wire.NewMsgTx(2)
	utxos := NewUtxoCache()
	for i, pkScript := range pkScripts {
		prevHash := chainhash.DoubleHashH([]byte{byte(i)})
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, 1), nil))
		var tokenData wire.TokenData
		if i == 0 {
			tokenData = wire.TokenData{
				CategoryID: prevHash,
				Amount:     1000,
				BitField:   wire.HAS_AMOUNT,
			}
		}
		utxos.AddEntry(i, *wire.NewTxOut(100000, pkScript, tokenData))
	}
	tx.AddTxOut(wire.NewTxOut(90000, []byte{OP_TRUE}, wire.TokenData{}))
	tx.AddTxOut(wire.NewTxOut(90000, []byte{OP_TRUE}, wire.TokenData{}))
	return tx, utxos
}

// TestCheckSigSpentOutputs ensures OP_CHECKSIG and OP_CHECKMULTISIG compute
// the expected signature hash for every hash type, with tokens enabled
// and disabled, whether or not the passed sighashes include the spent outputs,
// and that the passed sighashes are not modified.
func TestCheckSigSpentOutputs(t *testing.T) {
	t.Parallel()

	priv, pub := bchec.PrivKeyFromBytes(bchec.S256(),
		bytes.Repeat([]byte{1}, 32))
	pubKey := pub.SerializeCompressed()
	checkSig, err := NewScriptBuilder().AddData(pubKey).AddOp(OP_CHECKSIG).
		Script()
	if err != nil {
		t.Fatal(err)
	}
	multiSig, err := NewScriptBuilder().AddOp(OP_1).AddData(pubKey).
		AddOp(OP_1).AddOp(OP_CHECKMULTISIG).Script()
	if err != nil {
		t.Fatal(err)
	}
	pkScripts := [][]byte{checkSig, multiSig, checkSig}
	tx, utxos := spentOutputsTestTx(pkScripts)

	for _, tokens := range []bool{false, true} {
		flags := StandardVerifyFlags
		if tokens {
			flags |= ScriptAllowCashTokens
		}
		for _, base := range []SigHashType{SigHashAll, SigHashNone, SigHashSingle} {
			for _, extra := range []SigHashType{0, SigHashAnyOneCanPay, SigHashUTXO} {
				if extra == SigHashUTXO && !tokens {
					continue
				}
				hashType := base | extra | SigHashForkID

				// Sign with sighashes computed from scratch, including
				// the spent outputs when tokens are enabled.
				signing := NewTxSigHashes(tx)
				if tokens {
					signing.AddTxSigHashUtxoFromUtxoCache(tx, utxos)
				}
				for i, pkScript := range pkScripts {
					hash, _, err := CalcSignatureHash(pkScript, signing,
						hashType, tx, i, 100000, true)
					if err != nil {
						t.Fatal(err)
					}
					builder := NewScriptBuilder()
					var sig *bchec.Signature
					if bytes.Equal(pkScript, multiSig) {
						builder.AddOp(OP_0)
						sig, err = priv.SignECDSA(hash)
					} else {
						sig, err = priv.SignSchnorr(hash)
					}
					if err != nil {
						t.Fatal(err)
					}
					builder.AddData(append(sig.Serialize(), byte(hashType)))
					tx.TxIn[i].SignatureScript, err = builder.Script()
					if err != nil {
						t.Fatal(err)
					}
				}

				passed := []*TxSigHashes{nil, NewTxSigHashes(tx)}
				if tokens {
					passed = append(passed,
						NewTxSigHashes(tx).WithUtxos(tx, utxos))
				}
				for i, pkScript := range pkScripts {
					for j, sigHashes := range passed {
						var orig TxSigHashes
						if sigHashes != nil {
							orig = *sigHashes
						}
						vm, err := NewEngine(pkScript, tx, i, flags, nil,
							sigHashes, utxos, 100000)
						if err != nil {
							t.Fatal(err)
						}
						if err := vm.Execute(); err != nil {
							t.Errorf("tokens %v, hash type %#x, input "+
								"%d, sighashes %d: %v", tokens,
								hashType, i, j, err)
						}
						if sigHashes != nil &&
							!reflect.DeepEqual(*sigHashes, orig) {

							t.Errorf("tokens %v, hash type %#x, input "+
								"%d, sighashes %d: passed sighashes "+
								"modified", tokens, hashType, i, j)
						}
					}
				}
			}
		}
	}
}

// TestUtxoCacheMissingEntry ensures looking up a missing entry releases the
// cache lock, so the cache can still be written to afterwards.
func TestUtxoCacheMissingEntry(t *testing.T) {
	t.Parallel()

	cache := NewUtxoCache()
	if _, err := cache.GetEntry(3); err == nil {
		t.Fatal("missing entry found")
	}
	done := make(chan struct{})
	go func() {
		cache.AddEntry(3, wire.TxOut{Value: 5})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("AddEntry blocked after a missing GetEntry")
	}
	if out, err := cache.GetEntry(3); err != nil || out.Value != 5 {
		t.Fatalf("GetEntry: %v %v", out, err)
	}
}

// TestCalcSignatureHashUtxos ensures CalcSignatureHash returns an error for a
// hash type with SigHashUTXO when the sighashes lack an output the
// transaction spends, instead of hashing a zero hash for them, and computes
// it when they have them all.  It also ensures neither a hash type without
// SigHashUTXO nor the legacy algorithm, which covers no spent outputs, needs
// them.
func TestCalcSignatureHashUtxos(t *testing.T) {
	t.Parallel()

	tx, utxos := spentOutputsTestTx(nil)
	partial := NewUtxoCache()
	entry, err := utxos.GetEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	partial.AddEntry(0, entry)

	tests := []struct {
		name      string
		sigHashes *TxSigHashes
		ok        bool
	}{
		{"with the spent outputs", NewTxSigHashes(tx).WithUtxos(tx, utxos),
			true},
		{"without them", NewTxSigHashes(tx), false},
		{"with some", NewTxSigHashes(tx).WithUtxos(tx, partial), false},
		{"with none found", NewTxSigHashes(tx).WithUtxos(tx, nil), false},
	}
	hashType := SigHashAll | SigHashForkID | SigHashUTXO
	for _, test := range tests {
		for idx := range tx.TxIn {
			_, _, err := CalcSignatureHash([]byte{OP_TRUE}, test.sigHashes,
				hashType, tx, idx, 100000, true)
			if test.ok && err != nil {
				t.Errorf("%s, input %d: %v", test.name, idx, err)
			} else if !test.ok && err == nil {
				t.Errorf("%s, input %d: no error", test.name, idx)
			}
		}
	}

	_, _, err = CalcSignatureHash([]byte{OP_TRUE}, NewTxSigHashes(tx),
		SigHashAll|SigHashForkID, tx, 0, 100000, true)
	if err != nil {
		t.Errorf("without SigHashUTXO: %v", err)
	}
	_, _, err = CalcSignatureHash([]byte{OP_TRUE}, nil, hashType, tx, 0,
		100000, false)
	if err != nil {
		t.Errorf("legacy algorithm: %v", err)
	}
}

// TestSigHashesOtherInputs ensures CalcSignatureHash, and the engine, return
// an error instead of panicking or hashing for sighashes that include the
// spent outputs of a transaction with another number of inputs, and that the
// engine verifies with those of the transaction itself.
func TestSigHashesOtherInputs(t *testing.T) {
	t.Parallel()

	priv, pub := bchec.PrivKeyFromBytes(bchec.S256(),
		bytes.Repeat([]byte{1}, 32))
	checkSig, err := NewScriptBuilder().AddData(pub.SerializeCompressed()).
		AddOp(OP_CHECKSIG).Script()
	if err != nil {
		t.Fatal(err)
	}
	tx, utxos := spentOutputsTestTx([][]byte{checkSig, checkSig})
	sigHashes := NewTxSigHashes(tx).WithUtxos(tx, utxos)
	hashType := SigHashAll | SigHashForkID
	for idx := range tx.TxIn {
		hash, _, err := CalcSignatureHash(checkSig, sigHashes, hashType, tx,
			idx, 100000, true)
		if err != nil {
			t.Fatal(err)
		}
		sig, err := priv.SignSchnorr(hash)
		if err != nil {
			t.Fatal(err)
		}
		tx.TxIn[idx].SignatureScript, err = NewScriptBuilder().
			AddData(append(sig.Serialize(), byte(hashType))).Script()
		if err != nil {
			t.Fatal(err)
		}
	}

	// The first input alone, with the signature made with the sighashes
	// of both, which match it but for the hash of the inputs.
	first := tx.Copy()
	first.TxIn = first.TxIn[:1]

	// try runs f, failing the test if it panics.
	try := func(msg string, f func()) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("%s: panicked: %v", msg, r)
			}
		}()
		f()
	}
	flags := StandardVerifyFlags | ScriptAllowCashTokens
	tests := []struct {
		name      string
		tx        *wire.MsgTx
		sigHashes *TxSigHashes
	}{
		{"fewer inputs", tx, NewTxSigHashes(first).WithUtxos(first, utxos)},
		{"more inputs", first, sigHashes},
	}
	for _, test := range tests {
		for idx := range test.tx.TxIn {
			msg := fmt.Sprintf("%s, input %d", test.name, idx)
			try(msg+", CalcSignatureHash", func() {
				_, _, err := CalcSignatureHash(checkSig, test.sigHashes,
					hashType, test.tx, idx, 100000, true)
				if err == nil {
					t.Errorf("%s: CalcSignatureHash hashed", msg)
				}
			})
			try(msg+", engine", func() {
				vm, err := NewEngine(checkSig, test.tx, idx, flags, nil,
					test.sigHashes, utxos, 100000)
				if err != nil {
					t.Fatalf("%s: %v", msg, err)
				}
				if vm.Execute() == nil {
					t.Errorf("%s: the engine verified", msg)
				}
			})
		}
	}

	for idx := range tx.TxIn {
		vm, err := NewEngine(checkSig, tx, idx, flags, nil, sigHashes, utxos,
			100000)
		if err == nil {
			err = vm.Execute()
		}
		if err != nil {
			t.Errorf("matching sighashes, input %d: %v", idx, err)
		}
	}
}
