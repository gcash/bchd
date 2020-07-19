// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/simpleledgerinc/goslp"
	"github.com/simpleledgerinc/goslp/v1parser"
)

const (
	// slpIndexName is the human-readable name for the index.
	slpIndexName = "slp index"
)

var (
	// slpIndexKey is the key of the transaction index and the db bucket used
	// to house it.
	slpIndexKey = []byte("slptxbyhashidx")

	// tokenIDByHashIndexBucketName is the name of the db bucket used to house
	// the block id -> block hash index.
	tokenIDByHashIndexBucketName = []byte("tokenidbyhashidx")

	// tokenHashByIDIndexBucketName is the name of the db bucket used to house
	// the block hash -> block id index.
	tokenHashByIDIndexBucketName = []byte("tokenhashbyididx")

	// errNoBlockIDEntry is an error that indicates a requested entry does
	// not exist in the block ID index.
	errNoTokenIDEntry = errors.New("no entry in the Token ID index")
)

// -----------------------------------------------------------------------------
// The slp index consists of an entry for every SLP-like transaction in the main
// chain.  In order to significantly optimize the space requirements a separate
// index which provides an internal mapping between each TokenID that has been
// indexed and a unique ID for use within the hash to location mappings.  The ID
// is simply a sequentially incremented uint32.  This is useful because it is
// only 4 bytes versus 32 bytes hashes and thus saves a ton of space in the
// index.
//
// There are three buckets used in total.  The first bucket maps the TokenID
// hash to the specific uint32 ID location.  The second bucket maps the
// uint32 of each TokenID to the actual TokenID hash and the third maps that
// unique uint32 ID back to the TokenID hash.
//
//
// The serialized format for keys and values in the TokenID hash to ID bucket is:
//   <hash> = <ID>
//
//   Field           Type              Size
//   TokenID hash    chainhash.Hash    32 bytes
//   ID              uint32            4 bytes
//   -----
//   Total: 36 bytes
//
// The serialized format for keys and values in the ID to TokenID hash bucket is:
//   <ID> = <hash>
//
//   Field           Type              Size
//   ID              uint32            4 bytes
//   TokenID hash    chainhash.Hash    32 bytes
//   -----
//   Total: 36 bytes
//
// The serialized format for the keys and values in the slp index bucket is:
//
//   <txhash> = <token id><slp type flags>
//
//   Field           	Type              Size
//   txhash          	chainhash.Hash    32 bytes
//   token ID        	uint32            4 bytes
//   slp version	    uint8             1 byte
//	 op return			[]bytes			  typically <220 bytes
//   -----
//   Max: 257 bytes
// -----------------------------------------------------------------------------

// dbPutTokenIDIndexEntry uses an existing database transaction to update or add
// the index entries for the hash to id and id to hash mappings for the provided
// values.
func dbPutTokenIDIndexEntry(dbTx database.Tx, hash *chainhash.Hash, id uint32) error {
	// Serialize the height for use in the index entries.
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)

	// Add the token hash to token ID mapping to the index.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(tokenIDByHashIndexBucketName)
	if err := hashIndex.Put(hash[:], serializedID[:]); err != nil {
		return err
	}

	// Add the token ID to token hash mapping to the index.
	idIndex := meta.Bucket(tokenHashByIDIndexBucketName)
	return idIndex.Put(serializedID[:], hash[:])
}

// dbRemoveTokenIDIndexEntry uses an existing database transaction remove index
// entries from the hash to id and id to hash mappings for the provided hash.
func dbRemoveTokenIDIndexEntry(dbTx database.Tx, hash *chainhash.Hash) error {
	// Remove the block hash to ID mapping.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(tokenIDByHashIndexBucketName)
	serializedID := hashIndex.Get(hash[:])
	if serializedID == nil {
		return nil
	}
	if err := hashIndex.Delete(hash[:]); err != nil {
		return err
	}

	// Remove the block ID to hash mapping.
	idIndex := meta.Bucket(tokenHashByIDIndexBucketName)
	return idIndex.Delete(serializedID)
}

