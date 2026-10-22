// Copyright (c) 2015-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package treap

import (
	"bytes"
	"math/rand"
	"sync/atomic"
)

// cloneTreapNode returns a shallow copy of the passed node that belongs to the
// batch with the passed generation, or to no batch when it is zero.
func cloneTreapNode(node *treapNode, gen uint64) *treapNode {
	return &treapNode{
		key:      node.key,
		value:    node.value,
		priority: node.priority,
		left:     node.left,
		right:    node.right,
		gen:      gen,
	}
}

// ownTreapNode returns the passed node when it belongs to the batch with the
// passed generation, so the batch can change it in place, and otherwise a copy
// of it that belongs to the batch.  No node belongs to generation zero, so
// for it the node is always copied.
func ownTreapNode(node *treapNode, gen uint64) *treapNode {
	if gen != 0 && node.gen == gen {
		return node
	}
	return cloneTreapNode(node, gen)
}

// Immutable represents a treap data structure which is used to hold ordered
// key/value pairs using a combination of binary search tree and heap semantics.
// It is a self-organizing and randomized data structure that doesn't require
// complex operations to maintain balance.  Search, insert, and delete
// operations are all O(log n).  In addition, it provides O(1) snapshots for
// multi-version concurrency control (MVCC).
//
// All operations which result in modifying the treap return a new version of
// the treap with only the modified nodes updated.  All unmodified nodes are
// shared with the previous version.  This is extremely useful in concurrent
// applications since the caller only has to atomically replace the treap
// pointer with the newly returned version after performing any mutations.  All
// readers can simply use their existing pointer as a snapshot since the treap
// it points to is immutable.  This effectively provides O(1) snapshot
// capability with efficient memory usage characteristics since the old nodes
// only remain allocated until there are no longer any references to them.
type Immutable struct {
	root  *treapNode
	count int

	// totalSize is the best estimate of the total size of all data in
	// the treap including the keys, values, and node sizes.
	totalSize uint64
}

// newImmutable returns a new immutable treap given the passed parameters.
func newImmutable(root *treapNode, count int, totalSize uint64) *Immutable {
	return &Immutable{root: root, count: count, totalSize: totalSize}
}

// Len returns the number of items stored in the treap.
func (t *Immutable) Len() int {
	return t.count
}

// Size returns a best estimate of the total number of bytes the treap is
// consuming including all of the fields used to represent the nodes as well as
// the size of the keys and values.  Shared values are not detected, so the
// returned size assumes each value is pointing to different memory.
func (t *Immutable) Size() uint64 {
	return t.totalSize
}

// get returns the treap node that contains the passed key.  It will return nil
// when the key does not exist.
func (t *Immutable) get(key []byte) *treapNode {
	for node := t.root; node != nil; {
		// Traverse left or right depending on the result of the
		// comparison.
		compareResult := bytes.Compare(key, node.key)
		if compareResult < 0 {
			node = node.left
			continue
		}
		if compareResult > 0 {
			node = node.right
			continue
		}

		// The key exists.
		return node
	}

	// A nil node was reached which means the key does not exist.
	return nil
}

// Has returns whether or not the passed key exists.
func (t *Immutable) Has(key []byte) bool {
	if node := t.get(key); node != nil {
		return true
	}
	return false
}

// Get returns the value for the passed key.  The function will return nil when
// the key does not exist.
func (t *Immutable) Get(key []byte) []byte {
	if node := t.get(key); node != nil {
		return node.value
	}
	return nil
}

// Put inserts the passed key/value pair.
func (t *Immutable) Put(key, value []byte) *Immutable {
	root, count, totalSize := t.put(key, value, 0)
	return newImmutable(root, count, totalSize)
}

