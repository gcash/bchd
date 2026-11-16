// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// shvRequestBytes is a getshv payload of one request, for the proof of block 5
// to its peak in the commitment at height 10.  The block height is a compact
// size.
const shvRequestBytes = "01000a05"

// shvResponseBytes is an shv payload of one found response, the proof of the
// genesis block to the root of the commitment at height 0.  The entry starts
// with the found status, then the type, the heights, the header, the proof
// and the target.
const shvResponseBytes = "0100010000" +
	"01000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"3ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a" +
	"29ab5f49ffff001d1dac2b7c" +
	"00" +
	"6fe28c0ab6f1b372c1a6a246ae63f74f931e8365e15a089c68d6190000000000"

// mustDecodeSHVHex returns the bytes of the hex string.
func mustDecodeSHVHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// encodeSHV returns the payload of the message.
func encodeSHV(t *testing.T, msg Message) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := msg.BchEncode(&buf, ProtocolVersion, BaseEncoding); err != nil {
		t.Fatalf("%s: %v", msg.Command(), err)
	}
	return buf.Bytes()
}

// TestSHVWireVectors ensures a by-height getshv request and a found shv entry
// decode from and encode to the bytes above, with hashes in internal order,
// and that compact sizes take one, three and nine bytes as their values need.
func TestSHVWireVectors(t *testing.T) {
	requestRaw := mustDecodeSHVHex(t, shvRequestBytes)
	var request MsgGetSHV
	if err := request.BchDecode(bytes.NewReader(requestRaw), ProtocolVersion,
		BaseEncoding); err != nil {

		t.Fatal(err)
	}
	if len(request.Requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(request.Requests))
	}
	req := request.Requests[0]
	if req.Type != 0 || req.CommitmentHeight != 10 || req.BlockHeight() != 5 ||
		!req.IsValid() || req.ProofToRoot() || req.ByHash() {

		t.Errorf("got request %+v", req)
	}
	if want := NewSHVRequestByHeight(false, 10, 5); req != want {
		t.Errorf("got request %+v, want %+v", req, want)
	}
	if !bytes.Equal(encodeSHV(t, &request), requestRaw) {
		t.Error("the request does not encode to the bytes it came from")
	}

	responseRaw := mustDecodeSHVHex(t, shvResponseBytes)
	var response MsgSHV
	if err := response.BchDecode(bytes.NewReader(responseRaw),
		ProtocolVersion, BaseEncoding); err != nil {

		t.Fatal(err)
	}
	if len(response.Responses) != 1 {
		t.Fatalf("got %d responses, want 1", len(response.Responses))
	}
	resp := response.Responses[0]
	if resp.Status != SHVStatusFound || resp.Type != SHVProofToRoot ||
		resp.CommitmentHeight != 0 || resp.BlockHeight != 0 ||
		resp.Proof == nil || len(resp.Proof) != 0 {

		t.Errorf("got response %+v", resp)
	}
	genesis := "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
	if resp.Target.String() != genesis || resp.Header.BlockHash() != resp.Target {
		t.Errorf("target %v, header hash %v, want %v", resp.Target,
			resp.Header.BlockHash(), genesis)
	}
	if resp.SerializeSize() != len(responseRaw)-1 {
		t.Errorf("serialize size %d, want %d", resp.SerializeSize(),
			len(responseRaw)-1)
	}
	if !bytes.Equal(encodeSHV(t, &response), responseRaw) {
		t.Error("the response does not encode to the bytes it came from")
	}

	// The commands and the service bit.
	for cmd, want := range map[string]Message{CmdGetSHV: &MsgGetSHV{},
		CmdSHV: &MsgSHV{}} {

		msg, err := makeEmptyMessage(cmd)
		if err != nil || reflect.TypeOf(msg) != reflect.TypeOf(want) ||
			msg.Command() != cmd {

			t.Errorf("%s: got %T, %v", cmd, msg, err)
		}
	}
	if CmdGetSHV != "getshv" || CmdSHV != "shv" {
		t.Errorf("commands %q and %q", CmdGetSHV, CmdSHV)
	}

	// A commitment height of 2^32 takes nine bytes.
	large := NewMsgGetSHV(SHVRequest{Type: SHVProofToRoot | SHVByHash,
		CommitmentHeight: 0x100000000})
	largeRaw := mustDecodeSHVHex(t, "0103ff0000000001000000"+
		"0000000000000000000000000000000000000000000000000000000000000000")
	if !bytes.Equal(encodeSHV(t, large), largeRaw) {
		t.Errorf("got %x, want %x", encodeSHV(t, large), largeRaw)
	}
	for _, commitment := range []uint64{0xfc, 0xfd, 0xffff, 0x10000,
		0xffffffff, 0x100000000} {

		large.Requests[0].CommitmentHeight = commitment
		var decoded MsgGetSHV
		if err := decoded.BchDecode(bytes.NewReader(encodeSHV(t, large)),
			ProtocolVersion, BaseEncoding); err != nil ||
			!reflect.DeepEqual(&decoded, large) {

			t.Errorf("commitment height %#x: got %+v, %v", commitment,
				decoded, err)
		}
	}

	// Empty messages are a zero count.
	for _, msg := range []Message{NewMsgGetSHV(), NewMsgSHV()} {
		if raw := encodeSHV(t, msg); !bytes.Equal(raw, []byte{0}) {
			t.Errorf("%s: empty message encodes to %x", msg.Command(), raw)
		}
	}
}

