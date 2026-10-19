// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newDSProofPool returns a transaction pool holding tx1, which has a double
// spend proof against tx2, and the outputs of the funding transaction.
func newDSProofPool(t *testing.T) (pool *mempool.TxPool, tx1, tx2 *wire.MsgTx, spend func(op wire.OutPoint, value, fee int64) *wire.MsgTx, funding *wire.MsgTx) {
	t.Helper()
	params := &chaincfg.MainNetParams
	key, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatal(err)
	}
	addr, err := bchutil.NewAddressPubKeyHash(bchutil.Hash160(key.PubKey().SerializeCompressed()), params)
	if err != nil {
		t.Fatal(err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}
	funding = wire.NewMsgTx(wire.TxVersion)
	funding.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}}})
	for range 2 {
		funding.AddTxOut(&wire.TxOut{Value: 100000000, PkScript: pkScript})
	}
	utxos := blockchain.NewUtxoViewpoint()
	utxos.AddTxOuts(bchutil.NewTx(funding), 1000000)

	pool = mempool.New(&mempool.Config{
		Policy: mempool.Policy{
			LimitSigChecks: true,
			MinRelayTxFee:  mempool.DefaultMinRelayTxFee,
			MaxTxVersion:   2,
		},
		ChainParams: params,
		FetchUtxoView: func(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			view := blockchain.NewUtxoViewpoint()
			for _, in := range tx.MsgTx().TxIn {
				view.Entries()[in.PreviousOutPoint] = utxos.LookupEntry(in.PreviousOutPoint).Clone()
			}
			return view, nil
		},
		BestHeight:     func() int32 { return 1000000 },
		MedianTimePast: time.Now,
		CalcSequenceLock: func(*bchutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return &blockchain.SequenceLock{Seconds: -1, BlockHeight: -1}, nil
		},
	})
	spend = func(op wire.OutPoint, value, fee int64) *wire.MsgTx {
		tx := wire.NewMsgTx(wire.TxVersion)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op, Sequence: wire.MaxTxInSequenceNum})
		tx.AddTxOut(&wire.TxOut{Value: value - fee, PkScript: pkScript})
		script, err := txscript.SignatureScript(tx, 0, value, pkScript, txscript.SigHashAll, key, true)
		if err != nil {
			t.Fatal(err)
		}
		tx.TxIn[0].SignatureScript = script
		return tx
	}
	op := wire.OutPoint{Hash: funding.TxHash()}
	tx1, tx2 = spend(op, 100000000, 1000), spend(op, 100000000, 2000)
	if _, err := pool.ProcessTransaction(bchutil.NewTx(tx1), false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.ProcessTransaction(bchutil.NewTx(tx2), false, 0); err == nil {
		t.Fatal("double spend accepted")
	}
	return pool, tx1, tx2, spend, funding
}

