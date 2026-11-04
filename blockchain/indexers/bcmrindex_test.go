// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// bcmrTestChain stands in for the chain in the BCMR index tests.  It builds
// blocks, stores them, and connects them to an index, keeping its tip as the
// index manager does, and it answers the index's reads of the main chain's
// blocks and of their spent outputs.
type bcmrTestChain struct {
	t      *testing.T
	db     database.DB
	idx    *BcmrIndex
	blocks []*bchutil.Block

	// heights is the height of the block of each transaction, including
	// the funding transactions, which the tests pretend are in the
	// genesis block.
	heights map[chainhash.Hash]int32

	// stxos are the outputs each block spends, the way the spend journal
	// has them.
	stxos map[chainhash.Hash][]blockchain.SpentTxOut

	// fundings counts the funding outpoints handed out.
	fundings uint32
}

// newBcmrTestChain returns a test chain with an index on which CashTokens
// activated after the block at forkHeight, holding a genesis block.
func newBcmrTestChain(t *testing.T, forkHeight int32) *bcmrTestChain {
	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	params := chaincfg.RegressionNetParams
	params.Upgrade9ForkHeight = forkHeight
	params.Checkpoints = nil
	c := &bcmrTestChain{
		t:       t,
		db:      db,
		idx:     NewBcmrIndex(db, &params),
		heights: make(map[chainhash.Hash]int32),
		stxos:   make(map[chainhash.Hash][]blockchain.SpentTxOut),
	}
	c.idx.chain = c
	err = db.Update(func(dbTx database.Tx) error {
		_, err := dbTx.Metadata().CreateBucketIfNotExists(indexTipsBucketName)
		if err != nil {
			return err
		}
		for _, indexer := range []Indexer{c.idx} {
			if err := indexer.Create(dbTx); err != nil {
				return err
			}
			err := dbPutIndexerTip(dbTx, indexer.Key(), &chainhash.Hash{}, -1)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.idx.Init(); err != nil {
		t.Fatal(err)
	}
	c.connect(true)
	return c
}

// BlockHashByHeight returns the hash of the block at a height of the test
// chain.
func (c *bcmrTestChain) BlockHashByHeight(height int32) (*chainhash.Hash, error) {
	if height < 0 || int(height) >= len(c.blocks) {
		return nil, fmt.Errorf("no block at height %d", height)
	}
	return c.blocks[height].Hash(), nil
}

// FetchSpendJournal returns the outputs a block of the test chain spends.
func (c *bcmrTestChain) FetchSpendJournal(block *bchutil.Block) ([]blockchain.SpentTxOut, error) {
	stxos, ok := c.stxos[*block.Hash()]
	if !ok {
		return nil, fmt.Errorf("no spend journal for block %v", block.Hash())
	}
	return stxos, nil
}

// funding returns an outpoint for a transaction to spend, of a transaction
// the test chain pretends is in its genesis block.
func (c *bcmrTestChain) funding() wire.OutPoint {
	c.fundings++
	var hash chainhash.Hash
	binary.LittleEndian.PutUint32(hash[:], c.fundings)
	hash[31] = 0xfe
	c.heights[hash] = 0
	return wire.OutPoint{Hash: hash, Index: 1}
}

// connect builds a block of the passed transactions on the tip of the test
// chain, after a coinbase, stores it and connects it to the index, handing
// the index the block's spent outputs if withStxos is set, as the chain does
// for a block it connects, and none otherwise, as the index manager does while
// the index catches up.
func (c *bcmrTestChain) connect(withStxos bool, txs ...*wire.MsgTx) *bchutil.Block {
	c.t.Helper()
	height := int32(len(c.blocks))
	header := wire.BlockHeader{Version: 1, Nonce: uint32(height)}
	if height > 0 {
		header.PrevBlock = *c.blocks[height-1].Hash()
	}
	msgBlock := wire.NewMsgBlock(&header)

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{txscript.OP_DATA_4, byte(height), byte(height >> 8), 0, 0},
	})
	coinbase.AddTxOut(wire.NewTxOut(5000000000, []byte{txscript.OP_TRUE}, wire.TokenData{}))
	if err := msgBlock.AddTransaction(coinbase); err != nil {
		c.t.Fatal(err)
	}
	var stxos []blockchain.SpentTxOut
	for _, tx := range txs {
		if err := msgBlock.AddTransaction(tx); err != nil {
			c.t.Fatal(err)
		}
		for _, in := range tx.TxIn {
			prevHeight, ok := c.heights[in.PreviousOutPoint.Hash]
			if !ok {
				prevHeight = height
			}
			stxos = append(stxos, blockchain.SpentTxOut{Height: prevHeight})
		}
	}
	merkles := blockchain.BuildMerkleTreeStore(bchutil.NewBlock(msgBlock).Transactions())
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	block := bchutil.NewBlock(msgBlock)
	block.SetHeight(height)
	c.stxos[*block.Hash()] = stxos
	for _, tx := range block.Transactions() {
		c.heights[*tx.Hash()] = height
	}
	c.blocks = append(c.blocks, block)

	passed := stxos
	if !withStxos {
		passed = nil
	}
	err := c.db.Update(func(dbTx database.Tx) error {
		// A block connected again after it was disconnected is still
		// stored, as the chain keeps it.
		stored, err := dbTx.HasBlock(block.Hash())
		if err != nil {
			return err
		}
		if !stored {
			if err := dbTx.StoreBlock(block); err != nil {
				return err
			}
		}
		return dbIndexConnectBlock(dbTx, c.idx, block, passed)
	})
	if err != nil {
		c.t.Fatalf("connecting block %d: %v", height, err)
	}
	return block
}

// disconnect disconnects the tip of the test chain from the index.
func (c *bcmrTestChain) disconnect() {
	c.t.Helper()
	block := c.blocks[len(c.blocks)-1]
	err := c.db.Update(func(dbTx database.Tx) error {
		return dbIndexDisconnectBlock(dbTx, c.idx, block, c.stxos[*block.Hash()])
	})
	if err != nil {
		c.t.Fatalf("disconnecting block %d: %v", block.Height(), err)
	}
	c.blocks = c.blocks[:len(c.blocks)-1]
}

// snapshot returns every entry of the index, and the heads it keeps in
// memory, so that two states of the index can be compared.
func (c *bcmrTestChain) snapshot() map[string]string {
	c.t.Helper()
	entries := make(map[string]string)
	err := c.db.View(func(dbTx database.Tx) error {
		top := dbTx.Metadata().Bucket(bcmrIndexKey)
		if v := top.Get(bcmrNextSegmentKey); v != nil {
			entries["nextsegment"] = hex.EncodeToString(v)
		}
		for _, name := range bcmrBucketNames {
			err := top.Bucket(name).ForEach(func(k, v []byte) error {
				entries[string(name)+"/"+hex.EncodeToString(k)] = hex.EncodeToString(v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.t.Fatal(err)
	}
	for head, segment := range c.idx.heads {
		entries["memory/"+head.String()] = fmt.Sprint(segment)
	}
	return entries
}

// state returns what the index knows of the identity with the passed
// authbase.
func (c *bcmrTestChain) state(authbase *chainhash.Hash) *bcmrIdentityState {
	c.t.Helper()
	var state *bcmrIdentityState
	err := c.db.View(func(dbTx database.Tx) error {
		var err error
		state, err = fetchBcmrIdentityState(dbTx, authbase)
		return err
	})
	if err != nil {
		c.t.Fatal(err)
	}
	return state
}

// link returns the link with the passed txid, or nil if the index has none.
func (c *bcmrTestChain) link(hash *chainhash.Hash) *bcmrLink {
	c.t.Helper()
	var link *bcmrLink
	err := c.db.View(func(dbTx database.Tx) error {
		raw := dbTx.Metadata().Bucket(bcmrIndexKey).Bucket(bcmrBucketNames[bcmrLinks]).Get(hash[:])
		if raw == nil {
			return nil
		}
		var err error
		link, err = deserializeBcmrLink(raw)
		return err
	})
	if err != nil {
		c.t.Fatal(err)
	}
	return link
}

// checkLocation checks that a link locates its transaction in the block
// files.
func (c *bcmrTestChain) checkLocation(tx *wire.MsgTx) {
	c.t.Helper()
	hash := tx.TxHash()
	link := c.link(&hash)
	if link == nil {
		c.t.Fatalf("no link for %v", hash)
	}
	var raw []byte
	err := c.db.View(func(dbTx database.Tx) error {
		var err error
		raw, err = dbTx.FetchBlockRegion(&database.BlockRegion{
			Hash: &link.blockHash, Offset: link.offset, Len: link.length,
		})
		return err
	})
	if err != nil {
		c.t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		c.t.Fatal(err)
	}
	if !bytes.Equal(raw, buf.Bytes()) {
		c.t.Errorf("link %v locates other bytes than the transaction's", hash)
	}
	if link.height != c.heights[hash] {
		c.t.Errorf("link %v has height %d, want %d", hash, link.height, c.heights[hash])
	}
}

// bcmrTestTx returns a transaction that spends the passed outpoints and has
// the passed outputs.
func bcmrTestTx(ins []wire.OutPoint, outs ...*wire.TxOut) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	for _, in := range ins {
		tx.AddTxIn(wire.NewTxIn(&in, nil))
	}
	for _, out := range outs {
		tx.AddTxOut(out)
	}
	return tx
}

// bcmrOut returns a plain output.
func bcmrOut() *wire.TxOut {
	return wire.NewTxOut(1000, []byte{txscript.OP_TRUE}, wire.TokenData{})
}

// bcmrTokens returns an output with fungible tokens of a category.
func bcmrTokens(category chainhash.Hash, amount uint64) *wire.TxOut {
	return wire.NewTxOut(1000, []byte{txscript.OP_TRUE}, wire.TokenData{
		CategoryID: category, Amount: amount, BitField: wire.HAS_AMOUNT,
	})
}

// bcmrNFT returns an output with an NFT of a category without a commitment.
func bcmrNFT(category chainhash.Hash, capability byte) *wire.TxOut {
	return wire.NewTxOut(1000, []byte{txscript.OP_TRUE}, wire.TokenData{
		CategoryID: category, BitField: wire.HAS_NFT | capability,
	})
}

// bcmrTestPrefix is the locking bytecode prefix of a publication output.
var bcmrTestPrefix = []byte{txscript.OP_RETURN, txscript.OP_DATA_4, 'B', 'C', 'M', 'R'}

// bcmrPub returns a publication output of the hash, with the passed URIs.
func bcmrPub(hashByte byte, uris ...string) *wire.TxOut {
	script := append([]byte{}, bcmrTestPrefix...)
	script = append(script, txscript.OP_DATA_32)
	script = append(script, bytes.Repeat([]byte{hashByte}, 32)...)
	for _, uri := range uris {
		script = append(script, byte(len(uri)))
		script = append(script, uri...)
	}
	return wire.NewTxOut(0, script, wire.TokenData{})
}

// bcmrMalformedPub returns a publication output without a hash.
func bcmrMalformedPub() *wire.TxOut {
	return wire.NewTxOut(0, append([]byte{}, bcmrTestPrefix...), wire.TokenData{})
}

// bcmrOutPoint returns output 0 of a transaction.
func bcmrOutPoint(tx *wire.MsgTx) wire.OutPoint {
	return wire.OutPoint{Hash: tx.TxHash(), Index: 0}
}

// checkBcmrState checks an identity's authhead, whether it was burned, the
// link of its newest well-formed publication, and its supply.
func checkBcmrState(t *testing.T, name string, state *bcmrIdentityState,
	head *wire.MsgTx, burned bool, pub *wire.MsgTx, supply uint64) {

	t.Helper()
	if state == nil {
		t.Fatalf("%s: no identity", name)
	}
	if state.head != head.TxHash() {
		t.Errorf("%s: head %v, want %v", name, state.head, head.TxHash())
	}
	if state.burned != burned {
		t.Errorf("%s: burned %v, want %v", name, state.burned, burned)
	}
	var wantPub chainhash.Hash
	if pub != nil {
		wantPub = pub.TxHash()
	}
	if state.pubLink != wantPub {
		t.Errorf("%s: publication in %v, want %v", name, state.pubLink, wantPub)
	}
	if state.identity.supply != supply {
		t.Errorf("%s: supply %d, want %d", name, state.identity.supply, supply)
	}
}

// TestBcmrIndexAuthchain checks the authchain of a token from its authbase,
// with links that come before their parents in a block, a malformed
// publication, and a block that the index has nothing to do with.
func TestBcmrIndexAuthchain(t *testing.T) {
	c := newBcmrTestChain(t, 0)

	authbase := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, authbase)
	if len(c.snapshot()) != 0 {
		t.Fatal("a block without a token changed the index")
	}

	category := authbase.TxHash()
	genesis := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(authbase), c.funding()},
		bcmrTokens(category, 1000), bcmrPub(1, "example.com"))
	c.connect(true, genesis)
	checkBcmrState(t, "after the genesis", c.state(&category), genesis, false, genesis, 1000)
	c.checkLocation(authbase)
	c.checkLocation(genesis)
	if link := c.link(&category); link.next != genesis.TxHash() || link.place != 0 {
		t.Errorf("authbase leads to %v from place %d", link.next, link.place)
	}

	// The second link spends the first, but the block lists it first.
	first := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(genesis)}, bcmrTokens(category, 1000))
	second := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(first)}, bcmrTokens(category, 1000),
		bcmrMalformedPub())
	c.connect(true, second, first)
	checkBcmrState(t, "after two links", c.state(&category), second, false, genesis, 1000)
	c.checkLocation(first)
	c.checkLocation(second)
	genesisHash, firstHash, secondHash := genesis.TxHash(), first.TxHash(), second.TxHash()
	if link := c.link(&genesisHash); link.next != firstHash {
		t.Errorf("the genesis leads to %v, want %v", link.next, firstHash)
	}
	if link := c.link(&firstHash); link.next != secondHash || link.place != 2 {
		t.Errorf("the first link leads to %v from place %d", link.next, link.place)
	}
	link := c.link(&secondHash)
	if link.place != 3 || link.next != (chainhash.Hash{}) || link.pub == nil || !link.pub.Malformed {
		t.Errorf("the second link is at place %d, leads to %v, with publication %+v",
			link.place, link.next, link.pub)
	}

	// A block with no token and no spend of an identity output changes
	// nothing, and has no undo record.
	before := c.snapshot()
	c.connect(true, bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut()))
	if after := c.snapshot(); !reflect.DeepEqual(before, after) {
		t.Error("an unrelated block changed the index")
	}
}

