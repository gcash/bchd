// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package peer_test

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// unknownMsg is a message with a command the wire package doesn't know.
type unknownMsg struct{}

func (m *unknownMsg) BchDecode(io.Reader, uint32, wire.MessageEncoding) error { return nil }
func (m *unknownMsg) BchEncode(io.Writer, uint32, wire.MessageEncoding) error { return nil }
func (m *unknownMsg) Command() string                                         { return "wtxidrelay" }
func (m *unknownMsg) MaxPayloadLength(uint32) uint32                          { return 0 }

// rawRemote is the remote end of a connection to a peer, driven by the test
// with raw wire messages.
type rawRemote struct {
	t    *testing.T
	conn *conn
	msgs chan wire.Message
}

func (r *rawRemote) write(msg wire.Message) {
	r.t.Helper()
	err := wire.WriteMessage(r.conn, msg, wire.ProtocolVersion, wire.MainNet)
	if err != nil {
		r.t.Fatalf("write %s: %v", msg.Command(), err)
	}
}

// next returns the next message the peer sent.
func (r *rawRemote) next() wire.Message {
	r.t.Helper()
	select {
	case msg, ok := <-r.msgs:
		if !ok {
			r.t.Fatal("connection closed")
		}
		return msg
	case <-time.After(time.Second):
		r.t.Fatal("timeout waiting for a message")
	}
	return nil
}

func (r *rawRemote) writeVersion(pver uint32) {
	r.t.Helper()
	na := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.2"), 8333,
		wire.SFNodeNetwork)
	msg := wire.NewMsgVersion(na, na, 1, 0)
	msg.ProtocolVersion = int32(pver)
	msg.Services = wire.SFNodeNetwork
	r.write(msg)
}

// addrV2Peer holds a peer under test and what its listeners saw.
type addrV2Peer struct {
	*peer.Peer
	verack chan struct{}
	addrV2 chan *wire.MsgAddrV2
}

// newAddrV2Peer connects a new peer to a raw remote.  The remote reads the
// peer's version message for an outbound peer and writes its own version
// first for an inbound one.
func newAddrV2Peer(t *testing.T, inbound, handleAddrV2 bool,
	remotePver uint32) (*addrV2Peer, *rawRemote) {

	p := &addrV2Peer{
		verack: make(chan struct{}, 1),
		addrV2: make(chan *wire.MsgAddrV2, 1),
	}
	cfg := &peer.Config{
		Listeners: peer.MessageListeners{
			OnVerAck: func(*peer.Peer, *wire.MsgVerAck) {
				p.verack <- struct{}{}
			},
		},
		UserAgentName:          "peer",
		UserAgentVersion:       "1.0",
		ChainParams:            &chaincfg.MainNetParams,
		TstAllowSelfConnection: true,
	}
	if handleAddrV2 {
		cfg.Listeners.OnAddrV2 = func(_ *peer.Peer, msg *wire.MsgAddrV2) {
			p.addrV2 <- msg
		}
	}

	localConn, remoteConn := pipe(
		&conn{laddr: "10.0.0.1:8333", raddr: "10.0.0.2:8333"},
		&conn{laddr: "10.0.0.2:8333", raddr: "10.0.0.1:8333"},
	)
	r := &rawRemote{t: t, conn: remoteConn, msgs: make(chan wire.Message, 10)}
	go func() {
		defer close(r.msgs)
		for {
			msg, _, err := wire.ReadMessage(remoteConn,
				wire.ProtocolVersion, wire.MainNet)
			if err != nil {
				return
			}
			r.msgs <- msg
		}
	}()
	t.Cleanup(func() {
		p.Disconnect()
		remoteConn.Close()
	})

	if inbound {
		p.Peer = peer.NewInboundPeer(cfg)
		p.AssociateConnection(localConn)
		r.writeVersion(remotePver)
	} else {
		var err error
		p.Peer, err = peer.NewOutboundPeer(cfg, "10.0.0.2:8333")
		if err != nil {
			t.Fatal(err)
		}
		p.AssociateConnection(localConn)
		if _, ok := r.next().(*wire.MsgVersion); !ok {
			t.Fatal("first message is not version")
		}
		r.writeVersion(remotePver)
	}
	if inbound {
		if _, ok := r.next().(*wire.MsgVersion); !ok {
			t.Fatal("first message is not version")
		}
	}
	return p, r
}

func (p *addrV2Peer) waitVerAck(t *testing.T) {
	t.Helper()
	select {
	case <-p.verack:
	case <-time.After(time.Second):
		t.Fatal("verack timeout")
	}
}

