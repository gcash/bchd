// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

// walk walks behind the segments left for the walker until none are left,
// and fails the test on an error.
func (c *bcmrTestChain) walk() {
	c.t.Helper()
	if err := c.idx.walkPending(); err != nil {
		c.t.Fatalf("walking: %v", err)
	}
}

// pending returns the IDs of the segments left for the walker, in the order
// it walks behind them.
func (c *bcmrTestChain) pending() []uint64 {
	c.t.Helper()
	var pending []uint64
	err := c.db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(bcmrIndexKey).Bucket(bcmrBucketNames[bcmrPending])
		return bucket.ForEach(func(k, _ []byte) error {
			pending = append(pending, binary.BigEndian.Uint64(k[4:]))
			return nil
		})
	})
	if err != nil {
		c.t.Fatal(err)
	}
	return pending
}

// segmentOf returns the ID of the segment of the link of the passed
// transaction, and the segment.
func (c *bcmrTestChain) segmentOf(tx *wire.MsgTx) (uint64, *bcmrSegment) {
	c.t.Helper()
	hash := tx.TxHash()
	link := c.link(&hash)
	if link == nil {
		c.t.Fatalf("no link for %v", hash)
	}
	var s *bcmrSegment
	err := c.db.View(func(dbTx database.Tx) error {
		w, err := newBcmrWriter(dbTx)
		if err != nil {
			return err
		}
		s, err = w.segment(link.segment)
		return err
	})
	if err != nil {
		c.t.Fatal(err)
	}
	return link.segment, s
}

// checkPending checks that the segments left for the walker are those of
// the passed transactions.
func (c *bcmrTestChain) checkPending(txs ...*wire.MsgTx) {
	c.t.Helper()
	var want []uint64
	for _, tx := range txs {
		id, _ := c.segmentOf(tx)
		want = append(want, id)
	}
	got := c.pending()
	slices.Sort(got)
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		c.t.Errorf("segments left for the walker %v, want %v", got, want)
	}
}

// checkComplete checks whether the segments of the passed transactions are
// complete.
func (c *bcmrTestChain) checkComplete(complete bool, txs ...*wire.MsgTx) {
	c.t.Helper()
	for _, tx := range txs {
		if _, s := c.segmentOf(tx); s.complete != complete {
			c.t.Errorf("the segment of %v is complete %v, want %v", tx.TxHash(),
				s.complete, complete)
		}
	}
}

// checkFound checks how many transactions the walker added in and behind the
// segment of the passed transaction.
func (c *bcmrTestChain) checkFound(tx *wire.MsgTx, found uint32) {
	c.t.Helper()
	if _, s := c.segmentOf(tx); !s.complete || s.found != found {
		c.t.Errorf("the segment of %v is complete %v with %d found, want %d",
			tx.TxHash(), s.complete, s.found, found)
	}
}

// checkIdentity checks the answer for the identity whose authbase is the
// first of the passed transactions, which are its authchain: its authhead,
// the link of the publication to read, and no genesis supply.
func (c *bcmrTestChain) checkIdentity(t *testing.T, pub *wire.MsgTx, chain ...*wire.MsgTx) {
	t.Helper()
	info := c.mustQuery(t, newBcmrTestMempool(), chain[0].TxHash(), true, nil, 0)
	c.checkAuthchain(t, info, 0, false, chain...)
	if head := chain[len(chain)-1].TxHash(); info.Head.Hash != head {
		t.Errorf("%v: authhead %v, want %v", chain[0].TxHash(), info.Head.Hash, head)
	}
	if info.Publication == nil || info.Publication.Hash != pub.TxHash() {
		t.Errorf("%v: publication %+v, want %v's", chain[0].TxHash(), info.Publication, pub.TxHash())
	}
	if info.HasGenesisSupply {
		t.Errorf("%v: a genesis supply", chain[0].TxHash())
	}
}

// checkUntracked checks that the passed transactions answer as no authbase.
func (c *bcmrTestChain) checkUntracked(t *testing.T, txs ...*wire.MsgTx) {
	t.Helper()
	for _, tx := range txs {
		_, err := c.query(t, newBcmrTestMempool(), tx.TxHash(), false, nil, 0)
		if !errors.Is(err, ErrBcmrIdentityNotFound) {
			t.Errorf("%v, which the index doesn't track: %v", tx.TxHash(), err)
		}
	}
}

