package bchrpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newMempoolWithTx returns a transaction pool holding a single transaction
// with numOutputs outputs, along with that transaction.  The transaction
// spends an anyone-can-spend output from a fake utxo set, so the pool accepts
// it without any signing.
func newMempoolWithTx(t *testing.T,
	numOutputs int) (*mempool.TxPool, *bchutil.Tx) {

	t.Helper()

	// The outputs pay to a pay-to-pubkey-hash script so that the
	// transaction reaches the minimum transaction size.
	outs := make([]*wire.TxOut, numOutputs)
	for i := range outs {
		outs[i] = &wire.TxOut{Value: 10000000, PkScript: p2pkhScript(t)}
	}
	return newMempoolWithTxOuts(t, outs)
}

// p2pkhScript returns a pay-to-pubkey-hash script.
func p2pkhScript(t *testing.T) []byte {
	t.Helper()
	script, err := txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).
		AddOp(txscript.OP_HASH160).AddData(make([]byte, 20)).
		AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).Script()
	if err != nil {
		t.Fatal(err)
	}
	return script
}

// newMempoolWithTxOuts returns a transaction pool holding a single
// transaction with the passed outputs, along with that transaction, as
// newMempoolWithTx does.
func newMempoolWithTxOuts(t *testing.T,
	outs []*wire.TxOut) (*mempool.TxPool, *bchutil.Tx) {

	t.Helper()

	params := &chaincfg.RegressionNetParams
	opTrue := []byte{txscript.OP_TRUE}

	// The funding transaction is never added to the pool.  It only provides
	// the output that the pooled transaction spends.
	funding := wire.NewMsgTx(wire.TxVersion)
	funding.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	funding.AddTxOut(&wire.TxOut{Value: 100000000, PkScript: opTrue})
	fundingTx := bchutil.NewTx(funding)

	pool := mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			AcceptNonStd: true,
			MaxTxVersion: 2,
		},
		ChainParams: params,
		FetchUtxoView: func(*bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			view := blockchain.NewUtxoViewpoint()
			view.AddTxOuts(fundingTx, 1)
			return view, nil
		},
		BestHeight:     func() int32 { return 1 },
		MedianTimePast: time.Now,
		CalcSequenceLock: func(*bchutil.Tx,
			*blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {

			return &blockchain.SequenceLock{Seconds: -1, BlockHeight: -1}, nil
		},
	})

	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *fundingTx.Hash(), Index: 0},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	for _, out := range outs {
		msgTx.AddTxOut(out)
	}
	tx := bchutil.NewTx(msgTx)

	if _, _, err := pool.MaybeAcceptTransaction(tx, true); err != nil {
		t.Fatalf("MaybeAcceptTransaction: %v", err)
	}
	if !pool.HaveTransaction(tx.Hash()) {
		t.Fatal("transaction was not added to the mempool")
	}
	return pool, tx
}

// TestGetUnspentOutputMempoolIndexBounds ensures GetUnspentOutput rejects an
// output index that is out of range for a mempool transaction, including an
// index equal to the number of outputs, instead of indexing past the end of
// the outputs.
func TestGetUnspentOutputMempoolIndexBounds(t *testing.T) {
	const numOutputs = 2
	pool, tx := newMempoolWithTx(t, numOutputs)
	s := &GrpcServer{
		chainParams: &chaincfg.RegressionNetParams,
		txMemPool:   pool,
	}

	tests := []struct {
		name  string
		index uint32
		code  codes.Code
	}{
		{name: "first output", index: 0, code: codes.OK},
		{name: "last output", index: numOutputs - 1, code: codes.OK},
		{name: "index equal to output count", index: numOutputs, code: codes.InvalidArgument},
		{name: "index past output count", index: numOutputs + 1, code: codes.InvalidArgument},
		{name: "max index", index: ^uint32(0), code: codes.InvalidArgument},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("GetUnspentOutput panicked: %v", r)
				}
			}()

			resp, err := s.GetUnspentOutput(context.Background(),
				&pb.GetUnspentOutputRequest{
					Hash:           tx.Hash()[:],
					Index:          test.index,
					IncludeMempool: true,
				})
			if got := status.Code(err); got != test.code {
				t.Fatalf("GetUnspentOutput: got code %v, want %v (err: %v)",
					got, test.code, err)
			}
			if test.code != codes.OK {
				return
			}

			txOut := tx.MsgTx().TxOut[test.index]
			if resp.Value != txOut.Value {
				t.Errorf("value: got %d, want %d", resp.Value, txOut.Value)
			}
			if resp.Outpoint.Index != test.index {
				t.Errorf("outpoint index: got %d, want %d",
					resp.Outpoint.Index, test.index)
			}
		})
	}
}

