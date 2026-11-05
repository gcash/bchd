// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/base32"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// banTarget is what a ban applies to: a single host, or a subnet when subnet
// isn't nil.
type banTarget struct {
	host   string
	subnet *net.IPNet
}

// parseBanTarget returns the target an IP address, an onion address, or an IP
// address with a netmask names, or false when it names none.  As in BCHN, the
// netmask is a prefix length or an address, its bits past the netmask leave
// the address, and a mask of the whole address is a single host.
func parseBanTarget(s string) (banTarget, bool) {
	addr, maskStr, hasMask := strings.Cut(s, "/")
	if !hasMask && isOnionHost(s) {
		return banTarget{host: banHostKey(s)}, true
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return banTarget{}, false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if !hasMask {
		return banTarget{host: ip.String()}, true
	}
	mask, ok := parseBanMask(maskStr, len(ip))
	if !ok {
		return banTarget{}, false
	}
	if ones, bits := mask.Size(); ones == bits {
		return banTarget{host: ip.String()}, true
	}
	return banTarget{subnet: &net.IPNet{IP: ip.Mask(mask), Mask: mask}}, true
}

// isOnionHost returns whether s is an onion address, of Tor v2 or v3.
func isOnionHost(s string) bool {
	name, ok := strings.CutSuffix(strings.ToUpper(s), ".ONION")
	if !ok || (len(name) != 16 && len(name) != 56) {
		return false
	}
	_, err := base32.StdEncoding.DecodeString(name)
	return err == nil
}

// parseBanMask returns the netmask, a prefix length or an address, for an
// address of size bytes.
func parseBanMask(s string, size int) (net.IPMask, bool) {
	if n, err := strconv.ParseUint(s, 10, 8); err == nil {
		mask := net.CIDRMask(int(n), size*8)
		return mask, mask != nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, false
	}
	v4 := ip.To4()
	if (size == net.IPv4len) != (v4 != nil) {
		return nil, false
	}
	if v4 != nil {
		ip = v4
	}
	mask := net.IPMask(ip)

	// Size returns zeros for a mask with a one after a zero.
	if _, bits := mask.Size(); bits == 0 {
		return nil, false
	}
	return mask, true
}

// String returns the target as listbanned shows it: an IP address with its
// prefix length, as BCHN does, or an onion address as it is.
func (t banTarget) String() string {
	if t.subnet != nil {
		return t.subnet.String()
	}
	ip := net.ParseIP(t.host)
	switch {
	case ip == nil:
		return t.host
	case ip.To4() != nil:
		return t.host + "/32"
	}
	return t.host + "/128"
}

// banEntry is a ban in force from created until until.  A manual ban comes
// from setban, and the others from a peer's ban score.
type banEntry struct {
	target  banTarget
	created time.Time
	until   time.Time
	manual  bool
}

// banList holds the hosts and subnets that are banned, as BCHN's two tables:
// a ban by a peer's ban score is always of a host, and subnets, which only
// come from setban and are few, are the only bans that take a scan to find.
// It is safe for concurrent access.
type banList struct {
	mtx     sync.Mutex
	hosts   map[string]banEntry
	subnets map[string]banEntry
}

// newBanList returns an empty ban list.
func newBanList() *banList {
	return &banList{
		hosts:   make(map[string]banEntry),
		subnets: make(map[string]banEntry),
	}
}

// bannedUntil returns when the ban in force on host at now ends, the one of
// the host itself or of a subnet that contains it, and whether there is one.
func (b *banList) bannedUntil(host string, now time.Time) (time.Time, bool) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.findLocked(host, now)
}

// findLocked is bannedUntil with the mutex held.
func (b *banList) findLocked(host string, now time.Time) (time.Time, bool) {
	key := banHostKey(host)
	if entry, ok := b.hosts[key]; ok && now.Before(entry.until) {
		return entry.until, true
	}
	ip := net.ParseIP(key)
	if ip == nil {
		return time.Time{}, false
	}
	for _, entry := range b.subnets {
		if now.Before(entry.until) && entry.target.subnet.Contains(ip) {
			return entry.until, true
		}
	}
	return time.Time{}, false
}

// banHostKey returns host as the host table has it: one way of writing an IP
// address, so that an IPv4 address in IPv6 form is the same host, and with no
// zone, which a link-local peer's address has and a ban's doesn't.
func banHostKey(host string) string {
	host, _, _ = strings.Cut(host, "%")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return strings.ToLower(host)
}

// tableLocked returns the table of target and its key there.
func (b *banList) tableLocked(target banTarget) (map[string]banEntry, string) {
	if target.subnet != nil {
		return b.subnets, target.subnet.String()
	}
	return b.hosts, target.host
}

// addLocked puts entry in its table, unless a ban already there ends no
// sooner.  A manual ban in force stays manual when a peer's ban score extends
// it.
func (b *banList) addLocked(entry banEntry) {
	table, key := b.tableLocked(entry.target)
	old, ok := table[key]
	if ok && !old.until.Before(entry.until) {
		return
	}
	entry.manual = entry.manual || (ok && old.manual && entry.created.Before(old.until))
	table[key] = entry
}

// banPeer bans host from now until until, for its ban score.
func (b *banList) banPeer(host string, now, until time.Time) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.addLocked(banEntry{
		target:  banTarget{host: banHostKey(host)},
		created: now,
		until:   until,
	})
}

// add bans target from now until until, as setban asks for.  It returns false
// without changing anything if target is banned already: as in BCHN, a host
// by its own ban or a subnet's, and a subnet by a ban of that same subnet.
func (b *banList) add(target banTarget, now, until time.Time) bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if target.subnet == nil {
		if _, banned := b.findLocked(target.host, now); banned {
			return false
		}
	} else if old, ok := b.subnets[target.subnet.String()]; ok && now.Before(old.until) {
		return false
	}
	b.addLocked(banEntry{
		target:  target,
		created: now,
		until:   until,
		manual:  true,
	})
	return true
}

// remove lifts the ban of exactly target, and returns false if it has none.
// A host in a banned subnet stays banned.
func (b *banList) remove(target banTarget) bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	table, key := b.tableLocked(target)
	if _, ok := table[key]; !ok {
		return false
	}
	delete(table, key)
	return true
}

// clear lifts the bans of the chosen kinds.
func (b *banList) clear(manual, automatic bool) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	for _, table := range []map[string]banEntry{b.hosts, b.subnets} {
		for key, entry := range table {
			if (entry.manual && manual) || (!entry.manual && automatic) {
				delete(table, key)
			}
		}
	}
}

// list returns the bans in force at now, sorted by what they apply to.
func (b *banList) list(now time.Time) []banEntry {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	entries := make([]banEntry, 0, len(b.hosts)+len(b.subnets))
	for _, table := range []map[string]banEntry{b.hosts, b.subnets} {
		for _, entry := range table {
			if now.Before(entry.until) {
				entries = append(entries, entry)
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].target.String() < entries[j].target.String()
	})
	return entries
}

// sweep forgets the bans that ended by now.
func (b *banList) sweep(now time.Time) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	for _, table := range []map[string]banEntry{b.hosts, b.subnets} {
		for key, entry := range table {
			if !now.Before(entry.until) {
				delete(table, key)
			}
		}
	}
}
