// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchec

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"math/big"
)

// Errors returned by canonicalPadding.
var (
	errNegativeValue          = errors.New("value may be interpreted as negative")
	errExcessivelyPaddedValue = errors.New("value is excessively padded")
)

// SignatureType enumerates the type of signature. Either ECDSA or Schnorr
type SignatureType uint8

const (
	// SignatureTypeECDSA defines an ecdsa signature
	SignatureTypeECDSA SignatureType = iota

	// SignatureTypeSchnorr defines a schnorr signature
	SignatureTypeSchnorr
)

// Signature is a type representing either an ecdsa or schnorr signature.
type Signature struct {
	R       *big.Int
	S       *big.Int
	sigType SignatureType
}

var (
	// Used in RFC6979 implementation when testing the nonce for correctness
	one = big.NewInt(1)

	// oneInitializer is used to fill a byte slice with byte 0x01.  It is provided
	// here to avoid the need to create it multiple times.
	oneInitializer = []byte{0x01}
)

// Serialize returns the a serialized signature depending on the SignatureType.
// Note that the serialized bytes returned do not include the appended hash type
// used in Bitcoin signature scripts.
//
// ECDSA signature in the more strict DER format.
//
// encoding/asn1 is broken so we hand roll this output:
//
// 0x30 <length> 0x02 <length r> r 0x02 <length s> s
func (sig *Signature) Serialize() []byte {
	// Schnorr signatures are easy to serialize
	if sig.sigType == SignatureTypeSchnorr {
		return append(padIntBytes(sig.R), padIntBytes(sig.S)...)
	}
	// low 'S' malleability breaker
	sigS := sig.S
	if sigS.Cmp(S256().halfOrder) == 1 {
		sigS = new(big.Int).Sub(S256().N, sigS)
	}
	// Ensure the encoded bytes for the r and s values are canonical and
	// thus suitable for DER encoding.
	rb := canonicalizeInt(sig.R)
	sb := canonicalizeInt(sigS)

	// total length of returned signature is 1 byte for each magic and
	// length (6 total), plus lengths of r and s
	length := 6 + len(rb) + len(sb)
	b := make([]byte, length)

	b[0] = 0x30
	b[1] = byte(length - 2)
	b[2] = 0x02
	b[3] = byte(len(rb))
	offset := copy(b[4:], rb) + 4
	b[offset] = 0x02
	b[offset+1] = byte(len(sb))
	copy(b[offset+2:], sb)
	return b
}

// Verify verifies either an ECDSA or Schnorr signature depending
// on the SignatureType of the signature. It returns true if the
// signature is valid, false otherwise.
func (sig *Signature) Verify(hash []byte, pubKey *PublicKey) bool {
	if sig.sigType == SignatureTypeECDSA {
		return ecdsa.Verify(pubKey.ToECDSA(), hash, sig.R, sig.S)
	} else if sig.sigType == SignatureTypeSchnorr {
		return verifySchnorr(pubKey, hash, sig.R, sig.S)
	}
	// unknown signature type
	return false
}

// verifySchnorr verifies the schnorr signature of the hash using the pubkey key.
// It returns true if the signature is valid, false otherwise.
func verifySchnorr(pubKey *PublicKey, hash []byte, r *big.Int, s *big.Int) bool {
	// This schnorr specification is specific to the secp256k1 curve so if the
	// provided curve is not a KoblitizCurve then we'll just return false.
	curve, ok := pubKey.Curve.(*KoblitzCurve)
	if !ok {
		return false
	}

	// Signature is invalid if s >= order or r >= p.
	if s.Cmp(curve.Params().N) >= 0 || r.Cmp(curve.Params().P) >= 0 {
		return false
	}

	// Compute scalar e = Hash(r || compressed(P) || m) mod N
	eBytes := sha256.Sum256(append(append(padIntBytes(r), pubKey.SerializeCompressed()...), hash...))
	e := new(big.Int).SetBytes(eBytes[:])
	e.Mod(e, curve.Params().N)

	// Negate e
	e.Neg(e).Mod(e, curve.Params().N)

	// Compute point R = s * G - e * P.
	sgx, sgy, sgz := curve.scalarBaseMultJacobian(s.Bytes())
	epx, epy, epz := curve.scalarMultJacobian(pubKey.X, pubKey.Y, e.Bytes())
	rx, ry, rz := new(fieldVal), new(fieldVal), new(fieldVal)
	curve.addJacobian(sgx, sgy, sgz, epx, epy, epz, rx, ry, rz)

	// Check that R is not infinity
	if rz.Equals(new(fieldVal).SetInt(0)) {
		return false
	}

	// Check if R.y is quadratic residue
	yz := ry.Mul(rz).Normalize()
	b := yz.Bytes()
	if big.Jacobi(new(big.Int).SetBytes(b[:]), curve.P) != 1 {
		return false
	}

	// Check R values match
	// rx ≠ rz^2 * r mod p
	fieldR := new(fieldVal).SetByteSlice(r.Bytes())
	return rx.Equals(rz.Square().Mul(fieldR))
}

