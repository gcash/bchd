// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"reflect"
	"testing"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestBcmrRecords checks that the index's records decode to what they were,
// and that corrupt ones fail to decode.
func TestBcmrRecords(t *testing.T) {
	links := []*bcmrLink{
		{height: 7, offset: 81, length: 250, segment: 3, place: 2},
		{height: 1, next: chainhash.Hash{9}, pub: &bcmr.Publication{Output: 2, Malformed: true}},
		{blockHash: chainhash.Hash{1}, pub: &bcmr.Publication{Output: 1, Hash: [32]byte{5},
			URIs: []string{"example.com", "", "ipfs://bafkreiexample"}}},
		{pub: &bcmr.Publication{Output: 0, Hash: [32]byte{6}}},
	}
	for _, link := range links {
		serialized := serializeBcmrLink(link)
		got, err := deserializeBcmrLink(serialized)
		if err != nil || !reflect.DeepEqual(got, link) {
			t.Errorf("link %+v decoded as %+v, %v", link, got, err)
		}
		for n := 0; n < len(serialized); n++ {
			if _, err := deserializeBcmrLink(serialized[:n]); err == nil {
				t.Errorf("link cut to %d bytes decoded", n)
			}
		}
		if _, err := deserializeBcmrLink(append(serialized, 0)); err == nil {
			t.Error("link with a trailing byte decoded")
		}
	}

	segment := &bcmrSegment{length: 3, first: chainhash.Hash{1}, head: chainhash.Hash{2},
		pubLink: chainhash.Hash{3}, pubPlace: 1, next: 9, burned: true}
	if got, err := deserializeBcmrSegment(serializeBcmrSegment(segment)); err != nil ||
		!reflect.DeepEqual(got, segment) {

		t.Errorf("segment decoded as %+v, %v", got, err)
	}
	for _, identity := range []*bcmrIdentity{
		{segment: 4, place: 2, supply: 1 << 62},
		{segment: 5},
	} {
		if got, err := deserializeBcmrIdentity(serializeBcmrIdentity(identity)); err != nil ||
			!reflect.DeepEqual(got, identity) {

			t.Errorf("identity decoded as %+v, %v", got, err)
		}
	}

	undo := []bcmrUndoEntry{
		{bucket: bcmrLinks, key: []byte{1, 2}, hadOld: true, old: []byte{3}},
		{bucket: bcmrHeads, key: []byte{4}},
		{bucket: bcmrTop, key: bcmrNextSegmentKey, hadOld: true, old: []byte{}},
	}
	serialized := serializeBcmrUndo(undo)
	if got, err := deserializeBcmrUndo(serialized); err != nil || !reflect.DeepEqual(got, undo) {
		t.Errorf("undo record decoded as %+v, %v", got, err)
	}
	for n := 1; n < len(serialized); n++ {
		if got, err := deserializeBcmrUndo(serialized[:n]); err == nil &&
			reflect.DeepEqual(got, undo) {

			t.Errorf("undo record cut to %d bytes decoded whole", n)
		}
	}
	if _, err := deserializeBcmrUndo([]byte{bcmrUndo, 0, 0}); err == nil {
		t.Error("undo record for the undo bucket decoded")
	}
}