// TestBcmrIndexSharedChains checks authchains that share links: tokens from
// one genesis, a token whose authbase is another identity's authhead, and a
// merge, and that each identity's newest publication is the one on its own
// authchain.  It then checks a burn that carries a final publication.
func TestBcmrIndexSharedChains(t *testing.T) {
	c := newBcmrTestChain(t, 0)

	x1 := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1, "x1.example"))
	x2 := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	y := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, x1, x2, y)
	x1Cat, x2Cat, yCat := x1.TxHash(), x2.TxHash(), y.TxHash()

	// One genesis creates two tokens, and another creates a token with
	// only an NFT, and publishes.
	g := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(x1), bcmrOutPoint(x2)},
		bcmrTokens(x1Cat, 5), bcmrTokens(x2Cat, 7), bcmrTokens(x2Cat, 1))
	gy := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(y)}, bcmrNFT(yCat, wire.MINTING),
		bcmrPub(2, "y.example"))
	c.connect(true, g, gy)
	checkBcmrState(t, "x1", c.state(&x1Cat), g, false, x1, 5)
	checkBcmrState(t, "x2", c.state(&x2Cat), g, false, nil, 8)
	checkBcmrState(t, "y", c.state(&yCat), gy, false, gy, 0)

	// A genesis spends the authhead that x1 and x2 share, which makes it
	// the authbase of a new token.
	gCat := g.TxHash()
	g2 := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(g)}, bcmrTokens(gCat, 3),
		bcmrTokens(x1Cat, 5))
	c.connect(true, g2)
	checkBcmrState(t, "g", c.state(&gCat), g2, false, nil, 3)
	checkBcmrState(t, "x1 after g", c.state(&x1Cat), g2, false, x1, 5)

	// A merge: y's authhead and g2 are spent together.  Every identity
	// carries on through it, each with its own newest publication.
	merge := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(gy), bcmrOutPoint(g2)}, bcmrOut())
	c.connect(true, merge)
	checkBcmrState(t, "x1 merged", c.state(&x1Cat), merge, false, x1, 5)
	checkBcmrState(t, "x2 merged", c.state(&x2Cat), merge, false, nil, 8)
	checkBcmrState(t, "g merged", c.state(&gCat), merge, false, nil, 3)
	checkBcmrState(t, "y merged", c.state(&yCat), merge, false, gy, 0)

	// A publication after the merge is the newest for all of them.
	update := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(merge)}, bcmrOut(), bcmrPub(3, "all.example"))
	c.connect(true, update)
	for _, cat := range []chainhash.Hash{x1Cat, x2Cat, gCat, yCat} {
		if state := c.state(&cat); state.pubLink != update.TxHash() || state.head != update.TxHash() {
			t.Errorf("%v: head %v, publication %v, want %v", cat, state.head,
				state.pubLink, update.TxHash())
		}
	}

	// A burn that carries a final publication in its identity output.
	burn := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(update)}, bcmrPub(4, "final.example"))
	c.connect(true, burn)
	checkBcmrState(t, "x1 burned", c.state(&x1Cat), burn, true, burn, 5)
	checkBcmrState(t, "y burned", c.state(&yCat), burn, true, burn, 0)
	if len(c.idx.heads) != 0 {
		t.Errorf("the index still watches %d heads", len(c.idx.heads))
	}

	// The x1 identity runs through three segments: its authbase's, the
	// genesis's, and the merge's.
	if state := c.state(&x1Cat); len(state.path) != 3 {
		t.Errorf("x1's authchain runs through %d segments, want 3", len(state.path))
	}
	if state := c.state(&gCat); state.identity.place != 0 || len(state.path) != 2 {
		t.Errorf("g starts at place %d and runs through %d segments",
			state.identity.place, len(state.path))
	}
}

