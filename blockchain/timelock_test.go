// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// timelockVectorFile is the layout of the CHIP-2025-03 timelock vectors.
type timelockVectorFile struct {
	Constants map[string]string `json:"constants"`
	Schedules []struct {
		Name     string `json:"name"`
		Schedule []struct {
			Height string `json:"height"`
			Tau    string `json:"tau"`
		} `json:"schedule"`
	} `json:"schedules"`
	Vectors []struct {
		Name              string            `json:"name"`
		Kind              string            `json:"kind"`
		Schedule          string            `json:"schedule"`
		Args              map[string]string `json:"args"`
		SubjectIsCoinbase bool              `json:"subjectIsCoinbase"`
		Expected          struct {
			ReleaseBlock string `json:"releaseBlock"`
			Blocks       []struct {
				Block  string `json:"block"`
				Result bool   `json:"result"`
			} `json:"blocks"`
			Mempool []struct {
				Tip    string `json:"tip"`
				Result bool   `json:"result"`
			} `json:"mempool"`
		} `json:"expected"`
	} `json:"vectors"`
}

// vectorSchedule is a tick schedule of the vectors: blocks after each entry's
// height take its interval, and blocks up to the first entry take 600.
type vectorSchedule []struct{ height, tau int64 }

// ticks returns the ticks of the block at the passed height.
func (s vectorSchedule) ticks(height int64) int64 {
	var ticks, from int64
	tau := int64(600)
	for _, e := range s {
		if height <= e.height {
			break
		}
		ticks += (e.height - from) * tau
		from, tau = e.height, e.tau
	}
	return ticks + (height-from)*tau
}