// IsEqual compares this Signature instance to the one passed, returning true
// if both Signatures are equivalent. A signature is equivalent to another, if
// they both have the same scalar value for R and S.
func (sig *Signature) IsEqual(otherSig *Signature) bool {
	return sig.R.Cmp(otherSig.R) == 0 &&
		sig.S.Cmp(otherSig.S) == 0
}

// minSigLen is the minimum length of a DER encoded signature and is
// when both R and S are 1 byte each.
// 0x30 + <1-byte> + 0x02 + 0x01 + <byte> + 0x2 + 0x01 + <byte>
const minSigLen = 8

func parseECDSASig(sigStr []byte, curve elliptic.Curve, der bool) (*Signature, error) {
	// Originally this code used encoding/asn1 in order to parse the
	// signature, but a number of problems were found with this approach.
	// Despite the fact that signatures are stored as DER, the difference
	// between go's idea of a bignum (and that they have sign) doesn't agree
	// with the openssl one (where they do not). The above is true as of
	// Go 1.1. In the end it was simpler to rewrite the code to explicitly
	// understand the format which is this:
	// 0x30 <length of whole message> <0x02> <length of R> <R> 0x2
	// <length of S> <S>.

	signature := &Signature{sigType: SignatureTypeECDSA}

	if len(sigStr) < minSigLen {
		return nil, errors.New("malformed signature: too short")
	}
	// 0x30
	index := 0
	if sigStr[index] != 0x30 {
		return nil, errors.New("malformed signature: no header magic")
	}
	index++
	// length of remaining message
	siglen := sigStr[index]
	index++

	// siglen should be less than the entire message and greater than
	// the minimal message size.
	if int(siglen+2) > len(sigStr) || int(siglen+2) < minSigLen {
		return nil, errors.New("malformed signature: bad length")
	}
	// trim the slice we're working on so we only look at what matters.
	sigStr = sigStr[:siglen+2]

	// 0x02
	if sigStr[index] != 0x02 {
		return nil,
			errors.New("malformed signature: no 1st int marker")
	}
	index++

	// Length of signature R.
	rLen := int(sigStr[index])
	// must be positive, must be able to fit in another 0x2, <len> <s>
	// hence the -3. We assume that the length must be at least one byte.
	index++
	if rLen <= 0 || rLen > len(sigStr)-index-3 {
		return nil, errors.New("malformed signature: bogus R length")
	}

	// Then R itself.
	rBytes := sigStr[index : index+rLen]
	if der {
		switch err := canonicalPadding(rBytes); err {
		case errNegativeValue:
			return nil, errors.New("signature R is negative")
		case errExcessivelyPaddedValue:
			return nil, errors.New("signature R is excessively padded")
		}
	}
	signature.R = new(big.Int).SetBytes(rBytes)
	index += rLen
	// 0x02. length already checked in previous if.
	if sigStr[index] != 0x02 {
		return nil, errors.New("malformed signature: no 2nd int marker")
	}
	index++

	// Length of signature S.
	sLen := int(sigStr[index])
	index++
	// S should be the rest of the string.
	if sLen <= 0 || sLen > len(sigStr)-index {
		return nil, errors.New("malformed signature: bogus S length")
	}

	// Then S itself.
	sBytes := sigStr[index : index+sLen]
	if der {
		switch err := canonicalPadding(sBytes); err {
		case errNegativeValue:
			return nil, errors.New("signature S is negative")
		case errExcessivelyPaddedValue:
			return nil, errors.New("signature S is excessively padded")
		}
	}
	signature.S = new(big.Int).SetBytes(sBytes)
	index += sLen

	// sanity check length parsing
	if index != len(sigStr) {
		return nil, fmt.Errorf("malformed signature: bad final length %v != %v",
			index, len(sigStr))
	}

	// Verify also checks this, but we can be more sure that we parsed
	// correctly if we verify here too.
	// FWIW the ecdsa spec states that R and S must be | 1, N - 1 |
	// but crypto/ecdsa only checks for Sign != 0. Mirror that.
	if signature.R.Sign() != 1 {
		return nil, errors.New("signature R isn't 1 or more")
	}
	if signature.S.Sign() != 1 {
		return nil, errors.New("signature S isn't 1 or more")
	}
	if signature.R.Cmp(curve.Params().N) >= 0 {
		return nil, errors.New("signature R is >= curve.N")
	}
	if signature.S.Cmp(curve.Params().N) >= 0 {
		return nil, errors.New("signature S is >= curve.N")
	}

	return signature, nil
}