// dbFetchTokenIDByHash uses an existing database transaction to retrieve the
// block id for the provided hash from the index.
func dbFetchTokenIDByHash(dbTx database.Tx, hash *chainhash.Hash) (uint32, error) {
	hashIndex := dbTx.Metadata().Bucket(tokenIDByHashIndexBucketName)
	serializedID := hashIndex.Get(hash[:])
	if serializedID == nil {
		return 0, errNoBlockIDEntry
	}

	return byteOrder.Uint32(serializedID), nil
}

// dbFetchTokenHashBySerializedID uses an existing database transaction to
// retrieve the hash for the provided serialized block id from the index.
func dbFetchTokenHashBySerializedID(dbTx database.Tx, serializedID []byte) (*chainhash.Hash, error) {
	idIndex := dbTx.Metadata().Bucket(tokenHashByIDIndexBucketName)
	hashBytes := idIndex.Get(serializedID)
	if hashBytes == nil {
		return nil, errNoBlockIDEntry
	}

	var hash chainhash.Hash
	copy(hash[:], hashBytes)
	return &hash, nil
}

// dbFetchBlockHashByID uses an existing database transaction to retrieve the
// hash for the provided block id from the index.
func dbFetchTokenHashByID(dbTx database.Tx, id uint32) (*chainhash.Hash, error) {
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)
	return dbFetchTokenHashBySerializedID(dbTx, serializedID[:])
}

// putSlpIndexEntry serializes the provided values according to the format
// described about for a transaction index entry.  The target byte slice must
// be at least large enough to handle the number of bytes defined by the
// slpTxEntrySize constant or it will panic.
// func putSlpIndexEntry(target []byte, tokenID uint32, slpVersion uint8, slpMsgPkScript []byte) {
// 	byteOrder.PutUint32(target, tokenID)
// 	target[5] = slpVersion
// 	copy(target[6:], slpMsgPkScript)
// }

// dbPutSlpIndexEntry uses an existing database transaction to update the
// transaction index given the provided serialized data that is expected to have
// been serialized putSlpIndexEntry.
func dbPutSlpIndexEntry(idx *SlpIndex, dbTx database.Tx, txHash *chainhash.Hash, tokenIDHash *chainhash.Hash, slpVersion uint8, slpMsgPkScript []byte) error {

	// get current tokenID uint32 for the tokenID hash, add new if needed
	tokenID, err := dbFetchTokenIDByHash(dbTx, tokenIDHash)
	if err != nil {
		idx.curTokenID++
		tokenID = idx.curTokenID
		dbPutTokenIDIndexEntry(dbTx, tokenIDHash, tokenID)
	}

	target := make([]byte, 4+1+len(slpMsgPkScript))
	byteOrder.PutUint32(target[:], tokenID)
	target[5] = slpVersion
	copy(target[6:], slpMsgPkScript)
	slpIndex := dbTx.Metadata().Bucket(slpIndexKey)
	return slpIndex.Put(txHash[:], target)
}

// SlpIndexEntry is a valid SLP token stored in the SLP index
type SlpIndexEntry struct {
	tokenID        chainhash.Hash
	slpVersionType uint8
	slpOpReturn    []byte
}

// dbFetchSlpIndexEntry uses an existing database transaction to fetch the serialized slp
// index entry for the provided transaction hash.  When there is no entry for the provided hash,
// nil will be returned for the both the entry and the error.
func dbFetchSlpIndexEntry(dbTx database.Tx, txHash *chainhash.Hash, entry *SlpIndexEntry) error {
	// Load the record from the database and return now if it doesn't exist.
	SlpIndex := dbTx.Metadata().Bucket(slpIndexKey)
	serializedData := SlpIndex.Get(txHash[:])
	if len(serializedData) == 0 {
		return nil
	}

	// Ensure the serialized data has enough bytes to properly deserialize.
	if len(serializedData) < 12 { // TODO: get more accurate number for this (i.e., 4 + 1 + min SLP length)
		return database.Error{
			ErrorCode: database.ErrCorruption,
			Description: fmt.Sprintf("corrupt slp index "+
				"entry for %s", txHash),
		}
	}

	_tokenID, _ := dbFetchTokenHashByID(dbTx, binary.BigEndian.Uint32(serializedData[0:4]))
	if _tokenID == nil {
		return nil
	}
	entry.tokenID = *_tokenID
	entry.slpVersionType = uint8(serializedData[5])
	entry.slpOpReturn = serializedData[6:]

	return nil
}

