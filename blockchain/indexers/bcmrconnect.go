// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"fmt"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// bcmrTxOrder returns the positions of a block's transactions in an order in
// which each one comes after the transactions of the block whose outputs it
// spends.  Blocks list their transactions by txid since the canonical
// transaction ordering upgrade, so a link can come before the link whose
// identity output it spends.  positions is where each transaction is, by its
// txid, which the connector keeps too, so that it's built once a block.
func bcmrTxOrder(txs []*bchutil.Tx, positions map[chainhash.Hash]int) []int {
	children := make([][]int, len(txs))
	parents := make([]int, len(txs))
	for i, tx := range txs {
		for _, in := range tx.MsgTx().TxIn {
			j, ok := positions[in.PreviousOutPoint.Hash]
			if !ok || j == i {
				continue
			}
			children[j] = append(children[j], i)
			parents[i]++
		}
	}

	order := make([]int, 0, len(txs))
	for i := range txs {
		if parents[i] == 0 {
			order = append(order, i)
		}
	}
	for k := 0; k < len(order); k++ {
		for _, i := range children[order[k]] {
			parents[i]--
			if parents[i] == 0 {
				order = append(order, i)
			}
		}
	}
	return order
}

// bcmrConnector connects a block to the index.
type bcmrConnector struct {
	idx   *BcmrIndex
	dbTx  database.Tx
	w     *bcmrWriter
	block *bchutil.Block
	stxos []blockchain.SpentTxOut

	// stxoStarts is, for each transaction of the block, the position in
	// stxos of the output its first input spends.
	stxoStarts []int

	// locs and positions are where each transaction of the block is and
	// the position of each one in the block, made when first needed.
	locs      []wire.TxLoc
	positions map[chainhash.Hash]int

	// added and removed are the heads the block added and spent so far,
	// which with the heads of the index make the heads as of the
	// transaction being connected.
	added   map[chainhash.Hash]uint64
	removed map[chainhash.Hash]struct{}

	// blocks are the blocks read to find authbases in.
	blocks bcmrBlockCache

	// firstAt is the position of the first of the block's transactions that
	// blockMatters found to matter, and first what it parsed of it.  It
	// stopped there, and those before it hold no publication and create no
	// category.
	firstAt int
	first   bcmrParsedTx
}

// head returns the segment of which hash is the current head, if it is one.
func (c *bcmrConnector) head(hash chainhash.Hash) (uint64, bool) {
	if segment, ok := c.added[hash]; ok {
		return segment, true
	}
	if _, ok := c.removed[hash]; ok {
		return 0, false
	}
	segment, ok := c.idx.heads[hash]
	return segment, ok
}

// addHead makes hash the head of a segment, watching its identity output.
func (c *bcmrConnector) addHead(hash chainhash.Hash, segment uint64) error {
	c.added[hash] = segment
	delete(c.removed, hash)
	return c.w.put(bcmrHeads, hash[:], bcmrSegmentKey(segment))
}

// removeHead stops watching the identity output of hash, which the block
// spent.
func (c *bcmrConnector) removeHead(hash chainhash.Hash) error {
	delete(c.added, hash)
	c.removed[hash] = struct{}{}
	return c.w.delete(bcmrHeads, hash[:])
}

// link returns the link with the passed txid.
func (c *bcmrConnector) link(hash chainhash.Hash) (*bcmrLink, error) {
	return c.w.link(hash)
}

// putLink stores a link.
func (c *bcmrConnector) putLink(hash chainhash.Hash, l *bcmrLink) error {
	return c.w.put(bcmrLinks, hash[:], serializeBcmrLink(l))
}

// segment returns the segment with the passed ID.
func (c *bcmrConnector) segment(id uint64) (*bcmrSegment, error) {
	return c.w.segment(id)
}

// putSegment stores a segment.
func (c *bcmrConnector) putSegment(id uint64, s *bcmrSegment) error {
	return c.w.putSegment(id, s)
}

// leaveForWalker leaves the segment with the passed ID for the walker to walk
// behind, for a publication in the block at height.
func (c *bcmrConnector) leaveForWalker(height int32, segment uint64) error {
	return c.w.put(bcmrPending, bcmrPendingKey(height, segment), bcmrPendingValue)
}

