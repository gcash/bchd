// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/shv"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// quietSHVLog disables the index logger for the test, since the log rotator
// is not set up under test.  It is called after the test's database is
// created, whose cleanup hands the index logger back to the indexers.
func quietSHVLog(t *testing.T) {
	t.Helper()
	old := indxLog
	indxLog = bchlog.Disabled
	t.Cleanup(func() { indxLog = old })
}

// newSHVTestChain returns a regtest chain holding only the genesis block, and
// an SHV index over it that follows its notifications, with its database.
func newSHVTestChain(t *testing.T) (*blockchain.BlockChain, database.DB, *shvIndex) {
	t.Helper()
	chain, db := newTestChainDB(t)
	quietSHVLog(t)
	x, err := newSHVIndex(chain)
	if err != nil {
		t.Fatal(err)
	}
	chain.Subscribe(x.handleBlockchainNotification)
	return chain, db, x
}

// mineSHVBlock connects a block on the chain's tip whose coinbase pays less
// satoshis less than allowed, so that blocks on the same parent can differ.
func mineSHVBlock(t *testing.T, chain *blockchain.BlockChain, less int64) *bchutil.Block {
	t.Helper()
	block := solvedBlock(t, chain, -less, nil)
	isMainChain, isOrphan, err := chain.ProcessBlock(block, blockchain.BFNone)
	if err != nil || !isMainChain || isOrphan {
		t.Fatalf("block %v: main chain %v, orphan %v, %v", block.Hash(),
			isMainChain, isOrphan, err)
	}
	return block
}

// mainChainHashes returns the hashes of the main chain's blocks.
func mainChainHashes(t *testing.T, chain *blockchain.BlockChain) []chainhash.Hash {
	t.Helper()
	best := chain.BestSnapshot()
	hashes, err := chain.HeightRange(0, best.Height+1)
	if err != nil || len(hashes) != int(best.Height)+1 {
		t.Fatalf("main chain hashes: %d, %v", len(hashes), err)
	}
	return hashes
}

// checkSHVIndex ensures the index covers the chain's main chain: that at
// every commitment height its root is the root of an accumulator over the
// main chain's hashes up to that height, that each block up to there is
// proven to that root and to its peak with its own header, and that
// commitments above the tip and blocks above the main chain are refused.
func checkSHVIndex(t *testing.T, x *shvIndex, chain *blockchain.BlockChain) {
	t.Helper()
	hashes := mainChainHashes(t, chain)
	tip, inStep := x.tipHeight()
	if !inStep || int(tip) != len(hashes)-1 {
		t.Fatalf("index at height %d, in step %v, want %d", tip, inStep,
			len(hashes)-1)
	}

	var acc shv.Accumulator
	for c := range hashes {
		if err := acc.Extend(&hashes[c]); err != nil {
			t.Fatal(err)
		}
		commitment := uint64(c)
		if root, ok := x.blockRoot(&hashes[c], int32(c)); !ok ||
			root != acc.Root() {

			t.Fatalf("commitment %d: root %v, %v, want %v", c, root, ok,
				acc.Root())
		}
		for b := 0; b <= c; b++ {
			height := uint64(b)
			header, branch, root, err := x.proof(true, height, commitment)
			if err != nil || header.BlockHash() != hashes[b] ||
				root != acc.Root() ||
				!acc.VerifyProofToRoot(height, &hashes[b], branch) {

				t.Fatalf("block %d, commitment %d: proof to the root "+
					"not verified: %v", b, c, err)
			}
			header, branch, peak, err := x.proof(false, height, commitment)
			if err != nil || header.BlockHash() != hashes[b] ||
				!slices.Contains(acc.Peaks(), peak) ||
				!acc.VerifyProofToPeak(height, &hashes[b], branch) {

				t.Fatalf("block %d, commitment %d: proof to the peak "+
					"not verified: %v", b, c, err)
			}
		}
	}

	above := uint64(len(hashes))
	if _, _, _, err := x.proof(true, 0, above); err != errSHVCommitment {
		t.Errorf("commitment above the tip: %v", err)
	}
	if _, _, _, err := x.proof(false, above, above); err != errSHVBlockNotFound {
		t.Errorf("block above the tip: %v", err)
	}
	if _, ok := x.blockRoot(&hashes[0], int32(above)); ok {
		t.Error("root above the tip")
	}
}

