// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gcash/bchd/chaincfg"
)

// TestCalcBlockSubsidyHalvings ensures the subsidy of 600-tick blocks halves
// at every SubsidyReductionInterval chunks and reaches zero after 64 halvings,
// as on the main network, regtest and another interval.
func TestCalcBlockSubsidyHalvings(t *testing.T) {
	for _, interval := range []int32{210000, 150, 1000} {
		params := chaincfg.Params{SubsidyReductionInterval: interval}
		prev := int64(2 * baseSubsidy)
		for halvings := int64(0); halvings < 64; halvings++ {
			// The block after the first block of the era.
			ticks := halvings*int64(interval)*600 + 600
			subsidy := CalcBlockSubsidy(ticks, 600, &params)
			if subsidy > baseSubsidy || subsidy != prev/2 {
				t.Fatalf("interval %d, halving %d: subsidy %d, want %d",
					interval, halvings, subsidy, prev/2)
			}
			prev = subsidy
		}
		ticks := 64*int64(interval)*600 + 600
		if subsidy := CalcBlockSubsidy(ticks, 600, &params); subsidy != 0 {
			t.Fatalf("interval %d: subsidy %d after 64 halvings",
				interval, subsidy)
		}
	}
}

// TestCalcBlockSubsidyLimit ensures the subsidies of 14 million 10-minute
// blocks, and of 1-minute blocks over the same ticks, add up to the totals of
// CHIP-2025-03.  The 1-minute total is 0.2499 BCH short, as each block's
// subsidy is truncated.
func TestCalcBlockSubsidyLimit(t *testing.T) {
	params := &chaincfg.MainNetParams
	const end = 14000000 * 600
	tests := []struct {
		interval int64
		want     int64
	}{
		{600, 2099999997690000},
		{60, 2099999972700000},
	}
	for _, test := range tests {
		// Count each 600,000 ticks at the subsidy of their first block,
		// the sampling the totals were taken with, so the last block of
		// each era counts at that era's subsidy.
		const step = 1000 * 600
		var sum int64
		for prevTicks := int64(0); prevTicks < end; prevTicks += step {
			subsidy := CalcBlockSubsidy(prevTicks+test.interval,
				test.interval, params)
			if subsidy > baseSubsidy {
				t.Fatalf("subsidy %d above the base subsidy", subsidy)
			}
			sum += step / test.interval * subsidy
		}
		if sum != test.want {
			t.Errorf("interval %d: total %d, want %d", test.interval,
				sum, test.want)
		}
	}
}

// TestCalcBlockSubsidyChunks ensures the halving count follows the 600-tick
// chunk a block's interval lies in, and not its raw ticks, on regtest's
// 150-chunk eras.  A 60-tick block ending at tick 89,460 lies in chunk 150, so
// it already pays the halved subsidy.
func TestCalcBlockSubsidyChunks(t *testing.T) {
	params := &chaincfg.RegressionNetParams
	tests := []struct {
		ticks, interval, want int64
	}{
		{0, 600, 5000000000},
		{89400, 600, 5000000000},
		{90000, 600, 2500000000},
		{89400, 60, 500000000},
		{89460, 60, 250000000},
		{90000, 60, 250000000},
		{90060, 60, 250000000},
		{179400, 60, 250000000},
		{179460, 60, 125000000},
		{180000, 60, 125000000},
		{64 * 90000, 600, 0},
		{63*90000 + 600, 600, 0},
		{1 << 40, 60, 0},
	}
	for _, test := range tests {
		got := CalcBlockSubsidy(test.ticks, test.interval, params)
		if got != test.want {
			t.Errorf("ticks %d, interval %d: subsidy %d, want %d",
				test.ticks, test.interval, got, test.want)
		}
	}
}

// subsidyVectorFile is the layout of the CHIP-2025-03 subsidy vectors.
type subsidyVectorFile struct {
	Constants map[string]string `json:"constants"`
	Schedules []struct {
		Name     string `json:"name"`
		Schedule []struct {
			Height string `json:"height"`
			Tau    string `json:"tau"`
		} `json:"schedule"`
	} `json:"schedules"`
	HalvingsLegacy       []subsidyHalvingVector `json:"halvingsLegacy"`
	HalvingsActivation   []subsidyHalvingVector `json:"halvingsActivation"`
	HalvingSpotChecks    []subsidyHalvingVector `json:"halvingSpotChecks"`
	ErasLegacy           []subsidyEraVector     `json:"erasLegacy"`
	ErasActivation       []subsidyEraVector     `json:"erasActivation"`
	CumulativeLegacy     []subsidyCumVector     `json:"cumulativeLegacy"`
	CumulativeActivation []subsidyCumVector     `json:"cumulativeActivation"`
	Totals               []struct {
		Schedule  string `json:"schedule"`
		TotalSats string `json:"totalSats"`
	} `json:"totals"`
}

