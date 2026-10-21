// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// TestSequenceLocksActive tests the SequenceLockActive function to ensure it
// works as expected in all possible combinations/scenarios.
func TestSequenceLocksActive(t *testing.T) {
	seqLock := func(h int32, s int64) *SequenceLock {
		return &SequenceLock{
			Seconds:     s,
			BlockHeight: h,
		}
	}

	tests := []struct {
		seqLock     *SequenceLock
		blockHeight int32
		mtp         time.Time

		want bool
	}{
		// Block based sequence lock with equal block height.
		{seqLock: seqLock(1000, -1), blockHeight: 1001, mtp: time.Unix(9, 0), want: true},

		// Time based sequence lock with mtp past the absolute time.
		{seqLock: seqLock(-1, 30), blockHeight: 2, mtp: time.Unix(31, 0), want: true},

		// Block based sequence lock with current height below seq lock block height.
		{seqLock: seqLock(1000, -1), blockHeight: 90, mtp: time.Unix(9, 0), want: false},

		// Time based sequence lock with current time before lock time.
		{seqLock: seqLock(-1, 30), blockHeight: 2, mtp: time.Unix(29, 0), want: false},

		// Block based sequence lock at the same height, so shouldn't yet be active.
		{seqLock: seqLock(1000, -1), blockHeight: 1000, mtp: time.Unix(9, 0), want: false},

		// Time based sequence lock with current time equal to lock time, so shouldn't yet be active.
		{seqLock: seqLock(-1, 30), blockHeight: 2, mtp: time.Unix(30, 0), want: false},
	}

	t.Logf("Running %d sequence locks tests", len(tests))
	for i, test := range tests {
		got := SequenceLockActive(test.seqLock,
			test.blockHeight, test.mtp)
		if got != test.want {
			t.Fatalf("SequenceLockActive #%d got %v want %v", i,
				got, test.want)
		}
	}
}

// TestCheckConnectBlockTemplate tests the CheckConnectBlockTemplate function to
// ensure it fails.
func TestCheckConnectBlockTemplate(t *testing.T) {
	// Create a new database and chain instance to run tests against.
	chain, teardownFunc, err := chainSetup("checkconnectblocktemplate",
		&chaincfg.MainNetParams)
	if err != nil {
		t.Errorf("Failed to setup chain instance: %v", err)
		return
	}
	defer teardownFunc()

	// Since we're not dealing with the real block chain, set the coinbase
	// maturity to 1.
	chain.TstSetCoinbaseMaturity(1)

	// Load up blocks such that there is a side chain.
	// (genesis block) -> 1 -> 2 -> 3 -> 4
	//                          \-> 3a
	testFiles := []string{
		"blk_0_to_4.dat.bz2",
		"blk_3A.dat.bz2",
	}

	var blocks []*bchutil.Block
	for _, file := range testFiles {
		blockTmp, err := loadBlocks(file)
		if err != nil {
			t.Fatalf("Error loading file: %v\n", err)
		}
		blocks = append(blocks, blockTmp...)
	}

	for i := 1; i <= 3; i++ {
		isMainChain, _, err := chain.ProcessBlock(blocks[i], BFNone)
		if err != nil {
			t.Fatalf("CheckConnectBlockTemplate: Received unexpected error "+
				"processing block %d: %v", i, err)
		}
		if !isMainChain {
			t.Fatalf("CheckConnectBlockTemplate: Expected block %d to connect "+
				"to main chain", i)
		}
	}

	// Block 3 should fail to connect since it's already inserted.
	err = chain.CheckConnectBlockTemplate(blocks[3])
	if err == nil {
		t.Fatal("CheckConnectBlockTemplate: Did not received expected error " +
			"on block 3")
	}

	// Block 4 should connect successfully to tip of chain.
	err = chain.CheckConnectBlockTemplate(blocks[4])
	if err != nil {
		t.Fatalf("CheckConnectBlockTemplate: Received unexpected error on "+
			"block 4: %v", err)
	}

	// Block 3a should fail to connect since does not build on chain tip.
	err = chain.CheckConnectBlockTemplate(blocks[5])
	if err == nil {
		t.Fatal("CheckConnectBlockTemplate: Did not received expected error " +
			"on block 3a")
	}

	// Block 4 should connect even if proof of work is invalid.
	invalidPowBlock := *blocks[4].MsgBlock()
	invalidPowBlock.Header.Nonce++
	err = chain.CheckConnectBlockTemplate(bchutil.NewBlock(&invalidPowBlock))
	if err != nil {
		t.Fatalf("CheckConnectBlockTemplate: Received unexpected error on "+
			"block 4 with bad nonce: %v", err)
	}

	// Invalid block building on chain tip should fail to connect.
	invalidBlock := *blocks[4].MsgBlock()
	invalidBlock.Header.Bits--
	err = chain.CheckConnectBlockTemplate(bchutil.NewBlock(&invalidBlock))
	if err == nil {
		t.Fatal("CheckConnectBlockTemplate: Did not received expected error " +
			"on block 4 with invalid difficulty bits")
	}
}

