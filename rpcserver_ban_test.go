// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/gcash/bchd/btcjson"
)

// newBanTestRPCServer returns an RPC server of a test server with a running
// peer handler.
func newBanTestRPCServer(t *testing.T) (*rpcServer, *server) {
	t.Helper()
	s := newAddPeerTestServer(t, 10, 2)
	cfg.BanDuration = 24 * time.Hour
	startTestPeerHandler(t, s)
	return &rpcServer{cfg: rpcserverConfig{ConnMgr: &rpcConnManager{server: s}}}, s
}

// setBanCmd returns a setban command with its defaults filled in.
func setBanCmd(subnet string, command btcjson.SetBanSubCmd) *btcjson.SetBanCmd {
	return btcjson.NewSetBanCmd(subnet, command, btcjson.Int64(0), btcjson.Bool(false))
}

// wantRPCCode ensures err is an RPC error with the code.
func wantRPCCode(t *testing.T, err error, code btcjson.RPCErrorCode) {
	t.Helper()
	var rpcErr *btcjson.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != code {
		t.Fatalf("got error %v, want code %d", err, code)
	}
}

// TestBanRPCsAdminOnly ensures a limited user can't reach the ban commands.
func TestBanRPCsAdminOnly(t *testing.T) {
	for _, method := range []string{"setban", "listbanned", "clearbanned"} {
		if _, limited := rpcLimited[method]; limited {
			t.Errorf("%s is open to limited users", method)
		}
	}
}

// TestSetBanRPC ensures setban refuses what BCHN refuses with BCHN's error
// codes, and bans and lifts bans of addresses and subnets.
func TestSetBanRPC(t *testing.T) {
	r, s := newBanTestRPCServer(t)

	_, err := handleSetBan(r, setBanCmd("example.com", btcjson.SBAdd), nil)
	wantRPCCode(t, err, btcjson.ErrRPCClientInvalidIPOrSubnet)
	_, err = handleSetBan(r, setBanCmd("1.2.3.4", "other"), nil)
	wantRPCCode(t, err, btcjson.ErrRPCInvalidParameter)

	if _, err = handleSetBan(r, setBanCmd("10.0.0.0/24", btcjson.SBAdd), nil); err != nil {
		t.Fatal(err)
	}
	if _, banned := s.bans.bannedUntil("10.0.0.9", time.Now()); !banned {
		t.Fatal("the subnet isn't banned")
	}
	_, err = handleSetBan(r, setBanCmd("10.0.0.0/24", btcjson.SBAdd), nil)
	wantRPCCode(t, err, btcjson.ErrRPCClientNodeAlreadyAdded)
	_, err = handleSetBan(r, setBanCmd("10.0.0.9", btcjson.SBAdd), nil)
	wantRPCCode(t, err, btcjson.ErrRPCClientNodeAlreadyAdded)

	_, err = handleSetBan(r, setBanCmd("10.0.0.9", btcjson.SBRemove), nil)
	wantRPCCode(t, err, btcjson.ErrRPCClientInvalidIPOrSubnet)
	if _, err = handleSetBan(r, setBanCmd("10.0.0.0/24", btcjson.SBRemove), nil); err != nil {
		t.Fatal(err)
	}
	if _, banned := s.bans.bannedUntil("10.0.0.9", time.Now()); banned {
		t.Fatal("the subnet is still banned")
	}
}

// TestBanEnd ensures bantime and absolute work as in BCHN, and that a huge
// bantime can't overflow.
func TestBanEnd(t *testing.T) {
	useTestConfig(t, &config{BanDuration: time.Hour})
	now := time.Unix(1800000000, 0)
	tests := []struct {
		banTime  int64
		absolute bool
		want     int64
	}{
		{0, false, 1800003600},
		{-5, false, 1800003600},
		{0, true, 1800003600},
		{-5, true, 1800003600},
		{60, false, 1800000060},
		{1900000000, true, 1900000000},
		{1000, true, 1000},
		{1<<63 - 1, false, maxBanUnix},
		{1<<63 - 1, true, maxBanUnix},
	}
	for _, test := range tests {
		got := banEnd(now, test.banTime, test.absolute).Unix()
		if got != test.want {
			t.Errorf("%+v: got %d", test, got)
		}
	}
}