// TestSHVRequestValidity ensures the four request types are valid and a
// request with a reserved type bit is not, that bytes past a height are not
// part of it, and that an invalid request still decodes, so the server can
// answer it with a status.
func TestSHVRequestValidity(t *testing.T) {
	var req SHVRequest
	req.BlockHeightOrHash[0] = 0xef
	req.BlockHeightOrHash[7] = 0x12
	if height := req.BlockHeight(); height != 0x12000000000000ef {
		t.Errorf("block height %#x", height)
	}
	for typ := uint8(0); typ < 4; typ++ {
		req.Type = typ
		if !req.IsValid() || req.ProofToRoot() != (typ&1 != 0) ||
			req.ByHash() != (typ&2 != 0) {

			t.Errorf("type %d: valid %v, to root %v, by hash %v", typ,
				req.IsValid(), req.ProofToRoot(), req.ByHash())
		}
	}
	for bit := uint8(2); bit < 8; bit++ {
		req.Type = 1 << bit
		if req.IsValid() {
			t.Errorf("type %#x is valid", req.Type)
		}
	}

	// Bytes past the first eight are not part of a height, and are not
	// written.  A hash keeps all 32.
	req.Type = 0
	req.BlockHeightOrHash[8] = 1
	if !req.IsValid() || req.BlockHeight() != 0x12000000000000ef {
		t.Errorf("by height: valid %v, height %#x", req.IsValid(),
			req.BlockHeight())
	}
	var decodedHeight MsgGetSHV
	if err := decodedHeight.BchDecode(bytes.NewReader(encodeSHV(t,
		NewMsgGetSHV(req))), ProtocolVersion, BaseEncoding); err != nil {

		t.Fatal(err)
	}
	if got := decodedHeight.Requests[0]; got.BlockHeight() != req.BlockHeight() ||
		got.BlockHeightOrHash[8] != 0 {

		t.Errorf("by height read back %+v", got)
	}
	req.Type = SHVByHash
	if !req.IsValid() {
		t.Error("a hash is not valid")
	}
	req.Type |= 0x80
	if req.IsValid() {
		t.Error("a hash with a reserved type bit is valid")
	}

	hash := chainhash.Hash{1, 2, 3}
	byHash := NewSHVRequestByHash(true, 7, &hash)
	if !byHash.ByHash() || !byHash.ProofToRoot() || !byHash.IsValid() ||
		byHash.BlockHeightOrHash != hash || byHash.CommitmentHeight != 7 {

		t.Errorf("got request %+v", byHash)
	}

	msg := NewMsgGetSHV(req, byHash, NewSHVRequestByHeight(true, 3, 2))
	var decoded MsgGetSHV
	if err := decoded.BchDecode(bytes.NewReader(encodeSHV(t, msg)),
		ProtocolVersion, BaseEncoding); err != nil ||
		!reflect.DeepEqual(&decoded, msg) {

		t.Errorf("got %+v, %v, want %+v", decoded, err, msg)
	}
}

