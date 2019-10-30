package txscript

import "encoding/binary"

const (
	// slpLokadID is the prefix used to identify SLP transactions.
	slpLokadID = 0x534c5000
)

// GetSLPScriptClass returns the class of the SLP script passed.
//
// NonStandardTy will be returned when the script does not parse.
func GetSLPScriptClass(script []byte) ScriptClass {
	pops, err := parseScript(script)
	if err != nil {
		return NonStandardTy
	}
	if isSLPGenesis(pops) {
		return SLPGenesisTy
	} else if isSLPMint(pops) {
		return SLPMintTy
	} else if isSLPSpend(pops) {
		return SLPSpendTy
	}
	return typeOfScript(pops)
}

func isSLPGenesis(pops []parsedOpcode) bool {
	if len(pops) != 11 {
		return false
	}

	if pops[0].opcode.value != OP_RETURN {
		return false
	}

	// <transaction_type: 'GENESIS'> (7 bytes, ascii)
	if len(pops[3].data) != 7 || string(pops[3].data) != "GENESIS" {
		return false
	}

	// Only opcodes 0x01 to 0x4e are permitted
	if containsInvalidSPLPush(pops) {
		return false
	}

	// <lokad_id: 'SLP\x00'> (4 bytes, ascii)
	if len(pops[1].data) != 4 || binary.BigEndian.Uint32(pops[1].data) != slpLokadID {
		return false
	}

	// <token_type: 1> (1 to 2 byte integer)
	if len(pops[2].data) < 1 || len(pops[2].data) > 2 {
		return false
	}

	// <token_document_hash> (0 bytes or 32 bytes)
	if len(pops[7].data) != 0 && len(pops[7].data) != 32 {
		return false
	}

	// <decimals> (1 byte in range 0x00-0x09)
	if len(pops[8].data) != 1 {
		return false
	}
	if pops[8].data[0] > 0x09 {
		return false
	}

	// <mint_baton_vout> (0 bytes, or 1 byte in range 0x02-0xff)
	if len(pops[9].data) != 0 && len(pops[9].data) != 1 {
		return false
	}
	if len(pops[9].data) == 1 && pops[9].data[0] < 0x02 {
		return false
	}

	// <initial_token_mint_quantity> (8 byte integer)
	if len(pops[10].data) != 8 {
		return false
	}

	return true
}

func isSLPMint(pops []parsedOpcode) bool {
	if len(pops) != 7 {
		return false
	}

	if pops[0].opcode.value != OP_RETURN {
		return false
	}

	// <transaction_type: 'MINT'> (4 bytes, ascii)
	if len(pops[3].data) != 4 || string(pops[3].data) != "MINT" {
		return false
	}

	// Only opcodes 0x01 to 0x4e are permitted
	if containsInvalidSPLPush(pops) {
		return false
	}

	// <lokad_id: 'SLP\x00'> (4 bytes, ascii)
	if len(pops[1].data) != 4 || binary.BigEndian.Uint32(pops[1].data) != slpLokadID {
		return false
	}

	// <token_type: 1> (1 to 2 byte integer)
	if len(pops[2].data) < 1 || len(pops[2].data) > 2 {
		return false
	}

	// <token_id> (32 bytes)
	if len(pops[4].data) != 32 {
		return false
	}

	// <mint_baton_vout> (0 bytes or 1 byte between 0x02-0xff)
	if len(pops[5].data) != 0 && len(pops[5].data) != 1 {
		return false
	}
	if len(pops[5].data) == 1 && pops[5].data[0] < 0x02 {
		return false
	}

	// <additional_token_quantity> (8 byte integer)
	if len(pops[6].data) != 8 {
		return false
	}

	return true
}

func isSLPSpend(pops []parsedOpcode) bool {
	if len(pops) < 6 {
		return false
	}

	if pops[0].opcode.value != OP_RETURN {
		return false
	}

	// <transaction_type: 'MINT'> (4 bytes, ascii)
	if len(pops[3].data) != 4 || string(pops[3].data) != "SEND" {
		return false
	}

	// Only opcodes 0x01 to 0x4e are permitted
	if containsInvalidSPLPush(pops) {
		return false
	}

	// <lokad_id: 'SLP\x00'> (4 bytes, ascii)
	if len(pops[1].data) != 4 || binary.BigEndian.Uint32(pops[1].data) != slpLokadID {
		return false
	}

	// <token_type: 1> (1 to 2 byte integer)
	if len(pops[2].data) < 1 || len(pops[2].data) > 2 {
		return false
	}

	// <token_id> (32 bytes)
	if len(pops[4].data) != 32 {
		return false
	}

	// <token_output_quantity1> (required, 8 byte integer)
	if len(pops[5].data) != 8 {
		return false
	}

	// <token_output_quantity2> (optional, 8 byte integer)
	for _, pop := range pops[5:] {
		if len(pop.data) != 8 {
			return false
		}
	}

	return true
}

func containsInvalidSPLPush(pops []parsedOpcode) bool {
	for _, pop := range pops[1:] {
		if pop.opcode.value < OP_DATA_1 || pop.opcode.value > OP_PUSHDATA4 {
			return true
		}
	}
	return false
}
