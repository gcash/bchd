// Copyright (c) 2020-2021 Simple Ledger, Inc.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/slpgraphsearch"
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
	// the token id (hash) -> token id (uint32) index.
	tokenIDByHashIndexBucketName = []byte("tokenidbyhashidx")

	// tokenMetadataByIDIndexBucketName is the name of the db bucket used to house
	// the token id (uint32) -> token id (hash) and token metadata index.
	tokenMetadataByIDIndexBucketName = []byte("tokenhashbyididx")

	// errNoTokenMetadataEntry is an error that indicates a requested entry does
	// not exist in the token metadata index.
	errNoTokenMetadataEntry = errors.New("no entry in the token metadata db")

	// errNoTokenIDHashEntry is an error that indicates a requested entry does
	// not exist in the token id by hash index.
	errNoTokenIDHashEntry = errors.New("no entry in the token id by hash db")
)

// -----------------------------------------------------------------------------
// The slp index consists of an entry for every slp-like transaction in the main
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
// DB Bucket Variable: "tokenIDByHashIndexBucketName"
// The serialized format for keys and values in the TokenID hash to ID bucket is:
//   <hash> = <ID>
//
//   Field           Type              Size
//   -----           -----             -----
//   TokenID hash    chainhash.Hash    32 bytes
//   ID              uint32            4 bytes
//
//   Total Item Size: 36 bytes
//
//
// DB Bucket Variable: "tokenMetadataByIDIndexBucketName"
// The serialized format for keys and values in the ID to TokenID hash bucket is:
//   <ID> = <token id txid><mint baton hash><uint32>
//
//
//   Field            					Type              Size
//   -----                              -----             -----
//   ID               					uint32            4 bytes
//   TokenID hash                   	chainhash.Hash    32 bytes
//   slp version	    				uint16            2 bytes
//   Mint baton hash (or nft group id)  chainhash.Hash    32 bytes (optional)
//   Mint baton vout  					uint32			  4 bytes  (optional)
//
//   Max Item Size: 74 bytes
//   Min Item Size: 36 bytes
//
//
// DB Bucket Variable: "slpIndexKey"
// The serialized format for the keys and values in the slp index bucket is:
//   <txhash> = <token ID><slp version><slp op_return>
//
//   Field           	Type              Size
//   -----				-----			  -----
//   txhash          	chainhash.Hash    32 bytes
//   token ID        	uint32            4 bytes
//   slp version	    uint16            2 bytes
//	 op_return			[]bytes			  typically <220 bytes
//   -----
//   Max: 258 bytes (if op_return is limited to 220 bytes)
//	 Min: 43 bytes (4 + 2 + 37)
//
//   NOTE: The minimum possible slp op_return is 37 bytes, this is empty genesis
//
// -----------------------------------------------------------------------------

// TokenMetadata is used to hold the unmarshalled data parsed from the Token ID index
type TokenMetadata struct {
	TokenID       *chainhash.Hash
	SlpVersion    v1parser.TokenType
	NftGroupID    *chainhash.Hash
	MintBatonHash *chainhash.Hash
	MintBatonVout uint32
}

// dbPutTokenIDIndexEntry uses an existing database transaction to update or add
// the index entries for the hash to id and id to hash mappings for the provided
// values.
func dbPutTokenIDIndexEntry(dbTx database.Tx, id uint32, metadata *TokenMetadata) error {
	// Serialize the height for use in the index entries.
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)

	// Add the token ID by token hash mapping to the index.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(tokenIDByHashIndexBucketName)
	if err := hashIndex.Put(metadata.TokenID[:], serializedID[:]); err != nil {
		return err
	}

	// Add or update token metadata by uint32 tokenID mapping to the index.
	tmIndex := meta.Bucket(tokenMetadataByIDIndexBucketName)
	tokenMetadata := make([]byte, 32+2+32+4)

	copy(tokenMetadata[0:], metadata.TokenID[:])

	byteOrder.PutUint16(tokenMetadata[32:], uint16(metadata.SlpVersion))

	if metadata.NftGroupID != nil {
		copy(tokenMetadata[34:], metadata.NftGroupID[:])
		tokenMetadata = tokenMetadata[:66]
	} else if metadata.MintBatonHash != nil {
		copy(tokenMetadata[34:], metadata.MintBatonHash[:])
		byteOrder.PutUint32(tokenMetadata[66:], metadata.MintBatonVout)
	} else {
		tokenMetadata = tokenMetadata[:34]
	}

	if metadata.NftGroupID == nil && metadata.SlpVersion == v1parser.TokenTypeNft1Child41 {
		return fmt.Errorf("missing nft group id for NFT child %v", id)
	}

	return tmIndex.Put(serializedID[:], tokenMetadata)
}

// dbFetchTokenIDByHash uses an existing database transaction to retrieve the
// token id for the provided hash from the index.
func dbFetchTokenIDByHash(dbTx database.Tx, hash *chainhash.Hash) (uint32, error) {
	hashIndex := dbTx.Metadata().Bucket(tokenIDByHashIndexBucketName)
	if hashIndex == nil {
		return 0, fmt.Errorf("bucket nil for key: %s", tokenIDByHashIndexBucketName)
	}
	serializedID := hashIndex.Get(hash[:])
	if serializedID == nil {
		return 0, errNoTokenIDHashEntry
	}
	return byteOrder.Uint32(serializedID), nil
}

