// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
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

// TestIsPATFO ensures token-prefixed outputs created at or below the Upgrade9
// fork height are unspendable and those created after it are not.
func TestIsPATFO(t *testing.T) {
	const forkHeight = 792772
	token := wire.TokenData{CategoryID: [32]byte{1}, Amount: 1, BitField: wire.HAS_AMOUNT}
	p2pkh := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG}
	unparsed := []byte{wire.PREFIX_BYTE, txscript.OP_TRUE}

	tests := []struct {
		name      string
		tokenData wire.TokenData
		pkScript  []byte
		height    int32
		want      bool
	}{
		{"token before fork height", token, p2pkh, forkHeight - 1, true},
		{"token at fork height", token, p2pkh, forkHeight, true},
		{"token after fork height", token, p2pkh, forkHeight + 1, false},
		{"prefix byte at fork height", wire.TokenData{}, unparsed, forkHeight, true},
		{"prefix byte after fork height", wire.TokenData{}, unparsed, forkHeight + 1, false},
		{"no token at fork height", wire.TokenData{}, p2pkh, forkHeight, false},
	}
	for _, test := range tests {
		got := IsPATFO(test.tokenData, test.pkScript, test.height, forkHeight)
		if got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}

// manyInputsTestFlags are the script flags of the many input tests.
const manyInputsTestFlags = txscript.StandardVerifyFlags |
	txscript.ScriptAllowCashTokens

// manyInputsTestCategory is the token category spent by manyInputsTestTx.
var manyInputsTestCategory = [32]byte{0xca}

// manyInputsTestTx returns a transaction spending numInputs outputs added to
// view, each holding one fungible token.  The script of each spent output
// checks the value and token amount of the output spent by the next input.
// The transaction creates tokenAmount tokens.
func manyInputsTestTx(view *UtxoViewpoint, numInputs int, tag byte,
	tokenAmount uint64) *wire.MsgTx {

	prevTx := wire.NewMsgTx(2)
	prevTx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{
		Hash: chainhash.Hash{tag}}})
	for i := range numInputs {
		next := int64((i + 1) % numInputs)
		script, err := txscript.NewScriptBuilder().
			AddInt64(next).AddOp(txscript.OP_UTXOVALUE).
			AddInt64(1000 + next).AddOp(txscript.OP_NUMEQUALVERIFY).
			AddInt64(next).AddOp(txscript.OP_UTXOTOKENAMOUNT).
			AddInt64(1).AddOp(txscript.OP_NUMEQUAL).Script()
		if err != nil {
			panic(err)
		}
		prevTx.AddTxOut(wire.NewTxOut(1000+int64(i), script,
			wire.TokenData{CategoryID: manyInputsTestCategory,
				BitField: wire.HAS_AMOUNT, Amount: 1}))
	}
	view.AddTxOuts(bchutil.NewTx(prevTx), 10)

	tx := wire.NewMsgTx(2)
	prevHash := prevTx.TxHash()
	for i := range numInputs {
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, uint32(i)), nil))
	}
	tx.AddTxOut(wire.NewTxOut(1000, []byte{txscript.OP_TRUE},
		wire.TokenData{CategoryID: manyInputsTestCategory,
			BitField: wire.HAS_AMOUNT, Amount: tokenAmount}))
	return tx
}

// scriptValTestBlock returns a block holding a coinbase and txs.
func scriptValTestBlock(txs ...*wire.MsgTx) *bchutil.Block {
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: math.MaxUint32},
		SignatureScript:  []byte{0x01, 0x01},
	})
	cb.AddTxOut(wire.NewTxOut(1e8, []byte{txscript.OP_TRUE},
		wire.TokenData{}))
	return bchutil.NewBlock(&wire.MsgBlock{
		Transactions: append([]*wire.MsgTx{cb}, txs...),
	})
}

