// Copyright (c) 2013-2014 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package addrmgr_test

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/gcash/bchd/addrmgr"
	"github.com/gcash/bchd/wire"
)

// naTest is used to describe a test to be performed against the NetAddressKey
// method.
type naTest struct {
	in   *wire.NetAddressV2
	want string
}

// naTests houses all of the tests to be performed against the NetAddressKey
// method.
var naTests = make([]naTest, 0)

// Put some IP in here for convenience. Points to google.
var someIP = "173.194.115.66"

// addNaTests
func addNaTests() {
	// IPv4
	// Localhost
	addNaTest("127.0.0.1", 8333, "127.0.0.1:8333")
	addNaTest("127.0.0.1", 8334, "127.0.0.1:8334")

	// Class A
	addNaTest("1.0.0.1", 8333, "1.0.0.1:8333")
	addNaTest("2.2.2.2", 8334, "2.2.2.2:8334")
	addNaTest("27.253.252.251", 8335, "27.253.252.251:8335")
	addNaTest("123.3.2.1", 8336, "123.3.2.1:8336")

	// Private Class A
	addNaTest("10.0.0.1", 8333, "10.0.0.1:8333")
	addNaTest("10.1.1.1", 8334, "10.1.1.1:8334")
	addNaTest("10.2.2.2", 8335, "10.2.2.2:8335")
	addNaTest("10.10.10.10", 8336, "10.10.10.10:8336")

	// Class B
	addNaTest("128.0.0.1", 8333, "128.0.0.1:8333")
	addNaTest("129.1.1.1", 8334, "129.1.1.1:8334")
	addNaTest("180.2.2.2", 8335, "180.2.2.2:8335")
	addNaTest("191.10.10.10", 8336, "191.10.10.10:8336")

	// Private Class B
	addNaTest("172.16.0.1", 8333, "172.16.0.1:8333")
	addNaTest("172.16.1.1", 8334, "172.16.1.1:8334")
	addNaTest("172.16.2.2", 8335, "172.16.2.2:8335")
	addNaTest("172.16.172.172", 8336, "172.16.172.172:8336")

	// Class C
	addNaTest("193.0.0.1", 8333, "193.0.0.1:8333")
	addNaTest("200.1.1.1", 8334, "200.1.1.1:8334")
	addNaTest("205.2.2.2", 8335, "205.2.2.2:8335")
	addNaTest("223.10.10.10", 8336, "223.10.10.10:8336")

	// Private Class C
	addNaTest("192.168.0.1", 8333, "192.168.0.1:8333")
	addNaTest("192.168.1.1", 8334, "192.168.1.1:8334")
	addNaTest("192.168.2.2", 8335, "192.168.2.2:8335")
	addNaTest("192.168.192.192", 8336, "192.168.192.192:8336")

	// IPv6
	// Localhost
	addNaTest("::1", 8333, "[::1]:8333")
	addNaTest("fe80::1", 8334, "[fe80::1]:8334")

	// Link-local
	addNaTest("fe80::1:1", 8333, "[fe80::1:1]:8333")
	addNaTest("fe91::2:2", 8334, "[fe91::2:2]:8334")
	addNaTest("fea2::3:3", 8335, "[fea2::3:3]:8335")
	addNaTest("feb3::4:4", 8336, "[feb3::4:4]:8336")

	// Site-local
	addNaTest("fec0::1:1", 8333, "[fec0::1:1]:8333")
	addNaTest("fed1::2:2", 8334, "[fed1::2:2]:8334")
	addNaTest("fee2::3:3", 8335, "[fee2::3:3]:8335")
	addNaTest("fef3::4:4", 8336, "[fef3::4:4]:8336")

	// IPv4-mapped IPv6
	addNaTest("::ffff:1.2.3.4", 8333, "1.2.3.4:8333")

	// Tor v3, I2P and CJDNS
	torV3, _ := wire.DecodeTorV3(torV3Host)
	naTests = append(naTests, naTest{
		wire.NewNetAddressV2(wire.NetworkTorV3, torV3, 8333, 0),
		torV3Host + ":8333",
	})
	naTests = append(naTests, naTest{
		wire.NewNetAddressV2(wire.NetworkI2P, make([]byte, 32), 0, 0),
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.b32.i2p:0",
	})
	naTests = append(naTests, naTest{
		wire.NewNetAddressV2(wire.NetworkCJDNS, net.ParseIP("fc00::1"), 8333, 0),
		"[fc00::1]:8333",
	})
}

