// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// The tested address is the P2SH address of an OP_TRUE redeem script
	// and the other outputs pay to the P2SH address of OP_2, so both can
	// be spent without signatures.
	addrRedeem  = []byte{txscript.OP_TRUE}
	otherRedeem = []byte{txscript.OP_2}
)

// p2shScript returns the P2SH output script for the redeem script.
func p2shScript(tb testing.TB, redeem []byte) []byte {
	tb.Helper()

	addr, err := bchutil.NewAddressScriptHash(redeem,
		&chaincfg.RegressionNetParams)
	if err != nil {
		tb.Fatalf("NewAddressScriptHash: %v", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		tb.Fatalf("PayToAddrScript: %v", err)
	}
	return pkScript
}

// addrUnspentHarness is a regtest chain with an address index and a mempool
// that indexes its transactions.
type addrUnspentHarness struct {
	tb       testing.TB
	s        *GrpcServer
	chain    *blockchain.BlockChain
	pool     *mempool.TxPool
	txIndex  *indexers.TxIndex
	params   *chaincfg.Params
	height   int32
	prevHash chainhash.Hash
	start    time.Time
}

func newAddrUnspentHarness(tb testing.TB) *addrUnspentHarness {
	tb.Helper()
	return newAddrUnspentHarnessDB(tb, nil)
}

// newAddrUnspentHarnessDB is newAddrUnspentHarness with the address index
// reading the database through indexDB(db) when indexDB is not nil.
func newAddrUnspentHarnessDB(tb testing.TB,
	indexDB func(database.DB) database.DB) *addrUnspentHarness {

	tb.Helper()
	return newAddrUnspentHarnessWith(tb, indexDB, nil)
}

// newAddrUnspentHarnessWith is newAddrUnspentHarnessDB with the chain calling
// its index manager through manager(m) when manager is not nil.
func newAddrUnspentHarnessWith(tb testing.TB,
	indexDB func(database.DB) database.DB,
	manager func(blockchain.IndexManager) blockchain.IndexManager) *addrUnspentHarness {

	tb.Helper()

	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", tb.TempDir(), params.Net)
	if err != nil {
		tb.Fatalf("database.Create: %v", err)
	}
	tb.Cleanup(func() { db.Close() })

	txIndex := indexers.NewTxIndex(db)
	addrIndexDB := db
	if indexDB != nil {
		addrIndexDB = indexDB(db)
	}
	addrIndex := indexers.NewAddrIndex(addrIndexDB, &params)
	var indexManager blockchain.IndexManager = indexers.NewManager(db,
		[]indexers.Indexer{txIndex, addrIndex})
	if manager != nil {
		indexManager = manager(indexManager)
	}
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		IndexManager:       indexManager,
		UtxoCacheMaxSize:   1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		tb.Fatalf("blockchain.New: %v", err)
	}

	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			AcceptNonStd: true,
			MaxTxVersion: 2,
		},
		ChainParams:    &params,
		FetchUtxoView:  chain.FetchUtxoView,
		BestHeight:     func() int32 { return chain.BestSnapshot().Height },
		MedianTimePast: func() time.Time { return chain.BestSnapshot().MedianTime },
		CalcSequenceLock: func(tx *bchutil.Tx,
			view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {

			return chain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: chain.IsDeploymentActive,
		AddrIndex:          addrIndex,
	})

	return &addrUnspentHarness{
		tb:      tb,
		chain:   chain,
		pool:    pool,
		txIndex: txIndex,
		s: &GrpcServer{
			chain:       chain,
			chainParams: &params,
			db:          db,
			addrIndex:   addrIndex,
			txMemPool:   pool,
		},
		params:   &params,
		prevHash: *params.GenesisHash,
		start:    time.Now().Add(-24 * time.Hour).Truncate(time.Second),
	}
}

// mineBlock connects a block holding a coinbase with the passed outputs and
// the passed transactions, and returns the coinbase.  The transactions are
// not validated, so they may spend immature coinbases.
func (h *addrUnspentHarness) mineBlock(cbOuts []*wire.TxOut,
	txs []*wire.MsgTx) *wire.MsgTx {

	h.tb.Helper()

	coinbase := h.coinbaseTx(h.height+1, cbOuts)
	h.mineBlockWithCoinbase(coinbase, txs)
	return coinbase
}

// coinbaseTx returns a coinbase for the height with the passed outputs.
func (h *addrUnspentHarness) coinbaseTx(height int32,
	cbOuts []*wire.TxOut) *wire.MsgTx {

	h.tb.Helper()

	sigScript, err := txscript.NewScriptBuilder().AddInt64(int64(height)).
		AddData(make([]byte, 40)).Script()
	if err != nil {
		h.tb.Fatalf("coinbase script: %v", err)
	}
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  sigScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	for _, out := range cbOuts {
		coinbase.AddTxOut(out)
	}
	return coinbase
}

// mineBlockWithCoinbase connects a block holding the coinbase and the passed
// transactions.
func (h *addrUnspentHarness) mineBlockWithCoinbase(coinbase *wire.MsgTx,
	txs []*wire.MsgTx) {

	h.tb.Helper()

	h.height++
	sorted := append([]*wire.MsgTx(nil), txs...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i].TxHash(), sorted[j].TxHash()
		return a.Compare(&b) < 0
	})
	msgTxs := append([]*wire.MsgTx{coinbase}, sorted...)
	utilTxs := make([]*bchutil.Tx, len(msgTxs))
	for i, tx := range msgTxs {
		utilTxs[i] = bchutil.NewTx(tx)
	}
	merkles := blockchain.BuildMerkleTreeStore(utilTxs)
	block := wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    4,
			PrevBlock:  h.prevHash,
			MerkleRoot: *merkles[len(merkles)-1],
			Timestamp:  h.start.Add(time.Duration(h.height) * time.Second),
			Bits:       h.params.PowLimitBits,
		},
		Transactions: msgTxs,
	}
	_, isOrphan, err := h.chain.ProcessBlock(bchutil.NewBlock(&block),
		blockchain.BFFastAdd|blockchain.BFNoPoWCheck)
	if err != nil || isOrphan {
		h.tb.Fatalf("ProcessBlock: orphan %v, %v", isOrphan, err)
	}
	h.prevHash = block.BlockHash()
}

