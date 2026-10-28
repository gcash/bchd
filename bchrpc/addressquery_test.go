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
	return newAddrIndexServerBlocks(t, pkScripts, 1)
}

// newAddrIndexServerBlocks returns a server with an address index over a
// regtest chain of the passed number of blocks after the genesis block, each
// with a coinbase paying to each of the passed scripts.
func newAddrIndexServerBlocks(t *testing.T, pkScripts [][]byte, blocks int) *GrpcServer {
	t.Helper()
	return newAddrIndexServerPays(t, blocks, func(int) [][]byte {
		return pkScripts
	})
}

// newAddrIndexServerPays returns a server with an address index over a
// regtest chain of the passed number of blocks after the genesis block, each
// with a coinbase paying to each of the scripts pays returns for its height.
func newAddrIndexServerPays(t *testing.T, blocks int, pays func(height int) [][]byte) *GrpcServer {
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

	prevHash := *params.GenesisHash
	start := time.Now().Unix()
	for height := 1; height <= blocks; height++ {
		// The height in the coinbase script keeps the coinbases apart.
		sigScript, err := txscript.NewScriptBuilder().
			AddInt64(int64(height)).AddData(make([]byte, 40)).Script()
		if err != nil {
			t.Fatalf("coinbase script: %v", err)
		}
		coinbase := wire.NewMsgTx(1)
		coinbase.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
			SignatureScript:  sigScript,
			Sequence:         wire.MaxTxInSequenceNum,
		})
		for _, pkScript := range pays(height) {
			coinbase.AddTxOut(wire.NewTxOut(1000, pkScript,
				wire.TokenData{}))
		}

		txs := []*bchutil.Tx{bchutil.NewTx(coinbase)}
		merkles := blockchain.BuildMerkleTreeStore(txs)
		block := wire.MsgBlock{
			Header: wire.BlockHeader{
				Version:    4,
				PrevBlock:  prevHash,
				MerkleRoot: *merkles[len(merkles)-1],
				Timestamp:  time.Unix(start+int64(height), 0),
				Bits:       params.PowLimitBits,
			},
			Transactions: []*wire.MsgTx{coinbase},
		}
		_, isOrphan, err := chain.ProcessBlock(bchutil.NewBlock(&block),
			blockchain.BFNoPoWCheck)
		if err != nil || isOrphan {
			t.Fatalf("ProcessBlock %d: orphan %v, %v", height, isOrphan,
				err)
		}
		prevHash = block.BlockHash()
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
	if len(txResp.ConfirmedTransactions) != 1 || txResp.ConfirmedCount != 1 {
		t.Fatalf("got %d confirmed transactions and a count of %d, want 1",
			len(txResp.ConfirmedTransactions), txResp.ConfirmedCount)
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
		if len(txResp.ConfirmedTransactions) != 1 || txResp.ConfirmedCount != 1 {
			t.Fatalf("%s: got %d confirmed transactions and a count of "+
				"%d, want 1", address, len(txResp.ConfirmedTransactions),
				txResp.ConfirmedCount)
		}

		rawResp, err := s.GetRawAddressTransactions(ctx,
			&pb.GetRawAddressTransactionsRequest{Address: address})
		if err != nil {
			t.Fatalf("%s: GetRawAddressTransactions: %v", address, err)
		}
		if len(rawResp.ConfirmedTransactions) != 1 || rawResp.ConfirmedCount != 1 {
			t.Fatalf("%s: got %d raw confirmed transactions and a count "+
				"of %d, want 1", address, len(rawResp.ConfirmedTransactions),
				rawResp.ConfirmedCount)
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
	if len(rawResp.ConfirmedTransactions) != 1 || rawResp.ConfirmedCount != 1 {
		t.Fatalf("got %d raw confirmed transactions and a count of %d, want 1",
			len(rawResp.ConfirmedTransactions), rawResp.ConfirmedCount)
	}

	txResp, err := s.GetAddressTransactions(ctx,
		&pb.GetAddressTransactionsRequest{Address: address})
	if err != nil {
		t.Fatalf("GetAddressTransactions: %v", err)
	}
	if len(txResp.ConfirmedTransactions) != 1 || txResp.ConfirmedCount != 1 {
		t.Fatalf("got %d confirmed transactions and a count of %d, want 1",
			len(txResp.ConfirmedTransactions), txResp.ConfirmedCount)
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

// TestAddressUnspentNotBareMultisig ensures a bare multisig output is not one
// of the unspent outputs of the address of its first key, which cannot spend
// it alone, while a pay-to-pubkey output of the key is.
func TestAddressUnspentNotBareMultisig(t *testing.T) {
	_, pubKey := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{0x01}, 32))
	_, otherKey := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{0x02}, 32))
	serialized := pubKey.SerializeCompressed()
	p2pkScript := append(append([]byte{txscript.OP_DATA_33}, serialized...),
		txscript.OP_CHECKSIG)
	multisig, err := txscript.NewScriptBuilder().AddOp(txscript.OP_2).
		AddData(serialized).AddData(otherKey.SerializeCompressed()).
		AddOp(txscript.OP_2).AddOp(txscript.OP_CHECKMULTISIG).Script()
	if err != nil {
		t.Fatal(err)
	}
	s := newAddrIndexServer(t, [][]byte{p2pkScript, multisig})

	resp, err := s.GetAddressUnspentOutputs(context.Background(),
		&pb.GetAddressUnspentOutputsRequest{Address: hex.EncodeToString(serialized)})
	if err != nil {
		t.Fatalf("GetAddressUnspentOutputs: %v", err)
	}
	if len(resp.Outputs) != 1 || !bytes.Equal(resp.Outputs[0].PubkeyScript, p2pkScript) {
		t.Fatalf("got %d unspent outputs, want only the pay-to-pubkey one",
			len(resp.Outputs))
	}
}

