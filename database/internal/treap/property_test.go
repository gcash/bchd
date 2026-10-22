// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package treap

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"unsafe"
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

// copyModel returns a copy of the passed map.
func copyModel(model map[string][]byte) map[string][]byte {
	snap := make(map[string][]byte, len(model))
	for k, v := range model {
		snap[k] = v
	}
	return snap
}

// TestBatchMatchesMap applies random puts and deletes to an immutable treap
// in batches and ensures every version holds the entries the map held at that
// point.  The versions are taken before and after each batch and from the
// middle of some, so a batch never changes a treap it started from or
// returned.
func TestBatchMatchesMap(t *testing.T) {
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
		for range 40 {
			versions = append(versions, version{tr, copyModel(model)})
			b := tr.Batch()
			for op := r.Intn(200); op > 0; op-- {
				k := randKey(r, 300)
				if r.Intn(3) < 2 {
					v := make([]byte, r.Intn(8))
					r.Read(v)
					b.Put(k, v)
					model[string(k)] = v
				} else {
					b.Delete(k)
					delete(model, string(k))
				}
				if r.Intn(50) == 0 {
					versions = append(versions,
						version{b.Immutable(), copyModel(model)})
				}
			}
			tr = b.Immutable()
		}
		versions = append(versions, version{tr, model})
		for _, v := range versions {
			checkAgainstModel(t, v.tr.root, v.tr.Len(), v.tr.Size(),
				v.tr.Get, v.model)
		}
		checkIter(t, tr.Iterator(nil, nil), sortedKeys(model))
	}
}

// TestBatchEmpty ensures a batch can start from, and return to, an empty
// treap without changing a treap it returned.
func TestBatchEmpty(t *testing.T) {
	t.Parallel()

	b := NewImmutable().Batch()
	b.Put([]byte("a"), []byte("1"))
	one := b.Immutable()
	b.Delete([]byte("a"))
	empty := b.Immutable()
	b.Put([]byte("b"), nil)
	b.Put([]byte("a"), []byte("2"))
	two := b.Immutable()

	checkAgainstModel(t, one.root, one.Len(), one.Size(), one.Get,
		map[string][]byte{"a": []byte("1")})
	checkAgainstModel(t, empty.root, empty.Len(), empty.Size(), empty.Get,
		map[string][]byte{})
	checkAgainstModel(t, two.root, two.Len(), two.Size(), two.Get,
		map[string][]byte{"a": []byte("2"), "b": {}})
}

// TestBatchCopiesNodesOnce ensures a batch of puts allocates nothing but the
// nodes of the treap it returns that the treap it started from does not have,
// besides what a batch and the treap it returns take with no puts, so it
// copies no node twice.  Put on the treap itself copies the whole path to
// each key it inserts.
func TestBatchCopiesNodesOnce(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	keys := make([][]byte, 2000)
	for i := range keys {
		keys[i] = randKey(r, 1<<30)
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i], keys[j]) < 0
	})
	full := NewImmutable()
	for range 20000 {
		full = full.Put(randKey(r, 1<<30), nil)
	}

	for _, base := range []*Immutable{NewImmutable(), full} {
		// The priorities are random, so the nodes are counted for the
		// same run that was measured.
		var result *Immutable
		overhead := testing.AllocsPerRun(1, func() {
			result = base.Batch().Immutable()
		})
		allocs := testing.AllocsPerRun(1, func() {
			b := base.Batch()
			for _, k := range keys {
				b.Put(k, nil)
			}
			result = b.Immutable()
		})

		old := make(map[*treapNode]bool)
		var walk func(n *treapNode, fn func(*treapNode))
		walk = func(n *treapNode, fn func(*treapNode)) {
			if n != nil {
				fn(n)
				walk(n.left, fn)
				walk(n.right, fn)
			}
		}
		walk(base.root, func(n *treapNode) { old[n] = true })
		var fresh int
		walk(result.root, func(n *treapNode) {
			if !old[n] {
				fresh++
			}
		})
		if allocs != float64(fresh)+overhead {
			t.Fatalf("%d puts on %d nodes made %v allocations, want %v "+
				"for the %d new nodes and the batch", len(keys),
				base.Len(), allocs, float64(fresh)+overhead, fresh)
		}
	}
}

// TestNodeFieldsSize ensures nodeFieldsSize, which Size counts for each node,
// is the size of a node on 64-bit platforms.
func TestNodeFieldsSize(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("nodeFieldsSize assumes 64-bit pointers")
	}
	if got := unsafe.Sizeof(treapNode{}); got != nodeFieldsSize {
		t.Fatalf("a node is %d bytes, nodeFieldsSize is %d", got,
			nodeFieldsSize)
	}
}
