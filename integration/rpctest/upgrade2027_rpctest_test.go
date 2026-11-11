// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package rpctest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestUpgrade2027ActivatedHarness starts bchd with the 1-minute block upgrade
// activating in the past and mines with both the node's templates and the
// harness's own blocks.
func TestUpgrade2027ActivatedHarness(t *testing.T) {
	h, err := New(&chaincfg.RegressionNetParams, nil,
		[]string{Upgrade2027Arg(Upgrade2027InThePast)})
	if err != nil {
		t.Fatalf("unable to create harness: %v", err)
	}
	defer h.TearDown()
	if err := h.SetUp(false, 0); err != nil {
		t.Fatalf("unable to set up harness: %v", err)
	}

	hashes, err := h.Node.Generate(5)
	if err != nil {
		t.Fatalf("unable to generate blocks: %v", err)
	}
	for range 5 {
		block, err := h.GenerateAndSubmitBlock(nil, -1, time.Time{})
		if err != nil {
			t.Fatalf("unable to submit block: %v", err)
		}
		hashes = append(hashes, block.Hash())
	}

	info, err := h.Node.GetBlockChainInfo()
	if err != nil {
		t.Fatalf("unable to get chain info: %v", err)
	}
	if info.Blocks != 10 {
		t.Fatalf("chain height is %d, want 10", info.Blocks)
	}

	// Wall clock timestamps put the median time past above the activation
	// time either way, so check that bchd took the override instead.
	// TODO: check the activation state over RPC once bchd reports it.
	logFile := filepath.Join(h.node.config.logDir,
		chaincfg.RegressionNetParams.Name, "bchd.log")
	log, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("unable to read the node log: %v", err)
	}
	want := "Using upgrade 2027 activation time override: 1605451779"
	if !strings.Contains(string(log), want) {
		t.Fatalf("node log lacks %q", want)
	}

	// Every coinbase is version 2, including those up to the activation
	// block that pay 50 BCH and so must escape the forbidden shape.
	for i, hash := range hashes {
		checkCoinbaseVersion(t, h, i+1, hash)
	}

	// Blocks after the activation block pay a tenth of the subsidy, both
	// from the node's templates and the harness's own blocks.  The genesis
	// block is too old to activate, so block 1 at least pays 50 BCH.
	activation, err := h.upgrade2027Height(hashes[9], 10)
	if err != nil {
		t.Fatalf("unable to find the activation block: %v", err)
	}
	if activation < 1 || activation > 4 {
		t.Fatalf("activation block at %d, want one before block 5",
			activation)
	}
	for i, hash := range hashes {
		block, err := h.Node.GetBlock(hash)
		if err != nil {
			t.Fatalf("unable to get block %d: %v", i+1, err)
		}
		want := int64(500000000)
		if int32(i+1) <= activation {
			want = 5000000000
		}
		if v := block.Transactions[0].TxOut[0].Value; v != want {
			t.Errorf("coinbase of block %d pays %d, want %d", i+1, v,
				want)
		}
	}
}

