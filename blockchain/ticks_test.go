// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// tickDepth returns how many ticks apart two blocks are.  A nil block counts
// as zero ticks.
func tickDepth(a, b *blockNode) int64 {
	var ta, tb int64
	if a != nil {
		ta = a.ticks
	}
	if b != nil {
		tb = b.ticks
	}
	if ta < tb {
		return tb - ta
	}
	return ta - tb
}

// ancestorAtTickDepth returns the most recent ancestor of node, or node
// itself, that is at least the passed number of ticks deeper than node, or
// nil when there is none.
func (node *blockNode) ancestorAtTickDepth(ticks int64) *blockNode {
	if ticks > node.ticks {
		return nil
	}
	n := node
	for n != nil && node.ticks-n.ticks < ticks {
		if n.skip != nil && node.ticks-n.skip.ticks <= ticks {
			n = n.skip
		} else {
			n = n.parent
		}
	}
	return n
}

// TestBlockNodeSize ensures the tick fields keep a block node in the 128-byte
// size class on 64-bit platforms.
func TestBlockNodeSize(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("size class pinned on 64-bit platforms only")
	}
	if size := unsafe.Sizeof(blockNode{}); size != 128 {
		t.Fatalf("block node is %d bytes, want 128", size)
	}
}

// tickVectorFile is the layout of the CHIP-2025-03 ticks.json vectors.
type tickVectorFile struct {
	Schedules []struct {
		Name     string `json:"name"`
		Schedule []struct {
			Height string `json:"height"`
			Tau    string `json:"tau"`
		} `json:"schedule"`
		Queries []struct {
			Op       string `json:"op"`
			Arg      string `json:"arg"`
			Expected string `json:"expected"`
			Note     string `json:"note"`
		} `json:"queries"`
		RoundTrips []struct {
			Height            string `json:"height"`
			ExpectedRoundTrip string `json:"expectedRoundTrip"`
		} `json:"roundTrips"`
	} `json:"schedules"`
}

// TestTickScheduleVectors runs the CHIP-2025-03 tick mapper vectors from
// fablous tests/ticks/ticks.json against TickSchedule.  Only the schedules a
// chain can have, with no interval change or a single change to 60 ticks,
// apply, and the rows that saturate at heights or ticks a block can't reach
// are skipped.
func TestTickScheduleVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "chip202503",
		"ticks", "ticks.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vectors tickVectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}

	parse := func(s string) (int64, bool) {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatalf("bad number %q: %v", s, err)
		}
		return int64(v), v <= math.MaxInt32
	}

	ran := make(map[string]int)
	for _, sv := range vectors.Schedules {
		var sched TickSchedule
		switch {
		case len(sv.Schedule) == 0:
		case len(sv.Schedule) == 1 && sv.Schedule[0].Tau == "60":
			h, ok := parse(sv.Schedule[0].Height)
			if !ok {
				continue
			}
			sched = NewTickSchedule(math.MaxInt32, int32(h), true)
		default:
			continue
		}

		for _, q := range sv.Queries {
			arg, argOK := parse(q.Arg)
			want, wantOK := parse(q.Expected)

			// The last interval carries forward, so the highest
			// height stands in for the ones above it.
			interval := int32(math.MaxInt32)
			if argOK {
				interval = int32(arg)
			}
			var got int64
			switch q.Op {
			case "GetBlockIntervalTime":
				got = sched.BlockInterval(interval)
			case "GetNextBlockIntervalTime":
				got = sched.NextBlockInterval(interval)
			case "HeightToTick":
				if !argOK {
					continue
				}
				got = sched.HeightToTicks(int32(arg))
			case "TickToHeight":
				if !wantOK || arg < 0 {
					continue
				}
				got = int64(sched.TicksToHeight(arg))
			default:
				t.Fatalf("%s: unknown op %q", sv.Name, q.Op)
			}
			if got != want {
				t.Errorf("%s: %s(%s) = %d, want %d (%s)", sv.Name,
					q.Op, q.Arg, got, want, q.Note)
			}
			ran[sv.Name]++
		}
		for _, rt := range sv.RoundTrips {
			h, _ := parse(rt.Height)
			want, _ := parse(rt.ExpectedRoundTrip)
			got := sched.TicksToHeight(sched.HeightToTicks(int32(h)))
			if int64(got) != want {
				t.Errorf("%s: round trip of %d = %d, want %d",
					sv.Name, h, got, want)
			}
			ran[sv.Name]++
		}
	}

	// Guard against the vectors changing shape and the test passing
	// without checking anything.
	for name, want := range map[string]int{
		"empty":             11,
		"chip_activation":   14,
		"height_zero_entry": 9,
	} {
		if ran[name] != want {
			t.Errorf("%s: ran %d checks, want %d", name, ran[name],
				want)
		}
	}
}

