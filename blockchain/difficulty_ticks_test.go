// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
)

// asertVectorFile is the layout of fablous tests/asert/asert.json.
type asertVectorFile struct {
	PowLimit string `json:"powLimit"`
	Vectors  []struct {
		Name                 string `json:"name"`
		RefTarget            string `json:"refTarget"`
		TimeDiff             int64  `json:"nTimeDiff"`
		TickDelta            int64  `json:"tickDelta"`
		AnchorIdealBlockTime int64  `json:"anchorIdealBlockTime"`
		NextIdealBlockTime   int64  `json:"nextIdealBlockTime"`
		HalfLife             int64  `json:"nHalfLife"`
		ExpectedTarget       string `json:"expectedTarget"`
		ExpectedBits         string `json:"expectednBits"`
	} `json:"vectors"`
	Wrapper []struct {
		Name               string `json:"name"`
		AllowMinDifficulty bool   `json:"fPowAllowMinDifficultyBlocks"`
		PrevBlockTime      int64  `json:"nPrevBlockTime"`
		NextBlockTime      int64  `json:"nNextBlockTime"`
		PrevTick           int64  `json:"nPrevTick"`
		NextIdealBlockTime int64  `json:"nextIdealBlockTime"`
		Anchor             struct {
			Height            int32  `json:"nHeight"`
			Bits              string `json:"nBits"`
			PrevBlockTime     int64  `json:"nPrevBlockTime"`
			Tick              int64  `json:"nTick"`
			BlockIntervalTime int64  `json:"nBlockIntervalTime"`
		} `json:"anchor"`
		HalfLife     int64  `json:"nASERTHalfLife"`
		ExpectedBits string `json:"expectednBits"`
	} `json:"wrapper"`
}

func loadAsertVectors(t *testing.T) *asertVectorFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "chip202503",
		"asert", "asert.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vectors asertVectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	return &vectors
}

func hexToBig(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 16)
	if !ok {
		t.Fatalf("bad hex %q", s)
	}
	return n
}

func hexToBits(t *testing.T, s string) uint32 {
	t.Helper()
	n, err := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 32)
	if err != nil {
		t.Fatalf("bad bits %q: %v", s, err)
	}
	return uint32(n)
}

// TestAsertTickVectors runs the CHIP-2025-03 ASERT vectors of fablous
// tests/asert/asert.json against calcAsertTarget.
func TestAsertTickVectors(t *testing.T) {
	vectors := loadAsertVectors(t)
	powLimit := hexToBig(t, vectors.PowLimit)

	targets := make(map[string]*big.Int)
	for _, v := range vectors.Vectors {
		got := calcAsertTarget(hexToBig(t, v.RefTarget),
			v.AnchorIdealBlockTime, v.NextIdealBlockTime, v.TimeDiff,
			v.TickDelta, v.HalfLife, powLimit)
		if want := hexToBig(t, v.ExpectedTarget); got.Cmp(want) != 0 {
			t.Errorf("%s: target %064x, want %064x", v.Name, got, want)
		}
		if bits, want := BigToCompact(got), hexToBits(t, v.ExpectedBits); bits != want {
			t.Errorf("%s: bits %08x, want %08x", v.Name, bits, want)
		}
		targets[v.Name] = got
	}
	if len(targets) != 45 {
		t.Fatalf("ran %d vectors, want 45", len(targets))
	}

	// Each _60s sibling is 10 times its 600-tick vector, clamped.
	siblings := 0
	for name, got := range targets {
		base, ok := strings.CutSuffix(name, "_60s")
		if !ok || !strings.HasPrefix(name, "calc_args[") {
			continue
		}
		want := new(big.Int).Mul(targets[base], big.NewInt(10))
		if want.Cmp(powLimit) > 0 {
			want.Set(powLimit)
		}
		if got.Cmp(want) != 0 {
			t.Errorf("%s: target %064x, want 10 * %s = %064x", name,
				got, base, want)
		}
		siblings++
	}
	if siblings != 16 {
		t.Errorf("checked %d _60s siblings, want 16", siblings)
	}
	equiv := new(big.Int).Mul(targets["equiv_1M_600s"], big.NewInt(10))
	if targets["equiv_1M_60s"].Cmp(equiv) != 0 {
		t.Errorf("equiv_1M_60s is not 10 * equiv_1M_600s")
	}
}

