// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// newBlockStatsTestServer returns an RPC server of a regtest chain whose tip,
// at height 103, holds two transactions spending the coinbases of the first
// three blocks: one of 95 bytes paying a fee of 1000 satoshis to an OP_TRUE
// output and a pay-to-pubkey-hash output, and one of 102 bytes paying a fee
// of 3000 from two inputs to an OP_TRUE output.  It returns the tip too.
func newBlockStatsTestServer(t *testing.T) (*rpcServer, *bchutil.Block) {
	t.Helper()
	quietRPCLog(t)
	chain, db := newRPCTestChain(t)
	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Chain:       chain,
		DB:          db,
	}}

	var coinbases []*bchutil.Tx
	for range 3 {
		block := mineTestBlock(t, chain, nil)
		coinbases = append(coinbases, block.Transactions()[0])
	}
	for range chaincfg.RegressionNetParams.CoinbaseMaturity - 1 {
		mineTestBlock(t, chain, nil)
	}

	opTrue := []byte{txscript.OP_TRUE}
	p2pkh := mustDecodeHex(t, "76a914123456789012345678901234567890123456789088ac")
	tx1 := wire.NewMsgTx(1)
	tx1.AddTxIn(wire.NewTxIn(wire.NewOutPoint(coinbases[0].Hash(), 0), nil))
	tx1.AddTxOut(wire.NewTxOut(2000000000, opTrue, wire.TokenData{}))
	tx1.AddTxOut(wire.NewTxOut(2999999000, p2pkh, wire.TokenData{}))
	tx2 := wire.NewMsgTx(1)
	tx2.AddTxIn(wire.NewTxIn(wire.NewOutPoint(coinbases[1].Hash(), 0), nil))
	tx2.AddTxIn(wire.NewTxIn(wire.NewOutPoint(coinbases[2].Hash(), 0), nil))
	tx2.AddTxOut(wire.NewTxOut(9999997000, opTrue, wire.TokenData{}))
	if tx1.SerializeSize() != 95 || tx2.SerializeSize() != 102 {
		t.Fatalf("transaction sizes %d and %d, want 95 and 102",
			tx1.SerializeSize(), tx2.SerializeSize())
	}

	// The transactions after the coinbase are in txid order.
	txns := []*wire.MsgTx{tx1, tx2}
	slices.SortFunc(txns, func(a, b *wire.MsgTx) int {
		hashA, hashB := a.TxHash(), b.TxHash()
		return hashA.Compare(&hashB)
	})
	tip := mineTestBlock(t, chain, nil, txns...)
	if tip.Height() != 103 {
		t.Fatalf("tip at height %d, want 103", tip.Height())
	}
	return s, tip
}

// blockStats runs getblockstats on a hash or height with the stats.
func blockStats(s *rpcServer, hashOrHeight interface{}, stats ...string) (interface{}, error) {
	var statsArg *[]string
	if stats != nil {
		statsArg = &stats
	}
	cmd := btcjson.NewGetBlockStatsCmd(
		btcjson.HashOrHeight{Value: hashOrHeight}, statsArg)
	return handleGetBlockStats(s, cmd, nil)
}

// blockStatsMedianTime returns the median of the timestamps of the 11 blocks
// up to the block at the height, worked out from their headers.
func blockStatsMedianTime(t *testing.T, s *rpcServer, height int32) int64 {
	t.Helper()
	var times []int64
	for h := max(height-10, 0); h <= height; h++ {
		header, err := s.cfg.Chain.HeaderByHeight(h)
		if err != nil {
			t.Fatal(err)
		}
		times = append(times, header.Timestamp.Unix())
	}
	slices.Sort(times)
	return times[len(times)/2]
}

