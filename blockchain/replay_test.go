// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// This file is ignored during the regular tests due to the following build tag.
//go:build replay

package blockchain_test

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchutil"
)

// tempnetActivationTime is the upgrade 2027 activation time of tempnet, the
// public test chain of CHIP-2025-03 that forked off chipnet at block 327,989.
const tempnetActivationTime = 1792065600

// tempnetActivationBlock is the activation block of tempnet, its last 600-tick
// block.
const tempnetActivationBlock = 327988

// tempnetSubsidy is the sum of the subsidies of tempnet blocks 327,900 to
// 349,482, as the CHIP-2025-03 reference replay measured it.
const (
	tempnetSubsidyFrom = 327900
	tempnetSubsidyTo   = 349482
	tempnetSubsidy     = 5596000000000
)

// readReplayRows reads tab separated files, whose first two columns are the
// height and the hash of a block, into a map of rows by height.  A first line
// that does not start with a height names the columns.  A height in more than
// one file has the columns of all of them.
func readReplayRows(paths string) (map[int32]map[string]string, error) {
	rows := make(map[int32]map[string]string)
	for _, path := range filepath.SplitList(paths) {
		if err := readReplayFile(path, rows); err != nil {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
	}
	return rows, nil
}

// readReplayFile adds the rows of a tab separated file to rows.
func readReplayFile(path string, rows map[int32]map[string]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	names := []string{"height", "hash"}
	first := true
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		cols := strings.Split(strings.TrimSpace(scanner.Text()), "\t")
		if len(cols) < 2 {
			continue
		}
		height, err := strconv.ParseInt(cols[0], 10, 32)
		if err != nil {
			if first {
				names = cols
				first = false
				continue
			}
			return fmt.Errorf("bad height %q", cols[0])
		}
		first = false
		row := rows[int32(height)]
		if row == nil {
			row = make(map[string]string)
			rows[int32(height)] = row
		}
		for i := 1; i < len(cols) && i < len(names); i++ {
			if old, ok := row[names[i]]; ok && old != cols[i] {
				return fmt.Errorf("block %d has %s %s and %s",
					height, names[i], old, cols[i])
			}
			row[names[i]] = cols[i]
		}
	}
	return scanner.Err()
}

