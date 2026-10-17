// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"crypto/sha3"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// NetworkID identifies the network of an address in an addrv2 message as
// defined by BIP155.
type NetworkID uint8

const (
	// NetworkIPv4 is an IPv4 address.
	NetworkIPv4 NetworkID = 1

	// NetworkIPv6 is an IPv6 address.
	NetworkIPv6 NetworkID = 2

	// NetworkTorV2 is a Tor v2 onion address.  These are no longer
	// usable and are skipped when decoding.
	NetworkTorV2 NetworkID = 3

	// NetworkTorV3 is a Tor v3 onion address.
	NetworkTorV3 NetworkID = 4

	// NetworkI2P is an I2P address.
	NetworkI2P NetworkID = 5

	// NetworkCJDNS is a CJDNS address.
	NetworkCJDNS NetworkID = 6
)

// netIDStrings maps network IDs to their names.
var netIDStrings = map[NetworkID]string{
	NetworkIPv4:  "ipv4",
	NetworkIPv6:  "ipv6",
	NetworkTorV2: "torv2",
	NetworkTorV3: "torv3",
	NetworkI2P:   "i2p",
	NetworkCJDNS: "cjdns",
}

// String returns the NetworkID in human-readable form.
func (n NetworkID) String() string {
	if s, ok := netIDStrings[n]; ok {
		return s
	}
	return fmt.Sprintf("Unknown NetworkID (%d)", uint8(n))
}

// addrV2Size returns the address size in bytes for a known network and false
// for an unknown one.
func addrV2Size(n NetworkID) (int, bool) {
	switch n {
	case NetworkIPv4:
		return 4, true
	case NetworkIPv6:
		return 16, true
	case NetworkTorV2:
		return 10, true
	case NetworkTorV3:
		return TorV3Size, true
	case NetworkI2P:
		return 32, true
	case NetworkCJDNS:
		return 16, true
	}
	return 0, false
}

const (
	// TorV3Size is the size of a Tor v3 address, which is the ed25519
	// public key of the onion service.
	TorV3Size = 32

	// maxAddrV2Size is the largest address size allowed in an addrv2
	// message.
	maxAddrV2Size = 512

	// minNetAddressV2Payload is the smallest encoded NetAddressV2, an
	// empty address of an unknown network.
	minNetAddressV2Payload = 4 + 1 + 1 + 1 + 2

	// maxNetAddressV2Payload is the largest encoded NetAddressV2: a 4 byte
	// timestamp, services as a varint of up to 9 bytes, the network ID,
	// the address length varint (3 bytes for 512), the address and the
	// port.
	maxNetAddressV2Payload = 4 + 9 + 1 + 3 + maxAddrV2Size + 2
)

var (
	// ipv4InIPv6Prefix is the prefix of an IPv4-mapped IPv6 address.
	ipv4InIPv6Prefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}

	// onionCatPrefix is the IPv6 prefix that was used to embed Tor v2
	// addresses in addr messages.
	onionCatPrefix = []byte{0xfd, 0x87, 0xd8, 0x7e, 0xeb, 0x43}

	// errSkipAddrV2 means a decoded address is well formed but must be
	// ignored.
	errSkipAddrV2 = errors.New("skipped addrv2 address")
)

// NetAddressV2 defines information about a peer on the network as used in the
// addrv2 message (MsgAddrV2).  Unlike NetAddress it can hold the addresses of
// all BIP155 networks.
type NetAddressV2 struct {
	// Last time the address was seen.
	Timestamp time.Time

	// Bitfield which identifies the services supported by the address.
	Services ServiceFlag

	// Network is the BIP155 network of the address.
	Network NetworkID

	// Addr is the address in the encoding of its network.  IPv4 addresses
	// are 4 bytes.
	Addr []byte

	// Port the peer is using.
	Port uint16
}

// HasService returns whether the specified service is supported by the address.
func (na *NetAddressV2) HasService(service ServiceFlag) bool {
	return na.Services&service == service
}

// AddService adds service as a supported service by the peer generating the
// message.
func (na *NetAddressV2) AddService(service ServiceFlag) {
	na.Services |= service
}