// newSegment returns the ID for a new segment.
func (c *bcmrConnector) newSegment() (uint64, error) {
	return c.w.newSegment()
}

// putIdentity stores an identity, which must be new.
func (c *bcmrConnector) putIdentity(authbase chainhash.Hash, id *bcmrIdentity) error {
	if c.w.get(bcmrIdentities, authbase[:]) != nil {
		return AssertError(fmt.Sprintf("BCMR identity %v already exists", authbase))
	}
	return c.w.put(bcmrIdentities, authbase[:], serializeBcmrIdentity(id))
}

// bcmrTxPositions returns the position of each of a block's transactions, by
// txid.
func bcmrTxPositions(txs []*bchutil.Tx) map[chainhash.Hash]int {
	positions := make(map[chainhash.Hash]int, len(txs))
	for i, tx := range txs {
		positions[*tx.Hash()] = i
	}
	return positions
}

// bcmrStxoStarts returns where the spent outputs of each of a block's
// transactions start in its spend journal, which lists them in the order of
// the transactions and their inputs, the coinbase spending none, and how many
// spent outputs the block has.
func bcmrStxoStarts(txs []*bchutil.Tx) ([]int, int) {
	starts := make([]int, len(txs))
	n := 0
	for k, tx := range txs {
		starts[k] = n
		if k > 0 {
			n += len(tx.MsgTx().TxIn)
		}
	}
	return starts, n
}

// txLoc returns where the transaction at position i of the block is.
func (c *bcmrConnector) txLoc(i int) (wire.TxLoc, error) {
	if c.locs == nil {
		locs, err := blockTxLocs(c.block)
		if err != nil {
			return wire.TxLoc{}, err
		}
		c.locs = locs
	}
	return c.locs[i], nil
}

// position returns the position in the block of the transaction with the
// passed txid, if the block has it.
func (c *bcmrConnector) position(hash chainhash.Hash) (int, bool) {
	if c.positions == nil {
		c.positions = bcmrTxPositions(c.block.Transactions())
	}
	i, ok := c.positions[hash]
	return i, ok
}

// spentOutput returns the output that input j of the transaction at position i
// of the block spends.  While the index catches up, the index manager hands it
// the spent outputs only if another index needs them, and otherwise it reads
// them from the spend journal the first time it needs one.
func (c *bcmrConnector) spentOutput(i, j int) (*blockchain.SpentTxOut, error) {
	if c.stxos == nil {
		if c.idx.chain == nil {
			return nil, AssertError("the BCMR index has no chain to read spent outputs from")
		}
		stxos, err := c.idx.chain.FetchSpendJournal(c.block)
		if err != nil {
			return nil, err
		}
		c.stxos = stxos
	}
	if c.stxoStarts == nil {
		starts, n := bcmrStxoStarts(c.block.Transactions())
		if n != len(c.stxos) {
			return nil, AssertError(fmt.Sprintf("block %v spends %d outputs, "+
				"but has %d spent outputs", c.block.Hash(), n, len(c.stxos)))
		}
		c.stxoStarts = starts
	}
	return &c.stxos[c.stxoStarts[i]+j], nil
}

// fetchBlock returns the block of the main chain at a height.
func (c *bcmrConnector) fetchBlock(height int32) (*bcmrReadBlock, error) {
	if c.idx.chain == nil {
		return nil, AssertError("the BCMR index has no chain to read blocks from")
	}
	hash, err := c.idx.chain.BlockHashByHeight(height)
	if err != nil {
		return nil, err
	}
	return c.blocks.fetch(c.dbTx, hash, height)
}

