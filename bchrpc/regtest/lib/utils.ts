import { GrpcClient } from "./grpc";
import fs from "fs";
import { BitcoinRpc, BitcoinRpcClient } from "./rpc";

export const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

export const createGrpcClient = () => {
  try {
    // this will throw if not in a container
    return new GrpcClient({ url: "bchd1:18335", rootCertPath: "/data/rpc.bchd1.cert" });
  } catch (_) {
    // bchd1's certificate is for the name bchd1.
    return new GrpcClient({ url: "localhost:18335", rootCertPath: "./rpc.bchd1.cert", serverName: "bchd1" });
  }
};

export const createRpcClient = () => {
  try {
    // this will throw if not in a container
    fs.readFileSync("/data/rpc.bchd1.cert");
    return new BitcoinRpc('http://bitcoin:password@bchd2:18334') as BitcoinRpcClient;
  } catch (_) {
    return new BitcoinRpc('http://bitcoin:password@0.0.0.0:18334') as BitcoinRpcClient;
  }
};