//nolint:unused // retained for future tests
func newTestBlock(base, tip *wire.MsgBlock, coinbaseSigOps, tx1SigOps, tx2SigOps int) (*wire.MsgBlock, error) {
	prevHash := tip.Header.BlockHash()
	prevMRoot := tip.Header.MerkleRoot
	easyBits := chaincfg.MainNetParams.PowLimitBits
	addSigOps := map[int]int{
		0: coinbaseSigOps - 1,
		1: tx1SigOps - 1,
		2: tx2SigOps - 1,
	}

	// make a new valid block with duplicate transactions of the base block
	dup := wire.NewMsgBlock(wire.NewBlockHeader(
		1, &prevHash, &prevMRoot, easyBits, 2))
	dup.Header.Timestamp = tip.Header.Timestamp.Add(time.Second)
	for i, tx := range base.Transactions {
		err := dup.AddTransaction(tx.Copy())
		if err != nil {
			return nil, err
		}
		// also add the target number of sigOps
		dupTxOut := dup.Transactions[i].TxOut[0]
		dupTxOut.PkScript = append(dupTxOut.PkScript,
			repeatScript(addSigOps[i], txscript.OP_CHECKSIG)...)
	}

	setValidMerkleRoot(dup)
	//solveBlock(&dup.Header)
	return dup, nil
}

// calcMerkleRoot recalculates the merkle root.
//
//nolint:unused // retained for future tests
func setValidMerkleRoot(b *wire.MsgBlock) {
	if len(b.Transactions) == 0 {
		b.Header.MerkleRoot = chainhash.Hash{}
	}

	utilTxns := make([]*bchutil.Tx, 0, len(b.Transactions))
	for _, tx := range b.Transactions {
		utilTxns = append(utilTxns, bchutil.NewTx(tx))
	}
	merkles := BuildMerkleTreeStore(utilTxns)
	b.Header.MerkleRoot = *merkles[len(merkles)-1]
}

// repeatScript repeats a finite set of op codes n times.
//
//nolint:unused // retained for future tests
func repeatScript(n int, newBytes ...byte) []byte {
	script := make([]byte, n*len(newBytes))
	for i := range n {
		offset := i * len(newBytes)
		for j := range newBytes {
			script[offset+j] = newBytes[j]
		}
	}
	return script
}

// TestCheckBlockSanity tests the CheckBlockSanity function to ensure it works
// as expected.
func TestCheckBlockSanity(t *testing.T) {
	powLimit := chaincfg.MainNetParams.PowLimit
	block := bchutil.NewBlock(&Block100000)
	timeSource := NewMedianTime()
	err := CheckBlockSanity(block, powLimit, timeSource, false, false)
	if err != nil {
		t.Errorf("CheckBlockSanity: %v", err)
	}

	// Ensure a block that has a timestamp with a precision higher than one
	// second fails.
	timestamp := block.MsgBlock().Header.Timestamp
	block.MsgBlock().Header.Timestamp = timestamp.Add(time.Nanosecond)
	err = CheckBlockSanity(block, powLimit, timeSource, true, false)
	if err == nil {
		t.Errorf("CheckBlockSanity: error is nil when it shouldn't be")
	}
}

// TestCheckHeaderProofOfWork tests the CheckHeaderProofOfWork function to
// ensure it accepts a valid header and rejects headers with a hash above the
// target, a target above the proof of work limit, or a zero target.
func TestCheckHeaderProofOfWork(t *testing.T) {
	powLimit := chaincfg.MainNetParams.PowLimit
	valid := Block100000.Header

	tests := []struct {
		name    string
		mutate  func(h *wire.BlockHeader)
		wantErr ErrorCode
		valid   bool
	}{{
		name:   "valid header",
		mutate: func(h *wire.BlockHeader) {},
		valid:  true,
	}, {
		name:    "hash above target",
		mutate:  func(h *wire.BlockHeader) { h.Nonce++ },
		wantErr: ErrHighHash,
	}, {
		name:    "target above pow limit",
		mutate:  func(h *wire.BlockHeader) { h.Bits = 0x207fffff },
		wantErr: ErrUnexpectedDifficulty,
	}, {
		name:    "zero target",
		mutate:  func(h *wire.BlockHeader) { h.Bits = 0 },
		wantErr: ErrUnexpectedDifficulty,
	}}

	for _, test := range tests {
		header := valid
		test.mutate(&header)
		err := CheckHeaderProofOfWork(&header, powLimit)
		if test.valid {
			if err != nil {
				t.Errorf("%s: unexpected error: %v", test.name,
					err)
			}
			continue
		}
		var rerr RuleError
		if !errors.As(err, &rerr) || rerr.ErrorCode != test.wantErr {
			t.Errorf("%s: got error %v, want error code %v",
				test.name, err, test.wantErr)
		}
	}
}

