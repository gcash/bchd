// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"
)

// TestParseBanTarget ensures addresses and subnets are read as BCHN reads
// them, and shown as its listbanned shows them.
func TestParseBanTarget(t *testing.T) {
	onion := "pg6mmjiyjmcrsslvykfwnntlaru7p5svn6y2ymmju6nubxndf4pscryd.onion"
	tests := []struct {
		in, want string
		subnet   bool
	}{
		{in: "1.2.3.4", want: "1.2.3.4/32"},
		{in: "1.2.3.4/32", want: "1.2.3.4/32"},
		{in: "1.2.3.4/255.255.255.255", want: "1.2.3.4/32"},
		{in: "::ffff:1.2.3.4", want: "1.2.3.4/32"},
		{in: "2001:db8::1", want: "2001:db8::1/128"},
		{in: "2001:DB8:0::1/128", want: "2001:db8::1/128"},
		{in: "1.2.3.0/24", want: "1.2.3.0/24", subnet: true},
		{in: "1.2.3.4/24", want: "1.2.3.0/24", subnet: true},
		{in: "1.2.3.4/255.255.255.0", want: "1.2.3.0/24", subnet: true},
		{in: "0.0.0.0/0", want: "0.0.0.0/0", subnet: true},
		{in: "2001:db8::1/32", want: "2001:db8::/32", subnet: true},
		{in: "2001:db8::/ffff:ffff::", want: "2001:db8::/32", subnet: true},
		{in: onion, want: onion},
		{in: "PG6MMJIYJMCRSSLVYKFWNNTLARU7P5SVN6Y2YMMJU6NUBXNDF4PSCRYD.ONION", want: onion},
	}
	for _, test := range tests {
		target, ok := parseBanTarget(test.in)
		if !ok {
			t.Errorf("%s: refused", test.in)
			continue
		}
		if got := target.String(); got != test.want {
			t.Errorf("%s: got %s, want %s", test.in, got, test.want)
		}
		if (target.subnet != nil) != test.subnet {
			t.Errorf("%s: subnet %v, want %v", test.in, target.subnet, test.subnet)
		}
	}
	for _, in := range []string{
		"", "example.com", "1.2.3", "1.2.3.4/33", "1.2.3.4/256", "1.2.3.4/-1",
		"1.2.3.4/", "2001:db8::1/129", "1.2.3.4/255.0.255.0",
		"1.2.3.4/ffff:ffff::", "2001:db8::1/255.255.0.0", "1.2.3.4:8333",
		onion + "/128", ".onion", "foo.onion", "not base32 at all!.onion",
	} {
		if target, ok := parseBanTarget(in); ok {
			t.Errorf("%q: read as %v", in, target)
		}
	}
}

// mustBanTarget returns the target s names.
func mustBanTarget(t *testing.T, s string) banTarget {
	t.Helper()
	target, ok := parseBanTarget(s)
	if !ok {
		t.Fatalf("%s names no ban target", s)
	}
	return target
}

// TestBanListMatching ensures a ban applies to its host, or to the hosts of
// its subnet, until it ends, whichever way the host is written.
func TestBanListMatching(t *testing.T) {
	b := newBanList()
	now := time.Now()
	hour := now.Add(time.Hour)
	b.banPeer("10.0.0.1", now, hour)
	b.banPeer("2001:db8::1", now, hour)
	b.add(mustBanTarget(t, "192.168.1.0/24"), now, hour)
	b.add(mustBanTarget(t, "2001:db9::/32"), now, hour)
	b.add(mustBanTarget(t, "172.16.0.0/16"), now, now.Add(time.Second))

	for host, want := range map[string]bool{
		"10.0.0.1":             true,
		"10.0.0.2":             false,
		"::ffff:10.0.0.1":      true,
		"2001:0db8::1":         true,
		"192.168.1.200":        true,
		"::ffff:192.168.1.200": true,
		"192.168.2.1":          false,
		"2001:db9:1::5":        true,
		"2001:dba::1":          false,
		"example.onion":        false,
	} {
		if until, banned := b.bannedUntil(host, now); banned != want {
			t.Errorf("%s: banned %v, want %v", host, banned, want)
		} else if banned && until.IsZero() {
			t.Errorf("%s: no end of the ban", host)
		}
	}

	if _, banned := b.bannedUntil("172.16.0.1", now); !banned {
		t.Error("the subnet isn't banned before its ban ends")
	}
	if _, banned := b.bannedUntil("172.16.0.1", now.Add(time.Second)); banned {
		t.Error("the subnet is banned when its ban ends")
	}
	if _, banned := b.bannedUntil("10.0.0.1", hour); banned {
		t.Error("the host is banned when its ban ends")
	}
}

