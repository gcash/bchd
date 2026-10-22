// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import "testing"

// TestGetBlockVerboseTxRawTx ensures GetBlockVerboseTx returns the
// transactions of a block whether the server lists them under "tx", as BCHN
// does, or under "rawtx", as bchd does at verbosity 2.
func TestGetBlockVerboseTxRawTx(t *testing.T) {
	for _, field := range []string{"tx", "rawtx"} {
		reply := make(chan *response, 1)
		reply <- &response{result: []byte(`{"hash":"00","` + field +
			`":[{"txid":"aa"}]}`)}
		block, err := FutureGetBlockVerboseTxResult{Response: reply}.Receive()
		if err != nil {
			t.Fatalf("%s: %v", field, err)
		}
		if len(block.Tx) != 1 || block.Tx[0].Txid != "aa" {
			t.Fatalf("%s: got transactions %+v", field, block.Tx)
		}
	}
}
