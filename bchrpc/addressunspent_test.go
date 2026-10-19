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
			DisableRelayPriority: true,
			AcceptNonStd:         true,
			MaxTxVersion:         2,
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
		tb:    tb,
		chain: chain,
		pool:  pool,
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

	_, _, err := h.pool.MaybeAcceptTransaction(bchutil.NewTx(tx), true, false)
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
