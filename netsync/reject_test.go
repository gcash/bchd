// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	peerpkg "github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// sentReject is a reject message a peer received.
type sentReject struct {
	code   wire.RejectCode
	hash   chainhash.Hash
	reason string
}

// rejectTestPeer returns a peer registered with sm and connected to a remote
// peer, and the channel of the reject messages the remote peer receives.
func rejectTestPeer(t *testing.T, sm *SyncManager, params *chaincfg.Params) (*peerpkg.Peer, chan sentReject) {
	t.Helper()

	rejects := make(chan sentReject, 2)
	conn, remoteConn := net.Pipe()
	peer := peerpkg.NewInboundPeer(&peerpkg.Config{
		ChainParams:            params,
		TstAllowSelfConnection: true,
	})
	remote, err := peerpkg.NewOutboundPeer(&peerpkg.Config{
		ChainParams:            params,
		TstAllowSelfConnection: true,
		Listeners: peerpkg.MessageListeners{
			OnReject: func(_ *peerpkg.Peer, msg *wire.MsgReject) {
				rejects <- sentReject{msg.Code, msg.Hash, msg.Reason}
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
	return peer, rejects
}

// expectReject fails the test unless the next reject message rejects
// receives is want.
func expectReject(t *testing.T, rejects chan sentReject, want sentReject) {
	t.Helper()

	select {
	case got := <-rejects:
		if got != want {
			t.Fatalf("got a reject for %v, %v, %q, want one for %v, %v, "+
				"%q", got.hash, got.code, got.reason, want.hash,
				want.code, want.reason)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%v was not rejected", want.hash)
	}
}

// TestTxFailureSendsNoReject ensures a transaction the pool fails to check,
// as when the outputs it spends can't be read, gets no reject message, as
// BCHN sends none for an internal error, while one the pool refuses gets the
// reject code and BCHN's reason.
func TestTxFailureSendsNoReject(t *testing.T) {
	sm, params, _ := unrequestedTestManager(t)
	sm.txMemPool = mempool.New(&mempool.Config{
		Policy:      mempool.Policy{MaxTxVersion: 2},
		ChainParams: params,
		FetchUtxoView: func(*bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			return nil, errors.New("injected failure")
		},
		BestHeight:     func() int32 { return 0 },
		MedianTimePast: time.Now,
	})
	peer, rejects := rejectTestPeer(t, sm, params)

	// A spend whose outputs can't be fetched, and a coinbase, which the
	// pool refuses before it fetches any.
	pad := make([]byte, 60)
	spend := wire.NewMsgTx(1)
	spend.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	spend.AddTxOut(wire.NewTxOut(0, append([]byte{txscript.OP_RETURN,
		byte(len(pad))}, pad...), wire.TokenData{}))
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{txscript.OP_1, txscript.OP_1},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	coinbase.AddTxOut(spend.TxOut[0])
	for _, tx := range []*wire.MsgTx{spend, coinbase} {
		sm.handleTxMsg(&txMsg{tx: bchutil.NewTx(tx), peer: peer})
	}
	expectReject(t, rejects, sentReject{wire.RejectInvalid,
		coinbase.TxHash(), "bad-tx-coinbase"})
}

// TestBlockRejectVersion ensures a block whose version is too old gets
// BCHN's reason with the version, and its obsolete code.
func TestBlockRejectVersion(t *testing.T) {
	sm, params, _ := unrequestedTestManager(t)
	params.BIP0065Height = 1
	peer, rejects := rejectTestPeer(t, sm, params)

	block := mineBlock(t, params, params.GenesisBlock, 1, 0).MsgBlock()
	block.Header.Version = 3
	solveBlock(block)
	old := bchutil.NewBlock(block)
	sm.peerStates[peer].requestedBlocks[*old.Hash()] = struct{}{}
	sm.handleBlockMsg(&blockMsg{block: old, peer: peer})
	expectReject(t, rejects, sentReject{wire.RejectObsolete, *old.Hash(),
		"bad-version(0x00000003)"})
}
