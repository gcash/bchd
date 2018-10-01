package integration

import (
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/integration/rpctest"
	"testing"
)

func TestCTORActivation(t *testing.T) {
	t.Parallel()

	bchdCfg := []string{"--rejectnonstd"}
	params := chaincfg.SimNetParams
	params.MagneticAnomalyActivationTime = 0
	r, err := rpctest.New(&params, nil, bchdCfg)
	if err != nil {
		t.Fatal("unable to create primary harness: ", err)
	}
	if err := r.SetUp(true, 10); err != nil {
		t.Fatalf("unable to setup test chain: %v", err)
	}
	defer r.TearDown()

}
