# Client usage

Clients use gRPC to interact with the node.  A client may be implemented in any
language directly supported by [gRPC](http://www.grpc.io/), languages capable of
performing [FFI](https://en.wikipedia.org/wiki/Foreign_function_interface) with
these, and languages that share a common runtime (e.g. Scala, Kotlin, and Ceylon
for the JVM, F# for the CLR, etc.).  Exact instructions differ slightly
depending on the language being used, but the general process is the same for
each.  In short summary, to call RPC server methods, a client must:

1. Generate the language-specific client bindings using the `protoc` compiler and [bchrpc.proto](../bchrpc.proto)
2. Import or include the gRPC dependency
3. (Optional) Wrap the client bindings with application-specific types
4. Open a gRPC channel using the server's self-signed TLS certificate or a valid TLS certificate.

The only exception to these steps is if the client is being written in Go.  In
that case, the first step may be omitted by importing the bindings from
bchd itself.

The rest of this document provides short examples of how to quickly get started
by implementing a basic client from a testnet3 server listening on `localhost:18335` in several
different languages:

- [Go](#go)
- [Node.js](#node.js)
- [Python](#python)

Unless otherwise stated under the language example, it is assumed that
gRPC is already installed.  The gRPC installation procedure
can vary greatly depending on the operating system being used and
whether a gRPC source install is required.  Follow the [gRPC install
instructions](https://github.com/grpc/grpc/blob/master/INSTALL) if
gRPC is not already installed.  A full gRPC install also includes
[Protocol Buffers](https://github.com/google/protobuf) (compiled with
support for the proto3 language version), which contains the protoc
tool and language plugins used to compile this project's `.proto`
files to language-specific bindings.

## TLS
By default bchd uses a self signed certificate to encrypt and authenticate the
connection. To authenticate against the server the client will need access to the
certificate. For example, in Go:
```go
certificateFile := filepath.Join(bchutil.AppDataDir("bchwallet", false), "rpc.cert")
creds, err := credentials.NewClientTLSFromFile(certificateFile, "localhost")
if err != nil {
    fmt.Println(err)
    return
}
tlsOption := grpc.WithTransportCredentials(creds)
```

If the server is using a certificate signed by a valid certificate authority just use nil for the cert:
```go
tlsOption := gprc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")
```

## Authentication

The server may require client authentication via an auth token. This token must be provided with each request as part of the context metadata. 
The key is `AuthenticationToken`. For example in Go:
```go
md := metadata.Pairs("AuthenticationToken", "auth_token_here")
ctx := metadata.NewOutgoingContext(context.Background(), md)

// Make the RPC
response, err := client.SomeRPC(ctx, someRequest)
```

## Why Are Transaction IDs Backwards?
Bitcoin was originally coded by Satoshi in a somewhat quirky way. When a transaction ID is in byte format in memory it is
stored in little endian format (the bytes are in reverse order). When the ID is converted into a hex string the ID is reversed into
big endian format (the format you're used to seeing on block explorers). The bchd codebase continues this behavior. Byte arrays = little 
endian, hex strings = big endian. Because we send transaction IDs as byte arrays in gRPC, they are in little endian format. To get to
the familiar format you're used to simply reverse them.

The bchd library offers a `chainhash` function which can do this for you if you're using Go:
```go
hash, _ := chainhash.NewHash(txBytes) // Expects the bytes to be in little endian format.

fmt.Println(hash.String()) // Prints a hex string in big endian format.
```

## Go

The native gRPC library (gRPC Core) is not required for Go clients (a
pure Go implementation is used instead) and no additional setup is
required to generate Go bindings.

```Go
package main

import (
	"fmt"
	"path/filepath"

	pb "github.com/gcash/bchd/bchrpc/pb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gcash/bchutil"
)

var certificateFile = filepath.Join(bchutil.AppDataDir("bchd", false), "rpc.cert")

func main() {
	creds, err := credentials.NewClientTLSFromFile(certificateFile, "localhost")
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := grpc.Dial("localhost:18332", grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	c := pb.NewBchrpcClient(conn)
	
	blockchainInfoResp, err := c.GetBlockchainInfo(context.Background(), &pb.GetBlockchainInfoRequest{})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Blockchain Height: ", blockchainInfoResp.BestHeight)
}
```

## Node.js

```javascript
var PROTO_PATH = __dirname + '/bchrpc.proto';

var grpc = require('@grpc/grpc-js');
var protoLoader = require('@grpc/proto-loader');
var packageDefinition = protoLoader.loadSync(
    PROTO_PATH,
    {
        keepCase: true,
        longs: String,
        enums: String,
        defaults: true,
        oneofs: true
    });

var pb = grpc.loadPackageDefinition(packageDefinition).pb;
var client = new pb.bchrpc('bchd.greyh.at:8335', grpc.credentials.createSsl());


// Get current state of the mempool
client.GetMempoolInfo(pb.MempoolInfoRequest, function(error, resp) {
    if (error) {
        console.log("Error: " + error.code + ": " + error.message)
        console.log(error)
    } else {
    var mempool = resp
    console.log("\nGetMempoolInfo:")
    console.log(mempool)
    }
});
```
If connecting to a node using a self signed cert you will need to use:
```javascript
export NODE_TLS_REJECT_UNAUTHORIZED=0
```
**More Examples**: [here](https://github.com/gcash/bchd/tree/master/bchrpc/documentation/client-usage-examples/nodejs-grpc)

## Python

### Protoc

Install dependencies
```
python -m pip install grpcio
python -m pip install grpcio-tools
```

Generate libs
```
python -m grpc_tools.protoc -I=./ --python_out=./pb-py --grpc_python_out=./pb-py ./bchrpc.proto
```

### Example
```python
#!/usr/bin/env python3

import grpc
import bchrpc_pb2 as pb
import bchrpc_pb2_grpc as bchrpc

def run():
    with grpc.secure_channel('bchd.greyh.at:8335', grpc.ssl_channel_credentials()) as channel:
        
        # Get MempoolInfo
        stub = bchrpc.bchrpcStub(channel)
        req = pb.GetMempoolInfoRequest()

        resp = stub.GetMempoolInfo(req)

        print("mempool")
        print(resp)

        # Get block and parse tx hashes contained in block
        req = pb.GetBlockRequest()
        req.height = 555555
        # req.full_transactions = True

        resp = stub.GetBlock(req)

        hash = resp.block.info.hash
        hash = bytearray(hash[::-1])
        print("blockhash: " + hash.hex())

        for tx in resp.block.transaction_data:
            txhash = bytearray(tx.transaction_hash[::-1])
            print(txhash.hex())


if __name__ == '__main__':
    run()
```
With Python is may be difficult to setup gRPC with self-signed certs.
We highly recommend getting a certificate from Let's Encrypt.

TODO: Add examples in other languages
