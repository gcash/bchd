// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chainhash

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"sync"
)

// hasherBufSize is the size of the DoubleHasher write buffer.  Most
// transactions fit in it and are then hashed with a single Sum256 call.
const hasherBufSize = 2048

// DoubleHasher computes hash(hash(b)) of the bytes written to it.  Writes are
// gathered in a buffer, so serializing field by field is cheap, and no full
// serialization buffer is needed.  Get one with NewDoubleHasher, and give it
// back with Finish, which returns the hash, or with Release.  Sum returns the
// hash so far and can be called any number of times, as with a hash.Hash, so
// only those two end its use.
type DoubleHasher struct {
	h        hash.Hash // only used once a full buffer is flushed
	n        int       // bytes in buf
	len      int       // bytes written
	released bool      // given back to the pool
	buf      [hasherBufSize]byte
}

var hasherPool = sync.Pool{New: func() any { return new(DoubleHasher) }}

// NewDoubleHasher returns an empty DoubleHasher from a pool.
func NewDoubleHasher() *DoubleHasher {
	d := hasherPool.Get().(*DoubleHasher)
	d.n = 0
	d.len = 0
	d.released = false
	if d.h != nil {
		d.h.Reset()
	}
	return d
}

// flush feeds the buffer to sha256.
func (d *DoubleHasher) flush() {
	if d.h == nil {
		d.h = sha256.New()
	}
	d.h.Write(d.buf[:d.n])
	d.n = 0
}

// Write adds p to the hash.  It never returns an error.
func (d *DoubleHasher) Write(p []byte) (int, error) {
	n := len(p)
	d.len += n
	for len(p) > 0 {
		if d.n == hasherBufSize {
			d.flush()
		}
		c := copy(d.buf[d.n:], p)
		d.n += c
		p = p[c:]
	}
	return n, nil
}

// WriteByte adds c to the hash.  It never returns an error.
func (d *DoubleHasher) WriteByte(c byte) error {
	if d.n == hasherBufSize {
		d.flush()
	}
	d.buf[d.n] = c
	d.n++
	d.len++
	return nil
}

// WriteUint32 adds v to the hash in little-endian order.
func (d *DoubleHasher) WriteUint32(v uint32) {
	if d.n > hasherBufSize-4 {
		d.flush()
	}
	binary.LittleEndian.PutUint32(d.buf[d.n:], v)
	d.n += 4
	d.len += 4
}

// WriteUint64 adds v to the hash in little-endian order.
func (d *DoubleHasher) WriteUint64(v uint64) {
	if d.n > hasherBufSize-8 {
		d.flush()
	}
	binary.LittleEndian.PutUint64(d.buf[d.n:], v)
	d.n += 8
	d.len += 8
}

// Len returns the number of bytes written.
func (d *DoubleHasher) Len() int {
	return d.len
}

// Sum returns the double sha256 of the bytes written so far.  d stays usable,
// and more can be written.
func (d *DoubleHasher) Sum() Hash {
	var first [HashSize]byte
	if d.len == d.n {
		first = sha256.Sum256(d.buf[:d.n])
	} else {
		d.flush()
		copy(first[:], d.h.Sum(d.buf[:0]))
	}
	return Hash(sha256.Sum256(first[:]))
}

// Finish returns the double sha256 of the bytes written and gives d back to the
// pool.  d must not be used afterwards.
func (d *DoubleHasher) Finish() Hash {
	sum := d.Sum()
	d.Release()
	return sum
}

// Release gives d back to the pool without a hash.  d must not be used
// afterwards.  A second Release panics, as it would put d in the pool twice, and
// two callers would then share it and get each other's hashes.
func (d *DoubleHasher) Release() {
	if d.released {
		panic("chainhash: DoubleHasher released twice")
	}
	d.released = true
	hasherPool.Put(d)
}
