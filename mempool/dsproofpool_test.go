// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// dsRecorder records the callbacks of a memory pool.
type dsRecorder struct {
	sync.Mutex
	proofs []dsProofAnnouncement
	bad    []Tag
}

func (r *dsRecorder) take() ([]dsProofAnnouncement, []Tag) {
	r.Lock()
	defer r.Unlock()
	p, b := r.proofs, r.bad
	r.proofs, r.bad = nil, nil
	return p, b
}

// newDSProofHarness returns a pool harness whose double spend proof callbacks
// are recorded.
func newDSProofHarness(t *testing.T, params *chaincfg.Params) (*poolHarness, *dsRecorder) {
	t.Helper()
	harness, _, err := newPoolHarness(params)
	if err != nil {
		t.Fatal(err)
	}
	rec := &dsRecorder{}
	harness.txPool.cfg.OnDSProof = func(proof *wire.MsgDSProof, tx *bchutil.Tx) {
		rec.Lock()
		rec.proofs = append(rec.proofs, dsProofAnnouncement{proof, tx})
		rec.Unlock()
	}
	harness.txPool.cfg.OnBadDSProofs = func(peers []Tag) {
		rec.Lock()
		rec.bad = append(rec.bad, peers...)
		rec.Unlock()
	}
	return harness, rec
}

// TestDSProofCashTokens checks proofs for a coin carrying tokens, whose
// signatures commit to the token data.
func TestDSProofCashTokens(t *testing.T) {
	params := chaincfg.MainNetParams
	params.Upgrade9ForkHeight = 0
	harness, rec := newDSProofHarness(t, &params)
	coin := &wire.TxOut{Value: 100000000, PkScript: harness.payScript, TokenData: wire.TokenData{
		CategoryID: chainhash.Hash{0xaa},
		Amount:     1000,
		BitField:   0x10,
	}}
	op := addCoins(harness, coin)[0]
	mk := func(fee int64) *bchutil.Tx {
		tx := wire.NewMsgTx(2)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op, Sequence: wire.MaxTxInSequenceNum})
		tx.AddTxOut(&wire.TxOut{Value: coin.Value - fee, PkScript: harness.payScript, TokenData: coin.TokenData})
		signP2PKH(t, tx, 0, coin, harness.signKey, txscript.SigHashAll, true)
		return bchutil.NewTx(tx)
	}
	tx1, tx2 := mk(1000), mk(2000)
	harness.txPool.cfg.Policy.MaxTxVersion = 2

	if _, err := harness.txPool.ProcessTransaction(tx1, false, false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := harness.txPool.ProcessTransaction(tx2, false, false, 0); err == nil {
		t.Fatal("double spend accepted")
	}
	proofs, _ := rec.take()
	if len(proofs) != 1 || *proofs[0].tx.Hash() != *tx1.Hash() {
		t.Fatalf("got %d proofs", len(proofs))
	}
	proof := proofs[0].proof
	hash := proof.ProofHash()
	if v, err := validateDSProof(proof, &hash, coin, tx1.MsgTx(), testFlags); v != dsProofValid {
		t.Fatalf("%v %v", v, err)
	}
	// Without the token data in the signature hash the signatures fail.
	noTokens := testFlags &^ txscript.ScriptAllowCashTokens
	if v, _ := validateDSProof(proof, &hash, coin, tx1.MsgTx(), noTokens); v != dsProofInvalid {
		t.Fatalf("validated without the token data: %v", v)
	}
}

// bchnDupeProofTx is the transaction Bitcoin Cash Node's dsproof_tests.cpp
// builds its storage test proofs from.
const bchnDupeProofTx = "0100000001f1b76b251770f5d26334c41327ef54d52cba86f77f67e5fce35611d4dad729270000000" +
	"06441c70853c2bb31d8df457613cfcae7755bf1e1c558271804e2a82f86558c182cec731014ebdb70" +
	"9da6e642ed89042dbbd6faed1853ee6299393e46bb656a4c8dae4121035303d906d781995ba837f73" +
	"757e336446bbbc49e377cb95e98d86a64c6878898feffffff01bd4397964e0000001976a9140a373c" +
	"af0ab3c2b46cd05625b8d545c295b93d7a88acb4781500"