// acceptTx adds the transaction to the mempool.
func (h *addrUnspentHarness) acceptTx(tx *wire.MsgTx) {
	h.tb.Helper()

	_, _, err := h.pool.MaybeAcceptTransaction(bchutil.NewTx(tx), true)
	if err != nil {
		h.tb.Fatalf("MaybeAcceptTransaction: %v", err)
	}
}

// spendTx returns a transaction spending the P2SH output with the redeem
// script and paying to the outputs.
func spendTx(tb testing.TB, prev wire.OutPoint, redeem []byte,
	outs ...*wire.TxOut) *wire.MsgTx {

	tb.Helper()

	sigScript, err := txscript.NewScriptBuilder().AddData(redeem).Script()
	if err != nil {
		tb.Fatalf("sig script: %v", err)
	}
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: prev,
		SignatureScript:  sigScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	for _, out := range outs {
		tx.AddTxOut(out)
	}
	return tx
}

func outPoint(tx *wire.MsgTx, index uint32) wire.OutPoint {
	return wire.OutPoint{Hash: tx.TxHash(), Index: index}
}

// unspentKey identifies an unspent output with all its fields.
func unspentKey(u *pb.UnspentOutput) string {
	var hash chainhash.Hash
	copy(hash[:], u.Outpoint.Hash)
	token := "none"
	if u.CashToken != nil {
		token = fmt.Sprintf("%x/%d/%x/%x", u.CashToken.CategoryId,
			u.CashToken.Amount, u.CashToken.Commitment, u.CashToken.Bitfield)
	}
	return fmt.Sprintf("%v:%d value=%d script=%x coinbase=%v height=%d "+
		"token=%s slp=%v", hash, u.Outpoint.Index, u.Value, u.PubkeyScript,
		u.IsCoinbase, u.BlockHeight, token, u.SlpToken)
}

// wantUnspent returns the key of the output as an unspent output.
func wantUnspent(tx *wire.MsgTx, index uint32, coinbase bool,
	height int32) string {

	out := tx.TxOut[index]
	var token *pb.CashToken
	if !out.TokenData.IsEmpty() {
		token = getPbCashTokenDataFromTokenData(out.TokenData)
	}
	hash := tx.TxHash()
	return unspentKey(&pb.UnspentOutput{
		Outpoint:     &pb.Transaction_Input_Outpoint{Hash: hash[:], Index: index},
		Value:        out.Value,
		PubkeyScript: out.PkScript,
		CashToken:    token,
		IsCoinbase:   coinbase,
		BlockHeight:  height,
	})
}

// TestGetAddressUnspentOutputsSpends ensures the unspent outputs of an
// address leave out outputs spent in the chain, and with the mempool
// included, leave out outputs spent in the mempool and add the unspent
// outputs of mempool transactions.
func TestGetAddressUnspentOutputsSpends(t *testing.T) {
	h := newAddrUnspentHarness(t)
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}

	// Height 1: a coinbase paying the address twice, once to be spent.
	cb1 := h.mineBlock([]*wire.TxOut{
		pay(50e8, addrScript),
		pay(25e8, addrScript),
		pay(25e8, otherScript),
	}, nil)

	// Height 2: tx1 creates a token for the address, tx2 pays the
	// address from an output of another address.
	amount := uint64(1000)
	tokenData, err := wire.NewTokenData(cb1.TxHash(), &amount, nil, nil)
	if err != nil {
		t.Fatalf("NewTokenData: %v", err)
	}
	tx1 := spendTx(t, outPoint(cb1, 0), addrRedeem,
		pay(10e8, addrScript),
		wire.NewTxOut(20e8, addrScript, *tokenData),
		pay(19e8, otherScript))
	tx2 := spendTx(t, outPoint(cb1, 2), otherRedeem,
		pay(5e8, addrScript),
		pay(5e8, addrScript),
		pay(14e8, otherScript))
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
		[]*wire.MsgTx{tx1, tx2})

	// Height 3: tx3 spends a confirmed output of the address.
	tx3 := spendTx(t, outPoint(tx1, 0), addrRedeem,
		pay(4e8, otherScript),
		pay(5e8, addrScript))
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
		[]*wire.MsgTx{tx3})

	// The mempool spends a confirmed output of the address (m1), an
	// output of m1 (m2), and the token output (m4), and pays the address
	// from an output of another address (m3).
	m1 := spendTx(t, outPoint(tx2, 0), addrRedeem,
		pay(1e8, otherScript),
		pay(3e8, addrScript))
	m2 := spendTx(t, outPoint(m1, 1), addrRedeem,
		pay(2e8, addrScript),
		pay(5e7, otherScript))
	m3 := spendTx(t, outPoint(tx3, 0), otherRedeem,
		pay(3e8, addrScript),
		pay(5e7, otherScript))
	m4 := spendTx(t, outPoint(tx1, 1), addrRedeem,
		wire.NewTxOut(19e8, addrScript, *tokenData),
		pay(5e7, otherScript))
	for _, tx := range []*wire.MsgTx{m1, m2, m3, m4} {
		h.acceptTx(tx)
	}

	// Lookups that miss the empty cache read the database.
	if err := h.chain.FlushCachedState(blockchain.FlushRequired); err != nil {
		t.Fatalf("FlushCachedState: %v", err)
	}

	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}

	tests := []struct {
		name           string
		includeMempool bool
		want           []string
	}{
		{
			name: "confirmed",
			want: []string{
				wantUnspent(cb1, 1, true, 1),
				wantUnspent(tx1, 1, false, 2),
				wantUnspent(tx2, 0, false, 2),
				wantUnspent(tx2, 1, false, 2),
				wantUnspent(tx3, 1, false, 3),
			},
		},
		{
			name:           "with mempool",
			includeMempool: true,
			want: []string{
				wantUnspent(cb1, 1, true, 1),
				wantUnspent(tx2, 1, false, 2),
				wantUnspent(tx3, 1, false, 3),
				wantUnspent(m2, 0, false, mining.UnminedHeight),
				wantUnspent(m3, 0, false, mining.UnminedHeight),
				wantUnspent(m4, 0, false, mining.UnminedHeight),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cacheSize := h.chain.CachedStateSize()
			resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
				&pb.GetAddressUnspentOutputsRequest{
					Address:        addr.EncodeAddress(),
					IncludeMempool: test.includeMempool,
				})
			if err != nil {
				t.Fatalf("GetAddressUnspentOutputs: %v", err)
			}
			if got := h.chain.CachedStateSize(); got != cacheSize {
				t.Errorf("UTXO cache grew from %d to %d bytes", cacheSize,
					got)
			}
			got := make([]string, len(resp.Outputs))
			for i, u := range resp.Outputs {
				got[i] = unspentKey(u)
			}
			sort.Strings(got)
			want := append([]string(nil), test.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("got %d outputs, want %d:\n%v", len(got),
					len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("output %d:\ngot  %s\nwant %s", i, got[i],
						want[i])
				}
			}
		})
	}
}

