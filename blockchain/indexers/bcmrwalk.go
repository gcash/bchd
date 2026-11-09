// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

// The walker adds to the index every transaction behind each publication that
// the index doesn't track yet, so that each of them answers as an authbase.
// The transactions behind a link are those whose identity output it spends,
// and those behind them in turn.  Each link of a segment but the first spends
// only the identity output of the link before it, so what is behind any link
// of a segment is behind its first link, and the segment is complete once the
// index tracks all of that.  Connecting a block leaves each segment that gets
// a publication and isn't complete for the walker, and the chain's
// notification of the block wakes the walker once the block is committed.
//
// The walker reads the first link of a segment it is left, and goes behind it
// to each transaction whose identity output that spends.  From a link the
// index tracks, it goes on to the first link of that link's segment, unless
// the segment is complete.  A transaction the index doesn't track it finds in
// the block at the height that the spend journal of the spending
// transaction's block gives, and goes behind too.  It stops at transactions
// that spend no identity output.  Each transaction it found becomes a link:
// after the one whose identity output it spends, if that is its only one and
// was found too, and else first in a segment of its own, which continues into
// the segment of the transaction that spends its identity output.  That is
// always a segment's first link, since connecting a link that spends the
// identity output of a transaction the index doesn't track starts a segment.
// Every segment the walker went behind is then complete.
//
// The walker adds nothing behind a publication with more than bcmrMaxFound
// transactions behind it that the index doesn't otherwise track.  Each
// complete segment counts the transactions in and behind it that the walker
// added, so whether a publication has too many behind it is the same whatever
// order the walker goes behind publications in.  A segment with too many is
// marked complete with a count above the limit, which every segment that joins
// it later takes over, so the walker reads such a history once.
//
// The walker reads with the database unlocked, and writes what it found
// behind each segment it is left in one database transaction, once it checks
// that the segments it read are as they were and the blocks it found
// transactions in are still in the main chain; otherwise it reads again.  It
// adds what it writes to the undo records of the index's tip, so that
// disconnecting that block undoes it and leaves the segment for the walker
// again.

const (
	// bcmrWalkRetry is how long the walker waits after it failed before it
	// tries again, and bcmrWalkAgainDelay how long it waits after the chain
	// changed what a walk read.
	bcmrWalkRetry      = time.Minute
	bcmrWalkAgainDelay = time.Second

	// bcmrWalkLogInterval is how often the walker logs its progress through
	// a long run.
	bcmrWalkLogInterval = 30 * time.Second
)

var (
	// bcmrMaxFound is the most transactions behind a publication, other
	// than those the index tracks for their own sake, that the walker adds;
	// it adds none behind a publication with more.  On mainnet at height
	// 975,700, no publication had more than 2,632.  Tests lower it.
	bcmrMaxFound uint32 = 100000

	// bcmrPendingValue is the value of each entry of the bucket bcmrPending.
	bcmrPendingValue = []byte{1}

	// errBcmrWalkAgain is returned when the chain changed what a walk read
	// before it was written.
	errBcmrWalkAgain = errors.New("the chain changed what the BCMR walker read")
)

// StartWalker starts the walker, which walks behind the segments that
// connecting a block leaves for it, in the background.
func (idx *BcmrIndex) StartWalker() {
	idx.wg.Add(1)
	go idx.walker()
	idx.signalWalker()
}

// StopWalker stops the walker and waits for it to stop.
func (idx *BcmrIndex) StopWalker() {
	close(idx.quit)
	idx.wg.Wait()
}

// signalWalker wakes the walker.
func (idx *BcmrIndex) signalWalker() {
	select {
	case idx.walkSignal <- struct{}{}:
	default:
	}
}

// handleChainNotification wakes the walker once the chain has committed a
// block it connected or disconnected, either of which can leave segments for
// the walker.  A walker woken with none left only finds that out.
func (idx *BcmrIndex) handleChainNotification(n *blockchain.Notification) {
	if n.Type == blockchain.NTBlockConnected || n.Type == blockchain.NTBlockDisconnected {
		idx.signalWalker()
	}
}

