// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/addrmgr"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
)

// connectAddrV2Peer connects an inbound server peer to a raw remote end over
// TCP and completes the handshake, with the remote sending sendaddrv2 when
// wantsAddrV2 is set.
func connectAddrV2Peer(t *testing.T, s *server, wantsAddrV2 bool) (*serverPeer, net.Conn) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	remote, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	local, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { remote.Close() })

	sp := newServerPeer(s, false)
	sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams:            &chaincfg.MainNetParams,
		TstAllowSelfConnection: true,
		Listeners:              peer.MessageListeners{OnAddrV2: sp.OnAddrV2},
	})
	sp.AssociateConnection(local)
	t.Cleanup(sp.Disconnect)

	write := func(msg wire.Message) {
		t.Helper()
		err := wire.WriteMessage(remote, msg, wire.ProtocolVersion, wire.MainNet)
		if err != nil {
			t.Fatal(err)
		}
	}
	na := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.2"), 8333, 0)
	version := wire.NewMsgVersion(na, na, 1, 0)
	version.Services = wire.SFNodeNetwork
	write(version)
	if wantsAddrV2 {
		write(wire.NewMsgSendAddrV2())
	}
	write(wire.NewMsgVerAck())

	// The peer sends version, sendaddrv2 and verack.
	_ = remote.SetReadDeadline(time.Now().Add(5 * time.Second))
	for range 3 {
		if _, _, err := wire.ReadMessage(remote, wire.ProtocolVersion,
			wire.MainNet); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for !sp.VerAckReceived() {
		if time.Now().After(deadline) {
			t.Fatal("handshake timeout")
		}
		time.Sleep(time.Millisecond)
	}
	return sp, remote
}

// TestPushAddrMsgAddrV2 checks that addresses are gossiped with addrv2 to peers
// that sent sendaddrv2 and with addr to others, leaving out the addresses addr
// can't carry, and that addrv2 messages reach the address manager.
func TestPushAddrMsgAddrV2(t *testing.T) {
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold})
	s := &server{addrManager: addrmgr.New(t.TempDir(), nil)}

	torV3, err := wire.DecodeTorV3(
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion")
	if err != nil {
		t.Fatal(err)
	}
	addrs := []*wire.NetAddressV2{
		wire.NewNetAddressV2IPPort(net.ParseIP("1.2.3.4"), 8333,
			wire.SFNodeNetwork),
		wire.NewNetAddressV2(wire.NetworkTorV3, torV3, 8333,
			wire.SFNodeNetwork),
	}

	for _, wantsAddrV2 := range []bool{true, false} {
		sp, remote := connectAddrV2Peer(t, s, wantsAddrV2)
		if sp.WantsAddrV2() != wantsAddrV2 {
			t.Fatalf("WantsAddrV2 = %v", sp.WantsAddrV2())
		}
		sp.pushAddrMsg(addrs)

		msg, _, err := wire.ReadMessage(remote, wire.ProtocolVersion,
			wire.MainNet)
		if err != nil {
			t.Fatal(err)
		}
		switch msg := msg.(type) {
		case *wire.MsgAddrV2:
			if !wantsAddrV2 || len(msg.AddrList) != 2 {
				t.Fatalf("wantsAddrV2=%v: got addrv2 with %d "+
					"addresses", wantsAddrV2, len(msg.AddrList))
			}
		case *wire.MsgAddr:
			if wantsAddrV2 || len(msg.AddrList) != 1 ||
				!msg.AddrList[0].IP.Equal(net.ParseIP("1.2.3.4")) {

				t.Fatalf("wantsAddrV2=%v: got addr %v", wantsAddrV2,
					msg.AddrList)
			}
		default:
			t.Fatalf("got %s", msg.Command())
		}

		// Known addresses aren't sent again.
		for _, na := range addrs[:1] {
			if !sp.addressKnown(na) {
				t.Fatalf("%s not known", addrmgr.NetAddressKey(na))
			}
		}
	}

	// A received addrv2 message stores the Tor v3 address.
	sp, remote := connectAddrV2Peer(t, s, true)
	msg := wire.NewMsgAddrV2()
	msg.AddrList = addrs[1:]
	if err := wire.WriteMessage(remote, msg, wire.ProtocolVersion,
		wire.MainNet); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s.addrManager.NumAddresses() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if s.addrManager.NumAddresses() != 1 || !sp.addressKnown(addrs[1]) {
		t.Fatalf("addrv2 address not stored: %d addresses",
			s.addrManager.NumAddresses())
	}
}

