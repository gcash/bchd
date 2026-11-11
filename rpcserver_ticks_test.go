// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/mining/cpuminer"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// tickRegime is a regtest chain setup for the tick fields: either upgrade
// 2027 never activates, or the block at height 1 is its activation block, so
// block 2 is the first 60-tick block.
type tickRegime struct {
	name       string
	activation *uint64

	// The expected ticks, target time and next target time of the blocks
	// at heights 0 to 3.
	ticks, targetTime, nextTargetTime [4]int64
}

// tickRegimes returns the never active regime and the regime with the seam
// between heights 1 and 2.  Mined blocks are timestamped now, so an
// activation time just after the genesis block makes block 1 the first block
// with a median time past at or after it.
func tickRegimes() []tickRegime {
	activation := uint64(
		chaincfg.RegressionNetParams.GenesisBlock.Header.Timestamp.Unix() + 1)
	return []tickRegime{{
		name:           "never active",
		ticks:          [4]int64{0, 600, 1200, 1800},
		targetTime:     [4]int64{600, 600, 600, 600},
		nextTargetTime: [4]int64{600, 600, 600, 600},
	}, {
		name:           "active after block 1",
		activation:     &activation,
		ticks:          [4]int64{0, 600, 660, 720},
		targetTime:     [4]int64{600, 600, 60, 60},
		nextTargetTime: [4]int64{600, 60, 60, 60},
	}}
}

// tickSyncMgr is an rpcserverSyncManager for getblockchaininfo.
type tickSyncMgr struct {
	rpcserverSyncManager
}

func (tickSyncMgr) SyncHeight() uint64 { return 0 }

// newTickTestServer returns an RPC server on a fresh regtest chain of the
// regime with a transaction index and a memory pool.
func newTickTestServer(t *testing.T, regime tickRegime) *rpcServer {
	t.Helper()
	db := newRPCTestDB(t)
	blockchain.UseLogger(bchlog.Disabled)
	mining.UseLogger(bchlog.Disabled)
	t.Cleanup(func() {
		blockchain.UseLogger(chanLog)
		mining.UseLogger(minrLog)
	})
	params := &chaincfg.RegressionNetParams
	txIndex := indexers.NewTxIndex(db)
	chain, err := blockchain.New(&blockchain.Config{
		DB:                        db,
		ChainParams:               params,
		TimeSource:                blockchain.NewMedianTime(),
		UtxoCacheMaxSize:          1 << 20,
		ExcessiveBlockSize:        32000000,
		IndexManager:              indexers.NewManager(db, []indexers.Indexer{txIndex}),
		Upgrade2027ActivationTime: regime.activation,
	})
	if err != nil {
		t.Fatal(err)
	}
	timeSource := blockchain.NewMedianTime()
	gen := mining.NewBlkTmplGenerator(&mining.Policy{BlockMaxSize: 1000000},
		params, noTxSource{}, chain, timeSource, txscript.NewSigCache(10),
		txscript.NewHashCache(10))
	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			MaxTxVersion:    2,
			AcceptNonStd:    true,
			MinRelayTxFee:   mempool.DefaultMinRelayTxFee,
			MaxOrphanTxs:    5,
			MaxOrphanTxSize: 1000,
		},
		ChainParams:   params,
		FetchUtxoView: chain.FetchUtxoView,
		BestSnapshot:  chain.BestSnapshot,
		CalcSequenceLock: func(tx *bchutil.Tx, view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return chain.CalcSequenceLock(tx, view, true)
		},
		IsDeploymentActive: chain.IsDeploymentActive,
	})
	return &rpcServer{
		gbtWorkState: newGbtWorkState(timeSource),
		cfg: rpcserverConfig{
			ChainParams: params,
			Chain:       chain,
			DB:          db,
			TxIndex:     txIndex,
			TxMemPool:   pool,
			SyncMgr:     tickSyncMgr{},
			Generator:   gen,
			CPUMiner:    cpuminer.New(&cpuminer.Config{ChainParams: params}),
		},
	}
}