// IP returns the address as a net.IP for IPv4 and IPv6 addresses and nil for
// the other networks.
func (na *NetAddressV2) IP() net.IP {
	switch na.Network {
	case NetworkIPv4:
		if len(na.Addr) == 4 {
			return net.IPv4(na.Addr[0], na.Addr[1], na.Addr[2], na.Addr[3])
		}
	case NetworkIPv6:
		if len(na.Addr) == 16 {
			return net.IP(bytes.Clone(na.Addr))
		}
	}
	return nil
}

// ToLegacy returns the address as a NetAddress for use in addr messages, or
// nil if the address can't be represented in one.
func (na *NetAddressV2) ToLegacy() *NetAddress {
	ip := na.IP()
	if ip == nil {
		return nil
	}
	return &NetAddress{
		Timestamp: na.Timestamp,
		Services:  na.Services,
		IP:        ip,
		Port:      na.Port,
	}
}

// Host returns the address without the port: an IP address, a .onion name
// for Tor, a .b32.i2p name for I2P and an IPv6 address for CJDNS.
func (na *NetAddressV2) Host() string {
	switch na.Network {
	case NetworkIPv4, NetworkIPv6, NetworkCJDNS:
		if size, _ := addrV2Size(na.Network); len(na.Addr) == size {
			return net.IP(na.Addr).String()
		}
	case NetworkTorV2:
		return strings.ToLower(base32.StdEncoding.EncodeToString(na.Addr)) +
			".onion"
	case NetworkTorV3:
		if len(na.Addr) == TorV3Size {
			return encodeTorV3(na.Addr)
		}
	case NetworkI2P:
		return strings.ToLower(base32.StdEncoding.WithPadding(
			base32.NoPadding).EncodeToString(na.Addr)) + ".b32.i2p"
	}
	return fmt.Sprintf("invalid %v address %x", na.Network, na.Addr)
}

// NewNetAddressV2 returns a new NetAddressV2 for the given network and
// address with the current time as the timestamp.
func NewNetAddressV2(network NetworkID, addr []byte, port uint16,
	services ServiceFlag) *NetAddressV2 {

	return &NetAddressV2{
		Timestamp: time.Unix(time.Now().Unix(), 0),
		Services:  services,
		Network:   network,
		Addr:      addr,
		Port:      port,
	}
}

// NewNetAddressV2IPPort returns a new NetAddressV2 using the provided IP, port,
// and supported services with the current time as the timestamp.
func NewNetAddressV2IPPort(ip net.IP, port uint16, services ServiceFlag) *NetAddressV2 {
	return NetAddressV2FromLegacy(NewNetAddressIPPort(ip, port, services))
}

// NetAddressV2FromLegacy converts a NetAddress to a NetAddressV2.  An
// IPv4-mapped IPv6 address becomes an IPv4 address.
func NetAddressV2FromLegacy(na *NetAddress) *NetAddressV2 {
	nav2 := &NetAddressV2{
		Timestamp: na.Timestamp,
		Services:  na.Services,
		Port:      na.Port,
	}
	if ip4 := na.IP.To4(); ip4 != nil {
		nav2.Network = NetworkIPv4
		nav2.Addr = bytes.Clone(ip4)
		return nav2
	}
	nav2.Network = NetworkIPv6
	nav2.Addr = make([]byte, 16)
	copy(nav2.Addr, na.IP.To16())
	return nav2
}

// torV3Checksum returns the checksum of a Tor v3 address:
// SHA3-256(".onion checksum" | pubkey | version)[:2].
func torV3Checksum(pubKey []byte) []byte {
	h := sha3.New256()
	h.Write([]byte(".onion checksum"))
	h.Write(pubKey)
	h.Write([]byte{torV3Version})
	return h.Sum(nil)[:2]
}

// torV3Version is the version byte of a Tor v3 address.
const torV3Version = 3

// encodeTorV3 returns the .onion name of a Tor v3 public key.
func encodeTorV3(pubKey []byte) string {
	b := make([]byte, 0, TorV3Size+3)
	b = append(b, pubKey...)
	b = append(b, torV3Checksum(pubKey)...)
	b = append(b, torV3Version)
	return strings.ToLower(base32.StdEncoding.EncodeToString(b)) + ".onion"
}