// TestTickScheduleChangeAtTen ports the pins for a schedule that changes to
// 60 ticks after block 10, and for one that never changes, from the tick
// mapper tests of the CHIP-2025-03 reference node.
func TestTickScheduleChangeAtTen(t *testing.T) {
	s := NewTickSchedule(math.MaxInt32, 10, true)
	heightTicks := []struct {
		height int32
		ticks  int64
	}{
		{0, 0},
		{1, 600},
		{9, 9 * 600},
		{10, 10 * 600},
		{11, 10*600 + 60},
		{9123456, 10*600 + 9123446*60},
	}
	for _, test := range heightTicks {
		if got := s.HeightToTicks(test.height); got != test.ticks {
			t.Errorf("HeightToTicks(%d) = %d, want %d", test.height,
				got, test.ticks)
		}
	}
	ticksHeight := []struct {
		ticks  int64
		height int32
	}{
		{0, 0},
		{1, 0},
		{599, 0},
		{600, 1},
		{601, 1},
		{9*600 - 1, 8},
		{9 * 600, 9},
		{10*600 + 50*60 - 1, 59},
		{10*600 + 50*60, 60},
		{10*600 + 50*60 + 1, 60},
	}
	for _, test := range ticksHeight {
		if got := s.TicksToHeight(test.ticks); got != test.height {
			t.Errorf("TicksToHeight(%d) = %d, want %d", test.ticks,
				got, test.height)
		}
	}
	elapsed := []struct {
		from, to int32
		ticks    int64
	}{
		{0, 0, 0},
		{0, 1, 600},
		{0, 2, 1200},
		{0, 9, 600 * 9},
		{1, 9, 600 * 8},
		{9, 99, 600*1 + 60*89},
		{99, 100, 60},
	}
	for _, test := range elapsed {
		got := s.HeightToTicks(test.to) - s.HeightToTicks(test.from)
		if got != test.ticks {
			t.Errorf("ticks from %d to %d = %d, want %d", test.from,
				test.to, got, test.ticks)
		}
	}
	nextInterval := []struct {
		height   int32
		interval int64
	}{
		{0, 600},
		{1, 600},
		{9, 600},
		{10, 60},
		{99, 60},
		{99999123, 60},
	}
	for _, test := range nextInterval {
		got := s.NextBlockInterval(test.height)
		if got != test.interval {
			t.Errorf("NextBlockInterval(%d) = %d, want %d",
				test.height, got, test.interval)
		}
	}

	// Without the change every block is 600 ticks.
	var never TickSchedule
	for _, test := range []struct {
		height int32
		ticks  int64
	}{
		{0, 0}, {1, 600}, {9, 9 * 600}, {9123456, 9123456 * 600},
	} {
		if got := never.HeightToTicks(test.height); got != test.ticks {
			t.Errorf("never: HeightToTicks(%d) = %d, want %d",
				test.height, got, test.ticks)
		}
	}
	for _, test := range []struct {
		ticks  int64
		height int32
	}{
		{0, 0}, {1, 0}, {599, 0}, {600, 1}, {601, 1}, {9*600 - 1, 8},
		{9 * 600, 9}, {9*600 + 1, 9}, {9123456*600 - 1, 9123455},
		{9123456 * 600, 9123456}, {9123456*600 + 1, 9123456},
	} {
		if got := never.TicksToHeight(test.ticks); got != test.height {
			t.Errorf("never: TicksToHeight(%d) = %d, want %d",
				test.ticks, got, test.height)
		}
	}
	for _, h := range []int32{0, 1, 99, 99999123} {
		if got := never.NextBlockInterval(h); got != 600 {
			t.Errorf("never: NextBlockInterval(%d) = %d, want 600",
				h, got)
		}
	}

	// Ticks beyond the last height saturate at it.
	for _, sched := range []TickSchedule{s, never} {
		got := sched.TicksToHeight(math.MaxInt64)
		if got != math.MaxInt32 {
			t.Errorf("TicksToHeight(MaxInt64) = %d, want %d", got,
				int32(math.MaxInt32))
		}
	}
}