// put inserts the passed key/value pair into a version of the treap that
// shares every node it does not change, and returns the root, count and total
// size of that version.  It changes nodes that belong to the batch with the
// passed generation in place instead of copying them.
func (t *Immutable) put(key, value []byte, gen uint64) (*treapNode, int, uint64) {
	// Use an empty byte slice for the value when none was provided.  This
	// ultimately allows key existence to be determined from the value since
	// an empty byte slice is distinguishable from nil.
	if value == nil {
		value = emptySlice
	}

	// The node is the root of the tree if there isn't already one.
	if t.root == nil {
		root := newTreapNode(key, value, rand.Int())
		root.gen = gen
		return root, 1, nodeSize(root)
	}

	// Find the binary tree insertion point and construct a replaced list of
	// parents while doing so.  This is done because this is an immutable
	// data structure so regardless of where in the treap the new key/value
	// pair ends up, all ancestors up to and including the root need to be
	// replaced.
	//
	// When the key matches an entry already in the treap, replace the node
	// with a new one that has the new value set and return.
	var parents parentStack
	var compareResult int
	for node := t.root; node != nil; {
		// Clone the node, unless it is the batch's own, and link its
		// parent to it if needed.
		nodeCopy := ownTreapNode(node, gen)
		if oldParent := parents.At(0); oldParent != nil {
			if oldParent.left == node {
				oldParent.left = nodeCopy
			} else {
				oldParent.right = nodeCopy
			}
		}
		parents.Push(nodeCopy)

		// Traverse left or right depending on the result of comparing
		// the keys.
		compareResult = bytes.Compare(key, node.key)
		if compareResult < 0 {
			node = node.left
			continue
		}
		if compareResult > 0 {
			node = node.right
			continue
		}

		// The key already exists, so update its value.  The new size
		// is worked out first, since the node may be the batch's own.
		newTotalSize := t.totalSize - uint64(len(node.value)) +
			uint64(len(value))
		nodeCopy.value = value

		// Return new immutable treap with the replaced node and
		// ancestors up to and including the root of the tree.
		newRoot := parents.At(parents.Len() - 1)
		return newRoot, t.count, newTotalSize
	}

	// Link the new node into the binary tree in the correct position.
	node := newTreapNode(key, value, rand.Int())
	node.gen = gen
	parent := parents.At(0)
	if compareResult < 0 {
		parent.left = node
	} else {
		parent.right = node
	}

	// Perform any rotations needed to maintain the min-heap and replace
	// the ancestors up to and including the tree root.
	newRoot := parents.At(parents.Len() - 1)
	for parents.Len() > 0 {
		// There is nothing left to do when the node's priority is
		// greater than or equal to its parent's priority.
		parent = parents.Pop()
		if node.priority >= parent.priority {
			break
		}

		// Perform a right rotation if the node is on the left side or
		// a left rotation if the node is on the right side.
		if parent.left == node {
			node.right, parent.left = parent, node.right
		} else {
			node.left, parent.right = parent, node.left
		}

		// Either set the new root of the tree when there is no
		// grandparent or relink the grandparent to the node based on
		// which side the old parent the node is replacing was on.
		grandparent := parents.At(0)
		if grandparent == nil {
			newRoot = node
		} else if grandparent.left == parent {
			grandparent.left = node
		} else {
			grandparent.right = node
		}
	}

	return newRoot, t.count + 1, t.totalSize + nodeSize(node)
}

// Delete removes the passed key from the treap and returns the resulting treap
// if it exists.  The original immutable treap is returned if the key does not
// exist.
func (t *Immutable) Delete(key []byte) *Immutable {
	root, count, totalSize, ok := t.delete(key, 0)
	if !ok {
		return t
	}
	return newImmutable(root, count, totalSize)
}