// dbRemoveSlpIndexEntry uses an existing database transaction to remove the most
// recent transaction index entry for the given hash.
func dbRemoveSlpIndexEntry(dbTx database.Tx, txHash *chainhash.Hash) error {
	slpIndex := dbTx.Metadata().Bucket(slpIndexKey)
	serializedData := slpIndex.Get(txHash[:])
	if len(serializedData) == 0 {
		return fmt.Errorf("can't remove non-existent transaction %s "+
			"from the slp index", txHash)
	}

	return slpIndex.Delete(txHash[:])
}

// dbRemoveSlpIndexEntries uses an existing database transaction to remove the
// latest transaction entry for every transaction in the passed block.
func dbRemoveSlpIndexEntries(dbTx database.Tx, block *bchutil.Block) error {
	for _, tx := range block.Transactions() {
		err := dbRemoveSlpIndexEntry(dbTx, tx.Hash())
		if err != nil {
			return err
		}
	}

	return nil
}

// SlpIndex implements a transaction by hash index.  That is to say, it supports
// querying all transactions by their hash.
type SlpIndex struct {
	db database.DB
	//curBlockID uint32
	curTokenID uint32
}

// Ensure the SlpIndex type implements the Indexer interface.
var _ Indexer = (*SlpIndex)(nil)

// Init initializes the hash-based transaction index.  In particular, it finds
// the highest used block ID and stores it for later use when connecting or
// disconnecting blocks.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) Init() error {
	// Find the latest known block id field for the internal block id
	// index and initialize it.  This is done because it's a lot more
	// efficient to do a single search at initialize time than it is to
	// write another value to the database on every update.
	err := idx.db.View(func(dbTx database.Tx) error {
		// Scan forward in large gaps to find a block id that doesn't
		// exist yet to serve as an upper bound for the binary search
		// below.
		var highestKnown, nextUnknown uint32
		testTokenID := uint32(1)
		increment := uint32(100000)
		for {
			_, err := dbFetchTokenHashByID(dbTx, testTokenID)
			if err != nil {
				nextUnknown = testTokenID
				break
			}

			highestKnown = testTokenID
			testTokenID += increment
		}
		log.Tracef("Forward scan (highest known %d, next unknown %d)",
			highestKnown, nextUnknown)

		// No used block IDs due to new database.
		if nextUnknown == 1 {
			return nil
		}

		// Use a binary search to find the final highest used block id.
		// This will take at most ceil(log_2(increment)) attempts.
		for {
			testTokenID = (highestKnown + nextUnknown) / 2
			_, err := dbFetchBlockHashByID(dbTx, testTokenID)
			if err != nil {
				nextUnknown = testTokenID
			} else {
				highestKnown = testTokenID
			}
			log.Tracef("Binary scan (highest known %d, next "+
				"unknown %d)", highestKnown, nextUnknown)
			if highestKnown+1 == nextUnknown {
				break
			}
		}

		idx.curTokenID = highestKnown
		return nil
	})
	if err != nil {
		return err
	}

	log.Debugf("Current number of SLP tokens in index: %d", idx.curTokenID)
	return nil
}

// Migrate is only provided to satisfy the Indexer interface as there is nothing to
// migrate this index.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) Migrate(db database.DB, interrupt <-chan struct{}) error {
	// Nothing to do.
	return nil
}

// Key returns the database key to use for the index as a byte slice.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) Key() []byte {
	return slpIndexKey
}

// Name returns the human-readable name of the index.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) Name() string {
	return slpIndexName
}

