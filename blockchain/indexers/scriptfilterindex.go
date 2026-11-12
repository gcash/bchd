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
	// scriptFilterIndexName is the human-readable name for the index.
	scriptFilterIndexName = "script filter index"
)

// The script filter (filter type 1, gcash/bchd#767) lists the scripts of the
// outputs a block creates and spends, so building it needs the scripts of the
// spent outputs, from the spend journal. It therefore has an index of its own
// beside the committed filter index, which builds the basic filter from the
// block alone. Its filters, filter hashes and headers are kept in three
// buckets, by block hash, like the basic filter's.
var (
	// scriptFilterIndexParentBucketKey is the name of the parent bucket of
	// the index.
	scriptFilterIndexParentBucketKey = []byte("scriptfilterindexparentbucket")

	// scriptFilterKey, scriptFilterHeaderKey and scriptFilterHashKey are
	// the buckets of the served filters, their headers and their hashes.
	scriptFilterKey       = []byte("cf1byhashidx")
	scriptFilterHeaderKey = []byte("cf1headerbyhashidx")
	scriptFilterHashKey   = []byte("cf1hashbyhashidx")
)

// errNotScriptFilter is returned when the script filter index is asked for a
// filter type other than its own.
var errNotScriptFilter = errors.New("the script filter index only serves the script filter type")

// ScriptFilterIndex implements the script filter by hash index.
type ScriptFilterIndex struct {
	db          database.DB
	chainParams *chaincfg.Params
}

// Ensure the ScriptFilterIndex type implements the Indexer and NeedsInputser
// interfaces.
var (
	_ Indexer       = (*ScriptFilterIndex)(nil)
	_ NeedsInputser = (*ScriptFilterIndex)(nil)
)

// NeedsInputs signals that the index requires the outputs a block spends: the
// script filter lists their scripts.
//
// This implements the NeedsInputser interface.
func (idx *ScriptFilterIndex) NeedsInputs() bool {
	return true
}

// Init initializes the index. This is part of the Indexer interface.
func (idx *ScriptFilterIndex) Init() error {
	return nil
}

// StartBlock returns no start block, so the index is built from genesis.
//
// This is part of the Indexer interface.
func (idx *ScriptFilterIndex) StartBlock() (*chainhash.Hash, int32) {
	return nil, -1
}

// Migrate has nothing to do: this is the index's first version.
//
// This is part of the Indexer interface.
func (idx *ScriptFilterIndex) Migrate(database.DB, <-chan struct{}) error {
	return nil
}

// Key returns the database key to use for the index as a byte slice. This is
// part of the Indexer interface.
func (idx *ScriptFilterIndex) Key() []byte {
	return scriptFilterIndexParentBucketKey
}

// Name returns the human-readable name of the index. This is part of the
// Indexer interface.
func (idx *ScriptFilterIndex) Name() string {
	return scriptFilterIndexName
}

// Create is invoked when the indexer manager determines the index needs to be
// created for the first time. It creates the parent bucket and the buckets of
// the filters, their headers and their hashes.
func (idx *ScriptFilterIndex) Create(dbTx database.Tx) error {
	parent, err := dbTx.Metadata().CreateBucket(scriptFilterIndexParentBucketKey)
	if err != nil {
		return err
	}
	for _, key := range [][]byte{scriptFilterKey, scriptFilterHeaderKey, scriptFilterHashKey} {
		if _, err := parent.CreateBucket(key); err != nil {
			return err
		}
	}
	return nil
}

