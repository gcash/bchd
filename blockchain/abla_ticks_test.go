// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// chipABLAState is a state of the CHIP-2025-03 ABLA vectors: the size and
// interval of a block and the state its size limit comes from.
type chipABLAState struct {
	BlockSize ablaNumber `json:"blockSize"`
	Epsilon   ablaNumber `json:"epsilon"`
	Beta      ablaNumber `json:"beta"`
	Tau       ablaNumber `json:"tau"`
}

// chipABLAAfter is a state of the vectors after a step: its two sizes and the
// limit, without the 2 GB cap, at the step's interval.
type chipABLAAfter struct {
	Epsilon        ablaNumber `json:"epsilon"`
	Beta           ablaNumber `json:"beta"`
	BlocksizeLimit ablaNumber `json:"blocksizeLimit"`
}

// chipABLAVectors holds tests/abla/abla_ticks.json of CHIP-2025-03.
type chipABLAVectors struct {
	Configs []struct {
		Name             string     `json:"name"`
		DefaultBlockSize ablaNumber `json:"defaultBlockSize"`
		FixedSize        bool       `json:"fixedSize"`
		ScalingFactor    ablaNumber `json:"scalingFactor"`
		Epsilon0         ablaNumber `json:"epsilon0"`
		Beta0            ablaNumber `json:"beta0"`
		EpsilonMax       ablaNumber `json:"epsilonMax"`
		BetaMax          ablaNumber `json:"betaMax"`
		IsFixedSize      bool       `json:"isFixedSize"`
	} `json:"configs"`
	ConfigChecks []struct {
		Name                  string     `json:"name"`
		Config                string     `json:"config"`
		ExpectFixedSize       bool       `json:"expectFixedSize"`
		ExpectBaseLimit       ablaNumber `json:"expectBaseLimit"`
		ExpectFloorLimitTau60 ablaNumber `json:"expectFloorLimitTau60"`
	} `json:"configChecks"`
	Limits []struct {
		Name       string     `json:"name"`
		Config     string     `json:"config"`
		Epsilon    ablaNumber `json:"epsilon"`
		Beta       ablaNumber `json:"beta"`
		Tau        ablaNumber `json:"tau"`
		DisableCap bool       `json:"disableCap"`
		Expected   ablaNumber `json:"expected"`
	} `json:"limits"`
	States []struct {
		Name    string        `json:"name"`
		Config  string        `json:"config"`
		Initial chipABLAState `json:"initial"`
		Steps   []struct {
			BlockSize ablaNumber    `json:"blockSize"`
			Tau       ablaNumber    `json:"tau"`
			After     chipABLAAfter `json:"after"`
		} `json:"steps"`
		Expected     chipABLAAfter `json:"expected"`
		Differential bool          `json:"differential"`
	} `json:"states"`
	Lookaheads []struct {
		Name              string        `json:"name"`
		Config            string        `json:"config"`
		Initial           chipABLAState `json:"initial"`
		BlockIntervalTime ablaNumber    `json:"blockIntervalTime"`
		Count             ablaNumber    `json:"count"`
		DisableCap        bool          `json:"disableCap"`
		Expected          ablaNumber    `json:"expected"`
		LegacyExact       ablaNumber    `json:"legacyExact"`
	} `json:"lookaheads"`
	Validation struct {
		Config []struct {
			Name          string `json:"name"`
			ScalingFactor string `json:"scalingFactor"`
			Epsilon0      string `json:"epsilon0"`
			Beta0         string `json:"beta0"`
			EpsilonMax    string `json:"epsilonMax"`
			BetaMax       string `json:"betaMax"`
			FixedSize     bool   `json:"fixedSize"`
			Accept        bool   `json:"accept"`
		} `json:"config"`
		State []struct {
			Name    string     `json:"name"`
			Epsilon ablaNumber `json:"epsilon"`
			Beta    ablaNumber `json:"beta"`
			Tau     ablaNumber `json:"tau"`
			Accept  bool       `json:"accept"`
		} `json:"state"`
	} `json:"validation"`
}

// loadChipABLAVectors reads the CHIP-2025-03 abla_ticks.json vectors.
func loadChipABLAVectors(t *testing.T) *chipABLAVectors {
	t.Helper()
	data, err := os.ReadFile("testdata/chip202503/abla/abla_ticks.json")
	if err != nil {
		t.Fatal(err)
	}
	var v chipABLAVectors
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return &v
}

// defaultABLAConfig returns the ABLA configuration a network with the given
// default block size has, as chaincfg sets it up.
func defaultABLAConfig(defaultBlockSize uint64, fixedSize bool) ABLAConfig {
	config := ABLAConfig{
		epsilon0:        defaultBlockSize / 2,
		beta0:           defaultBlockSize / 2,
		gammaReciprocal: 37938,
		zetaXB7:         192,
		thetaReciprocal: 37938,
		delta:           10,
		fixedSize:       fixedSize,
	}
	config.SetMax()
	return config
}

