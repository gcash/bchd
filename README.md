bchd
====
[![Build Status](https://travis-ci.org/gcash/bchd.png?branch=master)](https://travis-ci.org/gcash/bchd)
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

[Go](http://golang.org) 1.8 or newer.

## Install

### Install Pre-built Packages

The easiest way to run the server is to download a pre-built binary. You can find binaries of our latest release for each operating system at the [releases page](https://github.com/gcash/bchd/releases).

### Build from Source

If you prefer to install from source do the following:

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Run the following commands to obtain btcd, all dependencies, and install it:

```bash
$ go get github.com/gcash/bchd
```

This will download and compile `bchd` and put it in your path.

## Getting Started

To start bchd with default options just run:

```bash
$ ./bchd
```

You'll find a large number of runtime options on the help menu. All of which can also be set in a config file.
See the [sample config file](https://github.com/gcash/bchd/blob/master/sample-bchd.conf) for an example of how to use it.

## Docker

Building and running `bchd` in docker is quite painless. To build the image:

```
docker build . -t bchd
```

To run the image:

```
docker run bchd
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

## License

bchd is licensed under the [copyfree](http://copyfree.org) ISC License.
