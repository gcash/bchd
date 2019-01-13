// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"github.com/gcash/bchd/avalanche"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// PeerNotifier exposes methods to notify peers of status changes to
// transactions, blocks, etc. Currently server (in the main package) implements
// this interface.
type PeerNotifier interface {
	AnnounceNewTransactions(newTxs []*mempool.TxDesc)

	UpdatePeerHeights(latestBlkHash *chainhash.Hash, latestHeight int32, updateSource *peer.Peer)

	RelayInventory(invVect *wire.InvVect, data interface{})

	TransactionConfirmed(tx *bchutil.Tx)
}

// AvalancheNotifier exposes a method to notify the avalanche manager of a new transaction.
type AvalancheNotifier interface {
	// NewTransaction submits the given transactions to the avalanche manager.
	NewTransaction(tx *avalanche.TxDesc)
}

// Config is a configuration struct used to initialize a new SyncManager.
type Config struct {
	PeerNotifier      PeerNotifier
	AvalancheNotifier AvalancheNotifier
	Chain             *blockchain.BlockChain
	TxMemPool         *mempool.TxPool
	ChainParams       *chaincfg.Params

	DisableCheckpoints bool
	MaxPeers           int

	FeeEstimator *mempool.FeeEstimator

	MinSyncPeerNetworkSpeed uint64

	FastSyncMode bool
}
