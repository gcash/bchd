#!/bin/bash
# The script does automatic checking on a Go package and its sub-packages, including:
# 1. gofmt         (http://golang.org/cmd/gofmt/)
# 2. golint        (https://github.com/golang/lint)
# 3. go vet        (http://golang.org/cmd/vet)
# 4. gosimple      (https://github.com/dominikh/go-simple)
# 5. unconvert     (https://github.com/mdempsky/unconvert)
#
# golangci-lint (https://github.com/golangci/golangci-lint) is used to run each static
# checker.

set -ex

# Make sure golangci-lint is installed and $GOPATH/bin is in your path.
if [ ! -x "$(type -p golangci-lint)" ]; then
  exit 1
fi

# Automatic checks
env GO111MODULE=on golangci-lint run
# The tests that need a BCHN node or a chain dump are only vetted, so that
# they keep compiling.
env GO111MODULE=on go vet -tags interop ./integration/
env GO111MODULE=on go vet -tags replay ./blockchain/

# The bchrpc/proxy tests skip those that need the remote BCHD node when it
# can't be reached, and go test shows skips only with -v, so that package is
# tested on its own with -v.  Its SKIP, ok and FAIL lines are shown, or all of
# its output when it fails.
proxy=github.com/gcash/bchd/bchrpc/proxy
env GO111MODULE=on go test -tags rpctest $(env GO111MODULE=on go list -tags rpctest ./... | grep -vx "$proxy")
# The integration tests run a second time with the upgrade to 1-minute blocks
# active from the first blocks of each harness that doesn't choose a time.
env GO111MODULE=on BCHD_RPCTEST_UPGRADE2027=past go test -tags rpctest ./integration/...
proxy_log=$(mktemp)
if ! env GO111MODULE=on go test -tags rpctest -v "$proxy" >"$proxy_log" 2>&1; then
  cat "$proxy_log"
  exit 1
fi
grep -E '^ *--- (SKIP|FAIL)|^(ok|FAIL)' "$proxy_log" || true
rm "$proxy_log"