// TestCheckSerializedHeight tests the checkSerializedHeight function with
// various serialized heights and also does negative tests to ensure errors
// and handled properly.
func TestCheckSerializedHeight(t *testing.T) {
	// Create an empty coinbase template to be used in the tests below.
	coinbaseOutpoint := wire.NewOutPoint(&chainhash.Hash{}, math.MaxUint32)
	coinbaseTx := wire.NewMsgTx(1)
	coinbaseTx.AddTxIn(wire.NewTxIn(coinbaseOutpoint, nil))

	// Expected rule errors.
	missingHeightError := RuleError{
		ErrorCode: ErrMissingCoinbaseHeight,
	}
	badHeightError := RuleError{
		ErrorCode: ErrBadCoinbaseHeight,
	}

	tests := []struct {
		sigScript  []byte // Serialized data
		wantHeight int32  // Expected height
		err        error  // Expected error type
	}{
		// No serialized height length.
		{[]byte{}, 0, missingHeightError},
		// Serialized height length with no height bytes.
		{[]byte{0x02}, 0, missingHeightError},
		// Serialized height length with too few height bytes.
		{[]byte{0x02, 0x4a}, 0, missingHeightError},
		// Serialized height that needs 2 bytes to encode.
		{[]byte{0x02, 0x4a, 0x52}, 21066, nil},
		// Serialized height that needs 2 bytes to encode, but backwards
		// endianness.
		{[]byte{0x02, 0x4a, 0x52}, 19026, badHeightError},
		// Serialized height that needs 3 bytes to encode.
		{[]byte{0x03, 0x40, 0x0d, 0x03}, 200000, nil},
		// Serialized height that needs 3 bytes to encode, but backwards
		// endianness.
		{[]byte{0x03, 0x40, 0x0d, 0x03}, 1074594560, badHeightError},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		msgTx := coinbaseTx.Copy()
		msgTx.TxIn[0].SignatureScript = test.sigScript
		tx := bchutil.NewTx(msgTx)

		err := checkSerializedHeight(tx, test.wantHeight)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("checkSerializedHeight #%d wrong error type "+
				"got: %v <%T>, want: %T", i, err, err, test.err)
			continue
		}

		if rerr, ok := err.(RuleError); ok {
			trerr := test.err.(RuleError)
			if rerr.ErrorCode != trerr.ErrorCode {
				t.Errorf("checkSerializedHeight #%d wrong "+
					"error code got: %v, want: %v", i,
					rerr.ErrorCode, trerr.ErrorCode)
				continue
			}
		}
	}
}

// TestCheckSerializedHeightMinimal ensures the coinbase signature script must
// start with the minimal push of the block height.
func TestCheckSerializedHeightMinimal(t *testing.T) {
	type vector struct {
		height  int32
		minimal []byte
		bad     [][]byte
	}
	vectors := []vector{
		{0, []byte{0x00}, [][]byte{{0x01, 0x00}, {0x02, 0x00, 0x00}}},
		{17, []byte{0x01, 0x11}, [][]byte{{0x02, 0x11, 0x00}}},
		{127, []byte{0x01, 0x7f}, [][]byte{{0x02, 0x7f, 0x00}}},
		{128, []byte{0x02, 0x80, 0x00}, [][]byte{{0x01, 0x80},
			{0x04, 0x80, 0x00, 0x00, 0x00}}},
		{255, []byte{0x02, 0xff, 0x00}, [][]byte{{0x01, 0xff},
			{0x03, 0xff, 0x00, 0x00}}},
		{256, []byte{0x02, 0x00, 0x01}, [][]byte{{0x03, 0x00, 0x01, 0x00}}},
		{32767, []byte{0x02, 0xff, 0x7f}, [][]byte{{0x03, 0xff, 0x7f, 0x00}}},
		{32768, []byte{0x03, 0x00, 0x80, 0x00}, [][]byte{{0x02, 0x00, 0x80},
			{0x04, 0x00, 0x80, 0x00, 0x00}}},
		{8388607, []byte{0x03, 0xff, 0xff, 0x7f}, [][]byte{
			{0x04, 0xff, 0xff, 0x7f, 0x00}}},
		{8388608, []byte{0x04, 0x00, 0x00, 0x80, 0x00}, [][]byte{
			{0x03, 0x00, 0x00, 0x80}}},
		{950000, []byte{0x03, 0xf0, 0x7e, 0x0e}, [][]byte{
			{0x04, 0xf0, 0x7e, 0x0e, 0x00},
			{0x08, 0xf0, 0x7e, 0x0e, 0x00, 0x00, 0x00, 0x00, 0x00},
			{txscript.OP_PUSHDATA1, 0x03, 0xf0, 0x7e, 0x0e},
		}},
	}
	// Heights 1 to 16 must use OP_1 to OP_16, not a data push.
	for h := int32(1); h <= 16; h++ {
		vectors = append(vectors, vector{h,
			[]byte{byte(txscript.OP_1 - 1 + h)},
			[][]byte{{0x01, byte(h)}, {0x02, byte(h), 0x00}}})
	}

	check := func(sigScript []byte, height int32) error {
		msgTx := wire.NewMsgTx(1)
		msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{},
			math.MaxUint32), sigScript))
		return checkSerializedHeight(bchutil.NewTx(msgTx), height)
	}
	for _, v := range vectors {
		if err := check(v.minimal, v.height); err != nil {
			t.Errorf("height %d: minimal %x rejected: %v", v.height,
				v.minimal, err)
		}
		// Data after the height push is allowed.
		extra := append(append([]byte{}, v.minimal...), 0x04, 1, 2, 3, 4)
		if err := check(extra, v.height); err != nil {
			t.Errorf("height %d: %x rejected: %v", v.height, extra, err)
		}
		if err := check(v.minimal, v.height+1); err == nil {
			t.Errorf("height %d: %x accepted for height %d",
				v.height, v.minimal, v.height+1)
		}
		for _, bad := range v.bad {
			err := check(bad, v.height)
			rerr, ok := err.(RuleError)
			if !ok || (rerr.ErrorCode != ErrBadCoinbaseHeight &&
				rerr.ErrorCode != ErrMissingCoinbaseHeight) {

				t.Errorf("height %d: non-minimal %x: got %v, want "+
					"%v", v.height, bad, err, ErrBadCoinbaseHeight)
			}
		}
	}
}