// dbFetchTokenMetadataBySerializedID uses an existing database transaction to
// retrieve the hash for the provided serialized token id from the index.
func dbFetchTokenMetadataBySerializedID(dbTx database.Tx, serializedID []byte) (*TokenMetadata, error) {
	idIndex := dbTx.Metadata().Bucket(tokenMetadataByIDIndexBucketName)
	if idIndex == nil {
		return nil, fmt.Errorf("bucket nil for key: %s", tokenMetadataByIDIndexBucketName)
	}
	serializedData := idIndex.Get(serializedID)
	if serializedData == nil {
		return nil, errNoTokenMetadataEntry
	}

	tokenIDHash, err := chainhash.NewHash(serializedData[0:32])
	if err != nil {
		return nil, fmt.Errorf("failed to create hash from %s", hex.EncodeToString(serializedData[0:32]))
	}
	if len(serializedData) < 34 {
		return nil, fmt.Errorf("missing token version type for token metadata of token ID %v", tokenIDHash)
	}

	slpVersion := v1parser.TokenType(byteOrder.Uint16(serializedData[32:34]))

	var (
		mintBatonHash *chainhash.Hash
		mintBatonVout uint32
		nft1GroupID   *chainhash.Hash
	)
	if len(serializedData) == 70 {
		if slpVersion == v1parser.TokenTypeNft1Child41 {
			return nil, errors.New("cannot have this stored data length with nft1 child, drop and add slpindex")
		}
		var err error
		mintBatonHash, err = chainhash.NewHash(serializedData[34:66])
		if err != nil {
			return nil, fmt.Errorf("could not create mint baton hash with data: %s", hex.EncodeToString(serializedData[34:66]))
		}
		mintBatonVout = byteOrder.Uint32(serializedData[66:])
	} else if len(serializedData) == 66 {
		if slpVersion != v1parser.TokenTypeNft1Child41 {
			return nil, errors.New("cannot have this stored data length if not nft1 child, drop and add slpindex")
		}
		var err error
		nft1GroupID, err = chainhash.NewHash(serializedData[34:])
		if err != nil {
			return nil, fmt.Errorf("could not create nft group id hash with data: %s", hex.EncodeToString(serializedData[34:]))
		}
	}

	tm := &TokenMetadata{
		TokenID:       tokenIDHash,
		NftGroupID:    nft1GroupID,
		MintBatonHash: mintBatonHash,
		MintBatonVout: mintBatonVout,
	}
	return tm, nil
}

// dbFetchTokenMetadataByID uses an existing database transaction to retrieve the
// hash for the provided token id from the index.
func dbFetchTokenMetadataByID(dbTx database.Tx, id uint32) (*TokenMetadata, error) {
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)
	return dbFetchTokenMetadataBySerializedID(dbTx, serializedID[:])
}

type dbSlpIndexEntry struct {
	tx             *wire.MsgTx
	slpMsg         v1parser.ParseResult
	tokenIDHash    *chainhash.Hash
	slpMsgPkScript []byte
}

// dbPutSlpIndexEntry uses an existing database transaction to update the
// transaction index given the provided serialized data that is expected to have
// been serialized putSlpIndexEntry.
func dbPutSlpIndexEntry(idx *SlpIndex, dbTx database.Tx, entryInfo *dbSlpIndexEntry) error {
	txHash := entryInfo.tx.TxHash()

	// get current tokenID uint32 for the tokenID hash, add new if needed
	tokenID, err := dbFetchTokenIDByHash(dbTx, entryInfo.tokenIDHash)
	if err != nil {
		tokenID = idx.curTokenID + 1
	}

	var (
		tokenMetadataNeedsUpdated = false
		mintBatonVout             uint32
		mintBatonHash             *chainhash.Hash
		nft1GroupID               *chainhash.Hash
	)

	switch entry := entryInfo.slpMsg.(type) {
	case *v1parser.SlpGenesis:
		idx.curTokenID++
		tokenMetadataNeedsUpdated = true
		if entry.MintBatonVout > 1 {
			mintBatonVout = uint32(entry.MintBatonVout)
			mintBatonHash = &txHash
		} else if entry.TokenType() == v1parser.TokenTypeNft1Child41 {
			if len(entryInfo.tx.TxIn) < 1 {
				return errors.New("entryInfo transaction has no inputs")
			}
			groupTokenEntry, err := dbFetchSlpIndexEntry(dbTx, &entryInfo.tx.TxIn[0].PreviousOutPoint.Hash)
			if err != nil {
				return fmt.Errorf("failed to fetch nft parent token ID %v: %v", entryInfo.tx.TxIn[0].PreviousOutPoint.Hash, err)
			}
			nft1GroupID = &groupTokenEntry.TokenIDHash
		}
	case *v1parser.SlpMint:
		tokenMetadataNeedsUpdated = true
		if entry.MintBatonVout > 1 {
			mintBatonVout = uint32(entry.MintBatonVout)
			mintBatonHash = &txHash
		}
	}

	// maybe update token metadata
	if tokenMetadataNeedsUpdated {
		err = dbPutTokenIDIndexEntry(dbTx, tokenID,
			&TokenMetadata{
				TokenID:       entryInfo.tokenIDHash,
				SlpVersion:    entryInfo.slpMsg.TokenType(),
				MintBatonHash: mintBatonHash,
				MintBatonVout: mintBatonVout,
				NftGroupID:    nft1GroupID,
			})
		if err != nil {
			return fmt.Errorf("failed to update db for token id: %v, this should never happen", entryInfo.tokenIDHash)
		}
	}

	err = idx.cache.AddSlpTxEntry(&txHash, SlpTxEntry{
		TokenID:        tokenID,
		TokenIDHash:    *entryInfo.tokenIDHash,
		SlpVersionType: entryInfo.slpMsg.TokenType(),
		SlpOpReturn:    entryInfo.slpMsgPkScript,
	})
	if err != nil {
		log.Criticalf("AddSlpTxEntry in dbPutSlpIndexEntry failed: ", err)
	}

	target := make([]byte, 4+2+len(entryInfo.slpMsgPkScript))
	byteOrder.PutUint32(target[:], tokenID)
	byteOrder.PutUint16(target[4:], uint16(entryInfo.slpMsg.TokenType()))
	copy(target[6:], entryInfo.slpMsgPkScript)
	slpIndex := dbTx.Metadata().Bucket(slpIndexKey)
	if slpIndex == nil {
		return fmt.Errorf("bucket nil for key: %s", slpIndexKey)
	}
	return slpIndex.Put(txHash[:], target)
}

