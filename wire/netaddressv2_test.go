// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// addrV2Entry returns an encoded addrv2 entry with a fixed timestamp,
// services and port around the given network ID, length and address hex.
func addrV2Entry(t *testing.T, netAddrHex string) []byte {
	t.Helper()
	b, err := hex.DecodeString("29ab5f49" + // timestamp
		"01" + // services
		netAddrHex +
		"208d") // port 8333
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestNetAddressV2Decode decodes the BIP155 address vectors from the Bitcoin
// Cash Node and Bitcoin Core unit tests.
func TestNetAddressV2Decode(t *testing.T) {
	tests := []struct {
		name    string
		in      string // network ID, length and address
		network NetworkID
		host    string
		skip    bool // well formed but skipped
		err     bool // malformed
	}{
		{name: "ipv4", in: "01" + "04" + "01020304",
			network: NetworkIPv4, host: "1.2.3.4"},
		{name: "ipv4 short", in: "01" + "04" + "0102", err: true},
		{name: "ipv4 bogus length", in: "01" + "05" + "01020304",
			err: true},
		{name: "ipv4 extreme length", in: "01" + "fd0102" + "01020304",
			err: true},
		{name: "ipv6", in: "02" + "10" + "0102030405060708090a0b0c0d0e0f10",
			network: NetworkIPv6, host: "102:304:506:708:90a:b0c:d0e:f10"},
		{name: "ipv6 bogus length", in: "02" + "04" + "00", err: true},
		{name: "ipv6 embedded ipv4",
			in: "02" + "10" + "00000000000000000000ffff01020304", skip: true},
		{name: "ipv6 embedded torv2",
			in: "02" + "10" + "fd87d87eeb430102030405060708090a", skip: true},
		{name: "torv2", in: "03" + "0a" + "f1f2f3f4f5f6f7f8f9fa", skip: true},
		{name: "torv2 bogus length", in: "03" + "07" + "00", err: true},
		{name: "torv3", in: "04" + "20" +
			"79bcc625184b05194975c28b66b66b0469f7f6556fb1ac3189a79b40dda32f1f",
			network: NetworkTorV3,
			host:    "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion"},
		{name: "torv3 bogus length", in: "04" + "00" + "00", err: true},
		{name: "i2p", in: "05" + "20" +
			"a2894dabaec08c0051a481a6dac88b64f98232ae42d4b6fd2fa81952dfe36a87",
			network: NetworkI2P,
			host:    "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p"},
		{name: "i2p bogus length", in: "05" + "03" + "00", err: true},
		{name: "cjdns", in: "06" + "10" + "fc000001000200030004000500060007",
			network: NetworkCJDNS, host: "fc00:1:2:3:4:5:6:7"},
		{name: "cjdns bogus length", in: "06" + "01" + "00", err: true},
		{name: "cjdns outside fc00::/8",
			in: "06" + "10" + "20014860000000000000000000000001", skip: true},
		{name: "unknown extreme length",
			in: "aa" + "fe00000002" + "01020304050607", err: true},
		{name: "unknown", in: "aa" + "04" + "01020304", skip: true},
	}

	for _, test := range tests {
		var na NetAddressV2
		r := bytes.NewReader(addrV2Entry(t, test.in))
		err := readNetAddressV2(r, ProtocolVersion, &na)
		switch {
		case test.err:
			if err == nil || err == errSkipAddrV2 {
				t.Errorf("%s: got err %v, want a decode error",
					test.name, err)
			}
			continue
		case test.skip:
			if err != errSkipAddrV2 {
				t.Errorf("%s: got err %v, want skip", test.name, err)
			}
		case err != nil:
			t.Errorf("%s: unexpected error %v", test.name, err)
			continue
		}
		// Skipped entries must be consumed completely.
		if r.Len() != 0 {
			t.Errorf("%s: %d bytes left", test.name, r.Len())
		}
		if test.skip {
			continue
		}
		if na.Network != test.network || na.Host() != test.host {
			t.Errorf("%s: got %v %s, want %v %s", test.name,
				na.Network, na.Host(), test.network, test.host)
		}
		if na.Port != 8333 || na.Services != SFNodeNetwork ||
			na.Timestamp.Unix() != 0x495fab29 {
			t.Errorf("%s: wrong port, services or timestamp: %+v",
				test.name, na)
		}

		// Encoding gives back the input.
		var buf bytes.Buffer
		if err := writeNetAddressV2(&buf, ProtocolVersion, &na); err != nil {
			t.Errorf("%s: write: %v", test.name, err)
			continue
		}
		if want := addrV2Entry(t, test.in); !bytes.Equal(buf.Bytes(), want) {
			t.Errorf("%s: encoded %x, want %x", test.name,
				buf.Bytes(), want)
		}
	}
}

// TestNetAddressV2Encode checks the BIP155 encoding of addresses created from
// IP addresses and onion names against the Bitcoin Cash Node vectors.
func TestNetAddressV2Encode(t *testing.T) {
	torV3, err := DecodeTorV3(
		"kpgvmscirrdqpekbqjsvw5teanhatztpp2gl6eee4zkowvwfxwenqaid.onion")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		na   *NetAddressV2
		want string
	}{
		{NewNetAddressV2IPPort(net.ParseIP("1.2.3.4"), 8333, SFNodeNetwork),
			"010401020304"},
		{NewNetAddressV2IPPort(net.IPv4(1, 2, 3, 4).To4(), 8333, SFNodeNetwork),
			"010401020304"},
		{NewNetAddressV2IPPort(
			net.ParseIP("1a1b:2a2b:3a3b:4a4b:5a5b:6a6b:7a7b:8a8b"), 8333,
			SFNodeNetwork),
			"02101a1b2a2b3a3b4a4b5a5b6a6b7a7b8a8b"},
		{NewNetAddressV2(NetworkTorV3, torV3, 8333, SFNodeNetwork),
			"042053cd5648488c4707914182655b7664034e09e66f7e8cbf1084e654eb56c5bd88"},
	}
	for i, test := range tests {
		test.na.Timestamp = time.Unix(0x495fab29, 0)
		var buf bytes.Buffer
		if err := writeNetAddressV2(&buf, ProtocolVersion, test.na); err != nil {
			t.Errorf("#%d: %v", i, err)
			continue
		}
		if want := addrV2Entry(t, test.want); !bytes.Equal(buf.Bytes(), want) {
			t.Errorf("#%d: encoded %x, want %x", i, buf.Bytes(), want)
		}
	}

	// Addresses with a wrong length or unknown network can't be written.
	bad := []*NetAddressV2{
		{Network: NetworkIPv4, Addr: make([]byte, 16)},
		{Network: NetworkTorV3, Addr: make([]byte, 10)},
		{Network: 0xaa, Addr: make([]byte, 4)},
	}
	for i, na := range bad {
		if err := writeNetAddressV2(io.Discard, ProtocolVersion, na); err == nil {
			t.Errorf("bad #%d: no error", i)
		}
	}
}

