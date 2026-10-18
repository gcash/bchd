// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// procTestBlock builds a solved block on top of prev at height prevHeight+1.
// The transactions are kept in the given order and the coinbase pays
// cbValue.
func procTestBlock(t *testing.T, prev *wire.MsgBlock, prevHeight int32,
	cbValue int64, bits uint32, txs []*wire.MsgTx) *bchutil.Block {

	t.Helper()
	height := prevHeight + 1
	cbScript, err := txscript.NewScriptBuilder().AddInt64(int64(height)).
		AddInt64(int64(prev.Header.Nonce)).Script()
	if err != nil {
		t.Fatal(err)
	}
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		Sequence:         wire.MaxTxInSequenceNum,
		SignatureScript:  cbScript,
	})
	cb.AddTxOut(wire.NewTxOut(cbValue, opTrueScript, wire.TokenData{}))
	cb.AddTxOut(wire.NewTxOut(0, procTestPad(nil), wire.TokenData{}))

	all := append([]*wire.MsgTx{cb}, txs...)
	utxs := make([]*bchutil.Tx, 0, len(all))
	for _, tx := range all {
		utxs = append(utxs, bchutil.NewTx(tx))
	}
	merkles := BuildMerkleTreeStore(utxs)
	ts := prev.Header.Timestamp.Add(time.Minute)
	if ts.Before(time.Unix(1700000000, 0)) {
		ts = time.Unix(1700000000, 0)
	}
	msgBlock := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    4,
			PrevBlock:  prev.BlockHash(),
			MerkleRoot: *merkles[len(merkles)-1],
			Timestamp:  ts,
			Bits:       bits,
		},
		Transactions: all,
	}
	if !solveBlock(&msgBlock.Header) {
		t.Fatal("unable to solve block")
	}
	return bchutil.NewBlock(msgBlock)
}

// procTestPad returns an OP_RETURN script pushing data padded to 60 bytes.
func procTestPad(data []byte) []byte {
	pad := make([]byte, 60)
	copy(pad, data)
	return append([]byte{txscript.OP_RETURN, txscript.OP_DATA_60}, pad...)
}

// procTestSpend returns a transaction spending prev into an OP_TRUE output
// padded with an OP_RETURN output tagged with tag.
func procTestSpend(prev wire.OutPoint, value int64, tag byte) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: prev, Sequence: wire.MaxTxInSequenceNum})
	tx.AddTxOut(wire.NewTxOut(value, opTrueScript, wire.TokenData{}))
	tx.AddTxOut(wire.NewTxOut(0, procTestPad([]byte{tag}), wire.TokenData{}))
	return tx
}

// TestOrphanContextChecks ensures a block that arrives before its parent is
// held to the same height-dependent rules as one that arrives in order.
func TestOrphanContextChecks(t *testing.T) {
	unordered := func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx {
		a, b := procTestSpend(o1, v, 1), procTestSpend(o2, v, 2)
		ha, hb := a.TxHash(), b.TxHash()
		if ha.Compare(&hb) < 0 {
			return []*wire.MsgTx{b, a}
		}
		return []*wire.MsgTx{a, b}
	}
	ordered := func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx {
		txs := unordered(o1, o2, v)
		return []*wire.MsgTx{txs[1], txs[0]}
	}
	undersized := func(o1, _ wire.OutPoint, v int64) []*wire.MsgTx {
		tx := wire.NewMsgTx(2)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: o1, Sequence: wire.MaxTxInSequenceNum})
		tx.AddTxOut(wire.NewTxOut(v, opTrueScript, wire.TokenData{}))
		return []*wire.MsgTx{tx}
	}

	tests := []struct {
		name       string
		forkHeight int32
		txs        func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx
		wantErr    ErrorCode
		valid      bool
	}{
		{name: "not in CTOR order", txs: unordered, wantErr: ErrInvalidTxOrder},
		{name: "not in CTOR order, first block after activation",
			forkHeight: 2, txs: unordered, wantErr: ErrInvalidTxOrder},
		{name: "undersized tx", txs: undersized, wantErr: ErrTxTooSmall},
		{name: "undersized tx, first block after activation",
			forkHeight: 2, txs: undersized, wantErr: ErrTxTooSmall},
		{name: "in CTOR order", txs: ordered, valid: true},
		{name: "not in CTOR order before activation", forkHeight: 3,
			txs: unordered, valid: true},
		{name: "undersized tx before activation", forkHeight: 3,
			txs: undersized, valid: true},
	}

	modes := []struct {
		name     string
		asOrphan bool
		flags    BehaviorFlags
	}{
		{"in order", false, BFNone},
		{"orphan", true, BFNone},
		{"unrequested orphan", true, BFUnrequested},
	}
	for _, test := range tests {
		for _, mode := range modes {
			name := fmt.Sprintf("%s/%s", test.name, mode.name)
			t.Run(name, func(t *testing.T) {
				testOrphanContext(t, test.forkHeight, test.txs,
					mode.asOrphan, mode.flags, test.valid,
					test.wantErr)
			})
		}
	}
}