// TestBanListZones ensures the zone of a link-local peer's address doesn't
// hide it from a ban, and that its own ban is listed in a form setban takes.
func TestBanListZones(t *testing.T) {
	b := newBanList()
	now := time.Now()
	hour := now.Add(time.Hour)
	b.add(mustBanTarget(t, "fe80::1"), now, hour)
	b.add(mustBanTarget(t, "fe90::/16"), now, hour)
	for _, host := range []string{"fe80::1%eth0", "fe90::5%eth0"} {
		if _, banned := b.bannedUntil(host, now); !banned {
			t.Errorf("%s isn't banned", host)
		}
	}

	b.banPeer("fe80::2%eth0", now, hour)
	entries := b.list(now)
	address := entries[0].target.String()
	if address == "fe80::1/128" {
		address = entries[1].target.String()
	}
	if address != "fe80::2/128" {
		t.Fatalf("listed: %+v", entries)
	}
	if !b.remove(mustBanTarget(t, address)) {
		t.Fatal("the ban couldn't be lifted by its listed address")
	}
}

// TestBanListAdd ensures setban's bans follow BCHN: a banned host or subnet
// isn't banned again, a host in a banned subnet is taken to be banned, and a
// subnet isn't taken to be banned by a larger one or by its hosts.
func TestBanListAdd(t *testing.T) {
	b := newBanList()
	now := time.Now()
	hour := now.Add(time.Hour)
	for i, step := range []struct {
		target string
		want   bool
	}{
		{"10.0.0.0/8", true},
		{"10.0.0.0/8", false},
		{"10.1.2.3", false},
		{"10.1.0.0/16", true},
		{"192.168.0.1", true},
		{"192.168.0.1/32", false},
		{"192.168.0.0/24", true},
	} {
		if got := b.add(mustBanTarget(t, step.target), now, hour); got != step.want {
			t.Fatalf("step %d: adding %s returned %v", i, step.target, got)
		}
	}

	// A ban that ended is no ban, even before the sweep.
	if !b.add(mustBanTarget(t, "172.16.0.1"), now, now.Add(time.Second)) {
		t.Fatal("first ban refused")
	}
	later := now.Add(time.Minute)
	if !b.add(mustBanTarget(t, "172.16.0.1"), later, later.Add(time.Hour)) {
		t.Fatal("a ban of a host whose ban ended was refused")
	}
	if until, ok := b.bannedUntil("172.16.0.1", later); !ok ||
		!until.Equal(later.Add(time.Hour)) {

		t.Fatalf("banned until %v, %v", until, ok)
	}
}

// TestBanPeerKeepsLongerBan ensures a peer's ban score doesn't shorten a ban,
// and that a manual ban it extends stays manual.
func TestBanPeerKeepsLongerBan(t *testing.T) {
	b := newBanList()
	now := time.Now()
	long, short := now.Add(48*time.Hour), now.Add(time.Hour)

	b.banPeer("10.0.0.1", now, long)
	b.banPeer("10.0.0.1", now, short)
	if until, _ := b.bannedUntil("10.0.0.1", now); !until.Equal(long) {
		t.Fatalf("the ban was shortened to %v", until)
	}

	b.add(mustBanTarget(t, "10.0.0.2"), now, short)
	b.banPeer("10.0.0.2", now, long)
	if until, _ := b.bannedUntil("10.0.0.2", now); !until.Equal(long) {
		t.Fatalf("the ban wasn't extended, it ends at %v", until)
	}
	if entries := b.list(now); len(entries) != 2 || entries[0].manual ||
		!entries[1].manual {

		t.Fatalf("bans: %+v", entries)
	}
}

