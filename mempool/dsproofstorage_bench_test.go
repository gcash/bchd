// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// benchOrphan returns a proof for an outpoint of its own and its id, both
// derived from n.  The storage only reads the outpoint of a proof.
func benchOrphan(n uint64) (*wire.MsgDSProof, chainhash.Hash) {
	var op wire.OutPoint
	binary.LittleEndian.PutUint64(op.Hash[:], n)
	var hash chainhash.Hash
	binary.LittleEndian.PutUint64(hash[:], n)
	hash[31] = 1
	return &wire.MsgDSProof{PrevOut: op}, hash
}

// fillBenchOrphans adds orphans to s until it holds num, each from the first
// peer at or after 100 with room left, one second apart starting at the
// clock's time.  It returns the next proof number.
func fillBenchOrphans(b *testing.B, s *dsProofStorage, now *time.Time, num int, next uint64) uint64 {
	b.Helper()
	peer := Tag(100)
	for s.numOrphans < num {
		for len(s.peerOrphans[peer]) >= s.maxOrphans/dsProofOrphanPeerShare {
			peer++
		}
		proof, hash := benchOrphan(next)
		next++
		*now = now.Add(time.Second)
		if !s.addOrphan(proof, hash, peer, true) {
			b.Fatalf("orphan %d refused", next)
		}
	}
	return next
}

// newBenchStorage returns a storage holding the most orphans it keeps, a
// quarter more than the maximum, and its clock.
func newBenchStorage(b *testing.B) (*dsProofStorage, *time.Time, uint64) {
	b.Helper()
	s := newDSProofStorage()
	now := time.Unix(2000000, 0)
	s.now = func() time.Time { return now }
	next := fillBenchOrphans(b, s, &now, s.maxOrphans+s.maxOrphans/4, 0)
	return s, &now, next
}

// BenchmarkDSProofRemovePeerOrphans measures a peer adding an orphan to a full
// orphan pool and disconnecting, which removes it.
func BenchmarkDSProofRemovePeerOrphans(b *testing.B) {
	s, now, next := newBenchStorage(b)
	proof, hash := benchOrphan(next)
	s.remove(s.sortedByOrphanTime()[0].hash)
	b.ResetTimer()
	for range b.N {
		*now = now.Add(time.Millisecond)
		s.addOrphan(proof, hash, 1, true)
		if s.removePeerOrphans(1) != 1 {
			b.Fatal("orphan not removed")
		}
	}
}

// BenchmarkDSProofExpireNone measures the minutely expiry of a full orphan
// pool none of whose orphans expire.
func BenchmarkDSProofExpireNone(b *testing.B) {
	s, _, _ := newBenchStorage(b)
	s.orphanTTL = 1000 * time.Hour
	b.ResetTimer()
	for range b.N {
		if len(s.expireOrphans()) != 0 {
			b.Fatal("orphans expired")
		}
	}
}

// BenchmarkDSProofExpireMinute measures the minutely expiry of a full orphan
// pool that takes in an orphan a second, so 60 expire each time.
func BenchmarkDSProofExpireMinute(b *testing.B) {
	s, now, next := newBenchStorage(b)
	for range 60 {
		s.remove(s.sortedByOrphanTime()[0].hash)
	}
	s.orphanTTL = time.Duration(s.numOrphans) * time.Second
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		for range 60 {
			proof, hash := benchOrphan(next)
			next++
			*now = now.Add(time.Second)
			s.addOrphan(proof, hash, Tag(next%64), true)
		}
		b.StartTimer()
		if n := len(s.expireOrphans()); n != 60 {
			b.Fatalf("%d orphans expired", n)
		}
	}
}

// BenchmarkDSProofLimitOrphans measures the orphan limit removing the oldest
// orphans of a full orphan pool, down to the maximum, when one more arrives.
func BenchmarkDSProofLimitOrphans(b *testing.B) {
	s, now, next := newBenchStorage(b)
	full := s.numOrphans
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		next = fillBenchOrphans(b, s, now, full, next)
		proof, hash := benchOrphan(next)
		next++
		*now = now.Add(time.Second)
		b.StartTimer()
		s.addOrphan(proof, hash, 1, true)
		if s.numOrphans != s.maxOrphans {
			b.Fatalf("%d orphans", s.numOrphans)
		}
		b.StopTimer()
		s.removePeerOrphans(1)
		b.StartTimer()
	}
}
