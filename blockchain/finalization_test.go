// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchutil"
)

// finalizationTestChain returns a regtest chain with the given finalization
// settings and five main chain blocks.
func finalizationTestChain(t *testing.T, depth int32, delay time.Duration) (*BlockChain, []*bchutil.Block) {
	t.Helper()
	return upgradeFinalizationTestChain(t, depth, delay, upgrade2027Never, 5)
}

// upgradeFinalizationTestChain returns a regtest chain with the given
// finalization settings, upgrade 2027 activation time and number of main
// chain blocks.
func upgradeFinalizationTestChain(t *testing.T, depth int32, delay time.Duration,
	activation uint64, numBlocks int32) (*BlockChain, []*bchutil.Block) {

	t.Helper()
	params := chaincfg.RegressionNetParams
	params.Upgrade2027ActivationTime = activation
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	chain, err := New(&Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         NewMedianTime(),
		SigCache:           txscript.NewSigCache(10),
		ExcessiveBlockSize: LegacyMaxBlockSize,
		MaxReorgDepth:      depth,
		FinalizationDelay:  delay,
	})
	if err != nil {
		t.Fatal(err)
	}
	blocks := []*bchutil.Block{bchutil.NewBlock(params.GenesisBlock)}
	for h := int32(1); h <= numBlocks; h++ {
		block := procTestBlock(t, blocks[h-1].MsgBlock(), h-1,
			childSubsidy(chain, blocks[h-1]), params.PowLimitBits, nil)
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("block %d: %v", h, err)
		}
		blocks = append(blocks, block)
	}
	return chain, blocks
}

// childSubsidy returns the subsidy of a block that extends the passed one.
func childSubsidy(chain *BlockChain, parent *bchutil.Block) int64 {
	node := chain.index.LookupNode(parent.Hash())
	interval := int64(node.nextInterval)
	return CalcBlockSubsidy(node.ticks+interval, interval, chain.chainParams)
}

// TestFinalization ensures blocks deeper than the maximum reorg depth are
// finalized once known for the finalization delay, and that blocks forking
// the chain below the finalized block are rejected.
func TestFinalization(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Nanosecond)
	params := chain.chainParams
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[3].Hash() {
		t.Fatalf("finalized %v, want block 3", fin)
	}

	// A fork from block 2 is below the finalized block 3, one from block 3
	// is not.
	fork := procTestBlock(t, blocks[2].MsgBlock(), 2,
		legacySubsidy(3, params)-1, params.PowLimitBits, nil)
	_, _, err := chain.ProcessBlock(fork, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) || rerr.ErrorCode != ErrForkBeforeFinalized {
		t.Fatalf("fork below the finalized block: got %v, want "+
			"ErrForkBeforeFinalized", err)
	}
	fork = procTestBlock(t, blocks[3].MsgBlock(), 3,
		legacySubsidy(4, params)-1, params.PowLimitBits, nil)
	if _, _, err := chain.ProcessBlock(fork, BFNone); err != nil {
		t.Fatalf("fork above the finalized block: %v", err)
	}

	// Invalidating the finalized block moves finalization back.
	if err := chain.InvalidateBlock(blocks[3].Hash()); err != nil {
		t.Fatal(err)
	}
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v after invalidating it, want block 2", fin)
	}
}

