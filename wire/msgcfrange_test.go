// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire_test

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// cfTestLeaf returns a leaf of the given kind whose bytes start with b.
func cfTestLeaf(b byte, kind byte) *wire.CFLeaf {
	var leaf wire.CFLeaf
	for i := range 32 {
		leaf[i] = b
	}
	leaf[wire.CFLeafSize-1] = kind
	return &leaf
}

// cfTestHashes returns n distinct hashes.
func cfTestHashes(n int) []chainhash.Hash {
	hashes := make([]chainhash.Hash, n)
	for i := range hashes {
		hashes[i][0] = byte(i + 1)
	}
	return hashes
}

// cfRoundTrip encodes msg, checks its size when it reports one, decodes it
// into into, and checks that the two are equal.
func cfRoundTrip(t *testing.T, name string, msg, into wire.Message, size int) {
	t.Helper()
	pver := wire.ProtocolVersion
	var buf bytes.Buffer
	if err := msg.BchEncode(&buf, pver, wire.BaseEncoding); err != nil {
		t.Fatalf("%s: encode: %v", name, err)
	}
	if size >= 0 && buf.Len() != size {
		t.Errorf("%s: %d bytes, want %d", name, buf.Len(), size)
	}
	if err := into.BchDecode(bytes.NewReader(buf.Bytes()), pver, wire.BaseEncoding); err != nil {
		t.Fatalf("%s: decode: %v", name, err)
	}
	if !reflect.DeepEqual(msg, into) {
		t.Errorf("%s: decoded %+v, want %+v", name, into, msg)
	}
}

// cfEncodeError checks that encoding msg fails with a MessageError.
func cfEncodeError(t *testing.T, name string, msg wire.Message) {
	t.Helper()
	var merr *wire.MessageError
	err := msg.BchEncode(io.Discard, wire.ProtocolVersion, wire.BaseEncoding)
	if !errors.As(err, &merr) {
		t.Errorf("%s: encode gave %v, want a message error", name, err)
	}
}

// TestGetCFRangeWire round-trips getcfrange and refuses a prefix longer than
// a key both ways.
func TestGetCFRangeWire(t *testing.T) {
	var prefix [32]byte
	prefix[0] = 0x5c
	msg := wire.NewMsgGetCFRange(wire.GCSFilterScripts, &chainhash.Hash{7}, 8, prefix)
	cfRoundTrip(t, "getcfrange", msg, &wire.MsgGetCFRange{}, 1+32+2+32)
	if got := msg.MaxPayloadLength(wire.ProtocolVersion); got != 1+32+2+32 {
		t.Errorf("getcfrange limit %d", got)
	}

	msg.PrefixBits = 257
	cfEncodeError(t, "257 bits", msg)

	var buf bytes.Buffer
	msg.PrefixBits = 256
	_ = msg.BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding)
	b := buf.Bytes()
	b[33], b[34] = 0x01, 0x01 // 257, little-endian
	var read wire.MsgGetCFRange
	var merr *wire.MessageError
	if err := read.BchDecode(bytes.NewReader(b), wire.ProtocolVersion, wire.BaseEncoding); !errors.As(err, &merr) {
		t.Errorf("257 bits decoded: %v", err)
	}
}

// TestCFRangeWire round-trips cfrange with and without each neighbour,
// carries a neighbour exactly where the tree has one, and refuses leaves
// past the tree and too many hashes.
func TestCFRangeWire(t *testing.T) {
	leaves := func(n int) []wire.CFLeaf {
		out := make([]wire.CFLeaf, n)
		for i := range out {
			out[i] = *cfTestLeaf(byte(0x50+i), byte(i%4))
		}
		return out
	}
	tests := []struct {
		name          string
		count, first  uint32
		n             int
		before, after bool
		hashes        int
	}{
		{"run inside the tree", 10, 4, 3, true, true, 3},
		{"run at the start", 10, 0, 2, false, true, 4},
		{"no leaf, past the last", 10, 10, 0, true, false, 2},
		{"no leaf, between two", 10, 7, 0, true, true, 3},
		{"every leaf", 10, 0, 10, false, false, 0},
		{"empty tree", 0, 0, 0, false, false, 0},
	}
	for _, test := range tests {
		msg := &wire.MsgCFRange{
			FilterType: wire.GCSFilterScripts,
			BlockHash:  chainhash.Hash{9},
			LeafCount:  test.count,
			FirstIndex: test.first,
			Leaves:     leaves(test.n),
			Hashes:     cfTestHashes(test.hashes),
		}
		if test.before {
			msg.Before = cfTestLeaf(0x40, 1)
		}
		if test.after {
			msg.After = cfTestLeaf(0x60, 2)
		}
		size := msg.SerializeSize()
		want := 1 + 32 + 4 + 4 + 1 + wire.CFLeafSize*test.n + 1 + 32*test.hashes
		if test.before {
			want += wire.CFLeafSize
		}
		if test.after {
			want += wire.CFLeafSize
		}
		if size != want {
			t.Errorf("%s: size %d, want %d", test.name, size, want)
		}
		cfRoundTrip(t, test.name, msg, &wire.MsgCFRange{}, size)
	}

	base := func() *wire.MsgCFRange {
		return &wire.MsgCFRange{LeafCount: 10, FirstIndex: 4, Leaves: leaves(3),
			Before: cfTestLeaf(0x40, 0), After: cfTestLeaf(0x60, 0)}
	}
	noBefore := base()
	noBefore.Before = nil
	cfEncodeError(t, "before neighbour missing", noBefore)
	extraAfter := base()
	extraAfter.FirstIndex, extraAfter.Leaves = 7, leaves(3)
	cfEncodeError(t, "after neighbour past the last leaf", extraAfter)
	past := base()
	past.Leaves = leaves(7)
	cfEncodeError(t, "leaves past the tree", past)
	tooMany := base()
	tooMany.Hashes = cfTestHashes(2*wire.MaxCFTreeDepth + 1)
	cfEncodeError(t, "too many hashes", tooMany)

	// A reply cut before its after neighbour, and one with too many hashes,
	// are refused when read.
	var buf bytes.Buffer
	_ = base().BchEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding)
	cut := buf.Bytes()[:buf.Len()-1-wire.CFLeafSize+1]
	if err := new(wire.MsgCFRange).BchDecode(bytes.NewReader(cut), wire.ProtocolVersion,
		wire.BaseEncoding); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("cut reply: %v", err)
	}
	full := append(buf.Bytes()[:buf.Len()-1], 2*wire.MaxCFTreeDepth+1)
	var merr *wire.MessageError
	if err := new(wire.MsgCFRange).BchDecode(bytes.NewReader(full), wire.ProtocolVersion,
		wire.BaseEncoding); !errors.As(err, &merr) {
		t.Errorf("65 hashes: %v", err)
	}
}