// TestGetDoubleSpendProof checks the double spend proof RPCs.
func TestGetDoubleSpendProof(t *testing.T) {
	pool, tx1, _, _, funding := newDSProofPool(t)
	s := &GrpcServer{chainParams: &chaincfg.MainNetParams, txMemPool: pool}
	ctx := context.Background()

	list, err := s.GetDoubleSpendProofs(ctx, &pb.GetDoubleSpendProofsRequest{})
	if err != nil || len(list.Proofs) != 1 {
		t.Fatalf("GetDoubleSpendProofs: %v %v", list, err)
	}
	proof := list.Proofs[0]
	var msg wire.MsgDSProof
	if err := msg.BchDecode(bytes.NewReader(proof.SerializedProof), 0, wire.BaseEncoding); err != nil {
		t.Fatal(err)
	}
	proofHash := msg.ProofHash()
	txHash := tx1.TxHash()
	if !bytes.Equal(proof.Hash, proofHash[:]) || !bytes.Equal(proof.TransactionHash, txHash[:]) {
		t.Fatal("hashes differ")
	}
	fundingHash := funding.TxHash()
	if !bytes.Equal(proof.Outpoint.Hash, fundingHash[:]) || proof.Outpoint.Index != 0 {
		t.Fatal("outpoint differs")
	}
	for i, sp := range []*pb.DoubleSpendProof_Spender{proof.FirstSpender, proof.DoubleSpender} {
		want := []*wire.DSProofSpender{&msg.FirstSpender, &msg.DoubleSpender}[i]
		if sp.Version != want.Version || sp.Sequence != want.Sequence ||
			!bytes.Equal(sp.HashOutputs, want.HashOutputs[:]) ||
			len(sp.PushData) != 1 || !bytes.Equal(sp.PushData[0], want.PushData[0]) {
			t.Fatalf("spender %d differs", i)
		}
	}

	for name, req := range map[string]*pb.GetDoubleSpendProofRequest{
		"hash": {Query: &pb.GetDoubleSpendProofRequest_ProofHash{ProofHash: proofHash[:]}},
		"tx":   {Query: &pb.GetDoubleSpendProofRequest_TransactionHash{TransactionHash: txHash[:]}},
		"outpoint": {Query: &pb.GetDoubleSpendProofRequest_Outpoint{
			Outpoint: &pb.Transaction_Input_Outpoint{Hash: fundingHash[:]}}},
	} {
		resp, err := s.GetDoubleSpendProof(ctx, req)
		if err != nil || !bytes.Equal(resp.Proof.Hash, proofHash[:]) {
			t.Fatalf("%s: %v", name, err)
		}
		if len(resp.Descendants) != 1 || !bytes.Equal(resp.Descendants[0], txHash[:]) {
			t.Fatalf("%s: descendants %x", name, resp.Descendants)
		}
		if (len(resp.Path) == 1) != (name == "tx") {
			t.Fatalf("%s: path %x", name, resp.Path)
		}
	}

	_, err = s.GetDoubleSpendProof(ctx, &pb.GetDoubleSpendProofRequest{
		Query: &pb.GetDoubleSpendProofRequest_ProofHash{ProofHash: make([]byte, 32)}})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown proof: %v", err)
	}
	_, err = s.GetDoubleSpendProof(ctx, &pb.GetDoubleSpendProofRequest{
		Query: &pb.GetDoubleSpendProofRequest_ProofHash{ProofHash: make([]byte, 5)}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("short hash: %v", err)
	}
	if _, err = s.GetDoubleSpendProof(ctx, &pb.GetDoubleSpendProofRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty query: %v", err)
	}

	score, err := s.GetDoubleSpendProofScore(ctx, &pb.GetDoubleSpendProofScoreRequest{TransactionHash: txHash[:]})
	if err != nil || score.Score != 0 {
		t.Fatalf("score: %v %v", score, err)
	}
	_, err = s.GetDoubleSpendProofScore(ctx, &pb.GetDoubleSpendProofScoreRequest{TransactionHash: fundingHash[:]})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("score of an unknown transaction: %v", err)
	}
}

// dsProofStream records the notifications sent on a SubscribeDoubleSpendProofs
// stream.
type dsProofStream struct {
	grpc.ServerStream
	ctx  context.Context
	sent chan *pb.DoubleSpendProofNotification
}

func (s *dsProofStream) Context() context.Context { return s.ctx }

func (s *dsProofStream) Send(n *pb.DoubleSpendProofNotification) error {
	s.sent <- n
	return nil
}