// TestSHVIndexFollowsChain ensures the SHV index of a real chain stays the MMR
// of its main chain, from the chain's notifications alone, as blocks are
// connected, disconnected and reorganized, and that building it again, also on
// the chain reopened from its database, gives the same index.
func TestSHVIndexFollowsChain(t *testing.T) {
	chain, db := newTestChainDB(t)
	quietSHVLog(t)

	// Once built, the index can't read hashes from the chain, so that it
	// follows the chain from the notifications alone, and never falls back
	// to reading the chain again.
	failing := &failingSHVChain{BlockChain: chain}
	x, err := newSHVIndex(failing)
	if err != nil {
		t.Fatal(err)
	}
	failing.Subscribe(x.handleBlockchainNotification)
	failing.fail.Store(true)
	checkSHVIndex(t, x, chain)

	for range 20 {
		mineSHVBlock(t, chain, 0)
		checkSHVIndex(t, x, chain)
	}
	original := mainChainHashes(t, chain)

	// Disconnect the blocks from height 18 up, and connect two others in
	// their place.
	if err := chain.InvalidateBlock(&original[18]); err != nil {
		t.Fatal(err)
	}
	if best := chain.BestSnapshot(); best.Height != 17 {
		t.Fatalf("tip at %d after invalidating block 18", best.Height)
	}
	checkSHVIndex(t, x, chain)
	for i := range 2 {
		mineSHVBlock(t, chain, int64(i)+1)
	}
	checkSHVIndex(t, x, chain)

	// A competing branch from block 13 reorganizes the chain once it is
	// longer: blocks 19 to 14 are disconnected and the branch's 14 to 20
	// connected, in one reorganization.
	prev := original[13]
	for height := int32(14); height <= 20; height++ {
		// Each block is a second later than the last, so its time is
		// past the branch's median time.
		block := solvedBlockWith(t, chain, func(msg *wire.MsgBlock) {
			msg.Transactions[0].TxOut[0].Value -= int64(height)
			msg.Header.PrevBlock = prev
			msg.Header.Timestamp = msg.Header.Timestamp.Add(
				time.Duration(height) * time.Second)
		})
		isMainChain, _, err := chain.ProcessBlock(block, blockchain.BFNone)
		if err != nil || isMainChain != (height == 20) {
			t.Fatalf("branch block %d: main chain %v, %v", height,
				isMainChain, err)
		}
		if height < 20 {
			checkSHVIndex(t, x, chain)
		}
		prev = *block.Hash()
	}
	if best := chain.BestSnapshot(); best.Hash != prev || best.Height != 20 {
		t.Fatalf("tip %v at %d, want the branch's block 20", best.Hash,
			best.Height)
	}
	checkSHVIndex(t, x, chain)
	if hashes := mainChainHashes(t, chain); !slices.Equal(hashes[:14],
		original[:14]) || hashes[14] == original[14] {

		t.Fatal("the reorganization did not replace blocks 14 and up")
	}
	original = mainChainHashes(t, chain)

	// An index built afresh from the chain is the same, also from the
	// chain reopened from its database, as when the node restarts.
	rebuilt, err := newSHVIndex(chain)
	if err != nil {
		t.Fatal(err)
	}
	checkSHVIndex(t, rebuilt, chain)
	if rebuilt.mmr.NodeCount() != x.mmr.NodeCount() || rebuilt.tip != x.tip {
		t.Fatalf("rebuilt index has %d nodes and tip %v, want %d and %v",
			rebuilt.mmr.NodeCount(), rebuilt.tip, x.mmr.NodeCount(), x.tip)
	}
	if err := chain.FlushCachedState(blockchain.FlushRequired); err != nil {
		t.Fatal(err)
	}
	reopened := newRPCTestChainOn(t, db)
	restarted, err := newSHVIndex(reopened)
	if err != nil {
		t.Fatal(err)
	}
	checkSHVIndex(t, restarted, reopened)
	for c := range original {
		want, _ := x.blockRoot(&original[c], int32(c))
		if got, ok := restarted.blockRoot(&original[c], int32(c)); !ok ||
			got != want {

			t.Fatalf("commitment %d: restarted root %v, want %v", c, got,
				want)
		}
	}
}

