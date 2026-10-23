// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"testing"

	"github.com/gcash/bchd/btcjson"
)

// TestMarshalRequestCreateRawTransaction ensures the arguments of
// createrawtransaction are sent as the JSON given, so the forms the server
// takes and btcjson's command cannot hold reach it, and the outputs keep their
// order, while the arguments of other commands are still decoded into their
// btcjson commands.
func TestMarshalRequestCreateRawTransaction(t *testing.T) {
	const txid = `"txid":"00112233445566778899aabbccddeeff0123456789abcdef0123456789abcdef"`
	const addrA = "qqqsyqcyq5rqwzqfpg9scrgwpugpzysnzs23v9ccrydpk8qarc0jqzchqcntn"
	const addrB = "qzqszqgqqzqszqgqqzqszqgqqzqszqgqqys0kr7c4cqc7s5j7dykhzw2gq6es"
	const tokenData = `{"category":"00112233445566778899aabbccddeeff0123456789abcdef0123456789abcdef","amount":"5"}`

	tests := []struct {
		name   string
		args   []interface{}
		params string
	}{{
		name: "outputs as an array, with data and token data, and the locktime third",
		args: []interface{}{`[{` + txid + `,"vout":1,"sequence":7}]`,
			`[{"` + addrB + `":0.2},{"data":["00ff","01"]},{"` + addrA +
				`":{"amount":0.1,"tokenData":` + tokenData + `}}]`,
			`500000`},
		params: `[[{` + txid + `,"vout":1,"sequence":7}],[{"` + addrB +
			`":0.2},{"data":["00ff","01"]},{"` + addrA + `":{"amount":0.1,` +
			`"tokenData":` + tokenData + `}}],500000]`,
	}, {
		name: "outputs as an object, not sorted",
		args: []interface{}{`[{` + txid + `,"vout":1}]`,
			`{"` + addrB + `":0.2, "` + addrA + `":0.1}`},
		params: `[[{` + txid + `,"vout":1}],{"` + addrB + `":0.2,"` + addrA +
			`":0.1}]`,
	}, {
		name:   "input without its vout",
		args:   []interface{}{`[{` + txid + `}]`, `{"` + addrA + `":0.1}`},
		params: `[[{` + txid + `}],{"` + addrA + `":0.1}]`,
	}, {
		name: "bchd's form with cashtokens and the locktime",
		args: []interface{}{`[]`, `{"` + addrA + `":0.1}`, `[{"address":"` +
			addrA + `","category":"00","amount":5}]`, `7`},
		params: `[[],{"` + addrA + `":0.1},[{"address":"` + addrA +
			`","category":"00","amount":5}],7]`,
	}}
	for _, test := range tests {
		got, err := marshalRequest("createrawtransaction", test.args)
		want := `{"jsonrpc":"1.0","method":"createrawtransaction","params":` +
			test.params + `,"id":1}`
		if err != nil || string(got) != want {
			t.Errorf("%s: got %s, err %v, want %s", test.name, got, err,
				want)
		}
	}

	// An argument that is not JSON is refused before it is sent.
	_, err := marshalRequest("createrawtransaction",
		[]interface{}{`[]`, `{"` + addrA + `":0.1`})
	var jerr btcjson.Error
	if !errors.As(err, &jerr) || jerr.ErrorCode != btcjson.ErrInvalidType ||
		jerr.Description != "parameter #2 must be valid JSON" {

		t.Errorf("argument not JSON: got error %v, want %v", err,
			btcjson.ErrInvalidType)
	}

	// The arguments of other commands are decoded into their commands, so a
	// string is quoted and a number that is not an integer is refused.
	got, err := marshalRequest("getblock", []interface{}{"00ab"})
	want := `{"jsonrpc":"1.0","method":"getblock","params":["00ab"],"id":1}`
	if err != nil || string(got) != want {
		t.Errorf("getblock: got %s, err %v, want %s", got, err, want)
	}
	_, err = marshalRequest("getblockhash", []interface{}{"1.5"})
	if !errors.As(err, &jerr) || jerr.ErrorCode != btcjson.ErrInvalidType {
		t.Errorf("getblockhash 1.5: got error %v, want %v", err,
			btcjson.ErrInvalidType)
	}
}
