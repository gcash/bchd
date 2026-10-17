// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestVectorCountBoundedByPayload ensures vectors and strings that claim more
// elements than their payload holds fail with the same error as before,
// whether or not the reader reports its length, and that an in-memory payload
// only allocates for the elements it can hold.
func TestVectorCountBoundedByPayload(t *testing.T) {
	tests := []struct {
		name   string
		max    uint64
		size   int
		prefix []byte
		msg    func() Message
	}{
		{"inv", MaxInvPerMsg, maxInvVectPayload, nil,
			func() Message { return &MsgInv{} }},
		{"getdata", MaxInvPerMsg, maxInvVectPayload, nil,
			func() Message { return &MsgGetData{} }},
		{"notfound", MaxInvPerMsg, maxInvVectPayload, nil,
			func() Message { return &MsgNotFound{} }},
		{"headers", MaxBlockHeadersPerMsg, blockHeaderLen + 1, nil,
			func() Message { return &MsgHeaders{} }},
		{"getheaders", MaxBlockLocatorsPerMsg, chainhash.HashSize,
			make([]byte, 4),
			func() Message { return &MsgGetHeaders{} }},
		{"getblocks", MaxBlockLocatorsPerMsg, chainhash.HashSize,
			make([]byte, 4),
			func() Message { return &MsgGetBlocks{} }},
		{"addr", MaxAddrPerMsg,
			int(maxNetAddressPayload(ProtocolVersion)), nil,
			func() Message { return &MsgAddr{} }},
		{"cfheaders", MaxCFHeadersPerMsg, chainhash.HashSize,
			make([]byte, 1+2*chainhash.HashSize),
			func() Message { return &MsgCFHeaders{} }},
		{"cfcheckpt", maxCFHeadersLen, chainhash.HashSize,
			make([]byte, 1+chainhash.HashSize),
			func() Message { return &MsgCFCheckpt{} }},
		{"reject command", 1 << 20, 1, nil,
			func() Message { return &MsgReject{} }},
		{"reject reason", 1 << 20, 1, []byte{0x00, byte(RejectInvalid)},
			func() Message { return &MsgReject{} }},
	}

	for _, test := range tests {
		var count bytes.Buffer
		_ = WriteVarInt(&count, 0, test.max)
		for _, elems := range []int{0, 1, 3} {
			for _, extra := range []int{0, 1, 4, test.size - 1} {
				name := fmt.Sprintf("%s/%d+%d", test.name, elems,
					extra)
				payload := bytes.Join([][]byte{test.prefix,
					count.Bytes(),
					make([]byte, elems*test.size+extra)}, nil)

				hidden := struct{ io.Reader }{bytes.NewReader(payload)}
				wantErr := test.msg().BchDecode(hidden,
					ProtocolVersion, BaseEncoding)

				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				err := test.msg().BchDecode(bytes.NewBuffer(payload),
					ProtocolVersion, BaseEncoding)
				runtime.ReadMemStats(&after)

				if err == nil || fmt.Sprint(err) != fmt.Sprint(wantErr) {
					t.Fatalf("%s: got error %v, want %v", name,
						err, wantErr)
				}
				if n := after.TotalAlloc - before.TotalAlloc; n > 4096 {
					t.Fatalf("%s: allocated %d bytes", name, n)
				}
			}
		}
	}
}

// TestVectorCountRoundTrip ensures messages with the largest allowed vectors,
// and reject reasons longer than any length limit, read back to the same
// message and encode to the same bytes.
func TestVectorCountRoundTrip(t *testing.T) {
	hash := func(i int) chainhash.Hash {
		return chainhash.Hash{byte(i), byte(i >> 8), 0xa5}
	}

	inv := NewMsgInv()
	getData := NewMsgGetData()
	notFound := NewMsgNotFound()
	for i := range MaxInvPerMsg {
		h := hash(i)
		iv := NewInvVect(InvType(i%5), &h)
		_ = inv.AddInvVect(iv)
		_ = getData.AddInvVect(iv)
		_ = notFound.AddInvVect(iv)
	}
	headers := NewMsgHeaders()
	for i := range MaxBlockHeadersPerMsg {
		prev, merkle := hash(i), hash(i+1)
		bh := NewBlockHeader(int32(i), &prev, &merkle, uint32(i),
			uint32(i)*7)
		bh.Timestamp = time.Unix(int64(i)+1231006505, 0)
		_ = headers.AddBlockHeader(bh)
	}
	getHeaders := NewMsgGetHeaders()
	getBlocks := NewMsgGetBlocks(&chainhash.Hash{0x01})
	for i := range MaxBlockLocatorsPerMsg {
		h := hash(i)
		_ = getHeaders.AddBlockLocatorHash(&h)
		_ = getBlocks.AddBlockLocatorHash(&h)
	}
	addr := NewMsgAddr()
	for i := range MaxAddrPerMsg {
		ip := net.IP{0x20, 0x01, 0x0d, 0xb8, 14: byte(i >> 8),
			15: byte(i)}
		na := NewNetAddressIPPort(ip, uint16(i), SFNodeNetwork)
		na.Timestamp = time.Unix(int64(i)+1231006505, 0)
		_ = addr.AddAddress(na)
	}
	cfHeaders := NewMsgCFHeaders()
	cfHeaders.StopHash = hash(1)
	cfHeaders.PrevFilterHeader = hash(2)
	for i := range MaxCFHeadersPerMsg {
		h := hash(i)
		_ = cfHeaders.AddCFHash(&h)
	}
	stop := hash(3)
	cfCheckpt := NewMsgCFCheckpt(GCSFilterRegular, &stop, maxCFHeadersLen)
	for i := range maxCFHeadersLen {
		h := hash(i)
		_ = cfCheckpt.AddCFHeader(&h)
	}
	reason := strings.Repeat("r", 100*1024)
	reject := NewMsgReject(CmdTx, RejectNonstandard, reason)
	reject.Hash = hash(1)

	for _, msg := range []Message{inv, getData, notFound, headers,
		getHeaders, getBlocks, addr, cfHeaders, cfCheckpt, reject} {

		var buf bytes.Buffer
		if err := WriteMessage(&buf, msg, ProtocolVersion,
			MainNet); err != nil {

			t.Fatalf("%s: WriteMessage: %v", msg.Command(), err)
		}
		encoded := bytes.Clone(buf.Bytes())

		decoded, _, err := ReadMessage(&buf, ProtocolVersion, MainNet)
		if err != nil {
			t.Fatalf("%s: ReadMessage: %v", msg.Command(), err)
		}
		if !reflect.DeepEqual(decoded, msg) {
			t.Fatalf("%s: decoded message does not match",
				msg.Command())
		}

		var reencoded bytes.Buffer
		if err := WriteMessage(&reencoded, decoded, ProtocolVersion,
			MainNet); err != nil {

			t.Fatalf("%s: WriteMessage: %v", msg.Command(), err)
		}
		if !bytes.Equal(reencoded.Bytes(), encoded) {
			t.Fatalf("%s: reencoded message does not match",
				msg.Command())
		}
	}
}
