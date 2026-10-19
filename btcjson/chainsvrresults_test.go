// Copyright (c) 2014 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcjson_test

import (
	"encoding/json"
	"testing"

	"github.com/gcash/bchd/btcjson"
)

// TestChainSvrCustomResults ensures any results that have custom marshalling
// work as inteded.
// and unmarshal code of results are as expected.
func TestChainSvrCustomResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   interface{}
		expected string
	}{
		{
			name: "custom vin marshal with coinbase",
			result: &btcjson.Vin{
				Coinbase: "021234",
				Sequence: 4294967295,
			},
			expected: `{"coinbase":"021234","sequence":4294967295}`,
		},
		{
			name: "custom vin marshal without coinbase",
			result: &btcjson.Vin{
				Txid: "123",
				Vout: 1,
				ScriptSig: &btcjson.ScriptSig{
					Asm: "0",
					Hex: "00",
				},
				Sequence: 4294967295,
			},
			expected: `{"txid":"123","vout":1,"scriptSig":{"asm":"0","hex":"00"},"sequence":4294967295}`,
		},
		{
			name: "custom vin marshal with prevout",
			result: &btcjson.Vin{
				Txid: "123",
				Vout: 1,
				ScriptSig: &btcjson.ScriptSig{
					Asm: "0",
					Hex: "00",
				},
				PrevOut: &btcjson.GetBlockPrevOutResult{
					Generated: true,
					Height:    100,
					Value:     50,
					ScriptPubKey: btcjson.PrevOutScriptPubKeyResult{
						Asm:  "1",
						Hex:  "51",
						Type: "nonstandard",
					},
				},
				Sequence: 4294967295,
			},
			expected: `{"txid":"123","vout":1,"scriptSig":{"asm":"0","hex":"00"},"prevout":{"generated":true,"height":100,"value":50,"scriptPubKey":{"asm":"1","hex":"51","type":"nonstandard"}},"sequence":4294967295}`,
		},
		{
			name: "txrawresult marshal with zero fee",
			result: &btcjson.TxRawResult{
				Txid: "123",
				Vin:  []btcjson.Vin{},
				Vout: []btcjson.Vout{},
				Fee:  new(float64),
			},
			expected: `{"txid":"123","version":0,"locktime":0,"vin":[],"vout":[],"fee":0}`,
		},
		{
			name: "custom vinprevout marshal with coinbase",
			result: &btcjson.VinPrevOut{
				Coinbase: "021234",
				Sequence: 4294967295,
			},
			expected: `{"coinbase":"021234","sequence":4294967295}`,
		},
		{
			name: "custom vinprevout marshal without coinbase",
			result: &btcjson.VinPrevOut{
				Txid: "123",
				Vout: 1,
				ScriptSig: &btcjson.ScriptSig{
					Asm: "0",
					Hex: "00",
				},
				PrevOut: &btcjson.PrevOut{
					Addresses: []string{"addr1"},
					Value:     0,
				},
				Sequence: 4294967295,
			},
			expected: `{"txid":"123","vout":1,"scriptSig":{"asm":"0","hex":"00"},"prevOut":{"addresses":["addr1"],"value":0},"sequence":4294967295}`,
		},
		{
			name: "testmempoolaccept allowed",
			result: &btcjson.TestMempoolAcceptResult{
				Txid:    "123",
				Allowed: true,
				Size:    191,
				Vsize:   191,
				Fees: &btcjson.TestMempoolAcceptFees{
					Base:             0.00000191,
					EffectiveFeeRate: 0.00001,
				},
			},
			expected: `{"txid":"123","allowed":true,"size":191,"vsize":191,"fees":{"base":0.00000191,"effective-feerate":0.00001}}`,
		},
		{
			name: "testmempoolaccept rejected",
			result: &btcjson.TestMempoolAcceptResult{
				Txid:          "123",
				RejectReason:  "256: absurdly-high-fee",
				RejectDetails: "absurdly-high-fee, 20000000 > 10000000",
			},
			expected: `{"txid":"123","allowed":false,"reject-reason":"256: absurdly-high-fee","reject-details":"absurdly-high-fee, 20000000 \u003e 10000000"}`,
		},
		{
			name: "validateaddress token-aware",
			result: &btcjson.ValidateAddressChainResult{
				IsValid:      true,
				Address:      "zr7fzmep8g7h7ymfxy74lgc0v950j3r295z4y4gq0v",
				IsTokenAware: btcjson.Bool(true),
			},
			expected: `{"isvalid":true,"address":"zr7fzmep8g7h7ymfxy74lgc0v950j3r295z4y4gq0v","istokenaware":true}`,
		},
		{
			name: "validateaddress not token-aware",
			result: &btcjson.ValidateAddressChainResult{
				IsValid:      true,
				Address:      "qr7fzmep8g7h7ymfxy74lgc0v950j3r2959lhtxxsl",
				IsTokenAware: btcjson.Bool(false),
			},
			expected: `{"isvalid":true,"address":"qr7fzmep8g7h7ymfxy74lgc0v950j3r2959lhtxxsl","istokenaware":false}`,
		},
		{
			name:     "validateaddress invalid",
			result:   &btcjson.ValidateAddressChainResult{},
			expected: `{"isvalid":false}`,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		marshalled, err := json.Marshal(test.result)
		if err != nil {
			t.Errorf("Test #%d (%s) unexpected error: %v", i,
				test.name, err)
			continue
		}
		if string(marshalled) != test.expected {
			t.Errorf("Test #%d (%s) unexpected marhsalled data - "+
				"got %s, want %s", i, test.name, marshalled,
				test.expected)
			continue
		}
	}
}

