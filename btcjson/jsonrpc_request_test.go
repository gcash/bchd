// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcjson_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gcash/bchd/btcjson"
)

// unmarshalRequest decodes raw into a Request and turns a panic into an error.
func unmarshalRequest(raw string) (req btcjson.Request, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	err = json.Unmarshal([]byte(raw), &req)
	return req, err
}

// TestRequestUnmarshalWrongFieldTypes ensures a method or jsonrpc member of
// the wrong JSON type is rejected with an error instead of a panic.
func TestRequestUnmarshalWrongFieldTypes(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"method":1,"params":[],"id":1}`,
		`{"method":null,"params":[],"id":1}`,
		`{"method":["getblockcount"],"params":[],"id":1}`,
		`{"method":"getblockcount","jsonrpc":2,"params":[],"id":1}`,
		`{"method":"getblockcount","jsonrpc":{},"params":[],"id":1}`,
	} {
		_, err := unmarshalRequest(raw)
		if err == nil {
			t.Errorf("%s: expected an error", raw)
			continue
		}
		if strings.HasPrefix(err.Error(), "panic:") {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

// TestRequestUnmarshalValid ensures well formed 1.0 and 2.0 requests still
// decode as before.
func TestRequestUnmarshalValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw     string
		jsonrpc string
		method  string
		id      interface{}
		params  int
	}{
		{`{"method":"getblockcount","params":[],"id":1}`, "", "getblockcount", json.Number("1"), 0},
		{`{"jsonrpc":"1.0","method":"getblock","params":["00",true],"id":"a"}`, "1.0", "getblock", "a", 2},
		{`{"jsonrpc":"2.0","method":"getblockcount","id":null}`, "2.0", "getblockcount", nil, 0},
		{`{"jsonrpc":"2.0","method":"getblockcount"}`, "2.0", "getblockcount", nil, 0},
	}
	for _, test := range tests {
		req, err := unmarshalRequest(test.raw)
		if err != nil {
			t.Errorf("%s: %v", test.raw, err)
			continue
		}
		if req.Jsonrpc != test.jsonrpc || req.Method != test.method ||
			req.ID != test.id || len(req.Params) != test.params {
			t.Errorf("%s: got %+v", test.raw, req)
		}
	}
}

// TestRequestUnmarshalKeepsParams ensures parameters are kept exactly as sent,
// so integers above 2^53 are not rounded, and that params which are not an
// array still decode to nil.
func TestRequestUnmarshalKeepsParams(t *testing.T) {
	t.Parallel()
	want := []string{`9007199254740993`, `9223372036854775807`,
		`{"amount":9007199254740993}`, `"x"`, `1.0`}
	raw := `{"method":"m","params":[` + strings.Join(want, ", ") + `],"id":1}`
	req, err := unmarshalRequest(raw)
	if err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	if len(req.Params) != len(want) {
		t.Fatalf("got %d params, want %d", len(req.Params), len(want))
	}
	for i, param := range req.Params {
		if string(param) != want[i] {
			t.Errorf("param %d: got %s, want %s", i, param, want[i])
		}
	}

	for _, raw := range []string{
		`{"method":"m","params":null,"id":1}`,
		`{"method":"m","params":{},"id":1}`,
		`{"method":"m","params":"x","id":1}`,
	} {
		req, err := unmarshalRequest(raw)
		if err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if req.Params != nil {
			t.Errorf("%s: got params %v, want nil", raw, req.Params)
		}
	}
}

// TestRequestIDKeptAsSent ensures a numeric request id is kept as sent and
// is given back in the response exactly, as BCHN gives it back, so integers
// above 2^53 are not rounded and other numbers keep their form.
func TestRequestIDKeptAsSent(t *testing.T) {
	t.Parallel()
	for _, id := range []string{`12345678901234567890`, `9007199254740993`,
		`-9223372036854775809`, `1.0`, `1e3`, `-0`, `0.1`, `"x"`} {

		raw := `{"jsonrpc":"1.0","method":"getblockcount","params":[],"id":` +
			id + `}`
		req, err := unmarshalRequest(raw)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		reply, err := btcjson.MarshalResponse(req.Jsonrpc, req.ID, 1, nil)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		want := `{"jsonrpc":"1.0","result":1,"error":null,"id":` + id + `}`
		if string(reply) != want {
			t.Errorf("%s: got reply %s, want %s", id, reply, want)
		}
	}
}

// TestRequestIDOtherTypesKeptAsSent ensures a request id that is a bool, an
// object or an array, which JSON-RPC 1.0 allows, is kept as a json.RawMessage
// of the JSON sent and given back in the response as sent, as BCHN gives it
// back, with an object's members in their order and numbers in their form.
func TestRequestIDOtherTypesKeptAsSent(t *testing.T) {
	t.Parallel()
	for _, id := range []string{`true`, `false`, `{"b":1,"a":[2,"x"]}`,
		`{"n":12345678901234567890,"n":1.0}`, `[1e3,null,{}]`, `{}`, `[]`} {

		raw := `{"jsonrpc":"1.0","method":"getblockcount","params":[],"id":` +
			id + `}`
		req, err := unmarshalRequest(raw)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if got, ok := req.ID.(json.RawMessage); !ok || string(got) != id {
			t.Errorf("%s: got id %#v, want it as a json.RawMessage", id,
				req.ID)
		}
		reply, err := btcjson.MarshalResponse(req.Jsonrpc, req.ID, 1, nil)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		want := `{"jsonrpc":"1.0","result":1,"error":null,"id":` + id + `}`
		if string(reply) != want {
			t.Errorf("%s: got reply %s, want %s", id, reply, want)
		}
	}
}