// TestBcmrIndexGenesisOnAuthchain checks a transaction that extends an
// authchain, by spending its authhead's identity output, and also creates a
// token whose authbase the index doesn't track yet: the two authchains join
// at it, and disconnecting it puts the index back as it was.
func TestBcmrIndexGenesisOnAuthchain(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	a := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1, "a.example"))
	x := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, a, x)
	aCat, xCat := a.TxHash(), x.TxHash()
	ga := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(aCat, 5))
	c.connect(true, ga)
	before := c.snapshot()

	m := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(ga), bcmrOutPoint(x)},
		bcmrTokens(xCat, 9), bcmrTokens(aCat, 5))
	c.connect(true, m)
	checkBcmrState(t, "a", c.state(&aCat), m, false, a, 5)
	checkBcmrState(t, "x", c.state(&xCat), m, false, nil, 9)
	for _, cat := range []chainhash.Hash{aCat, xCat} {
		if state := c.state(&cat); len(state.path) != 2 {
			t.Errorf("%v runs through %d segments, want 2", cat, len(state.path))
		}
	}
	for _, tx := range []*wire.MsgTx{x, m} {
		c.checkLocation(tx)
	}

	// A publication after the join is the newest for both.
	n := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(m)}, bcmrOut(), bcmrPub(2, "n.example"))
	c.connect(true, n)
	checkBcmrState(t, "a after n", c.state(&aCat), n, false, n, 5)
	checkBcmrState(t, "x after n", c.state(&xCat), n, false, n, 9)

	c.disconnect()
	c.disconnect()
	if got := c.snapshot(); !reflect.DeepEqual(got, before) {
		t.Error("disconnecting the join left a different index")
	}
}