// TestTokenDataResults ensures the CashTokens data of transaction outputs
// marshals in the format used by Bitcoin Cash Node, and that results for
// outputs without a token marshal exactly as they did before the field was
// added.
func TestTokenDataResults(t *testing.T) {
	t.Parallel()

	const category = "1234567890000000000000000000000000000000000000000000000987654321"
	scriptPubKey := btcjson.ScriptPubKeyResult{
		Asm:       "OP_DUP OP_HASH160 ea132286328cfc819457b9dec386c4b5c84faa5c OP_EQUALVERIFY OP_CHECKSIG",
		Hex:       "76a914ea132286328cfc819457b9dec386c4b5c84faa5c88ac",
		ReqSigs:   1,
		Type:      "pubkeyhash",
		Addresses: []string{"addr1"},
	}
	const scriptPubKeyJSON = `{"asm":"OP_DUP OP_HASH160 ea132286328cfc819457b9dec386c4b5c84faa5c OP_EQUALVERIFY OP_CHECKSIG",` +
		`"hex":"76a914ea132286328cfc819457b9dec386c4b5c84faa5c88ac","reqSigs":1,"type":"pubkeyhash",` +
		`"addresses":["addr1"]}`

	tests := []struct {
		name     string
		result   interface{}
		expected string
	}{
		{
			name: "vout without token",
			result: &btcjson.Vout{
				Value:        0.0001,
				N:            1,
				ScriptPubKey: scriptPubKey,
			},
			expected: `{"value":0.0001,"n":1,"scriptPubKey":` + scriptPubKeyJSON + `}`,
		},
		{
			name: "vout with fungible tokens only",
			result: &btcjson.Vout{
				Value:        0.00001,
				N:            0,
				ScriptPubKey: scriptPubKey,
				TokenData: &btcjson.TokenDataResult{
					Category: category,
					Amount:   "9223372036854775807",
				},
			},
			expected: `{"value":0.00001,"n":0,"scriptPubKey":` + scriptPubKeyJSON +
				`,"tokenData":{"category":"` + category + `","amount":"9223372036854775807"}}`,
		},
		{
			name: "vout with non-fungible token only",
			result: &btcjson.Vout{
				Value:        0.00001,
				N:            0,
				ScriptPubKey: scriptPubKey,
				TokenData: &btcjson.TokenDataResult{
					Category: category,
					Amount:   "0",
					NFT: &btcjson.NFTResult{
						Capability: "minting",
						Commitment: "",
					},
				},
			},
			expected: `{"value":0.00001,"n":0,"scriptPubKey":` + scriptPubKeyJSON +
				`,"tokenData":{"category":"` + category + `","amount":"0",` +
				`"nft":{"capability":"minting","commitment":""}}}`,
		},
		{
			name: "gettxout without token",
			result: &btcjson.GetTxOutResult{
				BestBlock:     "000000000000000000000000000000000000000000000000000000000000abcd",
				Confirmations: 6,
				Value:         0.0001,
				ScriptPubKey:  scriptPubKey,
				Coinbase:      false,
			},
			expected: `{"bestblock":"000000000000000000000000000000000000000000000000000000000000abcd",` +
				`"confirmations":6,"value":0.0001,"scriptPubKey":` + scriptPubKeyJSON +
				`,"coinbase":false}`,
		},
		{
			name: "gettxout with fungible and non-fungible tokens",
			result: &btcjson.GetTxOutResult{
				BestBlock:     "000000000000000000000000000000000000000000000000000000000000abcd",
				Confirmations: 6,
				Value:         0.0001,
				ScriptPubKey:  scriptPubKey,
				TokenData: &btcjson.TokenDataResult{
					Category: category,
					Amount:   "252",
					NFT: &btcjson.NFTResult{
						Capability: "none",
						Commitment: "cc",
					},
				},
				Coinbase: false,
			},
			expected: `{"bestblock":"000000000000000000000000000000000000000000000000000000000000abcd",` +
				`"confirmations":6,"value":0.0001,"scriptPubKey":` + scriptPubKeyJSON +
				`,"tokenData":{"category":"` + category + `","amount":"252",` +
				`"nft":{"capability":"none","commitment":"cc"}},"coinbase":false}`,
		},
		{
			name: "prevout with token",
			result: &btcjson.PrevOut{
				Addresses: []string{"addr1"},
				Value:     0.00001,
				TokenData: &btcjson.TokenDataResult{
					Category: category,
					Amount:   "1",
				},
			},
			expected: `{"addresses":["addr1"],"value":0.00001,"tokenData":` +
				`{"category":"` + category + `","amount":"1"}}`,
		},
	}

	for _, test := range tests {
		marshalled, err := json.Marshal(test.result)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
			continue
		}
		if string(marshalled) != test.expected {
			t.Errorf("%s: unexpected marshalled data - got %s, "+
				"want %s", test.name, marshalled, test.expected)
		}
	}
}
