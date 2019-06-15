# NodeJS - gRPC Instructions & Examples

## Prereqs

* It is advised to use the `@grpc/grpc-js` client package from the [github.com/grpc/grpc-node](https://github.com/grpc/grpc-node/) project for NodeJS development. [Project Page](https://github.com/grpc/grpc-node/tree/master/packages/grpc-js)
* The NodeJS gRPC library conveniently generates the client code at runtime, you need the following package to do that: `@grpc/proto-loader` [Project Page](https://github.com/grpc/grpc-node/tree/master/packages/proto-loader)
* Download the protobuf service definition file [
bchd/bchrpc/bchrpc.proto ](https://raw.githubusercontent.com/gcash/bchd/master/bchrpc/bchrpc.proto) 


```
npm install @grpc/grpc-js 
npm install @grpc/proto-loader
wget https://raw.githubusercontent.com/gcash/bchd/master/bchrpc/bchrpc.proto
```

## Examples

The example code will show you how to:

* How to get the mempool state with a simple `GetMempoolInfo` request
* How to convert a tx hash string to bin and change the edianness in order to use it as a request parameter for `GetRawTransactionRequest` and `GetTransactionRequest`
* How to define a `TransactionFilter`
* How to setup a live transaction stream with `SubscribeTransactions`

### Example code

[grpc.js](https://github.com/gcash/bchd/blob/master/bchrpc/documentation/client-usage-examples/nodejs-grpc/grpc.js)

### Run example

```
git clone https://github.com/gcash/bchd.git
cd bchd/bchrpc/documentation/client-usage-examples/nodejs-grpc/
node grpc.js
```