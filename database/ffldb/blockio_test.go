package ffldb

import (
	"fmt"
	"github.com/gcash/bchd/database"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestDeleteBlocks(t *testing.T) {
	dbPath := filepath.Join(os.TempDir(), "ffldb-interfacetest")
	_ = os.RemoveAll(dbPath)
	db, err := database.Create(dbType, dbPath, blockDataNet)
	if err != nil {
		t.Errorf("Failed to create test database (%s) %v", dbType, err)
		return
	}
	defer os.RemoveAll(dbPath)
	defer db.Close()

	// Add a bunch of blocks to the database
	err = db.Update(func(tx database.Tx) error {
		blocks, err := loadBlocks(t, blockDataFile, blockDataNet)
		if err != nil {
			return err
		}
		for _, block := range blocks {
			err := tx.StoreBlock(block)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(path.Join(dbPath, "000000000.fdb")); os.IsNotExist(err) {
		t.Fatal("failed to save blocks correctly")
	}

	// Delete them
	err = db.Update(func(tx database.Tx) error {
		blocks, err := loadBlocks(t, blockDataFile, blockDataNet)
		if err != nil {
			return err
		}
		for _, block := range blocks {
			err := tx.DeleteBlock(block)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(path.Join(dbPath, "000000000.fdb")); !os.IsNotExist(err) {
		t.Error("Failed to delete all blocks")
	}

	err = db.View(func(tx database.Tx) error {
		blocks, err := loadBlocks(t, blockDataFile, blockDataNet)
		if err != nil {
			return err
		}
		for _, block := range blocks {
			buf, err := tx.FetchBlock(block.Hash())
			if err == nil || len(buf) > 0 {
				return fmt.Errorf("block %s should have been deleted", block.Hash())
			}
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}
