// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

// cmpctBlockPrefix returns the encoding of a compact block header and nonce.
func cmpctBlockPrefix(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	if err := writeBlockHeader(&buf, ProtocolVersion, &blockOne.Header); err != nil {
		t.Fatal(err)
	}
	if err := writeElement(&buf, uint64(1)); err != nil {
		t.Fatal(err)
	}
	return &buf
}

// TestCmpctBlockDecodeBounds ensures compact block decoding rejects counts
// beyond the transactions a block can hold and prefilled indexes that don't
// fit in 32 bits instead of truncating them.
func TestCmpctBlockDecodeBounds(t *testing.T) {
	maxTxs := uint64(maxTxPerBlock())

	tooManyShortIDs := cmpctBlockPrefix(t)
	_ = WriteVarInt(tooManyShortIDs, 0, maxTxs+1)

	tooManyTxs := cmpctBlockPrefix(t)
	_ = WriteVarInt(tooManyTxs, 0, 1)
	tooManyTxs.Write(make([]byte, ShortIDSize))
	_ = WriteVarInt(tooManyTxs, 0, maxTxs)

	wideIndex := cmpctBlockPrefix(t)
	_ = WriteVarInt(wideIndex, 0, 0)
	_ = WriteVarInt(wideIndex, 0, 1)
	_ = WriteVarInt(wideIndex, 0, math.MaxUint32+1)
	if err := blockOne.Transactions[0].BchEncode(wideIndex, ProtocolVersion,
		BaseEncoding); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		buf  *bytes.Buffer
	}{
		{"too many short ids", tooManyShortIDs},
		{"too many transactions", tooManyTxs},
		{"prefilled index wider than 32 bits", wideIndex},
	}
	for _, test := range tests {
		var msg MsgCmpctBlock
		err := msg.BchDecode(test.buf, ProtocolVersion, BaseEncoding)
		var merr *MessageError
		if !errors.As(err, &merr) {
			t.Errorf("%s: got error %v, want a MessageError", test.name,
				err)
		}
	}
}

// TestBlockTxnsDecodeBounds ensures blocktxn decoding rejects a transaction
// count beyond the transactions a block can hold.
func TestBlockTxnsDecodeBounds(t *testing.T) {
	var buf bytes.Buffer
	_ = writeElement(&buf, blockOne.Header.PrevBlock)
	_ = WriteVarInt(&buf, 0, uint64(maxTxPerBlock())+1)

	var msg MsgBlockTxns
	err := msg.BchDecode(&buf, ProtocolVersion, BaseEncoding)
	var merr *MessageError
	if !errors.As(err, &merr) {
		t.Errorf("got error %v, want a MessageError", err)
	}
}
