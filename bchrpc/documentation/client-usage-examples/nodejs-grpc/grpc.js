// Example use of BCHD's gRPC bchrpc with nodejs. 

var PROTO_PATH = __dirname + '/bchrpc.proto';

var grpc = require('@grpc/grpc-js');
var protoLoader = require('@grpc/proto-loader');
var packageDefinition = protoLoader.loadSync(
    PROTO_PATH,
    {keepCase: true,
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


// Get a raw tx from tx hash

// Change endianness of tx hashes and convert the hex string to a
// byte array because bchd only handles the byte representation of hashes 
// and treats all byte arrays as little endian 
const changeEndianness = (string) => {
    const result = [];
    let len = string.length - 2;
    while (len >= 0) {
      result.push(string.substr(len, 2));
      len -= 2;
    }
    return result.join('');
}

var hex =  changeEndianness("fe58d09c218d6ea1a0d1ce726d1c5aa6e9c01a9e760aab621484aa21b1f673fb")
var bytes = []

for(var i=0; i< hex.length-1; i+=2){
    bytes.push(parseInt(hex.substr(i, 2), 16));
}

var getRawTransactionRequest = pb.GetRawTransactionRequest
getRawTransactionRequest.hash = bytes

client.GetRawTransaction(getRawTransactionRequest, function(error, resp) {
    if (error) {
        console.log("Error: " + error.code + ": " + error.message)
        console.log(error)
    } else {
    var tx = resp
    console.log("\nGetRawTransaction:")
    console.log(tx)
    }
});


// Get deserialized tx from tx hash
var getTransactionRequest = pb.GetTransactionRequest
getTransactionRequest.hash = bytes

client.GetTransaction(getTransactionRequest, function(error, resp) {
    if (error) {
        console.log("Error: " + error.code + ": " + error.message)
        console.log(error)
    } else {
    var tx = resp
    console.log("\nGetTransaction:")
    console.log(tx)
    }
});


// Build TransactionFilter & setup live transaction stream
var transactionFilter = pb.TransactionFilter
transactionFilter.all_transactions = true

var subscribreTransactionsRequest = pb.SubscribeTransactionsRequest
subscribreTransactionsRequest.include_mempool = true
subscribreTransactionsRequest.subscribe = transactionFilter

var stream = client.SubscribeTransactions(subscribreTransactionsRequest)

stream.on('data', function(message) {
    var tx = message
    console.log("\nSubscribeTransactions stream:")
    console.log(tx.unconfirmed_transaction.transaction.hash)
});
stream.on('status', function(status) {
    console.log(status)
});
stream.on('end', function(status) {
    console.log(status)
});