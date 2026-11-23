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
	"github.com/gcash/bchutil"
)

// parkingActivation makes block 20 the activation block of the upgrade to
// 1-minute blocks on a chain of procTestBlock blocks.
const parkingActivation = uint64(1700000000 + 60*14)

// parkingTest is a regtest chain whose blocks all have the same work, with
// finalization off unless configured.
type parkingTest struct {
	t      *testing.T
	dir    string
	params chaincfg.Params
	db     database.DB
	chain  *BlockChain

	// main is the main chain from genesis built by newParkingTest.
	main []*bchutil.Block

	// salt makes the blocks of each branch differ from the others'.
	salt int64

	// isMainChain is what ProcessBlock gave for the last block mined.
	isMainChain bool
}

// newParkingTest returns a parking test chain with the given activation time
// of the upgrade to 1-minute blocks and number of main chain blocks.
// configure, when not nil, changes the chain's configuration.
func newParkingTest(t *testing.T, activation uint64, numBlocks int32,
	configure func(*Config)) *parkingTest {

	t.Helper()
	p := &parkingTest{t: t, dir: filepath.Join(t.TempDir(), "db"),
		params: chaincfg.RegressionNetParams}
	p.params.Upgrade2027ActivationTime = activation
	p.open(configure)
	t.Cleanup(func() { p.db.Close() })

	p.main = []*bchutil.Block{bchutil.NewBlock(p.params.GenesisBlock)}
	p.main = append(p.main, p.mine(p.main[0], int(numBlocks))...)
	return p
}

// open opens the chain on the test's database, creating it the first time.
func (p *parkingTest) open(configure func(*Config)) {
	p.t.Helper()
	db, err := database.Open("ffldb", p.dir, p.params.Net)
	if err != nil {
		db, err = database.Create("ffldb", p.dir, p.params.Net)
		if err != nil {
			p.t.Fatal(err)
		}
	}
	config := &Config{
		DB:                 db,
		ChainParams:        &p.params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(10),
		ExcessiveBlockSize: LegacyMaxBlockSize,
		MaxReorgDepth:      -1,
		FinalizationDelay:  time.Nanosecond,
	}
	if configure != nil {
		configure(config)
	}
	chain, err := New(config)
	if err != nil {
		db.Close()
		p.t.Fatal(err)
	}
	p.db, p.chain = db, chain
}

// reopen closes the database and opens the chain on it again.
func (p *parkingTest) reopen(configure func(*Config)) {
	p.t.Helper()
	if err := p.chain.FlushCachedState(FlushRequired); err != nil {
		p.t.Fatal(err)
	}
	p.db.Close()
	p.open(configure)
}

// node returns the block's node.
func (p *parkingTest) node(block *bchutil.Block) *blockNode {
	p.t.Helper()
	node := p.chain.index.LookupNode(block.Hash())
	if node == nil {
		p.t.Fatalf("block %v is not known", block.Hash())
	}
	return node
}

// mine processes n blocks on prev, which differ from those of any other
// branch, and returns them.
func (p *parkingTest) mine(prev *bchutil.Block, n int) []*bchutil.Block {
	p.t.Helper()
	salt := p.salt
	p.salt++
	blocks := make([]*bchutil.Block, 0, n)
	for range n {
		height := p.node(prev).height
		block := procTestBlock(p.t, prev.MsgBlock(), height,
			childSubsidy(p.chain, prev)-salt, p.params.PowLimitBits, nil)
		isMainChain, _, err := p.chain.ProcessBlock(block, BFNone)
		if err != nil {
			p.t.Fatalf("block %d: %v", height+1, err)
		}
		p.isMainChain = isMainChain
		blocks = append(blocks, block)
		prev = block
	}
	return blocks
}

// tip returns the hash of the main chain tip.
func (p *parkingTest) tip() chainhash.Hash {
	return p.chain.BestSnapshot().Hash
}

// requireTip fails the test unless the block is the main chain tip.
func (p *parkingTest) requireTip(block *bchutil.Block, what string) {
	p.t.Helper()
	if tip := p.tip(); tip != *block.Hash() {
		p.t.Fatalf("%s: tip %v (height %d), want %v (height %d)", what,
			tip, p.chain.BestSnapshot().Height, block.Hash(),
			p.node(block).height)
	}
}

