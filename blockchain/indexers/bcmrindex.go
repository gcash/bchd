// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// The BCMR index keeps the authchain of every token category, as CHIP-BCMR
// defines it: the category's authbase, the transaction whose ID is the
// category ID, and then each transaction that spends output 0, the identity
// output, of the one before, up to the authhead, whose identity output is
// unspent.  It keeps each link's BCMR publication too, so that the newest
// one can be found without reading the chain.
//
// It keeps every authchain that holds a publication as well, from where it
// begins.  Nothing on chain says which identity a publication is for: a
// registry names its own only in its JSON, and its authbase can be the
// publishing transaction or any transaction behind it, one whose identity
// output that spends, or one behind that in turn.  So a transaction with a
// publication that isn't a link starts an authchain of its own, and the
// walker, in the background, adds every transaction behind each publication
// that the index doesn't track yet, unless there are more than bcmrMaxFound.
// The index answers for any link it tracks as an authbase.
//
// Authchains share links.  A transaction that spends two identity outputs
// merges the identities, and both carry on through it.  A genesis that
// creates several categories, each by spending the identity output of the
// category's authbase, is the link after the authbase on all of their
// authchains.  And a genesis that spends the identity output of another
// identity's authhead makes that authhead the new category's authbase, so
// the two authchains are the same from there on.  The index therefore keeps
// each link once, and groups links into segments, runs of links between the
// points where authchains join, or where one meets a transaction the index
// doesn't track.  An identity starts at a place in a segment, and its
// authchain runs through the segments that segment continues into, up to the
// current one, which holds the authhead.  A new link updates a single
// segment, however many identities share it.
//
// Every change a block makes is recorded with the value it replaced, in an
// undo record for the block, so that disconnecting the block puts the index
// back as it was.  The walker's changes are recorded the same way, after
// those of the block that is the index's tip when it writes them, so that
// disconnecting that block undoes them first.  The index keeps the undo
// records of the latest BcmrUndoDepth blocks only.

const (
	// bcmrIndexName is the human-readable name of the index.
	bcmrIndexName = "BCMR index"

	// bcmrIndexVersion is the version of the index's records.
	bcmrIndexVersion = 1

	// bcmrMaxPathSegments bounds the segments an identity's authchain runs
	// through when it is read, which only a corrupt index could exceed.
	bcmrMaxPathSegments = 1 << 24
)

var (
	// bcmrIndexKey is the key of the BCMR index and the name of the
	// database bucket that holds it.
	bcmrIndexKey = []byte("bcmridx")

	// bcmrNextSegmentKey is the key, in the index's bucket, of the ID that
	// the next segment gets.
	bcmrNextSegmentKey = []byte("nextsegment")

	// bcmrUndoFloorKey is the key, in the index's bucket, of the lowest
	// height whose block the index can disconnect, above the undo records it
	// pruned.
	bcmrUndoFloorKey = []byte("undofloor")
)

// BcmrUndoDepth is the number of the latest blocks whose undo records the
// index keeps, the deepest reorganization it can follow.  bchd rejects a
// reorganization below the blocks it finalized, --maxreorgdepth deep, 10 by
// default, so it refuses a deeper --maxreorgdepth with the index.  A deeper
// reorganization while finalization is off, or before it finalized a block,
// fails and asks for the index to be rebuilt.
const BcmrUndoDepth = 1000

// bcmrUndoDepth is BcmrUndoDepth, which tests lower.
var bcmrUndoDepth int32 = BcmrUndoDepth

// The buckets of the index, numbered as undo records name them.  All but
// bcmrTop are inside bcmrTop, the bucket named bcmrIndexKey.
const (
	// bcmrLinks maps the txid of each link to a bcmrLink.
	bcmrLinks byte = iota

	// bcmrSegments maps each segment's ID to a bcmrSegment.
	bcmrSegments

	// bcmrIdentities maps each identity's authbase to a bcmrIdentity.
	bcmrIdentities

	// bcmrHeads maps the authhead of each current segment whose identity
	// output can still be spent to the segment's ID.
	bcmrHeads

	// bcmrPending maps the key, by bcmrPendingKey, of each segment that
	// the walker is to walk behind, for a publication in it, to a byte
	// of 1.
	bcmrPending

	// bcmrUndo maps the height of each of the latest blocks that changed
	// the index to the block's undo record, and that height followed by a
	// sequence number to the undo record of each write of the walker while
	// the block was the index's tip.
	bcmrUndo

	// bcmrTop is the index's own bucket.
	bcmrTop

	bcmrNumBuckets
)