// TestFinalizationUpgrade ensures that after the upgrade to 1-minute blocks
// the default depth finalizes the block 100 blocks deep, so a fork from 50
// blocks deep is accepted, and that a fork below the finalized block is still
// rejected.
func TestFinalizationUpgrade(t *testing.T) {
	// Blocks are a minute apart from 1700000000, so block 20 is the
	// activation block and block 21 the first 1-minute block.
	activation := uint64(1700000000 + 60*14)
	chain, blocks := upgradeFinalizationTestChain(t, DefaultMaxReorgDepth,
		time.Nanosecond, activation, 150)
	params := chain.chainParams
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[50].Hash() {
		t.Fatalf("finalized %v, want block 50", fin)
	}
	n := chain.index.LookupNode(blocks[20].Hash())
	if n.nextInterval != 60 || n.parent.nextInterval != 600 {
		t.Fatal("block 20 is not the upgrade 2027 activation block")
	}
	if d := tickDepth(chain.bestChain.Tip(), chain.finalizedNode); d != 6000 {
		t.Fatalf("finalized block is %d ticks deep, want 6000", d)
	}

	for _, test := range []struct {
		forkHeight int32
		accept     bool
	}{
		{100, true},
		{50, true},
		{49, false},
		{10, false},
	} {
		h := test.forkHeight
		fork := procTestBlock(t, blocks[h].MsgBlock(), h,
			childSubsidy(chain, blocks[h])-1, params.PowLimitBits, nil)
		_, _, err := chain.ProcessBlock(fork, BFNone)
		if test.accept {
			if err != nil {
				t.Fatalf("fork from block %d: %v", h, err)
			}
			continue
		}
		var rerr RuleError
		if !errors.As(err, &rerr) || rerr.ErrorCode != ErrForkBeforeFinalized {
			t.Fatalf("fork from block %d: got %v, want "+
				"ErrForkBeforeFinalized", h, err)
		}
	}
}

// TestFinalizationDelay ensures nothing is finalized before the node has
// been running, and the block has been known, for the finalization delay.
func TestFinalizationDelay(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Hour)
	if chain.finalizedNode != nil {
		t.Fatal("finalized a block before the startup delay")
	}

	// Blocks received this session are not old enough, while blocks
	// loaded from disk, here only genesis, have no receive time.
	chain.startTime -= 2 * 3600
	tip := chain.bestChain.Tip()
	chain.maybeFinalize(tip, time.Now().Unix())
	if fin := chain.finalizedNode; fin == nil || fin.height != 0 {
		t.Fatalf("finalized %v, want genesis", fin)
	}
	chain.index.LookupNode(blocks[1].Hash()).received = 0
	chain.index.LookupNode(blocks[2].Hash()).received = 0
	chain.maybeFinalize(tip, time.Now().Unix())
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v, want block 2", fin)
	}

	// A negative depth disables finalization.
	chain.finalizedNode = nil
	chain.maxReorgDepth = -1
	chain.maybeFinalize(tip, time.Now().Unix())
	if chain.finalizedNode != nil {
		t.Fatal("finalized a block with finalization disabled")
	}
}

// TestFinalizationMinimumChainWork ensures nothing is finalized while the
// chain has less than the minimum chain work.
func TestFinalizationMinimumChainWork(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Nanosecond)
	tip := chain.bestChain.Tip()

	chain.finalizedNode = nil
	chain.chainParams.MinimumChainWork = new(big.Int).Add(tip.workSum,
		big.NewInt(1))
	chain.maybeFinalize(tip, time.Now().Unix())
	if chain.finalizedNode != nil {
		t.Fatalf("finalized %v below the minimum chain work",
			chain.finalizedNode)
	}

	chain.chainParams.MinimumChainWork = new(big.Int).Set(tip.workSum)
	chain.maybeFinalize(tip, time.Now().Unix())
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[3].Hash() {
		t.Fatalf("finalized %v, want block 3", fin)
	}
}

