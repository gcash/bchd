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
	// Keep the config file, data and logs of the networks used here out of
	// the home directory.
	tmpDir, err := os.MkdirTemp("", "bchd")
	if err != nil {
		t.Fatalf("Failed creating a temporary directory: %v", err)
	}
	defer func() {
		os.RemoveAll(tmpDir)
		os.Args = []string{"bchd"}
		activeNetParams = &mainNetParams
	}()

	tests := []struct {
		args []string
		want uint32
		uac  string
	}{
		{nil, 32000000, "EB32.0"},
		{[]string{"--excessiveblocksize=64000000"}, 64000000, "EB64.0"},
		{[]string{"--excessiveblocksize=1000000"}, 32000000, "EB32.0"},
		{[]string{"--testnet"}, 32000000, "EB32.0"},
		{[]string{"--testnet4"}, 2000000, "EB2.0"},
		{[]string{"--chipnet"}, 2000000, "EB2.0"},
		{[]string{"--chipnet", "--excessiveblocksize=1000000"}, 2000000, "EB2.0"},
		{[]string{"--testnet4", "--excessiveblocksize=8000000"}, 8000000, "EB8.0"},
	}
	for _, test := range tests {
		// loadConfig only sets the network params when a network is
		// chosen, so start every case on mainnet.
		activeNetParams = &mainNetParams
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		cfg, _, err := loadConfig()
		if err != nil {
			t.Errorf("%v: unexpected error: %v", test.args, err)
			continue
		}
		if cfg.ExcessiveBlockSize != test.want {
			t.Errorf("%v: got excessive block size %d, want %d",
				test.args, cfg.ExcessiveBlockSize, test.want)
		}
		if len(cfg.UserAgentComments) != 1 ||
			cfg.UserAgentComments[0] != test.uac {

			t.Errorf("%v: got user agent comments %v, want [%s]",
				test.args, cfg.UserAgentComments, test.uac)
		}
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
		// The default is not checked against the minimum relay fee.
		{args: []string{"--minrelaytxfee=0.2"}, want: 10000000},
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

func TestFinalizationOptions(t *testing.T) {
	defer func() { os.Args = []string{"bchd"} }()

	tests := []struct {
		args  []string
		depth int32
		delay int64
		ok    bool
	}{
		{[]string{"bchd"}, 10, 7200, true},
		{[]string{"bchd", "--maxreorgdepth=-1"}, -1, 7200, true},
		{[]string{"bchd", "--maxreorgdepth=-5"}, -5, 7200, true},
		{[]string{"bchd", "--maxreorgdepth=100", "--finalizationdelay=0"}, 100, 0, true},
		{[]string{"bchd", "--finalizationdelay=-60"}, 10, -60, true},
		{[]string{"bchd", "--maxreorgdepth=0"}, 0, 0, false},
	}
	for _, test := range tests {
		os.Args = test.args
		cfg, _, err := loadConfig()
		if !test.ok {
			if err == nil {
				t.Errorf("%v: no error", test.args)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if cfg.MaxReorgDepth != test.depth ||
			cfg.FinalizationDelay != test.delay {

			t.Errorf("%v: depth %d delay %d, want %d %d", test.args,
				cfg.MaxReorgDepth, cfg.FinalizationDelay, test.depth,
				test.delay)
		}
	}
}

// TestProfileListenAddr ensures a bare --profile port only listens on
// localhost and that IP:port is accepted.
func TestProfileListenAddr(t *testing.T) {
	tmpDir := t.TempDir()
	defer func() {
		os.Args = []string{"bchd"}
		activeNetParams = &mainNetParams
	}()

	tests := []struct {
		profile string
		want    string
	}{
		{"6060", "127.0.0.1:6060"},
		{"127.0.0.1:6060", "127.0.0.1:6060"},
		{"[::1]:6060", "[::1]:6060"},
		{"0.0.0.0:6060", "0.0.0.0:6060"},
		{":6060", ":6060"},
		{"80", ""},
		{"65536", ""},
		{"localhost", ""},
		{"localhost:6060", ""},
		{"127.0.0.1:x", ""},
	}
	for _, test := range tests {
		activeNetParams = &mainNetParams
		os.Args = []string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir,
			"--profile=" + test.profile}
		cfg, _, err := loadConfig()
		if test.want == "" {
			if err == nil {
				t.Errorf("%q: accepted", test.profile)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", test.profile, err)
			continue
		}
		if cfg.Profile != test.want {
			t.Errorf("%q: got %q, want %q", test.profile, cfg.Profile,
				test.want)
		}
	}
}
