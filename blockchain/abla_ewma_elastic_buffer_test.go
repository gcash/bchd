package blockchain

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"testing"
)

var ebaaVectorsFileNames = [39]string{
	"config1-test2-activation-random32.json",
	"config1-test2-random128.json",
	"config1-test2-random2000.json",
	"config1-test2-random32.json",
	"config1-test2-random512.json",
	"config1-test2-random8196.json",
	"config1-test3-randomUINT64MAX.json",
	"config1-test4-activation-max.json",
	"config1-test4-hard-max.json",
	"config1-test4-lookahead-0.json",
	"config1-test4-lookahead-1.json",
	"config1-test4-lookahead-2.json",
	"config1-test4-lookahead-3.json",
	"config1-test5-blocksize-limit-max.json",
	"config1-test5-elastic-buffer-floor.json",
	"config1-test5-elastic-buffer-max.json",
	"config1-test6-control-blocksize-floor-empty.json",
	"config1-test6-elastic-buffer-floor-empty.json",
	"config2-test2-activation-random32.json",
	"config2-test2-random128.json",
	"config2-test2-random2000.json",
	"config2-test2-random32.json",
	"config2-test2-random512.json",
	"config2-test2-random8196.json",
	"config2-test3-randomUINT64MAX.json",
	"config2-test4-hard-max.json",
	"config3-test2-activation-random32.json",
	"config3-test2-random128.json",
	"config3-test2-random2000.json",
	"config3-test2-random32.json",
	"config3-test2-random512.json",
	"config3-test2-random8196.json",
	"config3-test4-activation-max.json",
	"config4-test2-activation-random32.json",
	"config4-test2-random128.json",
	"config4-test2-random2000.json",
	"config4-test2-random32.json",
	"config4-test2-random512.json",
	"config4-test2-random8196.json",
}

type ABLAConfigData struct {
	Epsilon0        string `json:"epsilon0"`
	Beta0           string `json:"beta0"`
	N0              uint64 `json:"n0"`
	Zeta            uint64 `json:"zeta"`
	GammaReciprocal uint64 `json:"gammaReciprocal"`
	Delta           uint64 `json:"delta"`
	ThetaReciprocal uint64 `json:"thetaReciprocal"`
	Options         string `json:"options"`
}

type ABLAStateData struct {
	N       uint64 `json:"n"`
	Epsilon string `json:"epsilon"`
	Beta    string `json:"beta"`
}

type TestVector struct {
	Blocksize                       string        `json:"blocksize"`
	ABLAStateForNextBlock           ABLAStateData `json:"ABLAStateForNextBlock"`
	BlocksizeLimitForNextBlock      string        `json:"blocksizeLimitForNextBlock"`
	LookAhead                       string        `json:"lookahead"`
	ABLAStateForLookaheadBlock      ABLAStateData `json:"ABLAStateForLookaheadBlock"`
	BlocksizeLimitForLookaheadBlock string        `json:"blocksizeLimitForLookaheadBlock"`
}

type TestData struct {
	ABLAConfig            ABLAConfigData `json:"ABLAConfig"`
	TestDescription       string         `json:"testDescription"`
	ABLAStateInitial      ABLAStateData  `json:"ABLAStateInitial"`
	BlocksizeLimitInitial string         `json:"blocksizeLimitInitial"`
	TestVectors           []TestVector   `json:"testVector"`
}

func TestABLA(t *testing.T) {

	for _, fileName := range ebaaVectorsFileNames {
		file, err := ioutil.ReadFile("testdata/ebaa_vectors/" + fileName)

		if err != nil {
			t.Fatalf("TestScripts couldn't Unmarshal: %v", err)
		}

		var testData TestData
		json.Unmarshal(file, &testData)

		epsilon, _ := strconv.ParseUint(testData.ABLAStateInitial.Epsilon, 10, 64)
		beta, _ := strconv.ParseUint(testData.ABLAStateInitial.Beta, 10, 64)
		ablaState := ABLAState{
			blockHeight:       testData.ABLAStateInitial.N,
			controlBlockSize:  epsilon,
			elasticBufferSize: beta,
		}

		epsilon0, _ := strconv.ParseUint(testData.ABLAConfig.Epsilon0, 10, 64)
		beta0, _ := strconv.ParseUint(testData.ABLAConfig.Beta0, 10, 64)
		ablaConfig := ABLAConfig{
			epsilon0:        epsilon0,
			beta0:           beta0,
			n0:              testData.ABLAConfig.N0,
			gammaReciprocal: testData.ABLAConfig.GammaReciprocal,
			zetaXB7:         testData.ABLAConfig.Zeta,
			thetaReciprocal: testData.ABLAConfig.ThetaReciprocal,
			delta:           testData.ABLAConfig.Delta,
			fixedSize:       false,
		}
		ablaConfig.SetMax()

		if testData.ABLAConfig.Options == "-disable2GBLimit" {
			TEMP_32_BIT_MAX_SAFE_BLOCKSIZE_LIMIT = UINT64_MAX
		} else {
			TEMP_32_BIT_MAX_SAFE_BLOCKSIZE_LIMIT = uint64(2000000000)
		}

		for _, test := range testData.TestVectors {

			testBlockSizeLimit, _ := strconv.ParseUint(test.BlocksizeLimitForNextBlock, 10, 64)

			testLookAheadCount, _ := strconv.ParseUint(test.LookAhead, 10, 64)
			if testLookAheadCount > 0 {
				test.ABLAStateForNextBlock = test.ABLAStateForLookaheadBlock
				testBlockSizeLimit, _ = strconv.ParseUint(test.BlocksizeLimitForLookaheadBlock, 10, 64)

			}

			testBlockEpsilon, _ := strconv.ParseUint(test.ABLAStateForNextBlock.Epsilon, 10, 64)
			testBlockBeta, _ := strconv.ParseUint(test.ABLAStateForNextBlock.Beta, 10, 64)
			blockSize, _ := strconv.ParseUint(test.Blocksize, 10, 64)

			if testLookAheadCount > 0 {
				ablaState = ablaState.lookaheadState(&ablaConfig, uint(testLookAheadCount))
			} else {
				ablaState = ablaState.nextABLAState(&ablaConfig, blockSize)

				if ablaState.blockHeight != test.ABLAStateForNextBlock.N {
					t.Fatalf(
						"%s, Expected block height to be %d but got %d",
						fileName[:5],
						test.ABLAStateForNextBlock.N,
						ablaState.blockHeight,
					)
				}
			}

			if ablaState.controlBlockSize != testBlockEpsilon {
				t.Fatalf(
					"%s, Block Height: %d, Expected controlBlockSize to be %d but got %d",
					fileName[:5],
					test.ABLAStateForNextBlock.N,
					testBlockEpsilon,
					ablaState.controlBlockSize,
				)
			}

			if ablaState.elasticBufferSize != testBlockBeta {
				t.Fatalf(
					"%s, Block Height: %d, Expected elasticBufferSize to be %d but got %d",
					fileName[:5],
					test.ABLAStateForNextBlock.N,
					testBlockBeta,
					ablaState.elasticBufferSize,
				)
			}
			if ablaState.getBlockSizeLimit() != testBlockSizeLimit {
				t.Fatalf(
					"%s, Block Height: %d, Expected getBlockSizeLimit() to return %d but got %d",
					fileName[:5],
					test.ABLAStateForNextBlock.N,
					testBlockSizeLimit,
					ablaState.getBlockSizeLimit(),
				)
			}
		}
	}
}
