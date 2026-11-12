// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build rpctest

package integration

import (
	"net"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/integration/rpctest"
	"github.com/gcash/bchd/peer"
	"github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil/gcs/builder"
)

// cfReplyTimeout is how long a getcfrange or getcfproof that the node can
// answer is given, and how long the handshake and the final ping are given.
const cfReplyTimeout = 10 * time.Second

// cfNoReplyTimeout is how long a request the node must ignore is given
// before the test treats silence as success.
const cfNoReplyTimeout = 2 * time.Second

// cfP2PKHScript builds a P2PKH script whose 20-byte hash is the same byte
// repeated, so each tag is a different script.
func cfP2PKHScript(tag byte) []byte {
	script := make([]byte, 25)
	script[0] = txscript.OP_DUP
	script[1] = txscript.OP_HASH160
	script[2] = txscript.OP_DATA_20
	for i := 3; i < 23; i++ {
		script[i] = tag
	}
	script[23] = txscript.OP_EQUALVERIFY
	script[24] = txscript.OP_CHECKSIG
	return script
}

// cfMaskedPrefix keeps the first bits of a key and clears the rest. A
// getcfrange prefix is encoded that way: the bits past its length are zero.
func cfMaskedPrefix(key [32]byte, bits uint16) [32]byte {
	var prefix [32]byte
	full := int(bits / 8)
	copy(prefix[:full], key[:full])
	if rem := bits % 8; rem != 0 {
		prefix[full] = key[full] & (byte(0xff) << (8 - rem))
	}
	return prefix
}

// cfSpentScripts returns the locking script of every output the block spends,
// one for each input of each transaction after the coinbase, in that order.
// Simnet has no token outputs, so the script is the output's PkScript.
// A token output would need its token prefix in front, which is the form
// scriptFilterLeaves passes from the spend journal.
func cfSpentScripts(t *testing.T, node *rpcclient.Client, block *wire.MsgBlock) [][]byte {
	t.Helper()
	var scripts [][]byte
	for ti, tx := range block.Transactions {
		if ti == 0 {
			continue
		}
		for _, in := range tx.TxIn {
			prev, err := node.GetRawTransaction(&in.PreviousOutPoint.Hash)
			if err != nil {
				t.Fatalf("spent output %v: %v", in.PreviousOutPoint, err)
			}
			outs := prev.MsgTx().TxOut
			idx := in.PreviousOutPoint.Index
			if int(idx) >= len(outs) {
				t.Fatalf("spent output %v index %d with %d outputs",
					in.PreviousOutPoint, idx, len(outs))
			}
			scripts = append(scripts, outs[idx].PkScript)
		}
	}
	return scripts
}

// cfParseLeaves parses the leaves of a cfrange into the builder's type.
func cfParseLeaves(raw []wire.CFLeaf) ([]builder.ScriptLeaf, error) {
	leaves := make([]builder.ScriptLeaf, len(raw))
	for i := range raw {
		leaf, err := builder.ParseScriptLeaf(raw[i][:])
		if err != nil {
			return nil, err
		}
		leaves[i] = leaf
	}
	return leaves, nil
}

// cfSameLeaves reports whether two runs hold the same leaves in the same order.
func cfSameLeaves(a, b []builder.ScriptLeaf) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cfParseNeighbour parses the leaf just before or just after a run. The
// message leaves that neighbour out by encoding it as nil.
func cfParseNeighbour(raw *wire.CFLeaf) (*builder.ScriptLeaf, error) {
	if raw == nil {
		return nil, nil
	}
	leaf, err := builder.ParseScriptLeaf(raw[:])
	if err != nil {
		return nil, err
	}
	return &leaf, nil
}

// cfProofConn is a raw peer connected to the harness. Its channels receive
// the replies the listeners are handed. The channels are buffered so a
// listener, which runs on the peer's read goroutine, can return without
// waiting for the test to read.
type cfProofConn struct {
	t      *testing.T
	peer   *peer.Peer
	block  chainhash.Hash
	ranges chan *wire.MsgCFRange
	proofs chan *wire.MsgCFProof
	pongs  chan *wire.MsgPong
	verack chan struct{}
}