// testOrphanContext connects a block spending two coinbases with the given
// transactions at height 3, before or after its parent, processing it with
// the given flags.
func testOrphanContext(t *testing.T, forkHeight int32,
	txs func(o1, o2 wire.OutPoint, v int64) []*wire.MsgTx, asOrphan bool,
	flags BehaviorFlags, valid bool, wantErr ErrorCode) {

	params := chaincfg.RegressionNetParams
	params.MagneticAnonomalyForkHeight = forkHeight
	params.Upgrade9ForkHeight = forkHeight
	chain, teardown, err := chainSetup("orphanctx", &params)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	chain.TstSetCoinbaseMaturity(1)

	bits := params.PowLimitBits
	subsidy := CalcBlockSubsidy(1, &params)
	b1 := procTestBlock(t, params.GenesisBlock, 0, subsidy, bits, nil)
	if _, _, err := chain.ProcessBlock(b1, BFNone); err != nil {
		t.Fatalf("block 1: %v", err)
	}
	b2 := procTestBlock(t, b1.MsgBlock(), 1, subsidy, bits, nil)
	o1 := wire.OutPoint{Hash: b1.MsgBlock().Transactions[0].TxHash()}
	o2 := wire.OutPoint{Hash: b2.MsgBlock().Transactions[0].TxHash()}
	b3 := procTestBlock(t, b2.MsgBlock(), 2, subsidy, bits,
		txs(o1, o2, subsidy-1000))

	if asOrphan {
		_, isOrphan, err := chain.ProcessBlock(b3, flags)
		if err != nil || !isOrphan {
			t.Fatalf("orphan: orphan=%v err=%v", isOrphan, err)
		}
	}

	// The parent is accepted even when its orphan breaks a rule.
	if _, _, err := chain.ProcessBlock(b2, BFNone); err != nil {
		t.Fatalf("parent: %v", err)
	}

	// A block that breaks a rule is rejected when it arrives in order, and
	// when it is submitted again after being dropped as an orphan.
	if !asOrphan || !valid {
		_, _, err = chain.ProcessBlock(b3, flags)
	}
	wantHeight := int32(3)
	if valid {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	} else {
		var rerr RuleError
		if !errors.As(err, &rerr) || rerr.ErrorCode != wantErr {
			t.Fatalf("got error %v, want %v", err, wantErr)
		}
		wantHeight = 2
	}
	if h := chain.BestSnapshot().Height; h != wantHeight {
		t.Fatalf("tip height %d, want %d", h, wantHeight)
	}
}

// TestOrphanNoFastAddInheritance ensures an orphan does not inherit BFFastAdd
// from the parent that made it connectable.
func TestOrphanNoFastAddInheritance(t *testing.T) {
	params := chaincfg.RegressionNetParams
	chain, teardown, err := chainSetup("orphanfastadd", &params)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	subsidy := CalcBlockSubsidy(1, &params)
	parent := procTestBlock(t, params.GenesisBlock, 0, subsidy,
		params.PowLimitBits, nil)

	// The child pays too much in its coinbase and uses the wrong
	// difficulty, which are only checked without BFFastAdd.
	child := procTestBlock(t, parent.MsgBlock(), 1, subsidy*1000,
		0x207ffffe, nil)

	_, isOrphan, err := chain.ProcessBlock(child, BFNone)
	if err != nil || !isOrphan {
		t.Fatalf("child not held as orphan: orphan=%v err=%v", isOrphan, err)
	}
	// The child is dropped without failing the parent.
	if _, _, err := chain.ProcessBlock(parent, BFFastAdd); err != nil {
		t.Fatalf("parent: %v", err)
	}
	best := chain.BestSnapshot()
	if best.Hash != *parent.Hash() {
		t.Fatalf("tip is %v at height %d, want the parent %v",
			best.Hash, best.Height, parent.Hash())
	}
	if chain.index.LookupNode(child.Hash()) != nil {
		t.Fatal("invalid orphan was added to the block index")
	}

	// Submitted again, the child is rejected.
	_, _, err = chain.ProcessBlock(child, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) {
		t.Fatalf("child not rejected: %v", err)
	}
}