// delete removes the passed key from a version of the treap that shares every
// node it does not change, and returns the root, count and total size of that
// version, and whether the key existed.  It changes nodes that belong to the
// batch with the passed generation in place instead of copying them.
func (t *Immutable) delete(key []byte, gen uint64) (*treapNode, int, uint64, bool) {
	// Find the node for the key while constructing a list of parents while
	// doing so.
	var parents parentStack
	var delNode *treapNode
	for node := t.root; node != nil; {
		parents.Push(node)

		// Traverse left or right depending on the result of the
		// comparison.
		compareResult := bytes.Compare(key, node.key)
		if compareResult < 0 {
			node = node.left
			continue
		}
		if compareResult > 0 {
			node = node.right
			continue
		}

		// The key exists.
		delNode = node
		break
	}

	// There is nothing to do if the key does not exist.
	if delNode == nil {
		return t.root, t.count, t.totalSize, false
	}

	// When the only node in the tree is the root node and it is the one
	// being deleted, there is nothing else to do besides removing it.
	parent := parents.At(1)
	if parent == nil && delNode.left == nil && delNode.right == nil {
		return nil, 0, 0, true
	}

	// Construct a replaced list of parents and the node to delete itself.
	// This is done because this is an immutable data structure and
	// therefore all ancestors of the node that will be deleted, up to and
	// including the root, need to be replaced.
	var newParents parentStack
	for i := parents.Len(); i > 0; i-- {
		node := parents.At(i - 1)
		nodeCopy := ownTreapNode(node, gen)
		if oldParent := newParents.At(0); oldParent != nil {
			if oldParent.left == node {
				oldParent.left = nodeCopy
			} else {
				oldParent.right = nodeCopy
			}
		}
		newParents.Push(nodeCopy)
	}
	delNode = newParents.Pop()
	parent = newParents.At(0)

	// Perform rotations to move the node to delete to a leaf position while
	// maintaining the min-heap while replacing the modified children.
	var child *treapNode
	newRoot := newParents.At(newParents.Len() - 1)
	for delNode.left != nil || delNode.right != nil {
		// Choose the child with the lower priority.
		var isLeft bool
		if delNode.left == nil {
			child = delNode.right
		} else if delNode.right == nil {
			child = delNode.left
			isLeft = true
		} else if delNode.left.priority <= delNode.right.priority {
			child = delNode.left
			isLeft = true
		} else {
			child = delNode.right
		}

		// Rotate left or right depending on which side the child node
		// is on.  This has the effect of moving the node to delete
		// towards the bottom of the tree while maintaining the
		// min-heap.
		child = ownTreapNode(child, gen)
		if isLeft {
			child.right, delNode.left = delNode, child.right
		} else {
			child.left, delNode.right = delNode, child.left
		}

		// Either set the new root of the tree when there is no
		// grandparent or relink the grandparent to the node based on
		// which side the old parent the node is replacing was on.
		//
		// Since the node to be deleted was just moved down a level, the
		// new grandparent is now the current parent and the new parent
		// is the current child.
		if parent == nil {
			newRoot = child
		} else if parent.left == delNode {
			parent.left = child
		} else {
			parent.right = child
		}

		// The parent for the node to delete is now what was previously
		// its child.
		parent = child
	}

	// Delete the node, which is now a leaf node, by disconnecting it from
	// its parent.
	if parent.right == delNode {
		parent.right = nil
	} else {
		parent.left = nil
	}

	return newRoot, t.count - 1, t.totalSize - nodeSize(delNode), true
}

// ForEach invokes the passed function with every key/value pair in the treap
// in ascending order.
func (t *Immutable) ForEach(fn func(k, v []byte) bool) {
	// Add the root node and all children to the left of it to the list of
	// nodes to traverse and loop until they, and all of their child nodes,
	// have been traversed.
	var parents parentStack
	for node := t.root; node != nil; node = node.left {
		parents.Push(node)
	}
	for parents.Len() > 0 {
		node := parents.Pop()
		if !fn(node.key, node.value) {
			return
		}

		// Extend the nodes to traverse by all children to the left of
		// the current node's right child.
		for node := node.right; node != nil; node = node.left {
			parents.Push(node)
		}
	}
}

// NewImmutable returns a new empty immutable treap ready for use.  See the
// documentation for the Immutable structure for more details.
func NewImmutable() *Immutable {
	return &Immutable{}
}

// batchGen is the last generation given to a batch.  Each batch takes a new
// one, so a node that has its batch's generation was created by that batch
// and is in no treap other than the batch's own.
var batchGen atomic.Uint64

// Batch applies a series of puts and deletes to an immutable treap.  Put and
// Delete on an Immutable copy every node on the path to the key, so that the
// treap they were called on is left as it was.  A batch copies a node only the
// first time it changes it and then changes its own copy in place, so a large
// number of changes copies far fewer nodes.  It never changes the treap it
// started from or one Immutable returned.
//
// A Batch is not safe for concurrent use.
type Batch struct {
	t   Immutable
	gen uint64
}

// Batch returns a batch that applies changes to the treap.
func (t *Immutable) Batch() *Batch {
	return &Batch{t: *t, gen: batchGen.Add(1)}
}

// Put inserts the passed key/value pair.
func (b *Batch) Put(key, value []byte) {
	b.t.root, b.t.count, b.t.totalSize = b.t.put(key, value, b.gen)
}

// Delete removes the passed key if it exists.
func (b *Batch) Delete(key []byte) {
	b.t.root, b.t.count, b.t.totalSize, _ = b.t.delete(key, b.gen)
}

// Immutable returns the treap with the changes applied so far.  The batch
// takes a new generation, so later changes copy the nodes of the returned
// treap instead of changing them.
func (b *Batch) Immutable() *Immutable {
	b.gen = batchGen.Add(1)
	return newImmutable(b.t.root, b.t.count, b.t.totalSize)
}