// nonCanonical replaces the one-byte compact size at offset of raw with the
// same value in three bytes.
func nonCanonical(raw []byte, offset int) []byte {
	return bytes.Join([][]byte{raw[:offset], {0xfd, raw[offset], 0x00},
		raw[offset+1:]}, nil)
}

// TestSHVWireRefusals ensures truncated getshv and shv payloads fail to decode
// wherever they are cut, as do compact sizes that are not canonical, an shv
// message of more than 100 responses or with a proof of more than 64 hashes,
// and a getshv message of more requests than its payload limit holds, while
// a getshv message of more than 100 requests, which the server penalizes,
// decodes.  The largest messages read back as written.
func TestSHVWireRefusals(t *testing.T) {
	decode := func(msg Message, raw []byte) error {
		return msg.BchDecode(bytes.NewReader(raw), ProtocolVersion,
			BaseEncoding)
	}

	requestRaw := mustDecodeSHVHex(t, shvRequestBytes)
	var response MsgSHV
	if err := decode(&response, mustDecodeSHVHex(t, shvResponseBytes)); err != nil {
		t.Fatal(err)
	}
	response.Responses[0].Proof = []chainhash.Hash{{},
		response.Responses[0].Target}
	responseRaw := encodeSHV(t, &response)

	for size := range requestRaw {
		if err := decode(&MsgGetSHV{}, requestRaw[:size]); err == nil {
			t.Errorf("getshv cut to %d bytes decoded", size)
		}
	}
	for size := range responseRaw {
		if err := decode(&MsgSHV{}, responseRaw[:size]); err == nil {
			t.Errorf("shv cut to %d bytes decoded", size)
		}
	}

	// The request count, commitment height and block height.  The response
	// count, the commitment and block heights and the proof's hash count.
	// The proof count follows the status, the type, the two heights and
	// the header.
	for _, offset := range []int{0, 2, 3} {
		err := decode(&MsgGetSHV{}, nonCanonical(requestRaw, offset))
		if _, ok := err.(*MessageError); !ok {
			t.Errorf("getshv with a non-canonical size at %d: %v",
				offset, err)
		}
	}
	shvRaw := mustDecodeSHVHex(t, shvResponseBytes)
	proofCountAt := 5 + blockHeaderLen
	for _, offset := range []int{0, 3, 4, proofCountAt} {
		err := decode(&MsgSHV{}, nonCanonical(shvRaw, offset))
		if _, ok := err.(*MessageError); !ok {
			t.Errorf("shv with a non-canonical size at %d: %v", offset,
				err)
		}
	}

	// Too many responses, or hashes in a proof.
	if err := decode(&MsgSHV{}, []byte{MaxSHVRequestsPerMsg + 1}); err == nil {
		t.Error("shv of 101 responses decoded")
	} else if _, ok := err.(*MessageError); !ok {
		t.Errorf("shv of 101 responses: %v", err)
	}
	tooLong := bytes.Clone(shvRaw)
	tooLong[proofCountAt] = MaxSHVProofLength + 1
	if err := decode(&MsgSHV{}, tooLong); err == nil {
		t.Error("shv with a proof of 65 hashes decoded")
	} else if _, ok := err.(*MessageError); !ok {
		t.Errorf("shv with a proof of 65 hashes: %v", err)
	}
	over := NewMsgSHV(make([]SHVResponse, MaxSHVRequestsPerMsg + 1)...)
	if err := over.BchEncode(&bytes.Buffer{}, ProtocolVersion,
		BaseEncoding); err == nil {

		t.Error("shv of 101 responses encoded")
	}
	over = NewMsgSHV(SHVResponse{Proof: make([]chainhash.Hash,
		MaxSHVProofLength + 1)})
	if err := over.BchEncode(&bytes.Buffer{}, ProtocolVersion,
		BaseEncoding); err == nil {

		t.Error("shv with a proof of 65 hashes encoded")
	}

	// More than 100 requests decode, up to as many as the payload limit
	// holds, but no more.
	requests := make([]SHVRequest, MaxSHVRequestsPerMsg + 1)
	for i := range requests {
		requests[i] = NewSHVRequestByHeight(i%2 == 0, uint64(i), uint64(i))
	}
	var decoded MsgGetSHV
	if err := decode(&decoded, encodeSHV(t, NewMsgGetSHV(requests...))); err != nil ||
		!reflect.DeepEqual(decoded.Requests, requests) {

		t.Errorf("getshv of 101 requests: %v", err)
	}
	var count bytes.Buffer
	_ = WriteVarInt(&count, 0, maxSHVRequestsDecode + 1)
	if err := decode(&MsgGetSHV{}, count.Bytes()); err == nil {
		t.Error("getshv of more requests than fit decoded")
	} else if _, ok := err.(*MessageError); !ok {
		t.Errorf("getshv of more requests than fit: %v", err)
	}
	if err := NewMsgGetSHV(make([]SHVRequest,
		maxSHVRequestsDecode + 1)...).BchEncode(&bytes.Buffer{},
		ProtocolVersion, BaseEncoding); err == nil {

		t.Error("getshv of more requests than fit encoded")
	}

	// The largest messages: as many minimum requests as fit, and 100 found
	// responses with proofs of 64 hashes and the longest heights.  The
	// decode cap counts request bodies only, so a count whose compact
	// size no longer fits in the leftover bytes is one request smaller.
	n := maxSHVRequestsDecode
	for VarIntSerializeSize(uint64(n)) + n*minSHVRequestPayload >
		maxProtocolMessageLength {

		n--
	}
	full := make([]SHVRequest, n)
	for i := range full {
		full[i] = NewSHVRequestByHeight(true, uint64(i % 0xfd),
			uint64(i % 0xfd))
	}
	longest := make([]SHVResponse, MaxSHVRequestsPerMsg)
	for i := range longest {
		longest[i] = response.Responses[0]
		longest[i].CommitmentHeight = ^uint64(0)
		longest[i].BlockHeight = ^uint64(0)
		longest[i].Proof = make([]chainhash.Hash, MaxSHVProofLength)
		for j := range longest[i].Proof {
			longest[i].Proof[j] = chainhash.Hash{byte(i), byte(j)}
		}
	}
	for _, msg := range []Message{NewMsgGetSHV(full...),
		NewMsgSHV(longest...)} {

		var buf bytes.Buffer
		if err := WriteMessage(&buf, msg, ProtocolVersion, MainNet); err != nil {
			t.Fatalf("%s: %v", msg.Command(), err)
		}
		read, _, err := ReadMessage(&buf, ProtocolVersion, MainNet)
		if err != nil {
			t.Fatalf("%s: %v", msg.Command(), err)
		}
		if !reflect.DeepEqual(read, msg) {
			t.Errorf("%s: the message read back differs", msg.Command())
		}
	}
}