// TestGetBlockStatsRPC ensures getblockstats gives the statistics Bitcoin Cash
// Node gives, worked out by hand, for a block with transactions, by its hash
// and by its height, with the fees from its spend journal: whole satoshis, fee
// rates rounded down to satoshis per byte, truncated medians, and fee rate
// percentiles of the size of the transactions.
func TestGetBlockStatsRPC(t *testing.T) {
	s, tip := newBlockStatsTestServer(t)
	toBCH := func(satoshis int64) float64 {
		return bchutil.Amount(satoshis).ToBCH()
	}
	header := tip.MsgBlock().Header

	// The fee rates are 1000/95 = 10 and 3000/102 = 29 satoshis per byte.
	// Of the 197 bytes, the first 95 are at 10, which reach 19.7 and
	// 49.25, and the rest at 29.  The average is 4000/197 = 20.
	//
	// Each output adds its size and 41 to utxo_size_inc: 10 for OP_TRUE
	// outputs, of the coinbase, the first transaction and the second, 34
	// for the pay-to-pubkey-hash output, and each spent coinbase output,
	// of OP_TRUE too, takes 10 off: 4*51 + 24 - 3*51 = 75.
	want := &btcjson.GetBlockStatsResult{
		AvgFee:     toBCH(2000),
		AvgFeeRate: toBCH(20),
		AvgTxSize:  98,
		BlockHash:  tip.Hash().String(),
		FeeRatePercentiles: []float64{toBCH(10), toBCH(10), toBCH(29),
			toBCH(29), toBCH(29)},
		Height:           103,
		Ins:              3,
		MaxFee:           toBCH(3000),
		MaxFeeRate:       toBCH(29),
		MaxTxSize:        102,
		MedianFee:        toBCH(2000),
		MedianTime:       blockStatsMedianTime(t, s, 103),
		MedianTxSize:     98,
		MinFee:           toBCH(1000),
		MinFeeRate:       toBCH(10),
		MinTxSize:        95,
		Outs:             4,
		Subsidy:          50,
		Time:             header.Timestamp.Unix(),
		TotalOut:         toBCH(2000000000 + 2999999000 + 9999997000),
		TotalSize:        197,
		TotalFee:         toBCH(4000),
		Txs:              3,
		UTXOIncrease:     1,
		UTXOSizeIncrease: 75,
	}
	for _, hashOrHeight := range []interface{}{tip.Hash().String(), 103} {
		got, err := blockStats(s, hashOrHeight)
		if err != nil {
			t.Fatalf("%v: %v", hashOrHeight, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %+v, want %+v", hashOrHeight, got, want)
		}
	}

	// No statistics selected is the same as none given.
	got, err := blockStats(s, 103, []string{}...)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("empty stats: got %+v, %v", got, err)
	}
}

// TestGetBlockStatsCoinbaseOnly ensures the statistics of blocks with only a
// coinbase are zero where there are no transactions to take them from, and
// count the coinbase's outputs, as in Bitcoin Cash Node, for the genesis block
// too, which has no spend journal.
func TestGetBlockStatsCoinbaseOnly(t *testing.T) {
	s, _ := newBlockStatsTestServer(t)
	genesis := chaincfg.RegressionNetParams.GenesisBlock
	tests := []struct {
		height      int32
		outputSize  int64
		description string
	}{
		// The genesis output pays to a 65-byte public key.
		{0, 8 + 1 + 67, "genesis"},
		{1, 8 + 1 + 1, "block 1"},
	}
	for _, test := range tests {
		got, err := blockStats(s, int(test.height))
		if err != nil {
			t.Fatalf("%s: %v", test.description, err)
		}
		header, err := s.cfg.Chain.HeaderByHeight(test.height)
		if err != nil {
			t.Fatal(err)
		}
		hash := header.BlockHash()
		if test.height == 0 && hash != genesis.BlockHash() {
			t.Fatalf("genesis hash %v", hash)
		}
		want := &btcjson.GetBlockStatsResult{
			BlockHash:          hash.String(),
			FeeRatePercentiles: []float64{0, 0, 0, 0, 0},
			Height:             int64(test.height),
			MedianTime:         blockStatsMedianTime(t, s, test.height),
			Outs:               1,
			Subsidy:            50,
			Time:               header.Timestamp.Unix(),
			Txs:                1,
			UTXOIncrease:       1,
			UTXOSizeIncrease:   test.outputSize + 41,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, want %+v", test.description, got,
				want)
		}
	}
}

// TestGetBlockStatsSelected ensures selected statistics are returned alone,
// sorted and once each, as Bitcoin Cash Node returns them, and that a name
// that isn't a statistic is refused with Bitcoin Cash Node's error.
func TestGetBlockStatsSelected(t *testing.T) {
	s, _ := newBlockStatsTestServer(t)
	tests := []struct {
		stats []string
		want  string
	}{
		{[]string{"minfeerate", "avgfeerate"}, `{"avgfeerate":2e-7,"minfeerate":1e-7}`},
		{[]string{"txs", "height", "txs"}, `{"height":103,"txs":3}`},
		{[]string{"feerate_percentiles", "utxo_size_inc"},
			`{"feerate_percentiles":[1e-7,1e-7,2.9e-7,2.9e-7,2.9e-7],"utxo_size_inc":75}`},
	}
	for _, test := range tests {
		got, err := blockStats(s, 103, test.stats...)
		if err != nil {
			t.Fatalf("%v: %v", test.stats, err)
		}
		marshalled, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if string(marshalled) != test.want {
			t.Errorf("%v: got %s, want %s", test.stats, marshalled,
				test.want)
		}
	}

	// The first invalid name in sorted order is reported.
	_, err := blockStats(s, 103, "height", "zzz", "nonsense")
	assertRPCError(t, "invalid statistic", err,
		btcjson.ErrRPCInvalidParameter, "Invalid selected statistic nonsense")
}

