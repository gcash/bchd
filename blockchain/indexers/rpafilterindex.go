// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"errors"
	"fmt"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs/builder"
)

const (
	// rpaFilterIndexName is the human-readable name for the index.
	rpaFilterIndexName = "input prefix filter index"
)

// The input prefix filter (filter type 2) of a block holds the first two
// bytes of the double SHA-256 of each of the first 30 inputs of every
// transaction but the coinbase, so that a wallet on a reusable payment
// address finds the blocks that may pay it without telling anyone its
// prefix. It is built from the block alone and takes a few hundred bytes a
// block. Its filters, filter hashes and headers are kept in three buckets, by
// block hash, like the other filter types'.
var (
	// rpaFilterIndexParentBucketKey is the name of the parent bucket of the
	// index.
	rpaFilterIndexParentBucketKey = []byte("rpafilterindexparentbucket")

	// rpaFilterKey, rpaFilterHeaderKey and rpaFilterHashKey are the buckets
	// of the filters, their headers and their hashes.
	rpaFilterKey       = []byte("cf2byhashidx")
	rpaFilterHeaderKey = []byte("cf2headerbyhashidx")
	rpaFilterHashKey   = []byte("cf2hashbyhashidx")
)

// errNotRPAFilter is returned when the input prefix filter index is asked for
// a filter type other than its own.
var errNotRPAFilter = errors.New("the input prefix filter index only serves " +
	"the input prefix filter type")

// RPAFilterIndex implements the input prefix filter by hash index.
type RPAFilterIndex struct {
	db          database.DB
	chainParams *chaincfg.Params
}

// Ensure the RPAFilterIndex type implements the Indexer interface.
var _ Indexer = (*RPAFilterIndex)(nil)

// Init initializes the index. This is part of the Indexer interface.
func (idx *RPAFilterIndex) Init() error {
	return nil
}

// StartBlock returns no start block, so the index is built from genesis.
//
// This is part of the Indexer interface.
func (idx *RPAFilterIndex) StartBlock() (*chainhash.Hash, int32) {
	return nil, -1
}

// Migrate has nothing to do: this is the index's first version.
//
// This is part of the Indexer interface.
func (idx *RPAFilterIndex) Migrate(database.DB, <-chan struct{}) error {
	return nil
}

// Key returns the database key to use for the index as a byte slice. This is
// part of the Indexer interface.
func (idx *RPAFilterIndex) Key() []byte {
	return rpaFilterIndexParentBucketKey
}

// Name returns the human-readable name of the index. This is part of the
// Indexer interface.
func (idx *RPAFilterIndex) Name() string {
	return rpaFilterIndexName
}

// Create is invoked when the indexer manager determines the index needs to be
// created for the first time. It creates the parent bucket and the buckets of
// the filters, their headers and their hashes.
func (idx *RPAFilterIndex) Create(dbTx database.Tx) error {
	parent, err := dbTx.Metadata().CreateBucket(rpaFilterIndexParentBucketKey)
	if err != nil {
		return err
	}
	for _, key := range [][]byte{rpaFilterKey, rpaFilterHeaderKey, rpaFilterHashKey} {
		if _, err := parent.CreateBucket(key); err != nil {
			return err
		}
	}
	return nil
}