// TestSHVHeightAndStatus ensures a request by height writes that height as a
// compact size, a request by hash keeps its 32 bytes, an shv message of every
// status round-trips, a status above the reserved one does not decode, and a
// found entry cut short does not decode.
func TestSHVHeightAndStatus(t *testing.T) {
	decode := func(msg Message, raw []byte) error {
		return msg.BchDecode(bytes.NewReader(raw), ProtocolVersion,
			BaseEncoding)
	}
	roundTripGet := func(msg *MsgGetSHV) {
		t.Helper()
		var got MsgGetSHV
		if err := decode(&got, encodeSHV(t, msg)); err != nil ||
			!reflect.DeepEqual(&got, msg) {

			t.Fatalf("got %+v, %v, want %+v", got, err, msg)
		}
	}
	roundTripSHV := func(msg *MsgSHV) {
		t.Helper()
		var got MsgSHV
		if err := decode(&got, encodeSHV(t, msg)); err != nil ||
			!reflect.DeepEqual(&got, msg) {

			t.Fatalf("got %+v, %v, want %+v", got, err, msg)
		}
	}

	// Small and large heights, and the compact-size boundaries between.
	small := NewMsgGetSHV(NewSHVRequestByHeight(false, 10, 5))
	if raw := encodeSHV(t, small); !bytes.Equal(raw, mustDecodeSHVHex(t,
		"01000a05")) {

		t.Errorf("height 5 encodes to %x", raw)
	}
	large := NewMsgGetSHV(NewSHVRequestByHeight(false, 10, 0x100000000))
	if raw := encodeSHV(t, large); !bytes.Equal(raw, mustDecodeSHVHex(t,
		"01000aff0000000001000000")) {

		t.Errorf("height 2^32 encodes to %x", raw)
	}
	for _, height := range []uint64{0, 5, 0xfc, 0xfd, 0xffff, 0x10000,
		0xffffffff, 0x100000000, ^uint64(0)} {

		msg := NewMsgGetSHV(NewSHVRequestByHeight(true, height, height))
		roundTripGet(msg)
		if got := msg.Requests[0]; got.ByHash() || got.BlockHeight() != height {
			t.Errorf("height %#x: %+v", height, got)
		}
	}

	hash := chainhash.Hash{0x11, 0x22, 0x33}
	byHash := NewMsgGetSHV(NewSHVRequestByHash(false, 10, &hash))
	wantHash := "01020a112233" +
		"0000000000000000000000000000000000000000000000000000000000"
	if raw := encodeSHV(t, byHash); !bytes.Equal(raw,
		mustDecodeSHVHex(t, wantHash)) {

		t.Errorf("by hash encodes to %x", raw)
	}
	roundTripGet(byHash)
	roundTripGet(NewMsgGetSHV(NewSHVRequestByHash(true, 0x100000000, &hash)))

	// Bit 1 chooses the field, including when a reserved bit is also set.
	reservedHeight := NewSHVRequestByHeight(false, 1, 2)
	reservedHeight.Type |= 0x04
	if raw := encodeSHV(t, NewMsgGetSHV(reservedHeight)); !bytes.Equal(raw,
		[]byte{1, 0x04, 1, 2}) {

		t.Errorf("reserved by-height request encodes to %x", raw)
	}
	both := NewMsgGetSHV(NewSHVRequestByHeight(false, 10, 5),
		NewSHVRequestByHash(true, 7, &hash))
	roundTripGet(both)

	var found MsgSHV
	if err := decode(&found, mustDecodeSHVHex(t, shvResponseBytes)); err != nil {
		t.Fatal(err)
	}
	statuses := NewMsgSHV(
		found.Responses[0],
		SHVResponse{Status: SHVStatusNotFound},
		SHVResponse{Status: SHVStatusAboveCommitment},
		SHVResponse{Status: SHVStatusCommitmentAboveTip},
		SHVResponse{Status: SHVStatusReservedType},
	)
	roundTripSHV(statuses)
	for _, resp := range statuses.Responses[1:] {
		if resp.SerializeSize() != 1 {
			t.Errorf("status %d takes %d bytes", resp.Status,
				resp.SerializeSize())
		}
	}

	for _, status := range []byte{5, 0xff} {
		err := decode(&MsgSHV{}, []byte{1, status})
		if _, ok := err.(*MessageError); !ok {
			t.Errorf("status %d: %v", status, err)
		}
		bad := NewMsgSHV(SHVResponse{Status: status})
		if err := bad.BchEncode(&bytes.Buffer{}, ProtocolVersion,
			BaseEncoding); err == nil {

			t.Errorf("status %d encoded", status)
		} else if _, ok := err.(*MessageError); !ok {
			t.Errorf("status %d: %v", status, err)
		}
	}

	// A found entry needs the fields after its status.  A not-found entry
	// is complete at that byte.
	for _, raw := range [][]byte{
		{1},
		{1, SHVStatusFound},
		{1, SHVStatusFound, SHVProofToRoot},
	} {
		if err := decode(&MsgSHV{}, raw); err == nil {
			t.Errorf("%x decoded", raw)
		}
	}
	statusRaw := encodeSHV(t, statuses)
	for size := range statusRaw {
		if err := decode(&MsgSHV{}, statusRaw[:size]); err == nil {
			t.Errorf("shv cut to %d bytes decoded", size)
		}
	}
}