// TestSHVIndexCatchesUp ensures the index catches up with blocks it was not
// notified of, as fast sync adds headers without notifications, refusing
// commitments above its tip meanwhile, and builds again from genesis when its
// tip has left the main chain.
func TestSHVIndexCatchesUp(t *testing.T) {
	chain, _ := newTestChainDB(t)
	quietSHVLog(t)
	x, err := newSHVIndex(chain)
	if err != nil {
		t.Fatal(err)
	}

	// The index does not follow the chain's notifications here, so it
	// stays at genesis while blocks are connected, and refuses the
	// commitments above it.
	for range 5 {
		mineSHVBlock(t, chain, 0)
	}
	if _, _, _, err := x.proof(true, 0, 3); err != errSHVCommitment {
		t.Fatalf("commitment above the index's tip: %v", err)
	}
	if _, _, _, err := x.proof(true, 0, 0); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	// The next block's notification brings it in step.
	notify := func(typ blockchain.NotificationType, block *bchutil.Block) {
		x.handleBlockchainNotification(&blockchain.Notification{
			Type: typ, Data: block})
	}
	notify(blockchain.NTBlockConnected, mineSHVBlock(t, chain, 0))
	checkSHVIndex(t, x, chain)

	// Blocks disconnected and replaced without notifications leave the
	// index's tip out of the main chain, so the next notification builds
	// it again from genesis.
	hashes := mainChainHashes(t, chain)
	if err := chain.InvalidateBlock(&hashes[3]); err != nil {
		t.Fatal(err)
	}
	notify(blockchain.NTBlockConnected, mineSHVBlock(t, chain, 1))
	checkSHVIndex(t, x, chain)

	// A notification that does not fit the index, such as the
	// disconnection of a block that is not its tip, also brings it in step
	// with the chain.
	stale, err := chain.BlockByHeight(2)
	if err != nil {
		t.Fatal(err)
	}
	notify(blockchain.NTBlockDisconnected, stale)
	checkSHVIndex(t, x, chain)

	// Other notifications are ignored.
	notify(blockchain.NTBlockAccepted, stale)
	x.handleBlockchainNotification(&blockchain.Notification{
		Type: blockchain.NTBlockConnected, Data: "not a block"})
	checkSHVIndex(t, x, chain)
}

// failingSHVChain is a chain whose HeightRange fails while fail is set.
type failingSHVChain struct {
	*blockchain.BlockChain
	fail atomic.Bool
}

// HeightRange fails while fail is set, and otherwise reads the chain.
func (c *failingSHVChain) HeightRange(startHeight, endHeight int32) ([]chainhash.Hash, error) {
	if c.fail.Load() {
		return nil, errors.New("injected failure")
	}
	return c.BlockChain.HeightRange(startHeight, endHeight)
}

// TestSHVIndexInit ensures the server builds the SHV index and advertises
// SFNodeSHV by default, that --noshv turns both off, that a failure to build
// the index turns SHV off while the node runs on, and that a failure to bring
// the index in step later leaves it refusing proofs until a later block
// brings it in step.
func TestSHVIndexInit(t *testing.T) {
	newServerOn := func(noSHV bool, fail bool) (*server, *failingSHVChain) {
		t.Helper()
		useTestConfig(t, &config{NoSHV: noSHV})
		blockChain, _ := newTestChainDB(t)
		quietSHVLog(t)
		chain := &failingSHVChain{BlockChain: blockChain}
		chain.fail.Store(fail)
		s := &server{services: defaultServices}
		s.initSHVIndex(chain)
		return s, chain
	}

	s, _ := newServerOn(true, false)
	if s.shvIndex != nil || s.services&wire.SFNodeSHV != 0 {
		t.Errorf("--noshv: index %v, services %v", s.shvIndex, s.services)
	}
	s, _ = newServerOn(false, true)
	if s.shvIndex != nil || s.services&wire.SFNodeSHV != 0 {
		t.Errorf("failed build: index %v, services %v", s.shvIndex,
			s.services)
	}
	if s.services != defaultServices&^wire.SFNodeSHV {
		t.Errorf("failed build changed other services: %v", s.services)
	}

	s, chain := newServerOn(false, false)
	if s.shvIndex == nil || s.services != defaultServices {
		t.Fatalf("default: index %v, services %v", s.shvIndex, s.services)
	}
	x := s.shvIndex

	// The index follows the chain's notifications.
	for range 3 {
		mineSHVBlock(t, chain.BlockChain, 0)
	}
	checkSHVIndex(t, x, chain.BlockChain)

	// A notification that does not fit makes the index read the chain,
	// which fails, so the index refuses proofs.
	chain.fail.Store(true)
	stale, err := chain.BlockByHeight(1)
	if err != nil {
		t.Fatal(err)
	}
	x.handleBlockchainNotification(&blockchain.Notification{
		Type: blockchain.NTBlockDisconnected, Data: stale})
	if _, _, _, err := x.proof(true, 0, 0); err != errSHVNotInStep {
		t.Fatalf("proof while out of step: %v", err)
	}
	if _, ok := x.blockRoot(stale.Hash(), 1); ok {
		t.Fatal("root while out of step")
	}
	if _, inStep := x.tipHeight(); inStep {
		t.Fatal("in step after a failed build")
	}

	// The next block brings it back in step.
	chain.fail.Store(false)
	mineSHVBlock(t, chain.BlockChain, 0)
	checkSHVIndex(t, x, chain.BlockChain)
}