// TestBcmrIndexPublications checks that a transaction with a publication that
// is no link starts an authchain, before CashTokens activated too and in a
// coinbase, which answers as an authbase without a supply; that a link that
// spends the identity output of a transaction the index doesn't track starts
// a segment, which isn't complete; and that the index leaves a segment with a
// publication for the walker unless it is complete.
func TestBcmrIndexPublications(t *testing.T) {
	c := newBcmrTestChain(t, 10)

	// A publication that spends no identity output is complete at once.
	r := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1, "r.example"))
	c.connect(true, r)
	rHash := r.TxHash()
	state := c.state(&rHash)
	if state == nil || state.identity != nil || state.place != 0 || len(state.path) != 1 ||
		state.head != rHash || state.pubLink != rHash {

		t.Fatalf("the publication's identity is %+v", state)
	}
	c.checkLocation(r)
	c.checkFound(r, 0)
	c.checkPending()
	c.checkIdentity(t, r, r)

	// One that spends the identity output of a transaction the index
	// doesn't track is left for the walker, a malformed one too.
	u := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	before := c.snapshot()
	c.connect(true, u)
	if !reflect.DeepEqual(c.snapshot(), before) {
		t.Error("a block without a publication changed the index")
	}
	p := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(u)}, bcmrOut(), bcmrMalformedPub())
	c.connect(true, p)
	c.checkComplete(false, p)
	c.checkPending(p)
	pHash, uHash := p.TxHash(), u.TxHash()
	if state := c.state(&pHash); state == nil || state.head != pHash || state.pubLink != (chainhash.Hash{}) {
		t.Errorf("the malformed publication's identity is %+v", state)
	}
	if state := c.state(&uHash); state != nil {
		t.Error("the transaction behind it answers before the walk")
	}

	// A link that spends only a complete segment's authhead extends it.
	// One that also spends the identity output of a transaction the index
	// doesn't track starts a segment, which isn't complete.
	q := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r)}, bcmrOut())
	c.connect(true, q)
	if rSegment, _ := c.segmentOf(r); rSegment != c.link(ptr(q.TxHash())).segment {
		t.Error("q doesn't extend r's segment")
	}
	v := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, v)
	m := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(q), bcmrOutPoint(v)}, bcmrOut(), bcmrPub(2))
	c.connect(true, m)
	if link := c.link(ptr(m.TxHash())); link.place != 0 {
		t.Errorf("m is at place %d of its segment, want 0", link.place)
	}
	c.checkComplete(true, r)
	c.checkComplete(false, m)
	c.checkPending(p, m)
	if state := c.state(&rHash); state.head != m.TxHash() || state.pubLink != m.TxHash() ||
		len(state.path) != 2 {

		t.Errorf("r's authhead is %v, with the publication of %v, through %d segments",
			state.head, state.pubLink, len(state.path))
	}

	// A coinbase can publish too.
	c.coinbaseOuts = []*wire.TxOut{bcmrPub(3, "pool.example")}
	coinbase := c.connect(true).Transactions()[0].MsgTx()
	c.checkFound(coinbase, 0)
	c.checkIdentity(t, coinbase, coinbase)
}

// ptr returns a pointer to a copy of the passed hash.
func ptr(hash chainhash.Hash) *chainhash.Hash {
	return &hash
}

