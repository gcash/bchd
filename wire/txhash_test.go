// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// randScriptLen returns a script length, often one at a varint boundary.
func randScriptLen(rng *rand.Rand) int {
	lens := []int{0, 1, 25, 0xfc, 0xfd, 0xfe, 3000, 0xffff, 0x10000}
	if rng.Intn(4) == 0 {
		return lens[rng.Intn(len(lens))]
	}
	return rng.Intn(200)
}

// randTokenData returns random token data with every field combination and
// amounts of each varint size.
func randTokenData(rng *rand.Rand) TokenData {
	var td TokenData
	if rng.Intn(3) == 0 {
		return td
	}
	rng.Read(td.CategoryID[:])
	switch rng.Intn(3) {
	case 0:
		td.BitField = HAS_AMOUNT
	case 1:
		td.BitField = HAS_NFT | byte(rng.Intn(3))
	default:
		td.BitField = HAS_NFT | HAS_AMOUNT | byte(rng.Intn(3))
	}
	if td.HasNFT() && rng.Intn(2) == 0 {
		td.BitField |= HAS_COMMITMENT_LENGTH
		td.Commitment = make([]byte, 1+rng.Intn(MAX_COMMITMENT_LENGTH))
		rng.Read(td.Commitment)
	}
	if td.HasAmount() {
		amounts := []uint64{1, 0xfc, 0xfd, 0xffff, 0x10000, 0xffffffff,
			0x100000000, MAX_FT_AMOUNT}
		td.Amount = amounts[rng.Intn(len(amounts))]
	}
	return td
}

// randHashTx returns a random transaction.
func randHashTx(rng *rand.Rand) *MsgTx {
	tx := &MsgTx{Version: rng.Int31() - rng.Int31(), LockTime: rng.Uint32()}
	counts := []int{0, 1, 2, 5, 0xfd}
	for range counts[rng.Intn(len(counts))] {
		ti := &TxIn{Sequence: rng.Uint32()}
		rng.Read(ti.PreviousOutPoint.Hash[:])
		ti.PreviousOutPoint.Index = rng.Uint32()
		ti.SignatureScript = make([]byte, randScriptLen(rng))
		rng.Read(ti.SignatureScript)
		tx.TxIn = append(tx.TxIn, ti)
	}
	for range counts[rng.Intn(len(counts))] {
		to := &TxOut{Value: rng.Int63() - rng.Int63()}
		to.PkScript = make([]byte, randScriptLen(rng))
		rng.Read(to.PkScript)
		to.TokenData = randTokenData(rng)
		tx.TxOut = append(tx.TxOut, to)
	}
	return tx
}

// TestTxHashStreamed checks that hashing a transaction field by field
// matches hashing its serialization.
func TestTxHashStreamed(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	txs := []*MsgTx{&genesisCoinbaseTx, blockOne.Transactions[0], NewMsgTx(1)}
	for range 300 {
		txs = append(txs, randHashTx(rng))
	}
	for i, tx := range txs {
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		if got, want := tx.TxHash(), chainhash.DoubleHashH(buf.Bytes()); got != want {
			t.Fatalf("tx %d: TxHash %v, want %v", i, got, want)
		}

		d := chainhash.NewDoubleHasher()
		tx.SerializeToHasher(d)
		if d.Len() != buf.Len() {
			t.Fatalf("tx %d: hashed %d bytes, want %d", i, d.Len(), buf.Len())
		}
		_ = d.Sum()

		for j, to := range tx.TxOut {
			buf.Reset()
			if err := WriteTxOut(&buf, 0, 0, to); err != nil {
				t.Fatal(err)
			}
			d := chainhash.NewDoubleHasher()
			to.SerializeToHasher(d)
			if got, want := d.Sum(), chainhash.DoubleHashH(buf.Bytes()); got != want {
				t.Fatalf("tx %d out %d: got %v, want %v", i, j, got, want)
			}
		}
	}
}

// TestBlockHashFixed checks that BlockHash matches hashing the header as
// writeBlockHeader serializes it.
func TestBlockHashFixed(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	headers := []BlockHeader{blockOne.Header, {}}
	for range 100 {
		h := BlockHeader{
			Version:   rng.Int31() - rng.Int31(),
			Timestamp: time.Unix(rng.Int63n(1<<33)-(1<<31), 0),
			Bits:      rng.Uint32(),
			Nonce:     rng.Uint32(),
		}
		rng.Read(h.PrevBlock[:])
		rng.Read(h.MerkleRoot[:])
		headers = append(headers, h)
	}
	for i := range headers {
		var buf bytes.Buffer
		if err := writeBlockHeader(&buf, 0, &headers[i]); err != nil {
			t.Fatal(err)
		}
		if got, want := headers[i].BlockHash(), chainhash.DoubleHashH(buf.Bytes()); got != want {
			t.Fatalf("header %d: BlockHash %v, want %v", i, got, want)
		}
	}
}