// TestSubscribeDoubleSpendProofs checks that the proofs in the mempool and
// new ones are sent.
func TestSubscribeDoubleSpendProofs(t *testing.T) {
	pool, _, _, spend, funding := newDSProofPool(t)
	s := newDispatcherTestServer(t)
	s.txMemPool = pool

	ctx, cancel := context.WithCancel(context.Background())
	stream := &dsProofStream{ctx: ctx, sent: make(chan *pb.DoubleSpendProofNotification, 10)}
	done := make(chan error, 1)
	go func() {
		done <- s.SubscribeDoubleSpendProofs(&pb.SubscribeDoubleSpendProofsRequest{IncludeMempool: true}, stream)
	}()
	recv := func() *pb.DoubleSpendProofNotification {
		t.Helper()
		select {
		case n := <-stream.sent:
			return n
		case <-time.After(5 * time.Second):
			t.Fatal("no notification")
		}
		return nil
	}
	first := recv()

	// A second double spend.
	op := wire.OutPoint{Hash: funding.TxHash(), Index: 1}
	tx3, tx4 := spend(op, 100000000, 1000), spend(op, 100000000, 2000)
	if _, err := pool.ProcessTransaction(bchutil.NewTx(tx3), false, 0); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.ProcessTransaction(bchutil.NewTx(tx4), false, 0)
	desc, _, _, _ := pool.DSProofByTx(bchutil.NewTx(tx3).Hash(), false)
	if desc == nil {
		t.Fatal("no proof for the second double spend")
	}
	// The subscription was handed to the dispatcher before the first
	// notification was sent, and the dispatcher handles one message at a
	// time, so once it has taken this unsubscribe of a subscription it
	// does not hold, the subscription is registered.
	s.unsubscribe <- &rpcEventSubscription{}
	if got := s.numSubscriptions(); got != 1 {
		t.Fatalf("%d subscriptions in the dispatcher", got)
	}
	s.NotifyDSProof(desc.Proof, bchutil.NewTx(tx3))
	second := recv()
	if !bytes.Equal(second.Proof.Hash, desc.Hash[:]) || bytes.Equal(first.Proof.Hash, second.Proof.Hash) {
		t.Fatal("wrong proof notified")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("subscription ended with %v", err)
	}
}

// TestDoubleSpendProofRPCLimits checks the answers when proofs are disabled
// and when a transaction has too many unconfirmed ancestors to search.
func TestDoubleSpendProofRPCLimits(t *testing.T) {
	ctx := context.Background()
	disabled := &GrpcServer{txMemPool: mempool.New(&mempool.Config{DisableDSProofs: true})}
	if _, err := disabled.GetDoubleSpendProofs(ctx, &pb.GetDoubleSpendProofsRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("GetDoubleSpendProofs: %v", err)
	}
	if _, err := disabled.GetDoubleSpendProof(ctx, &pb.GetDoubleSpendProofRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("GetDoubleSpendProof: %v", err)
	}
	if _, err := disabled.GetDoubleSpendProofScore(ctx, &pb.GetDoubleSpendProofScoreRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("GetDoubleSpendProofScore: %v", err)
	}
	err := disabled.SubscribeDoubleSpendProofs(&pb.SubscribeDoubleSpendProofsRequest{},
		&dsProofStream{ctx: ctx})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("SubscribeDoubleSpendProofs: %v", err)
	}

	// A chain of 1001 unconfirmed transactions.
	pool, _, _, spend, funding := newDSProofPool(t)
	s := &GrpcServer{chainParams: &chaincfg.MainNetParams, txMemPool: pool}
	op := wire.OutPoint{Hash: funding.TxHash(), Index: 1}
	value := int64(100000000)
	var last chainhash.Hash
	for range 1001 {
		tx := spend(op, value, 1000)
		if _, err := pool.ProcessTransaction(bchutil.NewTx(tx), false, 0); err != nil {
			t.Fatal(err)
		}
		last = tx.TxHash()
		op = wire.OutPoint{Hash: last}
		value -= 1000
	}
	_, err = s.GetDoubleSpendProof(ctx, &pb.GetDoubleSpendProofRequest{
		Query: &pb.GetDoubleSpendProofRequest_TransactionHash{TransactionHash: last[:]}})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("deep lookup: %v", err)
	}
	score, err := s.GetDoubleSpendProofScore(ctx, &pb.GetDoubleSpendProofScoreRequest{TransactionHash: last[:]})
	if err != nil || score.Score != 0.25 {
		t.Fatalf("deep score: %v %v", score, err)
	}
}
