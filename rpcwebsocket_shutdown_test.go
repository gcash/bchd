// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/gcash/bchd/wire"
)

// TestNotificationManagerCallsAfterShutdown ensures that a client's requests
// to the websocket notification manager return once it has shut down.  A
// rescan makes some with the sync manager paused, which would then never
// stop.
func TestNotificationManagerCallsAfterShutdown(t *testing.T) {
	m := newWsNotificationManager(&rpcServer{})
	m.Start()
	m.Shutdown()
	m.WaitForShutdown()

	wsc := &wsClient{}
	calls := map[string]func(){
		"AddClient":                      func() { m.AddClient(wsc) },
		"RegisterBlockUpdates":           func() { m.RegisterBlockUpdates(wsc) },
		"UnregisterBlockUpdates":         func() { m.UnregisterBlockUpdates(wsc) },
		"RegisterDSProofUpdates":         func() { m.RegisterDSProofUpdates(wsc) },
		"UnregisterDSProofUpdates":       func() { m.UnregisterDSProofUpdates(wsc) },
		"RegisterNewMempoolTxsUpdates":   func() { m.RegisterNewMempoolTxsUpdates(wsc) },
		"UnregisterNewMempoolTxsUpdates": func() { m.UnregisterNewMempoolTxsUpdates(wsc) },
		"RegisterSpentRequests":          func() { m.RegisterSpentRequests(wsc, nil) },
		"UnregisterSpentRequest":         func() { m.UnregisterSpentRequest(wsc, &wire.OutPoint{}) },
		"RegisterTxOutAddressRequests":   func() { m.RegisterTxOutAddressRequests(wsc, nil) },
		"UnregisterTxOutAddressRequest":  func() { m.UnregisterTxOutAddressRequest(wsc, "") },
	}
	for name, call := range calls {
		done := make(chan struct{})
		go func() {
			call()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("%s didn't return", name)
		}
	}
}
