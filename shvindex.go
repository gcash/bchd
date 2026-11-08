// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/shv"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// shvBuildBatch is the number of block hashes the SHV index reads from the
// chain at a time while it builds.
const shvBuildBatch = 10000

var (
	// errSHVNotInStep is returned while the SHV index is not in step with
	// the main chain: before it first builds, after a build failed, and
	// while the chain is changing blocks the index covers.
	errSHVNotInStep = errors.New("the SHV index is not in step with the " +
		"main chain")

	// errSHVCommitment is returned for a commitment height above the
	// index's tip.
	errSHVCommitment = errors.New("commitment height is above the SHV " +
		"index's tip")

	// errSHVBlockNotFound is returned for a block height the main chain
	// has no block at.
	errSHVBlockNotFound = errors.New("block not found")
)

// shvChain is the part of the block chain the SHV index uses, which
// *blockchain.BlockChain implements.
type shvChain interface {
	BestSnapshot() *blockchain.BestState
	DisconnectCount() uint64
	MainChainHasBlock(hash *chainhash.Hash) bool
	HeightRange(startHeight, endHeight int32) ([]chainhash.Hash, error)
	BlockHashByHeight(height int32) (*chainhash.Hash, error)
	BlockHeightByHash(hash *chainhash.Hash) (int32, error)
	HeaderByHeight(height int32) (wire.BlockHeader, error)
	Subscribe(callback blockchain.NotificationCallback)
}

// shvIndex keeps a Merkle Mountain Range over the hashes of the main chain's
// blocks, from which it proves that a block's header is in the main chain to
// SHV clients (CHIP-2026-02, Simplified Header Verification).
//
// It is built from the block index at startup, and kept in step with the main
// chain as the chain's notifications connect and disconnect blocks.  Only the
// MMR's internal nodes are kept, about 32 bytes per block; the block hashes
// are read from the chain when a proof needs them.
//
// The chain notifies of a change after making it, so while the index handles
// a notification the chain may be a block ahead of it.  A block hash read
// from the chain is then the index's leaf as long as the index's tip is still
// in the main chain, which each proof checks once it has read the hashes.  A
// connected block whose parent is not the index's tip, or a disconnected block
// that is not its tip, as after the headers fast sync adds without
// notifications, brings the index in step with the chain again: it appends
// the blocks past its tip if its tip is still in the main chain, and otherwise
// builds again from genesis.
type shvIndex struct {
	chain shvChain

	// mtx guards the fields below.  Proofs hold it for reads, and the
	// chain's notifications for writes.
	mtx sync.RWMutex

	// mmr is the MMR over the hashes of the main chain's blocks from
	// genesis up to tip, whose height is its leaf count less one.
	mmr shv.MMR
	tip chainhash.Hash

	// inStep is set when the MMR covers the main chain up to tip.  It is
	// cleared while a build fails, until the next notification builds it
	// again.
	inStep bool
}

// newSHVIndex returns an SHV index built over the main chain of the passed
// chain.  The caller subscribes it to the chain's notifications.
func newSHVIndex(chain shvChain) (*shvIndex, error) {
	x := &shvIndex{chain: chain}
	x.mtx.Lock()
	defer x.mtx.Unlock()
	if err := x.syncLocked(); err != nil {
		return nil, err
	}
	return x, nil
}

// syncLocked brings the index in step with the main chain.  When the index's
// tip is still in the main chain, it appends the blocks past it, and
// otherwise it builds the MMR again from genesis.  The chain can change while
// the hashes are read, so it fails when a block was disconnected meanwhile,
// and the index then stays out of step until it next succeeds.
//
// This function MUST be called with the index lock held (for writes).
func (x *shvIndex) syncLocked() error {
	x.inStep = false
	disconnects := x.chain.DisconnectCount()
	if x.mmr.LeafCount() == 0 || !x.chain.MainChainHasBlock(&x.tip) {
		_ = x.mmr.Truncate(0)
	}
	x.mmr.Grow(uint64(max(x.chain.BestSnapshot().Height, 0)) + 1)

	// The hashes are read in batches until the chain runs out.
	for {
		start := x.mmr.LeafCount()
		end := min(start+shvBuildBatch, math.MaxInt32)
		hashes, err := x.chain.HeightRange(int32(start), int32(end))
		if err != nil {
			_ = x.mmr.Truncate(0)
			return err
		}
		for i := range hashes {
			x.mmr.Append(&hashes[i], &x.tip)
			x.tip = hashes[i]
		}
		if uint64(len(hashes)) < end-start {
			break
		}
	}

	// A block disconnected while the hashes were read may have left
	// hashes of different chains in the MMR.
	if x.mmr.LeafCount() == 0 || x.chain.DisconnectCount() != disconnects ||
		!x.chain.MainChainHasBlock(&x.tip) {

		_ = x.mmr.Truncate(0)
		return errors.New("the main chain changed while it was read")
	}
	x.inStep = true
	return nil
}

