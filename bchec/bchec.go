// Copyright 2010 The Go Authors. All rights reserved.
// Copyright 2011 ThePiachu. All rights reserved.
// Copyright 2013-2014 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchec

// References:
//   [SECG]: Recommended Elliptic Curve Domain Parameters
//     http://www.secg.org/sec2-v2.pdf
//
//   [GECC]: Guide to Elliptic Curve Cryptography (Hankerson, Menezes, Vanstone)

// The field and group arithmetic is provided by the secp256k1 package from
// github.com/decred/dcrd/dcrec/secp256k1/v4, which operates internally on
// Jacobian coordinates.

import (
	"crypto/elliptic"
	"math/big"
	"slices"
	"sync"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// KoblitzCurve supports a koblitz curve implementation that fits the ECC Curve
// interface from crypto/elliptic.
type KoblitzCurve struct {
	*elliptic.CurveParams
	q         *big.Int
	H         int      // cofactor of the curve.
	halfOrder *big.Int // half the order N

	// byteSize is simply the bit size / 8 and is provided for convenience
	// since it is calculated repeatedly.
	byteSize int
}

// Params returns the parameters for the curve.
func (curve *KoblitzCurve) Params() *elliptic.CurveParams {
	return curve.CurveParams
}

// bigAffineToJacobian converts the affine point (x, y) to a Jacobian point
// with a z value of one.
func bigAffineToJacobian(x, y *big.Int, result *secp.JacobianPoint) {
	result.X.SetByteSlice(x.Bytes())
	result.Y.SetByteSlice(y.Bytes())
	result.X.Normalize()
	result.Y.Normalize()
	result.Z.SetInt(1)
}

// jacobianToBigAffine converts the Jacobian point to an affine point as big
// integers.  The point at infinity is returned as (0, 0).
func jacobianToBigAffine(point *secp.JacobianPoint) (*big.Int, *big.Int) {
	point.ToAffine()
	x3, y3 := new(big.Int), new(big.Int)
	x3.SetBytes(point.X.Bytes()[:])
	y3.SetBytes(point.Y.Bytes()[:])
	return x3, y3
}

// isInfinity returns whether the Jacobian point is the point at infinity.
func isInfinity(point *secp.JacobianPoint) bool {
	return (point.X.IsZero() && point.Y.IsZero()) || point.Z.IsZero()
}

// IsOnCurve returns boolean if the point (x,y) is on the curve.
// Part of the elliptic.Curve interface. This function differs from the
// crypto/elliptic algorithm since a = 0 not -3.
func (curve *KoblitzCurve) IsOnCurve(x, y *big.Int) bool {
	var fx, fy secp.FieldVal
	fx.SetByteSlice(x.Bytes())
	fy.SetByteSlice(y.Bytes())

	// Elliptic curve equation for secp256k1 is: y^2 = x^3 + 7
	y2 := new(secp.FieldVal).SquareVal(&fy).Normalize()
	result := new(secp.FieldVal).SquareVal(&fx).Mul(&fx).AddInt(7).Normalize()
	return y2.Equals(result)
}

// Add returns the sum of (x1,y1) and (x2,y2). Part of the elliptic.Curve
// interface.
func (curve *KoblitzCurve) Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	// A point at infinity is the identity according to the group law for
	// elliptic curve cryptography.  Thus, ∞ + P = P and P + ∞ = P.
	if x1.Sign() == 0 && y1.Sign() == 0 {
		return x2, y2
	}
	if x2.Sign() == 0 && y2.Sign() == 0 {
		return x1, y1
	}

	var p1, p2, result secp.JacobianPoint
	bigAffineToJacobian(x1, y1, &p1)
	bigAffineToJacobian(x2, y2, &p2)
	secp.AddNonConst(&p1, &p2, &result)
	return jacobianToBigAffine(&result)
}

// Double returns 2*(x1,y1). Part of the elliptic.Curve interface.
func (curve *KoblitzCurve) Double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	if y1.Sign() == 0 {
		return new(big.Int), new(big.Int)
	}

	var point, result secp.JacobianPoint
	bigAffineToJacobian(x1, y1, &point)
	secp.DoubleNonConst(&point, &result)
	return jacobianToBigAffine(&result)
}

// moduloReduce reduces k from more than 32 bytes to 32 bytes and under.  This
// is done by doing a simple modulo curve.N.  We can do this since G^N = 1 and
// thus any other valid point on the elliptic curve has the same order.
func (curve *KoblitzCurve) moduloReduce(k []byte) []byte {
	// Since the order of G is curve.N, we can use a much smaller number
	// by doing modulo curve.N
	if len(k) > curve.byteSize {
		// Reduce k by performing modulo curve.N.
		tmpK := new(big.Int).SetBytes(k)
		tmpK.Mod(tmpK, curve.N)
		return tmpK.Bytes()
	}

	return k
}