// TestGetAddressUnspentOutputsOrder ensures confirmed outputs are returned
// newest block first.
func TestGetAddressUnspentOutputsOrder(t *testing.T) {
	h := newAddrUnspentHarness(t)
	addrScript := p2shScript(t, addrRedeem)

	var cbs []*wire.MsgTx
	for range 3 {
		cbs = append(cbs, h.mineBlock([]*wire.TxOut{
			wire.NewTxOut(50e8, addrScript, wire.TokenData{}),
		}, nil))
	}

	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}
	resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
		&pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()})
	if err != nil {
		t.Fatalf("GetAddressUnspentOutputs: %v", err)
	}
	if len(resp.Outputs) != len(cbs) {
		t.Fatalf("got %d outputs, want %d", len(resp.Outputs), len(cbs))
	}
	for i, u := range resp.Outputs {
		want := cbs[len(cbs)-1-i].TxHash()
		if !bytes.Equal(u.Outpoint.Hash, want[:]) {
			t.Errorf("output %d: got %x, want %v", i, u.Outpoint.Hash, want)
		}
	}
}

// TestGetAddressUnspentOutputsDuplicate ensures an output is returned once
// when the history of the address lists its transaction more than once, as
// for a coinbase repeated in a later block.  BFFastAdd skips the BIP30
// check, which allows the repeated coinbase as in two early mainnet blocks.
func TestGetAddressUnspentOutputsDuplicate(t *testing.T) {
	h := newAddrUnspentHarness(t)
	addrScript := p2shScript(t, addrRedeem)

	coinbase := h.coinbaseTx(1, []*wire.TxOut{
		wire.NewTxOut(50e8, addrScript, wire.TokenData{}),
	})
	h.mineBlockWithCoinbase(coinbase, nil)
	h.mineBlockWithCoinbase(coinbase, nil)

	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}
	txResp, err := h.s.GetAddressTransactions(context.Background(),
		&pb.GetAddressTransactionsRequest{Address: addr.EncodeAddress()})
	if err != nil {
		t.Fatalf("GetAddressTransactions: %v", err)
	}
	if len(txResp.ConfirmedTransactions) != 2 {
		t.Fatalf("got %d confirmed transactions, want 2",
			len(txResp.ConfirmedTransactions))
	}

	resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
		&pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()})
	if err != nil {
		t.Fatalf("GetAddressUnspentOutputs: %v", err)
	}
	want := coinbase.TxHash()
	if len(resp.Outputs) != 1 || !bytes.Equal(resp.Outputs[0].Outpoint.Hash,
		want[:]) {

		got := make([]string, len(resp.Outputs))
		for i, u := range resp.Outputs {
			var hash chainhash.Hash
			copy(hash[:], u.Outpoint.Hash)
			got[i] = fmt.Sprintf("%v:%d", hash, u.Outpoint.Index)
		}
		t.Fatalf("got %v, want only %v:0", got, want)
	}
}

// TestGetAddressUnspentOutputsConfirmedDuringScan ensures an output of a
// memory pool transaction that is confirmed while the address's history is
// being read is still returned, with or without the memory pool.
func TestGetAddressUnspentOutputsConfirmedDuringScan(t *testing.T) {
	for _, includeMempool := range []bool{true, false} {
		h := newAddrUnspentHarness(t)
		addrScript := p2shScript(t, addrRedeem)
		otherScript := p2shScript(t, otherRedeem)
		pay := func(value int64, script []byte) *wire.TxOut {
			return wire.NewTxOut(value, script, wire.TokenData{})
		}
		cb := h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, nil)
		funding := spendTx(t, outPoint(cb, 0), otherRedeem, pay(49e8, otherScript))
		h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
			[]*wire.MsgTx{funding})
		tx := spendTx(t, outPoint(funding, 0), otherRedeem,
			pay(48e8, addrScript))
		h.acceptTx(tx)

		// The transaction is confirmed once the history has been read,
		// leaving the memory pool as it does when its block connects.
		addressUnspentScanHook = func() {
			addressUnspentScanHook = nil
			h.pool.RemoveTransaction(bchutil.NewTx(tx), false)
			h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
				[]*wire.MsgTx{tx})
		}
		t.Cleanup(func() { addressUnspentScanHook = nil })

		addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
			&pb.GetAddressUnspentOutputsRequest{
				Address:        addr.EncodeAddress(),
				IncludeMempool: includeMempool,
			})
		if err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, u := range resp.Outputs {
			var hash chainhash.Hash
			copy(hash[:], u.Outpoint.Hash)
			found = found || hash == tx.TxHash()
		}
		if !found {
			t.Errorf("mempool %v: the output confirmed during the scan "+
				"was not returned", includeMempool)
		}
	}
}