// parked returns whether the block is parked.
func (p *parkingTest) parked(block *bchutil.Block) bool {
	p.t.Helper()
	return p.chain.index.NodeStatus(p.node(block)).Parked()
}

// requireNoParkedMainChain fails the test if a main chain block is parked.
func (p *parkingTest) requireNoParkedMainChain() {
	p.t.Helper()
	for n := p.chain.bestChain.Tip(); n != nil; n = n.parent {
		if p.chain.index.NodeStatus(n).Parked() {
			p.t.Fatalf("main chain block %v (height %d) is parked",
				n.hash, n.height)
		}
	}
}

// last returns the last of the blocks.
func last(blocks []*bchutil.Block) *bchutil.Block {
	return blocks[len(blocks)-1]
}

// TestParkTrigger ensures a block is parked when its chain would take more
// than one block off the main chain, counted in blocks also after the
// upgrade to 1-minute blocks.
func TestParkTrigger(t *testing.T) {
	for _, test := range []struct {
		name       string
		activation uint64
		tip        int32
	}{
		{"legacy", upgrade2027Never, 5},
		{"1-minute blocks", parkingActivation, 25},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A block extending the tip is not parked.
			p := newParkingTest(t, test.activation, test.tip, nil)
			child := p.mine(p.main[test.tip], 1)
			if p.parked(child[0]) || !p.isMainChain {
				t.Fatal("block extending the tip parked or not " +
					"connected")
			}

			// A sibling of the tip is not parked, and its chain
			// replaces the main chain with one block more.
			p = newParkingTest(t, test.activation, test.tip, nil)
			sibling := p.mine(p.main[test.tip-1], 1)
			if p.parked(sibling[0]) || p.isMainChain {
				t.Fatal("sibling of the tip parked or connected")
			}
			next := p.mine(sibling[0], 1)
			p.requireTip(next[0], "sibling chain with one block more")

			// A block forking 2 blocks below the tip is parked.
			p = newParkingTest(t, test.activation, test.tip, nil)
			deep := p.mine(p.main[test.tip-2], 1)
			if !p.parked(deep[0]) || p.isMainChain {
				t.Fatal("block forking 2 blocks deep not parked")
			}
		})
	}
}

// requireUnparkAt mines a chain on the main chain block depth blocks below
// the tip and ensures it stays parked with need-1 blocks and replaces the
// main chain with need blocks.  It returns the chain.
func (p *parkingTest) requireUnparkAt(depth, need int) []*bchutil.Block {
	p.t.Helper()
	tip := last(p.main)
	fork := p.main[len(p.main)-1-depth]
	branch := p.mine(fork, need-1)
	p.requireTip(tip, fmt.Sprintf("depth %d with %d blocks", depth, need-1))
	for _, block := range branch {
		if !p.parked(block) {
			p.t.Fatalf("depth %d: block %v not parked", depth,
				block.Hash())
		}
	}
	branch = append(branch, p.mine(last(branch), 1)...)
	if !p.isMainChain {
		p.t.Fatalf("depth %d: block %d not connected", depth, need)
	}
	p.requireTip(last(branch), fmt.Sprintf("depth %d with %d blocks", depth,
		need))
	p.requireNoParkedMainChain()
	return branch
}

// TestParkingLegacyReorg ensures a parked chain replaces the main chain with
// as many more blocks as the legacy rule asks for: one more at depth 2, two
// more at depth 3, and more than twice as many from depth 4.  A chain with
// as much work as the main chain never replaces it.
func TestParkingLegacyReorg(t *testing.T) {
	for _, test := range []struct{ depth, need int }{
		{2, 3},
		{3, 5},
		{4, 9},
		{6, 13},
	} {
		t.Run(fmt.Sprintf("depth %d", test.depth), func(t *testing.T) {
			p := newParkingTest(t, upgrade2027Never, 10, nil)
			p.requireUnparkAt(test.depth, test.need)
		})
	}
}