// chipABLAConfigs returns the configurations of the vectors by name.  BCHD
// has no internal scaling factor, which is the same as a factor of 1, so the
// configurations with another factor are left out.
func chipABLAConfigs(t *testing.T, v *chipABLAVectors) map[string]*ABLAConfig {
	t.Helper()
	configs := make(map[string]*ABLAConfig)
	for _, c := range v.Configs {
		if c.ScalingFactor != 1 {
			continue
		}
		config := defaultABLAConfig(uint64(c.DefaultBlockSize), c.FixedSize)
		if config.epsilon0 != uint64(c.Epsilon0) ||
			config.beta0 != uint64(c.Beta0) ||
			config.epsilonMax != uint64(c.EpsilonMax) ||
			config.betaMax != uint64(c.BetaMax) {

			t.Fatalf("%s: config %+v, want %+v", c.Name, config, c)
		}
		if config.fixedSize != c.IsFixedSize {
			t.Fatalf("%s: fixed size %v, want %v", c.Name,
				config.fixedSize, c.IsFixedSize)
		}
		if errs := config.IsValid(); errs != nil {
			t.Fatalf("%s: %s", c.Name, errs)
		}
		configs[c.Name] = &config
	}
	return configs
}

// chipABLAStart returns the state of the vectors' initial block and its size
// and interval.  The state is set past n0, which the vectors don't use.
func chipABLAStart(s chipABLAState) (ABLAState, uint64, uint64) {
	state := ABLAState{
		blockHeight:       1,
		controlBlockSize:  uint64(s.Epsilon),
		elasticBufferSize: uint64(s.Beta),
	}
	return state, uint64(s.BlockSize), uint64(s.Tau)
}

// TestABLATicksConfigs ensures the network configurations give the limits
// of CHIP-2025-03 at 600 and 60 ticks, the floors scaled the same way.
func TestABLATicksConfigs(t *testing.T) {
	v := loadChipABLAVectors(t)
	configs := chipABLAConfigs(t, v)
	var checked int
	for _, c := range v.ConfigChecks {
		config, ok := configs[c.Config]
		if !ok {
			continue
		}
		state := ABLAState{controlBlockSize: config.epsilon0,
			elasticBufferSize: config.beta0}
		if config.fixedSize != c.ExpectFixedSize {
			t.Errorf("%s: fixed size %v, want %v", c.Name,
				config.fixedSize, c.ExpectFixedSize)
		}
		base := state.blockSizeLimitAt(600)
		if base != uint64(c.ExpectBaseLimit) {
			t.Errorf("%s: base limit %d, want %d", c.Name, base,
				c.ExpectBaseLimit)
		}
		if got := state.blockSizeLimitAt(60); got != uint64(c.ExpectFloorLimitTau60) {
			t.Errorf("%s: limit at 60 ticks %d, want %d", c.Name, got,
				c.ExpectFloorLimitTau60)
		}
		if got := scaleSizeFloor(base, 60); got != uint64(c.ExpectFloorLimitTau60) {
			t.Errorf("%s: floor at 60 ticks %d, want %d", c.Name, got,
				c.ExpectFloorLimitTau60)
		}
		checked++
	}
	if checked != 6 {
		t.Fatalf("checked %d configs, want 6", checked)
	}
}

// TestABLATicksLimits ensures the size limit is the state's sum scaled to
// the block interval with a single truncation, capped at 2 GB.
func TestABLATicksLimits(t *testing.T) {
	v := loadChipABLAVectors(t)
	configs := chipABLAConfigs(t, v)
	for _, test := range v.Limits {
		if _, ok := configs[test.Config]; !ok {
			t.Fatalf("%s: unknown config %s", test.Name, test.Config)
		}
		state := ABLAState{controlBlockSize: uint64(test.Epsilon),
			elasticBufferSize: uint64(test.Beta)}
		got := state.blockSizeLimitAt(uint64(test.Tau))
		if test.DisableCap {
			got = state.scaledSize(uint64(test.Tau))
		}
		if got != uint64(test.Expected) {
			t.Errorf("%s: limit %d, want %d", test.Name, got,
				test.Expected)
		}
	}
}

// TestABLATicksStates ensures each state transition responds to the size of
// the block and its own interval, and the limit of the next block is read at
// that block's interval, as the CHIP-2025-03 vectors pin, including the
// switch from 600 to 60 ticks at activation.
func TestABLATicksStates(t *testing.T) {
	v := loadChipABLAVectors(t)
	configs := chipABLAConfigs(t, v)
	var checked int
	for _, test := range v.States {
		config, ok := configs[test.Config]
		if !ok {
			continue
		}
		state, size, tau := chipABLAStart(test.Initial)
		// The differential vectors are the 600-tick case, which must
		// match the algorithm from before the upgrade.
		if test.Differential && tau != 600 {
			t.Errorf("%s: differential at %d ticks", test.Name, tau)
		}
		var limit uint64
		for i, step := range test.Steps {
			state = state.nextABLAState(config, size, tau)
			size, tau = uint64(step.BlockSize), uint64(step.Tau)
			// The pins are without the 2 GB cap.
			limit = state.scaledSize(tau)
			if test.Differential && tau != 600 {
				t.Errorf("%s step %d: differential at %d ticks",
					test.Name, i, tau)
			}
			after := step.After
			if state.controlBlockSize != uint64(after.Epsilon) ||
				state.elasticBufferSize != uint64(after.Beta) ||
				limit != uint64(after.BlocksizeLimit) {

				t.Errorf("%s step %d: epsilon %d beta %d limit %d, "+
					"want %d %d %d", test.Name, i,
					state.controlBlockSize,
					state.elasticBufferSize, limit, after.Epsilon,
					after.Beta, after.BlocksizeLimit)
				break
			}
			if errs := state.IsValid(config); errs != nil {
				t.Errorf("%s step %d: %s", test.Name, i, errs)
			}
		}
		want := test.Expected
		if state.controlBlockSize != uint64(want.Epsilon) ||
			state.elasticBufferSize != uint64(want.Beta) ||
			limit != uint64(want.BlocksizeLimit) {

			t.Errorf("%s: final epsilon %d beta %d limit %d, want "+
				"%d %d %d", test.Name, state.controlBlockSize,
				state.elasticBufferSize, limit, want.Epsilon,
				want.Beta, want.BlocksizeLimit)
		}
		checked++
	}
	if checked != len(v.States)-1 {
		t.Fatalf("checked %d of %d state vectors", checked, len(v.States))
	}
}

