// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/merkleblock"
)

const (
	// BcmrMaxAuthchainEntries is the most links of an authchain an answer
	// holds.
	BcmrMaxAuthchainEntries = 1000

	// BcmrMaxAuthchainBytes is the size of the transactions after which an
	// answer holds no more links: it stops at the link that reaches it, and
	// holds at least one link.  A transaction is at most 1 MB, so an
	// answer's transactions stay under 2 MB, under the 4 MB most gRPC clients
	// accept by default, and under 4 MB as hex in JSON.  The URIs of
	// publications come on top of that, which standard transactions keep
	// small.
	BcmrMaxAuthchainBytes = 1000000

	// bcmrMaxMempoolLinks bounds the links a query follows through the
	// mempool, which would have to hold over 270 MB of transactions of one
	// authchain to exceed it, as it has no limit of its own.
	bcmrMaxMempoolLinks = 1 << 22
)

// BcmrMaxListedIdentities is the most identities ListIdentities returns.
const BcmrMaxListedIdentities = 1000

// bcmrMaxAuthchainBytes is BcmrMaxAuthchainBytes, which tests lower.
var bcmrMaxAuthchainBytes = BcmrMaxAuthchainBytes

var (
	// ErrBcmrIdentityNotFound is returned for an authbase that is no link
	// the index tracks, and that no genesis in the mempool spends the
	// identity output of either.
	ErrBcmrIdentityNotFound = errors.New("no identity with that authbase")

	// ErrBcmrNotALink is returned for a query that asks for the links after
	// a transaction that is not a link of the identity's authchain, which
	// a reorganization can make of a link the client knew.
	ErrBcmrNotALink = errors.New("the transaction is not a link of the " +
		"identity's authchain")
)

// BcmrMempool is what a query reads from the mempool.  The mempool's TxPool
// implements it.
type BcmrMempool interface {
	CheckSpend(op wire.OutPoint) *bchutil.Tx
	FetchTransaction(txHash *chainhash.Hash) (*bchutil.Tx, error)
}

// BcmrQuery is a query of the BCMR index.
type BcmrQuery struct {
	// Authbase is the identity's authbase: a token's category ID, or any
	// other link the index tracks.
	Authbase chainhash.Hash

	// Authchain asks for the links of the authchain, from the authbase,
	// or, when After is set, from the link after it.
	Authchain bool
	After     *chainhash.Hash

	// Limit is the most links to return, up to BcmrMaxAuthchainEntries,
	// which 0 also means.
	Limit int
}

// BcmrLinkInfo is a link of an authchain in an answer.
type BcmrLinkInfo struct {
	// Hash is the link's txid.
	Hash chainhash.Hash

	// Height and BlockHash are those of the link's block, or 0 and nil
	// while the link is in the mempool.
	Height    int32
	BlockHash *chainhash.Hash

	// Publication is the link's BCMR publication, or nil if it has none.
	Publication *bcmr.Publication

	// Tx is the serialized transaction, for the links of an authchain.
	Tx []byte
}

// BcmrIdentityInfo is what the index knows of an identity, with the links
// that the mempool adds to its authchain.
type BcmrIdentityInfo struct {
	// Authbase is the identity's authbase.
	Authbase chainhash.Hash

	// Burned reports that the authhead spent the identity output to a
	// data-carrier output, which ends the authchain.
	Burned bool

	// Head is the authhead, the newest link, confirmed or not.
	Head BcmrLinkInfo

	// Publication is the link with the publication to read: the
	// authhead's, when it has a well-formed one and Current is set, or
	// else the newest earlier link of the authchain with one.  It is nil
	// when no link of the authchain has one.
	Publication *BcmrLinkInfo
	Current     bool

	// HasGenesisSupply reports that the identity is a token category's,
	// and GenesisSupply is then the number of fungible tokens its genesis
	// created.
	HasGenesisSupply bool
	GenesisSupply    uint64

	// IdentityOutput is the token data of the authhead's identity output,
	// as it is, or nil if that output has none.
	IdentityOutput *wire.TokenData

	// FirstIndex and Authchain are the links a query for the authchain
	// asked for, with the index of the first, counted from the authbase
	// as 0, and More reports that later links didn't fit.
	FirstIndex uint32
	Authchain  []BcmrLinkInfo
	More       bool

	// Proof is a merkle proof of the newest confirmed link, which proves
	// every earlier link too, since each spends the one before.  ProofIndex
	// is that link's index.  An answer has it when it holds the authchain
	// up to the authhead and a link is confirmed.
	Proof      *wire.MsgMerkleBlock
	ProofIndex uint32

	// ProofBlock is the block of the proven link.
	ProofBlock *bchutil.Block
}