// TestBcmrIndexTokenFromHead checks a token whose authbase is the authhead of
// another identity, partway along a segment: a publication earlier in the
// segment is not on the new token's authchain, while its authbase's own is.
func TestBcmrIndexTokenFromHead(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	a := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1))
	b := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, a, b)
	aCat, bCat := a.TxHash(), b.TxHash()
	ga := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(aCat, 1))
	gb := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(b)}, bcmrTokens(bCat, 1), bcmrPub(2))
	c.connect(true, ga, gb)

	// New tokens from both authheads, at place 1 of their segments.
	gaCat, gbCat := ga.TxHash(), gb.TxHash()
	fromA := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(ga)}, bcmrTokens(gaCat, 2))
	fromB := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(gb)}, bcmrTokens(gbCat, 3))
	c.connect(true, fromA, fromB)
	checkBcmrState(t, "a", c.state(&aCat), fromA, false, a, 1)
	checkBcmrState(t, "token from a's authhead", c.state(&gaCat), fromA, false, nil, 2)
	checkBcmrState(t, "token from b's authhead", c.state(&gbCat), fromB, false, gb, 3)
	if state := c.state(&gaCat); state.identity.place != 1 || len(state.path) != 1 {
		t.Errorf("the token from a's authhead starts at place %d of %d segments",
			state.identity.place, len(state.path))
	}
}