// TestParkingTickReorg ensures a parked chain replaces a main chain of
// 1-minute blocks with more than an eighth more work since the fork short of
// 2400 ticks, and with more than twice the work from there on.
func TestParkingTickReorg(t *testing.T) {
	for _, test := range []struct{ depth, need int }{
		{2, 3},
		{3, 4},
		{4, 5},
		{5, 6},
		{6, 7},
		{7, 8},
		// An eighth of 8 blocks' work is a block's, which is not more.
		{8, 10},
		{39, 44},
		{40, 81},
	} {
		t.Run(fmt.Sprintf("depth %d", test.depth), func(t *testing.T) {
			// The fork is the activation block, so the main
			// chain after it is 1-minute blocks.
			p := newParkingTest(t, parkingActivation,
				20+int32(test.depth), nil)
			fork := p.node(p.main[20])
			if !isUpgrade2027Active(fork) || isUpgrade2027Active(fork.parent) {
				t.Fatal("block 20 is not the activation block")
			}
			d := p.chain.bestChain.Tip().ticks - fork.ticks
			if d != int64(60*test.depth) {
				t.Fatalf("tick depth %d", d)
			}
			p.requireUnparkAt(test.depth, test.need)
		})
	}
}

// TestParkingUpgradeSwitch ensures the legacy rule applies while the tip is
// the activation block and the tick rule from the block after it.
func TestParkingUpgradeSwitch(t *testing.T) {
	// The tip is the activation block: 4 blocks deep needs 9.
	p := newParkingTest(t, parkingActivation, 20, nil)
	tip := p.node(p.main[20])
	if !isUpgrade2027Active(tip) || isUpgrade2027Active(tip.parent) {
		t.Fatal("block 20 is not the activation block")
	}
	p.requireUnparkAt(4, 9)

	// The tip is the first 1-minute block: 4 blocks deep, 1860 ticks,
	// needs 5.
	p = newParkingTest(t, parkingActivation, 21, nil)
	p.requireUnparkAt(4, 5)
}

// TestParkingDisplacedChain ensures the chain a parked chain replaced is not
// parked itself, but its next block is when it forks 2 blocks below the new
// tip.
func TestParkingDisplacedChain(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 10, nil)
	branch := p.requireUnparkAt(2, 3)
	if p.parked(p.main[10]) {
		t.Fatal("displaced chain parked")
	}
	next := p.mine(p.main[10], 1)
	if !p.parked(next[0]) {
		t.Fatal("block on the displaced chain not parked")
	}
	p.requireTip(last(branch), "block on the displaced chain")
}

// TestNoParkDeepReorg ensures a chain is not parked with parking off, so
// that one more block replaces the main chain however deep the fork, while
// blocks parked before still need the work to be unparked.
func TestNoParkDeepReorg(t *testing.T) {
	noPark := func(c *Config) { c.NoParkDeepReorg = true }
	p := newParkingTest(t, upgrade2027Never, 10, noPark)
	branch := p.mine(p.main[6], 5)
	p.requireTip(last(branch), "5 blocks on a fork 4 deep")
	for _, block := range branch {
		if p.parked(block) {
			t.Fatal("block parked with parking off")
		}
	}

	// Blocks parked with parking on stay parked after a restart without.
	p = newParkingTest(t, upgrade2027Never, 10, nil)
	branch = p.mine(p.main[6], 3)
	p.reopen(noPark)
	if !p.parked(branch[0]) {
		t.Fatal("parked block not parked after a restart")
	}
	branch = append(branch, p.mine(last(branch), 5)...)
	if p.parked(last(branch)) {
		t.Fatal("block parked with parking off")
	}
	p.requireTip(p.main[10], "8 blocks on a parked fork 4 deep")
	branch = append(branch, p.mine(last(branch), 1)...)
	p.requireTip(last(branch), "9 blocks on a parked fork 4 deep")
	p.requireNoParkedMainChain()
}

// TestNoAutomaticUnparking ensures a parked chain stays parked whatever its
// work when automatic unparking is off.
func TestNoAutomaticUnparking(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 10, func(c *Config) {
		c.NoAutomaticUnparking = true
	})
	branch := p.mine(p.main[8], 10)
	p.requireTip(p.main[10], "10 blocks on a fork 2 deep")
	if !p.parked(branch[0]) {
		t.Fatal("block forking 2 deep not parked")
	}

	// A sibling of the tip is not parked, so it is connected as before.
	sibling := p.mine(p.main[9], 2)
	p.requireTip(last(sibling), "sibling chain with one block more")
}