// tickTestChain returns a chain of len(gaps) block nodes after parent, or
// from a genesis node when parent is nil, spaced by the passed timestamp gaps
// and using the passed activation time.
func tickTestChain(parent *blockNode, gaps []int64, activation uint64) []*blockNode {
	nodes := make([]*blockNode, 0, len(gaps))
	ts := int64(1700000000)
	if parent != nil {
		ts = parent.timestamp
	}
	for _, gap := range gaps {
		ts += gap
		header := wire.BlockHeader{
			Timestamp: time.Unix(ts, 0),
			Nonce:     testNoncePrng.Uint32(),
		}
		if parent != nil {
			header.PrevBlock = parent.hash
		}
		parent = newBlockNode(&header, parent, activation)
		nodes = append(nodes, parent)
	}
	return nodes
}

// regularGaps returns n timestamp gaps of 600 seconds, the first of them 0.
func regularGaps(n int) []int64 {
	gaps := make([]int64, n)
	for i := 1; i < n; i++ {
		gaps[i] = 600
	}
	return gaps
}

// checkNodeTicks ensures the ticks of every node of a chain follow from the
// intervals and that the activation block is the first node whose median time
// past reaches the activation time.  It returns the activation block's
// height, or -1.
func checkNodeTicks(t *testing.T, nodes []*blockNode, activation uint64) int32 {
	t.Helper()
	activationHeight := int32(-1)
	for _, node := range nodes {
		active := node.CalcPastMedianTime().Unix() >= int64(activation)
		wantNext := uint16(600)
		if active {
			wantNext = 60
			if activationHeight < 0 {
				activationHeight = node.height
			}
		}
		if node.nextInterval != wantNext {
			t.Fatalf("height %d: next interval %d, want %d",
				node.height, node.nextInterval, wantNext)
		}
		var wantTicks int64
		if node.parent != nil {
			wantTicks = node.parent.ticks + blockInterval(node)
		}
		if node.ticks != wantTicks {
			t.Fatalf("height %d: ticks %d, want %d", node.height,
				node.ticks, wantTicks)
		}
	}
	return activationHeight
}

// TestBlockNodeTicks ensures block nodes compute their ticks and the interval
// of their children from the activation time.
func TestBlockNodeTicks(t *testing.T) {
	// Without activation every block is 600 ticks.
	nodes := tickTestChain(nil, regularGaps(200), upgrade2027Never)
	if h := checkNodeTicks(t, nodes, upgrade2027Never); h != -1 {
		t.Fatalf("activated at %d", h)
	}
	for _, node := range nodes {
		if node.ticks != 600*int64(node.height) {
			t.Fatalf("height %d: ticks %d, want %d", node.height,
				node.ticks, 600*int64(node.height))
		}
	}

	// With activation at or before the genesis time, as a regtest
	// override may set, the genesis block is the activation block.
	for _, activation := range []uint64{0, 1700000000} {
		nodes = tickTestChain(nil, regularGaps(20), activation)
		if h := checkNodeTicks(t, nodes, activation); h != 0 {
			t.Fatalf("activation %d: activated at %d, want 0",
				activation, h)
		}
		if nodes[1].ticks != 60 || nodes[10].ticks != 600 {
			t.Fatalf("activation %d: ticks %d and %d, want 60 and "+
				"600", activation, nodes[1].ticks, nodes[10].ticks)
		}
	}

	// The seam: the activation block is still a 600-tick block and its
	// child is the first 60-tick block.  The median time past of block h
	// is that of block h-5 here.
	activation := uint64(1700000000 + 600*45)
	nodes = tickTestChain(nil, regularGaps(100), activation)
	ha := checkNodeTicks(t, nodes, activation)
	if ha != 50 {
		t.Fatalf("activated at %d, want 50", ha)
	}
	if got := blockInterval(nodes[ha]); got != 600 {
		t.Fatalf("activation block interval %d, want 600", got)
	}
	if got := blockInterval(nodes[ha+1]); got != 60 {
		t.Fatalf("first 1-minute block interval %d, want 60", got)
	}
	if nodes[ha].ticks != 50*600 || nodes[ha+1].ticks != 50*600+60 ||
		nodes[99].ticks != 50*600+49*60 {

		t.Fatalf("ticks %d, %d and %d around the seam", nodes[ha].ticks,
			nodes[ha+1].ticks, nodes[99].ticks)
	}

	// Irregular, sometimes very short, gaps.
	rng := rand.New(rand.NewSource(1))
	gaps := make([]int64, 500)
	for i := 1; i < len(gaps); i++ {
		gaps[i] = 1 + rng.Int63n(1200)
	}
	activation = uint64(1700000000 + 300*250)
	nodes = tickTestChain(nil, gaps, activation)
	if h := checkNodeTicks(t, nodes, activation); h <= 0 || h >= 499 {
		t.Fatalf("activated at %d", h)
	}
}

