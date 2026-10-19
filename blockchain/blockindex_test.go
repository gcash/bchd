// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/rand"
	"testing"

	"github.com/gcash/bchd/wire"
)

// linearAncestor returns the ancestor of node at the given height by
// following the parent links one at a time.
func linearAncestor(node *blockNode, height int32) *blockNode {
	if height < 0 || height > node.height {
		return nil
	}
	n := node
	for n.height != height {
		n = n.parent
	}
	return n
}

// randomTree returns numNodes block nodes forming a tree with many forks.
// Each node extends the latest node or, now and then, a random earlier one.
func randomTree(rng *rand.Rand, numNodes int) []*blockNode {
	nodes := make([]*blockNode, 0, numNodes)
	for i := range numNodes {
		var parent *blockNode
		switch {
		case i == 0:
		case rng.Intn(10) == 0:
			parent = nodes[rng.Intn(len(nodes))]
		default:
			parent = nodes[len(nodes)-1]
		}
		header := wire.BlockHeader{Nonce: rng.Uint32()}
		if parent != nil {
			header.PrevBlock = parent.hash
		}
		nodes = append(nodes, newBlockNode(&header, parent))
	}
	return nodes
}

// TestAncestorMatchesParentWalk ensures Ancestor and RelativeAncestor return
// the same nodes as a walk over the parent links on random block trees, and
// that every skip points at the ancestor at skipHeight.
func TestAncestorMatchesParentWalk(t *testing.T) {
	t.Parallel()

	for seed := int64(1); seed <= 5; seed++ {
		rng := rand.New(rand.NewSource(seed))
		nodes := randomTree(rng, 5000)
		for _, node := range nodes {
			if node.height == 0 {
				continue
			}
			want := linearAncestor(node, skipHeight(node.height))
			if node.skip != want {
				t.Fatalf("seed %d: node at height %d has the wrong "+
					"skip", seed, node.height)
			}
		}
		for range 20000 {
			node := nodes[rng.Intn(len(nodes))]
			height := rng.Int31n(node.height+3) - 1
			want := linearAncestor(node, height)
			if got := node.Ancestor(height); got != want {
				t.Fatalf("seed %d: Ancestor(%d) from height %d "+
					"returned the wrong node", seed, height,
					node.height)
			}
			distance := node.height - height
			if got := node.RelativeAncestor(distance); got != want {
				t.Fatalf("seed %d: RelativeAncestor(%d) from "+
					"height %d returned the wrong node", seed,
					distance, node.height)
			}
		}
	}
}

// TestAncestorSteps ensures the walk to any ancestor takes a logarithmic
// number of steps.
func TestAncestorSteps(t *testing.T) {
	t.Parallel()

	tip := tstTip(chainedNodes(nil, 100000))
	for height := range tip.height {
		steps := 0
		for n := tip; n.height != height; n = n.ancestorStep(height) {
			steps++
		}
		if steps > 120 {
			t.Fatalf("walk from %d to %d takes %d steps", tip.height,
				height, steps)
		}
	}
}

// BenchmarkAncestor measures a lookup far back from the tip of a long chain.
func BenchmarkAncestor(b *testing.B) {
	tip := tstTip(chainedNodes(nil, 500000))
	for i := 0; b.Loop(); i++ {
		if tip.Ancestor(int32(i%1000)) == nil {
			b.Fatal("no ancestor")
		}
	}
}
