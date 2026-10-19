// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"testing"

	"github.com/gcash/bchd/addrmgr"
	"github.com/gcash/bchd/wire"
)

// TestKnownAddressesCapped ensures a peer is remembered to know only its most
// recently added addresses, as many as Bitcoin Cash Node remembers.  Every
// address sent to or received from a peer was kept for as long as it stayed
// connected.
func TestKnownAddressesCapped(t *testing.T) {
	const limit = maxKnownAddrsPerPeer
	sp := newServerPeer(&server{}, false)
	addrs := make([]*wire.NetAddressV2, limit+100)
	for i := range addrs {
		ip := net.IPv4(10, byte(i>>16), byte(i>>8), byte(i))
		addrs[i] = wire.NewNetAddressV2IPPort(ip, 8333, 0)
	}

	// Added again once the set is full, the first address becomes the most
	// recent, so the next oldest are the ones dropped.
	sp.addKnownAddresses(addrs[:limit])
	sp.addKnownAddresses(addrs[:1])
	sp.addKnownAddresses(addrs[limit:])

	if !sp.addressKnown(addrs[0]) {
		t.Error("address added again is not known")
	}
	for i, na := range addrs[1:101] {
		if sp.addressKnown(na) {
			t.Fatalf("address %d of %d, %s, is still known", i+1,
				len(addrs), addrmgr.NetAddressKey(na))
		}
	}
	for _, na := range addrs[101:] {
		if !sp.addressKnown(na) {
			t.Fatalf("recent address %s is not known",
				addrmgr.NetAddressKey(na))
		}
	}
}