// TestDecodeTorV3 tests parsing of onion names, using the Bitcoin Cash Node
// test vectors.
func TestDecodeTorV3(t *testing.T) {
	const valid = "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion"
	pubKey, err := DecodeTorV3(valid)
	if err != nil {
		t.Fatal(err)
	}
	na := NetAddressV2{Network: NetworkTorV3, Addr: pubKey}
	if na.Host() != valid {
		t.Fatalf("got %s, want %s", na.Host(), valid)
	}
	if _, err := DecodeTorV3(strings.ToUpper(valid[:56]) + ".onion"); err != nil {
		t.Errorf("upper case: %v", err)
	}

	invalid := []string{
		// Wrong checksum.
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscsad.onion",
		// Wrong version.
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscrye.onion",
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd\x00wtf.onion",
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscry\n.onion",
		// Tor v2.
		"6hzph5hv6337r6p2.onion",
		"mfrggzak.onion",
		"mf*g zak.onion",
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd",
		"totally bogus",
		"",
	}
	for _, host := range invalid {
		if _, err := DecodeTorV3(host); err == nil {
			t.Errorf("DecodeTorV3(%q): no error", host)
		}
	}
}

// TestDecodeI2P tests that I2P names round trip.
func TestDecodeI2P(t *testing.T) {
	const valid = "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p"
	addr, err := DecodeI2P(valid)
	if err != nil {
		t.Fatal(err)
	}
	na := NetAddressV2{Network: NetworkI2P, Addr: addr}
	if na.Host() != valid {
		t.Fatalf("got %s, want %s", na.Host(), valid)
	}
	for _, host := range []string{"ukeu3k5o.b32.i2p", valid[:52] + ".onion",
		"ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkd*.b32.i2p"} {

		if _, err := DecodeI2P(host); err == nil {
			t.Errorf("DecodeI2P(%q): no error", host)
		}
	}
}