// TestTickSchedule ensures the tick schedule of every block matches the ticks
// of its ancestors, including across branches that activate at different
// heights or not at all, whichever branch the activation block cache holds.
func TestTickSchedule(t *testing.T) {
	activation := uint64(1700000000 + 600*45)
	main := tickTestChain(nil, regularGaps(100), activation)

	// Branches off the main chain: one from block 30 with 300-second gaps
	// activates later, one with 1-second gaps never does, one from block
	// 60 shares the activation block and one from genesis activates at
	// the same height on another block.
	gaps := func(n int, gap int64) []int64 {
		g := make([]int64, n)
		for i := range g {
			g[i] = gap
		}
		return g
	}
	all := append([]*blockNode{}, main...)
	all = append(all, tickTestChain(main[30], gaps(60, 300), activation)...)
	all = append(all, tickTestChain(main[30], gaps(60, 1), activation)...)
	all = append(all, tickTestChain(main[60], gaps(30, 600), activation)...)
	all = append(all, tickTestChain(main[0], gaps(80, 600), activation)...)
	b := &BlockChain{}
	rng := rand.New(rand.NewSource(2))
	for range 2000 {
		prev := all[rng.Intn(len(all))]
		sched := b.tickSchedule(prev)

		// The reference activation block is the first ancestor whose
		// children are 60-tick blocks.
		var want *blockNode
		for n := prev; n != nil && n.nextInterval == 60; n = n.parent {
			want = n
		}
		got := b.upgrade2027ActivationNode(prev)
		if got != want {
			t.Fatalf("height %d: activation block %v, want %v",
				prev.height, got, want)
		}
		ha, active := sched.Active()
		if active != (want != nil) || (want != nil && ha != want.height) {
			t.Fatalf("height %d: schedule active %v at %d, want %v",
				prev.height, active, ha, want)
		}
		if sched.NextHeight() != prev.height+1 {
			t.Fatalf("next height %d, want %d", sched.NextHeight(),
				prev.height+1)
		}
		nextTicks := prev.ticks + int64(prev.nextInterval)
		if got := sched.NextBlockTicks(); got != nextTicks {
			t.Fatalf("height %d: next block ticks %d, want %d",
				prev.height, got, nextTicks)
		}
		if got := sched.CoinTicks(mempoolHeight); got != nextTicks {
			t.Fatalf("height %d: mempool coin ticks %d, want %d",
				prev.height, got, nextTicks)
		}
		if got := sched.BlockInterval(prev.height + 1); got != int64(prev.nextInterval) {
			t.Fatalf("height %d: next block interval %d, want %d",
				prev.height, got, prev.nextInterval)
		}
		for n := prev; n != nil; n = n.parent {
			if got := sched.HeightToTicks(n.height); got != n.ticks {
				t.Fatalf("height %d: ticks of %d are %d, want %d",
					prev.height, n.height, got, n.ticks)
			}
			if got := sched.CoinTicks(n.height); got != n.ticks {
				t.Fatalf("height %d: coin ticks of %d are %d, "+
					"want %d", prev.height, n.height, got,
					n.ticks)
			}
			if got := sched.BlockInterval(n.height); got != blockInterval(n) {
				t.Fatalf("height %d: interval of %d is %d, want "+
					"%d", prev.height, n.height, got,
					blockInterval(n))
			}
			if got := sched.NextBlockInterval(n.height); got != int64(n.nextInterval) {
				t.Fatalf("height %d: next interval of %d is %d, "+
					"want %d", prev.height, n.height, got,
					n.nextInterval)
			}
			if got := sched.TicksToHeight(n.ticks); got != n.height {
				t.Fatalf("height %d: height at %d ticks is %d, "+
					"want %d", prev.height, n.ticks, got,
					n.height)
			}
		}
	}
}

