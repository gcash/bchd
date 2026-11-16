// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"slices"
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/shv"
	"github.com/gcash/bchd/wire"
)

// newSHVRPCServer returns an RPC server over a regtest chain of the passed
// number of blocks after genesis, with the chain's SHV index, and the main
// chain's block hashes.
func newSHVRPCServer(t *testing.T, blocks int) (*rpcServer, []chainhash.Hash) {
	t.Helper()
	chain, db := newRPCTestChain(t)
	quietSHVLog(t)
	quietRPCLog(t)
	x, err := newSHVIndex(chain)
	if err != nil {
		t.Fatal(err)
	}
	chain.Subscribe(x.handleBlockchainNotification)
	for range blocks {
		mineSHVBlock(t, chain, 0)
	}
	s := &rpcServer{cfg: rpcserverConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Chain:       chain,
		DB:          db,
		SHVIndex:    x,
	}}
	return s, mainChainHashes(t, chain)
}

// TestHandleGetSHV ensures getshv returns the header and the proof to the
// root or the peak, which an accumulator over the main chain verifies, with
// only the target its type asks for, and an empty branch for a block that is
// its own target.
func TestHandleGetSHV(t *testing.T) {
	s, hashes := newSHVRPCServer(t, 10)

	for _, test := range []struct {
		typ                string
		commitment, height int64
		emptyBranch        bool
	}{
		{"root", 10, 4, false},
		{"peak", 10, 4, false},
		{"root", 7, 7, false},
		{"peak", 9, 9, false},
		{"root", 0, 0, true},
		{"peak", 10, 10, true},
	} {
		result, err := handleGetSHV(s, btcjson.NewGetSHVCmd(test.typ,
			test.commitment, test.height), nil)
		if err != nil {
			t.Fatalf("%+v: %v", test, err)
		}
		reply := result.(*btcjson.GetSHVResult)

		rawHeader, err := hex.DecodeString(reply.Header)
		if err != nil {
			t.Fatal(err)
		}
		var header wire.BlockHeader
		if err := header.Deserialize(bytes.NewReader(rawHeader)); err != nil {
			t.Fatal(err)
		}
		leaf := header.BlockHash()
		if leaf != hashes[test.height] {
			t.Fatalf("%+v: header of %v, want %v", test, leaf,
				hashes[test.height])
		}
		branch := make([]chainhash.Hash, len(reply.Branch))
		for i, s := range reply.Branch {
			hash, err := chainhash.NewHashFromStr(s)
			if err != nil {
				t.Fatal(err)
			}
			branch[i] = *hash
		}
		if (len(branch) == 0) != test.emptyBranch {
			t.Fatalf("%+v: branch of %d hashes", test, len(branch))
		}

		var acc shv.Accumulator
		for i := range hashes[:test.commitment+1] {
			if err := acc.Extend(&hashes[i]); err != nil {
				t.Fatal(err)
			}
		}
		height := uint64(test.height)
		switch test.typ {
		case "root":
			if reply.Root != acc.Root().String() || reply.Peak != "" ||
				!acc.VerifyProofToRoot(height, &leaf, branch) {

				t.Fatalf("%+v: proof to the root not verified", test)
			}
		case "peak":
			peak, err := chainhash.NewHashFromStr(reply.Peak)
			if err != nil || reply.Root != "" ||
				!slices.Contains(acc.Peaks(), *peak) ||
				!acc.VerifyProofToPeak(height, &leaf, branch) {

				t.Fatalf("%+v: proof to the peak not verified", test)
			}
		}

		// The JSON has the CHIP's keys, and a branch that is empty is
		// an empty array.
		raw, err := json.Marshal(reply)
		if err != nil {
			t.Fatal(err)
		}
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keys); err != nil {
			t.Fatal(err)
		}
		if _, ok := keys[test.typ]; !ok || len(keys) != 3 ||
			keys["branch"][0] != '[' {

			t.Fatalf("%+v: got %s", test, raw)
		}
	}
}

