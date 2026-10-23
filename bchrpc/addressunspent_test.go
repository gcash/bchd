// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"fmt"
	"sort"
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

	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", tb.TempDir(), params.Net)
	if err != nil {
		tb.Fatalf("database.Create: %v", err)
	}
	tb.Cleanup(func() { db.Close() })

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