// TestTimelockVectors runs the CHIP-2025-03 timelock vectors from fablous
// 9c4b023 against the lock time, sequence lock and coinbase maturity rules.
// The schedules with at most one change to 60 ticks are also run through the
// exported functions with a TickSchedule.  A coinbase is final whatever its
// lock time only from the block after the activation block.
func TestTimelockVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "chip202503",
		"timelock", "timelock.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vectors timelockVectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	num := func(s string) int64 {
		v, err := strconv.ParseUint(s, 0, 63)
		if err != nil {
			t.Fatalf("bad number %q: %v", s, err)
		}
		return int64(v)
	}
	if num(vectors.Constants["LOCK_UNIT_TICKS"]) != lockTimeUnitTicks ||
		num(vectors.Constants["COINBASE_MATURITY_TICKS"]) !=
			int64(chaincfg.MainNetParams.CoinbaseMaturity)*lockTimeUnitTicks {

		t.Fatalf("constants %v", vectors.Constants)
	}
	// The upgrade applies from the block after the activation block.
	if num(vectors.Constants["UPGRADE_ACTIVE_FROM"]) !=
		num(vectors.Constants["ACTIVATION_HEIGHT_ILLUSTRATION"])+1 {

		t.Fatalf("constants %v", vectors.Constants)
	}
	if len(vectors.Vectors) != 27 {
		t.Fatalf("%d vectors, want 27", len(vectors.Vectors))
	}

	// The schedules, and the activation height of those a TickSchedule
	// can hold: -1 for none, 0 for one that never activates.
	schedules := make(map[string]vectorSchedule)
	activations := make(map[string]int64)
	for _, s := range vectors.Schedules {
		var sched vectorSchedule
		for _, e := range s.Schedule {
			sched = append(sched, struct{ height, tau int64 }{
				num(e.Height), num(e.Tau)})
		}
		schedules[s.Name] = sched
		switch {
		case len(sched) == 0:
			activations[s.Name] = 0
		case len(sched) == 1 && sched[0].tau == 60:
			activations[s.Name] = sched[0].height
		default:
			activations[s.Name] = -1
		}
	}

	if activations["activation_1M"] !=
		num(vectors.Constants["ACTIVATION_HEIGHT_ILLUSTRATION"]) {

		t.Fatalf("activation_1M activates at %d",
			activations["activation_1M"])
	}

	// tickSchedule returns the TickSchedule for block height, or false
	// when the schedule has no TickSchedule.
	tickSchedule := func(name string, height int64) (TickSchedule, bool) {
		ha := activations[name]
		if ha < 0 {
			return TickSchedule{}, false
		}
		active := ha > 0 && height > ha
		return NewTickSchedule(int32(height), int32(ha), active), true
	}

	var coinbaseRows, ran int
	for _, v := range vectors.Vectors {
		sched, ok := schedules[v.Schedule]
		if !ok {
			t.Fatalf("%s: unknown schedule %q", v.Name, v.Schedule)
		}

		// check evaluates the vector for the block at height with
		// ticks, as the block itself or as the block after a tip.
		check := func(height int64, want bool, where string) {
			t.Helper()
			ticks := sched.ticks(height)
			ts, hasTS := tickSchedule(v.Schedule, height)
			if hasTS && ts.NextBlockTicks() != ticks {
				t.Fatalf("%s: %s: schedule gives %d ticks, want %d",
					v.Name, where, ts.NextBlockTicks(), ticks)
			}
			ha := activations[v.Schedule]
			active := ha > 0 && height > ha

			var got bool
			switch v.Kind {
			case "locktime":
				tx := timelockTestTx(uint32(num(v.Args["lockTime"])),
					uint32(num(v.Args["inputSequence"])),
					v.SubjectIsCoinbase)
				if v.SubjectIsCoinbase && !active {
					coinbaseRows++
				}
				got = IsFinalizedTransaction(tx, ticks,
					time.Unix(0, 0), active)

			case "sequence":
				coin := num(v.Args["fundingBlock"])
				coinTicks := sched.ticks(coin)
				if hasTS && ts.HeightToTicks(int32(coin)) != coinTicks {
					t.Fatalf("%s: %s: schedule gives %d ticks "+
						"for the coin, want %d", v.Name, where,
						ts.HeightToTicks(int32(coin)), coinTicks)
				}
				lock := &SequenceLock{
					Seconds: -1,
					BlockTicks: sequenceLockTicks(coinTicks,
						num(v.Args["relativeLock"])),
				}
				got = SequenceLockActive(lock, ticks,
					time.Unix(math.MaxInt32, 0))

			case "maturity":
				coin := num(v.Args["creationBlock"])
				got = coinbaseMature(ticks, sched.ticks(coin),
					60000)
				if hasTS {
					err := checkCoinbaseSpend(ts, int32(coin))
					if (err == nil) != want {
						t.Errorf("%s: %s: CheckTransactionInputs "+
							"gave %v, want mature %v",
							v.Name, where, err, want)
					}
					if err != nil {
						requireRuleError(t, err,
							ErrImmatureSpend)
					}
				}

			default:
				t.Fatalf("%s: unknown kind %q", v.Name, v.Kind)
			}
			if got != want {
				t.Errorf("%s: %s: got %v, want %v", v.Name, where,
					got, want)
			}
			ran++
		}

		release := int64(-1)
		for _, b := range v.Expected.Blocks {
			if b.Result && release < 0 {
				release = num(b.Block)
			}
			check(num(b.Block), b.Result, "block "+b.Block)
		}
		if release != num(v.Expected.ReleaseBlock) {
			t.Errorf("%s: first final block %d, want release block "+
				"%s", v.Name, release, v.Expected.ReleaseBlock)
		}
		for _, m := range v.Expected.Mempool {
			check(num(m.Tip)+1, m.Result, "mempool tip "+m.Tip)
		}
	}
	if coinbaseRows != 12 {
		t.Errorf("checked %d coinbase rows at or below the activation "+
			"block, want 12", coinbaseRows)
	}
	if ran == 0 {
		t.Fatal("no vectors ran")
	}
}

// timelockTestTx returns a transaction, or a coinbase, with one input with
// the passed sequence and the passed lock time.
func timelockTestTx(lockTime, sequence uint32, coinbase bool) *bchutil.Tx {
	prevOut := wire.OutPoint{Hash: chainhash.Hash{1}}
	if coinbase {
		prevOut = wire.OutPoint{Index: math.MaxUint32}
	}
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut, Sequence: sequence})
	tx.AddTxOut(wire.NewTxOut(1, nil, wire.TokenData{}))
	tx.LockTime = lockTime
	return bchutil.NewTx(tx)
}

// checkCoinbaseSpend checks the inputs of a transaction that spends a
// mainnet coinbase output created at coinHeight in the block of the passed
// schedule.
func checkCoinbaseSpend(sched TickSchedule, coinHeight int32) error {
	tx := timelockTestTx(0, 0, false)
	view := NewUtxoViewpoint()
	view.Entries()[tx.MsgTx().TxIn[0].PreviousOutPoint] = NewUtxoEntry(
		wire.NewTxOut(1, nil, wire.TokenData{}), coinHeight, true)
	_, err := CheckTransactionInputs(tx, sched, view,
		&chaincfg.MainNetParams)
	return err
}

