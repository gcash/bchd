// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"bytes"
	"testing"
)

func TestDeserializeObservedTransactionTruncated(t *testing.T) {
	var encoded bytes.Buffer
	(&observedTransaction{}).Serialize(&encoded)
	data := encoded.Bytes()
	for length := 0; length < len(data); length++ {
		transaction, err := deserializeObservedTransaction(bytes.NewReader(data[:length]))
		if err == nil || transaction != nil {
			t.Errorf("accepted %d of %d bytes: transaction=%v err=%v", length, len(data), transaction, err)
		}
	}
	if _, err := deserializeObservedTransaction(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
