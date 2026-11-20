integration
===========

[![Build Status](https://github.com/gcash/bchd/actions/workflows/main.yml/badge.svg?branch=master)](https://github.com/gcash/bchd/actions/workflows/main.yml)
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)

This contains integration tests which make use of the
[rpctest](https://github.com/gcash/bchd/tree/master/integration/rpctest)
package to programmatically drive nodes via RPC.

Run them with `go test -tags rpctest ./integration/...`, and again with
`BCHD_RPCTEST_UPGRADE2027=past` set to run them with 1-minute blocks.

`TestBCHNInterop` (build tag `interop`) runs bchd beside a Bitcoin Cash Node
regtest node, mines across the seam of the 1-minute block upgrade on both and
checks that they agree.  It needs a `bitcoind` with 1-minute block support:

```bash
$ BCHN_BITCOIND=/path/to/bitcoind go test -tags interop -run TestBCHNInterop ./integration/
```

## License

This code is licensed under the [copyfree](http://copyfree.org) ISC License.
