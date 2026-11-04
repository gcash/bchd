// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// bcmrOpTrue is the locking bytecode of the outputs the BCMR test spends,
// which need no signature.
var bcmrOpTrue = []byte{txscript.OP_TRUE}

// bcmrIdentity calls getbcmridentity on a node with the passed parameters.
func bcmrIdentity(h *rpctest.Harness, params ...interface{}) (*btcjson.GetBcmrIdentityResult, error) {
	raw := make([]json.RawMessage, 0, len(params))
	for _, param := range params {
		encoded, err := json.Marshal(param)
		if err != nil {
			return nil, err
		}
		raw = append(raw, encoded)
	}
	reply, err := h.Node.RawRequest("getbcmridentity", raw)
	if err != nil {
		return nil, err
	}
	var result btcjson.GetBcmrIdentityResult
	if err := json.Unmarshal(reply, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// mustBcmrIdentity calls getbcmridentity and fails the test on an error.
func mustBcmrIdentity(t *testing.T, h *rpctest.Harness, params ...interface{}) *btcjson.GetBcmrIdentityResult {
	t.Helper()
	result, err := bcmrIdentity(h, params...)
	if err != nil {
		t.Fatalf("getbcmridentity %v: %v", params, err)
	}
	return result
}

// checkBcmrRPCError checks that a getbcmridentity call fails with code.
func checkBcmrRPCError(t *testing.T, h *rpctest.Harness, code btcjson.RPCErrorCode, params ...interface{}) {
	t.Helper()
	_, err := bcmrIdentity(h, params...)
	var rpcErr *btcjson.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != code {
		t.Errorf("getbcmridentity %v: %v, want code %d", params, err, code)
	}
}

// bcmrSpend returns a transaction that spends output index of prev, which
// pays to bcmrOpTrue, and has the passed outputs.
func bcmrSpend(prev *wire.MsgTx, index uint32, outs ...*wire.TxOut) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: prev.TxHash(), Index: index}, nil))
	for _, out := range outs {
		tx.AddTxOut(out)
	}
	return tx
}

// bcmrPublicationOut returns a publication output of a hash of 32 hashByte
// bytes and a URI.
func bcmrPublicationOut(hashByte byte, uri string) *wire.TxOut {
	script := []byte{txscript.OP_RETURN, txscript.OP_DATA_4, 'B', 'C', 'M', 'R', txscript.OP_DATA_32}
	script = append(script, bytes.Repeat([]byte{hashByte}, 32)...)
	script = append(script, byte(len(uri)))
	script = append(script, uri...)
	return wire.NewTxOut(0, script, wire.TokenData{})
}

// bcmrMine mines a block with the passed transactions.
func bcmrMine(t *testing.T, h *rpctest.Harness, txs ...*wire.MsgTx) *bchutil.Block {
	t.Helper()
	btxs := make([]*bchutil.Tx, 0, len(txs))
	for _, tx := range txs {
		btxs = append(btxs, bchutil.NewTx(tx))
	}
	block, err := h.GenerateAndSubmitBlock(btxs, -1, time.Time{})
	if err != nil {
		t.Fatalf("mining a block: %v", err)
	}
	return block
}

// bcmrHex returns the hex of a serialized transaction.
func bcmrHex(t *testing.T, tx *wire.MsgTx) string {
	t.Helper()
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(buf.Bytes())
}