// TestReconsiderBeforeFinalized ensures reconsidering a block that forks the
// chain below the finalized block moves finalization back to the fork point.
func TestReconsiderBeforeFinalized(t *testing.T) {
	chain, blocks := finalizationTestChain(t, 2, time.Nanosecond)
	params := chain.chainParams

	// Invalidate block 3 and finalize a fork from block 2 instead.
	if err := chain.InvalidateBlock(blocks[3].Hash()); err != nil {
		t.Fatal(err)
	}
	prev := blocks[2]
	for h := int32(3); h <= 6; h++ {
		fork := procTestBlock(t, prev.MsgBlock(), h-1,
			legacySubsidy(h, params)-1, params.PowLimitBits, nil)
		if _, _, err := chain.ProcessBlock(fork, BFNone); err != nil {
			t.Fatalf("fork block %d: %v", h, err)
		}
		prev = fork
	}
	if fin := chain.finalizedNode; fin == nil || fin.height != 4 {
		t.Fatalf("finalized %v, want the fork block at height 4", fin)
	}

	if err := chain.ReconsiderBlock(blocks[3].Hash()); err != nil {
		t.Fatalf("reconsider block 3: %v", err)
	}
	if fin := chain.finalizedNode; fin == nil || fin.hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v after reconsidering, want block 2", fin)
	}
}

// TestFindBlockToFinalizeLongChain ensures the block to finalize is the
// newest block that is deep enough and known for the delay, also when many
// recent blocks lie between it and the tip, as during a sync.
func TestFindBlockToFinalizeLongChain(t *testing.T) {
	chain := newFakeChain(&chaincfg.RegressionNetParams)
	chain.maxReorgDepth = DefaultMaxReorgDepth
	chain.finalizationDelay = time.Hour
	now := time.Now().Unix()
	chain.startTime = now - 2*3600

	// Blocks received one second apart, the newest just now.
	const n = 20000
	node := chain.bestChain.Genesis()
	for i := 1; i <= n; i++ {
		node = newFakeNode(node, 1, chain.chainParams.PowLimitBits,
			time.Unix(int64(i), 0))
		node.received = uint32(now - n + int64(i))
		chain.bestChain.SetTip(node)
	}
	tip := chain.bestChain.Tip()

	// The reference walks down from the candidate.
	want := tip.Ancestor(tip.height - chain.maxReorgDepth)
	for now < int64(want.received)+3600 {
		want = want.parent
	}
	for i := 0; i < 2; i++ {
		chain.maybeFinalize(tip, now)
		if chain.finalizedNode != want {
			t.Fatalf("finalized %v, want height %d", chain.finalizedNode,
				want.height)
		}
	}

	// Nothing is finalized while disabled.
	chain.finalizedNode = nil
	chain.maxReorgDepth = -1
	chain.maybeFinalize(tip, now)
	if chain.finalizedNode != nil {
		t.Fatal("finalized a block with finalization disabled")
	}
}

// TestForksBeforeFinalized ensures the check agrees with walking back from the
// parent to the height of the finalized block, for parents on the best chain
// and on side chains, with the finalized block on and off the best chain.
func TestForksBeforeFinalized(t *testing.T) {
	chain := newFakeChain(&chaincfg.RegressionNetParams)
	branch := func(parent *blockNode, n int, salt int64) []*blockNode {
		nodes := make([]*blockNode, 0, n)
		for i := range n {
			parent = newFakeNode(parent, 1, 0, time.Unix(salt+int64(i), 0))
			chain.index.AddNode(parent)
			nodes = append(nodes, parent)
		}
		return nodes
	}
	genesis := chain.bestChain.Genesis()
	main := append([]*blockNode{genesis}, branch(genesis, 20, 1)...)
	chain.bestChain.SetTip(main[len(main)-1])
	nodes := slices.Concat(main, branch(main[5], 6, 1000),
		branch(main[15], 8, 2000))

	for _, fin := range nodes {
		chain.finalizedNode = fin
		for _, prev := range nodes {
			want := prev.Ancestor(fin.height) != fin
			if got := chain.forksBeforeFinalized(prev); got != want {
				t.Errorf("finalized height %d, parent height %d: "+
					"got %v, want %v", fin.height, prev.height,
					got, want)
			}
		}
	}

	chain.finalizedNode = nil
	if chain.forksBeforeFinalized(main[len(main)-1]) {
		t.Error("forks before the finalized block when none is")
	}
}

