// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// failingDB is a database whose read-only transactions fail with err while it
// is set, as when stored data is corrupt or can't be read back.
type failingDB struct {
	database.DB
	err error
}

func (db *failingDB) View(fn func(database.Tx) error) error {
	if db.err != nil {
		return db.err
	}
	return db.DB.View(fn)
}

// TestDatabaseErrorRequestsShutdown ensures a block from a peer that fails on
// a database error, corruption or a failed read, makes the node shut down
// cleanly, which flushes its caches and closes the database, instead of the
// sync manager panicking or going on without the data, and that the peer is
// not sent a reject for it.
func TestDatabaseErrorRequestsShutdown(t *testing.T) {
	for _, dbErr := range []database.Error{
		{ErrorCode: database.ErrCorruption,
			Description: "injected corruption"},
		{ErrorCode: database.ErrDriverSpecific,
			Description: "injected read error"},
	} {
		t.Run(dbErr.ErrorCode.String(), func(t *testing.T) {
			testDatabaseErrorRequestsShutdown(t, dbErr)
		})
	}
}

// testDatabaseErrorRequestsShutdown runs TestDatabaseErrorRequestsShutdown
// for a database whose reads fail with dbErr.
func testDatabaseErrorRequestsShutdown(t *testing.T, dbErr database.Error) {
	params := chaincfg.RegressionNetParams
	ldb, err := database.Create("ffldb", t.TempDir(), params.Net)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ldb.Close() })
	db := &failingDB{DB: ldb}
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
		Chain:        chain,
		ChainParams:  &params,
		PeerNotifier: nopNotifier{},
		MaxPeers:     8,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The remote end records the rejects the sync manager sends.
	rejects := make(chan chainhash.Hash, 2)
	conn, remoteConn := net.Pipe()
	peer := peerpkg.NewInboundPeer(&peerpkg.Config{
		ChainParams:            &params,
		TstAllowSelfConnection: true,
	})
	remote, err := peerpkg.NewOutboundPeer(&peerpkg.Config{
		ChainParams:            &params,
		TstAllowSelfConnection: true,
		Listeners: peerpkg.MessageListeners{
			OnReject: func(_ *peerpkg.Peer, msg *wire.MsgReject) {
				rejects <- msg.Hash
			},
		},
	}, "10.0.0.2:18444")
	if err != nil {
		t.Fatal(err)
	}
	peer.AssociateConnection(tcpConn{conn})
	remote.AssociateConnection(tcpConn{remoteConn})
	t.Cleanup(func() {
		peer.Disconnect()
		remote.Disconnect()
	})
	deadline := time.Now().Add(5 * time.Second)
	for !peer.VerAckReceived() || !remote.VerAckReceived() {
		if time.Now().After(deadline) {
			t.Fatal("handshake did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	sm.peerStates[peer] = newPeerSyncState(false)

	block := mineBlock(t, &params, params.GenesisBlock, 1, 0)
	sm.peerStates[peer].requestedBlocks[*block.Hash()] = struct{}{}
	db.err = dbErr
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("the sync manager panicked: %v", r)
			}
		}()
		sm.handleBlockMsg(&blockMsg{block: block, peer: peer})
	}()
	select {
	case <-chain.RequestedProcessShutdown():
	default:
		t.Fatalf("no shutdown requested for %v", dbErr)
	}

	// An invalid block is rejected, so the first reject the peer gets
	// is for it if none was sent for the first block.
	db.err = nil
	invalid := mineBlock(t, &params, params.GenesisBlock, 1, 1).MsgBlock()
	invalid.Header.MerkleRoot = chainhash.Hash{}
	solveBlock(invalid)
	invalidBlock := bchutil.NewBlock(invalid)
	sm.peerStates[peer].requestedBlocks[*invalidBlock.Hash()] = struct{}{}
	sm.handleBlockMsg(&blockMsg{block: invalidBlock, peer: peer})
	select {
	case hash := <-rejects:
		if hash != *invalidBlock.Hash() {
			t.Fatalf("the peer was sent a reject for block %v, "+
				"which failed on %v", hash, dbErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the invalid block was not rejected")
	}
}
