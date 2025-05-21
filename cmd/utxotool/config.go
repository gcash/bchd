// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	flags "github.com/jessevdk/go-flags"
)

const (
	defaultDbType = "ffldb"
)

var (
	bchdHomeDir     = bchutil.AppDataDir("bchd", false)
	defaultDataDir  = filepath.Join(bchdHomeDir, "data")
	knownDbTypes    = database.SupportedDrivers()
	activeNetParams = &chaincfg.MainNetParams
)

// config defines the configuration options for findcheckpoint.
//
// See loadConfig for details on the configuration load process.
type config struct {
	DataDir        string `short:"d" long:"datadir" description:"Location of the bchd data directory"`
	Force          bool   `short:"f" long:"force" description:"Allow rollbacks deeper than the last checkpoint. This will use a lot of memory."`
	DbType         string `long:"dbtype" description:"Database backend to use for the Block Chain"`
	TestNet3       bool   `long:"testnet" description:"Use the test network"`
	RegressionTest bool   `long:"regtest" description:"Use the regression test network"`
	SimNet         bool   `long:"simnet" description:"Use the simulation test network"`
	BlockHeight    int32  `short:"b" long:"height" description:"The height at which to calculate the utxo cache for"`
	OutFile        string `short:"o" long:"out" description:"Export the serialized utxo set to this file. Leave empty if you do not want to export to file"`
}

// validDbType returns whether or not dbType is a supported database type.
func validDbType(dbType string) bool {
	return slices.Contains(knownDbTypes, dbType)
}

// netName returns the name used when referring to a bitcoin network.  At the
// time of writing, bchd currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory name
// as "testnet" when the passed active network matches wire.TestNet3.
//
// A proper upgrade to move the data and log directories for this network to
// "testnet3" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
func netName(chainParams *chaincfg.Params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}

// loadConfig initializes and parses the config using command line options.
func loadConfig() (*config, []string, error) {
	// Default config.
	cfg := config{
		DataDir: defaultDataDir,
		DbType:  defaultDbType,
	}

	// Parse command line options.
	parser := flags.NewParser(&cfg, flags.Default)
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return nil, nil, err
	}

	// Multiple networks can't be selected simultaneously.
	funcName := "loadConfig"
	numNets := 0
	// Count number of network flags passed; assign active network params
	// while we're at it
	if cfg.TestNet3 {
		numNets++
		activeNetParams = &chaincfg.TestNet3Params
	}
	if cfg.RegressionTest {
		numNets++
		activeNetParams = &chaincfg.RegressionNetParams
	}
	if cfg.SimNet {
		numNets++
		activeNetParams = &chaincfg.SimNetParams
	}
	if numNets > 1 {
		str := "%s: The testnet, regtest, and simnet params can't be " +
			"used together -- choose one of the three"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	if cfg.BlockHeight < activeNetParams.Checkpoints[len(activeNetParams.Checkpoints)-1].Height && !cfg.Force {
		str := "%s: You are attempting a rollback deeper than the last checkpoint height of %d. " +
			"This is expected to use a lot of memory as the utxos for each block that gets " +
			"rolled back are held in memory. If you wish to continue use --force."
		err := fmt.Errorf(str, funcName, activeNetParams.Checkpoints[len(activeNetParams.Checkpoints)-1].Height)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Validate database type.
	if !validDbType(cfg.DbType) {
		str := "%s: The specified database type [%v] is invalid -- " +
			"supported types %v"
		err := fmt.Errorf(str, "loadConfig", cfg.DbType, knownDbTypes)
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	// Append the network type to the data directory so it is "namespaced"
	// per network.  In addition to the block database, there are other
	// pieces of data that are saved to disk such as address manager state.
	// All data is specific to a network, so namespacing the data directory
	// means each individual piece of serialized data does not have to
	// worry about changing names per network and such.
	cfg.DataDir = filepath.Join(cfg.DataDir, netName(activeNetParams))

	return &cfg, remainingArgs, nil
}
