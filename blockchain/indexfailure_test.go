// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// failingIndexer is an index manager whose connection and disconnection of a
// block fail with the passed errors.
type failingIndexer struct {
	connectErr, disconnectErr error
}

func (m *failingIndexer) Init(*BlockChain, <-chan struct{}) error {
	return nil
}

func (m *failingIndexer) ConnectBlock(database.Tx, *bchutil.Block, []SpentTxOut) error {
	return m.connectErr
}

func (m *failingIndexer) DisconnectBlock(database.Tx, *bchutil.Block, []SpentTxOut) error {
	return m.disconnectErr
}

// namedIndexError is the error an index manager fails with that names the
// index that failed.
type namedIndexError struct {
	name string
}

func (e namedIndexError) Error() string {
	return "the index broke"
}

func (e namedIndexError) IndexName() string {
	return e.name
}

// TestIndexFailureLog ensures a block whose connection or disconnection an
// index fails logs the failure of that index, which the index manager's error
// names, rather than a failure to write the chain state, and requests the
// process to shut down, without the advice a corrupt database gets, while an
// error naming no index is still logged as a failure to write the chain state.
func TestIndexFailureLog(t *testing.T) {
	for _, c := range []struct {
		name    string
		connect bool
		err     error
		want    string
	}{
		{"connecting, named", true, namedIndexError{"test index"},
			"Failed to update the test index: the index broke"},
		{"disconnecting, named", false, namedIndexError{"test index"},
			"Failed to update the test index: the index broke"},
		{"connecting, unnamed", true, errors.New("the index broke"),
			"Failed to write the chain state: the index broke"},
	} {
		params := chaincfg.RegressionNetParams
		db, err := database.Create("ffldb", filepath.Join(t.TempDir(), "db"), params.Net)
		if err != nil {
			t.Fatal(err)
		}
		indexer := &failingIndexer{}
		chain, err := New(&Config{
			DB:                 db,
			ChainParams:        &params,
			TimeSource:         NewMedianTime(),
			SigCache:           txscript.NewSigCache(10),
			IndexManager:       indexer,
			ExcessiveBlockSize: LegacyMaxBlockSize,
			MaxReorgDepth:      -1,
		})
		if err != nil {
			t.Fatal(err)
		}
		var logged bytes.Buffer
		saved := log
		UseLogger(bchlog.NewBackend(&logged).Logger("CHAN"))

		blocks := []*bchutil.Block{bchutil.NewBlock(params.GenesisBlock)}
		for h := int32(1); h <= 2; h++ {
			if c.connect && h == 2 {
				indexer.connectErr = c.err
			}
			block := procTestBlock(t, blocks[h-1].MsgBlock(), h-1,
				CalcBlockSubsidy(h, &params), params.PowLimitBits, nil)
			_, _, err := chain.ProcessBlock(block, BFNone)
			if err != nil && (!c.connect || h != 2) {
				t.Fatalf("%s: block %d: %v", c.name, h, err)
			}
			blocks = append(blocks, block)
		}
		if !c.connect {
			indexer.disconnectErr = c.err
			if err := chain.InvalidateBlock(blocks[2].Hash()); err == nil {
				t.Errorf("%s: the disconnection did not fail", c.name)
			}
		}
		UseLogger(saved)
		db.Close()

		if !strings.Contains(logged.String(), c.want) {
			t.Errorf("%s: log %q, want a line with %q", c.name, logged.String(),
				c.want)
		}
		if _, ok := c.err.(namedIndexError); ok &&
			(strings.Contains(logged.String(), "Failed to write the chain state") ||
				strings.Contains(logged.String(), "reindexchainstate")) {

			t.Errorf("%s: log %q names the chain state or advises a "+
				"re-index", c.name, logged.String())
		}
		select {
		case <-chain.RequestedProcessShutdown():
		default:
			t.Errorf("%s: no shutdown requested", c.name)
		}
	}
}