// TestBcmrIndexDisconnect checks that disconnecting each block puts the index
// back exactly as it was before the block, and that connecting the blocks
// again rebuilds the same index.
func TestBcmrIndexDisconnect(t *testing.T) {
	c := newBcmrTestChain(t, 0)

	a := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	b := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1))
	ga := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(a.TxHash(), 10))
	gb := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(b)}, bcmrTokens(b.TxHash(), 20))
	gaCat := ga.TxHash()
	g := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(ga)}, bcmrTokens(gaCat, 1), bcmrPub(2))
	merge := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(g), bcmrOutPoint(gb)}, bcmrOut())
	link := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(merge)}, bcmrOut(), bcmrMalformedPub())
	burn := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(link)}, bcmrPub(3))
	blocks := [][]*wire.MsgTx{
		{a, b},
		{ga, gb},
		{g},
		{merge},
		{bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())},
		{link, burn},
	}

	var snapshots []map[string]string
	for _, txs := range blocks {
		snapshots = append(snapshots, c.snapshot())
		c.connect(true, txs...)
	}
	final := c.snapshot()

	for i := len(blocks) - 1; i >= 0; i-- {
		c.disconnect()
		if got := c.snapshot(); !reflect.DeepEqual(got, snapshots[i]) {
			t.Fatalf("disconnecting block %d left %d entries, want %d",
				i+1, len(got), len(snapshots[i]))
		}
	}
	for _, txs := range blocks {
		c.connect(true, txs...)
	}
	if got := c.snapshot(); !reflect.DeepEqual(got, final) {
		t.Fatal("connecting the blocks again built a different index")
	}

	// Disconnecting a block the index has no undo record for changes
	// nothing.
	c.connect(true, bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut()))
	before := c.snapshot()
	c.disconnect()
	if got := c.snapshot(); !reflect.DeepEqual(got, before) {
		t.Error("disconnecting an unrelated block changed the index")
	}
}

