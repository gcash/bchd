// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"fmt"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
)

const (
	// MaxDataCarrierSize is the maximum number of bytes allowed in pushed
	// data to be considered a nulldata transaction
	MaxDataCarrierSize = 223

	// MaxP2SScriptSize is the maximum length of a public key script of no
	// recognized form that is standard since the May 2026 upgrade (pay to
	// script).
	MaxP2SScriptSize = 201

	// StandardVerifyFlags are the script flags which are used when
	// executing transaction scripts to enforce additional checks which
	// are required for the script to be considered standard.  These checks
	// help reduce issues related to transaction malleability as well as
	// allow pay-to-script hash transactions.  Note these flags are
	// different than what is required for the consensus rules in that they
	// are more strict.
	//
	// TODO: This definition does not belong here.  It belongs in a policy
	// package.
	StandardVerifyFlags = ScriptBip16 |
		ScriptVerifyDERSignatures |
		ScriptVerifyStrictEncoding |
		ScriptVerifyMinimalData |
		ScriptStrictMultiSig |
		ScriptDiscourageUpgradableNops |
		ScriptVerifyCleanStack |
		ScriptVerifyNullFail |
		ScriptVerifyCheckLockTimeVerify |
		ScriptVerifyCheckSequenceVerify |
		ScriptVerifyLowS |
		ScriptStrictMultiSig |
		ScriptVerifyBip143SigHash |
		ScriptVerifySigPushOnly |
		ScriptVerifyCheckDataSig |
		ScriptVerifySchnorr |
		ScriptVerifySchnorrMultisig |
		ScriptVerifyReverseBytes |
		ScriptReportSigChecks |
		ScriptVerifyInputSigChecks |
		ScriptVerify64BitIntegers |
		ScriptVerifyNativeIntrospection

	// RelayVerifyFlags are the script flags a transaction must pass to be
	// relayed once all the upgrades this version knows are active, as the
	// memory pool sets them by default on mainnet: StandardVerifyFlags
	// with the CashTokens, May 2025 and May 2026 rules.  The memory pool's
	// tests keep them its default.
	RelayVerifyFlags = StandardVerifyFlags | ScriptAllowCashTokens |
		ScriptAllowMay2025 | ScriptAllowMay2025StandardOnly |
		ScriptAllowMay2026
)

// ScriptClass is an enumeration for the list of standard types of script.
type ScriptClass byte

// Classes of script payment known about in the blockchain.
const (
	NonStandardTy  ScriptClass = iota // None of the recognized forms.
	PubKeyTy                          // Pay pubkey.
	PubKeyHashTy                      // Pay pubkey hash.
	ScriptHashTy                      // Pay to script hash.
	ScriptHash32Ty                    // Pay to script hash 32
	MultiSigTy                        // Multi signature.
	NullDataTy                        // Empty data-only (provably prunable).
)

// scriptClassToName houses the human-readable strings which describe each
// script class.
var scriptClassToName = []string{
	NonStandardTy:  "nonstandard",
	PubKeyTy:       "pubkey",
	PubKeyHashTy:   "pubkeyhash",
	ScriptHashTy:   "scripthash",
	ScriptHash32Ty: "scripthash",
	MultiSigTy:     "multisig",
	NullDataTy:     "nulldata",
}

// String implements the Stringer interface by returning the name of
// the enum script class. If the enum is invalid then "Invalid" will be
// returned.
func (t ScriptClass) String() string {
	if int(t) >= len(scriptClassToName) || int(t) < 0 {
		return "Invalid"
	}
	return scriptClassToName[t]
}

// isValidPubKeySize returns whether the length of a public key fits its first
// byte, as BCHN requires of the keys in the pubkey and multisig templates: 33
// bytes after 0x02 or 0x03, and 65 bytes after 0x04, 0x06 or 0x07.
func isValidPubKeySize(pubKey []byte) bool {
	if len(pubKey) == 0 {
		return false
	}
	switch pubKey[0] {
	case 0x02, 0x03:
		return len(pubKey) == 33
	case 0x04, 0x06, 0x07:
		return len(pubKey) == 65
	}
	return false
}