// TestBcmrWalk checks that the walker adds every transaction behind a
// publication as a link, through a merge, each of which answers as an
// authbase, counting them; that a transaction it finds joins the first link
// of a segment, so that a token whose authbase is partway along a segment
// stays where it is; and that a later publication needs no walk behind what
// is complete.
func TestBcmrWalk(t *testing.T) {
	c := newBcmrTestChain(t, 0)

	// Behind p, m merges a, which spends r's identity output, and x.
	r := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, r)
	a := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r), c.funding()}, bcmrOut())
	x := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, a, x)
	m := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a), bcmrOutPoint(x)}, bcmrOut())
	c.connect(true, m)
	p := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(m)}, bcmrOut(), bcmrPub(1, "p.example"))
	c.connect(true, p)
	c.walk()

	c.checkPending()
	for _, tx := range []*wire.MsgTx{r, a, x, m} {
		c.checkLocation(tx)
	}
	c.checkFound(r, 2)
	c.checkFound(x, 1)
	c.checkFound(m, 4)
	c.checkFound(p, 4)
	c.checkIdentity(t, p, r, a, m, p)
	c.checkIdentity(t, p, a, m, p)
	c.checkIdentity(t, p, x, m, p)
	rHash := r.TxHash()
	if state := c.state(&rHash); len(state.path) != 3 {
		t.Errorf("r's authchain runs through %d segments, want 3", len(state.path))
	}

	// z extends p's authchain, merging y, which the index doesn't track,
	// so it starts a segment, and g creates a token whose authbase is z,
	// and publishes.  The walk goes behind g only as far as y.
	q := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(p)}, bcmrOut())
	y := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, q, y)
	z := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(q), bcmrOutPoint(y)}, bcmrOut())
	c.connect(true, z)
	zCat := z.TxHash()
	g := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(z)}, bcmrTokens(zCat, 100), bcmrPub(2, "g.example"))
	c.connect(true, g)
	c.checkFound(q, 4)
	c.checkComplete(false, z, g)
	c.checkPending(g)
	if state := c.state(&zCat); state.place != 0 || len(state.path) != 1 {
		t.Fatalf("z's token starts at place %d of %d segments, want 0 of 1",
			state.place, len(state.path))
	}
	before := c.state(&zCat)
	c.walk()

	c.checkPending()
	c.checkLocation(y)
	c.checkFound(y, 1)
	c.checkFound(z, 5)
	c.checkIdentity(t, g, r, a, m, p, q, z, g)
	c.checkIdentity(t, g, p, q, z, g)
	c.checkIdentity(t, g, y, z, g)
	checkBcmrState(t, "z's token", c.state(&zCat), g, false, g, 100)
	if after := c.state(&zCat); !reflect.DeepEqual(after, before) {
		t.Errorf("z's token moved from %+v to %+v", before, after)
	}
	if state := c.state(&rHash); len(state.path) != 4 {
		t.Errorf("r's authchain runs through %d segments, want 4", len(state.path))
	}

	// A later publication is complete, as is everything behind it.
	h := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(g)}, bcmrTokens(zCat, 100), bcmrPub(3))
	c.connect(true, h)
	c.checkFound(h, 5)
	c.checkPending()
}

// TestBcmrWalkAuthbase checks that the walker goes behind a token's authbase
// that spends an identity output the index doesn't track, when the genesis
// publishes.
func TestBcmrWalkAuthbase(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	w := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, w)
	authbase := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(w)}, bcmrOut())
	c.connect(true, authbase)
	category := authbase.TxHash()
	genesis := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(authbase)}, bcmrTokens(category, 5),
		bcmrPub(1))
	c.connect(true, genesis)
	c.checkComplete(false, authbase)
	c.checkPending(genesis)

	c.walk()
	c.checkFound(authbase, 1)
	c.checkIdentity(t, genesis, w, authbase, genesis)
	checkBcmrState(t, "the token", c.state(&category), genesis, false, genesis, 5)
}