// TestUpgrade2027CoinbaseMaturity sets up a harness with one mature output
// on a chain that activates 1-minute blocks in its first blocks, and checks
// that the wallet takes coinbase maturity in ticks and that the node accepts
// its spend.  The setup mines 1,001 blocks, after which only the coinbases
// of the first few blocks are 60,000 ticks below the tip.
func TestUpgrade2027CoinbaseMaturity(t *testing.T) {
	h, err := New(&chaincfg.RegressionNetParams, nil,
		[]string{Upgrade2027Arg(Upgrade2027InThePast)})
	if err != nil {
		t.Fatalf("unable to create harness: %v", err)
	}
	defer h.TearDown()
	if err := h.SetUp(true, 1); err != nil {
		t.Fatalf("unable to set up harness: %v", err)
	}

	tipHash, tip, err := h.Node.GetBestBlock()
	if err != nil {
		t.Fatalf("unable to get the tip: %v", err)
	}
	if tip != 1001 {
		t.Fatalf("chain height is %d, want 1001", tip)
	}
	activation, err := h.upgrade2027Height(tipHash, tip)
	if err != nil || activation < 1 || activation > 4 {
		t.Fatalf("activation block at %d (%v), want one before "+
			"block 5", activation, err)
	}
	lastMature := h.lastMatureCoinbase(tip)
	wantLast := (540*activation + 60) / 600
	if lastMature != wantLast {
		t.Fatalf("last mature coinbase at %d, want %d", lastMature,
			wantLast)
	}
	if got := h.ConfirmedBalance(); got != bchutil.Amount(lastMature)*
		50*bchutil.SatoshiPerBitcoin {

		t.Fatalf("confirmed balance %v with %d mature coinbases", got,
			lastMature)
	}

	addr, err := h.NewAddress()
	if err != nil {
		t.Fatalf("unable to get an address: %v", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("unable to make a script: %v", err)
	}
	out := wire.NewTxOut(bchutil.SatoshiPerBitcoin, pkScript,
		wire.TokenData{})
	if _, err := h.SendOutputs([]*wire.TxOut{out}, 10); err != nil {
		t.Fatalf("node refused the spend of a mature coinbase: %v", err)
	}
}

// TestUpgrade2027Timelocks submits, block after block on a chain that
// activates 1-minute blocks in its first blocks, the spend of the first
// coinbase the node holds immature, a version 2 spend with a relative lock
// of one unit and a spend with a height lock time.  The node must refuse
// each, for the reason of its rule, until the block after the tip is far
// enough along in ticks, and take it from then on.  The seam itself is
// covered by the blockchain and mempool tests, as the harness cannot place
// the activation block at will.
func TestUpgrade2027Timelocks(t *testing.T) {
	h, err := New(&chaincfg.RegressionNetParams, nil,
		[]string{Upgrade2027Arg(Upgrade2027InThePast)})
	if err != nil {
		t.Fatalf("unable to create harness: %v", err)
	}
	defer h.TearDown()
	if err := h.SetUp(true, 1); err != nil {
		t.Fatalf("unable to set up harness: %v", err)
	}

	// Fund two outputs of a fresh key and mine them.
	key, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatalf("unable to make a key: %v", err)
	}
	addr, err := bchutil.NewAddressPubKey(key.PubKey().SerializeCompressed(),
		h.ActiveNet)
	if err != nil {
		t.Fatalf("unable to make an address: %v", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr.AddressPubKeyHash())
	if err != nil {
		t.Fatalf("unable to make a script: %v", err)
	}
	const amount = bchutil.SatoshiPerBitcoin
	fundTx, err := h.CreateTransaction([]*wire.TxOut{
		wire.NewTxOut(amount, pkScript, wire.TokenData{}),
		wire.NewTxOut(amount, pkScript, wire.TokenData{}),
	}, 10, true)
	if err != nil {
		t.Fatalf("unable to fund the outputs: %v", err)
	}
	if _, err := h.Node.SendRawTransaction(fundTx, true); err != nil {
		t.Fatalf("node refused the funding: %v", err)
	}
	if _, err := h.Node.Generate(1); err != nil {
		t.Fatalf("unable to generate a block: %v", err)
	}
	fundHash, fund, err := h.Node.GetBestBlock()
	if err != nil {
		t.Fatalf("unable to get the tip: %v", err)
	}
	var funded []wire.OutPoint
	for i, out := range fundTx.TxOut {
		if bytes.Equal(out.PkScript, pkScript) {
			funded = append(funded, wire.OutPoint{
				Hash: fundTx.TxHash(), Index: uint32(i)})
		}
	}
	if len(funded) != 2 {
		t.Fatalf("funding pays the key %d times, want 2", len(funded))
	}

	activation, err := h.upgrade2027Height(fundHash, fund)
	if err != nil || activation < 1 || activation > 4 {
		t.Fatalf("activation block at %d (%v), want one before "+
			"block 5", activation, err)
	}
	ticks := func(height int32) int64 {
		return heightToTicks(activation, height)
	}
	// release returns the first block after the tip for which final
	// holds.
	release := func(final func(blockTicks int64) bool) int32 {
		b := fund + 1
		for !final(ticks(b)) {
			b++
		}
		return b
	}

	// The first coinbase that is not mature for the next block, which
	// pays the wallet's coinbase key.
	const maturity = 100 * 600
	coinHeight := int32(1)
	for ticks(fund+1)-ticks(coinHeight) >= maturity {
		coinHeight++
	}
	coinHash, err := h.Node.GetBlockHash(int64(coinHeight))
	if err != nil {
		t.Fatalf("unable to get block %d: %v", coinHeight, err)
	}
	coinBlock, err := h.Node.GetBlock(coinHash)
	if err != nil {
		t.Fatalf("unable to get block %d: %v", coinHeight, err)
	}
	coinbase := coinBlock.Transactions[0]

	// A height lock time a few blocks past the block after the tip.
	lockTime := uint32((ticks(fund+1)+599)/600 + 1)

	type spend struct {
		name    string
		tx      *wire.MsgTx
		release int32
		reason  string
	}
	spends := []spend{{
		name: "coinbase",
		tx: signedTimelockSpend(t, 1, wire.OutPoint{
			Hash: coinbase.TxHash()}, coinbase.TxOut[0].Value,
			coinbase.TxOut[0].PkScript, h.wallet.coinbaseKey,
			wire.MaxTxInSequenceNum, 0),
		release: release(func(b int64) bool {
			return b-ticks(coinHeight) >= maturity
		}),
		reason: "bad-txns-premature-spend-of-coinbase",
	}, {
		name: "relative lock",
		tx: signedTimelockSpend(t, 2, funded[0], amount, pkScript,
			key, blockchain.LockTimeToSequence(false, 1), 0),
		release: release(func(b int64) bool {
			return b-ticks(fund) >= 600
		}),
		reason: "non-BIP68-final",
	}, {
		name: "lock time",
		tx: signedTimelockSpend(t, 1, funded[1], amount, pkScript,
			key, wire.MaxTxInSequenceNum-1, lockTime),
		release: release(func(b int64) bool {
			return 600*int64(lockTime) < b
		}),
		reason: "bad-txns-nonfinal",
	}}
	if spends[1].release != fund+10 {
		t.Fatalf("relative lock releases at %d, want %d",
			spends[1].release, fund+10)
	}

	last := fund
	for _, s := range spends {
		last = max(last, s.release-1)
	}
	for tip := fund; tip <= last; tip++ {
		for _, s := range spends {
			if tip+1 > s.release {
				continue
			}
			_, err := h.Node.SendRawTransaction(s.tx, true)
			switch {
			case tip+1 < s.release && err == nil:
				t.Fatalf("%s: node took it at tip %d, want it "+
					"from tip %d", s.name, tip, s.release-1)
			case tip+1 < s.release &&
				!strings.HasPrefix(err.Error(), "-26: "+s.reason):
				t.Fatalf("%s: tip %d: got %v, want %s", s.name,
					tip, err, s.reason)
			case tip+1 == s.release && err != nil:
				t.Fatalf("%s: node refused it at tip %d: %v",
					s.name, tip, err)
			}
		}
		if _, err := h.Node.Generate(1); err != nil {
			t.Fatalf("unable to generate a block: %v", err)
		}
	}

	// The node's own templates mined every spend.
	pool, err := h.Node.GetRawMempool()
	if err != nil {
		t.Fatalf("unable to get the mempool: %v", err)
	}
	if len(pool) != 0 {
		t.Fatalf("%d transactions left in the mempool", len(pool))
	}
}

// signedTimelockSpend returns a transaction of the passed version and lock
// time that spends the pay-to-pubkey-hash output prevOut of key with the
// passed sequence.
func signedTimelockSpend(t *testing.T, version int32, prevOut wire.OutPoint,
	amount int64, pkScript []byte, key *bchec.PrivateKey, sequence,
	lockTime uint32) *wire.MsgTx {

	t.Helper()
	tx := wire.NewMsgTx(version)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut, Sequence: sequence})
	tx.AddTxOut(wire.NewTxOut(amount-10000, pkScript, wire.TokenData{}))
	tx.LockTime = lockTime
	sigScript, err := txscript.SignatureScript(tx, 0, amount, pkScript,
		txscript.SigHashAll, key, true)
	if err != nil {
		t.Fatalf("unable to sign: %v", err)
	}
	tx.TxIn[0].SignatureScript = sigScript
	return tx
}

func checkCoinbaseVersion(t *testing.T, h *Harness, height int,
	hash *chainhash.Hash) {

	t.Helper()
	block, err := h.Node.GetBlock(hash)
	if err != nil {
		t.Fatalf("unable to get block %d: %v", height, err)
	}
	if v := block.Transactions[0].Version; v != 2 {
		t.Errorf("coinbase of block %d has version %d, want 2", height, v)
	}
}
