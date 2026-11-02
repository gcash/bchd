import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import { bchrpc, GetBlockchainInfoResponse_BitcoinNet } from "./gen/bchrpc_pb.js";

// The --grpclisten address of BCHD, set with BCHD_URL in Node.js.
const transport = createGrpcWebTransport({
  baseUrl: globalThis.process?.env.BCHD_URL ?? "https://localhost:8335",
});
const client = createClient(bchrpc, transport);

// Hashes are little-endian, reverse them for display.
const toHex = (hash) =>
  Array.from(hash).reverse().map((b) => b.toString(16).padStart(2, "0")).join("");

const info = await client.getBlockchainInfo({});
console.log("network:", GetBlockchainInfoResponse_BitcoinNet[info.bitcoinNet]);
console.log("height:", info.bestHeight, "tip:", toHex(info.bestBlockHash));

console.log("waiting for blocks...");
for await (const notification of client.subscribeBlocks({})) {
  const block = notification.block.value;
  console.log("block:", block.height, toHex(block.hash));
}