// TestFinalizeBlock ensures blocks can be finalized by hand, also on a side
// chain, which then becomes the main chain.
func TestFinalizeBlock(t *testing.T) {
	chain, blocks := finalizationTestChain(t, -1, time.Hour)
	params := chain.chainParams
	if hash := chain.FinalizedBlockHash(); hash != nil {
		t.Fatalf("finalized %v before finalizing anything", hash)
	}

	// A side chain from block 2 with less work than the main chain.
	prev := blocks[2]
	var fork []*bchutil.Block
	for h := int32(3); h <= 4; h++ {
		block := procTestBlock(t, prev.MsgBlock(), h-1,
			legacySubsidy(h, params)-1, params.PowLimitBits, nil)
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("fork block %d: %v", h, err)
		}
		fork = append(fork, block)
		prev = block
	}

	if err := chain.FinalizeBlock(blocks[2].Hash()); err != nil {
		t.Fatalf("finalize main chain block 2: %v", err)
	}
	if hash := chain.FinalizedBlockHash(); hash == nil || *hash != *blocks[2].Hash() {
		t.Fatalf("finalized %v, want block 2", hash)
	}

	// Finalizing the side chain's tip switches to it and invalidates the
	// main chain after the fork point.  Both happen before the first
	// notification, while the chain lock is released.
	var checked bool
	chain.Subscribe(func(n *Notification) {
		if n.Type != NTBlockDisconnected || checked {
			return
		}
		checked = true
		if hash := chain.FinalizedBlockHash(); hash == nil || *hash != *fork[1].Hash() {
			t.Errorf("finalized %v during the reorganization, want the "+
				"side chain block", hash)
		}
		node := chain.index.LookupNode(blocks[4].Hash())
		if !chain.index.NodeStatus(node).KnownInvalid() {
			t.Error("old main chain is not invalid during the " +
				"reorganization")
		}
	})
	if err := chain.FinalizeBlock(fork[1].Hash()); err != nil {
		t.Fatalf("finalize side chain block: %v", err)
	}
	if tip := chain.bestChain.Tip(); tip.hash != *fork[1].Hash() {
		t.Fatalf("tip %v, want the finalized side chain block", tip.hash)
	}
	if hash := chain.FinalizedBlockHash(); hash == nil || *hash != *fork[1].Hash() {
		t.Fatalf("finalized %v, want the side chain block", hash)
	}
	if !checked {
		t.Fatal("no block was disconnected")
	}
	node := chain.index.LookupNode(blocks[3].Hash())
	if !chain.index.NodeStatus(node).KnownInvalid() {
		t.Fatal("old main chain block after the fork point is not invalid")
	}

	// Blocks that conflict with the finalized block are refused, and
	// ancestors of it are already final.
	if err := chain.FinalizeBlock(blocks[5].Hash()); err == nil {
		t.Fatal("finalized a block that conflicts with the finalized block")
	}
	if err := chain.FinalizeBlock(blocks[1].Hash()); err != nil {
		t.Fatalf("finalize an ancestor of the finalized block: %v", err)
	}
	if hash := chain.FinalizedBlockHash(); *hash != *fork[1].Hash() {
		t.Fatalf("finalized %v after finalizing an ancestor", hash)
	}
	child := procTestBlock(t, blocks[5].MsgBlock(), 5,
		legacySubsidy(6, params), params.PowLimitBits, nil)
	_, _, err := chain.ProcessBlock(child, BFNone)
	var rerr RuleError
	if !errors.As(err, &rerr) {
		t.Fatalf("block on the old main chain: got %v, want a rule error",
			err)
	}
}

// tickFinalizationChain returns a fake chain with the given finalization
// settings, no minimum chain work and the startup delay passed at now.
func tickFinalizationChain(depth int32, delay time.Duration, now int64) *BlockChain {
	params := chaincfg.RegressionNetParams
	params.MinimumChainWork = nil
	chain := newFakeChain(&params)
	chain.maxReorgDepth = depth
	chain.finalizationDelay = delay
	chain.startTime = now - int64(delay/time.Second)
	return chain
}

