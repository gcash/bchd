// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import "testing"

// TestPanicReleasesBlockIndexLock ensures that a panic under the lock of the
// block index, which the RPC server recovers from, leaves the lock free, as
// everything would wait for it otherwise.
func TestPanicReleasesBlockIndexLock(t *testing.T) {
	bi := newBlockIndex(nil, nil)
	calls := map[string]func(){
		"NodeStatus":       func() { bi.NodeStatus(nil) },
		"SetStatusFlags":   func() { bi.SetStatusFlags(nil, statusValid) },
		"UnsetStatusFlags": func() { bi.UnsetStatusFlags(nil, statusValid) },
		"LookupNode":       func() { bi.LookupNode(nil) },
		"HaveBlock":        func() { bi.HaveBlock(nil) },
		"AddNode":          func() { bi.AddNode(nil) },
	}
	for name, call := range calls {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s didn't panic", name)
				}
			}()
			call()
		}()
		if !bi.TryLock() {
			t.Fatalf("%s left the lock held", name)
		}
		bi.Unlock()
	}
}
