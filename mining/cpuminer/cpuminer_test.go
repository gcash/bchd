// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cpuminer

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// emptyTxSource is a mining.TxSource without transactions.
type emptyTxSource struct{}

func (emptyTxSource) LastUpdated() time.Time               { return time.Time{} }
func (emptyTxSource) MiningDescs() []*mining.TxDesc        { return nil }
func (emptyTxSource) HaveTransaction(*chainhash.Hash) bool { return false }

// testMiner is a CPU miner on a fresh regtest chain with BIP34 active.
type testMiner struct {
	*CPUMiner
	chain *blockchain.BlockChain
	gen   *mining.BlkTmplGenerator
}

// newTestMiner returns a test miner that submits blocks through
// processBlock and asks isCurrent whether the chain is current.
func newTestMiner(t *testing.T, addr bchutil.Address,
	processBlock func(*blockchain.BlockChain, *bchutil.Block) (bool, error),
	isCurrent func() bool) *testMiner {

	t.Helper()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	params := chaincfg.RegressionNetParams
	params.BIP0034Height = 0
	timeSource := blockchain.NewMedianTime()
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         timeSource,
		UtxoCacheMaxSize:   16 * 1024 * 1024,
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	gen := mining.NewBlkTmplGenerator(&mining.Policy{BlockMaxSize: 1000000},
		&params, emptyTxSource{}, chain, timeSource,
		txscript.NewSigCache(10), txscript.NewHashCache(10))
	miner := New(&Config{
		ChainParams:            &params,
		BlockTemplateGenerator: gen,
		MiningAddrs:            []bchutil.Address{addr},
		ProcessBlock: func(b *bchutil.Block, _ blockchain.BehaviorFlags) (bool, error) {
			return processBlock(chain, b)
		},
		ConnectedCount: func() int32 { return 1 },
		IsCurrent:      isCurrent,
	})
	return &testMiner{CPUMiner: miner, chain: chain, gen: gen}
}

// mineBlock builds, solves and connects a block on the tip without the CPU
// miner, as a block from the network would arrive.
func (m *testMiner) mineBlock(addr bchutil.Address) error {
	tmpl, err := m.gen.NewBlockTemplate(addr)
	if err != nil {
		return err
	}
	header := &tmpl.Block.Header
	target := blockchain.CompactToBig(header.Bits)
	for {
		hash := header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			break
		}
		header.Nonce++
	}
	_, isOrphan, err := m.chain.ProcessBlock(bchutil.NewBlock(tmpl.Block),
		blockchain.BFNone)
	if err == nil && isOrphan {
		err = errors.New("block is an orphan")
	}
	return err
}

func testAddress(t *testing.T) bchutil.Address {
	t.Helper()
	addr, err := bchutil.NewAddressPubKeyHash(make([]byte, 20),
		&chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

// TestMinerCoinbaseHeightAfterTipMoves ensures the coinbase of a mined block
// has the height of the template it was built from when the tip moves after
// the miner first reads it.
func TestMinerCoinbaseHeightAfterTipMoves(t *testing.T) {
	addr := testAddress(t)
	results := make(chan error, 1)
	var m *testMiner
	var moved atomic.Bool
	m = newTestMiner(t, addr,
		func(chain *blockchain.BlockChain, b *bchutil.Block) (bool, error) {
			_, isOrphan, err := chain.ProcessBlock(b, blockchain.BFNone)
			select {
			case results <- err:
			default:
			}
			return isOrphan, err
		},
		func() bool {
			// Connect a block from elsewhere between the miner
			// reading the height and building its template.
			if !moved.Swap(true) {
				if err := m.mineBlock(addr); err != nil {
					t.Error(err)
				}
			}
			return true
		})
	if err := m.mineBlock(addr); err != nil {
		t.Fatal(err)
	}

	m.numWorkers = 1
	m.Start()
	defer m.Stop()
	select {
	case err := <-results:
		if err != nil {
			t.Fatalf("mined block rejected: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no block mined")
	}
}

// badAddress is an address type that has no payment script.
type badAddress struct{}

func (badAddress) String() string                 { return "bad" }
func (badAddress) EncodeAddress() string          { return "bad" }
func (badAddress) ScriptAddress() []byte          { return nil }
func (badAddress) IsForNet(*chaincfg.Params) bool { return true }

// generate runs GenerateNBlocks with a timeout.
func generate(t *testing.T, miner *testMiner, n uint32) ([]*chainhash.Hash, error) {
	t.Helper()
	type result struct {
		hashes []*chainhash.Hash
		err    error
	}
	done := make(chan result, 1)
	go func() {
		hashes, err := miner.GenerateNBlocks(n)
		done <- result{hashes, err}
	}()
	select {
	case r := <-done:
		return r.hashes, r.err
	case <-time.After(30 * time.Second):
		t.Fatal("GenerateNBlocks did not return")
		return nil, nil
	}
}

func alwaysCurrent() bool { return true }

// TestGenerateNBlocksSkipsOrphans ensures blocks that were not accepted are
// not returned as generated.
func TestGenerateNBlocksSkipsOrphans(t *testing.T) {
	calls := 0
	miner := newTestMiner(t, testAddress(t),
		func(chain *blockchain.BlockChain, b *bchutil.Block) (bool, error) {
			calls++
			if calls == 1 {
				return true, nil // reported as an orphan
			}
			_, isOrphan, err := chain.ProcessBlock(b, blockchain.BFNone)
			return isOrphan, err
		}, alwaysCurrent)

	hashes, err := generate(t, miner, 2)
	if err != nil {
		t.Fatalf("GenerateNBlocks: %v", err)
	}
	for _, hash := range hashes {
		if !miner.chain.MainChainHasBlock(hash) {
			t.Fatalf("returned block %v is not in the chain", hash)
		}
	}
}

// TestGenerateNBlocksReportsRejection ensures a rejected block ends the run
// with an error instead of being returned as generated.
func TestGenerateNBlocksReportsRejection(t *testing.T) {
	miner := newTestMiner(t, testAddress(t),
		func(*blockchain.BlockChain, *bchutil.Block) (bool, error) {
			return false, blockchain.RuleError{
				ErrorCode:   blockchain.ErrBadMerkleRoot,
				Description: "rejected",
			}
		}, alwaysCurrent)

	hashes, err := generate(t, miner, 1)
	if err == nil {
		t.Fatalf("rejected blocks returned as generated: %v", hashes)
	}

	// The miner can be used again.
	if _, err := generate(t, miner, 1); err == nil {
		t.Fatal("expected the rejection again")
	}
}

// TestGenerateNBlocksTemplateError ensures a template error is returned
// instead of retried forever.
func TestGenerateNBlocksTemplateError(t *testing.T) {
	miner := newTestMiner(t, badAddress{},
		func(*blockchain.BlockChain, *bchutil.Block) (bool, error) {
			return false, nil
		}, alwaysCurrent)
	if _, err := generate(t, miner, 1); err == nil {
		t.Fatal("expected a template error")
	}
}