// torV3Host is a valid Tor v3 onion name.
const torV3Host = "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion"

func addNaTest(ip string, port uint16, want string) {
	nip := net.ParseIP(ip)
	na := wire.NewNetAddressV2IPPort(nip, port, wire.SFNodeNetwork)
	test := naTest{na, want}
	naTests = append(naTests, test)
}

func lookupFunc(_ string) ([]net.IP, error) {
	return nil, errors.New("not implemented")
}

func TestStartStop(t *testing.T) {
	n := addrmgr.New("teststartstop", lookupFunc)
	n.Start()
	err := n.Stop()
	if err != nil {
		t.Fatalf("Address Manager failed to stop: %v", err)
	}
}

func TestAddAddressByIP(t *testing.T) {
	fmtErr := fmt.Errorf("")
	addrErr := &net.AddrError{}
	var tests = []struct {
		addrIP string
		err    error
	}{
		{
			someIP + ":8333",
			nil,
		},
		{
			someIP,
			addrErr,
		},
		{
			someIP[:12] + ":8333",
			fmtErr,
		},
		{
			someIP + ":abcd",
			fmtErr,
		},
	}

	amgr := addrmgr.New("testaddressbyip", nil)
	for i, test := range tests {
		err := amgr.AddAddressByIP(test.addrIP)
		if test.err != nil && err == nil {
			t.Errorf("TestGood test %d failed expected an error and got none", i)
			continue
		}
		if test.err == nil && err != nil {
			t.Errorf("TestGood test %d failed expected no error and got one", i)
			continue
		}
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("TestGood test %d failed got %v, want %v", i,
				reflect.TypeOf(err), reflect.TypeOf(test.err))
			continue
		}
	}
}

func TestAddLocalAddress(t *testing.T) {
	var tests = []struct {
		host     string
		priority addrmgr.AddressPriority
		valid    bool
	}{
		{"192.168.0.100", addrmgr.InterfacePrio, false},
		{"204.124.1.1", addrmgr.InterfacePrio, true},
		{"204.124.1.1", addrmgr.BoundPrio, true},
		{"::1", addrmgr.InterfacePrio, false},
		{"fe80::1", addrmgr.InterfacePrio, false},
		{"2620:100::1", addrmgr.InterfacePrio, true},
		{"fd87:d87e:eb43::1", addrmgr.InterfacePrio, false},
		{torV3Host, addrmgr.ManualPrio, true},
	}
	amgr := addrmgr.New("testaddlocaladdress", lookupFunc)
	for x, test := range tests {
		na, err := amgr.HostToNetAddress(test.host, 8333, 0)
		if err != nil {
			t.Fatalf("HostToNetAddress(%s): %v", test.host, err)
		}
		result := amgr.AddLocalAddress(na, test.priority)
		if result == nil && !test.valid {
			t.Errorf("TestAddLocalAddress test #%d failed: %s should have "+
				"been accepted", x, test.host)
			continue
		}
		if result != nil && test.valid {
			t.Errorf("TestAddLocalAddress test #%d failed: %s should not have "+
				"been accepted", x, test.host)
			continue
		}
	}
}

// TestHostToNetAddress tests that onion names are parsed without a lookup and
// that Tor v2 and malformed onion names are rejected.
func TestHostToNetAddress(t *testing.T) {
	amgr := addrmgr.New("testhosttonetaddress", lookupFunc)
	na, err := amgr.HostToNetAddress(torV3Host, 8333, wire.SFNodeNetwork)
	if err != nil {
		t.Fatal(err)
	}
	if na.Network != wire.NetworkTorV3 || addrmgr.NetAddressKey(na) !=
		torV3Host+":8333" || na.Services != wire.SFNodeNetwork {

		t.Fatalf("got %v %s", na.Network, addrmgr.NetAddressKey(na))
	}

	for _, host := range []string{
		"aaaaaaaaaaaaaaaa.onion",
		"pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscsad.onion",
		"example.onion",
	} {
		_, err := amgr.HostToNetAddress(host, 8333, 0)
		if err == nil || err.Error() == "not implemented" {
			t.Errorf("HostToNetAddress(%s): got %v, want a parse error",
				host, err)
		}
	}

	na, err = amgr.HostToNetAddress("::ffff:1.2.3.4", 8333, 0)
	if err != nil || na.Network != wire.NetworkIPv4 {
		t.Fatalf("IPv4-mapped: %v %v", na, err)
	}
}

