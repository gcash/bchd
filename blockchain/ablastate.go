// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

const (
	// ablaSideCacheSize is the maximum number of side chain ABLA states
	// kept in memory before the cache is cleared.
	ablaSideCacheSize = 1024

	// ablaRebuildBatch is the number of blocks whose ABLA state is
	// rebuilt in one database transaction.
	ablaRebuildBatch = 1000
)

// ablaBaseState returns the ABLA state after a block at the given height
// that is below the activation height.  It is the configured initial state.
func (b *BlockChain) ablaBaseState(height int32) ABLAState {
	return ABLAState{
		blockHeight:       uint64(height + 1),
		controlBlockSize:  b.ablaConfig.epsilon0,
		elasticBufferSize: b.ablaConfig.beta0,
	}
}

// ablaNextState returns the ABLA state after a block of the given size at the
// given height, where prev is the state after its parent.  The activation
// block starts from the initial state.
func (b *BlockChain) ablaNextState(prev ABLAState, height int32, size uint64) ABLAState {
	forkHeight := b.chainParams.ABLAForkHeight
	if height < forkHeight {
		return b.ablaBaseState(height)
	}
	if height == forkHeight {
		prev = b.ablaBaseState(height - 1)
	}
	return prev.nextABLAState(&b.ablaConfig, size)
}

// ablaStateAfter returns the ABLA state after the passed node, which sets the
// size limit of its children.  Side chain states are computed from the fork
// point using the stored block sizes.  The sizes are taken from where the
// database stores the blocks, which it keeps for pruned blocks too, so no block
// data is read, as Bitcoin Cash Node keeps the state of every block in its
// block index.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) ablaStateAfter(node *blockNode) (ABLAState, error) {
	if node.height < b.chainParams.ABLAForkHeight {
		return b.ablaBaseState(node.height), nil
	}
	if node == b.bestChain.Tip() {
		return b.ablaState, nil
	}
	if b.bestChain.Contains(node) {
		var state ABLAState
		err := b.db.View(func(dbTx database.Tx) error {
			var err error
			state, err = b.dbFetchValidABLAState(dbTx, node.height)
			return err
		})
		return state, err
	}
	if state, ok := b.ablaSideCache[node.hash]; ok {
		return state, nil
	}

	// Walk back to the main chain, then replay the side chain blocks.
	var sideNodes []*blockNode
	start := node
	for ; start != nil && !b.bestChain.Contains(start); start = start.parent {
		if _, ok := b.ablaSideCache[start.hash]; ok {
			break
		}
		sideNodes = append(sideNodes, start)
	}
	if start == nil {
		return ABLAState{}, AssertError(fmt.Sprintf("block %v does not "+
			"connect to the main chain", node.hash))
	}
	state, err := b.ablaStateAfter(start)
	if err != nil {
		return ABLAState{}, err
	}
	err = b.db.View(func(dbTx database.Tx) error {
		for i := len(sideNodes) - 1; i >= 0; i-- {
			n := sideNodes[i]
			size, err := dbTx.FetchBlockSize(&n.hash)
			if err != nil {
				return err
			}
			state = b.ablaNextState(state, n.height, uint64(size))
			b.cacheSideABLAState(n.hash, state)
		}
		return nil
	})
	return state, err
}

// cacheSideABLAState remembers the ABLA state after a side chain block.  The
// state only depends on the block and its ancestors, so it never goes stale.
func (b *BlockChain) cacheSideABLAState(hash chainhash.Hash, state ABLAState) {
	if len(b.ablaSideCache) >= ablaSideCacheSize {
		b.ablaSideCache = make(map[chainhash.Hash]ABLAState)
	}
	b.ablaSideCache[hash] = state
}

// blockSizeLimit returns the maximum serialized size of a block at the given
// height whose parent is prevNode, with floor as the limit before ABLA and
// the floor of the ABLA limit after.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) blockSizeLimit(prevNode *blockNode, height int32, floor uint64) (uint64, error) {
	if height <= b.chainParams.UahfForkHeight {
		return LegacyMaxBlockSize, nil
	}
	if height <= b.chainParams.ABLAForkHeight {
		return floor, nil
	}
	state, err := b.ablaStateAfter(prevNode)
	if err != nil {
		return 0, err
	}
	// The floor is a minimum, as in Bitcoin Cash Node.
	return max(floor, state.getBlockSizeLimit()), nil
}

