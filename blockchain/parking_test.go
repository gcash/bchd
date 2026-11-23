// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/gcash/bchd/chaincfg"
)

// vectorWork parses a hex chain work of the vectors.
func vectorWork(t *testing.T, s string) *big.Int {
	t.Helper()
	w, ok := new(big.Int).SetString(s, 16)
	if !ok {
		t.Fatalf("bad work %q", s)
	}
	return w
}

// TestParkingVectors runs the parking vectors of CHIP-2025-03 (fablous
// tests/parking_finalization) against the work a parked chain needs, and,
// where the schedule is one a real chain has, against the whole rule.
func TestParkingVectors(t *testing.T) {
	vectors, schedules := loadCHIPVectors(t)
	for name, want := range map[string]string{
		"BASE_BLOCK_INTERVAL_TIME": strconv.Itoa(legacyBlockInterval),
		"PARKING_LOCK_IN_TICKS":    strconv.Itoa(parkingLockInTicks),
		"PARKING_SUB_CAP_SHIFT":    strconv.Itoa(parkingSubCapShift),
		"UNIT_OF_SPAN_WORK":        "5760000",
	} {
		if got := vectors.Constants[name]; got != want {
			t.Fatalf("constant %s is %q, want %q", name, got, want)
		}
	}
	unit, _ := new(big.Int).SetString(vectors.Constants["UNIT_OF_SPAN_WORK"], 10)

	if len(vectors.Parking) != 79 {
		t.Fatalf("%d parking vectors, want 79", len(vectors.Parking))
	}
	counts := make(map[string]int)
	for _, v := range vectors.Parking {
		counts[v.Schedule]++
	}
	for name, want := range map[string]int{"activation_1K": 53,
		"fine1": 12, "empty": 8, "mixed300": 6} {

		if counts[name] != want {
			t.Fatalf("%d vectors with schedule %s, want %d",
				counts[name], name, want)
		}
	}

	genesis := newFakeChain(&chaincfg.RegressionNetParams).bestChain.Genesis()
	for _, v := range vectors.Parking {
		t.Run(v.Name, func(t *testing.T) {
			sched, ok := schedules[v.Schedule]
			if !ok {
				t.Fatalf("unknown schedule %q", v.Schedule)
			}
			genesis := *genesis
			nodes := scheduledNodes(&genesis, sched,
				vectorInt32(t, v.DefenderTip))
			fork := nodes[vectorInt32(t, v.Fork)]
			tip := nodes[len(nodes)-1]

			// The schedule gives the tick depth, which the rule
			// reads on the defending branch.
			d := tip.ticks - fork.ticks
			if want, _ := strconv.ParseInt(v.TickDepth, 10, 64); d != want {
				t.Fatalf("tick depth %d, want %d", d, want)
			}

			forkWork := vectorWork(t, v.ForkChainWork)
			tipWork := vectorWork(t, v.TipChainWork)
			competing := vectorWork(t, v.CompetingChainWork)
			work := new(big.Int).Sub(tipWork, forkWork)
			if work.Cmp(vectorWork(t, v.WorkSinceFork)) != 0 {
				t.Fatalf("work since the fork %v", work)
			}
			competingWork := new(big.Int).Sub(competing, forkWork)
			if competingWork.Cmp(vectorWork(t, v.CompetingWorkSinceFork)) != 0 {
				t.Fatalf("competing work since the fork %v",
					competingWork)
			}

			required := unparkRequiredWork(tipWork, forkWork, d)
			if required.Cmp(vectorWork(t, v.RequiredWork)) != 0 {
				t.Fatalf("required work %x, want %s", required,
					v.RequiredWork)
			}
			penalty := new(big.Int).Sub(required, tipWork)
			if penalty.Cmp(vectorWork(t, v.Penalty)) != 0 {
				t.Fatalf("penalty %x, want %s", penalty, v.Penalty)
			}
			den := "1"
			if d < parkingLockInTicks {
				den = "8"
			}
			if v.PenaltyNum != "1" || v.PenaltyDen != den {
				t.Fatalf("penalty %s/%s, want 1/%s", v.PenaltyNum,
					v.PenaltyDen, den)
			}
			slack := new(big.Int).Sub(competingWork,
				new(big.Int).Add(work, penalty))
			if slack.String() != v.SlackVsThreshold {
				t.Fatalf("slack %v, want %s", slack,
					v.SlackVsThreshold)
			}
			if got := competing.Cmp(required) > 0; got != v.Accept {
				t.Fatalf("accepted %v, want %v", got, v.Accept)
			}

			// The whole rule, with the chain work of the blocks of
			// the defending branch spread evenly over it.
			span := int64(tip.height - fork.height)
			for i, n := range nodes[fork.height:] {
				share := new(big.Int).Mul(work, big.NewInt(int64(i)))
				share.Quo(share, big.NewInt(span))
				n.workSum = share.Add(share, forkWork)
			}
			candidate := &blockNode{workSum: competing}
			got := parkedChainHasEnoughWorkToUnpark(tip, fork, candidate)
			switch v.Schedule {
			case "activation_1K":
				// Every defending tip is past the activation
				// block, so the rule after the upgrade applies.
				if got != v.Accept {
					t.Fatalf("unparked %v, want %v", got, v.Accept)
				}

			case "empty":
				// A chain that never activates takes the legacy
				// rule, which asks for more at depths 1 to 3 and
				// the same at depth 4.
				if span*unit.Int64() != work.Int64() {
					t.Fatalf("work since the fork %v is not "+
						"%d units", work, span)
				}
				want := v.Accept && span >= 4
				if got != want {
					t.Fatalf("unparked %v by the legacy rule, "+
						"want %v", got, want)
				}

			default:
				// The other schedules have intervals other than
				// 60 ticks, which no chain has, so they pin the
				// required work only.
			}
		})
	}
}