// Create is invoked when the indexer manager determines the index needs
// to be created for the first time.  It creates the buckets for the hash-based
// transaction index and the internal block ID indexes.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) Create(dbTx database.Tx) error {
	meta := dbTx.Metadata()
	if _, err := meta.CreateBucket(tokenIDByHashIndexBucketName); err != nil {
		return err
	}
	if _, err := meta.CreateBucket(tokenHashByIDIndexBucketName); err != nil {
		return err
	}
	_, err := meta.CreateBucket(slpIndexKey)
	return err
}

type SlpTxOut struct {
	previousOutput  wire.OutPoint
	slpVersionType  uint8
	v1amount        uint64
	v1mintBaton     bool
	spentInBurn     bool
	invalidOpReturn bool
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain.  This indexer adds a hash-to-transaction mapping
// for every transaction in the passed block.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) ConnectBlock(dbTx database.Tx, block *bchutil.Block,
	stxos []blockchain.SpentTxOut) error {

	sortedTxns := topoSortTxs(block)

	txidSet := make(map[chainhash.Hash]struct{})

	for _, tx := range sortedTxns {
		txidSet[tx.TxHash()] = struct{}{}
		_hash, _ := goslp.GetSlpTokenID(tx)
		tokenIDHash, _ := chainhash.NewHash(_hash)
		slpMsg, _ := v1parser.ParseSLP(tx.TxOut[0].PkScript)

		if slpMsg == nil {
			continue
		}

		var (
			v1InputAmtSpent big.Int
			v1MintBatonVout int
		)

		v1InputAmtSpent.SetUint64(0)
		v1MintBatonVout = 0

		for i, txi := range tx.TxIn {
			prevIdx := int(txi.PreviousOutPoint.Index)
			slpEntry, err := idx.GetSlpIndexEntry(dbTx, &txi.PreviousOutPoint.Hash)
			if err != nil || slpEntry == nil {
				continue
			}

			_slpMsg, _ := v1parser.ParseSLP(slpEntry.slpOpReturn)
			if _slpMsg != nil {
				amt, _ := _slpMsg.GetVoutAmount(prevIdx)
				if tokenIDHash != nil {
					if slpMsg.TokenType == 0x41 && slpMsg.TransactionType == "GENESIS" { // checks inputs for NFT1 child GENESIS
						if _slpMsg.TokenType == 0x81 && i == 0 {
							v1InputAmtSpent.Add(&v1InputAmtSpent, amt)
						}
					} else if slpEntry.tokenID.Compare(tokenIDHash) == 0 { // checks SEND/MINT inputs
						v1InputAmtSpent.Add(&v1InputAmtSpent, amt)
						if _slpMsg.TransactionType == "GENESIS" { // check
							if prevIdx == _slpMsg.Data.(v1parser.SlpGenesis).MintBatonVout {
								v1MintBatonVout = prevIdx
							}
						} else if _slpMsg.TransactionType == "MINT" {
							if prevIdx == _slpMsg.Data.(v1parser.SlpMint).MintBatonVout {
								v1MintBatonVout = prevIdx
							}
						}
					} else {
						// TODO: check for burns and mark them somewhere...
					}
				} else {
					// TODO: check for burns and mark them somewhere...
				}
			}

		}

		// Check if tx is a valid SLP.  This requires only two things, first
		// the slpMsg must be valid, and second the input requirements for the
		// type of transaction must be satisfied.
		isValid := false
		outputAmt, _ := slpMsg.TotalSlpMsgOutputValue()
		if slpMsg.TransactionType == "GENESIS" {
			if slpMsg.TokenType == 0x04 &&
				big.NewInt(1).Cmp(&v1InputAmtSpent) < 1 {
				isValid = true
			} else {
				isValid = true
			}
		} else if slpMsg.TransactionType == "SEND" &&
			outputAmt.Cmp(&v1InputAmtSpent) < 1 {
			isValid = true
		} else if slpMsg.TransactionType == "MINT" &&
			v1MintBatonVout > 0 {
			isValid = true
		}

		if isValid {
			hash := tx.TxHash()
			dbPutSlpIndexEntry(idx, dbTx, &hash, tokenIDHash, uint8(slpMsg.TokenType), tx.TxOut[0].PkScript)
		}
	}

	return nil
}