// bcmrBucketNames are the names of the buckets inside the index's bucket.
var bcmrBucketNames = [bcmrTop][]byte{
	bcmrLinks:      []byte("links"),
	bcmrSegments:   []byte("segments"),
	bcmrIdentities: []byte("identities"),
	bcmrHeads:      []byte("heads"),
	bcmrPending:    []byte("pending"),
	bcmrUndo:       []byte("undo"),
}

// bcmrChain is the part of the chain the index reads: the main chain's block
// at a height, the outputs a block spends, which the index reads while it
// catches up and the walker reads for the heights of the transactions behind
// a publication, and for a query the unspent output that a genesis in the
// mempool spends.  The index manager hands the index the chain before it
// initializes it.
type bcmrChain interface {
	BlockHashByHeight(height int32) (*chainhash.Hash, error)
	FetchSpendJournal(block *bchutil.Block) ([]blockchain.SpentTxOut, error)
	FetchUtxoEntry(outpoint wire.OutPoint) (*blockchain.UtxoEntry, error)
}

// BcmrIndex implements an index of the authchains of all token categories,
// and of all that hold a BCMR publication, with their publications, as
// CHIP-BCMR defines them.  For each authchain it knows every link, with where
// it is in the block files and its publication, the authhead, and whether the
// identity was burned.
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
type BcmrIndex struct {
	db          database.DB
	chainParams *chaincfg.Params
	startHash   *chainhash.Hash
	startHeight int32

	// chain is the chain the index reads authbases' heights and blocks
	// from.
	chain bcmrChain

	// mtx protects heads.
	mtx sync.Mutex

	// heads holds what the bucket bcmrHeads holds, so that connecting a
	// block finds the inputs that spend an identity output without reading
	// the database for each input.
	heads map[chainhash.Hash]uint64

	// walkSignal wakes the walker, quit stops it, and wg waits for it.
	walkSignal chan struct{}
	quit       chan struct{}
	wg         sync.WaitGroup
}

// Ensure the BcmrIndex type implements the Indexer and ChainUser interfaces.
var (
	_ Indexer   = (*BcmrIndex)(nil)
	_ ChainUser = (*BcmrIndex)(nil)
)

// NewBcmrIndex returns a new BCMR index for the chain with the passed
// parameters.
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
func NewBcmrIndex(db database.DB, chainParams *chaincfg.Params) *BcmrIndex {
	idx := &BcmrIndex{
		db:          db,
		chainParams: chainParams,
		startHeight: -1,
		heads:       make(map[chainhash.Hash]uint64),
		walkSignal:  make(chan struct{}, 1),
		quit:        make(chan struct{}),
	}

	// No token can be created in a block before CashTokens activated, so
	// the index starts at the last checkpoint at or before that, whose hash
	// the chain is known to have.
	for _, checkpoint := range chainParams.Checkpoints {
		if checkpoint.Height <= chainParams.Upgrade9ForkHeight &&
			checkpoint.Height > idx.startHeight {

			idx.startHash = checkpoint.Hash
			idx.startHeight = checkpoint.Height
		}
	}
	return idx
}

// Key returns the database key to use for the index as a byte slice.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) Key() []byte {
	return bcmrIndexKey
}

// Name returns the human-readable name of the index.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) Name() string {
	return bcmrIndexName
}

// Create is invoked when the indexer manager determines the index needs to be
// created for the first time.  It creates the index's buckets and stores its
// version.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) Create(dbTx database.Tx) error {
	top, err := dbTx.Metadata().CreateBucket(bcmrIndexKey)
	if err != nil {
		return err
	}
	for _, name := range bcmrBucketNames {
		if _, err := top.CreateBucket(name); err != nil {
			return err
		}
	}
	return dbPutIndexVersion(dbTx, bcmrIndexKey, bcmrIndexVersion)
}

