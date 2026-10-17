// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

func testGetBestBlock(r *rpctest.Harness, t *testing.T) {
	_, prevbestHeight, err := r.Node.GetBestBlock()
	if err != nil {
		t.Fatalf("Call to `getbestblock` failed: %v", err)
	}

	// Create a new block connecting to the current tip.
	generatedBlockHashes, err := r.Node.Generate(1)
	if err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}

	bestHash, bestHeight, err := r.Node.GetBestBlock()
	if err != nil {
		t.Fatalf("Call to `getbestblock` failed: %v", err)
	}

	// Hash should be the same as the newly submitted block.
	if !bytes.Equal(bestHash[:], generatedBlockHashes[0][:]) {
		t.Fatalf("Block hashes do not match. Returned hash %v, wanted "+
			"hash %v", bestHash, generatedBlockHashes[0][:])
	}

	// Block height should now reflect newest height.
	if bestHeight != prevbestHeight+1 {
		t.Fatalf("Block heights do not match. Got %v, wanted %v",
			bestHeight, prevbestHeight+1)
	}
}

func testGetBlockCount(r *rpctest.Harness, t *testing.T) {
	// Save the current count.
	currentCount, err := r.Node.GetBlockCount()
	if err != nil {
		t.Fatalf("Unable to get block count: %v", err)
	}

	if _, err := r.Node.Generate(1); err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}

	// Count should have increased by one.
	newCount, err := r.Node.GetBlockCount()
	if err != nil {
		t.Fatalf("Unable to get block count: %v", err)
	}
	if newCount != currentCount+1 {
		t.Fatalf("Block count incorrect. Got %v should be %v",
			newCount, currentCount+1)
	}
}

func testGetBlockHash(r *rpctest.Harness, t *testing.T) {
	// Create a new block connecting to the current tip.
	generatedBlockHashes, err := r.Node.Generate(1)
	if err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}

	info, err := r.Node.GetInfo()
	if err != nil {
		t.Fatalf("call to getinfo cailed: %v", err)
	}

	blockHash, err := r.Node.GetBlockHash(int64(info.Blocks))
	if err != nil {
		t.Fatalf("Call to `getblockhash` failed: %v", err)
	}

	// Block hashes should match newly created block.
	if !bytes.Equal(generatedBlockHashes[0][:], blockHash[:]) {
		t.Fatalf("Block hashes do not match. Returned hash %v, wanted "+
			"hash %v", blockHash, generatedBlockHashes[0][:])
	}
}

func testGetBlockVerbosity3(r *rpctest.Harness, t *testing.T) {
	// Mine a block that holds a transaction spending a coinbase output.
	addr, err := r.NewAddress()
	if err != nil {
		t.Fatalf("Unable to generate address: %v", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("Unable to create script: %v", err)
	}
	output := wire.NewTxOut(bchutil.SatoshiPerBitcoin, pkScript,
		wire.TokenData{})
	txid, err := r.SendOutputs([]*wire.TxOut{output}, 10)
	if err != nil {
		t.Fatalf("Unable to send outputs: %v", err)
	}
	blockHashes, err := r.Node.Generate(1)
	if err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}

	// Verbosity 2 lists the transactions under "rawtx", without the
	// prevout and fee fields.
	raw, err := r.Node.RawRequest("getblock", []json.RawMessage{
		json.RawMessage(`"` + blockHashes[0].String() + `"`),
		json.RawMessage("2"),
	})
	if err != nil {
		t.Fatalf("Call to `getblock` failed: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("Unable to unmarshal block: %v", err)
	}
	_, hasRawTx := fields["rawtx"]
	_, hasTx := fields["tx"]
	if !hasRawTx || hasTx || bytes.Contains(raw, []byte(`"prevout"`)) ||
		bytes.Contains(raw, []byte(`"fee"`)) {

		t.Fatalf("Unexpected verbosity 2 block: %s", raw)
	}

	toAmount := func(value float64) bchutil.Amount {
		amount, err := bchutil.NewAmount(value)
		if err != nil {
			t.Fatalf("Invalid amount %v: %v", value, err)
		}
		return amount
	}

	// Verbosity 4 is treated as 3.
	for _, verbosity := range []string{"3", "4"} {
		raw, err := r.Node.RawRequest("getblock", []json.RawMessage{
			json.RawMessage(`"` + blockHashes[0].String() + `"`),
			json.RawMessage(verbosity),
		})
		if err != nil {
			t.Fatalf("Call to `getblock` failed: %v", err)
		}
		var block btcjson.GetBlockVerboseTxResult
		if err := json.Unmarshal(raw, &block); err != nil {
			t.Fatalf("Unable to unmarshal block: %v", err)
		}
		if len(block.Tx) != 2 || block.Tx[1].Txid != txid.String() ||
			block.Tx[0].Fee != nil || block.Tx[1].Fee == nil {

			t.Fatalf("Unexpected transactions in block: %s", raw)
		}

		// The fee is the value of the spent outputs minus the value of
		// the outputs.
		tx := block.Tx[1]
		var fee bchutil.Amount
		for _, vin := range tx.Vin {
			if vin.PrevOut == nil {
				t.Fatalf("Missing prevout: %s", raw)
			}
			if !vin.PrevOut.Generated || vin.PrevOut.Height <= 0 {
				t.Fatalf("Unexpected prevout: %s", raw)
			}
			fee += toAmount(vin.PrevOut.Value)
		}
		for _, vout := range tx.Vout {
			fee -= toAmount(vout.Value)
		}
		if toAmount(*tx.Fee) != fee {
			t.Fatalf("Unexpected fee %v, want %v", *tx.Fee, fee)
		}
	}
}

func testMempoolAcceptAndSend(r *rpctest.Harness, t *testing.T) {
	addr, err := r.NewAddress()
	if err != nil {
		t.Fatalf("Unable to generate address: %v", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("Unable to create script: %v", err)
	}
	newTx := func(feeRate bchutil.Amount) (*wire.MsgTx, string) {
		output := wire.NewTxOut(bchutil.SatoshiPerBitcoin, pkScript,
			wire.TokenData{})
		tx, err := r.CreateTransaction([]*wire.TxOut{output}, feeRate,
			true)
		if err != nil {
			t.Fatalf("Unable to create transaction: %v", err)
		}
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatalf("Unable to serialize transaction: %v", err)
		}
		return tx, hex.EncodeToString(buf.Bytes())
	}
	testAccept := func(txHex string, allowHighFees bool) btcjson.TestMempoolAcceptResult {
		raw, err := r.Node.RawRequest("testmempoolaccept", []json.RawMessage{
			json.RawMessage(`["` + txHex + `"]`),
			json.RawMessage(fmt.Sprint(allowHighFees)),
		})
		if err != nil {
			t.Fatalf("Call to `testmempoolaccept` failed: %v", err)
		}
		var results []btcjson.TestMempoolAcceptResult
		if err := json.Unmarshal(raw, &results); err != nil ||
			len(results) != 1 {

			t.Fatalf("Unexpected testmempoolaccept result: %s", raw)
		}
		return results[0]
	}

	// A transaction is allowed until it is in the mempool, and sending it
	// again returns its hash.
	tx, txHex := newTx(10)
	result := testAccept(txHex, false)
	if !result.Allowed || result.Txid != tx.TxHash().String() ||
		result.Size != int64(tx.SerializeSize()) || result.Fees == nil {

		t.Fatalf("Unexpected result for a valid transaction: %+v", result)
	}
	for range 2 {
		txid, err := r.Node.SendRawTransaction(tx, false)
		if err != nil || *txid != tx.TxHash() {
			t.Fatalf("Call to `sendrawtransaction` returned %v, %v",
				txid, err)
		}
	}
	result = testAccept(txHex, false)
	if result.Allowed ||
		result.RejectReason != "18: already have transaction "+result.Txid {

		t.Fatalf("Unexpected result for a transaction in the "+
			"mempool: %+v", result)
	}

	// A fee above the 0.1 BCH default maximum needs allowhighfees.
	tx, txHex = newTx(100000)
	result = testAccept(txHex, false)
	if result.Allowed || result.RejectReason != "256: absurdly-high-fee" {
		t.Fatalf("Unexpected result for a high fee: %+v", result)
	}
	if result = testAccept(txHex, true); !result.Allowed {
		t.Fatalf("Unexpected result for an allowed high fee: %+v",
			result)
	}
	_, err = r.Node.SendRawTransaction(tx, false)
	rpcErr, ok := err.(*btcjson.RPCError)
	if !ok || rpcErr.Code != btcjson.ErrRPCTxRejected {
		t.Fatalf("Unexpected error for a high fee: %v", err)
	}
	if _, err := r.Node.SendRawTransaction(tx, true); err != nil {
		t.Fatalf("Call to `sendrawtransaction` failed: %v", err)
	}

	if _, err := r.Node.Generate(1); err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}
}