// NAF takes a positive integer k and returns the Non-Adjacent Form (NAF) as two
// byte slices.  The first is where 1s will be.  The second is where -1s will
// be.  NAF is convenient in that on average, only 1/3rd of its values are
// non-zero.  This is algorithm 3.30 from [GECC].
//
// Essentially, this makes it possible to minimize the number of operations
// since the resulting ints returned will be at least 50% 0s.
func NAF(k []byte) ([]byte, []byte) {
	// The essence of this algorithm is that whenever we have consecutive 1s
	// in the binary, we want to put a -1 in the lowest bit and get a bunch
	// of 0s up to the highest bit of consecutive 1s.  This is due to this
	// identity:
	// 2^n + 2^(n-1) + 2^(n-2) + ... + 2^(n-k) = 2^(n+1) - 2^(n-k)
	//
	// The algorithm thus may need to go 1 more bit than the length of the
	// bits we actually have, hence bits being 1 bit longer than was
	// necessary.  Since we need to know whether adding will cause a carry,
	// we go from right-to-left in this addition.
	var carry, curIsOne, nextIsOne bool
	// these default to zero
	retPos := make([]byte, len(k)+1)
	retNeg := make([]byte, len(k)+1)
	for i, curByte := range slices.Backward(k) {
		for j := range uint(8) {
			curIsOne = curByte&1 == 1
			if j == 7 {
				if i == 0 {
					nextIsOne = false
				} else {
					nextIsOne = k[i-1]&1 == 1
				}
			} else {
				nextIsOne = curByte&2 == 2
			}
			if carry {
				if !curIsOne {
					// We've hit a 0 after some number of
					// 1s.
					if nextIsOne {
						// Start carrying again since
						// a new sequence of 1s is
						// starting.
						retNeg[i+1] += 1 << j
					} else {
						// Stop carrying since 1s have
						// stopped.
						carry = false
						retPos[i+1] += 1 << j
					}
				}
			} else if curIsOne {
				if nextIsOne {
					// If this is the start of at least 2
					// consecutive 1s, set the current one
					// to -1 and start carrying.
					retNeg[i+1] += 1 << j
					carry = true
				} else {
					// This is a singleton, not consecutive
					// 1s.
					retPos[i+1] += 1 << j
				}
			}
			curByte >>= 1
		}
	}
	if carry {
		retPos[0] = 1
		return retPos, retNeg
	}
	return retPos[1:], retNeg[1:]
}

// scalarFromBytes sets s to the big endian integer k reduced modulo N.
func (curve *KoblitzCurve) scalarFromBytes(k []byte, s *secp.ModNScalar) {
	s.SetByteSlice(curve.moduloReduce(k))
}

// ScalarMult returns k*(Bx, By) where k is a big endian integer.
// Part of the elliptic.Curve interface.
func (curve *KoblitzCurve) ScalarMult(Bx, By *big.Int, k []byte) (*big.Int, *big.Int) {
	var result secp.JacobianPoint
	curve.scalarMultJacobian(Bx, By, k, &result)
	return jacobianToBigAffine(&result)
}

// scalarMultJacobian stores k*(Bx, By) in result, where k is a big endian
// integer.
func (curve *KoblitzCurve) scalarMultJacobian(Bx, By *big.Int, k []byte, result *secp.JacobianPoint) {
	var kModN secp.ModNScalar
	curve.scalarFromBytes(k, &kModN)
	var point secp.JacobianPoint
	bigAffineToJacobian(Bx, By, &point)
	secp.ScalarMultNonConst(&kModN, &point, result)
}

// ScalarBaseMult returns k*G where G is the base point of the group and k is a
// big endian integer.
// Part of the elliptic.Curve interface.
func (curve *KoblitzCurve) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	var kModN secp.ModNScalar
	curve.scalarFromBytes(k, &kModN)
	var result secp.JacobianPoint
	secp.ScalarBaseMultNonConst(&kModN, &result)
	return jacobianToBigAffine(&result)
}

// QPlus1Div4 returns the Q+1/4 constant for the curve for use in calculating
// square roots via exponention.
func (curve *KoblitzCurve) QPlus1Div4() *big.Int {
	return curve.q
}

var initonce sync.Once
var secp256k1 KoblitzCurve

func initAll() {
	initS256()
}

// fromHex converts the passed hex string into a big integer pointer and will
// panic is there is an error.  This is only provided for the hard-coded
// constants so errors in the source code can bet detected. It will only (and
// must only) be called for initialization purposes.
func fromHex(s string) *big.Int {
	r, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("invalid hex in source file: " + s)
	}
	return r
}

func initS256() {
	// Curve parameters taken from [SECG] section 2.4.1.
	secp256k1.CurveParams = new(elliptic.CurveParams)
	secp256k1.P = fromHex("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F")
	secp256k1.N = fromHex("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141")
	secp256k1.B = fromHex("0000000000000000000000000000000000000000000000000000000000000007")
	secp256k1.Gx = fromHex("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
	secp256k1.Gy = fromHex("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8")
	secp256k1.BitSize = 256
	secp256k1.q = new(big.Int).Div(new(big.Int).Add(secp256k1.P,
		big.NewInt(1)), big.NewInt(4))
	secp256k1.H = 1
	secp256k1.halfOrder = new(big.Int).Rsh(secp256k1.N, 1)

	// Provided for convenience since this gets computed repeatedly.
	secp256k1.byteSize = secp256k1.BitSize / 8
}

// S256 returns a Curve which implements secp256k1.
func S256() *KoblitzCurve {
	initonce.Do(initAll)
	return &secp256k1
}