// TestTickDepth ports the tick difference and tick depth ancestor tests of the
// CHIP-2025-03 reference node to a chain that crosses the activation.
func TestTickDepth(t *testing.T) {
	activation := uint64(1700000000 + 600*45)
	nodes := tickTestChain(nil, regularGaps(150), activation)
	tip := nodes[len(nodes)-1]

	if tickDepth(tip, tip) != 0 {
		t.Fatal("tick depth of a block to itself is not 0")
	}
	if tickDepth(tip, nil) != tip.ticks || tickDepth(nil, tip) != tip.ticks {
		t.Fatal("tick depth to nil is not the ticks of the block")
	}
	if tickDepth(tip, nodes[0]) != tip.ticks {
		t.Fatal("tick depth to genesis is not the ticks of the block")
	}
	var prevDepth int64
	for i, node := range nodes[:len(nodes)-1] {
		depth := tickDepth(tip, node)
		if depth != tip.ticks-node.ticks || depth != tickDepth(node, tip) {
			t.Fatalf("height %d: tick depth %d", node.height, depth)
		}
		if i > 0 && depth != prevDepth-blockInterval(node) {
			t.Fatalf("height %d: tick depth %d after %d",
				node.height, depth, prevDepth)
		}
		prevDepth = depth
	}

	if tip.ancestorAtTickDepth(tip.ticks) != nodes[0] {
		t.Fatal("ancestor at the depth of the tip's ticks is not genesis")
	}
	if tip.ancestorAtTickDepth(tip.ticks+1) != nil {
		t.Fatal("ancestor beyond genesis is not nil")
	}
	rng := rand.New(rand.NewSource(3))
	for range 500 {
		a, b := nodes[rng.Intn(len(nodes))], nodes[rng.Intn(len(nodes))]
		if a.height < b.height {
			a, b = b, a
		}
		depth := tickDepth(a, b)
		if got := a.ancestorAtTickDepth(depth); got != b {
			t.Fatalf("%d from %d: ancestor %v, want %v", depth,
				a.height, got, b)
		}
		if got := a.ancestorAtTickDepth(depth + 1); got != b.parent {
			t.Fatalf("%d from %d: ancestor %v, want %v", depth+1,
				a.height, got, b.parent)
		}
		if next := int64(b.nextInterval); depth >= next {
			want := nodes[b.height+1]
			if got := a.ancestorAtTickDepth(depth - next); got != want {
				t.Fatalf("%d from %d: ancestor %v, want %v",
					depth-next, a.height, got, want)
			}
		}
	}
}

// openTicksTestChain opens the regtest chain in dir, creating it when it does
// not exist, with the passed activation time override.
func openTicksTestChain(t *testing.T, dir string, override *uint64) (*BlockChain, database.DB, error) {
	t.Helper()
	params := chaincfg.RegressionNetParams
	db, err := database.Open(testDbType, dir, params.Net)
	if err != nil {
		db, err = database.Create(testDbType, dir, params.Net)
		if err != nil {
			t.Fatalf("failed to create database: %v", err)
		}
	}
	chain, err := New(&Config{
		DB:                        db,
		ChainParams:               &params,
		TimeSource:                NewMedianTime(),
		SigCache:                  txscript.NewSigCache(1000),
		UtxoCacheMaxSize:          250 * 1024 * 1024,
		ExcessiveBlockSize:        32000000,
		Upgrade2027ActivationTime: override,
	})
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return chain, db, nil
}

