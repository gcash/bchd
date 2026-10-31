// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/bchutil/gcs/builder"
)

// TestMempoolFilterCache ensures that the filter of the mempool that
// getcfmempool serves is built once while the mempool stays the same, and
// built again after each change, matching a filter built fresh from the pool.
func TestMempoolFilterCache(t *testing.T) {
	h := newMempoolTestHarness(t, 1)
	pool := h.server.cfg.TxMemPool
	s := &server{txMemPool: pool}

	fresh := func() []byte {
		t.Helper()
		var txs []*wire.MsgTx
		for _, txDesc := range pool.TxDescs() {
			txs = append(txs, txDesc.Tx.MsgTx())
		}
		filter, err := builder.BuildMempoolFilter(txs)
		if err != nil {
			t.Fatal(err)
		}
		filterBytes, err := filter.NBytes()
		if err != nil {
			t.Fatal(err)
		}
		return filterBytes
	}
	served := func() []byte {
		t.Helper()
		filterBytes, err := s.mempoolFilter()
		if err != nil {
			t.Fatal(err)
		}
		return filterBytes
	}

	empty := served()
	if !bytes.Equal(empty, fresh()) {
		t.Fatal("the filter served for the empty mempool is not its own")
	}
	if again := served(); &again[0] != &empty[0] {
		t.Fatal("the filter was built again for an unchanged mempool")
	}

	tx := bchutil.NewTx(h.spend(t, h.outputs[0], 1000))
	if _, err := pool.ProcessTransaction(tx, false, 0); err != nil {
		t.Fatal(err)
	}
	added := served()
	if bytes.Equal(added, empty) || !bytes.Equal(added, fresh()) {
		t.Fatal("the filter served after a transaction joined the " +
			"mempool is not the mempool's")
	}
	if again := served(); &again[0] != &added[0] {
		t.Fatal("the filter was built again for an unchanged mempool")
	}

	pool.RemoveTransaction(tx, false)
	removed := served()
	if &removed[0] == &added[0] || !bytes.Equal(removed, fresh()) {
		t.Fatal("the filter served after a transaction left the " +
			"mempool is not the mempool's")
	}
}
