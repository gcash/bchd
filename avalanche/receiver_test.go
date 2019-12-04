package avalanche

import (
	"testing"

	"github.com/tfactorapp/assert"

	"github.com/gcash/bchd/chaincfg/chainhash"
)

func TestRPCReceiver(t *testing.T) {
	tsrs := &tripSetRPCServer{}
	rcp := NewRPCReceiver(tsrs)

	assert.False(t, tsrs.calledNotifyAvalancheFinalization)
	rcp.FinalizedVoteRecord(chainhash.Hash{}, VoteRecord{})
	assert.True(t, tsrs.calledNotifyAvalancheFinalization)

	assert.False(t, tsrs.calledNotifyAvalanchePeerConnect)
	rcp.PeerConnect(SignedIdentity{})
	assert.True(t, tsrs.calledNotifyAvalanchePeerConnect)

	assert.False(t, tsrs.calledNotifyAvalanchePeerDisconnect)
	rcp.PeerDisconnect(SignedIdentity{})
	assert.True(t, tsrs.calledNotifyAvalanchePeerDisconnect)
}

func TestCompositeReceiver(t *testing.T) {
	var (
		tsr1 = &tripSetReceiver{}
		tsr2 = &tripSetReceiver{}
		cr   = compositeReceiver{tsr1, tsr2}
	)

	assert.False(t, tsr1.calledPeerConnect)
	assert.False(t, tsr2.calledPeerConnect)
	cr.PeerConnect(SignedIdentity{})
	assert.True(t, tsr1.calledPeerConnect)
	assert.True(t, tsr2.calledPeerConnect)

	assert.False(t, tsr1.calledPeerDisconnect)
	assert.False(t, tsr2.calledPeerDisconnect)
	cr.PeerDisconnect(SignedIdentity{})
	assert.True(t, tsr1.calledPeerDisconnect)
	assert.True(t, tsr2.calledPeerDisconnect)

	assert.False(t, tsr1.calledNewVoteRecord)
	assert.False(t, tsr2.calledNewVoteRecord)
	cr.NewVoteRecord(chainhash.Hash{}, VoteRecord{})
	assert.True(t, tsr1.calledNewVoteRecord)
	assert.True(t, tsr2.calledNewVoteRecord)

	assert.False(t, tsr1.calledFinalizedVoteRecord)
	assert.False(t, tsr2.calledFinalizedVoteRecord)
	cr.FinalizedVoteRecord(chainhash.Hash{}, VoteRecord{})
	assert.True(t, tsr1.calledFinalizedVoteRecord)
	assert.True(t, tsr2.calledFinalizedVoteRecord)
}

type tripSetReceiver struct {
	calledPeerConnect         bool
	calledPeerDisconnect      bool
	calledNewVoteRecord       bool
	calledFinalizedVoteRecord bool
}

func (tsr *tripSetReceiver) PeerConnect(SignedIdentity)               { tsr.calledPeerConnect = true }
func (tsr *tripSetReceiver) PeerDisconnect(SignedIdentity)            { tsr.calledPeerDisconnect = true }
func (tsr *tripSetReceiver) NewVoteRecord(chainhash.Hash, VoteRecord) { tsr.calledNewVoteRecord = true }
func (tsr *tripSetReceiver) FinalizedVoteRecord(chainhash.Hash, VoteRecord) {
	tsr.calledFinalizedVoteRecord = true
}

type tripSetRPCServer struct {
	calledNotifyAvalancheFinalization   bool
	calledNotifyAvalanchePeerConnect    bool
	calledNotifyAvalanchePeerDisconnect bool
}

func (tsrs *tripSetRPCServer) NotifyAvalancheFinalization(chainhash.Hash, VoteRecord) {
	tsrs.calledNotifyAvalancheFinalization = true
}

func (tsrs *tripSetRPCServer) NotifyAvalanchePeerConnect(SignedIdentity) {
	tsrs.calledNotifyAvalanchePeerConnect = true
}

func (tsrs *tripSetRPCServer) NotifyAvalanchePeerDisconnect(SignedIdentity) {
	tsrs.calledNotifyAvalanchePeerDisconnect = true
}
