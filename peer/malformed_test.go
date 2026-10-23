// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package peer_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// rawMessage returns the wire encoding of a message on network bchnet with
// the passed command, payload length and checksum, followed by the payload.
func rawMessage(bchnet wire.BitcoinNet, command string, length uint32,
	checksum []byte, payload []byte) []byte {

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint32(bchnet))
	var cmd [wire.CommandSize]byte
	copy(cmd[:], command)
	buf.Write(cmd[:])
	_ = binary.Write(&buf, binary.LittleEndian, length)
	buf.Write(checksum[:4])
	buf.Write(payload)
	return buf.Bytes()
}

// validMessage returns the wire encoding of a message on network bchnet with
// the passed command and payload, whose header and checksum are right.
func validMessage(bchnet wire.BitcoinNet, command string, payload []byte) []byte {
	return rawMessage(bchnet, command, uint32(len(payload)),
		chainhash.DoubleHashB(payload), payload)
}

// handshakePeer returns an inbound peer on params whose handshake with the
// test, which plays the remote peer from remoteAddr over a pipe, completed,
// the remote end of the pipe, and the messages the peer sends from then on,
// a channel closed once its connection closes.
func handshakePeer(t *testing.T, params *chaincfg.Params,
	remoteAddr string) (*peer.Peer, *conn, <-chan wire.Message) {

	t.Helper()
	localConn, remoteConn := pipe(
		&conn{laddr: "10.0.0.1:8333", raddr: remoteAddr},
		&conn{laddr: remoteAddr, raddr: "10.0.0.1:8333"},
	)
	p := peer.NewInboundPeer(&peer.Config{
		ChainParams:            params,
		TstAllowSelfConnection: true,
	})
	p.AssociateConnection(localConn)
	t.Cleanup(p.Disconnect)

	na := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.2"), 8333, 0)
	version := wire.NewMsgVersion(na, na, 1, 0)
	err := wire.WriteMessage(remoteConn, version, wire.ProtocolVersion,
		params.Net)
	if err != nil {
		t.Fatal(err)
	}
	msgs := make(chan wire.Message, 10)
	go func() {
		defer close(msgs)
		for {
			msg, _, err := wire.ReadMessage(remoteConn,
				wire.ProtocolVersion, params.Net)
			if err != nil {
				return
			}
			msgs <- msg
		}
	}()
	for _, want := range []string{wire.CmdVersion, wire.CmdVerAck} {
		select {
		case msg := <-msgs:
			if msg == nil || msg.Command() != want {
				t.Fatalf("got %v in the handshake, want %s", msg, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no %s in the handshake", want)
		}
	}
	err = wire.WriteMessage(remoteConn, wire.NewMsgVerAck(),
		wire.ProtocolVersion, params.Net)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !p.VerAckReceived() {
		if time.Now().After(deadline) {
			t.Fatal("the peer did not get the verack")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return p, remoteConn, msgs
}

// describe returns msg's command, or for a reject its fields, quoted.
func describe(msg wire.Message) string {
	if msg == nil {
		return "nothing"
	}
	if reject, ok := msg.(*wire.MsgReject); ok {
		return fmt.Sprintf("reject %q, %v, %q", reject.Cmd, reject.Code,
			reject.Reason)
	}
	return msg.Command()
}

// sendRaw writes raw to the peer from the remote end.
func sendRaw(t *testing.T, remote *conn, raw []byte) {
	t.Helper()
	if _, err := remote.Write(raw); err != nil {
		t.Fatal(err)
	}
}

// nextMessage returns the next message the peer sends, or nil once it
// disconnects.
func nextMessage(t *testing.T, msgs <-chan wire.Message) wire.Message {
	t.Helper()
	select {
	case msg := <-msgs:
		return msg
	case <-time.After(5 * time.Second):
		t.Fatal("the peer sent nothing and did not disconnect")
		return nil
	}
}

// invOverLimit is the payload of an inv that claims one entry more than an
// inv may hold, which its decoder refuses.
var invOverLimit = []byte{0xfd, 0x51, 0xc3}

// undecodable are messages whose header, length and checksum are valid, but
// whose payload can't be decoded.
var undecodable = []struct {
	name    string
	command string
	payload []byte
}{
	// A ping without its nonce, which the decoder reads as the end of the
	// payload.
	{"empty ping", wire.CmdPing, nil},
	// A transaction cut off in its version.
	{"short tx", wire.CmdTx, []byte{1}},
	// An inv claiming more entries than an inv may hold.
	{"inv over its count limit", wire.CmdInv, invOverLimit},
}

// TestUndecodableMessageReject ensures a message whose payload can't be
// decoded gets BCHN's reject for it: the message's own command, the malformed
// code and "error parsing message", with a zero hash for a transaction, whose
// hash isn't known.  bchd sent the command "malformed" and its own error text,
// and none at all when the decoder ran out of payload at a field's start.
func TestUndecodableMessageReject(t *testing.T) {
	params := &chaincfg.MainNetParams
	for _, test := range undecodable {
		_, remote, msgs := handshakePeer(t, params, "10.0.0.2:8333")
		sendRaw(t, remote, validMessage(params.Net, test.command,
			test.payload))
		msg := nextMessage(t, msgs)
		reject, ok := msg.(*wire.MsgReject)
		if !ok {
			t.Errorf("%s: got %s, want a reject", test.name, describe(msg))
			continue
		}
		if reject.Cmd != test.command || reject.Code != wire.RejectMalformed ||
			reject.Reason != "error parsing message" ||
			reject.Hash != (chainhash.Hash{}) {

			t.Errorf("%s: reject %q, %v, %q, %v, want %q, %v, "+
				"\"error parsing message\" and no hash", test.name,
				reject.Cmd, reject.Code, reject.Reason, reject.Hash,
				test.command, wire.RejectMalformed)
		}
		if msg := nextMessage(t, msgs); msg != nil {
			t.Errorf("%s: got %s after the reject, want a disconnect",
				test.name, describe(msg))
		}
	}
}

// TestReadErrorsSendNoReject ensures a message refused before its payload is
// decoded gets no reject, as BCHN sends none for a bad checksum or another
// network's message, which it answers by discouraging and disconnecting the
// peer, for a header claiming too large a payload, which disconnects it, and
// for an invalid command, nor for a payload cut short by the connection's end;
// the peer is disconnected.  bchd sent a reject with the command "malformed"
// and its own error text.
func TestReadErrorsSendNoReject(t *testing.T) {
	params := &chaincfg.MainNetParams
	ping := make([]byte, 8)
	badCommand := validMessage(params.Net, "bogus", nil)
	badCommand[4] = 0x81
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"bad checksum", rawMessage(params.Net, wire.CmdPing, 8,
			[]byte{1, 2, 3, 4}, ping)},
		{"other network", validMessage(wire.TestNet3, wire.CmdPing, ping)},
		{"payload over the limit", rawMessage(params.Net, wire.CmdPing,
			^uint32(0), []byte{0, 0, 0, 0}, nil)},
		{"payload over the command's limit", validMessage(params.Net,
			wire.CmdVerAck, []byte{0})},
		{"invalid command", badCommand},
	} {
		_, remote, msgs := handshakePeer(t, params, "10.0.0.2:8333")
		sendRaw(t, remote, test.raw)
		if msg := nextMessage(t, msgs); msg != nil {
			t.Errorf("%s: got %s, want a disconnect without a reject",
				test.name, describe(msg))
		}
	}

	// The remote end stops sending halfway through a payload.
	_, remote, msgs := handshakePeer(t, params, "10.0.0.2:8333")
	sendRaw(t, remote, validMessage(params.Net, wire.CmdPing, ping)[:28])
	if err := remote.Close(); err != nil {
		t.Fatal(err)
	}
	if msg := nextMessage(t, msgs); msg != nil {
		t.Errorf("payload cut short: got %s, want a disconnect without a "+
			"reject", describe(msg))
	}
}

// TestUnreadableMessagesKept ensures an unknown command, and on the regression
// test network a malformed message from the local host, are skipped without a
// disconnect, as before: the peer still answers a ping after them.
func TestUnreadableMessagesKept(t *testing.T) {
	expectPong := func(remote *conn, msgs <-chan wire.Message,
		bchnet wire.BitcoinNet) {

		t.Helper()
		err := wire.WriteMessage(remote, wire.NewMsgPing(7),
			wire.ProtocolVersion, bchnet)
		if err != nil {
			t.Fatal(err)
		}
		msg := nextMessage(t, msgs)
		if pong, ok := msg.(*wire.MsgPong); !ok || pong.Nonce != 7 {
			t.Fatalf("got %s after the skipped message, want the pong",
				describe(msg))
		}
	}

	mainNet := &chaincfg.MainNetParams
	_, remote, msgs := handshakePeer(t, mainNet, "10.0.0.2:8333")
	sendRaw(t, remote, validMessage(mainNet.Net, "bogus", []byte{1}))
	expectPong(remote, msgs, mainNet.Net)

	regTest := &chaincfg.RegressionNetParams
	_, remote, msgs = handshakePeer(t, regTest, "127.0.0.1:18444")
	sendRaw(t, remote, validMessage(regTest.Net, wire.CmdInv, invOverLimit))
	expectPong(remote, msgs, regTest.Net)
}
