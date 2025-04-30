package txscript

// CHIP-2021-05 VM Limits: Targeted Virtual Machine Limits

const (
	// Max bytes pushable to the stack after upgrade 11.
	MaxScriptElementSizeUpgrade11 = 10000
	// Base cost for each executed opcode. No opcodes incur a cost less than this but some may incur more.
	OpcodeBaseCost = 100
	// Conditional stack depth limit. (Max depth of OP_IF and firends)
	MaxConditionalStackDepth = 100
	//Each sigcheck by an input adds this amount to the total op cost.
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