// TestTxVersionRule ensures the transaction version range is enforced for
// every transaction in a block, the coinbase included, from the first block
// after the Upgrade9 fork height.
func TestTxVersionRule(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.Upgrade9ForkHeight = 2
	chain, teardownFunc, err := chainSetup("txversionrule", &params)
	if err != nil {
		t.Fatalf("Failed to setup chain instance: %v", err)
	}
	defer teardownFunc()
	chain.TstSetCoinbaseMaturity(1)

	// Pad transactions above the minimum transaction size.
	padding := wire.NewTxOut(0, append([]byte{txscript.OP_RETURN, 64},
		make([]byte, 64)...), wire.TokenData{})

	tip := params.GenesisBlock
	newCoinbase := func(height int32, version int32) *wire.MsgTx {
		cb := wire.NewMsgTx(version)
		cb.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Index: math.MaxUint32},
			SignatureScript:  []byte{txscript.OP_DATA_2, byte(height), 0},
			Sequence:         wire.MaxTxInSequenceNum,
		})
		cb.AddTxOut(wire.NewTxOut(CalcBlockSubsidy(height, &params),
			[]byte{txscript.OP_TRUE}, wire.TokenData{}))
		cb.AddTxOut(padding)
		return cb
	}
	newBlock := func(txs ...*wire.MsgTx) *bchutil.Block {
		height := chain.BestSnapshot().Height + 1
		ts := tip.Header.Timestamp.Add(10 * time.Minute)
		bits, err := chain.CalcNextRequiredDifficulty(ts)
		if err != nil {
			t.Fatalf("CalcNextRequiredDifficulty: %v", err)
		}
		msgBlock := &wire.MsgBlock{
			Header: wire.BlockHeader{
				Version:   4,
				PrevBlock: tip.BlockHash(),
				Timestamp: ts,
				Bits:      bits,
			},
			Transactions: txs,
		}
		utxs := make([]*bchutil.Tx, 0, len(txs))
		for _, tx := range txs {
			utxs = append(utxs, bchutil.NewTx(tx))
		}
		merkles := BuildMerkleTreeStore(utxs)
		msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
		target := CompactToBig(bits)
		for {
			hash := msgBlock.Header.BlockHash()
			if HashToBig(&hash).Cmp(target) <= 0 {
				break
			}
			msgBlock.Header.Nonce++
		}
		block := bchutil.NewBlock(msgBlock)
		block.SetHeight(height)
		return block
	}
	process := func(block *bchutil.Block) error {
		_, _, err := chain.ProcessBlock(block, BFNone)
		if err == nil {
			tip = block.MsgBlock()
		}
		return err
	}
	mustAccept := func(desc string, block *bchutil.Block) {
		t.Helper()
		if err := process(block); err != nil {
			t.Fatalf("%s: block at height %d rejected: %v", desc,
				block.Height(), err)
		}
	}
	mustReject := func(desc string, block *bchutil.Block) {
		t.Helper()
		err := process(block)
		rerr, ok := err.(RuleError)
		if !ok || rerr.ErrorCode != ErrBadTxVersion {
			t.Fatalf("%s: block at height %d: got %v, want %v",
				desc, block.Height(), err, ErrBadTxVersion)
		}
	}
	spend := func(prev *wire.MsgTx, version int32) *wire.MsgTx {
		tx := wire.NewMsgTx(version)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: prev.TxHash()},
			Sequence:         wire.MaxTxInSequenceNum,
		})
		tx.AddTxOut(wire.NewTxOut(prev.TxOut[0].Value-1000,
			[]byte{txscript.OP_TRUE}, wire.TokenData{}))
		tx.AddTxOut(padding)
		return tx
	}

	badVersions := []int32{0, 3, -1, math.MaxInt32, math.MinInt32}

	// Block 1 provides an output to spend.
	cb1 := newCoinbase(1, 1)
	mustAccept("block 1", newBlock(cb1))

	// Block 2 is at the fork height, so the rule is not active yet.
	cb2 := newCoinbase(2, 3)
	mustAccept("coinbase version 3 before activation",
		newBlock(cb2, spend(cb1, 3)))

	// From block 3 every transaction must have version 1 or 2.
	for _, version := range badVersions {
		desc := fmt.Sprintf("coinbase version %d", version)
		mustReject(desc, newBlock(newCoinbase(3, version)))

		desc = fmt.Sprintf("transaction version %d", version)
		mustReject(desc, newBlock(newCoinbase(3, 1), spend(cb2, version)))
	}
	mustAccept("coinbase version 2", newBlock(newCoinbase(3, 2),
		spend(cb2, 1)))
	mustAccept("coinbase version 1", newBlock(newCoinbase(4, 1)))
}