// TestListBannedRPC ensures listbanned gives BCHN's fields, sorted, for bans of
// both kinds, and leaves out a ban that ended.
func TestListBannedRPC(t *testing.T) {
	r, s := newBanTestRPCServer(t)
	now := time.Now()
	s.bans.banPeer("10.0.0.1", now.Add(-time.Hour), now.Add(time.Hour))
	s.bans.banPeer("10.0.0.2", now.Add(-time.Hour), now.Add(-time.Second))
	cmd := btcjson.NewSetBanCmd("192.168.1.0/24", btcjson.SBAdd, btcjson.Int64(600),
		btcjson.Bool(false))
	if _, err := handleSetBan(r, cmd, nil); err != nil {
		t.Fatal(err)
	}

	res, err := handleListBanned(r, btcjson.NewListBannedCmd(), nil)
	if err != nil {
		t.Fatal(err)
	}
	bans := res.([]btcjson.ListBannedResult)
	if len(bans) != 2 {
		t.Fatalf("bans: %+v", bans)
	}
	auto, manual := bans[0], bans[1]
	if auto.Address != "10.0.0.1/32" || auto.BanReason != "node misbehaving" ||
		auto.BanDuration != auto.BannedUntil-auto.BanCreated ||
		auto.BanDuration < 7199 || auto.BanDuration > 7201 ||
		auto.TimeRemaining < 3500 || auto.TimeRemaining > 3601 {

		t.Fatalf("automatic ban: %+v", auto)
	}
	if manual.Address != "192.168.1.0/24" || manual.BanReason != "manually added" ||
		manual.BanDuration < 599 || manual.BanDuration > 601 {

		t.Fatalf("manual ban: %+v", manual)
	}
}

// TestListBannedEmpty ensures no bans make an empty array, not null.
func TestListBannedEmpty(t *testing.T) {
	r, _ := newBanTestRPCServer(t)
	res, err := handleListBanned(r, btcjson.NewListBannedCmd(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bans := res.([]btcjson.ListBannedResult); bans == nil || len(bans) != 0 {
		t.Fatalf("got %#v", bans)
	}
}

// TestClearBannedRPC ensures clearbanned clears the chosen kinds.
func TestClearBannedRPC(t *testing.T) {
	r, s := newBanTestRPCServer(t)
	now := time.Now()
	s.bans.banPeer("10.0.0.1", now, now.Add(time.Hour))
	s.bans.add(mustBanTarget(t, "10.0.0.2"), now, now.Add(time.Hour))

	cmd := btcjson.NewClearBannedCmd(btcjson.Bool(false), btcjson.Bool(true))
	if _, err := handleClearBanned(r, cmd, nil); err != nil {
		t.Fatal(err)
	}
	if bans := s.bans.list(now); len(bans) != 1 || !bans[0].manual {
		t.Fatalf("bans: %+v", bans)
	}
	cmd = btcjson.NewClearBannedCmd(btcjson.Bool(true), btcjson.Bool(true))
	if _, err := handleClearBanned(r, cmd, nil); err != nil {
		t.Fatal(err)
	}
	if bans := s.bans.list(now); len(bans) != 0 {
		t.Fatalf("bans: %+v", bans)
	}
}

// TestBanArgumentsNull ensures an argument passed as null means its default.
func TestBanArgumentsNull(t *testing.T) {
	r, s := newBanTestRPCServer(t)
	cmd := &btcjson.SetBanCmd{Subnet: "10.0.0.1", Command: btcjson.SBAdd,
		Absolute: btcjson.Bool(true)}
	if _, err := handleSetBan(r, cmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := handleClearBanned(r, &btcjson.ClearBannedCmd{
		Automatic: btcjson.Bool(false)}, nil); err != nil {

		t.Fatal(err)
	}
	if bans := s.bans.list(time.Now()); len(bans) != 0 {
		t.Fatalf("bans: %+v", bans)
	}
}

// TestSetBanDisconnects ensures a ban through the RPC layer disconnects a
// connected peer the peer handler knows of.
func TestSetBanDisconnects(t *testing.T) {
	r, s := newBanTestRPCServer(t)
	sp := newOutboundTestPeer(t, s, "10.0.0.5:8333")
	s.AddPeer(sp)
	for deadline := time.Now().Add(30 * time.Second); s.ConnectedCount() != 1; {
		if time.Now().After(deadline) {
			t.Fatal("the peer wasn't added")
		}
		time.Sleep(time.Millisecond)
	}

	if _, err := handleSetBan(r, setBanCmd("10.0.0.0/24", btcjson.SBAdd), nil); err != nil {
		t.Fatal(err)
	}
	if sp.Connected() {
		t.Fatal("the banned peer stayed connected")
	}
}

// TestDisconnectBanned ensures the peers a ban applies to are disconnected,
// persistent ones too, and that the others stay.
func TestDisconnectBanned(t *testing.T) {
	s := newAddPeerTestServer(t, 10, 2)
	state := newTestPeerState()
	inSubnet := newOutboundTestPeer(t, s, "10.0.0.5:8333")
	persistent := newOutboundTestPeer(t, s, "10.0.0.6:8333")
	other := newOutboundTestPeer(t, s, "10.0.1.5:8333")
	state.outboundPeers[1] = inSubnet
	state.persistentPeers[2] = persistent
	state.outboundPeers[3] = other

	s.bans.add(mustBanTarget(t, "10.0.0.0/24"), time.Now(), time.Now().Add(time.Hour))
	reply := make(chan struct{}, 1)
	s.handleQuery(state, disconnectBannedMsg{reply: reply})
	<-reply
	if inSubnet.Connected() || persistent.Connected() {
		t.Fatal("a banned peer stayed connected")
	}
	if !other.Connected() {
		t.Fatal("a peer that isn't banned was disconnected")
	}
}