// legacyUnparks returns whether a parked chain with work e past the fork
// is unparked when the tip is depth blocks of 100 work each past the fork.
// With upgraded, the tip's parent is past the upgrade to 1-minute blocks.
func legacyUnparks(depth int, e int64, upgraded bool) bool {
	nodes := make([]*blockNode, depth+1)
	for i := range nodes {
		nodes[i] = &blockNode{height: int32(i),
			workSum:      big.NewInt(100 * int64(i)),
			nextInterval: legacyBlockInterval}
		if i > 0 {
			nodes[i].parent = nodes[i-1]
		}
	}
	tip := nodes[depth]
	if upgraded && tip.parent != nil {
		tip.parent.nextInterval = upgrade2027BlockInterval
	}
	candidate := &blockNode{workSum: big.NewInt(e)}
	return parkedChainHasEnoughWorkToUnpark(tip, nodes[0], candidate)
}

// TestLegacyUnparkRequiredWork ensures a parked chain needs half a block's
// work more than the tip, of the first block at depths 1 and 2 and of the
// first two at depth 3, and twice the tip's work since the fork deeper,
// before the upgrade to 1-minute blocks applies to the tip.
func TestLegacyUnparkRequiredWork(t *testing.T) {
	for _, test := range []struct {
		depth int
		e     int64
		want  bool
	}{
		{1, 100 + 50, false},
		{1, 100 + 51, true},
		{2, 200 + 50, false},
		{2, 200 + 51, true},
		{3, 300 + 100, false},
		{3, 300 + 101, true},
		{4, 400 + 400, false},
		{4, 400 + 401, true},
		{10, 1000 + 1000, false},
		{10, 1000 + 1001, true},
	} {
		if got := legacyUnparks(test.depth, test.e, false); got != test.want {
			t.Errorf("depth %d, work %d: unparked %v, want %v",
				test.depth, test.e, got, test.want)
		}
	}

	// Once the tip's parent is past the upgrade, an eighth more work is
	// enough at a depth of fewer than 2400 ticks.
	for _, test := range []struct {
		depth int
		e     int64
		want  bool
	}{
		{4, 400 + 50, false},
		{4, 400 + 51, true},
		{1, 100 + 12, false},
		{1, 100 + 13, true},
	} {
		if got := legacyUnparks(test.depth, test.e, true); got != test.want {
			t.Errorf("upgraded depth %d, work %d: unparked %v, "+
				"want %v", test.depth, test.e, got, test.want)
		}
	}
}

