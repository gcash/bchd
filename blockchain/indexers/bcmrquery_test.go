// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"errors"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/merkleblock"
)

// bcmrTestMempool stands in for the mempool in the BCMR query tests.
type bcmrTestMempool struct {
	txs    map[chainhash.Hash]*bchutil.Tx
	spends map[wire.OutPoint]*bchutil.Tx
}

// newBcmrTestMempool returns a mempool holding the passed transactions.
func newBcmrTestMempool(txs ...*wire.MsgTx) *bcmrTestMempool {
	m := &bcmrTestMempool{
		txs:    make(map[chainhash.Hash]*bchutil.Tx),
		spends: make(map[wire.OutPoint]*bchutil.Tx),
	}
	for _, tx := range txs {
		btx := bchutil.NewTx(tx)
		m.txs[*btx.Hash()] = btx
		for _, in := range tx.TxIn {
			m.spends[in.PreviousOutPoint] = btx
		}
	}
	return m
}

// CheckSpend returns the transaction of the mempool that spends op.
func (m *bcmrTestMempool) CheckSpend(op wire.OutPoint) *bchutil.Tx {
	return m.spends[op]
}

// FetchTransaction returns a transaction of the mempool.
func (m *bcmrTestMempool) FetchTransaction(hash *chainhash.Hash) (*bchutil.Tx, error) {
	tx, ok := m.txs[*hash]
	if !ok {
		return nil, errors.New("not in the mempool")
	}
	return tx, nil
}

// bcmrChangingMempool stands in for a mempool that blocks drop transactions
// from while a query reads it: each spend is there only the first time it is
// asked for.  It counts the spends asked for.
type bcmrChangingMempool struct {
	*bcmrTestMempool
	asked map[wire.OutPoint]int
}

// CheckSpend returns the transaction that spends op the first time op is
// asked for, and nil after.
func (m *bcmrChangingMempool) CheckSpend(op wire.OutPoint) *bchutil.Tx {
	m.asked[op]++
	if m.asked[op] > 1 {
		return nil
	}
	return m.bcmrTestMempool.CheckSpend(op)
}

// query queries the test chain's index, with the passed mempool.
func (c *bcmrTestChain) query(t *testing.T, mempool BcmrMempool, authbase chainhash.Hash,
	authchain bool, after *chainhash.Hash, limit int) (*BcmrIdentityInfo, error) {

	t.Helper()
	return c.idx.Identity(&BcmrQuery{
		Authbase: authbase, Authchain: authchain, After: after, Limit: limit,
	}, mempool)
}

// mustQuery queries the test chain's index and fails the test on an error.
func (c *bcmrTestChain) mustQuery(t *testing.T, mempool BcmrMempool, authbase chainhash.Hash,
	authchain bool, after *chainhash.Hash, limit int) *BcmrIdentityInfo {

	t.Helper()
	info, err := c.query(t, mempool, authbase, authchain, after, limit)
	if err != nil {
		t.Fatalf("query for %v: %v", authbase, err)
	}
	return info
}

// checkBcmrAuthchain checks that an answer's authchain holds the passed
// transactions from index first, each with its height and block, and whether
// later links didn't fit.
func (c *bcmrTestChain) checkAuthchain(t *testing.T, info *BcmrIdentityInfo, first uint32,
	more bool, txs ...*wire.MsgTx) {

	t.Helper()
	if info.FirstIndex != first || info.More != more || len(info.Authchain) != len(txs) {
		t.Fatalf("authchain from %d with %d links (more %v), want from %d with %d (more %v)",
			info.FirstIndex, len(info.Authchain), info.More, first, len(txs), more)
	}
	for i, tx := range txs {
		entry := info.Authchain[i]
		raw, err := serializeBcmrTx(tx)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Hash != tx.TxHash() || !bytes.Equal(entry.Tx, raw) {
			t.Errorf("link %d is %v, want %v", int(first)+i, entry.Hash, tx.TxHash())
		}
		height, confirmed := c.heights[tx.TxHash()]
		switch {
		case confirmed && (entry.Height != height || entry.BlockHash == nil ||
			*entry.BlockHash != *c.blocks[height].Hash()):
			t.Errorf("link %d is at height %d of %v, want %d", int(first)+i,
				entry.Height, entry.BlockHash, height)
		case !confirmed && (entry.Height != 0 || entry.BlockHash != nil):
			t.Errorf("unconfirmed link %d is at height %d of %v", int(first)+i,
				entry.Height, entry.BlockHash)
		}
	}
}