// TestTxValidateItemsShareSpentOutputs ensures the inputs of a transaction
// share the outputs it spends and its sighashes instead of each looking them
// up or computing them again, also when the hash cache does not keep them,
// as when another transaction's entry evicts them.
func TestTxValidateItemsShareSpentOutputs(t *testing.T) {
	const numInputs = 50
	view := NewUtxoViewpoint()
	msgTx := manyInputsTestTx(view, numInputs, 1, numInputs)
	tx := bchutil.NewTx(msgTx)
	for n, hashCache := range []*txscript.HashCache{nil,
		txscript.NewHashCache(10), txscript.NewHashCache(0)} {

		items := txValidateItems(tx, view, manyInputsTestFlags, hashCache,
			new(uint32))
		if len(items) != numInputs {
			t.Fatalf("%d items, want %d", len(items), numInputs)
		}
		spent := items[0].spent
		want := txscript.NewTxSigHashes(msgTx).WithUtxos(msgTx, spent.utxos)
		if !reflect.DeepEqual(items[0].sigHashes, want) {
			t.Fatalf("cache %d: input 0 has the wrong sighashes, nil %v",
				n, items[0].sigHashes == nil)
		}
		for i, item := range items {
			if item.spent != spent {
				t.Fatalf("input %d has its own spent outputs", i)
			}
			if item.sigHashes != items[0].sigHashes {
				t.Fatalf("input %d has its own sighashes", i)
			}
		}
		if spent.missing != -1 {
			t.Fatalf("missing input %d", spent.missing)
		}
		for i, txIn := range tx.MsgTx().TxIn {
			utxo, err := spent.utxos.GetEntry(i)
			entry := view.LookupEntry(txIn.PreviousOutPoint)
			want := wire.NewTxOut(entry.Amount(), entry.PkScript(),
				entry.TokenData())
			if err != nil || !reflect.DeepEqual(&utxo, want) {
				t.Fatalf("spent output of input %d is %v, %v", i,
					utxo, err)
			}
		}
	}
}

// TestValidateManyInputs ensures the script of each input of a transaction
// with many inputs sees the outputs spent by the other inputs, and that the
// token validity algorithm checks the tokens of all of them.
func TestValidateManyInputs(t *testing.T) {
	const numInputs = 60
	for _, tokenAmount := range []uint64{numInputs, numInputs + 1} {
		view := NewUtxoViewpoint()
		msgTx := manyInputsTestTx(view, numInputs, 1, tokenAmount)
		tx := bchutil.NewTx(msgTx)
		check := func(name string, err error) {
			t.Helper()
			if tokenAmount == numInputs {
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				return
			}
			requireRuleError(t, err, ErrCashTokensValidation)
			want := fmt.Sprintf("transaction %s fails the token "+
				"validity algorithm", tx.Hash())
			if !strings.HasPrefix(err.Error(), want) {
				t.Fatalf("%s: got %v, want %s", name, err, want)
			}
		}
		for _, hashCache := range []*txscript.HashCache{nil,
			txscript.NewHashCache(10)} {

			_, err := ValidateTransactionScripts(tx, view,
				manyInputsTestFlags, nil, hashCache, 0)
			check("ValidateTransactionScripts", err)
		}
		err := checkBlockScripts(scriptValTestBlock(msgTx), view,
			manyInputsTestFlags, nil, txscript.NewHashCache(10), 0, 0)
		check("checkBlockScripts", err)
	}
}

// TestValidateMissingInput ensures a transaction spending an output missing
// from the view is rejected with ErrMissingTxOut naming that input.
func TestValidateMissingInput(t *testing.T) {
	view := NewUtxoViewpoint()
	msgTx := manyInputsTestTx(view, 3, 1, 3)
	missing := msgTx.TxIn[1].PreviousOutPoint
	view.RemoveEntry(missing)
	tx := bchutil.NewTx(msgTx)
	want := ruleError(ErrMissingTxOut, fmt.Sprintf("unable to find "+
		"unspent output %v referenced from transaction %s:1", missing,
		tx.Hash()))

	_, err := ValidateTransactionScripts(tx, view, manyInputsTestFlags, nil,
		nil, 0)
	if !reflect.DeepEqual(err, want) {
		t.Fatalf("ValidateTransactionScripts: got %v, want %v", err, want)
	}
	err = checkBlockScripts(scriptValTestBlock(msgTx), view,
		manyInputsTestFlags, nil, nil, 0, 0)
	if !reflect.DeepEqual(err, want) {
		t.Fatalf("checkBlockScripts: got %v, want %v", err, want)
	}
	err = ValidateTransactionInputScript(tx, 1, view, manyInputsTestFlags,
		nil, nil)
	if !reflect.DeepEqual(err, want) {
		t.Fatalf("ValidateTransactionInputScript: got %v, want %v", err,
			want)
	}
}

