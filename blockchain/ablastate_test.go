// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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

// TestABLARebuildBlockReadError ensures the rebuild of the stored states takes
// the size of each block from where the database stores it, as Bitcoin Cash
// Node reads only the size of a block to rebuild its state
// (node/blockstorage.cpp:520-533), so a block whose data is corrupt or pruned
// still gets its exact state, and the node starts.
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
	for _, test := range []struct {
		name    string
		pruned  bool
		corrupt bool
	}{
		{"corrupt block", false, true},
		{"corrupt block on a pruned datadir", true, true},
		{"pruned block", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
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
			if test.pruned {
				err = db.Update(func(dbTx database.Tx) error {
					return dbPutBlockchainType(dbTx,
						prunedBlockchainEntryValue)
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if !test.corrupt {
				pruneBlockData(t, db, blocks[4].Hash())
			}
			db.Close()

			// Corrupt the data of block 5.
			if test.corrupt {
				file := filepath.Join(dir, "000000000.fdb")
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var header bytes.Buffer
				err = blocks[4].MsgBlock().Header.Serialize(&header)
				if err != nil {
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
			}

			db, err = database.Open("ffldb", dir, params.Net)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			chain = ablaTestChain(t, db, params)
			if want := ablaExpectedState(chain, blocks); chain.ablaState != want {
				t.Fatalf("ABLA state %+v, want %+v", chain.ablaState, want)
			}
			err = db.View(func(dbTx database.Tx) error {
				for h := 0; h <= len(blocks); h++ {
					state, err := chain.dbFetchValidABLAState(dbTx, int32(h))
					if err != nil {
						return err
					}
					want := ablaExpectedState(chain, blocks[:h])
					if state != want {
						t.Errorf("height %d: ABLA state %+v, want %+v",
							h, state, want)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
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

// expectedLookAhead returns the size limit ABLA gives the block 2048 blocks
// after the chain's tip when every block in between is full, as BCHN's
// TipChanged computes its look-ahead guess.
func expectedLookAhead(chain *BlockChain) uint64 {
	tip := chain.bestChain.Tip().height
	state := chain.ablaState
	for height := tip + 1; height < tip+2048; height++ {
		state = chain.ablaNextState(state, height, state.getBlockSizeLimit())
	}
	return state.getBlockSizeLimit()
}

// TestMaxBlockSizeLookAhead ensures the chain reports the block size
// look-ahead to NotifyMaxBlockSizeLookAhead when it is loaded and whenever
// its tip changes.
func TestMaxBlockSizeLookAhead(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var notified []uint64
	open := func() *BlockChain {
		t.Helper()
		chain, err := New(&Config{
			DB:                 db,
			ChainParams:        params,
			TimeSource:         NewMedianTime(),
			SigCache:           txscript.NewSigCache(1000),
			UtxoCacheMaxSize:   1 << 20,
			ExcessiveBlockSize: 1000000,
			NotifyMaxBlockSizeLookAhead: func(size uint64) {
				notified = append(notified, size)
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return chain
	}
	check := func(desc string, chain *BlockChain, calls int) {
		t.Helper()
		if len(notified) != calls {
			t.Fatalf("%s: %d notifications, want %d", desc,
				len(notified), calls)
		}
		want := expectedLookAhead(chain)
		if got := notified[calls-1]; got != want {
			t.Fatalf("%s: look-ahead %d, want %d", desc, got, want)
		}
	}

	chain := open()
	check("loaded", chain, 1)
	atGenesis := notified[0]

	// Full blocks grow the look-ahead, and each one is reported.
	blocks := ablaTestMine(t, chain, 20, 999000)
	check("after 20 full blocks", chain, 21)
	if notified[20] <= atGenesis {
		t.Fatalf("look-ahead %d did not grow from %d", notified[20],
			atGenesis)
	}

	// Disconnecting the tip reports the look-ahead after its parent.
	if err := chain.InvalidateBlock(blocks[19].Hash()); err != nil {
		t.Fatal(err)
	}
	check("tip disconnected", chain, 22)
	if notified[21] >= notified[20] {
		t.Fatalf("look-ahead %d after the disconnect, want below %d",
			notified[21], notified[20])
	}

	// A restart reports the look-ahead of the stored tip.
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}
	chain = open()
	check("restart", chain, 23)
	if notified[22] != notified[21] {
		t.Fatalf("look-ahead %d after a restart, want %d", notified[22],
			notified[21])
	}
}

// blockMessageHeader returns the header of a block message on net that claims
// length bytes of payload.
func blockMessageHeader(net wire.BitcoinNet, length uint32) []byte {
	var hdr [wire.MessageHeaderSize]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(net))
	copy(hdr[4:16], wire.CmdBlock)
	binary.LittleEndian.PutUint32(hdr[16:20], length)
	return hdr[:]
}

// TestMaxBlockSizeLookAheadBlockMessages ensures that after a long run of full
// blocks, through which ABLA raises the block size limit past four times the
// excessive block size, wire's block message limit, kept up to date by the
// chain, still lets a block of the next block's size limit be received.  The
// limit was set once at startup, so such a block was refused at its header.
func TestMaxBlockSizeLookAheadBlockMessages(t *testing.T) {
	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(func() { wire.SetMaxBlockSizeLookAhead(0) })
	chain, err := New(&Config{
		DB:                          db,
		ChainParams:                 &params,
		TimeSource:                  NewMedianTime(),
		SigCache:                    txscript.NewSigCache(1000),
		UtxoCacheMaxSize:            1 << 20,
		ExcessiveBlockSize:          32000000,
		NotifyMaxBlockSizeLookAhead: wire.SetMaxBlockSizeLookAhead,
	})
	if err != nil {
		t.Fatal(err)
	}
	chain.TstSetCoinbaseMaturity(1)
	m := &ruleTestMiner{t: t, chain: chain, tip: params.GenesisBlock}
	m.mustAccept(m.build(nil))

	// Stand in for a long run of full blocks by advancing the state after
	// the tip, until the next block's limit is past wire's block message
	// limit, plus a thousand blocks.
	chain.chainLock.Lock()
	state := chain.ablaState
	for past := 0; past < 1000; {
		state = state.nextABLAState(&chain.ablaConfig,
			state.getBlockSizeLimit())
		if state.getBlockSizeLimit() > uint64(wire.MaxBlockPayload()) {
			past++
		}
	}
	chain.ablaState = state
	chain.chainLock.Unlock()
	m.mustAccept(m.build(nil))

	limit := chain.MaxBlockSize(true, true)
	if got := uint64(wire.MaxBlockPayload()); got < limit {
		t.Fatalf("block message limit %d is below the next block's "+
			"size limit %d", got, limit)
	}
	hdr := blockMessageHeader(params.Net, uint32(limit))
	_, _, err = wire.ReadMessage(bytes.NewReader(hdr), wire.ProtocolVersion,
		params.Net)
	if err != io.EOF {
		t.Fatalf("header of a block of %d bytes got %v, want its payload "+
			"read", limit, err)
	}
}

// TestMaxBlockSizeLookAheadHeights ensures the look-ahead is the same as one
// computed from the state at its own height, around the height n0 from which
// ABLA applies, and that the cached look-ahead moves the height into the range
// where it matters, so that tips whose states differ only in height share it.
func TestMaxBlockSizeLookAheadHeights(t *testing.T) {
	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var got uint64
	chain, err := New(&Config{
		DB:                          db,
		ChainParams:                 &params,
		TimeSource:                  NewMedianTime(),
		SigCache:                    txscript.NewSigCache(1000),
		UtxoCacheMaxSize:            1 << 20,
		ExcessiveBlockSize:          32000000,
		NotifyMaxBlockSizeLookAhead: func(size uint64) { got = size },
	})
	if err != nil {
		t.Fatal(err)
	}

	const n0 = 3000
	chain.chainLock.Lock()
	defer chain.chainLock.Unlock()
	config := &chain.ablaConfig
	config.n0 = n0
	initial := ABLAState{controlBlockSize: config.epsilon0,
		elasticBufferSize: config.beta0}
	grown := initial
	for range 100 {
		grown = grown.nextABLAState(config, grown.getBlockSizeLimit())
	}
	for _, state := range []ABLAState{initial, grown} {
		for height := uint64(n0 - 2100); height <= n0+3; height++ {
			state.blockHeight = height
			chain.ablaState = state
			chain.notifyMaxBlockSizeLookAhead()
			ahead := state.lookaheadState(config, 2047)
			if want := ahead.getBlockSizeLimit(); got != want {
				t.Fatalf("state %+v: look-ahead %d, want %d", state,
					got, want)
			}
			key := min(max(height, n0-2047), n0+1)
			if chain.lookAheadFrom.blockHeight != key {
				t.Fatalf("state %+v: cached at height %d, want %d",
					state, chain.lookAheadFrom.blockHeight, key)
			}
		}
	}
}

// paddedBlock returns block with bytes that hold extra data after its last
// transaction, as the payload of a block message can.  The block, and so its
// hash, is unchanged.
func paddedBlock(t *testing.T, block *bchutil.Block, pad int) *bchutil.Block {
	t.Helper()
	serialized, err := block.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	padded := append(bytes.Clone(serialized), make([]byte, pad)...)
	return bchutil.NewBlockFromBlockAndBytes(block.MsgBlock(), padded)
}

// checkStoredSerialization ensures the database holds block as its own
// serialization and gives its serialized size as its size.
func checkStoredSerialization(t *testing.T, db database.DB, block *bchutil.Block) {
	t.Helper()
	var want bytes.Buffer
	if err := block.MsgBlock().Serialize(&want); err != nil {
		t.Fatal(err)
	}
	err := db.View(func(dbTx database.Tx) error {
		got, err := dbTx.FetchBlock(block.Hash())
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want.Bytes()) {
			t.Errorf("block %v stored as %d bytes, want its %d-byte "+
				"serialization", block.Hash(), len(got), want.Len())
		}
		size, err := dbTx.FetchBlockSize(block.Hash())
		if err != nil {
			return err
		}
		if int(size) != want.Len() {
			t.Errorf("block %v has a stored size of %d bytes, want %d",
				block.Hash(), size, want.Len())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestABLAPaddedSideChainBlock ensures a side chain block given with data
// after its last transaction, as a block message can carry, is stored as its
// own serialization, so the ABLA state after it, which the side chain replay
// takes from the stored size, follows the block's serialized size, as Bitcoin
// Cash Node's does (validation.cpp:1602-1614).
func TestABLAPaddedSideChainBlock(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 3, 2000)

	// The side chain forks after block 1 with a small block whose bytes
	// are padded to the 1 MB limit, a size that would grow the state.
	side := ablaTestBlock(t, blocks[0].MsgBlock(), 2, 2001, params)
	if _, _, err := chain.ProcessBlock(paddedBlock(t, side, 1000000-2001),
		BFNone); err != nil {

		t.Fatalf("side block: %v", err)
	}
	checkStoredSerialization(t, db, side)
	got, err := chain.ablaStateAfter(chain.index.LookupNode(side.Hash()))
	if err != nil {
		t.Fatal(err)
	}
	want := ablaExpectedState(chain, []*bchutil.Block{blocks[0], side})
	if got != want {
		t.Fatalf("ABLA state after the padded side block %+v, want %+v",
			got, want)
	}
}

// TestABLAPaddedBlockRebuild ensures a main chain block given with data after
// its last transaction is stored as its own serialization, so the rebuild of
// the stored ABLA states, which takes each block's size from where it is
// stored, gives the states that follow the serialized sizes.
func TestABLAPaddedBlockRebuild(t *testing.T) {
	params := ablaTestParams()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaTestChain(t, db, params)
	var blocks []*bchutil.Block
	prev := params.GenesisBlock
	for h := int32(1); h <= 5; h++ {
		block := ablaTestBlock(t, prev, h, 2000, params)
		given := block
		if h == 3 {
			given = paddedBlock(t, block, 1000000-2000)
		}
		if _, _, err := chain.ProcessBlock(given, BFNone); err != nil {
			t.Fatalf("block %d: %v", h, err)
		}
		blocks = append(blocks, block)
		prev = block.MsgBlock()
	}
	checkStoredSerialization(t, db, blocks[2])
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}

	writeOldABLAStates(t, db, int32(len(blocks)))
	chain = ablaTestChain(t, db, params)
	if want := ablaExpectedState(chain, blocks); chain.ablaState != want {
		t.Fatalf("ABLA state %+v, want %+v", chain.ablaState, want)
	}
	err = db.View(func(dbTx database.Tx) error {
		for h := 0; h <= len(blocks); h++ {
			state, err := chain.dbFetchValidABLAState(dbTx, int32(h))
			if err != nil {
				return err
			}
			if want := ablaExpectedState(chain, blocks[:h]); state != want {
				t.Errorf("height %d: ABLA state %+v, want %+v", h,
					state, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// legacyPaddedDB is a database that gives the blocks in padded the way earlier
// versions stored a block made from a block message with data after its last
// transaction: with that data, in their bytes and in their stored size.  The
// blocks in unreadable can't be read, as if pruned.  It counts the reads of
// the padded blocks.
type legacyPaddedDB struct {
	database.DB
	padded     map[chainhash.Hash][]byte
	unreadable map[chainhash.Hash]bool
	reads      int
}

// View runs fn with a transaction that gives the padded blocks.
func (db *legacyPaddedDB) View(fn func(database.Tx) error) error {
	return db.DB.View(func(tx database.Tx) error {
		return fn(legacyPaddedTx{tx, db})
	})
}

// Update runs fn with a transaction that gives the padded blocks.
func (db *legacyPaddedDB) Update(fn func(database.Tx) error) error {
	return db.DB.Update(func(tx database.Tx) error {
		return fn(legacyPaddedTx{tx, db})
	})
}

// pad makes block one stored with pad bytes after it, which can't be read
// when unreadable is set.
func (db *legacyPaddedDB) pad(t *testing.T, block *bchutil.Block, pad int, unreadable bool) {
	t.Helper()
	var serialized bytes.Buffer
	if err := block.MsgBlock().Serialize(&serialized); err != nil {
		t.Fatal(err)
	}
	db.padded[*block.Hash()] = append(serialized.Bytes(), make([]byte, pad)...)
	db.unreadable[*block.Hash()] = unreadable
}

// legacyPaddedTx is a transaction of a legacyPaddedDB.
type legacyPaddedTx struct {
	database.Tx
	db *legacyPaddedDB
}

// FetchBlock returns the bytes of a padded block with the data after it.
func (tx legacyPaddedTx) FetchBlock(hash *chainhash.Hash) ([]byte, error) {
	padded, ok := tx.db.padded[*hash]
	if !ok {
		return tx.Tx.FetchBlock(hash)
	}
	tx.db.reads++
	if tx.db.unreadable[*hash] {
		return nil, database.Error{ErrorCode: database.ErrDriverSpecific,
			Err: os.ErrNotExist}
	}
	return padded, nil
}

// FetchBlockSize returns the size of a padded block with the data after it.
func (tx legacyPaddedTx) FetchBlockSize(hash *chainhash.Hash) (uint32, error) {
	if padded, ok := tx.db.padded[*hash]; ok {
		return uint32(len(padded)), nil
	}
	return tx.Tx.FetchBlockSize(hash)
}

// newLegacyPaddedDB returns a new ffldb database wrapped in a legacyPaddedDB.
func newLegacyPaddedDB(t *testing.T, params *chaincfg.Params) *legacyPaddedDB {
	t.Helper()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &legacyPaddedDB{DB: db, padded: make(map[chainhash.Hash][]byte),
		unreadable: make(map[chainhash.Hash]bool)}
}

// TestABLASideChainLegacyPaddedBlock ensures the side chain replay takes the
// serialized size of a block an earlier version stored with data after its
// last transaction when that data would change the ABLA state, reading the
// block only then, and the stored size when the block can't be read.
func TestABLASideChainLegacyPaddedBlock(t *testing.T) {
	params := ablaTestParams()
	db := newLegacyPaddedDB(t, params)
	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 3, 2000)
	cfg := &chain.ablaConfig
	forkState := ablaExpectedState(chain, blocks[:1])

	// Side blocks after block 1, the first with a second transaction, so
	// that its size is where its last transaction ends.
	parent := blocks[0].MsgBlock()
	spend := wire.NewMsgTx(2)
	spend.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: parent.Transactions[0].TxHash()},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	spend.AddTxOut(wire.NewTxOut(0, append([]byte{txscript.OP_RETURN},
		make([]byte, 1000)...), wire.TokenData{}))
	var sides []*bchutil.Block
	for i, size := range []int{3000, 2001, 2002} {
		var extra []*wire.MsgTx
		if i == 0 {
			extra = append(extra, spend)
		}
		side := ablaTestBlock(t, parent, 2, size, params, extra...)
		if _, _, err := chain.ProcessBlock(side, BFNone); err != nil {
			t.Fatalf("side block %d: %v", i, err)
		}
		sides = append(sides, side)
	}

	for _, test := range []struct {
		name       string
		side       *bchutil.Block
		pad        int
		unreadable bool
		size       uint64
		reads      int
	}{
		// Padded to the 1 MB limit, which would grow the state.
		{"padded", sides[0], 1000000 - 3000, false, 3000, 1},
		// Padded too little to change the state: not read.
		{"padded a little", sides[1], 1000, false, 3001, 0},
		// Padded to the limit, but unreadable: the stored size.
		{"padded and pruned", sides[2], 1000000 - 2002, true, 1000000, 1},
	} {
		db.reads = 0
		db.pad(t, test.side, test.pad, test.unreadable)
		got, err := chain.ablaStateAfter(chain.index.LookupNode(test.side.Hash()))
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if want := forkState.nextABLAState(cfg, test.size); got != want {
			t.Errorf("%s: ABLA state %+v, want %+v", test.name, got, want)
		}
		if db.reads != test.reads {
			t.Errorf("%s: %d reads of the block, want %d", test.name,
				db.reads, test.reads)
		}
	}
}

// TestABLASideChainLegacyPaddedAboveFloor ensures the check of a side chain
// block's stored size starts from the state after its parent: above the floor,
// even a little data an earlier version stored with a small block would change
// the state after it.
func TestABLASideChainLegacyPaddedAboveFloor(t *testing.T) {
	params := ablaTestParams()
	db := newLegacyPaddedDB(t, params)
	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 20, 999000)
	forkState := ablaExpectedState(chain, blocks[:19])
	if forkState.controlBlockSize == chain.ablaConfig.epsilon0 {
		t.Fatal("test blocks did not grow the ABLA state")
	}

	side := ablaTestBlock(t, blocks[18].MsgBlock(), 20, 2000, params)
	if _, _, err := chain.ProcessBlock(side, BFNone); err != nil {
		t.Fatalf("side block: %v", err)
	}
	db.pad(t, side, 28000, false)
	got, err := chain.ablaStateAfter(chain.index.LookupNode(side.Hash()))
	if err != nil {
		t.Fatal(err)
	}
	want := forkState.nextABLAState(&chain.ablaConfig, 2000)
	if got != want {
		t.Errorf("ABLA state %+v, want %+v", got, want)
	}
	if padded := forkState.nextABLAState(&chain.ablaConfig, 30000); padded == want {
		t.Errorf("the padded size gives the same state %+v", want)
	}
	if db.reads != 1 {
		t.Errorf("%d reads of the block, want 1", db.reads)
	}
}

// TestABLARebuildLegacyPaddedBlock ensures the rebuild of the stored ABLA
// states takes the serialized size of a block an earlier version stored with
// data after its last transaction, reading only that block, whose stored size
// would change the state.
func TestABLARebuildLegacyPaddedBlock(t *testing.T) {
	params := ablaTestParams()
	db := newLegacyPaddedDB(t, params)
	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 5, 2000)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}
	db.pad(t, blocks[2], 1000000-2000, false)
	db.pad(t, blocks[3], 1000, false)

	writeOldABLAStates(t, db, int32(len(blocks)))
	chain = ablaTestChain(t, db, params)
	if want := ablaExpectedState(chain, blocks); chain.ablaState != want {
		t.Fatalf("ABLA state %+v, want %+v", chain.ablaState, want)
	}
	err := db.View(func(dbTx database.Tx) error {
		for h := 0; h <= len(blocks); h++ {
			state, err := chain.dbFetchValidABLAState(dbTx, int32(h))
			if err != nil {
				return err
			}
			if want := ablaExpectedState(chain, blocks[:h]); state != want {
				t.Errorf("height %d: ABLA state %+v, want %+v", h,
					state, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if db.reads != 1 {
		t.Errorf("%d reads of the padded blocks, want 1", db.reads)
	}
}

// TestABLANextStateMonotone ensures the ABLA state after a block never
// decreases as the block's size grows, which ablaBlockSize relies on to leave
// a block unread when its stored size gives the state an empty block would.
func TestABLANextStateMonotone(t *testing.T) {
	for _, params := range []*chaincfg.Params{ablaTestParams(),
		&chaincfg.MainNetParams, &chaincfg.ChipNetParams} {

		c := params.ABLAConfig
		cfg := &ABLAConfig{epsilon0: c.Epsilon0, beta0: c.Beta0, n0: c.N0,
			gammaReciprocal: c.GammaReciprocal, zetaXB7: c.ZetaXB7,
			thetaReciprocal: c.ThetaReciprocal, delta: c.Delta,
			fixedSize: c.FixedSize}
		cfg.SetMax()
		floor := ABLAState{blockHeight: cfg.n0 + 1,
			controlBlockSize: cfg.epsilon0, elasticBufferSize: cfg.beta0}
		states := []ABLAState{floor,
			{floor.blockHeight, cfg.epsilon0 + 1, cfg.beta0},
			{floor.blockHeight, cfg.epsilon0 + 300, cfg.beta0 + 5000},
			{floor.blockHeight, 3 * cfg.epsilon0, 2 * cfg.beta0},
			{floor.blockHeight, cfg.epsilonMax, cfg.betaMax}}
		for _, prev := range states {
			limit := prev.controlBlockSize + prev.elasticBufferSize
			// Above this size the control block size grows.
			threshold := muldiv(prev.controlBlockSize, B7, cfg.zetaXB7)
			sizes := []uint64{0, 1, 1000, 25000, 100000, threshold - 1,
				threshold, threshold + 1, threshold + 100000, limit / 2,
				limit - 1, limit, limit + 1, limit + 100000}
			slices.Sort(sizes)
			last := prev.nextABLAState(cfg, sizes[0])
			for _, size := range sizes[1:] {
				next := prev.nextABLAState(cfg, size)
				if next.controlBlockSize < last.controlBlockSize ||
					next.elasticBufferSize < last.elasticBufferSize {

					t.Errorf("%s: state after %+v with %d bytes, %+v, "+
						"is below the one with fewer bytes, %+v",
						params.Name, prev, size, next, last)
				}
				last = next
			}
		}
	}
}

// TestABLARebuildLegacyPaddedAboveFloor ensures the rebuild checks a block's
// stored size from the state after its parent: above the floor, even a little
// data an earlier version stored with a small block would change the state.
func TestABLARebuildLegacyPaddedAboveFloor(t *testing.T) {
	params := ablaTestParams()
	db := newLegacyPaddedDB(t, params)
	chain := ablaTestChain(t, db, params)
	blocks := ablaTestMine(t, chain, 20, 999000)

	// A small block, then another one, so that the padded block is not
	// the tip, which the start reads for its own reasons.
	for h := int32(21); h <= 22; h++ {
		block := ablaTestBlock(t, blocks[h-2].MsgBlock(), h, 2000, params)
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, block)
	}
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}
	db.pad(t, blocks[20], 28000, false)

	writeOldABLAStates(t, db, int32(len(blocks)))
	chain = ablaTestChain(t, db, params)
	if want := ablaExpectedState(chain, blocks); chain.ablaState != want {
		t.Fatalf("ABLA state %+v, want %+v", chain.ablaState, want)
	}
	if db.reads != 1 {
		t.Errorf("%d reads of the padded block, want 1", db.reads)
	}
}