// TestNetAddressV2Legacy tests the conversions between NetAddress and
// NetAddressV2.
func TestNetAddressV2Legacy(t *testing.T) {
	ts := time.Unix(0x495fab29, 0)
	tests := []struct {
		ip      string
		network NetworkID
		addr    string
	}{
		{"1.2.3.4", NetworkIPv4, "01020304"},
		{"::ffff:1.2.3.4", NetworkIPv4, "01020304"},
		{"2001:db8::1", NetworkIPv6, "20010db8000000000000000000000001"},
		{"fd87:d87e:eb43::1", NetworkIPv6, "fd87d87eeb4300000000000000000001"},
	}
	for _, test := range tests {
		legacy := NewNetAddressTimestamp(ts, SFNodeNetwork,
			net.ParseIP(test.ip), 8333)
		na := NetAddressV2FromLegacy(legacy)
		if na.Network != test.network || hex.EncodeToString(na.Addr) != test.addr {
			t.Errorf("%s: got %v %x", test.ip, na.Network, na.Addr)
			continue
		}
		back := na.ToLegacy()
		if back == nil || !back.IP.Equal(legacy.IP) || back.Port != 8333 ||
			back.Services != SFNodeNetwork || !back.Timestamp.Equal(ts) {

			t.Errorf("%s: ToLegacy gave %+v", test.ip, back)
		}
	}

	for _, na := range []*NetAddressV2{
		{Network: NetworkTorV3, Addr: make([]byte, 32)},
		{Network: NetworkI2P, Addr: make([]byte, 32)},
		{Network: NetworkCJDNS, Addr: make([]byte, 16)},
		{Network: NetworkIPv4, Addr: make([]byte, 16)},
	} {
		if na.ToLegacy() != nil || na.IP() != nil {
			t.Errorf("%v address converted to legacy", na.Network)
		}
	}
}