// DisconnectBlock is invoked by the index manager when a block has been
// disconnected from the main chain.  This indexer removes the
// hash-to-transaction mapping for every transaction in the block.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) DisconnectBlock(dbTx database.Tx, block *bchutil.Block,
	stxos []blockchain.SpentTxOut) error {

	// Remove all of the transactions in the block from the index.
	if err := dbRemoveSlpIndexEntries(dbTx, block); err != nil {
		return err
	}

	// Remove the block ID index entry for the block being disconnected and
	// decrement the current internal block ID to account for it.
	if err := dbRemoveTokenIDIndexEntry(dbTx, block.Hash()); err != nil {
		return err
	}
	idx.curTokenID--
	return nil
}

// GetSlpIndexEntry returns a serialized slp index entry for the provided transaction hash
// from the slp index.  The slp index entry can in turn be used to quickly discover
// additional slp information about the transaction. When there is no entry for the provided hash, nil
// will be returned for the both the entry and the error, which would mean the transaction is invalid
//
// This function is safe for concurrent access.
func (idx *SlpIndex) GetSlpIndexEntry(dbTx database.Tx, hash *chainhash.Hash) (*SlpIndexEntry, error) {
	var slpEntry *SlpIndexEntry
	err := dbFetchSlpIndexEntry(dbTx, hash, slpEntry)
	return slpEntry, err
}

// NewSlpIndex returns a new instance of an indexer that is used to create a
// mapping of the hashes of all transactions in the blockchain to the respective
// block, location within the block, and size of the transaction.
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
func NewSlpIndex(db database.DB) *SlpIndex {
	return &SlpIndex{db: db}
}

// dropBlockIDIndex drops the internal block id index.
func dropTokenIDIndex(db database.DB) error {
	return db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		err := meta.DeleteBucket(tokenIDByHashIndexBucketName)
		if err != nil {
			return err
		}

		return meta.DeleteBucket(tokenHashByIDIndexBucketName)
	})
}

// DropSlpIndex drops the transaction index from the provided database if it
// exists.  Since the address index relies on it, the address index will also be
// dropped when it exists.
func DropSlpIndex(db database.DB, interrupt <-chan struct{}) error {
	err := dropIndex(db, slpIndexKey, slpIndexName, interrupt)
	if err != nil {
		return err
	}

	return dropIndex(db, slpIndexKey, slpIndexName, interrupt)
}

// topoSortTxs sorts a block into topological order.
// That is, the child transactions come after parents.
func topoSortTxs(block *bchutil.Block) []*wire.MsgTx {

	sorted := make([]*wire.MsgTx, 0, len(block.Transactions()))
	txids := make(map[chainhash.Hash]struct{})
	outpoints := make(map[wire.OutPoint]struct{})

	for _, tx := range block.Transactions() {
		for i := range tx.MsgTx().TxOut {
			op := wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(i),
			}
			outpoints[op] = struct{}{}
		}
	}

	for len(sorted) < len(block.Transactions()) {
		for _, tx := range block.Transactions() {
			if _, ok := txids[*tx.Hash()]; ok {
				continue
			}
			foundParent := false
			for _, in := range tx.MsgTx().TxIn {
				if _, ok := outpoints[in.PreviousOutPoint]; ok {
					foundParent = true
					break
				}
			}
			if !foundParent {
				sorted = append(sorted, tx.MsgTx())
				for i := range tx.MsgTx().TxOut {
					op := wire.OutPoint{
						Hash:  *tx.Hash(),
						Index: uint32(i),
					}
					delete(outpoints, op)
				}
				txids[*tx.Hash()] = struct{}{}
			}
		}
	}
	return sorted
}

func removeDups(txs []*wire.MsgTx) []*wire.MsgTx {
	keys := make(map[*wire.MsgTx]bool)
	var ret []*wire.MsgTx
	for _, tx := range txs {
		if _, ok := keys[tx]; !ok {
			keys[tx] = true
			ret = append(ret, tx)
		}
	}
	return ret
}