// SlpTxEntry is a valid slp token stored in the slp index
type SlpTxEntry struct {
	TokenID        uint32
	TokenIDHash    chainhash.Hash
	SlpVersionType v1parser.TokenType
	SlpOpReturn    []byte
}

// dbFetchSlpIndexEntry uses an existing database transaction to fetch the serialized slp
// index entry for the provided transaction hash.  When there is no entry for the provided hash,
// nil will be returned for the both the entry and the error.
func dbFetchSlpIndexEntry(dbTx database.Tx, txHash *chainhash.Hash) (*SlpTxEntry, error) {
	// Load the record from the database and return now if it doesn't exist.
	slpIndex := dbTx.Metadata().Bucket(slpIndexKey)
	if slpIndex == nil {
		return nil, fmt.Errorf("bucket nil for key: %s", slpIndexKey)
	}
	serializedData := slpIndex.Get(txHash[:])
	if len(serializedData) == 0 {
		return nil, fmt.Errorf("slp entry does not exist %v", txHash)
	}

	// Ensure the serialized data has enough bytes to properly deserialize.
	// The minimum possible entry size is 4 + 2 + 37 = 43, which is an empty GENESIS slp OP_RETURN.
	if len(serializedData) < 43 {
		return nil, database.Error{
			ErrorCode: database.ErrCorruption,
			Description: fmt.Sprintf("corrupt slp index "+
				"entry for %s", txHash),
		}
	}
	entry := &SlpTxEntry{
		TokenID: byteOrder.Uint32(serializedData[0:4]),
	}
	tokenMetadata, err := dbFetchTokenMetadataByID(dbTx, entry.TokenID)
	if err != nil {
		return nil, err
	}
	entry.TokenIDHash = *tokenMetadata.TokenID
	entry.SlpVersionType = v1parser.TokenType(byteOrder.Uint16(serializedData[4:6]))
	entry.SlpOpReturn = serializedData[6:]
	return entry, nil
}

// dbRemoveSlpIndexEntries uses an existing database transaction to remove the
// latest slp transaction entry for every transaction in the passed block.
//
// This method should only be called by DisconnectBlock()
func dbRemoveSlpIndexEntries(dbTx database.Tx, block *bchutil.Block) error {
	// toposort and reverse order so we can unwind slp token metadata state if needed
	txs := TopologicallySortTxs(block.Transactions())
	var txsRev []*wire.MsgTx
	for i := len(txs) - 1; i >= 0; i-- {
		txsRev = append(txsRev, txs[i])
	}

	// this method should only be called after a topological sort
	dbRemoveSlpIndexEntry := func(dbTx database.Tx, txHash *chainhash.Hash) error {
		slpIndex := dbTx.Metadata().Bucket(slpIndexKey)
		if slpIndex == nil {
			return fmt.Errorf("bucket nil for key: %s", slpIndexKey)
		}
		serializedData := slpIndex.Get(txHash[:])
		if len(serializedData) == 0 {
			return nil
		}

		// NOTE: We don't need to worry about updating mint baton token metadata here since it isn't
		// relied upon for the purpose of validation.  If a mint boton double spend occurs
		// then the token metadata record will be updated when ConnectBlock is called.

		return slpIndex.Delete(txHash[:])
	}

	for _, tx := range txsRev {
		hash := tx.TxHash()
		err := dbRemoveSlpIndexEntry(dbTx, &hash)
		if err != nil {
			return err
		}
	}

	return nil
}

// SlpIndex implements a transaction by hash index.  That is to say, it supports
// querying all transactions by their hash.
type SlpIndex struct {
	db            database.DB
	curTokenID    uint32
	config        *SlpConfig
	cache         *SlpCache
	graphSearchDb *slpgraphsearch.Db
}

// Ensure the SlpIndex type implements the Indexer interface.
var _ Indexer = (*SlpIndex)(nil)

// Init initializes the hash-based slp transaction index.  In particular, it finds
// the highest used Token ID and stores it for later use when a new token has been
// created.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) Init() error {
	// Find the latest known token id field for the internal token id
	// index and initialize it.  This is done because it's a lot more
	// efficient to do a single search at initialize time than it is to
	// write another value to the database on every update.
	err := idx.db.View(func(dbTx database.Tx) error {
		var highestKnown, nextUnknown uint32
		testTokenID := uint32(1)
		increment := uint32(1)
		for {
			md, err := dbFetchTokenMetadataByID(dbTx, testTokenID)
			if err != nil {
				if md != nil {
					return fmt.Errorf("could not init slp index: %v", err)
				}
				nextUnknown = testTokenID
				break
			}

			highestKnown = testTokenID
			testTokenID += increment
		}
		log.Tracef("Forward scan (highest known %d, next unknown %d)",
			highestKnown, nextUnknown)

		idx.curTokenID = highestKnown
		return nil
	})

	if err != nil {
		return err
	}

	log.Infof("Current number of slp tokens in index: %v", idx.curTokenID)
	return nil
}

// GraphSearchEnabled indicates if slp graph search is enabled
func (idx *SlpIndex) GraphSearchEnabled() bool {
	return idx.config.SlpGraphSearchEnabled
}

