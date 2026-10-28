// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/gcash/bchd/wire"
)

// paddedBlockMsg is a block message whose payload has extra data after the
// block, which decoding the block leaves unread.
type paddedBlockMsg struct {
	*wire.MsgBlock
	pad []byte
}

// BchEncode encodes the block followed by the extra data.
func (m paddedBlockMsg) BchEncode(w io.Writer, pver uint32, enc wire.MessageEncoding) error {
	if err := m.MsgBlock.BchEncode(w, pver, enc); err != nil {
		return err
	}
	_, err := w.Write(m.pad)
	return err
}

// TestPaddedBlockMessageStored ensures a block from a block message whose
// payload has data after the block is stored as its own serialization, as
// Bitcoin Cash Node decodes the block from the message and writes it to disk
// from its serialization (net_processing.cpp:3436-3437,
// node/blockstorage.cpp:345-368), so the data is not counted in its stored
// size.
func TestPaddedBlockMessageStored(t *testing.T) {
	h := newCmpctHarness(t)
	block := solvedBlock(t, h.s.chain, 0, nil)
	want, err := block.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	h.request(t, block)
	h.send(t, paddedBlockMsg{block.MsgBlock(), make([]byte, 1000)})

	deadline := time.Now().Add(5 * time.Second)
	for h.s.chain.BestSnapshot().Hash != *block.Hash() {
		if time.Now().After(deadline) {
			t.Fatalf("block %v was not connected", block.Hash())
		}
		time.Sleep(10 * time.Millisecond)
	}
	stored, err := h.s.chain.BlockByHash(block.Hash())
	if err != nil {
		t.Fatal(err)
	}
	got, err := stored.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("block stored as %d bytes, want its %d-byte serialization",
			len(got), len(want))
	}
}