// rpaFilterBucket returns one of the index's buckets.
func rpaFilterBucket(dbTx database.Tx, key []byte) (database.Bucket, error) {
	parent := dbTx.Metadata().Bucket(rpaFilterIndexParentBucketKey)
	if parent == nil {
		return nil, fmt.Errorf("bucket nil for key: %s", rpaFilterIndexParentBucketKey)
	}
	bucket := parent.Bucket(key)
	if bucket == nil {
		return nil, fmt.Errorf("bucket nil for key: %s", key)
	}
	return bucket, nil
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain. It builds the block's input prefix filter from
// the block, then stores the filter, its hash and its header. This is part of
// the Indexer interface.
func (idx *RPAFilterIndex) ConnectBlock(dbTx database.Tx, block *bchutil.Block,
	_ []blockchain.SpentTxOut) error {

	filter := builder.BuildRPAFilter(block.MsgBlock())
	filterHash := builder.GetRPAFilterHash(filter)

	// The previous block's header, or zero for genesis.
	var prevHeader chainhash.Hash
	headers, err := rpaFilterBucket(dbTx, rpaFilterHeaderKey)
	if err != nil {
		return err
	}
	if prev := &block.MsgBlock().Header.PrevBlock; !prev.IsEqual(&zeroHash) {
		stored := headers.Get(prev[:])
		if len(stored) != chainhash.HashSize {
			return fmt.Errorf("no input prefix filter header for block %v, "+
				"the parent of %v", prev, block.Hash())
		}
		copy(prevHeader[:], stored)
	}
	header := builder.MakeHeaderForRPAFilter(filter, prevHeader)

	h := block.Hash()
	for _, entry := range []struct {
		key   []byte
		value []byte
	}{
		{rpaFilterKey, filter},
		{rpaFilterHashKey, filterHash[:]},
		{rpaFilterHeaderKey, header[:]},
	} {
		bucket, err := rpaFilterBucket(dbTx, entry.key)
		if err != nil {
			return err
		}
		if err := bucket.Put(h[:], entry.value); err != nil {
			return err
		}
	}
	return nil
}

// DisconnectBlock is invoked by the index manager when a block has been
// disconnected from the main chain. It removes the block's filter, hash and
// header. This is part of the Indexer interface.
func (idx *RPAFilterIndex) DisconnectBlock(dbTx database.Tx, block *bchutil.Block,
	_ []blockchain.SpentTxOut) error {

	for _, key := range [][]byte{rpaFilterKey, rpaFilterHeaderKey, rpaFilterHashKey} {
		bucket, err := rpaFilterBucket(dbTx, key)
		if err != nil {
			return err
		}
		if err := bucket.Delete(block.Hash()[:]); err != nil {
			return err
		}
	}
	return nil
}

// entriesByBlockHashes fetches entries of one bucket for a slice of block
// hashes. An absent entry is an empty slice, as in the committed filter index.
func (idx *RPAFilterIndex) entriesByBlockHashes(key []byte, filterType wire.FilterType,
	blockHashes []*chainhash.Hash) ([][]byte, error) {

	if filterType != wire.GCSFilterInputPrefix {
		return nil, errNotRPAFilter
	}
	entries := make([][]byte, 0, len(blockHashes))
	err := idx.db.View(func(dbTx database.Tx) error {
		bucket, err := rpaFilterBucket(dbTx, key)
		if err != nil {
			return err
		}
		for _, blockHash := range blockHashes {
			entries = append(entries, bucket.Get(blockHash[:]))
		}
		return nil
	})
	return entries, err
}

// entryByBlockHash fetches one entry of one bucket by block hash.
func (idx *RPAFilterIndex) entryByBlockHash(key []byte, filterType wire.FilterType,
	h *chainhash.Hash) ([]byte, error) {

	entries, err := idx.entriesByBlockHashes(key, filterType, []*chainhash.Hash{h})
	if err != nil {
		return nil, err
	}
	return entries[0], nil
}

// FilterByBlockHash returns a block's input prefix filter.
func (idx *RPAFilterIndex) FilterByBlockHash(h *chainhash.Hash,
	filterType wire.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(rpaFilterKey, filterType, h)
}

// FiltersByBlockHashes returns the input prefix filters of a set of blocks.
func (idx *RPAFilterIndex) FiltersByBlockHashes(blockHashes []*chainhash.Hash,
	filterType wire.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(rpaFilterKey, filterType, blockHashes)
}

// FilterHeaderByBlockHash returns a block's input prefix filter header.
func (idx *RPAFilterIndex) FilterHeaderByBlockHash(h *chainhash.Hash,
	filterType wire.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(rpaFilterHeaderKey, filterType, h)
}

// FilterHeadersByBlockHashes returns the input prefix filter headers of a set
// of blocks.
func (idx *RPAFilterIndex) FilterHeadersByBlockHashes(blockHashes []*chainhash.Hash,
	filterType wire.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(rpaFilterHeaderKey, filterType, blockHashes)
}

// FilterHashByBlockHash returns the hash of a block's input prefix filter.
func (idx *RPAFilterIndex) FilterHashByBlockHash(h *chainhash.Hash,
	filterType wire.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(rpaFilterHashKey, filterType, h)
}

// FilterHashesByBlockHashes returns the hashes of the input prefix filters of a
// set of blocks.
func (idx *RPAFilterIndex) FilterHashesByBlockHashes(blockHashes []*chainhash.Hash,
	filterType wire.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(rpaFilterHashKey, filterType, blockHashes)
}

// NewRPAFilterIndex returns a new instance of an indexer that is used to
// create a mapping of the hashes of all blocks in the blockchain to their
// input prefix filters, filter hashes and filter headers.
func NewRPAFilterIndex(db database.DB, chainParams *chaincfg.Params) *RPAFilterIndex {
	return &RPAFilterIndex{db: db, chainParams: chainParams}
}

// DropRPAFilterIndex drops the input prefix filter index from the provided
// database if it exists.
func DropRPAFilterIndex(db database.DB, interrupt <-chan struct{}) error {
	return dropIndex(db, rpaFilterIndexParentBucketKey, rpaFilterIndexName, interrupt)
}
