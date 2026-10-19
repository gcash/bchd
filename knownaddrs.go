// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import "container/list"

// maxKnownAddrsPerPeer is how many addresses a peer is remembered to know, as
// many as Bitcoin Cash Node remembers.  Forgetting one at worst sends the peer
// an address it already has.
const maxKnownAddrsPerPeer = 5000

// knownAddrSet is a set of address keys that holds at most limit of them, and
// drops the least recently added to make room.  It is not safe for concurrent
// access.
type knownAddrSet struct {
	keys  map[string]*list.Element
	order *list.List // the most recently added at the front
	limit int
}

// newKnownAddrSet returns an empty set that holds at most limit keys.
func newKnownAddrSet(limit int) *knownAddrSet {
	return &knownAddrSet{
		keys:  make(map[string]*list.Element),
		order: list.New(),
		limit: limit,
	}
}

// add adds key to the set, or makes it the most recently added if it is in
// the set already.
func (s *knownAddrSet) add(key string) {
	if e, ok := s.keys[key]; ok {
		s.order.MoveToFront(e)
		return
	}
	if s.order.Len() >= s.limit {
		oldest := s.order.Back()
		delete(s.keys, oldest.Value.(string))
		s.order.Remove(oldest)
	}
	s.keys[key] = s.order.PushFront(key)
}

// has returns whether key is in the set.
func (s *knownAddrSet) has(key string) bool {
	_, ok := s.keys[key]
	return ok
}
