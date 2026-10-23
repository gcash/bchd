// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// progressTestHarness is a sync manager with a sync peer that sends the
// blocks of a regression test chain, and whose progress lines are logged to
// a buffer.
type progressTestHarness struct {
	t      *testing.T
	sm     *SyncManager
	params *chaincfg.Params
	peer   *peerpkg.Peer
	blocks []*bchutil.Block
	logged bytes.Buffer
}

// newProgressTestHarness returns a progress test harness with n blocks after
// the genesis block for the sync peer to send.
func newProgressTestHarness(t *testing.T, n int32) *progressTestHarness {
	t.Helper()

	sm, params, peer := unrequestedTestManager(t)
	h := &progressTestHarness{t: t, sm: sm, params: params, peer: peer}
	logger := bchlog.NewBackend(&h.logged).Logger("SYNC")
	logger.SetLevel(bchlog.LevelInfo)
	sm.progressLogger = newBlockProgressLogger("Processed", logger)
	sm.syncPeer = peer
	sm.syncPeerState = &syncPeerState{}

	h.blocks = []*bchutil.Block{bchutil.NewBlock(params.GenesisBlock)}
	for height := int32(1); height <= n; height++ {
		prev := h.blocks[height-1].MsgBlock()
		h.blocks = append(h.blocks, mineBlock(t, params, prev, height, 0))
	}
	return h
}

// send has the sync peer send the block at the height, as requested.
func (h *progressTestHarness) send(height int32) {
	h.sendBlock(h.blocks[height])
}

// sendBlock has the sync peer send the block, as requested.
func (h *progressTestHarness) sendBlock(block *bchutil.Block) {
	h.sm.peerStates[h.peer].requestedBlocks[*block.Hash()] = struct{}{}
	h.sm.handleBlockMsg(&blockMsg{block: block, peer: h.peer})
}

// due makes the next progress line due, as when 10 seconds have passed since
// the last one.
func (h *progressTestHarness) due() {
	h.sm.progressLogger.lastBlockLogTime = time.Now().Add(-time.Minute)
}

// hold makes the next progress line wait for the 10 seconds to pass.
func (h *progressTestHarness) hold() {
	h.sm.progressLogger.lastBlockLogTime = time.Now().Add(time.Hour)
}

// expectLines fails the test unless the progress lines logged are one for
// each of want, in order, and each contains its want.
func (h *progressTestHarness) expectLines(want ...string) {
	h.t.Helper()
	var lines []string
	for _, line := range strings.Split(h.logged.String(), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != len(want) {
		h.t.Fatalf("logged %d lines %q, want %d saying %q", len(lines),
			lines, len(want), want)
	}
	for i, line := range lines {
		if !strings.Contains(line, want[i]) {
			h.t.Errorf("line %d %q does not say %q", i, line, want[i])
		}
	}
}

// TestProgressLoggedAtSyncHeight ensures the block that brings the node to
// the sync peer's height is logged although the last line was logged less
// than 10 seconds before, so the log does not stay below 100% until the next
// block.
func TestProgressLoggedAtSyncHeight(t *testing.T) {
	h := newProgressTestHarness(t, 3)
	h.peer.UpdateLastBlockHeight(3)
	h.due()
	h.send(1)
	h.hold()
	h.send(2)
	h.send(3)
	h.expectLines("height 1/3 (33.33%)", "height 3 (100.00%)")
}

// TestProgressAtOrphanTip ensures that when a block connects orphans that
// build on it, the progress is that of the chain's tip.  At the end of a sync
// the sync peer's tip is such an orphan: the peer announces it after each
// batch of blocks it sends, and it arrives before its parents.
func TestProgressAtOrphanTip(t *testing.T) {
	h := newProgressTestHarness(t, 5)
	h.peer.UpdateLastBlockHeight(4)
	h.send(3)
	h.due()
	h.send(1)
	h.due()
	h.send(2)
	h.send(5)
	if got := h.sm.syncHeight(); got != 5 {
		t.Fatalf("sync height %d after the orphan, want 5", got)
	}
	h.hold()
	h.send(4)
	timestamp := func(height int32) time.Time {
		return h.blocks[height].MsgBlock().Header.Timestamp
	}
	h.expectLines("height 1/4 (25.00%)",
		fmt.Sprintf("height 3/4 (75.00%%), %s,", timestamp(3)),
		fmt.Sprintf("height 5 (100.00%%), %s,", timestamp(5)))
}

// TestProgressLimitKept ensures the lines keep the 10 second limit at and
// below the sync height, and that the block that catches up again after the
// node fell behind is logged.
func TestProgressLimitKept(t *testing.T) {
	h := newProgressTestHarness(t, 6)
	h.peer.UpdateLastBlockHeight(2)
	h.due()
	h.send(1)
	h.hold()
	h.send(2)
	h.hold()
	h.send(3)
	h.peer.UpdateLastBlockHeight(5)
	h.send(4)
	h.send(5)
	h.due()
	h.send(6)
	h.expectLines("height 1/2 (50.00%)", "height 2 (100.00%)",
		"height 5 (100.00%)", "height 6 (100.00%)")
}

// TestProgressSideChain ensures a block on a side chain is shown at its own
// height, below the tip, as when the node downloads the chain that replaces
// its own, so the node is not shown at the sync height before it gets there.
func TestProgressSideChain(t *testing.T) {
	h := newProgressTestHarness(t, 3)
	for _, block := range h.blocks[1:] {
		_, _, err := h.sm.chain.ProcessBlock(block, blockchain.BFNone)
		if err != nil {
			t.Fatal(err)
		}
	}
	h.peer.UpdateLastBlockHeight(2)
	h.due()
	h.sendBlock(mineBlock(t, h.params, h.params.GenesisBlock, 1, 1))
	h.expectLines("height 1/2 (50.00%)")
}

// TestProgressFirstLineAtSyncHeight ensures the block that brings the node to
// the sync height is logged at once when no line was logged before.
func TestProgressFirstLineAtSyncHeight(t *testing.T) {
	h := newProgressTestHarness(t, 1)
	h.peer.UpdateLastBlockHeight(1)
	h.hold()
	h.send(1)
	h.expectLines("height 1 (100.00%)")
}

// TestProgressWithoutSyncPeer ensures that without a sync peer the lines have
// no percentage and keep the 10 second limit, and that the block at the sync
// height after such blocks is logged at once.
func TestProgressWithoutSyncPeer(t *testing.T) {
	h := newProgressTestHarness(t, 5)
	h.sm.syncPeer = nil
	h.hold()
	h.send(1)
	h.due()
	h.send(2)
	h.sm.syncPeer = h.peer
	h.hold()
	h.send(3)
	h.sm.syncPeer = nil
	h.hold()
	h.send(4)
	h.sm.syncPeer = h.peer
	h.hold()
	h.send(5)
	h.expectLines("height 2, ", "height 3 (100.00%)", "height 5 (100.00%)")
}

// TestProgressTruncated ensures a line below the sync height is not rounded
// up to 100%.
func TestProgressTruncated(t *testing.T) {
	h := newProgressTestHarness(t, 0)
	block := bchutil.NewBlock(h.blocks[0].MsgBlock())
	block.SetHeight(19999)
	h.due()
	h.sm.progressLogger.LogBlockHeight(block, 20000, h.sm.chain)
	h.expectLines("height 19999/20000 (99.99%)")
}