// checkProof checks that an answer proves the passed link of the test chain,
// at index, the way verifytxoutproof checks a proof.
func (c *bcmrTestChain) checkProof(t *testing.T, info *BcmrIdentityInfo, tx *wire.MsgTx, index uint32) {
	t.Helper()
	if tx == nil {
		if info.Proof != nil {
			t.Errorf("the answer proves a link at index %d", info.ProofIndex)
		}
		return
	}
	if info.Proof == nil {
		t.Fatalf("the answer proves no link, want index %d", index)
	}
	hash := tx.TxHash()
	block := c.blocks[c.heights[hash]]
	partial := merkleblock.NewMerkleBlockFromMsg(*info.Proof)
	root := partial.ExtractMatches()
	matches := partial.GetMatches()
	if root == nil || partial.BadTree() || !info.Proof.Header.MerkleRoot.IsEqual(root) ||
		info.Proof.Header.BlockHash() != *block.Hash() ||
		len(matches) != 1 || *matches[0] != hash || info.ProofIndex != index {

		t.Errorf("the proof doesn't prove %v at index %d", hash, index)
	}
}

// bcmrTestAuthchain builds an authchain on a test chain: an authbase that
// publishes, a genesis with 1000 tokens in its identity output that
// publishes, and two links in one block, the second listed first and with a
// malformed publication.
func bcmrTestAuthchain(c *bcmrTestChain) []*wire.MsgTx {
	authbase := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1, "a.example"))
	c.connect(true, authbase)
	category := authbase.TxHash()
	genesis := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(authbase)}, bcmrTokens(category, 1000),
		bcmrPub(2, "g.example"))
	c.connect(true, genesis)
	first := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(genesis)}, bcmrTokens(category, 1000))
	second := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(first)}, bcmrTokens(category, 1000),
		bcmrMalformedPub())
	c.connect(true, second, first)
	return []*wire.MsgTx{authbase, genesis, first, second}
}

