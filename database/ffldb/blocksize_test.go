// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/database"
	"github.com/gcash/bchutil"
)

// TestPrunedBlockSize ensures the size of a block whose block file was deleted
// by pruning is still known, from the location the block index keeps, while
// reading the block fails as for a missing block file.
func TestPrunedBlockSize(t *testing.T) {
	pdb := createPruneTestDB(t, filepath.Join(t.TempDir(), "db"))
	defer pdb.Close()
	var blocks []*bchutil.Block
	for i := 1; i <= 4; i++ {
		block := pruneTestBlock(uint32(i), int32(i))
		storeTestBlock(t, pdb, block)
		blocks = append(blocks, block)
	}
	prunePruneTestDB(t, pdb, 3)
	pruneTestFiles(t, pdb, 4, 2, 3)

	err := pdb.View(func(tx database.Tx) error {
		for _, block := range blocks {
			_, err := tx.FetchBlock(block.Hash())
			var dbErr database.Error
			pruned := errors.As(err, &dbErr) &&
				dbErr.ErrorCode == database.ErrDriverSpecific &&
				errors.Is(dbErr.Err, os.ErrNotExist)
			if want := block.Height() < 3; pruned != want {
				t.Errorf("block %d: FetchBlock error %v, pruned %v, "+
					"want %v", block.Height(), err, pruned, want)
			}

			raw, err := block.Bytes()
			if err != nil {
				return err
			}
			size, err := tx.FetchBlockSize(block.Hash())
			if err != nil || size != uint32(len(raw)) {
				t.Errorf("block %d: FetchBlockSize %d, %v, want %d",
					block.Height(), size, err, len(raw))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestFetchBlockSizeCorruptLocation ensures a stored block length too short
// to hold a block header is reported as corruption instead of being taken as a
// size.
func TestFetchBlockSizeCorruptLocation(t *testing.T) {
	pdb := createPruneTestDB(t, filepath.Join(t.TempDir(), "db"))
	defer pdb.Close()
	block := pruneTestBlock(1, 1)
	storeTestBlock(t, pdb, block)

	for _, test := range []struct {
		blockLen uint32
		want     uint32
		corrupt  bool
	}{
		{blockHdrSize + 12, blockHdrSize, false},
		{blockHdrSize + 11, 0, true},
		{0, 0, true},
	} {
		err := pdb.Update(func(tx database.Tx) error {
			row := serializeBlockLoc(blockLocation{blockLen: test.blockLen})
			return tx.(*transaction).blockIdxBucket.Put(block.Hash()[:], row)
		})
		if err != nil {
			t.Fatal(err)
		}
		var size uint32
		err = pdb.View(func(tx database.Tx) error {
			var err error
			size, err = tx.FetchBlockSize(block.Hash())
			return err
		})
		if test.corrupt {
			checkDbError(t, "FetchBlockSize of a corrupt location", err,
				database.ErrCorruption)
			continue
		}
		if err != nil || size != test.want {
			t.Errorf("stored length %d: FetchBlockSize %d, %v, want %d",
				test.blockLen, size, err, test.want)
		}
	}
}

// TestFetchBlockRegionCorruptLocation ensures a stored block length shorter
// than the 12 bytes around a block in the block files leaves no region of the
// block to fetch, instead of the block's size wrapping around.
func TestFetchBlockRegionCorruptLocation(t *testing.T) {
	pdb := createPruneTestDB(t, filepath.Join(t.TempDir(), "db"))
	defer pdb.Close()
	block := pruneTestBlock(1, 1)
	storeTestBlock(t, pdb, block)

	err := pdb.Update(func(tx database.Tx) error {
		row := serializeBlockLoc(blockLocation{blockLen: 5})
		return tx.(*transaction).blockIdxBucket.Put(block.Hash()[:], row)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = pdb.View(func(tx database.Tx) error {
		region := database.BlockRegion{Hash: block.Hash(), Len: 1}
		got, err := tx.FetchBlockRegion(&region)
		if err == nil {
			t.Errorf("FetchBlockRegion: got %x, want an error", got)
		} else {
			checkDbError(t, "FetchBlockRegion", err,
				database.ErrBlockRegionInvalid)
		}
		_, err = tx.FetchBlockRegions([]database.BlockRegion{region})
		checkDbError(t, "FetchBlockRegions", err,
			database.ErrBlockRegionInvalid)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestStoreBlockOwnSerialization ensures a block whose bytes hold more than its
// serialization, as a block message whose payload has data after the last
// transaction, is stored as its own serialization, as Bitcoin Cash Node writes
// each block to disk (node/blockstorage.cpp:345-368), so that its stored size
// is its serialized size, while it is pending too.
func TestStoreBlockOwnSerialization(t *testing.T) {
	pdb := createPruneTestDB(t, filepath.Join(t.TempDir(), "db"))
	defer pdb.Close()
	block := pruneTestBlock(1, 1)
	want, err := block.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	padded := bchutil.NewBlockFromBlockAndBytes(block.MsgBlock(),
		append(bytes.Clone(want), 1, 2, 3, 4, 5))
	padded.SetHeight(block.Height())

	check := func(desc string, tx database.Tx) {
		t.Helper()
		got, err := tx.FetchBlock(block.Hash())
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: FetchBlock gave %d bytes, %v, want the %d-byte "+
				"serialization", desc, len(got), err, len(want))
		}
		size, err := tx.FetchBlockSize(block.Hash())
		if err != nil || size != uint32(len(want)) {
			t.Errorf("%s: FetchBlockSize %d, %v, want %d", desc, size,
				err, len(want))
		}
	}
	err = pdb.Update(func(tx database.Tx) error {
		if err := tx.StoreBlock(padded); err != nil {
			return err
		}
		check("pending block", tx)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = pdb.View(func(tx database.Tx) error {
		check("stored block", tx)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
