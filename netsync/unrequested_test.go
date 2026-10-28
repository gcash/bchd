// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"math/big"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mempool"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// nopNotifier is a PeerNotifier that does nothing.
type nopNotifier struct{}

func (nopNotifier) AnnounceNewTransactions([]*mempool.TxDesc)               {}
func (nopNotifier) UpdatePeerHeights(*chainhash.Hash, int32, *peerpkg.Peer) {}
func (nopNotifier) RelayInventory(*wire.InvVect, interface{})               {}
func (nopNotifier) TransactionConfirmed(*bchutil.Tx)                        {}
func (nopNotifier) Misbehaving(*peerpkg.Peer, uint32, string)               {}

// unrequestedTestManager returns a sync manager on a fresh regression test
// chain and a registered peer that may relay blocks directly.
func unrequestedTestManager(t *testing.T) (*SyncManager, *chaincfg.Params, *peerpkg.Peer) {
	t.Helper()

	params := chaincfg.RegressionNetParams
	db, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &params,
		TimeSource:         blockchain.NewMedianTime(),
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	txPool := mempool.New(&mempool.Config{
		ChainParams:   &params,
		FetchUtxoView: chain.FetchUtxoView,
		BestHeight:    func() int32 { return chain.BestSnapshot().Height },
		MedianTimePast: func() time.Time {
			return chain.BestSnapshot().MedianTime
		},
		CalcSequenceLock: func(tx *bchutil.Tx,
			view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {

			return chain.CalcSequenceLock(tx, view, true)
		},
	})
	sm, err := New(&Config{
		Chain:        chain,
		ChainParams:  &params,
		TxMemPool:    txPool,
		PeerNotifier: nopNotifier{},
		MaxPeers:     8,
	})
	if err != nil {
		t.Fatal(err)
	}

	peer := peerpkg.NewInboundPeer(&peerpkg.Config{ChainParams: &params})
	peer.SetAllowDirectBlockRelay(true)
	sm.peerStates[peer] = &peerSyncState{
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}
	t.Cleanup(peer.Disconnect)
	return sm, &params, peer
}

// TestUnrequestedBlockBelowMinimumChainWork ensures a block that a direct
// relay peer sends without being asked is not stored while it has less than
// the minimum chain work, and that the peer is not disconnected for it.
func TestUnrequestedBlockBelowMinimumChainWork(t *testing.T) {
	sm, params, peer := unrequestedTestManager(t)
	params.MinimumChainWork = new(big.Int).Lsh(big.NewInt(1), 128)

	block := mineBlock(t, params, params.GenesisBlock, 1, 0)
	sm.handleBlockMsg(&blockMsg{block: block, peer: peer})
	if have, _ := sm.chain.HaveBlock(block.Hash()); have {
		t.Fatal("unrequested block below the minimum chain work was " +
			"stored")
	}
	if isDisconnected(peer) {
		t.Fatal("direct relay peer was disconnected")
	}

	// It is stored once it has enough work.
	params.MinimumChainWork = nil
	sm.handleBlockMsg(&blockMsg{block: block, peer: peer})
	if have, _ := sm.chain.HaveBlock(block.Hash()); !have {
		t.Fatal("unrequested block extending the tip was not stored")
	}
}

// TestUnrequestedOrphanStored ensures an orphan that a direct relay peer sends
// without being asked is stored once its parent arrives, even though its chain
// still has less work than the tip, as when a peer relays the blocks of a
// competing chain while this node mines its own.
func TestUnrequestedOrphanStored(t *testing.T) {
	sm, params, peer := unrequestedTestManager(t)

	prev := params.GenesisBlock
	for height := int32(1); height <= 3; height++ {
		block := mineBlock(t, params, prev, height, 0)
		if _, _, err := sm.chain.ProcessBlock(block, blockchain.BFNone); err != nil {
			t.Fatal(err)
		}
		prev = block.MsgBlock()
	}

	// The first block of the competing chain has less work than the tip,
	// so it is ignored.
	fork1 := mineBlock(t, params, params.GenesisBlock, 1, 1)
	fork2 := mineBlock(t, params, fork1.MsgBlock(), 2, 1)
	sm.handleBlockMsg(&blockMsg{block: fork1, peer: peer})
	if have, _ := sm.chain.HaveBlock(fork1.Hash()); have {
		t.Fatal("unrequested block with less work than the tip was stored")
	}

	// The next one is an orphan, so its parent is requested.
	sm.handleBlockMsg(&blockMsg{block: fork2, peer: peer})
	if !sm.chain.IsKnownOrphan(fork2.Hash()) {
		t.Fatal("unrequested orphan was not kept")
	}
	sm.requestBlock(peer, sm.peerStates[peer], fork1.Hash(), false)
	sm.handleBlockMsg(&blockMsg{block: fork1, peer: peer})
	for _, block := range []*bchutil.Block{fork1, fork2} {
		if known, _, _ := sm.chain.BlockValidity(block.Hash()); !known {
			t.Fatalf("block %v of the competing chain was not stored",
				block.Hash())
		}
	}
}

// TestUnrequestedOrphanBelowMinimumChainWork ensures an orphan that a direct
// relay peer sends without being asked while the chain has less than the
// minimum chain work is not stored when its parent arrives, as its chain has
// less than that work.  Until then bchd only downloads the sync peer's chain,
// so a peer could otherwise fork it cheaply from a block about to arrive.
func TestUnrequestedOrphanBelowMinimumChainWork(t *testing.T) {
	sm, params, peer := unrequestedTestManager(t)
	params.MinimumChainWork = new(big.Int).Lsh(big.NewInt(1), 128)

	parent := mineBlock(t, params, params.GenesisBlock, 1, 0)
	orphan := mineBlock(t, params, parent.MsgBlock(), 2, 0)
	sm.handleBlockMsg(&blockMsg{block: orphan, peer: peer})
	if !sm.chain.IsKnownOrphan(orphan.Hash()) {
		t.Fatal("unrequested orphan was not kept")
	}
	sm.requestBlock(peer, sm.peerStates[peer], parent.Hash(), false)
	sm.handleBlockMsg(&blockMsg{block: parent, peer: peer})
	if known, _, _ := sm.chain.BlockValidity(parent.Hash()); !known {
		t.Fatal("requested parent was not stored")
	}
	if have, _ := sm.chain.HaveBlock(orphan.Hash()); have {
		t.Fatal("unrequested orphan below the minimum chain work was " +
			"stored")
	}

	// Once the chain has the minimum chain work, one is stored though its
	// chain has less work than the tip.
	params.MinimumChainWork = big.NewInt(1)
	prev := parent.MsgBlock()
	for height := int32(2); height <= 3; height++ {
		block := mineBlock(t, params, prev, height, 1)
		if _, _, err := sm.chain.ProcessBlock(block, blockchain.BFNone); err != nil {
			t.Fatal(err)
		}
		prev = block.MsgBlock()
	}
	fork1 := mineBlock(t, params, params.GenesisBlock, 1, 1)
	fork2 := mineBlock(t, params, fork1.MsgBlock(), 2, 1)
	sm.handleBlockMsg(&blockMsg{block: fork2, peer: peer})
	sm.requestBlock(peer, sm.peerStates[peer], fork1.Hash(), false)
	sm.handleBlockMsg(&blockMsg{block: fork1, peer: peer})
	if known, _, _ := sm.chain.BlockValidity(fork2.Hash()); !known {
		t.Fatal("unrequested orphan was not stored once the chain had " +
			"the minimum chain work")
	}
}