// TestSendAddrV2Handshake checks that sendaddrv2 is sent between version and
// verack to peers with protocol version 70016 or higher, that a sendaddrv2
// from the remote peer before verack is recorded, and that other messages
// before verack are ignored.
func TestSendAddrV2Handshake(t *testing.T) {
	for _, inbound := range []bool{false, true} {
		p, r := newAddrV2Peer(t, inbound, true, wire.AddrV2Version)
		r.write(&unknownMsg{})
		r.write(wire.NewMsgPing(1))
		r.write(wire.NewMsgSendAddrV2())
		r.write(&unknownMsg{})
		r.write(wire.NewMsgVerAck())
		p.waitVerAck(t)

		if _, ok := r.next().(*wire.MsgSendAddrV2); !ok {
			t.Fatalf("inbound=%v: sendaddrv2 not sent before verack",
				inbound)
		}
		if _, ok := r.next().(*wire.MsgVerAck); !ok {
			t.Fatalf("inbound=%v: verack not sent after sendaddrv2",
				inbound)
		}
		if !p.WantsAddrV2() {
			t.Fatalf("inbound=%v: sendaddrv2 not recorded", inbound)
		}

		// addrv2 messages reach the listener.
		torV3, err := wire.DecodeTorV3("pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2" +
			"ymmju6nubxndf4pscryd.onion")
		if err != nil {
			t.Fatal(err)
		}
		msg := wire.NewMsgAddrV2()
		msg.AddrList = append(msg.AddrList, wire.NewNetAddressV2(
			wire.NetworkTorV3, torV3, 8333, wire.SFNodeNetwork))
		r.write(msg)
		select {
		case got := <-p.addrV2:
			if len(got.AddrList) != 1 ||
				got.AddrList[0].Network != wire.NetworkTorV3 {

				t.Fatalf("inbound=%v: got %v", inbound, got.AddrList)
			}
		case <-time.After(time.Second):
			t.Fatalf("inbound=%v: addrv2 not received", inbound)
		}

		// PushAddrV2Msg sends at most MaxV2AddrPerMsg addresses.
		addrs := make([]*wire.NetAddressV2, wire.MaxV2AddrPerMsg+10)
		for i := range addrs {
			addrs[i] = wire.NewNetAddressV2IPPort(
				net.IPv4(1, 2, byte(i>>8), byte(i)), 8333, 0)
		}
		sent, err := p.PushAddrV2Msg(addrs)
		if err != nil || len(sent) != wire.MaxV2AddrPerMsg {
			t.Fatalf("PushAddrV2Msg: %d sent, err %v", len(sent), err)
		}
		for {
			if got, ok := r.next().(*wire.MsgAddrV2); ok {
				if len(got.AddrList) != wire.MaxV2AddrPerMsg {
					t.Fatalf("addrv2 with %d addresses",
						len(got.AddrList))
				}
				break
			}
		}
	}
}

// TestNoSendAddrV2 checks that sendaddrv2 is not sent to peers below protocol
// version 70016 or when addrv2 messages aren't handled, and that a peer that
// doesn't send sendaddrv2 gets no addrv2 messages.
func TestNoSendAddrV2(t *testing.T) {
	tests := []struct {
		pver         uint32
		handleAddrV2 bool
	}{
		{wire.AddrV2Version - 1, true},
		{wire.AddrV2Version, false},
	}
	for _, test := range tests {
		for _, inbound := range []bool{false, true} {
			p, r := newAddrV2Peer(t, inbound, test.handleAddrV2, test.pver)
			r.write(wire.NewMsgVerAck())
			p.waitVerAck(t)
			if msg := r.next(); msg.Command() != wire.CmdVerAck {
				t.Fatalf("%+v inbound=%v: sent %s, want verack",
					test, inbound, msg.Command())
			}
			if p.WantsAddrV2() {
				t.Fatalf("%+v inbound=%v: WantsAddrV2", test, inbound)
			}
		}
	}
}

// TestSendAddrV2AfterVerAck checks that a peer sending sendaddrv2 after verack
// is disconnected.
func TestSendAddrV2AfterVerAck(t *testing.T) {
	p, r := newAddrV2Peer(t, false, true, wire.AddrV2Version)
	r.write(wire.NewMsgVerAck())
	p.waitVerAck(t)
	r.write(wire.NewMsgSendAddrV2())

	disconnected := make(chan struct{})
	go func() {
		p.WaitForDisconnect()
		close(disconnected)
	}()
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("peer not disconnected")
	}
}
