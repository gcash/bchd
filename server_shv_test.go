// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"slices"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/shv"
	"github.com/gcash/bchd/wire"
)

// shvHarness is a server peer backed by a regtest chain and the server's SHV
// index, whose remote side the test drives.
type shvHarness struct {
	s      *server
	sp     *serverPeer
	chain  *blockchain.BlockChain
	remote net.Conn

	// services are the services the server peer advertised in its version
	// message.
	services wire.ServiceFlag

	// received holds the shv and pong messages the remote side read, in
	// the order they came.
	received chan wire.Message
}

// newSHVHarness returns a harness whose chain has the passed number of blocks
// after genesis, with SHV disabled when noSHV is set, as --noshv does.
func newSHVHarness(t *testing.T, noSHV bool, blocks int) *shvHarness {
	t.Helper()
	useTestConfig(t, &config{BanThreshold: defaultBanThreshold, NoSHV: noSHV})
	chain, _ := newTestChainDB(t)
	quietSHVLog(t)
	params := &chaincfg.RegressionNetParams
	s := &server{
		chainParams: params,
		chain:       chain,
		services:    defaultServices,
		banPeers:    make(chan *serverPeer, 1),
		quit:        make(chan struct{}),
	}
	s.initSHVIndex(chain)
	for range blocks {
		mineSHVBlock(t, chain, 0)
	}

	sp := newServerPeer(s, false)
	listeners := newPeerConfig(sp).Listeners
	sp.Peer = peer.NewInboundPeer(&peer.Config{
		ChainParams:            params,
		ProtocolVersion:        peer.MaxProtocolVersion,
		Services:               s.services,
		TstAllowSelfConnection: true,
		Listeners: peer.MessageListeners{
			OnGetSHV: listeners.OnGetSHV,
		},
	})
	local, remote := net.Pipe()
	sp.AssociateConnection(&addrConn{Conn: local,
		remote: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 18444}})
	t.Cleanup(func() {
		sp.Disconnect()
		remote.Close()
		sp.WaitForDisconnect()
	})

	h := &shvHarness{s: s, sp: sp, chain: chain, remote: remote,
		received: make(chan wire.Message, 64)}

	pver := wire.ProtocolVersion
	me := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.1"), 18444, 0)
	you := wire.NewNetAddressIPPort(net.ParseIP("10.0.0.2"), 18444, 0)
	if err := wire.WriteMessage(remote, wire.NewMsgVersion(me, you, 1, 0),
		pver, params.Net); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		msg, _, err := wire.ReadMessage(remote, pver, params.Net)
		if err != nil {
			t.Fatal(err)
		}
		if version, ok := msg.(*wire.MsgVersion); ok {
			h.services = version.Services
		}
	}
	if err := wire.WriteMessage(remote, wire.NewMsgVerAck(), pver,
		params.Net); err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			msg, _, err := wire.ReadMessage(remote, pver, params.Net)
			if err != nil {
				return
			}
			switch msg.(type) {
			case *wire.MsgSHV, *wire.MsgPong:
				h.received <- msg
			}
		}
	}()
	return h
}

// ask sends a getshv message of the passed requests from the remote side, and
// returns the shv messages the server peer sent before it answered a ping
// sent after it, so all it sent for the request.
func (h *shvHarness) ask(t *testing.T, nonce uint64, requests ...wire.SHVRequest) []*wire.MsgSHV {
	t.Helper()
	params := h.s.chainParams
	for _, msg := range []wire.Message{wire.NewMsgGetSHV(requests...),
		wire.NewMsgPing(nonce)} {

		if err := wire.WriteMessage(h.remote, msg, wire.ProtocolVersion,
			params.Net); err != nil {
			t.Fatal(err)
		}
	}

	// The peer sends its messages in order, so the replies come before the
	// pong.
	var replies []*wire.MsgSHV
	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg := <-h.received:
			switch m := msg.(type) {
			case *wire.MsgSHV:
				replies = append(replies, m)
			case *wire.MsgPong:
				if m.Nonce == nonce {
					return replies
				}
			}
		case <-deadline:
			t.Fatal("no pong")
		}
	}
}