// walker walks behind the segments left for it each time it is woken.
func (idx *BcmrIndex) walker() {
	defer idx.wg.Done()
	for {
		select {
		case <-idx.quit:
			return
		case <-idx.walkSignal:
		}

		err := idx.walkPending()
		var delay time.Duration
		switch {
		case err == nil:
			continue
		case errors.Is(err, errInterruptRequested):
			return
		case errors.Is(err, errBcmrWalkAgain):
			delay = bcmrWalkAgainDelay
		default:
			log.Errorf("Unable to walk behind BCMR publications, trying "+
				"again in %v: %v", bcmrWalkRetry, err)
			delay = bcmrWalkRetry
		}
		select {
		case <-idx.quit:
			return
		case <-time.After(delay):
			idx.signalWalker()
		}
	}
}

// walkPending walks behind the segments left for the walker, one at a time,
// until none are left.
func (idx *BcmrIndex) walkPending() error {
	start := time.Now()
	lastLog := start
	var walks, found int

	// Walks behind segments near one another read the same blocks, which
	// stay as they are whatever becomes of the chain, as a hash names each.
	var blocks bcmrBlockCache
	for {
		walk, err := idx.readBcmrWalk(&blocks)
		if err != nil {
			return err
		}
		if walk == nil {
			break
		}
		if err := idx.writeBcmrWalk(walk); err != nil {
			return err
		}
		walks++
		if walk.over {
			log.Infof("Added none of the more than %d transactions behind "+
				"the BCMR publications of segment %d", bcmrMaxFound,
				walk.root.id)
		} else {
			found += len(walk.found)
		}
		if time.Since(lastLog) >= bcmrWalkLogInterval {
			log.Infof("Walking behind BCMR publications: %d transactions "+
				"found so far", found)
			lastLog = time.Now()
		}
	}
	if walks == 0 {
		return nil
	}
	took := time.Since(start).Round(time.Millisecond)
	if took >= bcmrWalkLogInterval {
		log.Infof("Found %d transactions behind BCMR publications in %v", found, took)
	} else {
		log.Debugf("Found %d transactions behind BCMR publications in %v", found, took)
	}
	return nil
}

// bcmrWalk is a walk behind one segment left for the walker: what it read,
// with the database unlocked, and what it found.
type bcmrWalk struct {
	idx  *BcmrIndex
	dbTx database.Tx

	// tipHash and tipHeight are the index's tip as the walk read it.
	tipHash   chainhash.Hash
	tipHeight int32

	// pendingKey is the key of the segment in bcmrPending, and root the
	// segment.
	pendingKey []byte
	root       *bcmrWalkSegment

	// segments are the segments the walk read, by ID, and expanded those it
	// went behind, in the order it did, each after the one it reached it
	// from.
	segments map[uint64]*bcmrWalkSegment
	expanded []*bcmrWalkSegment

	// found are the transactions the walk found, in the order it found
	// them, each after the one it was found from.
	found []*bcmrWalkTx

	// total counts the transactions behind the root that the walker added
	// or adds: those the walk found and those behind the complete segments
	// it reached.  over reports that there are more than bcmrMaxFound,
	// when the walk stopped.
	total uint32
	over  bool

	// blocks are the blocks the walk read, which the walks of one pass share.
	blocks *bcmrBlockCache
}

// bcmrWalkSegment is a segment that a walk read.
type bcmrWalkSegment struct {
	id uint64

	// s is the segment as the walk read it.
	s *bcmrSegment

	// parents are the segments and found transactions whose identity
	// outputs the segment's first link spends, once the walk went behind
	// it, and count how many transactions the walker added behind it.
	parents []bcmrWalkParent
	count   uint32
}

// bcmrWalkParent is a segment, or a found transaction, behind a segment's
// first link.
type bcmrWalkParent struct {
	segment *bcmrWalkSegment
	tx      *bcmrWalkTx
}