// TestBlockTickFields ensures getblock and getblockheader report the ticks,
// target times and tick confirmations of each block on both sides of the
// upgrade 2027 seam.
func TestBlockTickFields(t *testing.T) {
	for _, regime := range tickRegimes() {
		s := newTickTestServer(t, regime)
		chain := s.cfg.Chain
		for range 3 {
			mineTestBlock(t, chain, nil)
		}
		const tip = 3
		for height := range int32(4) {
			hash, err := chain.BlockHashByHeight(height)
			if err != nil {
				t.Fatal(err)
			}
			wantConf := regime.ticks[tip] - regime.ticks[height] +
				regime.targetTime[height]

			res, err := handleGetBlockHeader(s,
				btcjson.NewGetBlockHeaderCmd(hash.String(), nil), nil)
			if err != nil {
				t.Fatalf("%s: getblockheader %d: %v", regime.name,
					height, err)
			}
			header := res.(btcjson.GetBlockHeaderVerboseResult)
			got := [4]int64{header.Ticks, header.TargetTime,
				header.NextTargetTime, header.ConfirmationsTicks}
			want := [4]int64{regime.ticks[height],
				regime.targetTime[height],
				regime.nextTargetTime[height], wantConf}
			if got != want {
				t.Errorf("%s: getblockheader %d: ticks, target_time, "+
					"next_target_time, confirmations_ticks %v, "+
					"want %v", regime.name, height, got, want)
			}

			for _, verbosity := range []btcjson.VerbosityLevel{1, 3} {
				res, err = handleGetBlock(s, btcjson.NewGetBlockCmd(
					hash.String(), &verbosity), nil)
				if err != nil {
					t.Fatalf("%s: getblock %d: %v", regime.name,
						height, err)
				}
				switch block := res.(type) {
				case btcjson.GetBlockVerboseResult:
					got = [4]int64{block.Ticks, block.TargetTime,
						block.NextTargetTime,
						block.ConfirmationsTicks}
				case btcjson.GetBlockVerboseTxResult:
					got = [4]int64{block.Ticks, block.TargetTime,
						block.NextTargetTime,
						block.ConfirmationsTicks}
				}
				if got != want {
					t.Errorf("%s: getblock %d verbosity %d: %v, "+
						"want %v", regime.name, height,
						verbosity, got, want)
				}
			}
		}
	}
}

