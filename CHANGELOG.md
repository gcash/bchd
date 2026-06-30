# bchd changelog

All notable user-visible changes to [bchd](https://github.com/gcash/bchd) — a full Bitcoin Cash node written in Go — are listed here, newest first. Each entry summarizes the consensus, networking, RPC/gRPC, indexing, mining, and packaging changes that landed in a release.

bchd forked from [btcd](https://github.com/btcsuite/btcd) 0.12.x. The original btcd changelog (0.3.0-alpha through 0.12.0) is preserved unchanged at the bottom of this file under **Legacy btcd changelog (pre-fork)**.

## 0.22.1 (2026-06-30)

### Consensus & Network
- Remove the defunct `dnsseed.electroncash.de` DNS seed.

### Mining
- Include the Upgrade9 (CashTokens) script flags when building block templates. Templates were previously validated with the bare `StandardVerifyFlags`, so on token-active networks valid CashToken transactions already in the mempool were dropped from the template, producing empty or near-empty blocks; the flags are now computed the same way the mempool and consensus block validation do.

### Other
- Modernize `sync/atomic` usage with the typed atomic APIs and adopt `WaitGroup.Go`.

## 0.22.0 (2026-04-23)

### Consensus & Network
- Add network parameters and activation logic for the May 15, 2026 BCH upgrade (Upgrade12), gating the new script flags by activation time across chains.
- Implement CHIP-2024-12 P2S (Pay to Script).
- Implement CHIP-2025-05 Functions: function definition and invocation operations.
- Implement CHIP-2021-05 Loops: bounded looping operations.
- Implement CHIP-2025-05: re-enable the bitwise opcodes OP_INVERT, OP_2MUL, OP_2DIV, OP_LSHIFT, and OP_RSHIFT.
- Align Upgrade12 consensus, policy, and VMB tests with BCHN, including corrected operation-cost accounting.

### Security
- Fix CVE-2018-17145 (inv-message memory-exhaustion denial-of-service vector).

### Build & Packaging
- Bump the Dockerfile to the latest Go release and trim the Docker build context.

### Other
- Add VMB (VM bytecode) test vectors for the May 2026 upgrade covering bitwise ops, loops, functions, and P2S.

## 0.21.1 (2025-05-18)

### Build & Packaging
- Gzip test fixtures and ignore `node_modules` to dramatically reduce the module/package size, resolving "module source tree too large" errors when installing through the Go module proxy.

## 0.21.0 (2025-05-13)

### Consensus & Network
- Implement CHIP-2021-05 VM Limits and CHIP-2024-07 BigInt high-precision arithmetic (the May 2025 "Upgrade 11"), adding VM limit constants, opcode cost accounting, big-integer script numbers, and activation rules (active by default on regtest and simnet) (#570).

## 0.20.0 (2025-01-14)

### Consensus & Network
- Add full support for the BCH May 2023 network upgrade (Upgrade 9): native CashTokens, new token-aware opcodes, P2SH32 outputs, and updated transaction size limits (#545).
- Implement ABLA (Adaptive Blocksize Limit Algorithm) for the dynamic block size limit, with state persisted to the database and a fixed-size config option (#545, #553).
- Add chipnet support and bring TestNet4 onto the correct network magic bytes with updated chain parameters (#545).
- Bump the P2P protocol version to 70016 and update DNS seeds to match BCHN, adding bchd.cash seeds.
- Add support for receiving the `sendaddrv2` message from peers, including before verack (#554).
- Fix a consensus incompatibility in `txscript.IsUnspendable` that diverged from other implementations and posed a fork/UTXO-commitment risk (#475).

### Mempool & Policy
- Actively prevent network discovery and outbound connections when running in regression-test (regnet) mode (#519).

### RPC & gRPC
- Add the `signrawtransactionwithkey` JSON-RPC method (#497).
- Expose CashTokens token data and TestNet4 in the gRPC (bchrpc) API, and populate token data correctly across endpoints.
- Increase RPC read/write timeouts to 5 minutes for expensive lookups (#515).

### Wallet & Indexing
- Fix an SLP graph indexer bug and correct SLP index map allocation.

### Performance
- Reduce map allocations during block processing and avoid slice reallocations in `processOrphans`.

### Security
- Fix an `opcodeCheckSig` bug where input token data was not passed to the BIP143 signature hash under certain conditions (#552).

### Build & Packaging
- Update to Go 1.23.4 and migrate CI from Travis to GitHub Actions.
- Fix `bchctl` in the Dockerfile and update the Kubernetes deployment to handle certificates.

### Bug Fixes
- Fix a bug when cloning the script engine stacks used by `meep` (#506).

## 0.19.0 (2022-05-05)

### Consensus & Network
- Implement the May 2022 BCH hard fork: 64-bit script integers, native introspection opcodes, and OP_MUL, with fork activation and new TestNet4 parameters (#499).

### Mempool & Policy
- Treat `OP_RETURN OP_1NEGATE` as a standard transaction (#476).

### RPC & gRPC
- Add HTTPS support to the gRPC proxy (#478).

### Bug Fixes
- Fix `NullDataScript` handling and standardness tests (#477).

## 0.18.1 (2021-05-24)

### Consensus & Network
- Improve peer and sync-manager candidate selection, including a regtest `isSyncCandidate` fix (#470).
- Isolate height checks in peer syncing (#471).

### Wallet & Indexing
- Guard against nil buckets in the address, cfilter, SLP, and tx indexers to prevent panics (#473).

### Other
- Promote select peer logging messages from trace to debug level (#469).

## 0.18.0 (2021-05-12)

### Consensus & Network
- Avoid connecting to Bitcoin ABC nodes and prevent the connection manager from dialing duplicate addresses (#450).
- Remove the dead `seed-bch.bitcoinforks.org` DNS seed (#443).
- Add a clean-shutdown check in `updateSyncPeer` to avoid races during node shutdown (#442).

### Mempool & Policy
- Allow multiple OP_RETURN (null-data) outputs per transaction via a configurable policy (#467).

### Mining
- Add a `SignMuSig` function to the `bchec` package (#457).

### RPC & gRPC
- Add SLP gRPC/REST endpoints and fields: `GetParsedSlpScript`, `GetTrustedValidation`, `GetTokenMetadata`, and `CheckSlpTransaction` (with `DisableSlpBurnErrors`), plus SLP token metadata/addresses on `GetTransaction`, `GetUnspentOutput`, and the transaction subscription filters (#387, #458).
- Validate SLP transactions by default in `SubmitTransaction` (clients must opt out via `skip_slp_validity_check`) to guard against accidental token burns (#387).
- Add a gRPC-gateway REST/JSON proxy server (with CORS and no-cache middleware) and regenerated Swagger definitions for the bchrpc API (#387).
- Return pubkey (P2PK) outputs from `GetAddressUnspentOutputs` (#432).

### Wallet & Indexing
- Add the SLP (Simple Ledger Protocol) token index, validating SLP/NFT1 token transactions (genesis, mint, send, mint-baton tracking, burn detection) with a new `--slpindex` config option (#387, #452).
- Add SLP Graph Search, building a token transaction graph to support trusted client-side validation (#451).
- Fix index-manager handling of indexers whose `StartBlock()` height is greater than 0, so SLP and other late-starting indexes sync correctly (#455).
- Refuse to start an index on a pruned node (print an error and exit) and prevent a crash when SLP index initialization fails (#433).

### Performance
- Flush the UTXO cache during state reconstruction and correctly set `lastFlushHash` on init, with tests for a hard shutdown mid-flush (#445, #446, #447).

### Build & Packaging
- Add Prometheus metrics support and per-component shutdown logging (#453).
- Add `darwin/arm64` (Apple Silicon) binary builds.
- Add `RegressionTestAnyHost` and `RegressionTestNoReset` regtest options for Docker-based setups (#462).

### Bug Fixes
- Fix a panic in the fee estimator triggered by chain reorgs (#434).
- Fix a crash related to unconfirmed NFT child tokens in the gRPC server (#387).

## 0.17.1 (2020-12-08)

### Consensus & Network
- Update mainnet chain params for post-hardfork (ASERT) activation, and add post-fork params for testnet3, regtest, and simnet (#430).
- Add a UTXO-set checkpoint hash and sources to the chain params.
- Fix the DNS seeds list after the hard fork (#428).

### RPC & gRPC
- Add an `RPCAuthTimeout` option to configure the RPC auth timeout (#421).

### Performance
- Check for known inventory before relaying a compact block, avoiding redundant relays.

### Build & Packaging
- Build with Go 1.15.5 (#423).

## 0.17.0 (2020-10-24)

### Consensus & Network
- Implement the aserti3-2d (ASERT) difficulty adjustment algorithm for the November 2020 BCH hard fork, and wire up its activation (#399, #402).
- Select a new sync peer when the median of sync-peer candidates is ahead of us, improving recovery from stalled syncs (#411).

### Mempool & Policy
- Increase the maximum standard transaction size to 100k (#405).
- Remove `ScriptVerifyMinimalIf` from the standard verification flags to match Bitcoin ABC policy (#413).

### RPC & gRPC
- Rebuild all protobuf bindings and document little-endian field formats in the gRPC comments (#394).
- Add a Java protobuf package (`com.bchd.rpc`) for the bchrpc API (#396).

### Build & Packaging
- Build with Go 1.15.3; drop darwin/386 support (removed in Go 1.15) (#419).

## 0.16.5 (2020-07-23)

### Mempool & Policy
- Default to disabling relay of free (very low fee) transactions (#383).

### RPC & gRPC
- Support cookie-based RPC authorization (#393).
- Accept either int or bool for the verbosity argument in `getrawtransaction` and `searchrawtransactions` (#391).
- Add a `getbalances` RPC client command and backport `getblock` verbosity fixes (#388, #389).
- Return the version in `getnetworkinfo` in the same format as ABC/BCHN.
- Remove custom HTTP handling so RPC connections support keep-alives (#384).

### Security
- Avoid a panic in `bchec` `fieldVal.SetByteSlice` for oversized inputs.
- Fix a deadlock in `dynamicbanscore`.

### Bug Fixes
- Prevent a peer's last block height from going backwards (#392).

### Other
- Document that CIFS users must turn off async preemption (#390).

## 0.16.4 (2020-07-06)

### RPC & gRPC
- Fix an off-by-one error in transaction confirmation counts in the gRPC API (#376).

### Wallet & Indexing
- Raise `MaxCFilterDataSize` to 2 MiB so larger compact filters can be served.

## 0.16.3 (2020-06-10)

### Consensus & Network
- Fix a bug calculating sigcheck density (#374).

## 0.16.2 (2020-05-19)

### Consensus & Network
- First-pass cleanup of pre-fork hard-fork code paths (#373).
- Add a new fastsync checkpoint and update the testnet checkpoint hash.

### RPC & gRPC
- Remove sigops references from the RPC server results.

## 0.16.1 (2020-05-15)

### Consensus & Network
- Fix bugs in OP_REVERSEBYTES and set the Phonon activation time in tests (#371).

## 0.16.0 (2020-05-05)

### Consensus & Network
- Activate the May 2020 (Phonon) hard fork, with sigcheck accounting replacing sigop counting (#348, #360, #366).
- Add the OP_REVERSEBYTES script opcode (#348).
- Do not disconnect peers that send unknown commands; only warn in the logs (#369).
- Remove dead/invalid DNS seeds and add back the bitcoinforks seeders (#337, #359, #364).

### Mining
- Update the mining package for the May 2020 hard fork (#348, #362).
- Add a config option to set `CoinbaseFlags` in the block-template coinbase input (#340).

### RPC & gRPC
- Improve memory usage of the `GetAddressUnspentOutputs` RPC call (#332, #333).

### Security
- Add a first-pass MuSig implementation with session commitments (#334, #347).

### Build & Packaging
- Build with Go 1.14.2 and update all dependencies, including a new bchutil (#344, #365, #368).

## 0.15.2 (2019-11-30)

### Consensus & Network
- Make verack part of the version handshake; add peers on `OnVerAck` rather than `OnVersion` to harden connection negotiation (#319, #320).
- Add configurable user-agent filtering for peer connections (#318).

### RPC & gRPC
- Update `ParsePkScript` to support multiple networks (#317).
- Guard against a nil request in `SubscribeTransactionStream` to prevent a crash (#324).
- Warn instead of exiting when binding the RPC/gRPC server to a non-localhost address (#314).

### Performance
- Skip transaction rescan when the client has subscribed to no addresses or UTXOs (#321).

### Build & Packaging
- Add a sample bchd systemd service unit.

## 0.15.1 (2019-11-17)

### Consensus & Network
- Fix a consensus bug in OP_CHECKMULTISIG where the dummy stack element was not copied correctly (#311).
- Update checkpoints with the new (Graviton) hard-fork parameters (#310).
- Add the `testnet-seed.bchd.cash` DNS seed (#309).

### Performance
- Improve initial block download and syncing performance (#307).
- Allow `wire.MsgTx` objects to be garbage-collected before the UTXO cache is flushed, reducing memory use (#292).

### Security
- Copy `pkScript` in `addTxOut` and `disconnectTransactions` to prevent UTXO data corruption (#293).
- Add a notification lock to guard against a double unlock (#308).

### Build & Packaging
- Switch the Docker image to a multi-stage build (#301).

## 0.15.0 (2019-10-21)

### Consensus & Network
- Add the November 2019 (Graviton) hard-fork rules (#284).

### RPC & gRPC
- Match bitcoind's RPC error codes when rejecting transactions (#278).
- Fix a gRPC bug returning unspent outputs and refactor `GetUnspentOutput` to match the JSON-RPC code (#281, #282).
- Check spent status in the mempool when returning unspent outputs (#286).

### Wallet & Indexing
- Migrate the committed-filter (cfindex) index to version 2; disable cfindex in fastsync mode and skip migration when the chain is pruned (#288).

## 0.14.7 (2019-08-20)

### RPC & gRPC
- Add the `GetMempool` gRPC method to retrieve the full mempool (#270).
- Add the `GetUnspentOutput` gRPC method and a `mempool` flag on `GetAddressUnspentOutputs` to include unconfirmed UTXOs (#267).
- Add streaming raw (serialized) transaction and block subscriptions to gRPC (#273, #274).
- Add median time to gRPC `BlockInfo` and fix median time in `GetBlockchainInfo` (#271).
- Add batched JSON-RPC 2.0 request support, and accept boolean verbosity in `getblock` (ElectrumX compatibility).
- Regenerate gRPC bindings, adding a Python library and updated JS (#275).

### Build & Packaging
- Convert the project from `dep` to Go modules (#266).

### Bug Fixes
- Ensure downstream peers receive new blocks accepted via reorg of previously-orphaned blocks (#265).
- Only scan `.fdb` files when loading the block database, ignoring stray files (#258).

## 0.14.6 (2019-06-15)

### Consensus & Network
- Fix `OP_NUM2BIN` to pop a byte array off the stack (matching Bitcoin ABC) rather than a scriptnum (#253).

## 0.14.5 (2019-06-13)

### Consensus & Network
- Fix a bug in Schnorr signature validation by normalizing field values before comparison (#250).

### Mempool & Policy
- Increase `DefaultBlockPrioritySize` to 1,600,000 bytes (#238).

### Mining
- Increase the default mining block size (`blockmaxsize`) to ~32 MB (#240).
- Fix block-template generation so coinbase padding no longer drops the coinbase below the 100-byte minimum (#238).

### RPC & gRPC
- Implement the `getnetworkinfo` JSON-RPC call (#241).
- Fix CORS preflight (OPTIONS) handling for grpc-web (#243).
- Generate grpc-web JavaScript client libraries (#248).

### Performance
- Set the fast-sync worker count based on `runtime.NumCPU()` (#251).

## 0.14.4 (2019-05-26)

### Consensus & Network
- Activate the GreatWall (May 15, 2019) hard fork at block height 582679, enforcing the Schnorr and SegWit-recovery script flags, and add a checkpoint at the fork block (#230).
- Switch transaction-signing functions to Schnorr and add `ScriptVerifySchnorr` to the standard verify flags (#228).

### RPC & gRPC
- Fix `GetAddressUnspentOutputs` to return all UTXOs for an address instead of only the last 100 (#225).

### Build & Packaging
- Expose the database cache size and flush interval as config options (#232).

### Bug Fixes
- Override the UTXO cache entry (using a clone) in the reorg case so spent/unspent transitions are applied correctly (#234).

## 0.14.3 (2019-05-03)

### RPC & gRPC
- Add a new bchrpc gRPC API server with full node and wallet query support, gRPC-Web via protocol multiplexing for browser clients, CORS, and server reflection (#217, #222).
- Add spent-output (stxo) metadata to transaction input responses (#217).
- Add API, client-usage, and wallet-operation documentation (#217).

### Bug Fixes
- Fix `GetMerkleProof` returning incorrect proofs (#217).
- Fix `GetAddressTransactions` and fetch address transactions in reverse order (#217).
- Fix a bug subscribing to blocks (#217).

## 0.14.2 (2019-04-17)

### Consensus & Network
- Update segwit-recovery handling to the latest May 2019 hard-fork spec (#216).
- Fix a bug in OP_NUM2BIN (#215).

### Mempool & Policy
- Add the `invalidateblock` RPC and set flags on ancestors of invalidated blocks (#214, #218).

### RPC & gRPC
- Fix `getinfo` to return the correct protocol version (#212).

### Wallet & Indexing
- cfindex no longer requires transaction inputs to build filters (#213).

## 0.14.1 (2019-04-10)

### Consensus & Network
- Fix a bug in OP_CAT and allow multiple data pushes in OP_RETURN (#210, #211).
- Add `ScriptVerifyCheckDataSig` to the standard verify flags (#206).

### Performance
- Add compact block (BIP152) relay and processing support, including `sendcmpct`/`getblocktxns` handling and direct block relay to requesting peers (#197).
- Use the hashcache for script validation where possible (#208).

### Wallet & Indexing
- Update bchutil to the new compact-filter format and add indexer database migrations (#204, #209).

## 0.14.0 (2019-03-21)

### Consensus & Network
- Implement the May 2019 BCH hard fork (Great Wall): activation, tests, and the `ScriptVerifyAllowSegwitRecovery` flag (#192).
- Add Schnorr signature support in the `bchec` package and wire OP_CHECKSIG/OP_CHECKDATASIG to accept Schnorr signatures (#180).
- Fix `MaxBlockSigOps` enforcement so the sigop limit scales per-MB of block size with accurate byte counting (#193).
- Fix bugs in UTXO rollback and `InitConsistentState` so the chain rolls back correctly (#174, #181).

### Wallet & Indexing
- Add `GetCFMempool` for compact block filters over the mempool (#190, #195).

### Performance
- Improve reliability of the FastSync UTXO-set download, add SOCKS5 proxy support, and show download progress (#188, #189, #198).

## 0.13.0 (2018-12-18)

### Overview
- Initial bchd release — a full Bitcoin Cash node forked from btcd 0.12.x.

### Consensus & Network
- Bitcoin Cash network parameters: BCH network magic and Bitcoin Cash DNS seeds (#15).
- UAHF support: enforce the must-be-big block consensus rule and replay-protected `SIGHASH_FORKID` signature hashing with BIP143-style digests (#22).
- New difficulty adjustment: implement the legacy EDA and the November 2017 DAA, with per-network activation heights.
- May 2018 hard fork rules.
- November 2018 Magnetic Anomaly hard fork: canonical transaction ordering (CTOR) enforced in block sanity checks plus new script verification flags, with configurable activation (#26, #135).
- Remove SegWit, keeping consensus aligned with the Bitcoin Cash chain (#16).
- Add an ECMH multiset implementation (`bchec`) as the basis for UTXO set commitments (#156).
- Configurable excessive block size (`--excessiveblocksize`) defaulting to 32 MB, with `EB` signaling advertised in the user-agent string (#118, #151).

### Mempool & Policy
- Magnetic Anomaly mempool activation, including one-transaction-input (OTI) ordering validation during fast block acceptance.

### Mining
- Block templates honor the configured excessive block size for `addblock` and mining (#118).

### RPC & gRPC
- Add `reconsiderblock` and `invalidateblock` RPCs for manual chain reorganization (#136).
- Add `gettxoutproof` and `verifytxoutproof` RPCs (and matching rpcclient helpers) (#153).

### Wallet & Indexing
- cashaddr address format support throughout the node, used as the default Bitcoin Cash address encoding (#28).
- Optional transaction index and address index for serving lite-client and block-explorer queries.

### Performance
- In-memory UTXO cache to speed up block validation (#21).
- Optional block-file pruning to reduce on-disk storage (#126).
- Fast-sync mode that downloads a committed UTXO set instead of replaying all historical blocks (#158, #164).

### Build & Packaging
- Dockerfile for easy node setup and Kubernetes deployment manifests (#39).
- Travis-based release builds and a published security-disclosure policy (#62, #65).

### Bug Fixes
- Fix testnet difficulty calculation after the DAA hard fork (#37).
- Fix a UTXO-set reorganization bug (#162).
- Correct `ExcessiveBlockSize` handling in the `addblock` utility (#118).

---

# Legacy btcd changelog (pre-fork)

> The entries below predate the Bitcoin Cash fork. They are the upstream
> [btcd](https://github.com/btcsuite/btcd) changelog (0.3.0-alpha through
> 0.12.0), preserved verbatim for historical reference. bchd forked from
> btcd 0.12.x; see the **0.13.0** entry above for the start of bchd's own
> history.

Changes in 0.12.0 (Fri Nov 20 2015)
  - Protocol and network related changes:
    - Add a new checkpoint at block height 382320 (#555)
    - Implement BIP0065 which includes support for version 4 blocks, a new
      consensus opcode (OP_CHECKLOCKTIMEVERIFY) that enforces transaction
      lock times, and a double-threshold switchover mechanism (#535, #459,
      #455)
    - Implement BIP0111 which provides a new bloom filter service flag and
      hence provides support for protocol version 70011 (#499)
    - Add a new parameter --nopeerbloomfilters to allow disabling bloom
      filter support (#499)
    - Reject non-canonically encoded variable length integers (#507)
    - Add mainnet peer discovery DNS seed (seed.bitcoin.jonasschnelli.ch)
      (#496)
    - Correct reconnect handling for persistent peers (#463, #464)
    - Ignore requests for block headers if not fully synced (#444)
    - Add CLI support for specifying the zone id on IPv6 addresses (#538)
    - Fix a couple of issues where the initial block sync could stall (#518,
      #229, #486)
    - Fix an issue which prevented the --onion option from working as
      intended (#446)
  - Transaction relay (memory pool) changes:
    - Require transactions to only include signatures encoded with the
    canonical 'low-s' encoding (#512)
    - Add a new parameter --minrelaytxfee to allow the minimum transaction
      fee in BTC/kB to be overridden (#520)
    - Retain memory pool transactions when they redeem another one that is
      removed when a block is accepted (#539)
    - Do not send reject messages for a transaction if it is valid but
      causes an orphan transaction which depends on it to be determined
      as invalid (#546)
    - Refrain from attempting to add orphans to the memory pool multiple
      times when the transaction they redeem is added (#551)
    - Modify minimum transaction fee calculations to scale based on bytes
      instead of full kilobyte boundaries (#521, #537)
  - Implement signature cache:
    - Provides a limited memory cache of validated signatures which is a
      huge optimization when verifying blocks for transactions that are
      already in the memory pool (#506)
    - Add a new parameter '--sigcachemaxsize' which allows the size of the
      new cache to be manually changed if desired (#506)
  - Mining support changes:
    - Notify getblocktemplate long polling clients when a block is pushed
      via submitblock (#488)
    - Speed up getblocktemplate by making use of the new signature cache
      (#506)
  - RPC changes:
    - Implement getmempoolinfo command (#453)
    - Implement getblockheader command (#461)
    - Modify createrawtransaction command to accept a new optional parameter
      'locktime' (#529)
    - Modify listunspent result to include the 'spendable' field (#440)
    - Modify getinfo command to include 'errors' field (#511)
    - Add timestamps to blockconnected and blockdisconnected notifications
      (#450)
    - Several modifications to searchrawtranscations command:
      - Accept a new optional parameter 'vinextra' which causes the results
        to include information about the outputs referenced by a transaction's
        inputs (#485, #487)
      - Skip entries in the mempool too (#495)
      - Accept a new optional parameter 'reverse' to return the results in
        reverse order (most recent to oldest) (#497)
      - Accept a new optional parameter 'filteraddrs' which causes the
        results to only include inputs and outputs which involve the
        provided addresses (#516)
    - Change the notification order to notify clients about mined
      transactions (recvtx, redeemingtx) before the blockconnected
      notification (#449)
    - Update verifymessage RPC to use the standard algorithm so it is
      compatible with other implementations (#515)
    - Improve ping statistics by pinging on an interval (#517)
  - Websocket changes:
    - Implement session command which returns a per-session unique id (#500,
      #503)
  - btcctl utility changes:
    - Add getmempoolinfo command (#453)
    - Add getblockheader command (#461)
    - Add getwalletinfo command (#471)
  - Notable developer-related package changes:
    - Introduce a new peer package which acts a common base for creating and
      concurrently managing bitcoin network peers (#445)
    - Various cleanup of the new peer package (#528, #531, #524, #534,
      #549)
    - Blocks heights now consistently use int32 everywhere (#481)
    - The BlockHeader type in the wire package now provides the BchDecode
      and BchEncode methods (#467)
    - Update wire package to recognize BIP0064 (getutxo) service bit (#489)
    - Export LockTimeThreshold constant from txscript package (#454)
    - Export MaxDataCarrierSize constant from txscript package (#466)
    - Provide new IsUnspendable function from the txscript package (#478)
    - Export variable length string functions from the wire package (#514)
    - Export DNS Seeds for each network from the chaincfg package (#544)
    - Preliminary work towards separating the memory pool into a separate
      package (#525, #548)
  - Misc changes:
    - Various documentation updates (#442, #462, #465, #460, #470, #473,
      #505, #530, #545)
    - Add installation instructions for gentoo (#542)
    - Ensure an error is shown if OS limits can't be set at startup (#498)
    - Tighten the standardness checks for multisig scripts (#526)
    - Test coverage improvement (#468, #494, #527, #543, #550)
    - Several optimizations (#457, #474, #475, #476, #508, #509)
    - Minor code cleanup and refactoring (#472, #479, #482, #519, #540)
  - Contributors (alphabetical order):
    - Ben Echols
    - Bruno Clermont
    - danda
    - Daniel Krawisz
    - Dario Nieuwenhuis
    - Dave Collins
    - David Hill
    - Javed Khan
    - Jonathan Gillham
    - Joseph Becher
    - Josh Rickmar
    - Justus Ranvier
    - Mawuli Adzoe
    - Olaoluwa Osuntokun
    - Rune T. Aune

Changes in 0.11.1 (Wed May 27 2015)
  - Protocol and network related changes:
    - Use correct sub-command in reject message for rejected transactions
      (#436, #437)
    - Add a new parameter --torisolation which forces new circuits for each
      connection when using tor (#430)
  - Transaction relay (memory pool) changes:
    - Reduce the default number max number of allowed orphan transactions
      to 1000 (#419)
    - Add a new parameter --maxorphantx which allows the maximum number of
      orphan transactions stored in the mempool to be specified (#419)
  - RPC changes:
    - Modify listtransactions result to include the 'involveswatchonly' and
      'vout' fields (#427)
    - Update getrawtransaction result to omit the 'confirmations' field
      when it is 0 (#420, #422)
    - Update signrawtransaction result to include errors (#423)
  - btcctl utility changes:
    - Add gettxoutproof command (#428)
    - Add verifytxoutproof command (#428)
  - Notable developer-related package changes:
    - The btcec package now provides the ability to perform ECDH
      encryption and decryption (#375)
    - The block and header validation in the blockchain package has been
      split to help pave the way toward concurrent downloads (#386)
  - Misc changes:
    - Minor peer optimization (#433)
  - Contributors (alphabetical order):
    - Dave Collins
    - David Hill
    - Federico Bond
    - Ishbir Singh
    - Josh Rickmar

Changes in 0.11.0 (Wed May 06 2015)
  - Protocol and network related changes:
    - **IMPORTANT: Update is required due to the following point**
    - Correct a few corner cases in script handling which could result in
      forking from the network on non-standard transactions (#425)
    - Add a new checkpoint at block height 352940 (#418)
    - Optimized script execution (#395, #400, #404, #409)
    - Fix a case that could lead stalled syncs (#138, #296)
  - Network address manager changes:
    - Implement eclipse attack countermeasures as proposed in
      http://cs-people.bu.edu/heilman/eclipse (#370, #373)
  - Optional address indexing changes:
    - Fix an issue where a reorg could cause an orderly shutdown when the
      address index is active (#340, #357)
  - Transaction relay (memory pool) changes:
    - Increase maximum allowed space for nulldata transactions to 80 bytes
      (#331)
    - Implement support for the following rules specified by BIP0062:
      - The S value in ECDSA signature must be at most half the curve order
        (rule 5) (#349)
      - Script execution must result in a single non-zero value on the stack
        (rule 6) (#347)
      - NOTE: All 7 rules of BIP0062 are now implemented
    - Use network adjusted time in finalized transaction checks to improve
      consistency across nodes (#332)
    - Process orphan transactions on acceptance of new transactions (#345)
  - RPC changes:
    - Add support for a limited RPC user which is not allowed admin level
      operations on the server (#363)
    - Implement node command for more unified control over connected peers
      (#79, #341)
    - Implement generate command for regtest/simnet to support
      deterministically mining a specified number of blocks (#362, #407)
    - Update searchrawtransactions to return the matching transactions in
      order (#354)
    - Correct an issue with searchrawtransactions where it could return
      duplicates (#346, #354)
    - Increase precision of 'difficulty' field in getblock result to 8
      (#414, #415)
    - Omit 'nextblockhash' field from getblock result when it is empty
      (#416, #417)
    - Add 'id' and 'timeoffset' fields to getpeerinfo result (#335)
  - Websocket changes:
    - Implement new commands stopnotifyspent, stopnotifyreceived,
      stopnotifyblocks, and stopnotifynewtransactions to allow clients to
      cancel notification registrations (#122, #342)
  - btcctl utility changes:
    - A single dash can now be used as an argument to cause that argument to
      be read from stdin (#348)
    - Add generate command
  - Notable developer-related package changes:
    - The new version 2 btcjson package has now replaced the deprecated
      version 1 package (#368)
    - The btcec package now performs all signing using RFC6979 deterministic
      signatures (#358, #360)
    - The txscript package has been significantly cleaned up and had a few
      API changes (#387, #388, #389, #390, #391, #392, #393, #395, #396,
      #400, #403, #404, #405, #406, #408, #409, #410, #412)
    - A new PkScriptLocs function has been added to the wire package MsgTx
      type which provides callers that deal with scripts optimization
      opportunities (#343)
  - Misc changes:
    - Minor wire hashing optimizations (#366, #367)
    - Other minor internal optimizations
  - Contributors (alphabetical order):
    - Alex Akselrod
    - Arne Brutschy
    - Chris Jepson
    - Daniel Krawisz
    - Dave Collins
    - David Hill
    - Jimmy Song
    - Jonas Nick
    - Josh Rickmar
    - Olaoluwa Osuntokun
    - Oleg Andreev

Changes in 0.10.0 (Sun Mar 01 2015)
  - Protocol and network related changes:
    - Add a new checkpoint at block height 343185
    - Implement BIP066 which includes support for version 3 blocks, a new
      consensus rule which prevents non-DER encoded signatures, and a
      double-threshold switchover mechanism
    - Rather than announcing all known addresses on getaddr requests which
      can possibly result in multiple messages, randomize the results and
      limit them to the max allowed by a single message (1000 addresses)
    - Add more reserved IP spaces to the address manager
  - Transaction relay (memory pool) changes:
    - Make transactions which contain reserved opcodes nonstandard
    - No longer accept or relay free and low-fee transactions that have
      insufficient priority to be mined in the next block
    - Implement support for the following rules specified by BIP0062:
      - ECDSA signature must use strict DER encoding (rule 1)
      - The signature script must only contain push operations (rule 2)
      - All push operations must use the smallest possible encoding (rule 3)
      - All stack values interpreted as a number must be encoding using the
        shortest possible form (rule 4)
      - NOTE: Rule 1 was already enforced, however the entire script now
        evaluates to false rather than only the signature verification as
        required by BIP0062
    - Allow transactions with nulldata transaction outputs to be treated as
      standard
  - Mining support changes:
    - Modify the getblocktemplate RPC to generate and return block templates
      for version 3 blocks which are compatible with BIP0066
    - Allow getblocktemplate to serve blocks when the current time is
      less than the minimum allowed time for a generated block template
      (https://github.com/gcash/bchd/issues/209)
  - Crypto changes:
    - Optimize scalar multiplication by the base point by using a
      pre-computed table which results in approximately a 35% speedup
     (https://github.com/btcsuite/btcec/issues/2)
    - Optimize general scalar multiplication by using the secp256k1
      endomorphism which results in approximately a 17-20% speedup
     (https://github.com/btcsuite/btcec/issues/1)
    - Optimize general scalar multiplication by using non-adjacent form
      which results in approximately an additional 8% speedup
     (https://github.com/btcsuite/btcec/issues/3)
  - Implement optional address indexing:
    - Add a new parameter --addrindex which will enable the creation of an
      address index which can be queried to determine all transactions which
      involve a given address
      (https://github.com/gcash/bchd/issues/190)
    - Add a new logging subsystem for address index related operations
    - Support new searchrawtransactions RPC
      (https://github.com/gcash/bchd/issues/185)
  - RPC changes:
    - Require TLS version 1.2 as the minimum version for all TLS connections
    - Provide support for disabling TLS when only listening on localhost
      (https://github.com/gcash/bchd/pull/192)
    - Modify help output for all commands to provide much more consistent
      and detailed information
    - Correct case in getrawtransaction which would refuse to serve certain
      transactions with invalid scripts
      (https://github.com/gcash/bchd/issues/210)
    - Correct error handling in the getrawtransaction RPC which could lead
      to a crash in rare cases
      (https://github.com/gcash/bchd/issues/196)
    - Update getinfo RPC to include the appropriate 'timeoffset' calculated
      from the median network time
    - Modify listreceivedbyaddress result type to include txids field so it
      is compatible
    - Add 'iswatchonly' field to validateaddress result
    - Add 'startingpriority' and 'currentpriority' fields to getrawmempool
      (https://github.com/gcash/bchd/issues/178)
    - Don't omit the 'confirmations' field from getrawtransaction when it is
      zero
  - Websocket changes:
    - Modify the behavior of the rescan command to automatically register
      for notifications about transactions paying to rescanned addresses
      or spending outputs from the final rescan utxo set when the rescan
      is through the best block in the chain
  - btcctl utility changes:
    - Make the list of commands available via the -l option rather than
      dumping the entire list on usage errors
    - Alphabetize and categorize the list of commands by chain and wallet
    - Make the help option only show the help options instead of also
      dumping all of the commands
    - Make the usage syntax much more consistent and correct a few cases of
      misnamed fields
      (https://github.com/gcash/bchd/issues/305)
    - Improve usage errors to show the specific parameter number, reason,
      and error code
    - Only show the usage for specific command is shown when a valid command
      is provided with invalid parameters
    - Add support for a SOCK5 proxy
    - Modify output for integer fields (such as timestamps) to display
      normally instead in scientific notation
    - Add invalidateblock command
    - Add reconsiderblock command
    - Add createnewaccount command
    - Add renameaccount command
    - Add searchrawtransactions command
    - Add importaddress command
    - Add importpubkey command
  - showblock utility changes:
    - Remove utility in favor of the RPC getblock method
  - Notable developer-related package changes:
    - Many of the core packages have been relocated into the btcd repository
      (https://github.com/gcash/bchd/issues/214)
    - A new version of the btcjson package that has been completely
      redesigned from the ground up based based upon how the project has
      evolved and lessons learned while using it since it was first written
      is now available in the btcjson/v2/btcjson directory
      - This will ultimately replace the current version so anyone making
        use of this package will need to update their code accordingly
    - The btcec package now provides better facilities for working directly
      with its public and private keys without having to mix elements from
      the ecdsa package
    - Update the script builder to ensure all rules specified by BIP0062 are
      adhered to when creating scripts
    - The blockchain package now provides a MedianTimeSource interface and
      concrete implementation for providing time samples from remote peers
      and using that data to calculate an offset against the local time
  - Misc changes:
    - Fix a slow memory leak due to tickers not being stopped
      (https://github.com/gcash/bchd/issues/189)
    - Fix an issue where a mix of orphans and SPV clients could trigger a
      condition where peers would no longer be served
      (https://github.com/gcash/bchd/issues/231)
    - The RPC username and password can now contain symbols which previously
      conflicted with special symbols used in URLs
    - Improve handling of obtaining random nonces to prevent cases where it
      could error when not enough entropy was available
    - Improve handling of home directory creation errors such as in the case
      of unmounted symlinks (https://github.com/gcash/bchd/issues/193)
    - Improve the error reporting for rejected transactions to include the
      inputs which are missing and/or being double spent
    - Update sample config file with new options and correct a comment
      regarding the fact the RPC server only listens on localhost by default
      (https://github.com/gcash/bchd/issues/218)
    - Update the continuous integration builds to run several tools which
      help keep code quality high
    - Significant amount of internal code cleanup and improvements
    - Other minor internal optimizations
  - Code Contributors (alphabetical order):
    - Beldur
    - Ben Holden-Crowther
    - Dave Collins
    - David Evans
    - David Hill
    - Guilherme Salgado
    - Javed Khan
    - Jimmy Song
    - John C. Vernaleo
    - Jonathan Gillham
    - Josh Rickmar
    - Michael Ford
    - Michail Kargakis
    - kac
    - Olaoluwa Osuntokun

Changes in 0.9.0 (Sat Sep 20 2014)
  - Protocol and network related changes:
    - Add a new checkpoint at block height 319400
    - Add support for BIP0037 bloom filters
      (https://github.com/conformal/btcd/issues/132)
    - Implement BIP0061 reject handling and hence support for protocol
      version 70002 (https://github.com/conformal/btcd/issues/133)
    - Add testnet DNS seeds for peer discovery (testnet-seed.alexykot.me
      and testnet-seed.bitcoin.schildbach.de)
    - Add mainnet DNS seed for peer discovery (seeds.bitcoin.open-nodes.org)
    - Make multisig transactions with non-null dummy data nonstandard
      (https://github.com/conformal/btcd/issues/131)
    - Make transactions with an excessive number of signature operations
      nonstandard
    - Perform initial DNS lookups concurrently which allows connections
      more quickly
    - Improve the address manager to significantly reduce memory usage and
      add tests
    - Remove orphan transactions when they appear in a mined block
      (https://github.com/conformal/btcd/issues/166)
    - Apply incremental back off on connection retries for persistent peers
      that give invalid replies to mirror the logic used for failed
      connections (https://github.com/conformal/btcd/issues/103)
    - Correct rate-limiting of free and low-fee transactions
  - Mining support changes:
    - Implement getblocktemplate RPC with the following support:
      (https://github.com/conformal/btcd/issues/124)
      - BIP0022 Non-Optional Sections
      - BIP0022 Long Polling
      - BIP0023 Basic Pool Extensions
      - BIP0023 Mutation coinbase/append
      - BIP0023 Mutations time, time/increment, and time/decrement
      - BIP0023 Mutation transactions/add
      - BIP0023 Mutations prevblock, coinbase, and generation
      - BIP0023 Block Proposals
    - Implement built-in concurrent CPU miner
      (https://github.com/conformal/btcd/issues/137)
      NOTE: CPU mining on mainnet is pointless.  This has been provided
      for testing purposes such as for the new simulation test network
    - Add --generate flag to enable CPU mining
    - Deprecate the --getworkkey flag in favor of --miningaddr which
      specifies which addresses generated blocks will choose from to pay
      the subsidy to
  - RPC changes:
    - Implement gettxout command
      (https://github.com/conformal/btcd/issues/141)
    - Implement validateaddress command
    - Implement verifymessage command
    - Mark getunconfirmedbalance RPC as wallet-only
    - Mark getwalletinfo RPC as wallet-only
    - Update getgenerate, setgenerate, gethashespersec, and getmininginfo
      to return the appropriate information about new CPU mining status
    - Modify getpeerinfo pingtime and pingwait field types to float64 so
      they are compatible
    - Improve disconnect handling for normal HTTP clients
    - Make error code returns for invalid hex more consistent
  - Websocket changes:
    - Switch to a new more efficient websocket package
      (https://github.com/conformal/btcd/issues/134)
    - Add rescanfinished notification
    - Modify the rescanprogress notification to include block hash as well
      as height (https://github.com/conformal/btcd/issues/151)
  - btcctl utility changes:
    - Accept --simnet flag which automatically selects the appropriate port
      and TLS certificates needed to communicate with btcd and btcwallet on
      the simulation test network
    - Fix createrawtransaction command to send amounts denominated in BTC
    - Add estimatefee command
    - Add estimatepriority command
    - Add getmininginfo command
    - Add getnetworkinfo command
    - Add gettxout command
    - Add lockunspent command
    - Add signrawtransaction command
  - addblock utility changes:
    - Accept --simnet flag which automatically selects the appropriate port
      and TLS certificates needed to communicate with btcd and btcwallet on
      the simulation test network
  - Notable developer-related package changes:
    - Provide a new bloom package in btcutil which allows creating and
      working with BIP0037 bloom filters
    - Provide a new hdkeychain package in btcutil which allows working with
      BIP0032 hierarchical deterministic key chains
    - Introduce a new btcnet package which houses network parameters
    - Provide new simnet network (--simnet) which is useful for private
      simulation testing
    - Enforce low S values in serialized signatures as detailed in BIP0062
    - Return errors from all methods on the btcdb.Db interface
      (https://github.com/conformal/btcdb/issues/5)
    - Allow behavior flags to alter btcchain.ProcessBlock
      (https://github.com/conformal/btcchain/issues/5)
    - Provide a new SerializeSize API for blocks
      (https://github.com/conformal/btcwire/issues/19)
    - Several of the core packages now work with Google App Engine
  - Misc changes:
    - Correct an issue where the database could corrupt under certain
      circumstances which would require a new chain download
    - Slightly optimize deserialization
    - Use the correct IP block for he.net
    - Fix an issue where it was possible the block manager could hang on
      shutdown
    - Update sample config file so the comments are on a separate line
      rather than the end of a line so they are not interpreted as settings
      (https://github.com/conformal/btcd/issues/135)
    - Correct an issue where getdata requests were not being properly
      throttled which could lead to larger than necessary memory usage
    - Always show help when given the help flag even when the config file
      contains invalid entries
    - General code cleanup and minor optimizations

Changes in 0.8.0-beta (Sun May 25 2014)
  - Btcd is now Beta (https://github.com/conformal/btcd/issues/130)
  - Add a new checkpoint at block height 300255
  - Protocol and network related changes:
    - Lower the minimum transaction relay fee to 1000 satoshi to match
      recent reference client changes
      (https://github.com/conformal/btcd/issues/100)
    - Raise the maximum signature script size to support standard 15-of-15
      multi-signature pay-to-sript-hash transactions with compressed pubkeys
      to remain compatible with the reference client
      (https://github.com/conformal/btcd/issues/128)
    - Reduce max bytes allowed for a standard nulldata transaction to 40 for
      compatibility with the reference client
    - Introduce a new btcnet package which houses all of the network params
      for each network (mainnet, testnet3, regtest) to ultimately enable
      easier addition and tweaking of networks without needing to change
      several packages
    - Fix several script discrepancies found by reference client test data
    - Add new DNS seed for peer discovery (seed.bitnodes.io)
    - Reduce the max known inventory cache from 20000 items to 1000 items
    - Fix an issue where unknown inventory types could lead to a hung peer
    - Implement inventory rebroadcast handler for sendrawtransaction
      (https://github.com/conformal/btcd/issues/99)
    - Update user agent to fully support BIP0014
      (https://github.com/conformal/btcwire/issues/10)
  - Implement initial mining support:
    - Add a new logging subsystem for mining related operations
    - Implement infrastructure for creating block templates
    - Provide options to control block template creation settings
    - Support the getwork RPC
    - Allow address identifiers to apply to more than one network since both
      testnet3 and the regression test network unfortunately use the same
      identifier
  - RPC changes:
    - Set the content type for HTTP POST RPC connections to application/json
      (https://github.com/conformal/btcd/issues/121)
    - Modified the RPC server startup so it only requires at least one valid
      listen interface
    - Correct an error path where it was possible certain errors would not
      be returned
    - Implement getwork command
      (https://github.com/conformal/btcd/issues/125)
    - Update sendrawtransaction command to reject orphans
    - Update sendrawtransaction command to include the reason a transaction
      was rejected
    - Update getinfo command to populate connection count field
    - Update getinfo command to include relay fee field
      (https://github.com/conformal/btcd/issues/107)
    - Allow transactions submitted with sendrawtransaction to bypass the
      rate limiter
    - Allow the getcurrentnet and getbestblock extensions to be accessed via
      HTTP POST in addition to Websockets
      (https://github.com/conformal/btcd/issues/127)
  - Websocket changes:
    - Rework notifications to ensure they are delivered in the order they
      occur
    - Rename notifynewtxs command to notifyreceived (funds received)
    - Rename notifyallnewtxs command to notifynewtransactions
    - Rename alltx notification to txaccepted
    - Rename allverbosetx notification to txacceptedverbose
      (https://github.com/conformal/btcd/issues/98)
    - Add rescan progress notification
    - Add recvtx notification
    - Add redeemingtx notification
    - Modify notifyspent command to accept an array of outpoints
      (https://github.com/conformal/btcd/issues/123)
    - Significantly optimize the rescan command to yield up to a 60x speed
      increase
  - btcctl utility changes:
    - Add createencryptedwallet command
    - Add getblockchaininfo command
    - Add importwallet command
    - Add addmultisigaddress command
    - Add setgenerate command
    - Accept --testnet and --wallet flags which automatically select
      the appropriate port and TLS certificates needed to communicate
      with btcd and btcwallet (https://github.com/conformal/btcd/issues/112)
    - Allow path expansion from config file entries
      (https://github.com/conformal/btcd/issues/113)
    - Minor refactor simplify handling of options
  - addblock utility changes:
    - Improve logging by making it consistent with the logging provided by
      btcd (https://github.com/conformal/btcd/issues/90)
  - Improve several package APIs for developers:
    - Add new amount type for consistently handling monetary values
    - Add new coin selector API
    - Add new WIF (Wallet Import Format) API
    - Add new crypto types for private keys and signatures
    - Add new API to sign transactions including script merging and hash
      types
    - Expose function to extract all pushed data from a script
      (https://github.com/conformal/btcscript/issues/8)
  - Misc changes:
    - Optimize address manager shuffling to do 67% less work on average
    - Resolve a couple of benign data races found by the race detector
      (https://github.com/conformal/btcd/issues/101)
    - Add IP address to all peer related errors to clarify which peer is the
      cause (https://github.com/conformal/btcd/issues/102)
    - Fix a UPNP case issue that prevented the --upnp option from working
      with some UPNP servers
    - Update documentation in the sample config file regarding debug levels
    - Adjust some logging levels to improve debug messages
    - Improve the throughput of query messages to the block manager
    - Several minor optimizations to reduce GC churn and enhance speed
    - Other minor refactoring
    - General code cleanup

Changes in 0.7.0 (Thu Feb 20 2014)
  - Fix an issue when parsing scripts which contain a multi-signature script
    which require zero signatures such as testnet block
    000000001881dccfeda317393c261f76d09e399e15e27d280e5368420f442632
    (https://github.com/conformal/btcscript/issues/7)
  - Add check to ensure all transactions accepted to mempool only contain
    canonical data pushes (https://github.com/conformal/btcscript/issues/6)
  - Fix an issue causing excessive memory consumption
  - Significantly rework and improve the websocket notification system:
    - Each client is now independent so slow clients no longer limit the
      speed of other connected clients
    - Potentially long-running operations such as rescans are now run in
      their own handler and rate-limited to one operation at a time without
      preventing simultaneous requests from the same client for the faster
      requests or notifications
    - A couple of scenarios which could cause shutdown to hang have been
      resolved
    - Update notifynewtx notifications to support all address types instead
      of only pay-to-pubkey-hash
    - Provide a --rpcmaxwebsockets option to allow limiting the number of
      concurrent websocket clients
    - Add a new websocket command notifyallnewtxs to request notifications
      (https://github.com/conformal/btcd/issues/86) (thanks @flammit)
  - Improve btcctl utility in the following ways:
    - Add getnetworkhashps command
    - Add gettransaction command (wallet-specific)
    - Add signmessage command (wallet-specific)
    - Update getwork command to accept
  - Continue cleanup and work on implementing the RPC API:
    - Implement getnettotals command
      (https://github.com/conformal/btcd/issues/84)
    - Implement networkhashps command
      (https://github.com/conformal/btcd/issues/87)
    - Update getpeerinfo to always include syncnode field even when false
    - Remove help addenda for getpeerinfo now that it supports all fields
  - Close standard RPC connections on auth failure
  - Provide a --rpcmaxclients option to allow limiting the number of
    concurrent RPC clients (https://github.com/conformal/btcd/issues/68)
  - Include IP address in RPC auth failure log messages
  - Resolve a rather harmless data races found by the race detector
    (https://github.com/conformal/btcd/issues/94)
  - Increase block priority size and max standard transaction size to 50k
    and 100k, respectively (https://github.com/conformal/btcd/issues/71)
  - Add rate limiting of free transactions to the memory pool to prevent
    penny flooding (https://github.com/conformal/btcd/issues/40)
  - Provide a --logdir option (https://github.com/conformal/btcd/issues/95)
  - Change the default log file path to include the network
  - Add a new ScriptBuilder interface to btcscript to support creation of
    custom scripts (https://github.com/conformal/btcscript/issues/5)
  - General code cleanup

Changes in 0.6.0 (Tue Feb 04 2014)
  - Fix an issue when parsing scripts which contain invalid signatures that
    caused a chain fork on block
    0000000000000001e4241fd0b3469a713f41c5682605451c05d3033288fb2244
  - Correct an issue which could lead to an error in removeBlockNode
    (https://github.com/conformal/btcchain/issues/4)
  - Improve addblock utility as follows:
    - Check imported blocks against all chain rules and checkpoints
    - Skip blocks which are already known so you can stop and restart the
      import or start the import after you have already downloaded a portion
      of the chain
    - Correct an issue where the utility did not shutdown cleanly after
      processing all blocks
    - Add error on attempt to import orphan blocks
    - Improve error handling and reporting
    - Display statistics after input file has been fully processed
  - Rework, optimize, and improve headers-first mode:
    - Resuming the chain sync from any point before the final checkpoint
      will now use headers-first mode
      (https://github.com/conformal/btcd/issues/69)
    - Verify all checkpoints as opposed to only the final one
    - Reduce and bound memory usage
    - Rollback to the last known good point when a header does not match a
      checkpoint
    - Log information about what is happening with headers
  - Improve btcctl utility in the following ways:
    - Add getaddednodeinfo command
    - Add getnettotals command
    - Add getblocktemplate command (wallet-specific)
    - Add getwork command (wallet-specific)
    - Add getnewaddress command (wallet-specific)
    - Add walletpassphrasechange command (wallet-specific)
    - Add walletlock command (wallet-specific)
    - Add sendfrom command (wallet-specific)
    - Add sendmany command (wallet-specific)
    - Add settxfee command (wallet-specific)
    - Add listsinceblock command (wallet-specific)
    - Add listaccounts command (wallet-specific)
    - Add keypoolrefill command (wallet-specific)
    - Add getreceivedbyaccount command (wallet-specific)
    - Add getrawchangeaddress command (wallet-specific)
    - Add gettxoutsetinfo command (wallet-specific)
    - Add listaddressgroupings command (wallet-specific)
    - Add listlockunspent command (wallet-specific)
    - Add listlock command (wallet-specific)
    - Add listreceivedbyaccount command (wallet-specific)
    - Add validateaddress command (wallet-specific)
    - Add verifymessage command (wallet-specific)
    - Add sendtoaddress command (wallet-specific)
  - Continue cleanup and work on implementing the RPC API:
    - Implement submitblock command
      (https://github.com/conformal/btcd/issues/61)
    - Implement help command
    - Implement ping command
    - Implement getaddednodeinfo command
      (https://github.com/conformal/btcd/issues/78)
    - Implement getinfo command
    - Update getpeerinfo to support bytesrecv and bytessent
      (https://github.com/conformal/btcd/issues/83)
  - Improve and correct several RPC server and websocket areas:
    - Change the connection endpoint for websockets from /wallet to /ws
      (https://github.com/conformal/btcd/issues/80)
    - Implement an alternative authentication for websockets so clients
      such as javascript from browsers that don't support setting HTTP
      headers can authenticate (https://github.com/conformal/btcd/issues/77)
    - Add an authentication deadline for RPC connections
      (https://github.com/conformal/btcd/issues/68)
    - Use standard authentication failure responses for RPC connections
    - Make automatically generated certificate more standard so it works
      from client such as node.js and Firefox
    - Correct some minor issues which could prevent the RPC server from
      shutting down in an orderly fashion
    - Make all websocket notifications require registration
    - Change the data sent over websockets to text since it is JSON-RPC
    - Allow connections that do not have an Origin header set
  - Expose and track the number of bytes read and written per peer
    (https://github.com/conformal/btcwire/issues/6)
  - Correct an issue with sendrawtransaction when invoked via websockets
    which prevented a minedtx notification from being added
  - Rescan operations issued from remote wallets are no stopped when
    the wallet disconnects mid-operation
    (https://github.com/conformal/btcd/issues/66)
  - Several optimizations related to fetching block information from the
    database
  - General code cleanup

Changes in 0.5.0 (Mon Jan 13 2014)
  - Optimize initial block download by introducing a new mode which
    downloads the block headers first (up to the final checkpoint)
  - Improve peer handling to remove the potential for slow peers to cause
    sluggishness amongst all peers
    (https://github.com/conformal/btcd/issues/63)
  - Fix an issue where the initial block sync could stall when the sync peer
    disconnects (https://github.com/conformal/btcd/issues/62)
  - Correct an issue where --externalip was doing a DNS lookup on the full
    host:port instead of just the host portion
    (https://github.com/conformal/btcd/issues/38)
  - Fix an issue which could lead to a panic on chain switches
    (https://github.com/conformal/btcd/issues/70)
  - Improve btcctl utility in the following ways:
    - Show getdifficulty output as floating point to 6 digits of precision
    - Show all JSON object replies formatted as standard JSON
    - Allow btcctl getblock to accept optional params
    - Add getaccount command (wallet-specific)
    - Add getaccountaddress command (wallet-specific)
    - Add sendrawtransaction command
  - Continue cleanup and work on implementing RPC API calls
    - Update getrawmempool to support new optional verbose flag
    - Update getrawtransaction to match the reference client
    - Update getblock to support new optional verbose flag
    - Update raw transactions to fully match the reference client including
      support for all transaction types and address types
    - Correct getrawmempool fee field to return BTC instead of Satoshi
    - Correct getpeerinfo service flag to return 8 digit string so it
      matches the reference client
    - Correct verifychain to return a boolean
    - Implement decoderawtransaction command
    - Implement createrawtransaction command
    - Implement decodescript command
    - Implement gethashespersec command
    - Allow RPC handler overrides when invoked via a websocket versus
      legacy connection
  - Add new DNS seed for peer discovery
  - Display user agent on new valid peer log message
    (https://github.com/conformal/btcd/issues/64)
  - Notify wallet when new transactions that pay to registered addresses
    show up in the mempool before being mined into a block
  - Support a tor-specific proxy in addition to a normal proxy
    (https://github.com/conformal/btcd/issues/47)
  - Remove deprecated sqlite3 imports from utilities
  - Remove leftover profile write from addblock utility
  - Quite a bit of code cleanup and refactoring to improve maintainability

Changes in 0.4.0 (Thu Dec 12 2013)
  - Allow listen interfaces to be specified via --listen instead of only the
    port (https://github.com/conformal/btcd/issues/33)
  - Allow listen interfaces for the RPC server to be specified via
    --rpclisten instead of only the port
    (https://github.com/conformal/btcd/issues/34)
  - Only disable listening when --connect or --proxy are used when no
    --listen interface are specified
    (https://github.com/conformal/btcd/issues/10)
  - Add several new standard transaction checks to transaction memory pool:
    - Support nulldata scripts as standard
    - Only allow a max of one nulldata output per transaction
    - Enforce a maximum of 3 public keys in multi-signature transactions
    - The number of signatures in multi-signature transactions must not
      exceed the number of public keys
    - The number of inputs to a signature script must match the expected
      number of inputs for the script type
    - The number of inputs pushed onto the stack by a redeeming signature
      script must match the number of inputs consumed by the referenced
      public key script
  - When a block is connected, remove any transactions from the memory pool
    which are now double spends as a result of the newly connected
    transactions
  - Don't relay transactions resurrected during a chain switch since
    other peers will also be switching chains and therefore already know
    about them
  - Cleanup a few cases where rejected transactions showed as an error
    rather than as a rejected transaction
  - Ignore the default configuration file when --regtest (regression test
    mode) is specified
  - Implement TLS support for RPC including automatic certificate generation
  - Support HTTP authentication headers for web sockets
  - Update address manager to recognize and properly work with Tor
    addresses (https://github.com/conformal/btcd/issues/36) and
    (https://github.com/conformal/btcd/issues/37)
  - Improve btcctl utility in the following ways:
    - Add the ability to specify a configuration file
    - Add a default entry for the RPC cert to point to the location
      it will likely be in the btcd home directory
    - Implement --version flag
    - Provide a --notls option to support non-TLS configurations
  - Fix a couple of minor races found by the Go race detector
  - Improve logging
    - Allow logging level to be specified on a per subsystem basis
      (https://github.com/conformal/btcd/issues/48)
    - Allow logging levels to be dynamically changed via RPC
      (https://github.com/conformal/btcd/issues/15)
    - Implement a rolling log file with a max of 10MB per file and a
      rotation size of 3 which results in a max logging size of 30 MB
  - Correct a minor issue with the rescanning websocket call
    (https://github.com/conformal/btcd/issues/54)
  - Fix a race with pushing address messages that could lead to a panic
    (https://github.com/conformal/btcd/issues/58)
  - Improve which external IP address is reported to peers based on which
    interface they are connected through
    (https://github.com/conformal/btcd/issues/35)
  - Add --externalip option to allow an external IP address to be specified
    for cases such as tor hidden services or advanced network configurations
    (https://github.com/conformal/btcd/issues/38)
  - Add --upnp option to support automatic port mapping via UPnP
    (https://github.com/conformal/btcd/issues/51)
  - Update Ctrl+C interrupt handler to properly sync address manager and
    remove the UPnP port mapping (if needed)
  - Continue cleanup and work on implementing RPC API calls
    - Add importprivkey (import private key) command to btcctl
    - Update getrawtransaction to provide addresses properly, support
      new verbose param, and match the reference implementation with the
      exception of MULTISIG (thanks @flammit)
    - Update getblock with new verbose flag (thanks @flammit)
    - Add listtransactions command to btcctl
    - Add getbalance command to btcctl
  - Add basic support for btcd to run as a native Windows service
    (https://github.com/conformal/btcd/issues/42)
  - Package addblock utility with Windows MSIs
  - Add support for TravisCI (continuous build integration)
  - Cleanup some documentation and usage
  - Several other minor bug fixes and general code cleanup

Changes in 0.3.3 (Wed Nov 13 2013)
  - Significantly improve initial block chain download speed
    (https://github.com/conformal/btcd/issues/20)
  - Add a new checkpoint at block height 267300
  - Optimize most recently used inventory handling
    (https://github.com/conformal/btcd/issues/21)
  - Optimize duplicate transaction input check
    (https://github.com/conformal/btcchain/issues/2)
  - Optimize transaction hashing
    (https://github.com/conformal/btcd/issues/25)
  - Rework and optimize wallet listener notifications
    (https://github.com/conformal/btcd/issues/22)
  - Optimize serialization and deserialization
    (https://github.com/conformal/btcd/issues/27)
  - Add support for minimum transaction fee to memory pool acceptance
    (https://github.com/conformal/btcd/issues/29)
  - Improve leveldb database performance by removing explicit GC call
  - Fix an issue where Ctrl+C was not always finishing orderly database
    shutdown
  - Fix an issue in the script handling for OP_CHECKSIG
  - Impose max limits on all variable length protocol entries to prevent
    abuse from malicious peers
  - Enforce DER signatures for transactions allowed into the memory pool
  - Separate the debug profile http server from the RPC server
  - Rework of the RPC code to improve performance and make the code cleaner
  - The getrawtransaction RPC call now properly checks the memory pool
    before consulting the db (https://github.com/conformal/btcd/issues/26)
  - Add support for the following RPC calls: getpeerinfo, getconnectedcount,
    addnode, verifychain
    (https://github.com/conformal/btcd/issues/13)
    (https://github.com/conformal/btcd/issues/17)
  - Implement rescan websocket extension to allow wallet rescans
  - Use correct paths for application data storage for all supported
    operating systems (https://github.com/conformal/btcd/issues/30)
  - Add a default redirect to the http profiling page when accessing the
    http profile server
  - Add a new --cpuprofile option which can be used to generate CPU
    profiling data on platforms that support it
  - Several other minor performance optimizations
  - Other minor bug fixes and general code cleanup

Changes in 0.3.2 (Tue Oct 22 2013)
  - Fix an issue that could cause the download of the block chain to stall
    (https://github.com/conformal/btcd/issues/12)
  - Remove deprecated sqlite as an available database backend
  - Close sqlite compile issue as sqlite has now been removed
    (https://github.com/conformal/btcd/issues/11)
  - Change default RPC ports to 8334 (mainnet) and 18334 (testnet)
  - Continue cleanup and work on implementing RPC API calls
  - Add support for the following RPC calls: getrawmempool,
    getbestblockhash, decoderawtransaction, getdifficulty,
    getconnectioncount, getpeerinfo, and addnode
  - Improve the btcctl utility that is used to issue JSON-RPC commands
  - Fix an issue preventing btcd from cleanly shutting down with the RPC
    stop command
  - Add a number of database interface tests to ensure backends implement
    the expected interface
  - Expose some additional information from btcscript to be used for
    identifying "standard"" transactions
  - Add support for plan9 - thanks @mischief
    (https://github.com/conformal/btcd/pull/19)
  - Other minor bug fixes and general code cleanup

Changes in 0.3.1-alpha (Tue Oct 15 2013)
  - Change default database to leveldb
    NOTE: This does mean you will have to redownload the block chain.  Since we
    are still in alpha, we didn't feel writing a converter was worth the time as
    it would take away from more important issues at this stage
  - Add a warning if there are multiple block chain databases of different types
  - Fix issue with unexpected EOF in leveldb -- https://github.com/conformal/btcd/issues/18
  - Fix issue preventing block 21066 on testnet -- https://github.com/conformal/btcchain/issues/1
  - Fix issue preventing block 96464 on testnet -- https://github.com/conformal/btcscript/issues/1
  - Optimize transaction lookups
  - Correct a few cases of list removal that could result in improper cleanup
    of no longer needed orphans
  - Add functionality to increase ulimits on non-Windows platforms
  - Add support for mempool command which allows remote peers to query the
    transaction memory pool via the bitcoin protocol
  - Clean up logging a bit
  - Add a flag to disable checkpoints for developers
  - Add a lot of useful debug logging such as message summaries
  - Other minor bug fixes and general code cleanup

Initial Release 0.3.0-alpha (Sat Oct 05 2013):
  - Initial release
