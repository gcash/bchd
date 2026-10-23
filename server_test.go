// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/addrmgr"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/connmgr"
	"github.com/gcash/bchd/internal/socks"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/netsync"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
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

// newTestPeerState returns the empty state of a peer handler.
func newTestPeerState() *peerState {
	return &peerState{
		inboundPeers:     make(map[int32]*serverPeer),
		outboundPeers:    make(map[int32]*serverPeer),
		persistentPeers:  make(map[int32]*serverPeer),
		directRelayPeers: make(map[int32]*serverPeer),
		banned:           make(map[string]time.Time),
		outboundGroups:   make(map[string]int),
		connectionCount:  make(map[string]int),
	}
}

// newOutboundTestPeer returns an outbound peer of s to addr whose handshake
// with a peer on the other end of a pipe has completed.
func newOutboundTestPeer(t *testing.T, s *server, addr string) *serverPeer {
	t.Helper()
	sp := newServerPeer(s, false)
	p, err := peer.NewOutboundPeer(&peer.Config{
		ChainParams:            s.chainParams,
		TstAllowSelfConnection: true,
	}, addr)
	if err != nil {
		t.Fatal(err)
	}
	sp.Peer = p
	remote := peer.NewInboundPeer(&peer.Config{
		ChainParams:            s.chainParams,
		TstAllowSelfConnection: true,
	})
	local, remoteConn := net.Pipe()
	sp.AssociateConnection(local)
	remote.AssociateConnection(&addrConn{Conn: remoteConn,
		remote: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 18444}})
	t.Cleanup(func() {
		sp.Disconnect()
		remote.Disconnect()
		sp.WaitForDisconnect()
	})
	deadline := time.Now().Add(5 * time.Second)
	for !sp.VerAckReceived() || !remote.VerAckReceived() {
		if time.Now().After(deadline) {
			t.Fatal("handshake did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return sp
}

// relaySignal passes the sync manager's reports to the server, signaling
// each relay before the server takes it.
type relaySignal struct {
	*server
	relaying chan struct{}
}

// RelayInventory signals the relay and passes it to the server.
func (n relaySignal) RelayInventory(invVect *wire.InvVect, data interface{}) {
	n.relaying <- struct{}{}
	n.server.RelayInventory(invVect, data)
}

// TestAddPeerWhileSyncManagerRelays ensures the peer handler adds an outbound
// peer while the sync manager waits for it to take a block relay, as the
// relay channel is full.  The peer handler used to ask the sync manager
// whether this node is current, and to wait for room in its message queue to
// tell it about the peer, so neither could go on.
func TestAddPeerWhileSyncManagerRelays(t *testing.T) {
	t.Run("asks whether current", func(t *testing.T) {
		testAddPeerWhileSyncManagerRelays(t, false)
	})
	t.Run("message queue full", func(t *testing.T) {
		testAddPeerWhileSyncManagerRelays(t, true)
	})
}

func testAddPeerWhileSyncManagerRelays(t *testing.T, fillQueue bool) {
	const maxPeers = 8
	useTestConfig(t, &config{MaxPeers: maxPeers, MaxPeersPerIP: 5,
		BanThreshold: defaultBanThreshold})
	oldSyncLevel, oldTxmpLevel := syncLog.Level(), txmpLog.Level()
	syncLog.SetLevel(bchlog.LevelOff)
	txmpLog.SetLevel(bchlog.LevelOff)
	t.Cleanup(func() {
		syncLog.SetLevel(oldSyncLevel)
		txmpLog.SetLevel(oldTxmpLevel)
	})
	params := &chaincfg.RegressionNetParams
	chain, _ := newTestChainDB(t)
	// A recent tip makes the chain current, so the sync manager relays the
	// blocks it accepts.
	tip := solvedBlock(t, chain, 0, nil)
	if _, _, err := chain.ProcessBlock(tip, blockchain.BFNone); err != nil {
		t.Fatal(err)
	}
	s := &server{
		chainParams: params,
		chain:       chain,
		addrManager: addrmgr.New(t.TempDir(), nil),
		relayInv:    make(chan relayMsg, 1),
		quit:        make(chan struct{}),
	}
	notifier := relaySignal{server: s, relaying: make(chan struct{}, 1)}
	sm, err := netsync.New(&netsync.Config{
		PeerNotifier: notifier,
		Chain:        chain,
		TxMemPool:    mempool.New(&mempool.Config{ChainParams: params}),
		ChainParams:  params,
		MaxPeers:     maxPeers,
	})
	if err != nil {
		t.Fatal(err)
	}
	s.syncManager = sm
	sm.Start()
	t.Cleanup(func() {
		close(s.quit)
		_ = sm.Stop()
	})

	// The sync manager accepts a block and waits to relay it.
	s.relayInv <- relayMsg{invVect: wire.NewInvVect(wire.InvTypeTx,
		&chainhash.Hash{1})}
	block := solvedBlock(t, chain, 0, nil)
	go func() { _, _ = sm.ProcessBlock(block, blockchain.BFNone) }()
	select {
	case <-notifier.relaying:
	case <-time.After(5 * time.Second):
		t.Fatal("the sync manager did not relay the block")
	}

	sp := newOutboundTestPeer(t, s, "10.0.0.2:18444")
	if fillQueue {
		// The sync manager queues three messages per peer.
		filled := make(chan struct{})
		go func() {
			for range 3 * maxPeers {
				sm.QueueInv(wire.NewMsgInv(), sp.Peer)
			}
			close(filled)
		}()
		select {
		case <-filled:
		case <-time.After(5 * time.Second):
			t.Fatal("the sync manager queues fewer messages")
		}
	}
	added := make(chan bool, 1)
	go func() { added <- s.handleAddPeerMsg(newTestPeerState(), sp) }()
	select {
	case ok := <-added:
		if !ok {
			t.Fatal("the outbound peer was not added")
		}
	case <-time.After(5 * time.Second):
		// Take both relays, so the sync manager and the peer handler
		// can go on and the test can end.
		<-s.relayInv
		<-s.relayInv
		<-added
		t.Fatal("the peer handler waited for the sync manager, which " +
			"waits for the peer handler")
	}
}

// TestRelayInventoryEndsOnQuit ensures a relay waiting for room in the full
// relay channel ends when the server quits, so the sync manager the peer
// handler stops on its way out cannot be left waiting to relay.
func TestRelayInventoryEndsOnQuit(t *testing.T) {
	s := &server{relayInv: make(chan relayMsg, 1), quit: make(chan struct{})}
	s.relayInv <- relayMsg{}
	done := make(chan struct{})
	go func() {
		s.RelayInventory(wire.NewInvVect(wire.InvTypeTx, &chainhash.Hash{1}),
			nil)
		close(done)
	}()
	close(s.quit)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		<-s.relayInv
		<-done
		t.Fatal("the relay still waits after the server quit")
	}
}

// newInboundTestPeer returns an inbound peer of s from host whose handshake
// with a peer on the other end of a pipe has completed.
func newInboundTestPeer(t *testing.T, s *server, host string) *serverPeer {
	t.Helper()
	sp := newServerPeer(s, false)
	sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams:            s.chainParams,
		TstAllowSelfConnection: true,
	})
	remote, err := peer.NewOutboundPeer(&peer.Config{
		ChainParams:            s.chainParams,
		TstAllowSelfConnection: true,
	}, "10.0.0.1:18444")
	if err != nil {
		t.Fatal(err)
	}
	local, remoteConn := net.Pipe()
	sp.AssociateConnection(&addrConn{Conn: local,
		remote: &net.TCPAddr{IP: net.ParseIP(host), Port: 18444}})
	remote.AssociateConnection(remoteConn)
	t.Cleanup(func() {
		sp.Disconnect()
		remote.Disconnect()
		sp.WaitForDisconnect()
	})
	deadline := time.Now().Add(5 * time.Second)
	for !sp.VerAckReceived() || !remote.VerAckReceived() {
		if time.Now().After(deadline) {
			t.Fatal("handshake did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return sp
}

// newAddPeerTestServer sets the global config to allow maxPeers peers, of
// which targetOutbound outbound, and returns a server with a running sync
// manager and connection manager for adding peers.
func newAddPeerTestServer(t *testing.T, maxPeers int,
	targetOutbound uint32) *server {

	t.Helper()
	useTestConfig(t, &config{MaxPeers: maxPeers, MaxPeersPerIP: 5,
		TargetOutboundPeers: targetOutbound,
		BanThreshold:        defaultBanThreshold})
	oldSyncLevel, oldTxmpLevel := syncLog.Level(), txmpLog.Level()
	oldAmgrLevel, oldCmgrLevel := amgrLog.Level(), cmgrLog.Level()
	syncLog.SetLevel(bchlog.LevelOff)
	txmpLog.SetLevel(bchlog.LevelOff)
	amgrLog.SetLevel(bchlog.LevelOff)
	cmgrLog.SetLevel(bchlog.LevelOff)
	t.Cleanup(func() {
		syncLog.SetLevel(oldSyncLevel)
		txmpLog.SetLevel(oldTxmpLevel)
		amgrLog.SetLevel(oldAmgrLevel)
		cmgrLog.SetLevel(oldCmgrLevel)
	})
	params := &chaincfg.RegressionNetParams
	chain, _ := newTestChainDB(t)
	sm, err := netsync.New(&netsync.Config{
		PeerNotifier: nopPeerNotifier{},
		Chain:        chain,
		TxMemPool:    mempool.New(&mempool.Config{ChainParams: params}),
		ChainParams:  params,
		MaxPeers:     maxPeers,
	})
	if err != nil {
		t.Fatal(err)
	}
	sm.Start()
	cm, err := connmgr.New(&connmgr.Config{
		Dial: func(net.Addr) (net.Conn, error) {
			return nil, errors.New("no dialing in tests")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cm.Start()
	t.Cleanup(func() {
		cm.Stop()
		_ = sm.Stop()
	})
	return &server{
		chainParams:  params,
		chain:        chain,
		addrManager:  addrmgr.New(t.TempDir(), nil),
		syncManager:  sm,
		connManager:  cm,
		quit:         make(chan struct{}),
		inboundHosts: make(map[string]int),
		inboundBans:  make(map[string]time.Time),
	}
}

// fillPeerState adds the passed numbers of inbound, outbound and persistent
// placeholder peers, which are not connected, to state.
func fillPeerState(state *peerState, inbound, outbound, persistent int) {
	id := int32(-1)
	for _, n := range []struct {
		peers map[int32]*serverPeer
		count int
	}{
		{state.inboundPeers, inbound},
		{state.outboundPeers, outbound},
		{state.persistentPeers, persistent},
	} {
		for range n.count {
			n.peers[id] = &serverPeer{
				Peer: peer.NewInboundPeer(&peer.Config{}),
			}
			id--
		}
	}
}

// TestAdmitInboundKeepsOutboundSlots ensures inbound connections are refused
// once they would leave too few of the --maxpeers slots for the outbound
// peers the connection manager keeps.
func TestAdmitInboundKeepsOutboundSlots(t *testing.T) {
	s := newInboundTestServer(t, 5, 4)
	cfg.TargetOutboundPeers = 2
	for i, want := range []bool{true, true, false} {
		addr := &net.TCPAddr{IP: net.IPv4(10, 0, 0, byte(i+1)), Port: 1}
		if got := s.admitInbound(addr) != nil; got != want {
			t.Fatalf("inbound connection %d admitted %v, want %v", i,
				got, want)
		}
	}
}

// TestAddPeerKeepsOutboundSlots ensures inbound peers cannot take the slots
// of the outbound peers the connection manager keeps, and that persistent
// peers are not limited by --maxpeers and do not count toward it.  An inbound
// peer could take every slot, and outbound and persistent peers were then
// refused after their handshake and only redialed.
func TestAddPeerKeepsOutboundSlots(t *testing.T) {
	tests := []struct {
		name                          string
		inbound, outbound, persistent int
		newPeer                       string // inbound, outbound or persistent
		want                          bool
	}{
		{"inbound slots full", 2, 0, 0, "inbound", false},
		{"inbound with room", 1, 1, 0, "inbound", true},
		{"inbound over the total", 1, 3, 0, "inbound", false},
		{"outbound while inbound full", 2, 1, 1, "outbound", true},
		{"outbound over the total", 2, 2, 0, "outbound", false},
		{"persistent while full", 2, 2, 0, "persistent", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newAddPeerTestServer(t, 4, 2)
			state := newTestPeerState()
			fillPeerState(state, test.inbound, test.outbound,
				test.persistent)
			var sp *serverPeer
			if test.newPeer == "inbound" {
				sp = newInboundTestPeer(t, s, "10.0.0.2")
			} else {
				sp = newOutboundTestPeer(t, s, "10.0.0.2:18444")
				sp.persistent = test.newPeer == "persistent"
			}
			if got := s.handleAddPeerMsg(state, sp); got != test.want {
				t.Fatalf("%s peer added %v, want %v", test.newPeer, got,
					test.want)
			}
		})
	}
}

// TestConnectNodeLimits ensures a node added as persistent through the RPC
// server is connected however many peers there are, and a one-time
// connection only while fewer peers than --maxpeers count toward it.
func TestConnectNodeLimits(t *testing.T) {
	tests := []struct {
		name                          string
		inbound, outbound, persistent int
		permanent, want               bool
	}{
		{"persistent while full", 2, 2, 0, true, true},
		{"one-time while full", 2, 2, 0, false, false},
		{"one-time beside persistent", 2, 1, 1, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newAddPeerTestServer(t, 4, 2)
			state := newTestPeerState()
			fillPeerState(state, test.inbound, test.outbound,
				test.persistent)
			reply := make(chan error, 1)
			s.handleQuery(state, connectNodeMsg{addr: "10.0.0.3:18444",
				permanent: test.permanent, reply: reply})
			if err := <-reply; (err == nil) != test.want {
				t.Fatalf("connect got %v, want success %v", err,
					test.want)
			}
		})
	}
}

// TestLocalPeerNotBanned ensures a peer from a local address, as inbound onion
// peers usually are through the Tor daemon, is not banned, as in BCHN, so one
// misbehaving peer cannot shut out the others.
func TestLocalPeerNotBanned(t *testing.T) {
	s := newInboundTestServer(t, 5, 10)
	cfg.BanDuration = time.Hour

	// Bans are logged at the info level and the log rotator is not
	// initialized in tests.
	oldLevel := srvrLog.Level()
	srvrLog.SetLevel(bchlog.LevelOff)
	defer srvrLog.SetLevel(oldLevel)
	for _, addr := range []string{"127.0.0.1:8333", "[::1]:8333"} {
		sp := newServerPeer(s, false)
		p, err := peer.NewOutboundPeer(newPeerConfig(sp), addr)
		if err != nil {
			t.Fatalf("NewOutboundPeer: %v", err)
		}
		sp.Peer = p
		state := newTestPeerState()
		s.handleBanPeerMsg(state, sp)
		if len(state.banned) != 0 || len(s.inboundBans) != 0 {
			t.Fatalf("local peer %s was banned", addr)
		}
	}
}

// TestLocalHostNoPerIPLimit ensures the per-IP limit does not apply to local
// addresses, from which inbound onion peers usually come, while it still
// applies to others.
func TestLocalHostNoPerIPLimit(t *testing.T) {
	s := newInboundTestServer(t, 2, 10)
	for _, host := range []string{"127.0.0.1", "::1", "10.0.0.1"} {
		local := host != "10.0.0.1"
		for i := range 3 {
			addr := &net.TCPAddr{IP: net.ParseIP(host), Port: i}
			want := local || i < 2
			if got := s.admitInbound(addr) != nil; got != want {
				t.Fatalf("connection %d from %s admitted %v, want %v", i,
					host, got, want)
			}
		}
	}

	s = newAddPeerTestServer(t, 10, 2)
	cfg.MaxPeersPerIP = 2
	state := newTestPeerState()
	state.connectionCount["127.0.0.1"] = 2
	sp := newInboundTestPeer(t, s, "127.0.0.1")
	if !s.handleAddPeerMsg(state, sp) {
		t.Fatal("peer from a local address was refused at the per-IP limit")
	}
	if n := state.CountIP("127.0.0.1"); n != 3 {
		t.Fatalf("%d peers counted for the local address, want 3", n)
	}
}

// TestDisconnectNodeReleasesIP ensures peers disconnected through the RPC
// server, by address as it does, no longer count toward their host's per-IP
// limit once they are done, including every outbound peer with the address.
// They were removed from their list before, so their done messages did not
// lower the count, and after the limit's worth of such disconnects their host
// could not connect again.  A peer that disconnects otherwise releases its
// count when it is done, and a persistent peer, which is not counted, leaves
// the count of its host alone.
func TestDisconnectNodeReleasesIP(t *testing.T) {
	s := newAddPeerTestServer(t, 10, 2)
	state := newTestPeerState()
	inbound := newInboundTestPeer(t, s, "10.0.0.4")
	other := newInboundTestPeer(t, s, "10.0.0.7")
	outbound := []*serverPeer{
		newOutboundTestPeer(t, s, "10.0.0.5:18444"),
		newOutboundTestPeer(t, s, "10.0.0.5:18444"),
	}
	persistent := newOutboundTestPeer(t, s, "10.0.0.5:18445")
	persistent.persistent = true
	for _, sp := range append([]*serverPeer{inbound, other, persistent},
		outbound...) {

		sp.connReq = &connmgr.ConnReq{}
		if !s.handleAddPeerMsg(state, sp) {
			t.Fatalf("peer %v not added", sp)
		}
	}

	s.handleDonePeerMsg(state, persistent)
	if n := state.CountIP("10.0.0.5"); n != 2 {
		t.Fatalf("%d peers counted for the persistent peer's host, want 2",
			n)
	}
	for _, addr := range []string{inbound.Addr(), "10.0.0.5:18444"} {
		reply := make(chan error, 1)
		s.handleQuery(state, disconnectNodeMsg{
			cmp:   func(sp *serverPeer) bool { return sp.Addr() == addr },
			reply: reply,
		})
		if err := <-reply; err != nil {
			t.Fatal(err)
		}
	}
	for _, sp := range append([]*serverPeer{inbound, other}, outbound...) {
		s.handleDonePeerMsg(state, sp)
	}
	if len(state.connectionCount) != 0 {
		t.Fatalf("hosts still counted: %v", state.connectionCount)
	}
}

// TestSweepBans ensures sweepBans forgets the bans that have ended, from the
// peer handler's state and from the bans checked when connections are
// accepted, and keeps the others.
func TestSweepBans(t *testing.T) {
	s := newInboundTestServer(t, 5, 10)
	state := newTestPeerState()
	now := time.Now()
	for _, bans := range []map[string]time.Time{state.banned, s.inboundBans} {
		bans["10.0.0.1"] = now.Add(-time.Second)
		bans["10.0.0.2"] = now
		bans["10.0.0.3"] = now.Add(time.Second)
	}
	s.sweepBans(state, now)
	for _, bans := range []map[string]time.Time{state.banned, s.inboundBans} {
		if len(bans) != 1 || bans["10.0.0.3"].IsZero() {
			t.Fatalf("bans left: %v", bans)
		}
	}
}

// TestPeerHandlerSweepsBans ensures the peer handler forgets ended bans on its
// own, every banSweepInterval.  An ended ban was only dropped when its host
// connected again, so the ban maps never shrank.
func TestPeerHandlerSweepsBans(t *testing.T) {
	oldInterval := banSweepInterval
	banSweepInterval = 10 * time.Millisecond
	t.Cleanup(func() { banSweepInterval = oldInterval })
	s := newAddPeerTestServer(t, 10, 2)
	cfg.DisableDNSSeed = true
	s.newPeers = make(chan *serverPeer, 1)
	s.donePeers = make(chan *serverPeer, 1)
	s.banPeers = make(chan *serverPeer, 1)
	s.peerHeightsUpdate = make(chan updatePeerHeightsMsg)
	s.relayInv = make(chan relayMsg, 1)
	s.relayCmpctBlock = make(chan *wire.MsgCmpctBlock)
	s.broadcast = make(chan broadcastMsg, 1)
	s.query = make(chan interface{})
	s.maybeAddDirectRelayPeer = make(chan *maybeAddDirectRelayPeerMsg)
	s.inboundBans["10.0.0.1"] = time.Now().Add(-time.Second)
	s.inboundBans["10.0.0.2"] = time.Now().Add(time.Hour)

	s.wg.Add(1)
	go s.peerHandler()
	t.Cleanup(func() {
		close(s.quit)
		s.wg.Wait()
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.inboundMtx.Lock()
		_, ended := s.inboundBans["10.0.0.1"]
		_, current := s.inboundBans["10.0.0.2"]
		s.inboundMtx.Unlock()
		if !current {
			t.Fatal("a ban in force was forgotten")
		}
		if !ended {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("an ended ban was not forgotten")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRebroadcastDropsTxsNotInPool ensures a transaction that left the memory
// pool without being mined, here because a block spent its input, is dropped
// from the rebroadcast set instead of announced again.  It stayed there until
// a restart, announced to every peer at each rebroadcast, while the node
// answered a getdata for it with notfound.
func TestRebroadcastDropsTxsNotInPool(t *testing.T) {
	h := newMempoolTestHarness(t, 2)
	pool := h.server.cfg.TxMemPool
	kept, err := pool.ProcessTransaction(bchutil.NewTx(
		h.spend(t, h.outputs[0], 1000)), false, 0)
	if err != nil {
		t.Fatal(err)
	}
	gone, err := pool.ProcessTransaction(bchutil.NewTx(
		h.spend(t, h.outputs[1], 1000)), false, 0)
	if err != nil {
		t.Fatal(err)
	}
	pool.RemoveDoubleSpends(bchutil.NewTx(h.spend(t, h.outputs[1], 2000)))
	if pool.IsTransactionInPool(gone[0].Tx.Hash()) {
		t.Fatal("the double spent transaction is still in the pool")
	}

	keptIV := wire.NewInvVect(wire.InvTypeTx, kept[0].Tx.Hash())
	goneIV := wire.NewInvVect(wire.InvTypeTx, gone[0].Tx.Hash())
	pendingInvs := map[wire.InvVect]interface{}{
		*keptIV: kept[0],
		*goneIV: gone[0],
	}
	s := &server{
		txMemPool: pool,
		relayInv:  make(chan relayMsg, 2),
		quit:      make(chan struct{}),
	}
	s.rebroadcast(pendingInvs)

	if n := len(s.relayInv); n != 1 {
		t.Fatalf("%d transactions relayed, want 1", n)
	}
	if msg := <-s.relayInv; *msg.invVect != *keptIV {
		t.Fatalf("relayed %v, want %v", msg.invVect, keptIV)
	}
	if _, ok := pendingInvs[*goneIV]; ok {
		t.Fatal("the transaction that left the pool is still pending")
	}
	if _, ok := pendingInvs[*keptIV]; !ok {
		t.Fatal("the transaction in the pool is no longer pending")
	}
}

// TestRebroadcastHandlerRounds ensures the rebroadcast handler's rounds, the
// first and the later ones, relay the pending transactions, that one added or
// removed between rounds is relayed or not from the next round, and that the
// handler ends when the server quits.  The rounds, which come after 5 minutes
// and then at random within 30, are shortened to milliseconds.
func TestRebroadcastHandlerRounds(t *testing.T) {
	h := newMempoolTestHarness(t, 2)
	pool := h.server.cfg.TxMemPool
	var txs [2]*mempool.TxDesc
	for i := range txs {
		accepted, err := pool.ProcessTransaction(bchutil.NewTx(
			h.spend(t, h.outputs[i], 1000)), false, 0)
		if err != nil {
			t.Fatal(err)
		}
		txs[i] = accepted[0]
	}
	first, second := txs[0].Tx.Hash(), txs[1].Tx.Hash()

	// The first round comes once the first transaction was added.
	oldDelay, oldInterval := rebroadcastDelay, rebroadcastInterval
	rebroadcastDelay = 50 * time.Millisecond
	rebroadcastInterval = 2 * time.Millisecond
	t.Cleanup(func() {
		rebroadcastDelay, rebroadcastInterval = oldDelay, oldInterval
	})

	// The relay queue holds what the rounds relay in far longer than the
	// test takes, so no round waits for the test to take a relay.
	s := &server{
		txMemPool:            pool,
		relayInv:             make(chan relayMsg, 100000),
		modifyRebroadcastInv: make(chan interface{}),
		quit:                 make(chan struct{}),
	}
	s.wg.Add(1)
	go s.rebroadcastHandler()

	// next returns the hash of the next transaction a round relays.
	next := func() chainhash.Hash {
		t.Helper()
		select {
		case msg := <-s.relayInv:
			return msg.invVect.Hash
		case <-time.After(5 * time.Second):
		}
		t.Fatal("no rebroadcast round relayed a transaction")
		return chainhash.Hash{}
	}

	// The first round and a later one relay the first transaction.
	s.AddRebroadcastInventory(wire.NewInvVect(wire.InvTypeTx, first), txs[0])
	for range 2 {
		if got := next(); got != *first {
			t.Fatalf("relayed %v, want %v", got, first)
		}
	}

	// The rounds after the second is added relay both.
	s.AddRebroadcastInventory(wire.NewInvVect(wire.InvTypeTx, second),
		txs[1])
	for seen := 0; seen < 2; {
		switch got := next(); got {
		case *second:
			seen++
		case *first:
		default:
			t.Fatalf("relayed %v, want %v or %v", got, first, second)
		}
	}

	// The rounds after the first is removed, as when a block confirms it,
	// relay only the second.  What earlier rounds relayed is skipped.
	s.RemoveRebroadcastInventory(wire.NewInvVect(wire.InvTypeTx, first))
	for range len(s.relayInv) {
		<-s.relayInv
	}
	for range 4 {
		if got := next(); got != *second {
			t.Fatalf("relayed %v after it was removed, want %v",
				got, second)
		}
	}

	close(s.quit)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	if !returnedWithin(done, 5*time.Second) {
		t.Fatal("the rebroadcast handler did not end when the server quit")
	}
}

// TestPeerHandlerSendsAfterQuit ensures adding, banning and broadcasting to
// peers, and a peer's end, return once the server quits, as the peer handler
// that takes them stops then.  Each queue holds --maxpeers entries, which the
// stopped handler drains once, and the ban queue never, so once that many
// came after it stopped, the next sender waited for good.
func TestPeerHandlerSendsAfterQuit(t *testing.T) {
	s := &server{
		newPeers:  make(chan *serverPeer, 1),
		donePeers: make(chan *serverPeer, 1),
		banPeers:  make(chan *serverPeer, 1),
		broadcast: make(chan broadcastMsg, 1),
		quit:      make(chan struct{}),
	}
	close(s.quit)
	sp := newServerPeer(s, false)
	sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams: &chaincfg.RegressionNetParams,
	})
	sp.Disconnect()

	// The queues are full.
	s.newPeers <- sp
	s.donePeers <- sp
	s.banPeers <- sp
	s.broadcast <- broadcastMsg{}

	calls := []struct {
		name string
		fn   func()
	}{
		{"AddPeer", func() { s.AddPeer(sp) }},
		{"BanPeer", func() { s.BanPeer(sp) }},
		{"BroadcastMessage", func() {
			s.BroadcastMessage(wire.NewMsgPing(1))
		}},
		{"the end of a peer", func() { s.peerDoneHandler(sp) }},
	}
	for _, call := range calls {
		done := make(chan struct{})
		go func() {
			call.fn()
			close(done)
		}()
		if !returnedWithin(done, 5*time.Second) {
			t.Errorf("%s still waits for the peer handler after the "+
				"server quit", call.name)
		}
	}
}

// answerQuery answers msg, a query of the peer handler, as the peer handler
// does, with a made up answer.
func answerQuery(msg interface{}) {
	switch msg := msg.(type) {
	case getConnCountMsg:
		msg.reply <- 1
	case getOutboundGroup:
		msg.reply <- 1
	case connectNodeMsg:
		msg.reply <- nil
	case removeNodeMsg:
		msg.reply <- nil
	case disconnectNodeMsg:
		msg.reply <- nil
	case getPeersMsg:
		msg.reply <- nil
	case getAddedNodesMsg:
		msg.reply <- nil
	}
}

// TestPeerHandlerQueriesAfterQuit ensures the calls that ask the peer handler
// a question, and a peer height update, return once the server quits, as the
// peer handler stops then.  They waited for good: the connection manager asks
// for outbound group counts from goroutines nothing waits for, the RPC server
// asks for the peers and the connection count and changes connections, and
// the sync manager updates peer heights from a goroutine of its own.  A query
// that the stopped peer handler's drain took was never answered either.
func TestPeerHandlerQueriesAfterQuit(t *testing.T) {
	connChange := func(name string, change func(*rpcConnManager) error) func(*server) {
		return func(s *server) {
			err := change(&rpcConnManager{server: s})
			if !errors.Is(err, errServerShutdown) {
				t.Errorf("%s: %v once the server quit, want %v", name,
					err, errServerShutdown)
			}
		}
	}
	queries := []struct {
		name string
		fn   func(*server)
	}{
		{"ConnectedCount", func(s *server) {
			if n := s.ConnectedCount(); n != 0 {
				t.Errorf("ConnectedCount %d once the server quit", n)
			}
		}},
		{"OutboundGroupCount", func(s *server) {
			if n := s.OutboundGroupCount("10.0.0.0"); n != 0 {
				t.Errorf("OutboundGroupCount %d once the server quit", n)
			}
		}},
		{"Connect", connChange("Connect", func(cm *rpcConnManager) error {
			return cm.Connect("10.0.0.1:8333", false)
		})},
		{"RemoveByID", connChange("RemoveByID", func(cm *rpcConnManager) error {
			return cm.RemoveByID(1)
		})},
		{"RemoveByAddr", connChange("RemoveByAddr", func(cm *rpcConnManager) error {
			return cm.RemoveByAddr("10.0.0.1:8333")
		})},
		{"DisconnectByID", connChange("DisconnectByID", func(cm *rpcConnManager) error {
			return cm.DisconnectByID(1)
		})},
		{"DisconnectByAddr", connChange("DisconnectByAddr", func(cm *rpcConnManager) error {
			return cm.DisconnectByAddr("10.0.0.1:8333")
		})},
		{"ConnectedPeers", func(s *server) {
			peers := (&rpcConnManager{server: s}).ConnectedPeers()
			if len(peers) != 0 {
				t.Errorf("ConnectedPeers %d once the server quit",
					len(peers))
			}
		}},
		{"PersistentPeers", func(s *server) {
			peers := (&rpcConnManager{server: s}).PersistentPeers()
			if len(peers) != 0 {
				t.Errorf("PersistentPeers %d once the server quit",
					len(peers))
			}
		}},
	}

	// Nothing takes the queries.
	for _, query := range queries {
		s := &server{query: make(chan interface{}), quit: make(chan struct{})}
		close(s.quit)
		done := make(chan struct{})
		go func() {
			query.fn(s)
			close(done)
		}()
		if !returnedWithin(done, 5*time.Second) {
			t.Errorf("%s still waits for the peer handler after the "+
				"server quit", query.name)
		}
	}

	// The peer handler takes each query, as its drain does, but the server
	// quits before an answer.  The query returns, and an answer still given
	// does not wait for it.
	for _, query := range queries {
		s := &server{query: make(chan interface{}), quit: make(chan struct{})}
		done := make(chan struct{})
		go func() {
			query.fn(s)
			close(done)
		}()
		msg := <-s.query
		close(s.quit)
		if !returnedWithin(done, 5*time.Second) {
			t.Errorf("%s still waits for its answer after the server "+
				"quit", query.name)
		}
		answered := make(chan struct{})
		go func() {
			answerQuery(msg)
			close(answered)
		}()
		if !returnedWithin(answered, 5*time.Second) {
			t.Errorf("the answer to a %s that gave up waits", query.name)
		}
	}

	s := &server{
		peerHeightsUpdate: make(chan updatePeerHeightsMsg),
		quit:              make(chan struct{}),
	}
	close(s.quit)
	done := make(chan struct{})
	go func() {
		s.UpdatePeerHeights(&chainhash.Hash{1}, 1, nil)
		close(done)
	}()
	if !returnedWithin(done, 5*time.Second) {
		t.Error("UpdatePeerHeights still waits for the peer handler " +
			"after the server quit")
	}
}

// TestOnRejectLogsAtDebug ensures a reject message from a peer is logged only
// at debug level, as BCHN logs rejects only with -debug=net, with its command,
// code, reason and hash, the command and the reason quoted, so a peer's text
// starts no log line of its own.  One with a reason over the 111 bytes BCHN
// reads, or a command over 12, is logged as unparseable, without them.  Each
// reject was logged at warn level with its whole reason, up to 2 MiB, as often
// as a peer sent them.
func TestOnRejectLogsAtDebug(t *testing.T) {
	var logged bytes.Buffer
	savedLog := peerLog
	peerLog = bchlog.NewBackend(&logged).Logger("PEER")
	t.Cleanup(func() { peerLog = savedLog })
	sp := newServerPeer(&server{}, false)
	sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams: &chaincfg.RegressionNetParams,
	})
	hash := chainhash.Hash{1}
	reject := func(cmd, reason string) string {
		logged.Reset()
		sp.OnReject(sp.Peer, &wire.MsgReject{Cmd: cmd,
			Code: wire.RejectInvalid, Reason: reason, Hash: hash})
		return logged.String()
	}
	reason := strings.Repeat("r", 111)

	peerLog.SetLevel(bchlog.LevelInfo)
	for _, r := range []string{reason, reason + "r"} {
		if line := reject(wire.CmdTx, r); line != "" {
			t.Errorf("a reject was logged at info level: %q", line)
		}
	}

	peerLog.SetLevel(bchlog.LevelDebug)
	line := reject(wire.CmdTx, reason)
	for _, want := range []string{"[DBG]", `"tx"`, "REJECT_INVALID",
		`"` + reason + `"`, hash.String()} {

		if !strings.Contains(line, want) {
			t.Errorf("reject logged as %q, which does not say %q", line,
				want)
		}
	}
	if line := reject(wire.CmdPing, "no hash"); strings.Contains(line,
		hash.String()) {

		t.Errorf("a ping's reject was logged with a hash: %q", line)
	}
	if line := reject(wire.CmdDSProof, "x"); !strings.Contains(line,
		`"`+wire.CmdDSProof+`"`) {

		t.Errorf("a reject for the 12-byte command %q was logged as %q",
			wire.CmdDSProof, line)
	}

	for _, unparseable := range []struct{ cmd, reason string }{
		{wire.CmdTx, reason + "r"},
		{"commandtoolong", "x"},
	} {
		line := reject(unparseable.cmd, unparseable.reason)
		if !strings.Contains(line, "Unparseable reject message") ||
			strings.Contains(line, unparseable.reason) ||
			strings.Contains(line, unparseable.cmd) {

			t.Errorf("reject for %q with a %d-byte reason logged as %q, "+
				"want it unparseable", unparseable.cmd,
				len(unparseable.reason), line)
		}
	}

	line = reject(wire.CmdBlock, "a\n2026-10-22 00:00:00.000 [INF] PEER: x")
	if n := strings.Count(line, "\n"); n != 1 {
		t.Errorf("a reason with a newline was logged in %d lines: %q", n,
			line)
	}
}