// TestValidateScriptFailureInput ensures the rule error of an input script
// that fails holds the index of the input and the script engine's error, and
// that ValidateTransactionInputScript runs that input alone, with the flags
// it is given.
func TestValidateScriptFailureInput(t *testing.T) {
	prevTx := wire.NewMsgTx(2)
	prevTx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{
		Hash: chainhash.Hash{4}}})
	for range 4 {
		prevTx.AddTxOut(wire.NewTxOut(1000, []byte{txscript.OP_TRUE},
			wire.TokenData{}))
	}
	view := NewUtxoViewpoint()
	view.AddTxOuts(bchutil.NewTx(prevTx), 10)
	prevHash := prevTx.TxHash()
	spend := func(first uint32, sigScript []byte) *bchutil.Tx {
		msgTx := wire.NewMsgTx(2)
		msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, first), nil))
		msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, first+1),
			sigScript))
		msgTx.AddTxOut(wire.NewTxOut(1500, []byte{txscript.OP_TRUE},
			wire.TokenData{}))
		return bchutil.NewTx(msgTx)
	}

	// The second input of the first transaction leaves an item too many
	// on the stack, and that of the second has a signature script that
	// is not push only.
	tests := []struct {
		tx        *bchutil.Tx
		code      ErrorCode
		scriptErr txscript.ErrorCode
		flag      txscript.ScriptFlags
	}{
		{spend(0, []byte{txscript.OP_TRUE}), ErrScriptValidation,
			txscript.ErrCleanStack, txscript.ScriptVerifyCleanStack},
		{spend(2, []byte{txscript.OP_TRUE, txscript.OP_DROP}),
			ErrScriptMalformed, txscript.ErrNotPushOnly,
			txscript.ScriptVerifySigPushOnly},
	}
	flags := txscript.StandardVerifyFlags
	for _, test := range tests {
		check := func(name string, err error) {
			t.Helper()
			rerr, ok := err.(RuleError)
			if !ok || rerr.ErrorCode != test.code || rerr.InputIndex != 1 ||
				!txscript.IsErrorCode(rerr.ScriptErr, test.scriptErr) {

				t.Fatalf("%s: got %#v, want %v at input 1 with %v", name,
					err, test.code, test.scriptErr)
			}
		}
		_, want := ValidateTransactionScripts(test.tx, view, flags, nil,
			nil, 0)
		check("ValidateTransactionScripts", want)
		err := checkBlockScripts(scriptValTestBlock(test.tx.MsgTx()), view,
			flags, nil, nil, 0, 0)
		check("checkBlockScripts", err)

		err = ValidateTransactionInputScript(test.tx, 1, view, flags, nil,
			nil)
		if !reflect.DeepEqual(err, want) {
			t.Fatalf("input 1: got %v, want %v", err, want)
		}
		err = ValidateTransactionInputScript(test.tx, 1, view,
			flags&^test.flag, nil, nil)
		if err != nil {
			t.Fatalf("input 1 without %v: %v", test.scriptErr, err)
		}
		err = ValidateTransactionInputScript(test.tx, 0, view, flags, nil,
			nil)
		if err != nil {
			t.Fatalf("input 0: %v", err)
		}
		err = ValidateTransactionInputScript(test.tx, 2, view, flags, nil,
			nil)
		if _, ok := err.(AssertError); !ok {
			t.Fatalf("input 2: got %v, want an assertion error", err)
		}
	}
}