// authbaseLink returns the link of the authbase of a category that the
// transaction at position i of the block creates, which the index doesn't
// track yet, and whether the authbase spends an identity output, which is of a
// transaction the index doesn't track either.  Its height is that of the
// output the genesis input spends, and the block at that height holds it.
func (c *bcmrConnector) authbaseLink(g bcmr.Genesis, i int) (*bcmrLink, bool, error) {
	block := c.block
	var loc wire.TxLoc
	pos, ok := c.position(g.Category)
	if ok {
		var err error
		loc, err = c.txLoc(pos)
		if err != nil {
			return nil, false, err
		}
	} else {
		stxo, err := c.spentOutput(i, g.Input)
		if err != nil {
			return nil, false, err
		}
		read, err := c.fetchBlock(stxo.Height)
		if err != nil {
			return nil, false, err
		}
		block = read.block
		pos, ok = read.positions[g.Category]
		if !ok {
			return nil, false, AssertError(fmt.Sprintf("authbase %v is not in "+
				"block %v", g.Category, block.Hash()))
		}
		loc = read.locs[pos]
	}
	msgTx := block.Transactions()[pos].MsgTx()
	return &bcmrLink{
		blockHash: *block.Hash(),
		height:    block.Height(),
		offset:    uint32(loc.TxStart),
		length:    uint32(loc.TxLen),
		pub:       bcmr.ParsePublication(msgTx),
	}, bcmrSpendsIdentity(msgTx), nil
}

// bcmrMaxReadBlocks is the serialized size of the blocks that a bcmrBlockCache
// keeps, unless it has a limit of its own.  Decoded, they take several times
// that.
var bcmrMaxReadBlocks = 256 << 20

// bcmrMinWalkBlocks is the least serialized size of the blocks that the
// walker's cache keeps, which is twice the largest block the network allows
// now if that's more.  It lives for a whole pass, alongside the cache of the
// block being connected, so it keeps less than that, but at least two of the
// largest blocks, as walks near one another share the blocks they read.
var bcmrMinWalkBlocks = 64 << 20

// bcmrReadBlock is a block that the index read to find transactions in, with
// what finding each of them takes, so that finding many in a block doesn't
// take time for each in proportion to the block.
type bcmrReadBlock struct {
	block *bchutil.Block

	// positions has the position of each transaction in the block, and
	// locs where it is in the serialized block.
	positions map[chainhash.Hash]int
	locs      []wire.TxLoc

	// stxos is the block's spend journal, and stxoStarts where each
	// transaction's spent outputs start in it, once they were asked for.
	stxos      []blockchain.SpentTxOut
	stxoStarts []int
}

// bcmrBlockCache keeps the blocks read while connecting a block or walking.  It
// starts over once they pass its limit, or bcmrMaxReadBlocks without one, so
// that a block whose transactions reach into many others needs no more memory
// than that.
type bcmrBlockCache struct {
	blocks map[chainhash.Hash]*bcmrReadBlock
	size   int
	limit  int
}

// max returns the serialized size of the blocks the cache keeps.
func (c *bcmrBlockCache) max() int {
	if c.limit > 0 {
		return c.limit
	}
	return bcmrMaxReadBlocks
}

// fetch returns the block with the passed hash, at the passed height, reading
// it from the database unless the cache has it.
func (c *bcmrBlockCache) fetch(dbTx database.Tx, hash *chainhash.Hash, height int32) (*bcmrReadBlock, error) {
	if read, ok := c.blocks[*hash]; ok {
		return read, nil
	}
	raw, err := dbTx.FetchBlock(hash)
	if err != nil {
		return nil, err
	}
	block, err := bchutil.NewBlockFromBytes(raw)
	if err != nil {
		return nil, err
	}
	block.SetHeight(height)
	locs, err := blockTxLocs(block)
	if err != nil {
		return nil, err
	}
	read := &bcmrReadBlock{
		block:     block,
		positions: bcmrTxPositions(block.Transactions()),
		locs:      locs,
	}
	if c.blocks == nil || c.size+len(raw) > c.max() {
		c.blocks = make(map[chainhash.Hash]*bcmrReadBlock)
		c.size = 0
	}
	c.blocks[*hash] = read
	c.size += len(raw)
	return read, nil
}

// bcmrSpendsIdentity reports whether a transaction spends an identity output.
func bcmrSpendsIdentity(msgTx *wire.MsgTx) bool {
	for _, in := range msgTx.TxIn {
		if in.PreviousOutPoint.Index == 0 {
			return true
		}
	}
	return false
}

