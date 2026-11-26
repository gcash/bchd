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

# The bchrpc/proxy tests skip those that need the remote BCHD node when it
# doesn't answer a call, and go test shows skips only with -v, so that package
# is tested on its own with -v.  The reason for a skip and its SKIP, ok and
# FAIL lines are shown, or all of its output when it fails.
proxy=github.com/gcash/bchd/bchrpc/proxy
env GO111MODULE=on go test -tags rpctest $(env GO111MODULE=on go list -tags rpctest ./... | grep -vx "$proxy")
proxy_log=$(mktemp)
if ! env GO111MODULE=on go test -tags rpctest -v "$proxy" >"$proxy_log" 2>&1; then
  cat "$proxy_log"
  exit 1
fi
grep -E "BCHD node .* doesn't answer, skipping|^ *--- (SKIP|FAIL)|^(ok|FAIL)" "$proxy_log" || true
rm "$proxy_log"