// TestBcmrQueryConfirmed checks the answer for an identity whose links are all
// confirmed: the authhead, the publication to read when the authhead's is
// malformed, the supply, the identity output, and the authchain in pages.
func TestBcmrQueryConfirmed(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	links := bcmrTestAuthchain(c)
	authbase, genesis, first, second := links[0], links[1], links[2], links[3]
	category := authbase.TxHash()
	mempool := newBcmrTestMempool()

	info := c.mustQuery(t, mempool, category, false, nil, 0)
	if info.Head.Hash != second.TxHash() || info.Head.Height != c.heights[second.TxHash()] ||
		info.Head.Publication == nil || !info.Head.Publication.Malformed {

		t.Errorf("head %v at %d with publication %+v", info.Head.Hash, info.Head.Height,
			info.Head.Publication)
	}
	if info.Publication == nil || info.Publication.Hash != genesis.TxHash() || info.Current ||
		info.Publication.Publication.URIs[0] != "g.example" {

		t.Errorf("publication %+v, current %v", info.Publication, info.Current)
	}
	if info.Burned || !info.HasGenesisSupply || info.GenesisSupply != 1000 {
		t.Errorf("burned %v, supply %d (%v)", info.Burned, info.GenesisSupply, info.HasGenesisSupply)
	}
	if out := info.IdentityOutput; out == nil || chainhash.Hash(out.CategoryID) != category ||
		out.Amount != 1000 {

		t.Errorf("identity output %+v", out)
	}
	if info.Authchain != nil || info.Proof != nil {
		t.Error("the answer has an authchain it wasn't asked for")
	}

	// The whole authchain, with the proof of the authhead.
	info = c.mustQuery(t, mempool, category, true, nil, 0)
	c.checkAuthchain(t, info, 0, false, links...)
	c.checkProof(t, info, second, 3)

	// Pages.
	info = c.mustQuery(t, mempool, category, true, nil, 2)
	c.checkAuthchain(t, info, 0, true, authbase, genesis)
	c.checkProof(t, info, nil, 0)
	genesisHash := genesis.TxHash()
	info = c.mustQuery(t, mempool, category, true, &genesisHash, 2)
	c.checkAuthchain(t, info, 2, false, first, second)
	c.checkProof(t, info, second, 3)
	secondHash := second.TxHash()
	info = c.mustQuery(t, mempool, category, true, &secondHash, 0)
	c.checkAuthchain(t, info, 4, false)
	c.checkProof(t, info, second, 3)

	// Pages cut by size hold one link each.
	saved := bcmrMaxAuthchainBytes
	bcmrMaxAuthchainBytes = 1
	defer func() { bcmrMaxAuthchainBytes = saved }()
	var after *chainhash.Hash
	for i, tx := range links {
		info = c.mustQuery(t, mempool, category, true, after, 0)
		last := i == len(links)-1
		c.checkAuthchain(t, info, uint32(i), !last, tx)
		if last {
			c.checkProof(t, info, second, 3)
		}
		hash := tx.TxHash()
		after = &hash
	}
	bcmrMaxAuthchainBytes = saved

	// A link of another identity's authchain, or no link, is refused, as
	// is an authbase without an identity.
	other := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, other)
	otherGenesis := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(other)},
		bcmrTokens(other.TxHash(), 1))
	c.connect(true, otherGenesis)
	otherHash := otherGenesis.TxHash()
	if _, err := c.query(t, mempool, category, true, &otherHash, 0); !errors.Is(err, ErrBcmrNotALink) {
		t.Errorf("a link of another authchain: %v", err)
	}
	if _, err := c.query(t, mempool, category, true, &chainhash.Hash{7}, 0); !errors.Is(err, ErrBcmrNotALink) {
		t.Errorf("no link: %v", err)
	}
	if _, err := c.query(t, mempool, chainhash.Hash{7}, false, nil, 0); !errors.Is(err, ErrBcmrIdentityNotFound) {
		t.Errorf("no identity: %v", err)
	}
}

// TestBcmrQueryMergedPath checks that a link of an identity's authchain from
// before it merged is not a link of the other identity's.
func TestBcmrQueryMergedPath(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	a := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	b := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, a, b)
	aCat, bCat := a.TxHash(), b.TxHash()
	ga := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(aCat, 1))
	gb := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(b)}, bcmrTokens(bCat, 1))
	c.connect(true, ga, gb)
	merge := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(ga), bcmrOutPoint(gb)}, bcmrOut())
	c.connect(true, merge)
	mempool := newBcmrTestMempool()

	info := c.mustQuery(t, mempool, aCat, true, nil, 0)
	c.checkAuthchain(t, info, 0, false, a, ga, merge)
	info = c.mustQuery(t, mempool, bCat, true, nil, 0)
	c.checkAuthchain(t, info, 0, false, b, gb, merge)

	gbHash := gb.TxHash()
	if _, err := c.query(t, mempool, aCat, true, &gbHash, 0); !errors.Is(err, ErrBcmrNotALink) {
		t.Errorf("b's genesis as a link of a: %v", err)
	}
	info = c.mustQuery(t, mempool, bCat, true, &gbHash, 0)
	c.checkAuthchain(t, info, 2, false, merge)
	c.checkProof(t, info, merge, 2)
}

