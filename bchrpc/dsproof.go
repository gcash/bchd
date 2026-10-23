// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchrpc

import (
	"bytes"
	"context"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// rpcEventDSProof indicates a double spend proof was associated with a
// mempool transaction.
type rpcEventDSProof struct {
	*mempool.DSProofDesc
}

// NotifyDSProof is called by the server when a double spend proof becomes
// associated with the mempool transaction tx.
func (s *GrpcServer) NotifyDSProof(proof *wire.MsgDSProof, tx *bchutil.Tx) {
	s.dispatchEvent(&rpcEventDSProof{&mempool.DSProofDesc{
		Proof:  proof,
		Hash:   proof.ProofHash(),
		TxHash: tx.Hash(),
	}})
}

// errDSProofsDisabled is returned by the double spend proof RPCs when proofs
// are disabled.
var errDSProofsDisabled = status.Error(codes.Unavailable, "double spend proofs are disabled")

// marshalDSProofSpender returns the protobuf form of a proof spender.
func marshalDSProofSpender(s *wire.DSProofSpender) *pb.DoubleSpendProof_Spender {
	return &pb.DoubleSpendProof_Spender{
		Version:         s.Version,
		Sequence:        s.Sequence,
		LockTime:        s.LockTime,
		HashPrevOutputs: s.HashPrevOutputs.CloneBytes(),
		HashSequence:    s.HashSequence.CloneBytes(),
		HashOutputs:     s.HashOutputs.CloneBytes(),
		PushData:        s.PushData,
	}
}

// marshalDSProof returns the protobuf form of a proof.
func marshalDSProof(desc *mempool.DSProofDesc) *pb.DoubleSpendProof {
	var buf bytes.Buffer
	buf.Grow(desc.Proof.SerializeSize())
	_ = desc.Proof.BchEncode(&buf, 0, wire.BaseEncoding)
	proof := &pb.DoubleSpendProof{
		Hash: desc.Hash.CloneBytes(),
		Outpoint: &pb.Transaction_Input_Outpoint{
			Hash:  desc.Proof.PrevOut.Hash.CloneBytes(),
			Index: desc.Proof.PrevOut.Index,
		},
		FirstSpender:    marshalDSProofSpender(&desc.Proof.FirstSpender),
		DoubleSpender:   marshalDSProofSpender(&desc.Proof.DoubleSpender),
		SerializedProof: buf.Bytes(),
	}
	if desc.TxHash != nil {
		proof.TransactionHash = desc.TxHash.CloneBytes()
	}
	return proof
}

// hashesToBytes returns the bytes of each hash.
func hashesToBytes(hashes []chainhash.Hash) [][]byte {
	if len(hashes) == 0 {
		return nil
	}
	b := make([][]byte, len(hashes))
	for i := range hashes {
		b[i] = hashes[i].CloneBytes()
	}
	return b
}

// GetDoubleSpendProof returns a double spend proof, looked up by its hash, by
// the double spent output, or by a mempool transaction.
func (s *GrpcServer) GetDoubleSpendProof(ctx context.Context, req *pb.GetDoubleSpendProofRequest) (*pb.GetDoubleSpendProofResponse, error) {
	if !s.txMemPool.DSProofsEnabled() {
		return nil, errDSProofsDisabled
	}

	var (
		desc              *mempool.DSProofDesc
		path, descendants []chainhash.Hash
	)
	switch q := req.Query.(type) {
	case *pb.GetDoubleSpendProofRequest_ProofHash:
		hash, err := chainhash.NewHash(q.ProofHash)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid proof hash: %v", err)
		}
		desc, descendants = s.txMemPool.DSProofByHash(hash)

	case *pb.GetDoubleSpendProofRequest_TransactionHash:
		hash, err := chainhash.NewHash(q.TransactionHash)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid transaction hash: %v", err)
		}
		desc, descendants, path, err = s.txMemPool.DSProofByTx(hash, !req.ExcludeAncestors)
		if err != nil {
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		}

	case *pb.GetDoubleSpendProofRequest_Outpoint:
		if q.Outpoint == nil {
			return nil, status.Error(codes.InvalidArgument, "outpoint is required")
		}
		hash, err := chainhash.NewHash(q.Outpoint.Hash)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid outpoint hash: %v", err)
		}
		desc, descendants = s.txMemPool.DSProofByOutPoint(wire.OutPoint{Hash: *hash, Index: q.Outpoint.Index})

	default:
		return nil, status.Error(codes.InvalidArgument, "a proof hash, transaction hash or outpoint is required")
	}

	if desc == nil {
		return nil, status.Error(codes.NotFound, "double spend proof not found")
	}
	return &pb.GetDoubleSpendProofResponse{
		Proof:       marshalDSProof(desc),
		Path:        hashesToBytes(path),
		Descendants: hashesToBytes(descendants),
	}, nil
}

// GetDoubleSpendProofs returns the double spend proofs for mempool
// transactions, and the orphan proofs when requested.
func (s *GrpcServer) GetDoubleSpendProofs(ctx context.Context, req *pb.GetDoubleSpendProofsRequest) (*pb.GetDoubleSpendProofsResponse, error) {
	if !s.txMemPool.DSProofsEnabled() {
		return nil, errDSProofsDisabled
	}
	descs := s.txMemPool.DSProofs(req.IncludeOrphans)
	resp := &pb.GetDoubleSpendProofsResponse{
		Proofs: make([]*pb.DoubleSpendProof, 0, len(descs)),
	}
	for _, desc := range descs {
		resp.Proofs = append(resp.Proofs, marshalDSProof(desc))
	}
	return resp, nil
}

// GetDoubleSpendProofScore returns how far a mempool transaction can be
// trusted not to be double spent.
func (s *GrpcServer) GetDoubleSpendProofScore(ctx context.Context, req *pb.GetDoubleSpendProofScoreRequest) (*pb.GetDoubleSpendProofScoreResponse, error) {
	if !s.txMemPool.DSProofsEnabled() {
		return nil, errDSProofsDisabled
	}
	hash, err := chainhash.NewHash(req.TransactionHash)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction hash: %v", err)
	}
	score, err := s.txMemPool.DSProofScore(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, "transaction not in mempool")
	}
	return &pb.GetDoubleSpendProofScoreResponse{Score: score}, nil
}

// SubscribeDoubleSpendProofs sends a notification for each double spend proof
// that becomes associated with a mempool transaction.
func (s *GrpcServer) SubscribeDoubleSpendProofs(req *pb.SubscribeDoubleSpendProofsRequest, stream pb.Bchrpc_SubscribeDoubleSpendProofsServer) error {
	if !s.txMemPool.DSProofsEnabled() {
		return errDSProofsDisabled
	}
	subscription, err := s.subscribeClient()
	if err != nil {
		return err
	}
	defer s.unsubscribeClient(subscription)

	if req.IncludeMempool {
		for _, desc := range s.txMemPool.DSProofs(false) {
			err := stream.Send(&pb.DoubleSpendProofNotification{Proof: marshalDSProof(desc)})
			if err != nil {
				return err
			}
		}
	}

	for {
		select {
		case event, ok := <-subscription.Events():
			if !ok {
				return subscription.closedErr()
			}
			ds, ok := event.(*rpcEventDSProof)
			if !ok {
				continue
			}
			err := stream.Send(&pb.DoubleSpendProofNotification{Proof: marshalDSProof(ds.DSProofDesc)})
			if err != nil {
				return err
			}

		case <-stream.Context().Done():
			return nil
		}
	}
}