// TestGetUnspentOutputMempoolSlpNoToken ensures GetUnspentOutput returns an
// output of an unconfirmed transaction whose first output reads as SLP but
// that holds no tokens without a token, rather than failing with Internal:
// the change of a GENESIS, and an output of a SEND that is not valid.  A
// token output still has its token and the token's metadata.
func TestGetUnspentOutputMempoolSlpNoToken(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	genesisScript, err := metadatamaker.TokenType1Genesis([]byte("T"),
		[]byte("Token"), nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatalf("TokenType1Genesis: %v", err)
	}
	// The SEND spends no tokens, so it is not valid.
	sendScript, err := metadatamaker.TokenType1Send(bytes.Repeat([]byte{1}, 32),
		[]uint64{10})
	if err != nil {
		t.Fatalf("TokenType1Send: %v", err)
	}
	pay := p2pkhScript(t)
	genesisPool, genesis := newMempoolWithTxOuts(t, []*wire.TxOut{
		{PkScript: genesisScript},
		{Value: 10000000, PkScript: pay},
		{Value: 10000000, PkScript: pay},
	})
	sendPool, send := newMempoolWithTxOuts(t, []*wire.TxOut{
		{PkScript: sendScript},
		{Value: 10000000, PkScript: pay},
	})
	for _, tx := range []*bchutil.Tx{genesis, send} {
		err := db.View(func(dbTx database.Tx) error {
			_, err := idx.AddPotentialSlpEntries(dbTx, tx.MsgTx())
			return err
		})
		if err != nil {
			t.Fatalf("AddPotentialSlpEntries: %v", err)
		}
	}

	tests := []struct {
		name  string
		pool  *mempool.TxPool
		tx    *bchutil.Tx
		index uint32
		token bool
	}{
		{"genesis token output", genesisPool, genesis, 1, true},
		{"genesis change", genesisPool, genesis, 2, false},
		{"invalid send", sendPool, send, 1, false},
	}
	for _, test := range tests {
		s := &GrpcServer{chainParams: &chaincfg.RegressionNetParams,
			txMemPool: test.pool, db: db, slpIndex: idx}
		resp, err := s.GetUnspentOutput(context.Background(),
			&pb.GetUnspentOutputRequest{
				Hash:           test.tx.Hash()[:],
				Index:          test.index,
				IncludeMempool: true,
			})
		if err != nil {
			t.Errorf("%s: GetUnspentOutput: %v", test.name, err)
			continue
		}
		if resp.Value != 10000000 {
			t.Errorf("%s: value %d, want 10000000", test.name, resp.Value)
		}
		if !test.token {
			if resp.SlpToken != nil || resp.TokenMetadata != nil {
				t.Errorf("%s: got token %v and metadata %v, want none",
					test.name, resp.SlpToken, resp.TokenMetadata)
			}
			continue
		}
		if resp.SlpToken == nil || resp.SlpToken.Amount != 1000 {
			t.Errorf("%s: got token %v, want 1000 tokens", test.name,
				resp.SlpToken)
		}
		if resp.TokenMetadata == nil {
			t.Errorf("%s: no token metadata", test.name)
		}
	}
}

