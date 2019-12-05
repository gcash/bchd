package avalanche

import (
	"github.com/gcash/bchd/chaincfg/chainhash"
)

var (
	_ Receiver = rpcReceiver{}
	_ Receiver = compositeReceiver{}
)

// Receiver accepts events from the Manager.
type Receiver interface {
	PeerConnect(SignedIdentity)
	PeerDisconnect(SignedIdentity)
	NewVoteRecord(chainhash.Hash, VoteRecord)
	FinalizedVoteRecord(chainhash.Hash, VoteRecord)
}

// rpcReceiver accepts events and sends them to an RPC server.
type rpcReceiver struct{ rpcServer }
type rpcServer interface {
	NotifyAvalancheFinalization(chainhash.Hash, VoteRecord)
	NotifyAvalanchePeerConnect(SignedIdentity)
	NotifyAvalanchePeerDisconnect(SignedIdentity)
}

func NewRPCReceiver(s rpcServer) Receiver             { return rpcReceiver{s} }
func (rr rpcReceiver) PeerConnect(ssi SignedIdentity) { rr.NotifyAvalanchePeerConnect(ssi) }
func (rr rpcReceiver) PeerDisconnect(ssi SignedIdentity) {
	rr.NotifyAvalanchePeerDisconnect(ssi)
}
func (rr rpcReceiver) NewVoteRecord(chainhash.Hash, VoteRecord) {}
func (rr rpcReceiver) FinalizedVoteRecord(h chainhash.Hash, vr VoteRecord) {
	rr.NotifyAvalancheFinalization(h, vr)
}

// compositeReceiver fans events out to multiple Receivers.
type compositeReceiver []Receiver

func (cr compositeReceiver) PeerConnect(ssi SignedIdentity) {
	for _, r := range cr {
		if r == nil {
			continue
		}
		r.PeerConnect(ssi)
	}
}

func (cr compositeReceiver) PeerDisconnect(ssi SignedIdentity) {
	for _, r := range cr {
		if r == nil {
			continue
		}
		r.PeerDisconnect(ssi)
	}
}

func (cr compositeReceiver) NewVoteRecord(h chainhash.Hash, vr VoteRecord) {
	for _, r := range cr {
		if r == nil {
			continue
		}
		r.NewVoteRecord(h, vr)
	}
}

func (cr compositeReceiver) FinalizedVoteRecord(h chainhash.Hash, vr VoteRecord) {
	for _, r := range cr {
		if r == nil {
			continue
		}
		r.FinalizedVoteRecord(h, vr)
	}
}