// handleBlockchainNotification keeps the index in step with the main chain as
// blocks are connected and disconnected.
func (x *shvIndex) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {
	case blockchain.NTBlockConnected, blockchain.NTBlockDisconnected:
	default:
		return
	}
	block, ok := notification.Data.(*bchutil.Block)
	if !ok {
		indxLog.Warnf("Chain notification %v is not for a block",
			notification.Type)
		return
	}

	x.mtx.Lock()
	defer x.mtx.Unlock()

	if x.inStep {
		header := &block.MsgBlock().Header
		switch {
		case notification.Type == blockchain.NTBlockConnected &&
			header.PrevBlock == x.tip:

			x.mmr.Append(block.Hash(), &x.tip)
			x.tip = *block.Hash()
			return

		case notification.Type == blockchain.NTBlockDisconnected &&
			*block.Hash() == x.tip && x.mmr.LeafCount() > 1:

			_ = x.mmr.Truncate(x.mmr.LeafCount() - 1)
			x.tip = header.PrevBlock
			return
		}
		indxLog.Debugf("SHV index at %v (height %d) is out of step with "+
			"the chain at block %v: catching up", x.tip,
			x.mmr.LeafCount()-1, block.Hash())
	}

	if err := x.syncLocked(); err != nil {
		indxLog.Warnf("Unable to bring the SHV index in step with the "+
			"main chain, trying again at the next block: %v", err)
		return
	}
	indxLog.Debugf("SHV index in step with the main chain at %v "+
		"(height %d)", x.tip, x.mmr.LeafCount()-1)
}

// leafFuncLocked returns a function that reads the leaves of the index's MMR,
// the hashes of the main chain's blocks, with the index's tip and, at height,
// the passed leaf.  The caller must check that the chain is still in step
// with the index once it has read them.
//
// This function MUST be called with the index lock held (for reads).
func (x *shvIndex) leafFuncLocked(height uint64, leaf *chainhash.Hash) shv.LeafFunc {
	tipHeight := x.mmr.LeafCount() - 1
	return func(index uint64) (chainhash.Hash, error) {
		switch {
		case index == height:
			return *leaf, nil
		case index == tipHeight:
			return x.tip, nil
		case index > tipHeight:
			return chainhash.Hash{}, errSHVCommitment
		}
		hash, err := x.chain.BlockHashByHeight(int32(index))
		if err != nil {
			return chainhash.Hash{}, err
		}
		return *hash, nil
	}
}

// checkInStepLocked returns errSHVNotInStep unless the index's tip, and the
// passed block, are in the main chain, so that the hashes read from the chain
// are the index's leaves.
//
// This function MUST be called with the index lock held (for reads).
func (x *shvIndex) checkInStepLocked(hash *chainhash.Hash) error {
	if !x.inStep || !x.chain.MainChainHasBlock(&x.tip) ||
		!x.chain.MainChainHasBlock(hash) {

		return errSHVNotInStep
	}
	return nil
}

// tipHeight returns the height of the last block the index covers, and false
// when the index is not in step with the main chain.
func (x *shvIndex) tipHeight() (int32, bool) {
	x.mtx.RLock()
	defer x.mtx.RUnlock()
	return int32(x.mmr.LeafCount()) - 1, x.inStep
}

// proof returns the header of the main chain's block at blockHeight and the
// proof that its hash is the leaf at that height of the commitment at
// commitmentHeight: to the root of the commitment when toRoot is set, and
// otherwise to the peak of the block's mountain, with the root or peak it
// leads to.
//
// It returns errSHVBlockNotFound if the main chain has no block at
// blockHeight, errSHVCommitment if commitmentHeight is above the index's tip,
// and errSHVNotInStep while the index is not in step with the main chain.
func (x *shvIndex) proof(toRoot bool, blockHeight, commitmentHeight uint64) (*wire.BlockHeader, []chainhash.Hash, chainhash.Hash, error) {
	if blockHeight > commitmentHeight {
		return nil, nil, chainhash.Hash{}, shv.ErrLeafIndex
	}
	if blockHeight > math.MaxInt32 {
		return nil, nil, chainhash.Hash{}, errSHVBlockNotFound
	}
	header, err := x.chain.HeaderByHeight(int32(blockHeight))
	if err != nil {
		return nil, nil, chainhash.Hash{}, errSHVBlockNotFound
	}
	leaf := header.BlockHash()

	x.mtx.RLock()
	defer x.mtx.RUnlock()

	if !x.inStep {
		return nil, nil, chainhash.Hash{}, errSHVNotInStep
	}
	if commitmentHeight >= x.mmr.LeafCount() {
		return nil, nil, chainhash.Hash{}, errSHVCommitment
	}

	leafAt := x.leafFuncLocked(blockHeight, &leaf)
	leafCount := commitmentHeight + 1
	var branch []chainhash.Hash
	var target chainhash.Hash
	if toRoot {
		branch, target, err = x.mmr.ProofToRoot(blockHeight, leafCount,
			leafAt)
	} else {
		branch, target, err = x.mmr.ProofToPeak(blockHeight, leafCount,
			leafAt)
	}
	if err != nil {
		return nil, nil, chainhash.Hash{}, err
	}
	if err := x.checkInStepLocked(&leaf); err != nil {
		return nil, nil, chainhash.Hash{}, err
	}
	return &header, branch, target, nil
}