// TestBcmrWalkDisconnect checks that disconnecting the index's tip undoes
// what the walker wrote while it was the tip, before the block's own changes,
// and leaves the segment for the walker again, and that walking again builds
// the same index.
func TestBcmrWalkDisconnect(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	r := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, r)
	a := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r)}, bcmrOut())
	b := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1))
	c.connect(true, a, b)
	g := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(b)}, bcmrTokens(b.TxHash(), 1))
	c.connect(true, g)
	m := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(g), bcmrOutPoint(a)}, bcmrOut(), bcmrPub(2))
	merged := c.snapshot()
	c.connect(true, m)
	connected := c.snapshot()
	c.walk()
	walked := c.snapshot()
	c.checkFound(m, 2)

	c.disconnect()
	if got := c.snapshot(); !reflect.DeepEqual(got, merged) {
		t.Fatalf("disconnecting the walked block left %d entries, want %d", len(got), len(merged))
	}
	c.connect(true, m)
	if got := c.snapshot(); !reflect.DeepEqual(got, connected) {
		t.Fatal("connecting the block again built a different index")
	}
	c.walk()
	if got := c.snapshot(); !reflect.DeepEqual(got, walked) {
		t.Fatal("walking again built a different index")
	}

	// The walker writes after a block that changed nothing: disconnecting
	// that block undoes the walk only, and leaves m's segment for the
	// walker again, which it is woken for.
	c.disconnect()
	c.connect(true, m)
	c.connect(true)
	c.walk()
	select {
	case <-c.idx.walkSignal:
	default:
	}
	c.disconnect()
	if got := c.snapshot(); !reflect.DeepEqual(got, connected) {
		t.Fatal("disconnecting the block the walker wrote after left a different index")
	}
	c.checkPending(m)
	select {
	case <-c.idx.walkSignal:
	default:
		t.Error("the walker wasn't woken to walk again")
	}
	c.walk()
	if got := c.snapshot(); !reflect.DeepEqual(got, walked) {
		t.Fatal("walking after the disconnect built a different index")
	}

	// Two publications of one block are walked behind in two writes after
	// it, and disconnecting the block undoes both.
	s1 := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	s2 := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, s1, s2)
	before := c.snapshot()
	p1 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(s1)}, bcmrOut(), bcmrPub(3))
	p2 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(s2)}, bcmrOut(), bcmrPub(4))
	block := c.connect(true, p1, p2)
	c.walk()
	c.checkFound(p1, 1)
	c.checkFound(p2, 1)
	walks := 0
	for key := range c.snapshot() {
		if strings.HasPrefix(key, fmt.Sprintf("undo/%08x", block.Height())) && len(key) > 13 {
			walks++
		}
	}
	if walks != 2 {
		t.Errorf("the walker wrote %d undo records after the block, want 2", walks)
	}
	c.disconnect()
	if got := c.snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatal("disconnecting the block undid the walks behind its publications differently")
	}
}

// TestBcmrWalkPruneUndo checks that the undo records of the walker's writes
// are pruned with those of the block they follow, whether or not the block
// has one, and raise the floor below which no block can be disconnected.
func TestBcmrWalkPruneUndo(t *testing.T) {
	saved := bcmrUndoDepth
	bcmrUndoDepth = 3
	defer func() { bcmrUndoDepth = saved }()

	c := newBcmrTestChain(t, 0)
	u := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, u)
	c.connect(true, bcmrTestTx([]wire.OutPoint{bcmrOutPoint(u)}, bcmrOut(), bcmrPub(1)))
	c.connect(true)
	c.walk()
	v := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, v)
	c.connect(true, bcmrTestTx([]wire.OutPoint{bcmrOutPoint(v)}, bcmrOut(), bcmrPub(2)))
	c.walk()

	// The walker wrote after blocks 3, which changed nothing, and 5; block
	// 5 pruned the record of block 2.
	undoKeys := func() []string {
		var keys []string
		for key := range c.snapshot() {
			if strings.HasPrefix(key, "undo/") {
				keys = append(keys, key)
			}
		}
		slices.Sort(keys)
		return keys
	}
	want := []string{"undo/0000000300000001", "undo/00000005", "undo/0000000500000001"}
	if got := undoKeys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("undo records %v, want %v", got, want)
	}
	c.connect(true)
	if got := undoKeys(); !reflect.DeepEqual(got, want[1:]) {
		t.Fatalf("undo records after block 6 %v, want %v", got, want[1:])
	}

	for i := 0; i < 3; i++ {
		c.disconnect()
	}
	block := c.blocks[len(c.blocks)-1]
	err := c.db.Update(func(dbTx database.Tx) error {
		return dbIndexDisconnectBlock(dbTx, c.idx, block, c.stxos[*block.Hash()])
	})
	if err == nil || !strings.Contains(err.Error(), "--dropbcmrindex") {
		t.Errorf("disconnecting block 3, whose walk record was pruned: %v", err)
	}
}