// TestCFProofWire round-trips getcfproof and cfproof, with the spent output's
// transaction exactly for the leaves of spent outputs.
func TestCFProofWire(t *testing.T) {
	get := wire.NewMsgGetCFProof(wire.GCSFilterScripts, &chainhash.Hash{3}, 17)
	cfRoundTrip(t, "getcfproof", get, &wire.MsgGetCFProof{}, 1+32+4)

	tx := func(lockTime uint32) *wire.MsgTx {
		msgTx := wire.NewMsgTx(2)
		msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{1}, 0), []byte{0x51}))
		msgTx.AddTxOut(wire.NewTxOut(1000, []byte{0x51}, wire.TokenData{}))
		msgTx.LockTime = lockTime
		return msgTx
	}
	for kind := byte(0); kind < 4; kind++ {
		msg := &wire.MsgCFProof{
			FilterType: wire.GCSFilterScripts,
			BlockHash:  chainhash.Hash{4},
			LeafCount:  10,
			LeafIndex:  9,
			Leaf:       *cfTestLeaf(0x70, kind),
			LeafBranch: cfTestHashes(4),
			Tx:         tx(1),
			TxBranch:   cfTestHashes(2),
		}
		if msg.Leaf.Spent() != (kind == 1 || kind == 3) {
			t.Errorf("kind %d: spent %v", kind, msg.Leaf.Spent())
		}
		if msg.Leaf.Spent() {
			msg.PrevBlockHash = chainhash.Hash{5}
			msg.PrevTxIndex = 3
			msg.PrevTx = tx(2)
			msg.PrevTxBranch = cfTestHashes(3)
		}
		cfRoundTrip(t, "cfproof", msg, &wire.MsgCFProof{}, -1)

		// The spent output's transaction is there exactly for kinds 1
		// and 3.
		wrong := *msg
		if wrong.PrevTx == nil {
			wrong.PrevTx = tx(2)
		} else {
			wrong.PrevTx = nil
		}
		cfEncodeError(t, "spent output's transaction for the wrong kind", &wrong)
	}

	past := &wire.MsgCFProof{LeafCount: 10, LeafIndex: 10, Tx: tx(1)}
	cfEncodeError(t, "leaf past the tree", past)
	noTx := &wire.MsgCFProof{LeafCount: 10, LeafIndex: 1}
	cfEncodeError(t, "no transaction", noTx)
	long := &wire.MsgCFProof{LeafCount: 10, LeafIndex: 1, Tx: tx(1),
		LeafBranch: cfTestHashes(wire.MaxCFTreeDepth + 1)}
	cfEncodeError(t, "branch longer than a tree", long)
}

// TestCFRangeMessages reads each new message back through ReadMessage, so
// its command is known.
func TestCFRangeMessages(t *testing.T) {
	msgs := []wire.Message{
		wire.NewMsgGetCFRange(wire.GCSFilterScripts, &chainhash.Hash{1}, 0, [32]byte{}),
		&wire.MsgCFRange{LeafCount: 1, Leaves: []wire.CFLeaf{*cfTestLeaf(1, 0)}},
		wire.NewMsgGetCFProof(wire.GCSFilterScripts, &chainhash.Hash{1}, 0),
		&wire.MsgCFProof{LeafCount: 1, Tx: wire.NewMsgTx(1)},
	}
	for _, msg := range msgs {
		var buf bytes.Buffer
		if err := wire.WriteMessage(&buf, msg, wire.ProtocolVersion, wire.MainNet); err != nil {
			t.Fatalf("%s: %v", msg.Command(), err)
		}
		read, _, err := wire.ReadMessage(&buf, wire.ProtocolVersion, wire.MainNet)
		if err != nil {
			t.Fatalf("%s: %v", msg.Command(), err)
		}
		if read.Command() != msg.Command() {
			t.Errorf("read %s, want %s", read.Command(), msg.Command())
		}
	}
}
