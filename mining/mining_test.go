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

// TestTxFeePrioHeap ensures the priority queue for transaction fees and
// priorities works as expected.
func TestTxFeePrioHeap(t *testing.T) {
	// Create some fake priority items that exercise the expected sort
	// edge conditions.
	testItems := []*txPrioItem{
		{feePerKB: 5678, priority: 3},
		{feePerKB: 5678, priority: 1},
		{feePerKB: 5678, priority: 1}, // Duplicate fee and prio
		{feePerKB: 5678, priority: 5},
		{feePerKB: 5678, priority: 2},
		{feePerKB: 1234, priority: 3},
		{feePerKB: 1234, priority: 1},
		{feePerKB: 1234, priority: 5},
		{feePerKB: 1234, priority: 5}, // Duplicate fee and prio
		{feePerKB: 1234, priority: 2},
		{feePerKB: 10000, priority: 0}, // Higher fee, smaller prio
		{feePerKB: 0, priority: 10000}, // Higher prio, lower fee
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
			priority: prng.Float64() * 100,
		})
	}

	// Test sorting by fee per KB then priority.
	var highest *txPrioItem
	priorityQueue := newTxPriorityQueue(len(testItems), true)
	for i := range testItems {
		prioItem := testItems[i]
		if highest == nil {
			highest = prioItem
		}
		if prioItem.feePerKB >= highest.feePerKB &&
			prioItem.priority > highest.priority {

			highest = prioItem
		}
		heap.Push(priorityQueue, prioItem)
	}

	for range testItems {
		prioItem := heap.Pop(priorityQueue).(*txPrioItem)
		if prioItem.feePerKB >= highest.feePerKB &&
			prioItem.priority > highest.priority {

			t.Fatalf("fee sort: item (fee per KB: %v, "+
				"priority: %v) higher than than prev "+
				"(fee per KB: %v, priority %v)",
				prioItem.feePerKB, prioItem.priority,
				highest.feePerKB, highest.priority)
		}
		highest = prioItem
	}

	// Test sorting by priority then fee per KB.
	highest = nil
	priorityQueue = newTxPriorityQueue(len(testItems), false)
	for i := range testItems {
		prioItem := testItems[i]
		if highest == nil {
			highest = prioItem
		}
		if prioItem.priority >= highest.priority &&
			prioItem.feePerKB > highest.feePerKB {

			highest = prioItem
		}
		heap.Push(priorityQueue, prioItem)
	}

	for range testItems {
		prioItem := heap.Pop(priorityQueue).(*txPrioItem)
		if prioItem.priority >= highest.priority &&
			prioItem.feePerKB > highest.feePerKB {

			t.Fatalf("priority sort: item (fee per KB: %v, "+
				"priority: %v) higher than than prev "+
				"(fee per KB: %v, priority %v)",
				prioItem.feePerKB, prioItem.priority,
				highest.feePerKB, highest.priority)
		}
		highest = prioItem
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
			// Regtest never activates Upgrade9, so every block mined above
			// the Magnetic Anomaly height must still reach 100 bytes.
			name:   "regtest above magnetic anomaly",
			params: &chaincfg.RegressionNetParams,
			height: chaincfg.RegressionNetParams.MagneticAnonomalyForkHeight + 1,
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