// asertWrapperChain returns a chain whose ASERT parameters are those of a
// fablous wrapper vector.
func asertWrapperChain(powLimit *big.Int, allowMin bool, halfLife int64) *BlockChain {
	params := chaincfg.TestNet4Params
	params.PowLimit = powLimit
	params.PowLimitBits = BigToCompact(powLimit)
	params.ReduceMinDifficulty = allowMin
	params.AsertDifficultyHalflife = halfLife
	return &BlockChain{chainParams: &params}
}

// TestAsertTickWrapper runs the CHIP-2025-03 wrapper vectors of fablous
// tests/asert/asert.json, which cover the testnet minimum difficulty
// threshold and the tick delta from a non-zero anchor tick, against
// calcAsertRequiredDifficulty.
func TestAsertTickWrapper(t *testing.T) {
	vectors := loadAsertVectors(t)
	powLimit := hexToBig(t, vectors.PowLimit)
	if len(vectors.Wrapper) != 10 {
		t.Fatalf("%d wrapper vectors, want 10", len(vectors.Wrapper))
	}
	for _, w := range vectors.Wrapper {
		// The anchor ticks and interval are constants, not inputs.
		if w.Anchor.Tick != int64(w.Anchor.Height)*600 ||
			w.Anchor.BlockIntervalTime != 600 {

			t.Fatalf("%s: anchor tick %d interval %d", w.Name,
				w.Anchor.Tick, w.Anchor.BlockIntervalTime)
		}
		b := asertWrapperChain(powLimit, w.AllowMinDifficulty, w.HalfLife)
		prev := &blockNode{
			height:       w.Anchor.Height + 1,
			timestamp:    w.PrevBlockTime,
			ticks:        w.PrevTick,
			nextInterval: uint16(w.NextIdealBlockTime),
		}
		bits, err := b.calcAsertRequiredDifficulty(prev, w.Anchor.Height,
			w.Anchor.PrevBlockTime, hexToBits(t, w.Anchor.Bits),
			time.Unix(w.NextBlockTime, 0))
		if err != nil {
			t.Errorf("%s: %v", w.Name, err)
			continue
		}
		if want := hexToBits(t, w.ExpectedBits); bits != want {
			t.Errorf("%s: bits %08x, want %08x", w.Name, bits, want)
		}
	}
}

