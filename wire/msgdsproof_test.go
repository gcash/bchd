// Copyright (c) 2020-2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/hex"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

// bchnDSProofVectors are two serialized proofs and their ids taken from the
// Bitcoin Cash Node functional test bchn-rpc-dsproof.py.
var bchnDSProofVectors = []struct {
	id  string
	hex string
}{
	{
		id: "978c2b3d829dbc934c170ff797c539a86d35fcfc0ec806a5753c00794cd5caad",
		hex: "326bd6eee699d18308a04720583f663ba039070a1abd8a59868ee03a1c250be10000000001000000feffffffbadb1500dbd7cf882" +
			"c620ed8fb660b64444bfb4febf6c553d0f19cafdc3070bc2c27664618606b350cd8bf565266bc352f0caddcf01e8fa789dd8a1538" +
			"6327cf8cabe198fdef7e5d2f370d4e96ab7cc22482f181b2c0e7e6275838aeed19eeedbfd378170141467adbad7deb7635bdf6bbe" +
			"4c605ce57c3dccd01f5fb9e32b22c3479a1d3f143f3f9592f9e1ef9ea96f01141b261e468c46d31a4a63cde692947d126f34641e3" +
			"4101000000feffffffbadb1500dbd7cf882c620ed8fb660b64444bfb4febf6c553d0f19cafdc3070bc2c27664618606b350cd8bf5" +
			"65266bc352f0caddcf01e8fa789dd8a15386327cf8cabe198188af582e7a09fcfe0b1a37ee3ca6c91f80c13006e595c79320ac38d" +
			"40a945cf0141210f8a36fe24b9fb1cb5a2a8cb01ac27d58410d8d8f3abf6fe935b2b1c1eadb285a4cdcd24727472af4d65b1c7ccb" +
			"120361bdcbcadfb2f1436df9bfe9b9a5b0641",
	},
	{
		id: "0e7f2e002073916cfa16692df9b44c2d52e808f7b55e75e7d941356dd90f2096",
		hex: "11565d6e11586e4d0b989358be23299458afd76d8eedad32f96a671f775970740000000001000000feffffff4fdc1500ac7850e9b" +
			"64559f703a9e6068bde8c175761408f0777299691083b0fc534aef618606b350cd8bf565266bc352f0caddcf01e8fa789dd8a1538" +
			"6327cf8cabe198cdf604b8294fe87f39c637bcab10869db7cc306b0ddbf35ed92ab526dd18af69014126b3e82473f456f9bcb2bf1" +
			"20ede1ad6e3ee588935b70033cfb625c317ced26f116b54d2effc3c9abf5efd38cffae57af50fb5fef88e1be7dc9d82a415fc1367" +
			"4101000000feffffff4fdc1500ac7850e9b64559f703a9e6068bde8c175761408f0777299691083b0fc534aef618606b350cd8bf5" +
			"65266bc352f0caddcf01e8fa789dd8a15386327cf8cabe1981194289492779938f938ce59ba48f916ba0b883803fbf2bfab22bf8d" +
			"b09227ba0141fdb9ff69c028a6a1e4143bedcf2f44b1ea2b6996bd463440f9d3037845ad7a879963acbb424d3850ba6affdf81325" +
			"e7753294a2e1959d9e84ba6108ce15e7cdc41",
	},
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestDSProofBCHNVectors decodes proofs serialized by Bitcoin Cash Node and
// checks that they encode back to the same bytes and hash to the same ids.
func TestDSProofBCHNVectors(t *testing.T) {
	for i, v := range bchnDSProofVectors {
		raw := decodeHex(t, v.hex)
		var msg MsgDSProof
		if err := msg.BchDecode(bytes.NewReader(raw), ProtocolVersion, BaseEncoding); err != nil {
			t.Fatalf("vector %d: decode: %v", i, err)
		}
		var buf bytes.Buffer
		if err := msg.BchEncode(&buf, ProtocolVersion, BaseEncoding); err != nil {
			t.Fatalf("vector %d: encode: %v", i, err)
		}
		if !bytes.Equal(buf.Bytes(), raw) {
			t.Fatalf("vector %d: encoding differs\n got: %x\nwant: %x", i, buf.Bytes(), raw)
		}
		if msg.SerializeSize() != len(raw) {
			t.Fatalf("vector %d: SerializeSize %d, want %d", i, msg.SerializeSize(), len(raw))
		}
		if got := msg.ProofHash(); got.String() != v.id {
			t.Fatalf("vector %d: id %v, want %v", i, got, v.id)
		}

		// Both spenders carry one 65 byte Schnorr signature with
		// SIGHASH_ALL|FORKID and spend the same output with version 1.
		for _, s := range []DSProofSpender{msg.FirstSpender, msg.DoubleSpender} {
			if s.Version != 1 || s.Sequence != 0xfffffffe {
				t.Fatalf("vector %d: version %d sequence %x", i, s.Version, s.Sequence)
			}
			if len(s.PushData) != 1 || len(s.PushData[0]) != 65 || s.PushData[0][64] != 0x41 {
				t.Fatalf("vector %d: unexpected push data %x", i, s.PushData)
			}
		}
		if msg.PrevOut.Index != 0 {
			t.Fatalf("vector %d: prevout index %d", i, msg.PrevOut.Index)
		}
		if c := msg.FirstSpender.HashOutputs.Compare(&msg.DoubleSpender.HashOutputs); c > 0 {
			t.Fatalf("vector %d: spenders not in canonical order", i)
		}

		// The same bytes must also come through the message reader.
		var wbuf bytes.Buffer
		if _, err := WriteMessageN(&wbuf, &msg, ProtocolVersion, MainNet); err != nil {
			t.Fatalf("vector %d: WriteMessageN: %v", i, err)
		}
		_, rmsg, payload, err := ReadMessageN(&wbuf, ProtocolVersion, MainNet)
		if err != nil {
			t.Fatalf("vector %d: ReadMessageN: %v", i, err)
		}
		if !bytes.Equal(payload, raw) || !reflect.DeepEqual(rmsg, &msg) {
			t.Fatalf("vector %d: message round trip differs", i)
		}
	}
}

// TestDSProofAPI checks the command, payload limit, inventory type and the
// layout of an empty proof.
func TestDSProofAPI(t *testing.T) {
	msg := NewMsgDSProof(OutPoint{Index: 1}, DSProofSpender{}, DSProofSpender{})
	if cmd := msg.Command(); cmd != "dsproof-beta" {
		t.Fatalf("command %q", cmd)
	}
	if len(CmdDSProof) != CommandSize {
		t.Fatalf("command length %d", len(CmdDSProof))
	}
	if got := msg.MaxPayloadLength(ProtocolVersion); got != 2*1024*1024 {
		t.Fatalf("max payload %d", got)
	}
	if InvTypeDSProof != 0x94a0 || InvTypeDSProof.String() != "MSG_DOUBLESPENDPROOF" {
		t.Fatalf("inv type %d %v", uint32(InvTypeDSProof), InvTypeDSProof)
	}

	// 32 byte hash, 4 byte index, then per spender 3*4 + 3*32 bytes and a
	// zero push count.
	var buf bytes.Buffer
	if err := msg.BchEncode(&buf, 0, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	want := make([]byte, 36+2*109)
	want[32] = 1
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("encoding %x", buf.Bytes())
	}

	var got MsgDSProof
	if err := got.BchDecode(bytes.NewReader(want), 0, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	if got.PrevOut != msg.PrevOut || len(got.FirstSpender.PushData) != 0 ||
		len(got.DoubleSpender.PushData) != 0 {
		t.Fatalf("decoded %+v", got)
	}
}

// TestDSProofSpenderFields checks that each field lands in its own spender,
// so values written for the second spender are read back into the second
// spender.
func TestDSProofSpenderFields(t *testing.T) {
	msg := &MsgDSProof{
		PrevOut: OutPoint{Hash: chainhash.Hash{1}, Index: 7},
		FirstSpender: DSProofSpender{
			Version: 1, Sequence: 2, LockTime: 3,
			HashPrevOutputs: chainhash.Hash{4}, HashSequence: chainhash.Hash{5},
			HashOutputs: chainhash.Hash{6}, PushData: [][]byte{{7}, {}},
		},
		DoubleSpender: DSProofSpender{
			Version: 0x80000001, Sequence: 9, LockTime: 10,
			HashPrevOutputs: chainhash.Hash{11}, HashSequence: chainhash.Hash{12},
			HashOutputs: chainhash.Hash{13}, PushData: [][]byte{bytes.Repeat([]byte{14}, 300)},
		},
	}
	var buf bytes.Buffer
	if err := msg.BchEncode(&buf, ProtocolVersion, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != msg.SerializeSize() {
		t.Fatalf("SerializeSize %d, encoded %d", msg.SerializeSize(), buf.Len())
	}
	var got MsgDSProof
	if err := got.BchDecode(bytes.NewReader(buf.Bytes()), ProtocolVersion, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&got, msg) {
		t.Fatalf("round trip\n got: %+v\nwant: %+v", got, msg)
	}
}

// TestDSProofDecodeErrors checks that every truncation of a valid proof fails
// to decode, that trailing bytes are ignored, and that oversized push counts
// and lengths are refused before allocating.
func TestDSProofDecodeErrors(t *testing.T) {
	raw := decodeHex(t, bchnDSProofVectors[0].hex)
	for n := 0; n < len(raw); n++ {
		var msg MsgDSProof
		err := msg.BchDecode(bytes.NewReader(raw[:n]), ProtocolVersion, BaseEncoding)
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			t.Fatalf("truncated to %d bytes: got error %v", n, err)
		}
	}

	var msg MsgDSProof
	extra := append(append([]byte{}, raw...), 0x02, 0x01, 0xff)
	if err := msg.BchDecode(bytes.NewReader(extra), ProtocolVersion, BaseEncoding); err != nil {
		t.Fatalf("trailing bytes: %v", err)
	}
	if msg.ProofHash().String() != bchnDSProofVectors[0].id {
		t.Fatalf("trailing bytes changed the id")
	}

	// Push counts above the limit are refused, even with the data there.
	head := make([]byte, 36+108)
	huge := append(append([]byte{}, head...), 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)
	if _, ok := msg.BchDecode(bytes.NewReader(huge), ProtocolVersion, BaseEncoding).(*MessageError); !ok {
		t.Fatal("huge push count accepted")
	}
	many := append(append([]byte{}, head...), 0xfd, 0xe9, 0x03)
	many = append(many, make([]byte, maxDSProofPushes+1)...)
	if _, ok := msg.BchDecode(bytes.NewReader(many), ProtocolVersion, BaseEncoding).(*MessageError); !ok {
		t.Fatal("too many empty pushes accepted")
	}
	atLimit := append(append([]byte{}, head...), 0xfd, 0xe8, 0x03)
	atLimit = append(atLimit, make([]byte, maxDSProofPushes+108+1)...)
	if err := msg.BchDecode(bytes.NewReader(atLimit), ProtocolVersion, BaseEncoding); err != nil {
		t.Fatalf("pushes at the limit: %v", err)
	}

	// A push longer than the payload limit.
	long := append(append([]byte{}, head...), 0x01, 0xfe, 0x01, 0x00, 0x20, 0x00)
	err := msg.BchDecode(bytes.NewReader(long), ProtocolVersion, BaseEncoding)
	if _, ok := err.(*MessageError); !ok || !strings.Contains(err.Error(), "dsproof push data") {
		t.Fatalf("long push: got %v", err)
	}

	// Non-canonical varints are refused as for every other message.
	nonCanon := append(append([]byte{}, head...), 0xfd, 0x01, 0x00)
	if err := msg.BchDecode(bytes.NewReader(nonCanon), ProtocolVersion, BaseEncoding); err == nil {
		t.Fatal("non-canonical push count accepted")
	}
}

// FuzzDSProofDecode checks that decoding never panics and that whatever
// decodes encodes back to the bytes it was decoded from.
func FuzzDSProofDecode(f *testing.F) {
	for _, v := range bchnDSProofVectors {
		raw, _ := hex.DecodeString(v.hex)
		f.Add(raw)
	}
	f.Add(make([]byte, 36+2*109))
	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		var msg MsgDSProof
		if err := msg.BchDecode(r, ProtocolVersion, BaseEncoding); err != nil {
			return
		}
		consumed := data[:len(data)-r.Len()]
		var buf bytes.Buffer
		if err := msg.BchEncode(&buf, ProtocolVersion, BaseEncoding); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf.Bytes(), consumed) {
			t.Fatalf("re-encoding differs\n got: %x\nwant: %x", buf.Bytes(), consumed)
		}
		if msg.SerializeSize() != len(consumed) {
			t.Fatalf("SerializeSize %d, consumed %d", msg.SerializeSize(), len(consumed))
		}
	})
}

// TestDSProofLimits pins the dsproof-beta message limits.
func TestDSProofLimits(t *testing.T) {
	if MaxDSProofPayload != 2*1024*1024 {
		t.Errorf("MaxDSProofPayload is %d", MaxDSProofPayload)
	}
	if maxDSProofPushes != 1000 {
		t.Errorf("maxDSProofPushes is %d", maxDSProofPushes)
	}
	if got := (&MsgDSProof{}).MaxPayloadLength(ProtocolVersion); got != MaxDSProofPayload {
		t.Errorf("MaxPayloadLength is %d", got)
	}
}
