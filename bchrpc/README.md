bchrpc
=======

[![Build Status](https://github.com/gcash/bchd/actions/workflows/main.yml/badge.svg?branch=master)](https://github.com/gcash/bchd/actions/workflows/main.yml)
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/gcash/bchd/bchrpc)

Package bchrpc implements a gRPC server.

## Overview

This package provides a gRPC API which when combined with the `addrindex` and `txindex` provides
a powerful API for supporting Bitcoin Cash applications. At present the API only exposes public
methods for interacting with transactions and blocks. It does not expose any methods which can
control the node so it is safe to expose the API publicly. To control the node continue using
the JSON-RPC API.

## Why gRPC?

With gRPC it is extremely easy to build well-defined, easy to reason about APIs. Frontend development changes significantly:

 * no more hunting down API documentation - `.proto` is the canonical format for API contracts.
 * no more hand-crafted JSON call objects - all requests and responses are strongly typed and code-generated, with hints available in the IDE.
 * no more dealing with methods, headers, body and low level networking - everything is handled by gRPC.
 * no more second-guessing the meaning of error codes - [gRPC status codes](https://godoc.org/google.golang.org/grpc/codes) are a canonical way of representing issues in APIs.
 * no more one-off server-side request handlers to avoid concurrent connections - gRPC is based on HTTP2, which multiplexes multiple streams over the [same connection](https://hpbn.co/http2/#streams-messages-and-frames).
 * no more problems streaming data from a server -  gRPC-Web supports both *1:1* RPCs and *1:many* streaming requests.
 * no more data parse errors when rolling out new binaries - [backwards and forwards-compatibility](https://developers.google.com/protocol-buffers/docs/gotutorial#extending-a-protocol-buffer) of requests and responses.

In short, gRPC moves the interaction between frontend code and the server from the sphere of hand-crafted HTTP requests to well-defined user-logic methods.

## Using the API

```bash
$ bchd --grpclisten=<your_interface>
```

## License

Package bchrpc is licensed under the [copyfree](http://copyfree.org) ISC License.