// TestGetBlockStatsSpendJournal ensures statistics that need the outputs a
// block spends fail with Bitcoin Cash Node's error when the spend journal of
// the block can't be read, and that the others are still given, as Bitcoin
// Cash Node reads its undo data only when it needs it.
func TestGetBlockStatsSpendJournal(t *testing.T) {
	s, tip := newBlockStatsTestServer(t)
	err := s.cfg.DB.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket([]byte("spendjournal"))
		return bucket.Delete(tip.Hash()[:])
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.cfg.Chain.FetchSpendJournal(tip); err == nil {
		t.Fatal("the spend journal can still be read")
	}

	for _, stat := range blockStatsSpentStats {
		_, err := blockStats(s, 103, "height", stat)
		assertRPCError(t, stat, err, btcjson.ErrRPCMisc,
			"Can't read undo data from disk")
	}
	_, err = blockStats(s, 103)
	assertRPCError(t, "all statistics", err, btcjson.ErrRPCMisc,
		"Can't read undo data from disk")

	got, err := blockStats(s, 103, "ins", "outs", "total_out", "total_size",
		"mintxsize", "mediantxsize", "txs", "utxo_increase")
	if err != nil {
		t.Fatal(err)
	}
	marshalled, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"ins":3,"mediantxsize":98,"mintxsize":95,"outs":4,` +
		`"total_out":149.99996,"total_size":197,"txs":3,"utxo_increase":1}`
	if string(marshalled) != want {
		t.Errorf("got %s, want %s", marshalled, want)
	}
}

// TestGetBlockStatsErrors ensures getblockstats refuses heights, hashes and
// blocks with the errors Bitcoin Cash Node gives.
func TestGetBlockStatsErrors(t *testing.T) {
	s, tip := newBlockStatsTestServer(t)

	// A block on a side chain, next to the tip.
	parent := tip.MsgBlock().Header.PrevBlock
	side := mineTestBlock(t, s.cfg.Chain, &parent)
	if s.cfg.Chain.MainChainHasBlock(side.Hash()) {
		t.Fatal("the side block is in the main chain")
	}

	unknown := strings.Repeat("0", 63) + "1"
	notHex := strings.Repeat("0", 63) + "g"
	tests := []struct {
		hashOrHeight interface{}
		code         btcjson.RPCErrorCode
		message      string
	}{
		{-1, btcjson.ErrRPCInvalidParameter, "Target block height -1 is negative"},
		{104, btcjson.ErrRPCInvalidParameter,
			"Target block height 104 after current tip 103"},
		{"abcd", btcjson.ErrRPCInvalidParameter,
			"hash_or_height must be of length 64 (not 4, for 'abcd')"},
		{"103", btcjson.ErrRPCInvalidParameter,
			"hash_or_height must be of length 64 (not 3, for '103')"},
		{notHex, btcjson.ErrRPCInvalidParameter,
			"hash_or_height must be hexadecimal string (not '" + notHex + "')"},
		{unknown, btcjson.ErrRPCInvalidAddressOrKey, "Block not found"},
		{side.Hash().String(), btcjson.ErrRPCInvalidParameter,
			"Block is not in chain regtest"},
		{nil, btcjson.ErrRPCInvalidParameter,
			"hash_or_height must be a block hash or height"},
	}
	for _, test := range tests {
		_, err := blockStats(s, test.hashOrHeight)
		assertRPCError(t, "getblockstats", err, test.code, test.message)
	}
}

// TestSpentOutputSize ensures the size of an output taken from a spend journal
// entry, whose script the chain stores after the output's token data, is the
// output's serialized size, token data included.
func TestSpentOutputSize(t *testing.T) {
	for _, test := range tokenDataTests {
		txOut := decodeTokenTestTxOut(t, test.prefix)
		pkScript := txOut.PkScript
		if !txOut.TokenData.IsEmpty() {
			buf := txOut.TokenData.TokenDataBuffer()
			buf.Write(pkScript)
			pkScript = buf.Bytes()
		}
		stxo := blockchain.SpentTxOut{Amount: txOut.Value, PkScript: pkScript}
		got, want := spentOutputSize(&stxo), txOut.SerializeSize()
		if got != want || test.prefix == "" && got != 8+1+25 {
			t.Errorf("%s: got %d, want %d", test.name, got, want)
		}
	}
}
