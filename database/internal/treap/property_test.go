// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package treap

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
)

// treapStats holds what checkTreap found while walking a treap.
type treapStats struct {
	count    int
	size     uint64
	depth    int
	heapErrs int
}

// checkTreap walks the treap rooted at root, fails the test if the keys are
// out of order and returns the node count, total size, maximum depth and the
// number of nodes with a lower priority than their parent.  Put keeps the
// priorities a min-heap, so a correct treap has no such nodes.
func checkTreap(t *testing.T, root *treapNode) treapStats {
	t.Helper()

	var stats treapStats
	var walk func(n *treapNode, lo, hi []byte, depth int)
	walk = func(n *treapNode, lo, hi []byte, depth int) {
		if n == nil {
			return
		}
		if lo != nil && bytes.Compare(n.key, lo) <= 0 {
			t.Fatalf("key %x not above lower bound %x", n.key, lo)
		}
		if hi != nil && bytes.Compare(n.key, hi) >= 0 {
			t.Fatalf("key %x not below upper bound %x", n.key, hi)
		}
		stats.count++
		stats.size += nodeSize(n)
		stats.depth = max(stats.depth, depth)
		for _, child := range []*treapNode{n.left, n.right} {
			if child != nil && child.priority < n.priority {
				stats.heapErrs++
			}
		}
		walk(n.left, lo, n.key, depth+1)
		walk(n.right, n.key, hi, depth+1)
	}
	walk(root, nil, nil, 1)
	return stats
}

// randKey returns a random 4-byte big-endian key below space.
func randKey(r *rand.Rand, space int) []byte {
	v := r.Intn(space)
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// sortedKeys returns the keys of m in ascending order.
func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checkIter fails the test unless iter yields exactly the keys of want in
// order going forwards and in reverse going backwards.
func checkIter(t *testing.T, iter *Iterator, want []string) {
	t.Helper()

	var got []string
	for ok := iter.First(); ok; ok = iter.Next() {
		got = append(got, string(iter.Key()))
	}
	var rgot []string
	for ok := iter.Last(); ok; ok = iter.Prev() {
		rgot = append(rgot, string(iter.Key()))
	}
	if len(got) != len(want) || len(rgot) != len(want) {
		t.Fatalf("iterator returned %d forward and %d reverse keys, "+
			"want %d", len(got), len(rgot), len(want))
	}
	for i := range want {
		if got[i] != want[i] || rgot[len(want)-1-i] != want[i] {
			t.Fatalf("iterator key %d mismatch", i)
		}
	}
}

// checkAgainstModel fails the test unless the treap rooted at root holds
// exactly the entries of model and is still a valid treap.
func checkAgainstModel(t *testing.T, root *treapNode, length int, size uint64,
	get func([]byte) []byte, model map[string][]byte) {

	t.Helper()

	stats := checkTreap(t, root)
	if stats.count != len(model) || length != len(model) {
		t.Fatalf("treap has %d nodes and Len %d, want %d", stats.count,
			length, len(model))
	}
	if stats.size != size {
		t.Fatalf("Size is %d, nodes add up to %d", size, stats.size)
	}
	if stats.heapErrs != 0 {
		t.Fatalf("%d nodes break the priority heap", stats.heapErrs)
	}
	for k, v := range model {
		if got := get([]byte(k)); !bytes.Equal(got, v) {
			t.Fatalf("Get(%x) = %x, want %x", k, got, v)
		}
	}
}

// TestMutableMatchesMap applies random puts and deletes to a mutable treap
// and a map and ensures the treap holds the same entries, stays a valid treap
// and iterates the same keys.
func TestMutableMatchesMap(t *testing.T) {
	t.Parallel()

	for seed := int64(1); seed <= 20; seed++ {
		r := rand.New(rand.NewSource(seed))
		tr := NewMutable()
		model := make(map[string][]byte)
		for op := 0; op < 3000; op++ {
			k := randKey(r, 500)
			if r.Intn(3) < 2 {
				v := make([]byte, r.Intn(8))
				r.Read(v)
				tr.Put(k, v)
				model[string(k)] = v
			} else {
				tr.Delete(k)
				delete(model, string(k))
			}
		}
		checkAgainstModel(t, tr.root, tr.Len(), tr.Size(), tr.Get, model)

		keys := sortedKeys(model)
		checkIter(t, tr.Iterator(nil, nil), keys)
		for i := 0; i < 50; i++ {
			start, limit := randKey(r, 520), randKey(r, 520)
			if bytes.Compare(start, limit) > 0 {
				start, limit = limit, start
			}
			var want []string
			for _, k := range keys {
				if k >= string(start) && k < string(limit) {
					want = append(want, k)
				}
			}
			checkIter(t, tr.Iterator(start, limit), want)
		}
	}
}

// TestImmutableMatchesMap applies random puts and deletes to an immutable
// treap and a map and ensures every version holds the entries the map held
// at that point, so older versions are never modified.
func TestImmutableMatchesMap(t *testing.T) {
	t.Parallel()

	type version struct {
		tr    *Immutable
		model map[string][]byte
	}
	for seed := int64(1); seed <= 10; seed++ {
		r := rand.New(rand.NewSource(seed))
		tr := NewImmutable()
		model := make(map[string][]byte)
		var versions []version
		for op := 0; op < 2000; op++ {
			k := randKey(r, 300)
			if r.Intn(3) < 2 {
				v := make([]byte, r.Intn(8))
				r.Read(v)
				tr = tr.Put(k, v)
				model[string(k)] = v
			} else {
				tr = tr.Delete(k)
				delete(model, string(k))
			}
			if op%100 == 0 {
				snap := make(map[string][]byte, len(model))
				for k, v := range model {
					snap[k] = v
				}
				versions = append(versions, version{tr, snap})
			}
		}
		versions = append(versions, version{tr, model})
		for _, v := range versions {
			checkAgainstModel(t, v.tr.root, v.tr.Len(), v.tr.Size(),
				v.tr.Get, v.model)
		}
		checkIter(t, tr.Iterator(nil, nil), sortedKeys(model))
	}
}

// TestDeleteKeepsHeap deletes inner nodes from both treaps under churn and
// ensures the priorities stay a min-heap, which keeps the expected depth
// logarithmic.
func TestDeleteKeepsHeap(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(7))
	mt := NewMutable()
	it := NewImmutable()
	for i := 0; i < 2000; i++ {
		k := randKey(r, 1<<30)
		mt.Put(k, nil)
		it = it.Put(k, nil)
	}
	for i := 0; i < 2000; i++ {
		k := randKey(r, 1<<30)
		mt.Put(k, nil)
		it = it.Put(k, nil)

		// Walk a short random path from the root so that inner nodes
		// with two children are deleted too.
		n := mt.root
		for r.Intn(4) != 0 {
			if r.Intn(2) == 0 && n.left != nil {
				n = n.left
			} else if n.right != nil {
				n = n.right
			} else {
				break
			}
		}
		mt.Delete(n.key)
		it = it.Delete(n.key)
	}
	ms := checkTreap(t, mt.root)
	is := checkTreap(t, it.root)
	if ms.heapErrs != 0 || is.heapErrs != 0 {
		t.Fatalf("priority heap broken after deletes: mutable %d, "+
			"immutable %d nodes (depth %d and %d)", ms.heapErrs,
			is.heapErrs, ms.depth, is.depth)
	}
}
