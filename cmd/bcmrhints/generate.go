// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// minDepth is how far below the tip the height must be, so that a
// reorganization can't change the file between two nodes.
const minDepth = 100

// firstPageLinks is how many links a walk asks for first.  Most tokens reach a
// link another's walk did within a few, so a full page of up to 1000, which the
// node reads for each, would mostly go unused.  The pages after it are full.
const firstPageLinks = 16

// source is what generating hints needs of a node.
type source interface {
	GetBlockCount() (int64, error)
	ListBcmrIdentities(after *chainhash.Hash, limit int) (*btcjson.ListBcmrIdentitiesResult, error)
	GetBcmrIdentity(authbase *chainhash.Hash, includeAuthchain bool, after *chainhash.Hash,
		limit int) (*btcjson.GetBcmrIdentityResult, error)
}

// generate returns the hints of every token as of the passed height: only
// what the blocks up to that height say, whatever the tip is, so that every
// node gives the same answer.
func generate(node source, height int32) ([]bcmr.Hint, error) {
	tip, err := node.GetBlockCount()
	if err != nil {
		return nil, err
	}
	if height <= 0 || int64(height) > tip-minDepth {
		return nil, fmt.Errorf("height %d must be above 0 and at least %d blocks "+
			"below the tip at %d", height, minDepth, tip)
	}

	walker := &chainWalker{node: node, height: height,
		from: make(map[chainhash.Hash]chainFrom)}
	var hints []bcmr.Hint
	var after *chainhash.Hash
	for {
		page, err := node.ListBcmrIdentities(after, 0)
		if err != nil {
			return nil, err
		}
		for i := range page.Identities {
			hint, err := walker.identityHint(&page.Identities[i])
			if err != nil {
				return nil, fmt.Errorf("identity %s: %w",
					page.Identities[i].Authbase, err)
			}
			if hint != nil {
				hints = append(hints, *hint)
			}
		}
		if !page.More {
			return hints, nil
		}
		if len(page.Identities) == 0 {
			return nil, fmt.Errorf("a page with more has no identities")
		}
		after, err = chainhash.NewHashFromStr(page.Identities[len(page.Identities)-1].Authbase)
		if err != nil {
			return nil, err
		}
	}
}

// chainWalker walks authchains back to the height, remembering where the chain
// stood then from each link it walked, so that the tokens whose authchains join
// walk the links they share once, and the work is in proportion to the links,
// not to them times the tokens through them.
type chainWalker struct {
	node   source
	height int32
	from   map[chainhash.Hash]chainFrom
}

// chainFrom is where an authchain stood at the height, from one of its links at
// or below it: the last link at or below it, and the newest well-formed
// publication of the links from the one to it, or nil.
type chainFrom struct {
	head, publication *btcjson.BcmrLinkResult
}

// identityHint returns the hint of an identity as of the height, or nil if the
// token's genesis came after it.
func (w *chainWalker) identityHint(identity *btcjson.GetBcmrIdentityResult) (*bcmr.Hint, error) {
	height := w.height
	category, err := chainhash.NewHashFromStr(identity.Authbase)
	if err != nil {
		return nil, err
	}

	// An authchain that hasn't moved since is as the node answered.
	if head := identity.Head; head.Height > 0 && head.Height <= height {
		hint := &bcmr.Hint{Category: *category, HeadHeight: head.Height,
			Burned: identity.Status == "burned"}
		headHash, err := chainhash.NewHashFromStr(head.Txid)
		if err != nil {
			return nil, err
		}
		hint.Head = *headHash
		if pub := identity.Publication; pub != nil && pub.Txid != head.Txid {
			txid, err := chainhash.NewHashFromStr(pub.Txid)
			if err != nil {
				return nil, err
			}
			hint.Publication = &bcmr.HintPublication{Txid: *txid, Height: pub.Height}
		}
		return hint, nil
	}
	return w.walkToHeight(category)
}