// StartBlock is used to indicate the proper start block for the index manager.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) StartBlock() (*chainhash.Hash, int32) {
	return idx.config.StartHash, idx.config.StartHeight
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
// transaction index and the internal token ID and token metadata indexes.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) Create(dbTx database.Tx) error {
	meta := dbTx.Metadata()
	if _, err := meta.CreateBucket(tokenIDByHashIndexBucketName); err != nil {
		return err
	}
	if _, err := meta.CreateBucket(tokenMetadataByIDIndexBucketName); err != nil {
		return err
	}
	_, err := meta.CreateBucket(slpIndexKey)
	return err
}

// BurnedInput represents a burned slp txo item
type BurnedInput struct {
	Tx      *wire.MsgTx
	TxInput *wire.TxIn
	SlpMsg  v1parser.ParseResult
	Entry   *SlpTxEntry
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain.  This indexer adds a hash-to-transaction mapping
// for every transaction in the passed block.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) ConnectBlock(dbTx database.Tx, block *bchutil.Block, stxos []blockchain.SpentTxOut) error {

	sortedTxns := TopologicallySortTxs(block.Transactions())

	getSlpIndexEntry := func(txiHash *chainhash.Hash) (*SlpTxEntry, error) {
		return idx.GetSlpIndexEntry(dbTx, txiHash)
	}

	putTxIndexEntry := func(tx *wire.MsgTx, slpMsg v1parser.ParseResult, tokenIDHash *chainhash.Hash) error {
		if len(tx.TxOut) < 1 {
			return fmt.Errorf("transaction has no outputs %v", tx.TxHash())
		}

		// Here we take special care to avoid the situation where a double-spend of an slp mint baton
		// has been mined but the SlpCache.tempTokenMetadata cache contains metadata with a current mint baton
		// outpoint location that is based the first seen mint baton spend. This scenario would not impact slp validation,
		// however it would result in the gRPC server sending the client the wrong mint baton hash until the cache
		// was cleared in a bchd restart or cleared from exceeding the SlpCache.maxEntries limit.
		//
		// The consequence of this is chained unconfirmed mint transactions may cause an outdated mint baton outpoint
		// to be returned if a chained parent is recently confirmed mint children are remaining in the mempool.
		// This situation only presents an inconvenience to the user.
		//
		// Since the mint baton outpoint location is the only stateful property of token metadata there are no other
		// similar situations with slp v1/nft1.
		if msg, ok := slpMsg.(v1parser.SlpMint); ok {
			hash, err := chainhash.NewHash(msg.TokenID())
			if err != nil {
				log.Criticalf("invalid hash for token id %s, %v", hex.EncodeToString(msg.TokenID()[:]), err)
			} else if _, ok := idx.cache.GetTokenMetadata(hash); ok {
				log.Debugf("clear token metadata cache for %s", hex.EncodeToString(msg.TokenID()[:]))
				idx.cache.RemoveTokenMetadata(*hash)
			}
		}

		return dbPutSlpIndexEntry(idx, dbTx, &dbSlpIndexEntry{
			tx:             tx,
			slpMsg:         slpMsg,
			tokenIDHash:    tokenIDHash,
			slpMsgPkScript: tx.TxOut[0].PkScript,
		})
	}

	burnedInputs := make([]*BurnedInput, 0)
	for _, tx := range sortedTxns {
		isValid, txnInputsBurned, err := CheckSlpTx(tx, getSlpIndexEntry, putTxIndexEntry)
		if err != nil {
			log.Critical(err)
			return err
		}

		// look for burned inputs within non-slp txns
		if !isValid {
			for _, txi := range tx.TxIn {
				slpEntry, err := idx.GetSlpIndexEntry(dbTx, &txi.PreviousOutPoint.Hash)
				if err != nil {
					log.Debugf("slp entry does not exist for this transaction %v", txi.PreviousOutPoint.Hash)
				}
				if slpEntry != nil {
					slpMsg, err := v1parser.ParseSLP(slpEntry.SlpOpReturn)
					if err != nil {
						log.Criticalf("failed to parse slp message stored in db %v", txi.PreviousOutPoint.Hash)
						return err
					}
					burnedInputs = append(burnedInputs, &BurnedInput{
						Tx:      tx,
						TxInput: txi,
						SlpMsg:  slpMsg,
						Entry:   slpEntry,
					})
				}
			}
		}

		if txnInputsBurned != nil {
			burnedInputs = append(burnedInputs, txnInputsBurned...)
		}

		// Add Graph search entries here once the gs db is available
		if idx.GraphSearchEnabled() && isValid {
			go idx.AddGraphSearchTxn(tx)
		}

	}

	// Remove block transactions from the slp mempool cache
	idx.removeMempoolSlpTxs(block.Transactions())

	// Loop through burned inputs and check for different situations
	// where token metadata will need to be updated.
	//
	// Currently the only stateful property is Mint Baton location.
	//
	for _, burn := range burnedInputs {
		_, err := idx.checkBurnedInputForMintBaton(dbTx, burn)
		if err != nil {
			log.Critical(err)
			return err
		}
	}
	return nil
}

// AddGraphSearchTxn adds a transaction to graph search.
func (idx *SlpIndex) AddGraphSearchTxn(tx *wire.MsgTx) {
	if idx.graphSearchDb == nil {
		return
	}

	if !idx.graphSearchDb.IsReady() {
		idx.graphSearchDb.SetReady()
	}

	err := idx.graphSearchDb.AddTxn(tx)
	if err != nil {
		log.Criticalf("Failed to add transcation %v to graph search db due to error: %v", tx.TxHash(), err)
	}
}

