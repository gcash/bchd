// Copyright (c) 2026 The BCHD developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"testing"

	"github.com/gcash/bchd/wire"
)

// tokenPrefixFailures lists the vectors BCHD does not pass, by file and
// index, with the reason.
var tokenPrefixFailures = map[string]string{
	"invalid #1": "short prefixes are taken as plain scripts",
	"invalid #2": "short prefixes are taken as plain scripts",
	"invalid #3": "short prefixes are taken as plain scripts",
	"valid #35":  "commitments longer than 128 bytes do not parse",
	"valid #36":  "commitments longer than 128 bytes do not parse",
	"valid #37":  "commitments longer than 128 bytes do not parse",
	"valid #46":  "commitments longer than 128 bytes do not parse",
	"valid #60":  "commitments longer than 128 bytes do not parse",
	"valid #61":  "commitments longer than 128 bytes do not parse",
}

// TestTokenPrefixVectors runs Bitcoin Cash Node's token prefix test vectors.
// Valid prefixes must parse to the given token data with an empty locking
// script and serialize back to the same bytes, invalid prefixes must fail to
// parse.
func TestTokenPrefixVectors(t *testing.T) {
	var valid []struct {
		Prefix string `json:"prefix"`
		Data   struct {
			Amount   string `json:"amount"`
			Category string `json:"category"`
			NFT      *struct {
				Commitment string `json:"commitment"`
				Capability string `json:"capability"`
			} `json:"nft"`
		} `json:"data"`
	}
	readJSON(t, "testdata/token_tests_prefix_valid.json", &valid)
	if len(valid) == 0 {
		t.Fatal("no valid vectors")
	}
	for i, test := range valid {
		prefix, err := hex.DecodeString(test.Prefix)
		if err != nil {
			t.Fatalf("valid #%d: %v", i, err)
		}
		var td wire.TokenData
		script, err := td.SeparateTokenDataFromPKScriptIfExists(
			slices.Clone(prefix), 0)
		if reason, ok := tokenPrefixFailures[fmt.Sprintf("valid #%d", i)]; ok {
			if err == nil {
				t.Errorf("valid #%d %s: listed as failing (%s) but "+
					"parses", i, test.Prefix, reason)
			}
			continue
		}
		if err != nil {
			t.Errorf("valid #%d %s: %v", i, test.Prefix, err)
			continue
		}
		if td.IsEmpty() || len(script) != 0 {
			t.Errorf("valid #%d %s: got token data %v, script %x", i,
				test.Prefix, td, script)
			continue
		}
		buf := td.TokenDataBuffer()
		if !bytes.Equal(buf.Bytes(), prefix) {
			t.Errorf("valid #%d %s: serialized as %x", i, test.Prefix,
				buf.Bytes())
		}

		category := slices.Clone(td.CategoryID[:])
		slices.Reverse(category)
		if hex.EncodeToString(category) != test.Data.Category {
			t.Errorf("valid #%d %s: category %x", i, test.Prefix, category)
		}
		amount, err := strconv.ParseUint(test.Data.Amount, 10, 64)
		if err != nil && test.Data.Amount != "" {
			t.Fatalf("valid #%d: bad amount %q", i, test.Data.Amount)
		}
		if td.HasAmount() != (amount != 0) || td.Amount != amount {
			t.Errorf("valid #%d %s: amount %d", i, test.Prefix, td.Amount)
		}
		if test.Data.NFT == nil {
			if td.HasNFT() {
				t.Errorf("valid #%d %s: unexpected NFT", i, test.Prefix)
			}
			continue
		}
		if hex.EncodeToString(td.Commitment) != test.Data.NFT.Commitment ||
			td.HasCommitmentLength() != (test.Data.NFT.Commitment != "") {

			t.Errorf("valid #%d %s: commitment %x", i, test.Prefix,
				td.Commitment)
		}
		capability := map[byte]string{wire.NONE: "none",
			wire.MUTABLE: "mutable", wire.MINTING: "minting"}[td.GetCapability()]
		if !td.HasNFT() || capability != test.Data.NFT.Capability {
			t.Errorf("valid #%d %s: capability %q", i, test.Prefix,
				capability)
		}
	}

	var invalid []struct {
		Prefix string `json:"prefix"`
		Error  string `json:"bchn_exception_message"`
	}
	readJSON(t, "testdata/token_tests_prefix_invalid.json", &invalid)
	if len(invalid) == 0 {
		t.Fatal("no invalid vectors")
	}
	for i, test := range invalid {
		prefix, err := hex.DecodeString(test.Prefix)
		if err != nil {
			t.Fatalf("invalid #%d: %v", i, err)
		}
		var td wire.TokenData
		script, err := td.SeparateTokenDataFromPKScriptIfExists(prefix, 0)
		if reason, ok := tokenPrefixFailures[fmt.Sprintf("invalid #%d", i)]; ok {
			if err != nil {
				t.Errorf("invalid #%d %s: listed as failing (%s) but "+
					"fails to parse", i, test.Prefix, reason)
			}
			continue
		}
		if err == nil {
			t.Errorf("invalid #%d %s (%s): parsed as token data %v, "+
				"script %x", i, test.Prefix, test.Error, td, script)
		}
	}
}

// readJSON unmarshals a JSON file.
func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}