// TestLockTimeTicksOverflow ensures a height-based lock time is multiplied by
// 600 without wrapping at 32 bits.  600*7158279 is 104 modulo 2^32.
func TestLockTimeTicksOverflow(t *testing.T) {
	tx := timelockTestTx(7158279, 0, false)
	for _, ticks := range []int64{105, 1 << 32, 600 * 7158279} {
		if IsFinalizedTransaction(tx, ticks, time.Unix(0, 0), true) {
			t.Errorf("final at %d ticks", ticks)
		}
	}
	if !IsFinalizedTransaction(tx, 600*7158279+1, time.Unix(0, 0), true) {
		t.Error("not final after its lock time")
	}
}

// TestCoinbaseFinality ensures a coinbase with a lock time it has not reached
// is final only when the block is after the activation block, and that the
// exemption does not reach other transactions.
func TestCoinbaseFinality(t *testing.T) {
	for _, coinbase := range []bool{false, true} {
		tx := timelockTestTx(1000, 0, coinbase)
		for _, active := range []bool{false, true} {
			got := IsFinalizedTransaction(tx, 600*1000,
				time.Unix(0, 0), active)
			if got != (coinbase && active) {
				t.Errorf("coinbase %v, active %v: final %v",
					coinbase, active, got)
			}
		}
	}
}

// TestSequenceLockSeam computes sequence locks from the block being connected
// and, as the mempool does, from its parent, on chains that activate 1-minute
// blocks at block 20 and that never do.  A coin from block 20 with a lock of
// one unit is released 600 ticks later: ten 60-tick blocks on, or one block
// on without the upgrade.
func TestSequenceLockSeam(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.CSVHeight = 1

	// Block h is stamped 600*(h-1) seconds after the genesis block, so
	// the median time past of block 20 is that of block 15.
	genesisTime := params.GenesisBlock.Header.Timestamp.Unix()
	tests := []struct {
		name       string
		activation uint64
		release    int32
	}{
		{"active", uint64(genesisTime + 600*14), 10},
		{"never", upgrade2027Never, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chain := newFakeChain(&params)
			chain.upgrade2027ActivationTime = test.activation
			nodes := tickTestChain(chain.bestChain.Tip(),
				regularGaps(40), test.activation)
			for _, node := range nodes {
				chain.index.AddNode(node)
			}
			chain.bestChain.SetTip(nodes[len(nodes)-1])
			coin := nodes[19]
			activation := checkNodeTicks(t, nodes, test.activation)
			if test.activation != upgrade2027Never &&
				activation != coin.height {

				t.Fatalf("activation block at %d", activation)
			}

			// An output of the coin block, and an output still in
			// the mempool, each spent with a lock of one unit.
			fundTx := bchutil.NewTx(&wire.MsgTx{
				TxOut: []*wire.TxOut{{Value: 10}},
			})
			poolTx := bchutil.NewTx(&wire.MsgTx{
				TxOut: []*wire.TxOut{{Value: 20}},
			})
			view := NewUtxoViewpoint()
			view.AddTxOuts(fundTx, coin.height)
			view.AddTxOuts(poolTx, mempoolHeight)
			spend := func(prev *bchutil.Tx) *bchutil.Tx {
				return bchutil.NewTx(&wire.MsgTx{
					Version: 2,
					TxIn: []*wire.TxIn{{
						PreviousOutPoint: wire.OutPoint{
							Hash: *prev.Hash(),
						},
						Sequence: LockTimeToSequence(false, 1),
					}},
				})
			}
			wantTicks := coin.ticks + 599

			mtp := time.Unix(math.MaxInt32, 0)
			for offset := int32(1); offset <= test.release; offset++ {
				block := nodes[coin.height+offset-1]
				prev := block.parent
				want := offset == test.release

				chain.chainLock.Lock()
				lock, err := chain.calcSequenceLock(block,
					spend(fundTx), view, false)
				chain.chainLock.Unlock()
				if err != nil {
					t.Fatalf("block mode: %v", err)
				}
				if lock.BlockTicks != wantTicks {
					t.Fatalf("block mode: %d ticks, want %d",
						lock.BlockTicks, wantTicks)
				}
				if got := SequenceLockActive(lock, block.ticks,
					mtp); got != want {

					t.Errorf("block %d: active %v, want %v",
						block.height, got, want)
				}

				lock, err = chain.NextBlockSequenceLock(&prev.hash,
					spend(fundTx), view)
				if err != nil {
					t.Fatalf("mempool mode: %v", err)
				}
				next := chain.tickSchedule(prev).NextBlockTicks()
				if next != block.ticks {
					t.Fatalf("tip %d: next block at %d ticks, "+
						"want %d", prev.height, next,
						block.ticks)
				}
				if got := SequenceLockActive(lock, next,
					mtp); got != want {

					t.Errorf("tip %d: active %v, want %v",
						prev.height, got, want)
				}

				// A mempool output is taken to be in the next
				// block.
				lock, err = chain.NextBlockSequenceLock(&prev.hash,
					spend(poolTx), view)
				if err != nil {
					t.Fatalf("mempool input: %v", err)
				}
				if lock.BlockTicks != next+599 {
					t.Errorf("tip %d: mempool input locked to "+
						"%d ticks, want %d", prev.height,
						lock.BlockTicks, next+599)
				}
			}
		})
	}
}

