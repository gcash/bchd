package txscript

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
	"golang.org/x/crypto/ripemd160"
)

const (
	// minECDSASigLen and maxECDSASigLen bound the length of a DER encoded
	// ECDSA signature with its sighash byte.
	minECDSASigLen = 9
	maxECDSASigLen = 73

	// schnorrSigLen is the length of a Schnorr signature with its sighash
	// byte.
	schnorrSigLen = 65

	// compressedPubKeyLen is the length in bytes of a compressed public
	// key.
	compressedPubKeyLen = 33

	// uncompressedPubKeyLen is the length in bytes of an uncompressed
	// public key.
	uncompressedPubKeyLen = 65

	// pubKeyHashLen is the length of a P2PKH script.
	pubKeyHashLen = 25

	// scriptHashLen is the length of a P2SH script.
	scriptHashLen = 23

	// scriptHash32Len is the length of a P2SH32 script.
	scriptHash32Len = 35

	// maxLen is the maximum script length supported by ParsePkScript.
	maxLen = scriptHash32Len
)

var (
	// ErrUnsupportedScriptType is an error returned when we attempt to
	// parse/re-compute an output script into a PkScript struct.
	ErrUnsupportedScriptType = errors.New("unsupported script type")
)

// PkScript is a wrapper struct around a byte array, allowing it to be used
// as a map index.
type PkScript struct {
	// class is the type of the script encoded within the byte array. This
	// is used to determine the correct length of the script within the byte
	// array.
	class ScriptClass

	// script is the script contained within a byte array. If the script is
	// smaller than the length of the byte array, it will be padded with 0s
	// at the end.
	script [maxLen]byte
}

// ParsePkScript parses an output script into the PkScript struct.
// ErrUnsupportedScriptType is returned when attempting to parse an unsupported
// script type.
func ParsePkScript(pkScript []byte, chainParams *chaincfg.Params) (PkScript, error) {
	var outputScript PkScript

	// Default to mainnet if no chainParams are provided.
	if chainParams == nil {
		chainParams = &chaincfg.MainNetParams
	}

	scriptClass, _, _, err := ExtractPkScriptAddrs(
		pkScript, chainParams,
	)
	if err != nil {
		return outputScript, fmt.Errorf("unable to parse script type: "+
			"%v", err)
	}

	if !isSupportedScriptType(scriptClass) {
		return outputScript, ErrUnsupportedScriptType
	}

	outputScript.class = scriptClass
	copy(outputScript.script[:], pkScript)

	return outputScript, nil
}

// isSupportedScriptType determines whether the script type is supported by the
// PkScript struct.
func isSupportedScriptType(class ScriptClass) bool {
	switch class {
	case PubKeyHashTy, ScriptHashTy, ScriptHash32Ty:
		return true
	default:
		return false
	}
}

// Class returns the script type.
func (s PkScript) Class() ScriptClass {
	return s.class
}

// Script returns the script as a byte slice without any padding.
func (s PkScript) Script() []byte {
	var script []byte

	switch s.class {
	case PubKeyHashTy:
		script = make([]byte, pubKeyHashLen)
		copy(script, s.script[:pubKeyHashLen])

	case ScriptHashTy:
		script = make([]byte, scriptHashLen)
		copy(script, s.script[:scriptHashLen])

	case ScriptHash32Ty:
		script = make([]byte, scriptHash32Len)
		copy(script, s.script[:scriptHash32Len])

	default:
		// Unsupported script type.
		return nil
	}

	return script
}

// Address encodes the script into an address for the given chain.
func (s PkScript) Address(chainParams *chaincfg.Params) (bchutil.Address, error) {
	_, addrs, _, err := ExtractPkScriptAddrs(s.Script(), chainParams)
	if err != nil {
		return nil, fmt.Errorf("unable to parse address: %v", err)
	}

	return addrs[0], nil
}

// String returns a hex-encoded string representation of the script.
func (s PkScript) String() string {
	str, _ := DisasmString(s.Script())
	return str
}

// ComputePkScript computes the pkScript of an transaction output by looking at
// the transaction input's signature script.
//
// NOTE: Only P2PKH and P2SH (20-byte hash) outputs can be derived. P2SH32 is
// not supported.
func ComputePkScript(sigScript []byte) (PkScript, error) {
	var pkScript PkScript

	// Both P2PKH and P2SH signature scripts only push data.
	if len(sigScript) == 0 || !IsPushOnlyScript(sigScript) {
		return pkScript, ErrUnsupportedScriptType
	}

	// The redeem script, or the public key, is the last push.
	var first, last []byte
	var numPushes int
	tokenizer := MakeScriptTokenizer(sigScript)
	for tokenizer.Next() {
		if numPushes == 0 {
			first = tokenizer.Data()
		}
		last = tokenizer.Data()
		numPushes++
	}
	if err := tokenizer.Err(); err != nil {
		return pkScript, err
	}

	return computePkScript(numPushes == 2 && isPubKeyHashSigPushes(first, last), last)
}

// isPubKeyHashSigPushes returns whether the two pushes of a signature script
// are a signature and a public key, as when spending a P2PKH output.
//
// A P2SH spend that pushes the same shapes can't be told apart from it.
func isPubKeyHashSigPushes(sig, pubKey []byte) bool {
	switch {
	case len(sig) == schnorrSigLen:
	case len(sig) >= minECDSASigLen && len(sig) <= maxECDSASigLen &&
		sig[0] == 0x30 && int(sig[1]) == len(sig)-3:
	default:
		return false
	}

	switch {
	case len(pubKey) == compressedPubKeyLen && bchec.IsCompressedPubKey(pubKey):
	case len(pubKey) == uncompressedPubKeyLen && pubKey[0] == 0x04:
	default:
		return false
	}
	_, err := bchec.ParsePubKey(pubKey, bchec.S256())
	return err == nil
}

// computePkScript returns the P2PKH script paying to the public key last, or
// the P2SH script paying to the redeem script last.
func computePkScript(isPubKeyHash bool, last []byte) (PkScript, error) {
	var pkScript PkScript

	if isPubKeyHash {
		script, err := payToPubKeyHashScript(hash160(last))
		if err != nil {
			return pkScript, err
		}

		pkScript.class = PubKeyHashTy
		copy(pkScript.script[:], script)
		return pkScript, nil
	}

	script, err := payToScriptHashScript(hash160(last))
	if err != nil {
		return pkScript, err
	}

	pkScript.class = ScriptHashTy
	copy(pkScript.script[:], script)
	return pkScript, nil
}

// hash160 returns the RIPEMD160 hash of the SHA-256 HASH of the given data.
func hash160(data []byte) []byte {
	h := sha256.Sum256(data)
	return ripemd160h(h[:])
}

// ripemd160h returns the RIPEMD160 hash of the given data.
func ripemd160h(data []byte) []byte {
	h := ripemd160.New()
	h.Write(data)
	return h.Sum(nil)
}
