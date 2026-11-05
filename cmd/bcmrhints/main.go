// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// bcmrhints writes the hints file of every token as of a height, from a node
// with --bcmrindex.  The same height gives the same file on every node, so
// anyone can check a published file against their own node by its SHA-256.
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gcash/bchd/bcmr"
	"github.com/gcash/bchd/rpcclient"
	"github.com/gcash/bchutil"
	flags "github.com/jessevdk/go-flags"
)

type config struct {
	RPCServer string `short:"s" long:"rpcserver" description:"RPC server to connect to"`
	RPCUser   string `short:"u" long:"rpcuser" env:"BCMRHINTS_RPCUSER" description:"RPC username"`
	RPCPass   string `short:"P" long:"rpcpass" env:"BCMRHINTS_RPCPASS" default-mask:"-" description:"RPC password, better given in BCMRHINTS_RPCPASS, as a process's arguments can be read"`
	RPCCert   string `short:"c" long:"rpccert" description:"RPC server certificate chain for validation"`
	NoTLS     bool   `long:"notls" description:"Disable TLS"`
	Height    int32  `long:"height" required:"true" description:"The height to describe the chain at, at least 100 blocks below the tip"`
	Out       string `short:"o" long:"out" required:"true" description:"The file to write"`
}

func run() error {
	cfg := config{
		RPCServer: "localhost:8334",
		RPCCert:   filepath.Join(bchutil.AppDataDir("bchd", false), "rpc.cert"),
	}
	if _, err := flags.Parse(&cfg); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return nil
		}
		return err
	}

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.RPCServer,
		User:         cfg.RPCUser,
		Pass:         cfg.RPCPass,
		HTTPPostMode: true,
		DisableTLS:   cfg.NoTLS,
	}
	if !cfg.NoTLS {
		cert, err := os.ReadFile(cfg.RPCCert)
		if err != nil {
			return err
		}
		connCfg.Certificates = cert
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return err
	}
	defer client.Shutdown()

	hints, err := generate(client, cfg.Height)
	if err != nil {
		return err
	}
	data := bcmr.EncodeHints(cfg.Height, hints)
	if err := os.WriteFile(cfg.Out, data, 0644); err != nil {
		return err
	}
	fmt.Printf("height %d, %d tokens, %d bytes\nsha256 %x\n", cfg.Height, len(hints),
		len(data), sha256.Sum256(data))
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
