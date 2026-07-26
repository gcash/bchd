// Copyright (c) 2015-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gcash/bchd/database"
)

// TestWriteBlockRolloverWithClosedFile ensures a block that rolls the store
// over to the next flat file is written correctly when the current file is not
// already open.
//
// The rollover path writes the height of the final block to the end of the
// current file before advancing, and that write requires the current file to be
// open.  It can legitimately be closed at that point -- on initial database
// load, or after a rollback that happened once file writes had started during a
// transaction commit -- which previously caused a nil pointer dereference.
func TestWriteBlockRolloverWithClosedFile(t *testing.T) {
	dbPath := filepath.Join(os.TempDir(), "ffldb-writeblockrollover")
	_ = os.RemoveAll(dbPath)
	idb, err := database.Create(dbType, dbPath, blockDataNet)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer os.RemoveAll(dbPath)

	// NOTE: The database is deliberately closed at the end rather than with a
	// defer.  A regression here panics while the current file lock is held, so
	// a deferred close would block on that lock during panic unwinding and
	// hang the test instead of reporting the failure.

	// Back the store with in-memory mock files and shrink the maximum file
	// size so a modest block forces a rollover.
	files := make(map[uint32]*lockableFile)
	store := idb.(*db).store
	store.maxBlockFileSize = 1024
	store.openWriteFileFunc = func(fileNum uint32) (filer, error) {
		if file, ok := files[fileNum]; ok {
			mock := file.file.(*mockFile)
			mock.Lock()
			mock.closed = false
			mock.Unlock()
			return mock, nil
		}

		file := &mockFile{maxSize: -1}
		files[fileNum] = &lockableFile{file: file}
		return file, nil
	}

	// Put the write cursor part way into the current file so the block below
	// exceeds the maximum size and triggers the rollover, then close the
	// current file to reproduce the state after an initial load or rollback.
	wc := store.writeCursor
	wc.Lock()
	wc.curFileNum = 0
	wc.curOffset = 768
	wc.curFile.Lock()
	if wc.curFile.file != nil {
		_ = wc.curFile.file.Close()
	}
	wc.curFile.file = nil
	wc.curFile.Unlock()
	wc.Unlock()

	// The write must roll over to the next file rather than panicking.
	rawBlock := make([]byte, 512)
	loc, err := store.writeBlock(rawBlock, 1)
	if err != nil {
		t.Fatalf("writeBlock: unexpected error: %v", err)
	}

	if loc.blockFileNum != 1 {
		t.Fatalf("writeBlock: block written to file %d, want file 1",
			loc.blockFileNum)
	}
	if loc.blockLen != uint32(len(rawBlock))+12 {
		t.Fatalf("writeBlock: block length %d, want %d", loc.blockLen,
			uint32(len(rawBlock))+12)
	}

	// The final block height for the file that was rolled away from must have
	// been recorded, which is the write that previously dereferenced the nil
	// file.
	store.fbhMutex.Lock()
	height, ok := store.fileBlockHeights[0]
	store.fbhMutex.Unlock()
	if !ok {
		t.Fatal("writeBlock: no final block height recorded for file 0")
	}
	if height != 0 {
		t.Fatalf("writeBlock: final block height for file 0 is %d, want 0",
			height)
	}

	if err := idb.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
}
