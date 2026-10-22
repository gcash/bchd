// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"errors"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// sizeFloorTestFloor is the default block size of the parameters
// sizeFloorTestParams returns, which allows ten sigchecks per block.
const sizeFloorTestFloor = 10 * BlockMaxBytesMaxSigChecksRatio

// sizeFloorTestParams returns regtest parameters on the given network with a
// default block size of sizeFloorTestFloor and ABLA starting at height 1000,
// so that the size limit of the first blocks is the network's floor.
func sizeFloorTestParams(net wire.BitcoinNet) *chaincfg.Params {
	params := chaincfg.RegressionNetParams
	params.Net = net
	params.ABLAForkHeight = 1000
	params.DefaultConsensusBlockSize = sizeFloorTestFloor
	return &params
}

// sigChecksTestSpend mines two blocks on the miner's chain, the second funding
// an output whose script runs sigChecks signature checks, and returns a
// transaction spending it.  Its input script is padded to pay for the checks
// under the VM limits.
func sigChecksTestSpend(t *testing.T, m *ruleTestMiner, sigChecks int) *wire.MsgTx {
	t.Helper()
	key, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{1}, 32))
	builder := txscript.NewScriptBuilder().AddOp(txscript.OP_SWAP).
		AddOp(txscript.OP_DROP).AddData(key.PubKey().SerializeCompressed())
	for range sigChecks - 1 {
		builder.AddOp(txscript.OP_2DUP).AddOp(txscript.OP_CHECKSIGVERIFY)
	}
	script, err := builder.AddOp(txscript.OP_CHECKSIG).Script()
	if err != nil {
		t.Fatal(err)
	}

	m.mustAccept(m.build(nil))
	cb := m.tip.Transactions[0]
	fund := ruleTestSpend(wire.OutPoint{Hash: cb.TxHash()},
		cb.TxOut[0].Value-1000, 1)
	fund.TxOut[0].PkScript = script
	m.mustAccept(m.build(nil, fund))

	spend := wire.NewMsgTx(2)
	spend.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: fund.TxHash()},
		Sequence:         0xffffffff,
	})
	spend.AddTxOut(wire.NewTxOut(fund.TxOut[0].Value-1000,
		[]byte{txscript.OP_TRUE}, wire.TokenData{}))
	sig, err := txscript.RawTxInSchnorrSignature(spend, 0, script,
		txscript.SigHashAll, key, fund.TxOut[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	spend.TxIn[0].SignatureScript, err = txscript.NewScriptBuilder().
		AddData(make([]byte, 40*sigChecks)).AddData(sig).Script()
	if err != nil {
		t.Fatal(err)
	}
	return spend
}

// TestNetworkSizeFloorValidation ensures blocks are validated against the
// network's default block size, for their size and for their sigchecks,
// rather than against a larger excessive block size, except on regtest and
// simnet.
func TestNetworkSizeFloorValidation(t *testing.T) {
	tests := []struct {
		name      string
		net       wire.BitcoinNet
		size      int
		sigChecks int
		reject    bool
		code      ErrorCode
	}{
		{name: "size at the floor", net: wire.MainNet,
			size: sizeFloorTestFloor},
		{name: "size above the floor", net: wire.MainNet,
			size: sizeFloorTestFloor + 1, reject: true, code: ErrBlockTooBig},
		{name: "size above the floor on regtest", net: wire.TestNet,
			size: sizeFloorTestFloor + 1},
		{name: "sigchecks at the floor's limit", net: wire.MainNet,
			sigChecks: 10},
		{name: "sigchecks above the floor's limit", net: wire.MainNet,
			sigChecks: 11, reject: true, code: ErrTooManySigChecks},
		{name: "sigchecks above the floor's limit on regtest",
			net: wire.TestNet, sigChecks: 11},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := newRuleTestMiner(t, sizeFloorTestParams(test.net))
			var block *bchutil.Block
			if test.sigChecks > 0 {
				block = m.build(nil, sigChecksTestSpend(t, m, test.sigChecks))
				if size := block.MsgBlock().SerializeSize(); size > sizeFloorTestFloor {
					t.Fatalf("the test block has %d bytes", size)
				}
			} else {
				params := m.chain.chainParams
				block = ablaTestBlock(t, params.GenesisBlock, 1, test.size,
					params)
			}

			_, _, err := m.chain.ProcessBlock(block, BFNone)
			if !test.reject {
				if err != nil {
					t.Fatalf("block rejected: %v", err)
				}
				return
			}
			var ruleErr RuleError
			if !errors.As(err, &ruleErr) || ruleErr.ErrorCode != test.code {
				t.Fatalf("got %v, want %v", err, test.code)
			}
		})
	}
}