// isPubKeyScript returns whether the script is a pay-to-pubkey script, as
// BCHN matches one: a direct push of a public key whose length fits its first
// byte, followed by OP_CHECKSIG.
func isPubKeyScript(script []byte) bool {
	// The key must be pushed directly, so the opcode is its length.
	n := len(script)
	return n >= 2 && int(script[0]) == n-2 &&
		isValidPubKeySize(script[1:n-1]) && script[n-1] == OP_CHECKSIG
}

// isPubKeyHashScript returns whether the script is a pay-to-pubkey-hash
// script.
func isPubKeyHashScript(script []byte) bool {
	return len(script) == 25 &&
		script[0] == OP_DUP &&
		script[1] == OP_HASH160 &&
		script[2] == OP_DATA_20 &&
		script[23] == OP_EQUALVERIFY &&
		script[24] == OP_CHECKSIG
}

// isMultiSigScript returns whether the script is a multisig script, as BCHN
// matches one: OP_1 to OP_16 for the number of signatures, direct pushes of
// one or more public keys whose lengths fit their first bytes, OP_1 to OP_16
// for the number of keys, which may not be less than the number of
// signatures, and OP_CHECKMULTISIG.
func isMultiSigScript(script []byte) bool {
	n := len(script)
	if n < 4 || script[0] < OP_1 || script[0] > OP_16 ||
		script[n-2] < OP_1 || script[n-2] > OP_16 ||
		script[n-1] != OP_CHECKMULTISIG {

		return false
	}
	numPubKeys := 0
	for i := 1; i < n-2; {
		// Keys must be pushed directly, so each opcode is the length of
		// its key.
		end := i + 1 + int(script[i])
		if end > n-2 || !isValidPubKeySize(script[i+1:end]) {
			return false
		}
		numPubKeys++
		i = end
	}
	numSigs := asSmallInt(&opcodeArray[script[0]])
	return asSmallInt(&opcodeArray[script[n-2]]) == numPubKeys &&
		numSigs <= numPubKeys
}

// isNullDataScript returns whether the script is a null data script:
// OP_RETURN followed only by pushes, and no longer than MaxDataCarrierSize.
func isNullDataScript(script []byte) bool {
	return len(script) <= MaxDataCarrierSize && isDataCarrierScript(script)
}

// isDataCarrierScript returns whether the script is OP_RETURN followed only
// by pushes, whatever its length.
func isDataCarrierScript(script []byte) bool {
	if len(script) < 1 || script[0] != OP_RETURN {
		return false
	}
	tokenizer := MakeScriptTokenizer(script[1:])
	for tokenizer.Next() {
		if tokenizer.Opcode() > OP_16 {
			return false
		}
	}
	return tokenizer.Err() == nil
}

// typeOfScript returns the type of the script being inspected from the known
// standard types.
func typeOfScript(script []byte) ScriptClass {
	switch {
	case isPubKeyScript(script):
		return PubKeyTy
	case isPubKeyHashScript(script):
		return PubKeyHashTy
	case IsPayToScriptHash(script):
		return ScriptHashTy
	case IsPayToScriptHash32(script):
		return ScriptHash32Ty
	case isMultiSigScript(script):
		return MultiSigTy
	case isNullDataScript(script):
		return NullDataTy
	}
	return NonStandardTy
}

// checkScriptParses returns the error parsing the script gives, if any.
func checkScriptParses(script []byte) error {
	tokenizer := MakeScriptTokenizer(script)
	for tokenizer.Next() {
	}
	return tokenizer.Err()
}

// GetScriptClass returns the class of the script passed.
//
// NonStandardTy will be returned when the script does not parse.
func GetScriptClass(script []byte) ScriptClass {
	return typeOfScript(script)
}

// ScriptTypeName returns the name RPC results give the type of a public key
// script.  It differs from the name of its class in two ways: OP_RETURN
// followed only by pushes is nulldata whatever its length, and any other
// script of no recognized form is script (pay to script, standard since the
// May 2026 upgrade) when it is no longer than MaxP2SScriptSize.
func ScriptTypeName(script []byte) string {
	class := typeOfScript(script)
	if class != NonStandardTy {
		return class.String()
	}
	switch {
	case isDataCarrierScript(script):
		return NullDataTy.String()
	case len(script) <= MaxP2SScriptSize:
		return "script"
	}
	return class.String()
}