// TestReplayTempnet replays a chain in bootstrap.dat format, each block as the
// network magic, its little-endian size and the block, through ProcessBlock
// with chipnet parameters and the tempnet activation time, from genesis.  It
// checks that every block joins the main chain, that tempnet activates the
// upgrade at its activation block, the subsidies of the blocks that pay no
// fees, and the values of the files of expected values that are given.
//
// It is skipped unless the BCHD_TEMPNET_DUMP environment variable names the
// file.  BCHD_TEMPNET_HASHES may name tab separated files of heights and
// hashes, such as the hashes.tsv and .hdr.tsv files of a capture, joined like
// a PATH.  Their mediantime, ticks, target_time and next_target_time columns
// are checked too.  BCHD_TEMPNET_ABLA may name the .abla.tsv file of a
// dump_chain.py capture, whose ABLA states and block size limits are checked.
// BCHD_TEMPNET_STOP may name a height after which the replay stops.  Blocks
// up to the last checkpoint are added as a syncing node adds them, unless
// BCHD_TEMPNET_FULL is set.
//
//	BCHD_TEMPNET_DUMP=bootstrap.dat go test -tags replay -run TestReplayTempnet -timeout 0 ./blockchain/
func TestReplayTempnet(t *testing.T) {
	dumpPath := os.Getenv("BCHD_TEMPNET_DUMP")
	if dumpPath == "" {
		t.Skip("BCHD_TEMPNET_DUMP is not set")
	}
	var rows, ablaRows map[int32]map[string]string
	if paths := os.Getenv("BCHD_TEMPNET_HASHES"); paths != "" {
		var err error
		rows, err = readReplayRows(paths)
		if err != nil {
			t.Fatalf("unable to read the hashes: %v", err)
		}
	}
	if path := os.Getenv("BCHD_TEMPNET_ABLA"); path != "" {
		ablaRows = make(map[int32]map[string]string)
		if err := readReplayFile(path, ablaRows); err != nil {
			t.Fatalf("unable to read %s: %v", path, err)
		}
	}
	stop := int32(1<<31 - 1)
	if s := os.Getenv("BCHD_TEMPNET_STOP"); s != "" {
		height, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			t.Fatalf("bad BCHD_TEMPNET_STOP %q: %v", s, err)
		}
		stop = int32(height)
	}
	full := os.Getenv("BCHD_TEMPNET_FULL") != ""

	params := chaincfg.ChipNetParams
	params.Upgrade2027ActivationTime = tempnetActivationTime
	activationTime := uint64(tempnetActivationTime)
	lastCheckpoint := params.Checkpoints[len(params.Checkpoints)-1].Height

	db, err := database.Create(testDbType,
		filepath.Join(t.TempDir(), "replay"), params.Net)
	if err != nil {
		t.Fatalf("unable to create database: %v", err)
	}
	defer db.Close()

	// The chain is pruned so the replay takes little disk.
	chain, err := blockchain.New(&blockchain.Config{
		DB:                        db,
		ChainParams:               &params,
		Checkpoints:               params.Checkpoints,
		TimeSource:                blockchain.NewMedianTime(),
		SigCache:                  txscript.NewSigCache(100000),
		HashCache:                 txscript.NewHashCache(100000),
		UtxoCacheMaxSize:          250 * 1024 * 1024,
		ExcessiveBlockSize:        32000000,
		Prune:                     true,
		PruneDepth:                10000,
		Upgrade2027ActivationTime: &activationTime,
	})
	if err != nil {
		t.Fatalf("unable to create chain: %v", err)
	}

	f, err := os.Open(dumpPath)
	if err != nil {
		t.Fatalf("unable to open %s: %v", dumpPath, err)
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)

	if row, ok := rows[0]; ok && row["hash"] != params.GenesisHash.String() {
		t.Fatalf("block 0 is %s, want %s", params.GenesisHash,
			row["hash"])
	}

	start := time.Now()
	height := int32(0)
	checked, ablaChecked, feeless := 0, 0, 0
	var subsidy int64
	for height < stop {
		var frame [8]byte
		if _, err := io.ReadFull(r, frame[:]); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("after block %d: %v", height, err)
		}
		if magic := binary.LittleEndian.Uint32(frame[:4]); magic != uint32(params.Net) {
			t.Fatalf("after block %d: magic %08x, want %08x", height,
				magic, uint32(params.Net))
		}
		raw := make([]byte, binary.LittleEndian.Uint32(frame[4:]))
		if _, err := io.ReadFull(r, raw); err != nil {
			t.Fatalf("after block %d: %v", height, err)
		}
		block, err := bchutil.NewBlockFromBytes(raw)
		if err != nil {
			t.Fatalf("after block %d: %v", height, err)
		}
		if *block.Hash() == *params.GenesisHash {
			continue
		}
		height++
		block.SetHeight(height)

		flags := blockchain.BFNone
		if height <= lastCheckpoint && !full {
			flags |= blockchain.BFFastAdd
		}
		epsilon, beta := chain.TipABLAState()
		isMainChain, isOrphan, err := chain.ProcessBlock(block, flags)
		if err != nil {
			t.Fatalf("block %d %s: %v", height, block.Hash(), err)
		}
		if !isMainChain || isOrphan {
			t.Fatalf("block %d %s: main chain %v, orphan %v", height,
				block.Hash(), isMainChain, isOrphan)
		}

		// A block after the activation block with only a coinbase
		// pays the whole subsidy of its ticks, as tempnet miners claim
		// all of it, so a subsidy that is too high shows too.
		ticks, err := chain.BlockTicksByHash(block.Hash())
		if err != nil {
			t.Fatalf("block %d: %v", height, err)
		}
		blockSubsidy := blockchain.CalcBlockSubsidy(ticks.Ticks,
			ticks.Interval, &params)
		if height >= tempnetSubsidyFrom && height <= tempnetSubsidyTo {
			subsidy += blockSubsidy
		}
		if txs := block.MsgBlock().Transactions; len(txs) == 1 {
			var value int64
			for _, out := range txs[0].TxOut {
				value += out.Value
			}
			if value != blockSubsidy &&
				height > tempnetActivationBlock {

				t.Fatalf("block %d %s pays %d, want the subsidy "+
					"%d", height, block.Hash(), value,
					blockSubsidy)
			}
			feeless++
		}

		if row, ok := rows[height]; ok {
			checkReplayRow(t, chain, block, ticks, row)
			checked++
		}
		if row, ok := ablaRows[height]; ok {
			checkReplayABLA(t, chain, block, ticks, epsilon, beta,
				row)
			ablaChecked++
		}
		if height%20000 == 0 {
			t.Logf("block %d after %v", height,
				time.Since(start).Round(time.Second))
		}
	}

	best := chain.BestSnapshot()
	t.Logf("replayed to block %d %s in %v, %d blocks checked against "+
		"the hashes, %d against the ABLA states, %d paid the subsidy "+
		"alone", best.Height, best.Hash,
		time.Since(start).Round(time.Second), checked, ablaChecked,
		feeless)
	if best.Height == 0 {
		t.Fatal("the dump has no blocks after genesis")
	}

	// Every row up to the tip was checked.  Rows after it are only noted,
	// since a capture may write its files at slightly different tips.
	for name, set := range map[string]map[int32]map[string]string{
		"hashes": rows, "ABLA states": ablaRows} {

		want, after := 0, 0
		for h := range set {
			switch {
			case h > best.Height:
				after++
			case h > 0:
				want++
			}
		}
		count := map[string]int{"hashes": checked,
			"ABLA states": ablaChecked}[name]
		if count != want || (len(set) > 0 && count == 0) {
			t.Errorf("%d of the %d rows of the %s up to the tip "+
				"were checked", count, want, name)
		}
		if after > 0 {
			t.Logf("%d rows of the %s are after the tip", after,
				name)
		}
	}

	if best.Height >= tempnetSubsidyTo {
		if subsidy != tempnetSubsidy {
			t.Errorf("blocks %d to %d pay a subsidy of %d, want %d",
				tempnetSubsidyFrom, tempnetSubsidyTo, subsidy,
				int64(tempnetSubsidy))
		}
	}

	// The activation block is the first block with a next target time of
	// 60, which the hashes give when they have that column.
	want := int32(tempnetActivationBlock)
	for h, row := range rows {
		if row["next_target_time"] == "60" &&
			rows[h-1]["next_target_time"] == "600" {

			want = h
		}
	}
	_, got, ok := chain.Upgrade2027ActivationBlock()
	switch {
	case best.Height > want && (!ok || got != want):
		t.Errorf("activation block %d (active %v), want %d", got, ok,
			want)
	case best.Height <= want && ok:
		t.Errorf("activation block %d at tip %d, want %d", got,
			best.Height, want)
	}
}