// TestBanPeerAfterManualBanEnded ensures a ban score's ban of a host whose
// manual ban ended, and isn't swept yet, isn't taken for a manual ban.
func TestBanPeerAfterManualBanEnded(t *testing.T) {
	b := newBanList()
	now := time.Now()
	b.add(mustBanTarget(t, "10.0.0.1"), now, now.Add(time.Minute))
	later := now.Add(time.Hour)
	b.banPeer("10.0.0.1", later, later.Add(time.Hour))
	if entries := b.list(later); len(entries) != 1 || entries[0].manual {
		t.Fatalf("bans: %+v", entries)
	}
}

// TestBanListRemove ensures a removal lifts exactly the named ban: the ban of
// a host, also one from a peer's ban score, or of a subnet, and not the ban of
// a subnet by naming a host in it.
func TestBanListRemove(t *testing.T) {
	b := newBanList()
	now := time.Now()
	hour := now.Add(time.Hour)
	b.banPeer("10.0.0.1", now, hour)
	b.add(mustBanTarget(t, "192.168.1.0/24"), now, hour)

	if b.remove(mustBanTarget(t, "10.0.0.2")) {
		t.Fatal("removed a ban that isn't there")
	}
	if !b.remove(mustBanTarget(t, "10.0.0.1/32")) {
		t.Fatal("the host's ban wasn't removed")
	}
	if _, banned := b.bannedUntil("10.0.0.1", now); banned {
		t.Fatal("the host is still banned")
	}
	if b.remove(mustBanTarget(t, "192.168.1.5")) {
		t.Fatal("a host in a banned subnet was removed")
	}
	if b.remove(mustBanTarget(t, "192.168.0.0/16")) {
		t.Fatal("a larger subnet was removed")
	}
	if _, banned := b.bannedUntil("192.168.1.5", now); !banned {
		t.Fatal("the subnet's ban was lifted")
	}
	if !b.remove(mustBanTarget(t, "192.168.1.9/24")) {
		t.Fatal("the subnet's ban wasn't removed")
	}
	if _, banned := b.bannedUntil("192.168.1.5", now); banned {
		t.Fatal("the subnet is still banned")
	}
}

// TestBanListClear ensures each kind of ban can be cleared on its own.
func TestBanListClear(t *testing.T) {
	now := time.Now()
	hour := now.Add(time.Hour)
	tests := []struct {
		manual, automatic bool
		want              []string
	}{
		{true, true, nil},
		{true, false, []string{"10.0.0.1/32"}},
		{false, true, []string{"10.0.0.2/32", "192.168.1.0/24"}},
		{false, false, []string{"10.0.0.1/32", "10.0.0.2/32", "192.168.1.0/24"}},
	}
	for _, test := range tests {
		b := newBanList()
		b.banPeer("10.0.0.1", now, hour)
		b.add(mustBanTarget(t, "10.0.0.2"), now, hour)
		b.add(mustBanTarget(t, "192.168.1.0/24"), now, hour)
		b.clear(test.manual, test.automatic)
		entries := b.list(now)
		if len(entries) != len(test.want) {
			t.Fatalf("%+v: bans left: %+v", test, entries)
		}
		for i, entry := range entries {
			if got := entry.target.String(); got != test.want[i] {
				t.Fatalf("%+v: bans left: %+v", test, entries)
			}
		}
	}
}

// TestBanListSweep ensures bans that ended are forgotten, from both tables,
// and aren't listed before that, while the others stay.
func TestBanListSweep(t *testing.T) {
	b := newBanList()
	now := time.Now()
	b.banPeer("10.0.0.1", now, now.Add(-time.Second))
	b.banPeer("10.0.0.2", now, now)
	b.banPeer("10.0.0.3", now, now.Add(time.Second))
	b.add(mustBanTarget(t, "192.168.1.0/24"), now, now.Add(-time.Second))
	b.add(mustBanTarget(t, "192.168.2.0/24"), now, now.Add(time.Second))

	if entries := b.list(now); len(entries) != 2 {
		t.Fatalf("listed: %+v", entries)
	}
	b.sweep(now)
	if len(b.hosts) != 1 || len(b.subnets) != 1 {
		t.Fatalf("left: %+v %+v", b.hosts, b.subnets)
	}
	if _, ok := b.hosts["10.0.0.3"]; !ok {
		t.Fatalf("the host in force was forgotten: %+v", b.hosts)
	}
	if _, ok := b.subnets["192.168.2.0/24"]; !ok {
		t.Fatalf("the subnet in force was forgotten: %+v", b.subnets)
	}
}
