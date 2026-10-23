// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"net"
	"reflect"
	"runtime"
	"testing"
	"testing/iotest"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// makeHeader is a convenience function to make a message header in the form of
// a byte slice.  It is used to force errors when reading messages.
func makeHeader(bchnet BitcoinNet, command string,
	payloadLen uint32, checksum uint32) []byte {

	// The length of a bitcoin message header is 24 bytes.
	// 4 byte magic number of the bitcoin network + 12 byte command + 4 byte
	// payload length + 4 byte checksum.
	buf := make([]byte, 24)
	binary.LittleEndian.PutUint32(buf, uint32(bchnet))
	copy(buf[4:], []byte(command))
	binary.LittleEndian.PutUint32(buf[16:], payloadLen)
	binary.LittleEndian.PutUint32(buf[20:], checksum)
	return buf
}

// TestMessage tests the Read/WriteMessage and Read/WriteMessageN API.
func TestMessage(t *testing.T) {
	pver := ProtocolVersion

	// Create the various types of messages to test.

	// MsgVersion.
	addrYou := &net.TCPAddr{IP: net.ParseIP("192.168.0.1"), Port: 8333}
	you := NewNetAddress(addrYou, SFNodeNetwork)
	you.Timestamp = time.Time{} // Version message has zero value timestamp.
	addrMe := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8333}
	me := NewNetAddress(addrMe, SFNodeNetwork)
	me.Timestamp = time.Time{} // Version message has zero value timestamp.
	msgVersion := NewMsgVersion(me, you, 123123, 0)

	msgVerack := NewMsgVerAck()
	msgGetAddr := NewMsgGetAddr()
	msgAddr := NewMsgAddr()
	msgGetBlocks := NewMsgGetBlocks(&chainhash.Hash{})
	msgBlock := &blockOne
	msgInv := NewMsgInv()
	msgGetData := NewMsgGetData()
	msgNotFound := NewMsgNotFound()
	msgTx := NewMsgTx(1)
	msgPing := NewMsgPing(123123)
	msgPong := NewMsgPong(123123)
	msgGetHeaders := NewMsgGetHeaders()
	msgHeaders := NewMsgHeaders()
	msgMemPool := NewMsgMemPool()
	msgFilterAdd := NewMsgFilterAdd([]byte{0x01})
	msgFilterClear := NewMsgFilterClear()
	msgFilterLoad := NewMsgFilterLoad([]byte{0x01}, 10, 0, BloomUpdateNone)
	bh := NewBlockHeader(1, &chainhash.Hash{}, &chainhash.Hash{}, 0, 0)
	msgMerkleBlock := NewMsgMerkleBlock(bh)
	msgReject := NewMsgReject("block", RejectDuplicate, "duplicate block")
	msgGetCFilters := NewMsgGetCFilters(GCSFilterRegular, 0, &chainhash.Hash{})
	msgGetCFHeaders := NewMsgGetCFHeaders(GCSFilterRegular, 0, &chainhash.Hash{})
	msgGetCFCheckpt := NewMsgGetCFCheckpt(GCSFilterRegular, &chainhash.Hash{})
	msgCFilter := NewMsgCFilter(GCSFilterRegular, &chainhash.Hash{},
		[]byte("payload"))
	msgCFHeaders := NewMsgCFHeaders()
	msgCFCheckpt := NewMsgCFCheckpt(GCSFilterRegular, &chainhash.Hash{}, 0)
	msgSendAddrV2 := NewMsgSendAddrV2()
	msgAddrV2 := NewMsgAddrV2()
	msgDSProof := NewMsgDSProof(OutPoint{}, DSProofSpender{PushData: [][]byte{}},
		DSProofSpender{PushData: [][]byte{}})

	tests := []struct {
		in     Message    // Value to encode
		out    Message    // Expected decoded value
		pver   uint32     // Protocol version for wire encoding
		bchnet BitcoinNet // Network to use for wire encoding
		bytes  int        // Expected num bytes read/written
	}{
		{msgVersion, msgVersion, pver, MainNet, 125},
		{msgVerack, msgVerack, pver, MainNet, 24},
		{msgGetAddr, msgGetAddr, pver, MainNet, 24},
		{msgAddr, msgAddr, pver, MainNet, 25},
		{msgGetBlocks, msgGetBlocks, pver, MainNet, 61},
		{msgBlock, msgBlock, pver, MainNet, 239},
		{msgInv, msgInv, pver, MainNet, 25},
		{msgGetData, msgGetData, pver, MainNet, 25},
		{msgNotFound, msgNotFound, pver, MainNet, 25},
		{msgTx, msgTx, pver, MainNet, 34},
		{msgPing, msgPing, pver, MainNet, 32},
		{msgPong, msgPong, pver, MainNet, 32},
		{msgGetHeaders, msgGetHeaders, pver, MainNet, 61},
		{msgHeaders, msgHeaders, pver, MainNet, 25},
		{msgMemPool, msgMemPool, pver, MainNet, 24},
		{msgFilterAdd, msgFilterAdd, pver, MainNet, 26},
		{msgFilterClear, msgFilterClear, pver, MainNet, 24},
		{msgFilterLoad, msgFilterLoad, pver, MainNet, 35},
		{msgMerkleBlock, msgMerkleBlock, pver, MainNet, 110},
		{msgReject, msgReject, pver, MainNet, 79},
		{msgGetCFilters, msgGetCFilters, pver, MainNet, 61},
		{msgGetCFHeaders, msgGetCFHeaders, pver, MainNet, 61},
		{msgGetCFCheckpt, msgGetCFCheckpt, pver, MainNet, 57},
		{msgCFilter, msgCFilter, pver, MainNet, 65},
		{msgCFHeaders, msgCFHeaders, pver, MainNet, 90},
		{msgCFCheckpt, msgCFCheckpt, pver, MainNet, 58},
		{msgSendAddrV2, msgSendAddrV2, pver, MainNet, 24},
		{msgAddrV2, msgAddrV2, pver, MainNet, 25},
		{msgDSProof, msgDSProof, pver, MainNet, 278},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to wire format.
		var buf bytes.Buffer
		nw, err := WriteMessageN(&buf, test.in, test.pver, test.bchnet)
		if err != nil {
			t.Errorf("WriteMessage #%d error %v", i, err)
			continue
		}

		// Ensure the number of bytes written match the expected value.
		if nw != test.bytes {
			t.Errorf("WriteMessage #%d unexpected num bytes "+
				"written - got %d, want %d", i, nw, test.bytes)
		}

		// Decode from wire format.
		rbuf := bytes.NewReader(buf.Bytes())
		nr, msg, _, err := ReadMessageN(rbuf, test.pver, test.bchnet)
		if err != nil {
			t.Errorf("ReadMessage #%d error %v, msg %v", i, err,
				spew.Sdump(msg))
			continue
		}
		if !reflect.DeepEqual(msg, test.out) {
			t.Errorf("ReadMessage #%d\n got: %v want: %v", i,
				spew.Sdump(msg), spew.Sdump(test.out))
			continue
		}

		// Ensure the number of bytes read match the expected value.
		if nr != test.bytes {
			t.Errorf("ReadMessage #%d unexpected num bytes read - "+
				"got %d, want %d", i, nr, test.bytes)
		}
	}

	// Do the same thing for Read/WriteMessage, but ignore the bytes since
	// they don't return them.
	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to wire format.
		var buf bytes.Buffer
		err := WriteMessage(&buf, test.in, test.pver, test.bchnet)
		if err != nil {
			t.Errorf("WriteMessage #%d error %v", i, err)
			continue
		}

		// Decode from wire format.
		rbuf := bytes.NewReader(buf.Bytes())
		msg, _, err := ReadMessage(rbuf, test.pver, test.bchnet)
		if err != nil {
			t.Errorf("ReadMessage #%d error %v, msg %v", i, err,
				spew.Sdump(msg))
			continue
		}
		if !reflect.DeepEqual(msg, test.out) {
			t.Errorf("ReadMessage #%d\n got: %v want: %v", i,
				spew.Sdump(msg), spew.Sdump(test.out))
			continue
		}
	}
}

