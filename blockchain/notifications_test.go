// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
)

// TestNotifications ensures that notification callbacks are fired on events.
func TestNotifications(t *testing.T) {
	blocks, err := loadBlocks("blk_0_to_4.dat.bz2")
	if err != nil {
		t.Fatalf("Error loading file: %v\n", err)
	}

	// Create a new database and chain instance to run tests against.
	chain, teardownFunc, err := chainSetup("notifications",
		&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("Failed to setup chain instance: %v", err)
	}
	defer teardownFunc()

	notificationCount := 0
	callback := func(notification *Notification) {
		if notification.Type == NTBlockAccepted {
			notificationCount++
		}
	}

	// Register callback multiple times then assert it is called that many
	// times.
	const numSubscribers = 3
	for range numSubscribers {
		chain.Subscribe(callback)
	}

	_, _, err = chain.ProcessBlock(blocks[1], BFNone)
	if err != nil {
		t.Fatalf("ProcessBlock fail on block 1: %v\n", err)
	}

	if notificationCount != numSubscribers {
		t.Fatalf("Expected notification callback to be executed %d "+
			"times, found %d", numSubscribers, notificationCount)
	}
}

// TestChainChangeDuringNotification ensures that invalidating, reconsidering
// or finalizing a block while the chain sends a notification, with the chain
// lock released, waits until the chain is done with its change, rather than
// changing the chain in the middle of it or deadlocking.
func TestChainChangeDuringNotification(t *testing.T) {
	t.Run("invalidate", func(t *testing.T) {
		chain, blocks := finalizationTestChain(t, -1, time.Hour)
		params := chain.chainParams
		block := procTestBlock(t, blocks[5].MsgBlock(), 5,
			legacySubsidy(6, params), params.PowLimitBits, nil)
		changeDuringNotification(t, chain, block, NTBlockConnected, block, func() error {
			return chain.InvalidateBlock(blocks[5].Hash())
		})
		if tip := chain.bestChain.Tip(); tip.hash != *blocks[4].Hash() {
			t.Fatalf("tip %v, want block 4", tip.hash)
		}
	})

	t.Run("invalidate when accepted", func(t *testing.T) {
		chain, blocks := finalizationTestChain(t, -1, time.Hour)
		params := chain.chainParams
		block := procTestBlock(t, blocks[5].MsgBlock(), 5,
			legacySubsidy(6, params), params.PowLimitBits, nil)
		changeDuringNotification(t, chain, block, NTBlockAccepted, block, func() error {
			return chain.InvalidateBlock(blocks[5].Hash())
		})
		if tip := chain.bestChain.Tip(); tip.hash != *blocks[4].Hash() {
			t.Fatalf("tip %v, want block 4", tip.hash)
		}
	})

	t.Run("reconsider", func(t *testing.T) {
		chain, blocks := finalizationTestChain(t, -1, time.Hour)
		params := chain.chainParams
		if err := chain.InvalidateBlock(blocks[5].Hash()); err != nil {
			t.Fatal(err)
		}
		block := procTestBlock(t, blocks[4].MsgBlock(), 4,
			legacySubsidy(5, params)-1, params.PowLimitBits, nil)
		changeDuringNotification(t, chain, block, NTBlockConnected, block, func() error {
			return chain.ReconsiderBlock(blocks[5].Hash())
		})
		node := chain.index.LookupNode(blocks[5].Hash())
		if chain.index.NodeStatus(node).KnownInvalid() {
			t.Fatal("block 5 is still invalid")
		}
	})

	t.Run("finalize", func(t *testing.T) {
		chain, blocks := finalizationTestChain(t, -1, time.Hour)
		params := chain.chainParams

		// A side chain from block 2 with less work than the main chain.
		prev := blocks[2]
		for h := int32(3); h <= 4; h++ {
			block := procTestBlock(t, prev.MsgBlock(), h-1,
				legacySubsidy(h, params)-1, params.PowLimitBits, nil)
			if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
				t.Fatalf("side chain block %d: %v", h, err)
			}
			prev = block
		}
		block := procTestBlock(t, blocks[5].MsgBlock(), 5,
			legacySubsidy(6, params), params.PowLimitBits, nil)
		changeDuringNotification(t, chain, block, NTBlockConnected, block, func() error {
			return chain.FinalizeBlock(prev.Hash())
		})
		if tip := chain.bestChain.Tip(); tip.hash != *prev.Hash() {
			t.Fatalf("tip %v, want the finalized side chain block",
				tip.hash)
		}
	})

	// Finalizing a block that a reorganization is about to disconnect
	// waits for the reorganization, and then takes the chain back to the
	// block, so that the finalized block stays in the main chain.
	t.Run("finalize during a reorganization", func(t *testing.T) {
		chain, blocks := finalizationTestChain(t, -1, time.Hour)
		params := chain.chainParams

		// A side chain from block 3 that block 6 makes the main chain.
		prev := blocks[3]
		for h := int32(4); h <= 5; h++ {
			block := procTestBlock(t, prev.MsgBlock(), h-1,
				legacySubsidy(h, params)-1, params.PowLimitBits, nil)
			if _, _, err := chain.ProcessBlock(block, BFNone); err != nil {
				t.Fatalf("side chain block %d: %v", h, err)
			}
			prev = block
		}
		block := procTestBlock(t, prev.MsgBlock(), 5,
			legacySubsidy(6, params)-1, params.PowLimitBits, nil)
		changeDuringNotification(t, chain, block, NTBlockDisconnected, blocks[5], func() error {
			return chain.FinalizeBlock(blocks[4].Hash())
		})
		if tip := chain.bestChain.Tip(); tip.hash != *blocks[4].Hash() {
			t.Fatalf("tip %v, want the finalized block 4", tip.hash)
		}
	})
}

// changeDuringNotification processes block, and at the notification of the
// given type for the block on, reads the chain, starts change, and keeps the
// notification open for a second.  It fails the test if the change is done
// before the chain is done with block, or if the two aren't both done within
// a minute.
func changeDuringNotification(t *testing.T, chain *BlockChain, block *bchutil.Block,
	typ NotificationType, on *bchutil.Block, change func() error) {

	t.Helper()
	var notified bool
	var changeErr error
	changed := make(chan struct{})
	chain.Subscribe(func(n *Notification) {
		data, ok := n.Data.(*bchutil.Block)
		if notified || n.Type != typ || !ok || *data.Hash() != *on.Hash() {
			return
		}
		notified = true
		chain.FinalizedBlockHash()
		go func() {
			changeErr = change()
			close(changed)
		}()
		select {
		case <-changed:
			t.Error("the change was made during the notification")
		case <-time.After(time.Second):
		}
	})

	processed := make(chan error, 1)
	go func() {
		_, _, err := chain.ProcessBlock(block, BFNone)
		processed <- err
	}()
	timeout := time.After(time.Minute)
	select {
	case err := <-processed:
		if err != nil {
			t.Fatalf("processing the block: %v", err)
		}
	case <-timeout:
		t.Fatal("processing the block deadlocked")
	}
	if !notified {
		t.Fatal("no notification of the block")
	}
	select {
	case <-changed:
	case <-timeout:
		t.Fatal("the change deadlocked")
	}
	if changeErr != nil {
		t.Fatalf("the change failed: %v", changeErr)
	}
}
