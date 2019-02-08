## Avalanche Spec
The following is the specification for avalanche pre-consensus as implemented in this branch. 
It is not intended to be a final spec and is likely not compatible with the implementation being
developed by Bitcoin ABC. The primary purpose is to give other developers something tangible to look at, think about, and discuss.

#### Service Bit
The following service bit is used to signal support for this specification. This service bit is in the "experimental" range and is
not intended to be the final service bit. 

```
NODE_AVALANCHE = (1 << 25)
```

If a node receives any avalanche message (from below) from a peer not signaling `NODE_AVALANCHE` they should disconnect the peer.

#### New Network Messages

#### `avapubkey`

| Field Size | Description     | Data Type | Comments                                             |
|------------|------------------|-----------|------------------------------------------------------|
| 33         | pubkey           | [33]byte  | A secp256k1 compressed public key                               |
| 1+         | signature length | var_int   | The varint signature length                          |
| ?          | signature        | []byte    | A signature covering the remote peer's version nonce |

The `avapubkey` message must be sent to any remote peer signaling `NODE_AVALANCHE` after the receipt of the `version` message. The message consists of
a compressed secp256k1 public key and a signature covering the nonce found in the remote peer's version message. The Bitcoin protocol does not
require that the nonce values be unique per remote peer however this is recommend so that nodes can detect connections to self.
However, this specification does require nodes signaling `NODE_AVALANCHE` to use unique nonces so that the nonce can serve
as an appropriate challenge for the remote peer to prove they actually own the private key associated with the public key they send.

// TODO: is 64 bits of entropy enough here? Maybe we should sign `nonce + timestamp` instead.

If a node signals `NODE_AVALANCHE` but does not send the `avapubkey` message in a timely manner they should be disconnected. If they send more than
one `avapubkey` message per session they should be disconnected.

// TODO: do we see any need to change public keys while remaining connected? If this need arises would they do so with the node running or would it require
a restart (meaning there's no need to update the key for the current session.)

#### `avarequest`

| Field Size | Description | Data Type  | Comments                                                                    |
|------------|-------------|------------|-----------------------------------------------------------------------------|
| 8          | request ID  | uint64     | A random ID to identify this request. It will be sent back in the response. |
| 1+         | count       | var_int    | The number of inventory entries                                             |
| 36x?       | inventory   | []inv_vect | A list of inventory vectors to vote on                                      |

The `avarequest` message is sent to a remote peer to request their avalanche vote on unfinalized inventory. The request ID field is set by the sender to a random uint64 and will be included in the response so that the 
node can match responses to requests. The message contains a list of inventory vectors to be voted on. 

The transactions included in this message should fall into the following categories:

- Transactions which are valid and have been added to the mempool.
- Transactions which are valid but were rejected from the mempool due to a policy violation.
- Transactions which were rejected from the mempool due to being a double spend.

The following transactions should *not* be included.

- Transactions which are invalid (violate the consensus rules).
- Transactions which have finalized.
- Transactions which are double spends of transactions which have finalized.

// TODO: can we compress this message by sending less than 36 bytes per inventory item?

#### `avaresponse`

| Field Size | Description      | Data Type     | Comments                                            |
|------------|------------------|---------------|-----------------------------------------------------|
| 8          | request ID       | uint64        | The request ID taken from the avarequest message    |
| 1+         | count            | var_int       | The number of vote records included in this message |
| 33x?       | vote records     | []vote_record | A list of vote records                              |
| 1+         | signature length | var_int       | The varint signature length                         |
| ?          | signature        | []byte        | A signature for this message                        |

The `vote_record` data structure is as follows:

| Field Size | Description | Data Type | Comments                                                                 |
|------------|-------------|-----------|--------------------------------------------------------------------------|
| 1          | vote        | bool      | True represents a vote in the affirmative. False a vote in the negative. |
| 32         | hash        | [32]byte  | The hash if the item being voted on                                      |

The request ID value should be copied from the `avarequest` message this message is responding to. The responding node
should attach a `vote_record` object for each item they wish to vote for regardless of whether they are voting in the 
affirmative or negative. The following describes voting behavior:
 
- The node should vote yes for a transaction if the transaction is valid and is currently marked by avalanche as `accepted`.
- The node should vote no for a transaction if the transaction is invalid or if it's not currently marked by avalanche as `accepted`.
- A node should abstain from voting for a transaction if it does not know about the transaction. The reason for this requirement is because
we don't want to block the response while the node attempts to download missing inventory. If we were to vote no for transactions we do not
know about then we run the risk of a transaction finalizing a no by avalanche due to propagation delays. Empirically this was happening
with our implementation which is why it was switching to having the ability to abstain.

// TODO: obviously we can reduce the size of the response here by not sending back the full txid. Ideally we would just send
back a bitfield with each bit representing a yes or no vote for each inv, however doing it this way does not allow us
to abstain. Sending two bits per inv would probably work. 

The signature covers `request ID + vote_records[]` where the vote records are just serialized one after another.

#### Node Operation

The node operation centers around the avalanche manager ― a single threaded process which manages the asynchronous request/response IO.

The manager tracks all the connected peers which are signaling `NODE_AVALANCHE`.   

The core of it's operation is the event loop which behaves as follows:
```go
ticker := time.NewTicker(time.Millisecond * 10)
for ticker.C {
	invs := getInvsForNextQuery()
	peer := getPeerToQuery()
	
	id := newRequestID()
	req := wire.NewMsgAvaRequest(id)
	req.AddInvVects(invs)
	
	outstandingRequests[id] = req
	
	peer.QueueMessage(req)
}
```

Every 10 milliseconds the event loop runs. On each iteration it grabs all the outstanding inventory which has not yet been finalized, picks a single peer 
from among our connected avalanche peers and then sends that peer an `avarequest` message. It does *not* wait for the response to come back before continuing to the 
next iteration of the loop. 

#### `getInvsForNextQuery()`
This function should return invs that meet the requirements specified in the `avarequest` message section above. These invs may include
transactions not in the node's mempool. The node must limit the outstanding requests per inv to `10` so this function should not
return invs for which there are 10 or more outstanding requests.

#### `getPeerToQuery()`
At present this function just selects a random peer from among the connected avalanche peers. However, this is obviously not sybil resistant. Future iterations of this
spec will require this function to select from a list of connected avalanche peers weighted by some anti-sybil metric. For example, peers which have mined previous blocks. 

#### Recording Votes

Remote peers will respond the the `avarequest` message with an `avaresponse`. If the request ID in the response does not match any outstanding
requests the node must ignore the message and may increment the remote peer's ban score. 

For each `vote_record` in the `avaresponse` the node must adjust the confidence score for that item. In code the algorithm looks like this:

```go
func (vr *VoteRecord) regsiterVote(vote bool) bool {
	vr.votes = (vr.votes << 1) | boolToUint8(vote)
	vr.consider = (vr.consider << 1) | boolToUint8(vote)

	yes := countBits8(vr.votes&vr.consider&0xff) > 6

	// The round is inconclusive
	if !yes && countBits8((-vr.votes-1)&vr.consider&0xff) <= 6 {
		return false
	}

	// Vote is conclusive and agrees with our current state
	if ((vr.confidence & 0x01) == 1) == yes {
		vr.confidence += 2
		return vr.confidence >> 1 == 128
	}

	// Vote is conclusive but does not agree with our current state
	vr.confidence = boolToUint16(yes)

	return true
}
```

In English:

- We track the last eight votes for any given transaction.
- If at least seven of the last eight votes are a `yes` we consider this "round" to be a conclusive `yes`11.
- If 