// changeDuringCallTest is a transaction that spends an output of the address
// and pays it change while its unspent outputs are being read.  A pooled one
// is in the memory pool when the call starts, and is confirmed during it.
type changeDuringCallTest struct {
	name           string
	beforeScan     bool
	pooled         bool
	confirm        bool
	includeMempool bool
}

// runChangeDuringCallTest ensures the result holds the change of the
// transaction and leaves out the output it spends, as the memory pool or the
// chain has them once the call ends.  Without the memory pool, a transaction
// that only reached the memory pool changes nothing.
func runChangeDuringCallTest(t *testing.T, test changeDuringCallTest) {
	h := newAddrUnspentHarness(t)
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}
	cb := h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, nil)
	funding := spendTx(t, outPoint(cb, 0), otherRedeem, pay(49e8, addrScript))
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
		[]*wire.MsgTx{funding})
	spend := spendTx(t, outPoint(funding, 0), addrRedeem,
		pay(30e8, addrScript), pay(18e8, otherScript))

	if test.pooled {
		h.acceptTx(spend)
	}
	arrive := func() {
		if !test.pooled {
			h.acceptTx(spend)
		}
		if test.confirm {
			h.pool.RemoveTransaction(bchutil.NewTx(spend), false)
			h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
				[]*wire.MsgTx{spend})
		}
	}
	hook := &addressUnspentScanHook
	if test.beforeScan {
		hook = &addressUnspentListHook
	}
	*hook = func() {
		*hook = nil
		arrive()
	}
	t.Cleanup(func() { *hook = nil })

	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
		&pb.GetAddressUnspentOutputsRequest{
			Address:        addr.EncodeAddress(),
			IncludeMempool: test.includeMempool,
		})
	if err != nil {
		t.Fatal(err)
	}

	var want string
	switch {
	case test.confirm:
		want = wantUnspent(spend, 0, false, h.height)
	case test.includeMempool:
		want = wantUnspent(spend, 0, false, mining.UnminedHeight)
	default:
		want = wantUnspent(funding, 0, false, 2)
	}
	got := make([]string, len(resp.Outputs))
	for i, u := range resp.Outputs {
		got[i] = unspentKey(u)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %v\nwant [%s]", got, want)
	}
}

// TestGetAddressUnspentOutputsChangeDuringCall ensures a transaction that
// spends an output of the address and pays it change, entering the memory
// pool or a block while the call reads the address's history, is seen as the
// memory pool or the chain has it once the call ends.
func TestGetAddressUnspentOutputsChangeDuringCall(t *testing.T) {
	for _, test := range []changeDuringCallTest{
		{name: "mempool before scan", beforeScan: true, includeMempool: true},
		{name: "mempool before scan, no mempool", beforeScan: true},
		{name: "mempool after scan", includeMempool: true},
		{name: "mempool after scan, no mempool"},
		{name: "confirmed before scan", beforeScan: true, confirm: true,
			includeMempool: true},
		{name: "confirmed before scan, no mempool", beforeScan: true,
			confirm: true},
		{name: "confirmed after scan", confirm: true, includeMempool: true},
		{name: "confirmed after scan, no mempool", confirm: true},
		{name: "pooled, confirmed after scan", pooled: true, confirm: true,
			includeMempool: true},
		{name: "pooled, confirmed after scan, no mempool", pooled: true,
			confirm: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runChangeDuringCallTest(t, test)
		})
	}
}

// TestGetAddressTransactionsFeePerKb ensures an unconfirmed transaction's
// fee_per_kb is its fee per kilobyte, as in the subscriptions, not per byte.
func TestGetAddressTransactionsFeePerKb(t *testing.T) {
	h := newAddrUnspentHarness(t)
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}
	cb := h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, nil)
	funding := spendTx(t, outPoint(cb, 0), otherRedeem, pay(49e8, otherScript))
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, []*wire.MsgTx{funding})
	const fee = 100000
	tx := spendTx(t, outPoint(funding, 0), otherRedeem, pay(49e8-fee, addrScript))
	h.acceptTx(tx)

	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := h.s.GetAddressTransactions(context.Background(),
		&pb.GetAddressTransactionsRequest{Address: addr.EncodeAddress()})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.UnconfirmedTransactions) != 1 {
		t.Fatalf("%d unconfirmed transactions, want 1",
			len(resp.UnconfirmedTransactions))
	}
	want := int64(fee * 1000 / tx.SerializeSize())
	if got := resp.UnconfirmedTransactions[0].FeePerKb; got != want {
		t.Fatalf("fee_per_kb %d, want %d", got, want)
	}
}

