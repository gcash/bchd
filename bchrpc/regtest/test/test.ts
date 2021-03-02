import { step } from 'mocha-steps';
import * as assert from "assert";
import { GetBlockchainInfoResponse, GrpcClient } from "grpc-bchrpc-node";
import { PrivateKey, Networks } from "bitcore-lib-cash";
import * as bchaddrjs from "bchaddrjs-slp";
import { BitcoinRpcClient } from "./utils/rpc";

// setup RPC clients (for mining)
const rpcClient = require('bitcoin-rpc-promise');
const bchd1Rpc = new rpcClient('http://bitcoin:password@0.0.0.0:18336') as BitcoinRpcClient;
const bchd2Rpc = new rpcClient('http://bitcoin:password@0.0.0.0:18334') as BitcoinRpcClient;

// setup gRPC client (for slp index info)
const bchd1Grpc = new GrpcClient({ url: "localhost:18335", rootCertPath: "./rpc.bchd1.cert", testnet: true });

// private key for the mining address (address is stored in bchd.conf)
//
// NOTE: bchd doesn't have generatetoaddress, only generate is available.
//
const privKey1 = new PrivateKey("cPgxbS8PaxXoU9qCn1AKqQzYwbRCpizbsG98xU2vZQzyZCJt4NjB", Networks.testnet);
const wallet1 = {
  _privKey: privKey1,
  address: bchaddrjs.toRegtestAddress(privKey1.toAddress().toString()),
  wif: privKey1.toWIF(),
  pubKey: privKey1.toPublicKey()
};wallet1

describe("network health check", () => {
  step("bchd1 ready", async () => {
    const info1 = await bchd1Grpc.getBlockchainInfo();
    assert.strictEqual(info1.getBitcoinNet(), GetBlockchainInfoResponse.BitcoinNet.REGTEST);
    console.log(`bchd1 on block ${info1.getBestHeight()}`);

    let res = await bchd2Rpc.getPeerInfo();
    assert.strictEqual(typeof res, "object");
    assert.ok(res.length == 1);
  
    let info2 = await bchd2Rpc.getBlockchainInfo();
    console.log(`bchd2 on block ${info2.blocks}`);

    assert.strictEqual(info1.getBestHeight(), info2.blocks);
  });
});

describe("basic tests", async () => {
  step("generate block to address", async () => {

    // get balance for address
    let resBal = await bchd1Grpc.getAddressUtxos({ address: wallet1.address });
    while (resBal.getOutputsList().length === 0) {
      let txids = await bchd1Rpc.generate(1);
      resBal = await bchd1Grpc.getAddressUtxos({ address: wallet1.address });
    }
    console.log(`${resBal.getOutputsList().length} outputs (balance: ${resBal.getOutputsList().reduce((p,c,i) => p += c.getValue() / 10**8, 0)} TBCH)`);

    assert.ok(1);
  });

  // step("submit an slp genesis transaction", async () => {
  //   // todo...
  //   assert.ok(0);
  // });

  // step("submit an slp send transaction", async () => {
  //   // todo...
  //   assert.ok(0);
  // });

  // step("submit an slp mint transaction", async () => {
  //   // todo...
  //   assert.ok(0);
  // });
});
