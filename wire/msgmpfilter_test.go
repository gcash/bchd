// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

// TestMpFilterWire round-trips the mempool filter messages through
// ReadMessage, with a transaction that is still in the mempool and one that
// left it, and checks the interval hash and the limits.
func TestMpFilterWire(t *testing.T) {
	pver := wire.ProtocolVersion
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{1}, 0), []byte{0x51}))
	tx.AddTxOut(wire.NewTxOut(1000, []byte{0x51}, wire.TokenData{}))
	gone := chainhash.Hash{9}

	msgs := []wire.Message{
		wire.NewMsgSendMpFilter(wire.GCSFilterScripts),
		wire.NewMsgGetMpFilters(wire.GCSFilterScripts, 1794461717),
		&wire.MsgMpFilter{FilterType: wire.GCSFilterScripts, Seq: 7, Time: 1794461717,
			IntervalHash: chainhash.Hash{3}, TxCount: 2, Data: []byte{0x02, 0xaa, 0xbb}},
		wire.NewMsgGetMpTxs(wire.GCSFilterScripts, 7, 9),
		&wire.MsgMpTxs{FilterType: wire.GCSFilterScripts, Seq: 7,
			Txs: []wire.MpTx{{Tx: tx, TxID: tx.TxHash()}, {TxID: gone}}},
	}
	sizes := []int{1, 5, 1 + 4 + 8 + 32 + 4 + 1 + 3, 9, 1 + 4 + 1 + 1 + tx.SerializeSize() + 1 + 32}
	for i, msg := range msgs {
		var buf bytes.Buffer
		if err := wire.WriteMessage(&buf, msg, pver, wire.MainNet); err != nil {
			t.Fatalf("%s: %v", msg.Command(), err)
		}
		if got := buf.Len() - wire.MessageHeaderSize; got != sizes[i] {
			t.Errorf("%s: %d bytes, want %d", msg.Command(), got, sizes[i])
		}
		read, _, err := wire.ReadMessage(&buf, pver, wire.MainNet)
		if err != nil {
			t.Fatalf("%s: %v", msg.Command(), err)
		}
		if !reflect.DeepEqual(read, msg) {
			t.Errorf("%s: read %+v, want %+v", msg.Command(), read, msg)
		}
	}

	// The interval hash is the double SHA-256 of the transaction IDs in
	// order.
	ids := []chainhash.Hash{tx.TxHash(), gone}
	want := chainhash.DoubleHashH(append(append([]byte(nil), ids[0][:]...), ids[1][:]...))
	if got := wire.MpIntervalHash(ids); got != want {
		t.Errorf("interval hash %v, want %v", got, want)
	}
	if wire.MpIntervalHash(ids) == wire.MpIntervalHash([]chainhash.Hash{gone, tx.TxHash()}) {
		t.Error("the interval hash does not depend on the order")
	}

	// More transactions than a message holds are refused before any is
	// read.
	var merr *wire.MessageError
	payload := []byte{1, 7, 0, 0, 0, 0xfe, 0xff, 0xff, 0xff, 0xff}
	if err := new(wire.MsgMpTxs).BchDecode(bytes.NewReader(payload), pver, wire.BaseEncoding); !errors.As(err, &merr) {
		t.Errorf("too many transactions: %v", err)
	}
}
