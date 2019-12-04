package snowglobe

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"

	"github.com/gcash/bchutil"
	"github.com/go-sql-driver/mysql"
	"github.com/gocraft/dbr/v2"
	"github.com/tyler-smith/env"

	"github.com/gcash/bchd/avalanche"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

var _ avalanche.Receiver = &dbReceiver{}

// dbReceiver provides an Receiver backed by an external database so that data
// is persisted.
type dbReceiver struct {
	idKeyStr string
	db       *dbr.Connection
}

func NewDBReceiverForEnv(idKeyStr string) (avalanche.Receiver, error) {
	use, err := env.GetBool("SNOWGLOBE_USE_DB_REPORTER", false)
	if !use || err != nil {
		return nil, err
	}

	db, err := getDBRConnectionForEnv()
	if err != nil {
		return nil, err
	}

	return &dbReceiver{idKeyStr, db}, nil
}

func (dbr *dbReceiver) PeerConnect(ssi avalanche.SignedIdentity) {
	IngestPeer(dbr.db.NewSession(nil), ssi)
}

func (dbr *dbReceiver) PeerDisconnect(ssi avalanche.SignedIdentity) {
	IngestPeer(dbr.db.NewSession(nil), ssi)
}

func (dbr *dbReceiver) NewBlock(block *bchutil.Block) {
	IngestBlock(dbr.db.NewSession(nil), block)
}

func (dbr *dbReceiver) NewTransaction(tx *bchutil.Tx) {
	IngestTransaction(dbr.db.NewSession(nil), tx)
}

func (dbr *dbReceiver) NewVoteRecord(h chainhash.Hash, vr avalanche.VoteRecord) {
	CreateVoteRecord(dbr.db.NewSession(nil), newVRModel(dbr.idKeyStr, h, vr))
}

func (dbr *dbReceiver) FinalizedVoteRecord(h chainhash.Hash, vr avalanche.VoteRecord) {
	FinalizedVoteRecord(dbr.db.NewSession(nil), newVRModel(dbr.idKeyStr, h, vr))
}

func getDBRConnectionForEnv() (*dbr.Connection, error) {
	var (
		caFile = env.GetString("SNOWGLOBE_CA_FILE", "")
		driver = env.GetString("SNOWGLOBE_DRIVER", "mysql")
		dsn    = env.GetString("SNOWGLOBE_DSN", "root:pass@tcp(0.0.0.0:3306)/sherpa_dev?parseTime=true")
	)

	// IF a custom TLS CA file is set for the DB then add it now
	if caFile != "" {
		rootCertPool := x509.NewCertPool()
		pem, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			return nil, errors.New("Failed to append PEM")
		}
		mysql.RegisterTLSConfig("custom", &tls.Config{RootCAs: rootCertPool})
	}

	return dbr.Open(driver, dsn, nil)
}

func newVRModel(peerIdentityKey string, h chainhash.Hash, vr avalanche.VoteRecord) VoteRecord {
	return VoteRecord{
		// PeerIdentityKey: peerIdentityKey,

		// StartedAt:   vr.StartedAt(),
		// FinalizedAt: vr.FinalizedAt(),
		// state:       vr.state().String(),

		// VertexHash: &h,
		// VertexType: vr.Vertex().Type().String(),
	}
}