// bcmrWalkTx is a transaction that a walk found.
type bcmrWalkTx struct {
	hash chainhash.Hash

	// link is the transaction as a link, with where it is and its
	// publication, and msgTx the transaction until the walk goes behind it,
	// and pos its position in its block.
	link  *bcmrLink
	msgTx *wire.MsgTx
	pos   int

	// child is the transaction that spends its identity output, and parents
	// the found transactions whose identity outputs it spends.
	child   chainhash.Hash
	parents []*bcmrWalkTx

	// count is how many transactions the walker adds in and behind it, and
	// segment the segment it is in, once the walk is written.
	count   uint32
	segment *bcmrWalkNewSegment
}

// bcmrWalkNewSegment is a segment of found transactions.
type bcmrWalkNewSegment struct {
	id   uint64
	s    *bcmrSegment
	head *bcmrWalkTx
}

// readBcmrWalk reads the walk behind the first segment left for the walker,
// with the database unlocked, or returns nil if none is left.  It reads blocks
// through the passed cache, which keeps them for the next walk.
func (idx *BcmrIndex) readBcmrWalk(blocks *bcmrBlockCache) (*bcmrWalk, error) {
	if idx.chain == nil {
		return nil, AssertError("the BCMR index has no chain to walk")
	}
	var walk *bcmrWalk
	err := idx.db.View(func(dbTx database.Tx) error {
		w, err := newBcmrWriter(dbTx)
		if err != nil {
			return err
		}
		cursor := w.buckets[bcmrPending].Cursor()
		if !cursor.First() {
			return nil
		}
		key := bytes.Clone(cursor.Key())
		if len(key) != len(bcmrPendingKey(0, 0)) {
			return errDeserialize("BCMR pending segment has the wrong size")
		}
		tipHash, tipHeight, err := dbFetchIndexerTip(dbTx, bcmrIndexKey)
		if err != nil {
			return err
		}
		walk = &bcmrWalk{
			idx:        idx,
			dbTx:       dbTx,
			tipHash:    *tipHash,
			tipHeight:  tipHeight,
			pendingKey: key,
			segments:   make(map[uint64]*bcmrWalkSegment),
			blocks:     blocks,
		}
		return walk.read(w, binary.BigEndian.Uint64(key[4:]))
	})
	if err != nil && walk != nil && !errors.Is(err, errInterruptRequested) {
		// A reorganization since the walk began can remove what it
		// reads.
		mainHash, mainErr := idx.chain.BlockHashByHeight(walk.tipHeight)
		if mainErr != nil || *mainHash != walk.tipHash {
			return nil, errBcmrWalkAgain
		}
	}
	if err != nil {
		return nil, err
	}
	if walk != nil {
		walk.dbTx, walk.blocks = nil, nil
	}
	return walk, nil
}

// read goes behind the segment with the passed ID, and behind everything it
// reaches in turn, until there is nothing left to go behind or more than
// bcmrMaxFound transactions are behind it.
func (walk *bcmrWalk) read(w *bcmrWriter, rootID uint64) error {
	root, err := walk.readSegment(w, rootID)
	if err != nil {
		return err
	}
	walk.root = root
	if root.s.complete {
		return nil
	}

	segments := []*bcmrWalkSegment{root}
	var txs []*bcmrWalkTx
	for len(segments) > 0 || len(txs) > 0 {
		if interruptRequested(walk.idx.quit) {
			return errInterruptRequested
		}
		var err error
		if n := len(txs); n > 0 {
			tx := txs[n-1]
			txs = txs[:n-1]
			txs, err = walk.expandTx(w, tx, txs)
		} else {
			s := segments[len(segments)-1]
			segments = segments[:len(segments)-1]
			segments, txs, err = walk.expandSegment(w, s, segments, txs)
		}
		if err != nil || walk.over {
			return err
		}
	}
	return nil
}

// readSegment returns the segment with the passed ID, as the walk read it.
func (walk *bcmrWalk) readSegment(w *bcmrWriter, id uint64) (*bcmrWalkSegment, error) {
	if ws, ok := walk.segments[id]; ok {
		return ws, nil
	}
	s, err := w.segment(id)
	if err != nil {
		return nil, err
	}
	ws := &bcmrWalkSegment{id: id, s: s}
	walk.segments[id] = ws
	return ws, nil
}

