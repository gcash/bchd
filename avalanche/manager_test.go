package avalanche

import (
	"net"
	"testing"

	"github.com/gcash/bchlog"
	"github.com/tfactorapp/assert"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/wire"
)

func TestManager(t *testing.T) {
	privKey, err := bchec.NewPrivateKey(bchec.S256())
	assert.NoError(t, err)

	m, err := New(*privKey, nil)
	assert.NoError(t, err)
	go m.Start()

	assert.EqualString(t,
		string(privKey.PubKey().SerializeCompressed()),
		string(m.Identity().PubKey.SerializeCompressed()))

	m.Stop()
}

func TestManagerPeers(t *testing.T) {
	privKey, err := bchec.NewPrivateKey(bchec.S256())
	assert.NoError(t, err)

	m, err := New(*privKey)
	assert.NoError(t, err)

	p1, err := newStubPeer()
	assert.NoError(t, err)
	p2, err := newStubPeer()
	assert.NoError(t, err)

	// Peers aren't connected
	assert.False(t, m.IsAddrConnected(p1.addr))
	assert.False(t, m.IsAddrConnected(p2.addr))

	// Add both peers
	assert.EqualInt(t, 0, len(m.peers))
	m.NewPeer(p1, &SignedIdentity{})
	assert.EqualInt(t, 1, len(m.peers))
	assert.True(t, m.IsAddrConnected(p1.addr))
	assert.False(t, m.IsAddrConnected(p2.addr))

	m.NewPeer(p2, &SignedIdentity{})
	assert.EqualInt(t, 2, len(m.peers))
	assert.True(t, m.IsAddrConnected(p1.addr))
	assert.True(t, m.IsAddrConnected(p2.addr))

	// Trying to add them again is a no-op
	m.NewPeer(p1, &SignedIdentity{})
	m.NewPeer(p2, &SignedIdentity{})
	m.NewPeer(p1, &SignedIdentity{})
	m.NewPeer(p2, &SignedIdentity{})
	assert.EqualInt(t, 2, len(m.peers))
	assert.True(t, m.IsAddrConnected(p1.addr))
	assert.True(t, m.IsAddrConnected(p2.addr))

	// Get a random peer a bunch and check that it was roughly uniform
	counts := [2]int{0, 0}
	for i := 0; i < 1000; i++ {
		counts[getRandomPeer(m.peers).ID()]++
	}
	assert.IntsWithin(t, counts[0], counts[1], 150)

	// Remove peers
	m.DonePeer(p1)
	assert.EqualInt(t, 1, len(m.peers))
	assert.False(t, m.IsAddrConnected(p1.addr))
	assert.True(t, m.IsAddrConnected(p2.addr))

	m.DonePeer(p2)
	assert.EqualInt(t, 0, len(m.peers))
	assert.False(t, m.IsAddrConnected(p1.addr))
	assert.False(t, m.IsAddrConnected(p2.addr))

	// Doesn't fail when trying to removing non-existing peers
	m.DonePeer(p1)
	m.DonePeer(p2)

	// Nil returned for when trying to get random peers
	assert.True(t, getRandomPeer(nil) == nil)
	assert.True(t, getRandomPeer(m.peers) == nil)
}

func TestQueryKey(t *testing.T) {
	for _, test := range []struct {
		peerID   int32
		queryID  uint64
		expected string
	}{
		{0, 0, "0|0"},
		{1, 0, "1|0"},
		{0, 1, "0|1"},
		{1, 1, "1|1"},
		{43, 99, "43|99"},
	} {
		assert.EqualString(t, test.expected, queryKey(test.peerID, test.queryID))
	}
}

func TestUseLogger(t *testing.T) {
	log = nil
	assert.False(t, log == bchlog.Disabled)
	UseLogger(bchlog.Disabled)
	assert.True(t, log == bchlog.Disabled)
}

type stubPeer struct {
	id    int32
	addr  *wire.NetAddress
	key   *bchec.PrivateKey
	queue chan (wire.Message)
}

var stubPeerID int32 = -1

func newStubPeer() (stubPeer, error) {
	k, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		return stubPeer{}, err
	}
	stubPeerID++
	return stubPeer{
		stubPeerID,
		wire.NewNetAddressIPPort(net.ParseIP("1.2.3.4"), 3000+uint16(stubPeerID), 0),
		k,
		make(chan wire.Message, 1024),
	}, nil
}

func (p stubPeer) ID() int32                                      { return p.id }
func (p stubPeer) NA() *wire.NetAddress                           { return p.addr }
func (p stubPeer) AvalanchePubkey() *bchec.PublicKey              { return p.key.PubKey() }
func (p stubPeer) QueueMessage(m wire.Message, _ chan<- struct{}) { p.queue <- m }
