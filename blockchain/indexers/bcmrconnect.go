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
// identity output it spends.
func bcmrTxOrder(txs []*bchutil.Tx) []int {
	positions := make(map[chainhash.Hash]int, len(txs))
	for i, tx := range txs {
		positions[*tx.Hash()] = i
	}
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
	blocks map[chainhash.Hash]*bchutil.Block
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
	raw := c.w.get(bcmrLinks, hash[:])
	if raw == nil {
		return nil, AssertError(fmt.Sprintf("BCMR link %v is missing", hash))
	}
	return deserializeBcmrLink(raw)
}

// putLink stores a link.
func (c *bcmrConnector) putLink(hash chainhash.Hash, l *bcmrLink) error {
	return c.w.put(bcmrLinks, hash[:], serializeBcmrLink(l))
}

// segment returns the segment with the passed ID.
func (c *bcmrConnector) segment(id uint64) (*bcmrSegment, error) {
	raw := c.w.get(bcmrSegments, bcmrSegmentKey(id))
	if raw == nil {
		return nil, AssertError(fmt.Sprintf("BCMR segment %d is missing", id))
	}
	return deserializeBcmrSegment(raw)
}

// putSegment stores a segment.
func (c *bcmrConnector) putSegment(id uint64, s *bcmrSegment) error {
	return c.w.put(bcmrSegments, bcmrSegmentKey(id), serializeBcmrSegment(s))
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
		txs := c.block.Transactions()
		c.positions = make(map[chainhash.Hash]int, len(txs))
		for i, tx := range txs {
			c.positions[*tx.Hash()] = i
		}
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
		txs := c.block.Transactions()
		starts := make([]int, len(txs))
		n := 0
		for k, tx := range txs {
			starts[k] = n
			if k > 0 {
				n += len(tx.MsgTx().TxIn)
			}
		}
		if n != len(c.stxos) {
			return nil, AssertError(fmt.Sprintf("block %v spends %d outputs, "+
				"but has %d spent outputs", c.block.Hash(), n, len(c.stxos)))
		}
		c.stxoStarts = starts
	}
	return &c.stxos[c.stxoStarts[i]+j], nil
}

// fetchBlock returns the block of the main chain at a height.
func (c *bcmrConnector) fetchBlock(height int32) (*bchutil.Block, error) {
	if c.idx.chain == nil {
		return nil, AssertError("the BCMR index has no chain to read blocks from")
	}
	hash, err := c.idx.chain.BlockHashByHeight(height)
	if err != nil {
		return nil, err
	}
	if block, ok := c.blocks[*hash]; ok {
		return block, nil
	}
	raw, err := c.dbTx.FetchBlock(hash)
	if err != nil {
		return nil, err
	}
	block, err := bchutil.NewBlockFromBytes(raw)
	if err != nil {
		return nil, err
	}
	block.SetHeight(height)
	if c.blocks == nil {
		c.blocks = make(map[chainhash.Hash]*bchutil.Block)
	}
	c.blocks[*hash] = block
	return block, nil
}

// authbaseLink returns the link of the authbase of a category that the
// transaction at position i of the block creates, which the index doesn't
// track yet.  Its height is that of the output the genesis input spends, and
// the block at that height holds it.
func (c *bcmrConnector) authbaseLink(g bcmr.Genesis, i int) (*bcmrLink, error) {
	block := c.block
	pos, ok := c.position(g.Category)
	if !ok {
		stxo, err := c.spentOutput(i, g.Input)
		if err != nil {
			return nil, err
		}
		block, err = c.fetchBlock(stxo.Height)
		if err != nil {
			return nil, err
		}
		pos = -1
		for k, tx := range block.Transactions() {
			if *tx.Hash() == g.Category {
				pos = k
				break
			}
		}
		if pos < 0 {
			return nil, AssertError(fmt.Sprintf("authbase %v is not in "+
				"block %v", g.Category, block.Hash()))
		}
	}

	var loc wire.TxLoc
	if block == c.block {
		var err error
		loc, err = c.txLoc(pos)
		if err != nil {
			return nil, err
		}
	} else {
		locs, err := blockTxLocs(block)
		if err != nil {
			return nil, err
		}
		loc = locs[pos]
	}
	return &bcmrLink{
		blockHash: *block.Hash(),
		height:    block.Height(),
		offset:    uint32(loc.TxStart),
		length:    uint32(loc.TxLen),
		pub:       bcmr.ParsePublication(block.Transactions()[pos].MsgTx()),
	}, nil
}