// Identity returns what the index knows of the identity with the authbase in
// the query, with the links the mempool adds to its authchain, or
// ErrBcmrIdentityNotFound.
//
// This function is safe for concurrent access.
func (idx *BcmrIndex) Identity(query *BcmrQuery, mempool BcmrMempool) (*BcmrIdentityInfo, error) {
	// A genesis in the mempool spends the authbase's identity output, which
	// may be all that shows where the authbase is.  The chain reads the
	// UTXO set in a database transaction of its own, and a second one in
	// this goroutine would hang if the database started to close between
	// the two, so the output is read before the query's.
	outpoint := wire.OutPoint{Hash: query.Authbase}
	genesis := mempool.CheckSpend(outpoint)
	var utxoEntry *blockchain.UtxoEntry
	if genesis != nil {
		if idx.chain == nil {
			return nil, AssertError("the BCMR index has no chain to read the UTXO set from")
		}
		var err error
		utxoEntry, err = idx.chain.FetchUtxoEntry(outpoint)
		if err != nil {
			return nil, err
		}
	}

	var info *BcmrIdentityInfo
	err := idx.db.View(func(dbTx database.Tx) error {
		q := &bcmrQuerier{idx: idx, dbTx: dbTx, query: query, mempool: mempool,
			genesis: genesis, utxoEntry: utxoEntry}
		var err error
		info, err = q.answer()
		return err
	})
	return info, err
}

// bcmrNoMempool is a mempool with nothing in it, for answers from the chain
// alone.
type bcmrNoMempool struct{}

// CheckSpend reports that nothing spends op.
func (bcmrNoMempool) CheckSpend(wire.OutPoint) *bchutil.Tx {
	return nil
}

// FetchTransaction reports that the mempool doesn't have the transaction.
func (bcmrNoMempool) FetchTransaction(hash *chainhash.Hash) (*bchutil.Tx, error) {
	return nil, fmt.Errorf("transaction %v is not in the mempool", hash)
}