// TestNodeByTickDepth ensures the main chain lookup by tick depth agrees with
// the tick depth ancestor of the tip, with and without the upgrade to 1-minute
// blocks.
func TestNodeByTickDepth(t *testing.T) {
	for _, activation := range []uint64{upgrade2027Never, 1700000000 + 600*45} {
		nodes := tickTestChain(nil, regularGaps(150), activation)
		for _, tip := range []*blockNode{nodes[0], nodes[30], nodes[60],
			nodes[149]} {

			view := newChainView(tip)
			for ticks := int64(-1); ticks <= tip.ticks+601; ticks++ {
				want := tip.ancestorAtTickDepth(ticks)
				if ticks < 0 {
					want = nil
				}
				if got := view.nodeByTickDepth(ticks); got != want {
					t.Fatalf("activation %d, tip %d, depth %d: "+
						"got %v, want %v", activation,
						tip.height, ticks, got, want)
				}
			}
		}
	}
	if newChainView(nil).nodeByTickDepth(0) != nil {
		t.Fatal("found a node in an empty chain view")
	}
}

// TestFinalizationSeam ensures the block to finalize is 10 blocks deep before
// the upgrade to 1-minute blocks and 100 blocks deep after it, and the newest
// block at least 6000 ticks deep across the activation.
func TestFinalizationSeam(t *testing.T) {
	now := time.Now().Unix()
	for _, activation := range []uint64{upgrade2027Never, 1700000000 + 600*45} {
		nodes := tickTestChain(nil, regularGaps(250), activation)
		activationHeight := checkNodeTicks(t, nodes, activation)
		for _, tip := range nodes {
			chain := tickFinalizationChain(DefaultMaxReorgDepth,
				time.Hour, now)
			chain.bestChain = newChainView(tip)

			var want *blockNode
			for n := tip; n != nil; n = n.parent {
				if tip.ticks-n.ticks >= 6000 {
					want = n
					break
				}
			}
			got := chain.findBlockToFinalize(tip, now)
			if got != want {
				t.Fatalf("activation height %d, tip %d: got %v, "+
					"want %v", activationHeight, tip.height,
					got, want)
			}

			deep := int32(10)
			switch {
			case activationHeight < 0 || tip.height <= activationHeight:
			case tip.height >= activationHeight+100:
				deep = 100
			default:
				continue
			}
			if tip.height >= deep && got.height != tip.height-deep {
				t.Fatalf("activation height %d, tip %d: finalized "+
					"height %d, want %d", activationHeight,
					tip.height, got.height, tip.height-deep)
			}
		}

		// Across the seam the candidate moves up one block for every
		// ten 60-tick blocks, until it reaches the activation block.
		if activationHeight < 0 {
			continue
		}
		for _, c := range []struct{ tip, want int32 }{
			{activationHeight, activationHeight - 10},
			{activationHeight + 1, activationHeight - 10},
			{activationHeight + 10, activationHeight - 9},
			{activationHeight + 99, activationHeight - 1},
			{activationHeight + 100, activationHeight},
			{activationHeight + 101, activationHeight + 1},
		} {
			chain := tickFinalizationChain(DefaultMaxReorgDepth,
				time.Hour, now)
			tip := nodes[c.tip]
			chain.bestChain = newChainView(tip)
			got := chain.findBlockToFinalize(tip, now)
			if got == nil || got.height != c.want {
				t.Fatalf("tip %d: finalized %v, want height %d",
					c.tip, got, c.want)
			}
		}
	}
}