// TestGetAddressUnspentOutputsCallEnds ensures GetAddressUnspentOutputs stops
// reading the address's history once its call's context ends, as it does when
// the client goes away or its deadline passes: before the scan, or, with a
// block connected meanwhile, before it reads that block's history.  It
// answers with the context's status.
func TestGetAddressUnspentOutputsCallEnds(t *testing.T) {
	tests := []struct {
		name      string
		afterScan bool
		deadline  bool
		want      codes.Code
	}{
		{"canceled before the scan", false, false, codes.Canceled},
		{"deadline passed", false, true, codes.DeadlineExceeded},
		{"canceled after the scan", true, false, codes.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newAddrUnspentHarness(t)
			addrScript := p2shScript(t, addrRedeem)
			pay := []*wire.TxOut{wire.NewTxOut(50e8, addrScript,
				wire.TokenData{})}
			h.mineBlock(pay, nil)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.deadline {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(ctx,
					time.Now().Add(-time.Second))
				defer stop()
			}
			hook := &addressUnspentListHook
			if test.afterScan {
				hook = &addressUnspentScanHook
			}
			*hook = func() {
				*hook = nil
				if test.afterScan {
					h.mineBlock(pay, nil)
				}
				cancel()
			}
			t.Cleanup(func() { *hook = nil })

			addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
			if err != nil {
				t.Fatal(err)
			}
			_, err = h.s.GetAddressUnspentOutputs(ctx,
				&pb.GetAddressUnspentOutputsRequest{
					Address: addr.EncodeAddress()})
			if status.Code(err) != test.want {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

// addrIndexReadsDB counts the bytes read from the address index's bucket
// through it.
type addrIndexReadsDB struct {
	database.DB
	bytes atomic.Int64
}

func (db *addrIndexReadsDB) View(fn func(database.Tx) error) error {
	return db.DB.View(func(tx database.Tx) error {
		return fn(addrIndexReadsTx{Tx: tx, db: db})
	})
}

type addrIndexReadsTx struct {
	database.Tx
	db *addrIndexReadsDB
}

func (tx addrIndexReadsTx) Metadata() database.Bucket {
	return addrIndexReadsBucket{dbBucket: tx.Tx.Metadata(), db: tx.db}
}

// dbBucket names the bucket addrIndexReadsBucket embeds, whose Bucket method
// would clash with a field named Bucket.
type dbBucket = database.Bucket

type addrIndexReadsBucket struct {
	dbBucket
	db      *addrIndexReadsDB
	counted bool
}

func (b addrIndexReadsBucket) Bucket(key []byte) database.Bucket {
	child := b.dbBucket.Bucket(key)
	if child == nil {
		return nil
	}
	return addrIndexReadsBucket{dbBucket: child, db: b.db,
		counted: string(key) == "txbyaddridx"}
}

func (b addrIndexReadsBucket) Get(key []byte) []byte {
	v := b.dbBucket.Get(key)
	if b.counted {
		b.db.bytes.Add(int64(len(v)))
	}
	return v
}

// TestGetAddressUnspentOutputsReadsHistoryOnce ensures GetAddressUnspentOutputs
// reads the address index entries of an address's history once, however many
// pages it loads its transactions in, where reading them again for each page
// costs time quadratic in the history's length, and that the outputs do not
// depend on the page size.
func TestGetAddressUnspentOutputsReadsHistoryOnce(t *testing.T) {
	var reads *addrIndexReadsDB
	h := newAddrUnspentHarnessDB(t, func(db database.DB) database.DB {
		reads = &addrIndexReadsDB{DB: db}
		return reads
	})
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}

	// A coinbase pays the address 30 outputs, 30 transactions spend them
	// and pay it change, and 15 spend every other change: 46 transactions
	// of the address and 15 unspent outputs.
	const n = 30
	outs := make([]*wire.TxOut, n)
	for i := range outs {
		outs[i] = pay(1e8, addrScript)
	}
	cb := h.mineBlock(outs, nil)
	change := make([]*wire.MsgTx, n)
	for i := range change {
		change[i] = spendTx(t, outPoint(cb, uint32(i)), addrRedeem,
			pay(9e7, addrScript), pay(1e6, otherScript))
	}
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, change)
	var spends []*wire.MsgTx
	for i := 0; i < n; i += 2 {
		spends = append(spends, spendTx(t, outPoint(change[i], 0),
			addrRedeem, pay(8e7, otherScript)))
	}
	h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, spends)
	const entries = 1 + n + n/2

	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatal(err)
	}
	saved := addressUnspentPageSize
	t.Cleanup(func() { addressUnspentPageSize = saved })
	for _, pageSize := range []int{10000, 1, 7, 10, entries, entries + 1} {
		addressUnspentPageSize = pageSize
		reads.bytes.Store(0)
		resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
			&pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()})
		if err != nil {
			t.Fatalf("pages of %d: %v", pageSize, err)
		}
		var got, want []string
		for _, u := range resp.Outputs {
			got = append(got, unspentKey(u))
		}
		for i := 1; i < n; i += 2 {
			want = append(want, wantUnspent(change[i], 0, false, 2))
		}
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("pages of %d: got %v, want %v", pageSize, got, want)
		}
		if b := reads.bytes.Load(); b != entries*12 {
			t.Errorf("pages of %d: read %d bytes of the address index, "+
				"want its %d", pageSize, b, entries*12)
		}
	}
}

// TestGetAddressUnspentOutputsShutdown ensures GetAddressUnspentOutputs stops
// reading the address's history once the server shuts down.
func TestGetAddressUnspentOutputsShutdown(t *testing.T) {
	h := newAddrUnspentHarness(t)
	h.mineBlock([]*wire.TxOut{wire.NewTxOut(50e8, p2shScript(t, addrRedeem),
		wire.TokenData{})}, nil)
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatal(err)
	}
	atomic.StoreInt32(&h.s.shutdown, 1)
	_, err = h.s.GetAddressUnspentOutputs(context.Background(),
		&pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()})
	if status.Code(err) != codes.Canceled {
		t.Errorf("got %v, want Canceled", err)
	}
}