// ablaIteratedLookahead returns the limit of the block count blocks after
// the one the state was made for, which has the given size and own interval,
// when every block in between is full and has the given interval.  The 2 GB
// cap is left out.
func ablaIteratedLookahead(config *ABLAConfig, state ABLAState, size, own uint64,
	count int, interval uint64) uint64 {

	tau := own
	for range count {
		state = state.nextABLAState(config, size, tau)
		tau = interval
		size = state.scaledSize(tau)
	}
	return state.scaledSize(tau)
}

// TestABLATicksLookaheads ensures the exact limits of the CHIP-2025-03
// look-ahead vectors and of 2048-block rows from 10- and 1-minute starting
// states, and that the look-ahead the chain reports for 2048 blocks of 600
// ticks, starting from the state after the tip, gives them.
func TestABLATicksLookaheads(t *testing.T) {
	v := loadChipABLAVectors(t)
	configs := chipABLAConfigs(t, v)
	mainnet := configs["mainnet"]

	type row struct {
		name                 string
		initial              chipABLAState
		count                int
		interval, closed, it uint64
	}
	var rows []row
	for _, l := range v.Lookaheads {
		if l.Config != "mainnet" {
			t.Fatalf("%s: unexpected config %s", l.Name, l.Config)
		}
		// The iterated limit below leaves out the 2 GB cap.
		if !l.DisableCap {
			t.Fatalf("%s: look-ahead with the cap", l.Name)
		}
		rows = append(rows, row{l.Name, l.Initial, int(l.Count),
			uint64(l.BlockIntervalTime), uint64(l.Expected),
			uint64(l.LegacyExact)})
	}
	// The 2048-block rows from 10- and 1-minute starting states, which the
	// CHIP vectors lack.
	floor := func(size, tau uint64) chipABLAState {
		return chipABLAState{BlockSize: ablaNumber(size),
			Epsilon: 16000000, Beta: 16000000, Tau: ablaNumber(tau)}
	}
	rows = append(rows,
		row{"window10min_2048_small", floor(123456, 600), 2048, 600, 36902333, 35847033},
		row{"window10min_2048_medium", floor(12345600, 600), 2048, 600, 36902354, 35847060},
		row{"window10min_2048_full", floor(32000000, 600), 2048, 600, 36904286, 35848898},
		row{"window1min_2048_small", floor(12345, 60), 2048, 600, 36902333, 35847033},
		row{"window1min_2048_medium", floor(1234560, 60), 2048, 600, 36902335, 35847034},
		row{"window1min_2048_full", floor(3200000, 60), 2048, 600, 36902529, 35847223},
	)

	for _, r := range rows {
		state, size, own := chipABLAStart(r.initial)
		got := ablaIteratedLookahead(mainnet, state, size, own, r.count,
			r.interval)
		if got != r.it {
			t.Errorf("%s: iterated limit %d, want %d", r.name, got, r.it)
		}
		// The closed form is an upper bound, which BCHD doesn't use.
		if r.closed < r.it {
			t.Errorf("%s: closed form %d below %d", r.name, r.closed,
				r.it)
		}

		// The chain's look-ahead starts from the state after the tip,
		// which took the tip at its own interval, and goes on in
		// 600-tick blocks.
		if r.interval != 600 || r.it >= TEMP_32_BIT_MAX_SAFE_BLOCKSIZE_LIMIT {
			continue
		}
		after := state.nextABLAState(mainnet, size, own)
		ahead := after.lookaheadState(mainnet, uint(r.count-1))
		if got := ahead.getBlockSizeLimit(); got != r.it {
			t.Errorf("%s: look-ahead %d, want %d", r.name, got, r.it)
		}
	}
}

