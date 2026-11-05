// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
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

	var hints []bcmr.Hint
	var after *chainhash.Hash
	for {
		page, err := node.ListBcmrIdentities(after, 0)
		if err != nil {
			return nil, err
		}
		for i := range page.Identities {
			hint, err := identityHint(node, &page.Identities[i], height)
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

// identityHint returns the hint of an identity as of height, or nil if the
// token's genesis came after it.
func identityHint(node source, identity *btcjson.GetBcmrIdentityResult, height int32) (*bcmr.Hint, error) {
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
	return walkToHeight(node, category, height)
}

// walkToHeight reads an authchain that moved past height, for the hint of where
// it stood then.  The link after the last at height spent its identity output
// later, so that link didn't burn the identity.
func walkToHeight(node source, category *chainhash.Hash, height int32) (*bcmr.Hint, error) {
	var head, publication *btcjson.BcmrLinkResult
	var after *chainhash.Hash
	sawGenesis := false
walk:
	for {
		answer, err := node.GetBcmrIdentity(category, true, after, 0)
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
			head = link
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
		return int(int64(a.index) - int64(b.index))
	})
	return links, nil
}
