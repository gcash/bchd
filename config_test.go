package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gcash/bchlog"
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
		{[]string{"--tempnet"}, 2000000, "EB2.0"},
		{[]string{"--testnet4", "--excessiveblocksize=8000000"}, 8000000, "EB8.0"},
		{[]string{"--regtest", "--excessiveblocksize=2000000000"},
			2000000000, "EB2000.0"},
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

// TestExcessiveBlockSizeMax ensures an excessive block size above the largest
// block size limit is refused.
func TestExcessiveBlockSizeMax(t *testing.T) {
	tmpDir := t.TempDir()
	defer func() {
		os.Args = []string{"bchd"}
		activeNetParams = &mainNetParams
	}()

	for _, net := range []string{"--regtest", "--testnet4"} {
		activeNetParams = &mainNetParams
		os.Args = []string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir, net,
			"--excessiveblocksize=2000000001"}
		if _, _, err := loadConfig(); err == nil {
			t.Errorf("%s: an excessive block size above 2000000000 "+
				"was taken", net)
		}
	}
}

func TestMaxTxFee(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() { os.Args = []string{"bchd"} }()

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
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
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
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
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
		os.Args = append([]string{test.args[0],
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args[1:]...)
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

// TestUpgrade2027ActivationTime checks that the override can't be before the
// network's ASERT anchor block, except on regtest and simnet, which don't
// adjust the difficulty.
func TestUpgrade2027ActivationTime(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() {
		os.Args = []string{"bchd"}
		activeNetParams = &mainNetParams
	}()

	tests := []struct {
		args []string
		want int64 // -1 for no override, -2 for a rejected option
	}{
		{[]string{}, -1},
		{[]string{"--upgrade2027activationtime=1810382400"}, 1810382400},
		{[]string{"--upgrade2027activationtime=1605447844"}, 1605447844},
		{[]string{"--upgrade2027activationtime=1605447843"}, -2},
		{[]string{"--chipnet", "--upgrade2027activationtime=1792065600"}, 1792065600},
		{[]string{"--chipnet", "--upgrade2027activationtime=1605451779"}, 1605451779},
		{[]string{"--chipnet", "--upgrade2027activationtime=1605451778"}, -2},
		{[]string{"--tempnet", "--upgrade2027activationtime=1794744000"}, 1794744000},
		{[]string{"--tempnet", "--upgrade2027activationtime=1605451778"}, -2},
		{[]string{"--testnet", "--upgrade2027activationtime=1605445399"}, -2},
		{[]string{"--testnet4", "--upgrade2027activationtime=1605451778"}, -2},
		{[]string{"--regtest", "--upgrade2027activationtime=0"}, 0},
		{[]string{"--regtest", "--upgrade2027activationtime=1"}, 1},
		{[]string{"--simnet", "--upgrade2027activationtime=0"}, 0},
		{[]string{"--regtest", "--upgrade2027activationtime=-1"}, -2},
		{[]string{"--regtest", "--upgrade2027activationtime=9223372036854775808"}, -2},
	}
	for _, test := range tests {
		activeNetParams = &mainNetParams
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		cfg, _, err := loadConfig()
		if test.want == -2 {
			if err == nil {
				t.Errorf("%v: no error", test.args)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: %v", test.args, err)
			continue
		}
		got := int64(-1)
		if cfg.Upgrade2027Time != nil {
			got = int64(*cfg.Upgrade2027Time)
		}
		if got != test.want {
			t.Errorf("%v: override %d, want %d", test.args, got, test.want)
		}
	}
}

// TestBcmrIndexOptions checks the options that --bcmrindex can't be used
// with: those that keep no blocks from before CashTokens activated, and a
// --maxreorgdepth deeper than the index can undo.
func TestBcmrIndexOptions(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() { os.Args = []string{"bchd"} }()

	tests := []struct {
		args []string
		ok   bool
	}{
		{[]string{"--bcmrindex"}, true},
		{[]string{"--bcmrindex", "--maxreorgdepth=1000"}, true},
		{[]string{"--bcmrindex", "--maxreorgdepth=-1"}, true},
		{[]string{"--bcmrindex", "--maxreorgdepth=1001"}, false},
		{[]string{"--bcmrindex", "--prune"}, false},
		{[]string{"--bcmrindex", "--fastsync"}, false},
	}
	for _, test := range tests {
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		_, _, err := loadConfig()
		if (err == nil) != test.ok {
			t.Errorf("%v: error %v, want it accepted %v", test.args, err, test.ok)
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

// TestMinRelayTxFee ensures --minrelaytxfee must be above 0.
func TestMinRelayTxFee(t *testing.T) {
	defer func() { os.Args = []string{"bchd"} }()
	dir := t.TempDir()
	configFile := filepath.Join(dir, "bchd.conf")
	if err := os.WriteFile(configFile, nil, 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		args    []string
		want    bchutil.Amount
		wantErr bool
	}{
		{args: nil, want: 1000},
		{args: []string{"--minrelaytxfee=0.00002"}, want: 2000},
		{args: []string{"--minrelaytxfee=0.00000001"}, want: 1},
		{args: []string{"--minrelaytxfee=0"}, wantErr: true},
		{args: []string{"--minrelaytxfee=-0.00001"}, wantErr: true},
		// Rounds to 0 satoshis.
		{args: []string{"--minrelaytxfee=0.000000001"}, wantErr: true},
		// At most every coin there is per kilobyte.
		{args: []string{"--minrelaytxfee=21000000"}, want: bchutil.MaxSatoshi},
		{args: []string{"--minrelaytxfee=21000000.00000001"}, wantErr: true},
	}
	for _, test := range tests {
		os.Args = append([]string{"bchd", "--configfile=" + configFile,
			"--logdir=" + dir}, test.args...)
		cfg, _, err := loadConfig()
		if test.wantErr {
			if err == nil || !strings.Contains(err.Error(),
				"invalid minrelaytxfee") {

				t.Errorf("%v: expected an invalid minrelaytxfee "+
					"error, got %v", test.args, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: unexpected error: %v", test.args, err)
			continue
		}
		if cfg.minRelayTxFee != test.want {
			t.Errorf("%v: got minrelaytxfee %v, want %v", test.args,
				cfg.minRelayTxFee, test.want)
		}
	}
}

// TestDeprecatedOptions ensures the options of the removed free and priority
// relay are still accepted, from the command line or the config file, and
// log a warning.
func TestDeprecatedOptions(t *testing.T) {
	defer func() { os.Args = []string{"bchd"} }()
	var logged bytes.Buffer
	savedLog := bchdLog
	bchdLog = bchlog.NewBackend(&logged).Logger("BCHD")
	defer func() { bchdLog = savedLog }()

	dir := t.TempDir()
	emptyConfig := filepath.Join(dir, "empty.conf")
	if err := os.WriteFile(emptyConfig, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, opt := range []struct{ name, arg, line string }{
		{"limitfreerelay", "--limitfreerelay=15", "limitfreerelay=15"},
		{"norelaypriority", "--norelaypriority", "norelaypriority=1"},
		{"blockminsize", "--blockminsize=1000000", "blockminsize=1000000"},
		{"blockprioritysize", "--blockprioritysize=50000",
			"blockprioritysize=50000"},
	} {
		want := "The --" + opt.name + " option is deprecated and has " +
			"no effect"
		fileConfig := filepath.Join(dir, opt.name+".conf")
		err := os.WriteFile(fileConfig, []byte(opt.line+"\n"), 0600)
		if err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{"--configfile=" + emptyConfig, opt.arg},
			{"--configfile=" + fileConfig},
		} {
			logged.Reset()
			os.Args = append([]string{"bchd", "--logdir=" + dir},
				args...)
			if _, _, err := loadConfig(); err != nil {
				t.Errorf("%v: unexpected error: %v", args, err)
				continue
			}
			if !strings.Contains(logged.String(), want) {
				t.Errorf("%v: no deprecation warning, logged %q",
					args, logged.String())
			}
		}
	}

	// Nothing is logged when none of them is set.
	logged.Reset()
	os.Args = []string{"bchd", "--logdir=" + dir,
		"--configfile=" + emptyConfig}
	if _, _, err := loadConfig(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(logged.String(), "deprecated") {
		t.Errorf("unexpected warning: %q", logged.String())
	}
}

// TestGrpcMaxFilterEntries ensures --grpcmaxfilterentries defaults to 100000,
// takes 0 and larger values, and refuses a negative one.
func TestGrpcMaxFilterEntries(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() { os.Args = []string{"bchd"} }()

	for _, test := range []struct {
		args    []string
		want    int
		wantErr bool
	}{
		{args: nil, want: 100000},
		{args: []string{"--grpcmaxfilterentries=500000"}, want: 500000},
		{args: []string{"--grpcmaxfilterentries=0"}, want: 0},
		{args: []string{"--grpcmaxfilterentries=-1"}, wantErr: true},
	} {
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		cfg, _, err := loadConfig()
		if test.wantErr {
			if err == nil {
				t.Errorf("%v: expected an error", test.args)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if cfg.GrpcMaxFilterEntries != test.want {
			t.Errorf("%v: got %d, want %d", test.args,
				cfg.GrpcMaxFilterEntries, test.want)
		}
	}
}

// TestGrpcMaxSubscriptions ensures --grpcmaxsubscriptions defaults to 100 and
// can be set.
func TestGrpcMaxSubscriptions(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() { os.Args = []string{"bchd"} }()

	for _, test := range []struct {
		args []string
		want int
	}{
		{nil, 100},
		{[]string{"--grpcmaxsubscriptions=5"}, 5},
	} {
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		cfg, _, err := loadConfig()
		if err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if cfg.GrpcMaxSubscriptions != test.want {
			t.Errorf("%v: got %d, want %d", test.args,
				cfg.GrpcMaxSubscriptions, test.want)
		}
	}
}

// TestGrpcMaxConcurrentCalls ensures the limit on gRPC calls defaults to 16,
// as many as BCHN's HTTP server runs at once, can be set, and may not be
// negative.
func TestGrpcMaxConcurrentCalls(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() { os.Args = []string{"bchd"} }()

	for _, test := range []struct {
		args []string
		want int
	}{
		{nil, 16},
		{[]string{"--grpcmaxconcurrentcalls=5"}, 5},
		{[]string{"--grpcmaxconcurrentcalls=-1"}, -1},
	} {
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		cfg, _, err := loadConfig()
		if test.want < 0 {
			if err == nil {
				t.Errorf("%v: no error", test.args)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if cfg.GrpcMaxConcurrentCalls != test.want {
			t.Errorf("%v: got %d, want %d", test.args,
				cfg.GrpcMaxConcurrentCalls, test.want)
		}
	}
}

// TestMaxPeers ensures --maxpeers defaults to 125, takes 0 and larger values,
// and refuses a negative one, with which the server could not make its peer
// queues and panicked at start.
func TestMaxPeers(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() { os.Args = []string{"bchd"} }()

	for _, test := range []struct {
		args    []string
		want    int
		wantErr string
	}{
		{args: nil, want: 125},
		{args: []string{"--maxpeers=0"}, want: 0},
		{args: []string{"--maxpeers=10"}, want: 10},
		{args: []string{"--maxpeers=-1"}, wantErr: "loadConfig: The " +
			"maxpeers option may not be less than 0 -- parsed [-1]"},
	} {
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		cfg, _, err := loadConfig()
		if test.wantErr != "" {
			if err == nil || err.Error() != test.wantErr {
				t.Errorf("%v: got error %v, want %s", test.args, err,
					test.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if cfg.MaxPeers != test.want {
			t.Errorf("%v: got %d, want %d", test.args, cfg.MaxPeers,
				test.want)
		}
	}
}

// TestSampleConfigEmbedded ensures that the sample configuration in bindata.go,
// which a first run writes out, is sample-bchd.conf.  make bindata updates it.
func TestSampleConfigEmbedded(t *testing.T) {
	embedded, err := Asset("sample-bchd.conf")
	if err != nil {
		t.Fatal(err)
	}
	sample, err := os.ReadFile("sample-bchd.conf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(embedded, sample) {
		t.Fatal("bindata.go is out of date: run make bindata")
	}
}

// TestNetworkSelection ensures each network option selects its params and
// data and log directories, and that two networks can't be chosen at once.
func TestNetworkSelection(t *testing.T) {
	// Keep the config file, data and logs out of the home directory.
	tmpDir := t.TempDir()
	defer func() {
		os.Args = []string{"bchd"}
		activeNetParams = &mainNetParams
	}()

	tests := []struct {
		args []string
		want *params // nil for a rejected combination
		dir  string
	}{
		{nil, &mainNetParams, "mainnet"},
		{[]string{"--testnet"}, &testNet3Params, "testnet"},
		{[]string{"--testnet4"}, &testNet4Params, "testnet4"},
		{[]string{"--chipnet"}, &chipNetParams, "chipnet"},
		{[]string{"--tempnet"}, &tempNetParams, "tempnet"},
		{[]string{"--regtest"}, &regressionNetParams, "regtest"},
		{[]string{"--simnet"}, &simNetParams, "simnet"},
		{[]string{"--tempnet", "--chipnet"}, nil, ""},
		{[]string{"--tempnet", "--testnet4"}, nil, ""},
		{[]string{"--chipnet", "--regtest"}, nil, ""},
	}
	for _, test := range tests {
		activeNetParams = &mainNetParams
		os.Args = append([]string{"bchd",
			"--configfile=" + filepath.Join(tmpDir, "bchd.conf"),
			"--datadir=" + tmpDir, "--logdir=" + tmpDir}, test.args...)
		cfg, _, err := loadConfig()
		if test.want == nil {
			want := "loadConfig: The testnet, testnet4, chipnet, " +
				"tempnet, regtest, and simnet params can't be used " +
				"together -- choose one of them"
			if err == nil || err.Error() != want {
				t.Errorf("%v: got error %v, want %s", test.args, err,
					want)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: %v", test.args, err)
			continue
		}
		if activeNetParams != test.want {
			t.Errorf("%v: got network %s, want %s", test.args,
				activeNetParams.Name, test.want.Name)
		}
		if want := filepath.Join(tmpDir, test.dir); cfg.DataDir != want ||
			cfg.LogDir != want {

			t.Errorf("%v: got data dir %s and log dir %s, want %s",
				test.args, cfg.DataDir, cfg.LogDir, want)
		}
	}

	// Tempnet shares chipnet's ports.
	if tempNetParams.DefaultPort != chipNetParams.DefaultPort ||
		tempNetParams.rpcPort != chipNetParams.rpcPort ||
		tempNetParams.gRRPPort != chipNetParams.gRRPPort {

		t.Errorf("tempnet ports %s %s %s, chipnet's %s %s %s",
			tempNetParams.DefaultPort, tempNetParams.rpcPort,
			tempNetParams.gRRPPort, chipNetParams.DefaultPort,
			chipNetParams.rpcPort, chipNetParams.gRRPPort)
	}
}
