package snowglobe

import (
	"encoding/hex"
	"strings"

	"github.com/gcash/bchutil"
	"github.com/gocraft/dbr/v2"

	"github.com/gcash/bchd/avalanche"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

func IngestPeer(db *dbr.Session, ssi avalanche.SignedIdentity) error {
	// Create a database transaction to use for the entire mutation
	dbTX, err := db.Begin()
	if err != nil {
		return err
	}
	defer dbTX.RollbackUnlessCommitted()

	err = insertPeer(dbTX, ssi)
	if err != nil {
		return err
	}

	return dbTX.Commit()
}

func IngestBlock(db *dbr.Session, block *bchutil.Block) error {
	// Create a database transaction to use for the entire mutation
	dbTX, err := db.Begin()
	if err != nil {
		return err
	}
	defer dbTX.RollbackUnlessCommitted()

	err = insertBlock(dbTX, block)
	if err != nil {
		return err
	}

	return dbTX.Commit()
}

func IngestTransaction(db *dbr.Session, tx *bchutil.Tx) error {
	// Create a database transaction to use for the entire mutation
	dbTX, err := db.Begin()
	if err != nil {
		return err
	}
	defer dbTX.RollbackUnlessCommitted()

	err = insertTransaction(dbTX, tx)
	if err != nil {
		return err
	}

	return dbTX.Commit()
}

func CreateVoteRecord(db *dbr.Session, vr VoteRecord) error {
	_, err := db.
		InsertInto("vote_records").
		Columns("peer_identity_key", "vertex_type", "vertex_hash", "started_at", "state", "initial_state").
		Values(vr.PeerIdentityKey, vr.VertexType, vr.VertexHash.String(), vr.StartedAt, vr.State, vr.State).
		Exec()
	if err != nil && !isDuplicateEntryErr(err) {
		return err
	}
	return nil
}

func FinalizedVoteRecord(db *dbr.Session, vr VoteRecord) error {
	_, err := db.
		Update("vote_records").
		Set("state", vr.State).
		Set("finalized_at", vr.FinalizedAt).
		Where("peer_identity_key = ? and vertex_type = ? and vertex_hash = ? and ",
			vr.PeerIdentityKey,
			vr.VertexType,
			vr.VertexHash.String()).
		Limit(1).
		Exec()
	if err != nil {
		return err
	}
	return err
}

func insertPeer(db dbr.SessionRunner, ssi avalanche.SignedIdentity) error {
	_, err := db.
		InsertInto("peers").

		// TODO: Set sequence, signature, stake_message based on staking message
		Columns("identity_key", "sequence", "signature", "stake_message").
		Values(hex.EncodeToString(ssi.PubKey.SerializeCompressed()), 0, "", "").
		Exec()
	if err != nil && !isDuplicateEntryErr(err) {
		return err
	}

	return nil
}

func insertBlock(db dbr.SessionRunner, block *bchutil.Block) error {
	_, err := db.
		InsertInto("blocks").
		Columns("hash", "previous_hash", "height").
		Values(block.Hash().String(), block.MsgBlock().Header.PrevBlock.String(), block.Height()).
		Exec()
	if err != nil && !isDuplicateEntryErr(err) {
		return err
	}

	for _, tx := range block.Transactions() {
		_, err = db.
			InsertInto("blocks_transactions").
			Columns("block_hash", "transaction_hash").
			Values(block.Hash().String(), tx.Hash().String()).
			Exec()
		if err != nil && !isDuplicateEntryErr(err) {
			return err
		}
	}

	return nil
}

func insertTransaction(db dbr.SessionRunner, tx *bchutil.Tx) error {
	// Create transaction record
	// _, err = db.
	// 	InsertInto("transactions").
	// 	Columns("hash", "fee", "accepted_to_mempool_at").
	// 	Values(tx.Tx.Hash().String(), tx.Fee, tx.Added).
	// 	Exec()
	// if err != nil {
	// 	return err
	// }
	var err error

	// Link transaction to input outpoints
	for _, input := range tx.MsgTx().TxIn {
		_, err = db.
			InsertInto("transaction_inputs").
			Columns("transaction_hash", "outpoint_transaction_hash", "outpoint_index").
			Values(tx.Hash().String(), input.PreviousOutPoint.Hash.String(), input.PreviousOutPoint.Index).
			Exec()
		if err != nil && !isDuplicateEntryErr(err) {
			return err
		}
	}

	// Create outputs and link transaction
	var idx uint32
	for _, output := range tx.MsgTx().TxOut {
		err = createOutPoint(db, *tx.Hash(), idx, output.Value)
		if err != nil {
			return err
		}

		idx++
	}

	return nil
}

func createOutPoint(db dbr.SessionRunner, hash chainhash.Hash, index uint32, amount int64) error {
	_, err := db.
		InsertInto("outpoints").
		Columns("transaction_hash", "index", "amount").
		Values(hash.String(), index, amount).
		Exec()
	if err != nil && !isDuplicateEntryErr(err) {
		return err
	}
	return nil
}

func isDuplicateEntryErr(err error) bool {
	return strings.HasPrefix(err.Error(), "Error 1062")
}