func parseSchnorrSig(sigStr []byte) (*Signature, error) {
	if len(sigStr) != 64 {
		return nil, errors.New("malformed schnorr signature: not 64 bytes")
	}
	bigR := new(big.Int).SetBytes(sigStr[:32])
	bigS := new(big.Int).SetBytes(sigStr[32:64])
	return &Signature{
		R:       bigR,
		S:       bigS,
		sigType: SignatureTypeSchnorr,
	}, nil
}

// ParseBERSignature parses an ECDSA signature in BER format for the curve type `curve'
// into a Signature type, perfoming some basic sanity checks.  If parsing
// according to the more strict DER format is needed, use ParseDERSignature.
func ParseBERSignature(sigStr []byte, curve elliptic.Curve) (*Signature, error) {
	return parseECDSASig(sigStr, curve, false)
}

// ParseDERSignature parses a signature in DER format for the curve type
// `curve` into a Signature type.  If parsing according to the less strict
// BER format is needed, use ParseBERSignature.
func ParseDERSignature(sigStr []byte, curve elliptic.Curve) (*Signature, error) {
	return parseECDSASig(sigStr, curve, true)
}

// ParseSchnorrSignature parses a 64 byte schnorr signature into a Signature type.
func ParseSchnorrSignature(sigStr []byte) (*Signature, error) {
	return parseSchnorrSig(sigStr)
}

// canonicalizeInt returns the bytes for the passed big integer adjusted as
// necessary to ensure that a big-endian encoded integer can't possibly be
// misinterpreted as a negative number.  This can happen when the most
// significant bit is set, so it is padded by a leading zero byte in this case.
// Also, the returned bytes will have at least a single byte when the passed
// value is 0.  This is required for DER encoding.
func canonicalizeInt(val *big.Int) []byte {
	b := val.Bytes()
	if len(b) == 0 {
		b = []byte{0x00}
	}
	if b[0]&0x80 != 0 {
		paddedBytes := make([]byte, len(b)+1)
		copy(paddedBytes[1:], b)
		b = paddedBytes
	}
	return b
}

// padIntBytes pads a big int bytes with leading zeros if they
// are missing to get the length up to 32 bytes.
func padIntBytes(val *big.Int) []byte {
	b := val.Bytes()
	pad := bytes.Repeat([]byte{0x00}, 32-len(b))
	return append(pad, b...)
}

// canonicalPadding checks whether a big-endian encoded integer could
// possibly be misinterpreted as a negative number (even though OpenSSL
// treats all numbers as unsigned), or if there is any unnecessary
// leading zero padding.
func canonicalPadding(b []byte) error {
	switch {
	case b[0]&0x80 == 0x80:
		return errNegativeValue
	case len(b) > 1 && b[0] == 0x00 && b[1]&0x80 != 0x80:
		return errExcessivelyPaddedValue
	default:
		return nil
	}
}