// TestCheckBlockScriptsFailingTx ensures a block in which one transaction
// fails is rejected with the error of that transaction, whatever its
// position in the block.
func TestCheckBlockScriptsFailingTx(t *testing.T) {
	const numInputs = 40
	view := NewUtxoViewpoint()
	good := manyInputsTestTx(view, numInputs, 1, numInputs)
	badTokens := manyInputsTestTx(view, numInputs, 2, numInputs+1)

	// A transaction whose only input fails its script.
	prevTx := wire.NewMsgTx(2)
	prevTx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{
		Hash: chainhash.Hash{3}}})
	prevTx.AddTxOut(wire.NewTxOut(1000, []byte{txscript.OP_FALSE},
		wire.TokenData{}))
	view.AddTxOuts(bchutil.NewTx(prevTx), 10)
	badScript := wire.NewMsgTx(2)
	prevHash := prevTx.TxHash()
	badScript.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, 0), nil))
	badScript.AddTxOut(wire.NewTxOut(900, []byte{txscript.OP_TRUE},
		wire.TokenData{}))

	tests := []struct {
		bad  *wire.MsgTx
		code ErrorCode
	}{
		{badTokens, ErrCashTokensValidation},
		{badScript, ErrScriptValidation},
	}
	for _, test := range tests {
		badHash := test.bad.TxHash()
		for _, txs := range [][]*wire.MsgTx{{good, test.bad},
			{test.bad, good}} {

			err := checkBlockScripts(scriptValTestBlock(txs...), view,
				manyInputsTestFlags, nil, txscript.NewHashCache(10),
				0, 0)
			requireRuleError(t, err, test.code)
			if !strings.Contains(err.Error(), badHash.String()) {
				t.Fatalf("error %v doesn't name %v", err, badHash)
			}
		}
	}
}

// TestTokenValidityFailureManyInputs ensures a block whose transaction
// spends several token outputs and creates more tokens than they hold is
// rejected and marked invalid, while spending the same amount is accepted.
func TestTokenValidityFailureManyInputs(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.MagneticAnonomalyForkHeight = 0
	params.Upgrade9ForkHeight = 0
	m := newRuleTestMiner(t, &params)

	b1 := m.build(nil)
	m.mustAccept(b1)
	cb := b1.MsgBlock().Transactions[0]

	// Create a category from the coinbase and split it over outputs.
	const numOutputs = 8
	category := cb.TxHash()
	split := ruleTestSpend(wire.OutPoint{Hash: category}, 1000, 1)
	split.TxOut = split.TxOut[1:]
	for range numOutputs {
		split.AddTxOut(wire.NewTxOut(1000, []byte{txscript.OP_TRUE},
			wire.TokenData{CategoryID: category,
				BitField: wire.HAS_AMOUNT, Amount: 10}))
	}
	m.mustAccept(m.build(nil, split))

	spend := func(amount uint64) *wire.MsgTx {
		tx := wire.NewMsgTx(2)
		splitHash := split.TxHash()
		for i := range numOutputs {
			tx.AddTxIn(&wire.TxIn{
				PreviousOutPoint: wire.OutPoint{Hash: splitHash,
					Index: uint32(i + 1)},
				Sequence: math.MaxUint32,
			})
		}
		tx.AddTxOut(wire.NewTxOut(numOutputs*1000-1000,
			[]byte{txscript.OP_TRUE}, wire.TokenData{
				CategoryID: category, BitField: wire.HAS_AMOUNT,
				Amount: amount}))
		tx.AddTxOut(ruleTestPadding(2))
		return tx
	}

	block := m.build(nil, spend(numOutputs*10+1))
	err := m.accept(block)
	requireRuleError(t, err, ErrCashTokensValidation)
	known, _, invalid := m.chain.BlockValidity(block.Hash())
	if !known || !invalid {
		t.Fatalf("block known %v, invalid %v, want both", known, invalid)
	}
	m.mustAccept(m.build(nil, spend(numOutputs*10)))
}