// expectedInputs returns the number of arguments required by a script.
// If the script is of unknown type such that the number can not be determined
// then -1 is returned. We are an internal function and thus assume that class
// is the real class of script (and we can thus assume things that were
// determined while finding out the type).
func expectedInputs(script []byte, class ScriptClass) int {
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
		return asSmallInt(&opcodeArray[script[0]]) + 1

	case NullDataTy:
		fallthrough
	default:
		return -1
	}
}

// ScriptInfo houses information about a script pair that is determined by
// CalcScriptInfo.
type ScriptInfo struct {
	// PkScriptClass is the class of the public key script and is equivalent
	// to calling GetScriptClass on it.
	PkScriptClass ScriptClass

	// NumInputs is the number of inputs provided by the public key script.
	NumInputs int

	// ExpectedInputs is the number of outputs required by the signature
	// script and any pay-to-script-hash scripts. The number will be -1 if
	// unknown.
	ExpectedInputs int
}

// CalcScriptInfo returns a structure providing data about the provided script
// pair.  It will error if the pair is in someway invalid such that they can not
// be analysed, i.e. if they do not parse or the pkScript is not a push-only
// script
func CalcScriptInfo(sigScript, pkScript []byte, scriptFlags ScriptFlags) (*ScriptInfo, error) {

	sigPops, err := parseScript(sigScript)
	if err != nil {
		return nil, err
	}

	if err := checkScriptParses(pkScript); err != nil {
		return nil, err
	}

	// Push only sigScript makes little sense.
	si := new(ScriptInfo)
	si.PkScriptClass = typeOfScript(pkScript)

	// Can't have a signature script that doesn't just push data.
	if !isPushOnly(sigPops) {
		return nil, scriptError(ErrNotPushOnly,
			"signature script is not push only")
	}

	si.ExpectedInputs = expectedInputs(pkScript, si.PkScriptClass)

	switch {
	case si.PkScriptClass == ScriptHashTy && scriptFlags.HasFlag(ScriptBip16):
		// The pay-to-hash-script is the final data push of the
		// signature script, so an empty one has none.
		if len(sigPops) == 0 {
			return nil, scriptError(ErrInvalidStackOperation,
				"signature script is empty")
		}
		script := sigPops[len(sigPops)-1].data
		if err := checkScriptParses(script); err != nil {
			return nil, err
		}

		shInputs := expectedInputs(script, typeOfScript(script))
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

// CalcMultiSigStats returns the number of public keys and signatures from
// a multi-signature transaction script.  The passed script MUST already be
// known to be a multi-signature script.
func CalcMultiSigStats(script []byte) (int, int, error) {

	// A multi-signature script is of the pattern:
	//  NUM_SIGS PUBKEY PUBKEY PUBKEY... NUM_PUBKEYS OP_CHECKMULTISIG
	// Therefore the number of signatures is the oldest item on the stack
	// and the number of pubkeys is the 2nd to last.  Also, the absolute
	// minimum for a multi-signature script is 1 pubkey, so at least 4
	// items must be on the stack per:
	//  OP_1 PUBKEY OP_1 OP_CHECKMULTISIG
	var first, secondToLast, last *opcode
	numOpcodes := 0
	tokenizer := MakeScriptTokenizer(script)
	for tokenizer.Next() {
		if numOpcodes == 0 {
			first = tokenizer.op
		}
		secondToLast, last = last, tokenizer.op
		numOpcodes++
	}
	if err := tokenizer.Err(); err != nil {
		return 0, 0, err
	}
	if numOpcodes < 4 {
		str := fmt.Sprintf("script %x is not a multisig script", script)
		return 0, 0, scriptError(ErrNotMultisigScript, str)
	}

	numSigs := asSmallInt(first)
	numPubKeys := asSmallInt(secondToLast)
	return numPubKeys, numSigs, nil
}

// payToPubKeyHashScript creates a new script to pay a transaction
// output to a 20-byte pubkey hash. It is expected that the input is a valid
// hash.
func payToPubKeyHashScript(pubKeyHash []byte) ([]byte, error) {
	return NewScriptBuilder().AddOp(OP_DUP).AddOp(OP_HASH160).
		AddData(pubKeyHash).AddOp(OP_EQUALVERIFY).AddOp(OP_CHECKSIG).
		Script()
}

// payToScriptHashScript creates a new script to pay a transaction output to a
// script hash. It is expected that the input is a valid hash.
func payToScriptHashScript(scriptHash []byte) ([]byte, error) {
	return NewScriptBuilder().AddOp(OP_HASH160).AddData(scriptHash).
		AddOp(OP_EQUAL).Script()
}

// payToScriptHashScript32 creates a new script to pay a transaction output to a
// script hash. It is expected that the input is a valid hash.
func payToScriptHashScript32(scriptHash []byte) ([]byte, error) {
	return NewScriptBuilder().AddOp(OP_HASH256).AddData(scriptHash).
		AddOp(OP_EQUAL).Script()
}

// payToPubKeyScript creates a new script to pay a transaction output to a
// public key. It is expected that the input is a valid pubkey.
func payToPubKeyScript(serializedPubKey []byte) ([]byte, error) {
	return NewScriptBuilder().AddData(serializedPubKey).
		AddOp(OP_CHECKSIG).Script()
}

// PayToAddrScript creates a new script to pay a transaction output to a the
// specified address.
func PayToAddrScript(addr bchutil.Address) ([]byte, error) {
	const nilAddrErrStr = "unable to generate payment script for nil address"

	switch addr := addr.(type) {
	case *bchutil.LegacyAddressPubKeyHash:
		if addr == nil {
			return nil, scriptError(ErrUnsupportedAddress,
				nilAddrErrStr)
		}
		return payToPubKeyHashScript(addr.ScriptAddress())
	case *bchutil.AddressPubKeyHash:
		if addr == nil {
			return nil, scriptError(ErrUnsupportedAddress,
				nilAddrErrStr)
		}
		return payToPubKeyHashScript(addr.ScriptAddress())
	case *bchutil.LegacyAddressScriptHash:
		if addr == nil {
			return nil, scriptError(ErrUnsupportedAddress,
				nilAddrErrStr)
		}
		return payToScriptHashScript(addr.ScriptAddress())
	case *bchutil.AddressScriptHash:
		if addr == nil {
			return nil, scriptError(ErrUnsupportedAddress,
				nilAddrErrStr)
		}
		return payToScriptHashScript(addr.ScriptAddress())
	case *bchutil.AddressScriptHash32:
		if addr == nil {
			return nil, scriptError(ErrUnsupportedAddress,
				nilAddrErrStr)
		}
		return payToScriptHashScript32(addr.ScriptAddress())
	case *bchutil.AddressPubKey:
		if addr == nil {
			return nil, scriptError(ErrUnsupportedAddress,
				nilAddrErrStr)
		}
		return payToPubKeyScript(addr.ScriptAddress())
	}

	str := fmt.Sprintf("unable to generate payment script for unsupported "+
		"address type %T", addr)
	return nil, scriptError(ErrUnsupportedAddress, str)
}

// NullDataScript creates a provably-prunable script containing OP_RETURN
// followed by the passed data.  An Error with the error code ErrTooMuchNullData
// will be returned if the length of the passed data exceeds MaxDataCarrierSize.
func NullDataScript(data []byte) ([]byte, error) {
	maxDataSize := MaxDataCarrierSize - 3 // subtract size of OP_RETURN PUSHDATA1 <len>
	if len(data) > maxDataSize {
		str := fmt.Sprintf("data size %d is larger than max "+
			"allowed size %d", len(data), MaxDataCarrierSize)
		return nil, scriptError(ErrTooMuchNullData, str)
	}

	return NewScriptBuilder().AddOp(OP_RETURN).AddData(data).Script()
}

// MultiSigScript returns a valid script for a multisignature redemption where
// nrequired of the keys in pubkeys are required to have signed the transaction
// for success.  An Error with the error code ErrTooManyRequiredSigs will be
// returned if nrequired is larger than the number of keys provided.
func MultiSigScript(pubkeys []*bchutil.AddressPubKey, nrequired int) ([]byte, error) {
	if len(pubkeys) < nrequired {
		str := fmt.Sprintf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", nrequired, len(pubkeys))
		return nil, scriptError(ErrTooManyRequiredSigs, str)
	}

	builder := NewScriptBuilder().AddInt64(int64(nrequired))
	for _, key := range pubkeys {
		builder.AddData(key.ScriptAddress())
	}
	builder.AddInt64(int64(len(pubkeys)))
	builder.AddOp(OP_CHECKMULTISIG)

	return builder.Script()
}

// PushedData returns an array of byte slices containing any pushed data found
// in the passed script.  This includes OP_0, but not OP_1 - OP_16.
func PushedData(script []byte) ([][]byte, error) {
	var data [][]byte
	tokenizer := MakeScriptTokenizer(script)
	for tokenizer.Next() {
		if tokenizer.Data() != nil {
			data = append(data, tokenizer.Data())
		} else if tokenizer.Opcode() == OP_0 {
			data = append(data, nil)
		}
	}
	if err := tokenizer.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

// ExtractPkScriptAddrs returns the type of script, addresses and required
// signatures associated with the passed PkScript.  Note that it only works for
// 'standard' transaction script types.  Any data such as public keys which are
// invalid are omitted from the results.
func ExtractPkScriptAddrs(pkScript []byte, chainParams *chaincfg.Params) (ScriptClass, []bchutil.Address, int, error) {
	var addrs []bchutil.Address
	var requiredSigs int

	scriptClass := typeOfScript(pkScript)
	switch scriptClass {
	case PubKeyHashTy:
		// A pay-to-pubkey-hash script is of the form:
		//  OP_DUP OP_HASH160 <hash> OP_EQUALVERIFY OP_CHECKSIG
		// Therefore the pubkey hash is the 3rd item on the stack.
		// Skip the pubkey hash if it's invalid for some reason.
		requiredSigs = 1
		addr, err := bchutil.NewAddressPubKeyHash(pkScript[3:23],
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
		tokenizer := MakeScriptTokenizer(pkScript)
		tokenizer.Next()
		addr, err := bchutil.NewAddressPubKey(tokenizer.Data(), chainParams)
		if err == nil {
			addrs = append(addrs, addr)
		}

	case ScriptHashTy:
		// A pay-to-script-hash script is of the form:
		//  OP_HASH160 <scripthash> OP_EQUAL
		// Therefore the script hash is the 2nd item on the stack.
		// Skip the script hash if it's invalid for some reason.
		requiredSigs = 1
		addr, err := bchutil.NewAddressScriptHashFromHash(pkScript[2:22],
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
		addr, err := bchutil.NewAddressScriptHash32FromHash(pkScript[2:34],
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
		requiredSigs = asSmallInt(&opcodeArray[pkScript[0]])
		numPubKeys := asSmallInt(&opcodeArray[pkScript[len(pkScript)-2]])

		// Extract the public keys while skipping any that are invalid.
		addrs = make([]bchutil.Address, 0, numPubKeys)
		tokenizer := MakeScriptTokenizer(pkScript[1:])
		for range numPubKeys {
			tokenizer.Next()
			addr, err := bchutil.NewAddressPubKey(tokenizer.Data(),
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
		// nonstandard transactions, and return the error of scripts that
		// don't parse.
		if err := checkScriptParses(pkScript); err != nil {
			return NonStandardTy, nil, 0, err
		}
	}

	return scriptClass, addrs, requiredSigs, nil
}

// AtomicSwapDataPushes houses the data pushes found in atomic swap contracts.
type AtomicSwapDataPushes struct {
	RecipientHash160 [20]byte
	RefundHash160    [20]byte
	SecretHash       [32]byte
	SecretSize       int64
	LockTime         int64
}

// ExtractAtomicSwapDataPushes returns the data pushes from an atomic swap
// contract.  If the script is not an atomic swap contract,
// ExtractAtomicSwapDataPushes returns (nil, nil).  Non-nil errors are returned
// for unparsable scripts.
//
// NOTE: Atomic swaps are not considered standard script types by the dcrd
// mempool policy and should be used with P2SH.  The atomic swap format is also
// expected to change to use a more secure hash function in the future.
//
// This function is only defined in the txscript package due to API limitations
// which prevent callers using txscript to parse nonstandard scripts.
func ExtractAtomicSwapDataPushes(version uint16, pkScript []byte) (*AtomicSwapDataPushes, error) {
	// Keep the first 20 opcodes, but walk the whole script so scripts that
	// don't parse still return their error.
	var pops [20]parsedOpcode
	numOpcodes := 0
	tokenizer := MakeScriptTokenizer(pkScript)
	for tokenizer.Next() {
		if numOpcodes < len(pops) {
			pops[numOpcodes] = parsedOpcode{opcode: tokenizer.op,
				data: tokenizer.Data()}
		}
		numOpcodes++
	}
	if err := tokenizer.Err(); err != nil {
		return nil, err
	}

	if numOpcodes != 20 {
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