// TestBcmrIdentity checks getbcmridentity on a regtest node with
// --bcmrindex: a token's identity from its genesis, a link in the mempool,
// the authchain in pages with a proof that verifytxoutproof accepts, a block
// invalidated and reconsidered, a burn, and the errors.
func TestBcmrIdentity(t *testing.T) {
	h, err := rpctest.New(&chaincfg.RegressionNetParams, nil, []string{"--bcmrindex"})
	if err != nil {
		t.Fatalf("creating the harness: %v", err)
	}
	defer func() {
		if err := h.TearDown(); err != nil {
			t.Errorf("tearing down the harness: %v", err)
		}
	}()
	if err := h.SetUp(true, 1); err != nil {
		t.Fatalf("setting up the harness: %v", err)
	}

	fund, err := h.CreateTransaction([]*wire.TxOut{
		wire.NewTxOut(100000000, bcmrOpTrue, wire.TokenData{}),
	}, 10, true)
	if err != nil {
		t.Fatalf("funding: %v", err)
	}
	bcmrMine(t, h, fund)
	fundIndex := uint32(0)
	for i, out := range fund.TxOut {
		if bytes.Equal(out.PkScript, bcmrOpTrue) {
			fundIndex = uint32(i)
		}
	}

	// The authbase publishes, and the genesis creates 1000 tokens in its
	// identity output and publishes.
	authbase := bcmrSpend(fund, fundIndex,
		wire.NewTxOut(99000000, bcmrOpTrue, wire.TokenData{}), bcmrPublicationOut(1, "a.example"))
	bcmrMine(t, h, authbase)
	category := authbase.TxHash()
	tokens := wire.TokenData{CategoryID: category, Amount: 1000, BitField: wire.HAS_AMOUNT}
	genesis := bcmrSpend(authbase, 0,
		wire.NewTxOut(98000000, bcmrOpTrue, tokens), bcmrPublicationOut(2, "g.example"))
	genesisBlock := bcmrMine(t, h, genesis)
	genesisHash := genesis.TxHash()

	result := mustBcmrIdentity(t, h, category.String())
	if result.Status != "active" || result.Head.Txid != genesisHash.String() ||
		result.Head.Height != genesisBlock.Height() ||
		result.Head.BlockHash != genesisBlock.Hash().String() {

		t.Errorf("status %s, head %+v", result.Status, result.Head)
	}
	if pub := result.Publication; pub == nil || pub.Txid != genesisHash.String() || !pub.Current ||
		pub.Output != 1 || pub.Hash != hex.EncodeToString(bytes.Repeat([]byte{2}, 32)) ||
		len(pub.URIs) != 1 || pub.URIs[0] != "g.example" {

		t.Errorf("publication %+v", pub)
	}
	if result.GenesisSupply != "1000" || result.IdentityOutput == nil ||
		result.IdentityOutput.Category != category.String() || result.IdentityOutput.Amount != "1000" {

		t.Errorf("genesis supply %s, identity output %+v", result.GenesisSupply, result.IdentityOutput)
	}
	if result.Authchain != nil || result.More != nil || result.Proof != nil {
		t.Error("the answer has an authchain it wasn't asked for")
	}

	// A link in the mempool.
	link := bcmrSpend(genesis, 0, wire.NewTxOut(97000000, bcmrOpTrue, tokens))
	if _, err := h.Node.SendRawTransaction(link, true); err != nil {
		t.Fatalf("sending the link: %v", err)
	}
	linkHash := link.TxHash()
	result = mustBcmrIdentity(t, h, category.String(), true)
	if result.Head.Txid != linkHash.String() || result.Head.Height != 0 || result.Head.BlockHash != "" {
		t.Errorf("head %+v", result.Head)
	}
	if pub := result.Publication; pub == nil || pub.Txid != genesisHash.String() || pub.Current {
		t.Errorf("publication %+v", pub)
	}
	want := map[string]string{
		"0": bcmrHex(t, authbase),
		"1": bcmrHex(t, genesis),
		"2": bcmrHex(t, link),
	}
	if len(*result.Authchain) != len(want) || result.More == nil || *result.More {
		t.Fatalf("authchain of %d links, more %v", len(*result.Authchain), result.More)
	}
	for key, tx := range want {
		if (*result.Authchain)[key] != tx {
			t.Errorf("link %s differs", key)
		}
	}
	if info := (*result.AuthchainInfo)["1"]; info.Txid != genesisHash.String() ||
		info.BlockHash != genesisBlock.Hash().String() || info.Publication == nil {

		t.Errorf("link 1: %+v", info)
	}
	if result.Proof == nil || result.Proof.Index != "1" {
		t.Fatalf("proof %+v", result.Proof)
	}
	proofParam, _ := json.Marshal(result.Proof.MerkleBlock)
	reply, err := h.Node.RawRequest("verifytxoutproof", []json.RawMessage{proofParam})
	if err != nil {
		t.Fatalf("verifytxoutproof: %v", err)
	}
	var proven []string
	if err := json.Unmarshal(reply, &proven); err != nil || len(proven) != 1 ||
		proven[0] != genesisHash.String() {

		t.Errorf("verifytxoutproof proves %v, %v", proven, err)
	}

	// The client's calls give the same answers.
	byClient, err := h.Node.GetBcmrIdentity(&category, true, nil, 0)
	if err != nil {
		t.Fatalf("GetBcmrIdentity: %v", err)
	}
	if byClient.Head != result.Head || len(*byClient.Authchain) != len(*result.Authchain) ||
		byClient.Proof == nil || *byClient.Proof != *result.Proof {

		t.Errorf("GetBcmrIdentity differs: head %+v, proof %+v", byClient.Head, byClient.Proof)
	}
	// Pages, and the page after the authhead, which has no links but
	// still an authchain.
	result = mustBcmrIdentity(t, h, category.String(), true, genesisHash.String(), 1)
	if len(*result.Authchain) != 1 || (*result.Authchain)["2"] != want["2"] || *result.More ||
		result.Proof == nil || result.Proof.Index != "1" {

		t.Errorf("page after the genesis: %d links, more %v, proof %+v",
			len(*result.Authchain), *result.More, result.Proof)
	}
	result = mustBcmrIdentity(t, h, category.String(), true, nil, 1)
	if len(*result.Authchain) != 1 || (*result.Authchain)["0"] != want["0"] || !*result.More ||
		result.Proof != nil {

		t.Errorf("first page: %d links, more %v, proof %+v",
			len(*result.Authchain), *result.More, result.Proof)
	}

	empty := mustBcmrIdentity(t, h, category.String(), true, linkHash.String())
	if empty.Authchain == nil || len(*empty.Authchain) != 0 || empty.AuthchainInfo == nil ||
		*empty.More || empty.Proof == nil {

		t.Errorf("page after the authhead: authchain %v, more %v, proof %+v",
			empty.Authchain, *empty.More, empty.Proof)
	}

	// Mined, then the block is invalidated and reconsidered.
	hashes, err := h.Node.Generate(1)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	result = mustBcmrIdentity(t, h, category.String())
	if result.Head.Txid != linkHash.String() || result.Head.BlockHash != hashes[0].String() {
		t.Errorf("mined head %+v", result.Head)
	}
	blockParam, _ := json.Marshal(hashes[0].String())
	if _, err := h.Node.RawRequest("invalidateblock", []json.RawMessage{blockParam}); err != nil {
		t.Fatalf("invalidateblock: %v", err)
	}
	result = mustBcmrIdentity(t, h, category.String())
	if result.Head.BlockHash == hashes[0].String() {
		t.Errorf("the head is still in the invalidated block: %+v", result.Head)
	}
	if _, err := h.Node.RawRequest("reconsiderblock", []json.RawMessage{blockParam}); err != nil {
		t.Fatalf("reconsiderblock: %v", err)
	}
	result = mustBcmrIdentity(t, h, category.String())
	if result.Head.Txid != linkHash.String() || result.Head.BlockHash != hashes[0].String() {
		t.Errorf("reconsidered head %+v", result.Head)
	}

	// A burn that publishes a final registry.
	burn := bcmrSpend(link, 0, bcmrPublicationOut(4, "final.example"),
		wire.NewTxOut(96000000, bcmrOpTrue, wire.TokenData{}))
	bcmrMine(t, h, burn)
	burnHash := burn.TxHash()
	result = mustBcmrIdentity(t, h, category.String())
	if result.Status != "burned" || result.Head.Txid != burnHash.String() ||
		result.Publication == nil || result.Publication.Txid != burnHash.String() ||
		!result.Publication.Current || result.Publication.Output != 0 || result.IdentityOutput != nil {

		t.Errorf("burned: status %s, head %+v, publication %+v, identity output %+v",
			result.Status, result.Head, result.Publication, result.IdentityOutput)
	}

	// Errors.
	checkBcmrRPCError(t, h, btcjson.ErrRPCInvalidAddressOrKey, chainhash.Hash{7}.String())
	checkBcmrRPCError(t, h, btcjson.ErrRPCInvalidAddressOrKey, category.String(), true,
		chainhash.Hash{7}.String())
	checkBcmrRPCError(t, h, btcjson.ErrRPCInvalidParameter, category.String(), true, nil, 0)
	checkBcmrRPCError(t, h, btcjson.ErrRPCInvalidParameter, category.String(), false,
		genesisHash.String())
	checkBcmrRPCError(t, h, btcjson.ErrRPCDecodeHexString, "xyz")
}
