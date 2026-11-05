// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"testing"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// bcmrTestBlock returns a block at height on prev with a coinbase and txs, and
// the outputs its transactions spend, each from the height in heights of the
// transaction it spends.
func bcmrTestBlock(t *testing.T, height int32, prev *chainhash.Hash,
	heights map[chainhash.Hash]int32, txs ...*wire.MsgTx) (*bchutil.Block, []blockchain.SpentTxOut) {

	t.Helper()
	header := wire.BlockHeader{Version: 1, Nonce: uint32(height)}
	if prev != nil {
		header.PrevBlock = *prev
	}
	msgBlock := wire.NewMsgBlock(&header)
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{txscript.OP_DATA_4, byte(height), 0, 0, 0},
	})
	coinbase.AddTxOut(wire.NewTxOut(5000000000, []byte{txscript.OP_TRUE}, wire.TokenData{}))
	if err := msgBlock.AddTransaction(coinbase); err != nil {
		t.Fatal(err)
	}
	var stxos []blockchain.SpentTxOut
	for _, tx := range txs {
		if err := msgBlock.AddTransaction(tx); err != nil {
			t.Fatal(err)
		}
		for _, in := range tx.TxIn {
			prevHeight, ok := heights[in.PreviousOutPoint.Hash]
			if !ok {
				prevHeight = height
			}
			stxos = append(stxos, blockchain.SpentTxOut{Height: prevHeight})
		}
	}
	merkles := blockchain.BuildMerkleTreeStore(bchutil.NewBlock(msgBlock).Transactions())
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	block := bchutil.NewBlock(msgBlock)
	block.SetHeight(height)
	for _, tx := range block.Transactions() {
		heights[*tx.Hash()] = height
	}
	return block, stxos
}

// bcmrTestOut returns an output that pays to OP_TRUE, with token data.
func bcmrTestOut(tokenData wire.TokenData) *wire.TxOut {
	return wire.NewTxOut(1000, []byte{txscript.OP_TRUE}, tokenData)
}

// bcmrTestPublication returns a publication output of a hash of 32 hashByte
// bytes and a URI.
func bcmrTestPublication(hashByte byte, uri string) *wire.TxOut {
	script := []byte{txscript.OP_RETURN, txscript.OP_DATA_4, 'B', 'C', 'M', 'R', txscript.OP_DATA_32}
	script = append(script, bytes.Repeat([]byte{hashByte}, 32)...)
	script = append(script, byte(len(uri)))
	script = append(script, uri...)
	return wire.NewTxOut(0, script, wire.TokenData{})
}