// scriptFilterBucket returns one of the index's buckets.
func scriptFilterBucket(dbTx database.Tx, key []byte) (database.Bucket, error) {
	parent := dbTx.Metadata().Bucket(scriptFilterIndexParentBucketKey)
	if parent == nil {
		return nil, fmt.Errorf("bucket nil for key: %s", scriptFilterIndexParentBucketKey)
	}
	bucket := parent.Bucket(key)
	if bucket == nil {
		return nil, fmt.Errorf("bucket nil for key: %s", key)
	}
	return bucket, nil
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain. It builds the block's script filter from the
// block and the scripts of the outputs it spends, then stores the served
// filter, its hash and its header. This is part of the Indexer interface.
func (idx *ScriptFilterIndex) ConnectBlock(dbTx database.Tx, block *bchutil.Block,
	stxos []blockchain.SpentTxOut) error {

	// The spend journal keeps each spent output's token prefix in front of
	// its script, as the builder expects.
	prevOutScripts := make([][]byte, len(stxos))
	for i := range stxos {
		prevOutScripts[i] = stxos[i].PkScript
	}
	filter, root, err := builder.BuildScriptFilter(block.MsgBlock(), prevOutScripts)
	if err != nil {
		return err
	}
	served, err := builder.ScriptFilterBytes(filter, root)
	if err != nil {
		return err
	}
	filterHash, err := builder.GetScriptFilterHash(filter, root)
	if err != nil {
		return err
	}

	// The previous block's header, or zero for genesis.
	var prevHeader chainhash.Hash
	headers, err := scriptFilterBucket(dbTx, scriptFilterHeaderKey)
	if err != nil {
		return err
	}
	if prev := &block.MsgBlock().Header.PrevBlock; !prev.IsEqual(&zeroHash) {
		stored := headers.Get(prev[:])
		if len(stored) != chainhash.HashSize {
			return fmt.Errorf("no script filter header for block %v, "+
				"the parent of %v", prev, block.Hash())
		}
		copy(prevHeader[:], stored)
	}
	header, err := builder.MakeHeaderForScriptFilter(filter, root, prevHeader)
	if err != nil {
		return err
	}

	h := block.Hash()
	for _, entry := range []struct {
		key   []byte
		value []byte
	}{
		{scriptFilterKey, served},
		{scriptFilterHashKey, filterHash[:]},
		{scriptFilterHeaderKey, header[:]},
	} {
		bucket, err := scriptFilterBucket(dbTx, entry.key)
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
func (idx *ScriptFilterIndex) DisconnectBlock(dbTx database.Tx, block *bchutil.Block,
	_ []blockchain.SpentTxOut) error {

	for _, key := range [][]byte{scriptFilterKey, scriptFilterHeaderKey, scriptFilterHashKey} {
		bucket, err := scriptFilterBucket(dbTx, key)
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
func (idx *ScriptFilterIndex) entriesByBlockHashes(key []byte, filterType wire.FilterType,
	blockHashes []*chainhash.Hash) ([][]byte, error) {

	if filterType != wire.GCSFilterScripts {
		return nil, errNotScriptFilter
	}
	entries := make([][]byte, 0, len(blockHashes))
	err := idx.db.View(func(dbTx database.Tx) error {
		bucket, err := scriptFilterBucket(dbTx, key)
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
func (idx *ScriptFilterIndex) entryByBlockHash(key []byte, filterType wire.FilterType,
	h *chainhash.Hash) ([]byte, error) {

	entries, err := idx.entriesByBlockHashes(key, filterType, []*chainhash.Hash{h})
	if err != nil {
		return nil, err
	}
	return entries[0], nil
}

// FilterByBlockHash returns a block's served script filter: the root of its
// entry tree, then the GCS filter.
func (idx *ScriptFilterIndex) FilterByBlockHash(h *chainhash.Hash,
	filterType wire.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(scriptFilterKey, filterType, h)
}

// FiltersByBlockHashes returns the served script filters of a set of blocks.
func (idx *ScriptFilterIndex) FiltersByBlockHashes(blockHashes []*chainhash.Hash,
	filterType wire.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(scriptFilterKey, filterType, blockHashes)
}

// FilterHeaderByBlockHash returns a block's script filter header.
func (idx *ScriptFilterIndex) FilterHeaderByBlockHash(h *chainhash.Hash,
	filterType wire.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(scriptFilterHeaderKey, filterType, h)
}

// FilterHeadersByBlockHashes returns the script filter headers of a set of
// blocks.
func (idx *ScriptFilterIndex) FilterHeadersByBlockHashes(blockHashes []*chainhash.Hash,
	filterType wire.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(scriptFilterHeaderKey, filterType, blockHashes)
}

// FilterHashByBlockHash returns the hash of a block's served script filter.
func (idx *ScriptFilterIndex) FilterHashByBlockHash(h *chainhash.Hash,
	filterType wire.FilterType) ([]byte, error) {
	return idx.entryByBlockHash(scriptFilterHashKey, filterType, h)
}

// FilterHashesByBlockHashes returns the hashes of the served script filters of
// a set of blocks.
func (idx *ScriptFilterIndex) FilterHashesByBlockHashes(blockHashes []*chainhash.Hash,
	filterType wire.FilterType) ([][]byte, error) {
	return idx.entriesByBlockHashes(scriptFilterHashKey, filterType, blockHashes)
}

// NewScriptFilterIndex returns a new instance of an indexer that is used to
// create a mapping of the hashes of all blocks in the blockchain to their
// script filters, filter hashes and filter headers.
func NewScriptFilterIndex(db database.DB, chainParams *chaincfg.Params) *ScriptFilterIndex {
	return &ScriptFilterIndex{db: db, chainParams: chainParams}
}

// DropScriptFilterIndex drops the script filter index from the provided
// database if it exists.
func DropScriptFilterIndex(db database.DB, interrupt <-chan struct{}) error {
	return dropIndex(db, scriptFilterIndexParentBucketKey, scriptFilterIndexName, interrupt)
}