// bcmrAnswer returns what the index answers for an identity, with its whole
// authchain, as a string to compare.
func (c *bcmrTestChain) bcmrAnswer(t *testing.T, authbase chainhash.Hash) string {
	t.Helper()
	info, err := c.query(t, newBcmrTestMempool(), authbase, true, nil, 0)
	if errors.Is(err, ErrBcmrIdentityNotFound) {
		return "not found"
	}
	if err != nil {
		t.Fatal(err)
	}
	answer := fmt.Sprintf("head %v burned %v supply %v %d from %d:", info.Head.Hash,
		info.Burned, info.HasGenesisSupply, info.GenesisSupply, info.FirstIndex)
	if info.Publication != nil {
		answer += fmt.Sprintf(" publication %v current %v", info.Publication.Hash, info.Current)
	}
	for _, link := range info.Authchain {
		answer += fmt.Sprintf(" %v@%d", link.Hash, link.Height)
	}
	return answer
}

// TestBcmrWalkCap checks that the walker adds nothing behind a publication
// with more than bcmrMaxFound transactions behind it that the index doesn't
// otherwise track, and only reads them once; that what it adds is the same in
// whichever order it goes behind publications; and that a segment that joins
// one with too many behind it takes that over.
func TestBcmrWalkCap(t *testing.T) {
	saved := bcmrMaxFound
	bcmrMaxFound = 3
	defer func() { bcmrMaxFound = saved }()

	// p1 has two transactions behind it, t1 and t2.  p2 extends p1's
	// authchain and merges u1 and u2, so that four are behind it.  p3 has
	// five behind it, w1 to w5, and p4 extends its authchain.
	build := func(p1First bool) (*bcmrTestChain, map[string]*wire.MsgTx) {
		c := newBcmrTestChain(t, 0)
		txs := make(map[string]*wire.MsgTx)
		chain := func(prefix string, n int) *wire.MsgTx {
			tx := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
			txs[prefix+"1"] = tx
			c.connect(true, tx)
			for i := 2; i <= n; i++ {
				tx = bcmrTestTx([]wire.OutPoint{bcmrOutPoint(tx)}, bcmrOut())
				txs[fmt.Sprintf("%s%d", prefix, i)] = tx
				c.connect(true, tx)
			}
			return tx
		}
		t2, u2, w5 := chain("t", 2), chain("u", 2), chain("w", 5)
		txs["p1"] = bcmrTestTx([]wire.OutPoint{bcmrOutPoint(t2)}, bcmrOut(), bcmrPub(1))
		txs["p3"] = bcmrTestTx([]wire.OutPoint{bcmrOutPoint(w5)}, bcmrOut(), bcmrPub(3))
		c.connect(true, txs["p1"], txs["p3"])
		txs["p2"] = bcmrTestTx([]wire.OutPoint{bcmrOutPoint(txs["p1"]), bcmrOutPoint(u2)},
			bcmrOut(), bcmrPub(2))
		c.connect(true, txs["p2"])
		p1Segment, _ := c.segmentOf(txs["p1"])
		p2Segment, _ := c.segmentOf(txs["p2"])

		// The walker goes behind p1 first, as it does for an older
		// publication, or else behind p2 first.
		if !p1First {
			err := c.db.Update(func(dbTx database.Tx) error {
				pending := dbTx.Metadata().Bucket(bcmrIndexKey).
					Bucket(bcmrBucketNames[bcmrPending])
				if err := pending.Delete(bcmrPendingKey(c.heights[txs["p2"].TxHash()], p2Segment)); err != nil {
					return err
				}
				return pending.Put(bcmrPendingKey(0, p2Segment), bcmrPendingValue)
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if got := c.pending()[0]; (got == p1Segment) != p1First {
			t.Fatalf("the walker goes behind segment %d first", got)
		}
		c.walk()

		// p4 extends p3's authchain, whose segment has too many behind
		// it, and so needs no walk.
		txs["p4"] = bcmrTestTx([]wire.OutPoint{bcmrOutPoint(txs["p3"])}, bcmrOut(), bcmrPub(4))
		c.connect(true, txs["p4"])
		c.checkPending()
		return c, txs
	}

	c, txs := build(true)
	c.checkFound(txs["p1"], 2)
	c.checkFound(txs["p2"], bcmrMaxFound+1)
	c.checkFound(txs["p3"], bcmrMaxFound+1)
	c.checkFound(txs["p4"], bcmrMaxFound+1)
	c.checkIdentity(t, txs["p2"], txs["t1"], txs["t2"], txs["p1"], txs["p2"])
	c.checkIdentity(t, txs["p4"], txs["p3"], txs["p4"])
	c.checkUntracked(t, txs["u1"], txs["u2"], txs["w1"], txs["w5"])

	// Whichever of p1 and p2 the walker goes behind first, it adds what is
	// behind p1, and none of the four behind p2, which are too many.
	other, _ := build(false)
	for name, tx := range txs {
		hash := tx.TxHash()
		if got, want := other.bcmrAnswer(t, hash), c.bcmrAnswer(t, hash); got != want {
			t.Errorf("going behind p2 first, %s answers\n%s\nwant\n%s", name, got, want)
		}
	}
}

// TestBcmrWalkChainChanges checks that a walk is written over a block that
// extended a segment it read, or that a disconnect and a connect put back,
// and read again if a disconnect removed a segment it read, or the
// publication that left the segment for the walker, before it wrote or while
// it read.
func TestBcmrWalkChainChanges(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	r := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, r)
	p := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r)}, bcmrOut(), bcmrPub(1))
	c.connect(true, p)

	// A block extends p's segment after the walk read it.
	walk, err := c.idx.readBcmrWalk(new(bcmrBlockCache))
	if err != nil || walk == nil {
		t.Fatalf("reading a walk: %v, %v", walk, err)
	}
	q := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(p)}, bcmrOut())
	c.connect(true, q)
	if err := c.idx.writeBcmrWalk(walk); err != nil {
		t.Fatalf("writing over an extended segment: %v", err)
	}
	c.checkPending()
	c.checkFound(q, 1)
	c.checkIdentity(t, p, r, p, q)

	// A disconnect removes the publication the walk read.
	s := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, s)
	ps := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(s)}, bcmrOut(), bcmrPub(2))
	c.connect(true, ps)
	walk, err = c.idx.readBcmrWalk(new(bcmrBlockCache))
	if err != nil || walk == nil {
		t.Fatalf("reading a walk: %v, %v", walk, err)
	}
	c.disconnect()
	before := c.snapshot()
	if err := c.idx.writeBcmrWalk(walk); !errors.Is(err, errBcmrWalkAgain) {
		t.Errorf("writing over a disconnected segment: %v", err)
	}
	if !reflect.DeepEqual(c.snapshot(), before) {
		t.Error("a walk written over a disconnected segment changed the index")
	}

	// A disconnect and a connect put the block back.
	c.connect(true, ps)
	walk, err = c.idx.readBcmrWalk(new(bcmrBlockCache))
	if err != nil || walk == nil {
		t.Fatalf("reading a walk: %v, %v", walk, err)
	}
	c.disconnect()
	c.connect(true, ps)
	if err := c.idx.writeBcmrWalk(walk); err != nil {
		t.Fatalf("writing over a block put back: %v", err)
	}
	c.checkPending()
	c.checkIdentity(t, ps, s, ps)

	// A disconnect removes the publication that left a segment for the
	// walker, but not the segment, which a token's authbase, whose identity
	// output a transaction the index doesn't track spent, started before.
	x := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, x)
	authbase := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(x)}, bcmrOut())
	c.connect(true, authbase)
	genesis := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(authbase)},
		bcmrTokens(authbase.TxHash(), 5))
	c.connect(true, genesis)
	c.checkComplete(false, genesis)
	before = c.snapshot()
	pg := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(genesis)},
		bcmrTokens(authbase.TxHash(), 5), bcmrPub(4))
	c.connect(true, pg)
	walk, err = c.idx.readBcmrWalk(new(bcmrBlockCache))
	if err != nil || walk == nil {
		t.Fatalf("reading a walk: %v, %v", walk, err)
	}
	c.disconnect()
	if err := c.idx.writeBcmrWalk(walk); !errors.Is(err, errBcmrWalkAgain) {
		t.Errorf("writing after the publication's block was disconnected: %v", err)
	}
	if !reflect.DeepEqual(c.snapshot(), before) {
		t.Error("a walk written after its publication's block was disconnected changed the index")
	}
	c.checkUntracked(t, x)

	// A disconnect while the walk reads removes the block it reads the
	// spend journal of.
	u := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, u)
	pu := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(u)}, bcmrOut(), bcmrPub(3))
	c.connect(true, pu)
	c.onJournal = func() {
		c.onJournal = nil
		c.disconnect()
	}
	if _, err := c.idx.readBcmrWalk(new(bcmrBlockCache)); !errors.Is(err, errBcmrWalkAgain) {
		t.Errorf("reading while a disconnect removes the block read: %v", err)
	}
	if got := c.pending(); len(got) != 0 {
		t.Errorf("segments %v left for the walker after the disconnect", got)
	}
}