// TestBcmrQueryMempool checks links in the mempool after the confirmed ones,
// and a burn in the mempool.
func TestBcmrQueryMempool(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	links := bcmrTestAuthchain(c)
	authbase, genesis, first, second := links[0], links[1], links[2], links[3]
	category := authbase.TxHash()

	m1 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(second)}, bcmrOut(), bcmrPub(3, "m1.example"))
	m2 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(m1)}, bcmrTokens(category, 1000))
	mempool := newBcmrTestMempool(m2, m1)
	info := c.mustQuery(t, mempool, category, true, nil, 0)
	if info.Head.Hash != m2.TxHash() || info.Head.Height != 0 || info.Head.BlockHash != nil {
		t.Errorf("head %v at %d", info.Head.Hash, info.Head.Height)
	}
	if info.Publication == nil || info.Publication.Hash != m1.TxHash() || info.Current {
		t.Errorf("publication %+v, current %v", info.Publication, info.Current)
	}
	c.checkAuthchain(t, info, 0, false, authbase, genesis, first, second, m1, m2)
	c.checkProof(t, info, second, 3)

	m1Hash := m1.TxHash()
	info = c.mustQuery(t, mempool, category, true, &m1Hash, 0)
	c.checkAuthchain(t, info, 5, false, m2)
	c.checkProof(t, info, second, 3)
	secondHash := second.TxHash()
	info = c.mustQuery(t, mempool, category, true, &secondHash, 1)
	c.checkAuthchain(t, info, 4, true, m1)
	c.checkProof(t, info, nil, 0)

	// A burn in the mempool, which publishes.
	burn := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(m2)}, bcmrPub(4, "final.example"))
	mempool = newBcmrTestMempool(m1, m2, burn)
	info = c.mustQuery(t, mempool, category, false, nil, 0)
	if !info.Burned || info.Head.Hash != burn.TxHash() || info.Publication == nil ||
		info.Publication.Hash != burn.TxHash() || !info.Current || info.IdentityOutput != nil {

		t.Errorf("burned %v, head %v, publication %+v, current %v, identity output %+v",
			info.Burned, info.Head.Hash, info.Publication, info.Current, info.IdentityOutput)
	}
}