// TestParkingRestart ensures parked blocks stay parked after a restart, and
// the next block of their chain is checked against the stored tip.
func TestParkingRestart(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 10, nil)
	branch := p.mine(p.main[6], 3)
	p.reopen(nil)
	for _, block := range branch {
		if !p.parked(block) {
			t.Fatalf("block %v not parked after a restart",
				block.Hash())
		}
	}
	p.requireNoParkedMainChain()
	branch = append(branch, p.mine(last(branch), 5)...)
	p.requireTip(p.main[10], "8 blocks on a fork 4 deep")
	branch = append(branch, p.mine(last(branch), 1)...)
	p.requireTip(last(branch), "9 blocks on a fork 4 deep")
	p.requireNoParkedMainChain()

	// A main chain block stored as parked is unparked on load.
	p.chain.index.SetStatusFlags(p.node(branch[4]), statusParked)
	if err := p.chain.index.flushToDB(); err != nil {
		t.Fatal(err)
	}
	p.reopen(nil)
	p.requireNoParkedMainChain()
}

// TestParkingFinalization ensures a block forking the chain below the
// finalized block is rejected before it could be parked, that finalizing a
// parked block unparks its chain, and that a chain forking below the
// finalized block is not switched to.
func TestParkingFinalization(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 5, func(c *Config) {
		c.MaxReorgDepth = 2
	})
	if fin := p.chain.finalizedNode; fin == nil || fin.height != 3 {
		t.Fatalf("finalized %v, want block 3", fin)
	}
	height := p.node(p.main[2]).height
	block := procTestBlock(t, p.main[2].MsgBlock(), height,
		childSubsidy(p.chain, p.main[2])-100, p.params.PowLimitBits, nil)
	_, _, err := p.chain.ProcessBlock(block, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != ErrForkBeforeFinalized {
		t.Fatalf("fork below the finalized block: got %v, want "+
			"ErrForkBeforeFinalized", err)
	}
	if p.chain.index.LookupNode(block.Hash()) != nil {
		t.Fatal("fork below the finalized block stored")
	}

	// Finalizing a parked block unparks its chain.
	p = newParkingTest(t, upgrade2027Never, 5, nil)
	branch := p.mine(p.main[2], 2)
	if !p.parked(branch[0]) || !p.parked(branch[1]) {
		t.Fatal("blocks forking 3 deep not parked")
	}
	if err := p.chain.FinalizeBlock(branch[1].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(branch[1], "finalized parked block")
	p.requireNoParkedMainChain()

	// A chain that forks below the finalized block is passed over, also
	// when unparked with enough work.
	p = newParkingTest(t, upgrade2027Never, 10, func(c *Config) {
		c.NoAutomaticUnparking = true
	})
	branch = p.mine(p.main[6], 10)
	p.requireTip(p.main[10], "parked chain")
	nodes := p.chain.index.descendants(p.node(p.main[6]))
	p.chain.chainLock.Lock()
	p.chain.finalizedNode = p.node(p.main[8])
	p.chain.chainLock.Unlock()
	p.chain.automaticUnparking = true
	p.chain.processLock.Lock()
	err = p.chain.reorganizeToBest(nodes)
	p.chain.processLock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[10], "chain forking below the finalized block")
	if !p.parked(branch[0]) {
		t.Fatal("chain forking below the finalized block unparked")
	}

	p.chain.chainLock.Lock()
	p.chain.finalizedNode = p.node(p.main[6])
	p.chain.chainLock.Unlock()
	p.chain.processLock.Lock()
	err = p.chain.reorganizeToBest(nodes)
	p.chain.processLock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	p.requireTip(last(branch), "chain forking at the finalized block")
	p.requireNoParkedMainChain()
}

