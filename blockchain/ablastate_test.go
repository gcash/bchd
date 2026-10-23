// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// ablaTestParams returns regtest parameters with ABLA active from genesis and
// a 1 MB initial limit, so that full blocks of about 1 MB grow it.
func ablaTestParams() *chaincfg.Params {
	params := chaincfg.RegressionNetParams
	params.ABLAForkHeight = 0
	params.ABLAConfig = chaincfg.ABLAConstants{
		Epsilon0:        500000,
		Beta0:           500000,
		GammaReciprocal: 37938,
		ZetaXB7:         192,
		ThetaReciprocal: 37938,
		Delta:           10,
	}
	return &params
}

// ablaTestChain opens a chain on db.
func ablaTestChain(t *testing.T, db database.DB, params *chaincfg.Params) *BlockChain {
	t.Helper()
	chain, err := New(&Config{
		DB:                 db,
		ChainParams:        params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(1000),
		UtxoCacheMaxSize:   1 << 20,
		ExcessiveBlockSize: 1000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	chain.TstSetCoinbaseMaturity(1)
	return chain
}

// ablaTestBlock builds a solved block of exactly size bytes on top of prev,
// padding the coinbase with an unexecuted OP_RETURN output.
func ablaTestBlock(t *testing.T, prev *wire.MsgBlock, height int32, size int,
	params *chaincfg.Params, extra ...*wire.MsgTx) *bchutil.Block {

	t.Helper()
	cbScript, err := txscript.NewScriptBuilder().AddInt64(int64(height)).
		AddInt64(int64(size)).Script()
	if err != nil {
		t.Fatal(err)
	}
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		Sequence:         wire.MaxTxInSequenceNum,
		SignatureScript:  cbScript,
	})
	cb.AddTxOut(wire.NewTxOut(CalcBlockSubsidy(height, params), opTrueScript,
		wire.TokenData{}))
	pad := wire.NewTxOut(0, []byte{txscript.OP_RETURN}, wire.TokenData{})
	cb.AddTxOut(pad)

	msgBlock := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prev.BlockHash(),
			Timestamp: prev.Header.Timestamp.Add(time.Second),
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

// ablaTestMine extends the chain with n blocks of the given size and returns
// them.
func ablaTestMine(t *testing.T, chain *BlockChain, n, size int) []*bchutil.Block {
	t.Helper()
	params := chain.chainParams
	prev := params.GenesisBlock
	var blocks []*bchutil.Block
	for h := int32(1); h <= int32(n); h++ {
		block := ablaTestBlock(t, prev, h, size, params)
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("block %d: %v", h, err)
		}
		blocks = append(blocks, block)
		prev = block.MsgBlock()
	}
	return blocks
}

// ablaExpectedState replays the ABLA algorithm over the block sizes, starting
// from the initial state at the genesis block.
func ablaExpectedState(chain *BlockChain, blocks []*bchutil.Block) ABLAState {
	cfg := &chain.ablaConfig
	genesisSize := uint64(chain.chainParams.GenesisBlock.SerializeSize())
	state := ABLAState{blockHeight: 0, controlBlockSize: cfg.epsilon0,
		elasticBufferSize: cfg.beta0}
	state = state.nextABLAState(cfg, genesisSize)
	for _, block := range blocks {
		state = state.nextABLAState(cfg, uint64(block.MsgBlock().SerializeSize()))
	}
	return state
}

