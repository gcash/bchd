// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/rand"
	"testing"
	"time"

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
		nodes = append(nodes, newBlockNode(&header, parent,
			upgrade2027Never))
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

// TestPastMedianTime ensures the median time past initBlockNode computes is
// the one CalcPastMedianTime returns, on short chains and with timestamps out
// of order.
func TestPastMedianTime(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	nodes := randomTree(rng, 5000)
	for _, node := range nodes {
		node.timestamp = 1700000000 + rng.Int63n(20) - 10 +
			int64(node.height)*rng.Int63n(1200)
	}
	for _, node := range nodes {
		want := node.CalcPastMedianTime().Unix()
		if got := node.pastMedianTime(); got != want {
			t.Fatalf("node at height %d: median time past %d, want %d",
				node.height, got, want)
		}
	}
}

// BenchmarkInitBlockNode measures creating a node of the block index as the
// index is loaded, on a chain of jittered timestamps.
func BenchmarkInitBlockNode(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	headers := make([]wire.BlockHeader, 1<<16)
	ts := time.Unix(1700000000, 0)
	for i := range headers {
		ts = ts.Add(time.Duration(rng.Intn(1300)-100) * time.Second)
		headers[i] = wire.BlockHeader{Nonce: uint32(i), Timestamp: ts}
	}
	nodes := make([]blockNode, len(headers))
	activation := uint64(ts.Unix())
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		j := i % len(nodes)
		var parent *blockNode
		if j > 0 {
			parent = &nodes[j-1]
		}
		initBlockNode(&nodes[j], &headers[j], parent, activation)
	}
}