// makeDupeProofs builds num proofs for the same outpoint, each against a copy
// of the transaction paying one more satoshi less, without checking the
// signatures.  A non-zero fuzz changes the outpoint.
func makeDupeProofs(t *testing.T, num int, fuzz uint64) []*wire.MsgDSProof {
	t.Helper()
	raw, _ := hex.DecodeString(bchnDupeProofTx)
	var tx1 wire.MsgTx
	if err := tx1.Deserialize(bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	if fuzz != 0 {
		for i := range 8 {
			tx1.TxIn[0].PreviousOutPoint.Hash[i] ^= byte(fuzz >> (8 * i))
		}
	}
	op := tx1.TxIn[0].PreviousOutPoint
	proofs := make([]*wire.MsgDSProof, 0, num)
	for i := range num {
		tx2 := tx1.Copy()
		tx2.TxOut[0].Value -= int64(i + 1)
		spenders := make([]wire.DSProofSpender, 2)
		for j, tx := range []*wire.MsgTx{&tx1, tx2} {
			pushes, err := txscript.PushedData(tx.TxIn[0].SignatureScript)
			if err != nil {
				t.Fatal(err)
			}
			spenders[j] = wire.DSProofSpender{
				Version: uint32(tx.Version), Sequence: tx.TxIn[0].Sequence,
				LockTime: tx.LockTime, PushData: [][]byte{pushes[0]},
			}
			hashDSProofSpender(&spenders[j], tx, 0, txscript.NewTxSigHashes(tx))
		}
		if compareDSProofSpenders(&spenders[0], &spenders[1]) > 0 {
			spenders[0], spenders[1] = spenders[1], spenders[0]
		}
		proof := wire.NewMsgDSProof(op, spenders[0], spenders[1])
		hash := proof.ProofHash()
		if err := checkDSProofSanity(proof, &hash, testFlags); err != nil {
			t.Fatal(err)
		}
		proofs = append(proofs, proof)
	}
	return proofs
}

// TestDSProofStorageSameOutPoint checks that many proofs for one outpoint
// get distinct ids and are found by outpoint, up to the per outpoint limit,
// beyond which proofs from peers holding one each are refused and the first
// seen are kept.
func TestDSProofStorageSameOutPoint(t *testing.T) {
	s := newDSProofStorage()
	proofs := makeDupeProofs(t, 100, 0)
	ids := make(map[chainhash.Hash]struct{})
	for i, p := range proofs {
		h := p.ProofHash()
		ids[h] = struct{}{}
		if s.addOrphan(p, h, Tag(i+1), true) != (i < maxDSProofOrphansPerOutPoint) {
			t.Fatalf("orphan %d stored %v", i, i >= maxDSProofOrphansPerOutPoint)
		}
	}
	if len(ids) != 100 {
		t.Fatalf("%d unique ids", len(ids))
	}
	found := s.findOrphans(proofs[0].PrevOut)
	if len(found) != maxDSProofOrphansPerOutPoint || s.numOrphans != len(found) {
		t.Fatalf("found %d orphans, %d counted", len(found), s.numOrphans)
	}
	for i, e := range found {
		if e.hash != proofs[i].ProofHash() {
			t.Fatalf("orphan %d is not one of the first seen", i)
		}
	}
}

// TestDSProofStorageOutPointLimits checks that one peer cannot hold more
// than its share of an outpoint's orphans, that a full outpoint makes room
// only by dropping an orphan of a peer holding more than one, never the
// oldest orphan of a peer holding one, and that orphans without a peer are
// never dropped to make room.
func TestDSProofStorageOutPointLimits(t *testing.T) {
	s := newDSProofStorage()
	proofs := makeDupeProofs(t, 2*maxDSProofOrphansPerOutPoint, 0)

	// The oldest orphan comes from a peer holding one.
	if !s.addOrphan(proofs[0], proofs[0].ProofHash(), 50, true) {
		t.Fatal("first orphan dropped")
	}
	for i := 1; i <= maxDSProofOrphansPerPeerOutPoint; i++ {
		if !s.addOrphan(proofs[i], proofs[i].ProofHash(), 1, true) {
			t.Fatalf("orphan %d of peer 1 dropped", i)
		}
	}
	next := maxDSProofOrphansPerPeerOutPoint + 1
	if s.addOrphan(proofs[next], proofs[next].ProofHash(), 1, true) {
		t.Fatal("peer exceeded its share of the outpoint")
	}
	// A known proof is not new, even from a peer at its share.
	if !s.addOrphan(proofs[2], proofs[2].ProofHash(), 1, true) {
		t.Fatal("known orphan refused")
	}

	// Fill the outpoint with one orphan per peer.
	for peer := Tag(10); len(s.findOrphans(proofs[0].PrevOut)) < maxDSProofOrphansPerOutPoint; peer++ {
		s.addOrphan(proofs[next], proofs[next].ProofHash(), peer, true)
		next++
	}
	// The next one drops an orphan of peer 1, the only peer holding two.
	if !s.addOrphan(proofs[next], proofs[next].ProofHash(), 100, true) {
		t.Fatal("orphan refused while a peer holds two")
	}
	if s.lookup(proofs[0].ProofHash()) == nil {
		t.Fatal("dropped the oldest orphan of a peer holding one")
	}
	if s.lookup(proofs[1].ProofHash()) != nil || s.lookup(proofs[2].ProofHash()) == nil {
		t.Fatal("did not drop the oldest orphan of peer 1")
	}
	if len(s.findOrphans(proofs[0].PrevOut)) != maxDSProofOrphansPerOutPoint {
		t.Fatal("outpoint over its limit")
	}
	// Now every peer holds one, so newcomers are refused.
	next++
	if s.addOrphan(proofs[next], proofs[next].ProofHash(), 101, true) {
		t.Fatal("orphan accepted while every peer holds one")
	}

	// An outpoint full of orphans without a peer refuses new ones.
	s2 := newDSProofStorage()
	for i := range maxDSProofOrphansPerOutPoint {
		s2.addOrphan(proofs[i], proofs[i].ProofHash(), 0, false)
	}
	p := proofs[maxDSProofOrphansPerOutPoint]
	if s2.addOrphan(p, p.ProofHash(), 7, true) {
		t.Fatal("orphan without a peer evicted")
	}
}

// TestDSProofStorageHonestOrphanSurvives checks that an orphan seen first
// survives any number of peers sending one orphan each for its outpoint.
func TestDSProofStorageHonestOrphanSurvives(t *testing.T) {
	s := newDSProofStorage()
	proofs := makeDupeProofs(t, 3*maxDSProofOrphansPerOutPoint, 0)
	honest := proofs[0].ProofHash()
	s.addOrphan(proofs[0], honest, 1, true)
	for i, p := range proofs[1:] {
		s.addOrphan(p, p.ProofHash(), Tag(i+2), true)
	}
	if s.lookup(honest) == nil {
		t.Fatal("honest orphan dropped")
	}
	if n := len(s.findOrphans(proofs[0].PrevOut)); n != maxDSProofOrphansPerOutPoint {
		t.Fatalf("%d orphans for the outpoint", n)
	}
}

// TestDSProofStorageClaimAndRemove checks claiming orphans, making them
// orphans again and removing them.
func TestDSProofStorageClaimAndRemove(t *testing.T) {
	s := newDSProofStorage()
	var proofs []*wire.MsgDSProof
	for i := range 100 {
		proofs = append(proofs, makeDupeProofs(t, 1, uint64(i+1))[0])
	}
	for i, p := range proofs {
		s.addOrphan(p, p.ProofHash(), Tag(i), true)
	}
	if s.numOrphans != 100 || len(s.proofs) != 100 {
		t.Fatalf("%d orphans, %d proofs", s.numOrphans, len(s.proofs))
	}
	for i, p := range proofs {
		if s.add(p, p.ProofHash()) {
			t.Fatal("claimed orphan reported as new")
		}
		if s.numOrphans != 100-i-1 {
			t.Fatalf("%d orphans after %d claims", s.numOrphans, i+1)
		}
	}
	for _, p := range proofs {
		s.addOrphan(p, p.ProofHash(), 0, false)
	}
	if s.numOrphans != 100 || len(s.proofs) != 100 {
		t.Fatalf("%d orphans, %d proofs", s.numOrphans, len(s.proofs))
	}
	for _, p := range proofs {
		if !s.remove(p.ProofHash()) || s.lookup(p.ProofHash()) != nil {
			t.Fatal("remove failed")
		}
		if s.remove(p.ProofHash()) {
			t.Fatal("removed twice")
		}
	}
	if s.numOrphans != 0 || len(s.proofs) != 0 || len(s.byOutPoint) != 0 {
		t.Fatalf("%d orphans, %d proofs left", s.numOrphans, len(s.proofs))
	}
}

// TestDSProofStorageOrphanLimit checks that the orphan count stays within a
// quarter above the limit and drops the oldest.
func TestDSProofStorageOrphanLimit(t *testing.T) {
	s := newDSProofStorage()
	s.maxOrphans = 20
	now := time.Unix(2000000, 0)
	s.now = func() time.Time { return now }
	proofs := makeDupeProofs(t, 200, 7)
	for i, p := range proofs {
		now = now.Add(time.Second)
		s.addOrphan(p, p.ProofHash(), Tag(i), true)
		if s.numOrphans > 25 {
			t.Fatalf("%d orphans", s.numOrphans)
		}
	}
	if s.numOrphans < 20 || len(s.proofs) != s.numOrphans {
		t.Fatalf("%d orphans, %d proofs", s.numOrphans, len(s.proofs))
	}
	if len(s.findOrphans(proofs[0].PrevOut)) != s.numOrphans {
		t.Fatal("outpoint index out of sync")
	}
	if s.lookup(proofs[len(proofs)-1].ProofHash()) == nil {
		t.Fatal("newest orphan removed")
	}
	if s.lookup(proofs[0].ProofHash()) != nil {
		t.Fatal("oldest orphan kept")
	}
}

// TestDSProofStorageExpiry checks that only orphans older than the TTL
// expire, and that the peers that sent them are returned.
func TestDSProofStorageExpiry(t *testing.T) {
	s := newDSProofStorage()
	s.orphanTTL = 50 * time.Second
	start := time.Unix(2000000, 0)
	now := start
	s.now = func() time.Time { return now }
	var proofs []*wire.MsgDSProof
	for i := range 200 {
		proofs = append(proofs, makeDupeProofs(t, 1, uint64(i+1))[0])
	}
	added := make(map[chainhash.Hash]time.Time)
	for i, p := range proofs {
		now = start.Add(time.Duration(2*i) * time.Second)
		s.addOrphan(p, p.ProofHash(), Tag(i), i%2 == 0)
		added[p.ProofHash()] = now
	}
	// A proof that is not an orphan never expires.
	keep := makeDupeProofs(t, 1, 999)[0]
	s.add(keep, keep.ProofHash())

	peers := s.expireOrphans()
	cutoff := now.Add(-s.orphanTTL)
	for h, at := range added {
		if exists := s.lookup(h) != nil; exists != at.After(cutoff) {
			t.Fatalf("orphan added at %v: exists %v", at, exists)
		}
	}
	if s.numOrphans != 25 {
		t.Fatalf("%d orphans left", s.numOrphans)
	}
	if len(peers) != 88 {
		t.Fatalf("%d peers to blame", len(peers))
	}
	if s.lookup(keep.ProofHash()) == nil {
		t.Fatal("non-orphan expired")
	}
}

// TestDSProofRejected checks the rejected id memory.
func TestDSProofRejected(t *testing.T) {
	s := newDSProofStorage()
	first := chainhash.Hash{1}
	s.markRejected(first)
	if !s.isRejected(first) || s.isRejected(chainhash.Hash{2}) {
		t.Fatal("rejected set wrong")
	}
	for i := range maxRejectedDSProofs / 2 {
		s.markRejected(chainhash.Hash{2, byte(i), byte(i >> 8), byte(i >> 16)})
	}
	if !s.isRejected(first) {
		t.Fatal("dropped too early")
	}
	for i := range maxRejectedDSProofs / 2 {
		s.markRejected(chainhash.Hash{3, byte(i), byte(i >> 8), byte(i >> 16)})
	}
	if s.isRejected(first) {
		t.Fatal("kept too long")
	}
}

// TestDSProofMempoolDoubleSpend checks that rejecting a double spend creates
// one proof for the memory pool transaction and that the getters find it.
func TestDSProofMempoolDoubleSpend(t *testing.T) {
	harness, rec := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	op, coin, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
	tx3 := spend(t, harness, op, coin, 2, 3000, txscript.SigHashAll, false)

	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	if score, err := pool.DSProofScore(tx1.Hash()); err != nil || score != 1 {
		t.Fatalf("score %v %v", score, err)
	}
	if _, err := pool.ProcessTransaction(tx2, true, false, 0); err == nil {
		t.Fatal("double spend accepted")
	}
	proofs, _ := rec.take()
	if len(proofs) != 1 || *proofs[0].tx.Hash() != *tx1.Hash() {
		t.Fatalf("got %d announcements", len(proofs))
	}
	proof := proofs[0].proof
	hash := proof.ProofHash()
	want, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	if err != nil || want.ProofHash() != hash {
		t.Fatalf("stored proof differs: %v", err)
	}

	// One proof per transaction: another double spend adds nothing.
	if _, err := pool.ProcessTransaction(tx3, true, false, 0); err == nil {
		t.Fatal("double spend accepted")
	}
	if p, _ := rec.take(); len(p) != 0 {
		t.Fatal("second proof for the same transaction")
	}

	if !pool.HaveDSProof(&hash) {
		t.Fatal("HaveDSProof")
	}
	if got, err := pool.FetchDSProof(&hash); err != nil || got != proof {
		t.Fatalf("FetchDSProof: %v", err)
	}
	list := pool.DSProofs(false)
	if len(list) != 1 || list[0].Hash != hash || *list[0].TxHash != *tx1.Hash() {
		t.Fatalf("DSProofs: %+v", list)
	}
	desc, desc2 := pool.DSProofByHash(&hash)
	if desc == nil || *desc.TxHash != *tx1.Hash() || len(desc2) != 1 {
		t.Fatal("DSProofByHash")
	}
	if d, _ := pool.DSProofByOutPoint(op); d == nil || d.Hash != hash {
		t.Fatal("DSProofByOutPoint")
	}
	if d, _, _, _ := pool.DSProofByTx(tx1.Hash(), false); d == nil || d.Hash != hash {
		t.Fatal("DSProofByTx")
	}
	if score, err := pool.DSProofScore(tx1.Hash()); err != nil || score != 0 {
		t.Fatalf("score %v %v", score, err)
	}

	// The same proof from a peer is ignored, as is a conflicting one.
	if r, err := pool.ProcessDSProof(proof, 1, true); r != DSProofIgnored || err != nil {
		t.Fatalf("resent proof: %v %v", r, err)
	}
	other, err := newDSProof(tx1.MsgTx(), tx3.MsgTx(), op, coin, testFlags, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := pool.ProcessDSProof(other, 1, true); r != DSProofIgnored || err != nil {
		t.Fatalf("conflicting proof: %v %v", r, err)
	}
	if len(pool.DSProofs(true)) != 1 {
		t.Fatal("conflicting proof stored")
	}
}

// TestDSProofMempoolReceived checks proofs received from peers: before the
// transaction (orphans that are rescued), after it, and invalid ones.
func TestDSProofMempoolReceived(t *testing.T) {
	harness, rec := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool

	// Received after the transaction.
	op, coin, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
	proof, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	if r, err := pool.ProcessDSProof(proof, 7, true); r != DSProofAccepted || err != nil {
		t.Fatalf("%v %v", r, err)
	}
	if p, _ := rec.take(); len(p) != 1 || *p[0].tx.Hash() != *tx1.Hash() {
		t.Fatal("no announcement")
	}

	// Received before the transaction.
	op, coin, tx1, tx2 = doubleSpendPair(t, harness, txscript.SigHashAll, false)
	proof, _ = newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	if r, err := pool.ProcessDSProof(proof, 7, true); r != DSProofOrphan || err != nil {
		t.Fatalf("%v %v", r, err)
	}
	hash := proof.ProofHash()
	if d, _ := pool.DSProofByHash(&hash); d == nil || d.TxHash != nil {
		t.Fatal("orphan not stored")
	}
	if d, _ := pool.DSProofByOutPoint(op); d == nil || d.Hash != hash || d.TxHash != nil {
		t.Fatal("orphan not found by outpoint")
	}
	if len(pool.DSProofs(false)) != 1 || len(pool.DSProofs(true)) != 2 {
		t.Fatal("orphan listed wrong")
	}
	if _, err := pool.ProcessTransaction(tx2, true, false, 0); err != nil {
		t.Fatal(err)
	}
	if p, _ := rec.take(); len(p) != 1 || *p[0].tx.Hash() != *tx2.Hash() {
		t.Fatal("orphan not rescued")
	}
	if d, _ := pool.DSProofByHash(&hash); d == nil || *d.TxHash != *tx2.Hash() {
		t.Fatal("rescued orphan not associated")
	}
	if pool.dsProofs.numOrphans != 0 {
		t.Fatal("orphan count")
	}

	// Received before the output it spends exists.
	parent := spend(t, harness, addCoins(harness, &wire.TxOut{Value: 100000000, PkScript: harness.payScript})[0],
		&wire.TxOut{Value: 100000000, PkScript: harness.payScript}, 1, 1000, txscript.SigHashAll, true)
	pOp := wire.OutPoint{Hash: *parent.Hash()}
	pCoin := parent.MsgTx().TxOut[0]
	c1 := spend(t, harness, pOp, pCoin, 1, 1000, txscript.SigHashAll, true)
	c2 := spend(t, harness, pOp, pCoin, 1, 2000, txscript.SigHashAll, true)
	cProof, err := newDSProof(c1.MsgTx(), c2.MsgTx(), pOp, pCoin, testFlags, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := pool.ProcessDSProof(cProof, 8, true); r != DSProofOrphan || err != nil {
		t.Fatalf("%v %v", r, err)
	}
	if _, err := pool.ProcessTransaction(parent, true, false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.ProcessTransaction(c1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	if p, _ := rec.take(); len(p) != 1 || *p[0].tx.Hash() != *c1.Hash() {
		t.Fatal("orphan for an unconfirmed output not rescued")
	}

	// An invalid proof is rejected and remembered.
	op, coin, tx1, tx2 = doubleSpendPair(t, harness, txscript.SigHashAll, true)
	bad, _ := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	bad.FirstSpender.LockTime = 5
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	badHash := bad.ProofHash()
	if _, err := pool.ProcessDSProof(bad, 9, true); err == nil {
		t.Fatal("invalid proof accepted")
	}
	if !pool.HaveDSProof(&badHash) {
		t.Fatal("invalid proof not remembered")
	}
	if _, err := pool.FetchDSProof(&badHash); err == nil {
		t.Fatal("invalid proof served")
	}

	// An invalid orphan blames its peer once the transaction arrives.
	op, coin, tx1, tx2 = doubleSpendPair(t, harness, txscript.SigHashAll, true)
	badOrphan, _ := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	badOrphan.DoubleSpender.Sequence = 1
	good, _ := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	if r, _ := pool.ProcessDSProof(badOrphan, 10, true); r != DSProofOrphan {
		t.Fatal("orphan not stored")
	}
	if r, _ := pool.ProcessDSProof(good, 11, true); r != DSProofOrphan {
		t.Fatal("orphan not stored")
	}
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	p, bad2 := rec.take()
	if len(p) != 1 || p[0].proof.ProofHash() != good.ProofHash() {
		t.Fatal("valid orphan not rescued")
	}
	if len(bad2) != 1 || bad2[0] != 10 {
		t.Fatalf("peers blamed: %v", bad2)
	}
	if pool.dsProofs.numOrphans != 0 {
		t.Fatal("orphans left")
	}

	// Disabled, nothing is created or stored.
	pool.cfg.DisableDSProofs = true
	op, coin, tx1, tx2 = doubleSpendPair(t, harness, txscript.SigHashAll, true)
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.ProcessTransaction(tx2, true, false, 0)
	proof, _ = newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	if r, err := pool.ProcessDSProof(proof, 1, true); r != DSProofIgnored || err != nil {
		t.Fatal("disabled pool handled a proof")
	}
	if p, _ := rec.take(); len(p) != 0 {
		t.Fatal("disabled pool created a proof")
	}
}

// TestDSProofMempoolRemoval checks that the proof of a transaction leaving
// the pool becomes an orphan that comes back with the transaction and
// otherwise expires.
func TestDSProofMempoolRemoval(t *testing.T) {
	harness, rec := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	now := time.Now()
	pool.dsProofs.now = func() time.Time { return now }
	_, _, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.ProcessTransaction(tx2, true, false, 0)
	p, _ := rec.take()
	if len(p) != 1 {
		t.Fatal("no proof")
	}
	hash := p[0].proof.ProofHash()

	// Confirmed: the proof becomes an orphan.
	pool.RemoveTransaction(tx1, false)
	if len(pool.DSProofs(false)) != 0 || len(pool.DSProofs(true)) != 1 {
		t.Fatal("proof not orphaned")
	}
	if d, _ := pool.DSProofByHash(&hash); d == nil || d.TxHash != nil {
		t.Fatal("orphan still associated")
	}

	// Back after a reorg: associated again and announced.
	if _, _, err := pool.MaybeAcceptTransaction(tx1, false, false); err != nil {
		t.Fatal(err)
	}
	if p, _ := rec.take(); len(p) != 1 || p[0].proof.ProofHash() != hash {
		t.Fatal("proof not restored")
	}
	if d, _ := pool.DSProofByHash(&hash); d == nil || d.TxHash == nil {
		t.Fatal("proof not associated")
	}

	// Removed again and expired.
	pool.RemoveTransaction(tx1, false)
	now = now.Add(DefaultDSProofOrphanTTL - time.Second)
	pool.ExpireDSProofOrphans()
	if len(pool.DSProofs(true)) != 1 {
		t.Fatal("expired too early")
	}
	now = now.Add(time.Second)
	if peers := pool.ExpireDSProofOrphans(); len(peers) != 0 {
		t.Fatalf("created proof blamed peers %v", peers)
	}
	if len(pool.DSProofs(true)) != 0 {
		t.Fatal("orphan did not expire")
	}
}

// TestDSProofMempoolSearch checks the ancestor search, descendants and the
// confidence score.
func TestDSProofMempoolSearch(t *testing.T) {
	harness, rec := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	op, coin, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}

	// A chain of children of tx1.
	chain := []*bchutil.Tx{tx1}
	for range 5 {
		parent := chain[len(chain)-1]
		out := parent.MsgTx().TxOut[0]
		child := spend(t, harness, wire.OutPoint{Hash: *parent.Hash()}, out, 1, 1000,
			txscript.SigHashAll, true)
		if _, err := pool.ProcessTransaction(child, true, false, 0); err != nil {
			t.Fatal(err)
		}
		chain = append(chain, child)
	}
	leaf := chain[len(chain)-1]
	if score, _ := pool.DSProofScore(leaf.Hash()); score != 1 {
		t.Fatalf("score %v", score)
	}
	if d, _, _, err := pool.DSProofByTx(leaf.Hash(), true); d != nil || err != nil {
		t.Fatal("proof found before the double spend")
	}

	_, _ = pool.ProcessTransaction(tx2, true, false, 0)
	if p, _ := rec.take(); len(p) != 1 {
		t.Fatal("no proof")
	}
	_ = op
	_ = coin

	d, desc, path, err := pool.DSProofByTx(leaf.Hash(), true)
	if err != nil || d == nil || *d.TxHash != *tx1.Hash() {
		t.Fatalf("recursive lookup: %v", err)
	}
	if len(path) != len(chain) || path[0] != *leaf.Hash() || path[len(path)-1] != *tx1.Hash() {
		t.Fatalf("path %v", path)
	}
	if len(desc) != len(chain) {
		t.Fatalf("%d descendants", len(desc))
	}
	for i := 1; i < len(desc); i++ {
		if desc[i-1].Compare(&desc[i]) >= 0 {
			t.Fatal("descendants not sorted")
		}
	}
	if d, _, _, _ := pool.DSProofByTx(leaf.Hash(), false); d != nil {
		t.Fatal("non-recursive lookup found an ancestor's proof")
	}
	for _, tx := range chain {
		if score, _ := pool.DSProofScore(tx.Hash()); score != 0 {
			t.Fatalf("score %v", score)
		}
	}
	if _, err := pool.DSProofScore(&chainhash.Hash{9}); err == nil {
		t.Fatal("score for an unknown transaction")
	}

	// A transaction not signing all outputs scores 0.25, and so do its
	// children.
	op2 := addCoins(harness, &wire.TxOut{Value: 100000000, PkScript: harness.payScript})[0]
	acp := spend(t, harness, op2, &wire.TxOut{Value: 100000000, PkScript: harness.payScript}, 1, 1000,
		txscript.SigHashAll|txscript.SigHashAnyOneCanPay, true)
	if _, err := pool.ProcessTransaction(acp, true, false, 0); err != nil {
		t.Fatal(err)
	}
	if score, _ := pool.DSProofScore(acp.Hash()); score != 0.25 {
		t.Fatalf("score %v", score)
	}
	acpChild := spend(t, harness, wire.OutPoint{Hash: *acp.Hash()}, acp.MsgTx().TxOut[0], 1, 1000,
		txscript.SigHashAll, false)
	if _, err := pool.ProcessTransaction(acpChild, true, false, 0); err != nil {
		t.Fatal(err)
	}
	if score, _ := pool.DSProofScore(acpChild.Hash()); score != 0.25 {
		t.Fatalf("score %v", score)
	}

	// A transaction spending a pay-to-pubkey output cannot have a proof.
	pubKey := (*bchec.PublicKey)(&harness.signKey.PublicKey).SerializeCompressed()
	p2pk, _ := txscript.NewScriptBuilder().AddData(pubKey).AddOp(txscript.OP_CHECKSIG).Script()
	pkCoin := &wire.TxOut{Value: 100000000, PkScript: p2pk}
	pkOp := addCoins(harness, pkCoin)[0]
	pkTx := wire.NewMsgTx(1)
	pkTx.AddTxIn(&wire.TxIn{PreviousOutPoint: pkOp, Sequence: wire.MaxTxInSequenceNum})
	pkTx.AddTxOut(&wire.TxOut{Value: pkCoin.Value - 1000, PkScript: harness.payScript})
	sig, err := txscript.RawTxInSchnorrSignature(pkTx, 0, p2pk, txscript.SigHashAll, harness.signKey, pkCoin.Value)
	if err != nil {
		t.Fatal(err)
	}
	pkTx.TxIn[0].SignatureScript, _ = txscript.NewScriptBuilder().AddData(sig).Script()
	if _, err := pool.ProcessTransaction(bchutil.NewTx(pkTx), true, false, 0); err != nil {
		t.Fatal(err)
	}
	if score, _ := pool.DSProofScore(bchutil.NewTx(pkTx).Hash()); score != 0 {
		t.Fatalf("score %v", score)
	}
}

// TestDSProofMempoolOrphanFlood checks that invalid orphans sent for one
// outpoint from many peers are bounded, and that a valid orphan sent before
// them is still found when the transaction arrives.
func TestDSProofMempoolOrphanFlood(t *testing.T) {
	harness, rec := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	op, coin, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
	good, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := pool.ProcessDSProof(good, 1000, true); r != DSProofOrphan {
		t.Fatalf("valid orphan %v", r)
	}
	for i := range 500 {
		junk := mutateProof(good, func(p *wire.MsgDSProof) { p.FirstSpender.LockTime = uint32(i + 1) })
		if _, err := pool.ProcessDSProof(junk, Tag(i/3), true); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(pool.dsProofs.findOrphans(op)); n != maxDSProofOrphansPerOutPoint {
		t.Fatalf("%d orphans for the outpoint", n)
	}
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	p, bad := rec.take()
	if len(p) != 1 || p[0].proof.ProofHash() != good.ProofHash() {
		t.Fatal("valid orphan not rescued")
	}
	// The valid orphan is the oldest, so it is checked first and the rest
	// are dropped unchecked.
	if len(bad) != 0 {
		t.Fatalf("%d peers blamed", len(bad))
	}
	if pool.dsProofs.numOrphans != 0 {
		t.Fatalf("%d orphans left", pool.dsProofs.numOrphans)
	}
}

// TestDSProofSearchOrder checks that the ancestor search visits parents in
// the order they entered the pool, not the order of the inputs.
func TestDSProofSearchOrder(t *testing.T) {
	harness, _ := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	var parents []*bchutil.Tx
	for range 2 {
		_, _, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
		if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
			t.Fatal(err)
		}
		_, _ = pool.ProcessTransaction(tx2, true, false, 0)
		parents = append(parents, tx1)
	}

	// The child spends the second parent first.
	child := wire.NewMsgTx(1)
	var total int64
	for _, p := range []*bchutil.Tx{parents[1], parents[0]} {
		child.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Hash: *p.Hash()},
			Sequence: wire.MaxTxInSequenceNum})
		total += p.MsgTx().TxOut[0].Value
	}
	child.AddTxOut(&wire.TxOut{Value: total - 2000, PkScript: harness.payScript})
	for i, p := range []*bchutil.Tx{parents[1], parents[0]} {
		signP2PKH(t, child, i, p.MsgTx().TxOut[0], harness.signKey, txscript.SigHashAll, true)
	}
	if _, err := pool.ProcessTransaction(bchutil.NewTx(child), true, false, 0); err != nil {
		t.Fatal(err)
	}
	d, _, path, err := pool.DSProofByTx(bchutil.NewTx(child).Hash(), true)
	if err != nil || d == nil || *d.TxHash != *parents[0].Hash() || len(path) != 2 {
		t.Fatalf("found %v via %v: %v", d, path, err)
	}
}

// TestDSProofSearchLimit checks that a chain of unconfirmed transactions
// deeper than the search limit scores 0.25 and fails the recursive lookup.
func TestDSProofSearchLimit(t *testing.T) {
	harness, _ := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	coin := &wire.TxOut{Value: 100000000, PkScript: harness.payScript}
	op := addCoins(harness, coin)[0]
	var last *bchutil.Tx
	for range dsProofSearchMaxDepth + 1 {
		tx := spend(t, harness, op, coin, 1, 0, txscript.SigHashAll, true)
		if _, err := pool.ProcessTransaction(tx, true, false, 0); err != nil {
			t.Fatal(err)
		}
		last = tx
		op = wire.OutPoint{Hash: *tx.Hash()}
		coin = tx.MsgTx().TxOut[0]
	}
	if score, err := pool.DSProofScore(last.Hash()); err != nil || score != 0.25 {
		t.Fatalf("score %v %v", score, err)
	}
	if _, _, _, err := pool.DSProofByTx(last.Hash(), true); err != ErrDSProofSearchLimit {
		t.Fatalf("lookup: %v", err)
	}
	if d, _, _, err := pool.DSProofByTx(last.Hash(), false); d != nil || err != nil {
		t.Fatalf("non-recursive lookup: %v %v", d, err)
	}
}

// TestDSProofSearchOrderReorg checks that transactions added back from a
// disconnected block are searched before those already in the pool.
func TestDSProofSearchOrderReorg(t *testing.T) {
	harness, _ := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	var parents []*bchutil.Tx
	for i := range 2 {
		_, _, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
		// The second parent comes back from a disconnected block.
		if _, _, err := pool.MaybeAcceptTransaction(tx1, i == 0, false); err != nil {
			t.Fatal(err)
		}
		_, _ = pool.ProcessTransaction(tx2, true, false, 0)
		parents = append(parents, tx1)
	}

	child := wire.NewMsgTx(1)
	var total int64
	for _, p := range parents {
		child.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Hash: *p.Hash()},
			Sequence: wire.MaxTxInSequenceNum})
		total += p.MsgTx().TxOut[0].Value
	}
	child.AddTxOut(&wire.TxOut{Value: total - 2000, PkScript: harness.payScript})
	for i, p := range parents {
		signP2PKH(t, child, i, p.MsgTx().TxOut[0], harness.signKey, txscript.SigHashAll, true)
	}
	if _, err := pool.ProcessTransaction(bchutil.NewTx(child), true, false, 0); err != nil {
		t.Fatal(err)
	}
	d, _, _, err := pool.DSProofByTx(bchutil.NewTx(child).Hash(), true)
	if err != nil || d == nil || *d.TxHash != *parents[1].Hash() {
		t.Fatalf("found %v: %v", d, err)
	}
}

