package avalanche

import (
	"math/rand"
	"time"

	"github.com/gcash/bchlog"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
)

const (
	// version is the version number of the implemented version of the algorithm.
	version = 0

	minStakeAmount = 0

	// queryLoopTimeStep is the amount of time to wait between query loop ticks.
	queryLoopTimeStep = 10 * time.Millisecond

	// maxQueriesPerRequest is the max number of invs to send in a single request.
	maxQueriesPerRequest int = 4096

	// maxQueryAge is the amount of time to wait for a response to a query.
	maxQueryAge = 1 * time.Minute

	// maxVoteRecordAge is the longest we'll retain an unfinalized vote record.
	maxVoteRecordAge = 3600 * 6
)

var (
	randomGen                = rand.New(rand.NewSource(time.Now().UnixNano()))
	clock            clocker = realClock{}
	globalTimeOffset         = clock.now().Unix()
	log                      = bchlog.Disabled
)

type (
	query struct {
		timestamp int64
		invs      []*wire.InvVect
	}

	peerer interface {
		ID() int32
		NA() *wire.NetAddress
		AvalanchePubkey() *bchec.PublicKey
		QueueMessage(wire.Message, chan<- struct{})
	}

	queryMap      map[string]query
	voteRecordMap map[chainhash.Hash]*VoteRecord
	peerMap       map[peerer]*SignedIdentity

	// Create a clock abstraction to allow us to inject fixed times for testing
	clocker    interface{ now() time.Time }
	realClock  struct{}
	fixedClock time.Time
)

func (realClock) now() time.Time    { return time.Now() }
func (c fixedClock) now() time.Time { return time.Time(c) }

func UseLogger(logger bchlog.Logger) { log = logger }