// blockBeforeViewDB connects a block, once, when a read transaction is
// started through it, before the transaction starts.
type blockBeforeViewDB struct {
	database.DB
	connect func()
}

func (db *blockBeforeViewDB) View(fn func(database.Tx) error) error {
	if connect := db.connect; connect != nil {
		db.connect = nil
		connect()
	}
	return db.DB.View(fn)
}

// TestGetAddressTransactionsOneSnapshot ensures GetAddressTransactions reads
// an address's index entries in the database transaction it reads their
// transactions in, so that a block connected meanwhile is in both or in
// neither, where entries read in a transaction of their own could name a
// block the other could not read, and the call failed.
func TestGetAddressTransactionsOneSnapshot(t *testing.T) {
	var index *blockBeforeViewDB
	h := newAddrUnspentHarnessDB(t, func(db database.DB) database.DB {
		index = &blockBeforeViewDB{DB: db}
		return index
	})
	pay := []*wire.TxOut{wire.NewTxOut(50e8, p2shScript(t, addrRedeem),
		wire.TokenData{})}
	cb1 := h.mineBlock(pay, nil)
	index.connect = func() { h.mineBlock(pay, nil) }
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, h.params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}

	resp, err := h.s.GetAddressTransactions(context.Background(),
		&pb.GetAddressTransactionsRequest{Address: addr.EncodeAddress()})
	if err != nil {
		t.Fatalf("GetAddressTransactions: %v", err)
	}
	hash := cb1.TxHash()
	if len(resp.ConfirmedTransactions) != 1 ||
		!bytes.Equal(resp.ConfirmedTransactions[0].Hash, hash[:]) {

		t.Errorf("%d confirmed transactions, want cb1 only",
			len(resp.ConfirmedTransactions))
	}
	if index.connect == nil {
		t.Error("the address index read its entries in a database " +
			"transaction of its own")
	}
}

// viewCountdownDB runs hook once, before the views-th read transaction
// started through it.
type viewCountdownDB struct {
	database.DB
	views int
	hook  func()
}

func (db *viewCountdownDB) View(fn func(database.Tx) error) error {
	if db.views > 0 {
		db.views--
		if db.views == 0 {
			db.hook()
		}
	}
	return db.DB.View(fn)
}

// TestGetAddressUnspentOutputsReorgDuringCall ensures a GetAddressUnspentOutputs
// call during which the chain reorganizes below the blocks it read fails with
// Aborted, so that the client calls again, and that the next call gives the
// outputs of the new chain: when blocks of the address's history are replaced
// between two of its pages, whose entries would name the blocks that took
// over their IDs, and when blocks above the history are replaced after it
// was read, which the call reads again only from above the height it
// started at.
func TestGetAddressUnspentOutputsReorgDuringCall(t *testing.T) {
	saved := addressUnspentPageSize
	t.Cleanup(func() { addressUnspentPageSize = saved })
	addressUnspentPageSize = 1
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) []*wire.TxOut {
		return []*wire.TxOut{wire.NewTxOut(value, script, wire.TokenData{})}
	}
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}

	for _, test := range []struct {
		name string
		// The scripts paid by heights 1 to 3, then the height whose
		// block is replaced, before which read of the address index, and
		// the scripts paid by its replacement and those after it.
		paid     [3][]byte
		replace  int32
		view     int
		replaced [][]byte
		want     int // outputs of the new chain
	}{
		{"history", [3][]byte{addrScript, addrScript, addrScript}, 2, 2,
			[][]byte{otherScript, otherScript, otherScript}, 1},
		{"above the history", [3][]byte{addrScript, otherScript, otherScript}, 3, 2,
			[][]byte{addrScript}, 2},
	} {
		var index *viewCountdownDB
		h := newAddrUnspentHarnessDB(t, func(db database.DB) database.DB {
			index = &viewCountdownDB{DB: db}
			return index
		})
		var hashes []chainhash.Hash
		for i, script := range test.paid {
			h.mineBlock(pay(int64(i+1)*1e8, script), nil)
			hashes = append(hashes, h.prevHash)
		}
		index.views = test.view
		index.hook = func() {
			if err := h.chain.InvalidateBlock(&hashes[test.replace-1]); err != nil {
				t.Fatalf("InvalidateBlock: %v", err)
			}
			h.height, h.prevHash = test.replace-1, hashes[test.replace-2]
			for i, script := range test.replaced {
				h.mineBlock(pay(int64(i+10)*1e8, script), nil)
			}
		}

		req := &pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()}
		_, err := h.s.GetAddressUnspentOutputs(context.Background(), req)
		if status.Code(err) != codes.Aborted {
			t.Errorf("%s: got %v, want Aborted", test.name, err)
		}
		resp, err := h.s.GetAddressUnspentOutputs(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: the next call: %v", test.name, err)
		}
		if len(resp.Outputs) != test.want {
			t.Errorf("%s: the next call gave %d outputs, want %d", test.name,
				len(resp.Outputs), test.want)
		}
	}
}