// TestABLAStateDBRoundTrip ensures a stored ABLA state reads back unchanged.
func TestABLAStateDBRoundTrip(t *testing.T) {
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), wire.MainNet)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	in := ABLAState{blockHeight: 900001, controlBlockSize: 20000000,
		elasticBufferSize: 25000000}
	var out *ABLAState
	err = db.Update(func(dbTx database.Tx) error {
		_, err := dbTx.Metadata().CreateBucket(ablaStateBucketName)
		if err != nil {
			return err
		}
		if err := dbPutABLAStateIndex(dbTx, in, 900000); err != nil {
			return err
		}
		out, err = dbFetchAblaStateByHeight(dbTx, 900000)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if *out != in {
		t.Fatalf("ABLA state round trip: got %+v, want %+v", *out, in)
	}
}

// TestABLAStateRestart ensures the ABLA state, and so the next block's size
// limit, is the same after a restart, and that states written by earlier
// versions are rebuilt from the block sizes.
func TestABLAStateRestart(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 20, 999000)
	want := ablaExpectedState(chain, blocks)
	if chain.ablaState != want {
		t.Fatalf("ABLA state %+v, want %+v", chain.ablaState, want)
	}
	if want.controlBlockSize == chain.ablaConfig.epsilon0 {
		t.Fatal("test blocks did not grow the ABLA state")
	}
	wantLimit := chain.MaxBlockSize(true, true)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}

	check := func(desc string) {
		t.Helper()
		chain := ablaTestChain(t, db, params)
		if chain.ablaState != want {
			t.Fatalf("%s: ABLA state %+v, want %+v", desc,
				chain.ablaState, want)
		}
		if got := chain.MaxBlockSize(true, true); got != wantLimit {
			t.Fatalf("%s: next block size limit %d, want %d", desc,
				got, wantLimit)
		}
		err := db.View(func(dbTx database.Tx) error {
			for h := int32(0); h <= int32(len(blocks)); h++ {
				_, err := chain.dbFetchValidABLAState(dbTx, h)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s: %v", desc, err)
		}
	}
	check("restart")

	// Earlier versions wrote the elastic buffer size over the control
	// block size and left the rest zero.
	err = db.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(ablaStateBucketName)
		for h := int32(0); h <= int32(len(blocks)); h++ {
			var key [4]byte
			byteOrder.PutUint32(key[:], uint32(h))
			state, err := dbFetchAblaStateByHeight(dbTx, h)
			if err != nil {
				return err
			}
			var old [16]byte
			byteOrder.PutUint64(old[:], state.elasticBufferSize)
			if h == 7 {
				if err := bucket.Delete(key[:]); err != nil {
					return err
				}
				continue
			}
			if err := bucket.Put(key[:], old[:]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	check("rebuild")

	// A single bad state in the middle is rebuilt, and a stale state above
	// the tip is ignored.
	err = db.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(ablaStateBucketName)
		var key [4]byte
		byteOrder.PutUint32(key[:], 12)
		if err := bucket.Put(key[:], make([]byte, 16)); err != nil {
			return err
		}
		byteOrder.PutUint32(key[:], uint32(len(blocks)+5))
		return bucket.Put(key[:], []byte{1})
	})
	if err != nil {
		t.Fatal(err)
	}
	check("partial rebuild")
}

// TestABLASideChainLimit ensures a side chain block is checked against the
// size limit that follows from its own parent, not from the chain tip.
func TestABLASideChainLimit(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 20, 999000)
	tipLimit := chain.MaxBlockSize(true, true)
	sideState := ablaExpectedState(chain, blocks[:1])
	sideLimit := int(sideState.getBlockSizeLimit())
	if sideLimit >= int(tipLimit) {
		t.Fatalf("side limit %d not below tip limit %d", sideLimit, tipLimit)
	}

	// The side chain forks after block 1.  A transaction spending its
	// coinbase lets the block grow beyond the transaction size limit.
	parent := blocks[0].MsgBlock()
	cbOut := wire.OutPoint{Hash: parent.Transactions[0].TxHash()}
	spend := wire.NewMsgTx(2)
	spend.AddTxIn(&wire.TxIn{PreviousOutPoint: cbOut, Sequence: wire.MaxTxInSequenceNum})
	spend.AddTxOut(wire.NewTxOut(0, append([]byte{txscript.OP_RETURN},
		make([]byte, 100000)...), wire.TokenData{}))

	tooBig := ablaTestBlock(t, parent, 2, sideLimit+1, params, spend)
	_, _, err = chain.ProcessBlock(tooBig, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != ErrBlockTooBig {
		t.Fatalf("side block of %d bytes over its limit of %d: got %v, "+
			"want ErrBlockTooBig", sideLimit+1, sideLimit, err)
	}

	atLimit := ablaTestBlock(t, parent, 2, sideLimit, params, spend)
	if _, _, err := chain.ProcessBlock(atLimit, BFNone); err != nil {
		t.Fatalf("side block at its limit: %v", err)
	}
	got, err := chain.ablaStateAfter(chain.index.LookupNode(atLimit.Hash()))
	if err != nil {
		t.Fatal(err)
	}
	if want := sideState.nextABLAState(&chain.ablaConfig, uint64(sideLimit)); got != want {
		t.Fatalf("side chain ABLA state %+v, want %+v", got, want)
	}
}

// pruneBlockData makes the data of the stored block with the given hash
// unreadable, as pruning does.  Pruning deletes whole block files but keeps the
// location of every block, with its length, in ffldb's block index, so the
// location is moved to a block file that does not exist.
func pruneBlockData(t *testing.T, db database.DB, hash *chainhash.Hash) {
	t.Helper()
	err := db.Update(func(dbTx database.Tx) error {
		blockIdx := dbTx.Metadata().Bucket([]byte("ffldb-blockidx"))
		row := blockIdx.Get(hash[:])
		if len(row) != 12 {
			return fmt.Errorf("location of block %v is %x", hash, row)
		}

		// The location starts with the block file number.
		moved := bytes.Clone(row)
		binary.LittleEndian.PutUint32(moved[0:4], 999999)
		return blockIdx.Put(hash[:], moved)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestABLASideChainPrunedParent ensures a block whose parent is a side chain
// block with pruned data is checked against the size limit that follows from
// its parent and does not stop the node.  Bitcoin Cash Node keeps the ABLA
// state of every block in its block index, so it never reads the data of an
// ancestor for it (validation.cpp:4208, 5998-6008).
func TestABLASideChainPrunedParent(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 20, 999000)
	cfg := &chain.ablaConfig

	// spend returns a transaction spending the first output of the
	// coinbase of block, which lets a block grow beyond the transaction
	// size limit.
	spend := func(block *wire.MsgBlock) *wire.MsgTx {
		tx := wire.NewMsgTx(2)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: block.Transactions[0].TxHash()},
			Sequence:         wire.MaxTxInSequenceNum,
		})
		tx.AddTxOut(wire.NewTxOut(0, append([]byte{txscript.OP_RETURN},
			make([]byte, 100000)...), wire.TokenData{}))
		return tx
	}

	// The side chain forks after block 1 with a block at its limit, whose
	// data is then pruned.
	parent := blocks[0].MsgBlock()
	sideState := ablaExpectedState(chain, blocks[:1])
	sideSize := int(sideState.getBlockSizeLimit())
	side := ablaTestBlock(t, parent, 2, sideSize, params, spend(parent))
	if _, _, err := chain.ProcessBlock(side, BFNone); err != nil {
		t.Fatalf("side block: %v", err)
	}
	pruneBlockData(t, db, side.Hash())
	err = db.View(func(dbTx database.Tx) error {
		_, err := dbTx.FetchBlock(side.Hash())
		return err
	})
	var dbErr database.Error
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrDriverSpecific ||
		!errors.Is(dbErr.Err, os.ErrNotExist) {

		t.Fatalf("reading the pruned side block: got %v, want a missing "+
			"block file", err)
	}

	// The next block's limit follows from the size of the pruned block.
	nextState := sideState.nextABLAState(cfg, uint64(sideSize))
	nextLimit := int(max(1000000, nextState.getBlockSizeLimit()))
	tooBig := ablaTestBlock(t, side.MsgBlock(), 3, nextLimit+1, params,
		spend(side.MsgBlock()))
	_, _, err = chain.ProcessBlock(tooBig, BFNone)
	select {
	case <-chain.RequestedProcessShutdown():
		t.Fatalf("a block on a pruned side block requested a shutdown: %v",
			err)
	default:
	}
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != ErrBlockTooBig {
		t.Fatalf("block of %d bytes over its limit of %d: got %v, want "+
			"ErrBlockTooBig", nextLimit+1, nextLimit, err)
	}

	atLimit := ablaTestBlock(t, side.MsgBlock(), 3, nextLimit, params,
		spend(side.MsgBlock()))
	if _, _, err := chain.ProcessBlock(atLimit, BFNone); err != nil {
		t.Fatalf("block at its limit on the pruned side block: %v", err)
	}
	got, err := chain.ablaStateAfter(chain.index.LookupNode(atLimit.Hash()))
	if err != nil {
		t.Fatal(err)
	}
	if want := nextState.nextABLAState(cfg, uint64(nextLimit)); got != want {
		t.Fatalf("side chain ABLA state %+v, want %+v", got, want)
	}
}

