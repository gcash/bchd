package bchrpc

import (
	"context"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
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
			DisableRelayPriority: true,
			AcceptNonStd:         true,
			MaxTxVersion:         2,
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

	// The outputs pay to a pay-to-pubkey-hash script so that the
	// transaction reaches the minimum transaction size.
	payScript, err := txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).
		AddOp(txscript.OP_HASH160).AddData(make([]byte, 20)).
		AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).Script()
	if err != nil {
		t.Fatal(err)
	}
	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *fundingTx.Hash(), Index: 0},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	for range numOutputs {
		msgTx.AddTxOut(&wire.TxOut{Value: 10000000, PkScript: payScript})
	}
	tx := bchutil.NewTx(msgTx)

	if _, _, err := pool.MaybeAcceptTransaction(tx, true, false); err != nil {
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