// TestBcmrWalkerRuns checks that the walker, once started, walks behind what
// connecting a block left for it once the block is committed, and again after
// a disconnect undid its walk, and that it stops.
func TestBcmrWalkerRuns(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	r := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, r)
	p := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r)}, bcmrOut(), bcmrPub(1))
	c.connect(true, p)
	c.connect(true)

	waitFor := func(what string, done func() bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for !done() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	rHash := r.TxHash()
	walked := func() bool {
		return len(c.pending()) == 0 && c.link(&rHash) != nil
	}

	c.idx.StartWalker()
	waitFor("the walk behind p", walked)

	// Disconnecting the block the walker wrote after undoes the walk, and
	// the walker walks again.
	c.disconnect()
	waitFor("the walk behind p again", walked)

	// The walker walks behind a publication of a block connected now.
	s := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, s)
	ps := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(s)}, bcmrOut(), bcmrPub(2))
	c.connect(true, ps)
	sHash := s.TxHash()
	waitFor("the walk behind ps", func() bool {
		return len(c.pending()) == 0 && c.link(&sHash) != nil
	})
	c.idx.StopWalker()
}

// TestBcmrQueryLink checks the answers for links that are no token's
// authbase: paging from a link of their own authchain, refusing one before
// the authbase, and the genesis supply of a genesis in the mempool that spends
// the authhead's identity output, before and after it confirms.
func TestBcmrQueryLink(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	r := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, r)
	a := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r)}, bcmrOut(), bcmrPub(1))
	c.connect(true, a)
	c.walk()
	mempool := newBcmrTestMempool()

	rHash, aHash := r.TxHash(), a.TxHash()
	info := c.mustQuery(t, mempool, rHash, true, nil, 0)
	c.checkAuthchain(t, info, 0, false, r, a)
	c.checkProof(t, info, a, 1)
	info = c.mustQuery(t, mempool, rHash, true, &rHash, 0)
	c.checkAuthchain(t, info, 1, false, a)
	c.checkProof(t, info, a, 1)
	info = c.mustQuery(t, mempool, aHash, true, nil, 0)
	c.checkAuthchain(t, info, 0, false, a)
	c.checkProof(t, info, a, 0)

	// r, before a, is no link of a's authchain.
	if _, err := c.query(t, mempool, aHash, true, &rHash, 0); !errors.Is(err, ErrBcmrNotALink) {
		t.Errorf("a link before the authbase: %v", err)
	}

	// A genesis in the mempool makes the identity a token's.
	genesis := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(aHash, 77))
	info = c.mustQuery(t, newBcmrTestMempool(genesis), aHash, true, nil, 0)
	if !info.HasGenesisSupply || info.GenesisSupply != 77 || info.Head.Hash != genesis.TxHash() {
		t.Errorf("with the genesis in the mempool: supply %d (%v), authhead %v",
			info.GenesisSupply, info.HasGenesisSupply, info.Head.Hash)
	}
	c.checkAuthchain(t, info, 0, false, a, genesis)
	c.checkProof(t, info, a, 0)
	info = c.mustQuery(t, newBcmrTestMempool(genesis), rHash, false, nil, 0)
	if info.HasGenesisSupply {
		t.Error("the genesis gives r, whose identity output it doesn't spend, a supply")
	}
	c.connect(true, genesis)
	checkBcmrState(t, "the token", c.state(&aHash), genesis, false, a, 77)
}