// TestUnparkUpgradeSwitch ensures the activation block as the tip still takes
// the legacy rule and the block after it the rule after the upgrade, on
// nodes whose intervals come from the schedule, and that a parked child of
// the tip is unparked by either rule.
func TestUnparkUpgradeSwitch(t *testing.T) {
	// Block 10 is the activation block: its children are 60 ticks apart.
	genesis := *newFakeChain(&chaincfg.RegressionNetParams).bestChain.Genesis()
	nodes := scheduledNodes(&genesis, []scheduleChange{{10, 60}}, 12)
	for i, n := range nodes {
		n.workSum = big.NewInt(100 * int64(i))
	}
	if !isUpgrade2027Active(nodes[10]) || isUpgrade2027Active(nodes[9]) {
		t.Fatal("block 10 is not the activation block")
	}
	unparks := func(tip, fork *blockNode, e int64) bool {
		candidate := &blockNode{workSum: new(big.Int).Add(fork.workSum,
			big.NewInt(e))}
		return parkedChainHasEnoughWorkToUnpark(tip, fork, candidate)
	}

	for _, test := range []struct {
		tip, fork int
		e         int64
		want      bool
	}{
		// Depth 1: half a block before, an eighth after.
		{10, 9, 100 + 50, false},
		{10, 9, 100 + 51, true},
		{11, 10, 100 + 12, false},
		{11, 10, 100 + 13, true},
		{10, 9, 100 + 13, false},

		// Depth 4: twice the work before, an eighth after, as the
		// tip is 1860 ticks past the fork.
		{10, 6, 400 + 400, false},
		{10, 6, 400 + 401, true},
		{11, 7, 400 + 50, false},
		{11, 7, 400 + 51, true},
		{10, 6, 400 + 51, false},

		// A parked child of the tip only needs more work than the tip.
		{10, 10, 0, false},
		{10, 10, 1, true},
		{11, 11, 0, false},
		{11, 11, 1, true},
	} {
		tip, fork := nodes[test.tip], nodes[test.fork]
		if got := unparks(tip, fork, test.e); got != test.want {
			t.Errorf("tip %d, fork %d, work %d: unparked %v, want %v",
				test.tip, test.fork, test.e, got, test.want)
		}
	}
	if d := nodes[11].ticks - nodes[7].ticks; d != 1860 {
		t.Fatalf("tick depth %d, want 1860", d)
	}
}

// tickUnparks returns whether a parked chain with work e past the fork is
// unparked when the tip is an upgraded block d ticks and work past the fork.
func tickUnparks(d int64, work, e, forkWork *big.Int) bool {
	fork := &blockNode{ticks: 600000000, workSum: forkWork}
	prev := &blockNode{nextInterval: upgrade2027BlockInterval}
	tip := &blockNode{parent: prev, ticks: fork.ticks + d,
		workSum: new(big.Int).Add(forkWork, work)}
	candidate := &blockNode{workSum: new(big.Int).Add(forkWork, e)}
	return parkedChainHasEnoughWorkToUnpark(tip, fork, candidate)
}