// LoadSlpGraphSearchDb is used to load data needed for slp graph search
//
// NOTE: this is launched as a goroutine and does not return errors!
func (idx *SlpIndex) LoadSlpGraphSearchDb(fetchTxn func(txnHash *chainhash.Hash) ([]byte, error), initWg *sync.WaitGroup, interupt *int32) {

	// this method shouldn't be called more than once or if gs is disabled
	if idx.graphSearchDb != nil || !idx.config.SlpGraphSearchEnabled {
		return
	}

	idx.graphSearchDb = slpgraphsearch.NewDb()

	// now that graphSearchDB is set we can call Done()
	if initWg != nil {
		initWg.Done()
	}

	// build an initial map containing all slp transactions mapped to their token ID hash
	txnTokenIDMap, err := idx.buildGraphSearchTokenMap()
	if err != nil {
		log.Debugf("slp graph search failed to load with error: %v", err)
		return
	}

	// loop through the map and load full transactions into s.slpGraphSearchDb
	//
	// Future TODO: this can be optimized by storing slp txn block region in an index
	//
	// Future TODO: to reduce memory footprint either lazy loading and/or a whitelist
	//	      of token IDs for graph search can be added.
	log.Debugf("fetching %s transactions for slp graph search db...", fmt.Sprint(len(*txnTokenIDMap)))
	getTxn := func(txnHash chainhash.Hash, _ *chainhash.Hash, wg *sync.WaitGroup) error {
		if wg != nil {
			defer wg.Done()
		}

		var msgTx wire.MsgTx

		txBytes, err := fetchTxn(&txnHash)
		if err != nil {
			log.Debugf("slp graph search transaction %v was not found")
		} else {
			err = msgTx.Deserialize(bytes.NewReader(txBytes))
			if err != nil {
				return fmt.Errorf("loadSlpGraphSearchDb failed to deserialize transaction: %v", err)
			}
		}

		return idx.graphSearchDb.AddTxn(&msgTx)
	}

	// Limit the number of goroutines to do script validation based on the
	// number of processor cores.  This helps ensure the system stays
	// reasonably responsive under heavy load.
	maxGoroutines := runtime.NumCPU() * 3
	guard := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	txnCount := 0
	for txnHash, tokenIDHash := range *txnTokenIDMap {
		if interupt != nil && atomic.LoadInt32(interupt) == 1 {
			log.Warn("slp graph search loading interupted signal received")
			break
		}

		if txnCount > 0 && txnCount%10000 == 0 {
			log.Infof("slp graph search %s transactions loaded...", fmt.Sprint(txnCount))
		}
		txnCount++

		if len(*txnTokenIDMap) > maxGoroutines && maxGoroutines > 0 {
			if txnCount == 1 {
				log.Debug("loading slp graph search transactions concurrently")
			}
			guard <- struct{}{}
			wg.Add(1)
			go func(txnHash chainhash.Hash, tokenIDHash *chainhash.Hash, wg *sync.WaitGroup) {
				err := getTxn(txnHash, tokenIDHash, wg)
				if err != nil {
					log.Warn(err.Error())
				}
				<-guard
			}(txnHash, tokenIDHash, &wg)
		} else {
			err := getTxn(txnHash, tokenIDHash, nil)
			if err != nil {
				log.Warn(err.Error())
			}
		}
	}

	wg.Wait()

	// try to set db state to loaded
	err = idx.graphSearchDb.SetLoaded()
	if err != nil {
		log.Debug("couldn't set state to loaded: %v", err)
	} else {
		log.Infof("slp graph search finished fetching %s transactions", fmt.Sprint(txnCount))
	}
}

// GetGraphSearchDb checks if graph search is enabled and returns the db
func (idx *SlpIndex) GetGraphSearchDb() (*slpgraphsearch.Db, error) {
	if !idx.config.SlpGraphSearchEnabled {
		return nil, errors.New("slp graph search is not enabled")
	}

	if idx.graphSearchDb == nil {
		return nil, errors.New("an internal error has occurred, slp graph search db has not been created yet")
	}

	if !idx.graphSearchDb.IsLoaded() {
		return idx.graphSearchDb, fmt.Errorf("graph search db is loading")
	}

	if !idx.graphSearchDb.IsReady() {
		return idx.graphSearchDb, fmt.Errorf("graph search db is waiting on the next block")
	}

	return idx.graphSearchDb, nil
}

func (idx *SlpIndex) checkBurnedInputForMintBaton(dbTx database.Tx, burn *BurnedInput) (bool, error) {

	// we can skip nft children since they don't have mint batons
	if burn.SlpMsg.TokenType() == v1parser.TokenTypeNft1Child41 {
		return false, nil
	}

	// check if input is the mint baton from either Genesis or Mint parent data
	switch msg := burn.SlpMsg.(type) {
	case *v1parser.SlpGenesis:
		if msg.MintBatonVout != int(burn.TxInput.PreviousOutPoint.Index) {
			return false, nil
		}
	case *v1parser.SlpMint:
		if msg.MintBatonVout != int(burn.TxInput.PreviousOutPoint.Index) {
			return false, nil
		}
	default:
		return false, nil
	}

	// double-check this burned mint baton was a valid slp token
	if burn.Entry == nil {
		return false, nil
	}

	err := dbPutTokenIDIndexEntry(dbTx, burn.Entry.TokenID,
		&TokenMetadata{
			TokenID:       &burn.Entry.TokenIDHash,
			SlpVersion:    burn.Entry.SlpVersionType,
			MintBatonHash: nil,
			MintBatonVout: 0,
			NftGroupID:    nil,
		},
	)
	if err != nil {
		return false, fmt.Errorf("could not update token metadata for token id: %v", burn.Entry.TokenIDHash)
	}

	return true, nil
}