// subsidyBlockVector is a block of a subsidy vector.
type subsidyBlockVector struct {
	Height        string `json:"height"`
	Tick          string `json:"tick"`
	Chunk         string `json:"chunk"`
	Tau           string `json:"tau"`
	SubsidySats   string `json:"subsidySats"`
	HalvingsCount string `json:"halvingsCount"`
}

// subsidyHalvingVector is the pair of blocks around a halving.
type subsidyHalvingVector struct {
	Schedule       string             `json:"schedule"`
	Halving        string             `json:"halving"`
	BoundaryChunk  string             `json:"boundaryChunk"`
	BoundaryTick   string             `json:"boundaryTick"`
	BoundaryHeight string             `json:"boundaryHeight"`
	Before         subsidyBlockVector `json:"before"`
	At             subsidyBlockVector `json:"at"`
}

// subsidyEraVector is the subsidy of one era.
type subsidyEraVector struct {
	Schedule              string `json:"schedule"`
	Era                   string `json:"era"`
	FirstHeight           string `json:"firstHeight"`
	LastHeight            string `json:"lastHeight"`
	BlockCount            string `json:"blockCount"`
	SubsidyFirstBlockSats string `json:"subsidyFirstBlockSats"`
	SubsidyLastBlockSats  string `json:"subsidyLastBlockSats"`
	EraTotalSats          string `json:"eraTotalSats"`
}

// subsidyCumVector is the subsidy of the eras up to and including one.
type subsidyCumVector struct {
	Schedule       string `json:"schedule"`
	Era            string `json:"era"`
	ThroughHeight  string `json:"throughHeight"`
	ThroughTick    string `json:"throughTick"`
	EraTotalSats   string `json:"eraTotalSats"`
	CumulativeSats string `json:"cumulativeSats"`
}

// subsidySchedule is a vector schedule: 600-tick blocks up to and including
// the block at height change, and tau-tick blocks after it.
type subsidySchedule struct {
	change, tau int64
}

// ticks returns the ticks of the block at the passed height.
func (s subsidySchedule) ticks(height int64) int64 {
	if height <= s.change {
		return height * 600
	}
	return s.change*600 + (height-s.change)*s.tau
}

// interval returns the target interval of the block at the passed height.
func (s subsidySchedule) interval(height int64) int64 {
	if height <= s.change {
		return 600
	}
	return s.tau
}

// lastHeightAt returns the height of the last block whose ticks are not
// above the passed ticks.
func (s subsidySchedule) lastHeightAt(ticks int64) int64 {
	if ticks <= s.change*600 {
		return ticks / 600
	}
	return s.change + (ticks-s.change*600)/s.tau
}

// total returns the sum of the subsidies of every block of the schedule.  It
// adds up runs of blocks with the same interval and halving count, checking
// that the first and last block of each run pay the same subsidy.
func (s subsidySchedule) total(t *testing.T, params *chaincfg.Params) int64 {
	t.Helper()
	cpe := int64(params.SubsidyReductionInterval)
	var total int64
	for height := int64(0); ; {
		ticks, interval := s.ticks(height), s.interval(height)
		subsidy := CalcBlockSubsidy(ticks, interval, params)
		if subsidy == 0 && (height > s.change || interval == s.tau) {
			return total
		}
		era := (ticks + 599) / 600 / cpe
		last := s.lastHeightAt(((era+1)*cpe - 1) * 600)
		if height <= s.change {
			last = min(last, s.change)
		}
		got := CalcBlockSubsidy(s.ticks(last), s.interval(last), params)
		if got != subsidy {
			t.Fatalf("subsidy %d at height %d, %d at height %d", subsidy,
				height, got, last)
		}
		total += subsidy * (last - height + 1)
		height = last + 1
	}
}

