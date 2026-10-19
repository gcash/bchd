// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
)

// errDSProofDisabled is returned by the double spend proof commands when
// proofs are disabled.
var errDSProofDisabled = &btcjson.RPCError{
	Code:    btcjson.ErrRPCMethodDisabled,
	Message: "Double spend proofs are disabled. Restart without --nodoublespendproof to enable them.",
}

// dsProofVerbosity returns the verbosity requested, or def when it is
// missing.
func dsProofVerbosity(v *btcjson.DSProofVerbosity, def int) (int, error) {
	verbosity := def
	if v != nil {
		verbosity = int(*v)
	}
	if verbosity < 0 || verbosity > 3 {
		return 0, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: "Bad verbosity",
		}
	}
	return verbosity, nil
}

// sigHashTypeNames are the names of the hash types a double spend proof
// signature can have.
var sigHashTypeNames = map[txscript.SigHashType]string{
	txscript.SigHashAll:    "ALL",
	txscript.SigHashNone:   "NONE",
	txscript.SigHashSingle: "SINGLE",
}

// dsProofPushDataAsm returns the assembly of a spender's pushes, showing the
// hash type of a signature by name.
func dsProofPushDataAsm(pushes [][]byte) string {
	var buf bytes.Buffer
	for i, data := range pushes {
		if i > 0 {
			buf.WriteByte(' ')
		}
		if len(data) > 0 {
			hashType := txscript.SigHashType(data[len(data)-1])
			name, ok := sigHashTypeNames[hashType&^(txscript.SigHashForkID|txscript.SigHashAnyOneCanPay)]
			if ok && hashType&txscript.SigHashForkID != 0 &&
				txscript.CheckTransactionSignatureEncoding(data, txscript.StandardVerifyFlags) == nil {

				name += "|FORKID"
				if hashType&txscript.SigHashAnyOneCanPay != 0 {
					name += "|ANYONECANPAY"
				}
				buf.WriteString(hex.EncodeToString(data[:len(data)-1]))
				buf.WriteString("[" + name + "]")
				continue
			}
		}
		buf.WriteString(hex.EncodeToString(data))
	}
	return buf.String()
}

// dsProofSpenderResult returns the JSON model of a proof spender.
func dsProofSpenderResult(s *wire.DSProofSpender) btcjson.DSProofSpender {
	builder := txscript.NewScriptBuilder()
	for _, data := range s.PushData {
		builder.AddData(data)
	}
	script, _ := builder.Script()
	return btcjson.DSProofSpender{
		TxVersion:       s.Version,
		Sequence:        s.Sequence,
		LockTime:        s.LockTime,
		HashPrevOutputs: s.HashPrevOutputs.String(),
		HashSequence:    s.HashSequence.String(),
		HashOutputs:     s.HashOutputs.String(),
		PushData: btcjson.DSProofPushData{
			Asm: dsProofPushDataAsm(s.PushData),
			Hex: hex.EncodeToString(script),
		},
	}
}

// dsProofResult returns the JSON model of a proof at the given verbosity,
// which must be at least 1.
func dsProofResult(desc *mempool.DSProofDesc, verbosity int, path, descendants []chainhash.Hash) *btcjson.GetDSProofResult {
	result := &btcjson.GetDSProofResult{}
	if desc.TxHash != nil {
		txID := desc.TxHash.String()
		result.TxID = &txID
	}
	if verbosity == 1 {
		var buf bytes.Buffer
		buf.Grow(desc.Proof.SerializeSize())
		_ = desc.Proof.BchEncode(&buf, 0, wire.BaseEncoding)
		result.Hex = hex.EncodeToString(buf.Bytes())
	} else {
		result.DSPID = desc.Hash.String()
		result.OutPoint = &btcjson.DSProofOutPoint{
			TxID: desc.Proof.PrevOut.Hash.String(),
			Vout: desc.Proof.PrevOut.Index,
		}
	}
	for _, h := range path {
		result.Path = append(result.Path, h.String())
	}
	for _, h := range descendants {
		result.Descendants = append(result.Descendants, h.String())
	}
	if verbosity == 3 {
		result.Spenders = []btcjson.DSProofSpender{
			dsProofSpenderResult(&desc.Proof.FirstSpender),
			dsProofSpenderResult(&desc.Proof.DoubleSpender),
		}
	}
	return result
}