// TestBcmrDisconnectBelowFloor ensures that a block below the floor can't be
// disconnected although the walker wrote at it since, as its own undo record
// is gone.
func TestBcmrDisconnectBelowFloor(t *testing.T) {
	saved := bcmrUndoDepth
	bcmrUndoDepth = 3
	defer func() { bcmrUndoDepth = saved }()

	c := newBcmrTestChain(t, 0)
	r := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, r)
	p := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r)}, bcmrOut(), bcmrPub(1, "p.example"))
	c.connect(true, p)
	c.connect(true)
	c.connect(true)
	c.connect(true)
	c.disconnect()
	c.disconnect()
	c.disconnect()
	c.walk()

	block := c.blocks[len(c.blocks)-1]
	err := c.db.Update(func(dbTx database.Tx) error {
		return dbIndexDisconnectBlock(dbTx, c.idx, block, c.stxos[*block.Hash()])
	})
	if err == nil || !strings.Contains(err.Error(), "--dropbcmrindex") {
		t.Fatalf("disconnecting below the floor: %v", err)
	}
}

// TestBcmrBlockCache ensures that a block is read once, with the place of each
// of its transactions, and that the cache starts over at its limit.
func TestBcmrBlockCache(t *testing.T) {
	saved := bcmrMaxReadBlocks
	defer func() { bcmrMaxReadBlocks = saved }()

	c := newBcmrTestChain(t, 0)
	txs := []*wire.MsgTx{
		bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut()),
		bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut()),
	}
	first := c.connect(true, txs...)
	second := c.connect(true, bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut()))

	err := c.db.View(func(dbTx database.Tx) error {
		var cache bcmrBlockCache
		read, err := cache.fetch(dbTx, first.Hash(), first.Height())
		if err != nil {
			return err
		}
		again, err := cache.fetch(dbTx, first.Hash(), first.Height())
		if err != nil {
			return err
		}
		if read != again {
			t.Error("a block was read twice")
		}
		locs, err := blockTxLocs(first)
		if err != nil {
			return err
		}
		for k, tx := range first.Transactions() {
			if pos, ok := read.positions[*tx.Hash()]; !ok || pos != k ||
				read.locs[pos] != locs[k] {

				t.Errorf("transaction %d is at %d", k, pos)
			}
		}

		// A limit below the size of two blocks keeps only the second.
		bcmrMaxReadBlocks = cache.size
		if _, err := cache.fetch(dbTx, second.Hash(), second.Height()); err != nil {
			return err
		}
		if len(cache.blocks) != 1 {
			t.Errorf("%d blocks kept, want 1", len(cache.blocks))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestBcmrWalkSharesBlocks checks that the walks of one pass share the blocks
// they read: a walk behind a second publication keeps what the walk behind
// the first read, as two segments behind one block need it read only once.
func TestBcmrWalkSharesBlocks(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	r1 := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	r2 := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	shared := c.connect(true, r1, r2)
	p1 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r1)}, bcmrOut(), bcmrPub(1))
	c.connect(true, p1)
	p2 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(r2)}, bcmrOut(), bcmrPub(2))
	c.connect(true, p2)

	var blocks bcmrBlockCache
	walk, err := c.idx.readBcmrWalk(&blocks)
	if err != nil || walk == nil {
		t.Fatalf("reading the first walk: %v, %v", walk, err)
	}
	if err := c.idx.writeBcmrWalk(walk); err != nil {
		t.Fatal(err)
	}
	read := maps.Clone(blocks.blocks)
	if _, ok := read[*shared.Hash()]; !ok {
		t.Fatal("the first walk didn't read the shared block")
	}
	walk, err = c.idx.readBcmrWalk(&blocks)
	if err != nil || walk == nil {
		t.Fatalf("reading the second walk: %v, %v", walk, err)
	}
	for hash, block := range read {
		if blocks.blocks[hash] != block {
			t.Fatalf("the second walk read block %v again", hash)
		}
	}
}