// TestChainInfoTickFields ensures getblockchaininfo, getmininginfo and
// getblocktemplate report the target times of the tip and the next block,
// and getblockchaininfo the ticks and the upgrade status, with the tip
// before, at and after the upgrade 2027 activation block.
func TestChainInfoTickFields(t *testing.T) {
	for _, regime := range tickRegimes() {
		s := newTickTestServer(t, regime)
		chain := s.cfg.Chain
		hashes := []chainhash.Hash{*chaincfg.RegressionNetParams.GenesisHash}
		for tip := range int32(3) {
			if tip > 0 {
				hashes = append(hashes, *mineTestBlock(t, chain, nil).Hash())
			}

			res, err := handleGetBlockChainInfo(s, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			info := res.(*btcjson.GetBlockChainInfoResult)
			got := [4]int64{info.BlocksTicks, info.HeadersTicks,
				info.TargetTime, info.NextTargetTime}
			want := [4]int64{regime.ticks[tip], regime.ticks[tip],
				regime.targetTime[tip], regime.nextTargetTime[tip]}
			if got != want {
				t.Errorf("%s: tip %d: blocks_ticks, headers_ticks, "+
					"target_time, next_target_time %v, want %v",
					regime.name, tip, got, want)
			}
			checkUpgradeStatus(t, regime, tip, hashes, info.UpgradeStatus)

			res, err = handleGetMiningInfo(s, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			mi := res.(*btcjson.GetMiningInfoResult)
			if mi.TargetTime != regime.targetTime[tip] ||
				mi.NextTargetTime != regime.nextTargetTime[tip] {

				t.Errorf("%s: tip %d: getmininginfo target_time %d, "+
					"next_target_time %d, want %d and %d",
					regime.name, tip, mi.TargetTime,
					mi.NextTargetTime, regime.targetTime[tip],
					regime.nextTargetTime[tip])
			}

			state := s.gbtWorkState
			state.Lock()
			err = state.updateBlockTemplate(s, true)
			var tmpl *btcjson.GetBlockTemplateResult
			if err == nil {
				tmpl, err = state.blockTemplateResult(true, nil)
			}
			state.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			if tmpl.TargetTime != regime.nextTargetTime[tip] {
				t.Errorf("%s: tip %d: getblocktemplate target_time "+
					"%d, want %d", regime.name, tip,
					tmpl.TargetTime, regime.nextTargetTime[tip])
			}
		}
	}
}

// checkUpgradeStatus checks the upgrade_status of getblockchaininfo with the
// chain's tip at the passed height.
func checkUpgradeStatus(t *testing.T, regime tickRegime, tip int32, hashes []chainhash.Hash, status *btcjson.UpgradeStatus) {
	t.Helper()
	if status == nil {
		t.Fatalf("%s: tip %d: no upgrade_status", regime.name, tip)
	}
	wantMTP := chaincfg.RegressionNetParams.Upgrade2027ActivationTime
	if regime.activation != nil {
		wantMTP = *regime.activation
	}
	activated := regime.nextTargetTime[tip] == 60
	if status.Name != "May 2027 Upgrade (regtestnet)" ||
		status.MempoolActivated != activated ||
		status.MempoolActivationMTP != wantMTP {

		t.Errorf("%s: tip %d: upgrade_status %+v, want activated %v at "+
			"%d", regime.name, tip, status, activated, wantMTP)
	}

	// With the upgrade active, block 1 is the preactivation block and block
	// 2, once mined, the postactivation block.
	checkBlock := func(what string, height *int32, hash *string, want int32) {
		t.Helper()
		if !activated || int(want) >= len(hashes) {
			if height != nil || hash != nil {
				t.Errorf("%s: tip %d: %s block %v %v, want null",
					regime.name, tip, what, height, hash)
			}
			return
		}
		if height == nil || hash == nil || *height != want ||
			*hash != hashes[want].String() {

			t.Errorf("%s: tip %d: %s block %v %v, want %d %v",
				regime.name, tip, what, height, hash, want,
				hashes[want])
		}
	}
	checkBlock("preactivation", status.BlockPreActivationHeight,
		status.BlockPreActivationHash, 1)
	checkBlock("postactivation", status.BlockPostActivationHeight,
		status.BlockPostActivationHash, 2)
}

// TestTxTickConfirmations ensures getrawtransaction and gettxout report the
// confirmations of a block's coinbase in ticks on both sides of the upgrade
// 2027 seam.
func TestTxTickConfirmations(t *testing.T) {
	for _, regime := range tickRegimes() {
		s := newTickTestServer(t, regime)
		chain := s.cfg.Chain
		var blocks []*bchutil.Block
		for range 3 {
			blocks = append(blocks, mineTestBlock(t, chain, nil))
		}
		const tip = 3
		verbose := btcjson.VerboseLevel(1)
		noMempool := false
		for i, block := range blocks {
			height := i + 1
			want := regime.ticks[tip] - regime.ticks[height] +
				regime.targetTime[height]
			txid := block.Transactions()[0].Hash().String()

			res, err := handleGetRawTransaction(s,
				btcjson.NewGetRawTransactionCmd(txid, &verbose), nil)
			if err != nil {
				t.Fatalf("%s: getrawtransaction %d: %v", regime.name,
					height, err)
			}
			if got := res.(btcjson.TxRawResult).ConfirmationsTicks; got != want {
				t.Errorf("%s: getrawtransaction %d: "+
					"confirmations_ticks %d, want %d", regime.name,
					height, got, want)
			}

			res, err = handleGetTxOut(s,
				btcjson.NewGetTxOutCmd(txid, 0, &noMempool), nil)
			if err != nil {
				t.Fatalf("%s: gettxout %d: %v", regime.name, height,
					err)
			}
			if got := res.(*btcjson.GetTxOutResult).ConfirmationsTicks; got != want {
				t.Errorf("%s: gettxout %d: confirmations_ticks %d, "+
					"want %d", regime.name, height, got, want)
			}
		}
	}
}

// TestMempoolTxTickConfirmations ensures a memory pool transaction has no
// confirmations in ticks on both sides of the upgrade 2027 seam: gettxout
// reports 0 for its output and verbose getrawtransaction leaves the field out.
func TestMempoolTxTickConfirmations(t *testing.T) {
	for _, regime := range tickRegimes() {
		s := newTickTestServer(t, regime)
		chain := s.cfg.Chain
		params := s.cfg.ChainParams
		first := mineTestBlock(t, chain, nil)
		maturity := int64(params.CoinbaseMaturity) * 600
		firstTicks := chain.BestSnapshot().TickSchedule.HeightToTicks(1)
		for chain.BestSnapshot().TickSchedule.NextBlockTicks()-firstTicks <
			maturity {

			mineTestBlock(t, chain, nil)
		}

		// Spend the first coinbase, which pays to OP_TRUE.  The script is
		// long enough for the transaction to pass the minimum size.
		script := append(bytes.Repeat([]byte{txscript.OP_NOP}, 10),
			txscript.OP_TRUE)
		coinbase := first.Transactions()[0]
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: *coinbase.Hash()}, nil))
		tx.AddTxOut(wire.NewTxOut(coinbase.MsgTx().TxOut[0].Value-10000,
			script, wire.TokenData{}))
		_, err := s.cfg.TxMemPool.ProcessTransaction(bchutil.NewTx(tx),
			false, 0)
		if err != nil {
			t.Fatalf("%s: transaction not accepted: %v", regime.name, err)
		}
		txid := tx.TxHash().String()

		res, err := handleGetTxOut(s, btcjson.NewGetTxOutCmd(txid, 0, nil),
			nil)
		if err != nil {
			t.Fatalf("%s: gettxout: %v", regime.name, err)
		}
		reply, _ := res.(*btcjson.GetTxOutResult)
		if reply == nil {
			t.Fatalf("%s: gettxout: got null, want an output",
				regime.name)
		}
		if reply.Confirmations != 0 || reply.ConfirmationsTicks != 0 {
			t.Errorf("%s: gettxout: confirmations %d, "+
				"confirmations_ticks %d, want 0 and 0", regime.name,
				reply.Confirmations, reply.ConfirmationsTicks)
		}

		verbose := btcjson.VerboseLevel(1)
		res, err = handleGetRawTransaction(s,
			btcjson.NewGetRawTransactionCmd(txid, &verbose), nil)
		if err != nil {
			t.Fatalf("%s: getrawtransaction: %v", regime.name, err)
		}
		marshalled, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(marshalled, &fields); err != nil {
			t.Fatal(err)
		}
		if _, ok := fields["confirmations_ticks"]; ok {
			t.Errorf("%s: getrawtransaction: confirmations_ticks "+
				"reported for a memory pool transaction", regime.name)
		}
	}
}

// TestUpgradeNetName ensures the upgrade_status name gives each network the
// name other nodes give it.
func TestUpgradeNetName(t *testing.T) {
	for _, test := range []struct {
		params *chaincfg.Params
		want   string
	}{
		{&chaincfg.MainNetParams, "mainnet"},
		{&chaincfg.TestNet3Params, "testnet"},
		{&chaincfg.TestNet4Params, "test4net"},
		{&chaincfg.ChipNetParams, "chipnet"},
		{&chaincfg.TempNetParams, "tempnet"},
		{&chaincfg.RegressionNetParams, "regtestnet"},
		{&chaincfg.SimNetParams, "simnet"},
	} {
		if got := upgradeNetName(test.params); got != test.want {
			t.Errorf("%s: got %q, want %q", test.params.Name, got,
				test.want)
		}
	}
}