// tickSnapshot returns the ticks and next interval of every block node.
func tickSnapshot(chain *BlockChain) map[chainhash.Hash][2]int64 {
	chain.index.RLock()
	defer chain.index.RUnlock()
	nodes := make(map[chainhash.Hash][2]int64)
	for hash, node := range chain.index.index {
		nodes[hash] = [2]int64{node.ticks, int64(node.nextInterval)}
	}
	return nodes
}

// TestTicksReload ensures the block index loaded from the database has the
// ticks the nodes had when the blocks were connected, side branches included,
// and that the best state reports them.
func TestTicksReload(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	activation := uint64(1700000000 + 600*19)
	chain, db, err := openTicksTestChain(t, dir, &activation)
	if err != nil {
		t.Fatalf("failed to open chain: %v", err)
	}
	m := &ruleTestMiner{t: t, chain: chain,
		tip: chaincfg.RegressionNetParams.GenesisBlock}
	for range 40 {
		m.mustAccept(m.build(nil))
	}
	fork := chain.bestChain.NodeByHeight(30)
	m.tip, m.height = forkBlock(t, chain, fork.hash), fork.height
	for range 3 {
		cb := m.coinbase(m.height + 1)
		cb.TxOut[1] = ruleTestPadding(1)
		m.mustAccept(m.build(cb))
	}

	want := tickSnapshot(chain)
	if len(want) != 44 {
		t.Fatalf("%d nodes, want 44", len(want))
	}
	if h := checkNodeTicks(t, chain.bestChain.nodes, activation); h != 25 {
		t.Fatalf("activated at %d, want 25", h)
	}
	best := chain.BestSnapshot()
	if best.Height != 40 || best.Ticks != 25*600+15*60 ||
		best.NextBlockInterval != 60 || !best.Upgrade2027Active {

		t.Fatalf("best state at %d: ticks %d, next interval %d, active "+
			"%v", best.Height, best.Ticks, best.NextBlockInterval,
			best.Upgrade2027Active)
	}
	if ha, active := best.TickSchedule.Active(); !active || ha != 25 {
		t.Fatalf("best tick schedule active %v at %d", active, ha)
	}
	hash, height, ok := chain.Upgrade2027ActivationBlock()
	if !ok || height != 25 || hash != chain.bestChain.NodeByHeight(25).hash {
		t.Fatalf("activation block %v at %d (%v)", hash, height, ok)
	}
	tipTicks, err := chain.BlockTicksByHash(&best.Hash)
	if err != nil || tipTicks != (BlockTicks{25*600 + 15*60, 60, 60}) {
		t.Fatalf("tip ticks %+v (%v)", tipTicks, err)
	}
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatalf("flush: %v", err)
	}
	db.Close()

	chain, db, err = openTicksTestChain(t, dir, &activation)
	if err != nil {
		t.Fatalf("failed to reopen chain: %v", err)
	}
	defer db.Close()
	got := tickSnapshot(chain)
	if len(got) != len(want) {
		t.Fatalf("%d nodes after reload, want %d", len(got), len(want))
	}
	for hash, w := range want {
		if got[hash] != w {
			t.Fatalf("%v: ticks and next interval %v after reload, "+
				"want %v", hash, got[hash], w)
		}
	}
	if got := chain.BestSnapshot(); got.Ticks != best.Ticks ||
		got.TickSchedule != best.TickSchedule {

		t.Fatalf("best state ticks %d after reload, want %d", got.Ticks,
			best.Ticks)
	}
}

// forkBlock returns the stored block with the passed hash.
func forkBlock(t *testing.T, chain *BlockChain, hash chainhash.Hash) *wire.MsgBlock {
	t.Helper()
	block, err := chain.BlockByHash(&hash)
	if err != nil {
		t.Fatalf("block %v: %v", hash, err)
	}
	return block.MsgBlock()
}