// newBcmrTestServer returns a gRPC server with a BCMR index of a token: an
// authbase and its genesis, which publishes, in block 1, and a link in block
// 2, and an empty mempool.
func newBcmrTestServer(t *testing.T) (*GrpcServer, []*wire.MsgTx, []*bchutil.Block) {
	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewBcmrIndex(db, &chaincfg.RegressionNetParams)
	if err := db.Update(idx.Create); err != nil {
		t.Fatal(err)
	}
	if err := idx.Init(); err != nil {
		t.Fatal(err)
	}

	authbase := wire.NewMsgTx(2)
	authbase.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{1}, Index: 1}, nil))
	authbase.AddTxOut(bcmrTestOut(wire.TokenData{}))
	category := authbase.TxHash()
	tokens := wire.TokenData{CategoryID: category, Amount: 1000, BitField: wire.HAS_AMOUNT}
	genesis := wire.NewMsgTx(2)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: category}, nil))
	genesis.AddTxOut(bcmrTestOut(tokens))
	genesis.AddTxOut(bcmrTestPublication(2, "g.example"))
	link := wire.NewMsgTx(2)
	link.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: genesis.TxHash()}, nil))
	link.AddTxOut(bcmrTestOut(tokens))

	heights := map[chainhash.Hash]int32{{1}: 0}
	block1, stxos1 := bcmrTestBlock(t, 1, nil, heights, genesis, authbase)
	block2, stxos2 := bcmrTestBlock(t, 2, block1.Hash(), heights, link)
	for _, b := range []struct {
		block *bchutil.Block
		stxos []blockchain.SpentTxOut
	}{{block1, stxos1}, {block2, stxos2}} {
		err := db.Update(func(dbTx database.Tx) error {
			if err := dbTx.StoreBlock(b.block); err != nil {
				return err
			}
			return idx.ConnectBlock(dbTx, b.block, b.stxos)
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	s := &GrpcServer{bcmrIndex: idx, txMemPool: mempool.New(&mempool.Config{})}
	return s, []*wire.MsgTx{authbase, genesis, link}, []*bchutil.Block{block1, block2}
}

// TestGetBcmrIdentity checks the GetBcmrIdentity answer for a token, a page of
// its authchain, and the errors.
func TestGetBcmrIdentity(t *testing.T) {
	s, txs, blocks := newBcmrTestServer(t)
	authbase, genesis, link := txs[0], txs[1], txs[2]
	category, genesisHash, linkHash := authbase.TxHash(), genesis.TxHash(), link.TxHash()
	ctx := context.Background()

	resp, err := s.GetBcmrIdentity(ctx, &pb.GetBcmrIdentityRequest{Authbase: category[:]})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(resp.Authbase, category[:]) || resp.Status != pb.GetBcmrIdentityResponse_ACTIVE ||
		!bytes.Equal(resp.Head.Hash, linkHash[:]) || resp.Head.Height != 2 ||
		!bytes.Equal(resp.Head.BlockHash, blocks[1].Hash()[:]) || resp.Head.Publication != nil ||
		len(resp.Head.Transaction) != 0 {

		t.Errorf("authbase %x, status %v, head %v", resp.Authbase, resp.Status, resp.Head)
	}
	pub := resp.Publication
	if pub == nil || !bytes.Equal(pub.Hash, genesisHash[:]) || resp.PublicationCurrent ||
		pub.Publication == nil || pub.Publication.Output != 1 ||
		!bytes.Equal(pub.Publication.Hash, bytes.Repeat([]byte{2}, 32)) ||
		len(pub.Publication.Uris) != 1 || pub.Publication.Uris[0] != "g.example" {

		t.Errorf("publication %v, current %v", pub, resp.PublicationCurrent)
	}
	if !resp.HasGenesisSupply || resp.GenesisSupply != 1000 || resp.IdentityOutput == nil ||
		!bytes.Equal(resp.IdentityOutput.CategoryId, category[:]) || resp.IdentityOutput.Amount != 1000 {

		t.Errorf("genesis supply %v, identity output %v", resp.GenesisSupply, resp.IdentityOutput)
	}
	if len(resp.Authchain) != 0 || resp.Proof != nil || resp.More {
		t.Error("the answer has an authchain it wasn't asked for")
	}

	// A page that doesn't reach the authhead, and so has no proof.
	resp, err = s.GetBcmrIdentity(ctx, &pb.GetBcmrIdentityRequest{
		Authbase: category[:], IncludeAuthchain: true, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Authchain) != 2 || resp.FirstIndex != 0 || !resp.More || resp.Proof != nil {
		t.Fatalf("page of %d links from %d, more %v, proof %v", len(resp.Authchain),
			resp.FirstIndex, resp.More, resp.Proof)
	}
	for i, tx := range []*wire.MsgTx{authbase, genesis} {
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		hash := tx.TxHash()
		entry := resp.Authchain[i]
		if !bytes.Equal(entry.Hash, hash[:]) || !bytes.Equal(entry.Transaction, buf.Bytes()) ||
			entry.Height != 1 || !bytes.Equal(entry.BlockHash, blocks[0].Hash()[:]) {

			t.Errorf("link %d: %v", i, entry)
		}
	}

	// Errors.
	unknown := chainhash.Hash{7}
	checks := []struct {
		name string
		s    *GrpcServer
		req  *pb.GetBcmrIdentityRequest
		code codes.Code
	}{
		{"no index", &GrpcServer{}, &pb.GetBcmrIdentityRequest{Authbase: category[:]}, codes.Unavailable},
		{"bad authbase", s, &pb.GetBcmrIdentityRequest{Authbase: []byte{1, 2, 3}}, codes.InvalidArgument},
		{"no identity", s, &pb.GetBcmrIdentityRequest{Authbase: unknown[:]}, codes.NotFound},
		{"after without the authchain", s, &pb.GetBcmrIdentityRequest{
			Authbase: category[:], After: genesisHash[:]}, codes.InvalidArgument},
		{"limit without the authchain", s, &pb.GetBcmrIdentityRequest{
			Authbase: category[:], Limit: 1}, codes.InvalidArgument},
		{"bad after", s, &pb.GetBcmrIdentityRequest{
			Authbase: category[:], IncludeAuthchain: true, After: []byte{1}}, codes.InvalidArgument},
		{"after no link", s, &pb.GetBcmrIdentityRequest{
			Authbase: category[:], IncludeAuthchain: true, After: unknown[:]}, codes.NotFound},
	}
	for _, check := range checks {
		_, err := check.s.GetBcmrIdentity(ctx, check.req)
		if status.Code(err) != check.code {
			t.Errorf("%s: %v, want %v", check.name, err, check.code)
		}
	}
}
