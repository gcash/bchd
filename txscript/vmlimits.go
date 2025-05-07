package txscript

// CHIP-2021-05 VM Limits: Targeted Virtual Machine Limits

const (
	// Max bytes pushable to the stack after upgrade 11.
	MaxScriptElementSizeUpgrade11 = 10000
	// Base cost for each executed opcode. No opcodes incur a cost less than this but some may incur more.
	OpcodeBaseCost = 100
	// Conditional stack depth limit. (Max depth of OP_IF and firends)
	MaxConditionalStackDepth = 100
	// Each sigcheck by an input adds this amount to the total op cost.
	SigCheckCostFactor = 26000

	// 'non-standard' txns (block txns) get a 7x bonus to their hash iteration limit.
	HashIterBonusForNonStandardTxns = 7
	// Op cost allowance factor; This is multiplied by the input byte size to determine the total op cost allowance for an input.
	OpCostBudgetPerInputByte = 800
	// The penalty paid by 'standard' (relay) txns per hash op; That is 'standard' hash ops cost 3x.
	HashCostPenaltyForStdTxns = 3
	// All hashers supported by VM opcodes (OP_HASH160, OP_HASH256, etc) use a 64-byte block size; Update if adding hashers.
	HashBlockSize = 64
	// As per VM Limits CHIP, each input script has this fixed serialization overhead we credit to it, in bytes.
	InputScriptSizeFixedCredit = 41
)

// Returns the hash iteration limit for an input, given: 1) whether "standard" rules are in effect, and 2) the input's
// scriptSig size. See: https://github.com/bitjson/bch-vm-limits?tab=readme-ov-file#maximum-hashing-density
func GetInputHashIterationsLimit(scriptSigSize int, isStandard bool) int {
	bonusFactor := 1

	if !isStandard {
		bonusFactor = HashIterBonusForNonStandardTxns
	}

	return ((InputScriptSizeFixedCredit + scriptSigSize) * bonusFactor) / 2
}

// Returns the op cost limit for an input, given an input's scriptSig size.
// See: https://github.com/bitjson/bch-vm-limits?tab=readme-ov-file#operation-cost-limit
func GetInputOperationCostLimit(scriptSigSize int) int {
	return (scriptSigSize + InputScriptSizeFixedCredit) * OpCostBudgetPerInputByte
}

// Returns the pre-hash iteration op cost, either 64 if isStandard == false or 192 if isStandard == true flag is set.
// See: https://github.com/bitjson/bch-vm-limits?tab=readme-ov-file#hash-digest-iteration-cost
func GetHashIterationCostFactor(isStandard bool) int {
	if isStandard {
		return HashBlockSize * HashCostPenaltyForStdTxns
	} else {
		return HashBlockSize
	}
}

// Returns the hash iteration count given a particular message length and whether the hasher was a double hash or not.
// See: https://github.com/bitjson/bch-vm-limits?tab=readme-ov-file#digest-iteration-count
func GetDigestIterationCount(messageLength int, isDouble bool) int {
	iterationCount := 1 + ((messageLength + 8) / HashBlockSize)

	if isDouble {
		iterationCount += 1
	}

	return iterationCount
}

type ScriptExecutionMetrics struct {
	// CHIP-2021-05 VM Limits: Targeted Virtual Machine Limits
	numSigChecks            int
	numOpCost               int64
	numHashDigestIterations int64

	// Max allowed op cost and hash iterations
	opCostLimit               int64
	hashDigestIterationsLimit int64
}

func NewScriptExecutionMetrics(scriptSigSize int, isStandard bool) *ScriptExecutionMetrics {
	return &ScriptExecutionMetrics{
		numSigChecks:            0,
		numOpCost:               0,
		numHashDigestIterations: 0,

		opCostLimit:               int64(GetInputOperationCostLimit(scriptSigSize)),
		hashDigestIterationsLimit: int64(GetInputHashIterationsLimit(scriptSigSize, isStandard)),
	}
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) AddOPCost(cost int) {
	scriptExecutionMetrics.numOpCost += int64(cost)
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) AddHashCost(messageLength int, isDouble bool) {
	scriptExecutionMetrics.numHashDigestIterations += int64(GetDigestIterationCount(messageLength, isDouble))
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) AddNumSigChecks(numScigChecks int) {
	scriptExecutionMetrics.numSigChecks += numScigChecks
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) GetMaxOpCostLimit() int64 {
	return scriptExecutionMetrics.opCostLimit
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) GetMaxDigestIterationLimit() int64 {
	return scriptExecutionMetrics.hashDigestIterationsLimit
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) GetHashDigestIterations() int64 {
	return scriptExecutionMetrics.numHashDigestIterations
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) GetCompositeOPCost(isStandard bool) int64 {
	hashIterFactor := GetHashIterationCostFactor(isStandard)

	// Base cost : encompasses ops + pushes, etc
	compositeCost := scriptExecutionMetrics.numOpCost

	// Additional cost: add hash iterations * {192 or 64}
	compositeCost += scriptExecutionMetrics.numHashDigestIterations * int64(hashIterFactor)

	// Additional cost: add sig checks * 26000
	compositeCost += int64(scriptExecutionMetrics.numSigChecks * SigCheckCostFactor)

	return compositeCost
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) IsOverOpCostLimit(isStandard bool) bool {
	return scriptExecutionMetrics.GetCompositeOPCost(isStandard) > scriptExecutionMetrics.opCostLimit
}

func (scriptExecutionMetrics *ScriptExecutionMetrics) IsOverHashIterationsLimit(isStandard bool) bool {
	return scriptExecutionMetrics.numHashDigestIterations > scriptExecutionMetrics.hashDigestIterationsLimit
}
