import { createRpcClient, sleep } from '../lib/utils';

// setup RPC clients (used primarily for generating blocks only)
const bchd2Rpc = createRpcClient();

// use a floating promise to generate 1 block each second
(async () => {
  let counter = 0;
  while (counter < 1000) {
    counter++;
    await bchd2Rpc.generate(1);
    await sleep(1000);
  }
})();
