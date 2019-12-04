package snowglobe

import (
	"encoding/hex"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

type Common struct {
	CreatedAt time.Time `json:"created_at"`
}

type UTXO struct {
	Common

	TransactionHash chainhash.Hash `json:"transaction_hash"`
	Index           int8           `json:"index"`
	Amount          int64          `json:"amount"`
	Height          int32          `json:"height"`
}

// type Transaction struct {
// 	Common
// 	Hash chainhash.Hash `json:"hash"`
// }

type Block struct {
	Common
	Hash     chainhash.Hash `json:"hash"`
	PrevHash chainhash.Hash `json:"previous_hash"`
	Height   int32          `json:"height"`
}

type Peer struct {
	Common
}

type VoteRecord struct {
	Common

	PeerIdentityKey string `json:"peer_identity_key"`

	VertexType string          `json:"vertex_type"`
	VertexHash *chainhash.Hash `json:"vertex_hash"`

	State        string `json:"state"`
	InitialState string `json:"initial_state"`

	StartedAt   int64 `json:"started_at"`
	FinalizedAt int64 `json:"finalized_at"`
}

func (vr VoteRecord) FinalizationLatency() int64 {
	return vr.FinalizedAt - vr.StartedAt
}

func IdentityStringFromPublicKey(pk bchec.PublicKey) string {
	return hex.EncodeToString(pk.SerializeCompressed())
}