// connectTx adds the transaction at position i of the block to the index if
// it spends the identity output of an authhead or creates a token category.
func (c *bcmrConnector) connectTx(i int, tokensActive bool) error {
	tx := c.block.Transactions()[i]
	msgTx := tx.MsgTx()

	var spent []chainhash.Hash
	for _, in := range msgTx.TxIn {
		if in.PreviousOutPoint.Index != 0 {
			continue
		}
		if _, ok := c.head(in.PreviousOutPoint.Hash); ok {
			spent = append(spent, in.PreviousOutPoint.Hash)
		}
	}
	var geneses []bcmr.Genesis
	if tokensActive {
		geneses = bcmr.Geneses(msgTx)
	}
	if len(spent) == 0 && len(geneses) == 0 {
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
		pub:       bcmr.ParsePublication(msgTx),
	}
	burned := bcmr.IsBurn(msgTx)

	// The categories whose authbase the index doesn't track yet each start
	// an authchain of their own, which comes into this transaction, as does
	// the segment of each authhead it spends.
	var fresh []bcmr.Genesis
	for _, g := range geneses {
		if _, ok := c.head(g.Category); !ok {
			fresh = append(fresh, g)
		}
	}
	segments := make([]uint64, 0, len(spent))
	for _, prev := range spent {
		segment, _ := c.head(prev)
		segments = append(segments, segment)
	}

	var segmentID uint64
	switch {
	case len(segments) == 1 && len(fresh) == 0:
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

	case len(segments) == 0 && len(fresh) == 1:
		// The transaction creates a token whose authbase starts a
		// segment, which this transaction continues.
		g := fresh[0]
		segmentID, err = c.newSegment()
		if err != nil {
			return err
		}
		auth, err := c.authbaseLink(g, i)
		if err != nil {
			return err
		}
		auth.segment, auth.place, auth.next = segmentID, 0, txHash
		link.segment, link.place = segmentID, 1
		s := &bcmrSegment{length: 2, first: g.Category, head: txHash, burned: burned}
		s.notePublication(g.Category, 0, auth.pub)
		s.notePublication(txHash, 1, link.pub)
		if err := c.putLink(g.Category, auth); err != nil {
			return err
		}
		if err := c.putSegment(segmentID, s); err != nil {
			return err
		}
		err = c.putIdentity(g.Category, &bcmrIdentity{
			segment: segmentID, supply: g.Supply,
		})
		if err != nil {
			return err
		}

	default:
		// Authchains join at the transaction: it starts a segment,
		// which each of them continues into.
		segmentID, err = c.newSegment()
		if err != nil {
			return err
		}
		link.segment, link.place = segmentID, 0
		s := &bcmrSegment{length: 1, first: txHash, head: txHash, burned: burned}
		s.notePublication(txHash, 0, link.pub)
		if err := c.putSegment(segmentID, s); err != nil {
			return err
		}
		for _, id := range segments {
			prevSegment, err := c.segment(id)
			if err != nil {
				return err
			}
			prevSegment.next = segmentID
			if err := c.putSegment(id, prevSegment); err != nil {
				return err
			}
		}
		for _, g := range fresh {
			authSegmentID, err := c.newSegment()
			if err != nil {
				return err
			}
			auth, err := c.authbaseLink(g, i)
			if err != nil {
				return err
			}
			auth.segment, auth.place, auth.next = authSegmentID, 0, txHash
			authSegment := &bcmrSegment{
				length: 1, first: g.Category, head: g.Category,
				next: segmentID,
			}
			authSegment.notePublication(g.Category, 0, auth.pub)
			if err := c.putLink(g.Category, auth); err != nil {
				return err
			}
			if err := c.putSegment(authSegmentID, authSegment); err != nil {
				return err
			}
			err = c.putIdentity(g.Category, &bcmrIdentity{
				segment: authSegmentID, supply: g.Supply,
			})
			if err != nil {
				return err
			}
		}
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
		if containsBcmrGenesis(fresh, g.Category) {
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
	if burned {
		return nil
	}
	return c.addHead(txHash, segmentID)
}

// containsBcmrGenesis returns whether geneses creates the category.
func containsBcmrGenesis(geneses []bcmr.Genesis, category chainhash.Hash) bool {
	for _, g := range geneses {
		if g.Category == category {
			return true
		}
	}
	return false
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