// TestGetUnspentOutputConfirmedSlpToken ensures GetUnspentOutput returns a
// confirmed output that holds tokens with its token and the token's metadata,
// as it returns an unconfirmed one, and a confirmed output that holds none
// without a token, and that it answers with an internal error, logging the
// failure, when the slp index fails to read the token or its metadata.
func TestGetUnspentOutputConfirmedSlpToken(t *testing.T) {
	var logged syncBuffer
	saved := log
	UseLogger(bchlog.NewBackend(&logged).Logger("GRPC"))
	defer UseLogger(saved)

	genesisScript, err := metadatamaker.TokenType1Genesis([]byte("T"),
		[]byte("Token"), nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatalf("TokenType1Genesis: %v", err)
	}
	// Block 1's coinbase is a GENESIS, whose output 1 holds its tokens and
	// whose output 2 holds none.
	opTrue := []byte{txscript.OP_TRUE}
	s := newAddrIndexServerBlocks(t, [][]byte{genesisScript, opTrue, opTrue}, 1)
	s.txMemPool = mempool.New(&mempool.Config{})
	newIndex := func() *indexers.SlpIndex {
		return indexers.NewSlpIndex(s.db, &indexers.SlpConfig{MaxCacheSize: 100})
	}
	connected := newIndex()
	block, err := s.chain.BlockByHeight(1)
	if err != nil {
		t.Fatal(err)
	}
	err = s.db.Update(func(dbTx database.Tx) error {
		if err := connected.Create(dbTx); err != nil {
			return err
		}
		return connected.ConnectBlock(dbTx, block, nil)
	})
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	genesis := block.Transactions()[0]
	token, err := goslp.GetSlpTokenID(genesis.MsgTx())
	if err != nil {
		t.Fatal(err)
	}
	unspent := func(index uint32, includeMempool bool) (*pb.GetUnspentOutputResponse, error) {
		return s.GetUnspentOutput(context.Background(), &pb.GetUnspentOutputRequest{
			Hash: genesis.Hash()[:], Index: index, IncludeMempool: includeMempool})
	}

	// The entries are read from the cache of the index that connected the
	// block, and from the database by another index, as after a restart.
	for _, idx := range []*indexers.SlpIndex{connected, newIndex()} {
		s.slpIndex = idx
		for _, includeMempool := range []bool{false, true} {
			resp, err := unspent(1, includeMempool)
			if err != nil {
				t.Fatalf("token output, mempool %v: %v", includeMempool, err)
			}
			if resp.SlpToken == nil || resp.SlpToken.Amount != 1000 ||
				!bytes.Equal(resp.SlpToken.TokenId, token) {
				t.Errorf("token output, mempool %v: got token %v, want 1000 "+
					"tokens of token ID %x", includeMempool, resp.SlpToken, token)
			}
			if resp.TokenMetadata == nil ||
				!bytes.Equal(resp.TokenMetadata.TokenId, token) {
				t.Errorf("token output, mempool %v: got metadata %v, want that "+
					"of token ID %x", includeMempool, resp.TokenMetadata, token)
			}
			resp, err = unspent(2, includeMempool)
			if err != nil {
				t.Fatalf("change, mempool %v: %v", includeMempool, err)
			}
			if resp.SlpToken != nil || resp.TokenMetadata != nil {
				t.Errorf("change, mempool %v: got token %v and metadata %v, "+
					"want none", includeMempool, resp.SlpToken, resp.TokenMetadata)
			}
		}
	}

	check := func(what string, err error, msg, logLine string) {
		t.Helper()
		if status.Code(err) != codes.Internal || status.Convert(err).Message() != msg {
			t.Errorf("%s: got %v, want Internal with %q", what, err, msg)
		}
		if !strings.Contains(logged.String(), logLine) {
			t.Errorf("%s: log %q, want a line with %q", what, logged.String(),
				logLine)
		}
	}

	// The read of the token's metadata, in the call's last database
	// transaction, fails.
	last := &lastViewDB{DB: s.db, err: errors.New("the database failed")}
	s.db = last
	if _, err := unspent(1, false); err != nil {
		t.Fatalf("GetUnspentOutput: %v", err)
	}
	last.fail, last.views = last.views.Load(), atomic.Int32{}
	_, err = unspent(1, false)
	check("the metadata's read failing", err, "failed to load slp token metadata",
		"[ERR] GRPC: Failed to load slp token metadata of token ID "+
			hex.EncodeToString(token)+": the database failed")
	s.db = last.DB

	// The output's entry is corrupt, which an index whose cache does not
	// hold the entry reads.
	err = s.db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().Bucket([]byte("slptxbyhashidx")).Put(
			genesis.Hash()[:], []byte{1})
	})
	if err != nil {
		t.Fatal(err)
	}
	s.slpIndex = newIndex()
	_, err = unspent(1, false)
	op := wire.OutPoint{Hash: *genesis.Hash(), Index: 1}
	check("a corrupt entry", err, "failed to load slp index entry",
		"[ERR] GRPC: Failed to load the slp token of output "+op.String()+
			": corrupt slp index entry for "+op.Hash.String())
}
