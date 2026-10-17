package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gcash/bchutil"
)

var (
	rpcuserRegexp = regexp.MustCompile("(?m)^rpcuser=.+$")
	rpcpassRegexp = regexp.MustCompile("(?m)^rpcpass=.+$")
)

func TestExcessiveBlockSizeUserAgentComment(t *testing.T) {
	// Wipe test args.
	os.Args = []string{"bchd"}

	cfg, _, err := loadConfig()
	if err != nil {
		t.Fatal("Failed to load configuration")
	}

	if len(cfg.UserAgentComments) != 1 {
		t.Fatal("Expected EB UserAgentComment")
	}

	uac := cfg.UserAgentComments[0]
	uacExpected := "EB32.0"
	if uac != uacExpected {
		t.Fatalf("Expected UserAgentComments to contain %s but got %s", uacExpected, uac)
	}

	// Custom excessive block size.
	os.Args = []string{"bchd", "--excessiveblocksize=64000000"}

	cfg, _, err = loadConfig()
	if err != nil {
		t.Fatal("Failed to load configuration")
	}

	if len(cfg.UserAgentComments) != 1 {
		t.Fatal("Expected EB UserAgentComment")
	}

	uac = cfg.UserAgentComments[0]
	uacExpected = "EB64.0"
	if uac != uacExpected {
		t.Fatalf("Expected UserAgentComments to contain %s but got %s", uacExpected, uac)
	}
}

func TestMaxTxFee(t *testing.T) {
	tests := []struct {
		args    []string
		want    bchutil.Amount
		wantErr bool
	}{
		{args: nil, want: 10000000},
		{args: []string{"--maxtxfee=0.5"}, want: 50000000},
		{args: []string{"--maxtxfee=0.00001"}, want: 1000},
		{args: []string{"--maxtxfee=0.000009"}, wantErr: true},
		{args: []string{"--maxtxfee=0.01", "--minrelaytxfee=0.02"}, wantErr: true},
	}
	for _, test := range tests {
		os.Args = append([]string{"bchd"}, test.args...)
		cfg, _, err := loadConfig()
		if test.wantErr {
			if err == nil {
				t.Errorf("%v: expected an error", test.args)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: unexpected error: %v", test.args, err)
			continue
		}
		if cfg.maxTxFee != test.want {
			t.Errorf("%v: got maxtxfee %v, want %v", test.args,
				cfg.maxTxFee, test.want)
		}
	}
}

func TestCreateDefaultConfigFile(t *testing.T) {
	// Setup a temporary directory
	tmpDir, err := os.MkdirTemp("", "bchd")
	if err != nil {
		t.Fatalf("Failed creating a temporary directory: %v", err)
	}
	testpath := filepath.Join(tmpDir, "test.conf")

	// Clean-up
	defer func() {
		os.Remove(testpath)
		os.Remove(tmpDir)
	}()

	err = createDefaultConfigFile(testpath)

	if err != nil {
		t.Fatalf("Failed to create a default config file: %v", err)
	}

	content, err := os.ReadFile(testpath)
	if err != nil {
		t.Fatalf("Failed to read generated default config file: %v", err)
	}

	if !rpcuserRegexp.Match(content) {
		t.Error("Could not find rpcuser in generated default config file.")
	}

	if !rpcpassRegexp.Match(content) {
		t.Error("Could not find rpcpass in generated default config file.")
	}
}