// TestBcmrQueryMempoolGenesis checks tokens whose genesis is in the mempool:
// with an authbase in a block, one in the mempool, and one that is the
// authhead of another identity.
func TestBcmrQueryMempoolGenesis(t *testing.T) {
	c := newBcmrTestChain(t, 0)

	// An authbase in a block, which publishes.
	x := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1, "x.example"))
	c.connect(true, x)
	xCat := x.TxHash()
	gx := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(x)}, bcmrTokens(xCat, 5))

	// An authbase in the mempool.
	y := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	yCat := y.TxHash()
	gy := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(y)}, bcmrTokens(yCat, 6), bcmrPub(2))

	// The authhead of another identity.
	links := bcmrTestAuthchain(c)
	head := links[3]
	hCat := head.TxHash()
	gh := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(head)}, bcmrTokens(hCat, 7))

	mempool := newBcmrTestMempool(gx, y, gy, gh)

	info := c.mustQuery(t, mempool, xCat, true, nil, 0)
	if info.Head.Hash != gx.TxHash() || info.GenesisSupply != 5 || info.Publication == nil ||
		info.Publication.Hash != xCat || info.Current {

		t.Errorf("x: head %v, supply %d, publication %+v", info.Head.Hash,
			info.GenesisSupply, info.Publication)
	}
	c.checkAuthchain(t, info, 0, false, x, gx)
	c.checkProof(t, info, x, 0)
	info = c.mustQuery(t, mempool, xCat, true, &xCat, 0)
	c.checkAuthchain(t, info, 1, false, gx)
	c.checkProof(t, info, x, 0)
	gxHash := gx.TxHash()
	info = c.mustQuery(t, mempool, xCat, true, &gxHash, 0)
	c.checkAuthchain(t, info, 2, false)

	info = c.mustQuery(t, mempool, yCat, true, nil, 0)
	if info.Head.Hash != gy.TxHash() || info.GenesisSupply != 6 || !info.Current {
		t.Errorf("y: head %v, supply %d, current %v", info.Head.Hash, info.GenesisSupply,
			info.Current)
	}
	c.checkAuthchain(t, info, 0, false, y, gy)
	c.checkProof(t, info, nil, 0)

	info = c.mustQuery(t, mempool, hCat, true, nil, 0)
	if info.Head.Hash != gh.TxHash() || info.GenesisSupply != 7 {
		t.Errorf("h: head %v, supply %d", info.Head.Hash, info.GenesisSupply)
	}
	c.checkAuthchain(t, info, 0, false, head, gh)
	c.checkProof(t, info, head, 0)

	// The identity whose authhead the genesis spends carries on through
	// it.
	info = c.mustQuery(t, mempool, links[0].TxHash(), false, nil, 0)
	if info.Head.Hash != gh.TxHash() {
		t.Errorf("the identity carries on to %v, want %v", info.Head.Hash, gh.TxHash())
	}

	// Pages of an authchain whose authbase is in the mempool too.
	gy2 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(gy)}, bcmrTokens(yCat, 6))
	mempool = newBcmrTestMempool(gx, y, gy, gy2, gh)
	info = c.mustQuery(t, mempool, yCat, true, &yCat, 0)
	c.checkAuthchain(t, info, 1, false, gy, gy2)
	gyHash := gy.TxHash()
	info = c.mustQuery(t, mempool, yCat, true, &gyHash, 0)
	c.checkAuthchain(t, info, 2, false, gy2)
	c.checkProof(t, info, nil, 0)

	// A genesis that leaves the mempool while the query reads it, as when
	// a block confirms it, is still the next link.
	changing := &bcmrChangingMempool{newBcmrTestMempool(gx, y, gy, gh),
		make(map[wire.OutPoint]int)}
	info = c.mustQuery(t, changing, xCat, true, nil, 0)
	if info.Head.Hash != gx.TxHash() {
		t.Errorf("x with a changing mempool: head %v, want %v", info.Head.Hash, gx.TxHash())
	}
	c.checkAuthchain(t, info, 0, false, x, gx)

	// Confirmed, the token whose authbase was another identity's authhead
	// starts partway along that authchain's segment: links before it are
	// not its own.
	c.connect(true, gh)
	mempool = newBcmrTestMempool()
	if state := c.state(&hCat); state == nil || state.place == 0 {
		t.Fatal("h doesn't start partway along a segment")
	}
	info = c.mustQuery(t, mempool, hCat, true, nil, 0)
	c.checkAuthchain(t, info, 0, false, head, gh)
	c.checkProof(t, info, gh, 1)
	info = c.mustQuery(t, mempool, hCat, true, nil, 1)
	c.checkAuthchain(t, info, 0, true, head)
	info = c.mustQuery(t, mempool, hCat, true, &hCat, 0)
	c.checkAuthchain(t, info, 1, false, gh)
	c.checkProof(t, info, gh, 1)
	firstHash := links[2].TxHash()
	if _, err := c.query(t, mempool, hCat, true, &firstHash, 0); !errors.Is(err, ErrBcmrNotALink) {
		t.Errorf("a link before h's authbase: %v", err)
	}

	// No genesis spends this one's identity output.
	lone := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, lone)
	spend := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(lone)}, wire.NewTxOut(1,
		[]byte{txscript.OP_TRUE}, wire.TokenData{}))
	if _, err := c.query(t, newBcmrTestMempool(spend), lone.TxHash(), false, nil, 0); !errors.Is(err, ErrBcmrIdentityNotFound) {
		t.Errorf("a spend that creates no token: %v", err)
	}
}
