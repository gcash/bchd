# Server Configuration

Running the gRPC server is done via the command line or config file options.

To run the gRPC server on localhost:
```bash
bchd --grpclisten=127.0.0.1
```

By default it will run on port 8335 on mainnet (18335 on testnet and regtest). If you wish
to change the port you can do so:

```bash
bchd --grpclisten=127.0.0.1:443
```

To bind to all interfaces and allow anyone to connect use:
```bash
bchd --grpclisten=0.0.0.0
```

It should be noted that Go's HTTP/2 implementation does not support cleartext connections. So TLS must always
be used. By default bchd's self signed cert (found in the data directory) will be used. If you wish to use
an actual TLS certificate signed by a valid CA then you can use the following options:

```bash
bchd  --grpclisten=0.0.0.0 --rpccert=/path/to/your/cert --rpckey=/path/to/your/key
```

To add authentication to the server use:
```bash
bchd --grpclisten=127.0.0.1 --grpcauthtoken=your_token_here
```

The authentication token will need to be sent by the client with each request as part of the context metadata. 
The key is `AuthenticationToken`. For example in Go:
```go
md := metadata.Pairs("AuthenticationToken", "your_token_here")
ctx := metadata.NewOutgoingContext(context.Background(), md)

// Make the RPC
response, err := client.SomeRPC(ctx, someRequest)
```

Clients may hold at most 100 subscriptions at once, over all the subscription calls, and a
call over the limit fails with `ResourceExhausted`. A public server with more subscribers can
raise it:

```bash
bchd --grpclisten=0.0.0.0 --grpcmaxsubscriptions=1000
```

A subscription's transaction filter may hold at most 100,000 entries, an address, outpoint or
token ID counting once and a data element once for each 20 bytes, and a request that takes a
filter past that fails with `ResourceExhausted`. A server for wallets that watch more can raise
it with `--grpcmaxfilterentries`.

At most 16 other calls are in progress at once, from reading the request until the reply is
ready to send. Up to 64 more wait for a place, and a call beyond those fails with
`ResourceExhausted`. A public server with many clients can raise the limit:

```bash
bchd --grpclisten=0.0.0.0 --grpcmaxconcurrentcalls=64
```

Finally you don't need to use indexes to use the gRPC API but it's recommended so
you have access to all API calls. This obviously means you should not run in either
`--prune` mode or `--fastsync` mode. To use the indexes run with:

```bash
bchd --grpclisten=0.0.0.0 --txindex --addrindex
```