// TestABLASideChainSizeError ensures a side chain block whose size the database
// can't give, here one without a location in ffldb's block index, fails the
// next block with the database error instead of being replayed without it.
func TestABLASideChainSizeError(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 3, 2000)
	side := ablaTestBlock(t, blocks[0].MsgBlock(), 2, 2001, params)
	if _, _, err := chain.ProcessBlock(side, BFNone); err != nil {
		t.Fatalf("side block: %v", err)
	}
	err = db.Update(func(dbTx database.Tx) error {
		blockIdx := dbTx.Metadata().Bucket([]byte("ffldb-blockidx"))
		return blockIdx.Delete(side.Hash()[:])
	})
	if err != nil {
		t.Fatal(err)
	}

	next := ablaTestBlock(t, side.MsgBlock(), 3, 2000, params)
	_, _, err = chain.ProcessBlock(next, BFNone)
	var dbErr database.Error
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrBlockNotFound {
		t.Fatalf("block on a side block without a location: got %v, "+
			"want ErrBlockNotFound", err)
	}
}

// writeOldABLAStates rewrites the stored states up to height n in the format
// of earlier versions, which kept only the elastic buffer size.
func writeOldABLAStates(t *testing.T, db database.DB, n int32) {
	t.Helper()
	err := db.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(ablaStateBucketName)
		for h := int32(0); h <= n; h++ {
			var key [4]byte
			byteOrder.PutUint32(key[:], uint32(h))
			state, err := dbFetchAblaStateByHeight(dbTx, h)
			if err != nil {
				return err
			}
			var old [16]byte
			byteOrder.PutUint64(old[:], state.elasticBufferSize)
			if err := bucket.Put(key[:], old[:]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestABLANetworkDefaults ensures the ABLA configuration comes from the chain
// parameters alone and that the floor of the limit is the network's default
// block size, which a larger excessive block size does not raise.
func TestABLANetworkDefaults(t *testing.T) {
	tests := []struct {
		params *chaincfg.Params
		eb     uint32
		limit  uint64
	}{
		{&chaincfg.MainNetParams, 32000000, 32000000},
		{&chaincfg.TestNet3Params, 32000000, 32000000},
		{&chaincfg.ChipNetParams, 32000000, 2000000},
		{&chaincfg.TestNet4Params, 32000000, 2000000},
		{&chaincfg.MainNetParams, 64000000, 32000000},
		{&chaincfg.ChipNetParams, 8000000, 2000000},
	}
	for _, test := range tests {
		db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
			test.params.Net)
		if err != nil {
			t.Fatal(err)
		}
		params := *test.params
		chain, err := New(&Config{
			DB:                 db,
			ChainParams:        &params,
			TimeSource:         NewMedianTime(),
			ExcessiveBlockSize: test.eb,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := chain.MaxBlockSize(true, true); got != test.limit {
			t.Errorf("%s EB %d: ABLA limit %d, want %d", params.Name,
				test.eb, got, test.limit)
		}
		floor := uint64(params.DefaultConsensusBlockSize)
		if got := chain.MaxBlockSize(true, false); got != floor {
			t.Errorf("%s EB %d: limit without ABLA %d, want %d",
				params.Name, test.eb, got, floor)
		}
		if chain.ablaConfig.epsilon0 != params.ABLAConfig.Epsilon0 ||
			chain.ablaConfig.beta0 != params.ABLAConfig.Beta0 {
			t.Errorf("%s EB %d: ABLA config %+v, want epsilon0 %d beta0 %d",
				params.Name, test.eb, chain.ablaConfig,
				params.ABLAConfig.Epsilon0, params.ABLAConfig.Beta0)
		}
		db.Close()
	}
}

// TestABLAConfigChange ensures all stored states are rebuilt when they were
// computed with another ABLA configuration, even though they are valid for
// the new one.
func TestABLAConfigChange(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 20, 999000)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}

	other := *params
	other.ABLAConfig.Epsilon0 = 400000
	other.ABLAConfig.Beta0 = 400000
	for _, p := range []*chaincfg.Params{&other, params, params} {
		chain := ablaTestChain(t, db, p)
		want := ablaExpectedState(chain, blocks)
		if chain.ablaState != want {
			t.Fatalf("epsilon0 %d: ABLA state %+v, want %+v",
				p.ABLAConfig.Epsilon0, chain.ablaState, want)
		}
		wantLimit := max(uint64(1000000), want.getBlockSizeLimit())
		if got := chain.MaxBlockSize(true, true); got != wantLimit {
			t.Fatalf("epsilon0 %d: limit %d, want %d",
				p.ABLAConfig.Epsilon0, got, wantLimit)
		}
		err := db.View(func(dbTx database.Tx) error {
			config := dbTx.Metadata().Get(ablaConfigKeyName)
			if !bytes.Equal(config, chain.serializeABLAConfig()) {
				t.Fatalf("stored ABLA config %x", config)
			}
			state, err := chain.dbFetchValidABLAState(dbTx, int32(len(blocks)))
			if err != nil || state != want {
				t.Fatalf("stored tip state %+v %v, want %+v", state,
					err, want)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestABLAPrunedState ensures the state of a pruned block is approximated
// from the old format only when the stored state is in that format.
func TestABLAPrunedState(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	chain := ablaTestChain(t, db, params)
	cfg := &chain.ablaConfig

	put := func(dbTx database.Tx, h int32, control, elastic uint64) error {
		var key [4]byte
		byteOrder.PutUint32(key[:], uint32(h))
		var value [16]byte
		byteOrder.PutUint64(value[0:8], control)
		byteOrder.PutUint64(value[8:16], elastic)
		return dbTx.Metadata().Bucket(ablaStateBucketName).Put(key[:], value[:])
	}
	base := func(h int32) ABLAState { return chain.ablaBaseState(h) }
	withElastic := func(h int32, elastic uint64) ABLAState {
		state := base(h)
		state.elasticBufferSize = elastic
		return state
	}
	valid := ABLAState{blockHeight: 5, controlBlockSize: cfg.epsilon0 + 1000,
		elasticBufferSize: cfg.beta0 + 2000}
	tests := []struct {
		height           int32
		control, elastic uint64
		want             ABLAState
	}{
		// The old format kept the elastic buffer size in the first field.
		{3, cfg.beta0 + 7000, 0, withElastic(3, cfg.beta0+7000)},
		{4, valid.controlBlockSize, valid.elasticBufferSize, valid},
		// Invalid in the current format, not read as the old one.
		{5, cfg.beta0 + 9000, 1, base(5)},
	}
	err = db.Update(func(dbTx database.Tx) error {
		for _, test := range tests {
			if err := put(dbTx, test.height, test.control, test.elastic); err != nil {
				return err
			}
		}
		for _, test := range tests {
			if got := chain.prunedABLAState(dbTx, test.height); got != test.want {
				t.Errorf("height %d: state %+v, want %+v", test.height,
					got, test.want)
			}
		}
		if got := chain.prunedABLAState(dbTx, 6); got != base(6) {
			t.Errorf("missing state: %+v, want %+v", got, base(6))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestABLARebuildBlockReadError ensures the rebuild on a pruned datadir only
// approximates the state of blocks whose data is missing, and reports other
// errors reading a block.
func TestABLARebuildBlockReadError(t *testing.T) {
	notExist := &os.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}
	for _, test := range []struct {
		err  error
		want bool
	}{
		{database.Error{ErrorCode: database.ErrBlockNotFound}, true},
		{database.Error{ErrorCode: database.ErrDriverSpecific, Err: notExist}, true},
		{database.Error{ErrorCode: database.ErrDriverSpecific, Err: errors.New("io")}, false},
		{database.Error{ErrorCode: database.ErrCorruption}, false},
		{errors.New("other"), false},
	} {
		if got := isBlockDataMissing(test.err); got != test.want {
			t.Errorf("isBlockDataMissing(%v) = %v, want %v", test.err,
				got, test.want)
		}
	}

	params := ablaTestParams()
	dir := filepath.Join(t.TempDir(), "db")
	db, err := database.Create("ffldb", dir, params.Net)
	if err != nil {
		t.Fatal(err)
	}
	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 10, 999000)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}
	writeOldABLAStates(t, db, int32(len(blocks)))
	err = db.Update(func(dbTx database.Tx) error {
		return dbPutBlockchainType(dbTx, prunedBlockchainEntryValue)
	})
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Corrupt the data of block 5.
	file := filepath.Join(dir, "000000000.fdb")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var header bytes.Buffer
	if err := blocks[4].MsgBlock().Header.Serialize(&header); err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(data, header.Bytes())
	if i < 0 {
		t.Fatal("block 5 not found in the block file")
	}
	data[i+1000] ^= 0xff
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open("ffldb", dir, params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = New(&Config{
		DB:                 db,
		ChainParams:        params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(1000),
		UtxoCacheMaxSize:   1 << 20,
		ExcessiveBlockSize: 1000000,
	})
	var dbErr database.Error
	if !errors.As(err, &dbErr) || dbErr.ErrorCode != database.ErrCorruption {
		t.Fatalf("start with a corrupt block: got %v, want ErrCorruption", err)
	}
}

// TestNetworkSizeFloor ensures block templates are built to the network's
// default block size rather than a larger excessive block size, except on
// regtest and simnet and for chain parameters without a default, and never to
// more than the excessive block size.
func TestNetworkSizeFloor(t *testing.T) {
	noDefault := chaincfg.MainNetParams
	noDefault.DefaultConsensusBlockSize = 0

	tests := []struct {
		name   string
		params *chaincfg.Params
		eb     uint32
		want   uint64
	}{
		{"mainnet", &chaincfg.MainNetParams, 32000000, 32000000},
		{"mainnet, raised", &chaincfg.MainNetParams, 64000000, 32000000},
		{"mainnet, below default", &chaincfg.MainNetParams, 1000000, 1000000},
		{"testnet4, raised", &chaincfg.TestNet4Params, 32000000, 2000000},
		{"chipnet", &chaincfg.ChipNetParams, 2000000, 2000000},
		{"regtest, raised", &chaincfg.RegressionNetParams, 64000000, 64000000},
		{"simnet, raised", &chaincfg.SimNetParams, 64000000, 64000000},
		{"no default", &noDefault, 64000000, 64000000},
	}
	for _, test := range tests {
		chain := newFakeChain(test.params)
		chain.excessiveBlockSize = test.eb
		if got := chain.networkSizeFloor(); got != test.want {
			t.Errorf("%s: got %d, want %d", test.name, got, test.want)
		}
	}
}
