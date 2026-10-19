// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newAddrIndexServer returns a server with an address index over a regtest
// chain whose only block after the genesis block has a coinbase paying to
// each of the passed scripts.
func newAddrIndexServer(t *testing.T, pkScripts [][]byte) *GrpcServer {
	t.Helper()

	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// The address index refers to blocks by the ids the transaction
	// index keeps.
	txIndex := indexers.NewTxIndex(db)
	addrIndex := indexers.NewAddrIndex(db, &params)
	indexManager := indexers.NewManager(db,
		[]indexers.Indexer{txIndex, addrIndex})
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		IndexManager:       indexManager,
		UtxoCacheMaxSize:   1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatalf("blockchain.New: %v", err)
	}

	sigScript, err := txscript.NewScriptBuilder().AddInt64(1).
		AddData(make([]byte, 40)).Script()
	if err != nil {
		t.Fatalf("coinbase script: %v", err)
	}
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  sigScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	for _, pkScript := range pkScripts {
		coinbase.AddTxOut(wire.NewTxOut(1000, pkScript, wire.TokenData{}))
	}

	txs := []*bchutil.Tx{bchutil.NewTx(coinbase)}
	merkles := blockchain.BuildMerkleTreeStore(txs)
	block := wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    4,
			PrevBlock:  *params.GenesisHash,
			MerkleRoot: *merkles[len(merkles)-1],
			Timestamp:  time.Unix(time.Now().Unix(), 0),
			Bits:       params.PowLimitBits,
		},
		Transactions: []*wire.MsgTx{coinbase},
	}
	_, isOrphan, err := chain.ProcessBlock(bchutil.NewBlock(&block),
		blockchain.BFNoPoWCheck)
	if err != nil || isOrphan {
		t.Fatalf("ProcessBlock: orphan %v, %v", isOrphan, err)
	}

	return &GrpcServer{
		chain:       chain,
		chainParams: &params,
		db:          db,
		addrIndex:   addrIndex,
	}
}

// TestAddressQueriesP2SH32 ensures the transactions and unspent outputs of a
// P2SH32 address are returned, and not those of the P2SH address whose hash
// is the start of the P2SH32 hash.
func TestAddressQueriesP2SH32(t *testing.T) {
	hash32 := bytes.Repeat([]byte{0xab}, 32)
	p2sh32Script := append(append([]byte{txscript.OP_HASH256, txscript.OP_DATA_32},
		hash32...), txscript.OP_EQUAL)
	p2shScript := append(append([]byte{txscript.OP_HASH160, txscript.OP_DATA_20},
		hash32[:20]...), txscript.OP_EQUAL)
	s := newAddrIndexServer(t, [][]byte{p2shScript, p2sh32Script})

	p2sh32, err := bchutil.NewAddressScriptHash32FromHash(hash32, s.chainParams)
	if err != nil {
		t.Fatalf("NewAddressScriptHash32FromHash: %v", err)
	}
	address := p2sh32.EncodeAddress()

	txResp, err := s.GetAddressTransactions(context.Background(),
		&pb.GetAddressTransactionsRequest{Address: address})
	if err != nil {
		t.Fatalf("GetAddressTransactions: %v", err)
	}
	if len(txResp.ConfirmedTransactions) != 1 {
		t.Fatalf("got %d confirmed transactions, want 1",
			len(txResp.ConfirmedTransactions))
	}

	utxoResp, err := s.GetAddressUnspentOutputs(context.Background(),
		&pb.GetAddressUnspentOutputsRequest{Address: address})
	if err != nil {
		t.Fatalf("GetAddressUnspentOutputs: %v", err)
	}
	if len(utxoResp.Outputs) != 1 {
		t.Fatalf("got %d unspent outputs, want 1", len(utxoResp.Outputs))
	}
	out := utxoResp.Outputs[0]
	if out.Outpoint.Index != 1 || !bytes.Equal(out.PubkeyScript, p2sh32Script) {
		t.Fatalf("got output %d with script %x, want output 1 with "+
			"script %x", out.Outpoint.Index, out.PubkeyScript, p2sh32Script)
	}

	// The P2SH address only has the first output.
	p2sh, err := bchutil.NewAddressScriptHashFromHash(hash32[:20], s.chainParams)
	if err != nil {
		t.Fatalf("NewAddressScriptHashFromHash: %v", err)
	}
	utxoResp, err = s.GetAddressUnspentOutputs(context.Background(),
		&pb.GetAddressUnspentOutputsRequest{Address: p2sh.EncodeAddress()})
	if err != nil {
		t.Fatalf("GetAddressUnspentOutputs: %v", err)
	}
	if len(utxoResp.Outputs) != 1 || utxoResp.Outputs[0].Outpoint.Index != 0 {
		t.Fatalf("got %v, want only output 0", utxoResp.Outputs)
	}
}