// TestCalcAsertTargetTicks ports the 1-minute assertions of the
// calculate_asert_test case of the CHIP-2025-03 reference node: a 60-tick
// target is the 600-tick target for the same schedule offset times 10.
func TestCalcAsertTargetTicks(t *testing.T) {
	powLimit := chaincfg.MainNetParams.PowLimit
	halfLife := chaincfg.MainNetParams.AsertDifficultyHalflife
	initial := new(big.Int).Rsh(powLimit, 4)
	ten := big.NewInt(10)
	calc := func(ref *big.Int, next, tDelta, tickDelta int64) *big.Int {
		return calcAsertTarget(ref, 600, next, 600+tDelta, tickDelta,
			halfLife, powLimit)
	}
	div10 := func(n *big.Int) *big.Int {
		return new(big.Int).Quo(n, ten)
	}
	check := func(name string, got, want *big.Int) {
		t.Helper()
		if got.Cmp(want) != 0 {
			t.Errorf("%s: got %064x, want %064x", name, got, want)
		}
	}

	// On schedule, the 600-tick target is the anchor target.
	check("steady", calc(initial, 600, 600, 600), initial)
	check("steady 60", div10(calc(initial, 60, 60, 60)), initial)

	// 300 seconds ahead of schedule gives the same difficulty at either
	// interval.
	early := calc(initial, 600, 600+300, 1200)
	if early.Cmp(initial) >= 0 {
		t.Fatalf("early target %064x not below %064x", early, initial)
	}
	check("early 60", div10(calc(initial, 60, 600+300, 1200)), early)

	// 30 seconds ahead raises the difficulty less, however the ticks
	// were reached.
	early30 := calc(initial, 60, 60+30, 120)
	if div10(early30).Cmp(initial) >= 0 || div10(early30).Cmp(early) <= 0 {
		t.Errorf("30 s early target %064x not between", early30)
	}
	check("early 30 after 600s", calc(initial, 60, 600*9+30, 600*9+60), early30)
	check("early 30 after 60s", calc(initial, 60, 60*9+30, 600), early30)

	// Getting 300 seconds ahead a little at a time ramps up to the same
	// difficulty.
	var prev *big.Int
	for delta, iter := int64(0), int64(0); delta < 300; iter++ {
		delta += min(1+iter*7%29, 300-delta)
		ideal := (iter + 1) * 60
		got := calc(initial, 60, ideal-delta, ideal)
		if prev != nil && got.Cmp(prev) > 0 {
			t.Fatalf("ramp step %d: target rose", iter)
		}
		if prev == nil && div10(got).Cmp(initial) > 0 {
			t.Fatalf("ramp step 0: target above the anchor target")
		}
		prev = got
	}
	check("ramp", div10(prev), early)

	// Making up the shortfall restores the anchor target.
	caughtUp := calc(initial, 60, 30+90, 120)
	if caughtUp.Cmp(early30) <= 0 {
		t.Errorf("caught up target %064x not above %064x", caughtUp, early30)
	}
	check("caught up 60", div10(caughtUp), initial)

	// Two days ahead doubles the target and two days behind halves it.
	target := new(big.Int).Rsh(powLimit, 16)
	ahead := calc(target, 60, 288*1200, 2880*60)
	check("ahead 60", div10(ahead), new(big.Int).Lsh(target, 1))
	behind := calc(new(big.Int).Lsh(target, 1), 60, 0, 2880*60)
	check("behind 60", behind, new(big.Int).Rsh(ahead, 1))
	check("behind 60 scaled", div10(behind), target)

	// Far behind, the target floors at 1 even after scaling.
	low := calcAsertTarget(powLimit, 600, 60, 0, 2*(256-33)*1440*60,
		halfLife, powLimit)
	if low.Cmp(bigOne) <= 0 || div10(low).Cmp(bigOne) != 0 {
		t.Errorf("underflow: got %064x", low)
	}
	check("underflow floor", calcAsertTarget(powLimit, 600, 60, 0,
		(2*(256-33)+2)*1440*60, halfLife, powLimit), bigOne)

	// A longer next interval divides a target that went past 2^256 from
	// powLimit, and clamps one that is only above powLimit.
	overflow := calcAsertTarget(powLimit, 60, 600, 40*halfLife+60, 0,
		halfLife, powLimit)
	check("overflow longer", overflow, new(big.Int).Quo(powLimit, ten))
	check("above limit longer", calcAsertTarget(powLimit, 60, 600,
		10*halfLife+60, 0, halfLife, powLimit), powLimit)
}

// asertTestParams returns testnet parameters with the ASERT anchor at the
// passed height, its parent at the passed time, and every block from
// height 1 on using ASERT.
func asertTestParams(anchorHeight int32, anchorParentTime int64) *chaincfg.Params {
	params := chaincfg.TestNet4Params
	params.UahfForkHeight = 0
	params.DaaForkHeight = 0
	params.AxionActivationHeight = anchorHeight
	params.AsertDifficultyAnchorHeight = anchorHeight
	params.AsertDifficultyAnchorParentTimestamp = anchorParentTime
	params.AsertDifficultyAnchorBits = 0x1b0404cb
	params.Checkpoints = nil
	return &params
}