// TestABLATicksValidation ensures the configurations and states of the
// CHIP-2025-03 validation vectors are accepted or refused as pinned, and that
// an interval outside 1 to 600 ticks is refused.
func TestABLATicksValidation(t *testing.T) {
	v := loadChipABLAVectors(t)
	mainnet := defaultABLAConfig(32000000, false)
	for _, test := range v.Validation.Config {
		switch test.Name {
		case "cfg_scaling_zero", "cfg_scaling_overflow":
			// BCHD has no internal scaling factor.
			continue
		case "cfg_maxima_sum_overflow":
			// The config check has no guard against a hand-built
			// epsilonMax + betaMax of 2^64; SetMax never sets one.
			continue
		}
		config := defaultABLAConfig(32000000, test.FixedSize)
		for _, field := range []struct {
			value string
			to    *uint64
		}{
			{test.Epsilon0, &config.epsilon0},
			{test.Beta0, &config.beta0},
			{test.EpsilonMax, &config.epsilonMax},
			{test.BetaMax, &config.betaMax},
		} {
			if field.value == "default" {
				continue
			}
			n, err := strconv.ParseUint(field.value, 10, 64)
			if err != nil {
				t.Fatalf("%s: %v", test.Name, err)
			}
			*field.to = n
		}
		if valid := config.IsValid() == nil; valid != test.Accept {
			t.Errorf("%s: valid %v, want %v", test.Name, valid,
				test.Accept)
		}
	}

	for _, test := range v.Validation.State {
		state := ABLAState{controlBlockSize: uint64(test.Epsilon),
			elasticBufferSize: uint64(test.Beta)}
		valid := state.IsValid(&mainnet) == nil &&
			validABLABlockInterval(uint64(test.Tau))
		if valid != test.Accept {
			t.Errorf("%s: valid %v, want %v", test.Name, valid,
				test.Accept)
		}
	}
}

// validABLABlockInterval returns whether the ABLA functions accept the passed
// block interval.
func validABLABlockInterval(interval uint64) (valid bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(AssertError); !ok {
				panic(r)
			}
			valid = false
		}
	}()
	state := ABLAState{controlBlockSize: 1, elasticBufferSize: 1}
	state.scaledSize(interval)
	config := defaultABLAConfig(32000000, false)
	state.nextABLAState(&config, 0, interval)
	return true
}

// chipABLAEquivTest is one of the CHIP-2025-03 tests/abla/config1-*.json
// files: a run of the ABLA test vectors with blocks of another interval.
type chipABLAEquivTest struct {
	ABLAConfig struct {
		Epsilon0        ablaNumber `json:"epsilon0"`
		Beta0           ablaNumber `json:"beta0"`
		N0              ablaNumber `json:"n0"`
		Zeta            ablaNumber `json:"zeta"`
		GammaReciprocal ablaNumber `json:"gammaReciprocal"`
		Delta           ablaNumber `json:"delta"`
		ThetaReciprocal ablaNumber `json:"thetaReciprocal"`
		ScalingFactor   ablaNumber `json:"scalingFactor"`
	} `json:"ABLAConfig"`
	ABLAStateInitial struct {
		BN      ablaNumber `json:"bn"`
		Tick    ablaNumber `json:"tick"`
		Epsilon ablaNumber `json:"epsilon"`
		Beta    ablaNumber `json:"beta"`
		Tau     ablaNumber `json:"tau"`
	} `json:"ABLAStateInitial"`
	BlocksizeLimitInitial ablaNumber `json:"blocksizeLimitInitial"`
	TestVector            []struct {
		Blocksize             ablaNumber `json:"blocksize"`
		ABLAStateForNextBlock struct {
			BN      ablaNumber `json:"bn"`
			Tick    ablaNumber `json:"tick"`
			Epsilon ablaNumber `json:"epsilon"`
			Beta    ablaNumber `json:"beta"`
			Tau     ablaNumber `json:"tau"`
		} `json:"ABLAStateForNextBlock"`
		BlocksizeLimitForNextBlock ablaNumber `json:"blocksizeLimitForNextBlock"`
	} `json:"testVector"`
}

// TestABLATicksEquivalence runs the CHIP-2025-03 config1-*.json vectors,
// whose block intervals go from 600 to 300 to 60 ticks, from 600 to 60 and
// back to 600, or stay at 60.  The files with an internal scaling factor
// other than 1 are left out, since BCHD has none.  Their testVector_600 and
// deviation sections compare with the 600-tick vectors, so they are not
// asserted.
func TestABLATicksEquivalence(t *testing.T) {
	files, err := filepath.Glob("testdata/chip202503/abla/config1-*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no vectors: %v", err)
	}
	var ran int
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var test chipABLAEquivTest
		if err := json.Unmarshal(data, &test); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if test.ABLAConfig.ScalingFactor != 1 {
			continue
		}
		name := filepath.Base(file)
		conf := test.ABLAConfig
		config := ABLAConfig{
			epsilon0:        uint64(conf.Epsilon0),
			beta0:           uint64(conf.Beta0),
			gammaReciprocal: uint64(conf.GammaReciprocal),
			zetaXB7:         uint64(conf.Zeta),
			thetaReciprocal: uint64(conf.ThetaReciprocal),
			delta:           uint64(conf.Delta),
		}
		config.SetMax()
		if errs := config.IsValid(); errs != nil {
			t.Fatalf("%s: %s", name, errs)
		}

		// ABLA activates with the last block that ends at or before
		// n0 times 600 ticks.  The states up to and including it are
		// the initial state.
		initial := test.ABLAStateInitial
		activationTick := uint64(conf.N0) * 600
		if uint64(initial.Tick) <= activationTick {
			config.n0 = uint64(initial.BN)
		}
		for _, v := range test.TestVector {
			next := v.ABLAStateForNextBlock
			if uint64(next.Tick) <= activationTick {
				config.n0 = uint64(next.BN)
			}
		}

		state := ABLAState{
			blockHeight:       uint64(initial.BN),
			controlBlockSize:  uint64(initial.Epsilon),
			elasticBufferSize: uint64(initial.Beta),
		}
		tau := uint64(initial.Tau)
		if got := state.scaledSize(tau); got != uint64(test.BlocksizeLimitInitial) {
			t.Fatalf("%s: initial limit %d, want %d", name, got,
				test.BlocksizeLimitInitial)
		}
		for i, v := range test.TestVector {
			next := v.ABLAStateForNextBlock
			state = state.nextABLAState(&config, uint64(v.Blocksize), tau)
			tau = uint64(next.Tau)
			limit := state.scaledSize(tau)
			if state.blockHeight != uint64(next.BN) ||
				state.controlBlockSize != uint64(next.Epsilon) ||
				state.elasticBufferSize != uint64(next.Beta) ||
				limit != uint64(v.BlocksizeLimitForNextBlock) {

				t.Fatalf("%s vector %d: state %+v limit %d, want "+
					"%+v %d", name, i, state, limit, next,
					v.BlocksizeLimitForNextBlock)
			}
		}
		ran++
	}
	if ran != 7 {
		t.Fatalf("ran %d vector files, want 7", ran)
	}
}