// finalizationVectorFile is the part of the CHIP-2025-03 parking and
// finalization vectors that finalization uses.
type finalizationVectorFile struct {
	Constants struct {
		FinalizationDepthTicks string `json:"FINALIZATION_DEPTH_TICKS"`
	} `json:"constants"`
	Schedules []struct {
		Name     string `json:"name"`
		Schedule []struct {
			Height string `json:"height"`
			Tau    string `json:"tau"`
		} `json:"schedule"`
	} `json:"schedules"`
	Finalization []struct {
		Name              string `json:"name"`
		Schedule          string `json:"schedule"`
		Tip               string `json:"tip"`
		AlreadyFinalized  string `json:"alreadyFinalized"`
		Now               int64  `json:"now"`
		FinalizationDelay int64  `json:"finalizationDelay"`
		Received          []struct {
			Height      string `json:"height"`
			ReceiveTime int64  `json:"receiveTime"`
		} `json:"received"`
		Candidate      string `json:"candidate"`
		FinalizedBlock string `json:"finalizedBlock"`
	} `json:"finalization"`
}

// TestFinalizationVectors runs the finalization vectors of CHIP-2025-03
// (fablous tests/parking_finalization).  Their schedules give the height
// after which blocks have another interval, also intervals other than 60.
// Blocks not listed as received are loaded from disk.
func TestFinalizationVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "chip202503",
		"parking_finalization", "parking_finalization.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vectors finalizationVectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	parse := func(s string) int32 {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			t.Fatalf("bad number %q: %v", s, err)
		}
		return int32(v)
	}
	height := func(s string) int32 {
		if s == "none" {
			return -1
		}
		return parse(s)
	}
	if vectors.Constants.FinalizationDepthTicks !=
		strconv.Itoa(DefaultMaxReorgDepth*legacyBlockInterval) {

		t.Fatalf("finalization depth %s ticks",
			vectors.Constants.FinalizationDepthTicks)
	}

	type change struct{ height, tau int32 }
	schedules := make(map[string][]change)
	for _, s := range vectors.Schedules {
		for _, c := range s.Schedule {
			schedules[s.Name] = append(schedules[s.Name],
				change{parse(c.Height), parse(c.Tau)})
		}
	}

	if len(vectors.Finalization) != 13 {
		t.Fatalf("%d finalization vectors, want 13",
			len(vectors.Finalization))
	}
	for _, v := range vectors.Finalization {
		t.Run(v.Name, func(t *testing.T) {
			sched, ok := schedules[v.Schedule]
			if !ok && v.Schedule != "empty" {
				t.Fatalf("unknown schedule %q", v.Schedule)
			}
			chain := tickFinalizationChain(DefaultMaxReorgDepth,
				time.Duration(v.FinalizationDelay)*time.Second, v.Now)

			// The blocks after a schedule change have its interval.
			nodes := []*blockNode{chain.bestChain.Genesis()}
			for h := int32(1); h <= parse(v.Tip); h++ {
				parent := nodes[h-1]
				for _, c := range sched {
					if parent.height >= c.height {
						parent.nextInterval = uint16(c.tau)
					}
				}
				nodes = append(nodes, newFakeNode(parent, 1, 0,
					time.Unix(int64(h), 0)))
			}
			tip := nodes[len(nodes)-1]
			chain.bestChain.SetTip(tip)
			for _, r := range v.Received {
				nodes[parse(r.Height)].received = uint32(r.ReceiveTime)
			}
			if h := height(v.AlreadyFinalized); h >= 0 {
				chain.finalizedNode = nodes[h]
			}

			candidate := chain.bestChain.nodeByTickDepth(
				int64(DefaultMaxReorgDepth) * legacyBlockInterval)
			if want := height(v.Candidate); (candidate == nil &&
				want >= 0) || (candidate != nil &&
				candidate.height != want) {

				t.Fatalf("candidate %v, want height %d",
					candidate, want)
			}
			got := chain.findBlockToFinalize(tip, v.Now)
			if want := height(v.FinalizedBlock); (got == nil &&
				want >= 0) || (got != nil && got.height != want) {

				t.Fatalf("finalized %v, want height %d", got, want)
			}
		})
	}
}