// ListIdentities returns what the index knows of the identities after the
// passed authbase, or from the first when it is nil, in the order of their
// authbases' bytes, up to limit of them, and whether there are more.  An answer
// is from the chain alone, without the mempool.  The identities of a call and
// of the next can be of different blocks, and an identity a reorganization
// removes meanwhile is left out.
//
// This function is safe for concurrent access.
func (idx *BcmrIndex) ListIdentities(after *chainhash.Hash, limit int) ([]*BcmrIdentityInfo, bool, error) {
	if limit <= 0 || limit > BcmrMaxListedIdentities {
		limit = BcmrMaxListedIdentities
	}

	// One more than the limit shows whether there are more.  Identities
	// whose authchains join share what's read of the segments from there.
	var infos []*BcmrIdentityInfo
	var more bool
	err := idx.db.View(func(dbTx database.Tx) error {
		var authbases []chainhash.Hash
		bucket := dbTx.Metadata().Bucket(bcmrIndexKey).Bucket(bcmrBucketNames[bcmrIdentities])
		cursor := bucket.Cursor()
		var ok bool
		if after == nil {
			ok = cursor.First()
		} else {
			ok = cursor.Seek(after[:])
			if ok && bytes.Equal(cursor.Key(), after[:]) {
				ok = cursor.Next()
			}
		}
		for ; ok && len(authbases) <= limit; ok = cursor.Next() {
			var authbase chainhash.Hash
			copy(authbase[:], cursor.Key())
			authbases = append(authbases, authbase)
		}
		more = len(authbases) > limit
		if more {
			authbases = authbases[:limit]
		}

		suffixes := make(bcmrSuffixes)
		infos = make([]*BcmrIdentityInfo, 0, len(authbases))
		for i := range authbases {
			state, err := fetchBcmrIdentitySummary(dbTx, &authbases[i], suffixes)
			if err != nil {
				return err
			}
			if state == nil {
				continue
			}
			q := &bcmrQuerier{idx: idx, dbTx: dbTx, query: &BcmrQuery{Authbase: authbases[i]},
				mempool: bcmrNoMempool{}, summary: state}
			info, err := q.answer()
			if err != nil {
				return err
			}
			infos = append(infos, info)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return infos, more, nil
}

// bcmrQuerier answers a query in a database transaction.
type bcmrQuerier struct {
	idx     *BcmrIndex
	dbTx    database.Tx
	query   *BcmrQuery
	mempool BcmrMempool

	// genesis is the transaction in the mempool that spends the authbase's
	// identity output, and utxoEntry that output in the UTXO set.
	genesis   *bchutil.Tx
	utxoEntry *blockchain.UtxoEntry

	// state is what the index knows of the identity, or nil for a token
	// whose genesis is in the mempool.  A summary, set before the query is
	// answered, is the state without its path, for an answer without the
	// authchain.
	state   *bcmrIdentityState
	summary *bcmrIdentityState

	// unconfirmedBase is the authbase of a token whose genesis is in the
	// mempool, when it is in the mempool too.
	unconfirmedBase *bchutil.Tx

	// mempoolLinks are the links in the mempool, after the newest confirmed
	// one, or after the authbase of a token whose genesis is in the
	// mempool, and mempoolIndex is the index of the first in the authchain.
	mempoolLinks []*bchutil.Tx
	mempoolIndex uint32

	// authbase is the authbase of a token whose genesis is in the mempool,
	// with its transaction, once it is found.
	authbase *BcmrLinkInfo

	// blocks are the blocks the query read, as the authbase's block can
	// also be the proof's.
	blocks bcmrBlockCache
}

// mempoolLinksFrom returns the links in the mempool that the identity output
// of the transaction with the passed hash leads to, up to the first that burns
// the identity.
func (q *bcmrQuerier) mempoolLinksFrom(hash chainhash.Hash) ([]*bchutil.Tx, error) {
	var links []*bchutil.Tx
	for last := hash; ; {
		tx := q.genesis
		if last != q.query.Authbase {
			tx = q.mempool.CheckSpend(wire.OutPoint{Hash: last})
		}
		if tx == nil {
			return links, nil
		}
		if len(links) >= bcmrMaxMempoolLinks {
			return nil, fmt.Errorf("the mempool holds more than %d links of the "+
				"authchain", bcmrMaxMempoolLinks)
		}
		links = append(links, tx)
		if bcmr.IsBurn(tx.MsgTx()) {
			return links, nil
		}
		last = *tx.Hash()
	}
}

// answer answers the query.
func (q *bcmrQuerier) answer() (*BcmrIdentityInfo, error) {
	info := &BcmrIdentityInfo{Authbase: q.query.Authbase}
	state := q.summary
	if state == nil {
		var err error
		state, err = fetchBcmrIdentityState(q.dbTx, &q.query.Authbase)
		if err != nil {
			return nil, err
		}
	} else if q.query.Authchain {
		return nil, AssertError("a summary of a BCMR identity has no path for its authchain")
	}
	q.state = state
	var err error

	// Follow the identity output through the mempool, from the newest
	// confirmed link, or from the genesis of a token whose genesis is in the
	// mempool: its authbase is in a block or in the mempool, and the genesis
	// is the next link.
	if state != nil {
		info.Burned = state.burned
		if state.identity != nil {
			info.HasGenesisSupply = true
			info.GenesisSupply = state.identity.supply
		}
		q.mempoolIndex = state.length()
		if !state.burned {
			q.mempoolLinks, err = q.mempoolLinksFrom(state.head)
			if err != nil {
				return nil, err
			}
		}

		// A genesis in the mempool that spends the identity output of an
		// authhead that is no token's authbase makes it one, as it does
		// once it confirms.
		if state.identity == nil && len(q.mempoolLinks) > 0 {
			g, ok := bcmr.FindGenesis(q.mempoolLinks[0].MsgTx(), q.query.Authbase)
			if ok {
				info.HasGenesisSupply = true
				info.GenesisSupply = g.Supply
			}
		}
	} else {
		genesis := q.genesis
		if genesis == nil {
			return nil, ErrBcmrIdentityNotFound
		}
		g, ok := bcmr.FindGenesis(genesis.MsgTx(), q.query.Authbase)
		if !ok {
			return nil, ErrBcmrIdentityNotFound
		}
		info.HasGenesisSupply = true
		info.GenesisSupply = g.Supply
		base, err := q.mempool.FetchTransaction(&q.query.Authbase)
		if err == nil {
			q.unconfirmedBase = base
		}
		q.mempoolIndex = 1
		q.mempoolLinks = []*bchutil.Tx{genesis}
		if !bcmr.IsBurn(genesis.MsgTx()) {
			later, err := q.mempoolLinksFrom(*genesis.Hash())
			if err != nil {
				return nil, err
			}
			q.mempoolLinks = append(q.mempoolLinks, later...)
		}
	}
	if n := len(q.mempoolLinks); n > 0 {
		info.Burned = bcmr.IsBurn(q.mempoolLinks[n-1].MsgTx())
	}

	if err := q.answerHead(info); err != nil {
		return nil, err
	}
	if q.query.Authchain {
		if err := q.answerAuthchain(info); err != nil {
			return nil, err
		}
	}
	return info, nil
}

// length returns the number of links of the identity's authchain that the
// index holds.
func (s *bcmrIdentityState) length() uint32 {
	var n uint32
	for _, length := range s.lengths {
		n += length
	}
	return n - s.place
}

// answerHead fills in the authhead, the publication to read, and the token
// data of the identity output.
func (q *bcmrQuerier) answerHead(info *BcmrIdentityInfo) error {
	// The authhead is the last link in the mempool, or else the newest
	// confirmed one: the identity's authhead, or the authbase of a token
	// whose genesis is in the mempool.
	var headTx *wire.MsgTx
	if n := len(q.mempoolLinks); n > 0 {
		head := q.mempoolLinks[n-1]
		info.Head = mempoolBcmrLinkInfo(head)
		headTx = head.MsgTx()
	} else {
		head, err := q.confirmedLinkInfo(q.state.head, true)
		if err != nil {
			return err
		}
		info.Head = *head
		headTx, err = deserializeBcmrTx(head.Tx)
		if err != nil {
			return err
		}
		info.Head.Tx = nil
	}
	if !info.Head.Publication.WellFormed() {
		info.Publication, info.Current = nil, false
	} else {
		head := info.Head
		info.Publication, info.Current = &head, true
	}
	if tokenData := headTx.TxOut[0].TokenData; !tokenData.IsEmpty() {
		info.IdentityOutput = &tokenData
	}
	if info.Current {
		return nil
	}

	// Without a well-formed publication at the authhead, the publication
	// to read is that of the newest earlier link with one.
	for i := len(q.mempoolLinks) - 2; i >= 0; i-- {
		link := mempoolBcmrLinkInfo(q.mempoolLinks[i])
		if link.Publication.WellFormed() {
			info.Publication = &link
			return nil
		}
	}
	switch {
	case q.state != nil && q.state.pubLink != (chainhash.Hash{}):
		link, err := q.confirmedLinkInfo(q.state.pubLink, false)
		if err != nil {
			return err
		}
		info.Publication = link

	case q.state == nil:
		base, err := q.authbaseInfo(false)
		if err != nil {
			return err
		}
		if base.Publication.WellFormed() {
			info.Publication = base
		}
	}
	return nil
}

// mempoolBcmrLinkInfo returns a link in the mempool.
func mempoolBcmrLinkInfo(tx *bchutil.Tx) BcmrLinkInfo {
	return BcmrLinkInfo{
		Hash:        *tx.Hash(),
		Publication: bcmr.ParsePublication(tx.MsgTx()),
	}
}

// deserializeBcmrTx decodes a serialized transaction.
func deserializeBcmrTx(raw []byte) (*wire.MsgTx, error) {
	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(raw)); err != nil {
		return nil, err
	}
	return &msgTx, nil
}

// confirmedLinkInfo returns a link the index holds, with its transaction if
// withTx is set.
func (q *bcmrQuerier) confirmedLinkInfo(hash chainhash.Hash, withTx bool) (*BcmrLinkInfo, error) {
	link, err := q.fetchLink(hash)
	if err != nil {
		return nil, err
	}
	return q.linkInfo(hash, link, withTx)
}

// fetchLink returns the link with the passed txid, which the index must hold.
func (q *bcmrQuerier) fetchLink(hash chainhash.Hash) (*bcmrLink, error) {
	raw := q.dbTx.Metadata().Bucket(bcmrIndexKey).Bucket(bcmrBucketNames[bcmrLinks]).Get(hash[:])
	if raw == nil {
		return nil, AssertError(fmt.Sprintf("BCMR link %v is missing", hash))
	}
	return deserializeBcmrLink(raw)
}

// linkInfo returns a link the index holds, with its transaction, which it
// reads from the block files, if withTx is set.
func (q *bcmrQuerier) linkInfo(hash chainhash.Hash, link *bcmrLink, withTx bool) (*BcmrLinkInfo, error) {
	blockHash := link.blockHash
	info := &BcmrLinkInfo{
		Hash:        hash,
		Height:      link.height,
		BlockHash:   &blockHash,
		Publication: link.pub,
	}
	if withTx {
		raw, err := q.dbTx.FetchBlockRegion(&database.BlockRegion{
			Hash: &blockHash, Offset: link.offset, Len: link.length,
		})
		if err != nil {
			return nil, err
		}
		info.Tx = bytes.Clone(raw)
	}
	return info, nil
}

// authbaseInfo returns the authbase of a token whose genesis is in the
// mempool, with its transaction if withTx is set.  It finds it once a query.
func (q *bcmrQuerier) authbaseInfo(withTx bool) (*BcmrLinkInfo, error) {
	if q.authbase == nil {
		info, err := q.findAuthbase()
		if err != nil {
			return nil, err
		}
		q.authbase = info
	}
	info := *q.authbase
	if !withTx {
		info.Tx = nil
	}
	return &info, nil
}

// findAuthbase returns the authbase of a token whose genesis is in the
// mempool, with its transaction.  It is in the mempool, or in the block where
// the UTXO set has its identity output: a link the index tracks would have an
// identity.
func (q *bcmrQuerier) findAuthbase() (*BcmrLinkInfo, error) {
	authbase := q.query.Authbase
	if q.unconfirmedBase != nil {
		info := mempoolBcmrLinkInfo(q.unconfirmedBase)
		raw, err := serializeBcmrTx(q.unconfirmedBase.MsgTx())
		if err != nil {
			return nil, err
		}
		info.Tx = raw
		return &info, nil
	}

	if q.utxoEntry == nil || q.utxoEntry.IsSpent() {
		return nil, ErrBcmrIdentityNotFound
	}
	height := q.utxoEntry.BlockHeight()
	blockHash, err := q.idx.chain.BlockHashByHeight(height)
	if err != nil {
		return nil, err
	}
	read, err := q.blocks.fetch(q.dbTx, blockHash, height)
	if err != nil {
		return nil, err
	}
	k, ok := read.positions[authbase]
	if !ok {
		return nil, AssertError(fmt.Sprintf("authbase %v is not in block %v", authbase, blockHash))
	}
	raw, err := read.block.Bytes()
	if err != nil {
		return nil, err
	}
	loc := read.locs[k]
	return &BcmrLinkInfo{
		Hash:        authbase,
		Height:      height,
		BlockHash:   blockHash,
		Publication: bcmr.ParsePublication(read.block.Transactions()[k].MsgTx()),
		Tx:          bytes.Clone(raw[loc.TxStart : loc.TxStart+loc.TxLen]),
	}, nil
}

// serializeBcmrTx serializes a transaction.
func serializeBcmrTx(msgTx *wire.MsgTx) ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(msgTx.SerializeSize())
	if err := msgTx.Serialize(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// linkIndex returns the index in the identity's authchain of a link the
// index holds, or ErrBcmrNotALink if the link is not on the authchain.
func (q *bcmrQuerier) linkIndex(link *bcmrLink) (uint32, error) {
	var base uint32
	for i, segment := range q.state.path {
		if segment == link.segment {
			if i == 0 && link.place < q.state.place {
				return 0, ErrBcmrNotALink
			}
			return base + link.place - q.state.place, nil
		}
		base += q.state.lengths[i]
	}
	return 0, ErrBcmrNotALink
}

// answerAuthchain fills in the links of the authchain the query asks for, and
// the proof when they reach the authhead.
func (q *bcmrQuerier) answerAuthchain(info *BcmrIdentityInfo) error {
	limit := q.query.Limit
	if limit <= 0 || limit > BcmrMaxAuthchainEntries {
		limit = BcmrMaxAuthchainEntries
	}

	// Find where the links to return start: the link after After, or the
	// authbase.  next is the confirmed link to start at, and mempoolStart
	// the link in the mempool after the confirmed ones.
	var next chainhash.Hash
	var index uint32
	mempoolStart := 0
	size := 0
	after := q.query.After
	switch {
	case after == nil && q.state != nil:
		next = q.query.Authbase

	case after == nil:
		base, err := q.authbaseInfo(true)
		if err != nil {
			return err
		}
		info.Authchain = append(info.Authchain, *base)
		size += len(base.Tx)

	default:
		k := -1
		for i, tx := range q.mempoolLinks {
			if *tx.Hash() == *after {
				k = i
				break
			}
		}
		switch {
		case k >= 0:
			mempoolStart = k + 1
			index = q.mempoolIndex + uint32(k) + 1

		case q.state == nil && *after == q.query.Authbase:
			index = q.mempoolIndex

		case q.state == nil:
			return ErrBcmrNotALink

		default:
			raw := q.dbTx.Metadata().Bucket(bcmrIndexKey).
				Bucket(bcmrBucketNames[bcmrLinks]).Get(after[:])
			if raw == nil {
				return ErrBcmrNotALink
			}
			link, err := deserializeBcmrLink(raw)
			if err != nil {
				return err
			}
			linkIndex, err := q.linkIndex(link)
			if err != nil {
				return err
			}
			index = linkIndex + 1
			if *after != q.state.head {
				next = link.next
			}
		}
	}
	info.FirstIndex = index

	// The links the index holds come first, then those in the mempool.
	full := func() bool {
		return len(info.Authchain) >= limit ||
			(len(info.Authchain) > 0 && size >= bcmrMaxAuthchainBytes)
	}
	for next != (chainhash.Hash{}) {
		if full() {
			info.More = true
			return nil
		}
		link, err := q.fetchLink(next)
		if err != nil {
			return err
		}
		entry, err := q.linkInfo(next, link, true)
		if err != nil {
			return err
		}
		info.Authchain = append(info.Authchain, *entry)
		size += len(entry.Tx)
		if next == q.state.head {
			break
		}
		next = link.next
	}
	for _, tx := range q.mempoolLinks[mempoolStart:] {
		if full() {
			info.More = true
			return nil
		}
		entry := mempoolBcmrLinkInfo(tx)
		raw, err := serializeBcmrTx(tx.MsgTx())
		if err != nil {
			return err
		}
		entry.Tx = raw
		info.Authchain = append(info.Authchain, entry)
		size += len(raw)
	}

	// The answer reaches the authhead, so it carries the proof of the
	// newest confirmed link.
	var proofLink *BcmrLinkInfo
	switch {
	case q.state != nil:
		link, err := q.confirmedLinkInfo(q.state.head, false)
		if err != nil {
			return err
		}
		proofLink = link
		info.ProofIndex = q.mempoolIndex - 1

	case q.unconfirmedBase == nil:
		base, err := q.authbaseInfo(false)
		if err != nil {
			return err
		}
		proofLink = base
		info.ProofIndex = 0
	}
	if proofLink == nil {
		return nil
	}
	read, err := q.blocks.fetch(q.dbTx, proofLink.BlockHash, proofLink.Height)
	if err != nil {
		return err
	}
	block := read.block
	info.Proof, _ = merkleblock.NewMerkleBlockWithTxnSet(block,
		[]*chainhash.Hash{&proofLink.Hash})
	info.ProofBlock = block
	return nil
}