// GetSlpIndexEntryHandler provides a function interface for CheckSlpTx
type GetSlpIndexEntryHandler func(*chainhash.Hash) (*SlpTxEntry, error)

// AddTxIndexEntryHandler provides a function interface for CheckSlpTx
type AddTxIndexEntryHandler func(*wire.MsgTx, v1parser.ParseResult, *chainhash.Hash) error

// CheckSlpTx checks a transaction for validity and adds valid transactions to the db
func CheckSlpTx(tx *wire.MsgTx, getSlpIndexEntry GetSlpIndexEntryHandler, putTxIndexEntry AddTxIndexEntryHandler) (bool, []*BurnedInput, error) {

	if len(tx.TxOut) < 1 {
		return false, nil, nil
	}
	txSlpMsg, err := v1parser.ParseSLP(tx.TxOut[0].PkScript)
	if err != nil {
		log.Debugf("slp parsing failed for %v: %v", tx.TxHash(), err)
	}
	if txSlpMsg == nil {
		return false, nil, nil
	}

	burnedInputs := make([]*BurnedInput, 0)

	tokenID, err := goslp.GetSlpTokenID(tx)
	if err != nil {
		return false, nil, err
	}
	tokenIDHash, err := chainhash.NewHash(tokenID[:])
	if err != nil {
		return false, nil, err
	}

	v1InputAmtSpent := big.NewInt(0)
	v1MintBatonVout := 0

	// loop through inputs to look for valid slp contributions, and check for burned inputs
	for i, txi := range tx.TxIn {
		prevIdx := int(txi.PreviousOutPoint.Index)

		slpEntry, err := getSlpIndexEntry(&txi.PreviousOutPoint.Hash)
		if err != nil {
			log.Tracef("no slp input entry for %v, %v", txi.PreviousOutPoint.Hash, err)
		}
		if slpEntry == nil {
			continue
		}

		inputSlpMsg, err := v1parser.ParseSLP(slpEntry.SlpOpReturn)
		if err != nil {
			return false, nil, fmt.Errorf("previously saved slp scriptPubKey cannot be parsed: %v", err)
		}

		amt, _ := inputSlpMsg.GetVoutValue(prevIdx)
		if amt == nil {
			amt = new(big.Int).SetUint64(0)
		}

		_, isGenesis := txSlpMsg.(*v1parser.SlpGenesis)
		if txSlpMsg.TokenType() == v1parser.TokenTypeNft1Child41 && isGenesis { // checks inputs for NFT1 child GENESIS
			if inputSlpMsg.TokenType() == v1parser.TokenTypeNft1Group81 && i == 0 {
				v1InputAmtSpent.Add(v1InputAmtSpent, amt)
			}
		} else {
			switch txSlpMsg.(type) {
			case *v1parser.SlpMint:
				if slpEntry.TokenIDHash.Compare(tokenIDHash) == 0 && inputSlpMsg.TokenType() == txSlpMsg.TokenType() {
					switch inMsg := inputSlpMsg.(type) {
					case *v1parser.SlpGenesis:
						if prevIdx == inMsg.MintBatonVout {
							v1MintBatonVout = prevIdx
						}
					case *v1parser.SlpMint:
						if prevIdx == inMsg.MintBatonVout {
							v1MintBatonVout = prevIdx
						}
					}
				} else {
					burnedInputs = append(burnedInputs, &BurnedInput{
						Tx:      tx,
						TxInput: txi,
						SlpMsg:  inputSlpMsg,
						Entry:   slpEntry,
					})
				}
			case *v1parser.SlpSend:
				if slpEntry.TokenIDHash.Compare(tokenIDHash) == 0 && inputSlpMsg.TokenType() == txSlpMsg.TokenType() {
					v1InputAmtSpent.Add(v1InputAmtSpent, amt)

					// catch mint batons burned in a valid SEND transaction
					switch inMsg := inputSlpMsg.(type) {
					case *v1parser.SlpGenesis:
						if prevIdx == inMsg.MintBatonVout {
							burnedInputs = append(burnedInputs, &BurnedInput{
								Tx:      tx,
								TxInput: txi,
								SlpMsg:  inputSlpMsg,
								Entry:   slpEntry,
							})
						}
						if prevIdx == inMsg.MintBatonVout {
							v1MintBatonVout = prevIdx
						}
					case *v1parser.SlpMint:
						if prevIdx == inMsg.MintBatonVout {
							burnedInputs = append(burnedInputs, &BurnedInput{
								Tx:      tx,
								TxInput: txi,
								SlpMsg:  inputSlpMsg,
								Entry:   slpEntry,
							})
						}
						if prevIdx == inMsg.MintBatonVout {
							v1MintBatonVout = prevIdx
						}
					}
				} else {
					burnedInputs = append(burnedInputs, &BurnedInput{
						Tx:      tx,
						TxInput: txi,
						SlpMsg:  inputSlpMsg,
						Entry:   slpEntry,
					})
				}
			}
		}
	}

	// Check if tx is a valid slp. Slp validity has two requirements:
	//  (1) the slpMsg must be valid, and
	//  (2) the input requirements must be satisfied.
	isValid := false
	outputAmt, err := txSlpMsg.TotalSlpMsgOutputValue()
	if err != nil {
		return false, nil, err
	}
	switch txSlpMsg.(type) {
	case *v1parser.SlpGenesis:
		if txSlpMsg.TokenType() == v1parser.TokenTypeNft1Child41 && big.NewInt(1).Cmp(v1InputAmtSpent) < 1 {
			isValid = true
		} else if txSlpMsg.TokenType() == v1parser.TokenTypeFungible01 || txSlpMsg.TokenType() == v1parser.TokenTypeNft1Group81 {
			isValid = true
		}
	case *v1parser.SlpSend:
		if outputAmt.Cmp(v1InputAmtSpent) < 1 {
			isValid = true
		}
	case *v1parser.SlpMint:
		if v1MintBatonVout > 1 {
			isValid = true
		}
	}

	if isValid {
		err := putTxIndexEntry(tx, txSlpMsg, tokenIDHash)
		if err != nil {
			return false, nil, err
		}
	}
	return isValid, burnedInputs, nil
}

