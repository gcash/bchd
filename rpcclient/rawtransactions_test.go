// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// newCreateRawTxTestClient starts a JSON-RPC server that answers each
// createrawtransaction request with the hex of tx, and returns a client of it
// in HTTP POST mode and a channel of the params of each request it receives.
func newCreateRawTxTestClient(t *testing.T, tx *wire.MsgTx) (*Client, <-chan string) {
	t.Helper()

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	reply := `{"result":"` + hex.EncodeToString(buf.Bytes()) +
		`","error":null,"id":1}`
	params := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		body, err := io.ReadAll(r.Body)
		if err == nil {
			err = json.Unmarshal(body, &req)
		}
		if err != nil || req.Method != "createrawtransaction" {
			t.Errorf("unexpected request %s: %v", body, err)
		}
		params <- string(req.Params)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(server.Close)

	client, err := New(&ConnConfig{
		Host:         server.Listener.Addr().String(),
		User:         "user",
		Pass:         "pass",
		HTTPPostMode: true,
		DisableTLS:   true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Shutdown)
	return client, params
}

// createRawTxTestAddr returns a mainnet pay-to-pubkey-hash address of a hash
// filled with the given byte.
func createRawTxTestAddr(t *testing.T, fill byte) bchutil.Address {
	t.Helper()

	addr, err := bchutil.NewAddressPubKeyHash(bytes.Repeat([]byte{fill}, 20),
		&chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

// TestCreateRawTransactionOrdered ensures CreateRawTransactionOrdered sends
// the outputs in the order given, as an array of objects of one address each,
// with the cashtokens and locktime in forms bchd takes, the locktime third
// without cashtokens as BCHN takes it, and returns the transaction made.
func TestCreateRawTransactionOrdered(t *testing.T) {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Index: 1}, nil))
	tx.AddTxOut(wire.NewTxOut(10000, []byte{0x51}, wire.TokenData{}))
	client, params := newCreateRawTxTestClient(t, tx)

	addrs := []bchutil.Address{createRawTxTestAddr(t, 0x91),
		createRawTxTestAddr(t, 0x37), createRawTxTestAddr(t, 0x05)}
	var outputs []CreateRawTransactionOutput
	var outputsJSON []string
	for i, addr := range addrs {
		outputs = append(outputs, CreateRawTransactionOutput{
			Address: addr,
			Amount:  bchutil.Amount(i+1) * 10000,
		})
		outputsJSON = append(outputsJSON, `{"`+addr.String()+`":0.000`+
			string(rune('1'+i))+`}`)
	}
	sentOutputs := "[" + strings.Join(outputsJSON, ",") + "]"
	inputs := []btcjson.TransactionInput{{Txid: "00", Vout: 1}}
	sentInputs := `[{"txid":"00","vout":1}]`
	tokens := []btcjson.TransactionOutputCashToken{{
		Address:  addrs[1].String(),
		Category: "00",
		Amount:   5,
	}}
	sentTokens, err := json.Marshal(tokens)
	if err != nil {
		t.Fatal(err)
	}
	lockTime := int64(7)

	tests := []struct {
		name     string
		inputs   []btcjson.TransactionInput
		tokens   *[]btcjson.TransactionOutputCashToken
		lockTime *int64
		params   string
	}{
		{"outputs", inputs, nil, nil,
			"[" + sentInputs + "," + sentOutputs + "]"},
		{"no inputs, locktime", nil, nil, &lockTime,
			"[[]," + sentOutputs + ",7]"},
		{"cashtokens and locktime", inputs, &tokens, &lockTime,
			"[" + sentInputs + "," + sentOutputs + "," +
				string(sentTokens) + ",7]"},
		{"cashtokens", inputs, &tokens, nil,
			"[" + sentInputs + "," + sentOutputs + "," +
				string(sentTokens) + "]"},
	}
	for _, test := range tests {
		got, err := client.CreateRawTransactionOrdered(test.inputs,
			outputs, test.tokens, test.lockTime)
		if err != nil || got.TxHash() != tx.TxHash() {
			t.Errorf("%s: got %v, err %v, want %v", test.name, got,
				err, tx)
		}
		if sent := <-params; sent != test.params {
			t.Errorf("%s: sent params %s, want %s", test.name, sent,
				test.params)
		}
	}
}

// TestCreateRawTransactionCashTokenNames ensures CreateRawTransaction and
// CreateRawTransactionOrdered send cashtokens entries with the names bchd
// documents, rather than Go's field names.
func TestCreateRawTransactionCashTokenNames(t *testing.T) {
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Index: 1}, nil))
	tx.AddTxOut(wire.NewTxOut(10000, []byte{0x51}, wire.TokenData{}))
	client, params := newCreateRawTxTestClient(t, tx)

	addr := createRawTxTestAddr(t, 0x91)
	tokens := []btcjson.TransactionOutputCashToken{{
		Address:  addr.String(),
		Category: "00",
		Amount:   5,
	}}
	sentTokens := `[{"address":"` + addr.String() + `","category":"00",` +
		`"amount":5}]`

	_, err := client.CreateRawTransaction(nil,
		map[bchutil.Address]bchutil.Amount{addr: 10000}, &tokens, nil)
	want := `[[],{"` + addr.String() + `":0.0001},` + sentTokens + `]`
	if sent := <-params; err != nil || sent != want {
		t.Errorf("CreateRawTransaction: sent params %s, err %v, want %s",
			sent, err, want)
	}

	_, err = client.CreateRawTransactionOrdered(nil,
		[]CreateRawTransactionOutput{{Address: addr, Amount: 10000}},
		&tokens, nil)
	want = `[[],[{"` + addr.String() + `":0.0001}],` + sentTokens + `]`
	if sent := <-params; err != nil || sent != want {
		t.Errorf("CreateRawTransactionOrdered: sent params %s, err %v, "+
			"want %s", sent, err, want)
	}
}