// count adds n to the transactions behind the root, and stops the walk once
// they are more than bcmrMaxFound.
func (walk *bcmrWalk) count(n uint32) {
	walk.total = addBcmrFound(walk.total, n)
	walk.over = walk.total > bcmrMaxFound
}

// expandSegment goes behind the segment ws, from its first link, and returns
// the segments and found transactions to go behind in turn.
func (walk *bcmrWalk) expandSegment(w *bcmrWriter, ws *bcmrWalkSegment,
	segments []*bcmrWalkSegment, txs []*bcmrWalkTx) ([]*bcmrWalkSegment, []*bcmrWalkTx, error) {

	walk.expanded = append(walk.expanded, ws)
	first, err := w.link(ws.s.first)
	if err != nil {
		return nil, nil, err
	}
	raw, err := walk.dbTx.FetchBlockRegion(&database.BlockRegion{
		Hash: &first.blockHash, Offset: first.offset, Len: first.length,
	})
	if err != nil {
		return nil, nil, err
	}
	msgTx, err := deserializeBcmrTx(raw)
	if err != nil {
		return nil, nil, err
	}

	var spent []blockchain.SpentTxOut
	for j, in := range msgTx.TxIn {
		prev := in.PreviousOutPoint
		if prev.Index != 0 {
			continue
		}

		// A link the index tracks is the head of its segment, as the
		// first link spends its identity output.
		if raw := w.get(bcmrLinks, prev.Hash[:]); raw != nil {
			l, err := deserializeBcmrLink(raw)
			if err != nil {
				return nil, nil, err
			}
			parent, err := walk.readSegment(w, l.segment)
			if err != nil {
				return nil, nil, err
			}
			ws.parents = append(ws.parents, bcmrWalkParent{segment: parent})
			if parent.s.complete {
				walk.count(parent.s.found)
			} else {
				segments = append(segments, parent)
			}
			continue
		}

		if spent == nil {
			spent, err = walk.spentOutputs(ws.s.first, first, -1, len(msgTx.TxIn))
			if err != nil {
				return nil, nil, err
			}
		}
		tx, err := walk.find(ws.s.first, prev.Hash, spent[j].Height)
		if err != nil {
			return nil, nil, err
		}
		ws.parents = append(ws.parents, bcmrWalkParent{tx: tx})
		txs = append(txs, tx)
		walk.count(1)
	}
	return segments, txs, nil
}

// expandTx goes behind the found transaction tx, and returns the found
// transactions to go behind in turn.
func (walk *bcmrWalk) expandTx(w *bcmrWriter, tx *bcmrWalkTx, txs []*bcmrWalkTx) ([]*bcmrWalkTx, error) {
	msgTx := tx.msgTx
	tx.msgTx = nil
	var spent []blockchain.SpentTxOut
	for j, in := range msgTx.TxIn {
		prev := in.PreviousOutPoint
		if prev.Index != 0 {
			continue
		}

		// The index tracks the transaction whose identity output each link
		// spends, so the transactions behind one it doesn't track are
		// untracked too.
		if w.get(bcmrLinks, prev.Hash[:]) != nil {
			return nil, AssertError(fmt.Sprintf("the BCMR index tracks %v "+
				"but not %v, which spends its identity output",
				prev.Hash, tx.hash))
		}
		if spent == nil {
			var err error
			spent, err = walk.spentOutputs(tx.hash, tx.link, tx.pos, len(msgTx.TxIn))
			if err != nil {
				return nil, err
			}
		}
		parent, err := walk.find(tx.hash, prev.Hash, spent[j].Height)
		if err != nil {
			return nil, err
		}
		tx.parents = append(tx.parents, parent)
		txs = append(txs, parent)
		walk.count(1)
	}
	return txs, nil
}

