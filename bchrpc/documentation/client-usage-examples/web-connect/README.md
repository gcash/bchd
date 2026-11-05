# Browser - gRPC-Web with Connect-ES

`index.js` calls BCHD over gRPC-Web with [Connect-ES](https://connectrpc.com/docs/web/getting-started): it gets the
chain tip with `GetBlockchainInfo` and then prints each new block from the `SubscribeBlocks` stream. See
[web.md](../../web.md) for what a browser needs from BCHD.

The code only uses `fetch`, so it runs in Node.js and, bundled with any bundler, in a browser.

## Install and generate the code

The code for `bchrpc.proto` is generated into `gen/` with [buf](https://buf.build/docs/) and
`@bufbuild/protoc-gen-es`, see `buf.gen.yaml`.

```
cd bchd/bchrpc/documentation/client-usage-examples/web-connect/
npm install
npm run generate
```

## Run example

BCHD serves gRPC-Web on its `--grpclisten` address, over TLS. If the node uses its self signed cert, point
`NODE_EXTRA_CA_CERTS` at its `rpc.cert` so that Node.js trusts it:

```
export BCHD_URL=https://localhost:8335
export NODE_EXTRA_CA_CERTS=~/.bchd/rpc.cert
npm start
```

Tested with Node.js 24 against a regtest node, with the versions pinned in `package.json`.