// walkToHeight reads an authchain that moved past the height, for the hint of
// where it stood then, up to the first link another token's walk reached.  The
// link after the last at the height spent its identity output later, so that
// link didn't burn the identity.
func (w *chainWalker) walkToHeight(category *chainhash.Hash) (*bcmr.Hint, error) {
	height := w.height
	var head, publication *btcjson.BcmrLinkResult
	var walked []*btcjson.BcmrLinkResult
	var reached *chainFrom
	var after *chainhash.Hash
	sawGenesis := false
walk:
	for {
		limit := 0
		if after == nil {
			limit = firstPageLinks
		}
		answer, err := w.node.GetBcmrIdentity(category, true, after, limit)
		if err != nil {
			return nil, err
		}
		links, err := orderedLinks(answer)
		if err != nil {
			return nil, err
		}
		for i := range links {
			link := &links[i].info
			if link.Height <= 0 || link.Height > height {
				break walk
			}
			txid, err := chainhash.NewHashFromStr(link.Txid)
			if err != nil {
				return nil, err
			}
			if from, ok := w.from[*txid]; ok {
				// The chain from here is as another token's walk
				// found it, which ran past this link, unless the
				// link was its head.
				reached = &from
				if links[i].index >= 1 || from.head.Txid != link.Txid {
					sawGenesis = true
				}
				break walk
			}
			head = link
			walked = append(walked, link)
			if links[i].index >= 1 {
				sawGenesis = true
			}
			if link.Publication != nil && !link.Publication.Malformed {
				publication = link
			}
		}
		if answer.More == nil || !*answer.More || len(links) == 0 {
			break
		}
		after, err = chainhash.NewHashFromStr(links[len(links)-1].info.Txid)
		if err != nil {
			return nil, err
		}
	}

	if reached != nil {
		head = reached.head
		if reached.publication != nil {
			publication = reached.publication
		}
	}
	w.remember(walked, head, reached)

	// The authbase can come before the genesis, which makes the token.
	if !sawGenesis {
		return nil, nil
	}
	headHash, err := chainhash.NewHashFromStr(head.Txid)
	if err != nil {
		return nil, err
	}
	hint := &bcmr.Hint{Category: *category, Head: *headHash, HeadHeight: head.Height}
	if publication != nil && publication != head {
		txid, err := chainhash.NewHashFromStr(publication.Txid)
		if err != nil {
			return nil, err
		}
		hint.Publication = &bcmr.HintPublication{Txid: *txid, Height: publication.Height}
	}
	return hint, nil
}

// remember keeps where the chain stood at the height from each of the links a
// walk read, all at or below it, which end at head or at the link reached.
func (w *chainWalker) remember(walked []*btcjson.BcmrLinkResult, head *btcjson.BcmrLinkResult,
	reached *chainFrom) {

	var publication *btcjson.BcmrLinkResult
	if reached != nil {
		publication = reached.publication
	}
	for i := len(walked) - 1; i >= 0; i-- {
		link := walked[i]
		if publication == nil && link.Publication != nil && !link.Publication.Malformed {
			publication = link
		}
		txid, err := chainhash.NewHashFromStr(link.Txid)
		if err != nil {
			continue
		}
		w.from[*txid] = chainFrom{head: head, publication: publication}
	}
}

// indexedLink is a link of an answer with its index from the authbase.
type indexedLink struct {
	index uint64
	info  btcjson.BcmrLinkResult
}

// orderedLinks returns the links of an answer in the order of the authchain.
func orderedLinks(answer *btcjson.GetBcmrIdentityResult) ([]indexedLink, error) {
	if answer.AuthchainInfo == nil {
		return nil, fmt.Errorf("the answer has no authchain")
	}
	links := make([]indexedLink, 0, len(*answer.AuthchainInfo))
	for key, info := range *answer.AuthchainInfo {
		index, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			return nil, err
		}
		links = append(links, indexedLink{index, info})
	}
	slices.SortFunc(links, func(a, b indexedLink) int {
		return cmp.Compare(a.index, b.index)
	})
	return links, nil
}