// Init loads the authheads whose identity outputs the index watches, unless
// the index is of an earlier version, which Migrate rebuilds, and refuses an
// index that a later bchd built.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) Init() error {
	version, err := checkIndexVersion(idx.db, bcmrIndexKey, bcmrIndexName,
		"dropbcmrindex", bcmrIndexVersion)
	if err != nil || version < bcmrIndexVersion {
		return err
	}

	idx.mtx.Lock()
	defer idx.mtx.Unlock()

	heads := make(map[chainhash.Hash]uint64)
	err = idx.db.View(func(dbTx database.Tx) error {
		w, err := newBcmrWriter(dbTx)
		if err != nil {
			return err
		}
		return w.buckets[bcmrHeads].ForEach(func(k, v []byte) error {
			if len(k) != chainhash.HashSize || len(v) != 8 {
				return errDeserialize("BCMR head has the wrong size")
			}
			var head chainhash.Hash
			copy(head[:], k)
			heads[head] = binary.BigEndian.Uint64(v)
			return nil
		})
	})
	if err != nil {
		return err
	}
	idx.heads = heads
	return nil
}

// Migrate rebuilds an index of an earlier version, or one that stores no
// version, from the index's start block.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) Migrate(db database.DB, interrupt <-chan struct{}) error {
	var version uint32
	err := db.View(func(dbTx database.Tx) error {
		var err error
		version, err = dbFetchIndexVersion(dbTx, bcmrIndexKey)
		return err
	})
	if err != nil || version >= bcmrIndexVersion {
		return err
	}

	log.Infof("The %s was built by an earlier version and will be rebuilt "+
		"from scratch", bcmrIndexName)
	if err := dropIndex(db, bcmrIndexKey, bcmrIndexName, interrupt); err != nil {
		return err
	}
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		hash, height := idx.StartBlock()
		if hash == nil {
			hash, height = &chainhash.Hash{}, -1
		}
		return dbPutIndexerTip(dbTx, bcmrIndexKey, hash, height)
	})
	if err != nil {
		return err
	}
	idx.mtx.Lock()
	idx.heads = make(map[chainhash.Hash]uint64)
	idx.mtx.Unlock()
	return nil
}

// StartBlock returns the last checkpoint at or before the block after which
// CashTokens activated, as the index needs no block before that.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) StartBlock() (*chainhash.Hash, int32) {
	return idx.startHash, idx.startHeight
}

// UseChain hands the index the chain, which it reads the height and block of
// an authbase from, and, while it catches up, the outputs a block spends.  The
// index's walker takes the chain's notifications of the blocks it connects
// and disconnects.
//
// This is part of the ChainUser interface.
func (idx *BcmrIndex) UseChain(chain *blockchain.BlockChain) {
	idx.chain = chain
	chain.Subscribe(idx.handleChainNotification)
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain.  It adds the links of the block to the
// authchains they extend, the authchains of the token categories the block
// creates, and those its publications start.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) ConnectBlock(dbTx database.Tx, block *bchutil.Block,
	stxos []blockchain.SpentTxOut) error {

	idx.mtx.Lock()
	defer idx.mtx.Unlock()

	if err := pruneBcmrUndo(dbTx, block.Height()); err != nil {
		return err
	}

	// A block before CashTokens activated can create no token, and only
	// extends an authchain the index already tracks.
	tokensActive := block.Height() > idx.chainParams.Upgrade9ForkHeight
	if !idx.blockMatters(block, tokensActive) {
		return nil
	}

	w, err := newBcmrWriter(dbTx)
	if err != nil {
		return err
	}
	c := &bcmrConnector{
		idx:     idx,
		dbTx:    dbTx,
		w:       w,
		block:   block,
		stxos:   stxos,
		added:   make(map[chainhash.Hash]uint64),
		removed: make(map[chainhash.Hash]struct{}),
	}
	txs := block.Transactions()
	for _, i := range bcmrTxOrder(txs) {
		if err := c.connectTx(i, tokensActive); err != nil {
			return err
		}
	}
	return c.finish()
}

