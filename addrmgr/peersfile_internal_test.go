package addrmgr

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gcash/bchd/wire"
)

// testLookup refuses to resolve anything so tests never touch the network.
func testLookup(host string) ([]net.IP, error) {
	return nil, errors.New("no such host " + host)
}

// wireAddr returns a full node address for ip on port 8333.
func wireAddr(ip net.IP) *wire.NetAddressV2 {
	return wire.NewNetAddressV2IPPort(ip, 8333, wire.SFNodeNetwork)
}

// writePeersFile writes body as the peers file in dir.
func writePeersFile(t *testing.T, dir, body string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, "peers.json"), []byte(body), 0600)
	if err != nil {
		t.Fatal(err)
	}
}

// assertCounters checks that nNew and nTried match the bucket contents.
func assertCounters(t *testing.T, a *AddrManager) {
	t.Helper()

	nNew, nTried := 0, 0
	for _, ka := range a.addrIndex {
		if ka.refs > 0 {
			nNew++
		}
	}
	for i := range a.addrTried {
		nTried += a.addrTried[i].Len()
	}
	if a.nNew != nNew || a.nTried != nTried {
		t.Fatalf("counters nNew=%d nTried=%d, tables hold %d new and "+
			"%d tried", a.nNew, a.nTried, nNew, nTried)
	}
}

// assertGetAddressReturns fails if GetAddress doesn't return promptly.
func assertGetAddressReturns(t *testing.T, a *AddrManager) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		_ = a.GetAddress()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetAddress did not return")
	}
}

// TestLoadPeersRejectedFileResetsCounters ensures a peers file that fails to
// load leaves the counters consistent with the emptied tables.
func TestLoadPeersRejectedFileResetsCounters(t *testing.T) {
	dir := t.TempDir()
	a := New(dir, testLookup)
	for i := range 10 {
		a.AddAddress(
			wireAddr(net.IPv4(1, 2, 3, byte(i+1))),
			wireAddr(net.IPv4(5, 6, 7, 8)))
	}
	a.Good(wireAddr(net.IPv4(1, 2, 3, 1)))
	writePeersFile(t, dir, `{"Version":99}`)

	a.loadPeers()
	assertCounters(t, a)
	if n := a.NumAddresses(); n != 0 {
		t.Fatalf("NumAddresses = %d, want 0", n)
	}
	assertGetAddressReturns(t, a)
}

// TestResetClearsCounters ensures reset empties the counters.
func TestResetClearsCounters(t *testing.T) {
	a := New(t.TempDir(), testLookup)
	for i := range 10 {
		a.AddAddress(
			wireAddr(net.IPv4(1, 2, 3, byte(i+1))),
			wireAddr(net.IPv4(5, 6, 7, 8)))
	}
	a.Good(wireAddr(net.IPv4(1, 2, 3, 1)))

	a.reset()
	if n := a.NumAddresses(); n != 0 {
		t.Fatalf("NumAddresses after reset = %d, want 0", n)
	}
	assertGetAddressReturns(t, a)
}

// TestLoadPeersRejectedAfterNewBuckets ensures a fresh manager whose peers
// file is rejected after its new buckets were counted ends up with zero
// counters.
func TestLoadPeersRejectedAfterNewBuckets(t *testing.T) {
	dir := t.TempDir()
	const addr = "1.2.3.4:8333"
	sam := serializedAddrManager{
		Version: serialisationVersion,
		Addresses: []*serializedKnownAddress{{
			Addr:        addr,
			Src:         "5.6.7.8:8333",
			Services:    wire.SFNodeNetwork,
			SrcServices: wire.SFNodeNetwork,
		}},
	}
	sam.NewBuckets[0] = []string{addr}
	sam.TriedBuckets[0] = []string{"9.9.9.9:8333"}
	body, err := json.Marshal(&sam)
	if err != nil {
		t.Fatal(err)
	}
	writePeersFile(t, dir, string(body))

	a := New(dir, testLookup)
	a.loadPeers()
	assertCounters(t, a)
	if n := a.NumAddresses(); n != 0 {
		t.Fatalf("NumAddresses = %d, want 0", n)
	}
	assertGetAddressReturns(t, a)
}

