// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import (
	"strings"
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// TestGetBcmrIdentityCmdLimit ensures that a limit reaches the server without
// after, which commands would stop at otherwise.
func TestGetBcmrIdentityCmdLimit(t *testing.T) {
	cmd := newGetBcmrIdentityCmd(&chainhash.Hash{}, true, nil, 10)
	raw, err := btcjson.MarshalCmd("1.0", 1, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `,true,"",10]`) {
		t.Fatalf("the limit is not in %s", raw)
	}
}
