// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
)

// TestParkBlockRPCs ensures parkblock and unparkblock are for admins only and
// give the errors finalizeblock gives.
func TestParkBlockRPCs(t *testing.T) {
	for _, method := range []string{"parkblock", "unparkblock"} {
		if _, limited := rpcLimited[method]; limited {
			t.Errorf("%s is open to limited users", method)
		}
	}

	chain, _ := newRPCTestChain(t)
	s := &rpcServer{cfg: rpcserverConfig{Chain: chain}}
	genesis := chaincfg.RegressionNetParams.GenesisHash.String()
	unknown := "00000000000000000000000000000000000000000000000000000000000000ff"

	_, err := handleParkBlock(s, btcjson.NewParkBlockCmd("zz"), nil)
	wantRPCCode(t, err, btcjson.ErrRPCDecodeHexString)
	_, err = handleUnparkBlock(s, btcjson.NewUnparkBlockCmd("zz"), nil)
	wantRPCCode(t, err, btcjson.ErrRPCDecodeHexString)
	_, err = handleParkBlock(s, btcjson.NewParkBlockCmd(unknown), nil)
	wantRPCCode(t, err, btcjson.ErrRPCBlockNotFound)
	_, err = handleUnparkBlock(s, btcjson.NewUnparkBlockCmd(unknown), nil)
	wantRPCCode(t, err, btcjson.ErrRPCBlockNotFound)

	// The genesis block can't be parked, and unparking a block that is
	// not parked is not an error.
	_, err = handleParkBlock(s, btcjson.NewParkBlockCmd(genesis), nil)
	wantRPCCode(t, err, btcjson.ErrRPCDatabase)
	if _, err := handleUnparkBlock(s, btcjson.NewUnparkBlockCmd(genesis), nil); err != nil {
		t.Fatalf("unparkblock of a block that is not parked: %v", err)
	}
}
