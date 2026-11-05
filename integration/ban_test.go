// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/integration/rpctest"
)

// TestBanClient ensures rpcclient's ban calls work against a node: their
// arguments reach it, their answers come back, and a nil argument before one
// that is set keeps the set one.
func TestBanClient(t *testing.T) {
	r, err := rpctest.New(&chaincfg.SimNetParams, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetUp(false, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.TearDown(); err != nil {
			t.Error(err)
		}
	}()
	client := r.Node

	banTime := int64(600)
	if err := client.SetBan("10.0.0.0/24", btcjson.SBAdd, &banTime, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.SetBan("10.0.1.1", btcjson.SBAdd, nil, nil); err != nil {
		t.Fatal(err)
	}
	bans, err := client.ListBanned()
	if err != nil {
		t.Fatal(err)
	}
	if len(bans) != 2 || bans[0].Address != "10.0.0.0/24" ||
		bans[0].BanDuration != banTime || bans[1].Address != "10.0.1.1/32" {

		t.Fatalf("bans: %+v", bans)
	}

	err = client.SetBan("10.0.1.1", btcjson.SBAdd, nil, nil)
	var rpcErr *btcjson.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != btcjson.ErrRPCClientNodeAlreadyAdded {
		t.Fatalf("banning twice: %v", err)
	}
	if err := client.SetBan("10.0.1.1", btcjson.SBRemove, nil, nil); err != nil {
		t.Fatal(err)
	}

	// Asked to keep the automatic bans, it clears the manual ones.
	keep := false
	if err := client.ClearBanned(nil, &keep); err != nil {
		t.Fatal(err)
	}
	if bans, err = client.ListBanned(); err != nil || len(bans) != 0 {
		t.Fatalf("bans: %+v, %v", bans, err)
	}

	// Asked to keep the manual bans, it keeps them, and with no arguments
	// it clears everything.
	if err := client.SetBan("10.0.2.2", btcjson.SBAdd, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.ClearBanned(&keep, nil); err != nil {
		t.Fatal(err)
	}
	if bans, err = client.ListBanned(); err != nil || len(bans) != 1 {
		t.Fatalf("bans: %+v, %v", bans, err)
	}
	if err := client.ClearBanned(nil, nil); err != nil {
		t.Fatal(err)
	}
	if bans, err = client.ListBanned(); err != nil || len(bans) != 0 {
		t.Fatalf("bans: %+v, %v", bans, err)
	}
}