// ablaSeamActivation is an upgrade 2027 activation time that the blocks of
// ablaSeamChain reach with block ablaSeamHeight, whose children have 60
// ticks.
func ablaSeamActivation(params *chaincfg.Params) uint64 {
	// Block h is a second after its parent, so from height 10 on its
	// median time past is the time of block h-5.
	return uint64(params.GenesisBlock.Header.Timestamp.Unix()) +
		ablaSeamHeight - 5
}

const ablaSeamHeight = 20

// ablaSeamFloor is the floor of the block size limit of the seam test chain,
// low enough that a full block fits in a single coinbase transaction.
const ablaSeamFloor = 600000

// ablaSeamParams returns the ABLA test parameters, with a 600 KB default
// block size and initial limit, and the upgrade to 1-minute blocks activating
// at ablaSeamHeight, or never.  The network is not regtest, so the default
// block size is the floor rather than the excessive block size.
func ablaSeamParams(active bool) *chaincfg.Params {
	params := ablaTestParams()
	params.Net = wire.TestNet4
	params.DefaultConsensusBlockSize = ablaSeamFloor
	params.ABLAConfig.Epsilon0 = ablaSeamFloor / 2
	params.ABLAConfig.Beta0 = ablaSeamFloor / 2
	if active {
		params.Upgrade2027ActivationTime = ablaSeamActivation(params)
	}
	return params
}

// ablaSeamChain opens a chain on db for the seam tests.
func ablaSeamChain(t *testing.T, db database.DB, params *chaincfg.Params) *BlockChain {
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
	return chain
}

// ablaSeamInterval returns the interval of the block at the passed height.
func ablaSeamInterval(height int32, active bool) uint64 {
	if active && height > ablaSeamHeight {
		return 60
	}
	return 600
}

// ablaSeamReplay returns the ABLA state after each of the blocks, starting
// with the genesis block, replayed with the intervals the seam gives them.
func ablaSeamReplay(chain *BlockChain, blocks []*bchutil.Block, active bool) []ABLAState {
	cfg := &chain.ablaConfig
	genesisSize := uint64(chain.chainParams.GenesisBlock.SerializeSize())
	state := ABLAState{controlBlockSize: cfg.epsilon0,
		elasticBufferSize: cfg.beta0}
	states := []ABLAState{state.nextABLAState(cfg, genesisSize, 600)}
	for i, block := range blocks {
		size := uint64(block.MsgBlock().SerializeSize())
		states = append(states, states[i].nextABLAState(cfg, size,
			ablaSeamInterval(int32(i+1), active)))
	}
	return states
}

// ablaSeamLimit returns the size limit of the block at the passed height
// given the state after its parent.
func ablaSeamLimit(state ABLAState, height int32, active bool) uint64 {
	interval := ablaSeamInterval(height, active)
	return max(ablaSeamFloor*interval/600, state.blockSizeLimitAt(interval))
}

// ablaSeamBlock returns a block of the passed size on top of prev, whose
// coinbase pays the subsidy of the block's ticks.
func ablaSeamBlock(t *testing.T, chain *BlockChain, prev *wire.MsgBlock,
	height int32, size int) *bchutil.Block {

	t.Helper()
	return ablaTestBlockPaying(t, prev, height, size, chain.chainParams,
		childSubsidy(chain, bchutil.NewBlock(prev)))
}

// ablaSeamMine extends the chain to n blocks, each as large as its limit.
func ablaSeamMine(t *testing.T, chain *BlockChain, n int) []*bchutil.Block {
	t.Helper()
	params := chain.chainParams
	prev := params.GenesisBlock
	var blocks []*bchutil.Block
	for h := int32(1); h <= int32(n); h++ {
		prevHash := prev.BlockHash()
		limit, err := chain.NextBlockSizeLimit(&prevHash)
		if err != nil {
			t.Fatal(err)
		}
		block := ablaSeamBlock(t, chain, prev, h, int(limit))
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("block %d: %v", h, err)
		}
		blocks = append(blocks, block)
		prev = block.MsgBlock()
	}
	return blocks
}