// spentOutputs returns the outputs that the inputs of the transaction with
// the passed hash spend, of which it has numIns, from the spend journal of its
// block, where l has it, at position pos, or wherever it is for a pos of -1.
func (walk *bcmrWalk) spentOutputs(hash chainhash.Hash, l *bcmrLink, pos, numIns int) ([]blockchain.SpentTxOut, error) {
	block, err := walk.fetchBlock(l.blockHash, l.height)
	if err != nil {
		return nil, err
	}
	if pos < 0 {
		var ok bool
		pos, ok = block.positions[hash]
		if !ok {
			pos = -1
		}
	}

	// The coinbase spends no output, and so has no spent outputs.
	if pos < 1 {
		return nil, AssertError(fmt.Sprintf("transaction %v spends no "+
			"output of block %v", hash, block.block.Hash()))
	}
	if block.stxoStarts == nil {
		block.stxos, err = walk.idx.chain.FetchSpendJournal(block.block)
		if err != nil {
			return nil, err
		}
		block.stxoStarts, _ = bcmrStxoStarts(block.block.Transactions())
	}
	start := block.stxoStarts[pos]
	if start+numIns > len(block.stxos) {
		return nil, AssertError(fmt.Sprintf("block %v has %d spent outputs, "+
			"fewer than its transactions spend", block.block.Hash(), len(block.stxos)))
	}
	return block.stxos[start : start+numIns], nil
}

// find returns the transaction with the passed hash, whose identity output the
// transaction child spends and which the index doesn't track, from the main
// chain's block at the passed height, which the spend journal gave.
func (walk *bcmrWalk) find(child, hash chainhash.Hash, height int32) (*bcmrWalkTx, error) {
	blockHash, err := walk.idx.chain.BlockHashByHeight(height)
	if err != nil {
		return nil, err
	}
	block, err := walk.fetchBlock(*blockHash, height)
	if err != nil {
		return nil, err
	}
	k, ok := block.positions[hash]
	if !ok {
		return nil, AssertError(fmt.Sprintf("transaction %v is not in block %v",
			hash, blockHash))
	}
	msgTx := block.block.Transactions()[k].MsgTx()
	found := &bcmrWalkTx{
		hash: hash,
		link: &bcmrLink{
			blockHash: *blockHash,
			height:    height,
			offset:    uint32(block.locs[k].TxStart),
			length:    uint32(block.locs[k].TxLen),
			pub:       bcmr.ParsePublication(msgTx),
		},
		msgTx: msgTx,
		pos:   k,
		child: child,
	}
	walk.found = append(walk.found, found)
	return found, nil
}

// fetchBlock returns the block with the passed hash and height.
func (walk *bcmrWalk) fetchBlock(hash chainhash.Hash, height int32) (*bcmrReadBlock, error) {
	return walk.blocks.fetch(walk.dbTx, &hash, height)
}

// writeBcmrWalk writes what a walk found, unless the chain changed what the
// walk read since, when it returns errBcmrWalkAgain.
func (idx *BcmrIndex) writeBcmrWalk(walk *bcmrWalk) error {
	again := false
	err := idx.db.Update(func(dbTx database.Tx) error {
		w, err := newBcmrWriter(dbTx)
		if err != nil {
			return err
		}
		unchanged, err := walk.unchanged(w)
		if err != nil {
			return err
		}
		if !unchanged {
			again = true
			return nil
		}
		if err := walk.write(w); err != nil {
			return err
		}

		// Disconnecting the index's tip undoes the walk, after anything the
		// walker wrote later, and before the block's own changes.
		_, height, err := dbFetchIndexerTip(dbTx, bcmrIndexKey)
		if err != nil {
			return err
		}
		key := nextBcmrWalkUndoKey(w.buckets[bcmrUndo], height)
		return w.buckets[bcmrUndo].Put(key, serializeBcmrUndo(w.undo))
	})
	if err == nil && again {
		return errBcmrWalkAgain
	}
	return err
}

