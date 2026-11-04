// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
)

// bcmrLink is a transaction of an authchain as the index keeps it.
type bcmrLink struct {
	// blockHash, height, offset and length locate the transaction in the
	// block files.
	blockHash chainhash.Hash
	height    int32
	offset    uint32
	length    uint32

	// segment and place are the segment the link is in and its place in
	// that segment, counted from 0.
	segment uint64
	place   uint32

	// next is the transaction that spent the link's identity output, or
	// the zero hash while it is unspent.
	next chainhash.Hash

	// pub is the link's BCMR publication, or nil when it has none.
	pub *bcmr.Publication
}

// bcmrLinkFixedSize is the size of the fields of a serialized link that come
// before its publication.
const bcmrLinkFixedSize = chainhash.HashSize + 4 + 4 + 4 + 8 + 4 + chainhash.HashSize

// The first byte of a serialized publication tells which kind it is.
const (
	bcmrNoPublication byte = iota
	bcmrWellFormedPublication
	bcmrMalformedPublication
)

// serializeBcmrLink serializes a link as
//
//	<block hash><height><offset><length><segment><place><next><publication>
//
// where the publication is a byte telling its kind, followed, if there is
// one, by the output it is in, and, if it is well-formed, by its hash, its
// number of URIs, and each URI's length and bytes, as varints where they are
// counts or lengths.
func serializeBcmrLink(l *bcmrLink) []byte {
	buf := make([]byte, bcmrLinkFixedSize, bcmrLinkFixedSize+1+4+chainhash.HashSize)
	copy(buf[0:32], l.blockHash[:])
	byteOrder.PutUint32(buf[32:36], uint32(l.height))
	byteOrder.PutUint32(buf[36:40], l.offset)
	byteOrder.PutUint32(buf[40:44], l.length)
	byteOrder.PutUint64(buf[44:52], l.segment)
	byteOrder.PutUint32(buf[52:56], l.place)
	copy(buf[56:88], l.next[:])

	switch pub := l.pub; {
	case pub == nil:
		buf = append(buf, bcmrNoPublication)
	case pub.Malformed:
		buf = append(buf, bcmrMalformedPublication)
		buf = binary.LittleEndian.AppendUint32(buf, pub.Output)
	default:
		buf = append(buf, bcmrWellFormedPublication)
		buf = binary.LittleEndian.AppendUint32(buf, pub.Output)
		buf = append(buf, pub.Hash[:]...)
		buf = binary.AppendUvarint(buf, uint64(len(pub.URIs)))
		for _, uri := range pub.URIs {
			buf = binary.AppendUvarint(buf, uint64(len(uri)))
			buf = append(buf, uri...)
		}
	}
	return buf
}

// deserializeBcmrLink decodes a link that serializeBcmrLink serialized.
func deserializeBcmrLink(b []byte) (*bcmrLink, error) {
	if len(b) < bcmrLinkFixedSize+1 {
		return nil, errDeserialize("BCMR link too short")
	}
	l := &bcmrLink{
		height:  int32(byteOrder.Uint32(b[32:36])),
		offset:  byteOrder.Uint32(b[36:40]),
		length:  byteOrder.Uint32(b[40:44]),
		segment: byteOrder.Uint64(b[44:52]),
		place:   byteOrder.Uint32(b[52:56]),
	}
	copy(l.blockHash[:], b[0:32])
	copy(l.next[:], b[56:88])

	kind, rest := b[bcmrLinkFixedSize], b[bcmrLinkFixedSize+1:]
	if kind == bcmrNoPublication {
		if len(rest) != 0 {
			return nil, errDeserialize("BCMR link has trailing bytes")
		}
		return l, nil
	}
	if (kind != bcmrWellFormedPublication && kind != bcmrMalformedPublication) || len(rest) < 4 {
		return nil, errDeserialize("BCMR link has a bad publication")
	}
	pub := &bcmr.Publication{Output: binary.LittleEndian.Uint32(rest[:4])}
	rest = rest[4:]
	l.pub = pub
	if kind == bcmrMalformedPublication {
		if len(rest) != 0 {
			return nil, errDeserialize("BCMR link has trailing bytes")
		}
		pub.Malformed = true
		return l, nil
	}

	if len(rest) < chainhash.HashSize {
		return nil, errDeserialize("BCMR publication hash too short")
	}
	copy(pub.Hash[:], rest[:chainhash.HashSize])
	rest = rest[chainhash.HashSize:]
	count, n := binary.Uvarint(rest)
	if n <= 0 || count > uint64(len(rest)) {
		return nil, errDeserialize("BCMR publication has a bad URI count")
	}
	rest = rest[n:]
	for i := uint64(0); i < count; i++ {
		size, n := binary.Uvarint(rest)
		if n <= 0 || size > uint64(len(rest)-n) {
			return nil, errDeserialize("BCMR publication has a bad URI")
		}
		pub.URIs = append(pub.URIs, string(rest[n:n+int(size)]))
		rest = rest[n+int(size):]
	}
	if len(rest) != 0 {
		return nil, errDeserialize("BCMR link has trailing bytes")
	}
	return l, nil
}

