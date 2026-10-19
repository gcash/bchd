// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"container/heap"
	"math/rand"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"

	"github.com/gcash/bchutil"
)

// TestTxFeePrioHeap ensures the priority queue pops transactions by fee per
// kilobyte, highest first.
func TestTxFeePrioHeap(t *testing.T) {
	testItems := []*txPrioItem{
		{feePerKB: 5678},
		{feePerKB: 5678}, // Duplicate fee
		{feePerKB: 1234},
		{feePerKB: 10000},
		{feePerKB: 0},
	}

	// Add random data in addition to the edge conditions already manually
	// specified.
	randSeed := rand.Int63()
	defer func() {
		if t.Failed() {
			t.Logf("Random numbers using seed: %v", randSeed)
		}
	}()
	prng := rand.New(rand.NewSource(randSeed))
	for range 1000 {
		testItems = append(testItems, &txPrioItem{
			feePerKB: int64(prng.Float64() * bchutil.SatoshiPerBitcoin),
		})
	}

	priorityQueue := newTxPriorityQueue(len(testItems))
	for _, prioItem := range testItems {
		heap.Push(priorityQueue, prioItem)
	}
	prev := heap.Pop(priorityQueue).(*txPrioItem)
	for priorityQueue.Len() > 0 {
		prioItem := heap.Pop(priorityQueue).(*txPrioItem)
		if prioItem.feePerKB > prev.feePerKB {
			t.Fatalf("fee sort: item with fee per KB %v popped "+
				"after %v", prioItem.feePerKB, prev.feePerKB)
		}
		prev = prioItem
	}
}

// Test_createCoinbaseTx tests that the coinbase is padded to be over the
// minimum transaction size that is actually enforced for the block being built.
// Upgrade9 lowered that minimum, so until it activates the larger Magnetic
// Anomaly minimum still applies.
func Test_createCoinbaseTx(t *testing.T) {
	miningAddr, err := bchutil.DecodeAddress("qr0ayr8hdlg6zl7kcn8mgc8cz04aczyw4567fpu8rl", &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		params *chaincfg.Params
		height int32
		addr   bchutil.Address
	}{
		{
			// Magnetic Anomaly is active but Upgrade9 is not, so the
			// minimum transaction size is still 100.
			name:   "mainnet between magnetic anomaly and upgrade9",
			params: &chaincfg.MainNetParams,
			height: 584412,
			addr:   miningAddr,
		},
		{
			name:   "mainnet after upgrade9",
			params: &chaincfg.MainNetParams,
			height: chaincfg.MainNetParams.Upgrade9ForkHeight + 1,
			addr:   miningAddr,
		},
		{
			// Simnet never activates Upgrade9, so every block mined above
			// the Magnetic Anomaly height must still reach 100 bytes.
			name:   "simnet above magnetic anomaly",
			params: &chaincfg.SimNetParams,
			height: chaincfg.SimNetParams.MagneticAnonomalyForkHeight + 1,
			addr:   nil,
		},
		{
			name:   "regtest",
			params: &chaincfg.RegressionNetParams,
			height: 1,
			addr:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			coinbaseScript, err := standardCoinbaseScript(test.height, 123456789)
			if err != nil {
				t.Fatal(err)
			}
			coinbase, err := createCoinbaseTx(test.params,
				coinbaseScript[:len(coinbaseScript)-2], test.height, test.addr)
			if err != nil {
				t.Fatal(err)
			}

			// Use the fork state that genuinely applies at this height rather
			// than asserting a fixed one.
			magneticAnomalyActive := test.height > test.params.MagneticAnonomalyForkHeight
			upgrade9Active := test.height > test.params.Upgrade9ForkHeight

			err = blockchain.CheckTransactionSanity(coinbase, magneticAnomalyActive,
				upgrade9Active, txscript.StandardVerifyFlags)
			if err != nil {
				t.Fatalf("coinbase of %d bytes failed sanity at height %d "+
					"(magneticAnomaly=%v upgrade9=%v): %v",
					coinbase.MsgTx().SerializeSize(), test.height,
					magneticAnomalyActive, upgrade9Active, err)
			}
		})
	}
}