// dialCFProofPeer connects a raw peer to the harness and waits until the
// node has sent its verack. AssociateConnection negotiates in the background,
// and a handshake that never finishes fails the test here, at setup.
func dialCFProofPeer(t *testing.T, address string, block chainhash.Hash) *cfProofConn {
	t.Helper()
	c := &cfProofConn{
		t:      t,
		block:  block,
		ranges: make(chan *wire.MsgCFRange, 4),
		proofs: make(chan *wire.MsgCFProof, 4),
		pongs:  make(chan *wire.MsgPong, 4),
		verack: make(chan struct{}, 1),
	}
	p, err := peer.NewOutboundPeer(&peer.Config{
		UserAgentName:    "relay",
		UserAgentVersion: "1.0.0",
		ChainParams:      &chaincfg.SimNetParams,
		Listeners: peer.MessageListeners{
			OnVerAck: func(*peer.Peer, *wire.MsgVerAck) {
				select {
				case c.verack <- struct{}{}:
				default:
				}
			},
			OnCFRange: func(_ *peer.Peer, msg *wire.MsgCFRange) {
				c.ranges <- msg
			},
			OnCFProof: func(_ *peer.Peer, msg *wire.MsgCFProof) {
				c.proofs <- msg
			},
			OnPong: func(_ *peer.Peer, msg *wire.MsgPong) {
				c.pongs <- msg
			},
		},
	}, address)
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	netConn, err := net.Dial("tcp", p.Addr())
	if err != nil {
		t.Fatalf("dial %s: %v", p.Addr(), err)
	}
	p.AssociateConnection(netConn)
	c.peer = p
	select {
	case <-c.verack:
	case <-time.After(cfReplyTimeout):
		p.Disconnect()
		t.Fatal("the peer did not finish the handshake")
	}
	return c
}

// askRange sends getcfrange for the block's script filter and returns the
// cfrange. ok is false when nothing arrives within 10 seconds.
func (c *cfProofConn) askRange(bits uint16, prefix [32]byte) (*wire.MsgCFRange, bool) {
	c.peer.QueueMessage(wire.NewMsgGetCFRange(wire.GCSFilterScripts, &c.block, bits, prefix), nil)
	select {
	case msg := <-c.ranges:
		return msg, true
	case <-time.After(cfReplyTimeout):
		return nil, false
	}
}

// askProof sends getcfproof for one leaf of the block's script filter and
// returns the cfproof. ok is false when nothing arrives within 10 seconds.
func (c *cfProofConn) askProof(index uint32) (*wire.MsgCFProof, bool) {
	c.peer.QueueMessage(wire.NewMsgGetCFProof(wire.GCSFilterScripts, &c.block, index), nil)
	select {
	case msg := <-c.proofs:
		return msg, true
	case <-time.After(cfReplyTimeout):
		return nil, false
	}
}

// expectNoRange sends a getcfrange the node has to ignore and fails the test
// when a cfrange arrives within 2 seconds.
func (c *cfProofConn) expectNoRange(name string, msg *wire.MsgGetCFRange) {
	c.t.Helper()
	c.peer.QueueMessage(msg, nil)
	select {
	case <-c.ranges:
		c.t.Errorf("getcfrange %s was answered", name)
	case <-time.After(cfNoReplyTimeout):
	}
}

// expectNoProof sends a getcfproof the node has to ignore and fails the test
// when a cfproof arrives within 2 seconds.
func (c *cfProofConn) expectNoProof(name string, msg *wire.MsgGetCFProof) {
	c.t.Helper()
	c.peer.QueueMessage(msg, nil)
	select {
	case <-c.proofs:
		c.t.Errorf("getcfproof %s was answered", name)
	case <-time.After(cfNoReplyTimeout):
	}
}

// expectPong sends a ping and fails the test when the pong with that nonce
// does not come back within 10 seconds. The nonce check is how the test
// tells that the peer is still the one it shook hands with.
func (c *cfProofConn) expectPong() {
	c.t.Helper()
	const nonce = 42
	c.peer.QueueMessage(wire.NewMsgPing(nonce), nil)
	select {
	case pong := <-c.pongs:
		if pong.Nonce != nonce {
			c.t.Errorf("pong nonce %d, want %d", pong.Nonce, nonce)
		}
	case <-time.After(cfReplyTimeout):
		c.t.Errorf("no pong within %s", cfReplyTimeout)
	}
}

