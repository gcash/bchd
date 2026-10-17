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
		{`{"method":"getblockcount","params":[],"id":1}`, "", "getblockcount", float64(1), 0},
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
