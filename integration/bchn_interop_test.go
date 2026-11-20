// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build interop

package integration

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// interopNode calls the JSON-RPC methods of a node.
type interopNode struct {
	name string
	call func(method string, params ...any) (json.RawMessage, error)
}

// mustCall calls a method and decodes its result into result, if not nil.
func (n *interopNode) mustCall(t *testing.T, result any, method string,
	params ...any) {

	t.Helper()
	raw, err := n.call(method, params...)
	if err != nil {
		t.Fatalf("%s %s: %v", n.name, method, err)
	}
	if result != nil {
		if err := json.Unmarshal(raw, result); err != nil {
			t.Fatalf("%s %s: decoding %s: %v", n.name, method, raw,
				err)
		}
	}
}

// interopHeader holds fields of getblockheader.  Both nodes report all but
// the median time past, which only BCHN reports.
type interopHeader struct {
	Hash           string `json:"hash"`
	Height         int32  `json:"height"`
	Bits           string `json:"bits"`
	MedianTime     int64  `json:"mediantime"`
	Ticks          int64  `json:"ticks"`
	TargetTime     int64  `json:"target_time"`
	NextTargetTime int64  `json:"next_target_time"`
}

// bchnNode starts bitcoind on regtest in a fresh data directory and returns a
// client for it.  The node is stopped when the test ends.
func bchnNode(t *testing.T, bitcoind string, activationTime uint64) (*interopNode, string) {
	t.Helper()
	dataDir := t.TempDir()
	port, rpcPort := freePort(t), freePort(t)
	cmd := exec.Command(bitcoind, "-regtest", "-datadir="+dataDir,
		fmt.Sprintf("-upgrade2027activationtime=%d", activationTime),
		"-expire=0", "-finalizationdelay=0", "-server", "-listen",
		"-bind=127.0.0.1", "-rpcbind=127.0.0.1",
		"-rpcallowip=127.0.0.1", "-whitelist=127.0.0.1",
		"-dnsseed=0", "-discover=0", "-disablewallet",
		"-printtoconsole=0", "-rpcuser=interop", "-rpcpassword=interop",
		fmt.Sprintf("-port=%d", port), fmt.Sprintf("-rpcport=%d", rpcPort))
	if err := cmd.Start(); err != nil {
		t.Fatalf("unable to start bitcoind: %v", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/", rpcPort)
	var id int
	node := &interopNode{name: "BCHN", call: func(method string,
		params ...any) (json.RawMessage, error) {

		id++
		if params == nil {
			params = []any{}
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "1.0",
			"id": id, "method": method, "params": params})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth("interop", "interop")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var reply struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
			return nil, fmt.Errorf("HTTP %s: %v", resp.Status, err)
		}
		if reply.Error != nil {
			return nil, fmt.Errorf("%d: %s", reply.Error.Code,
				reply.Error.Message)
		}
		return reply.Result, nil
	}}

	t.Cleanup(func() {
		node.call("stop")
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(60 * time.Second):
			cmd.Process.Kill()
			<-done
		}
		if t.Failed() {
			log, _ := os.ReadFile(filepath.Join(dataDir, "regtest",
				"debug.log"))
			if len(log) > 20000 {
				log = log[len(log)-20000:]
			}
			t.Logf("bitcoind debug.log tail:\n%s", log)
		}
	})

	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, err := node.call("getblockcount"); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("bitcoind RPC did not come up: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return node, fmt.Sprintf("127.0.0.1:%d", port)
}

