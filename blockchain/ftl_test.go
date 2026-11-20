// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
)

// ftlVectorFile is the layout of the CHIP-2025-03 future time limit vectors.
type ftlVectorFile struct {
	Constants map[string]string `json:"constants"`
	Vectors   []struct {
		Name                string `json:"name"`
		RulesActiveOnParent bool   `json:"rulesActiveOnParent"`
		Now                 int64  `json:"now"`
		MtpPrev             int64  `json:"mtpPrev"`
		MaxTime             int64  `json:"maxTime"`
		ValidTimestamps     int64  `json:"validTimestamps"`
		Probes              []struct {
			Time  int64 `json:"time"`
			Valid bool  `json:"valid"`
		} `json:"probes"`
	} `json:"vectors"`
}

// ftlTestParent returns a block whose median time past is mtp and whose
// child takes 1-minute blocks when active is set.
func ftlTestParent(mtp int64, active bool) *blockNode {
	var node *blockNode
	for i := 0; i < medianTimeBlocks; i++ {
		node = &blockNode{parent: node, timestamp: mtp,
			nextInterval: legacyBlockInterval}
		if node.parent != nil {
			node.height = node.parent.height + 1
		}
	}
	if active {
		node.nextInterval = upgrade2027BlockInterval
	}
	return node
}

// TestFTLVectors runs the CHIP-2025-03 future time limit vectors: the latest
// accepted timestamp of a block for its parent's median time past and the
// node's clock, and whether each probed timestamp is accepted.
func TestFTLVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "chip202503", "ftl",
		"ftl.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vectors ftlVectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	c := vectors.Constants
	if c["LEGACY_MAX_FUTURE_BLOCK_TIME"] != strconv.Itoa(MaxTimeOffsetSeconds) ||
		c["MAX_FUTURE_BLOCK_TIME"] != strconv.Itoa(Upgrade2027MaxTimeOffsetSeconds) {

		t.Fatalf("constants %v", c)
	}
	if len(vectors.Vectors) != 20 {
		t.Fatalf("%d vectors, want 20", len(vectors.Vectors))
	}

	var probes int
	for _, v := range vectors.Vectors {
		parent := ftlTestParent(v.MtpPrev, v.RulesActiveOnParent)
		if got := parent.CalcPastMedianTime().Unix(); got != v.MtpPrev {
			t.Fatalf("%s: median time past %d, want %d", v.Name, got,
				v.MtpPrev)
		}
		maxTime := maxBlockTime(parent, time.Unix(v.Now, 0)).Unix()
		if maxTime != v.MaxTime {
			t.Errorf("%s: max time %d, want %d", v.Name, maxTime,
				v.MaxTime)
		}
		if valid := max(0, maxTime-v.MtpPrev); valid != v.ValidTimestamps {
			t.Errorf("%s: %d valid timestamps, want %d", v.Name,
				valid, v.ValidTimestamps)
		}
		for _, p := range v.Probes {
			valid := p.Time > v.MtpPrev && p.Time <= maxTime
			if valid != p.Valid {
				t.Errorf("%s: time %d valid %v, want %v", v.Name,
					p.Time, valid, p.Valid)
			}
			probes++
		}
	}
	if probes != 81 {
		t.Errorf("%d probes, want 81", probes)
	}
}

// TestFTLHeaderContext ensures a header is refused as too new against the
// node's clock, at 12 minutes once 1-minute blocks apply to it and at 2 hours
// before, and that the check is made before that of the block version, as in
// Bitcoin Cash Node.
func TestFTLHeaderContext(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.Checkpoints = nil
	params.BIP0034Height = 0
	params.BIP0065Height = 0
	params.BIP0066Height = 0
	b := newFakeChain(&params)
	now := time.Unix(1800000000, 0)
	b.timeSource = &fixedTimeSource{now: now}

	for _, test := range []struct {
		name   string
		active bool
		ahead  int64
		reject bool
	}{
		{"legacy at 2 hours", false, MaxTimeOffsetSeconds, false},
		{"legacy past 2 hours", false, MaxTimeOffsetSeconds + 1, true},
		{"active at 12 minutes", true, Upgrade2027MaxTimeOffsetSeconds, false},
		{"active past 12 minutes", true, Upgrade2027MaxTimeOffsetSeconds + 1, true},
	} {
		parent := ftlTestParent(now.Unix()-3000, test.active)
		header := &wire.BlockHeader{
			Version:   4,
			Timestamp: now.Add(time.Duration(test.ahead) * time.Second),
		}
		err := b.checkBlockHeaderContext(header, parent, BFFastAdd)
		var rerr RuleError
		switch {
		case !test.reject && err != nil:
			t.Errorf("%s: %v", test.name, err)
		case test.reject && (!errors.As(err, &rerr) ||
			rerr.ErrorCode != ErrTimeTooNew):

			t.Errorf("%s: got %v, want ErrTimeTooNew", test.name, err)
		}

		// An old block version is refused after the time.
		want := ErrBlockVersionTooOld
		if test.reject {
			want = ErrTimeTooNew
		}
		header.Version = 1
		err = b.checkBlockHeaderContext(header, parent, BFFastAdd)
		if !errors.As(err, &rerr) || rerr.ErrorCode != want {
			t.Errorf("%s: version 1 gave %v, want %v", test.name, err,
				want)
		}
	}
}