// TestAddressTransactionsCount ensures GetAddressTransactions and
// GetRawAddressTransactions give the number of confirmed transactions of the
// address whatever the skip, fetch and start block of the request, newest
// first, 0 for an address without any, and that without an address index
// both fail as before.
func TestAddressTransactionsCount(t *testing.T) {
	hash := bytes.Repeat([]byte{0xef}, 20)
	p2pkhScript := append(append([]byte{txscript.OP_DUP, txscript.OP_HASH160,
		txscript.OP_DATA_20}, hash...), txscript.OP_EQUALVERIFY,
		txscript.OP_CHECKSIG)
	const blocks = 12
	s := newAddrIndexServerBlocks(t, [][]byte{p2pkhScript}, blocks)
	ctx := context.Background()

	addr, err := bchutil.NewAddressPubKeyHash(hash, s.chainParams)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: %v", err)
	}
	unused, err := bchutil.NewAddressPubKeyHash(bytes.Repeat([]byte{0x12}, 20),
		s.chainParams)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: %v", err)
	}

	tests := []struct {
		name        string
		address     string
		skip, fetch uint32
		height      int32
		wantTxs     int
		wantNewest  int32
		wantCount   uint32
	}{
		{"all", addr.EncodeAddress(), 0, 0, 0, blocks, blocks, blocks},
		{"first page", addr.EncodeAddress(), 0, 5, 0, 5, blocks, blocks},
		{"last page", addr.EncodeAddress(), 10, 5, 0, 2, 2, blocks},
		{"past the end", addr.EncodeAddress(), 20, 5, 0, 0, 0, blocks},
		{"from a height", addr.EncodeAddress(), 0, 0, 9, 4, blocks, blocks},
		{"unused", unused.EncodeAddress(), 0, 0, 0, 0, 0, 0},
	}
	for _, test := range tests {
		req := &pb.GetAddressTransactionsRequest{Address: test.address,
			NbSkip: test.skip, NbFetch: test.fetch}
		rawReq := &pb.GetRawAddressTransactionsRequest{Address: test.address,
			NbSkip: test.skip, NbFetch: test.fetch}
		if test.height > 0 {
			req.StartBlock = &pb.GetAddressTransactionsRequest_Height{
				Height: test.height}
			rawReq.StartBlock = &pb.GetRawAddressTransactionsRequest_Height{
				Height: test.height}
		}

		resp, err := s.GetAddressTransactions(ctx, req)
		if err != nil {
			t.Fatalf("%s: GetAddressTransactions: %v", test.name, err)
		}
		if len(resp.ConfirmedTransactions) != test.wantTxs ||
			resp.ConfirmedCount != test.wantCount {

			t.Errorf("%s: got %d transactions and a count of %d, want "+
				"%d and %d", test.name, len(resp.ConfirmedTransactions),
				resp.ConfirmedCount, test.wantTxs, test.wantCount)
		}
		if test.wantTxs > 0 &&
			resp.ConfirmedTransactions[0].BlockHeight != test.wantNewest {

			t.Errorf("%s: first transaction at height %d, want %d",
				test.name, resp.ConfirmedTransactions[0].BlockHeight,
				test.wantNewest)
		}

		rawResp, err := s.GetRawAddressTransactions(ctx, rawReq)
		if err != nil {
			t.Fatalf("%s: GetRawAddressTransactions: %v", test.name, err)
		}
		if len(rawResp.ConfirmedTransactions) != test.wantTxs ||
			rawResp.ConfirmedCount != test.wantCount {

			t.Errorf("%s: got %d raw transactions and a count of %d, "+
				"want %d and %d", test.name,
				len(rawResp.ConfirmedTransactions),
				rawResp.ConfirmedCount, test.wantTxs, test.wantCount)
		}
	}

	noIndex := &GrpcServer{chain: s.chain, chainParams: s.chainParams, db: s.db}
	_, err = noIndex.GetAddressTransactions(ctx,
		&pb.GetAddressTransactionsRequest{Address: addr.EncodeAddress()})
	if status.Code(err) != codes.Unavailable {
		t.Errorf("no address index: got %v, want Unavailable", err)
	}
	_, err = noIndex.GetRawAddressTransactions(ctx,
		&pb.GetRawAddressTransactionsRequest{Address: addr.EncodeAddress()})
	if status.Code(err) != codes.Unavailable {
		t.Errorf("no address index, raw: got %v, want Unavailable", err)
	}
}

