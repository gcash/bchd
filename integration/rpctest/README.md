rpctest
=======

[![Build Status](https://github.com/gcash/bchd/actions/workflows/main.yml/badge.svg?branch=master)](https://github.com/gcash/bchd/actions/workflows/main.yml)
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/gcash/bchd/integration/rpctest)

Package rpctest provides a bchd-specific RPC testing harness crafting and
executing integration tests by driving a `bchd` instance via the `RPC`
interface. Each instance of an active harness comes equipped with a simple
in-memory HD wallet capable of properly syncing to the generated chain,
creating new addresses, and crafting fully signed transactions paying to an
arbitrary set of outputs.

This package was designed specifically to act as an RPC testing harness for
`bchd`. However, the constructs presented are general enough to be adapted to
any project wishing to programmatically drive a `bchd` instance of its
systems/integration tests.

## Installation and Updating

```bash
$ go get -u github.com/gcash/bchd/integration/rpctest
```

## License

Package rpctest is licensed under the [copyfree](http://copyfree.org) ISC
License.