// checkReplayRow checks a connected block, the chain tip, against its row of
// the hashes.
func checkReplayRow(t *testing.T, chain *blockchain.BlockChain,
	block *bchutil.Block, ticks blockchain.BlockTicks,
	row map[string]string) {

	t.Helper()
	height := block.Height()
	if want, ok := row["hash"]; ok && block.Hash().String() != want {
		t.Fatalf("block %d is %s, want %s", height, block.Hash(), want)
	}
	got := map[string]string{
		"mediantime": strconv.FormatInt(
			chain.BestSnapshot().MedianTime.Unix(), 10),
		"ticks":            strconv.FormatInt(ticks.Ticks, 10),
		"target_time":      strconv.FormatInt(ticks.Interval, 10),
		"next_target_time": strconv.FormatInt(ticks.NextInterval, 10),
	}
	for name, value := range got {
		if want, ok := row[name]; ok && want != value {
			t.Fatalf("block %d %s: %s %s, want %s", height,
				block.Hash(), name, value, want)
		}
	}
}

// checkReplayABLA checks a connected block, the chain tip, against its row of
// the ABLA states: the size limits of the block and the next one, and the
// control and elastic buffer sizes the block was limited by, those of the
// state after its parent.
func checkReplayABLA(t *testing.T, chain *blockchain.BlockChain,
	block *bchutil.Block, ticks blockchain.BlockTicks, epsilon,
	beta uint64, row map[string]string) {

	t.Helper()
	height := block.Height()
	limit, err := chain.NextBlockSizeLimit(&block.MsgBlock().Header.PrevBlock)
	if err != nil {
		t.Fatalf("block %d: %v", height, err)
	}
	nextLimit, err := chain.NextBlockSizeLimit(block.Hash())
	if err != nil {
		t.Fatalf("block %d: %v", height, err)
	}
	got := map[string]uint64{
		"epsilon":            epsilon,
		"beta":               beta,
		"blocksize":          uint64(block.MsgBlock().SerializeSize()),
		"blockinterval":      uint64(ticks.Interval),
		"blocksizelimit":     limit,
		"nextblockinterval":  uint64(ticks.NextInterval),
		"nextblocksizelimit": nextLimit,
	}
	for name, value := range got {
		want, ok := row[name]
		if ok && want != strconv.FormatUint(value, 10) {
			t.Fatalf("block %d %s: %s %d, want %s", height,
				block.Hash(), name, value, want)
		}
	}
}