// TestSHVIndexConcurrentProofs ensures proofs served while blocks are
// connected, disconnected and reorganized are each consistent: the header is
// one of the chain's, and the proof leads from its hash to the root returned.
func TestSHVIndexConcurrentProofs(t *testing.T) {
	chain, _, x := newSHVTestChain(t)

	// A block is known before it is connected, as a proof of it may be
	// served as soon as it is.
	var known sync.Map
	known.Store(chain.BestSnapshot().Hash, true)
	mine := func(less int64) {
		t.Helper()
		block := solvedBlock(t, chain, -less, nil)
		known.Store(*block.Hash(), true)
		isMainChain, _, err := chain.ProcessBlock(block, blockchain.BFNone)
		if err != nil || !isMainChain {
			t.Fatalf("block %v: main chain %v, %v", block.Hash(),
				isMainChain, err)
		}
	}
	for range 8 {
		mine(0)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	var served, refused atomic.Int64
	for worker := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-done:
					return
				default:
				}
				commitment := uint64(i % 16)
				height := uint64(worker+i) % (commitment + 1)
				header, branch, root, err := x.proof(true, height,
					commitment)
				if err != nil {
					refused.Add(1)
					continue
				}
				leaf := header.BlockHash()
				if _, ok := known.Load(leaf); !ok {
					t.Errorf("block %d, commitment %d: unknown header "+
						"%v", height, commitment, leaf)
					return
				}
				if !shv.VerifyProofToRoot(&root, commitment+1, height,
					&leaf, branch) {

					t.Errorf("block %d, commitment %d: proof not "+
						"verified", height, commitment)
					return
				}
				served.Add(1)
			}
		}()
	}

	deadline := time.Now().Add(10 * time.Second)
	for round := int64(1); round <= 6 && time.Now().Before(deadline); round++ {
		for range 2 {
			mine(0)
		}
		hashes := mainChainHashes(t, chain)
		fork := hashes[len(hashes)-3]
		if err := chain.InvalidateBlock(&fork); err != nil {
			t.Fatal(err)
		}
		for range 3 {
			mine(round)
		}
	}
	close(done)
	wg.Wait()

	if served.Load() == 0 {
		t.Fatal("no proof was served")
	}
	t.Logf("%d proofs served, %d refused", served.Load(), refused.Load())
	checkSHVIndex(t, x, chain)
}

// TestSHVResponseStatus ensures each request is answered with the status for
// its outcome: found, with the header, when the block is in the commitment,
// and otherwise not found, above the commitment, above the index's tip, or a
// reserved type bit.
func TestSHVResponseStatus(t *testing.T) {
	chain, _, x := newSHVTestChain(t)
	for range 4 {
		mineSHVBlock(t, chain, 0)
	}
	hashes := mainChainHashes(t, chain)
	tip := uint64(len(hashes) - 1)
	unknown := chainhash.Hash{0x01}
	reserved := wire.NewSHVRequestByHeight(false, tip, 1)
	reserved.Type |= 0x04

	tests := []struct {
		name   string
		req    wire.SHVRequest
		status uint8
	}{
		{"by height", wire.NewSHVRequestByHeight(true, tip, 1),
			wire.SHVStatusFound},
		{"by hash", wire.NewSHVRequestByHash(false, tip, &hashes[2]),
			wire.SHVStatusFound},
		{"reserved type", reserved, wire.SHVStatusReservedType},
		{"no such block", wire.NewSHVRequestByHeight(false, tip + 5, tip + 2),
			wire.SHVStatusNotFound},
		{"unknown hash", wire.NewSHVRequestByHash(true, tip, &unknown),
			wire.SHVStatusNotFound},
		{"above the commitment", wire.NewSHVRequestByHeight(false, 1, 3),
			wire.SHVStatusAboveCommitment},
		{"hash above the commitment", wire.NewSHVRequestByHash(false, 0,
			&hashes[len(hashes) - 1]), wire.SHVStatusAboveCommitment},
		{"commitment above the tip", wire.NewSHVRequestByHeight(true,
			tip + 5, 1), wire.SHVStatusCommitmentAboveTip},
	}
	for _, test := range tests {
		resp := x.response(&test.req)
		if resp.Status != test.status {
			t.Errorf("%s: status %d, want %d", test.name, resp.Status,
				test.status)
			continue
		}
		if test.status != wire.SHVStatusFound {
			if resp.SerializeSize() != 1 {
				t.Errorf("%s: size %d, want 1", test.name,
					resp.SerializeSize())
			}
			continue
		}
		if resp.Type != test.req.Type ||
			resp.CommitmentHeight != test.req.CommitmentHeight ||
			resp.BlockHeight >= uint64(len(hashes)) ||
			resp.Header.BlockHash() != hashes[int(resp.BlockHeight)] {

			t.Errorf("%s: %+v", test.name, resp)
		}
	}
}