// TestLoadPeersV2File ensures a version 2 peers file loads with its Tor v2
// addresses dropped, and is written back as version 3.
func TestLoadPeersV2File(t *testing.T) {
	dir := t.TempDir()
	const (
		ipv4   = "1.2.3.4:8333"
		ipv6   = "[2001:4860::1]:8333"
		onion  = "aaaaaaaaaaaaaaaa.onion:8333"
		onion2 = "bbbbbbbbbbbbbbbb.onion:8333"
		tried  = "5.6.7.8:8333"
	)
	sam := serializedAddrManager{
		Version: 2,
		Addresses: []*serializedKnownAddress{
			{Addr: ipv4, Src: "9.9.9.9:8333", Services: 1, SrcServices: 1},
			{Addr: ipv6, Src: onion, Services: 1, SrcServices: 1},
			{Addr: onion, Src: "9.9.9.9:8333", Services: 1, SrcServices: 1},
			{Addr: onion2, Src: "9.9.9.9:8333", Services: 1, SrcServices: 1},
			{Addr: tried, Src: "9.9.9.9:8333", Services: 1, SrcServices: 1},
		},
	}
	sam.NewBuckets[0] = []string{ipv4, ipv6, onion}
	sam.NewBuckets[1] = []string{ipv4}
	sam.TriedBuckets[0] = []string{tried, onion2}
	body, err := json.Marshal(&sam)
	if err != nil {
		t.Fatal(err)
	}
	writePeersFile(t, dir, string(body))

	a := New(dir, testLookup)
	a.loadPeers()
	assertCounters(t, a)
	if n := a.NumAddresses(); n != 2 {
		t.Fatalf("NumAddresses = %d, want 2", n)
	}
	for _, key := range []string{ipv4, tried} {
		if _, ok := a.addrIndex[key]; !ok {
			t.Fatalf("%s not loaded", key)
		}
	}

	a.savePeers()
	body, err = os.ReadFile(filepath.Join(dir, "peers.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved serializedAddrManager
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version != serialisationVersion || serialisationVersion != 3 {
		t.Fatalf("saved version %d", saved.Version)
	}
	for _, ska := range saved.Addresses {
		if ska.Network != wire.NetworkIPv4 || ska.SrcNetwork != wire.NetworkIPv4 {
			t.Fatalf("saved %+v", ska)
		}
	}

	b := New(dir, testLookup)
	b.loadPeers()
	assertCounters(t, b)
	if n := b.NumAddresses(); n != 2 {
		t.Fatalf("NumAddresses after reload = %d, want 2", n)
	}
}

// TestDeserializeAddr tests parsing the addresses of each network from the
// peers file.
func TestDeserializeAddr(t *testing.T) {
	const torV3 = "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion"
	const i2p = "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p"
	tests := []struct {
		addr    string
		network wire.NetworkID
		want    wire.NetworkID // 0 for an error
	}{
		{"1.2.3.4:8333", 0, wire.NetworkIPv4},
		{"[2001:db8::1]:8333", 0, wire.NetworkIPv6},
		{"aaaaaaaaaaaaaaaa.onion:8333", 0, 0},
		{torV3 + ":8333", 0, 0},
		{"example.com:8333", 0, 0},
		{"1.2.3.4:8333", wire.NetworkIPv4, wire.NetworkIPv4},
		{"1.2.3.4:8333", wire.NetworkIPv6, 0},
		{"[2001:db8::1]:8333", wire.NetworkIPv6, wire.NetworkIPv6},
		{torV3 + ":8333", wire.NetworkTorV3, wire.NetworkTorV3},
		{"aaaaaaaaaaaaaaaa.onion:8333", wire.NetworkTorV3, 0},
		{i2p + ":0", wire.NetworkI2P, wire.NetworkI2P},
		{"[fc00:1:2:3:4:5:6:7]:8333", wire.NetworkCJDNS, wire.NetworkCJDNS},
		{"[2001:4860::1]:8333", wire.NetworkCJDNS, 0},
		{"1.2.3.4:8333", wire.NetworkCJDNS, 0},
		{"1.2.3.4:8333", wire.NetworkTorV2, 0},
		{"1.2.3.4", wire.NetworkIPv4, 0},
	}
	for _, test := range tests {
		na, err := deserializeAddr(test.addr, test.network, 1)
		if test.want == 0 {
			if err == nil {
				t.Errorf("%s %v: no error", test.addr, test.network)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s %v: %v", test.addr, test.network, err)
			continue
		}
		if na.Network != test.want || NetAddressKey(na) != test.addr {
			t.Errorf("%s %v: got %v %s", test.addr, test.network,
				na.Network, NetAddressKey(na))
		}
	}
}

// TestCJDNSOutsideFC00 ensures a CJDNS address outside fc00::/8, whose key
// would be the same as that of an IPv6 address, is not stored, in either
// order of adding the two.
func TestCJDNSOutsideFC00(t *testing.T) {
	ip := net.ParseIP("2001:4860::1")
	src := wireAddr(net.IPv4(5, 6, 7, 8))
	for _, cjdnsFirst := range []bool{true, false} {
		a := New(t.TempDir(), testLookup)
		ipv6 := wireAddr(ip)
		cjdns := wire.NewNetAddressV2(wire.NetworkCJDNS, ip.To16(), 8333,
			wire.SFNodeNetwork)
		if NetAddressKey(cjdns) != NetAddressKey(ipv6) {
			t.Fatal("keys differ")
		}
		if cjdnsFirst {
			a.AddAddress(cjdns, src)
			a.AddAddress(ipv6, src)
		} else {
			a.AddAddress(ipv6, src)
			a.AddAddress(cjdns, src)
		}
		ka := a.addrIndex[NetAddressKey(ipv6)]
		if a.NumAddresses() != 1 || ka == nil ||
			ka.na.Network != wire.NetworkIPv6 {

			t.Fatalf("cjdnsFirst=%v: %d addresses, stored %+v",
				cjdnsFirst, a.NumAddresses(), ka)
		}
		if IsRoutable(cjdns) || GroupKey(cjdns) != "unroutable" {
			t.Fatalf("CJDNS outside fc00::/8 is routable")
		}
	}
}
