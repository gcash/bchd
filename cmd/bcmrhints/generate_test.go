// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// testLink is a link of a test authchain, at a height, with a publication if
// pub is set, and malformed then if bad is.  A link with a txid is that
// transaction, which other authchains can share, and else one of its own.
type testLink struct {
	height   int32
	pub, bad bool
	txid     chainhash.Hash
}

// testNode answers for authchains from their links, paging both calls.
type testNode struct {
	tip       int64
	chains    map[chainhash.Hash][]testLink
	burned    map[chainhash.Hash]bool
	pageSize  int
	linkPage  int
	listCalls int
	served    int
}

// linkTxid returns the txid of link i of the authchain of the category.
func (n *testNode) linkTxid(category chainhash.Hash, i int) chainhash.Hash {
	if txid := n.chains[category][i].txid; txid != (chainhash.Hash{}) {
		return txid
	}
	return testHash(category[0], i)
}

// testHash returns the hash of link i of the authchain of category c.
func testHash(c byte, i int) chainhash.Hash {
	return chainhash.Hash{c, byte(i + 1)}
}

func (n *testNode) GetBlockCount() (int64, error) {
	return n.tip, nil
}

// categories returns the categories in the order the index lists them.
func (n *testNode) categories() []chainhash.Hash {
	var all []chainhash.Hash
	for c := 0; c < 256; c++ {
		h := testHash(byte(c), 0)
		if _, ok := n.chains[h]; ok {
			all = append(all, h)
		}
	}
	return all
}

func (n *testNode) identity(category chainhash.Hash) *btcjson.GetBcmrIdentityResult {
	links := n.chains[category]
	last := len(links) - 1
	result := &btcjson.GetBcmrIdentityResult{
		Authbase: category.String(),
		Status:   "active",
		Head: btcjson.BcmrLinkResult{
			Txid:   n.linkTxid(category, last).String(),
			Height: links[last].height,
		},
	}
	if n.burned[category] {
		result.Status = "burned"
	}
	for i := last; i >= 0; i-- {
		if links[i].pub && !links[i].bad {
			result.Publication = &btcjson.BcmrPublicationResult{
				Txid:   n.linkTxid(category, i).String(),
				Height: links[i].height,
			}
			break
		}
	}
	return result
}

func (n *testNode) ListBcmrIdentities(after *chainhash.Hash, _ int) (*btcjson.ListBcmrIdentitiesResult, error) {
	n.listCalls++
	all := n.categories()
	start := 0
	if after != nil {
		for i := range all {
			if all[i] == *after {
				start = i + 1
			}
		}
	}
	end := min(start+n.pageSize, len(all))
	result := &btcjson.ListBcmrIdentitiesResult{More: end < len(all)}
	for _, category := range all[start:end] {
		result.Identities = append(result.Identities, *n.identity(category))
	}
	return result, nil
}

func (n *testNode) GetBcmrIdentity(authbase *chainhash.Hash, _ bool, after *chainhash.Hash,
	_ int) (*btcjson.GetBcmrIdentityResult, error) {

	links := n.chains[*authbase]
	start := 0
	if after != nil {
		for i := range links {
			if n.linkTxid(*authbase, i) == *after {
				start = i + 1
			}
		}
	}
	end := min(start+n.linkPage, len(links))
	n.served += end - start
	result := n.identity(*authbase)
	info := make(map[string]btcjson.BcmrLinkResult)
	for i := start; i < end; i++ {
		link := btcjson.BcmrLinkResult{
			Txid:   n.linkTxid(*authbase, i).String(),
			Height: links[i].height,
		}
		if links[i].pub {
			link.Publication = &btcjson.BcmrPublicationOutputResult{Malformed: links[i].bad}
		}
		info[strconv.Itoa(i)] = link
	}
	more := end < len(links)
	result.AuthchainInfo, result.More = &info, &more
	return result, nil
}

// newTestNode returns a node with these authchains, by their first byte.
func newTestNode(chains map[byte][]testLink, burned ...byte) *testNode {
	n := &testNode{
		tip:      10000,
		chains:   make(map[chainhash.Hash][]testLink),
		burned:   make(map[chainhash.Hash]bool),
		pageSize: 2,
		linkPage: 2,
	}
	for c, links := range chains {
		n.chains[testHash(c, 0)] = links
	}
	for _, c := range burned {
		n.burned[testHash(c, 0)] = true
	}
	return n
}