// dsProofQueryHash parses a proof id or transaction id parameter.
func dsProofQueryHash(s, name string) (*chainhash.Hash, error) {
	if len(s) != 2*chainhash.HashSize {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: fmt.Sprintf("%s must be of length 64 (not %d, for '%s')", name, len(s), s),
		}
	}
	hash, err := chainhash.NewHashFromStr(s)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: fmt.Sprintf("%s must be hexadecimal string (not '%s')", name, s),
		}
	}
	return hash, nil
}

// handleGetDSProof implements the getdsproof command.
func handleGetDSProof(s *rpcServer, cmd interface{}, closeNotifier <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetDSProofCmd)
	if !s.cfg.TxMemPool.DSProofsEnabled() {
		return nil, errDSProofDisabled
	}
	verbosity, err := dsProofVerbosity(c.Verbosity, 2)
	if err != nil {
		return nil, err
	}
	recursive := c.Recursive == nil || *c.Recursive
	pool := s.cfg.TxMemPool

	var (
		desc              *mempool.DSProofDesc
		path, descendants []chainhash.Hash
	)
	if c.Query.IsOutPoint() {
		op, err := c.Query.OutPoint()
		if err != nil {
			return nil, btcjson.NewRPCError(btcjson.ErrRPCType, err.Error())
		}
		if op.Vout < 0 || op.Vout > math.MaxUint32 {
			return nil, btcjson.NewRPCError(btcjson.ErrRPCInvalidParameter,
				"vout must be positive")
		}
		hash, err := dsProofQueryHash(op.TxID, "txid")
		if err != nil {
			return nil, err
		}
		desc, descendants = pool.DSProofByOutPoint(wire.OutPoint{Hash: *hash, Index: uint32(op.Vout)})
	} else {
		hash, err := dsProofQueryHash(string(c.Query), "dspid|txid")
		if err != nil {
			return nil, err
		}
		desc, descendants = pool.DSProofByHash(hash)
		if desc == nil {
			desc, descendants, path, err = pool.DSProofByTx(hash, recursive)
			if err != nil {
				return nil, &btcjson.RPCError{
					Code:    btcjson.ErrRPCMisc,
					Message: err.Error(),
				}
			}
		}
	}

	if desc == nil {
		return nil, nil
	}
	// Descendants are only listed from verbosity 1.
	if verbosity == 0 {
		descendants = nil
	}
	return dsProofResult(desc, max(verbosity, 1), path, descendants), nil
}

// handleGetDSProofList implements the getdsprooflist command.
func handleGetDSProofList(s *rpcServer, cmd interface{}, closeNotifier <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetDSProofListCmd)
	if !s.cfg.TxMemPool.DSProofsEnabled() {
		return nil, errDSProofDisabled
	}
	verbosity, err := dsProofVerbosity(c.Verbosity, 0)
	if err != nil {
		return nil, err
	}
	includeOrphans := c.IncludeOrphans != nil && *c.IncludeOrphans

	descs := s.cfg.TxMemPool.DSProofs(includeOrphans)
	if verbosity == 0 {
		ids := make([]string, 0, len(descs))
		for _, desc := range descs {
			ids = append(ids, desc.Hash.String())
		}
		return ids, nil
	}
	results := make([]*btcjson.GetDSProofResult, 0, len(descs))
	for _, desc := range descs {
		results = append(results, dsProofResult(desc, verbosity, nil, nil))
	}
	return results, nil
}

// handleGetDSProofScore implements the getdsproofscore command.
func handleGetDSProofScore(s *rpcServer, cmd interface{}, closeNotifier <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetDSProofScoreCmd)
	if !s.cfg.TxMemPool.DSProofsEnabled() {
		return nil, errDSProofDisabled
	}
	txHash, err := dsProofQueryHash(c.TxID, "txid")
	if err != nil {
		return nil, err
	}
	score, err := s.cfg.TxMemPool.DSProofScore(txHash)
	if err != nil {
		return nil, btcjson.NewRPCError(btcjson.ErrRPCInvalidAddressOrKey,
			"Transaction not in mempool")
	}
	return score, nil
}
