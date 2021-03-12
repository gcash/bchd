import { GrpcClient } from "grpc-bchrpc-node";
import fs from "fs";
import { BitcoinRpcClient } from "./rpc";
const rpcClient = require('bitcoin-rpc-promise');

export const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

export const createGrpcClient = () => {
  try {
    // this will throw if not in a container
    return new GrpcClient({ url: "bchd1:18335", rootCertPath: "/data/rpc.bchd1.cert", testnet: true });
  } catch (_) {
    return new GrpcClient({ url: "localhost:18335", rootCertPath: "./rpc.bchd1.cert", testnet: true });
  }
};

export const createRpcClient = () => {
  try {
    // this will throw if not in a container
    fs.readFileSync("/data/rpc.bchd1.cert");
    return new rpcClient('http://bitcoin:password@bchd2:18334') as BitcoinRpcClient;
  } catch (_) {
    return new rpcClient('http://bitcoin:password@0.0.0.0:18334') as BitcoinRpcClient;
  }
};