// TestOrphanSiblingAfterFailedOrphan ensures an orphan that fails to connect
// neither fails its parent nor stops the parent's other orphans from
// connecting.
func TestOrphanSiblingAfterFailedOrphan(t *testing.T) {
	params := chaincfg.RegressionNetParams
	chain, teardown, err := chainSetup("orphansibling", &params)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	bits := params.PowLimitBits
	subsidy := CalcBlockSubsidy(1, &params)
	b1 := procTestBlock(t, params.GenesisBlock, 0, subsidy, bits, nil)
	if _, _, err := chain.ProcessBlock(b1, BFNone); err != nil {
		t.Fatalf("block 1: %v", err)
	}
	parent := procTestBlock(t, b1.MsgBlock(), 1, subsidy, bits, nil)

	// The first child pays too much in its coinbase, which is only found
	// when it is connected, and has a child of its own.  The second child
	// is valid.  The orphans of a block are processed in the order they
	// were added, so the invalid child is connected first.
	bad := procTestBlock(t, parent.MsgBlock(), 2, subsidy*1000, bits, nil)
	badChild := procTestBlock(t, bad.MsgBlock(), 3, subsidy, bits, nil)
	good := procTestBlock(t, parent.MsgBlock(), 2, subsidy-1, bits, nil)
	for _, block := range []*bchutil.Block{bad, badChild, good} {
		_, isOrphan, err := chain.ProcessBlock(block, BFNone)
		if err != nil || !isOrphan {
			t.Fatalf("block %v: orphan=%v err=%v", block.Hash(),
				isOrphan, err)
		}
	}

	if _, _, err := chain.ProcessBlock(parent, BFNone); err != nil {
		t.Fatalf("parent: %v", err)
	}
	best := chain.BestSnapshot()
	if best.Hash != *good.Hash() {
		t.Fatalf("tip is %v at height %d, want the valid child %v",
			best.Hash, best.Height, good.Hash())
	}
	for _, block := range []*bchutil.Block{bad, badChild, good} {
		if chain.IsKnownOrphan(block.Hash()) {
			t.Fatalf("block %v left in the orphan pool", block.Hash())
		}
	}
	if !chain.knownInvalid(bad.Hash()) {
		t.Fatal("invalid child not marked invalid")
	}
	if chain.index.LookupNode(badChild.Hash()) != nil {
		t.Fatal("child of the invalid block was added to the block index")
	}
}

// TestOrphanFailsSideChainParent ensures a parent accepted to a side chain is
// still rejected when the orphan that makes its chain the best one shows it
// to be invalid.
func TestOrphanFailsSideChainParent(t *testing.T) {
	params := chaincfg.RegressionNetParams
	chain, teardown, err := chainSetup("orphansideparent", &params)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	bits := params.PowLimitBits
	subsidy := CalcBlockSubsidy(1, &params)
	b1 := procTestBlock(t, params.GenesisBlock, 0, subsidy, bits, nil)
	b2 := procTestBlock(t, b1.MsgBlock(), 1, subsidy, bits, nil)
	for _, block := range []*bchutil.Block{b1, b2} {
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("block %v: %v", block.Hash(), err)
		}
	}

	// The parent forks from b1 and pays too much in its coinbase, which
	// is only found when its chain becomes the best one.  Its child makes
	// it so.
	parent := procTestBlock(t, b1.MsgBlock(), 1, subsidy*1000, bits, nil)
	child := procTestBlock(t, parent.MsgBlock(), 2, subsidy, bits, nil)
	_, isOrphan, err := chain.ProcessBlock(child, BFNone)
	if err != nil || !isOrphan {
		t.Fatalf("child not held as orphan: orphan=%v err=%v", isOrphan, err)
	}

	_, _, err = chain.ProcessBlock(parent, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != ErrBadCoinbaseValue {
		t.Fatalf("parent: got error %v, want %v", err, ErrBadCoinbaseValue)
	}
	if !chain.knownInvalid(parent.Hash()) {
		t.Fatal("parent not marked invalid")
	}
	if chain.IsKnownOrphan(child.Hash()) {
		t.Fatal("child left in the orphan pool")
	}
	if best := chain.BestSnapshot(); best.Hash != *b2.Hash() {
		t.Fatalf("tip is %v at height %d, want b2 %v", best.Hash,
			best.Height, b2.Hash())
	}
}

