// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bcmr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// A hints file says where the authchain of every token stood at a height, so
// that a wallet that can only scan block filters starts there rather than at
// the token's genesis.  A wallet can't verify what comes before that height, so
// it trusts whoever gave it the file, and the file is built so that nobody has
// to take that on faith: the same height gives the same bytes on every node.
//
// The file is a header of the magic, the height, the number of entries and the
// number of publications, then the entries, sorted by the bytes of their
// categories for a binary search, then the publications.  An entry is
//
//	<category><head><head height><publication>
//
// with the two hashes in wire order, and 32-bit little-endian numbers.  The top
// bit of the head height marks a burned identity.  The publication is the
// number, from 1, of the entry's publication, for one that isn't on the head,
// or else 0.  A publication is <txid><height>.
const (
	hintsMagic      = "BCMRHNT1"
	hintsHeaderSize = len(hintsMagic) + 4 + 4 + 4
	hintEntrySize   = 2*chainhash.HashSize + 4 + 4
	hintPubSize     = chainhash.HashSize + 4
	hintBurnedBit   = uint32(1) << 31
)

// ErrBadHints is the error of a hints file that can't be read.
var ErrBadHints = errors.New("invalid BCMR hints file")

// HintPublication is a link with a publication, older than the head.
type HintPublication struct {
	Txid   chainhash.Hash
	Height int32
}

// Hint is where the authchain of a token stood at the height of the hints.
type Hint struct {
	// Category is the token's category ID.
	Category chainhash.Hash

	// Head and HeadHeight are the authhead and the height of its block.
	Head       chainhash.Hash
	HeadHeight int32

	// Burned reports that the head ended the authchain.
	Burned bool

	// Publication is the newest link with a publication when that isn't
	// the head.  It is nil when the wallet is to read the head's.
	Publication *HintPublication
}

// EncodeHints returns the hints file of the passed hints at the passed height.
// The order of the hints makes no difference.
func EncodeHints(height int32, hints []Hint) []byte {
	sorted := slices.Clone(hints)
	slices.SortFunc(sorted, func(a, b Hint) int {
		return bytes.Compare(a.Category[:], b.Category[:])
	})

	var entries, pubs []byte
	numPubs := uint32(0)
	for i := range sorted {
		hint := &sorted[i]
		pub := uint32(0)
		if hint.Publication != nil {
			numPubs++
			pub = numPubs
			pubs = append(pubs, hint.Publication.Txid[:]...)
			pubs = binary.LittleEndian.AppendUint32(pubs, uint32(hint.Publication.Height))
		}
		headHeight := uint32(hint.HeadHeight)
		if hint.Burned {
			headHeight |= hintBurnedBit
		}
		entries = append(entries, hint.Category[:]...)
		entries = append(entries, hint.Head[:]...)
		entries = binary.LittleEndian.AppendUint32(entries, headHeight)
		entries = binary.LittleEndian.AppendUint32(entries, pub)
	}

	data := make([]byte, 0, hintsHeaderSize+len(entries)+len(pubs))
	data = append(data, hintsMagic...)
	data = binary.LittleEndian.AppendUint32(data, uint32(height))
	data = binary.LittleEndian.AppendUint32(data, uint32(len(sorted)))
	data = binary.LittleEndian.AppendUint32(data, numPubs)
	data = append(data, entries...)
	return append(data, pubs...)
}

// Hints is a hints file that was read.
type Hints struct {
	// Height is the height the hints describe the chain at.
	Height int32

	numEntries, numPubs int
	entries, pubs       []byte
}

// ParseHints returns the hints of a hints file, which it keeps a reference to.
// It returns an error wrapping ErrBadHints for a file that is damaged.
func ParseHints(data []byte) (*Hints, error) {
	if len(data) < hintsHeaderSize || string(data[:len(hintsMagic)]) != hintsMagic {
		return nil, fmt.Errorf("%w: no header", ErrBadHints)
	}
	header := data[len(hintsMagic):]
	height := binary.LittleEndian.Uint32(header[0:4])
	numEntries := uint64(binary.LittleEndian.Uint32(header[4:8]))
	numPubs := uint64(binary.LittleEndian.Uint32(header[8:12]))
	size := uint64(hintsHeaderSize) + numEntries*uint64(hintEntrySize) +
		numPubs*uint64(hintPubSize)
	if height >= hintsHeightLimit || uint64(len(data)) != size {
		return nil, fmt.Errorf("%w: wrong size", ErrBadHints)
	}

	body := data[hintsHeaderSize:]
	entriesEnd := int(numEntries) * hintEntrySize
	h := &Hints{
		Height:     int32(height),
		numEntries: int(numEntries),
		numPubs:    int(numPubs),
		entries:    body[:entriesEnd],
		pubs:       body[entriesEnd:],
	}
	for i := 0; i < h.numEntries; i++ {
		entry := h.entry(i)
		if i > 0 && bytes.Compare(h.entry(i - 1)[:chainhash.HashSize],
			entry[:chainhash.HashSize]) >= 0 {

			return nil, fmt.Errorf("%w: entries out of order", ErrBadHints)
		}
		pub := binary.LittleEndian.Uint32(entry[hintEntrySize-4:])
		if uint64(pub) > numPubs {
			return nil, fmt.Errorf("%w: no publication %d", ErrBadHints, pub)
		}
	}
	return h, nil
}

// hintsHeightLimit is the first height too high for a block.
const hintsHeightLimit = uint32(1) << 31

// entry returns the bytes of entry i.
func (h *Hints) entry(i int) []byte {
	return h.entries[i*hintEntrySize : (i+1)*hintEntrySize]
}

// Len returns the number of tokens the hints have.
func (h *Hints) Len() int {
	return h.numEntries
}

// Lookup returns the hint of a token's category, or false if the hints don't
// have it, which means it was minted after their height.
func (h *Hints) Lookup(category chainhash.Hash) (Hint, bool) {
	i := sort.Search(h.numEntries, func(i int) bool {
		return bytes.Compare(h.entry(i)[:chainhash.HashSize], category[:]) >= 0
	})
	if i == h.numEntries || !bytes.Equal(h.entry(i)[:chainhash.HashSize], category[:]) {
		return Hint{}, false
	}

	entry := h.entry(i)
	headHeight := binary.LittleEndian.Uint32(entry[2*chainhash.HashSize:])
	hint := Hint{
		Category:   category,
		HeadHeight: int32(headHeight &^ hintBurnedBit),
		Burned:     headHeight&hintBurnedBit != 0,
	}
	copy(hint.Head[:], entry[chainhash.HashSize:])
	if pub := binary.LittleEndian.Uint32(entry[hintEntrySize-4:]); pub != 0 {
		raw := h.pubs[int(pub-1)*hintPubSize:]
		hint.Publication = &HintPublication{
			Height: int32(binary.LittleEndian.Uint32(raw[chainhash.HashSize:])),
		}
		copy(hint.Publication.Txid[:], raw)
	}
	return hint, true
}