// Block100000 defines block 100,000 of the block chain.  It is used to
// test Block operations.
var Block100000 = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version: 1,
		PrevBlock: chainhash.Hash([32]byte{ // Make go vet happy.
			0x50, 0x12, 0x01, 0x19, 0x17, 0x2a, 0x61, 0x04,
			0x21, 0xa6, 0xc3, 0x01, 0x1d, 0xd3, 0x30, 0xd9,
			0xdf, 0x07, 0xb6, 0x36, 0x16, 0xc2, 0xcc, 0x1f,
			0x1c, 0xd0, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		}), // 000000000002d01c1fccc21636b607dfd930d31d01c3a62104612a1719011250
		MerkleRoot: chainhash.Hash([32]byte{ // Make go vet happy.
			0x66, 0x57, 0xa9, 0x25, 0x2a, 0xac, 0xd5, 0xc0,
			0xb2, 0x94, 0x09, 0x96, 0xec, 0xff, 0x95, 0x22,
			0x28, 0xc3, 0x06, 0x7c, 0xc3, 0x8d, 0x48, 0x85,
			0xef, 0xb5, 0xa4, 0xac, 0x42, 0x47, 0xe9, 0xf3,
		}), // f3e94742aca4b5ef85488dc37c06c3282295ffec960994b2c0d5ac2a25a95766
		Timestamp: time.Unix(1293623863, 0), // 2010-12-29 11:57:43 +0000 UTC
		Bits:      0x1b04864c,               // 453281356
		Nonce:     0x10572b0f,               // 274148111
	},
	Transactions: []*wire.MsgTx{
		{
			Version: 1,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{
						0x04, 0x4c, 0x86, 0x04, 0x1b, 0x02, 0x06, 0x02,
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0x12a05f200, // 5000000000
					PkScript: []byte{
						0x41, // OP_DATA_65
						0x04, 0x1b, 0x0e, 0x8c, 0x25, 0x67, 0xc1, 0x25,
						0x36, 0xaa, 0x13, 0x35, 0x7b, 0x79, 0xa0, 0x73,
						0xdc, 0x44, 0x44, 0xac, 0xb8, 0x3c, 0x4e, 0xc7,
						0xa0, 0xe2, 0xf9, 0x9d, 0xd7, 0x45, 0x75, 0x16,
						0xc5, 0x81, 0x72, 0x42, 0xda, 0x79, 0x69, 0x24,
						0xca, 0x4e, 0x99, 0x94, 0x7d, 0x08, 0x7f, 0xed,
						0xf9, 0xce, 0x46, 0x7c, 0xb9, 0xf7, 0xc6, 0x28,
						0x70, 0x78, 0xf8, 0x01, 0xdf, 0x27, 0x6f, 0xdf,
						0x84, // 65-byte signature
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
		{
			Version: 1,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash: chainhash.Hash([32]byte{ // Make go vet happy.
							0x03, 0x2e, 0x38, 0xe9, 0xc0, 0xa8, 0x4c, 0x60,
							0x46, 0xd6, 0x87, 0xd1, 0x05, 0x56, 0xdc, 0xac,
							0xc4, 0x1d, 0x27, 0x5e, 0xc5, 0x5f, 0xc0, 0x07,
							0x79, 0xac, 0x88, 0xfd, 0xf3, 0x57, 0xa1, 0x87,
						}), // 87a157f3fd88ac7907c05fc55e271dc4acdc5605d187d646604ca8c0e9382e03
						Index: 0,
					},
					SignatureScript: []byte{
						0x49, // OP_DATA_73
						0x30, 0x46, 0x02, 0x21, 0x00, 0xc3, 0x52, 0xd3,
						0xdd, 0x99, 0x3a, 0x98, 0x1b, 0xeb, 0xa4, 0xa6,
						0x3a, 0xd1, 0x5c, 0x20, 0x92, 0x75, 0xca, 0x94,
						0x70, 0xab, 0xfc, 0xd5, 0x7d, 0xa9, 0x3b, 0x58,
						0xe4, 0xeb, 0x5d, 0xce, 0x82, 0x02, 0x21, 0x00,
						0x84, 0x07, 0x92, 0xbc, 0x1f, 0x45, 0x60, 0x62,
						0x81, 0x9f, 0x15, 0xd3, 0x3e, 0xe7, 0x05, 0x5c,
						0xf7, 0xb5, 0xee, 0x1a, 0xf1, 0xeb, 0xcc, 0x60,
						0x28, 0xd9, 0xcd, 0xb1, 0xc3, 0xaf, 0x77, 0x48,
						0x01, // 73-byte signature
						0x41, // OP_DATA_65
						0x04, 0xf4, 0x6d, 0xb5, 0xe9, 0xd6, 0x1a, 0x9d,
						0xc2, 0x7b, 0x8d, 0x64, 0xad, 0x23, 0xe7, 0x38,
						0x3a, 0x4e, 0x6c, 0xa1, 0x64, 0x59, 0x3c, 0x25,
						0x27, 0xc0, 0x38, 0xc0, 0x85, 0x7e, 0xb6, 0x7e,
						0xe8, 0xe8, 0x25, 0xdc, 0xa6, 0x50, 0x46, 0xb8,
						0x2c, 0x93, 0x31, 0x58, 0x6c, 0x82, 0xe0, 0xfd,
						0x1f, 0x63, 0x3f, 0x25, 0xf8, 0x7c, 0x16, 0x1b,
						0xc6, 0xf8, 0xa6, 0x30, 0x12, 0x1d, 0xf2, 0xb3,
						0xd3, // 65-byte pubkey
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0x2123e300, // 556000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0xc3, 0x98, 0xef, 0xa9, 0xc3, 0x92, 0xba, 0x60,
						0x13, 0xc5, 0xe0, 0x4e, 0xe7, 0x29, 0x75, 0x5e,
						0xf7, 0xf5, 0x8b, 0x32,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
				{
					Value: 0x108e20f00, // 4444000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0x94, 0x8c, 0x76, 0x5a, 0x69, 0x14, 0xd4, 0x3f,
						0x2a, 0x7a, 0xc1, 0x77, 0xda, 0x2c, 0x2f, 0x6b,
						0x52, 0xde, 0x3d, 0x7c,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
		{
			Version: 1,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash: chainhash.Hash([32]byte{ // Make go vet happy.
							0xc3, 0x3e, 0xbf, 0xf2, 0xa7, 0x09, 0xf1, 0x3d,
							0x9f, 0x9a, 0x75, 0x69, 0xab, 0x16, 0xa3, 0x27,
							0x86, 0xaf, 0x7d, 0x7e, 0x2d, 0xe0, 0x92, 0x65,
							0xe4, 0x1c, 0x61, 0xd0, 0x78, 0x29, 0x4e, 0xcf,
						}), // cf4e2978d0611ce46592e02d7e7daf8627a316ab69759a9f3df109a7f2bf3ec3
						Index: 1,
					},
					SignatureScript: []byte{
						0x47, // OP_DATA_71
						0x30, 0x44, 0x02, 0x20, 0x03, 0x2d, 0x30, 0xdf,
						0x5e, 0xe6, 0xf5, 0x7f, 0xa4, 0x6c, 0xdd, 0xb5,
						0xeb, 0x8d, 0x0d, 0x9f, 0xe8, 0xde, 0x6b, 0x34,
						0x2d, 0x27, 0x94, 0x2a, 0xe9, 0x0a, 0x32, 0x31,
						0xe0, 0xba, 0x33, 0x3e, 0x02, 0x20, 0x3d, 0xee,
						0xe8, 0x06, 0x0f, 0xdc, 0x70, 0x23, 0x0a, 0x7f,
						0x5b, 0x4a, 0xd7, 0xd7, 0xbc, 0x3e, 0x62, 0x8c,
						0xbe, 0x21, 0x9a, 0x88, 0x6b, 0x84, 0x26, 0x9e,
						0xae, 0xb8, 0x1e, 0x26, 0xb4, 0xfe, 0x01,
						0x41, // OP_DATA_65
						0x04, 0xae, 0x31, 0xc3, 0x1b, 0xf9, 0x12, 0x78,
						0xd9, 0x9b, 0x83, 0x77, 0xa3, 0x5b, 0xbc, 0xe5,
						0xb2, 0x7d, 0x9f, 0xff, 0x15, 0x45, 0x68, 0x39,
						0xe9, 0x19, 0x45, 0x3f, 0xc7, 0xb3, 0xf7, 0x21,
						0xf0, 0xba, 0x40, 0x3f, 0xf9, 0x6c, 0x9d, 0xee,
						0xb6, 0x80, 0xe5, 0xfd, 0x34, 0x1c, 0x0f, 0xc3,
						0xa7, 0xb9, 0x0d, 0xa4, 0x63, 0x1e, 0xe3, 0x95,
						0x60, 0x63, 0x9d, 0xb4, 0x62, 0xe9, 0xcb, 0x85,
						0x0f, // 65-byte pubkey
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0xf4240, // 1000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0xb0, 0xdc, 0xbf, 0x97, 0xea, 0xbf, 0x44, 0x04,
						0xe3, 0x1d, 0x95, 0x24, 0x77, 0xce, 0x82, 0x2d,
						0xad, 0xbe, 0x7e, 0x10,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
				{
					Value: 0x11d260c0, // 299000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0x6b, 0x12, 0x81, 0xee, 0xc2, 0x5a, 0xb4, 0xe1,
						0xe0, 0x79, 0x3f, 0xf4, 0xe0, 0x8a, 0xb1, 0xab,
						0xb3, 0x40, 0x9c, 0xd9,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
		{
			Version: 1,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash: chainhash.Hash([32]byte{ // Make go vet happy.
							0x0b, 0x60, 0x72, 0xb3, 0x86, 0xd4, 0xa7, 0x73,
							0x23, 0x52, 0x37, 0xf6, 0x4c, 0x11, 0x26, 0xac,
							0x3b, 0x24, 0x0c, 0x84, 0xb9, 0x17, 0xa3, 0x90,
							0x9b, 0xa1, 0xc4, 0x3d, 0xed, 0x5f, 0x51, 0xf4,
						}), // f4515fed3dc4a19b90a317b9840c243bac26114cf637522373a7d486b372600b
						Index: 0,
					},
					SignatureScript: []byte{
						0x49, // OP_DATA_73
						0x30, 0x46, 0x02, 0x21, 0x00, 0xbb, 0x1a, 0xd2,
						0x6d, 0xf9, 0x30, 0xa5, 0x1c, 0xce, 0x11, 0x0c,
						0xf4, 0x4f, 0x7a, 0x48, 0xc3, 0xc5, 0x61, 0xfd,
						0x97, 0x75, 0x00, 0xb1, 0xae, 0x5d, 0x6b, 0x6f,
						0xd1, 0x3d, 0x0b, 0x3f, 0x4a, 0x02, 0x21, 0x00,
						0xc5, 0xb4, 0x29, 0x51, 0xac, 0xed, 0xff, 0x14,
						0xab, 0xba, 0x27, 0x36, 0xfd, 0x57, 0x4b, 0xdb,
						0x46, 0x5f, 0x3e, 0x6f, 0x8d, 0xa1, 0x2e, 0x2c,
						0x53, 0x03, 0x95, 0x4a, 0xca, 0x7f, 0x78, 0xf3,
						0x01, // 73-byte signature
						0x41, // OP_DATA_65
						0x04, 0xa7, 0x13, 0x5b, 0xfe, 0x82, 0x4c, 0x97,
						0xec, 0xc0, 0x1e, 0xc7, 0xd7, 0xe3, 0x36, 0x18,
						0x5c, 0x81, 0xe2, 0xaa, 0x2c, 0x41, 0xab, 0x17,
						0x54, 0x07, 0xc0, 0x94, 0x84, 0xce, 0x96, 0x94,
						0xb4, 0x49, 0x53, 0xfc, 0xb7, 0x51, 0x20, 0x65,
						0x64, 0xa9, 0xc2, 0x4d, 0xd0, 0x94, 0xd4, 0x2f,
						0xdb, 0xfd, 0xd5, 0xaa, 0xd3, 0xe0, 0x63, 0xce,
						0x6a, 0xf4, 0xcf, 0xaa, 0xea, 0x4e, 0xa1, 0x4f,
						0xbb, // 65-byte pubkey
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0xf4240, // 1000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0x39, 0xaa, 0x3d, 0x56, 0x9e, 0x06, 0xa1, 0xd7,
						0x92, 0x6d, 0xc4, 0xbe, 0x11, 0x93, 0xc9, 0x9b,
						0xf2, 0xeb, 0x9e, 0xe0,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
	},
}

// TestCoinbaseTokenRule ensures a coinbase output with a token prefix is
// accepted at the Upgrade9 fork height and rejected after it.
func TestCoinbaseTokenRule(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.Upgrade9ForkHeight = 2
	chain, teardownFunc, err := chainSetup("coinbasetokenrule", &params)
	if err != nil {
		t.Fatalf("Failed to setup chain instance: %v", err)
	}
	defer teardownFunc()

	// Pad transactions above the minimum transaction size.
	padding := wire.NewTxOut(0, append([]byte{txscript.OP_RETURN, 64},
		make([]byte, 64)...), wire.TokenData{})

	tip := params.GenesisBlock
	newCoinbase := func(height int32, extra *wire.TxOut) *wire.MsgTx {
		cb := wire.NewMsgTx(1)
		cb.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Index: math.MaxUint32},
			SignatureScript:  []byte{txscript.OP_DATA_2, byte(height), 0},
			Sequence:         wire.MaxTxInSequenceNum,
		})
		cb.AddTxOut(wire.NewTxOut(CalcBlockSubsidy(height, &params),
			[]byte{txscript.OP_TRUE}, wire.TokenData{}))
		cb.AddTxOut(padding)
		if extra != nil {
			cb.AddTxOut(extra)
		}
		return cb
	}
	newBlock := func(cb *wire.MsgTx) *bchutil.Block {
		height := chain.BestSnapshot().Height + 1
		ts := tip.Header.Timestamp.Add(10 * time.Minute)
		bits, err := chain.CalcNextRequiredDifficulty(ts)
		if err != nil {
			t.Fatalf("CalcNextRequiredDifficulty: %v", err)
		}
		msgBlock := &wire.MsgBlock{
			Header: wire.BlockHeader{
				Version:    4,
				PrevBlock:  tip.BlockHash(),
				MerkleRoot: cb.TxHash(),
				Timestamp:  ts,
				Bits:       bits,
			},
			Transactions: []*wire.MsgTx{cb},
		}
		target := CompactToBig(bits)
		for {
			hash := msgBlock.Header.BlockHash()
			if HashToBig(&hash).Cmp(target) <= 0 {
				break
			}
			msgBlock.Header.Nonce++
		}
		block := bchutil.NewBlock(msgBlock)
		block.SetHeight(height)
		return block
	}
	process := func(block *bchutil.Block) error {
		_, _, err := chain.ProcessBlock(block, BFNone)
		if err == nil {
			tip = block.MsgBlock()
		}
		return err
	}
	mustAccept := func(desc string, block *bchutil.Block) {
		t.Helper()
		if err := process(block); err != nil {
			t.Fatalf("%s: block at height %d rejected: %v", desc,
				block.Height(), err)
		}
	}
	mustReject := func(desc string, block *bchutil.Block) {
		t.Helper()
		err := process(block)
		rerr, ok := err.(RuleError)
		if !ok || rerr.ErrorCode != ErrCoinbaseHasTokens {
			t.Fatalf("%s: block at height %d: got %v, want %v",
				desc, block.Height(), err, ErrCoinbaseHasTokens)
		}
	}

	amount := uint64(1000)
	tokenData, err := wire.NewTokenData([32]byte{1}, &amount, nil, nil)
	if err != nil {
		t.Fatalf("NewTokenData: %v", err)
	}
	tokenOutput := wire.NewTxOut(0, []byte{txscript.OP_TRUE}, *tokenData)
	// A token prefix byte that does not start valid token data.
	badPrefixOutput := wire.NewTxOut(0,
		[]byte{wire.PREFIX_BYTE, txscript.OP_TRUE}, wire.TokenData{})

	mustAccept("block 1", newBlock(newCoinbase(1, nil)))

	// Block 2 is at the fork height, so the rule is not active yet.
	mustAccept("token output before activation",
		newBlock(newCoinbase(2, tokenOutput)))

	// From block 3 the coinbase must not have a token prefix.
	mustReject("token output", newBlock(newCoinbase(3, tokenOutput)))
	mustReject("invalid token prefix",
		newBlock(newCoinbase(3, badPrefixOutput)))
	mustAccept("no token output", newBlock(newCoinbase(3, nil)))
}

// TestTokenValidityFailureMarksBlockInvalid ensures a block with a transaction
// that fails the token validity algorithm is rejected with
// ErrCashTokensValidation and marked invalid, so it is known to be invalid
// when it arrives again.
func TestTokenValidityFailureMarksBlockInvalid(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.MagneticAnonomalyForkHeight = 0
	params.Upgrade9ForkHeight = 0
	m := newRuleTestMiner(t, &params)

	b1 := m.build(nil)
	m.mustAccept(b1)
	cb := b1.MsgBlock().Transactions[0]
	prevOut := wire.OutPoint{Hash: cb.TxHash(), Index: 0}

	// An NFT of a category that no input creates or carries.
	capability := byte(wire.NONE)
	nft, err := wire.NewTokenData([32]byte{1}, nil, nil, &capability)
	if err != nil {
		t.Fatalf("NewTokenData: %v", err)
	}
	tx := ruleTestSpend(prevOut, cb.TxOut[0].Value-1000, 1)
	tx.TxOut[0].TokenData = *nft

	block := m.build(nil, tx)
	err = m.accept(block)
	requireRuleError(t, err, ErrCashTokensValidation)
	known, _, invalid := m.chain.BlockValidity(block.Hash())
	if !known || !invalid {
		t.Fatalf("block known %v, invalid %v, want both", known, invalid)
	}
}

// TestRegtestP2SHBeforeActivationTime ensures that on regtest, where the
// Magnetic Anomaly rules apply from the first block, the pay-to-script-hash
// rules are enforced in blocks timestamped before txscript.Bip16Activation,
// so a redeem script that evaluates to false is rejected while one that
// evaluates to true is accepted.
func TestRegtestP2SHBeforeActivationTime(t *testing.T) {
	params := chaincfg.RegressionNetParams
	m := newRuleTestMiner(t, &params)

	// build returns a block like the miner's with a timestamp ten minutes
	// after the tip, which on regtest is before the activation time.
	build := func(txs ...*wire.MsgTx) *bchutil.Block {
		blk := m.build(nil, txs...).MsgBlock()
		blk.Header.Timestamp = m.tip.Header.Timestamp.Add(10 * time.Minute)
		if !blk.Header.Timestamp.Before(txscript.Bip16Activation) {
			t.Fatalf("block timestamp %v is not before %v",
				blk.Header.Timestamp, txscript.Bip16Activation)
		}
		return m.solve(blk, m.height+1)
	}

	b1 := build()
	m.mustAccept(b1)
	cb := b1.MsgBlock().Transactions[0]
	value := cb.TxOut[0].Value - 1000

	// Pay to two script hashes, one of a redeem script that leaves false
	// on the stack and one of a redeem script that leaves true.
	falseRedeem := []byte{txscript.OP_FALSE}
	trueRedeem := []byte{txscript.OP_TRUE}
	p2sh := func(redeem []byte) []byte {
		script, err := txscript.NewScriptBuilder().AddOp(txscript.OP_HASH160).
			AddData(bchutil.Hash160(redeem)).AddOp(txscript.OP_EQUAL).Script()
		if err != nil {
			t.Fatalf("failed to build script: %v", err)
		}
		return script
	}
	fund := ruleTestSpend(wire.OutPoint{Hash: cb.TxHash(), Index: 0}, value/2, 1)
	fund.TxOut[0].PkScript = p2sh(falseRedeem)
	fund.AddTxOut(wire.NewTxOut(value/2, p2sh(trueRedeem), wire.TokenData{}))
	m.mustAccept(build(fund))

	spend := func(index uint32, redeem []byte, tag byte) *wire.MsgTx {
		sigScript, err := txscript.NewScriptBuilder().AddData(redeem).Script()
		if err != nil {
			t.Fatalf("failed to build script: %v", err)
		}
		prevOut := wire.OutPoint{Hash: fund.TxHash(), Index: index}
		tx := ruleTestSpend(prevOut, value/2-1000, tag)
		tx.TxIn[0].SignatureScript = sigScript
		return tx
	}
	err := m.accept(build(spend(0, falseRedeem, 2)))
	requireRuleError(t, err, ErrScriptValidation)
	m.mustAccept(build(spend(2, trueRedeem, 3)))
}

// TestCheckBlockHeaderContextParent ensures a header is checked against its
// own parent, which must be known and not known to be invalid.
func TestCheckBlockHeaderContextParent(t *testing.T) {
	params := &chaincfg.RegressionNetParams
	chain := newFakeChain(params)
	invalid := newFakeNode(chain.bestChain.Tip(), 1, params.PowLimitBits,
		time.Now())
	chain.index.AddNode(invalid)
	chain.index.SetStatusFlags(invalid, statusValidateFailed)

	header := func(prev chainhash.Hash) *wire.BlockHeader {
		header := &wire.BlockHeader{
			Version:   1,
			PrevBlock: prev,
			Bits:      params.PowLimitBits,
			Timestamp: time.Unix(time.Now().Unix(), 0),
		}
		for CheckHeaderProofOfWork(header, params.PowLimit) != nil {
			header.Nonce++
		}
		return header
	}
	tests := []struct {
		name string
		prev chainhash.Hash
		want ErrorCode
	}{
		{"unknown parent", chainhash.Hash{1}, ErrPreviousBlockUnknown},
		{"invalid parent", invalid.hash, ErrInvalidAncestorBlock},
	}
	for _, test := range tests {
		err := chain.CheckBlockHeaderContext(header(test.prev))
		var rerr RuleError
		if !errors.As(err, &rerr) || rerr.ErrorCode != test.want {
			t.Errorf("%s: got %v, want %v", test.name, err, test.want)
		}
	}
}