// orphanTestBlock returns a solved regtest block of exactly size bytes whose
// parent is unknown, padding the coinbase with an OP_RETURN output.
func orphanTestBlock(t *testing.T, params *chaincfg.Params, nonce uint32,
	size int, extra ...*wire.MsgTx) *bchutil.Block {

	t.Helper()
	cbScript, err := txscript.NewScriptBuilder().AddInt64(5).
		AddInt64(int64(nonce)).Script()
	if err != nil {
		t.Fatal(err)
	}
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		Sequence:         wire.MaxTxInSequenceNum,
		SignatureScript:  cbScript,
	})
	cb.AddTxOut(wire.NewTxOut(CalcBlockSubsidy(5, params), opTrueScript,
		wire.TokenData{}))
	pad := wire.NewTxOut(0, []byte{txscript.OP_RETURN}, wire.TokenData{})
	cb.AddTxOut(pad)

	msgBlock := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: chainhash.Hash{0x01, byte(nonce), byte(nonce >> 8)},
			Timestamp: params.GenesisBlock.Header.Timestamp.Add(time.Hour),
			Bits:      params.PowLimitBits,
		},
		Transactions: append([]*wire.MsgTx{cb}, extra...),
	}
	for i := 0; msgBlock.SerializeSize() != size; i++ {
		diff := size - msgBlock.SerializeSize()
		if i > 3 || len(pad.PkScript)+diff < 1 {
			t.Fatalf("can't pad block to %d bytes", size)
		}
		pad.PkScript = append([]byte{txscript.OP_RETURN},
			make([]byte, len(pad.PkScript)-1+diff)...)
	}

	utxs := make([]*bchutil.Tx, 0, len(msgBlock.Transactions))
	for _, tx := range msgBlock.Transactions {
		utxs = append(utxs, bchutil.NewTx(tx))
	}
	merkles := BuildMerkleTreeStore(utxs)
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	if !solveBlock(&msgBlock.Header) {
		t.Fatal("unable to solve block")
	}
	return bchutil.NewBlock(msgBlock)
}

// TestOrphanPoolLimits ensures orphans too large for any nearby height are
// rejected and that the orphan pool is limited by its total size.
func TestOrphanPoolLimits(t *testing.T) {
	// A fixed 1 MB ABLA limit keeps the size limit at the excessive block
	// size.
	params := chaincfg.RegressionNetParams
	params.ABLAConfig.Epsilon0 = 500000
	params.ABLAConfig.Beta0 = 500000
	params.ABLAConfig.FixedSize = true
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	chain, err := New(&Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(1000),
		ExcessiveBlockSize: LegacyMaxBlockSize,
	})
	if err != nil {
		t.Fatal(err)
	}

	limit := chain.orphanSizeLimit()
	if limit != LegacyMaxBlockSize {
		t.Fatalf("orphan size limit %d, want %d", limit, LegacyMaxBlockSize)
	}
	spend := wire.NewMsgTx(2)
	spend.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1},
		Sequence: wire.MaxTxInSequenceNum})
	spend.AddTxOut(wire.NewTxOut(0, append([]byte{txscript.OP_RETURN},
		make([]byte, 200000)...), wire.TokenData{}))
	oversized := orphanTestBlock(t, &params, 0, int(limit)+1, spend)
	_, isOrphan, err := chain.ProcessBlock(oversized, BFNone)
	var rerr RuleError
	if isOrphan || !errors.As(err, &rerr) || rerr.ErrorCode != ErrBlockTooBig {
		t.Fatalf("orphan over the size limit %d: orphan=%v err=%v", limit,
			isOrphan, err)
	}

	const size = 999000
	for i := uint32(1); i <= 70; i++ {
		_, isOrphan, err := chain.ProcessBlock(orphanTestBlock(t, &params,
			i, size), BFNone)
		if err != nil || !isOrphan {
			t.Fatalf("orphan %d: orphan=%v err=%v", i, isOrphan, err)
		}
	}
	if n := len(chain.orphans); n != minOrphanBytesLimit/size {
		t.Fatalf("%d orphans of %d bytes in the pool, want %d", n, size,
			minOrphanBytesLimit/size)
	}
	if chain.orphanBytes != uint64(len(chain.orphans)*size) {
		t.Fatalf("orphan bytes %d, want %d", chain.orphanBytes,
			len(chain.orphans)*size)
	}
}

