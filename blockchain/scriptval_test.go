// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestCheckBlockScripts ensures that validating the all of the scripts in a
// known-good block doesn't return an error.
func TestCheckBlockScripts(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	testBlockNum := 277647
	blockDataFile := fmt.Sprintf("%d.dat.bz2", testBlockNum)
	blocks, err := loadBlocks(blockDataFile)
	if err != nil {
		t.Errorf("Error loading file: %v\n", err)
		return
	}
	if len(blocks) > 1 {
		t.Errorf("The test block file must only have one block in it")
		return
	}
	if len(blocks) == 0 {
		t.Errorf("The test block file may not be empty")
		return
	}

	storeDataFile := fmt.Sprintf("%d.utxostore.bz2", testBlockNum)
	view, err := loadUtxoView(storeDataFile)
	if err != nil {
		t.Errorf("Error loading txstore: %v\n", err)
		return
	}

	scriptFlags := txscript.ScriptBip16
	err = checkBlockScripts(blocks[0], view, scriptFlags, nil, nil, 0, 0)
	if err != nil {
		t.Errorf("Transaction script validation failed: %v\n", err)
		return
	}
}

// TestValidateTransactionScriptsCachedSigHashes ensures validating a
// transaction with tokens enabled leaves its sighashes in the hash cache as
// they were.
func TestValidateTransactionScriptsCachedSigHashes(t *testing.T) {
	prevTx := wire.NewMsgTx(1)
	prevTx.AddTxOut(wire.NewTxOut(1e8, []byte{txscript.OP_TRUE},
		wire.TokenData{}))
	msgTx := wire.NewMsgTx(2)
	prevHash := prevTx.TxHash()
	msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, 0), nil))
	msgTx.AddTxOut(wire.NewTxOut(1e8-1000, []byte{txscript.OP_TRUE},
		wire.TokenData{}))
	tx := bchutil.NewTx(msgTx)

	view := NewUtxoViewpoint()
	view.AddTxOuts(bchutil.NewTx(prevTx), 1)
	hashCache := txscript.NewHashCache(10)
	flags := txscript.StandardVerifyFlags | txscript.ScriptAllowCashTokens
	for range 2 {
		_, err := ValidateTransactionScripts(tx, view, flags, nil,
			hashCache, 0)
		if err != nil {
			t.Fatal(err)
		}
	}

	sigHashes, ok := hashCache.GetSigHashes(tx.Hash())
	if !ok {
		t.Fatal("sighashes not cached")
	}
	if !reflect.DeepEqual(sigHashes, txscript.NewTxSigHashes(msgTx)) {
		t.Fatal("validation modified the cached sighashes")
	}
}