// checkSHVResponse ensures resp answers the request for the block at height
// in the commitment at commitment, of the passed type, with the block's
// header and a proof that an accumulator over the main chain verifies.
func checkSHVResponse(t *testing.T, hashes []chainhash.Hash, resp *wire.SHVResponse,
	typ uint8, commitment, height uint64) {

	t.Helper()
	if resp.Status != wire.SHVStatusFound || resp.Type != typ ||
		resp.CommitmentHeight != commitment || resp.BlockHeight != height {

		t.Fatalf("response status %d, type %d, commitment %d, height %d, "+
			"want %d %d %d %d", resp.Status, resp.Type,
			resp.CommitmentHeight, resp.BlockHeight, wire.SHVStatusFound,
			typ, commitment, height)
	}
	leaf := resp.Header.BlockHash()
	if leaf != hashes[height] {
		t.Fatalf("block %d: header of %v, want %v", height, leaf,
			hashes[height])
	}
	acc, err := shv.NewAccumulator(0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range hashes[:commitment+1] {
		if err := acc.Extend(&hashes[i]); err != nil {
			t.Fatal(err)
		}
	}
	if typ&wire.SHVProofToRoot != 0 {
		if resp.Target != acc.Root() ||
			!acc.VerifyProofToRoot(height, &leaf, resp.Proof) {

			t.Fatalf("block %d, commitment %d: proof to the root not "+
				"verified", height, commitment)
		}
		return
	}
	if !slices.Contains(acc.Peaks(), resp.Target) ||
		!acc.VerifyProofToPeak(height, &leaf, resp.Proof) {

		t.Fatalf("block %d, commitment %d: proof to the peak not verified",
			height, commitment)
	}
}

// checkSHVStatus ensures resp is the passed status and nothing after it.
func checkSHVStatus(t *testing.T, resp *wire.SHVResponse, status uint8) {
	t.Helper()
	if resp.Status != status || resp.SerializeSize() != 1 {
		t.Fatalf("status %d, size %d, want status %d", resp.Status,
			resp.SerializeSize(), status)
	}
}

// TestSHVServerAnswers ensures a node with SHV advertises SFNodeSHV and
// answers a getshv message with one shv message of one entry per request, in
// order, found entries carrying the proof and the others the status for a
// reserved type, a block above the commitment, a commitment above the tip or
// a block the main chain does not have.  An empty getshv message gets an
// empty shv message, and 100 requests are all answered.
func TestSHVServerAnswers(t *testing.T) {
	h := newSHVHarness(t, false, 12)
	if h.services&wire.SFNodeSHV == 0 {
		t.Fatalf("services %v advertised without SFNodeSHV", h.services)
	}
	hashes := mainChainHashes(t, h.chain)

	reserved := wire.NewSHVRequestByHeight(false, 10, 3)
	reserved.Type |= 0x04
	unknown := chainhash.Hash{0x01}
	requests := []wire.SHVRequest{
		wire.NewSHVRequestByHeight(false, 10, 3),
		reserved,
		wire.NewSHVRequestByHash(true, 12, &hashes[7]),
		wire.NewSHVRequestByHeight(false, 10, 11),
		wire.NewSHVRequestByHeight(true, 13, 1),
		wire.NewSHVRequestByHash(true, 12, &unknown),
		wire.NewSHVRequestByHeight(true, 0, 0),
		wire.NewSHVRequestByHeight(false, 12, 12),
		wire.NewSHVRequestByHash(false, 5, &hashes[9]),
		wire.NewSHVRequestByHash(false, 12, &hashes[9]),
	}
	replies := h.ask(t, 1, requests...)
	if len(replies) != 1 || len(replies[0].Responses) != len(requests) {
		t.Fatalf("got %d replies, want one of %d responses", len(replies),
			len(requests))
	}
	resps := replies[0].Responses
	checkSHVResponse(t, hashes, &resps[0], 0, 10, 3)
	checkSHVStatus(t, &resps[1], wire.SHVStatusReservedType)
	checkSHVResponse(t, hashes, &resps[2], wire.SHVByHash|wire.SHVProofToRoot,
		12, 7)
	checkSHVStatus(t, &resps[3], wire.SHVStatusAboveCommitment)
	checkSHVStatus(t, &resps[4], wire.SHVStatusCommitmentAboveTip)
	checkSHVStatus(t, &resps[5], wire.SHVStatusNotFound)
	checkSHVResponse(t, hashes, &resps[6], wire.SHVProofToRoot, 0, 0)
	checkSHVResponse(t, hashes, &resps[7], 0, 12, 12)
	checkSHVStatus(t, &resps[8], wire.SHVStatusAboveCommitment)
	checkSHVResponse(t, hashes, &resps[9], wire.SHVByHash, 12, 9)

	// The genesis block proven in a commitment of itself, and the last
	// block of an odd commitment proven to its peak, have empty proofs.
	if len(resps[6].Proof) != 0 || resps[6].Target != hashes[0] ||
		len(resps[7].Proof) != 0 || resps[7].Target != hashes[12] {

		t.Error("genesis or lone peak proofs not empty")
	}

	// A reserved request is answered with that status.  An empty getshv
	// message gets an empty shv message.
	replies = h.ask(t, 2, reserved)
	if len(replies) != 1 || len(replies[0].Responses) != 1 {
		t.Fatalf("got %d replies to a reserved request", len(replies))
	}
	checkSHVStatus(t, &replies[0].Responses[0], wire.SHVStatusReservedType)
	replies = h.ask(t, 3)
	if len(replies) != 1 || len(replies[0].Responses) != 0 {
		t.Fatalf("got %d replies, want one empty one", len(replies))
	}

	// A block that left the main chain can't be proven by its hash, while
	// the block that replaced it can.
	if err := h.chain.InvalidateBlock(&hashes[12]); err != nil {
		t.Fatal(err)
	}
	mineSHVBlock(t, h.chain, 1)
	newHashes := mainChainHashes(t, h.chain)
	replies = h.ask(t, 4, wire.NewSHVRequestByHash(true, 12, &hashes[12]),
		wire.NewSHVRequestByHash(true, 12, &newHashes[12]))
	if len(replies) != 1 || len(replies[0].Responses) != 2 {
		t.Fatalf("got %d replies, want one of two responses", len(replies))
	}
	checkSHVStatus(t, &replies[0].Responses[0], wire.SHVStatusNotFound)
	checkSHVResponse(t, newHashes, &replies[0].Responses[1],
		wire.SHVByHash|wire.SHVProofToRoot, 12, 12)

	// 100 requests are all answered.
	full := make([]wire.SHVRequest, wire.MaxSHVRequestsPerMsg)
	for i := range full {
		full[i] = wire.NewSHVRequestByHeight(i%2 == 0, 12, uint64(i % 13))
	}
	replies = h.ask(t, 5, full...)
	if len(replies) != 1 ||
		len(replies[0].Responses) != wire.MaxSHVRequestsPerMsg {

		t.Fatalf("got %d replies to 100 requests", len(replies))
	}
	for i := range replies[0].Responses {
		checkSHVResponse(t, newHashes, &replies[0].Responses[i],
			full[i].Type, 12, uint64(i % 13))
	}
	if score := h.sp.banScore.Int(); score != 0 {
		t.Fatalf("ban score %d after valid requests", score)
	}
}

// TestSHVServerTooManyRequests ensures a getshv message of more than 100
// requests gets one shv message with no entries and adds BCHN's 20 to the
// peer's ban score, without disconnecting it.
func TestSHVServerTooManyRequests(t *testing.T) {
	h := newSHVHarness(t, false, 3)
	requests := make([]wire.SHVRequest, wire.MaxSHVRequestsPerMsg + 1)
	for i := range requests {
		requests[i] = wire.NewSHVRequestByHeight(true, 3, 1)
	}
	replies := h.ask(t, 1, requests...)
	if len(replies) != 1 || len(replies[0].Responses) != 0 {
		t.Fatalf("got %d replies to 101 requests", len(replies))
	}
	if score := h.sp.banScore.Int(); score != 20 {
		t.Fatalf("ban score %d, want 20", score)
	}

	// The peer is still served.
	replies = h.ask(t, 2, wire.NewSHVRequestByHeight(true, 3, 1))
	if len(replies) != 1 || len(replies[0].Responses) != 1 {
		t.Fatalf("got %d replies after the oversized request",
			len(replies))
	}
}

// TestSHVServerDisabled ensures a node run with --noshv does not advertise
// SFNodeSHV and ignores getshv messages without a word or a ban score
// increase.
func TestSHVServerDisabled(t *testing.T) {
	h := newSHVHarness(t, true, 3)
	if h.services&wire.SFNodeSHV != 0 || h.s.shvIndex != nil {
		t.Fatalf("services %v advertised, index %v", h.services,
			h.s.shvIndex)
	}
	requests := make([]wire.SHVRequest, wire.MaxSHVRequestsPerMsg + 1)
	for i := range requests {
		requests[i] = wire.NewSHVRequestByHeight(true, 3, 1)
	}
	for nonce, reqs := range [][]wire.SHVRequest{requests[:1], requests} {
		if replies := h.ask(t, uint64(nonce) + 1, reqs...); len(replies) != 0 {
			t.Fatalf("got %d replies with SHV disabled", len(replies))
		}
	}
	if score := h.sp.banScore.Int(); score != 0 {
		t.Fatalf("ban score %d with SHV disabled", score)
	}
}