// TestGetAddressUnspentOutputsReorgAboveStart ensures a GetAddressUnspentOutputs
// call fails with Aborted when a block connected during it is replaced, though
// the block it started at stays on the main chain: the replaced block spent an
// output of the address that a later page of the history holds, so the call
// read the output as spent, and its replacements do not spend it.
func TestGetAddressUnspentOutputsReorgAboveStart(t *testing.T) {
	saved := addressUnspentPageSize
	t.Cleanup(func() { addressUnspentPageSize = saved })
	addressUnspentPageSize = 1
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}

	var index *viewCountdownDB
	h := newAddrUnspentHarnessDB(t, func(db database.DB) database.DB {
		index = &viewCountdownDB{DB: db}
		return index
	})
	var paid []*wire.MsgTx
	for i := int64(1); i <= 3; i++ {
		paid = append(paid, h.mineBlock([]*wire.TxOut{pay(i*1e8, addrScript)}, nil))
	}
	startHash := h.prevHash

	// Once the first page, the output of block 3, has been read, block 4
	// spends the output of block 1, which the last page holds.
	var spending chainhash.Hash
	index.views = 2
	index.hook = func() {
		spend := spendTx(t, outPoint(paid[0], 0), addrRedeem,
			pay(1e8-1000, otherScript))
		h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
			[]*wire.MsgTx{spend})
		spending = h.prevHash
	}

	// Once the history has been read, two blocks that spend nothing of the
	// address replace block 4.
	addressUnspentScanHook = func() {
		addressUnspentScanHook = nil
		if err := h.chain.InvalidateBlock(&spending); err != nil {
			t.Fatalf("InvalidateBlock: %v", err)
		}
		h.height, h.prevHash = 3, startHash
		h.mineBlock([]*wire.TxOut{pay(49e8, otherScript)}, nil)
		h.mineBlock([]*wire.TxOut{pay(49e8, otherScript)}, nil)
	}
	t.Cleanup(func() { addressUnspentScanHook = nil })

	req := &pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()}
	resp, err := h.s.GetAddressUnspentOutputs(context.Background(), req)
	if status.Code(err) != codes.Aborted {
		t.Errorf("got %d outputs and %v, want Aborted", len(resp.GetOutputs()),
			err)
	}
	resp, err = h.s.GetAddressUnspentOutputs(context.Background(), req)
	if err != nil {
		t.Fatalf("the next call: %v", err)
	}
	if len(resp.Outputs) != 3 {
		t.Errorf("the next call gave %d outputs, want 3", len(resp.Outputs))
	}
}

// regionsHookDB runs hook once, before the regions-th read of block regions in
// the read transactions started through it, and fails that read when hook is
// nil.
type regionsHookDB struct {
	database.DB
	regions int
	hook    func()
}

func (db *regionsHookDB) View(fn func(database.Tx) error) error {
	return db.DB.View(func(dbTx database.Tx) error {
		return fn(regionsHookTx{Tx: dbTx, db: db})
	})
}

type regionsHookTx struct {
	database.Tx
	db *regionsHookDB
}

func (tx regionsHookTx) FetchBlockRegions(regions []database.BlockRegion) ([][]byte, error) {
	if tx.db.regions > 0 {
		tx.db.regions--
		if tx.db.regions == 0 {
			if tx.db.hook == nil {
				return nil, errors.New("block regions not read")
			}
			tx.db.hook()
		}
	}
	return tx.Tx.FetchBlockRegions(regions)
}

// TestGetAddressUnspentOutputsReorgFailsRead ensures a GetAddressUnspentOutputs
// call fails with Aborted, not with the error of its read, when a block whose
// transactions it is about to read leaves the main chain: a block of the
// history's first, middle or last page, the middle one after its page passed
// the check of the history's newest block, or a block connected during the
// call, which the call reads once it has read the history.  A read that fails
// with the chain unchanged keeps its error.
func TestGetAddressUnspentOutputsReorgFailsRead(t *testing.T) {
	saved := addressUnspentPageSize
	t.Cleanup(func() { addressUnspentPageSize = saved })
	addressUnspentPageSize = 1
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) []*wire.TxOut {
		return []*wire.TxOut{wire.NewTxOut(value, script, wire.TokenData{})}
	}
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}

	for _, test := range []struct {
		name string
		// The block left the main chain, before which read of block
		// regions, counted from the call's start or, for a block
		// connected during the call, from once the history was read.
		height    int32
		regions   int
		connected bool
		want      codes.Code
	}{
		{"first page", 4, 1, false, codes.Aborted},
		{"middle page", 3, 2, false, codes.Aborted},
		{"last page", 2, 3, false, codes.Aborted},
		{"connected during the call", 5, 1, true, codes.Aborted},
		{"no reorg", 0, 2, false, codes.InvalidArgument},
	} {
		h := newAddrUnspentHarness(t)
		db := &regionsHookDB{DB: h.s.db}
		h.s.db = db
		h.mineBlock(pay(1e8, otherScript), nil)
		for i := int64(2); i <= 4; i++ {
			h.mineBlock(pay(i*1e8, addrScript), nil)
		}

		// The block is replaced as its transactions are about to be read.
		replace := func() {
			hash, err := h.chain.BlockHashByHeight(test.height)
			if err != nil {
				t.Fatalf("%s: BlockHashByHeight: %v", test.name, err)
			}
			if err := h.chain.InvalidateBlock(hash); err != nil {
				t.Fatalf("%s: InvalidateBlock: %v", test.name, err)
			}
			h.height = test.height - 1
			h.prevHash = h.chain.BestSnapshot().Hash
			h.mineBlock(pay(60e8, otherScript), nil)
		}
		if test.height == 0 {
			replace = nil
		}
		if test.connected {
			addressUnspentScanHook = func() {
				addressUnspentScanHook = nil
				h.mineBlock(pay(5e8, addrScript), nil)
				db.regions, db.hook = test.regions, replace
			}
			t.Cleanup(func() { addressUnspentScanHook = nil })
		} else {
			db.regions, db.hook = test.regions, replace
		}

		req := &pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()}
		_, err := h.s.GetAddressUnspentOutputs(context.Background(), req)
		if status.Code(err) != test.want {
			t.Errorf("%s: got %v, want %v", test.name, err, test.want)
		}
		if db.regions != 0 {
			t.Errorf("%s: the read was not reached", test.name)
		}
	}
}

