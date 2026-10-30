// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpctest

import (
	"net"
	"strconv"
	"testing"
)

// TestListeningAddressesBelowEphemeralPorts checks that the ports a harness
// listens on stay in their ranges and below 32768, where Linux starts the
// ephemeral ports of outgoing connections. The RPC ports reached 60000, so a
// connection that the test made could hold the RPC port of a harness, which
// then failed to start with "address already in use".
func TestListeningAddressesBelowEphemeralPorts(t *testing.T) {
	harnessStateMtx.Lock()
	defer harnessStateMtx.Unlock()

	savedPID, savedInstances := processID, numTestInstances
	defer func() {
		processID, numTestInstances = savedPID, savedInstances
	}()

	port := func(addr string) int {
		t.Helper()

		_, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatal(err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			t.Fatal(err)
		}
		return port
	}

	// Linux gives processes IDs up to 4194304 by default, and a test
	// process runs a handful of harnesses.
	for pid := 1; pid <= 4194304; pid += 997 {
		for instances := 0; instances < 20; instances++ {
			processID, numTestInstances = pid, instances

			p2p, rpc := generateListeningAddresses()
			p2pPort, rpcPort := port(p2p), port(rpc)
			if p2pPort < minPeerPort || p2pPort >= maxPeerPort {
				t.Fatalf("pid %d, harness %d: p2p port %d outside "+
					"[%d, %d)", pid, instances, p2pPort,
					minPeerPort, maxPeerPort)
			}
			if rpcPort < minRPCPort || rpcPort >= maxRPCPort ||
				rpcPort >= 32768 {

				t.Fatalf("pid %d, harness %d: rpc port %d outside "+
					"[%d, %d) or not below 32768", pid,
					instances, rpcPort, minRPCPort, maxRPCPort)
			}
		}
	}
}