// TestAddressQueriesLegacy ensures a legacy address gets the same results as
// its cash address, and a legacy address for another network is rejected.
func TestAddressQueriesLegacy(t *testing.T) {
	hash := bytes.Repeat([]byte{0xcd}, 20)
	p2pkhScript := append(append([]byte{txscript.OP_DUP, txscript.OP_HASH160,
		txscript.OP_DATA_20}, hash...), txscript.OP_EQUALVERIFY,
		txscript.OP_CHECKSIG)
	p2shScript := append(append([]byte{txscript.OP_HASH160, txscript.OP_DATA_20},
		hash...), txscript.OP_EQUAL)
	s := newAddrIndexServer(t, [][]byte{p2pkhScript, p2shScript})
	ctx := context.Background()

	newAddrs := []func([]byte, *chaincfg.Params) (bchutil.Address, error){
		func(h []byte, p *chaincfg.Params) (bchutil.Address, error) {
			return bchutil.NewAddressPubKeyHash(h, p)
		},
		func(h []byte, p *chaincfg.Params) (bchutil.Address, error) {
			return bchutil.NewLegacyAddressPubKeyHash(h, p)
		},
		func(h []byte, p *chaincfg.Params) (bchutil.Address, error) {
			return bchutil.NewAddressScriptHashFromHash(h, p)
		},
		func(h []byte, p *chaincfg.Params) (bchutil.Address, error) {
			return bchutil.NewLegacyAddressScriptHashFromHash(h, p)
		},
	}
	for i, newAddr := range newAddrs {
		addr, err := newAddr(hash, s.chainParams)
		if err != nil {
			t.Fatalf("%d: new address: %v", i, err)
		}
		address := addr.EncodeAddress()
		wantIndex := uint32(i / 2)

		txResp, err := s.GetAddressTransactions(ctx,
			&pb.GetAddressTransactionsRequest{Address: address})
		if err != nil {
			t.Fatalf("%s: GetAddressTransactions: %v", address, err)
		}
		if len(txResp.ConfirmedTransactions) != 1 {
			t.Fatalf("%s: got %d confirmed transactions, want 1", address,
				len(txResp.ConfirmedTransactions))
		}

		rawResp, err := s.GetRawAddressTransactions(ctx,
			&pb.GetRawAddressTransactionsRequest{Address: address})
		if err != nil {
			t.Fatalf("%s: GetRawAddressTransactions: %v", address, err)
		}
		if len(rawResp.ConfirmedTransactions) != 1 {
			t.Fatalf("%s: got %d raw confirmed transactions, want 1",
				address, len(rawResp.ConfirmedTransactions))
		}

		utxoResp, err := s.GetAddressUnspentOutputs(ctx,
			&pb.GetAddressUnspentOutputsRequest{Address: address})
		if err != nil {
			t.Fatalf("%s: GetAddressUnspentOutputs: %v", address, err)
		}
		if len(utxoResp.Outputs) != 1 ||
			utxoResp.Outputs[0].Outpoint.Index != wantIndex {

			t.Fatalf("%s: got %v, want only output %d", address,
				utxoResp.Outputs, wantIndex)
		}
	}

	mainnetAddr, err := bchutil.NewLegacyAddressPubKeyHash(hash,
		&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("NewLegacyAddressPubKeyHash: %v", err)
	}
	_, err = s.GetAddressUnspentOutputs(ctx, &pb.GetAddressUnspentOutputsRequest{
		Address: mainnetAddr.EncodeAddress(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mainnet address: got %v, want InvalidArgument", err)
	}
}

// TestAddressQueriesPubKey ensures a hex pubkey gets the history and unspent
// outputs of the P2PKH address of the key, which the address index files pay
// to pubkey outputs under.
func TestAddressQueriesPubKey(t *testing.T) {
	_, pubKey := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{0x01}, 32))
	serialized := pubKey.SerializeCompressed()
	p2pkScript := append(append([]byte{txscript.OP_DATA_33}, serialized...),
		txscript.OP_CHECKSIG)
	s := newAddrIndexServer(t, [][]byte{p2pkScript})
	ctx := context.Background()
	address := hex.EncodeToString(serialized)

	rawResp, err := s.GetRawAddressTransactions(ctx,
		&pb.GetRawAddressTransactionsRequest{Address: address})
	if err != nil {
		t.Fatalf("GetRawAddressTransactions: %v", err)
	}
	if len(rawResp.ConfirmedTransactions) != 1 {
		t.Fatalf("got %d raw confirmed transactions, want 1",
			len(rawResp.ConfirmedTransactions))
	}

	txResp, err := s.GetAddressTransactions(ctx,
		&pb.GetAddressTransactionsRequest{Address: address})
	if err != nil {
		t.Fatalf("GetAddressTransactions: %v", err)
	}
	if len(txResp.ConfirmedTransactions) != 1 {
		t.Fatalf("got %d confirmed transactions, want 1",
			len(txResp.ConfirmedTransactions))
	}

	utxoResp, err := s.GetAddressUnspentOutputs(ctx,
		&pb.GetAddressUnspentOutputsRequest{Address: address})
	if err != nil {
		t.Fatalf("GetAddressUnspentOutputs: %v", err)
	}
	if len(utxoResp.Outputs) != 1 {
		t.Fatalf("got %d unspent outputs, want 1", len(utxoResp.Outputs))
	}
}
