// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

// The template harness is a regtest-derived BlockChain backed by ffldb plus a
// fake TxSource, so NewBlockTemplate can be exercised end to end.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// fakeTxSource is a trivial TxSource returning a fixed set of descriptors.
type fakeTxSource struct {
	descs []*TxDesc
	have  map[chainhash.Hash]struct{}
}

func (s *fakeTxSource) LastUpdated() time.Time { return time.Now() }
func (s *fakeTxSource) MiningDescs() []*TxDesc { return s.descs }
func (s *fakeTxSource) HaveTransaction(h *chainhash.Hash) bool {
	_, ok := s.have[*h]
	return ok
}

func (s *fakeTxSource) add(tx *wire.MsgTx, fee int64) {
	if s.have == nil {
		s.have = make(map[chainhash.Hash]struct{})
	}
	s.have[tx.TxHash()] = struct{}{}
	size := int64(tx.SerializeSize())
	s.descs = append(s.descs, &TxDesc{
		Tx:       bchutil.NewTx(tx),
		Added:    time.Now(),
		Fee:      fee,
		FeePerKB: fee * 1000 / size,
	})
}

// templateParams returns a regtest copy with CSV, Magnetic Anomaly (CTOR),
// Phonon and Upgrade9 active from the start so the template rules match
// today's mainnet rule set.
func templateParams() chaincfg.Params {
	p := chaincfg.RegressionNetParams
	p.CSVHeight = 1
	p.MagneticAnonomalyForkHeight = 0
	p.PhononForkHeight = 0
	p.Upgrade9ForkHeight = 0
	return p
}

type templateHarness struct {
	t       *testing.T
	params  *chaincfg.Params
	chain   *blockchain.BlockChain
	src     *fakeTxSource
	gen     *BlkTmplGenerator
	nextTS  time.Time
	cbTxs   []*wire.MsgTx // coinbase of block i at index i-1
	hashCch *txscript.HashCache
}

var bigPolicy = Policy{BlockMaxSize: 31999000}

func newTemplateHarness(t *testing.T, params chaincfg.Params, excessive uint32, policy Policy) *templateHarness {
	t.Helper()
	db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"),
		wire.TestNet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	p := params
	ts := blockchain.NewMedianTime()
	sigCache := txscript.NewSigCache(1000)
	hashCache := txscript.NewHashCache(1000)
	chain, err := blockchain.New(&blockchain.Config{
		DB:                 db,
		ChainParams:        &p,
		TimeSource:         ts,
		SigCache:           sigCache,
		HashCache:          hashCache,
		UtxoCacheMaxSize:   16 * 1024 * 1024,
		ExcessiveBlockSize: excessive,
	})
	if err != nil {
		t.Fatal(err)
	}
	src := &fakeTxSource{}
	pol := policy
	h := &templateHarness{
		t: t, params: &p, chain: chain, src: src, hashCch: hashCache,
		gen: NewBlkTmplGenerator(&pol, &p, src, chain, ts, sigCache, hashCache),
	}
	return h
}

// mine builds a template from the current source, stamps it with a
// deterministic past timestamp (600s spacing ending shortly before now), solves
// it and connects it.  The source is cleared afterwards.
func (h *templateHarness) mine() *bchutil.Block {
	h.t.Helper()
	tmpl, err := h.gen.NewBlockTemplate(nil)
	if err != nil {
		h.t.Fatalf("NewBlockTemplate: %v", err)
	}
	msg := tmpl.Block
	msg.Header.Timestamp = h.nextTS
	h.nextTS = h.nextTS.Add(600 * time.Second)
	bits, err := h.chain.CalcNextRequiredDifficulty(msg.Header.Timestamp)
	if err != nil {
		h.t.Fatal(err)
	}
	msg.Header.Bits = bits
	target := blockchain.CompactToBig(bits)
	for n := uint32(0); ; n++ {
		msg.Header.Nonce = n
		hash := msg.Header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			break
		}
	}
	blk := bchutil.NewBlock(msg)
	_, isOrphan, err := h.chain.ProcessBlock(blk, blockchain.BFNone)
	if err != nil || isOrphan {
		h.t.Fatalf("ProcessBlock: orphan=%v err=%v", isOrphan, err)
	}
	h.cbTxs = append(h.cbTxs, msg.Transactions[0])
	h.src.descs, h.src.have = nil, nil
	return blk
}

// mineN mines n blocks with timestamps spaced 600s apart, the last one
// landing ~20 minutes before the wall clock.
func (h *templateHarness) mineN(n int) {
	h.nextTS = time.Now().Add(-time.Duration(n+2) * 600 * time.Second).Truncate(time.Second)
	for range n {
		h.mine()
	}
}

// spend builds a transaction spending output 0 of prev (an OP_TRUE output),
// paying value-fee back to OP_TRUE, with an OP_RETURN pad so the transaction
// clears the 65-byte minimum.
func spend(prev *wire.MsgTx, fee int64) *wire.MsgTx {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prev.TxHash(), Index: 0},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: prev.TxOut[0].Value - fee, PkScript: []byte{txscript.OP_TRUE}})
	pad, _ := txscript.NullDataScript(make([]byte, 20))
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: pad})
	return tx
}
