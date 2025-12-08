// Copyright (c) 2015-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"fmt"

	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
)

var log = bchlog.Disabled

const (
	dbType = "ffldb"
)

// parseArgs parses the arguments from the database Open/Create methods.
func parseArgs(funcName string, args ...interface{}) (string, wire.BitcoinNet, uint64, uint32, error) {
	if len(args) < 2 || len(args) > 4 {
		return "", 0, 0, 0, fmt.Errorf("invalid arguments to %s.%s -- "+
			"expected database path and block network with optional cache size "+
			"and flush seconds", dbType, funcName)
	}

	dbPath, ok := args[0].(string)
	if !ok {
		return "", 0, 0, 0, fmt.Errorf("first argument to %s.%s is invalid -- "+
			"expected database path string", dbType, funcName)
	}

	network, ok := args[1].(wire.BitcoinNet)
	if !ok {
		return "", 0, 0, 0, fmt.Errorf("second argument to %s.%s is invalid -- "+
			"expected block network", dbType, funcName)
	}

	var cacheSize uint64
	var flushSecs uint32
	if len(args) > 2 {
		cacheSize, ok = args[2].(uint64)
		if !ok {
			return "", 0, 0, 0, fmt.Errorf("third argument to %s.%s is invalid -- "+
				"expected cache size", dbType, funcName)
		}
	}

	if len(args) > 3 {
		flushSecs, ok = args[3].(uint32)
		if !ok {
			return "", 0, 0, 0, fmt.Errorf("third argument to %s.%s is invalid -- "+
				"expected flush seconds", dbType, funcName)
		}
	}

	return dbPath, network, cacheSize, flushSecs, nil
}

// openDBDriver is the callback provided during driver registration that opens
// an existing database for use.
func openDBDriver(args ...interface{}) (database.DB, error) {
	dbPath, network, cacheSize, flushSecs, err := parseArgs("Open", args...)
	if err != nil {
		return nil, err
	}

	return openDB(dbPath, network, false, cacheSize, flushSecs)
}

// createDBDriver is the callback provided during driver registration that
// creates, initializes, and opens a database for use.
func createDBDriver(args ...interface{}) (database.DB, error) {
	dbPath, network, cacheSize, flushSecs, err := parseArgs("Create", args...)
	if err != nil {
		return nil, err
	}

	return openDB(dbPath, network, true, cacheSize, flushSecs)
}

// useLogger is the callback provided during driver registration that sets the
// current logger to the provided one.
func useLogger(logger bchlog.Logger) {
	log = logger
}

func init() {
	// Register the driver.
	driver := database.Driver{
		DbType:    dbType,
		Create:    createDBDriver,
		Open:      openDBDriver,
		UseLogger: useLogger,
	}
	if err := database.RegisterDriver(driver); err != nil {
		panic(fmt.Sprintf("Failed to register database driver '%s': %v",
			dbType, err))
	}
}
