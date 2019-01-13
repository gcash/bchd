package avalanche

import (
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"time"
)

// RequestRecord is a poll request for more votes
type RequestRecord struct {
	timestamp int64
	invs      []wire.InvVect
}

// NewRequestRecord creates a new RequestRecord
func NewRequestRecord(timestamp int64, invs []wire.InvVect) RequestRecord {
	return RequestRecord{timestamp, invs}
}

// GetTimestamp returns the timestamp that the request was created
func (r RequestRecord) GetTimestamp() int64 {
	return r.timestamp
}

// GetInvs returns the poll Invs for the request
func (r RequestRecord) GetInvs() map[chainhash.Hash]wire.InvVect {
	m := make(map[chainhash.Hash]wire.InvVect)
	for _, inv := range r.invs {
		m[inv.Hash] = inv
	}
	return m
}

// IsExpired returns true if the request has expired
func (r RequestRecord) IsExpired() bool {
	return time.Unix(r.timestamp, 0).Add(AvalancheRequestTimeout).Before(time.Now())
}
