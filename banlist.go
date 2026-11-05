// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/base32"
	"encoding/json"
	"net"
	"os"
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
//
// Manual bans are kept in file, when it is set, and those of a peer's ban score
// end with the process, as BCHN's discouragements do.
type banList struct {
	mtx     sync.Mutex
	hosts   map[string]banEntry
	subnets map[string]banEntry
	file    string
}

// newBanList returns an empty ban list that keeps nothing on disk.
func newBanList() *banList {
	return &banList{
		hosts:   make(map[string]banEntry),
		subnets: make(map[string]banEntry),
	}
}

// savedBan is a manual ban in the file of a ban list, with its times in
// seconds since the Unix epoch.
type savedBan struct {
	Address string `json:"address"`
	Created int64  `json:"created"`
	Until   int64  `json:"until"`
}

// loadBanList returns a ban list that keeps its manual bans in file, with those
// there that haven't ended.  A file that can't be read is logged and starts an
// empty list, as a damaged list of bans is no reason not to run.
func loadBanList(file string, now time.Time) *banList {
	b := newBanList()

	// The file is set last, so that adding its bans doesn't write it.
	defer func() { b.file = file }()
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return b
	}
	var saved []savedBan
	if err == nil {
		err = json.Unmarshal(data, &saved)
	}
	if err != nil {
		srvrLog.Errorf("Failed to read the bans in %s: %v", file, err)
		return b
	}
	for _, ban := range saved {
		target, ok := parseBanTarget(ban.Address)
		if !ok {
			srvrLog.Warnf("Skipping the invalid ban %q in %s", ban.Address, file)
			continue
		}
		until := time.Unix(ban.Until, 0)
		if now.Before(until) {
			b.addLocked(banEntry{
				target:  target,
				created: time.Unix(ban.Created, 0),
				until:   until,
				manual:  true,
			})
		}
	}
	return b
}

// saveLocked writes the manual bans to the file, through a synced temporary
// file so that a crash leaves the old list whole.  A failure is only logged, as the bans
// stay in force until the process ends.  The file is small, and it is written
// only when a manual ban changes, so the mutex is held through the write to
// keep the writes in order.
func (b *banList) saveLocked() {
	if b.file == "" {
		return
	}
	saved := []savedBan{}
	for _, table := range []map[string]banEntry{b.hosts, b.subnets} {
		for _, entry := range table {
			if entry.manual {
				saved = append(saved, savedBan{
					Address: entry.target.String(),
					Created: entry.created.Unix(),
					Until:   entry.until.Unix(),
				})
			}
		}
	}
	sort.Slice(saved, func(i, j int) bool {
		return saved[i].Address < saved[j].Address
	})
	data, err := json.MarshalIndent(saved, "", "  ")
	if err == nil {
		err = replaceBanFile(b.file, data)
	}
	if err != nil {
		srvrLog.Errorf("Failed to save the bans in %s: %v", b.file, err)
	}
}

// replaceBanFile replaces file with data through a temporary file.
func replaceBanFile(file string, data []byte) error {
	tmp := file + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, file)
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
	if entry.manual {
		b.saveLocked()
	}
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
	entry, ok := table[key]
	if !ok {
		return false
	}
	delete(table, key)
	if entry.manual {
		b.saveLocked()
	}
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
	if manual {
		b.saveLocked()
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

// sweep forgets the bans that ended by now.  The file keeps those until it is
// next written, as they are left out when it is read.
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