// TestCalcBlockSubsidyVectors runs the CHIP-2025-03 subsidy vectors: the
// blocks around every halving, the subsidy of every era and the totals, for
// 10-minute blocks, an activation of 1-minute blocks after block 1,000,000
// and the 3- and 1-second schedules.
func TestCalcBlockSubsidyVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "chip202503",
		"subsidy", "subsidy.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vectors subsidyVectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	num := func(s string) int64 {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("bad number %q: %v", s, err)
		}
		return v
	}

	// Every schedule of the vectors uses the main network's eras.
	params := &chaincfg.MainNetParams
	c := vectors.Constants
	if num(c["CHUNKS_PER_ERA_BCH"]) != int64(params.SubsidyReductionInterval) ||
		num(c["CHUNKS_PER_ERA_REGTEST"]) != int64(chaincfg.RegressionNetParams.SubsidyReductionInterval) ||
		num(c["BASE_SUBSIDY_BCH"])*num(c["COIN_SATS"]) != baseSubsidy {

		t.Fatalf("constants %v", c)
	}
	for _, era := range []struct {
		name string
		tau  int64
	}{
		{"LAST_NONZERO_ERA_600", 600}, {"LAST_NONZERO_ERA_60", 60},
		{"LAST_NONZERO_ERA_TAU3", 3}, {"LAST_NONZERO_ERA_TAU1", 1},
	} {
		// The last block of the last era with a subsidy, and the first
		// block after it.
		last := num(c[era.name])
		ticks := (last+1)*int64(params.SubsidyReductionInterval)*600 - 600
		if CalcBlockSubsidy(ticks, era.tau, params) == 0 ||
			CalcBlockSubsidy(ticks+era.tau, era.tau, params) != 0 {

			t.Errorf("%s: era %d is not the last with a subsidy",
				era.name, last)
		}
	}

	schedules := make(map[string]subsidySchedule)
	for _, sv := range vectors.Schedules {
		s := subsidySchedule{change: 1 << 62, tau: 600}
		switch len(sv.Schedule) {
		case 0:
		case 1:
			s = subsidySchedule{num(sv.Schedule[0].Height),
				num(sv.Schedule[0].Tau)}
		default:
			t.Fatalf("schedule %s has %d changes", sv.Name,
				len(sv.Schedule))
		}
		schedules[sv.Name] = s
	}
	schedule := func(name string) subsidySchedule {
		s, ok := schedules[name]
		if !ok {
			t.Fatalf("unknown schedule %q", name)
		}
		return s
	}

	// The 10-minute and the activation schedules are the ones a chain can
	// have, so their ticks must also be those of the tick schedule.
	tickSchedule := func(name string) (TickSchedule, bool) {
		switch name {
		case "empty":
			return TickSchedule{}, true
		case "activation_1M":
			return NewTickSchedule(1<<31-1, 1000000, true), true
		}
		return TickSchedule{}, false
	}

	checkBlock := func(name string, b subsidyBlockVector) {
		t.Helper()
		s := schedule(name)
		height, ticks, tau := num(b.Height), num(b.Tick), num(b.Tau)
		if s.ticks(height) != ticks || s.interval(height) != tau ||
			(ticks+599)/600 != num(b.Chunk) {

			t.Fatalf("%s: block %d at tick %d, interval %d", name, height,
				ticks, tau)
		}
		if ts, ok := tickSchedule(name); ok &&
			ts.HeightToTicks(int32(height)) != ticks {

			t.Fatalf("%s: tick schedule puts block %d at tick %d", name,
				height, ts.HeightToTicks(int32(height)))
		}
		got := CalcBlockSubsidy(ticks, tau, params)
		if got != num(b.SubsidySats) {
			t.Errorf("%s: block %d at tick %d: subsidy %d, want %s", name,
				height, ticks, got, b.SubsidySats)
		}
		halvings := num(b.HalvingsCount)
		if halvings < 64 && (baseSubsidy>>halvings)*tau/600 != got {
			t.Errorf("%s: block %d: subsidy %d is not that of %d "+
				"halvings", name, height, got, halvings)
		}
	}

	var halvings []subsidyHalvingVector
	halvings = append(halvings, vectors.HalvingsLegacy...)
	halvings = append(halvings, vectors.HalvingsActivation...)
	halvings = append(halvings, vectors.HalvingSpotChecks...)
	if len(halvings) != 72 {
		t.Fatalf("%d halving vectors, want 72", len(halvings))
	}
	for _, v := range halvings {
		checkBlock(v.Schedule, v.Before)
		checkBlock(v.Schedule, v.At)
		if num(v.At.Height) != num(v.Before.Height)+1 ||
			num(v.At.Height) != num(v.BoundaryHeight) ||
			num(v.At.Chunk) != num(v.BoundaryChunk) ||
			num(v.BoundaryTick) != num(v.BoundaryChunk)*600 ||
			num(v.At.HalvingsCount) != num(v.Halving) {

			t.Errorf("%s: halving %s vector is inconsistent",
				v.Schedule, v.Halving)
		}
	}

	// Add up every block of the 10-minute and activation schedules.
	if testing.Short() {
		t.Log("skipping the era sums in short mode")
	} else {
		var eras []subsidyEraVector
		eras = append(eras, vectors.ErasLegacy...)
		eras = append(eras, vectors.ErasActivation...)
		sums := make(map[string]int64)
		for _, v := range eras {
			s := schedule(v.Schedule)
			first, last := num(v.FirstHeight), num(v.LastHeight)
			if last-first+1 != num(v.BlockCount) ||
				first != sums[v.Schedule+"/next"] {

				t.Fatalf("%s: era %s does not follow the last one",
					v.Schedule, v.Era)
			}
			var sum int64
			for h := first; h <= last; h++ {
				sum += CalcBlockSubsidy(s.ticks(h), s.interval(h), params)
			}
			firstSubsidy := CalcBlockSubsidy(s.ticks(first),
				s.interval(first), params)
			lastSubsidy := CalcBlockSubsidy(s.ticks(last),
				s.interval(last), params)
			era := num(v.Era)
			cpe := int64(params.SubsidyReductionInterval)
			if sum != num(v.EraTotalSats) ||
				firstSubsidy != num(v.SubsidyFirstBlockSats) ||
				lastSubsidy != num(v.SubsidyLastBlockSats) ||
				(s.ticks(first)+599)/600/cpe != era ||
				(s.ticks(last)+599)/600/cpe != era ||
				(s.ticks(last+1)+599)/600/cpe != era+1 {

				t.Errorf("%s: era %d: total %d, first %d, last %d", v.Schedule,
					era, sum, firstSubsidy, lastSubsidy)
			}
			sums[v.Schedule+"/next"] = last + 1
			sums[v.Schedule+"/"+v.Era] = sum
		}

		var cumulative []subsidyCumVector
		cumulative = append(cumulative, vectors.CumulativeLegacy...)
		cumulative = append(cumulative, vectors.CumulativeActivation...)
		for _, v := range cumulative {
			sums[v.Schedule] += sums[v.Schedule+"/"+v.Era]
			s := schedule(v.Schedule)
			through := num(v.ThroughHeight)
			if sums[v.Schedule] != num(v.CumulativeSats) ||
				sums[v.Schedule+"/"+v.Era] != num(v.EraTotalSats) ||
				s.ticks(through) != num(v.ThroughTick) {

				t.Errorf("%s: through era %s: total %d, want %s",
					v.Schedule, v.Era, sums[v.Schedule], v.CumulativeSats)
			}
		}
	}

	if len(vectors.Totals) != 4 {
		t.Fatalf("%d totals, want 4", len(vectors.Totals))
	}
	for _, v := range vectors.Totals {
		got := schedule(v.Schedule).total(t, params)
		if got != num(v.TotalSats) {
			t.Errorf("%s: total %d, want %s", v.Schedule, got, v.TotalSats)
		}
	}
}