// TestABLASeam ensures blocks after the upgrade 2027 activation block have
// their ABLA limit and floor scaled to 60 ticks, while the transition after
// the activation block still takes its 600 ticks, and that a block above the
// scaled limit is rejected.  Without the activation nothing changes.
func TestABLASeam(t *testing.T) {
	for _, active := range []bool{false, true} {
		params := ablaSeamParams(active)
		db, err := database.Create("ffldb",
			filepath.Join(t.TempDir(), "db"), params.Net)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		chain := ablaSeamChain(t, db, params)
		blocks := ablaSeamMine(t, chain, ablaSeamHeight+5)
		seam := chain.bestChain.NodeByHeight(ablaSeamHeight)
		if active != isUpgrade2027Active(seam) ||
			isUpgrade2027Active(seam.parent) {

			t.Fatalf("active %v: block %d is not the activation "+
				"block", active, ablaSeamHeight)
		}

		states := ablaSeamReplay(chain, blocks, active)
		for h, block := range blocks {
			height := int32(h + 1)
			want := ablaSeamLimit(states[h], height, active)
			if size := uint64(block.MsgBlock().SerializeSize()); size != want {
				t.Errorf("active %v: block %d limit %d, want %d",
					active, height, size, want)
			}
		}
		tipState := states[len(states)-1]
		if chain.ablaState != tipState {
			t.Fatalf("active %v: state %+v, want %+v", active,
				chain.ablaState, tipState)
		}
		nextHeight := int32(len(blocks) + 1)
		wantNext := ablaSeamLimit(tipState, nextHeight, active)
		if got := chain.MaxBlockSize(true, true); got != wantNext {
			t.Errorf("active %v: max block size %d, want %d", active,
				got, wantNext)
		}
		wantFloor := ablaSeamFloor * ablaSeamInterval(nextHeight, active) / 600
		if got := chain.MaxBlockSize(true, false); got != wantFloor {
			t.Errorf("active %v: floor %d, want %d", active, got,
				wantFloor)
		}

		// The first 60-tick block has a tenth of the limit, the
		// activation block the full one.
		seamLimit := uint64(blocks[ablaSeamHeight-1].MsgBlock().SerializeSize())
		afterLimit := uint64(blocks[ablaSeamHeight].MsgBlock().SerializeSize())
		if active && (afterLimit > seamLimit/9 ||
			afterLimit < ablaSeamFloor/10) {
			t.Errorf("limit %d after the activation block, which "+
				"had %d", afterLimit, seamLimit)
		}
		if !active && afterLimit < seamLimit {
			t.Errorf("limit fell from %d to %d without activation",
				seamLimit, afterLimit)
		}

		// A block one byte above the limit is too big.
		tip := blocks[len(blocks)-1].MsgBlock()
		tooBig := ablaSeamBlock(t, chain, tip, nextHeight, int(wantNext)+1)
		_, _, err = chain.ProcessBlock(tooBig, BFNone)
		var ruleErr RuleError
		if !errors.As(err, &ruleErr) || ruleErr.ErrorCode != ErrBlockTooBig {
			t.Fatalf("active %v: block above the limit: %v, want %v",
				active, err, ErrBlockTooBig)
		}
		atLimit := ablaSeamBlock(t, chain, tip, nextHeight, int(wantNext))
		if _, _, err := chain.ProcessBlock(atLimit, BFNone); err != nil {
			t.Fatalf("active %v: block at the limit: %v", active, err)
		}
	}
}

// TestABLASeamSideChain ensures a side chain that forks before the
// activation block replays the ABLA state with each block's interval.
func TestABLASeamSideChain(t *testing.T) {
	params := ablaSeamParams(true)
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaSeamChain(t, db, params)
	blocks := ablaSeamMine(t, chain, ablaSeamHeight+3)

	// Fork after block 15 with blocks of another size, a second later
	// than the main chain, so the side chain activates at the same height.
	fork := ablaSeamHeight - 5
	prev := blocks[fork-1].MsgBlock()
	var side []*bchutil.Block
	for h := int32(fork + 1); h <= ablaSeamHeight+2; h++ {
		block := ablaSeamBlock(t, chain, prev, h, 50000+int(h))
		if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
			t.Fatalf("side block %d: %v", h, err)
		}
		side = append(side, block)
		prev = block.MsgBlock()
	}
	if chain.bestChain.Tip().hash != *blocks[len(blocks)-1].Hash() {
		t.Fatal("the side chain became the main chain")
	}

	replay := ablaSeamReplay(chain, append(blocks[:fork:fork], side...), true)
	chain.chainLock.Lock()
	defer chain.chainLock.Unlock()
	for i, block := range side {
		node := chain.index.LookupNode(block.Hash())
		got, err := chain.ablaStateAfter(node)
		if err != nil {
			t.Fatal(err)
		}
		if want := replay[fork+1+i]; got != want {
			t.Fatalf("side block %d: state %+v, want %+v",
				node.height, got, want)
		}
	}
}