// disconnectHookManager is an index manager that runs hook, when set, before
// the indexes hear of a block disconnection, which is under way then.
type disconnectHookManager struct {
	blockchain.IndexManager
	hook func()
}

func (m *disconnectHookManager) DisconnectBlock(dbTx database.Tx,
	block *bchutil.Block, stxos []blockchain.SpentTxOut) error {

	if m.hook != nil {
		m.hook()
	}
	return m.IndexManager.DisconnectBlock(dbTx, block, stxos)
}

// TestGetAddressUnspentOutputsDisconnectUnderWay ensures a GetAddressUnspentOutputs
// call that starts while the block it starts at is being disconnected fails
// with Aborted, though the count of disconnections already holds that
// disconnection when the call reads it: the block is not in the address's
// history, so no page fails, and the call would miss an output of the block
// replacing it, at the height it starts at, which it does not read again.
func TestGetAddressUnspentOutputsDisconnectUnderWay(t *testing.T) {
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) []*wire.TxOut {
		return []*wire.TxOut{wire.NewTxOut(value, script, wire.TokenData{})}
	}
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}
	manager := &disconnectHookManager{}
	h := newAddrUnspentHarnessWith(t, nil,
		func(m blockchain.IndexManager) blockchain.IndexManager {
			manager.IndexManager = m
			return manager
		})
	h.mineBlock(pay(1e8, addrScript), nil)
	h.mineBlock(pay(2e8, addrScript), nil)
	h.mineBlock(pay(3e8, otherScript), nil)

	// The call starts once the disconnection of block 3 has been counted,
	// and has listed the memory pool, so read its tip, before the
	// disconnection goes on.  Once it has read the history, a block paying
	// the address replaces block 3.
	type result struct {
		resp *pb.GetAddressUnspentOutputsResponse
		err  error
	}
	results := make(chan result, 1)
	listed := make(chan struct{})
	addressUnspentListHook = func() {
		addressUnspentListHook = nil
		close(listed)
	}
	addressUnspentScanHook = func() {
		addressUnspentScanHook = nil
		best := h.chain.BestSnapshot()
		h.height, h.prevHash = best.Height, best.Hash
		h.mineBlock(pay(4e8, addrScript), nil)
	}
	t.Cleanup(func() {
		addressUnspentListHook = nil
		addressUnspentScanHook = nil
	})
	manager.hook = func() {
		manager.hook = nil
		go func() {
			resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
				&pb.GetAddressUnspentOutputsRequest{Address: addr.EncodeAddress()})
			results <- result{resp, err}
		}()
		<-listed
	}
	tip := h.prevHash
	if err := h.chain.InvalidateBlock(&tip); err != nil {
		t.Fatalf("InvalidateBlock: %v", err)
	}
	select {
	case r := <-results:
		if status.Code(r.err) != codes.Aborted {
			t.Errorf("got %d outputs and %v, want Aborted",
				len(r.resp.GetOutputs()), r.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the call did not end")
	}
}

// TestGetAddressUnspentOutputsConfirmedAfterReread ensures a GetAddressUnspentOutputs
// call does not return an output together with the change of the memory pool
// transaction spending it, when that transaction is confirmed after the call
// has read the blocks connected during it: its change is found in the utxo
// set, so the output it spends is left out, with or without the memory pool.
func TestGetAddressUnspentOutputsConfirmedAfterReread(t *testing.T) {
	addrScript := p2shScript(t, addrRedeem)
	otherScript := p2shScript(t, otherRedeem)
	pay := func(value int64, script []byte) *wire.TxOut {
		return wire.NewTxOut(value, script, wire.TokenData{})
	}
	addr, err := bchutil.NewAddressScriptHash(addrRedeem, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("NewAddressScriptHash: %v", err)
	}

	for _, includeMempool := range []bool{false, true} {
		h := newAddrUnspentHarness(t)
		db := &regionsHookDB{DB: h.s.db}
		h.s.db = db
		cb := h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)}, nil)
		funding := spendTx(t, outPoint(cb, 0), otherRedeem, pay(49e8, addrScript))
		h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
			[]*wire.MsgTx{funding})
		spend := spendTx(t, outPoint(funding, 0), addrRedeem,
			pay(30e8, addrScript), pay(18e8, otherScript))
		h.acceptTx(spend)

		// Once the history has been read, block 3 pays the address, so the
		// call reads it, and as it reads the block's transactions, block 4
		// confirms the spend.
		var paid *wire.MsgTx
		addressUnspentScanHook = func() {
			addressUnspentScanHook = nil
			paid = h.mineBlock([]*wire.TxOut{pay(50e8, addrScript)}, nil)
			db.regions, db.hook = 1, func() {
				h.pool.RemoveTransaction(bchutil.NewTx(spend), false)
				h.mineBlock([]*wire.TxOut{pay(50e8, otherScript)},
					[]*wire.MsgTx{spend})
			}
		}
		t.Cleanup(func() { addressUnspentScanHook = nil })

		resp, err := h.s.GetAddressUnspentOutputs(context.Background(),
			&pb.GetAddressUnspentOutputsRequest{
				Address:        addr.EncodeAddress(),
				IncludeMempool: includeMempool,
			})
		if err != nil {
			t.Fatalf("mempool %v: %v", includeMempool, err)
		}
		if db.regions != 0 {
			t.Fatalf("mempool %v: the spend was not confirmed during the call",
				includeMempool)
		}
		got := make([]string, len(resp.Outputs))
		for i, u := range resp.Outputs {
			got[i] = unspentKey(u)
		}
		sort.Strings(got)
		want := []string{wantUnspent(paid, 0, true, 3),
			wantUnspent(spend, 0, false, 4)}
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("mempool %v: got %v\nwant %v", includeMempool, got, want)
		}
	}
}