// TestUpgrade2027Subsidy ensures connected blocks pay at most the subsidy of
// their interval on regtest: 50 BCH up to and including the activation block,
// 5 BCH after it, and 2.5 BCH from block 231, whose interval lies in the first
// chunk of the second era.  A chain that never activates halves at block 150.
func TestUpgrade2027Subsidy(t *testing.T) {
	tests := []struct {
		name       string
		activation uint64
		subsidies  map[int32]int64
	}{{
		// The miner stamps block h with 1700000000+600*(h-1), so block
		// 140 is the first whose median time past reaches this time.
		name:       "active",
		activation: 1700000000 + 600*134,
		subsidies: map[int32]int64{
			140: 5000000000, 141: 500000000, 230: 500000000,
			231: 250000000,
		},
	}, {
		name:       "never",
		activation: upgrade2027Never,
		subsidies: map[int32]int64{
			141: 5000000000, 149: 5000000000, 150: 2500000000,
			231: 2500000000,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := chaincfg.RegressionNetParams
			params.Upgrade2027ActivationTime = test.activation
			m := newRuleTestMiner(t, &params)
			for m.height < 231 {
				height := m.height + 1
				want, ok := test.subsidies[height]
				if !ok {
					m.mustAccept(m.build(nil))
					continue
				}
				cb := m.coinbase(height)
				cb.TxOut[0].Value = want + 1
				err := m.accept(m.build(cb))
				requireRuleError(t, err, ErrBadCoinbaseValue)
				cb = m.coinbase(height)
				cb.TxOut[0].Value = want
				m.mustAccept(m.build(cb))
			}
			_, height, active := m.chain.Upgrade2027ActivationBlock()
			if active != (test.activation != upgrade2027Never) ||
				active && height != 140 {

				t.Fatalf("activation block at %d (%v)", height, active)
			}
		})
	}
}
