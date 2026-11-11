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

A harness starts `bchd` with the May 2027 upgrade to 1-minute blocks set to
never activate, so tests don't change rules with the date. A test passes
`rpctest.Upgrade2027Arg(rpctest.Upgrade2027InThePast)` among the extra
arguments of `rpctest.New` to mine a chain that activates it within its first
few blocks.

The `BCHD_RPCTEST_UPGRADE2027` environment variable changes the default of the
harnesses whose test does not choose a time: `never` (the default), `past` for
`rpctest.Upgrade2027InThePast`, or a time in seconds.  Run the integration tests
under both rule sets with:

```bash
$ go test -tags rpctest ./integration/...
$ BCHD_RPCTEST_UPGRADE2027=past go test -tags rpctest ./integration/...
```

## Installation and Updating

```bash
$ go get -u github.com/gcash/bchd/integration/rpctest
```

## License

Package rpctest is licensed under the [copyfree](http://copyfree.org) ISC
License.