// TestAddressTransactionsCountThreshold ensures GetAddressTransactions and
// GetRawAddressTransactions give the number of confirmed transactions of an
// address with more than the 2040 the address index counts from its levels,
// whose count it stores, and of one with 2040, in every page, and that the
// pages hold that many transactions, newest first.
func TestAddressTransactionsCountThreshold(t *testing.T) {
	scripts := make([][]byte, 2)
	addrs := make([]string, 2)
	for i := range scripts {
		hash := bytes.Repeat([]byte{byte(0x30 + i)}, 20)
		scripts[i] = append(append([]byte{txscript.OP_DUP,
			txscript.OP_HASH160, txscript.OP_DATA_20}, hash...),
			txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)
		addr, err := bchutil.NewAddressPubKeyHash(hash,
			&chaincfg.RegressionNetParams)
		if err != nil {
			t.Fatal(err)
		}
		addrs[i] = addr.EncodeAddress()
	}

	// The first address is paid in every block, the second in all but
	// the first.
	const blocks = 2041
	s := newAddrIndexServerPays(t, blocks, func(height int) [][]byte {
		if height == 1 {
			return scripts[:1]
		}
		return scripts
	})
	ctx := context.Background()
	for i, wantCount := range []uint32{blocks, blocks - 1} {
		for _, fetch := range []uint32{1000, 777} {
			wantHeight := int32(blocks)
			for skip := uint32(0); ; skip += fetch {
				resp, err := s.GetAddressTransactions(ctx,
					&pb.GetAddressTransactionsRequest{Address: addrs[i],
						NbSkip: skip, NbFetch: fetch})
				if err != nil {
					t.Fatal(err)
				}
				raw, err := s.GetRawAddressTransactions(ctx,
					&pb.GetRawAddressTransactionsRequest{Address: addrs[i],
						NbSkip: skip, NbFetch: fetch})
				if err != nil {
					t.Fatal(err)
				}
				if resp.ConfirmedCount != wantCount ||
					raw.ConfirmedCount != wantCount ||
					len(raw.ConfirmedTransactions) !=
						len(resp.ConfirmedTransactions) {

					t.Fatalf("address %d skip %d: counts %d and %d, "+
						"%d and %d transactions, want a count of %d",
						i, skip, resp.ConfirmedCount, raw.ConfirmedCount,
						len(resp.ConfirmedTransactions),
						len(raw.ConfirmedTransactions), wantCount)
				}
				for _, tx := range resp.ConfirmedTransactions {
					if tx.BlockHeight != wantHeight {
						t.Fatalf("address %d skip %d: a transaction "+
							"at height %d, want %d", i, skip,
							tx.BlockHeight, wantHeight)
					}
					wantHeight--
				}
				if len(resp.ConfirmedTransactions) < int(fetch) {
					break
				}
			}
			if got := int32(blocks) - wantHeight; got != int32(wantCount) {
				t.Fatalf("address %d: pages of %d hold %d transactions, "+
					"want %d", i, fetch, got, wantCount)
			}
		}
	}
}
