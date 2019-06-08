bchrpc
=======

[![Build Status](https://travis-ci.org/gcash/bchd.png?branch=master)](https://travis-ci.org/gcash/bchd)
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/gcash/bchd/bchrpc)

Package bchrpc implements a gRPC server.

## Overview

This package provides a gRPC which when combined with the `addrindex` and `txindex` provides
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
 * no more one-off server-side request handlers to avoid concurrent connections - gRPC is based on HTTP2, with multiplexes multiple streams over the [same connection](https://hpbn.co/http2/#streams-messages-and-frames).
 * no more problems streaming data from a server -  gRPC-Web supports both *1:1* RPCs and *1:many* streaming requests.
 * no more data parse errors when rolling out new binaries - [backwards and forwards-compatibility](https://developers.google.com/protocol-buffers/docs/gotutorial#extending-a-protocol-buffer) of requests and responses.

In short, gRPC moves the interaction between frontend code and the server from the sphere of hand-crafted HTTP requests to well-defined user-logic methods.

## Using the API

```bash
$ bchd --grpclisten=<your_interface>
```

## Serving bchrpc API through an NGINX reverse proxy (optional)
For various reasons, like load balancing, ssl handling, etc. It might be handy to serve the bchrpc API thourgh a reverse proxy, although not necessary. Here we provide a sample config for NGINX.

### Upstream
With load balancing
```
upstream bchrpc {
    ip_hash; # Session persistence: make same client always connect to same server
    server bchd01.bitcoin.cash:8335;
    server bchd02.bitcoin.cash:8335;
}
```

Without load balancing
```
upstream bchrpc {
    server bchd01.bitcoin.cash:8335;
}
```

### Location
```
location / {
        # Raise default timeout because blocks can take longer than 10 minutes (600 seconds) in between, this causes a timeout on SubscribeBlocks stream
        proxy_connect_timeout       3600;
        proxy_send_timeout          3600;
        proxy_read_timeout          3600;
        send_timeout                3600;
        # grpc requires http/2
        http2_push_preload          on;
        proxy_hide_header           X-Frame-Options;
        proxy_http_version          1.1;
        proxy_set_header            Upgrade $http_upgrade;
        proxy_set_header            Connection "upgrade";
        proxy_set_header            X-Real-IP $remote_addr;
        proxy_set_header            Host $http_host;
        proxy_set_header            X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header            X-Forwarded-Proto $scheme;
        proxy_redirect              off;

        proxy_pass https://bchrpc;
    }
```





## License

Package bchrpc is licensed under the [copyfree](http://copyfree.org) ISC License.