// freePort returns a TCP port that is free on the loopback interface.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestBCHNInterop runs a bchd and a BCHN regtest node with the upgrade to
// 1-minute blocks (CHIP-2025-03) set to activate at a time in the past, mines
// across the seam on both, and checks that they agree on every block and on
// the outcome of the rules that change at the seam: the subsidy, coinbase
// maturity, relative locks and lock times released at the tick boundary, the
// forbidden coinbase shape and its escapes, the finality of a coinbase whatever
// its lock time, both also in the blocks on each side of the seam, and
// finalization at 6,000 ticks, also across the seam.
//
// The test needs a BCHN bitcoind with 1-minute block support at the path in
// the BCHN_BITCOIND environment variable and is skipped without one:
//
//	BCHN_BITCOIND=/path/to/bitcoind go test -tags interop -run TestBCHNInterop ./integration/
func TestBCHNInterop(t *testing.T) {
	bitcoind := os.Getenv("BCHN_BITCOIND")
	if bitcoind == "" {
		t.Skip("BCHN_BITCOIND is not set")
	}

	// A fixed activation time in the past.  The first blocks are mined
	// at earlier times, so that bchd and BCHN both mine 600-tick blocks
	// before the seam, which the median time past crosses six blocks
	// after the wall clock blocks start.
	const activationTime = rpctest.Upgrade2027InThePast
	bchn, bchnAddr := bchnNode(t, bitcoind, activationTime)

	h, err := rpctest.New(&chaincfg.RegressionNetParams, nil,
		[]string{rpctest.Upgrade2027Arg(activationTime),
			"--finalizationdelay=0"})
	if err != nil {
		t.Fatalf("unable to create harness: %v", err)
	}
	defer h.TearDown()
	if err := h.SetUp(false, 0); err != nil {
		t.Fatalf("unable to set up harness: %v", err)
	}
	bchd := &interopNode{name: "bchd", call: func(method string,
		params ...any) (json.RawMessage, error) {

		raw := make([]json.RawMessage, len(params))
		for i, p := range params {
			b, err := json.Marshal(p)
			if err != nil {
				return nil, err
			}
			raw[i] = b
		}
		return h.Node.RawRequest(method, raw)
	}}
	if err := h.Node.AddNode(bchnAddr, rpcclient.ANAdd); err != nil {
		t.Fatalf("unable to connect to BCHN: %v", err)
	}
	nodes := []*interopNode{bchn, bchd}

	// The coinbases BCHN mines pay a key of the test.
	params := &chaincfg.RegressionNetParams
	key, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatal(err)
	}
	addr, err := bchutil.NewAddressPubKeyHash(bchutil.Hash160(
		key.PubKey().SerializeCompressed()), params)
	if err != nil {
		t.Fatal(err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}

	tip := func(n *interopNode) interopHeader {
		t.Helper()
		var hash string
		n.mustCall(t, &hash, "getbestblockhash")
		var header interopHeader
		n.mustCall(t, &header, "getblockheader", hash)
		return header
	}
	sync := func() interopHeader {
		t.Helper()
		deadline := time.Now().Add(3 * time.Minute)
		for {
			a, b := tip(bchn), tip(bchd)
			if a.Hash == b.Hash {
				return a
			}
			if time.Now().After(deadline) {
				t.Fatalf("tips differ: BCHN %d %s, bchd %d %s",
					a.Height, a.Hash, b.Height, b.Hash)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	mineBCHN := func(n int) {
		t.Helper()
		bchn.mustCall(t, nil, "generatetoaddress", n,
			addr.EncodeAddress())
	}
	mineBCHD := func() {
		t.Helper()
		if _, err := h.Node.Generate(1); err != nil {
			t.Fatalf("bchd generate: %v", err)
		}
	}
	block := func(n *interopNode, height int32) *wire.MsgBlock {
		t.Helper()
		var hash, blockHex string
		n.mustCall(t, &hash, "getblockhash", height)
		n.mustCall(t, &blockHex, "getblock", hash, 0)
		raw, err := hex.DecodeString(blockHex)
		if err != nil {
			t.Fatal(err)
		}
		var msg wire.MsgBlock
		if err := msg.Deserialize(bytes.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
		return &msg
	}

	// ---------------------------------------------------------------------
	// Helpers for blocks and transactions built by the test.
	// ---------------------------------------------------------------------

	tipTicks := func() int64 { return sync().Ticks }

	// mineToTicks mines with BCHN until the tip has the passed ticks.
	mineToTicks := func(ticks int64) {
		t.Helper()
		current := tipTicks()
		if ticks < current || (ticks-current)%60 != 0 {
			t.Fatalf("cannot mine from tick %d to %d", current, ticks)
		}
		for n := (ticks - current) / 60; n > 0; n -= min(n, 200) {
			mineBCHN(int(min(n, 200)))
		}
		sync()
	}

	type coin struct {
		outPoint wire.OutPoint
		value    int64
		ticks    int64
	}
	coinbaseCoin := func(height int32) coin {
		t.Helper()
		cb := block(bchn, height).Transactions[0]
		if !bytes.Equal(cb.TxOut[0].PkScript, pkScript) {
			t.Fatalf("block %d was not mined by BCHN", height)
		}
		var hash string
		bchn.mustCall(t, &hash, "getblockhash", height)
		var header interopHeader
		bchn.mustCall(t, &header, "getblockheader", hash)
		return coin{wire.OutPoint{Hash: cb.TxHash(), Index: 0},
			cb.TxOut[0].Value, header.Ticks}
	}

	// spend returns a signed transaction that pays the coin less the fee
	// back to the key.
	spend := func(c coin, fee int64, version int32, sequence,
		lockTime uint32) *wire.MsgTx {

		t.Helper()
		tx := wire.NewMsgTx(version)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: c.outPoint,
			Sequence: sequence})
		tx.AddTxOut(wire.NewTxOut(c.value-fee, pkScript,
			wire.TokenData{}))
		tx.LockTime = lockTime
		sigScript, err := txscript.SignatureScript(tx, 0, c.value,
			pkScript, txscript.SigHashAll, key, true)
		if err != nil {
			t.Fatal(err)
		}
		tx.TxIn[0].SignatureScript = sigScript
		return tx
	}

	// nextBlock returns a solved block after the tip with the passed
	// transactions and a coinbase paying the subsidy and fees plus extra.
	nextBlock := func(txs []*wire.MsgTx, fees, extra int64,
		coinbase func(*wire.MsgTx)) *wire.MsgBlock {

		t.Helper()
		prev := sync()
		height := prev.Height + 1
		ticks := prev.Ticks + prev.NextTargetTime
		subsidy := blockchain.CalcBlockSubsidy(ticks, prev.NextTargetTime,
			params)

		cbScript, err := txscript.NewScriptBuilder().AddInt64(
			int64(height)).AddData(make([]byte, 80)).Script()
		if err != nil {
			t.Fatal(err)
		}
		cb := wire.NewMsgTx(2)
		cb.AddTxIn(&wire.TxIn{
			PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
				wire.MaxPrevOutIndex),
			SignatureScript: cbScript,
			Sequence:        wire.MaxTxInSequenceNum,
		})
		cb.AddTxOut(wire.NewTxOut(subsidy+fees+extra, pkScript,
			wire.TokenData{}))
		if coinbase != nil {
			coinbase(cb)
		}

		// Transactions after the coinbase are in txid order.
		utxs := []*bchutil.Tx{bchutil.NewTx(cb)}
		for _, tx := range txs {
			utxs = append(utxs, bchutil.NewTx(tx))
		}
		for i := 1; i < len(utxs); i++ {
			for j := i; j > 1 && bytes.Compare(
				utxs[j].Hash()[:], utxs[j-1].Hash()[:]) < 0; j-- {

				utxs[j], utxs[j-1] = utxs[j-1], utxs[j]
			}
		}
		merkles := blockchain.BuildMerkleTreeStore(utxs)
		prevHash, err := chainhash.NewHashFromStr(prev.Hash)
		if err != nil {
			t.Fatal(err)
		}
		bits, err := strconv.ParseUint(prev.Bits, 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		msg := &wire.MsgBlock{Header: wire.BlockHeader{
			Version:    0x20000000,
			PrevBlock:  *prevHash,
			MerkleRoot: *merkles[len(merkles)-1],
			Timestamp: time.Unix(max(time.Now().Unix(),
				prev.MedianTime+1), 0),
			Bits: uint32(bits),
		}}
		for _, tx := range utxs {
			msg.Transactions = append(msg.Transactions, tx.MsgTx())
		}
		target := blockchain.CompactToBig(msg.Header.Bits)
		for {
			hash := msg.Header.BlockHash()
			if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
				break
			}
			msg.Header.Nonce++
		}
		return msg
	}

	// submit submits a block to both nodes and checks the reason they
	// give, "" when they accept it.  It goes to bchd first, since the
	// other node may relay a block before it has fully checked it, and
	// bchd would then judge it as a block from a peer.
	submit := func(msg *wire.MsgBlock, want string) {
		t.Helper()
		var buf bytes.Buffer
		if err := msg.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		blockHex := hex.EncodeToString(buf.Bytes())
		for _, n := range []*interopNode{bchd, bchn} {
			var reason *string
			n.mustCall(t, &reason, "submitblock", blockHex)
			got := ""
			if reason != nil {
				got = *reason
			}
			if want == "" && (got == "duplicate" ||
				got == "inconclusive") {

				// Relayed by bchd first.
				continue
			}
			if got != want {
				t.Fatalf("%s: submitblock %s at height %d gave %q, "+
					"want %q", n.name, msg.BlockHash(),
					tip(n).Height+1, got, want)
			}
		}
		if want == "" {
			if got := sync().Hash; got != msg.BlockHash().String() {
				t.Fatalf("tip %s, want submitted block %s", got,
					msg.BlockHash())
			}
		}
	}

	// propose checks the reason both nodes give for the block as a
	// proposal, "" when they would accept it.  The chain is unchanged.
	propose := func(msg *wire.MsgBlock, want string) {
		t.Helper()
		var buf bytes.Buffer
		if err := msg.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		request := map[string]string{"mode": "proposal",
			"data": hex.EncodeToString(buf.Bytes())}
		for _, n := range nodes {
			var reason *string
			n.mustCall(t, &reason, "getblocktemplate", request)
			got := ""
			if reason != nil {
				got = *reason
			}
			if got != want {
				t.Fatalf("%s: proposal %s at height %d gave %q, "+
					"want %q", n.name, msg.BlockHash(),
					tip(n).Height+1, got, want)
			}
		}
	}

	// ---------------------------------------------------------------------
	// 600-tick blocks mined by BCHN at times before the activation time,
	// then blocks at the wall clock mined by each node in turn until the
	// median time past reaches the activation time, and past the seam.
	// ---------------------------------------------------------------------

	// Enough of them for a coinbase to be mature in the block after the
	// seam.
	const pastBlocks = 110
	for i := range pastBlocks {
		mockTime := int64(activationTime) - 600*(2*pastBlocks-int64(i))
		bchn.mustCall(t, nil, "setmocktime", mockTime)
		mineBCHN(1)
	}
	bchn.mustCall(t, nil, "setmocktime", 0)
	sync()

	// The coinbase rules that change at the seam, checked in every block
	// up to and including the activation block and in the block after it.
	// A coinbase with a lock time that is not final yet is refused, and
	// final after the seam.  A version 1 coinbase with lock time 0 and a
	// final sequence that pays 25 BCH or more is valid, and forbidden
	// after the seam.
	lockedCoinbase := func(cb *wire.MsgTx) {
		cb.LockTime = uint32(sync().Height + 1)
		cb.TxIn[0].Sequence = wire.MaxTxInSequenceNum - 1
	}
	legacyCoinbase := func(cb *wire.MsgTx) {
		cb.Version = 1
	}
	checkCoinbasesBefore := func() {
		t.Helper()
		submit(nextBlock(nil, 0, 0, lockedCoinbase), "bad-txns-nonfinal")
		propose(nextBlock(nil, 0, 0, legacyCoinbase), "")
	}
	checkCoinbasesAfter := func() {
		t.Helper()

		// The 5 BCH subsidy and a 20 BCH fee: exactly 25 BCH.
		const fee = 20 * bchutil.SatoshiPerBitcoin
		tx := spend(coinbaseCoin(5), fee, 2, wire.MaxTxInSequenceNum, 0)
		submit(nextBlock([]*wire.MsgTx{tx}, fee, 0, legacyCoinbase),
			"bad-cb-shape")
		propose(nextBlock(nil, 0, 0, lockedCoinbase), "")
	}

	activation := int32(-1)
	checkedBefore := int32(-1)
	for i := 0; activation < 0 || tip(bchn).Height < activation+4; i++ {
		switch prev := sync(); {
		case prev.NextTargetTime == 600:
			checkCoinbasesBefore()
			checkedBefore = prev.Height + 1
		case prev.Height == activation:
			checkCoinbasesAfter()
		}
		if i%2 == 0 {
			mineBCHD()
		} else {
			mineBCHN(1)
		}
		header := sync()
		if activation < 0 && header.NextTargetTime == 60 {
			activation = header.Height
		}
		if i > 20 {
			t.Fatalf("no seam after %d blocks", i)
		}
	}
	t.Logf("activation block %d", activation)
	if checkedBefore != activation {
		t.Fatalf("coinbases last checked before the seam in block %d, "+
			"want the activation block %d", checkedBefore, activation)
	}

	// Each node mined a block on each side of the seam.
	miners := make(map[string]bool)
	for height := activation - 1; height <= activation+2; height++ {
		cb := block(bchn, height).Transactions[0]
		side := "after"
		if height <= activation {
			side = "before"
		}
		if bytes.Equal(cb.TxOut[0].PkScript, pkScript) {
			miners["BCHN "+side] = true
		} else {
			miners["bchd "+side] = true
		}
	}
	if len(miners) != 4 {
		t.Fatalf("the seam blocks were mined by %v", miners)
	}

	// mempoolAccepts checks that both nodes give the same answer to
	// testmempoolaccept for the transaction.
	mempoolAccepts := func(tx *wire.MsgTx, want bool) {
		t.Helper()
		var buf bytes.Buffer
		if err := tx.Serialize(&buf); err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			var results []struct {
				Allowed bool   `json:"allowed"`
				Reason  string `json:"reject-reason"`
			}
			n.mustCall(t, &results, "testmempoolaccept",
				[]string{hex.EncodeToString(buf.Bytes())})
			if len(results) != 1 || results[0].Allowed != want {
				t.Fatalf("%s: testmempoolaccept of %v at height "+
					"%d gave %+v, want allowed %v", n.name,
					tx.TxHash(), tip(n).Height, results, want)
			}
		}
	}

	// releasedAt checks that a transaction is refused by both nodes, in
	// their memory pools and in a block, in the block before the one with
	// the passed ticks, and accepted in that block.  The tip is then the
	// block with the transaction.
	releasedAt := func(tx *wire.MsgTx, fee int64, ticks int64,
		reason string) {

		t.Helper()
		mineToTicks(ticks - 120)
		mempoolAccepts(tx, false)
		submit(nextBlock([]*wire.MsgTx{tx}, fee, 0, nil), reason)
		mineBCHN(1)
		sync()
		mempoolAccepts(tx, true)
		submit(nextBlock([]*wire.MsgTx{tx}, fee, 0, nil), "")
	}

	// checkFinalized checks that both nodes finalized the block at the
	// passed height, the shallowest block at least 6,000 ticks below the
	// tip.
	checkFinalized := func(height int32) {
		t.Helper()
		tip := sync()
		var hash, childHash string
		var header, child interopHeader
		bchn.mustCall(t, &hash, "getblockhash", height)
		bchn.mustCall(t, &header, "getblockheader", hash)
		bchn.mustCall(t, &childHash, "getblockhash", height+1)
		bchn.mustCall(t, &child, "getblockheader", childHash)
		if tip.Ticks-header.Ticks < 6000 || tip.Ticks-child.Ticks >= 6000 {
			t.Fatalf("block %d at tick %d is not the shallowest "+
				"6,000 ticks below the tip at tick %d", height,
				header.Ticks, tip.Ticks)
		}
		for _, n := range nodes {
			var fin string
			n.mustCall(t, &fin, "getfinalizedblockhash")
			if fin != hash {
				t.Fatalf("%s: finalized block %s, want %s at %d",
					n.name, fin, hash, height)
			}
		}
	}

	// ---------------------------------------------------------------------
	// Finalization 50 blocks after the seam, where the 6,000 ticks reach
	// five 600-tick blocks below the activation block.
	// ---------------------------------------------------------------------

	var activationHash string
	bchn.mustCall(t, &activationHash, "getblockhash", activation)
	var activationHeader interopHeader
	bchn.mustCall(t, &activationHeader, "getblockheader", activationHash)
	mineToTicks(activationHeader.Ticks + 50*60)
	checkFinalized(activation - 5)

	const fee = 1000

	// ---------------------------------------------------------------------
	// Coinbase maturity: the coinbase of the activation block, mined by
	// BCHN, may be spent 60,000 ticks after it, 1,000 blocks after the
	// seam, by a transaction that funds the relative lock test.
	// ---------------------------------------------------------------------

	matureCoin := coinbaseCoin(activation)
	if matureCoin.value != 50*bchutil.SatoshiPerBitcoin {
		t.Fatalf("activation block coinbase pays %d", matureCoin.value)
	}
	fund := spend(matureCoin, fee, 2, wire.MaxTxInSequenceNum, 0)
	releasedAt(fund, fee, matureCoin.ticks+60000,
		"bad-txns-premature-spend-of-coinbase")
	if got := tip(bchd).Height; got != activation+1000 {
		t.Fatalf("activation block coinbase spent at %d, want %d", got,
			activation+1000)
	}

	// ---------------------------------------------------------------------
	// Relative lock of one 600-tick unit, released 10 blocks later.
	// ---------------------------------------------------------------------

	fundTip := sync()
	csvCoin := coin{wire.OutPoint{Hash: fund.TxHash(), Index: 0},
		fund.TxOut[0].Value, fundTip.Ticks}
	csvTx := spend(csvCoin, fee, 2, 1, 0)
	releasedAt(csvTx, fee, csvCoin.ticks+600, "bad-txns-nonfinal")
	if got := tip(bchd).Height; got != fundTip.Height+10 {
		t.Fatalf("relative lock released at %d, want %d", got,
			fundTip.Height+10)
	}

	// ---------------------------------------------------------------------
	// Lock time: final once the block's ticks exceed 600 times it.
	// ---------------------------------------------------------------------

	lockTime := tipTicks()/600 + 2
	lockTx := spend(coinbaseCoin(1), fee, 2, wire.MaxTxInSequenceNum-1,
		uint32(lockTime))
	releasedAt(lockTx, fee, lockTime*600+60, "bad-txns-nonfinal")

	// ---------------------------------------------------------------------
	// Subsidy and coinbase shape after the seam.
	// ---------------------------------------------------------------------

	submit(nextBlock(nil, 0, 45*bchutil.SatoshiPerBitcoin, nil),
		"bad-cb-amount")

	// The coinbase pays the subsidy and a 46 BCH fee, which is the
	// forbidden shape unless it escapes it.
	const shapeFee = 46 * bchutil.SatoshiPerBitcoin
	shapeTx := spend(coinbaseCoin(2), shapeFee, 2, wire.MaxTxInSequenceNum,
		0)
	legacy := func(sequence uint32) func(*wire.MsgTx) {
		return func(cb *wire.MsgTx) {
			cb.Version = 1
			cb.TxIn[0].Sequence = sequence
		}
	}
	submit(nextBlock([]*wire.MsgTx{shapeTx}, shapeFee, 0,
		legacy(wire.MaxTxInSequenceNum)), "bad-cb-shape")
	submit(nextBlock([]*wire.MsgTx{shapeTx}, shapeFee, 0,
		legacy(wire.MaxTxInSequenceNum-1)), "")

	// A version 2 coinbase and a coinbase with a lock time escape it too.
	shapeTx = spend(coinbaseCoin(3), shapeFee, 2, wire.MaxTxInSequenceNum,
		0)
	submit(nextBlock([]*wire.MsgTx{shapeTx}, shapeFee, 0, nil), "")
	shapeTx = spend(coinbaseCoin(4), shapeFee, 2, wire.MaxTxInSequenceNum,
		0)
	lockedLegacy := func(cb *wire.MsgTx) {
		legacy(wire.MaxTxInSequenceNum)(cb)
		cb.LockTime = uint32(tip(bchn).Height)
	}
	submit(nextBlock([]*wire.MsgTx{shapeTx}, shapeFee, 0, lockedLegacy),
		"")

	// ---------------------------------------------------------------------
	// A coinbase is final after the seam whatever its lock time: the
	// height of its parent with a non-final sequence, which is far more
	// 600-tick units than the block's ticks, and a time far ahead.
	// ---------------------------------------------------------------------

	locked := func(lockTime uint32) func(*wire.MsgTx) {
		return func(cb *wire.MsgTx) {
			cb.LockTime = lockTime
			cb.TxIn[0].Sequence = wire.MaxTxInSequenceNum - 1
		}
	}
	parentHeight := tip(bchn).Height
	if blockTicks := tipTicks() + 60; int64(parentHeight)*600 < blockTicks {
		t.Fatalf("lock time %d is final at tick %d", parentHeight,
			blockTicks)
	}
	submit(nextBlock(nil, 0, 0, locked(uint32(parentHeight))), "")
	submit(nextBlock(nil, 0, 0, locked(0xfffffffe)), "")

	// ---------------------------------------------------------------------
	// Finalization of 60-tick blocks: 100 blocks below the tip.
	// ---------------------------------------------------------------------

	mineBCHN(1)
	final := sync()
	checkFinalized(final.Height - 100)

	// ---------------------------------------------------------------------
	// Both nodes report the same headers for every block, and the blocks
	// with only a coinbase pay the subsidy of their ticks: a tenth of the
	// 600-tick subsidy after the seam, halved from the 600-tick chunk
	// that ends at 90,000 ticks.
	// ---------------------------------------------------------------------

	for height := int32(1); height <= final.Height; height++ {
		var headers [2]interopHeader
		for i, n := range nodes {
			var hash string
			n.mustCall(t, &hash, "getblockhash", height)
			n.mustCall(t, &headers[i], "getblockheader", hash)
		}
		// bchd's getblockheader has no mediantime.
		headers[0].MedianTime = 0
		if headers[0] != headers[1] {
			t.Fatalf("block %d: BCHN %+v, bchd %+v", height,
				headers[0], headers[1])
		}
		wantInterval := int64(600)
		if height > activation {
			wantInterval = 60
		}
		if headers[0].TargetTime != wantInterval {
			t.Fatalf("block %d has target time %d, want %d", height,
				headers[0].TargetTime, wantInterval)
		}
		msg := block(bchd, height)
		if len(msg.Transactions) != 1 {
			continue
		}
		var value int64
		for _, out := range msg.Transactions[0].TxOut {
			value += out.Value
		}
		era := (headers[0].Ticks + 600 - headers[0].TargetTime) /
			(int64(params.SubsidyReductionInterval) * 600)
		want := (int64(50*bchutil.SatoshiPerBitcoin) >> era) *
			headers[0].TargetTime / 600
		if value != want {
			t.Fatalf("block %d coinbase pays %d, want %d", height,
				value, want)
		}
	}
}