// expectBanned sends n getcfrange and n getcfproof messages without waiting
// for the answers, and fails the test unless the node then disconnects the
// peer within 10 seconds.
func (c *cfProofConn) expectBanned(n int) {
	c.t.Helper()
	// The answers are read and dropped, so that the peer keeps reading and
	// sees the connection close.
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-c.ranges:
			case <-c.proofs:
			case <-done:
				return
			}
		}
	}()
	for i := 0; i < n; i++ {
		c.peer.QueueMessage(wire.NewMsgGetCFRange(wire.GCSFilterScripts, &c.block, 0, [32]byte{}), nil)
		c.peer.QueueMessage(wire.NewMsgGetCFProof(wire.GCSFilterScripts, &c.block, 0), nil)
	}
	gone := make(chan struct{})
	go func() {
		c.peer.WaitForDisconnect()
		close(gone)
	}()
	select {
	case <-gone:
	case <-time.After(cfReplyTimeout):
		c.t.Errorf("the node kept a peer that sent %d getcfrange and %d getcfproof", n, n)
	}
}

// checkCFRange checks one cfrange against the reference leaves of the block.
func checkCFRange(t *testing.T, name string, msg *wire.MsgCFRange, blockHash *chainhash.Hash,
	leaves []builder.ScriptLeaf, root chainhash.Hash, prefix *[32]byte, bits uint16) {

	t.Helper()
	if msg.FilterType != wire.GCSFilterScripts {
		t.Errorf("cfrange %s filter type %v, want the script filter", name, msg.FilterType)
	}
	if msg.BlockHash != *blockHash {
		t.Errorf("cfrange %s block %v, want %v", name, msg.BlockHash, blockHash)
	}
	if msg.LeafCount != uint32(len(leaves)) {
		t.Errorf("cfrange %s leaf count %d, want %d", name, msg.LeafCount, len(leaves))
	}
	run, err := cfParseLeaves(msg.Leaves)
	if err != nil {
		t.Errorf("cfrange %s leaves: %v", name, err)
		return
	}
	before, err := cfParseNeighbour(msg.Before)
	if err != nil {
		t.Errorf("cfrange %s leaf before the run: %v", name, err)
		return
	}
	after, err := cfParseNeighbour(msg.After)
	if err != nil {
		t.Errorf("cfrange %s leaf after the run: %v", name, err)
		return
	}
	first, end, err := builder.ScriptLeavesWithPrefix(leaves, prefix, bits)
	if err != nil {
		t.Errorf("cfrange %s reference run: %v", name, err)
		return
	}
	if msg.FirstIndex != uint32(first) {
		t.Errorf("cfrange %s first index %d, want %d", name, msg.FirstIndex, first)
	}
	if !cfSameLeaves(run, leaves[first:end]) {
		t.Errorf("cfrange %s leaves are not the reference run [%d:%d]", name, first, end)
	}
	switch {
	case first > 0 && before == nil:
		t.Errorf("cfrange %s has no leaf before index %d", name, first)
	case first > 0 && *before != leaves[first-1]:
		t.Errorf("cfrange %s leaf before index %d differs from the reference", name, first)
	case first == 0 && before != nil:
		t.Errorf("cfrange %s has a leaf before a run that starts at 0", name)
	}
	switch {
	case end < len(leaves) && after == nil:
		t.Errorf("cfrange %s has no leaf after index %d", name, end)
	case end < len(leaves) && *after != leaves[end]:
		t.Errorf("cfrange %s leaf after the run differs from the reference", name)
	case end == len(leaves) && after != nil:
		t.Errorf("cfrange %s has a leaf after a run that ends at the last leaf", name)
	}
	err = builder.CheckScriptRange(root, msg.LeafCount, msg.FirstIndex, run, before, after,
		msg.Hashes, prefix, bits)
	if err != nil {
		t.Errorf("cfrange %s does not rebuild the filter root: %v", name, err)
	}
}