func TestAttempt(t *testing.T) {
	n := addrmgr.New("testattempt", lookupFunc)

	// Add a new address and get it
	err := n.AddAddressByIP(someIP + ":8333")
	if err != nil {
		t.Fatalf("Adding address failed: %v", err)
	}
	ka := n.GetAddress()

	if !ka.LastAttempt().IsZero() {
		t.Errorf("Address should not have attempts, but does")
	}

	na := ka.NetAddress()
	n.Attempt(na)

	if ka.LastAttempt().IsZero() {
		t.Errorf("Address should have an attempt, but does not")
	}
}

func TestConnected(t *testing.T) {
	n := addrmgr.New("testconnected", lookupFunc)

	// Add a new address and get it
	err := n.AddAddressByIP(someIP + ":8333")
	if err != nil {
		t.Fatalf("Adding address failed: %v", err)
	}
	ka := n.GetAddress()
	na := ka.NetAddress()
	// make it an hour ago
	na.Timestamp = time.Unix(time.Now().Add(time.Hour*-1).Unix(), 0)

	n.Connected(na)

	if !ka.NetAddress().Timestamp.After(na.Timestamp) {
		t.Errorf("Address should have a new timestamp, but does not")
	}
}

func TestNeedMoreAddresses(t *testing.T) {
	n := addrmgr.New("testneedmoreaddresses", lookupFunc)
	addrsToAdd := 1500
	b := n.NeedMoreAddresses()
	if !b {
		t.Errorf("Expected that we need more addresses")
	}
	addrs := make([]*wire.NetAddressV2, addrsToAdd)

	var err error
	for i := range addrsToAdd {
		s := fmt.Sprintf("%d.%d.173.147:8333", i/128+60, i%128+60)
		addrs[i], err = n.DeserializeNetAddress(s, wire.SFNodeNetwork)
		if err != nil {
			t.Errorf("Failed to turn %s into an address: %v", s, err)
		}
	}

	srcAddr := wire.NewNetAddressV2IPPort(net.IPv4(173, 144, 173, 111), 8333, 0)

	n.AddAddresses(addrs, srcAddr)
	numAddrs := n.NumAddresses()
	if numAddrs > addrsToAdd {
		t.Errorf("Number of addresses is too many %d vs %d", numAddrs, addrsToAdd)
	}

	b = n.NeedMoreAddresses()
	if b {
		t.Errorf("Expected that we don't need more addresses")
	}
}

func TestGood(t *testing.T) {
	n := addrmgr.New("testgood", lookupFunc)
	addrsToAdd := 64 * 64
	addrs := make([]*wire.NetAddressV2, addrsToAdd)

	var err error
	for i := range addrsToAdd {
		s := fmt.Sprintf("%d.173.147.%d:8333", i/64+60, i%64+60)
		addrs[i], err = n.DeserializeNetAddress(s, wire.SFNodeNetwork)
		if err != nil {
			t.Errorf("Failed to turn %s into an address: %v", s, err)
		}
	}

	srcAddr := wire.NewNetAddressV2IPPort(net.IPv4(173, 144, 173, 111), 8333, 0)

	n.AddAddresses(addrs, srcAddr)
	for _, addr := range addrs {
		n.Good(addr)
	}

	numAddrs := n.NumAddresses()
	if numAddrs >= addrsToAdd {
		t.Errorf("Number of addresses is too many: %d vs %d", numAddrs, addrsToAdd)
	}

	numCache := len(n.AddressCache())
	if numCache >= numAddrs/4 {
		t.Errorf("Number of addresses in cache: got %d, want %d", numCache, numAddrs/4)
	}
}

