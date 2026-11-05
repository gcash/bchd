// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chainhash

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
)

// TestDoubleHasher checks DoubleHasher against DoubleHashH for inputs around
// the buffer size, written in pieces of random size and type.
func TestDoubleHasher(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	sizes := []int{0, 1, 3, 4, 8, 63, 64, 65, hasherBufSize - 9,
		hasherBufSize - 1, hasherBufSize, hasherBufSize + 1,
		2*hasherBufSize - 3, 2 * hasherBufSize, 100000}
	for _, size := range sizes {
		for range 20 {
			data := make([]byte, size)
			rng.Read(data)
			want := DoubleHashH(data)

			d := NewDoubleHasher()
			rest := data
			for len(rest) > 0 {
				switch k := rng.Intn(5); {
				case k == 0:
					_ = d.WriteByte(rest[0])
					rest = rest[1:]
				case k == 1 && len(rest) >= 4:
					d.WriteUint32(binary.LittleEndian.Uint32(rest))
					rest = rest[4:]
				case k == 2 && len(rest) >= 8:
					d.WriteUint64(binary.LittleEndian.Uint64(rest))
					rest = rest[8:]
				default:
					n := rng.Intn(min(len(rest), 3*hasherBufSize)) + 1
					if _, err := d.Write(rest[:n]); err != nil {
						t.Fatal(err)
					}
					rest = rest[n:]
				}
			}
			if d.Len() != size {
				t.Fatalf("size %d: Len %d", size, d.Len())
			}
			if got := d.Finish(); got != want {
				t.Fatalf("size %d: got %v, want %v", size, got, want)
			}
		}
	}
}

// TestDoubleHasherAllocs checks that a pooled DoubleHasher does not
// allocate, for small and large inputs.
func TestDoubleHasherAllocs(t *testing.T) {
	data := make([]byte, 3*hasherBufSize)
	for _, size := range []int{100, len(data)} {
		allocs := testing.AllocsPerRun(100, func() {
			d := NewDoubleHasher()
			_, _ = d.Write(data[:size])
			d.WriteUint32(1)
			_ = d.Finish()
		})
		if allocs != 0 {
			t.Errorf("size %d: %v allocs", size, allocs)
		}
	}
}

// BenchmarkDoubleHasher measures hashing with DoubleHasher against
// DoubleHashH on a buffer.
func BenchmarkDoubleHasher(b *testing.B) {
	data := make([]byte, 4000)
	for _, size := range []int{200, 4000} {
		b.Run(fmt.Sprintf("hasher/%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				d := NewDoubleHasher()
				_, _ = d.Write(data[:size])
				_ = d.Finish()
			}
		})
		b.Run(fmt.Sprintf("DoubleHashH/%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				_ = DoubleHashH(data[:size])
			}
		})
	}
}

// TestDoubleHasherSumRepeats ensures Sum can be called any number of times, and
// that writing can go on after it, whether the hash fits the buffer or not.
func TestDoubleHasherSumRepeats(t *testing.T) {
	for _, size := range []int{0, 10, hasherBufSize, hasherBufSize + 1, 3 * hasherBufSize} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i)
		}
		d := NewDoubleHasher()
		_, _ = d.Write(data)
		first, second := d.Sum(), d.Sum()
		if want := DoubleHashH(data); first != want || second != want {
			t.Fatalf("%d bytes: got %v and %v, want %v", size, first, second, want)
		}
		_, _ = d.Write([]byte{1, 2, 3})
		if got, want := d.Finish(), DoubleHashH(append(data, 1, 2, 3)); got != want {
			t.Fatalf("%d bytes and more: got %v, want %v", size, got, want)
		}
	}
}

// TestDoubleHasherReleaseTwice ensures giving a hasher back twice is refused,
// as two callers would then share it.
func TestDoubleHasherReleaseTwice(t *testing.T) {
	d := NewDoubleHasher()
	d.Release()
	defer func() {
		if recover() == nil {
			t.Fatal("a second release went through")
		}
	}()
	d.Release()
}
