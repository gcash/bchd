# Web

It is currently impossible to implement the HTTP/2 gRPC spec3 in the browser, as there is simply no browser API with enough fine-grained control over the requests. For example: there is no way to force the use of HTTP/2, and even if there was, raw HTTP/2 frames are inaccessible in browsers.

For this reason the gRPC team developed [gRPC-Web](https://github.com/grpc/grpc-web) which uses a modified gRPC compatible with browsers.

Traditionally using gRPC-Web requires running a proxy server that translates between gRPC and gRPC-Web,
however in bchd we are multiplexing both protocols over the same address. The server determines which protocol to use based on the incoming
HTTP headers.

The one major difference from a client perspective is gRPC-Web does not support bi-directional streaming. This
means that the `SubscribeTransactionStream` RPC cannot be used by the browser. Instead the server provides a `SubscribeTransactions`
RPC with the same functionality, but only using server side streaming. When using this endpoint you'll need
to close and re-open the stream every time you want to update your `TransactionFilter`.

A gRPC-Web module can be [found on npm](https://www.npmjs.com/package/grpc-web). It supports both Javascript and TypeScript. 