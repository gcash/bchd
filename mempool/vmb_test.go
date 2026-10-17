// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// vmbTestDir holds the Bitcoin Cash VM bytecode (VMB) test vectors from
// libauth, split by upgrade and by expected result.
const vmbTestDir = "../txscript/data/vmb_tests"

// vmbClass is the expected result of a VMB test vector.
type vmbClass int

const (
	vmbStandard vmbClass = iota
	vmbNonStandard
	vmbInvalid
)

var vmbClassNames = [...]string{"standard", "nonstandard", "invalid"}

func (c vmbClass) String() string { return vmbClassNames[c] }

// vmbVector is one VMB test vector.  The vector files are arrays of
// [id, description, unlocking asm, locking asm, tx, source outputs] with an
// optional seventh field giving the input to evaluate.
type vmbVector struct {
	id         string
	desc       string
	file       string
	class      vmbClass
	tx         []byte
	utxos      []byte
	inputIndex int

	// scriptOnly vectors only evaluate the input at inputIndex.  The
	// transaction as a whole need not be valid.
	scriptOnly bool
}

// vmbEra holds the rules a set of vectors is run with.
type vmbEra struct {
	name string

	// mtp is the median time past of the tip, which gates the May 2025 and
	// May 2026 upgrades.
	mtp time.Time

	// height is the height of the tip, which gates the May 2023 upgrade.
	height int32

	upgrade9  bool
	upgrade11 bool
	upgrade12 bool
}

// vmbEras returns the eras with vectors, oldest first.  All use mainnet
// parameters.  The source outputs are created in the tip block.
func vmbEras() []vmbEra {
	params := &chaincfg.MainNetParams
	day := 24 * time.Hour
	u11 := time.Unix(int64(params.Upgrade11ActivationTime), 0)
	u12 := time.Unix(int64(params.Upgrade12ActivationTime), 0)
	return []vmbEra{
		{name: "2022", mtp: u11.Add(-365 * day),
			height: params.Upgrade9ForkHeight - 1},
		{name: "2023", mtp: u11.Add(-day), height: 900000,
			upgrade9: true},
		{name: "2025", mtp: u11.Add(day), height: 900000,
			upgrade9: true, upgrade11: true},
		{name: "2026", mtp: u12.Add(day), height: 900000,
			upgrade9: true, upgrade11: true, upgrade12: true},
	}
}