// DisconnectBlock is invoked by the index manager when a block has been
// disconnected from the main chain.  This indexer removes the
// hash-to-transaction mapping for every transaction in the block.
//
// This is part of the Indexer interface.
func (idx *SlpIndex) DisconnectBlock(dbTx database.Tx, block *bchutil.Block, stxos []blockchain.SpentTxOut) error {
	// Remove all of the transactions in the block from the index.
	return dbRemoveSlpIndexEntries(dbTx, block)
}

// GetSlpIndexEntry returns a serialized slp index entry for the provided transaction hash
// from the slp index.  The slp index entry can in turn be used to quickly discover
// additional slp information about the transaction. When there is no entry for the provided hash, nil
// will be returned for the both the entry and the error, which would mean the transaction is invalid
//
// This function is safe for concurrent access.
func (idx *SlpIndex) GetSlpIndexEntry(dbTx database.Tx, hash *chainhash.Hash) (*SlpTxEntry, error) {
	if entry, ok := idx.cache.GetSlpTxEntry(hash); ok {
		log.Debugf("using slp txn entry cache for txid %v", hash)
		return &entry, nil
	}

	// fallback to fetch entry from db
	entry, err := dbFetchSlpIndexEntry(dbTx, hash)
	if err != nil {
		return nil, err
	}

	err = idx.cache.AddSlpTxEntry(hash, *entry)
	if err != nil {
		log.Criticalf("AddSlpTxEntry in GetSlpINdexEntry failed: ", err)
	}
	return entry, nil
}

// GetTokenMetadata fetches token metadata properties from an SlpIndexEntry
func (idx *SlpIndex) GetTokenMetadata(dbTx database.Tx, entry *SlpTxEntry) (*TokenMetadata, error) {
	if tm, ok := idx.cache.GetTokenMetadata(&entry.TokenIDHash); ok {
		log.Debugf("using token metadata cache for %s", hex.EncodeToString(entry.TokenIDHash[:]))
		return &tm, nil
	}

	if entry.TokenID == 0 {
		id, err := dbFetchTokenIDByHash(dbTx, &entry.TokenIDHash)
		if err != nil {
			log.Debugf("db is missing tokenID %s", hex.EncodeToString(entry.TokenIDHash[:]))
			return nil, err
		}
		entry.TokenID = id
	}

	// fallback to fetch token metadata from db
	tm, err := dbFetchTokenMetadataByID(dbTx, entry.TokenID)
	if err != nil {
		return nil, err
	}

	err = idx.cache.AddTempTokenMetadata(*tm)
	if err != nil {
		log.Criticalf("AddTempTokenMetadata in GetTokenMetadata failed: ", err)
	}
	return tm, nil
}

// AddPotentialSlpEntries checks if a transaction is slp valid and then will add a
// new SlpIndexEntry to the shared cache of valid slp transactions.
//
// This method should be used to assess slp validity of newly received mempool items and also in rpc
// client subscriber methods that return notifications for both mempool and block events to prevent
// any possibility of a race conditions with manageSlpEntryCache.
func (idx *SlpIndex) AddPotentialSlpEntries(dbTx database.Tx, msgTx *wire.MsgTx) (bool, error) {

	getSlpIndexEntry := func(txiHash *chainhash.Hash) (*SlpTxEntry, error) {
		entry, err := idx.GetSlpIndexEntry(dbTx, txiHash)
		if entry != nil {
			return entry, nil
		}

		return nil, err
	}

	putTxIndexEntry := func(tx *wire.MsgTx, slpMsg v1parser.ParseResult, tokenIDHash *chainhash.Hash) error {
		scriptPubKey := tx.TxOut[0].PkScript
		hash := tx.TxHash()

		// add item to slp txn cache
		idx.cache.AddMempoolSlpTxEntry(&hash, SlpTxEntry{
			TokenID:        0,
			TokenIDHash:    *tokenIDHash,
			SlpVersionType: slpMsg.TokenType(),
			SlpOpReturn:    scriptPubKey,
		})

		// add or update token metadata cache
		switch t := slpMsg.(type) {
		case *v1parser.SlpGenesis:
			// add genesis token metadata to cache
			log.Debugf("adding slp genesis token metadata for %v", hex.EncodeToString(tokenIDHash[:]))
			tm := TokenMetadata{
				TokenID:    tokenIDHash,
				SlpVersion: slpMsg.TokenType(),
			}
			if slpMsg.TokenType() == v1parser.TokenTypeNft1Child41 {
				// handle special case for NFT child with group id
				err := idx.db.View(func(dbTx database.Tx) error {
					entry, err := idx.GetSlpIndexEntry(dbTx, &tx.TxIn[0].PreviousOutPoint.Hash)
					if err != nil {
						return fmt.Errorf("nft child genesis has invalid group in txn %v: %v", tx.TxHash(), err)
					}
					tm.NftGroupID = &entry.TokenIDHash
					if tm.NftGroupID == nil {
						return fmt.Errorf("nft child group ID could not be resolved in txn %v", tx.TxHash())
					}
					return nil
				})
				if err != nil {
					log.Debugf("AddPotentialSlpEntries: %v", err)
				}
			} else if t.MintBatonVout > 1 {
				hash := tx.TxHash()
				tm.MintBatonHash = &hash
				tm.MintBatonVout = uint32(t.MintBatonVout)
			}
			err := idx.cache.AddTempTokenMetadata(tm)
			if err != nil {
				log.Criticalf("AddTempTokenMetadata in AddPotentialSlpEntries failed for Genesis: ", err)
			}
		case *v1parser.SlpMint:
			// update the mint baton location
			log.Debugf("adding slp mint token metadata for %v", hex.EncodeToString(tokenIDHash[:]))
			err := idx.db.View(func(dbTx database.Tx) error {
				hash := tx.TxHash()
				entry, err := idx.GetSlpIndexEntry(dbTx, &hash)
				if err != nil {
					return fmt.Errorf("could not retreive token entry for mint txn %v: %v", hash, err)
				}
				tm, err := idx.GetTokenMetadata(dbTx, entry)
				if err != nil {
					return fmt.Errorf("could not retreive token metadata for mint txn %v: %v", hash, err)
				}
				if t.MintBatonVout > 1 {
					hash := tx.TxHash()
					tm.MintBatonHash = &hash
					tm.MintBatonVout = uint32(t.MintBatonVout)
					err := idx.cache.AddTempTokenMetadata(*tm)
					if err != nil {
						log.Criticalf("AddTempTokenMetadata in AddPotentialSlpEntries failed for Mint: ", err)
					}
				} else {
					return fmt.Errorf("invalid mint baton for mint txn %v: %v", hash, err)
				}
				return nil
			})
			if err != nil {
				log.Debugf("AddPotentialSlpEntries: %v", err)
			}
		}
		return nil
	}

	valid, _, err := CheckSlpTx(msgTx, getSlpIndexEntry, putTxIndexEntry)
	if err != nil {
		return valid, err
	}

	return valid, nil
}