// TestParkingFinalizeDescendants ensures finalizing a parked block connects
// the parked blocks built on it when they have more work, and that a failed
// finalization leaves the block's chain parked.
func TestParkingFinalizeDescendants(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 5, nil)
	branch := p.mine(p.main[2], 4)
	p.requireTip(p.main[5], "chain forking 3 deep with one block more")
	if err := p.chain.FinalizeBlock(branch[1].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(last(branch), "blocks built on the finalized block")
	p.requireNoParkedMainChain()

	// A block that overpays its coinbase is only found invalid when
	// connected, so it is stored and parked.
	p = newParkingTest(t, upgrade2027Never, 10, nil)
	branch = p.mine(p.main[8], 1)
	height := p.node(branch[0]).height
	bad := procTestBlock(t, branch[0].MsgBlock(), height,
		childSubsidy(p.chain, branch[0])+1, p.params.PowLimitBits, nil)
	if _, _, err := p.chain.ProcessBlock(bad, BFNone); err != nil {
		t.Fatal(err)
	}
	if err := p.chain.FinalizeBlock(bad.Hash()); err == nil {
		t.Fatal("invalid block finalized")
	}
	p.requireTip(p.main[10], "failed finalization")
	if !p.parked(branch[0]) {
		t.Fatal("chain unparked by a failed finalization")
	}
}

// TestParkingLaterBlock ensures a chain whose first block off the main chain
// is not parked, but a later one is, still needs the work to be unparked.
func TestParkingLaterBlock(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 10, nil)
	sibling := p.mine(p.main[9], 1)
	if p.parked(sibling[0]) {
		t.Fatal("sibling of the tip parked")
	}
	main := p.mine(p.main[10], 2)

	// The chain forks 3 blocks below the tip, so it needs 5 blocks.
	branch := p.mine(sibling[0], 3)
	if !p.parked(branch[0]) {
		t.Fatal("block forking 2 deep not parked")
	}
	p.requireTip(main[1], "chain with 4 blocks")
	branch = append(branch, p.mine(last(branch), 1)...)
	p.requireTip(last(branch), "chain with 5 blocks")
	p.requireNoParkedMainChain()
}

// TestParkingInvalidBlock ensures a parked chain with enough work that holds
// an invalid block leaves the main chain in place.  Its valid blocks stay
// unparked, as in Bitcoin Cash Node, which unparks a chain before connecting
// it, and a block built on them is parked again once the main chain grows.
func TestParkingInvalidBlock(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 10, nil)
	branch := p.mine(p.main[8], 2)
	height := p.node(branch[1]).height
	bad := procTestBlock(t, branch[1].MsgBlock(), height,
		childSubsidy(p.chain, branch[1])+1, p.params.PowLimitBits, nil)
	_, _, err := p.chain.ProcessBlock(bad, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != ErrBadCoinbaseValue {
		t.Fatalf("got %v, want ErrBadCoinbaseValue", err)
	}
	p.requireTip(p.main[10], "chain with an invalid block")
	if p.parked(branch[0]) || p.parked(branch[1]) {
		t.Fatal("valid blocks of the chain still parked")
	}
	main := p.mine(p.main[10], 1)
	p.requireTip(main[0], "main chain")
	more := p.mine(branch[1], 1)
	if !p.parked(more[0]) {
		t.Fatal("block on the unparked chain not parked")
	}
	p.requireTip(main[0], "block on the unparked chain")
	p.requireNoParkedMainChain()
}

// TestInvalidateBlockOtherChain ensures invalidating the tip connects the
// chain with the most work the node has, unparking it when it has enough work
// over the new tip, and passes over a chain that turns out invalid.
func TestInvalidateBlockOtherChain(t *testing.T) {
	for _, noPark := range []bool{false, true} {
		t.Run(fmt.Sprint("noparkdeepreorg ", noPark), func(t *testing.T) {
			p := newParkingTest(t, upgrade2027Never, 10,
				func(c *Config) { c.NoParkDeepReorg = noPark })

			// A chain forking 2 deep with as much work, which is
			// parked, and an invalid sibling of the tip, received
			// before it.
			branch := p.mine(p.main[8], 2)
			if p.parked(branch[0]) == noPark {
				t.Fatal("parked with parking off or not parked")
			}
			height := p.node(p.main[9]).height
			bad := procTestBlock(t, p.main[9].MsgBlock(), height,
				childSubsidy(p.chain, p.main[9])+1,
				p.params.PowLimitBits, nil)
			if _, _, err := p.chain.ProcessBlock(bad, BFNone); err != nil {
				t.Fatal(err)
			}
			p.node(bad).received = p.node(branch[1]).received - 1

			// The chain needs half a block more than the new tip at
			// depth 1, so it is connected.
			if err := p.chain.InvalidateBlock(p.main[10].Hash()); err != nil {
				t.Fatal(err)
			}
			p.requireTip(last(branch), "chain with more work")
			p.requireNoParkedMainChain()
			status := p.chain.index.NodeStatus(p.node(bad))
			if !status.KnownInvalid() {
				t.Fatal("invalid block not marked invalid")
			}
		})
	}
}