// bcmrSegment is a run of links between the points where authchains join.
type bcmrSegment struct {
	// length is the number of links in the segment.
	length uint32

	// first and head are the segment's first and last links.
	first chainhash.Hash
	head  chainhash.Hash

	// pubLink and pubPlace are the link of the segment with the newest
	// well-formed publication and its place, or the zero hash if none of
	// its links has one.
	pubLink  chainhash.Hash
	pubPlace uint32

	// next is the segment that this one continues into, or 0 while it is
	// current.
	next uint64

	// burned reports, for a current segment, that its head spent the
	// identity output to a data-carrier output, which ends the authchain.
	burned bool
}

// bcmrSegmentSize is the size of a serialized segment.
const bcmrSegmentSize = 4 + 3*chainhash.HashSize + 4 + 8 + 1

// serializeBcmrSegment serializes a segment as
//
//	<length><first><head><publication link><publication place><next><burned>
func serializeBcmrSegment(s *bcmrSegment) []byte {
	buf := make([]byte, bcmrSegmentSize)
	byteOrder.PutUint32(buf[0:4], s.length)
	copy(buf[4:36], s.first[:])
	copy(buf[36:68], s.head[:])
	copy(buf[68:100], s.pubLink[:])
	byteOrder.PutUint32(buf[100:104], s.pubPlace)
	byteOrder.PutUint64(buf[104:112], s.next)
	if s.burned {
		buf[112] = 1
	}
	return buf
}

// deserializeBcmrSegment decodes a segment that serializeBcmrSegment
// serialized.
func deserializeBcmrSegment(b []byte) (*bcmrSegment, error) {
	if len(b) != bcmrSegmentSize {
		return nil, errDeserialize("BCMR segment has the wrong size")
	}
	s := &bcmrSegment{
		length:   byteOrder.Uint32(b[0:4]),
		pubPlace: byteOrder.Uint32(b[100:104]),
		next:     byteOrder.Uint64(b[104:112]),
		burned:   b[112] == 1,
	}
	copy(s.first[:], b[4:36])
	copy(s.head[:], b[36:68])
	copy(s.pubLink[:], b[68:100])
	return s, nil
}

// notePublication makes the link at place the segment's newest publication
// if it has a well-formed one.  Links come to a segment in order, so the last
// one noted is the newest.
func (s *bcmrSegment) notePublication(txid chainhash.Hash, place uint32, pub *bcmr.Publication) {
	if pub.WellFormed() {
		s.pubLink = txid
		s.pubPlace = place
	}
}

// bcmrIdentity is the identity of a token category the index tracks.
type bcmrIdentity struct {
	// segment and place are where the authbase is.
	segment uint64
	place   uint32

	// supply is the number of fungible tokens the category's genesis
	// created.
	supply uint64
}

// bcmrIdentitySize is the size of a serialized identity.
const bcmrIdentitySize = 8 + 4 + 8

// serializeBcmrIdentity serializes an identity as
//
//	<segment><place><supply>
func serializeBcmrIdentity(id *bcmrIdentity) []byte {
	buf := make([]byte, bcmrIdentitySize)
	byteOrder.PutUint64(buf[0:8], id.segment)
	byteOrder.PutUint32(buf[8:12], id.place)
	byteOrder.PutUint64(buf[12:20], id.supply)
	return buf
}

// deserializeBcmrIdentity decodes an identity that serializeBcmrIdentity
// serialized.
func deserializeBcmrIdentity(b []byte) (*bcmrIdentity, error) {
	if len(b) != bcmrIdentitySize {
		return nil, errDeserialize("BCMR identity has the wrong size")
	}
	return &bcmrIdentity{
		segment: byteOrder.Uint64(b[0:8]),
		place:   byteOrder.Uint32(b[8:12]),
		supply:  byteOrder.Uint64(b[12:20]),
	}, nil
}

// bcmrSegmentKey returns the database key of a segment, its ID in big-endian
// order, so that a cursor goes through segments in the order they were made.
func bcmrSegmentKey(id uint64) []byte {
	return binary.BigEndian.AppendUint64(nil, id)
}

// bcmrUndoKey returns the database key of the undo record of the block at a
// height, the height in big-endian order.  The index holds one block at each
// height, the main chain's, so the height names it.
func bcmrUndoKey(height int32) []byte {
	return binary.BigEndian.AppendUint32(nil, uint32(height))
}