// TestMsgAddrV2 tests encoding and decoding of addrv2 messages.
func TestMsgAddrV2(t *testing.T) {
	pver := ProtocolVersion
	msg := NewMsgAddrV2()
	if cmd := msg.Command(); cmd != "addrv2" {
		t.Errorf("wrong command %v", cmd)
	}
	if got, want := msg.MaxPayloadLength(pver), uint32(9+1000*531); got != want {
		t.Errorf("MaxPayloadLength: got %d, want %d", got, want)
	}

	torV3, _ := hex.DecodeString(
		"79bcc625184b05194975c28b66b66b0469f7f6556fb1ac3189a79b40dda32f1f")
	ts := time.Unix(0x495fab29, 0)
	addrs := []*NetAddressV2{
		NewNetAddressV2IPPort(net.ParseIP("1.2.3.4"), 8333, SFNodeNetwork),
		NewNetAddressV2IPPort(net.ParseIP("2001:db8::1"), 8333, 0),
		NewNetAddressV2(NetworkTorV3, torV3, 8333, SFNodeNetwork|SFNodeBloom),
		NewNetAddressV2(NetworkI2P, make([]byte, 32), 0, SFNodeNetwork),
		NewNetAddressV2(NetworkCJDNS, net.ParseIP("fc00::1"), 8333, SFNodeNetwork),
	}
	for _, na := range addrs {
		na.Timestamp = ts
	}
	msg.AddrList = addrs

	var buf bytes.Buffer
	if err := msg.BchEncode(&buf, pver, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	var got MsgAddrV2
	if err := got.BchDecode(bytes.NewReader(buf.Bytes()), pver, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	if len(got.AddrList) != len(addrs) {
		t.Fatalf("got %d addresses, want %d", len(got.AddrList), len(addrs))
	}
	for i, na := range got.AddrList {
		want := addrs[i]
		if na.Network != want.Network || !bytes.Equal(na.Addr, want.Addr) ||
			na.Port != want.Port || na.Services != want.Services ||
			!na.Timestamp.Equal(want.Timestamp) {

			t.Errorf("#%d: got %+v, want %+v", i, na, want)
		}
	}

	// Skipped entries are dropped while the following ones are still read.
	var mixed bytes.Buffer
	mixed.WriteByte(4)
	mixed.Write(addrV2Entry(t, "aa"+"04"+"01020304"))
	mixed.Write(addrV2Entry(t, "03"+"0a"+"f1f2f3f4f5f6f7f8f9fa"))
	mixed.Write(addrV2Entry(t, "02"+"10"+"00000000000000000000ffff01020304"))
	mixed.Write(addrV2Entry(t, "01"+"04"+"01020304"))
	got = MsgAddrV2{}
	if err := got.BchDecode(&mixed, pver, BaseEncoding); err != nil {
		t.Fatal(err)
	}
	if len(got.AddrList) != 1 || got.AddrList[0].Host() != "1.2.3.4" {
		t.Fatalf("got %+v, want only 1.2.3.4", got.AddrList)
	}

	// A malformed entry fails the message.
	var bad bytes.Buffer
	bad.WriteByte(2)
	bad.Write(addrV2Entry(t, "01"+"04"+"01020304"))
	bad.Write(addrV2Entry(t, "04"+"00"+""))
	var msgErr *MessageError
	if err := got.BchDecode(&bad, pver, BaseEncoding); !errors.As(err, &msgErr) {
		t.Errorf("malformed entry: got %v, want a MessageError", err)
	}

	// Too many addresses.
	var many bytes.Buffer
	_ = WriteVarInt(&many, pver, MaxV2AddrPerMsg+1)
	if err := got.BchDecode(&many, pver, BaseEncoding); !errors.As(err, &msgErr) {
		t.Errorf("decode too many: got %v, want a MessageError", err)
	}
	msg.AddrList = make([]*NetAddressV2, MaxV2AddrPerMsg+1)
	if err := msg.BchEncode(io.Discard, pver, BaseEncoding); !errors.As(err, &msgErr) {
		t.Errorf("encode too many: got %v, want a MessageError", err)
	}

	// A full message of the largest entries fits MaxPayloadLength, which
	// allows for a 9 byte count.
	var full bytes.Buffer
	_ = WriteVarInt(&full, pver, MaxV2AddrPerMsg)
	entry := addrV2Entry(t, "aa"+"fd0002"+strings.Repeat("00", 512))
	entry = append([]byte{0x29, 0xab, 0x5f, 0x49,
		0xff, 0, 0, 0, 0, 1, 0, 0, 0}, entry[5:]...)
	for range MaxV2AddrPerMsg {
		full.Write(entry)
	}
	if full.Len()+6 != int(msg.MaxPayloadLength(pver)) {
		t.Errorf("largest message is %d bytes, max payload %d",
			full.Len(), msg.MaxPayloadLength(pver))
	}
	got = MsgAddrV2{}
	if err := got.BchDecode(&full, pver, BaseEncoding); err != nil ||
		len(got.AddrList) != 0 {

		t.Errorf("largest message: %v, %d addresses", err, len(got.AddrList))
	}
}