// TestDSProofRejectedNotRechecked checks that a proof whose id was rejected
// is refused with the same error without being checked again, even once
// it could no longer be checked.
func TestDSProofRejectedNotRechecked(t *testing.T) {
	harness, _ := newDSProofHarness(t, &chaincfg.MainNetParams)
	pool := harness.txPool
	op, coin, tx1, tx2 := doubleSpendPair(t, harness, txscript.SigHashAll, true)
	good, err := newDSProof(tx1.MsgTx(), tx2.MsgTx(), op, coin, testFlags, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.ProcessTransaction(tx1, true, false, 0); err != nil {
		t.Fatal(err)
	}
	bad := mutateProof(good, func(p *wire.MsgDSProof) { p.FirstSpender.LockTime ^= 1 })
	hash := bad.ProofHash()
	if r, err := pool.ProcessDSProof(bad, 7, true); r != DSProofIgnored || err == nil {
		t.Fatalf("bad proof: %v %v", r, err)
	}
	if !pool.HaveDSProof(&hash) {
		t.Fatal("not remembered as rejected")
	}

	// Without the transaction a new check would make it an orphan.
	pool.RemoveTransaction(tx1, true)
	for range 2 {
		if r, err := pool.ProcessDSProof(bad, 7, true); r != DSProofIgnored || err == nil {
			t.Fatalf("rejected proof: %v %v", r, err)
		}
	}
	if len(pool.DSProofs(true)) != 0 {
		t.Fatal("rejected proof stored")
	}
}

// checkPeerOrphans checks the per peer orphan counts against the entries.
func checkPeerOrphans(t *testing.T, s *dsProofStorage) {
	t.Helper()
	want := make(map[Tag]int)
	for _, e := range s.proofs {
		if e.orphan && e.hasPeer {
			want[e.peer]++
		}
	}
	if len(want) != len(s.peerOrphans) {
		t.Fatalf("peer orphan counts %v, want %v", s.peerOrphans, want)
	}
	for p, n := range want {
		if s.peerOrphans[p] != n {
			t.Fatalf("peer orphan counts %v, want %v", s.peerOrphans, want)
		}
	}
}

// TestDSProofStoragePeerShare checks that one peer's orphans cannot take more
// than their share of the orphan limit, and that the share frees up as its
// orphans are claimed, removed or expire.
func TestDSProofStoragePeerShare(t *testing.T) {
	s := newDSProofStorage()
	s.maxOrphans = 40
	share := s.maxOrphans / dsProofOrphanPeerShare
	now := time.Unix(2000000, 0)
	s.now = func() time.Time { return now }

	other := makeDupeProofs(t, 1, 5000)[0]
	if !s.addOrphan(other, other.ProofHash(), 1, true) {
		t.Fatal("orphan of another peer refused")
	}
	var proofs []*wire.MsgDSProof
	for i := range 3 * share {
		p := makeDupeProofs(t, 1, uint64(i+1))[0]
		proofs = append(proofs, p)
		if got := s.addOrphan(p, p.ProofHash(), 2, true); got != (i < share) {
			t.Fatalf("orphan %d: added %v", i, got)
		}
		checkPeerOrphans(t, s)
	}
	if s.numOrphans != share+1 || s.lookup(other.ProofHash()) == nil {
		t.Fatalf("%d orphans", s.numOrphans)
	}

	// Orphans without a peer and a known proof are not limited.
	extra := proofs[share]
	if !s.addOrphan(extra, extra.ProofHash(), 0, false) {
		t.Fatal("orphan without a peer refused")
	}
	if !s.addOrphan(proofs[0], proofs[0].ProofHash(), 2, true) {
		t.Fatal("known orphan refused")
	}
	checkPeerOrphans(t, s)

	// A claimed orphan and a removed one free two places.
	s.add(proofs[0], proofs[0].ProofHash())
	s.remove(proofs[1].ProofHash())
	checkPeerOrphans(t, s)
	for i := share + 1; i < share+4; i++ {
		got := s.addOrphan(proofs[i], proofs[i].ProofHash(), 2, true)
		if got != (i < share+3) {
			t.Fatalf("orphan %d: added %v", i, got)
		}
	}
	checkPeerOrphans(t, s)

	// Expired orphans free the share.
	now = now.Add(time.Hour)
	s.expireOrphans()
	checkPeerOrphans(t, s)
	if len(s.peerOrphans) != 0 {
		t.Fatalf("counts left: %v", s.peerOrphans)
	}
	p := proofs[len(proofs)-1]
	if !s.addOrphan(p, p.ProofHash(), 2, true) {
		t.Fatal("orphan refused after expiry")
	}
	if s.orphanExisting(proofs[0].ProofHash()); s.peerOrphans[2] != 1 {
		t.Fatalf("counts %v", s.peerOrphans)
	}
	checkPeerOrphans(t, s)
}