// readMaybeGzip reads a file, decompressing it when it ends in .gz.
func readMaybeGzip(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasSuffix(path, ".gz") {
		return data, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// loadVMBVectors loads every vector of an era.
func loadVMBVectors(t *testing.T, era string) []vmbVector {
	t.Helper()

	var vectors []vmbVector
	for class := vmbStandard; class <= vmbInvalid; class++ {
		dir := filepath.Join(vmbTestDir, "bch_"+era+"_"+class.String())

		scriptOnly := make(map[string]bool)
		files, _ := filepath.Glob(filepath.Join(dir, "*.scriptonly.json*"))
		for _, file := range files {
			data, err := readMaybeGzip(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			var ids []string
			if err := json.Unmarshal(data, &ids); err != nil {
				t.Fatalf("unmarshalling %s: %v", file, err)
			}
			for _, id := range ids {
				scriptOnly[id] = true
			}
		}

		files, _ = filepath.Glob(filepath.Join(dir, "*.vmb_tests.json*"))
		if len(files) == 0 {
			t.Fatalf("no vectors in %s", dir)
		}
		sort.Strings(files)
		for _, file := range files {
			data, err := readMaybeGzip(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			var tests [][]any
			if err := json.Unmarshal(data, &tests); err != nil {
				t.Fatalf("unmarshalling %s: %v", file, err)
			}
			for _, test := range tests {
				if len(test) < 6 {
					t.Fatalf("%s: short vector %v", file, test)
				}
				v := vmbVector{
					id:    test[0].(string),
					desc:  test[1].(string),
					file:  filepath.Base(file),
					class: class,
				}
				v.tx, err = hex.DecodeString(test[4].(string))
				if err != nil {
					t.Fatalf("%s %s: %v", file, v.id, err)
				}
				v.utxos, err = hex.DecodeString(test[5].(string))
				if err != nil {
					t.Fatalf("%s %s: %v", file, v.id, err)
				}
				if len(test) > 6 {
					v.inputIndex = int(test[6].(float64))
				}
				v.scriptOnly = scriptOnly[v.id]
				delete(scriptOnly, v.id)
				vectors = append(vectors, v)
			}
		}
		for id := range scriptOnly {
			t.Errorf("%s: script only vector %s not found", dir, id)
		}
	}
	return vectors
}

// vmbChain is a chain tip holding the source outputs of a vector.
type vmbChain struct {
	utxos  *blockchain.UtxoViewpoint
	height int32
	mtp    time.Time
}

func (c *vmbChain) FetchUtxoView(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
	view := blockchain.NewUtxoViewpoint()
	prevOut := wire.OutPoint{Hash: *tx.Hash()}
	for i := range tx.MsgTx().TxOut {
		prevOut.Index = uint32(i)
		view.Entries()[prevOut] = c.utxos.LookupEntry(prevOut).Clone()
	}
	for _, txIn := range tx.MsgTx().TxIn {
		entry := c.utxos.LookupEntry(txIn.PreviousOutPoint)
		view.Entries()[txIn.PreviousOutPoint] = entry.Clone()
	}
	return view, nil
}

func (c *vmbChain) BestHeight() int32         { return c.height }
func (c *vmbChain) MedianTimePast() time.Time { return c.mtp }

// CalcSequenceLock follows BIP68 for outputs created in the tip block, using
// the tip's median time past for the block before it.
func (c *vmbChain) CalcSequenceLock(tx *bchutil.Tx,
	view *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {

	lock := &blockchain.SequenceLock{Seconds: -1, BlockHeight: -1}
	if tx.MsgTx().Version < 2 {
		return lock, nil
	}
	for _, txIn := range tx.MsgTx().TxIn {
		entry := view.LookupEntry(txIn.PreviousOutPoint)
		if entry == nil {
			return nil, errors.New("missing input")
		}
		seq := txIn.Sequence
		value := int64(seq & wire.SequenceLockTimeMask)
		switch {
		case seq&wire.SequenceLockTimeDisabled != 0:
		case seq&wire.SequenceLockTimeIsSeconds != 0:
			s := c.mtp.Unix() + value<<wire.SequenceLockTimeGranularity - 1
			lock.Seconds = max(lock.Seconds, s)
		default:
			h := entry.BlockHeight() + int32(value) - 1
			lock.BlockHeight = max(lock.BlockHeight, h)
		}
	}
	return lock, nil
}

// vmbBlockFlags returns the script flags of a mainnet block at the tip.
// Keep in sync with the flags set in BlockChain.checkConnectBlock.
func vmbBlockFlags(era vmbEra) txscript.ScriptFlags {
	flags := txscript.ScriptBip16 |
		txscript.ScriptVerifyDERSignatures |
		txscript.ScriptVerifyCheckLockTimeVerify |
		txscript.ScriptVerifyCheckSequenceVerify |
		txscript.ScriptVerifyStrictEncoding |
		txscript.ScriptVerifyBip143SigHash |
		txscript.ScriptVerifyLowS |
		txscript.ScriptVerifyNullFail |
		txscript.ScriptVerifySigPushOnly |
		txscript.ScriptVerifyCleanStack |
		txscript.ScriptVerifyCheckDataSig |
		txscript.ScriptVerifySchnorr |
		txscript.ScriptVerifyAllowSegwitRecovery |
		txscript.ScriptVerifyMinimalData |
		txscript.ScriptVerifySchnorrMultisig |
		txscript.ScriptReportSigChecks |
		txscript.ScriptVerifyReverseBytes |
		txscript.ScriptVerify64BitIntegers |
		txscript.ScriptVerifyNativeIntrospection
	if era.upgrade9 {
		flags |= txscript.ScriptAllowCashTokens
	}
	if era.upgrade11 {
		flags |= txscript.ScriptAllowMay2025
	}
	if era.upgrade12 {
		flags |= txscript.ScriptAllowMay2026
	}
	return flags
}

// vmbMempoolFlags returns the script flags the mempool uses at the tip.
func vmbMempoolFlags(era vmbEra) txscript.ScriptFlags {
	flags := txscript.StandardVerifyFlags
	if era.upgrade9 {
		flags |= txscript.ScriptAllowCashTokens
	}
	if era.upgrade11 {
		flags |= txscript.ScriptAllowMay2025 |
			txscript.ScriptAllowMay2025StandardOnly
	}
	if era.upgrade12 {
		flags |= txscript.ScriptAllowMay2026
	}
	return flags
}

// vmbCase is a decoded vector ready to run.
type vmbCase struct {
	tx    *bchutil.Tx
	chain *vmbChain
}

// decodeVMBVector decodes the transaction and its source outputs.  A
// transaction that does not decode is rejected by every check.
func decodeVMBVector(v *vmbVector, height int32) (*vmbCase, error) {
	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(v.tx)); err != nil {
		return nil, fmt.Errorf("decode: %v", err)
	}

	r := bytes.NewReader(v.utxos)
	count, err := wire.ReadVarInt(r, 0)
	if err != nil {
		return nil, fmt.Errorf("source outputs: %v", err)
	}
	if count != uint64(len(msgTx.TxIn)) {
		return nil, fmt.Errorf("source outputs: %d outputs for %d inputs",
			count, len(msgTx.TxIn))
	}
	if v.inputIndex >= len(msgTx.TxIn) {
		return nil, fmt.Errorf("input index %d out of range", v.inputIndex)
	}
	view := blockchain.NewUtxoViewpoint()
	for _, txIn := range msgTx.TxIn {
		var txOut wire.TxOut
		if _, err := wire.ReadTxOut(r, 0, 0, &txOut); err != nil {
			return nil, fmt.Errorf("source outputs: %v", err)
		}
		entry := blockchain.NewUtxoEntry(&txOut, height, false)
		view.Entries()[txIn.PreviousOutPoint] = entry
	}
	if r.Len() != 0 {
		return nil, errors.New("source outputs: trailing bytes")
	}

	return &vmbCase{
		tx:    bchutil.NewTx(&msgTx),
		chain: &vmbChain{utxos: view, height: height},
	}, nil
}

// vmbPolicy is the mempool policy of a default node, with these changes so
// that fees and priority do not decide the result:
//   - DisableRelayPriority is set, so free transactions need no priority.
//   - No maximum fee is passed to CheckMempoolAcceptance.
//   - A transaction rejected only for paying less than the minimum relay fee
//     is run again with a zero minimum relay fee.  The dust and other
//     standardness checks run before the fee check, so they use the default
//     fee in the first run.
//
// The other limits match the defaults in server.go, including MaxTxVersion.
func vmbPolicy(minRelayTxFee bchutil.Amount, acceptNonStd bool) Policy {
	return Policy{
		AcceptNonStd:         acceptNonStd,
		DisableRelayPriority: true,
		FreeTxRelayLimit:     15.0,
		MaxOrphanTxs:         100,
		MaxOrphanTxSize:      100000,
		LimitSigChecks:       true,
		MinRelayTxFee:        minRelayTxFee,
		MaxTxVersion:         2,
	}
}

// checkVMBMempool runs a transaction through CheckMempoolAcceptance, with
// AcceptNonStd set as for a node run with --relaynonstd if acceptNonStd is
// true.  It reports whether the minimum relay fee was waived.
func checkVMBMempool(c *vmbCase, era vmbEra, acceptNonStd bool) (bool, error) {
	run := func(minRelayTxFee bchutil.Amount) error {
		pool := New(&Config{
			Policy:           vmbPolicy(minRelayTxFee, acceptNonStd),
			ChainParams:      &chaincfg.MainNetParams,
			FetchUtxoView:    c.chain.FetchUtxoView,
			BestHeight:       c.chain.BestHeight,
			MedianTimePast:   c.chain.MedianTimePast,
			CalcSequenceLock: c.chain.CalcSequenceLock,
		})
		result, err := pool.CheckMempoolAcceptance(c.tx, 0)
		if err == nil && len(result.MissingParents) > 0 {
			err = errors.New("missing inputs")
		}
		return err
	}

	err := run(DefaultMinRelayTxFee)
	var rerr RuleError
	if errors.As(err, &rerr) {
		var terr TxRuleError
		if errors.As(rerr.Err, &terr) &&
			terr.RejectCode == wire.RejectInsufficientFee &&
			strings.Contains(terr.Description, "under the required amount") {

			return true, run(0)
		}
	}
	return false, err
}

// checkVMBBlock runs the checks a block applies to a transaction that spends
// outputs of an earlier block.
func checkVMBBlock(c *vmbCase, era vmbEra) error {
	params := &chaincfg.MainNetParams
	flags := vmbBlockFlags(era)
	nextHeight := c.chain.height + 1
	tx := c.tx

	err := blockchain.CheckTransactionSanity(tx, true, era.upgrade9, flags)
	if err != nil {
		return err
	}
	if blockchain.IsCoinBase(tx) {
		return errors.New("coinbase")
	}
	if !blockchain.IsFinalizedTransaction(tx, nextHeight, era.mtp) {
		return errors.New("not final")
	}
	view, _ := c.chain.FetchUtxoView(tx)
	lock, err := c.chain.CalcSequenceLock(tx, view)
	if err != nil {
		return err
	}
	if !blockchain.SequenceLockActive(lock, nextHeight, era.mtp) {
		return errors.New("sequence locks not met")
	}
	if _, err := blockchain.CheckTransactionInputs(tx, nextHeight, view,
		params); err != nil {
		return err
	}
	_, err = blockchain.ValidateTransactionScripts(tx, view, flags, nil, nil,
		params.Upgrade9ForkHeight)
	return err
}

// runVMBInput evaluates one input of a transaction.
func runVMBInput(c *vmbCase, idx int, flags txscript.ScriptFlags) error {
	_, err := runVMBInputEngine(c, idx, flags)
	return err
}

// runVMBInputEngine evaluates one input of a transaction and returns the
// engine that ran it.
func runVMBInputEngine(c *vmbCase, idx int,
	flags txscript.ScriptFlags) (*txscript.Engine, error) {

	msgTx := c.tx.MsgTx()
	cache := txscript.NewUtxoCache()
	for i, txIn := range msgTx.TxIn {
		entry := c.chain.utxos.LookupEntry(txIn.PreviousOutPoint)
		cache.AddEntry(i, *wire.NewTxOut(entry.Amount(), entry.PkScript(),
			entry.TokenData()))
	}
	sigHashes := txscript.NewTxSigHashes(msgTx).WithUtxos(msgTx, cache)
	entry := c.chain.utxos.LookupEntry(msgTx.TxIn[idx].PreviousOutPoint)
	vm, err := txscript.NewEngine(entry.PkScript(), msgTx, idx, flags, nil,
		sigHashes, cache, entry.Amount())
	if err != nil {
		return nil, err
	}
	return vm, vm.Execute()
}

// checkVMBScriptOnly checks a script only vector.  The standard check is the
// transaction and input standardness checks plus the input under mempool
// flags, and the block check is the input under block flags.  Finality is not
// part of standardness here, so it is passed with a far height and time.
func checkVMBScriptOnly(c *vmbCase, v *vmbVector, era vmbEra) (error, error) {
	stdErr := checkTransactionStandard(c.tx, math.MaxInt32,
		time.Unix(math.MaxUint32+1, 0), DefaultMinRelayTxFee, 2, true,
		era.upgrade12)
	if stdErr == nil {
		stdErr = checkInputsStandard(c.tx, c.chain.utxos,
			vmbMempoolFlags(era))
	}
	if stdErr == nil {
		stdErr = runVMBInput(c, v.inputIndex, vmbMempoolFlags(era))
	}
	blockErr := runVMBInput(c, v.inputIndex, vmbBlockFlags(era))
	return stdErr, blockErr
}

// vmbResult is the outcome of running a vector.
type vmbResult struct {
	mempoolErr error
	blockErr   error

	// relayNonStdErr is the result of a mempool that accepts nonstandard
	// transactions.  It is not set for script only vectors.
	relayNonStdErr error
	relayNonStdRun bool

	feeWaived bool
}

// mismatch describes how a result differs from the vector's class, or
// returns "" when it matches.
func (r *vmbResult) mismatch(class vmbClass) string {
	accepted, valid := r.mempoolErr == nil, r.blockErr == nil
	var problems []string
	switch class {
	case vmbStandard:
		if !accepted {
			problems = append(problems, "mempool rejects: "+
				r.mempoolErr.Error())
		}
		if r.relayNonStdRun && r.relayNonStdErr != nil {
			problems = append(problems, "mempool with "+
				"nonstandard relay rejects: "+
				r.relayNonStdErr.Error())
		}
		if !valid {
			problems = append(problems, "block rejects: "+
				r.blockErr.Error())
		}
	case vmbNonStandard:
		if accepted {
			problems = append(problems, "mempool accepts")
		}
		if !valid {
			problems = append(problems, "block rejects: "+
				r.blockErr.Error())
		}
	case vmbInvalid:
		if accepted {
			problems = append(problems, "mempool accepts")
		}
		if r.relayNonStdRun && r.relayNonStdErr == nil {
			problems = append(problems, "mempool with "+
				"nonstandard relay accepts")
		}
		if valid {
			problems = append(problems, "block accepts")
		}
	}
	return strings.Join(problems, "; ")
}

func runVMBVector(v *vmbVector, era vmbEra) *vmbResult {
	c, err := decodeVMBVector(v, era.height)
	if err != nil {
		return &vmbResult{mempoolErr: err, blockErr: err}
	}
	c.chain.mtp = era.mtp
	if v.scriptOnly {
		stdErr, blockErr := checkVMBScriptOnly(c, v, era)
		return &vmbResult{mempoolErr: stdErr, blockErr: blockErr}
	}
	var res vmbResult
	res.feeWaived, res.mempoolErr = checkVMBMempool(c, era, false)
	_, res.relayNonStdErr = checkVMBMempool(c, era, true)
	res.relayNonStdRun = true
	res.blockErr = checkVMBBlock(c, era)
	return &res
}

// TestVMBAcceptance runs every VMB vector through CheckMempoolAcceptance and
// through the checks of block validation, under the rules of its upgrade.
// Standard vectors must pass both, nonstandard vectors must fail the mempool
// and pass the block checks, and invalid vectors must fail both.
func TestVMBAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping VMB vectors in short mode")
	}

	// Set BCHD_VMB_REPORT to log the result of every vector.
	report := os.Getenv("BCHD_VMB_REPORT") != ""
	for _, era := range vmbEras() {
		t.Run(era.name, func(t *testing.T) {
			vectors := loadVMBVectors(t, era.name)
			var feeWaived int
			counts := make(map[vmbClass]int)
			for i := range vectors {
				v := &vectors[i]
				key := era.name + "/" + v.id
				res := runVMBVector(v, era)
				counts[v.class]++
				if res.feeWaived {
					feeWaived++
				}
				if report {
					t.Logf("RESULT\t%s\t%s\t%s\t%v\t%v\t%v",
						key, v.class, v.file, res.mempoolErr,
						res.blockErr, res.relayNonStdErr)
				}
				if problem := res.mismatch(v.class); problem != "" {
					t.Errorf("%s (%s, %s): %s: %s", key,
						v.class, v.file, v.desc, problem)
				}
			}
			t.Logf("%s: %d vectors (%d standard, %d nonstandard, "+
				"%d invalid), %d with the relay fee waived", era.name,
				len(vectors), counts[vmbStandard],
				counts[vmbNonStandard], counts[vmbInvalid], feeWaived)
		})
	}
}

// vmbMetrics are the metrics of one evaluated input, as BCHN records them in
// libauth_expected_test_metrics.json.  A limit of -1 means none applies.
type vmbMetrics [7]int64

// loadVMBMetrics reads BCHN's expected metrics, keyed by era, id, the
// vector's class and the evaluation mode (S for standard, N for
// nonstandard).
func loadVMBMetrics(t *testing.T) map[string][]vmbMetrics {
	t.Helper()

	data, err := readMaybeGzip(filepath.Join(vmbTestDir,
		"libauth_expected_test_metrics.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	metrics := make(map[string][]vmbMetrics, len(rows))
	for _, row := range rows {
		key := fmt.Sprintf("%s/%s/%s/%s", row[0], row[1], row[2], row[3])
		for _, in := range row[4].([]any) {
			var m vmbMetrics
			for i, v := range in.([]any) {
				m[i] = -1
				if v != nil {
					m[i] = int64(v.(float64))
				}
			}
			metrics[key] = append(metrics[key], m)
		}
	}
	return metrics
}

// TestVMBMetrics checks the op cost, hash iterations and signature checks of
// every input BCHN records metrics for, in standard and nonstandard
// evaluation, against BCHN's expected metrics.
func TestVMBMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping VMB vectors in short mode")
	}

	expected := loadVMBMetrics(t)
	classLetters := map[vmbClass]string{vmbStandard: "S",
		vmbNonStandard: "N", vmbInvalid: "I"}
	for _, era := range vmbEras() {
		t.Run(era.name, func(t *testing.T) {
			var checked int
			for _, v := range loadVMBVectors(t, era.name) {
				c, err := decodeVMBVector(&v, era.height)
				if err != nil {
					continue
				}
				c.chain.mtp = era.mtp
				for _, eval := range []string{"S", "N"} {
					key := fmt.Sprintf("%s/%s/%s/%s", era.name, v.id,
						classLetters[v.class], eval)
					want, ok := expected[key]
					if !ok {
						continue
					}
					flags := vmbBlockFlags(era)
					if eval == "S" {
						flags = vmbMempoolFlags(era)
					}
					got := make([]vmbMetrics, 0, len(want))
					for _, w := range want {
						got = append(got, vmbInputMetrics(c,
							int(w[0]), flags))
					}
					checked++
					if fmt.Sprint(got) != fmt.Sprint(want) {
						t.Errorf("%s (%s): %s: got metrics %v, "+
							"want %v", key, v.file, v.desc, got,
							want)
					}
				}
			}
			t.Logf("%s: %d metrics checked", era.name, checked)
		})
	}
}

// vmbInputMetrics evaluates one input and returns its metrics.  The op cost
// fields are -2 when the input fails.
func vmbInputMetrics(c *vmbCase, idx int,
	flags txscript.ScriptFlags) vmbMetrics {

	vm, err := runVMBInputEngine(c, idx, flags)
	if err != nil {
		return vmbMetrics{int64(idx), -2, -2, -2, -2, -2, -2}
	}
	metrics := vm.GetMetrics()
	standard := flags.HasFlag(txscript.ScriptAllowMay2025StandardOnly)
	m := vmbMetrics{int64(idx), metrics.GetCompositeOPCost(standard), -1,
		metrics.GetHashDigestIterations(), -1, int64(vm.SigChecks()), -1}
	if flags.HasFlag(txscript.ScriptAllowMay2025) {
		m[2] = metrics.GetMaxOpCostLimit()
		m[4] = metrics.GetMaxDigestIterationLimit()
	}
	if flags.HasFlag(txscript.ScriptVerifyInputSigChecks) {
		scriptSig := c.tx.MsgTx().TxIn[idx].SignatureScript
		m[6] = int64(len(scriptSig)+60) / 43
	}
	return m
}