// TestUnparkRequiredWork ensures a parked chain needs more than an eighth
// more work than the tip since the fork short of 2400 ticks past it, and
// more than twice from there on, also for work that doesn't fit 64 bits.
func TestUnparkRequiredWork(t *testing.T) {
	// The work at which a chain stays parked and one more unparks.
	bigInt := func(s string) *big.Int {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			t.Fatalf("bad number %q", s)
		}
		return n
	}
	zero := new(big.Int)
	for _, test := range []struct {
		d       int64
		work, p string
	}{
		{600, "5760000", "720000"},
		{1200, "11520000", "1440000"},
		{1800, "17280000", "2160000"},
		{2400, "23040000", "23040000"},
		{60, "5760001", "720000"},
		{120, "11520000", "1440000"},
		{180, "17280000", "2160000"},
		{420, "40320000", "5040000"},
		{480, "46080000", "5760000"},
		{600, "57600000", "7200000"},
		{1200, "115200000", "14400000"},
		{1800, "172800000", "21600000"},
		{2340, "224640007", "28080000"},
		{2400, "230400000", "230400000"},
		{120, "16", "2"},
		{120, "17", "2"},
		{120, "23", "2"},
		{120, "7", "0"},
		{700, "8590065666", "1073758208"},
		{120, "16000000000000000000", "2000000000000000000"},
		{1, "5760007", "720000"},
		{2399, "5760001", "720000"},
		{2400, "5760001", "5760001"},
		{2401, "5760001", "5760001"},
		{60, "5760000", "720000"},
		{660, "5760000", "720000"},
		{1320, "11520003", "1440000"},
		{1860, "23040001", "2880000"},
		{2460, "28800001", "28800001"},
		{2100, "40320005", "5040000"},
		{2400, "40320005", "40320005"},
	} {
		work := bigInt(test.work)
		e := new(big.Int).Add(work, bigInt(test.p))
		if tickUnparks(test.d, work, e, zero) {
			t.Errorf("d %d, work %s: unparked with %v", test.d,
				test.work, e)
		}
		e.Add(e, big.NewInt(1))
		if !tickUnparks(test.d, work, e, zero) {
			t.Errorf("d %d, work %s: parked with %v", test.d,
				test.work, e)
		}
	}

	// Same work, only the depth differs.
	for _, test := range []struct {
		d       int64
		work, e int64
		want    bool
	}{
		{2399, 5760001, 8640000, true},
		{2400, 5760001, 8640000, false},
		{1860, 23040000, 34560000, true},
		{2460, 23040000, 34560000, false},

		// Two blocks each, the parked chain with 3% and 15% more work.
		{120, 11520000, 11865600, false},
		{120, 11520000, 13248000, true},
	} {
		got := tickUnparks(test.d, big.NewInt(test.work),
			big.NewInt(test.e), zero)
		if got != test.want {
			t.Errorf("d %d, work %d, competing %d: unparked %v, "+
				"want %v", test.d, test.work, test.e, got, test.want)
		}
	}

	// Large work: 2^200 + 7, and 2^255 - 1.
	one := big.NewInt(1)
	shl := func(n uint) *big.Int { return new(big.Int).Lsh(one, n) }
	add := func(xs ...*big.Int) *big.Int {
		sum := new(big.Int)
		for _, x := range xs {
			sum.Add(sum, x)
		}
		return sum
	}
	big200 := add(shl(200), big.NewInt(7))
	big255 := new(big.Int).Sub(shl(255), one)
	for _, test := range []struct {
		d       int64
		work, e *big.Int
		want    bool
	}{
		{120, big200, add(big200, shl(197)), false},
		{120, big200, add(big200, shl(197), one), true},
		{120, big255, new(big.Int).Sub(add(big255, shl(252)), one), false},
		{120, big255, add(big255, shl(252)), true},
		{2400, big255, add(big255, big255), false},
		{2400, big255, add(big255, big255, one), true},
	} {
		if got := tickUnparks(test.d, test.work, test.e, zero); got != test.want {
			t.Errorf("d %d, work %v, competing %v: unparked %v, "+
				"want %v", test.d, test.work, test.e, got, test.want)
		}
	}

	// The work below the fork does not matter.
	forkWork := shl(248)
	work := big.NewInt(11520000)
	if tickUnparks(120, work, big.NewInt(11520000+1440000), forkWork) {
		t.Error("unparked with the work of the tip and the penalty")
	}
	if !tickUnparks(120, work, big.NewInt(11520000+1440001), forkWork) {
		t.Error("parked with more than the work of the tip and the penalty")
	}
	if tickUnparks(120, work, big.NewInt(11520000+1440000),
		add(forkWork, one)) {

		t.Error("unparked with the work of the tip and the penalty")
	}
}
