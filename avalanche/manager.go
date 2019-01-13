package avalanche

import (
	"github.com/gcash/bchd/peer"
	"sync"
)

// newPeerMsg signifies a newly connected peer to the handler.
type newPeerMsg struct {
	peer  *peer.Peer
}

// donePeerMsg signifies a disconnected peer to the handler.
type donePeerMsg struct {
	peer  *peer.Peer
}


type AvalancheManager struct {
	peers map[*peer.Peer]struct{}
	wg sync.WaitGroup
	quit           chan struct{}
	msgChan        chan interface{}
}

func New() *AvalancheManager {
	return &AvalancheManager{
		peers: make(map[*peer.Peer]struct{}),
		wg: sync.WaitGroup{},
		quit: make(chan struct{}),
		msgChan: make(chan interface{}),
	}
}

// Start begins the core handler which processes peers and avalanche messages.
func (am *AvalancheManager) Start() {
	am.wg.Add(1)
	go am.handler()
}

// Stop gracefully shuts down the avalanche manager by stopping all asynchronous
// handlers and waiting for them to finish.
func (am *AvalancheManager) Stop() {
	close(am.quit)
	am.wg.Wait()
}

func (am *AvalancheManager) handler() {
	out:
	for {
		select {
		case m := <-am.msgChan:
			switch msg := m.(type) {
			case *newPeerMsg:
				am.handleNewPeer(msg.peer)
			case *donePeerMsg:
				am.handleDonePeer(msg.peer)
			}

		case <-am.quit:
			break out
		}
	}
	am.wg.Done()
}

// NewPeer adds a new peer to the manager
func (am *AvalancheManager) NewPeer(p *peer.Peer) {
	am.msgChan <- &newPeerMsg{p}
}

func (am *AvalancheManager) handleNewPeer(p *peer.Peer) {
	log.Infof("New avalanche peer %s (%s)", p, p.UserAgent())
	am.peers[p] = struct{}{}
}

// DonePeer removes a peer from the manager
func (am *AvalancheManager) DonePeer(p *peer.Peer) {
	am.msgChan <- &donePeerMsg{p}
}

func (am *AvalancheManager) handleDonePeer(p *peer.Peer) {
	_, exists := am.peers[p]
	if !exists {
		log.Warnf("Received done avalanche peer message for unknown peer %s", p)
		return
	}

	// Remove the peer from the list of peers.
	delete(am.peers, p)

	log.Infof("Lost avalanche peer %s", p)
}