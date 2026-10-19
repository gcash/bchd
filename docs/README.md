### Table of Contents

1. [About](#About)
2. [Getting Started](#GettingStarted)
   1. [Installation](#Installation)
      1. [Building From Source](#BuildingFromSource)
   2. [Configuration](#Configuration)
      1. [Checkpoints and Initial Block Download](#Checkpoints)
      2. [Finalization and Deep Reorganizations](#Finalization)
   3. [Controlling and Querying bchd via bchctl](#BchctlConfig)
   4. [Mining](#Mining)
3. [Help](#Help)
   1. [Startup](#Startup)
      1. [Using bootstrap.dat](#BootstrapDat)
   2. [Network Configuration](#NetworkConfig)
   3. [Memory Use](#MemoryUse)
   4. [Wallet](#Wallet)
4. [Developer Resources](#DeveloperResources)
   1. [Code Contribution Guidelines](#ContributionGuidelines)
   2. [JSON-RPC Reference](#JSONRPCReference)
   3. [The gcash Bitcoin Cash-related Go Packages](#GoPackages)

<a name="About" />

### 1. About

bchd is an alternative full node bitcoin cash implementation written in Go (golang).

This project is a port of the [btcd](https://github.com/btcsuite/btcd) codebase to Bitcoin Cash. It provides a high powered
and reliable blockchain server which makes it a suitable backend to serve blockchain data to lite clients and block explorers
or to power your local wallet.

bchd does not include any wallet functionality by design as it makes the codebase more modular and easy to maintain.
The [bchwallet](https://github.com/gcash/bchwallet) is a separate application that provides a secure Bitcoin Cash wallet
that communicates with your running bchd instance via the API.

<a name="GettingStarted" />

### 2. Getting Started

<a name="Installation" />

**2.1 Installation**

The easiest way to run the server is to download a pre-built binary. You can find binaries of our latest release for each operating system at the [releases page](https://github.com/gcash/bchd/releases).

<a name="BuildingFromSource" />

**2.1.1 Building From Source**

If you prefer to install from source do the following:

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Run the following commands to obtain bchd, all dependencies, and install it:

```bash
$ go get github.com/gcash/bchd
```

This will download and compile `bchd` and put it in your path.

<a name="Configuration" />

**2.2 Configuration**

bchd has a number of [configuration](http://godoc.org/github.com/gcash/bchd)
options, which can be viewed by running: `$ bchd --help`.

<a name="Checkpoints" />

**2.2.1 Checkpoints and Initial Block Download**

bchd has built-in checkpoints for mainnet, testnet3, testnet4 and chipnet, the
same as Bitcoin Cash Node's; the latest are mainnet block 970892, testnet3 block
1730264, testnet4 block 325709 and chipnet block 325627. Blocks at the height
of a checkpoint must match it, and blocks that fork the chain before the last
checkpoint are rejected.

During the initial block download, bchd first fetches the headers up to the
last checkpoint and then adds the blocks up to it with only the checks that
don't depend on the chain state, such as proof of work, the merkle root, the
block size and the transaction format. Their spends, fees, coinbase value and
scripts are not checked, which makes the download much faster. Blocks at or
below the last checkpoint that arrive any other way get every check except
scripts. `--nocheckpoints` turns this off and validates every block in full.
The node does not consider itself synced until its tip is past the last
checkpoint.

Each of these networks also has a minimum chain work, again Bitcoin Cash Node's:
the total proof of work its chain is known to have. Until the best chain has
at least that much work, the node does not consider itself synced either, so
it does not relay blocks, answer `getheaders` or filter requests, or build
block templates. A block that a peer relays without being asked for it is only
stored if its chain has at least the minimum chain work and the work of the
current tip, and is at most 288 blocks ahead of the tip.

`addcheckpoint` adds checkpoints and `nocheckpoints` disables the built-in
ones, in which case every script is validated. With `fastsync`, the UTXO set of
the last checkpoint that has one (mainnet block 661648) is downloaded instead,
and only the checkpoints up to that block are used.

<a name="Finalization" />

**2.2.2 Finalization and Deep Reorganizations**

Once a main chain block is 10 blocks deep (`maxreorgdepth`) and has been known
for two hours (`finalizationdelay`, in seconds), it is finalized: blocks that
fork the chain below the finalized block are rejected, so the node will not
reorganize its chain deeper than that. Nothing is finalized until the node has
been running for the finalization delay, and blocks loaded from disk at startup
count as known since startup. The finalized block is kept in memory only. A
negative `maxreorgdepth` disables finalization.

```
[Application Options]
maxreorgdepth=10
finalizationdelay=7200
```

The `getfinalizedblockhash` RPC returns the finalized block, and `finalizeblock`
finalizes a block by hand.

If the network does reorganize below the finalized block, for example after a
chain split, the node stays on its chain: blocks of the other chain are
rejected without being stored, and peers that send them are disconnected. To
follow the other chain, call `invalidateblock` with the first block of the
node's chain after the fork point. This disconnects the node's chain down to
the fork point and moves finalization back with it, after which the other chain
is fetched from peers again. `reconsiderblock` on a block the node stored
earlier also moves finalization back to where its chain forks.
A block of the other chain that the node has already stored can instead be
passed to `finalizeblock`, which disconnects the node's chain down to the fork
point, marks it invalid and switches to the other chain up to that block.
Starting the node with `maxreorgdepth=-1` disables finalization, and
restarting it clears the finalized block until the finalization delay has
passed again.

<a name="BchctlConfig" />

**2.3 Controlling and Querying bchd via bchctl**

bchctl is a command line utility that can be used to both control and query bchd
via [RPC](http://www.wikipedia.org/wiki/Remote_procedure_call). bchd does
**not** enable its RPC server by default; You must configure at minimum both an
RPC username and password or both an RPC limited username and password:

- bchd.conf configuration file

```
[Application Options]
rpcuser=myuser
rpcpass=SomeDecentp4ssw0rd
rpclimituser=mylimituser
rpclimitpass=Limitedp4ssw0rd
```

- bchctl.conf configuration file

```
[Application Options]
rpcuser=myuser
rpcpass=SomeDecentp4ssw0rd
```

OR

```
[Application Options]
rpclimituser=mylimituser
rpclimitpass=Limitedp4ssw0rd
```

For a list of available options, run: `$ bchctl --help`

<a name="Mining" />

**2.4 Mining**

bchd supports the `getblocktemplate` RPC.
The limited user cannot access this RPC.

**1. Add the payment addresses with the `miningaddr` option.**

```
[Application Options]
rpcuser=myuser
rpcpass=SomeDecentp4ssw0rd
miningaddr=12c6DSiU4Rq3P4ZxziKxzrL5LmMBrzjrJX
miningaddr=1M83ju3EChKYyysmM2FXtLNftbacagd8FR
```

**2. Add bchd's RPC TLS certificate to system Certificate Authority list.**

`cgminer` uses [curl](http://curl.haxx.se/) to fetch data from the RPC server.
Since curl validates the certificate by default, we must install the `bchd` RPC
certificate into the default system Certificate Authority list.

**Ubuntu**

1. Grant root privileges: `# sudo su -`
2. Copy rpc.cert to /usr/share/ca-certificates: `# cp /home/{USER}/.bchd/rpc.cert /usr/share/ca-certificates/bchd.crt`
3. Add "bchd.crt" to /etc/ca-certificates.conf: `# echo bchd.crt >> /etc/ca-certificates.conf`
4. Update the CA certificate list: `# update-ca-certificates`

**3. Set your mining software url to use https.**

`$ cgminer -o https://127.0.0.1:8334 -u rpcuser -p rpcpassword`

<a name="Help" />

### 3. Help

<a name="Startup" />

**3.1 Startup**

Typically bchd will run and start downloading the block chain with no extra
configuration necessary, however, there is an optional method to use a
`bootstrap.dat` file that may speed up the initial block chain download process.

<a name="BootstrapDat" />

**3.1.1 bootstrap.dat**

- [Using bootstrap.dat](https://github.com/gcash/bchd/tree/master/docs/using_bootstrap_dat.md)

<a name="NetworkConfig" />

**3.2 Network Configuration**

- [What Ports Are Used by Default?](https://github.com/gcash/bchd/tree/master/docs/default_ports.md)
- [How To Listen on Specific Interfaces](https://github.com/gcash/bchd/tree/master/docs/configure_peer_server_listen_interfaces.md)
- [How To Configure RPC Server to Listen on Specific Interfaces](https://github.com/gcash/bchd/tree/master/docs/configure_rpc_server_listen_interfaces.md)
- [Configuring bchd with Tor](https://github.com/gcash/bchd/tree/master/docs/configuring_tor.md)
- [Configuring bchd with CIFS](https://github.com/gcash/bchd/tree/master/docs/cifs.md)

<a name="MemoryUse" />

**3.3 Memory Use**

bchd keeps recently used unspent outputs (the UTXO cache) and pending database
writes in memory to speed up syncing. With the default cache sizes of 450 MiB
and 500 MiB it can use a gigabyte or more. Right after the first sync its
memory use can stay high for half an hour or so, until the database cache is
written to disk. To use less memory, lower the cache sizes (in MiB); smaller
caches make the first sync slower:

```
[Application Options]
utxocachemaxsize=250
dbcachesize=250
```

On Linux, the memory reported for bchd can also keep growing after sync even
though bchd is not using it. This comes from a kernel setting called
transparent huge pages. If `cat /sys/kernel/mm/transparent_hugepage/enabled`
shows `[always]`, do one of the following:

- Change it to `madvise`, which most distributions use by default. This lasts
  until the next reboot:

  ```
  echo madvise | sudo tee /sys/kernel/mm/transparent_hugepage/enabled
  ```

- Turn it off for bchd only by starting it with `GODEBUG=disablethp=1 bchd`.

<a name="Wallet" />

**3.4 Wallet**

bchd was intentionally developed without an integrated wallet for security
reasons. Please see [bchwallet](https://github.com/gcash/bchwallet) for more
information.

<a name="DeveloperResources" />

### 4. Developer Resources

<a name="ContributionGuidelines" />

- [Code Contribution Guidelines](https://github.com/gcash/bchd/tree/master/docs/code_contribution_guidelines.md)

<a name="JSONRPCReference" />

- [JSON-RPC Reference](https://github.com/gcash/bchd/tree/master/docs/json_rpc_api.md)
  - [RPC Examples](https://github.com/gcash/bchd/tree/master/docs/json_rpc_api.md#ExampleCode)

<a name="GoPackages" />

- The gcash Bitcoin Cash-related Go Packages:
  - [rpcclient](https://github.com/gcash/bchd/tree/master/rpcclient) - Implements a
    robust and easy to use Websocket-enabled Bitcoin JSON-RPC client
  - [btcjson](https://github.com/gcash/bchd/tree/master/btcjson) - Provides an extensive API
    for the underlying JSON-RPC command and return values
  - [wire](https://github.com/gcash/bchd/tree/master/wire) - Implements the
    Bitcoin wire protocol
  - [peer](https://github.com/gcash/bchd/tree/master/peer) -
    Provides a common base for creating and managing Bitcoin network peers.
  - [blockchain](https://github.com/gcash/bchd/tree/master/blockchain) -
    Implements Bitcoin block handling and chain selection rules
  - [blockchain/fullblocktests](https://github.com/gcash/bchd/tree/master/blockchain/fullblocktests) -
    Provides a set of block tests for testing the consensus validation rules
  - [txscript](https://github.com/gcash/bchd/tree/master/txscript) -
    Implements the Bitcoin transaction scripting language
  - [bchec](https://github.com/gcash/bchd/tree/master/bchec) - Implements
    support for the elliptic curve cryptographic functions needed for the
    Bitcoin scripts
  - [database](https://github.com/gcash/bchd/tree/master/database) -
    Provides a database interface for the Bitcoin block chain
  - [mempool](https://github.com/gcash/bchd/tree/master/mempool) -
    Package mempool provides a policy-enforced pool of unmined bitcoin
    transactions.
  - [bchutil](https://github.com/gcash/bchutil) - Provides Bitcoin-specific
    convenience functions and types
  - [chainhash](https://github.com/gcash/bchd/tree/master/chaincfg/chainhash) -
    Provides a generic hash type and associated functions that allows the
    specific hash algorithm to be abstracted.
  - [connmgr](https://github.com/gcash/bchd/tree/master/connmgr) -
    Package connmgr implements a generic Bitcoin network connection manager.