// TestHandleGetSHVErrors ensures getshv fails with the CHIP's codes and
// messages: when SHV is disabled, for an unknown type, negative heights, a
// block above the commitment, a block above the chain's tip, and a commitment
// above the tip.
func TestHandleGetSHVErrors(t *testing.T) {
	s, _ := newSHVRPCServer(t, 5)
	for _, test := range []struct {
		cmd     *btcjson.GetSHVCmd
		code    btcjson.RPCErrorCode
		message string
	}{
		{btcjson.NewGetSHVCmd("leaf", 5, 1), btcjson.ErrRPCInvalidParameter,
			"Unknown type"},
		{btcjson.NewGetSHVCmd("Root", 5, 1), btcjson.ErrRPCInvalidParameter,
			"Unknown type"},
		{btcjson.NewGetSHVCmd("root", -1, 0), btcjson.ErrRPCInvalidParameter,
			"commitment_height must be non-negative"},
		{btcjson.NewGetSHVCmd("peak", 5, -1), btcjson.ErrRPCInvalidParameter,
			"block_height must be non-negative"},
		{btcjson.NewGetSHVCmd("root", 3, 4), btcjson.ErrRPCInvalidParameter,
			"block_height cannot exceed commitment_height"},
		{btcjson.NewGetSHVCmd("root", 6, 6), btcjson.ErrRPCInvalidParameter,
			"Block not found"},
		{btcjson.NewGetSHVCmd("peak", 1<<40, 1<<35),
			btcjson.ErrRPCInvalidParameter, "Block not found"},
		{btcjson.NewGetSHVCmd("root", 6, 5), btcjson.ErrRPCInvalidParameter,
			"Failed to generate proof"},
		{btcjson.NewGetSHVCmd("peak", 1<<40, 0),
			btcjson.ErrRPCInvalidParameter, "Failed to generate proof"},
	} {
		_, err := handleGetSHV(s, test.cmd, nil)
		assertRPCError(t, test.cmd.Type, err, test.code, test.message)
	}

	s.cfg.SHVIndex = nil
	_, err := handleGetSHV(s, btcjson.NewGetSHVCmd("root", 5, 1), nil)
	assertRPCError(t, "disabled", err, btcjson.ErrRPCMisc,
		"MMR index is not enabled")
}

// TestSHVRootInBlockResults ensures verbose getblock and getblockheader give
// shvroot, the root of the commitment at the block's height, when SHV is
// enabled, and leave it out when it is disabled, while the hex results are
// unchanged.
func TestSHVRootInBlockResults(t *testing.T) {
	s, hashes := newSHVRPCServer(t, 6)
	index := s.cfg.SHVIndex

	var acc shv.Accumulator
	for height := range hashes {
		if err := acc.Extend(&hashes[height]); err != nil {
			t.Fatal(err)
		}
		want := acc.Root().String()
		hash := hashes[height].String()

		for _, enabled := range []bool{true, false} {
			s.cfg.SHVIndex = nil
			wantRoot := ""
			if enabled {
				s.cfg.SHVIndex = index
				wantRoot = want
			}

			result, err := handleGetBlockHeader(s,
				btcjson.NewGetBlockHeaderCmd(hash, btcjson.Bool(true)), nil)
			if err != nil {
				t.Fatal(err)
			}
			headerReply := result.(btcjson.GetBlockHeaderVerboseResult)
			checkSHVRootJSON(t, "getblockheader", headerReply,
				headerReply.SHVRoot, wantRoot)

			for _, verbosity := range []btcjson.VerbosityLevel{1, 2, 3} {
				result, err := handleGetBlock(s, btcjson.NewGetBlockCmd(hash,
					btcjson.Verbositylevel(verbosity)), nil)
				if err != nil {
					t.Fatal(err)
				}
				var root string
				switch reply := result.(type) {
				case btcjson.GetBlockVerboseResult:
					root = reply.SHVRoot
				case btcjson.GetBlockVerboseTxResult:
					root = reply.SHVRoot
				default:
					t.Fatalf("getblock verbosity %d: got %T", verbosity,
						result)
				}
				checkSHVRootJSON(t, "getblock", result, root, wantRoot)
			}

			// The hex results don't change.
			result, err = handleGetBlockHeader(s,
				btcjson.NewGetBlockHeaderCmd(hash, btcjson.Bool(false)), nil)
			if _, ok := result.(string); err != nil || !ok {
				t.Fatalf("getblockheader hex: %v, %v", result, err)
			}
		}
	}
}

// checkSHVRootJSON ensures a block result has the passed shvroot, and that its
// JSON has an shvroot key only when the root is not empty.
func checkSHVRootJSON(t *testing.T, method string, result interface{}, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: shvroot %q, want %q", method, got, want)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"shvroot"`)) != (want != "") {
		t.Fatalf("%s: got %s", method, raw)
	}
}
