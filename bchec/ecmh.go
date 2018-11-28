package bchec

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"math/big"
	"sync"
)

// Multiset tracks the state of a multiset as used to calculate the ECMH
// (elliptic curve multiset hash) hash of an unordered set. The state is
// a point on the curve. New elements are hashed onto a point on the curve
// and then added to the current state. Hence elements can be added in any
// order and we can also remove elements to return to a prior hash.
type Multiset struct {
	curve *KoblitzCurve
	x     *big.Int
	y     *big.Int
	mtx   sync.RWMutex
}

// NewMultiset returns an empty multiset. The hash of an empty set
// is the 32 byte value of zero.
func NewMultiset(curve *KoblitzCurve) *Multiset {
	return &Multiset{curve: curve, mtx: sync.RWMutex{}}
}

// NewMultisetFromPoint initializes a new multiset with the given x, y
// coordinate.
func NewMultisetFromPoint(curve *KoblitzCurve, x, y *big.Int) *Multiset {
	var copyX, copyY big.Int
	if x != nil && y != nil {
		copyX, copyY = *x, *y
	}
	return &Multiset{curve: curve, x: &copyX, y: &copyY, mtx: sync.RWMutex{}}
}

// Add hashes the data onto the curve and updates the state
// of the multiset.
func (ms *Multiset) Add(data []byte) {
	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	x, y := hashToPoint(ms.curve, data)
	if ms.x == nil || ms.y == nil {
		ms.x, ms.y = x, y
	} else {
		ms.x, ms.y = ms.curve.Add(ms.x, ms.y, x, y)
	}
}

// Remove hashes the data onto the curve and subtracts the value
// from the state. This function will execute regardless of whether
// or not the passed data was previously added to the set. Hence,
// adding a bunch of elements and then removing them all will not
// result in a zero hash.
func (ms *Multiset) Remove(data []byte) {
	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	x, y := hashToPoint(ms.curve, data)
	fx1, fy1 := ms.curve.bigAffineToField(x, y)
	fy1 = fy1.Negate(1)
	fx2, fy2 := ms.curve.bigAffineToField(ms.x, ms.y)
	fx3, fy3, fz3 := new(fieldVal), new(fieldVal), new(fieldVal)
	fOne := new(fieldVal).SetInt(1)
	ms.curve.addJacobian(fx1, fy1, fOne, fx2, fy2, fOne, fx3, fy3, fz3)
	ms.x, ms.y = ms.curve.fieldJacobianToBigAffine(fx3, fy3, fz3)
}

// Hash serializes and returns the hash of the multiset. The hash of an empty
// set is the 32 byte value of zero. The hash of a non-empty multiset is the
// sha256 hash of the 32 byte x value concatenated with the 32 byte y value.
func (ms *Multiset) Hash() chainhash.Hash {
	ms.mtx.RLock()
	defer ms.mtx.RUnlock()

	if ms.x == nil || ms.y == nil {
		return chainhash.Hash{}
	}
	h := sha256.Sum256(append(ms.x.Bytes(), ms.y.Bytes()...))
	var reversed [32]byte
	for i, b := range h {
		reversed[len(h)-i-1] = b
	}
	return chainhash.Hash(reversed)
}

// Point returns a copy of the x and y coordinates of the current multiset state.
func (ms *Multiset) Point() (x *big.Int, y *big.Int) {
	ms.mtx.RLock()
	defer ms.mtx.RUnlock()

	if ms.x == nil || ms.y == nil {
		return
	}
	copyX, copyY := *ms.x, *ms.y
	return &copyX, &copyY
}

// hashToPoint hashes the passed data into a point on the curve. The x value
// is sha256(n, sha256(data)) where n starts at zero. If the resulting x value
// is not in the field or x^3+7 is not quadratic residue then n is incremented
// and we try again. There is a 50% chance of success for any given iteration.
func hashToPoint(curve *KoblitzCurve, data []byte) (*big.Int, *big.Int) {
	i := uint64(0)
	var x, y *big.Int
	var err error
	h := sha256.Sum256(data)
	n := make([]byte, 8)
	for {
		binary.LittleEndian.PutUint64(n, i)
		h2 := sha256.Sum256(append(n, h[:]...))

		x = new(big.Int).SetBytes(h2[:])
		y, err = decompressPoint(curve, x, false)
		if err == nil {
			break
		}
		i++
	}
	return x, y
}