// TestParkedStatusRow ensures the parked flag is stored with a block's status
// and that the other status checks ignore it, so that an older version
// takes a parked block as a side chain block.
func TestParkedStatusRow(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 1, nil)
	node := p.node(p.main[1])
	for status := statusNone; status < statusParked; status++ {
		parked := status | statusParked
		if parked.HaveData() != status.HaveData() ||
			parked.KnownValid() != status.KnownValid() ||
			parked.KnownInvalid() != status.KnownInvalid() {

			t.Fatalf("status %08b reads differently when parked",
				status)
		}
		if status.Parked() || !parked.Parked() {
			t.Fatalf("status %08b: parked flag misread", status)
		}

		row := *node
		row.status = parked
		var got blockStatus
		err := p.db.Update(func(dbTx database.Tx) error {
			if err := dbStoreBlockNode(dbTx, &row); err != nil {
				return err
			}
			bucket := dbTx.Metadata().Bucket(blockIndexBucketName)
			value := bucket.Get(blockIndexKey(&row.hash,
				uint32(row.height)))
			var err error
			_, got, err = deserializeBlockRow(value)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != parked {
			t.Fatalf("stored status %08b, read %08b", parked, got)
		}
	}
}

// requireStatus fails the test unless the block's parked and invalid flags
// are as given.
func (p *parkingTest) requireStatus(block *bchutil.Block, parked, invalid bool) {
	p.t.Helper()
	status := p.chain.index.NodeStatus(p.node(block))
	if status.Parked() != parked || status.KnownInvalid() != invalid {
		p.t.Fatalf("block %v parked %v, invalid %v, want %v and %v",
			block.Hash(), status.Parked(), status.KnownInvalid(),
			parked, invalid)
	}
}

// TestParkUnparkBlock ensures parking a block takes it and the blocks built
// on it off the main chain while automatic unparking is off, and unparking
// any of them brings them back, and that parking and validity are kept
// apart in every order of parking, invalidating, reconsidering and
// unparking, as in Bitcoin Cash Node's parked chain test.
func TestParkUnparkBlock(t *testing.T) {
	type step struct {
		name  string
		apply func(*BlockChain, *chainhash.Hash) error
	}
	park := step{"park", (*BlockChain).ParkBlock}
	unpark := step{"unpark", (*BlockChain).UnparkBlock}
	invalidate := step{"invalidate", (*BlockChain).InvalidateBlock}
	reconsider := step{"reconsider", (*BlockChain).ReconsiderBlock}

	for _, test := range []struct {
		steps []step

		// tips are the main chain tip after each step as a main chain
		// block, and parked and invalid the flags of block 8.
		tips    []int
		parked  []bool
		invalid []bool
	}{{
		steps:   []step{invalidate, park, unpark, reconsider},
		tips:    []int{7, 7, 7, 10},
		parked:  []bool{false, true, false, false},
		invalid: []bool{true, true, true, false},
	}, {
		steps:   []step{park, invalidate, reconsider, unpark},
		tips:    []int{7, 7, 7, 10},
		parked:  []bool{true, true, true, false},
		invalid: []bool{false, true, false, false},
	}, {
		steps:   []step{invalidate, park, reconsider, unpark},
		tips:    []int{7, 7, 7, 10},
		parked:  []bool{false, true, true, false},
		invalid: []bool{true, true, false, false},
	}, {
		steps:   []step{park, invalidate, unpark, reconsider},
		tips:    []int{7, 7, 7, 10},
		parked:  []bool{true, true, false, false},
		invalid: []bool{false, true, true, false},
	}} {
		var names []string
		for _, s := range test.steps {
			names = append(names, s.name)
		}
		t.Run(fmt.Sprint(names), func(t *testing.T) {
			p := newParkingTest(t, upgrade2027Never, 10, func(c *Config) {
				c.NoAutomaticUnparking = true
			})
			for i, s := range test.steps {
				if err := s.apply(p.chain, p.main[8].Hash()); err != nil {
					t.Fatalf("%s: %v", s.name, err)
				}
				p.requireTip(p.main[test.tips[i]], s.name)
				p.requireStatus(p.main[8], test.parked[i],
					test.invalid[i])
			}
			p.requireNoParkedMainChain()
		})
	}

	// Parking a block 2 below the tip takes the chain off, and unparking
	// the old tip unparks the blocks below it too.
	p := newParkingTest(t, upgrade2027Never, 10, func(c *Config) {
		c.NoAutomaticUnparking = true
	})
	if err := p.chain.ParkBlock(p.main[8].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[7], "parked block 8")
	if err := p.chain.ParkBlock(p.main[8].Hash()); err != nil {
		t.Fatalf("park a parked block: %v", err)
	}
	if err := p.chain.UnparkBlock(p.main[10].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[10], "unparked block 10")
	p.requireStatus(p.main[8], false, false)
	if err := p.chain.UnparkBlock(p.main[10].Hash()); err != nil {
		t.Fatalf("unpark a block that is not parked: %v", err)
	}

	// Unparking a block 2 below the tip unparks the blocks built on it.
	if err := p.chain.ParkBlock(p.main[9].Hash()); err != nil {
		t.Fatal(err)
	}
	if err := p.chain.ParkBlock(p.main[10].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[8], "parked blocks 9 and 10")
	if err := p.chain.UnparkBlock(p.main[9].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[10], "unparked block 9")
	p.requireNoParkedMainChain()

	// Unknown blocks and the genesis block are refused.
	if err := p.chain.ParkBlock(&chainhash.Hash{1}); err == nil {
		t.Fatal("parked an unknown block")
	}
	if err := p.chain.UnparkBlock(&chainhash.Hash{1}); err == nil {
		t.Fatal("unparked an unknown block")
	}
	if err := p.chain.ParkBlock(p.main[0].Hash()); err == nil {
		t.Fatal("parked the genesis block")
	}
}

// TestParkBlockAutomaticUnparking ensures a parked tip is connected again at
// once while unparking is automatic, as in Bitcoin Cash Node.
func TestParkBlockAutomaticUnparking(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 10, nil)
	if err := p.chain.ParkBlock(p.main[10].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[10], "parked tip")
	p.requireNoParkedMainChain()

	// Parking a block deeper connects the chain again the same way.
	if err := p.chain.ParkBlock(p.main[7].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[10], "parked block 7")
	p.requireNoParkedMainChain()
}