// TestUpgrade2027Timelocks connects regtest blocks across the activation
// block, 140, and checks that lock times, sequence locks and coinbase
// maturity count ticks and that a coinbase is exempt from its lock time
// after the activation block only.  The coinbase maturity is one block, so
// 600 ticks.  A coin from block 141 is released 600 ticks later: at block
// 151 with the upgrade, and at block 142 without it.
func TestUpgrade2027Timelocks(t *testing.T) {
	tests := []struct {
		name       string
		activation uint64
		release    int32
	}{
		// Block 140 is the first whose median time past reaches
		// this time.
		{"active", 1700000000 + 600*134, 151},
		{"never", upgrade2027Never, 142},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := chaincfg.RegressionNetParams
			params.Upgrade2027ActivationTime = test.activation
			params.CSVHeight = 1
			m := newRuleTestMiner(t, &params)
			coinbases := make(map[int32]*wire.MsgTx)
			mine := func(block *bchutil.Block) {
				t.Helper()
				m.mustAccept(block)
				coinbases[m.height] = block.MsgBlock().Transactions[0]
			}
			spend := func(height int32, tag byte) *wire.MsgTx {
				cb := coinbases[height]
				return ruleTestSpend(wire.OutPoint{Hash: cb.TxHash()},
					cb.TxOut[0].Value, tag)
			}
			for m.height < 139 {
				mine(m.build(nil))
			}

			// A coinbase locked to a far height is not final in
			// the activation block, and is in the block after it
			// once the upgrade applies.
			lockedCoinbase := func(height int32) *wire.MsgTx {
				cb := m.coinbase(height)
				cb.LockTime = 500000
				cb.TxIn[0].Sequence = 0
				return cb
			}
			err := m.accept(m.build(lockedCoinbase(140)))
			requireRuleError(t, err, ErrUnfinalizedTx)
			mine(m.build(nil))
			if test.activation == upgrade2027Never {
				err := m.accept(m.build(lockedCoinbase(141)))
				requireRuleError(t, err, ErrUnfinalizedTx)
			}

			// Block 141 also creates the output spent with a
			// sequence lock of one unit.
			fund := spend(100, 1)
			cb := m.coinbase(141)
			if test.activation != upgrade2027Never {
				cb = lockedCoinbase(141)
			}
			mine(m.build(cb, fund))

			// The coinbase of block 141, a transaction locked to
			// height 141 and one that spends the output of block
			// 141 with a lock of one unit.
			matureSpend := spend(141, 2)
			lockTx := spend(101, 3)
			lockTx.LockTime = 141
			lockTx.TxIn[0].Sequence = 0
			seqTx := ruleTestSpend(wire.OutPoint{Hash: fund.TxHash()},
				fund.TxOut[0].Value, 4)
			seqTx.TxIn[0].Sequence = LockTimeToSequence(false, 1)
			for m.height+1 < test.release {
				err := m.accept(m.build(nil, matureSpend))
				requireRuleError(t, err, ErrImmatureSpend)
				err = m.accept(m.build(nil, lockTx))
				requireRuleError(t, err, ErrUnfinalizedTx)
				err = m.accept(m.build(nil, seqTx))
				requireRuleError(t, err, ErrUnfinalizedTx)
				mine(m.build(nil))
			}
			mine(m.build(nil, matureSpend, lockTx, seqTx))
		})
	}
}