// addBcmrFound returns the sum of two counts of transactions the walker
// added, which stops at bcmrMaxFound+1, standing for any more.
func addBcmrFound(a, b uint32) uint32 {
	return min(a+b, bcmrMaxFound+1)
}

// connectTx adds the transaction at position i of the block to the index if
// it spends the identity output of an authhead, creates a token category, or
// has a publication.  One that has a publication and does neither starts an
// authchain of its own.
func (c *bcmrConnector) connectTx(i int, tokensActive bool) error {
	tx := c.block.Transactions()[i]
	msgTx := tx.MsgTx()

	var spent []chainhash.Hash
	identityInputs := 0
	for _, in := range msgTx.TxIn {
		if in.PreviousOutPoint.Index != 0 {
			continue
		}
		identityInputs++
		if _, ok := c.head(in.PreviousOutPoint.Hash); ok {
			spent = append(spent, in.PreviousOutPoint.Hash)
		}
	}
	var parsed bcmrParsedTx
	switch {
	case i < c.firstAt:
		parsed = bcmrParsedTx{pubRead: true, genesesRead: tokensActive}
	case i == c.firstAt:
		parsed = c.first
	}
	var geneses []bcmr.Genesis
	if tokensActive {
		geneses = parsed.geneses
		if !parsed.genesesRead {
			geneses = bcmr.Geneses(msgTx)
		}
	}
	pub := parsed.pub
	if !parsed.pubRead {
		pub = bcmr.ParsePublication(msgTx)
	}
	if len(spent) == 0 && len(geneses) == 0 && pub == nil {
		return nil
	}

	txHash := *tx.Hash()
	loc, err := c.txLoc(i)
	if err != nil {
		return err
	}
	link := &bcmrLink{
		blockHash: *c.block.Hash(),
		height:    c.block.Height(),
		offset:    uint32(loc.TxStart),
		length:    uint32(loc.TxLen),
		pub:       pub,
	}
	burned := bcmr.IsBurn(msgTx)

	// The categories whose authbase the index doesn't track yet each start
	// an authchain of their own, which comes into this transaction, as does
	// the segment of each authhead it spends.  Any other identity output it
	// spends is of a transaction the index doesn't track.
	var fresh []bcmr.Genesis
	isFresh := make(map[chainhash.Hash]struct{})
	for _, g := range geneses {
		if _, ok := c.head(g.Category); !ok {
			fresh = append(fresh, g)
			isFresh[g.Category] = struct{}{}
		}
	}
	segments := make([]uint64, 0, len(spent))
	for _, prev := range spent {
		segment, _ := c.head(prev)
		segments = append(segments, segment)
	}
	untracked := identityInputs - len(spent) - len(fresh)

	var segmentID uint64
	var complete bool
	switch {
	case len(segments) == 1 && len(fresh) == 0 && untracked == 0:
		// The transaction extends a segment.
		segmentID = segments[0]
		s, err := c.segment(segmentID)
		if err != nil {
			return err
		}
		link.segment, link.place = segmentID, s.length
		s.length++
		s.head = txHash
		s.notePublication(txHash, link.place, link.pub)
		s.burned = burned
		if err := c.putSegment(segmentID, s); err != nil {
			return err
		}
		complete = s.complete

	case len(segments) == 0 && len(fresh) == 1 && untracked == 0:
		// The transaction creates a token whose authbase starts a
		// segment, which this transaction continues.
		g := fresh[0]
		segmentID, err = c.newSegment()
		if err != nil {
			return err
		}
		auth, behind, err := c.authbaseLink(g, i)
		if err != nil {
			return err
		}
		auth.segment, auth.place, auth.next = segmentID, 0, txHash
		link.segment, link.place = segmentID, 1
		complete = !behind
		s := &bcmrSegment{length: 2, first: g.Category, head: txHash, burned: burned,
			complete: complete}
		s.notePublication(g.Category, 0, auth.pub)
		s.notePublication(txHash, 1, link.pub)
		if err := c.putLink(g.Category, auth); err != nil {
			return err
		}
		if err := c.putSegment(segmentID, s); err != nil {
			return err
		}
		if auth.pub != nil && !complete {
			if err := c.leaveForWalker(auth.height, segmentID); err != nil {
				return err
			}
		}
		err = c.putIdentity(g.Category, &bcmrIdentity{
			segment: segmentID, supply: g.Supply,
		})
		if err != nil {
			return err
		}

	default:
		// Authchains join at the transaction, or it meets one the index
		// doesn't track, or its publication starts one: it starts a
		// segment, which each segment it spends the authhead of continues
		// into.  The segment is complete if each of those is and the
		// transaction spends no identity output the index doesn't track.
		segmentID, err = c.newSegment()
		if err != nil {
			return err
		}
		link.segment, link.place = segmentID, 0
		s := &bcmrSegment{length: 1, first: txHash, head: txHash, burned: burned,
			complete: untracked == 0}
		s.notePublication(txHash, 0, link.pub)
		for _, id := range segments {
			prevSegment, err := c.segment(id)
			if err != nil {
				return err
			}
			prevSegment.next = segmentID
			if err := c.putSegment(id, prevSegment); err != nil {
				return err
			}
			s.complete = s.complete && prevSegment.complete
			s.found = addBcmrFound(s.found, prevSegment.found)
		}
		for _, g := range fresh {
			authSegmentID, err := c.newSegment()
			if err != nil {
				return err
			}
			auth, behind, err := c.authbaseLink(g, i)
			if err != nil {
				return err
			}
			auth.segment, auth.place, auth.next = authSegmentID, 0, txHash
			authSegment := &bcmrSegment{
				length: 1, first: g.Category, head: g.Category,
				next: segmentID, complete: !behind,
			}
			authSegment.notePublication(g.Category, 0, auth.pub)
			if err := c.putLink(g.Category, auth); err != nil {
				return err
			}
			if err := c.putSegment(authSegmentID, authSegment); err != nil {
				return err
			}
			if auth.pub != nil && behind {
				err := c.leaveForWalker(auth.height, authSegmentID)
				if err != nil {
					return err
				}
			}
			err = c.putIdentity(g.Category, &bcmrIdentity{
				segment: authSegmentID, supply: g.Supply,
			})
			if err != nil {
				return err
			}
			s.complete = s.complete && !behind
		}
		if !s.complete {
			s.found = 0
		}
		if err := c.putSegment(segmentID, s); err != nil {
			return err
		}
		complete = s.complete
	}

	// The authheads the transaction spent now lead to it, and their
	// identity outputs are spent.
	for _, prev := range spent {
		prevLink, err := c.link(prev)
		if err != nil {
			return err
		}
		prevLink.next = txHash
		if err := c.putLink(prev, prevLink); err != nil {
			return err
		}
		if err := c.removeHead(prev); err != nil {
			return err
		}
	}

	// A category whose authbase was an authhead starts where that is.
	for _, g := range geneses {
		if _, ok := isFresh[g.Category]; ok {
			continue
		}
		auth, err := c.link(g.Category)
		if err != nil {
			return err
		}
		err = c.putIdentity(g.Category, &bcmrIdentity{
			segment: auth.segment, place: auth.place, supply: g.Supply,
		})
		if err != nil {
			return err
		}
	}

	if err := c.putLink(txHash, link); err != nil {
		return err
	}
	if link.pub != nil && !complete {
		if err := c.leaveForWalker(c.block.Height(), segmentID); err != nil {
			return err
		}
	}
	if burned {
		return nil
	}
	return c.addHead(txHash, segmentID)
}

// finish stores the block's undo record and updates the heads of the index.
func (c *bcmrConnector) finish() error {
	if len(c.w.undo) > 0 {
		err := c.w.buckets[bcmrUndo].Put(bcmrUndoKey(c.block.Height()),
			serializeBcmrUndo(c.w.undo))
		if err != nil {
			return err
		}
	}
	for head := range c.removed {
		delete(c.idx.heads, head)
	}
	for head, segment := range c.added {
		c.idx.heads[head] = segment
	}
	return nil
}
