// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mempool"
	peerpkg "github.com/gcash/bchd/peer"
)

// tcpConn gives a pipe the TCP remote address an inbound peer needs.
type tcpConn struct {
	net.Conn
}

func (tcpConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 18444}
}

// minWorkTestManager returns a sync manager whose regression test chain has
// three blocks.
func minWorkTestManager(t *testing.T) (*SyncManager, *chaincfg.Params) {
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
	sm, err := New(&Config{
		Chain:       chain,
		ChainParams: &params,
		TxMemPool:   mempool.New(&mempool.Config{ChainParams: &params}),
		MaxPeers:    8,
	})
	if err != nil {
		t.Fatal(err)
	}

	prev := params.GenesisBlock
	for h := int32(1); h <= 3; h++ {
		block := mineBlock(t, &params, prev, h, 0)
		if _, _, err := chain.ProcessBlock(block, blockchain.BFNone); err != nil {
			t.Fatalf("block %d: %v", h, err)
		}
		prev = block.MsgBlock()
	}
	return sm, &params
}

// syncCandidate registers and returns a connected sync candidate whose last
// block is at the passed height.  Its handshake never completes, which leaves
// it connected for far longer than a test runs.
func syncCandidate(t *testing.T, sm *SyncManager, params *chaincfg.Params,
	height int32) *peerpkg.Peer {

	t.Helper()

	conn, remote := net.Pipe()
	p := peerpkg.NewInboundPeer(&peerpkg.Config{ChainParams: params})
	p.AssociateConnection(tcpConn{conn})
	t.Cleanup(func() {
		p.Disconnect()
		remote.Close()
	})
	if !p.Connected() {
		t.Fatal("peer is not connected")
	}
	p.UpdateLastBlockHeight(height)
	sm.peerStates[p] = &peerSyncState{
		syncCandidate:   true,
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}
	return p
}

// setStalledSyncPeer makes p the sync peer, with its last block received
// longer ago than a sync peer may go without one.
func setStalledSyncPeer(sm *SyncManager, p *peerpkg.Peer) {
	p.SetSyncPeer(true)
	sm.syncPeer = p
	sm.syncPeerState = &syncPeerState{
		lastBlockTime: time.Now().Add(-maxLastBlockTime - time.Minute),
	}
}

// TestSyncPeerBelowMinimumChainWorkReplaced ensures a sync peer whose blocks
// this node all has is replaced when the chain they make up has less than the
// minimum chain work, by a peer behind that chain in height if need be.  Below
// the minimum chain work the node ignores other peers' block announcements,
// and it kept a sync peer whose blocks it all had, so a sync peer could hold
// it on a low-work fork longer than the real chain.
func TestSyncPeerBelowMinimumChainWorkReplaced(t *testing.T) {
	sm, params := minWorkTestManager(t)
	params.MinimumChainWork = new(big.Int).Lsh(big.NewInt(1), 128)

	forker := syncCandidate(t, sm, params, 3)
	honest := syncCandidate(t, sm, params, 1)
	setStalledSyncPeer(sm, forker)

	sm.handleCheckSyncPeer()
	if !isDisconnected(forker) {
		t.Fatal("sync peer whose chain is below the minimum chain work " +
			"was kept")
	}
	if sm.syncPeer != honest {
		t.Fatal("peer behind the low-work chain did not become the sync " +
			"peer")
	}
}

// TestSyncPeerWithMinimumChainWorkKept ensures a sync peer whose blocks this
// node all has is kept, though it sent none for a while, once the chain has
// the minimum chain work: it just has no new block.
func TestSyncPeerWithMinimumChainWorkKept(t *testing.T) {
	sm, params := minWorkTestManager(t)
	params.MinimumChainWork = big.NewInt(1)

	peer := syncCandidate(t, sm, params, 3)
	syncCandidate(t, sm, params, 1)
	setStalledSyncPeer(sm, peer)

	sm.handleCheckSyncPeer()
	if sm.syncPeer != peer || isDisconnected(peer) {
		t.Fatal("sync peer at the tip of a chain with the minimum chain " +
			"work was replaced")
	}
}

// TestStartSyncFromPeerBehind ensures sync starts from a peer behind this
// node's chain in height only when no peer is level or ahead and the chain has
// less than the minimum chain work.
func TestStartSyncFromPeerBehind(t *testing.T) {
	sm, params := minWorkTestManager(t)
	behind := syncCandidate(t, sm, params, 1)

	params.MinimumChainWork = big.NewInt(1)
	sm.startSync()
	if sm.syncPeer != nil {
		t.Fatal("sync started from a peer behind a chain with the minimum " +
			"chain work")
	}

	params.MinimumChainWork = new(big.Int).Lsh(big.NewInt(1), 128)
	sm.startSync()
	if sm.syncPeer != behind {
		t.Fatal("sync did not start from the peer behind a chain below " +
			"the minimum chain work")
	}
}