// TestParkBlockOtherChain ensures parking the tip connects another chain with
// more work than its parent, as Bitcoin Cash Node chooses its chain again
// among all blocks.
func TestParkBlockOtherChain(t *testing.T) {
	p := newParkingTest(t, upgrade2027Never, 10, func(c *Config) {
		c.NoAutomaticUnparking = true
	})
	sibling := p.mine(p.main[9], 1)
	if err := p.chain.ParkBlock(p.main[10].Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(sibling[0], "sibling of the parked tip")
	p.requireStatus(p.main[10], true, false)
}

// TestUnparkBlockChain ensures unparking a chain that stays parked with
// automatic unparking off connects it, unless it forks the chain below the
// finalized block.
func TestUnparkBlockChain(t *testing.T) {
	noUnparking := func(c *Config) { c.NoAutomaticUnparking = true }
	p := newParkingTest(t, upgrade2027Never, 10, noUnparking)
	branch := p.mine(p.main[8], 3)
	p.requireTip(p.main[10], "parked chain")
	if err := p.chain.UnparkBlock(last(branch).Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(last(branch), "unparked chain")
	p.requireNoParkedMainChain()

	// A chain parked before the main chain was finalized past its fork.
	p = newParkingTest(t, upgrade2027Never, 10, noUnparking)
	branch = p.mine(p.main[6], 10)
	p.chain.chainLock.Lock()
	p.chain.finalizedNode = p.node(p.main[8])
	p.chain.chainLock.Unlock()
	if err := p.chain.UnparkBlock(last(branch).Hash()); err != nil {
		t.Fatal(err)
	}
	p.requireTip(p.main[10], "unparked chain forking below the finalized "+
		"block")
	p.requireStatus(branch[0], false, false)
}
