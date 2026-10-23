// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
	"google.golang.org/protobuf/proto"
)

// graphSearchChain returns a server whose slp index holds, from the memory
// pool, a GENESIS and a chain of n sends, each spending the token output of
// the one before, with graph search loaded and ready, and those transactions
// in that order.  Each send has an output with a script of extra bytes after
// its token output.
func graphSearchChain(t *testing.T, n, extra int) (*GrpcServer, []*wire.MsgTx) {
	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100,
		SlpGraphSearchEnabled: true})
	if err := db.Update(idx.Create); err != nil {
		t.Fatalf("Create: %v", err)
	}

	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatalf("TokenType1Genesis: %v", err)
	}
	genesis := wire.NewMsgTx(1)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{1}}, nil))
	genesis.AddTxOut(&wire.TxOut{PkScript: script})
	genesis.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	tokenID, err := goslp.GetSlpTokenID(genesis)
	if err != nil {
		t.Fatal(err)
	}
	send, err := metadatamaker.TokenType1Send(tokenID, []uint64{1000})
	if err != nil {
		t.Fatalf("TokenType1Send: %v", err)
	}
	txs := []*wire.MsgTx{genesis}
	for i := 0; i < n; i++ {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: txs[i].TxHash(),
			Index: 1}, nil))
		tx.AddTxOut(&wire.TxOut{PkScript: send})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		if extra > 0 {
			tx.AddTxOut(&wire.TxOut{Value: 546,
				PkScript: bytes.Repeat([]byte{txscript.OP_TRUE}, extra)})
		}
		txs = append(txs, tx)
	}

	serialized := make(map[chainhash.Hash][]byte, len(txs))
	for _, tx := range txs {
		err := db.View(func(dbTx database.Tx) error {
			valid, err := idx.AddPotentialSlpEntries(dbTx, tx)
			if err == nil && !valid {
				err = errors.New("not valid")
			}
			return err
		})
		if err != nil {
			t.Fatalf("AddPotentialSlpEntries(%v): %v", tx.TxHash(), err)
		}
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		serialized[tx.TxHash()] = buf.Bytes()
	}
	fetch := func(hash *chainhash.Hash) ([]byte, error) {
		if buf, ok := serialized[*hash]; ok {
			return buf, nil
		}
		return nil, errors.New("no such transaction")
	}
	idx.LoadSlpGraphSearchDb(fetch, nil, nil)
	idx.AddGraphSearchTxn(genesis)
	if _, err := idx.GetGraphSearchDb(); err != nil {
		t.Fatalf("GetGraphSearchDb: %v", err)
	}
	return &GrpcServer{db: db, slpIndex: idx, txIndex: indexers.NewTxIndex(db)},
		txs
}

// countingContext is a context that counts the times it is looked at.
type countingContext struct {
	context.Context
	looks int
}

func (c *countingContext) Err() error {
	c.looks++
	return nil
}

// TestSlpTrustedValidationCountOnce ensures GetSlpTrustedValidation runs the
// graph search from a transaction once for the queries of a request that ask
// for it with the same valid hashes, and gives each query the count a request
// of its own gets.  A request carries as many queries as fit in 4 MB, and a
// search can take the whole of a large token's graph.
func TestSlpTrustedValidationCountOnce(t *testing.T) {
	s, txs := graphSearchChain(t, 3, 0)
	hash := func(i int) []byte {
		h := txs[i].TxHash()
		return h[:]
	}
	query := func(i int, valid ...int) *pb.GetSlpTrustedValidationRequest_Query {
		q := &pb.GetSlpTrustedValidationRequest_Query{PrevOutHash: hash(i),
			PrevOutVout: 1}
		for _, v := range valid {
			q.GraphsearchValidHashes = append(q.GraphsearchValidHashes, hash(v))
		}
		return q
	}

	// Five searches: from the last send, alone, stopping at the first send,
	// and stopping at the second, from the second send, and from the
	// GENESIS.
	queries := []*pb.GetSlpTrustedValidationRequest_Query{
		query(3), query(3), query(3, 1), query(2), query(3, 1),
		query(3, 2, 0), query(0), query(3), query(3, 2, 0),
	}
	want := []uint32{4, 4, 3, 3, 3, 2, 1, 4, 2}
	ctx := &countingContext{Context: context.Background()}
	resp, err := s.GetSlpTrustedValidation(ctx,
		&pb.GetSlpTrustedValidationRequest{Queries: queries,
			IncludeGraphsearchCount: true})
	if err != nil {
		t.Fatalf("GetSlpTrustedValidation: %v", err)
	}
	if ctx.looks != 5 {
		t.Errorf("%d queries ran %d graph searches, want 5", len(queries),
			ctx.looks)
	}
	for i, q := range queries {
		if got := resp.Results[i].GraphsearchTxnCount; got != want[i] {
			t.Errorf("query %d: count %d, want %d", i, got, want[i])
		}
		one, err := s.GetSlpTrustedValidation(context.Background(),
			&pb.GetSlpTrustedValidationRequest{
				Queries:                 []*pb.GetSlpTrustedValidationRequest_Query{q},
				IncludeGraphsearchCount: true})
		if err != nil {
			t.Fatalf("query %d alone: %v", i, err)
		}
		if !proto.Equal(resp.Results[i], one.Results[0]) {
			t.Errorf("query %d: result %v, alone %v", i, resp.Results[i],
				one.Results[0])
		}
	}
}

// TestSlpTrustedValidationCountSize ensures the graph search count of
// GetSlpTrustedValidation does not serialize the transactions it counts:
// what it allocates does not grow with their size.  It once serialized every
// transaction of the search, only to count them.
func TestSlpTrustedValidationCountSize(t *testing.T) {
	const sends, extra = 2000, 10000
	allocated := func(extra int) uint64 {
		s, txs := graphSearchChain(t, sends, extra)
		tip := txs[sends].TxHash()
		req := &pb.GetSlpTrustedValidationRequest{
			Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
				PrevOutHash: tip[:],
				PrevOutVout: 1,
			}},
			IncludeGraphsearchCount: true,
		}
		least := ^uint64(0)
		for i := 0; i < 3; i++ {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			resp, err := s.GetSlpTrustedValidation(context.Background(), req)
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatalf("GetSlpTrustedValidation: %v", err)
			}
			if got := resp.Results[0].GraphsearchTxnCount; got != sends+1 {
				t.Fatalf("count %d, want %d", got, sends+1)
			}
			least = min(least, after.TotalAlloc-before.TotalAlloc)
		}
		return least
	}
	grown := int64(allocated(extra)) - int64(allocated(0))
	t.Logf("allocated %d bytes more", grown)
	if grown > 1<<20 {
		t.Errorf("counting %d transactions of %d bytes more allocated %d "+
			"bytes more", sends+1, extra, grown)
	}
}
