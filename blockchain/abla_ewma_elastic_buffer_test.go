package blockchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ablaNumber is a number of the ABLA test vectors, which are given either as
// JSON numbers or as strings.
type ablaNumber uint64

func (n *ablaNumber) UnmarshalJSON(b []byte) error {
	v, err := strconv.ParseUint(strings.Trim(string(b), `"`), 10, 64)
	if err != nil {
		return err
	}
	*n = ablaNumber(v)
	return nil
}

type ablaTestState struct {
	N       ablaNumber `json:"n"`
	Epsilon ablaNumber `json:"epsilon"`
	Beta    ablaNumber `json:"beta"`
}

type ablaTestVector struct {
	Blocksize                       ablaNumber    `json:"blocksize"`
	ABLAStateForNextBlock           ablaTestState `json:"ABLAStateForNextBlock"`
	BlocksizeLimitForNextBlock      ablaNumber    `json:"blocksizeLimitForNextBlock"`
	Lookahead                       *ablaNumber   `json:"lookahead"`
	ABLAStateForLookaheadBlock      ablaTestState `json:"ABLAStateForLookaheadBlock"`
	BlocksizeLimitForLookaheadBlock ablaNumber    `json:"blocksizeLimitForLookaheadBlock"`
}

type ablaTest struct {
	ABLAConfig struct {
		Epsilon0        ablaNumber `json:"epsilon0"`
		Beta0           ablaNumber `json:"beta0"`
		N0              ablaNumber `json:"n0"`
		Zeta            ablaNumber `json:"zeta"`
		GammaReciprocal ablaNumber `json:"gammaReciprocal"`
		Delta           ablaNumber `json:"delta"`
		ThetaReciprocal ablaNumber `json:"thetaReciprocal"`
		Options         string     `json:"options"`
	} `json:"ABLAConfig"`
	ABLAStateInitial      ablaTestState    `json:"ABLAStateInitial"`
	BlocksizeLimitInitial ablaNumber       `json:"blocksizeLimitInitial"`
	TestVector            []ablaTestVector `json:"testVector"`
}

// TestABLA runs the ABLA test vectors the way Bitcoin Cash Node's
// abla_test_vectors test does.
func TestABLA(t *testing.T) {
	files, err := filepath.Glob("testdata/ebaa_vectors/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no ABLA test vectors: %v", err)
	}
	defer func(limit uint64) {
		TEMP_32_BIT_MAX_SAFE_BLOCKSIZE_LIMIT = limit
	}(TEMP_32_BIT_MAX_SAFE_BLOCKSIZE_LIMIT)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var test ablaTest
		if err := json.Unmarshal(data, &test); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if err := runABLATest(&test); err != nil {
			t.Errorf("%s: %v", filepath.Base(file), err)
		}
	}
}

// runABLATest runs one ABLA test vector file.
func runABLATest(test *ablaTest) error {
	conf := test.ABLAConfig
	config := ABLAConfig{
		epsilon0:        uint64(conf.Epsilon0),
		beta0:           uint64(conf.Beta0),
		n0:              uint64(conf.N0),
		gammaReciprocal: uint64(conf.GammaReciprocal),
		zetaXB7:         uint64(conf.Zeta),
		thetaReciprocal: uint64(conf.ThetaReciprocal),
		delta:           uint64(conf.Delta),
	}
	config.SetMax()
	if errs := config.IsValid(); errs != nil {
		return fmt.Errorf("invalid config: %s", errs)
	}

	TEMP_32_BIT_MAX_SAFE_BLOCKSIZE_LIMIT = uint64(2000000000)
	if strings.Contains(conf.Options, "-disable2GBLimit") {
		TEMP_32_BIT_MAX_SAFE_BLOCKSIZE_LIMIT = UINT64_MAX
	}

	initial := test.ABLAStateInitial
	state := ABLAState{
		blockHeight:       uint64(initial.N),
		controlBlockSize:  uint64(initial.Epsilon),
		elasticBufferSize: uint64(initial.Beta),
	}
	if errs := state.IsValid(&config); errs != nil {
		return fmt.Errorf("invalid initial state: %s", errs)
	}
	initialLimit := uint64(test.BlocksizeLimitInitial)

	vectors := test.TestVector
	if len(vectors) == 1 && vectors[0].Lookahead != nil {
		v := vectors[0]
		lookahead := uint64(*v.Lookahead)
		final := v.ABLAStateForLookaheadBlock
		finalLimit := uint64(v.BlocksizeLimitForLookaheadBlock)
		switch {
		case lookahead == 0:
			return fmt.Errorf("lookahead is 0")
		case initialLimit != uint64(initial.Epsilon+initial.Beta) ||
			state.getBlockSizeLimit() != initialLimit:
			return fmt.Errorf("bad initial limit %d", initialLimit)
		case finalLimit != uint64(final.Epsilon+final.Beta):
			return fmt.Errorf("bad lookahead limit %d", finalLimit)
		case uint64(initial.N)+lookahead != uint64(final.N):
			return fmt.Errorf("bad lookahead height %d", final.N)
		}

		// The lookahead only applies from the activation block on.
		state.blockHeight = max(config.n0, state.blockHeight)
		if state.blockHeight >= uint64(final.N) {
			return nil
		}
		ahead := state.lookaheadState(&config,
			uint(uint64(final.N)-state.blockHeight))
		got := ahead.getBlockSizeLimit()
		if got != finalLimit {
			return fmt.Errorf("lookahead limit %d, want %d", got,
				finalLimit)
		}
		return nil
	}

	n := uint64(initial.N)
	want := initial
	limit := initialLimit
	var blockSize uint64
	check := func(nextLimit uint64, pastTheEnd bool) error {
		if n != uint64(want.N) {
			return fmt.Errorf("height %d, want %d", n, want.N)
		}
		if got := state.getBlockSizeLimit(); got != limit {
			return fmt.Errorf("height %d: limit %d, want %d", n, got,
				limit)
		}
		if state.controlBlockSize != uint64(want.Epsilon) ||
			state.elasticBufferSize != uint64(want.Beta) {

			return fmt.Errorf("height %d: state %d %d, want %d %d", n,
				state.controlBlockSize, state.elasticBufferSize,
				want.Epsilon, want.Beta)
		}
		state.blockHeight = n
		if n >= config.n0 {
			next := state.nextABLAState(&config, blockSize)
			if !pastTheEnd && next.getBlockSizeLimit() != nextLimit {
				return fmt.Errorf("height %d: next limit %d, want %d",
					n, next.getBlockSizeLimit(), nextLimit)
			}
			state = next
		} else if !pastTheEnd && initialLimit != nextLimit {
			return fmt.Errorf("height %d: next limit %d before "+
				"activation, want %d", n, nextLimit, initialLimit)
		}
		if errs := state.IsValid(&config); errs != nil {
			return fmt.Errorf("height %d: %s", n, errs)
		}
		return nil
	}
	for _, v := range vectors {
		blockSize = uint64(v.Blocksize)
		nextLimit := uint64(v.BlocksizeLimitForNextBlock)
		if err := check(nextLimit, false); err != nil {
			return err
		}
		want = v.ABLAStateForNextBlock
		limit = nextLimit
		n++
	}
	return check(0, true)
}
