// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// bcmrGrpcAddress returns a free local address for a gRPC listener, below the
// ephemeral ports that outgoing connections take, as the harness's ports are.
func bcmrGrpcAddress(t *testing.T) string {
	t.Helper()
	for port := 29000 + os.Getpid()%2000; port < 32000; port++ {
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		l, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		l.Close()
		return addr
	}
	t.Fatal("no free port for gRPC")
	return ""
}

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

// checkBcmrGrpc checks that GetBcmrIdentity gives the answer that
// getbcmridentity gave with includeauthchain, for a token whose genesis is in
// genesisBlock, and errors for an unknown authbase and a link of no authchain.
func checkBcmrGrpc(t *testing.T, address string, category chainhash.Hash,
	result *btcjson.GetBcmrIdentityResult, genesisBlock *bchutil.Block) {

	t.Helper()
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(
		credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatalf("connecting to gRPC: %v", err)
	}
	defer conn.Close()
	client := pb.NewBchrpcClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.GetBcmrIdentity(ctx, &pb.GetBcmrIdentityRequest{
		Authbase: category[:], IncludeAuthchain: true,
	})
	if err != nil {
		t.Fatalf("GetBcmrIdentity: %v", err)
	}
	head, _ := chainhash.NewHashFromStr(result.Head.Txid)
	if !bytes.Equal(resp.Head.Hash, head[:]) || resp.Head.Height != 0 ||
		resp.Status != pb.GetBcmrIdentityResponse_ACTIVE {

		t.Errorf("gRPC head %v, status %v", resp.Head, resp.Status)
	}
	pubLink, _ := chainhash.NewHashFromStr(result.Publication.Txid)
	if resp.Publication == nil || !bytes.Equal(resp.Publication.Hash, pubLink[:]) ||
		resp.PublicationCurrent != result.Publication.Current ||
		hex.EncodeToString(resp.Publication.Publication.Hash) != result.Publication.Hash {

		t.Errorf("gRPC publication %v", resp.Publication)
	}
	if !resp.HasGenesisSupply || strconv.FormatUint(resp.GenesisSupply, 10) != result.GenesisSupply {
		t.Errorf("gRPC genesis supply %d (%v)", resp.GenesisSupply, resp.HasGenesisSupply)
	}
	if len(resp.Authchain) != len(*result.Authchain) || resp.More {
		t.Fatalf("gRPC authchain of %d links, more %v", len(resp.Authchain), resp.More)
	}
	for i, link := range resp.Authchain {
		key := strconv.Itoa(int(resp.FirstIndex) + i)
		if hex.EncodeToString(link.Transaction) != (*result.Authchain)[key] {
			t.Errorf("gRPC link %s differs", key)
		}
	}

	// The proof is of the genesis, with the hashes and flags of the merkle
	// block that getbcmridentity encodes.
	raw, err := hex.DecodeString(result.Proof.MerkleBlock)
	if err != nil {
		t.Fatal(err)
	}
	var merkleBlock wire.MsgMerkleBlock
	err = merkleBlock.BchDecode(bytes.NewReader(raw), wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		t.Fatal(err)
	}
	proof := resp.Proof
	if proof == nil || strconv.Itoa(int(proof.Index)) != result.Proof.Index ||
		!bytes.Equal(proof.Block.Hash, genesisBlock.Hash()[:]) ||
		!bytes.Equal(proof.Flags, merkleBlock.Flags) || len(proof.Hashes) != len(merkleBlock.Hashes) ||
		proof.Transactions != merkleBlock.Transactions {

		t.Fatalf("gRPC proof %v", proof)
	}
	for i, hash := range merkleBlock.Hashes {
		if !bytes.Equal(proof.Hashes[i], hash[:]) {
			t.Errorf("gRPC proof hash %d differs", i)
		}
	}

	unknown := chainhash.Hash{7}
	_, err = client.GetBcmrIdentity(ctx, &pb.GetBcmrIdentityRequest{Authbase: unknown[:]})
	if status.Code(err) != codes.NotFound {
		t.Errorf("gRPC for an unknown authbase: %v", err)
	}
	_, err = client.GetBcmrIdentity(ctx, &pb.GetBcmrIdentityRequest{
		Authbase: category[:], IncludeAuthchain: true, After: unknown[:],
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("gRPC after a link of no authchain: %v", err)
	}
}

// TestBcmrIdentity checks getbcmridentity on a regtest node with
// --bcmrindex: a token's identity from its genesis, a link in the mempool,
// the authchain in pages with a proof that verifytxoutproof accepts, a block
// invalidated and reconsidered, a burn, and the errors.  It checks that
// GetBcmrIdentity gives the same answers over gRPC.
func TestBcmrIdentity(t *testing.T) {
	grpcAddress := bcmrGrpcAddress(t)
	h, err := rpctest.New(&chaincfg.RegressionNetParams, nil,
		[]string{"--bcmrindex", "--grpclisten=" + grpcAddress})
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

	// The gRPC calls give the same answers, with hashes little-endian.
	checkBcmrGrpc(t, grpcAddress, category, result, genesisBlock)

	// The client's calls give the same answers.
	byClient, err := h.Node.GetBcmrIdentity(&category, true, nil, 0)
	if err != nil {
		t.Fatalf("GetBcmrIdentity: %v", err)
	}
	if byClient.Head != result.Head || len(*byClient.Authchain) != len(*result.Authchain) ||
		byClient.Proof == nil || *byClient.Proof != *result.Proof {

		t.Errorf("GetBcmrIdentity differs: head %+v, proof %+v", byClient.Head, byClient.Proof)
	}
	firstPage, err := h.Node.GetBcmrIdentity(&category, true, nil, 1)
	if err != nil {
		t.Fatalf("GetBcmrIdentity with a limit: %v", err)
	}
	if len(*firstPage.Authchain) != 1 || firstPage.More == nil || !*firstPage.More {
		t.Errorf("first page of one: %d links, more %v", len(*firstPage.Authchain),
			firstPage.More)
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

// waitForBcmrIdentity calls getbcmridentity until the walker has found the
// identity, and fails the test if it doesn't in time.
func waitForBcmrIdentity(t *testing.T, h *rpctest.Harness, params ...interface{}) *btcjson.GetBcmrIdentityResult {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		result, err := bcmrIdentity(h, params...)
		if err == nil {
			return result
		}
		var rpcErr *btcjson.RPCError
		if !errors.As(err, &rpcErr) || rpcErr.Code != btcjson.ErrRPCInvalidAddressOrKey {
			t.Fatalf("getbcmridentity %v: %v", params, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("getbcmridentity %v: no identity in time", params)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// checkBcmrAuthchain checks that an answer's authchain is the passed
// transactions, from the authbase, with the publication of pub and no genesis
// supply.
func checkBcmrAuthchain(t *testing.T, result *btcjson.GetBcmrIdentityResult, pub *wire.MsgTx,
	txs ...*wire.MsgTx) {

	t.Helper()
	if len(*result.Authchain) != len(txs) || *result.More {
		t.Fatalf("authchain of %d links, more %v, want %d", len(*result.Authchain),
			*result.More, len(txs))
	}
	for i, tx := range txs {
		if (*result.Authchain)[strconv.Itoa(i)] != bcmrHex(t, tx) {
			t.Errorf("link %d differs", i)
		}
	}
	if head := txs[len(txs)-1].TxHash(); result.Head.Txid != head.String() {
		t.Errorf("head %s, want %v", result.Head.Txid, head)
	}
	if pubHash := pub.TxHash(); result.Publication == nil || result.Publication.Txid != pubHash.String() {
		t.Errorf("publication %+v, want %v's", result.Publication, pubHash)
	}
	if result.GenesisSupply != "" {
		t.Errorf("genesis supply %s", result.GenesisSupply)
	}
}

// TestBcmrPublications checks, on a regtest node with --bcmrindex, that a
// registry's authbase behind its first publication answers once the walker
// has gone behind the publication, and that a branch the index didn't track,
// which merges into a token's authchain before a publication, answers from
// where it begins, while the token's authchain is unchanged; and that a block
// invalidated and reconsidered undoes the walk and has it walked again.
func TestBcmrPublications(t *testing.T) {
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

	// The outputs spent below are none an output 0, so that nothing is
	// behind the transactions that spend them.
	split := bcmrSpend(fund, fundIndex)
	for i := 0; i < 4; i++ {
		split.AddTxOut(wire.NewTxOut(20000000, bcmrOpTrue, wire.TokenData{}))
	}
	bcmrMine(t, h, split)

	// A registry publishes for the first time, a transaction after its
	// authbase.  Outputs of change keep the transactions above the
	// smallest size.
	change := wire.NewTxOut(10000, bcmrOpTrue, wire.TokenData{})
	base := bcmrSpend(split, 1, wire.NewTxOut(19000000, bcmrOpTrue, wire.TokenData{}), change)
	bcmrMine(t, h, base)
	registry := bcmrSpend(base, 0, wire.NewTxOut(18000000, bcmrOpTrue, wire.TokenData{}),
		bcmrPublicationOut(1, "registry.example"))
	bcmrMine(t, h, registry)
	result := waitForBcmrIdentity(t, h, base.TxHash().String(), true)
	checkBcmrAuthchain(t, result, registry, base, registry)

	// A token's authchain, which a branch the index doesn't track merges
	// into before a publication.
	authbase := bcmrSpend(split, 2, wire.NewTxOut(19000000, bcmrOpTrue, wire.TokenData{}), change)
	branch := bcmrSpend(split, 3, wire.NewTxOut(19000000, bcmrOpTrue, wire.TokenData{}), change)
	bcmrMine(t, h, authbase)
	bcmrMine(t, h, branch)
	category := authbase.TxHash()
	tokens := wire.TokenData{CategoryID: category, Amount: 50, BitField: wire.HAS_AMOUNT}
	genesis := bcmrSpend(authbase, 0, wire.NewTxOut(18000000, bcmrOpTrue, tokens), change)
	bcmrMine(t, h, genesis)
	link := bcmrSpend(genesis, 0, wire.NewTxOut(17000000, bcmrOpTrue, tokens), change)
	bcmrMine(t, h, link)
	merge := bcmrSpend(link, 0, wire.NewTxOut(35000000, bcmrOpTrue, tokens))
	merge.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: branch.TxHash(), Index: 0}, nil))
	bcmrMine(t, h, merge)
	update := bcmrSpend(merge, 0, wire.NewTxOut(34000000, bcmrOpTrue, tokens),
		bcmrPublicationOut(2, "token.example"))
	updateBlock := bcmrMine(t, h, update)

	result = waitForBcmrIdentity(t, h, branch.TxHash().String(), true)
	checkBcmrAuthchain(t, result, update, branch, merge, update)
	result = mustBcmrIdentity(t, h, category.String(), true)
	if len(*result.Authchain) != 5 || result.GenesisSupply != "50" ||
		(*result.Authchain)["3"] != bcmrHex(t, merge) || (*result.Authchain)["4"] != bcmrHex(t, update) ||
		result.Publication == nil || result.Publication.Txid != update.TxHash().String() {

		t.Errorf("the token's authchain of %d links, supply %s, publication %+v",
			len(*result.Authchain), result.GenesisSupply, result.Publication)
	}

	// Invalidating the block of the publication undoes the walk behind it,
	// and reconsidering it has the walker walk again.
	blockParam, _ := json.Marshal(updateBlock.Hash().String())
	if _, err := h.Node.RawRequest("invalidateblock", []json.RawMessage{blockParam}); err != nil {
		t.Fatalf("invalidateblock: %v", err)
	}
	checkBcmrRPCError(t, h, btcjson.ErrRPCInvalidAddressOrKey, branch.TxHash().String())
	if _, err := h.Node.RawRequest("reconsiderblock", []json.RawMessage{blockParam}); err != nil {
		t.Fatalf("reconsiderblock: %v", err)
	}
	result = waitForBcmrIdentity(t, h, branch.TxHash().String(), true)
	checkBcmrAuthchain(t, result, update, branch, merge, update)
}