// removeMempoolSlpTxs removes a list of transactions from the temporary cache that holds
// both mempool and recently queried SlpIndexEntries
func (idx *SlpIndex) removeMempoolSlpTxs(txs []*bchutil.Tx) {
	// skip removal if slp graph search enabled but is still loading, this way gs db won't miss any txns
	if idx.config.SlpGraphSearchEnabled && (idx.graphSearchDb == nil || !idx.graphSearchDb.IsLoaded()) {
		return
	}
	idx.cache.RemoveMempoolSlpTxItems(txs)
}

// buildGraphSearchTokenMap is used to load transactions and associated tokenID
func (idx *SlpIndex) buildGraphSearchTokenMap() (*map[chainhash.Hash]*chainhash.Hash, error) {

	db := make(map[chainhash.Hash]*chainhash.Hash)

	err := idx.db.View(func(dbTx database.Tx) error {
		idxBucket := dbTx.Metadata().Bucket(slpIndexKey)
		if idxBucket == nil {
			return fmt.Errorf("bucket nil for key: %s", slpIndexKey)
		}

		// loop through all of the valid slp txn items stored in the db
		err := idxBucket.ForEach(func(k []byte, v []byte) error {
			ch, err := chainhash.NewHash(k)
			if err != nil {
				return err
			}
			tm, err := dbFetchTokenMetadataBySerializedID(dbTx, v[0:4])
			if err != nil {
				return err
			}
			db[*ch] = tm.TokenID
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// loop through all of the valid slp txn items in the mempool slp cache
	err = idx.cache.ForEachMempoolItem(func(h *chainhash.Hash, e *SlpTxEntry) error {
		db[*h] = &e.TokenIDHash
		return nil
	})
	if err != nil {
		return nil, err
	}

	log.Infof("SLP graph search is loading %s transactions...", fmt.Sprint(len(db)))
	return &db, nil
}

// SlpConfig provides the proper starting height and hash
type SlpConfig struct {
	StartHash             *chainhash.Hash
	StartHeight           int32
	AddrPrefix            string
	MaxCacheSize          int
	SlpGraphSearchEnabled bool
}

// NewSlpIndex returns a new instance of an indexer that is used to create a
// mapping of the hashes of all slp transactions in the blockchain to the respective
// token ID, and token metadata.
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
func NewSlpIndex(db database.DB, cfg *SlpConfig) *SlpIndex {
	idx := &SlpIndex{
		db:     db,
		config: cfg,
		cache:  InitSlpCache(cfg.MaxCacheSize),
	}

	return idx
}

// dropTokenIndexes drops the internal token id index.
func dropTokenIndexes(db database.DB) error {
	return db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		err := meta.DeleteBucket(tokenIDByHashIndexBucketName)
		if err != nil {
			return err
		}

		return meta.DeleteBucket(tokenMetadataByIDIndexBucketName)
	})
}

// DropSlpIndex drops the transaction index from the provided database if it
// exists.  Since the address index relies on it, the address index will also be
// dropped when it exists.
func DropSlpIndex(db database.DB, interrupt <-chan struct{}) error {
	return dropIndex(db, slpIndexKey, slpIndexName, interrupt)
}

// TopologicallySortTxs sorts a list of transactions into topological order.
// That is, the child transactions come after parents.
func TopologicallySortTxs(transactions []*bchutil.Tx) []*wire.MsgTx {

	sorted := make([]*wire.MsgTx, 0, len(transactions))
	txids := make(map[chainhash.Hash]struct{}, len(transactions))
	outpoints := make(map[wire.OutPoint]struct{})

	for _, tx := range transactions {
		for i := range tx.MsgTx().TxOut {
			op := wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(i),
			}
			outpoints[op] = struct{}{}
		}
	}

	for len(sorted) < len(transactions) {
		for _, tx := range transactions {
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