// blockMatters returns whether the block extends an authchain the index
// tracks, creates a token category or holds a publication.  A transaction that
// spends the identity output of a transaction in the same block extends an
// authchain only if that transaction does, so this needs no order of the
// block's transactions.
func (idx *BcmrIndex) blockMatters(block *bchutil.Block, tokensActive bool) bool {
	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()
		if bcmr.ParsePublication(msgTx) != nil {
			return true
		}
		for _, in := range msgTx.TxIn {
			if in.PreviousOutPoint.Index != 0 {
				continue
			}
			if _, ok := idx.heads[in.PreviousOutPoint.Hash]; ok {
				return true
			}
		}
		if tokensActive && len(bcmr.Geneses(msgTx)) > 0 {
			return true
		}
	}
	return false
}

// DisconnectBlock is invoked by the index manager when a block has been
// disconnected from the main chain.  It puts back every entry of the index
// that the block changed, and that the walker changed while the block was the
// index's tip, from their undo records.
//
// This is part of the Indexer interface.
func (idx *BcmrIndex) DisconnectBlock(dbTx database.Tx, block *bchutil.Block,
	_ []blockchain.SpentTxOut) error {

	idx.mtx.Lock()
	defer idx.mtx.Unlock()

	w, err := newBcmrWriter(dbTx)
	if err != nil {
		return err
	}

	// Below the floor the index pruned the block's undo records, though
	// the walker can have written some since at that tip.
	floor := w.get(bcmrTop, bcmrUndoFloorKey)
	if floor != nil && block.Height() < int32(byteOrder.Uint32(floor)) {
		return fmt.Errorf("the %s can't disconnect block %v at height "+
			"%d, deeper than the latest %d blocks it keeps undo "+
			"records for; drop it with --dropbcmrindex, and it "+
			"rebuilds", bcmrIndexName, block.Hash(), block.Height(),
			bcmrUndoDepth)
	}
	keys, entries, err := bcmrUndoRecords(w, block.Height())
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		bucket := w.buckets[e.bucket]
		if e.hadOld {
			err = bucket.Put(e.key, e.old)
		} else {
			err = bucket.Delete(e.key)
		}
		if err != nil {
			return err
		}
	}
	for _, key := range keys {
		if err := w.buckets[bcmrUndo].Delete(key); err != nil {
			return err
		}
	}

	// The heads in memory take the values the bucket was put back to.
	for _, e := range entries {
		if e.bucket != bcmrHeads {
			continue
		}
		var head chainhash.Hash
		copy(head[:], e.key)
		if v := w.get(bcmrHeads, e.key); v != nil {
			idx.heads[head] = binary.BigEndian.Uint64(v)
		} else {
			delete(idx.heads, head)
		}
	}
	return nil
}

// bcmrUndoRecords returns the keys of the undo records of the block at a
// height and of the walker's writes while that block was the index's tip, and
// their entries, in the order they were written.
func bcmrUndoRecords(w *bcmrWriter, height int32) ([][]byte, []bcmrUndoEntry, error) {
	keys := bcmrUndoKeys(w.buckets[bcmrUndo], height)
	var entries []bcmrUndoEntry
	for _, key := range keys {
		record, err := deserializeBcmrUndo(w.get(bcmrUndo, key))
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, record...)
	}
	return keys, entries, nil
}

// bcmrUndoKeys returns the keys of the undo records of the block at a height
// and of the walker's writes while that block was the index's tip, in the
// order they were written: the key of the block's sorts before those that
// extend it with a sequence number.
func bcmrUndoKeys(undo database.Bucket, height int32) [][]byte {
	prefix := bcmrUndoKey(height)
	var keys [][]byte
	cursor := undo.Cursor()
	for ok := cursor.Seek(prefix); ok && bytes.HasPrefix(cursor.Key(), prefix); ok = cursor.Next() {
		keys = append(keys, bytes.Clone(cursor.Key()))
	}
	return keys
}

// nextBcmrWalkUndoKey returns the key of the undo record of the walker's next
// write while the block at a height is the index's tip, after those it has.
func nextBcmrWalkUndoKey(undo database.Bucket, height int32) []byte {
	// The last of them is right before the key of the highest sequence.
	cursor := undo.Cursor()
	var ok bool
	if cursor.Seek(bcmrWalkUndoKey(height, math.MaxUint32)) {
		ok = cursor.Prev()
	} else {
		ok = cursor.Last()
	}
	first := bcmrWalkUndoKey(height, 1)
	seq := uint32(1)
	if key := cursor.Key(); ok && len(key) == len(first) &&
		bytes.HasPrefix(key, first[:len(first)-4]) {

		seq = binary.BigEndian.Uint32(key[len(key)-4:]) + 1
	}
	return bcmrWalkUndoKey(height, seq)
}

