bchd
====
[![Build Status](https://github.com/gcash/bchd/actions/workflows/main.yml/badge.svg?branch=master)](https://github.com/gcash/bchd/actions/workflows/main.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/gcash/bchd)](https://goreportcard.com/report/github.com/gcash/bchd)
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/gcash/bchd)

bchd is an alternative full node bitcoin cash implementation written in Go (golang).

This project is a port of the [btcd](https://github.com/btcsuite/btcd) codebase to Bitcoin Cash. It provides a high powered
and reliable blockchain server which makes it a suitable backend to serve blockchain data to lite clients and block explorers
or to power your local wallet.

bchd does not include any wallet functionality by design as it makes the codebase more modular and easy to maintain.
The [bchwallet](https://github.com/gcash/bchwallet) is a separate application that provides a secure Bitcoin Cash wallet
that communicates with your running bchd instance via the API.

## Table of Contents

- [Requirements](#requirements)
- [Install](#install)
  - [Install prebuilt packages](#install-pre-built-packages)
  - [Build from Source](#build-from-source)
- [Getting Started](#getting-started)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Requirements

[Go](http://golang.org) 1.23.4 or newer.

## Install

### Install Pre-built Packages

The easiest way to run the server is to download a pre-built binary. You can find binaries of our latest release for each operating system at the [releases page](https://github.com/gcash/bchd/releases).

### Build from Source

If you prefer to install from source do the following:

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Run the following commands to obtain bchd, all dependencies, and install it:

```bash
go install github.com/gcash/bchd@v0.20.0-rc2
```

This will download the source code into your GOPATH and compile `bchd` and install it in your path.

For developers if you wish to place the working directory outside your GOPATH you can do so with Go >=1.12.x as follows:
```bash
mkdir workspace
cd workspace
git clone https://github.com/gcash/bchd.git
cd bchd
go install (or build or run, etc)
```
Dependencies will be automatically installed to `$GOPATH/pkg/mod`.

If you are a bchd contributor and would like to change the default config file (`bchd.conf`), make any changes to `sample-bchd.conf` and then run the following commands:

```bash
go-bindata sample-bchd.conf  # requires github.com/go-bindata/go-bindata/
gofmt -s -w bindata.go
```

## Getting Started

To start bchd with default options just run:

```bash
./bchd
```

You'll find a large number of runtime options with the help flag. All of them can also be set in a config file.
See the [sample config file](https://github.com/gcash/bchd/blob/master/sample-bchd.conf) for an example of how to use it.

```bash
./bchd --help
```

You can use the common json RPC interface through the `bchctl` command:

```bash
./bchctl --help

./bchctl --listcommands
```

Bchd separates the node and the wallet. Commands for the wallet will work when you are also running
[bchwallet](https://github.com/gcash/bchwallet):

```bash
./bchctl -u username -P password --wallet getnewaddress
```

## Docker

Building and running `bchd` in docker is quite painless. To build the image:

```
docker build . -t bchd
```

To run the image:

```
# Use stop-timeout to make sure the container exits cleanly!
docker run --stop-timeout=1200 bchd
```

To run `bchctl` and connect to your `bchd` instance:

```
# Find the running bchd container.
docker ps

# Exec bchctl.
docker exec <container> bchctl <command>
```

## Documentation

The documentation is a work-in-progress.  It is located in the [docs](https://github.com/gcash/bchd/tree/master/docs) folder.

## Contributing

Contributions are definitely welcome! Please read the contributing [guidelines](https://github.com/gcash/bchd/blob/master/docs/code_contribution_guidelines.md) before starting.

## Security Disclosures

To report security issues please contact:

Chris Pacia (ctpacia@gmail.com) - GPG Fingerprint: 0150 2502 DD3A 928D CE52 8CB9 B895 6DBF EE7C 105C

or

Josh Ellithorpe (quest@mac.com) - GPG Fingerprint: B6DE 3514 E07E 30BB 5F40  8D74 E49B 7E00 0022 8DDD

## License

bchd is licensed under the [copyfree](http://copyfree.org) ISC License.