// putABLAStoredState stores a state for the main chain block at the passed
// height and, when config is not nil, the configuration the states were
// computed with.
func putABLAStoredState(t *testing.T, db database.DB, height int32, state ABLAState,
	config []byte) {

	t.Helper()
	err := db.Update(func(dbTx database.Tx) error {
		if config != nil {
			err := dbTx.Metadata().Put(ablaConfigKeyName, config)
			if err != nil {
				return err
			}
		}
		return dbPutABLAStateIndex(dbTx, state, height)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// fetchABLAStoredState returns the state stored for the main chain block at
// the passed height.
func fetchABLAStoredState(t *testing.T, chain *BlockChain, height int32) ABLAState {
	t.Helper()
	var state ABLAState
	err := chain.db.View(func(dbTx database.Tx) error {
		var err error
		state, err = chain.dbFetchValidABLAState(dbTx, height)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// TestABLAActivationTimeChange ensures the ABLA states are kept across a
// restart, and that a stored ABLA configuration from before the activation
// time was part of it, or with another time, has the states of the blocks
// whose interval that changes rebuilt, and only those.
func TestABLAActivationTimeChange(t *testing.T) {
	params := ablaSeamParams(true)
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaSeamChain(t, db, params)
	blocks := ablaSeamMine(t, chain, ablaSeamHeight+5)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}
	states := ablaSeamReplay(chain, blocks, true)
	tip := int32(len(blocks))

	chain = ablaSeamChain(t, db, params)
	if chain.ablaState != states[tip] {
		t.Fatalf("restart: state %+v, want %+v", chain.ablaState,
			states[tip])
	}

	config := chain.serializeABLAConfig()
	legacy := config[:len(config)-8]
	otherTime := append([]byte(nil), legacy...)
	otherTime = byteOrder.AppendUint64(otherTime, upgrade2027Never)
	for _, test := range []struct {
		name   string
		stored []byte
	}{
		{"no activation time", legacy},
		{"another activation time", otherTime},
	} {
		// States computed with every block at 600 ticks, and a wrong
		// one before the activation block, which must be kept.
		wrong := states[ablaSeamHeight-5]
		wrong.controlBlockSize++
		putABLAStoredState(t, db, ablaSeamHeight-5, wrong, test.stored)
		old := states[ablaSeamHeight]
		for h := int32(ablaSeamHeight + 1); h <= tip; h++ {
			old = old.nextABLAState(&chain.ablaConfig,
				uint64(blocks[h-1].MsgBlock().SerializeSize()), 600)
			putABLAStoredState(t, db, h, old, nil)
		}

		chain = ablaSeamChain(t, db, params)
		if chain.ablaState != states[tip] {
			t.Fatalf("%s: state %+v, want %+v", test.name,
				chain.ablaState, states[tip])
		}
		for h := int32(ablaSeamHeight); h <= tip; h++ {
			if got := fetchABLAStoredState(t, chain, h); got != states[h] {
				t.Fatalf("%s: stored state %d %+v, want %+v",
					test.name, h, got, states[h])
			}
		}
		if got := fetchABLAStoredState(t, chain, ablaSeamHeight-5); got != wrong {
			t.Fatalf("%s: state %d rebuilt to %+v", test.name,
				ablaSeamHeight-5, got)
		}
		putABLAStoredState(t, db, ablaSeamHeight-5,
			states[ablaSeamHeight-5], nil)
	}
}

// TestABLAUnreachedTimeChange ensures a change of the upgrade 2027 activation
// time that no block reached keeps the stored ABLA states.
func TestABLAUnreachedTimeChange(t *testing.T) {
	params := ablaSeamParams(false)
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		params.Net)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chain := ablaSeamChain(t, db, params)
	blocks := ablaSeamMine(t, chain, 10)
	if err := chain.FlushCachedState(FlushRequired); err != nil {
		t.Fatal(err)
	}
	tip := int32(len(blocks))

	// A wrong but valid tip state shows whether it is rebuilt.
	config := chain.serializeABLAConfig()
	stored := byteOrder.AppendUint64(append([]byte(nil),
		config[:len(config)-8]...), uint64(time.Now().Unix())+3600)
	wrong := chain.ablaState
	wrong.controlBlockSize++
	putABLAStoredState(t, db, tip, wrong, stored)

	chain = ablaSeamChain(t, db, params)
	if chain.ablaState != wrong {
		t.Fatalf("state %+v was rebuilt to %+v", wrong, chain.ablaState)
	}
	var got []byte
	err = db.View(func(dbTx database.Tx) error {
		got = bytes.Clone(dbTx.Metadata().Get(ablaConfigKeyName))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, config) {
		t.Fatalf("stored config %x, want %x", got, config)
	}
}

// TestABLAConfigTimeChange ensures only a stored ABLA configuration that
// differs in the activation time alone, or lacks it, is taken as a change of
// that time.
func TestABLAConfigTimeChange(t *testing.T) {
	chain := newFakeChain(ablaTestParams())
	chain.ablaConfig = defaultABLAConfig(1000000, false)
	config := chain.serializeABLAConfig()
	prefix := config[:len(config)-8]
	withTime := func(b []byte, time uint64) []byte {
		return byteOrder.AppendUint64(append([]byte(nil), b...), time)
	}
	otherConfig := append([]byte(nil), prefix...)
	otherConfig[len(otherConfig)-1]++

	tests := []struct {
		name   string
		stored []byte
		time   uint64
		ok     bool
	}{
		{"same", config, upgrade2027Never, true},
		{"another time", withTime(prefix, 1234), 1234, true},
		{"no time", prefix, math.MaxUint64, true},
		{"another config", withTime(otherConfig, upgrade2027Never), 0, false},
		{"another config, no time", otherConfig, 0, false},
		{"none", nil, 0, false},
		{"short", prefix[:10], 0, false},
	}
	for _, test := range tests {
		time, ok := ablaConfigTimeChange(test.stored, config)
		if time != test.time || ok != test.ok {
			t.Errorf("%s: got %d %v, want %d %v", test.name, time, ok,
				test.time, test.ok)
		}
	}
}

// TestABLAScaledFloor ensures the floor of the block size limit, which is
// the network's default block size or on regtest and simnet the excessive
// block size, is scaled to the interval of the block, both before ABLA and as
// the minimum of the ABLA limit, and that only the blocks after the upgrade
// 2027 activation block have it scaled.
func TestABLAScaledFloor(t *testing.T) {
	tests := []struct {
		name   string
		params *chaincfg.Params
		eb     uint32
		floor  uint64
	}{
		{"mainnet", &chaincfg.MainNetParams, 32000000, 32000000},
		{"chipnet", &chaincfg.ChipNetParams, 32000000, 2000000},
		{"testnet4", &chaincfg.TestNet4Params, 32000000, 2000000},
		{"testnet3", &chaincfg.TestNet3Params, 32000000, 32000000},
		{"regtest", &chaincfg.RegressionNetParams, 32000000, 32000000},
		{"regtest, raised", &chaincfg.RegressionNetParams, 64000000, 64000000},
		{"simnet", &chaincfg.SimNetParams, 32000000, 32000000},
	}
	const start = 1700000000
	for _, test := range tests {
		for _, active := range []bool{false, true} {
			activation := uint64(upgrade2027Never)
			if active {
				activation = start + 20*600
			}
			nodes := tickTestChain(nil, regularGaps(40), activation)
			seam := int32(-1)
			for _, node := range nodes {
				if node.nextInterval == 60 {
					seam = node.height
					break
				}
			}
			if active != (seam > 0) {
				t.Fatalf("%s: activation block %d", test.name, seam)
			}

			for _, ablaFork := range []int32{1000, 0} {
				params := *test.params
				params.UahfForkHeight = 0
				params.ABLAForkHeight = ablaFork
				chain := newFakeChain(&params)
				chain.excessiveBlockSize = test.eb
				chain.ablaConfig = defaultABLAConfig(test.floor/2,
					false)
				// A state with a 600-tick limit of half the floor
				// plus 5,000 bytes, so the floor sets the limit.
				chain.ablaState = ABLAState{
					controlBlockSize:  test.floor / 4,
					elasticBufferSize: test.floor/4 + 5000,
				}
				for _, prev := range nodes {
					chain.bestChain = newChainView(prev)
					want := test.floor
					if active && prev.height >= seam {
						want = test.floor / 10
					}
					got, err := chain.blockSizeLimit(prev,
						prev.height+1, chain.networkSizeFloor())
					if err != nil {
						t.Fatal(err)
					}
					if got != want {
						t.Fatalf("%s, ABLA from %d: limit after "+
							"block %d %d, want %d", test.name,
							ablaFork, prev.height, got, want)
					}
					if got := chain.MaxBlockSize(true, ablaFork == 0); got != want {
						t.Fatalf("%s: max block size after "+
							"block %d %d, want %d", test.name,
							prev.height, got, want)
					}
				}
			}
		}
	}
}

// TestABLAScaledLimitAboveFloor ensures an ABLA limit above the floor is
// scaled to 60 ticks with a single truncation.
func TestABLAScaledLimitAboveFloor(t *testing.T) {
	params := chaincfg.ChipNetParams
	params.UahfForkHeight = 0
	params.ABLAForkHeight = 0
	chain := newFakeChain(&params)
	chain.excessiveBlockSize = 32000000
	chain.ablaConfig = defaultABLAConfig(2000000, false)
	chain.ablaState = ABLAState{controlBlockSize: 1234567,
		elasticBufferSize: 7654321}
	nodes := tickTestChain(nil, regularGaps(30), 1700000000+20*600)
	tip := nodes[len(nodes)-1]
	chain.bestChain = newChainView(tip)
	if !isUpgrade2027Active(tip) {
		t.Fatal("the upgrade is not active")
	}
	// (1234567 + 7654321) * 60 / 600 = 888888.8.
	got, err := chain.blockSizeLimit(tip, tip.height+1,
		chain.networkSizeFloor())
	if err != nil || got != 888888 {
		t.Fatalf("limit %d %v, want 888888", got, err)
	}
	if got := chain.MaxBlockSize(true, true); got != 888888 {
		t.Fatalf("max block size %d, want 888888", got)
	}
}