// checkCFProof checks one cfproof against one reference leaf: the leaf and
// its branch, the transaction it names in the block, and, for a spent
// output, the transaction of that output in its own block.
func checkCFProof(t *testing.T, node *rpcclient.Client, msg *wire.MsgCFProof,
	block *wire.MsgBlock, leaves []builder.ScriptLeaf, root chainhash.Hash, index int) {

	t.Helper()
	if msg.FilterType != wire.GCSFilterScripts {
		t.Errorf("cfproof %d filter type %v, want the script filter", index, msg.FilterType)
	}
	blockHash := block.BlockHash()
	if msg.BlockHash != blockHash {
		t.Errorf("cfproof %d block %v, want %v", index, msg.BlockHash, blockHash)
	}
	if msg.LeafCount != uint32(len(leaves)) {
		t.Errorf("cfproof %d leaf count %d, want %d", index, msg.LeafCount, len(leaves))
	}
	if msg.LeafIndex != uint32(index) {
		t.Errorf("cfproof leaf index %d, want %d", msg.LeafIndex, index)
	}
	leaf, err := builder.ParseScriptLeaf(msg.Leaf[:])
	if err != nil {
		t.Errorf("cfproof %d leaf: %v", index, err)
		return
	}
	if leaf != leaves[index] {
		t.Errorf("cfproof %d leaf differs from the reference", index)
	}
	rebuilt, err := builder.ScriptRangeRoot(msg.LeafCount, uint32(index),
		[]builder.ScriptLeaf{leaf}, msg.LeafBranch)
	if err != nil {
		t.Errorf("cfproof %d branch: %v", index, err)
	} else if rebuilt != root {
		t.Errorf("cfproof %d rebuilds %v, want the filter root %v", index, rebuilt, root)
	}
	if msg.Tx == nil {
		t.Errorf("cfproof %d has no transaction", index)
		return
	}
	if int(leaf.TxIndex) >= len(block.Transactions) {
		t.Errorf("cfproof %d names transaction %d of a block of %d",
			index, leaf.TxIndex, len(block.Transactions))
		return
	}
	txHash := block.Transactions[leaf.TxIndex].TxHash()
	if msg.Tx.TxHash() != txHash {
		t.Errorf("cfproof %d transaction %v, block transaction %d is %v",
			index, msg.Tx.TxHash(), leaf.TxIndex, txHash)
	}
	gotRoot := blockchain.MerkleBranchRoot(txHash, leaf.TxIndex, msg.TxBranch)
	if gotRoot != block.Header.MerkleRoot {
		t.Errorf("cfproof %d transaction branch leads to %v, block merkle root is %v",
			index, gotRoot, block.Header.MerkleRoot)
	}

	switch leaves[index].Kind {
	case builder.ScriptLeafOutput:
		checkCFProofOutput(t, msg, leaf, index)
	case builder.ScriptLeafSpent:
		checkCFProofSpent(t, node, msg, leaf, index)
	default:
		t.Errorf("cfproof %d has kind %d", index, leaves[index].Kind)
	}
}

// checkCFProofOutput checks a leaf of an output the block creates: the key
// is the script of that output, and the proof carries no spent transaction.
func checkCFProofOutput(t *testing.T, msg *wire.MsgCFProof, leaf builder.ScriptLeaf, index int) {
	t.Helper()
	if msg.PrevTx != nil {
		t.Errorf("cfproof %d is an output the block creates and carries a spent transaction", index)
	}
	if int(leaf.Index) >= len(msg.Tx.TxOut) {
		t.Errorf("cfproof %d output %d of a transaction with %d outputs",
			index, leaf.Index, len(msg.Tx.TxOut))
		return
	}
	if builder.ScriptKey(msg.Tx.TxOut[leaf.Index].PkScript) != leaf.Key {
		t.Errorf("cfproof %d output script hashes to a different key", index)
	}
}

