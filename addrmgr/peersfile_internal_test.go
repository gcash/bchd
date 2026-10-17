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
func wireAddr(ip net.IP) *wire.NetAddress {
	return wire.NewNetAddressIPPort(ip, 8333, wire.SFNodeNetwork)
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
