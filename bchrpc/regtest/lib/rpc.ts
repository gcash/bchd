export interface BitcoinRpcClient {
    getBlockchainInfo: () => Promise<RpcBlockchainInfoRes>;
    addNode: (node: string, command: string) => Promise<void>;  // "command" can be 'add', 'remove', or 'onetry'
    getPeerInfo: () => Promise<RpcPeerInfoRes[]>;
    getNewAddress: (label?: string) => Promise<string>;
    dumpPrivKey: (address: string) => Promise<string>;
    listUnspent: (minconf?: number, maxconf?: number, addresses?: string[], include_safe?: boolean, query_options?: ListUnspentQueryOptions) => Promise<RpcListUnspentRes[]>;
    getRawTransaction: (tid: string, verbose?: boolean, blockhash?: string) => Promise<string|any>;
    sendRawTransaction: (txnHex: string, allowHighFees?: boolean ) => Promise<string>;
    generateToAddress: (nblocks: number, address: string, maxtries?: number) => Promise<string[]>;
    generate: (nblocks: number) => Promise<string[]>;
}

export interface RpcBlockchainInfoRes {
    chain: string;                  // "chain": "xxxx",              (string) current network name as defined in BIP70 (main, test, regtest)
    blocks: number;                 // "blocks": xxxxxx,             (numeric) the current number of blocks processed in the server
    headers: number;                // "headers": xxxxxx,            (numeric) the current number of headers we have validated
    bestblockhash: string;          // "bestblockhash": "...",       (string) the hash of the currently best block
    difficulty: number;             // "difficulty": xxxxxx,         (numeric) the current difficulty
    mediantime: number;             // "mediantime": xxxxxx,         (numeric) median time for the current best block
    verificationprogress: number;   // "verificationprogress": xxxx, (numeric) estimate of verification progress [0..1]
    initialblockdownload: boolean;  // "initialblockdownload": xxxx, (bool) (debug information) estimate of whether this node is in Initial Block Download mode.
    chainwork: string;              // "chainwork": "xxxx"           (string) total amount of work in active chain, in hexadecimal
    size_on_disk: number;           // "size_on_disk": xxxxxx,       (numeric) the estimated size of the block and undo files on disk
    pruned: boolean;                // "pruned": xx,                 (boolean) if the blocks are subject to pruning
    pruneheight: number;            // "pruneheight": xxxxxx,        (numeric) lowest-height complete block stored (only present if pruning is enabled)
    automatic_pruning: boolean;     // "automatic_pruning": xx,      (boolean) whether automatic pruning is enabled (only present if pruning is enabled)
    prune_target_size: number;      // "prune_target_size": xxxxxx,  (numeric) the target size used by pruning (only present if automatic pruning is enabled)
    warnings: string;               // "warnings" : "...",           (string) any network and blockchain warnings.
}

export interface RpcPeerInfoRes {
    id: number;
    addr: string;
    addnode: boolean;

    // TODO: type this interface
    //     "id": n,                   (numeric) Peer index
    //     "addr":"host:port",      (string) The IP address and port of the peer
    //     "addrbind":"ip:port",    (string) Bind address of the connection to the peer
    //     "addrlocal":"ip:port",   (string) Local address as reported by the peer
    //     "services":"xxxxxxxxxxxxxxxx",   (string) The services offered
    //     "relaytxes":true|false,    (boolean) Whether peer has asked us to relay transactions to it
    //     "lastsend": ttt,           (numeric) The time in seconds since epoch (Jan 1 1970 GMT) of the last send
    //     "lastrecv": ttt,           (numeric) The time in seconds since epoch (Jan 1 1970 GMT) of the last receive
    //     "bytessent": n,            (numeric) The total bytes sent
    //     "bytesrecv": n,            (numeric) The total bytes received
    //     "conntime": ttt,           (numeric) The connection time in seconds since epoch (Jan 1 1970 GMT)
    //     "timeoffset": ttt,         (numeric) The time offset in seconds
    //     "pingtime": n,             (numeric) ping time (if available)
    //     "minping": n,              (numeric) minimum observed ping time (if any at all)
    //     "pingwait": n,             (numeric) ping wait (if non-zero)
    //     "version": v,              (numeric) The peer version, such as 70001
    //     "subver": "/Satoshi:0.8.5/",  (string) The string version
    //     "inbound": true|false,     (boolean) Inbound (true) or Outbound (false)
    //     "addnode": true|false,     (boolean) Whether connection was due to addnode/-connect or if it was an automatic/inbound connection
    //     "startingheight": n,       (numeric) The starting height (block) of the peer
    //     "banscore": n,             (numeric) The ban score
    //     "synced_headers": n,       (numeric) The last header we have in common with this peer
    //     "synced_blocks": n,        (numeric) The last block we have in common with this peer
    //     "inflight": [
    //        n,                        (numeric) The heights of blocks we're currently asking from this peer
    //        ...
    //     ],
    //     "whitelisted": true|false, (boolean) Whether the peer is whitelisted
    //     "minfeefilter": n,         (numeric) The minimum fee rate for transactions this peer accepts
    //     "bytessent_per_msg": {
    //        "addr": n,              (numeric) The total bytes sent aggregated by message type
    //        ...
    //     },
    //     "bytesrecv_per_msg": {
    //        "addr": n,              (numeric) The total bytes received aggregated by message type
    //        ...
    //     }
}

export interface ListUnspentQueryOptions {
    minimumAmount?: number|string;   // "minimumAmount"    (numeric or string, default=0) Minimum value of each UTXO in BCH
    maximumAmount?: number|string;   // "maximumAmount"    (numeric or string, default=unlimited) Maximum value of each UTXO in BCH
    maximumCount?: number|string;    // "maximumCount"     (numeric or string, default=unlimited) Maximum number of UTXOs
    minimumSumAmount: number|string; // "minimumSumAmount" (numeric or string, default=unlimited) Minimum sum value of all UTXOs in BCH
}

export interface RpcListUnspentRes {
    txid: string;           // "txid" : "txid",          (string) the transaction id
    vout: number;           // "vout" : n,               (numeric) the vout value
    address: string;        // "address" : "address",    (string) the bitcoin address
    label: string;          // "label" : "label",        (string) The associated label, or "" for the default label
    scriptPubKey: string;   // "scriptPubKey" : "key",   (string) the script key
    amount: number;         // "amount" : x.xxx,         (numeric) the transaction output amount in BCH
    confirmations: number;  // "confirmations" : n,      (numeric) The number of confirmations
    redeemScript: number;   // "redeemScript" : n        (string) The redeemScript if scriptPubKey is P2SH
    spendable:boolean;      // "spendable" : xxx,        (bool) Whether we have the private keys to spend this output
    solvable: boolean;      // "solvable" : xxx,         (bool) Whether we know how to spend this output, ignoring the lack of keys
    safe:boolean;           // "safe" : xxx              (bool) Whether this output is considered safe to spend. Unconfirmed transactions from outside keys are considered unsafe and are not eligible for spending by fundrawtransaction and sendtoaddress.
}