// TestUtxoReadErrorNotInvalid ensures a corrupt utxo entry read while
// connecting a block is returned as a database corruption error without
// marking the block invalid, so the block connects once the entry is fixed.
func TestUtxoReadErrorNotInvalid(t *testing.T) {
	tests := []struct {
		name    string
		corrupt []byte
	}{
		{name: "undecodable entry", corrupt: []byte{0x02}},
		{name: "zero-length entry", corrupt: []byte{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testUtxoReadError(t, test.corrupt)
		})
	}
}

// testUtxoReadError connects a block spending an output whose stored entry
// is replaced with corrupt, then restores the entry and connects a child.
func testUtxoReadError(t *testing.T, corrupt []byte) {
	params := chaincfg.RegressionNetParams
	chain, teardown, err := chainSetup("utxoreaderr", &params)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	chain.TstSetCoinbaseMaturity(1)

	bits := params.PowLimitBits
	subsidy := CalcBlockSubsidy(1, &params)
	b1 := procTestBlock(t, params.GenesisBlock, 0, subsidy, bits, nil)
	b2 := procTestBlock(t, b1.MsgBlock(), 1, subsidy, bits, nil)
	for _, b := range []*bchutil.Block{b1, b2} {
		if _, _, err := chain.ProcessBlock(b, BFNone); err != nil {
			t.Fatal(err)
		}
	}

	// Flush the cache so the spent output is read from the database, then
	// corrupt its stored entry.
	if err := chain.utxoCache.Flush(FlushRequired, chain.BestSnapshot()); err != nil {
		t.Fatal(err)
	}
	prevOut := wire.OutPoint{Hash: b1.MsgBlock().Transactions[0].TxHash()}
	key := outpointKey(prevOut)
	defer recycleOutpointKey(key)
	var stored []byte
	setEntry := func(serialized []byte) {
		err := chain.db.Update(func(dbTx database.Tx) error {
			bucket := dbTx.Metadata().Bucket(utxoSetBucketName)
			if stored == nil {
				stored = append([]byte(nil), bucket.Get(*key)...)
			}
			return bucket.Put(*key, serialized)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	setEntry(corrupt)
	if len(stored) == 0 {
		t.Fatal("spent output was not flushed to the database")
	}

	// The error is checked with a type assertion, as netsync does.
	b3 := procTestBlock(t, b2.MsgBlock(), 2, subsidy, bits,
		[]*wire.MsgTx{procTestSpend(prevOut, subsidy-1000, 1)})
	_, _, err = chain.ProcessBlock(b3, BFNone)
	dbErr, ok := err.(database.Error)
	if !ok || dbErr.ErrorCode != database.ErrCorruption {
		t.Fatalf("got error %v, want a database corruption error", err)
	}
	if chain.index.NodeStatus(chain.index.LookupNode(b3.Hash())).KnownInvalid() {
		t.Fatal("block marked invalid after a failed utxo read")
	}

	// With the entry restored, the block connects along with its child.
	setEntry(stored)
	b4 := procTestBlock(t, b3.MsgBlock(), 3, subsidy, bits, nil)
	if _, _, err := chain.ProcessBlock(b4, BFNone); err != nil {
		t.Fatal(err)
	}
	if best := chain.BestSnapshot(); best.Hash != *b4.Hash() {
		t.Fatalf("tip is %v at height %d, want %v", best.Hash,
			best.Height, b4.Hash())
	}
}