// networkSizeFloor returns the floor of the block size limit that the network
// enforces: its default block size, as in Bitcoin Cash Node.  Blocks are
// validated against it, so a larger excessive block size does not loosen
// validation.  On regtest and simnet, which have no network to agree with,
// and for chain parameters without a default block size, it is the excessive
// block size.
func (b *BlockChain) networkSizeFloor() uint64 {
	floor := uint64(b.excessiveBlockSize)
	switch b.chainParams.Net {
	case wire.TestNet, wire.SimNet:
		return floor
	}
	if size := b.chainParams.DefaultConsensusBlockSize; size != 0 {
		floor = min(floor, uint64(size))
	}
	return floor
}

// NextBlockSizeLimit returns the maximum serialized size the network allows a
// block that extends the block with the passed hash: the larger of its default
// block size and the ABLA limit.  Block templates are built to it.  See
// networkSizeFloor for when the excessive block size counts instead.
//
// This function is safe for concurrent access.
func (b *BlockChain) NextBlockSizeLimit(prevHash *chainhash.Hash) (uint64, error) {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	prevNode := b.index.LookupNode(prevHash)
	if prevNode == nil {
		return 0, fmt.Errorf("block %v is not known", prevHash)
	}
	return b.blockSizeLimit(prevNode, prevNode.height+1, b.networkSizeFloor())
}

// dbFetchValidABLAState fetches the stored ABLA state after the main chain
// block at the given height and checks it against the configuration.
func (b *BlockChain) dbFetchValidABLAState(dbTx database.Tx, height int32) (ABLAState, error) {
	state, err := dbFetchAblaStateByHeight(dbTx, height)
	if err != nil {
		return ABLAState{}, err
	}
	if errs := state.IsValid(&b.ablaConfig); errs != nil {
		return ABLAState{}, AssertError(fmt.Sprintf("invalid ABLA state "+
			"at height %d: %s", height, errs.String()))
	}
	return *state, nil
}

// serializeABLAConfig returns the activation height and the ABLA parameters
// that determine the stored states.
func (b *BlockChain) serializeABLAConfig() []byte {
	c := &b.ablaConfig
	fields := []uint64{c.epsilon0, c.beta0, c.n0, c.gammaReciprocal,
		c.zetaXB7, c.thetaReciprocal, c.delta, c.epsilonMax, c.betaMax}
	serialized := make([]byte, 4, 4+8*len(fields))
	byteOrder.PutUint32(serialized, uint32(b.chainParams.ABLAForkHeight))
	for _, field := range fields {
		serialized = byteOrder.AppendUint64(serialized, field)
	}
	return serialized
}

// isBlockDataMissing returns whether err reports that the data of a block is
// not stored, as for a pruned block.
func isBlockDataMissing(err error) bool {
	var dbErr database.Error
	if !errors.As(err, &dbErr) {
		return false
	}
	return dbErr.ErrorCode == database.ErrBlockNotFound ||
		(dbErr.ErrorCode == database.ErrDriverSpecific &&
			errors.Is(dbErr.Err, os.ErrNotExist))
}