// hashToInt converts a hash value to an integer. There is some disagreement
// about how this is done. [NSA] suggests that this is done in the obvious
// manner, but [SECG] truncates the hash to the bit-length of the curve order
// first. We follow [SECG] because that's what OpenSSL does. Additionally,
// OpenSSL right shifts excess bits from the number if the hash is too large
// and we mirror that too.
// This is borrowed from crypto/ecdsa.
func hashToInt(hash []byte, c elliptic.Curve) *big.Int {
	orderBits := c.Params().N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}

	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}

// recoverKeyFromSignature recovers a public key from the ECSDA signature "sig"
// on the given message hash "msg". Based on the algorithm found in section 5.1.5
// of SEC 1 Ver 2.0, page 47-48 (53 and 54 in the pdf). This performs the details
// in the inner loop in Step 1. The counter provided is actually the j parameter
// of the loop * 2 - on the first iteration of j we do the R case, else the -R
// case in step 1.6. This counter is used in the bitcoin compressed signature
// format and thus we match bitcoind's behaviour here.
func recoverKeyFromSignature(curve *KoblitzCurve, sig *Signature, msg []byte,
	iter int, doChecks bool) (*PublicKey, error) {
	// 1.1 x = (n * i) + r
	Rx := new(big.Int).Mul(curve.Params().N,
		new(big.Int).SetInt64(int64(iter/2)))
	Rx.Add(Rx, sig.R)
	if Rx.Cmp(curve.Params().P) != -1 {
		return nil, errors.New("calculated Rx is larger than curve P")
	}

	// convert 02<Rx> to point R. (step 1.2 and 1.3). If we are on an odd
	// iteration then 1.6 will be done with -R, so we calculate the other
	// term when uncompressing the point.
	Ry, err := decompressPoint(curve, Rx, iter%2 == 1)
	if err != nil {
		return nil, err
	}

	// 1.4 Check n*R is point at infinity
	if doChecks {
		nRx, nRy := curve.ScalarMult(Rx, Ry, curve.Params().N.Bytes())
		if nRx.Sign() != 0 || nRy.Sign() != 0 {
			return nil, errors.New("n*R does not equal the point at infinity")
		}
	}

	// 1.5 calculate e from message using the same algorithm as ecdsa
	// signature calculation.
	e := hashToInt(msg, curve)

	// Step 1.6.1:
	// We calculate the two terms sR and eG separately multiplied by the
	// inverse of r (from the signature). We then add them to calculate
	// Q = r^-1(sR-eG)
	invr := new(big.Int).ModInverse(sig.R, curve.Params().N)

	// first term.
	invrS := new(big.Int).Mul(invr, sig.S)
	invrS.Mod(invrS, curve.Params().N)
	sRx, sRy := curve.ScalarMult(Rx, Ry, invrS.Bytes())

	// second term.
	e.Neg(e)
	e.Mod(e, curve.Params().N)
	e.Mul(e, invr)
	e.Mod(e, curve.Params().N)
	minuseGx, minuseGy := curve.ScalarBaseMult(e.Bytes())

	// TODO: this would be faster if we did a mult and add in one
	// step to prevent the jacobian conversion back and forth.
	Qx, Qy := curve.Add(sRx, sRy, minuseGx, minuseGy)

	return &PublicKey{
		Curve: curve,
		X:     Qx,
		Y:     Qy,
	}, nil
}