// TestGenerate ensures each hint says what the chain said at the height,
// whatever happened after it.
func TestGenerate(t *testing.T) {
	node := newTestNode(map[byte][]testLink{
		// Hasn't moved since the height.
		1: {{height: 100}, {height: 110}, {height: 120, pub: true}},
		// Burned before the height.
		2: {{height: 100}, {height: 110, pub: true}, {height: 120}},
		// Burned after the height, with links past it.
		3: {{height: 100, pub: true}, {height: 110}, {height: 900}, {height: 950}},
		// A publication after the height doesn't count, and a head
		// after it is walked back through several pages.
		4: {{height: 100}, {height: 110, pub: true}, {height: 120}, {height: 130},
			{height: 140}, {height: 900, pub: true}},
		// Only an authbase at the height: the token came after.
		5: {{height: 100}, {height: 900}},
		// Wholly after the height.
		6: {{height: 800}, {height: 900}},
		// A malformed publication doesn't count, in either path.
		7: {{height: 100, pub: true}, {height: 110, pub: true, bad: true}, {height: 900}},
		8: {{height: 100, pub: true}, {height: 110, pub: true, bad: true}},
		// Moved past the height in the same block as the next link.
		9: {{height: 100}, {height: 500}, {height: 500}, {height: 600}},
	}, 2, 3)

	hints, err := generate(node, 500)
	if err != nil {
		t.Fatal(err)
	}
	pub := func(c byte, i int, height int32) *bcmr.HintPublication {
		return &bcmr.HintPublication{Txid: testHash(c, i), Height: height}
	}
	want := []bcmr.Hint{
		{Category: testHash(1, 0), Head: testHash(1, 2), HeadHeight: 120},
		{Category: testHash(2, 0), Head: testHash(2, 2), HeadHeight: 120, Burned: true,
			Publication: pub(2, 1, 110)},
		{Category: testHash(3, 0), Head: testHash(3, 1), HeadHeight: 110,
			Publication: pub(3, 0, 100)},
		{Category: testHash(4, 0), Head: testHash(4, 4), HeadHeight: 140,
			Publication: pub(4, 1, 110)},
		{Category: testHash(7, 0), Head: testHash(7, 1), HeadHeight: 110,
			Publication: pub(7, 0, 100)},
		{Category: testHash(8, 0), Head: testHash(8, 1), HeadHeight: 110,
			Publication: pub(8, 0, 100)},
		{Category: testHash(9, 0), Head: testHash(9, 2), HeadHeight: 500},
	}
	if !reflect.DeepEqual(hints, want) {
		t.Fatalf("got  %+v\nwant %+v", hints, want)
	}
}

// TestGenerateIgnoresLaterChain ensures a tip that moved on doesn't change the
// answer for a height: this is what lets anyone check a published file.
func TestGenerateIgnoresLaterChain(t *testing.T) {
	early := newTestNode(map[byte][]testLink{
		1: {{height: 100}, {height: 110, pub: true}},
		2: {{height: 100}, {height: 110}},
	})
	late := newTestNode(map[byte][]testLink{
		1: {{height: 100}, {height: 110, pub: true}, {height: 700, pub: true}},
		2: {{height: 100}, {height: 110}, {height: 800}},
		3: {{height: 600}, {height: 610, pub: true}},
	}, 2)
	late.tip = 20000

	a, err := generate(early, 500)
	if err != nil {
		t.Fatal(err)
	}
	b, err := generate(late, 500)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bcmr.EncodeHints(500, a), bcmr.EncodeHints(500, b)) {
		t.Fatalf("the file changed: %+v\n%+v", a, b)
	}
}

// TestGenerateHeightLimits ensures a height a reorganization could change, or
// that isn't one, is refused before anything is read.
func TestGenerateHeightLimits(t *testing.T) {
	node := newTestNode(map[byte][]testLink{1: {{height: 100}, {height: 110}}})
	for _, height := range []int32{0, -1, 9901, 20000} {
		if _, err := generate(node, height); err == nil {
			t.Errorf("height %d was accepted", height)
		}
	}
	if node.listCalls != 0 {
		t.Fatal("the index was read for a refused height")
	}
	if _, err := generate(node, 9900); err != nil {
		t.Fatalf("the deepest allowed height: %v", err)
	}
}

// TestGenerateSharedChain ensures tokens whose authchains join walk the links
// they share once, so that the work is in proportion to the links rather than
// to them times the tokens, and that each still gets its own hint: the head of
// the shared chain, and the newest publication of its own links and the
// shared ones.
func TestGenerateSharedChain(t *testing.T) {
	const tokens, sharedLinks = 20, 60
	var shared []testLink
	for i := 0; i < sharedLinks; i++ {
		shared = append(shared, testLink{height: int32(200 + i),
			txid: chainhash.Hash{0xee, byte(i + 1)}})
	}
	// A publication at link 10, and one after the height.
	shared[10].pub = true
	shared = append(shared, testLink{height: 900, pub: true, txid: chainhash.Hash{0xee, 0xff}})

	chains := make(map[byte][]testLink)
	for c := 1; c <= tokens; c++ {
		chains[byte(c)] = append([]testLink{{height: 100}}, shared...)
	}
	// One with a publication of its own before the join, and one that
	// joins after link 10's publication, which it doesn't have.
	chains[1] = append([]testLink{{height: 100, pub: true}}, shared...)
	chains[2] = append([]testLink{{height: 100, pub: true}}, shared[11:]...)
	node := newTestNode(chains)

	hints, err := generate(node, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != tokens {
		t.Fatalf("%d hints, want %d", len(hints), tokens)
	}
	head := shared[sharedLinks-1].txid
	for _, hint := range hints {
		want := bcmr.Hint{Category: hint.Category, Head: head, HeadHeight: 200 + sharedLinks - 1,
			Publication: &bcmr.HintPublication{Txid: shared[10].txid, Height: 210}}
		if hint.Category == testHash(2, 0) {
			want.Publication = &bcmr.HintPublication{Txid: testHash(2, 0), Height: 100}
		}
		if !reflect.DeepEqual(hint, want) {
			t.Errorf("got  %+v\nwant %+v", hint, want)
		}
	}

	// The first token walks the shared links, and each other walks its
	// authbase and a page into them.
	if limit := sharedLinks + 1 + tokens*2*node.linkPage; node.served > limit {
		t.Errorf("the node served %d links, more than %d", node.served, limit)
	}
}
