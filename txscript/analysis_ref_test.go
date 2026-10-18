// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

// This file keeps the parsedOpcode-based script analysis functions that the
// tokenizer-based ones replaced.  The differential tests and fuzz targets
// compare the two.

import (
	"fmt"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// refIsScriptHash returns true if the script passed is a pay-to-script-hash
// transaction, false otherwise.
func refIsScriptHash(pops []parsedOpcode) bool {
	return len(pops) == 3 &&
		pops[0].opcode.value == OP_HASH160 &&
		pops[1].opcode.value == OP_DATA_20 &&
		pops[2].opcode.value == OP_EQUAL
}

// refIsScriptHash32 returns true if the script passed is a pay-to-script-hash-32
// transaction, false otherwise.
func refIsScriptHash32(pops []parsedOpcode) bool {
	return len(pops) == 3 &&
		pops[0].opcode.value == OP_HASH256 &&
		pops[1].opcode.value == OP_DATA_32 &&
		pops[2].opcode.value == OP_EQUAL
}

// refIsPayToScriptHash returns true if the script is in the standard
// pay-to-script-hash (P2SH) format, false otherwise.
func refIsPayToScriptHash(script []byte) bool {
	pops, err := parseScript(script)
	if err != nil {
		return false
	}
	return refIsScriptHash(pops)
}

// refIsPayToScriptHash32 returns true if the script is in the standard
// pay-to-script-hash-32 (P2SH32) format, false otherwise.
func refIsPayToScriptHash32(script []byte) bool {
	pops, err := parseScript(script)
	if err != nil {
		return false
	}
	return refIsScriptHash32(pops)
}

// refIsPushOnly returns true if the script only pushes data, false otherwise.
func refIsPushOnly(pops []parsedOpcode) bool {
	// NOTE: This function does NOT verify opcodes directly since it is
	// internal and is only called with parsed opcodes for scripts that did
	// not have any parse errors.  Thus, consensus is properly maintained.

	for _, pop := range pops {
		// All opcodes up to OP_16 are data push instructions.
		// NOTE: This does consider OP_RESERVED to be a data push
		// instruction, but execution of OP_RESERVED will fail anyways
		// and matches the behavior required by consensus.
		if pop.opcode.value > OP_16 {
			return false
		}
	}
	return true
}

// refIsPushOnlyScript returns whether or not the passed script only pushes data.
//
// False will be returned when the script does not parse.
func refIsPushOnlyScript(script []byte) bool {
	pops, err := parseScript(script)
	if err != nil {
		return false
	}
	return refIsPushOnly(pops)
}

// refExtractDataElements returns a slice of all the data elements in the
// given script.
func refExtractDataElements(script []byte) ([][]byte, error) {
	var dataElements [][]byte
	pops, err := parseScript(script)
	if err != nil {
		return nil, err
	}
	for _, pop := range pops {
		// The only opcodes which carry data are OP_DATA_1 to OP_PUSHDATA4.
		// OP_0 and OP_1 - OP_16 are ignored for the purpose of this function
		// even though they push data to the stack.
		if pop.opcode.value > OP_0 && pop.opcode.value <= OP_PUSHDATA4 {
			dataElements = append(dataElements, pop.data)
		}
	}
	return dataElements, nil
}

// refIsPubKeyPush returns whether pop is a direct push of a public key whose
// length fits its first byte, as BCHN requires of the keys in the pubkey and
// multisig templates.
func refIsPubKeyPush(pop parsedOpcode) bool {
	switch pop.opcode.value {
	case OP_DATA_33:
		return pop.data[0] == 0x02 || pop.data[0] == 0x03
	case OP_DATA_65:
		return pop.data[0] == 0x04 || pop.data[0] == 0x06 ||
			pop.data[0] == 0x07
	}
	return false
}

// refIsPubkey returns true if the script passed is a pay-to-pubkey transaction,
// false otherwise.
func refIsPubkey(pops []parsedOpcode) bool {
	return len(pops) == 2 && refIsPubKeyPush(pops[0]) &&
		pops[1].opcode.value == OP_CHECKSIG
}

// refIsPubkeyHash returns true if the script passed is a pay-to-pubkey-hash
// transaction, false otherwise.
func refIsPubkeyHash(pops []parsedOpcode) bool {
	return len(pops) == 5 &&
		pops[0].opcode.value == OP_DUP &&
		pops[1].opcode.value == OP_HASH160 &&
		pops[2].opcode.value == OP_DATA_20 &&
		pops[3].opcode.value == OP_EQUALVERIFY &&
		pops[4].opcode.value == OP_CHECKSIG

}

// refIsMultiSig returns true if the passed script is a multisig transaction, false
// otherwise.
func refIsMultiSig(pops []parsedOpcode) bool {
	// The absolute minimum is 1 pubkey:
	// OP_1-16 <pubkey> OP_1 OP_CHECKMULTISIG
	l := len(pops)
	if l < 4 {
		return false
	}
	if !isSmallInt(pops[0].opcode) || pops[0].opcode.value == OP_0 {
		return false
	}
	if !isSmallInt(pops[l-2].opcode) {
		return false
	}
	if pops[l-1].opcode.value != OP_CHECKMULTISIG {
		return false
	}

	// Verify the number of pubkeys specified matches the actual number
	// of pubkeys provided, and is no less than the number of signatures.
	numPubKeys := asSmallInt(pops[l-2].opcode)
	if l-2-1 != numPubKeys || asSmallInt(pops[0].opcode) > numPubKeys {
		return false
	}

	for _, pop := range pops[1 : l-2] {
		if !refIsPubKeyPush(pop) {
			return false
		}
	}
	return true
}

// refIsNullData returns true if the passed script is a null data transaction,
// false otherwise.
func refIsNullData(pops []parsedOpcode) bool {
	// A nulldata transaction is either a single OP_RETURN or an
	// OP_RETURN SMALLDATA (where SMALLDATA is a data push up to
	// MaxDataCarrierSize bytes).
	//
	// Multiple pushes of data are allowed but the total script
	// may not exceed MaxDataCarrierSize.
	l := len(pops)
	if l < 1 || pops[0].opcode.value != OP_RETURN {
		return false
	}

	scriptLen := 1
	for _, pop := range pops[1:] {
		isDataOpcode := false
		if isSmallInt(pop.opcode) {
			isDataOpcode = true
			scriptLen++
		} else if pop.opcode.value < OP_PUSHDATA1 {
			isDataOpcode = true
			scriptLen += pop.opcode.length
		} else if pop.opcode.value >= OP_PUSHDATA1 && pop.opcode.value <= OP_PUSHDATA4 {
			isDataOpcode = true
			scriptLen += len(pop.data)
			scriptLen += -pop.opcode.length + 1
		} else if pop.opcode.value < OP_16 {
			isDataOpcode = true
			scriptLen++
		}
		if !isDataOpcode {
			return false
		}
	}
	return scriptLen <= MaxDataCarrierSize
}

// refTypeOfScript returns the type of the script being inspected from the known
// standard types.
func refTypeOfScript(pops []parsedOpcode) ScriptClass {
	if refIsPubkey(pops) {
		return PubKeyTy
	} else if refIsPubkeyHash(pops) {
		return PubKeyHashTy
	} else if refIsScriptHash(pops) {
		return ScriptHashTy
	} else if refIsScriptHash32(pops) {
		return ScriptHash32Ty
	} else if refIsMultiSig(pops) {
		return MultiSigTy
	} else if refIsNullData(pops) {
		return NullDataTy
	}
	return NonStandardTy
}

// refGetScriptClass returns the class of the script passed.
//
// NonStandardTy will be returned when the script does not parse.
func refGetScriptClass(script []byte) ScriptClass {
	pops, err := parseScript(script)
	if err != nil {
		return NonStandardTy
	}
	return refTypeOfScript(pops)
}

// refExpectedInputs returns the number of arguments required by a script.
// If the script is of unknown type such that the number can not be determined
// then -1 is returned. We are an internal function and thus assume that class
// is the real class of pops (and we can thus assume things that were determined
// while finding out the type).
func refExpectedInputs(pops []parsedOpcode, class ScriptClass) int {
	switch class {
	case PubKeyTy:
		return 1

	case PubKeyHashTy:
		return 2

	case ScriptHashTy:
		// Not including script.  That is handled by the caller.
		return 1

	case MultiSigTy:
		// Standard multisig has a push a small number for the number
		// of sigs and number of keys.  Check the first push instruction
		// to see how many arguments are expected. typeOfScript already
		// checked this so we know it'll be a small int.  Also, due to
		// the original bitcoind bug where OP_CHECKMULTISIG pops an
		// additional item from the stack, add an extra expected input
		// for the extra push that is required to compensate.
		return asSmallInt(pops[0].opcode) + 1

	case NullDataTy:
		fallthrough
	default:
		return -1
	}
}

// refCalcScriptInfo returns a structure providing data about the provided script
// pair.  It will error if the pair is in someway invalid such that they can not
// be analysed, i.e. if they do not parse or the pkScript is not a push-only
// script
func refCalcScriptInfo(sigScript, pkScript []byte, scriptFlags ScriptFlags) (*ScriptInfo, error) {

	sigPops, err := parseScript(sigScript)
	if err != nil {
		return nil, err
	}

	pkPops, err := parseScript(pkScript)
	if err != nil {
		return nil, err
	}

	// Push only sigScript makes little sense.
	si := new(ScriptInfo)
	si.PkScriptClass = refTypeOfScript(pkPops)

	// Can't have a signature script that doesn't just push data.
	if !refIsPushOnly(sigPops) {
		return nil, scriptError(ErrNotPushOnly,
			"signature script is not push only")
	}

	si.ExpectedInputs = refExpectedInputs(pkPops, si.PkScriptClass)

	switch {
	case si.PkScriptClass == ScriptHashTy && scriptFlags.HasFlag(ScriptBip16):
		// The pay-to-hash-script is the final data push of the
		// signature script.
		script := sigPops[len(sigPops)-1].data
		shPops, err := parseScript(script)
		if err != nil {
			return nil, err
		}

		shInputs := refExpectedInputs(shPops, refTypeOfScript(shPops))
		if shInputs == -1 {
			si.ExpectedInputs = -1
		} else {
			si.ExpectedInputs += shInputs
		}

		// All entries pushed to stack (or are OP_RESERVED and exec
		// will fail).
		si.NumInputs = len(sigPops)

	default:
		// All entries pushed to stack (or are OP_RESERVED and exec
		// will fail).
		si.NumInputs = len(sigPops)
	}

	return si, nil
}

// refCalcMultiSigStats returns the number of public keys and signatures from
// a multi-signature transaction script.  The passed script MUST already be
// known to be a multi-signature script.
func refCalcMultiSigStats(script []byte) (int, int, error) {
	pops, err := parseScript(script)
	if err != nil {
		return 0, 0, err
	}

	// A multi-signature script is of the pattern:
	//  NUM_SIGS PUBKEY PUBKEY PUBKEY... NUM_PUBKEYS OP_CHECKMULTISIG
	// Therefore the number of signatures is the oldest item on the stack
	// and the number of pubkeys is the 2nd to last.  Also, the absolute
	// minimum for a multi-signature script is 1 pubkey, so at least 4
	// items must be on the stack per:
	//  OP_1 PUBKEY OP_1 OP_CHECKMULTISIG
	if len(pops) < 4 {
		str := fmt.Sprintf("script %x is not a multisig script", script)
		return 0, 0, scriptError(ErrNotMultisigScript, str)
	}

	numSigs := asSmallInt(pops[0].opcode)
	numPubKeys := asSmallInt(pops[len(pops)-2].opcode)
	return numPubKeys, numSigs, nil
}

// refPushedData returns an array of byte slices containing any pushed data found
// in the passed script.  This includes OP_0, but not OP_1 - OP_16.
func refPushedData(script []byte) ([][]byte, error) {
	pops, err := parseScript(script)
	if err != nil {
		return nil, err
	}

	var data [][]byte
	for _, pop := range pops {
		if pop.data != nil {
			data = append(data, pop.data)
		} else if pop.opcode.value == OP_0 {
			data = append(data, nil)
		}
	}
	return data, nil
}

// refExtractPkScriptAddrs returns the type of script, addresses and required
// signatures associated with the passed PkScript.  Note that it only works for
// 'standard' transaction script types.  Any data such as public keys which are
// invalid are omitted from the results.
func refExtractPkScriptAddrs(pkScript []byte, chainParams *chaincfg.Params) (ScriptClass, []bchutil.Address, int, error) {
	var addrs []bchutil.Address
	var requiredSigs int

	// No valid addresses or required signatures if the script doesn't
	// parse.
	pops, err := parseScript(pkScript)
	if err != nil {
		return NonStandardTy, nil, 0, err
	}

	scriptClass := refTypeOfScript(pops)
	switch scriptClass {
	case PubKeyHashTy:
		// A pay-to-pubkey-hash script is of the form:
		//  OP_DUP OP_HASH160 <hash> OP_EQUALVERIFY OP_CHECKSIG
		// Therefore the pubkey hash is the 3rd item on the stack.
		// Skip the pubkey hash if it's invalid for some reason.
		requiredSigs = 1
		addr, err := bchutil.NewAddressPubKeyHash(pops[2].data,
			chainParams)
		if err == nil {
			addrs = append(addrs, addr)
		}

	case PubKeyTy:
		// A pay-to-pubkey script is of the form:
		//  <pubkey> OP_CHECKSIG
		// Therefore the pubkey is the first item on the stack.
		// Skip the pubkey if it's invalid for some reason.
		requiredSigs = 1
		addr, err := bchutil.NewAddressPubKey(pops[0].data, chainParams)
		if err == nil {
			addrs = append(addrs, addr)
		}

	case ScriptHashTy:
		// A pay-to-script-hash script is of the form:
		//  OP_HASH160 <scripthash> OP_EQUAL
		// Therefore the script hash is the 2nd item on the stack.
		// Skip the script hash if it's invalid for some reason.
		requiredSigs = 1
		addr, err := bchutil.NewAddressScriptHashFromHash(pops[1].data,
			chainParams)
		if err == nil {
			addrs = append(addrs, addr)
		}

	case ScriptHash32Ty:
		// A pay-to-script-hash32 script is of the form:
		//  OP_HASH256 <scripthash> OP_EQUAL
		// Therefore the script hash is the 2nd item on the stack.
		// Skip the script hash if it's invalid for some reason.
		requiredSigs = 1
		addr, err := bchutil.NewAddressScriptHash32FromHash(pops[1].data,
			chainParams)
		if err == nil {
			addrs = append(addrs, addr)
		} else {
			log.Debugf("%v", err)
		}

	case MultiSigTy:
		// A multi-signature script is of the form:
		//  <numsigs> <pubkey> <pubkey> <pubkey>... <numpubkeys> OP_CHECKMULTISIG
		// Therefore the number of required signatures is the 1st item
		// on the stack and the number of public keys is the 2nd to last
		// item on the stack.
		requiredSigs = asSmallInt(pops[0].opcode)
		numPubKeys := asSmallInt(pops[len(pops)-2].opcode)

		// Extract the public keys while skipping any that are invalid.
		addrs = make([]bchutil.Address, 0, numPubKeys)
		for i := range numPubKeys {
			addr, err := bchutil.NewAddressPubKey(pops[i+1].data,
				chainParams)
			if err == nil {
				addrs = append(addrs, addr)
			}
		}

	case NullDataTy:
		// Null data transactions have no addresses or required
		// signatures.

	case NonStandardTy:
		// Don't attempt to extract addresses or required signatures for
		// nonstandard transactions.
	}

	return scriptClass, addrs, requiredSigs, nil
}

// refExtractAtomicSwapDataPushes returns the data pushes from an atomic swap
// contract.  If the script is not an atomic swap contract,
// refExtractAtomicSwapDataPushes returns (nil, nil).  Non-nil errors are returned
// for unparsable scripts.
//
// NOTE: Atomic swaps are not considered standard script types by the dcrd
// mempool policy and should be used with P2SH.  The atomic swap format is also
// expected to change to use a more secure hash function in the future.
//
// This function is only defined in the txscript package due to API limitations
// which prevent callers using txscript to parse nonstandard scripts.
func refExtractAtomicSwapDataPushes(version uint16, pkScript []byte) (*AtomicSwapDataPushes, error) {
	pops, err := parseScript(pkScript)
	if err != nil {
		return nil, err
	}

	if len(pops) != 20 {
		return nil, nil
	}
	isAtomicSwap := pops[0].opcode.value == OP_IF &&
		pops[1].opcode.value == OP_SIZE &&
		canonicalPush(pops[2]) &&
		pops[3].opcode.value == OP_EQUALVERIFY &&
		pops[4].opcode.value == OP_SHA256 &&
		pops[5].opcode.value == OP_DATA_32 &&
		pops[6].opcode.value == OP_EQUALVERIFY &&
		pops[7].opcode.value == OP_DUP &&
		pops[8].opcode.value == OP_HASH160 &&
		pops[9].opcode.value == OP_DATA_20 &&
		pops[10].opcode.value == OP_ELSE &&
		canonicalPush(pops[11]) &&
		pops[12].opcode.value == OP_CHECKLOCKTIMEVERIFY &&
		pops[13].opcode.value == OP_DROP &&
		pops[14].opcode.value == OP_DUP &&
		pops[15].opcode.value == OP_HASH160 &&
		pops[16].opcode.value == OP_DATA_20 &&
		pops[17].opcode.value == OP_ENDIF &&
		pops[18].opcode.value == OP_EQUALVERIFY &&
		pops[19].opcode.value == OP_CHECKSIG
	if !isAtomicSwap {
		return nil, nil
	}

	pushes := new(AtomicSwapDataPushes)
	copy(pushes.SecretHash[:], pops[5].data)
	copy(pushes.RecipientHash160[:], pops[9].data)
	copy(pushes.RefundHash160[:], pops[16].data)
	if pops[2].data != nil {
		locktime, err := makeScriptNum(pops[2].data, true, 5)
		if err != nil {
			return nil, nil
		}
		pushes.SecretSize = locktime.Int64()
	} else if op := pops[2].opcode; isSmallInt(op) {
		pushes.SecretSize = int64(asSmallInt(op))
	} else {
		return nil, nil
	}
	if pops[11].data != nil {
		locktime, err := makeScriptNum(pops[11].data, true, 5)
		if err != nil {
			return nil, nil
		}
		pushes.LockTime = locktime.Int64()
	} else if op := pops[11].opcode; isSmallInt(op) {
		pushes.LockTime = int64(asSmallInt(op))
	} else {
		return nil, nil
	}
	return pushes, nil
}

// refComputePkScript computes the pkScript of an transaction output by looking at
// the transaction input's signature script.
//
// NOTE: Only P2PKH and P2SH (20-byte hash) outputs can be derived. P2SH32 is
// not supported.
func refComputePkScript(sigScript []byte) (PkScript, error) {
	var pkScript PkScript

	// Both P2PKH and P2SH signature scripts only push data.
	if len(sigScript) == 0 || !refIsPushOnlyScript(sigScript) {
		return pkScript, ErrUnsupportedScriptType
	}

	// The redeem script, or the public key, is the last push.
	pops, err := parseScript(sigScript)
	if err != nil {
		return pkScript, err
	}
	last := pops[len(pops)-1].data

	return computePkScript(len(pops) == 2 &&
		isPubKeyHashSigPushes(pops[0].data, last), last)
}

// refNewEngineChecks returns the error, script index and bip16 state the
// checks at the start of NewEngine gave before they used the tokenizer.
func refNewEngineChecks(scriptPubKey []byte, tx *wire.MsgTx, txIdx int,
	flags ScriptFlags) (int, bool, error) {

	if txIdx < 0 || txIdx >= len(tx.TxIn) {
		str := fmt.Sprintf("transaction input index %d is negative or "+
			">= %d", txIdx, len(tx.TxIn))
		return 0, false, scriptError(ErrInvalidIndex, str)
	}
	scriptSig := tx.TxIn[txIdx].SignatureScript

	if flags.HasFlag(ScriptAllowCashTokens) {
		if tx.Version < MinTxVersion || tx.Version > MaxTxVersion {
			str := fmt.Sprintf("transaction version %d is not in the valid range of %d-%d.",
				tx.Version, MinTxVersion, MaxTxVersion)
			return 0, false, scriptError(ErrOpCost, str)
		}
	}

	if len(scriptSig) == 0 && len(scriptPubKey) == 0 {
		return 0, false, scriptError(ErrEvalFalse,
			"false stack entry at end of script execution")
	}

	if flags.HasFlag(ScriptVerifyCleanStack) && !flags.HasFlag(ScriptBip16) {
		return 0, false, scriptError(ErrInvalidFlags,
			"invalid flags combination")
	}

	if flags.HasFlag(ScriptVerifySigPushOnly) && !refIsPushOnlyScript(scriptSig) {
		return 0, false, scriptError(ErrNotPushOnly,
			"signature script is not push only")
	}

	scripts := [][]byte{scriptSig, scriptPubKey}
	parsed := make([][]parsedOpcode, len(scripts))
	for i, scr := range scripts {
		if len(scr) > MaxScriptSize {
			str := fmt.Sprintf("script size %d is larger than max "+
				"allowed size %d", len(scr), MaxScriptSize)
			return 0, false, scriptError(ErrScriptTooBig, str)
		}
		var err error
		parsed[i], err = parseScript(scr)
		if err != nil {
			return 0, false, err
		}
	}

	scriptIdx := 0
	if len(scripts[0]) == 0 {
		scriptIdx++
	}

	bip16 := false
	if flags.HasFlag(ScriptBip16) && refIsScriptHash(parsed[1]) {
		if !refIsPushOnly(parsed[0]) {
			return 0, false, scriptError(ErrNotPushOnly,
				"pay to script hash is not push only")
		}
		bip16 = true
	}

	if flags.HasFlag(ScriptBip16) && flags.HasFlag(ScriptAllowCashTokens) &&
		refIsScriptHash32(parsed[1]) {

		if !refIsPushOnly(parsed[0]) {
			return 0, false, scriptError(ErrNotPushOnly,
				"pay to script hash is not push only")
		}
		bip16 = true
	}

	return scriptIdx, bip16, nil
}