// SignCompact produces a compact ECDSA signature of the data in hash with the given
// private key on the given koblitz curve. The isCompressed  parameter should
// be used to detail if the given signature should reference a compressed
// public key or not. If successful the bytes of the compact signature will be
// returned in the format:
// <(byte of 27+public key solution)+4 if compressed >< padded bytes for signature R><padded bytes for signature S>
// where the R and S parameters are padde up to the bitlengh of the curve.
func SignCompact(curve *KoblitzCurve, key *PrivateKey,
	hash []byte, isCompressedKey bool) ([]byte, error) {
	sig, err := key.SignECDSA(hash)
	if err != nil {
		return nil, err
	}

	// bitcoind checks the bit length of R and S here. The ecdsa signature
	// algorithm returns R and S mod N therefore they will be the bitsize of
	// the curve, and thus correctly sized.
	for i := 0; i < (curve.H+1)*2; i++ {
		pk, err := recoverKeyFromSignature(curve, sig, hash, i, true)
		if err == nil && pk.X.Cmp(key.X) == 0 && pk.Y.Cmp(key.Y) == 0 {
			result := make([]byte, 1, 2*curve.byteSize+1)
			result[0] = 27 + byte(i)
			if isCompressedKey {
				result[0] += 4
			}
			// Not sure this needs rounding but safer to do so.
			curvelen := (curve.BitSize + 7) / 8

			// Pad R and S to curvelen if needed.
			bytelen := (sig.R.BitLen() + 7) / 8
			if bytelen < curvelen {
				result = append(result,
					make([]byte, curvelen-bytelen)...)
			}
			result = append(result, sig.R.Bytes()...)

			bytelen = (sig.S.BitLen() + 7) / 8
			if bytelen < curvelen {
				result = append(result,
					make([]byte, curvelen-bytelen)...)
			}
			result = append(result, sig.S.Bytes()...)

			return result, nil
		}
	}

	return nil, errors.New("no valid solution for pubkey found")
}

// RecoverCompact verifies the compact ECDSA signature "signature" of "hash" for the
// Koblitz curve in "curve". If the signature matches then the recovered public
// key will be returned as well as a boolen if the original key was compressed
// or not, else an error will be returned.
func RecoverCompact(curve *KoblitzCurve, signature,
	hash []byte) (*PublicKey, bool, error) {
	bitlen := (curve.BitSize + 7) / 8
	if len(signature) != 1+bitlen*2 {
		return nil, false, errors.New("invalid compact signature size")
	}

	iteration := int((signature[0] - 27) & ^byte(4))

	// format is <header byte><bitlen R><bitlen S>
	sig := &Signature{
		R: new(big.Int).SetBytes(signature[1 : bitlen+1]),
		S: new(big.Int).SetBytes(signature[bitlen+1:]),
	}
	// The iteration used here was encoded
	key, err := recoverKeyFromSignature(curve, sig, hash, iteration, false)
	if err != nil {
		return nil, false, err
	}

	return key, ((signature[0] - 27) & 4) == 4, nil
}

// signRFC6979 generates a deterministic ECDSA signature according to RFC 6979 and BIP 62.
func signRFC6979(privateKey *PrivateKey, hash []byte) (*Signature, error) {

	privkey := privateKey.ToECDSA()
	N := S256().N
	halfOrder := S256().halfOrder
	k := nonceRFC6979(privkey.D, hash, nil)
	inv := new(big.Int).ModInverse(k, N)
	r, _ := privkey.Curve.ScalarBaseMult(k.Bytes())
	r.Mod(r, N)

	if r.Sign() == 0 {
		return nil, errors.New("calculated R is zero")
	}

	e := hashToInt(hash, privkey.Curve)
	s := new(big.Int).Mul(privkey.D, r)
	s.Add(s, e)
	s.Mul(s, inv)
	s.Mod(s, N)

	if s.Cmp(halfOrder) == 1 {
		s.Sub(N, s)
	}
	if s.Sign() == 0 {
		return nil, errors.New("calculated S is zero")
	}
	return &Signature{R: r, S: s, sigType: SignatureTypeECDSA}, nil
}