// checkCFProofSpent checks a leaf of an output the block spends. The spent
// transaction has to be the one the input names, its branch has to lead to
// the merkle root of the header the proof names, and its script has to be
// the leaf's key.
func checkCFProofSpent(t *testing.T, node *rpcclient.Client, msg *wire.MsgCFProof,
	leaf builder.ScriptLeaf, index int) {

	t.Helper()
	if msg.PrevTx == nil {
		t.Errorf("cfproof %d is an output the block spends and has no spent transaction", index)
		return
	}
	if int(leaf.Index) >= len(msg.Tx.TxIn) {
		t.Errorf("cfproof %d input %d of a transaction with %d inputs",
			index, leaf.Index, len(msg.Tx.TxIn))
		return
	}
	prevOut := msg.Tx.TxIn[leaf.Index].PreviousOutPoint
	if msg.PrevTx.TxHash() != prevOut.Hash {
		t.Errorf("cfproof %d spent transaction %v, the input spends %v",
			index, msg.PrevTx.TxHash(), prevOut.Hash)
	}
	header, err := node.GetBlockHeader(&msg.PrevBlockHash)
	if err != nil {
		t.Errorf("cfproof %d block %v of the spent transaction: %v",
			index, msg.PrevBlockHash, err)
	} else {
		gotRoot := blockchain.MerkleBranchRoot(prevOut.Hash, msg.PrevTxIndex, msg.PrevTxBranch)
		if header.MerkleRoot != gotRoot {
			t.Errorf("cfproof %d spent transaction branch leads to %v, header merkle root is %v",
				index, gotRoot, header.MerkleRoot)
		}
	}
	if int(prevOut.Index) >= len(msg.PrevTx.TxOut) {
		t.Errorf("cfproof %d spends output %d of a transaction with %d outputs",
			index, prevOut.Index, len(msg.PrevTx.TxOut))
		return
	}
	if builder.ScriptKey(msg.PrevTx.TxOut[prevOut.Index].PkScript) != leaf.Key {
		t.Errorf("cfproof %d spent script hashes to a different key", index)
	}
}

