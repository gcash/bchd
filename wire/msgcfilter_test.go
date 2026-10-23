// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire_test

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil/gcs/builder"
)

// TestCFilterLimit ensures the cfilter limit is the size of the largest
// regular filter of a block a block message may hold, at least 4 MiB, and
// that encoding and decoding apply it.
func TestCFilterLimit(t *testing.T) {
	pver := wire.ProtocolVersion
	defer func() {
		wire.SetMaxBlockSizeLookAhead(0)
		wire.SetLimits(42111000)
	}()

	tests := []struct {
		excessiveBlockSize uint32 // Four times it is the block limit
		lookAhead          uint64 // Twice it is the block limit, if larger
		dataSize           int    // Largest filter
	}{
		{1000000, 0, 4194304},
		{5000000, 0, 5374290},
		{32000000, 0, 34395425},
		{32000000, 100000000, 53742848},
		{500000000, 0, 537428432},
	}
	for _, test := range tests {
		wire.SetLimits(test.excessiveBlockSize)
		wire.SetMaxBlockSizeLookAhead(test.lookAhead)
		want := uint32(wire.VarIntSerializeSize(uint64(test.dataSize))) +
			uint32(test.dataSize) + chainhash.HashSize + 1
		if got := (&wire.MsgCFilter{}).MaxPayloadLength(pver); got != want {
			t.Errorf("block limit %d: cfilter limit %d, want %d",
				wire.MaxBlockPayload(), got, want)
		}
	}

	// At 20 MB blocks a filter at the limit is written and read, and one a
	// byte larger is refused both ways.
	wire.SetLimits(5000000)
	wire.SetMaxBlockSizeLookAhead(0)
	const dataSize = 5374290
	msg := wire.NewMsgCFilter(wire.GCSFilterRegular, &chainhash.Hash{},
		make([]byte, dataSize))
	var buf bytes.Buffer
	if err := msg.BchEncode(&buf, pver, wire.BaseEncoding); err != nil {
		t.Fatalf("filter of %d bytes not encoded: %v", dataSize, err)
	}
	var read wire.MsgCFilter
	if err := read.BchDecode(&buf, pver, wire.BaseEncoding); err != nil {
		t.Fatalf("filter of %d bytes not decoded: %v", dataSize, err)
	}
	if len(read.Data) != dataSize {
		t.Fatalf("decoded %d bytes, want %d", len(read.Data), dataSize)
	}

	msg.Data = make([]byte, dataSize+1)
	if err := msg.BchEncode(&buf, pver, wire.BaseEncoding); err == nil {
		t.Fatalf("filter of %d bytes encoded", dataSize+1)
	}
	buf.Reset()
	buf.WriteByte(byte(wire.GCSFilterRegular))
	buf.Write(make([]byte, chainhash.HashSize))
	if err := wire.WriteVarBytes(&buf, pver, msg.Data); err != nil {
		t.Fatal(err)
	}
	if err := read.BchDecode(&buf, pver, wire.BaseEncoding); err == nil {
		t.Fatalf("filter of %d bytes decoded", dataSize+1)
	}
}

// TestCFilterOfLargeBlock ensures the cfilter of a valid block well within the
// block message limit can be written and read.  The block holds 1.6 million
// outputs with distinct 3-byte scripts, about 19 MB, whose filter is larger
// than the 4 MiB cfilter messages were limited to, so a server could not send
// it and a client could not read it.
func TestCFilterOfLargeBlock(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the filter of a 19 MB block")
	}
	pver := wire.ProtocolVersion
	wire.SetLimits(32000000)
	defer wire.SetLimits(42111000)

	const numOutputs = 1600000
	scripts := make([]byte, 3*numOutputs)
	outputs := make([]wire.TxOut, numOutputs)
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{0x04, 1, 0, 0, 0},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	cb.TxOut = make([]*wire.TxOut, numOutputs)
	for i := range outputs {
		script := scripts[3*i : 3*i+3 : 3*i+3]
		script[0], script[1], script[2] = byte(i>>16), byte(i>>8), byte(i)
		outputs[i].PkScript = script
		cb.TxOut[i] = &outputs[i]
	}
	block := wire.NewMsgBlock(&wire.BlockHeader{})
	if err := block.AddTransaction(cb); err != nil {
		t.Fatal(err)
	}
	if size := block.SerializeSize(); size > int(wire.MaxBlockPayload()) {
		t.Fatalf("block of %d bytes is over the block limit %d", size,
			wire.MaxBlockPayload())
	}

	filter, err := builder.BuildBasicFilter(block)
	if err != nil {
		t.Fatal(err)
	}
	data, err := filter.NBytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= 4*1024*1024 {
		t.Fatalf("filter of %d bytes is not over 4 MiB", len(data))
	}

	hash := block.BlockHash()
	msg := wire.NewMsgCFilter(wire.GCSFilterRegular, &hash, data)
	var buf bytes.Buffer
	n, err := wire.WriteMessageN(&buf, msg, pver, wire.MainNet)
	if err != nil {
		t.Fatalf("cfilter of %d bytes not written: %v", len(data), err)
	}
	payload := n - wire.MessageHeaderSize
	read, _, err := wire.ReadMessage(&buf, pver, wire.MainNet)
	if err != nil {
		t.Fatalf("cfilter of %d bytes not read: %v", len(data), err)
	}
	got, ok := read.(*wire.MsgCFilter)
	if !ok || got.BlockHash != hash || !bytes.Equal(got.Data, data) {
		t.Fatal("cfilter read back differently")
	}

	// The limit holds the filter at a block limit of the block's own size.
	wire.SetLimits(uint32(block.SerializeSize()+3) / 4)
	if mpl := msg.MaxPayloadLength(pver); uint64(payload) > uint64(mpl) {
		t.Fatalf("limit %d at a block limit of %d is below the filter's "+
			"payload of %d bytes", mpl, wire.MaxBlockPayload(), payload)
	}
}