// unchanged reports whether the segment is still left for the walker, which
// a disconnect of the block of the publication that left it undoes, whether
// the segments the walk read still start at the same links, and are as
// complete as they were, with the same counts, and whether the transactions
// it found are still untracked, in the main chain's blocks it found them in.
// A block that extended a segment since, or joined it to another, changed
// nothing behind it.
func (walk *bcmrWalk) unchanged(w *bcmrWriter) (bool, error) {
	if w.get(bcmrPending, walk.pendingKey) == nil {
		return false, nil
	}
	for id, ws := range walk.segments {
		raw := w.get(bcmrSegments, bcmrSegmentKey(id))
		if raw == nil {
			return false, nil
		}
		s, err := deserializeBcmrSegment(raw)
		if err != nil {
			return false, err
		}
		if s.first != ws.s.first || s.complete != ws.s.complete || s.found != ws.s.found {
			return false, nil
		}
	}
	for _, tx := range walk.found {
		if w.get(bcmrLinks, tx.hash[:]) != nil {
			return false, nil
		}
		mainHash, err := walk.idx.chain.BlockHashByHeight(tx.link.height)
		if err != nil || *mainHash != tx.link.blockHash {
			return false, nil
		}
	}
	return true, nil
}

// write writes what the walk found: the links of the transactions it found,
// in segments of their own, and every segment it went behind complete, with
// how many transactions the walker added in and behind it.  A walk with more
// than bcmrMaxFound behind the root writes the root complete, with a count
// above the limit, and nothing else.
func (walk *bcmrWalk) write(w *bcmrWriter) error {
	if err := w.delete(bcmrPending, walk.pendingKey); err != nil {
		return err
	}
	root := walk.root
	switch {
	case root.s.complete:
		return nil

	case walk.over:
		s, err := w.segment(root.id)
		if err != nil {
			return err
		}
		s.complete, s.found = true, bcmrMaxFound+1
		return w.putSegment(root.id, s)
	}

	// Each found transaction becomes a link, after the one whose identity
	// output it spends, if that is its only one, and else first in a
	// segment of its own.  Those found last are behind those found first,
	// so going from the last, each one's parents come before it.
	var made []*bcmrWalkNewSegment
	for k := len(walk.found) - 1; k >= 0; k-- {
		tx := walk.found[k]
		tx.count = 1
		for _, p := range tx.parents {
			tx.count = addBcmrFound(tx.count, p.count)
		}
		l := tx.link
		l.next = tx.child
		if len(tx.parents) == 1 {
			ns := tx.parents[0].segment
			l.segment, l.place = ns.id, ns.s.length
			ns.s.length++
			ns.s.head, ns.head = tx.hash, tx
			ns.s.found = tx.count
			tx.segment = ns
		} else {
			id, err := w.newSegment()
			if err != nil {
				return err
			}
			l.segment, l.place = id, 0
			tx.segment = &bcmrWalkNewSegment{id: id, head: tx, s: &bcmrSegment{
				length: 1, first: tx.hash, head: tx.hash, complete: true,
				found: tx.count,
			}}
			made = append(made, tx.segment)
		}
		tx.segment.s.notePublication(tx.hash, l.place, l.pub)
		if err := w.put(bcmrLinks, tx.hash[:], serializeBcmrLink(l)); err != nil {
			return err
		}
	}

	// Each new segment continues into the segment of the transaction that
	// spends the identity output of its last link.
	for _, ns := range made {
		next, err := w.link(ns.head.child)
		if err != nil {
			return err
		}
		ns.s.next = next.segment
		if err := w.putSegment(ns.id, ns.s); err != nil {
			return err
		}
	}

	// Every segment the walk went behind is complete, with the count of
	// what is behind its first link.  Each was reached from one it went
	// behind before, so going from the last, each one's parents come first.
	for k := len(walk.expanded) - 1; k >= 0; k-- {
		ws := walk.expanded[k]
		for _, p := range ws.parents {
			switch {
			case p.tx != nil:
				ws.count = addBcmrFound(ws.count, p.tx.count)
			case p.segment.s.complete:
				ws.count = addBcmrFound(ws.count, p.segment.s.found)
			default:
				ws.count = addBcmrFound(ws.count, p.segment.count)
			}
		}
		s, err := w.segment(ws.id)
		if err != nil {
			return err
		}
		s.complete, s.found = true, ws.count
		if err := w.putSegment(ws.id, s); err != nil {
			return err
		}
	}
	return nil
}