// TestBcmrIndexCatchUp checks that an index that reads the spent outputs of
// the blocks from the spend journal, as it does while it catches up, builds
// the same index as one that the chain hands them, and that it finds an
// authbase in the same block as its genesis.
func TestBcmrIndexCatchUp(t *testing.T) {
	build := func(withStxos bool) map[string]string {
		c := newBcmrTestChain(t, 0)
		a := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1))
		c.connect(withStxos, a, bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut()))
		c.connect(withStxos)
		ga := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(a.TxHash(), 10))
		b := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
		gb := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(b)}, bcmrTokens(b.TxHash(), 4))
		c.connect(withStxos, gb, ga, b)

		aCat, bCat := a.TxHash(), b.TxHash()
		checkBcmrState(t, "a", c.state(&aCat), ga, false, a, 10)
		checkBcmrState(t, "b", c.state(&bCat), gb, false, nil, 4)
		c.checkLocation(a)
		c.checkLocation(b)
		return c.snapshot()
	}
	if !reflect.DeepEqual(build(true), build(false)) {
		t.Error("the spend journal built a different index")
	}
}

// TestBcmrIndexBeforeActivation checks that a block before CashTokens
// activated creates no token.
func TestBcmrIndexBeforeActivation(t *testing.T) {
	c := newBcmrTestChain(t, 2)
	a := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	b := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, a, b)

	ga := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(a.TxHash(), 10))
	c.connect(true, ga)
	aCat := a.TxHash()
	if state := c.state(&aCat); state != nil {
		t.Error("a block before activation created a token")
	}

	gb := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(b)}, bcmrTokens(b.TxHash(), 4))
	c.connect(true, gb)
	bCat := b.TxHash()
	checkBcmrState(t, "after activation", c.state(&bCat), gb, false, nil, 4)
}

// TestBcmrIndexStartBlock checks that the index starts at the last
// checkpoint before CashTokens activated, or at the genesis block without
// one.
func TestBcmrIndexStartBlock(t *testing.T) {
	hash, height := NewBcmrIndex(nil, &chaincfg.MainNetParams).StartBlock()
	if height != 773784 || hash == nil ||
		hash.String() != "0000000000000000045cc0dbdd5cbbb86f7f63596e699ac5a11b2d41c65c6993" {

		t.Errorf("mainnet starts at %v, height %d", hash, height)
	}
	hash, height = NewBcmrIndex(nil, &chaincfg.RegressionNetParams).StartBlock()
	if hash != nil || height != -1 {
		t.Errorf("regtest starts at %v, height %d", hash, height)
	}
}