// DecodeTorV3 returns the public key of a Tor v3 .onion name.  It returns an
// error for any other name, including Tor v2 names, and when the checksum or
// version doesn't match.
func DecodeTorV3(host string) ([]byte, error) {
	name, ok := strings.CutSuffix(host, ".onion")
	if !ok {
		return nil, fmt.Errorf("%q is not an onion address", host)
	}
	if len(name) == 16 {
		return nil, fmt.Errorf("tor v2 address %s is not supported", host)
	}
	if len(name) != 56 {
		return nil, fmt.Errorf("invalid onion address %q", host)
	}
	b, err := base32.StdEncoding.DecodeString(strings.ToUpper(name))
	if err != nil || len(b) != TorV3Size+3 {
		return nil, fmt.Errorf("invalid onion address %q", host)
	}
	pubKey := b[:TorV3Size]
	if b[TorV3Size+2] != torV3Version ||
		!bytes.Equal(b[TorV3Size:TorV3Size+2], torV3Checksum(pubKey)) {

		return nil, fmt.Errorf("invalid onion address %q: bad "+
			"checksum or version", host)
	}
	return pubKey, nil
}

// DecodeI2P returns the address of an I2P .b32.i2p name.
func DecodeI2P(host string) ([]byte, error) {
	name, ok := strings.CutSuffix(host, ".b32.i2p")
	if !ok || len(name) != 52 {
		return nil, fmt.Errorf("invalid i2p address %q", host)
	}
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).
		DecodeString(strings.ToUpper(name))
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("invalid i2p address %q", host)
	}
	return b, nil
}

// readNetAddressV2 reads an encoded NetAddressV2 from r.  It returns
// errSkipAddrV2 for an address that is well formed but must be ignored: one of
// an unknown network, a Tor v2 address, a CJDNS address outside fc00::/8, or
// an IPv6 address that embeds an IPv4 or Tor v2 address.
func readNetAddressV2(r io.Reader, pver uint32, na *NetAddressV2) error {
	err := readElement(r, (*uint32Time)(&na.Timestamp))
	if err != nil {
		return err
	}

	services, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	na.Services = ServiceFlag(services)

	var netID uint8
	if err := readElement(r, &netID); err != nil {
		return err
	}
	na.Network = NetworkID(netID)

	addrLen, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if addrLen > maxAddrV2Size {
		str := fmt.Sprintf("address too long [len %v, max %v]",
			addrLen, maxAddrV2Size)
		return messageError("readNetAddressV2", str)
	}
	size, known := addrV2Size(na.Network)
	if known && addrLen != uint64(size) {
		str := fmt.Sprintf("invalid %v address length [len %v, "+
			"want %v]", na.Network, addrLen, size)
		return messageError("readNetAddressV2", str)
	}

	na.Addr = make([]byte, addrLen)
	if _, err := io.ReadFull(r, na.Addr); err != nil {
		return err
	}
	na.Port, err = binarySerializer.Uint16(r, bigEndian)
	if err != nil {
		return err
	}

	switch {
	case !known, na.Network == NetworkTorV2:
		return errSkipAddrV2
	case na.Network == NetworkCJDNS && na.Addr[0] != 0xfc:
		// CJDNS addresses are in fc00::/8.
		return errSkipAddrV2
	case na.Network == NetworkIPv6 &&
		(bytes.HasPrefix(na.Addr, ipv4InIPv6Prefix) ||
			bytes.HasPrefix(na.Addr, onionCatPrefix)):
		return errSkipAddrV2
	}
	return nil
}

// writeNetAddressV2 serializes a NetAddressV2 to w.
func writeNetAddressV2(w io.Writer, pver uint32, na *NetAddressV2) error {
	size, known := addrV2Size(na.Network)
	if !known || len(na.Addr) != size {
		str := fmt.Sprintf("invalid %v address length %d",
			na.Network, len(na.Addr))
		return messageError("writeNetAddressV2", str)
	}

	err := writeElement(w, uint32(na.Timestamp.Unix()))
	if err != nil {
		return err
	}
	if err := WriteVarInt(w, pver, uint64(na.Services)); err != nil {
		return err
	}
	if err := writeElement(w, uint8(na.Network)); err != nil {
		return err
	}
	if err := WriteVarBytes(w, pver, na.Addr); err != nil {
		return err
	}
	return binary.Write(w, bigEndian, na.Port)
}
