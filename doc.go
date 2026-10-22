// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/*
bchd is a full-node bitcoin cash implementation written in Go.

The default options are sane for most users.  This means bchd will work 'out of
the box' for most users.  However, there are also a wide variety of flags that
can be used to control it.

The following section provides a usage overview which enumerates the flags.  An
interesting point to note is that the long form of all of these options
(except -C) can be specified in a configuration file that is automatically
parsed when bchd starts up.  By default, the configuration file is located at
~/.bchd/bchd.conf on POSIX-style operating systems and %LOCALAPPDATA%\bchd\bchd.conf
on Windows.  The -C (--configfile) flag, as shown below, can be used to override
this location.

Usage:

	bchd [OPTIONS]

Application Options:

	-V, --version                  Display version information and exit
	-C, --configfile=              Path to configuration file (default:
	                               ~/.bchd/bchd.conf)
	-b, --datadir=                 Directory to store data (default:
	                               ~/.bchd/data)
	    --logdir=                  Directory to log output. (default:
	                               ~/.bchd/logs)
	-a, --addpeer=                 Add a peer to connect with at startup
	    --connect=                 Connect only to the specified peers at startup
	    --nolisten                 Disable listening for incoming connections --
	                               NOTE: Listening is automatically disabled if
	                               the --connect or --proxy options are used
	                               without also specifying listen interfaces via
	                               --listen
	    --listen=                  Add an interface/port to listen for
	                               connections (default all interfaces port:
	                               8333, testnet: 18333)
	    --maxpeers=                Max number of inbound and outbound peers
	                               (default: 125)
	    --maxpeersperip=           Max number of inbound and outbound peers per
	                               IP (default: 5)
	    --minsyncpeernetworkspeed= Disconnect sync peers slower than this
	                               threshold in bytes/sec (default: 51200)
	    --nobanning                Disable banning of misbehaving peers
	    --banduration=             How long to ban misbehaving peers.  Valid time
	                               units are {s, m, h}.  Minimum 1 second
	                               (default: 24h0m0s)
	    --banthreshold=            Maximum allowed ban score before disconnecting
	                               and banning misbehaving peers. (default: 100)
	    --whitelist=               Add an IP network or IP that will not be
	                               banned. (eg. 192.168.1.0/24 or ::1)
	    --agentblacklist=          A comma separated list of user-agent
	                               substrings which will cause bchd to reject any
	                               peers whose user-agent contains any of the
	                               blacklisted substrings.
	    --agentwhitelist=          A comma separated list of user-agent
	                               substrings which will cause bchd to require
	                               all peers' user-agents to contain one of the
	                               whitelisted substrings. The blacklist is
	                               applied before the whitelist, and an empty
	                               whitelist will allow all agents that do not
	                               fail the blacklist.
	-u, --rpcuser=                 Username for RPC connections
	-P, --rpcpass=                 Password for RPC connections
	    --rpclimituser=            Username for limited RPC connections
	    --rpclimitpass=            Password for limited RPC connections
	    --rpclisten=               Add an interface/port to listen for RPC
	                               connections (default port: 8334, testnet:
	                               18334)
	    --rpccert=                 File containing the certificate file (default:
	                               ~/.bchd/rpc.cert)
	    --rpckey=                  File containing the certificate key (default:
	                               ~/.bchd/rpc.key)
	    --rpcmaxclients=           Max number of RPC clients for standard
	                               connections (default: 10)
	    --rpcmaxwebsockets=        Max number of RPC websocket connections
	                               (default: 25)
	    --rpcmaxconcurrentreqs=    Max number of concurrent RPC requests that may
	                               be processed concurrently (default: 20)
	    --rpcquirks                Mirror some JSON-RPC quirks of Bitcoin Core --
	                               NOTE: Discouraged unless interoperability
	                               issues need to be worked around
	    --rpcauthtimeout=          The number of seconds a connection to the RPC
	                               server is allowed to stay open without
	                               authenticating. To disable the timeout use 0.
	                               (default: 10)
	    --norpc                    Disable built-in RPC server -- NOTE: The RPC
	                               server is disabled by default if no
	                               rpcuser/rpcpass or rpclimituser/rpclimitpass
	                               is specified
	    --notls                    Disable TLS for the RPC server -- NOTE: This
	                               is only allowed if the RPC server is bound to
	                               localhost
	    --nodnsseed                Disable DNS seeding for peers
	    --externalip=              Add an ip or Tor v3 onion address to the list
	                               of local addresses we claim to listen on to
	                               peers
	    --proxy=                   Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)
	    --proxyuser=               Username for proxy server
	    --proxypass=               Password for proxy server
	    --onion=                   Connect to tor hidden services via SOCKS5
	                               proxy (eg. 127.0.0.1:9050)
	    --onionuser=               Username for onion proxy server
	    --onionpass=               Password for onion proxy server
	    --noonion                  Disable connecting to tor hidden services
	    --torisolation             Enable Tor stream isolation by randomizing
	                               user credentials for each connection.
	    --testnet                  Use the test network
	    --testnet4                 Use the test 4 network
	    --chipnet                  Use the chip network
	    --regtest                  Use the regression test network
	    --regtestanyhost           In regression test mode, allow connections
	                               from any host, not just localhost
	    --regtestnoreset           In regression test mode, don't reset the
	                               network db on node restart
	    --simnet                   Use the simulation test network
	    --addcheckpoint=           Add a custom checkpoint.  Format:
	                               '<height>:<hash>'
	    --nocheckpoints            Disable built-in checkpoints.  Don't do this
	                               unless you know what you're doing.
	    --maxreorgdepth=           Depth at which main chain blocks are
	                               finalized, so that blocks forking the chain
	                               below them are rejected (-1 to disable)
	                               (default: 10)
	    --finalizationdelay=       Time in seconds a block must have been known,
	                               and the node running, before the block can be
	                               finalized (default: 7200)
	    --dbtype=                  Database backend to use for the Block Chain
	                               (default: ffldb)
	    --profile=                 Enable HTTP profiling on the given port on
	                               localhost, or on IP:port -- NOTE port must be
	                               between 1024 and 65535
	    --cpuprofile=              Write CPU profile to the specified file
	-d, --debuglevel=              Logging level for all subsystems {trace,
	                               debug, info, warn, error, critical} -- You may
	                               also specify
	                               <subsystem>=<level>,<subsystem2>=<level>,...
	                               to set the log level for individual subsystems
	                               -- Use show to list available subsystems
	                               (default: info)
	    --upnp                     Use UPnP to map our listening port outside of
	                               NAT
	    --excessiveblocksize=      The maximum size block (in bytes) this node
	                               will accept -- 0 uses the network's default,
	                               which is also the minimum: 2000000 on testnet4
	                               and chipnet, otherwise 32000000
	    --minrelaytxfee=           The minimum fee rate in BCH/kB a transaction
	                               must pay to be relayed, accepted into the
	                               mempool and mined -- Must be above 0 (default:
	                               1e-05)
	    --maxtxfee=                The maximum total fee in BCH of a transaction
	                               sent with sendrawtransaction or checked with
	                               testmempoolaccept, unless allowhighfees is set
	                               (default: 0.1)
	    --limitfreerelay=          Deprecated, has no effect: free transactions
	                               are no longer relayed
	    --norelaypriority          Deprecated, has no effect: transaction
	                               priority is no longer used
	    --trickleinterval=         Minimum time between attempts to send new
	                               inventory to a connected peer (default: 50ms)
	    --maxorphantx=             Max number of orphan transactions to keep in
	                               memory (default: 100)
	    --generate                 Generate (mine) bitcoins using the CPU
	    --miningaddr=              Add the specified payment address to the list
	                               of addresses to use for generated blocks -- At
	                               least one address is required if the generate
	                               option is set
	    --blockminsize=            Deprecated, has no effect: blocks no longer
	                               include transactions below the minimum relay
	                               fee
	    --blockmaxsize=            Maximum block size in bytes to be used when
	                               creating a block -- 0 follows the consensus
	                               limit less 1000 bytes
	    --blockprioritysize=       Deprecated, has no effect: blocks no longer
	                               have a high-priority area
	    --cbflags=                 Comment to append to the coinbase input when
	                               generating a block template. (default: /bchd/)
	    --uacomment=               Comment to add to the user agent -- See BIP 14
	                               for more information.
	    --nopeerbloomfilters       Disable bloom filtering support
	    --nocfilters               Disable committed filtering (CF) support
	    --dropcfindex              Deletes the index used for committed filtering
	                               (CF) support from the database on start up and
	                               then exits.
	    --sigcachemaxsize=         The maximum number of entries in the signature
	                               verification cache (default: 100000)
	    --utxocachemaxsize=        The maximum size in MiB of the UTXO cache
	                               (default: 450)
	    --gcpercent=               Garbage collection target percentage -- Higher
	                               values use more memory and less CPU; a
	                               negative value disables the collector (use
	                               with GOMEMLIMIT). Ignored if GOGC is set
	                               (default: 50)
	    --blocksonly               Do not accept transactions from remote peers.
	    --nodoublespendproof       Do not create, relay or answer queries for
	                               double spend proofs
	    --txindex                  Maintain a full hash-based transaction index
	                               which makes all transactions available via the
	                               getrawtransaction RPC
	    --droptxindex              Deletes the hash-based transaction index from
	                               the database on start up and then exits.
	    --addrindex                Maintain a full address-based transaction
	                               index which makes the searchrawtransactions
	                               RPC available
	    --dropaddrindex            Deletes the address-based transaction index
	                               from the database on start up and then exits.
	    --slpindex                 Maintain an index which makes slp transaction
	                               validity and token metadata available via
	                               various gRPC methods
	    --slpcachemaxsize=         The maximum number of entries in the slp
	                               indexer cache (default: 100000)
	    --dropslpindex             Deletes the slp index from the database on
	                               start up and then exits.
	    --slpgraphsearch           Enables gRPC calls related to slp graph search.
	    --relaynonstd              Relay non-standard transactions regardless of
	                               the default settings for the active network.
	    --rejectnonstd             Reject non-standard transactions regardless of
	                               the default settings for the active network.
	    --prune                    Delete historical blocks from the chain. A
	                               buffer of blocks will be retained in case of a
	                               reorg.
	    --prunedepth=              The number of blocks to retain when running in
	                               pruned mode. Cannot be less than 288.
	                               (default: 4320)
	    --targetoutboundpeers=     Number of outbound connections to maintain
	                               (default: 8)
	    --reindexchainstate        Rebuild the UTXO database from currently
	                               indexed blocks on disk.
	    --fastsync                 Download the UTXO set of the last checkpoint
	                               that has one and sync full blocks from there
	                               to the tip rather than from genesis.
	    --grpclisten=              Add an interface/port to listen for
	                               experimental gRPC connections (default port:
	                               8335, testnet: 18335)
	    --grpcauthtoken=           An authentication token for the gRPC API to
	                               authenticate clients
	    --dbcachesize=             The maximum size in MiB of the database cache
	                               (default: 500)
	    --dbflushinterval=         The number of seconds between database flushes
	                               (default: 1800)
	    --prometheus=              Specify an (addr):port to serve prometheus
	                               metrics (for example :9000 or
	                               my-interface:9000, default disabled)

Help Options:

	-h, --help                     Show this help message
*/
package main