// TestBcmrIndexVersion checks that the index stores its version, that one a
// later bchd built is refused before it is read, and that one without a
// version, as an earlier one has, is rebuilt from its start block to the same
// index.
func TestBcmrIndexVersion(t *testing.T) {
	c := newBcmrTestChain(t, 0)
	a := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut(), bcmrPub(1))
	c.connect(true, a)
	ga := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(a)}, bcmrTokens(a.TxHash(), 10))
	c.connect(true, ga)
	built := c.snapshot()

	version := func() uint32 {
		t.Helper()
		var version uint32
		err := c.db.View(func(dbTx database.Tx) error {
			var err error
			version, err = dbFetchIndexVersion(dbTx, bcmrIndexKey)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return version
	}
	if got := version(); got != bcmrIndexVersion {
		t.Fatalf("created at version %d, want %d", got, bcmrIndexVersion)
	}

	err := c.db.Update(func(dbTx database.Tx) error {
		return dbPutIndexVersion(dbTx, bcmrIndexKey, bcmrIndexVersion+1)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.idx.Init(); err == nil || !strings.Contains(err.Error(), "--dropbcmrindex") {
		t.Errorf("initializing an index of a later version: %v", err)
	}

	err = c.db.Update(func(dbTx database.Tx) error {
		tips := dbTx.Metadata().Bucket(indexTipsBucketName)
		return tips.Delete(indexVersionKey(bcmrIndexKey))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.idx.Init(); err != nil {
		t.Fatalf("initializing an index without a version: %v", err)
	}
	if err := c.idx.Migrate(c.db, nil); err != nil {
		t.Fatal(err)
	}
	if got := c.snapshot(); len(got) != 0 {
		t.Errorf("the rebuilt index starts with %d entries", len(got))
	}
	if got := version(); got != bcmrIndexVersion {
		t.Errorf("rebuilt at version %d, want %d", got, bcmrIndexVersion)
	}

	// Connecting the blocks again, as the index manager does from the
	// index's tip, builds the same index.
	for _, block := range c.blocks {
		err := c.db.Update(func(dbTx database.Tx) error {
			return dbIndexConnectBlock(dbTx, c.idx, block, c.stxos[*block.Hash()])
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := c.snapshot(); !reflect.DeepEqual(got, built) {
		t.Error("the rebuilt index differs from the one built before")
	}
}

// TestBcmrIndexPruneUndo checks that the index keeps the undo records of the
// latest blocks only, and that it refuses to disconnect a block deeper than
// that, rather than leave the index wrong.
func TestBcmrIndexPruneUndo(t *testing.T) {
	saved := bcmrUndoDepth
	bcmrUndoDepth = 3
	defer func() { bcmrUndoDepth = saved }()

	c := newBcmrTestChain(t, 0)
	authbase := bcmrTestTx([]wire.OutPoint{c.funding()}, bcmrOut())
	c.connect(true, authbase)
	category := authbase.TxHash()
	link := bcmrTestTx([]wire.OutPoint{bcmrOutPoint(authbase)}, bcmrTokens(category, 10))
	c.connect(true, link)
	for i := 0; i < 4; i++ {
		link = bcmrTestTx([]wire.OutPoint{bcmrOutPoint(link)}, bcmrTokens(category, 10))
		c.connect(true, link)
	}

	// Blocks 2 to 6 changed the index; the records of 2 and 3 are pruned.
	undoHeights := func() []int32 {
		var heights []int32
		for key := range c.snapshot() {
			var height int32
			if _, err := fmt.Sscanf(key, "undo/%08x", &height); err == nil {
				heights = append(heights, height)
			}
		}
		slices.Sort(heights)
		return heights
	}
	if got := undoHeights(); !reflect.DeepEqual(got, []int32{4, 5, 6}) {
		t.Fatalf("undo records at heights %v, want 4, 5 and 6", got)
	}

	for i := 0; i < 3; i++ {
		c.disconnect()
	}
	block := c.blocks[len(c.blocks)-1]
	err := c.db.Update(func(dbTx database.Tx) error {
		return dbIndexDisconnectBlock(dbTx, c.idx, block, c.stxos[*block.Hash()])
	})
	if err == nil || !strings.Contains(err.Error(), "--dropbcmrindex") {
		t.Errorf("disconnecting a block below the pruned records: %v", err)
	}
}
