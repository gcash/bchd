// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/addrmgr"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/internal/socks"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchlog"
)

// newInboundTestServer sets the global config to allow maxPerIP inbound
// connections per host and maxPeers overall, restoring it when the test ends,
// and returns a server with just enough state to accept inbound connections.
func newInboundTestServer(t *testing.T, maxPerIP, maxPeers int) *server {
	oldCfg := cfg
	cfg = &config{MaxPeersPerIP: maxPerIP, MaxPeers: maxPeers}
	t.Cleanup(func() { cfg = oldCfg })
	return &server{
		chainParams:  &chaincfg.RegressionNetParams,
		addrManager:  addrmgr.New(t.TempDir(), nil),
		donePeers:    make(chan *serverPeer, 10),
		inboundHosts: make(map[string]int),
		inboundBans:  make(map[string]time.Time),
	}
}

// TestAdmitInbound ensures admitInbound enforces the per-IP and total limits
// counting connections that have not been released, that a release frees
// exactly one slot, and that bans refuse connections until they end.
func TestAdmitInbound(t *testing.T) {
	s := newInboundTestServer(t, 2, 3)
	releases := make(map[int]func())
	steps := []struct {
		op   string // admit, release, ban or unban
		arg  string // host for admit, ban and unban
		idx  int    // step whose release to call for release
		want bool   // whether admit succeeds
	}{
		{op: "admit", arg: "10.0.0.1", want: true},
		{op: "admit", arg: "10.0.0.1", want: true},
		{op: "admit", arg: "10.0.0.1", want: false}, // per-IP limit
		{op: "admit", arg: "10.0.0.2", want: true},
		{op: "admit", arg: "10.0.0.3", want: false}, // total limit
		{op: "release", idx: 0},
		{op: "release", idx: 0}, // double release is a no-op
		{op: "admit", arg: "10.0.0.3", want: true},
		{op: "admit", arg: "10.0.0.4", want: false}, // total limit
		{op: "release", idx: 1},
		{op: "ban", arg: "10.0.0.4"},
		{op: "admit", arg: "10.0.0.4", want: false}, // banned
		{op: "unban", arg: "10.0.0.4"},
		{op: "admit", arg: "10.0.0.4", want: true},
	}
	for i, step := range steps {
		switch step.op {
		case "admit":
			addr := &net.TCPAddr{IP: net.ParseIP(step.arg), Port: i}
			release := s.admitInbound(addr)
			if (release != nil) != step.want {
				t.Fatalf("step %d: admit %s got %v, want %v", i,
					step.arg, release != nil, step.want)
			}
			releases[i] = release
		case "release":
			releases[step.idx]()
		case "ban":
			s.inboundBans[step.arg] = time.Now().Add(time.Hour)
		case "unban":
			s.inboundBans[step.arg] = time.Now().Add(-time.Second)
		}
	}
}

// addrConn wraps a net.Conn to report a chosen remote address.
type addrConn struct {
	net.Conn
	remote net.Addr
}

func (c *addrConn) RemoteAddr() net.Addr { return c.remote }

// TestInboundPeerConnectedLimits ensures a refused inbound connection is
// closed before the handshake and that closing an admitted connection before
// the handshake completes frees its slot.
func TestInboundPeerConnectedLimits(t *testing.T) {
	s := newInboundTestServer(t, 1, 10)
	connect := func() net.Conn {
		client, srv := net.Pipe()
		addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 1}
		s.inboundPeerConnected(&addrConn{Conn: srv, remote: addr})
		return client
	}

	admitted := connect()
	refused := connect()
	_ = refused.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := refused.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("refused connection read: got %v, want EOF", err)
	}

	// Closing the admitted connection before the handshake must release
	// its slot once the peer has disconnected.  Waiting for the peer here
	// also keeps its goroutines from outliving the config swap.
	admitted.Close()
	select {
	case sp := <-s.donePeers:
		<-sp.quit
	case <-time.After(5 * time.Second):
		t.Fatal("admitted peer did not disconnect")
	}
	if release := s.admitInbound(&net.TCPAddr{
		IP: net.ParseIP("10.0.0.1")}); release == nil {
		t.Fatal("slot was not released on disconnect")
	}
}

// TestHandleBanPeerMsgMirrorsBan ensures banning a peer also refuses new
// inbound connections from its host when they are accepted.
func TestHandleBanPeerMsgMirrorsBan(t *testing.T) {
	s := newInboundTestServer(t, 5, 10)
	cfg.BanDuration = time.Hour

	// The ban is logged at the info level and the log rotator is not
	// initialized in tests.
	oldLevel := srvrLog.Level()
	srvrLog.SetLevel(bchlog.LevelOff)
	defer srvrLog.SetLevel(oldLevel)

	sp := newServerPeer(s, false)
	p, err := peer.NewOutboundPeer(newPeerConfig(sp), "10.0.0.1:8333")
	if err != nil {
		t.Fatalf("NewOutboundPeer: %v", err)
	}
	sp.Peer = p
	s.handleBanPeerMsg(&peerState{banned: make(map[string]time.Time)}, sp)

	addr := &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 1}
	if s.admitInbound(addr) != nil {
		t.Fatal("inbound connection from banned host was admitted")
	}
}

// TestDonePeerFreesDirectRelaySlot ensures a peer holding one of the
// maxDirectRelayPeers direct relay slots frees it when it disconnects.  The
// slot was freed only for a peer missing from its list, so once that many
// such peers had disconnected, no peer was asked to relay blocks directly
// again.
func TestDonePeerFreesDirectRelaySlot(t *testing.T) {
	useTestConfig(t, &config{})
	s := &server{}
	sp := newServerPeer(s, false)
	sp.Peer = peer.NewInboundPeer(&peer.Config{})
	state := &peerState{
		inboundPeers:     map[int32]*serverPeer{sp.ID(): sp},
		outboundPeers:    make(map[int32]*serverPeer),
		persistentPeers:  make(map[int32]*serverPeer),
		directRelayPeers: map[int32]*serverPeer{sp.ID(): sp},
		connectionCount:  make(map[string]int),
	}

	s.handleDonePeerMsg(state, sp)
	if _, ok := state.inboundPeers[sp.ID()]; ok {
		t.Fatal("disconnected peer is still an inbound peer")
	}
	if _, ok := state.directRelayPeers[sp.ID()]; ok {
		t.Fatal("disconnected peer still holds a direct relay slot")
	}
}

// TestIsWhitelistedProxiedAddr checks that a peer reached through a proxy is
// whitelisted by its own address rather than the proxy's, so whitelisting a
// local Tor proxy's address doesn't whitelist every peer reached through it.
func TestIsWhitelistedProxiedAddr(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })

	parseCIDR := func(s string) *net.IPNet {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatal(err)
		}
		return ipnet
	}
	loopback := parseCIDR("127.0.0.0/8")

	tests := []struct {
		host  string
		peers *net.IPNet
	}{
		{"192.0.2.1", parseCIDR("192.0.2.0/24")},
		{"2001:db8::1", parseCIDR("2001:db8::/32")},
	}
	for _, test := range tests {
		addr := &socks.ProxiedAddr{Net: "tcp", Host: test.host, Port: 8333}

		cfg = &config{whitelists: []*net.IPNet{loopback}}
		if isWhitelisted(addr) {
			t.Fatalf("%s whitelisted by the proxy's network", addr)
		}

		cfg = &config{whitelists: []*net.IPNet{test.peers}}
		if !isWhitelisted(addr) {
			t.Fatalf("%s not whitelisted by its own network", addr)
		}
	}
}