// blockRoot returns the root of the commitment at the height of the main
// chain's block with the passed hash, at the passed height, and false when
// the index does not cover the block or is not in step with the main chain.
func (x *shvIndex) blockRoot(hash *chainhash.Hash, height int32) (chainhash.Hash, bool) {
	x.mtx.RLock()
	defer x.mtx.RUnlock()

	if !x.inStep || height < 0 || uint64(height) >= x.mmr.LeafCount() {
		return chainhash.Hash{}, false
	}
	root, err := x.mmr.Root(uint64(height)+1, x.leafFuncLocked(
		uint64(height), hash))
	if err != nil || x.checkInStepLocked(hash) != nil {
		return chainhash.Hash{}, false
	}
	return root, true
}

// response returns the shv entry for one getshv request.  The status is
// SHVStatusFound, with the header and the proof, when the request can be
// answered.  Otherwise it is the reason there is no proof:
// SHVStatusReservedType when a reserved type bit is set, SHVStatusNotFound
// when the main chain has no such block, SHVStatusAboveCommitment when the
// block is above the commitment, and SHVStatusCommitmentAboveTip when the
// commitment is above the index's tip.  While the index is not in step with
// the main chain, proof cannot show the block is on it, and the status is
// SHVStatusNotFound: proof reports that before it reports a commitment above
// the tip.
func (x *shvIndex) response(req *wire.SHVRequest) wire.SHVResponse {
	if !req.IsValid() {
		return wire.SHVResponse{Status: wire.SHVStatusReservedType}
	}

	var height uint64
	if req.ByHash() {
		blockHeight, err := x.chain.BlockHeightByHash(
			&req.BlockHeightOrHash)
		if err != nil {
			return wire.SHVResponse{Status: wire.SHVStatusNotFound}
		}
		height = uint64(blockHeight)
	} else {
		height = req.BlockHeight()
	}
	if height > req.CommitmentHeight {
		return wire.SHVResponse{Status: wire.SHVStatusAboveCommitment}
	}

	header, branch, target, err := x.proof(req.ProofToRoot(), height,
		req.CommitmentHeight)
	if err != nil {
		// proof returns errSHVBlockNotFound when the main chain has no
		// block at the height, and errSHVCommitment when the commitment
		// is above the index's tip.  A block above the commitment was
		// answered above; proof reports that case as ErrLeafIndex too.
		status := wire.SHVStatusNotFound
		switch {
		case errors.Is(err, errSHVCommitment):
			status = wire.SHVStatusCommitmentAboveTip
		case errors.Is(err, shv.ErrLeafIndex):
			status = wire.SHVStatusAboveCommitment
		}
		return wire.SHVResponse{Status: status}
	}

	// A reorganization may have replaced the block with the hash since
	// its height was looked up.
	if req.ByHash() && header.BlockHash() != req.BlockHeightOrHash {
		return wire.SHVResponse{Status: wire.SHVStatusNotFound}
	}

	return wire.SHVResponse{
		Status:           wire.SHVStatusFound,
		Type:             req.Type,
		CommitmentHeight: req.CommitmentHeight,
		BlockHeight:      height,
		Header:           *header,
		Proof:            branch,
		Target:           target,
	}
}

// initSHVIndex builds the SHV index over the main chain, unless --noshv is
// set, and keeps it in step with the chain's notifications.  When the index
// can't be built, SHV is disabled and the node runs without it.  Without the
// index, the server does not advertise SFNodeSHV.
func (s *server) initSHVIndex(chain shvChain) {
	if cfg.NoSHV {
		s.services &^= wire.SFNodeSHV
		return
	}

	start := time.Now()
	index, err := newSHVIndex(chain)
	if err != nil {
		indxLog.Errorf("Unable to build the SHV index, so SHV is "+
			"disabled: %v", err)
		s.services &^= wire.SFNodeSHV
		return
	}
	chain.Subscribe(index.handleBlockchainNotification)
	s.shvIndex = index

	height, _ := index.tipHeight()
	indxLog.Infof("SHV index is enabled, built over %d blocks in %v",
		int64(height)+1, time.Since(start).Round(time.Millisecond))
}