// signSchnorr signs the hash using the schnorr signature algorithm.
func signSchnorr(privateKey *PrivateKey, hash []byte) (*Signature, error) {
	// The rfc6979 nonce derivation function accepts additional entropy.
	// We are using the same entropy that is used by bitcoin-abc so our test
	// vectors will be compatible. This byte string is chosen to avoid collisions
	// with ECDSA which would render the signature insecure.
	//
	// See https://github.com/bitcoincashorg/bitcoincash.org/blob/master/spec/2019-05-15-schnorr.md#recommended-practices-for-secure-signature-generation
	additionalData := []byte{'S', 'c', 'h', 'n', 'o', 'r', 'r', '+', 'S', 'H', 'A', '2', '5', '6', ' ', ' '}
	k := nonceRFC6979(privateKey.D, hash, additionalData)
	// Compute point R = k * G
	rx, ry := privateKey.Curve.ScalarBaseMult(k.Bytes())

	//  Negate nonce if R.y is not a quadratic residue.
	if big.Jacobi(ry, privateKey.Params().P) != 1 {
		k = k.Neg(k)
	}

	// Compute scalar e = Hash(R.x || compressed(P) || m) mod N
	eBytes := sha256.Sum256(append(append(padIntBytes(rx), privateKey.PubKey().SerializeCompressed()...), hash...))
	e := new(big.Int).SetBytes(eBytes[:])
	e.Mod(e, privateKey.Params().N)

	// Compute scalar s = (k + e * x) mod N
	x := new(big.Int).SetBytes(privateKey.Serialize())
	s := e.Mul(e, x)
	s.Add(s, k)
	s.Mod(s, privateKey.Params().N)
	return &Signature{
		R:       rx,
		S:       s,
		sigType: SignatureTypeSchnorr,
	}, nil
}

// nonceRFC6979 generates an ECDSA nonce (`k`) deterministically according to RFC 6979.
// It takes a 32-byte hash as an input and returns 32-byte nonce to be used in ECDSA algorithm.
func nonceRFC6979(privkey *big.Int, hash []byte, additionalData []byte) *big.Int {

	curve := S256()
	q := curve.Params().N
	x := privkey
	alg := sha256.New

	qlen := q.BitLen()
	holen := alg().Size()
	rolen := (qlen + 7) >> 3
	bx := append(int2octets(x, rolen), bits2octets(hash, curve, rolen)...)

	// Step B
	v := bytes.Repeat(oneInitializer, holen)

	// Step C (Go zeroes the all allocated memory)
	k := make([]byte, holen)

	// Step D
	if additionalData != nil {
		k = mac(alg, k, append(append(append(v, 0x00), bx...), additionalData...))
	} else {
		k = mac(alg, k, append(append(v, 0x00), bx...))
	}

	// Step E
	v = mac(alg, k, v)

	// Step F
	if additionalData != nil {
		k = mac(alg, k, append(append(append(v, 0x01), bx...), additionalData...))
	} else {
		k = mac(alg, k, append(append(v, 0x01), bx...))
	}

	// Step G
	v = mac(alg, k, v)

	// Step H
	for {
		// Step H1
		var t []byte

		// Step H2
		for len(t)*8 < qlen {
			v = mac(alg, k, v)
			t = append(t, v...)
		}

		// Step H3
		secret := hashToInt(t, curve)
		if secret.Cmp(one) >= 0 && secret.Cmp(q) < 0 {
			return secret
		}
		k = mac(alg, k, append(v, 0x00))
		v = mac(alg, k, v)
	}
}

// mac returns an HMAC of the given key and message.
func mac(alg func() hash.Hash, k, m []byte) []byte {
	h := hmac.New(alg, k)
	h.Write(m)
	return h.Sum(nil)
}

// https://tools.ietf.org/html/rfc6979#section-2.3.3
func int2octets(v *big.Int, rolen int) []byte {
	out := v.Bytes()

	// left pad with zeros if it's too short
	if len(out) < rolen {
		out2 := make([]byte, rolen)
		copy(out2[rolen-len(out):], out)
		return out2
	}

	// drop most significant bytes if it's too long
	if len(out) > rolen {
		out2 := make([]byte, rolen)
		copy(out2, out[len(out)-rolen:])
		return out2
	}

	return out
}

// https://tools.ietf.org/html/rfc6979#section-2.3.4
func bits2octets(in []byte, curve elliptic.Curve, rolen int) []byte {
	z1 := hashToInt(in, curve)
	z2 := new(big.Int).Sub(z1, curve.Params().N)
	if z2.Sign() < 0 {
		return int2octets(z1, rolen)
	}
	return int2octets(z2, rolen)
}