// pruneBcmrUndo removes the undo records of the block bcmrUndoDepth below the
// one being connected at height and of the walker's writes while it was the
// tip, if there are any, and raises the floor above it.
func pruneBcmrUndo(dbTx database.Tx, height int32) error {
	pruned := height - bcmrUndoDepth
	if pruned < 0 {
		return nil
	}
	top := dbTx.Metadata().Bucket(bcmrIndexKey)
	undo := top.Bucket(bcmrBucketNames[bcmrUndo])
	keys := bcmrUndoKeys(undo, pruned)
	if len(keys) == 0 {
		return nil
	}
	for _, key := range keys {
		if err := undo.Delete(key); err != nil {
			return err
		}
	}
	floor := make([]byte, 4)
	byteOrder.PutUint32(floor, uint32(pruned+1))
	return top.Put(bcmrUndoFloorKey, floor)
}

// bcmrIdentityState is what the index knows of an identity.
type bcmrIdentityState struct {
	// identity is the token's identity, with its genesis supply, when the
	// authbase is a token category's, and nil otherwise: any other link the
	// index tracks is the authbase of an identity without a genesis supply.
	identity *bcmrIdentity

	// place is the authbase's place in the first segment of path.
	place uint32

	// path is the segments the identity's authchain runs through, from
	// the one that holds its authbase to the current one, and lengths
	// their lengths.
	path    []uint64
	lengths []uint32

	// head is the authhead, and burned reports that it burned the
	// identity.
	head   chainhash.Hash
	burned bool

	// pubLink is the newest link of the authchain with a well-formed
	// publication, or the zero hash if none has one.
	pubLink chainhash.Hash
}

// fetchBcmrIdentityState returns what the index knows of the identity with
// the passed authbase, a token's category ID or any other link the index
// tracks, or nil if it tracks no such link.
func fetchBcmrIdentityState(dbTx database.Tx, authbase *chainhash.Hash) (*bcmrIdentityState, error) {
	w, err := newBcmrWriter(dbTx)
	if err != nil {
		return nil, err
	}
	state := &bcmrIdentityState{}
	var segmentID uint64
	if raw := w.get(bcmrIdentities, authbase[:]); raw != nil {
		id, err := deserializeBcmrIdentity(raw)
		if err != nil {
			return nil, err
		}
		state.identity = id
		segmentID, state.place = id.segment, id.place
	} else {
		raw := w.get(bcmrLinks, authbase[:])
		if raw == nil {
			return nil, nil
		}
		link, err := deserializeBcmrLink(raw)
		if err != nil {
			return nil, err
		}
		segmentID, state.place = link.segment, link.place
	}

	for {
		raw := w.get(bcmrSegments, bcmrSegmentKey(segmentID))
		if raw == nil {
			return nil, AssertError(fmt.Sprintf("BCMR segment %d is missing", segmentID))
		}
		s, err := deserializeBcmrSegment(raw)
		if err != nil {
			return nil, err
		}
		// In the segment that holds the authbase, only the links from
		// the authbase on are the identity's.
		if s.pubLink != (chainhash.Hash{}) &&
			(len(state.path) > 0 || s.pubPlace >= state.place) {

			state.pubLink = s.pubLink
		}
		state.path = append(state.path, segmentID)
		state.lengths = append(state.lengths, s.length)
		if s.next == 0 {
			state.head = s.head
			state.burned = s.burned
			return state, nil
		}
		if len(state.path) >= bcmrMaxPathSegments {
			return nil, AssertError(fmt.Sprintf("BCMR identity %v runs "+
				"through too many segments", authbase))
		}
		segmentID = s.next
	}
}

// DropBcmrIndex drops the BCMR index from the provided database if it exists.
func DropBcmrIndex(db database.DB, interrupt <-chan struct{}) error {
	return dropIndex(db, bcmrIndexKey, bcmrIndexName, interrupt)
}