// TestReadMessageWireErrors performs negative tests against wire decoding into
// concrete messages to confirm error paths work correctly.
func TestReadMessageWireErrors(t *testing.T) {
	pver := ProtocolVersion
	bchnet := MainNet

	// Ensure message errors are as expected with no function specified.
	wantErr := "something bad happened"
	testErr := MessageError{Description: wantErr}
	if testErr.Error() != wantErr {
		t.Errorf("MessageError: wrong error - got %v, want %v",
			testErr.Error(), wantErr)
	}

	// Ensure message errors are as expected with a function specified.
	wantFunc := "foo"
	testErr = MessageError{Func: wantFunc, Description: wantErr}
	if testErr.Error() != wantFunc+": "+wantErr {
		t.Errorf("MessageError: wrong error - got %v, want %v",
			testErr.Error(), wantErr)
	}

	// Wire encoded bytes for main and testnet3 networks magic identifiers.
	testNet3Bytes := makeHeader(TestNet3, "", 0, 0)

	// Wire encoded bytes for a message that exceeds max overall message
	// length.
	mpl := maxMessagePayload()
	exceedMaxPayloadBytes := makeHeader(bchnet, "getaddr", mpl+1, 0)

	// Wire encoded bytes for a command which is invalid utf-8.
	badCommandBytes := makeHeader(bchnet, "bogus", 0, 0)
	badCommandBytes[4] = 0x81

	// Wire encoded bytes for a command which is valid, but not supported.
	unsupportedCommandBytes := makeHeader(bchnet, "bogus", 0, 0)

	// Wire encoded bytes for a message which exceeds the max payload for
	// a specific message type.
	exceedTypePayloadBytes := makeHeader(bchnet, "getaddr", 1, 0)

	// Wire encoded bytes for a message which does not deliver the full
	// payload according to the header length.
	shortPayloadBytes := makeHeader(bchnet, "version", 115, 0)

	// Wire encoded bytes for a message with a bad checksum.
	badChecksumBytes := makeHeader(bchnet, "version", 2, 0xbeef)
	badChecksumBytes = append(badChecksumBytes, []byte{0x0, 0x0}...)

	// Wire encoded bytes for a message which has a valid header, but is
	// the wrong format.  An addr starts with a varint of the number of
	// contained in the message.  Claim there is two, but don't provide
	// them.  At the same time, forge the header fields so the message is
	// otherwise accurate.
	badMessageBytes := makeHeader(bchnet, "addr", 1, 0xeaadc31c)
	badMessageBytes = append(badMessageBytes, 0x2)

	// Wire encoded bytes for a message which the header claims has 15k
	// bytes of data to discard.
	discardBytes := makeHeader(bchnet, "bogus", 15*1024, 0)

	tests := []struct {
		buf     []byte     // Wire encoding
		pver    uint32     // Protocol version for wire encoding
		bchnet  BitcoinNet // Bitcoin network for wire encoding
		max     int        // Max size of fixed buffer to induce errors
		readErr error      // Expected read error
		bytes   int        // Expected num bytes read
	}{
		// Latest protocol version with intentional read errors.

		// Short header.
		{
			[]byte{},
			pver,
			bchnet,
			0,
			io.EOF,
			0,
		},

		// Wrong network.  Want MainNet, but giving TestNet3.
		{
			testNet3Bytes,
			pver,
			bchnet,
			len(testNet3Bytes),
			&MessageError{},
			24,
		},

		// Exceed max overall message payload length.
		{
			exceedMaxPayloadBytes,
			pver,
			bchnet,
			len(exceedMaxPayloadBytes),
			&MessageError{},
			24,
		},

		// Invalid UTF-8 command.
		{
			badCommandBytes,
			pver,
			bchnet,
			len(badCommandBytes),
			&MessageError{},
			24,
		},

		// Valid, but unsupported command.
		{
			unsupportedCommandBytes,
			pver,
			bchnet,
			len(unsupportedCommandBytes),
			&MessageError{},
			24,
		},

		// Exceed max allowed payload for a message of a specific type.
		{
			exceedTypePayloadBytes,
			pver,
			bchnet,
			len(exceedTypePayloadBytes),
			&MessageError{},
			24,
		},

		// Message with a payload shorter than the header indicates.
		{
			shortPayloadBytes,
			pver,
			bchnet,
			len(shortPayloadBytes),
			io.EOF,
			24,
		},

		// Message with a bad checksum.
		{
			badChecksumBytes,
			pver,
			bchnet,
			len(badChecksumBytes),
			&MessageError{},
			26,
		},

		// Message with a valid header, but wrong format.
		{
			badMessageBytes,
			pver,
			bchnet,
			len(badMessageBytes),
			io.EOF,
			25,
		},

		// 15k bytes of data to discard.
		{
			discardBytes,
			pver,
			bchnet,
			len(discardBytes),
			&MessageError{},
			24,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Decode from wire format.
		r := newFixedReader(test.max, test.buf)
		nr, _, _, err := ReadMessageN(r, test.pver, test.bchnet)
		if reflect.TypeOf(err) != reflect.TypeOf(test.readErr) {
			t.Errorf("ReadMessage #%d wrong error got: %v <%T>, "+
				"want: %T", i, err, err, test.readErr)
			continue
		}

		// Ensure the number of bytes written match the expected value.
		if nr != test.bytes {
			t.Errorf("ReadMessage #%d unexpected num bytes read - "+
				"got %d, want %d", i, nr, test.bytes)
		}

		// For errors which are not of type MessageError, check them for
		// equality.
		if _, ok := err.(*MessageError); !ok {
			if err != test.readErr {
				t.Errorf("ReadMessage #%d wrong error got: %v <%T>, "+
					"want: %v <%T>", i, err, err,
					test.readErr, test.readErr)
				continue
			}
		}
	}
}

// TestReadMessageShortPayloadAlloc ensures that reading a message whose header
// claims a large payload, but which only delivers a few bytes of it, fails
// with the expected error and does not allocate memory for the full claimed
// length.
func TestReadMessageShortPayloadAlloc(t *testing.T) {
	pver := ProtocolVersion
	bchnet := MainNet

	// The cmpctblock message accepts payloads larger than a block, so a
	// header claiming its max payload passes the length checks and the
	// payload read is attempted.
	payloadLen := (&MsgCmpctBlock{}).MaxPayloadLength(pver)
	if payloadLen <= MaxBlockPayload() {
		t.Fatalf("test payload length %d is not above the block "+
			"limit %d", payloadLen, MaxBlockPayload())
	}

	tests := []struct {
		name    string // Name of the test
		sent    int    // Num payload bytes delivered before EOF
		readErr error  // Expected read error
	}{
		{"no payload", 0, io.EOF},
		{"a few bytes", 5, io.ErrUnexpectedEOF},
		{"64 KiB", 64 * 1024, io.ErrUnexpectedEOF},
	}

	for _, test := range tests {
		buf := makeHeader(bchnet, CmdCmpctBlock, payloadLen, 0)
		buf = append(buf, make([]byte, test.sent)...)
		r := bytes.NewReader(buf)

		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		nr, msg, payload, err := ReadMessageN(r, pver, bchnet)
		runtime.ReadMemStats(&after)

		if err != test.readErr {
			t.Errorf("%s: wrong error - got %v <%T>, want %v <%T>",
				test.name, err, err, test.readErr, test.readErr)
		}
		if msg != nil || payload != nil {
			t.Errorf("%s: unexpected message or payload returned",
				test.name)
		}
		if want := MessageHeaderSize + test.sent; nr != want {
			t.Errorf("%s: unexpected num bytes read - got %d, "+
				"want %d", test.name, nr, want)
		}

		// The claimed length is over a hundred MiB, while the bytes that
		// actually arrive are at most 64 KiB.  Reading them should only
		// take a small multiple of that, so 1 MiB leaves plenty of room
		// for incidental allocations while still being far below the
		// claimed length.
		const maxAlloc = 1024 * 1024
		alloc := after.TotalAlloc - before.TotalAlloc
		if alloc > maxAlloc {
			t.Errorf("%s: allocated %d bytes for a header claiming "+
				"%d bytes with %d delivered, want at most %d",
				test.name, alloc, payloadLen, test.sent,
				maxAlloc)
		}
	}
}

// TestCFilterClaimedSizeAlloc ensures a cfilter whose filter claims the
// largest size, while its payload ends sooner, is refused without allocating
// the claimed size.  The cfilter limit grows with the block limit, to 34 MB
// at the default block size, so a message of a few dozen bytes could
// otherwise make the node allocate that much.
func TestCFilterClaimedSizeAlloc(t *testing.T) {
	pver := ProtocolVersion
	SetLimits(32000000)
	t.Cleanup(func() { SetLimits(fixedExcessiveBlockSize) })

	claimed := maxCFilterDataSize()
	tests := []struct {
		sent int   // Filter bytes in the payload
		err  error // Expected decode error
	}{
		{0, io.EOF},
		{5, io.ErrUnexpectedEOF},
	}
	for _, test := range tests {
		var payload bytes.Buffer
		payload.WriteByte(byte(GCSFilterRegular))
		payload.Write(make([]byte, chainhash.HashSize))
		_ = WriteVarInt(&payload, pver, uint64(claimed))
		payload.Write(make([]byte, test.sent))

		var msg MsgCFilter
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		err := msg.BchDecode(&payload, pver, BaseEncoding)
		runtime.ReadMemStats(&after)
		if err != test.err {
			t.Errorf("%d bytes sent: got %v, want %v", test.sent, err,
				test.err)
		}
		const maxAlloc = 1024 * 1024
		if alloc := after.TotalAlloc - before.TotalAlloc; alloc > maxAlloc {
			t.Errorf("%d bytes sent: allocated %d bytes for a filter "+
				"claiming %d, want at most %d", test.sent, alloc,
				claimed, maxAlloc)
		}
	}
}

// TestReadMessageLargePayload ensures that large messages, whose payloads
// span many reads and several increases of the payload buffer, still read
// back identically to what was written.
func TestReadMessageLargePayload(t *testing.T) {
	pver := ProtocolVersion
	bchnet := MainNet

	// Build blocks of a few MiB by adding transactions with large output
	// scripts.  The sizes are chosen so the payload lengths are not powers
	// of two.
	for _, size := range []int{3*1024*1024 + 12345, 9 * 1000 * 1000} {
		block := NewMsgBlock(&blockOne.Header)
		const scriptLen = 100000
		for i := 0; i < size/scriptLen; i++ {
			tx := blockOne.Transactions[0].Copy()
			tx.TxOut[0].PkScript = bytes.Repeat([]byte{byte(i)},
				scriptLen)
			if err := block.AddTransaction(tx); err != nil {
				t.Fatalf("AddTransaction: %v", err)
			}
		}

		var buf bytes.Buffer
		nw, err := WriteMessageN(&buf, block, pver, bchnet)
		if err != nil {
			t.Fatalf("WriteMessageN: %v", err)
		}
		encoded := buf.Bytes()

		// Read through a reader that returns only part of what is
		// requested to exercise short reads.
		r := iotest.HalfReader(bytes.NewReader(encoded))
		nr, msg, payload, err := ReadMessageN(r, pver, bchnet)
		if err != nil {
			t.Fatalf("ReadMessageN (size %d): %v", size, err)
		}
		if nr != nw {
			t.Errorf("ReadMessageN (size %d): unexpected num bytes "+
				"read - got %d, want %d", size, nr, nw)
		}
		if cap(payload) != len(payload) {
			t.Errorf("ReadMessageN (size %d): payload capacity %d "+
				"does not match its length %d", size,
				cap(payload), len(payload))
		}
		if !bytes.Equal(payload, encoded[MessageHeaderSize:]) {
			t.Errorf("ReadMessageN (size %d): payload does not "+
				"match the written payload", size)
		}
		if !reflect.DeepEqual(msg, block) {
			t.Errorf("ReadMessageN (size %d): message does not "+
				"match the written block", size)
		}
	}
}

// TestPayloadLimits ensures the message types whose limit was set by the block
// size read the largest message a peer can validly send at the block size the
// limits are set for, and refuse a header claiming more than their limit, so
// its payload is skipped rather than held.  tx, getblocktxn and reject have
// the 2 MiB BCHN applies to every message but block, cmpctblock and blocktxn;
// the others the largest valid message.  A tx message could be as large as a
// block message, and the others about twice as large, so a peer could make a
// node hold that much for messages that are never near that size.
func TestPayloadLimits(t *testing.T) {
	pver := ProtocolVersion
	bchnet := MainNet

	// A small block size keeps the largest messages small, while blocks
	// can still hold the largest transaction.
	SetLimits(500000)
	t.Cleanup(func() { SetLimits(fixedExcessiveBlockSize) })
	blockLimit := MaxBlockPayload()
	maxTxs := maxTxPerBlock()

	// The largest block holds as many of the smallest transactions, with
	// no inputs and no outputs, as fit, and an output on the last one that
	// fills the block limit exactly.
	block := NewMsgBlock(&blockOne.Header)
	n := (int(blockLimit) - MaxBlockHeaderPayload - 2*MaxVarIntPayload) /
		minTxPayload
	for range n {
		_ = block.AddTransaction(&MsgTx{Version: 1})
	}
	pad := int(blockLimit) - block.SerializeSize() - MinTxOutPayload
	block.Transactions[n-1].AddTxOut(NewTxOut(0, make([]byte, pad),
		TokenData{}))
	if size := block.SerializeSize(); size != int(blockLimit) {
		t.Fatalf("largest block is %d bytes, want %d", size, blockLimit)
	}

	// The largest transaction consensus allows.
	tx := &MsgTx{Version: 1}
	tx.AddTxOut(NewTxOut(0, make([]byte, 1000000-23), TokenData{}))

	// A request for every transaction of the largest block, and the reply.
	getBlockTxns := NewMsgGetBlockTxns(block.BlockHash(), make([]uint32, n))
	blockTxns := NewMsgBlockTxns(block.BlockHash(), block.Transactions)

	// The largest block with every transaction prefilled.
	cmpctBlock, err := NewMsgCmpctBlockFromBlock(block, nil)
	if err != nil {
		t.Fatal(err)
	}

	// As many filter headers as a cfcheckpt can have.
	cfCheckpt := NewMsgCFCheckpt(GCSFilterRegular, &chainhash.Hash{},
		maxCFHeadersLen)
	for range maxCFHeadersLen {
		_ = cfCheckpt.AddCFHeader(&chainhash.Hash{})
	}

	// A reject whose reason fills the limit other nodes apply to every
	// message but block, cmpctblock and blocktxn.
	reject := NewMsgReject(CmdTx, RejectInvalid,
		string(make([]byte, 2*1024*1024-41)))

	tests := []struct {
		msg   Message
		size  int    // Payload size of the message
		limit uint32 // Largest payload the header may claim
	}{
		{tx, 1000000, 2 * 1024 * 1024},
		{getBlockTxns, chainhash.HashSize + 5 + n, 2 * 1024 * 1024},
		{blockTxns, int(blockLimit) - MaxBlockHeaderPayload +
			chainhash.HashSize, blockLimit},
		{cmpctBlock, int(blockLimit) + 9 + n, blockLimit + 9 + maxTxs},
		{cfCheckpt, 1 + chainhash.HashSize + 5 +
			maxCFHeadersLen*chainhash.HashSize,
			1 + chainhash.HashSize + 5 +
				maxCFHeadersLen*chainhash.HashSize},
		{reject, 2 * 1024 * 1024, 2 * 1024 * 1024},
	}
	for _, test := range tests {
		cmd := test.msg.Command()
		if mpl := test.msg.MaxPayloadLength(pver); mpl != test.limit {
			t.Errorf("%s: limit is %d bytes, want %d", cmd, mpl,
				test.limit)
		}

		var buf bytes.Buffer
		nw, err := WriteMessageN(&buf, test.msg, pver, bchnet)
		if err != nil {
			t.Errorf("%s: WriteMessageN: %v", cmd, err)
			continue
		}
		if size := nw - MessageHeaderSize; size != test.size {
			t.Errorf("%s: largest message is %d bytes, want %d",
				cmd, size, test.size)
		}
		written := bytes.Clone(buf.Bytes())
		msg, _, err := ReadMessage(&buf, pver, bchnet)
		if err != nil {
			t.Errorf("%s: largest message not read: %v", cmd, err)
		} else {
			var again bytes.Buffer
			_ = WriteMessage(&again, msg, pver, bchnet)
			if !bytes.Equal(again.Bytes(), written) {
				t.Errorf("%s: largest message read back "+
					"differently", cmd)
			}
		}

		// A header claiming one byte more is refused as too long, rather
		// than read until its payload runs out.
		hdr := makeHeader(bchnet, cmd, test.limit+1, 0)
		_, _, err = ReadMessage(bytes.NewReader(hdr), pver, bchnet)
		if _, ok := err.(*MessageError); !ok {
			t.Errorf("%s: header claiming %d bytes got %v, want it "+
				"refused at the header", cmd, test.limit+1, err)
		}
	}
}

// TestSetLimitsLargeBlockSize ensures the limits stay usable at the largest
// excessive block sizes: the block limit, four times the size, stops at the 2
// GB no block may exceed, so neither it nor the limits computed from it wrap
// around 32 bits.  At 512 MB the overall limit wrapped to 0, so no message
// with a payload, not even a version message, could be read.
func TestSetLimitsLargeBlockSize(t *testing.T) {
	pver := ProtocolVersion
	bchnet := MainNet
	t.Cleanup(func() { SetLimits(fixedExcessiveBlockSize) })

	version := NewMsgVersion(&NetAddress{}, &NetAddress{}, 1, 0)
	tests := []struct {
		size       uint32 // Excessive block size
		blockLimit uint32 // Limit of a block message
	}{
		{32000000, 128000000},
		{499999999, 1999999996},
		{500000000, 2000000000},
		{512000000, 2000000000},
		{1 << 30, 2000000000},
		{math.MaxUint32, 2000000000},
	}
	for _, test := range tests {
		SetLimits(test.size)
		if got := MaxBlockPayload(); got != test.blockLimit {
			t.Errorf("size %d: block limit %d, want %d", test.size,
				got, test.blockLimit)
		}
		if got := maxMessagePayload(); got < MaxBlockPayload() {
			t.Errorf("size %d: overall limit %d is below the block "+
				"limit %d", test.size, got, MaxBlockPayload())
		}
		for _, msg := range []Message{&MsgBlock{}, &MsgBlockTxns{},
			&MsgCmpctBlock{}} {

			if got := msg.MaxPayloadLength(pver); got < MaxBlockPayload() {
				t.Errorf("size %d: %s limit %d is below the "+
					"block limit %d", test.size,
					msg.Command(), got, MaxBlockPayload())
			}
		}

		var buf bytes.Buffer
		if err := WriteMessage(&buf, version, pver, bchnet); err != nil {
			t.Errorf("size %d: WriteMessage: %v", test.size, err)
			continue
		}
		if _, _, err := ReadMessage(&buf, pver, bchnet); err != nil {
			t.Errorf("size %d: version message not read: %v",
				test.size, err)
		}
	}
}

// TestSetMaxBlockSizeLookAhead ensures the block message limit follows the
// block size look-ahead the chain sets, at twice it as in BCHN, never below
// the limit SetLimits sets nor above 2 GB, and that a block header claiming
// the limit is read on while one claiming a byte more is refused.  The limit
// was fixed at startup, so a valid block larger than four times the excessive
// block size, which ABLA allows after a long run of full blocks, could not be
// received.
func TestSetMaxBlockSizeLookAhead(t *testing.T) {
	pver := ProtocolVersion
	bchnet := MainNet
	SetLimits(32000000)
	t.Cleanup(func() {
		SetMaxBlockSizeLookAhead(0)
		SetLimits(fixedExcessiveBlockSize)
	})

	tests := []struct {
		size       uint64 // Block size look-ahead
		blockLimit uint32 // Limit of a block message
	}{
		{0, 128000000},
		{35847033, 128000000}, // ABLA's floor on mainnet
		{64000000, 128000000},
		{64000001, 128000002},
		{170902942, 341805884},
		{999999999, 1999999998},
		{1000000000, 2000000000},
		{2000000000, 2000000000}, // ABLA's largest block
		{math.MaxUint64, 2000000000},
		{35847033, 128000000},
	}
	for _, test := range tests {
		SetMaxBlockSizeLookAhead(test.size)
		if got := MaxBlockPayload(); got != test.blockLimit {
			t.Errorf("look-ahead %d: block limit %d, want %d",
				test.size, got, test.blockLimit)
		}
		cmpctLimit := (&MsgCmpctBlock{}).MaxPayloadLength(pver)
		if got := maxMessagePayload(); got < cmpctLimit {
			t.Errorf("look-ahead %d: overall limit %d is below the "+
				"cmpctblock limit %d", test.size, got, cmpctLimit)
		}

		hdr := makeHeader(bchnet, CmdBlock, test.blockLimit, 0)
		_, _, err := ReadMessage(bytes.NewReader(hdr), pver, bchnet)
		if err != io.EOF {
			t.Errorf("look-ahead %d: header claiming %d bytes got %v, "+
				"want the payload read", test.size, test.blockLimit,
				err)
		}
		hdr = makeHeader(bchnet, CmdBlock, test.blockLimit+1, 0)
		_, _, err = ReadMessage(bytes.NewReader(hdr), pver, bchnet)
		if _, ok := err.(*MessageError); !ok {
			t.Errorf("look-ahead %d: header claiming %d bytes got %v, "+
				"want it refused at the header", test.size,
				test.blockLimit+1, err)
		}
	}

	// A larger limit from SetLimits still applies.
	SetMaxBlockSizeLookAhead(170902942)
	SetLimits(100000000)
	if got := MaxBlockPayload(); got != 400000000 {
		t.Errorf("block limit %d, want 400000000", got)
	}
}

// TestSetMaxBlockSizeLookAheadConcurrent ensures the block size look-ahead can
// be set, as the chain does whenever its tip changes, while peers read
// messages, which the race detector checks.
func TestSetMaxBlockSizeLookAheadConcurrent(t *testing.T) {
	t.Cleanup(func() { SetMaxBlockSizeLookAhead(0) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		for size := range uint64(1000) {
			SetMaxBlockSizeLookAhead(100000000 + size)
		}
	}()
	hdr := makeHeader(MainNet, CmdBlock, 200000000, 0)
	for range 1000 {
		_, _, _ = ReadMessage(bytes.NewReader(hdr), ProtocolVersion,
			MainNet)
	}
	<-done
}

// TestReadMessageLengthFitsInt ensures a header claiming more than an int can
// hold, which a cmpctblock may claim at the largest block sizes, is refused
// rather than read.  On 32-bit platforms the payload buffer's size, taken as
// an int, turned negative, and growing the buffer panicked.
func TestReadMessageLengthFitsInt(t *testing.T) {
	if math.MaxInt > math.MaxUint32 {
		t.Skip("every 32-bit length fits in an int")
	}
	pver := ProtocolVersion
	bchnet := MainNet
	SetLimits(500000000)
	t.Cleanup(func() { SetLimits(fixedExcessiveBlockSize) })

	const length uint32 = math.MaxInt32 + 1
	if mpl := (&MsgCmpctBlock{}).MaxPayloadLength(pver); mpl < length {
		t.Fatalf("cmpctblock limit %d is below the claimed %d", mpl,
			length)
	}
	buf := makeHeader(bchnet, CmdCmpctBlock, length, 0)
	buf = append(buf, make([]byte, 64*1024)...)
	_, _, err := ReadMessage(bytes.NewReader(buf), pver, bchnet)
	if _, ok := err.(*MessageError); !ok {
		t.Fatalf("header claiming %d bytes got %v, want it refused",
			length, err)
	}
}

// TestWriteMessageWireErrors performs negative tests against wire encoding from
// concrete messages to confirm error paths work correctly.
func TestWriteMessageWireErrors(t *testing.T) {
	pver := ProtocolVersion
	bchnet := MainNet
	wireErr := &MessageError{}

	// Fake message with a command that is too long.
	badCommandMsg := &fakeMessage{command: "somethingtoolong"}

	// Fake message with a problem during encoding
	encodeErrMsg := &fakeMessage{forceEncodeErr: true}

	// Fake message that has payload which exceeds max overall message size.
	exceedOverallPayload := make([]byte, maxMessagePayload()+1)
	exceedOverallPayloadErrMsg := &fakeMessage{payload: exceedOverallPayload}

	// Fake message that has payload which exceeds max allowed per message.
	exceedPayload := make([]byte, 1)
	exceedPayloadErrMsg := &fakeMessage{payload: exceedPayload, forceLenErr: true}

	// Fake message that is used to force errors in the header and payload
	// writes.
	bogusPayload := []byte{0x01, 0x02, 0x03, 0x04}
	bogusMsg := &fakeMessage{command: "bogus", payload: bogusPayload}

	tests := []struct {
		msg    Message    // Message to encode
		pver   uint32     // Protocol version for wire encoding
		bchnet BitcoinNet // Bitcoin network for wire encoding
		max    int        // Max size of fixed buffer to induce errors
		err    error      // Expected error
		bytes  int        // Expected num bytes written
	}{
		// Command too long.
		{badCommandMsg, pver, bchnet, 0, wireErr, 0},
		// Force error in payload encode.
		{encodeErrMsg, pver, bchnet, 0, wireErr, 0},
		// Force error due to exceeding max overall message payload size.
		{exceedOverallPayloadErrMsg, pver, bchnet, 0, wireErr, 0},
		// Force error due to exceeding max payload for message type.
		{exceedPayloadErrMsg, pver, bchnet, 0, wireErr, 0},
		// Force error in header write.
		{bogusMsg, pver, bchnet, 0, io.ErrShortWrite, 0},
		// Force error in payload write.
		{bogusMsg, pver, bchnet, 24, io.ErrShortWrite, 24},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode wire format.
		w := newFixedWriter(test.max)
		nw, err := WriteMessageN(w, test.msg, test.pver, test.bchnet)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("WriteMessage #%d wrong error got: %v <%T>, "+
				"want: %T", i, err, err, test.err)
			continue
		}

		// Ensure the number of bytes written match the expected value.
		if nw != test.bytes {
			t.Errorf("WriteMessage #%d unexpected num bytes "+
				"written - got %d, want %d", i, nw, test.bytes)
		}

		// For errors which are not of type MessageError, check them for
		// equality.
		if _, ok := err.(*MessageError); !ok {
			if err != test.err {
				t.Errorf("ReadMessage #%d wrong error got: %v <%T>, "+
					"want: %v <%T>", i, err, err,
					test.err, test.err)
				continue
			}
		}
	}
}

// TestNextPayloadSize ensures the payload buffer grows to twice its size, at
// most the payload's length, without overflowing a 32-bit int: twice a buffer
// over 1 GiB is more than math.MaxInt32, which made a 32-bit build panic once
// a peer had sent that much of one message.
func TestNextPayloadSize(t *testing.T) {
	tests := []struct {
		size   int
		length uint32
		want   int
	}{
		{32 * 1024, 100000, 64 * 1024},
		{64 * 1024, 100000, 100000},
		{1 << 29, 1<<31 - 1, 1 << 30},
		{1 << 30, 1<<31 - 1, 1<<31 - 1},
		{1<<30 + 1, 1<<31 - 1, 1<<31 - 1},
	}
	for _, test := range tests {
		got := nextPayloadSize(test.size, test.length)
		if got != test.want {
			t.Errorf("nextPayloadSize(%d, %d) = %d, want %d",
				test.size, test.length, got, test.want)
		}
	}
}
