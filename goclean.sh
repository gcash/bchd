#!/bin/bash
# The script does automatic checking on a Go package and its sub-packages, including:
# 1. gofmt         (http://golang.org/cmd/gofmt/)
# 2. golint        (https://github.com/golang/lint)
# 3. go vet        (http://golang.org/cmd/vet)
# 4. gosimple      (https://github.com/dominikh/go-simple)
# 5. unconvert     (https://github.com/mdempsky/unconvert)
#
# gometalinter.v2 (gopkg.in/alecthomas/gometalinter.v2) is used to run each static
# checker.

set -ex

# Make sure golangci-lint is installed and $GOPATH/bin is in your path.
if [ ! -x "$(type -p golangci-lint)" ]; then
  exit 1
fi

# Automatic checks
test -z "$(env GO111MODULE=on golangci-lint run --disable-all \
--enable=gofmt \
--enable=golint \
--enable=vet \
--enable=gosimple \
--enable=unconvert \
--deadline=10m \
--exclude="OP_" \
--exclude="ALL_CAPS" \
2>&1 | tee /dev/stderr)"
env GO111MODULE=on go test -tags rpctest ./...