// TestAsertAnchorTicksGuard ensures a block whose parent has fewer ticks
// than the ASERT anchor, as on a chain that activated 1-minute blocks before
// the anchor, is rejected with ErrUnexpectedDifficulty, also when the
// minimum difficulty rule would apply.  CHIP-2025-03 has no vector for it.
func TestAsertAnchorTicksGuard(t *testing.T) {
	const anchorHeight = 20
	genesis := int64(1700000000)
	params := asertTestParams(anchorHeight, genesis+600*(anchorHeight-1))
	b := newFakeChain(params)

	gaps := regularGaps(anchorHeight + 1)
	for _, test := range []struct {
		name       string
		activation uint64
		reject     bool
	}{
		{"never active", upgrade2027Never, false},
		{"active from genesis", uint64(genesis), true},
	} {
		nodes := tickTestChain(nil, gaps, test.activation)
		prev := nodes[anchorHeight]

		// Far enough ahead for the minimum difficulty.
		header := &wire.BlockHeader{
			Version:   4,
			Bits:      params.PowLimitBits,
			Timestamp: time.Unix(prev.timestamp+3600, 0),
		}
		err := b.checkBlockHeaderContext(header, prev, BFNone)
		if !test.reject {
			if err != nil {
				t.Errorf("%s: %v", test.name, err)
			}
			continue
		}
		rerr, ok := err.(RuleError)
		if !ok || rerr.ErrorCode != ErrUnexpectedDifficulty {
			t.Errorf("%s: got %v, want ErrUnexpectedDifficulty",
				test.name, err)
		}
	}

	// The guard holds up to the anchor tick itself.
	prev := &blockNode{height: anchorHeight, timestamp: genesis,
		ticks: anchorHeight * 600, nextInterval: 60}
	if _, err := b.calcAsertRequiredDifficulty(prev, anchorHeight, genesis,
		params.AsertDifficultyAnchorBits, time.Unix(genesis+60, 0)); err != nil {

		t.Errorf("at the anchor tick: %v", err)
	}
	prev.ticks -= 60
	_, err := b.calcNextRequiredDifficulty(prev, time.Unix(genesis+60, 0),
		DifficultyAsert)
	if err != errAsertAnchorTicks {
		t.Errorf("below the anchor tick: got %v", err)
	}
}

// TestAsertTicksSeam ensures the target for the first 60-tick block is 10
// times the 600-tick target with the same inputs, that the activation block
// keeps the 600-tick target, and that the minimum difficulty rule applies
// after two intervals of the new block: 1200 seconds up to the activation
// block and 120 seconds after it.
func TestAsertTicksSeam(t *testing.T) {
	genesis := int64(1700000000)
	params := asertTestParams(1, genesis)
	b := newFakeChain(params)
	activation := uint64(genesis + 600*20)

	never := tickTestChain(nil, regularGaps(40), upgrade2027Never)
	active := tickTestChain(nil, regularGaps(40), activation)
	ha := checkNodeTicks(t, active, activation)
	if ha < 2 || ha > 35 {
		t.Fatalf("activation height %d", ha)
	}

	bitsAt := func(prev *blockNode, gap int64) uint32 {
		t.Helper()
		bits, err := b.calcNextRequiredDifficulty(prev,
			time.Unix(prev.timestamp+gap, 0), DifficultyAsert)
		if err != nil {
			t.Fatalf("height %d: %v", prev.height+1, err)
		}
		return bits
	}
	target600 := func(prev *blockNode) *big.Int {
		return calcAsertTarget(CompactToBig(params.AsertDifficultyAnchorBits),
			600, 600, prev.timestamp-genesis, prev.ticks-600,
			params.AsertDifficultyHalflife, params.PowLimit)
	}

	for h := int32(1); h < 40; h++ {
		prevNever, prevActive := never[h], active[h]
		want := BigToCompact(target600(prevNever))
		if got := bitsAt(prevNever, 60); got != want {
			t.Errorf("never active, height %d: bits %08x, want %08x",
				h+1, got, want)
		}
		if h >= ha {
			want = BigToCompact(new(big.Int).Mul(target600(prevActive),
				big.NewInt(10)))
		}
		if got := bitsAt(prevActive, 60); got != want {
			t.Errorf("active, height %d: bits %08x, want %08x", h+1,
				got, want)
		}
	}

	for _, test := range []struct {
		name      string
		prev      *blockNode
		threshold int64
	}{
		{"never active after H_a", never[ha], 1200},
		{"before H_a", active[ha-1], 1200},
		{"after H_a", active[ha], 120},
		{"after H_a+1", active[ha+1], 120},
	} {
		if bitsAt(test.prev, test.threshold) == params.PowLimitBits {
			t.Errorf("%s: minimum difficulty at %d s", test.name,
				test.threshold)
		}
		if bitsAt(test.prev, test.threshold+1) != params.PowLimitBits {
			t.Errorf("%s: no minimum difficulty at %d s", test.name,
				test.threshold+1)
		}
	}
}