func TestGetAddress(t *testing.T) {
	n := addrmgr.New("testgetaddress", lookupFunc)

	// Get an address from an empty set (should error)
	if rv := n.GetAddress(); rv != nil {
		t.Errorf("GetAddress failed: got: %v want: %v\n", rv, nil)
	}

	// Add a new address and get it
	err := n.AddAddressByIP(someIP + ":8333")
	if err != nil {
		t.Fatalf("Adding address failed: %v", err)
	}
	ka := n.GetAddress()
	if ka == nil {
		t.Fatalf("Did not get an address where there is one in the pool")
	}
	if ka.NetAddress().IP().String() != someIP {
		t.Errorf("Wrong IP: got %v, want %v", ka.NetAddress().IP().String(), someIP)
	}

	// Mark this as a good address and get it
	n.Good(ka.NetAddress())
	ka = n.GetAddress()
	if ka == nil {
		t.Fatalf("Did not get an address where there is one in the pool")
	}
	if ka.NetAddress().IP().String() != someIP {
		t.Errorf("Wrong IP: got %v, want %v", ka.NetAddress().IP().String(), someIP)
	}

	numAddrs := n.NumAddresses()
	if numAddrs != 1 {
		t.Errorf("Wrong number of addresses: got %d, want %d", numAddrs, 1)
	}
}

func TestGetBestLocalAddress(t *testing.T) {
	amgr := addrmgr.New("testgetbestlocaladdress", lookupFunc)
	addr := func(host string) *wire.NetAddressV2 {
		na, err := amgr.HostToNetAddress(host, 8333, 0)
		if err != nil {
			t.Fatalf("HostToNetAddress(%s): %v", host, err)
		}
		return na
	}
	const (
		localTor  = "kpgvmscirrdqpekbqjsvw5teanhatztpp2gl6eee4zkowvwfxwenqaid.onion"
		remoteTor = torV3Host
	)
	localAddrs := []string{"192.168.0.100", "::1", "fe80::1", "2001:470::1"}

	var tests = []struct {
		remoteAddr string
		want0      string
		want1      string
		want2      string
		want3      string
	}{
		{
			// Remote connection from public IPv4
			"204.124.8.1",
			"0.0.0.0",
			"0.0.0.0",
			"204.124.8.100",
			"204.124.8.100",
		},
		{
			// Remote connection from private IPv4
			"172.16.0.254",
			"0.0.0.0",
			"0.0.0.0",
			"0.0.0.0",
			"0.0.0.0",
		},
		{
			// Remote connection from public IPv6
			"2602:100:abcd::102",
			"::",
			"2001:470::1",
			"2001:470::1",
			"2001:470::1",
		},
		{
			// Remote connection from Tor
			remoteTor,
			"0.0.0.0",
			"2001:470::1",
			"204.124.8.100",
			localTor,
		},
	}

	check := func(step int, want func(int) string) {
		t.Helper()
		for x, test := range tests {
			got := amgr.GetBestLocalAddress(addr(test.remoteAddr))
			if got.Host() != want(x) {
				t.Errorf("TestGetBestLocalAddress test%d #%d failed "+
					"for remote address %s: want %s got %s", step,
					x, test.remoteAddr, want(x), got.Host())
			}
		}
	}

	// Test against default when there's no address
	check(0, func(x int) string { return tests[x].want0 })

	for _, localAddr := range localAddrs {
		_ = amgr.AddLocalAddress(addr(localAddr), addrmgr.InterfacePrio)
	}
	check(1, func(x int) string { return tests[x].want1 })

	// Add a public IP to the list of local addresses.
	_ = amgr.AddLocalAddress(addr("204.124.8.100"), addrmgr.InterfacePrio)
	check(2, func(x int) string { return tests[x].want2 })

	// Add a Tor address.
	_ = amgr.AddLocalAddress(addr(localTor), addrmgr.InterfacePrio)
	check(3, func(x int) string { return tests[x].want3 })
}

func TestNetAddressKey(t *testing.T) {
	addNaTests()

	t.Logf("Running %d tests", len(naTests))
	for i, test := range naTests {
		key := addrmgr.NetAddressKey(test.in)
		if key != test.want {
			t.Errorf("NetAddressKey #%d\n got: %s want: %s", i, key, test.want)
			continue
		}
	}

}