// bcmrUndoEntry is the value an entry of the index had before a block
// changed it.
type bcmrUndoEntry struct {
	bucket byte
	key    []byte
	hadOld bool
	old    []byte
}

// serializeBcmrUndo serializes a block's undo record, its entries in the
// order the block first changed them, each as
//
//	<bucket><key length><key><whether it had a value><value length><value>
//
// with the lengths as varints, and the value's length and bytes only when it
// had one.
func serializeBcmrUndo(entries []bcmrUndoEntry) []byte {
	var buf []byte
	for _, e := range entries {
		buf = append(buf, e.bucket)
		buf = binary.AppendUvarint(buf, uint64(len(e.key)))
		buf = append(buf, e.key...)
		if !e.hadOld {
			buf = append(buf, 0)
			continue
		}
		buf = append(buf, 1)
		buf = binary.AppendUvarint(buf, uint64(len(e.old)))
		buf = append(buf, e.old...)
	}
	return buf
}

// deserializeBcmrUndo decodes an undo record that serializeBcmrUndo
// serialized.
func deserializeBcmrUndo(b []byte) ([]bcmrUndoEntry, error) {
	var entries []bcmrUndoEntry
	for len(b) > 0 {
		e := bcmrUndoEntry{bucket: b[0]}
		if e.bucket >= bcmrNumBuckets || e.bucket == bcmrUndo {
			return nil, errDeserialize("BCMR undo record names a bad bucket")
		}
		size, n := binary.Uvarint(b[1:])
		if n <= 0 || size > uint64(len(b)-1-n) {
			return nil, errDeserialize("BCMR undo record has a bad key")
		}
		b = b[1+n:]
		e.key, b = bytes.Clone(b[:size]), b[size:]
		if len(b) == 0 || b[0] > 1 {
			return nil, errDeserialize("BCMR undo record has a bad flag")
		}
		e.hadOld, b = b[0] == 1, b[1:]
		if e.hadOld {
			size, n := binary.Uvarint(b)
			if n <= 0 || size > uint64(len(b)-n) {
				return nil, errDeserialize("BCMR undo record has a bad value")
			}
			e.old, b = bytes.Clone(b[n:n+int(size)]), b[n+int(size):]
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// bcmrWriter writes the changes a block makes to the index in a database
// transaction, and records the value each entry had before, for the block's
// undo record.
type bcmrWriter struct {
	buckets [bcmrNumBuckets]database.Bucket
	touched map[string]struct{}
	undo    []bcmrUndoEntry
}

// newBcmrWriter returns a writer for the index's buckets in dbTx.
func newBcmrWriter(dbTx database.Tx) (*bcmrWriter, error) {
	w := &bcmrWriter{touched: make(map[string]struct{})}
	top := dbTx.Metadata().Bucket(bcmrIndexKey)
	if top == nil {
		return nil, fmt.Errorf("the %s has no bucket", bcmrIndexName)
	}
	w.buckets[bcmrTop] = top
	for b, name := range bcmrBucketNames {
		w.buckets[b] = top.Bucket(name)
		if w.buckets[b] == nil {
			return nil, fmt.Errorf("the %s has no %s bucket", bcmrIndexName, name)
		}
	}
	return w, nil
}

// get returns the value of key in bucket b, which is only valid during the
// database transaction.
func (w *bcmrWriter) get(b byte, key []byte) []byte {
	return w.buckets[b].Get(key)
}

// put sets the value of key in bucket b.
func (w *bcmrWriter) put(b byte, key, value []byte) error {
	w.record(b, key)
	return w.buckets[b].Put(key, value)
}

// delete removes key from bucket b.
func (w *bcmrWriter) delete(b byte, key []byte) error {
	w.record(b, key)
	return w.buckets[b].Delete(key)
}

// record adds the value key has in bucket b to the undo record, the first
// time the block changes it.
func (w *bcmrWriter) record(b byte, key []byte) {
	id := string(b) + string(key)
	if _, ok := w.touched[id]; ok {
		return
	}
	w.touched[id] = struct{}{}
	old := w.buckets[b].Get(key)
	w.undo = append(w.undo, bcmrUndoEntry{
		bucket: b,
		key:    bytes.Clone(key),
		hadOld: old != nil,
		old:    bytes.Clone(old),
	})
}

// newSegment returns the ID for a new segment.
func (w *bcmrWriter) newSegment() (uint64, error) {
	id := uint64(1)
	if raw := w.get(bcmrTop, bcmrNextSegmentKey); raw != nil {
		if len(raw) != 8 {
			return 0, errDeserialize("BCMR next segment has the wrong size")
		}
		id = byteOrder.Uint64(raw)
	}
	next := make([]byte, 8)
	byteOrder.PutUint64(next, id+1)
	return id, w.put(bcmrTop, bcmrNextSegmentKey, next)
}