// TestCFRangeAndCFProof mines a block that pays six distinct P2PKH scripts
// and spends coins the harness wallet already has, then checks the script
// filter entry tree the node serves for that block. The test derives the
// leaves and their root from the block itself. A raw peer checks getcfrange
// and getcfproof against those leaves, and checks that the requests the node
// cannot answer are ignored while the peer stays connected.
func TestCFRangeAndCFProof(t *testing.T) {
	h, err := rpctest.New(&chaincfg.SimNetParams, nil, []string{"--txindex"})
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	defer func() { _ = h.TearDown() }()
	if err = h.SetUp(true, 25); err != nil {
		t.Fatalf("set up harness: %v", err)
	}

	// Six outputs, each a different P2PKH script, plus change. The change
	// script and the coinbase are further leaves, and the inputs are the
	// spent ones.
	outputs := make([]*wire.TxOut, 0, 6)
	var third []byte
	for tag := byte(1); tag <= 6; tag++ {
		script := cfP2PKHScript(tag)
		if tag == 3 {
			third = script
		}
		outputs = append(outputs, wire.NewTxOut(10000, script, wire.TokenData{}))
	}
	// 10 satoshis per byte is the fee the other harness payments use.
	tx, err := h.CreateTransaction(outputs, 10, true)
	if err != nil {
		t.Fatalf("create the payment: %v", err)
	}
	if _, err = h.Node.SendRawTransaction(tx, true); err != nil {
		t.Fatalf("send the payment: %v", err)
	}
	mined, err := h.Node.Generate(1)
	if err != nil {
		t.Fatalf("mine the block: %v", err)
	}
	if len(mined) != 1 {
		t.Fatalf("mining the block returned %d hashes", len(mined))
	}
	blockHash := mined[0]
	block, err := h.Node.GetBlock(blockHash)
	if err != nil {
		t.Fatalf("get the block: %v", err)
	}
	if got := block.BlockHash(); got != *blockHash {
		t.Fatalf("getblock returned %v, the miner returned %v", got, blockHash)
	}
	payment := tx.TxHash()
	inBlock := false
	for _, mtx := range block.Transactions {
		if mtx.TxHash() == payment {
			inBlock = true
			break
		}
	}
	if !inBlock {
		t.Fatalf("block %v does not contain the payment %v", blockHash, payment)
	}

	spent := cfSpentScripts(t, h.Node, block)
	leaves, err := builder.ScriptLeaves(block, spent)
	if err != nil {
		t.Fatalf("reference leaves: %v", err)
	}
	root := builder.ScriptLeavesRoot(leaves)
	served, err := h.Node.GetCFilter(blockHash, wire.GCSFilterScripts)
	if err != nil {
		t.Fatalf("script filter of %v: %v", blockHash, err)
	}
	_, servedRoot, err := builder.ParseScriptFilter(served.Data)
	if err != nil {
		t.Fatalf("parse the script filter of %v: %v", blockHash, err)
	}
	if servedRoot != root {
		t.Fatalf("script filter root %v, reference leaves root %v", servedRoot, root)
	}

	conn := dialCFProofPeer(t, h.P2PAddress(), *blockHash)
	defer conn.peer.Disconnect()

	thirdKey := builder.ScriptKey(third)
	ranges := []struct {
		name   string
		bits   uint16
		prefix [32]byte
	}{
		{name: "exact key of the third P2PKH script", bits: 256, prefix: thirdKey},
		{name: "exact key of a script the block does not have", bits: 256,
			prefix: builder.ScriptKey([]byte("no such script"))},
		{name: "first 4 bits of the third P2PKH key", bits: 4,
			prefix: cfMaskedPrefix(thirdKey, 4)},
		{name: "every leaf", bits: 0, prefix: [32]byte{}},
	}
	for _, tc := range ranges {
		msg, ok := conn.askRange(tc.bits, tc.prefix)
		if !ok {
			t.Errorf("no cfrange for %s within %s", tc.name, cfReplyTimeout)
			continue
		}
		checkCFRange(t, tc.name, msg, blockHash, leaves, root, &tc.prefix, tc.bits)
	}

	sawOutput := false
	sawSpent := false
	for i := range leaves {
		switch leaves[i].Kind {
		case builder.ScriptLeafOutput:
			sawOutput = true
		case builder.ScriptLeafSpent:
			sawSpent = true
		}
		msg, ok := conn.askProof(uint32(i))
		if !ok {
			t.Errorf("no cfproof for leaf %d within %s", i, cfReplyTimeout)
			continue
		}
		checkCFProof(t, h.Node, msg, block, leaves, root, i)
	}
	if !sawOutput {
		t.Errorf("no leaf of an output the block creates")
	}
	if !sawSpent {
		t.Errorf("no leaf of an output the block spends")
	}

	conn.expectNoRange("regular filter",
		wire.NewMsgGetCFRange(wire.GCSFilterRegular, blockHash, 0, [32]byte{}))
	badPrefix := cfMaskedPrefix(thirdKey, 4)
	badPrefix[0] |= 0x01
	conn.expectNoRange("4 bits with a bit set past them",
		wire.NewMsgGetCFRange(wire.GCSFilterScripts, blockHash, 4, badPrefix))
	missing := chainhash.Hash{0xff}
	conn.expectNoRange("unknown block",
		wire.NewMsgGetCFRange(wire.GCSFilterScripts, &missing, 0, [32]byte{}))
	conn.expectNoProof("leaf past the tree",
		wire.NewMsgGetCFProof(wire.GCSFilterScripts, blockHash, uint32(len(leaves))))
	conn.expectNoProof("regular filter",
		wire.NewMsgGetCFProof(wire.GCSFilterRegular, blockHash, 0))
	conn.expectPong()

	// Each request adds 1 to the ban score the node keeps for a peer, and
	// it bans at 100. Settling a disagreement takes about one proof for
	// each level of the tree, 30 for a billion leaves, so a peer that
	// asks for 60 in a row is still served.
	quiet := dialCFProofPeer(t, h.P2PAddress(), *blockHash)
	defer quiet.peer.Disconnect()
	for i := 0; i < 60; i++ {
		if _, ok := quiet.askProof(0); !ok {
			t.Fatalf("no cfproof for request %d of 60 in a row", i+1)
		}
	}
	quiet.expectPong()

	// A peer that sends 60 of each is banned, which 60 of one kind alone
	// would not do. The ban is on the address, so this comes last.
	flood := dialCFProofPeer(t, h.P2PAddress(), *blockHash)
	defer flood.peer.Disconnect()
	flood.expectBanned(60)
}
