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

To bind to all interfaces use and allow anyone to connect use:
```bash
bchd --grpclisten=0.0.0.0
```

By default TLS is always used. It will use the `rpc.cert` file found in the bchd data directory.
If you want to disable TLS you may do so but only if the server is running on localhost:

```bash
bchd --grpclisten=127.0.0.1 --notls
```

To use your own TLS certificate use:
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