var rpcTestCases = []rpctest.HarnessTestCase{
	testGetBestBlock,
	testGetBlockCount,
	testGetBlockHash,
	testGetBlockVerbosity3,
	testMempoolAcceptAndSend,
}

var primaryHarness *rpctest.Harness

func TestMain(m *testing.M) {
	var (
		err      error
		exitCode int
	)
	// Whatever happens, clean up active harnesses and exit with the last exit code.
	// Note that Exit at the end of the function would ignore any defers.
	defer func() {
		// Clean up any active harnesses that are still currently running.This
		// includes removing all temporary directories, and shutting down any
		// created processes.
		if err = rpctest.TearDownAll(); err != nil {
			// Don't set exitCode for a teardown error. Just make it visible.
			fmt.Println("unable to tear down all harnesses:", err)
		}
		os.Exit(exitCode)
	}()

	// In order to properly test scenarios on as if we were on mainnet,
	// ensure that non-standard transactions aren't accepted into the
	// mempool or relayed.
	bchdCfg := []string{"--rejectnonstd"}
	primaryHarness, err = rpctest.New(&chaincfg.SimNetParams, nil, bchdCfg)
	if err != nil {
		fmt.Println("unable to create primary harness: ", err)
		exitCode = 1
		return
	}
	defer func() {
		if err := primaryHarness.TearDown(); err != nil {
			// Don't set exitCode for a teardown error. Just make it visible.
			fmt.Println("unable to tear down primary harness:", err)
		}
	}()

	// Initialize the primary mining node with a chain of length 125,
	// providing 25 mature coinbases to allow spending from for testing
	// purposes.
	if err := primaryHarness.SetUp(true, 25); err != nil {
		fmt.Println("unable to setup test chain: ", err)
		exitCode = 1
		return
	}

	exitCode = m.Run()
}

func TestRpcServer(t *testing.T) {
	var currentTestNum int
	defer func() {
		// If one of the integration tests caused a panic within the main
		// goroutine, then tear down all the harnesses in order to avoid
		// any leaked bchd processes.
		if r := recover(); r != nil {
			fmt.Println("recovering from test panic: ", r)
			if err := rpctest.TearDownAll(); err != nil {
				fmt.Println("unable to tear down all harnesses: ", err)
			}
			t.Fatalf("test #%v panicked: %s", currentTestNum, debug.Stack())
		}
	}()

	for _, testCase := range rpcTestCases {
		testCase(primaryHarness, t)

		currentTestNum++
	}
}