// initABLAState loads the ABLA state after the chain tip.  Missing or invalid
// stored states, such as those written by earlier versions, which kept only
// the elastic buffer size, are rebuilt from the stored block sizes.  All of
// them are rebuilt when they were stored with another ABLA configuration.
func (b *BlockChain) initABLAState() error {
	b.ablaSideCache = make(map[chainhash.Hash]ABLAState)

	var pruned, configChanged bool
	config := b.serializeABLAConfig()
	err := b.db.Update(func(dbTx database.Tx) error {
		pruned = bytes.Equal(dbFetchBlockchainType(dbTx), prunedBlockchainEntryValue)
		meta := dbTx.Metadata()
		configChanged = !bytes.Equal(meta.Get(ablaConfigKeyName), config)
		_, err := meta.CreateBucketIfNotExists(ablaStateBucketName)
		return err
	})
	if err != nil {
		return err
	}
	putConfig := func() error {
		if !configChanged {
			return nil
		}
		return b.db.Update(func(dbTx database.Tx) error {
			return dbTx.Metadata().Put(ablaConfigKeyName, config)
		})
	}

	tip := b.bestChain.Tip()
	forkHeight := b.chainParams.ABLAForkHeight
	if tip.height < forkHeight {
		b.ablaState = b.ablaBaseState(tip.height)
		return putConfig()
	}

	// Find the first missing or invalid state.  One pass over the bucket
	// is much faster than a read per height.
	firstBad := int32(-1)
	if configChanged {
		firstBad = forkHeight
	}
	err = b.db.View(func(dbTx database.Tx) error {
		if firstBad >= 0 {
			return nil
		}
		valid := make([]bool, tip.height-forkHeight+1)
		bucket := dbTx.Metadata().Bucket(ablaStateBucketName)
		err := bucket.ForEach(func(k, v []byte) error {
			if len(k) != 4 {
				return nil
			}
			h := int32(byteOrder.Uint32(k))
			if h < forkHeight || h > tip.height {
				return nil
			}
			state, err := deserializeABLAState(h, v)
			valid[h-forkHeight] = err == nil &&
				state.IsValid(&b.ablaConfig) == nil
			return nil
		})
		if err != nil {
			return err
		}
		for i, ok := range valid {
			if !ok {
				firstBad = forkHeight + int32(i)
				return nil
			}
		}
		state, err := b.dbFetchValidABLAState(dbTx, tip.height)
		b.ablaState = state
		return err
	})
	if err != nil || firstBad < 0 {
		return err
	}

	log.Infof("Rebuilding the ABLA state for blocks %d to %d. This "+
		"might take a while...", firstBad, tip.height)
	state := b.ablaBaseState(firstBad - 1)
	if firstBad > forkHeight {
		err := b.db.View(func(dbTx database.Tx) error {
			var err error
			state, err = b.dbFetchValidABLAState(dbTx, firstBad-1)
			return err
		})
		if err != nil {
			return err
		}
	}
	var approximated int
	for h := firstBad; h <= tip.height; {
		err := b.db.Update(func(dbTx database.Tx) error {
			for i := 0; i < ablaRebuildBatch && h <= tip.height; i++ {
				// The size is taken from where the database
				// stores the block, which it keeps for pruned
				// blocks too, so the block is not read, as
				// Bitcoin Cash Node reads only the size.  A block
				// the database has no location for, as one a
				// fast sync never stored, is approximated.
				node := b.bestChain.NodeByHeight(h)
				size, err := dbTx.FetchBlockSize(&node.hash)
				switch {
				case err == nil:
					state = b.ablaNextState(state, h, uint64(size))
				case (pruned || b.pruneMode) && isBlockDataMissing(err):
					state = b.prunedABLAState(dbTx, h)
					approximated++
				default:
					return err
				}
				if err := dbPutABLAStateIndex(dbTx, state, h); err != nil {
					return err
				}
				h++
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if approximated > 0 {
		log.Warnf("The ABLA state of %d pruned blocks was approximated "+
			"from the stored elastic buffer size", approximated)
	}
	if err := putConfig(); err != nil {
		return err
	}
	b.ablaState = state
	log.Info("Done rebuilding the ABLA state")
	return nil
}

// prunedABLAState approximates the ABLA state after a main chain block whose
// size the database does not have, using the state stored for it.  Earlier
// versions stored only the elastic buffer size, in the first eight bytes and
// with zero after it, so the control block size is then taken to be at its
// floor.  Without a usable stored state, the initial state is used.
func (b *BlockChain) prunedABLAState(dbTx database.Tx, height int32) ABLAState {
	state := b.ablaBaseState(height)
	var serializedHeight [4]byte
	byteOrder.PutUint32(serializedHeight[:], uint32(height))
	bucket := dbTx.Metadata().Bucket(ablaStateBucketName)
	data := bucket.Get(serializedHeight[:])
	switch {
	case len(data) == 16 && byteOrder.Uint64(data[8:16]) == 0:
		elastic := byteOrder.Uint64(data[:8])
		state.elasticBufferSize = min(max(elastic, b.ablaConfig.beta0),
			b.ablaConfig.betaMax)
	case data != nil:
		stored, err := deserializeABLAState(height, data)
		if err == nil && stored.IsValid(&b.ablaConfig) == nil {
			state = *stored
		}
	}
	return state
}