// TestOnionAddrString checks that addnode accepts Tor v3 onion addresses when
// a proxy is set and returns an error otherwise.
func TestOnionAddrString(t *testing.T) {
	const onion = "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion:8333"
	tests := []struct {
		cfg  config
		addr string
		ok   bool
	}{
		{config{Proxy: "127.0.0.1:9050"}, onion, true},
		{config{OnionProxy: "127.0.0.1:9050"}, onion, true},
		{config{}, onion, false},
		{config{Proxy: "127.0.0.1:9050", NoOnion: true}, onion, false},
		{config{Proxy: "127.0.0.1:9050"}, "aaaaaaaaaaaaaaaa.onion:8333", false},
		{config{Proxy: "127.0.0.1:9050"},
			"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscsad.onion:8333",
			false},
	}
	for i, test := range tests {
		useTestConfig(t, &test.cfg)
		netAddr, err := addrStringToNetAddr(test.addr)
		if test.ok != (err == nil) {
			t.Errorf("#%d: got err %v, want ok %v", i, err, test.ok)
			continue
		}
		if test.ok && netAddr.String() != test.addr {
			t.Errorf("#%d: got %s", i, netAddr)
		}
	}
}

// TestPermanentPeerAddrs checks that --connect and --addpeer skip onion
// addresses that can't be connected to instead of failing, while --noonion
// with an onion address still fails.
func TestPermanentPeerAddrs(t *testing.T) {
	const onion = "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion:8333"
	addrs := []string{
		"1.2.3.4:8333",
		"aaaaaaaaaaaaaaaa.onion:8333",
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscsad.onion:8333",
		onion,
	}

	useTestConfig(t, &config{Proxy: "127.0.0.1:9050"})
	got, err := permanentPeerAddrs(addrs)
	if err != nil || len(got) != 2 || got[0].String() != "1.2.3.4:8333" ||
		got[1].String() != onion {

		t.Fatalf("with a proxy: got %v, %v", got, err)
	}

	useTestConfig(t, &config{})
	got, err = permanentPeerAddrs(addrs)
	if err != nil || len(got) != 1 || got[0].String() != "1.2.3.4:8333" {
		t.Fatalf("without a proxy: got %v, %v", got, err)
	}

	useTestConfig(t, &config{Proxy: "127.0.0.1:9050", NoOnion: true})
	if _, err := permanentPeerAddrs(addrs); err == nil {
		t.Fatal("--noonion with an onion address did not fail")
	}
}

// TestCanDial checks that outbound connections are only made to the networks
// we can reach.
func TestCanDial(t *testing.T) {
	torV3 := wire.NewNetAddressV2(wire.NetworkTorV3, make([]byte, 32), 8333, 0)
	i2p := wire.NewNetAddressV2(wire.NetworkI2P, make([]byte, 32), 0, 0)
	cjdns := wire.NewNetAddressV2(wire.NetworkCJDNS, net.ParseIP("fc00::1"),
		8333, 0)
	ipv4 := wire.NewNetAddressV2IPPort(net.ParseIP("1.2.3.4"), 8333, 0)

	useTestConfig(t, &config{})
	if !canDial(ipv4) || canDial(torV3) || canDial(i2p) || canDial(cjdns) {
		t.Fatal("without a proxy only IP addresses can be dialed")
	}
	useTestConfig(t, &config{OnionProxy: "127.0.0.1:9050"})
	if !canDial(torV3) || canDial(i2p) || canDial(cjdns) {
		t.Fatal("with an onion proxy Tor addresses can be dialed")
	}
	useTestConfig(t, &config{Proxy: "127.0.0.1:9050", NoOnion: true})
	if canDial(torV3) {
		t.Fatal("--noonion must stop Tor connections")
	}
}
