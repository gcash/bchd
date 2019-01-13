# Avalanche Pre-consensus Spec
The following is the specification for avalanche pre-consensus as implemented in this branch. 
It is not intended to be a final spec and is likely not compatible with the implementation being
developed by Bitcoin ABC. The primary purpose is to give other developers something tangible to look at, think about, and discuss.

### Service Bit
The following service bit is used to signal support for this specification. This service bit is in the "experimental" range and is
not intended to be the final service bit. 

```
NODE_AVALANCHE = (1 << 25)
```

If a node receives any avalanche message (from below) from a peer not signaling `NODE_AVALANCHE` they should disconnect the peer.

### New Network Messages

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
| 1+         | count            | var_int       | The number of votes included in this message        |
| ?          | votes            | []byte        | A byte array of votes. One byte per vote.           |
| 1+         | signature length | var_int       | The varint signature length                         |
| ?          | signature        | []byte        | A signature for this message                        |

The request ID value should be copied from the `avarequest` message this message is responding to. The responding node
must append a one byte vote to `votes` for each inv in the `avarequest`. The votes should be in the same order as the `invs`.

The following describes voting behavior:
 
- The node should vote yes (0x01) for a transaction if the transaction is valid and is currently marked by avalanche as `Accepted`.
- The node should vote no (0x00) for a transaction if the transaction is invalid or if it's currently marked by avalanche as `Rejected`.
- A node should vote neutral (0x80) for a transaction if it does not know about the transaction. The reason for this requirement is because
we don't want to block the response while the node attempts to download missing inventory.

// TODO: We might be able to reduce the size of the response further by sending only two bits per inv rather than eight.

The signature covers `cat(request ID, votes)`. A node must reject (and probably ban) a peer which returned an `avaresponse` with
a bad signature.

### Node Operation

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

### Recording Votes

Remote peers will respond the the `avarequest` message with an `avaresponse`. If the request ID in the response does not match any outstanding
requests the node must ignore the message and may increment the remote peer's ban score. 

For each `vote` in the `avaresponse` the node must adjust the confidence score for that item. In code the algorithm looks like this:

```go
func (vr *VoteRecord) regsiterVote(vote uint8) bool {
	vr.votes = (vr.votes << 1) | boolToUint8(vote == 1)
	vr.consider = (vr.consider << 1) | boolToUint8(int8(vote) >= 0)

	yes := countBits8(vr.votes&vr.consider&0xff) > 6

	// The round is inconclusive
	if !yes {
		no := countBits8((-vr.votes-1)&vr.consider&0xff) > 6
		if !no {
			return false
		}
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

- Transactions start out in one of two states. Either `Accepted` or `Rejected` based on the node's local mempool policy.
- We track the last eight votes for any given transaction.
- If at least seven of the last eight votes are a `yes` we consider this "round" to be a conclusive `yes`.
- If at least seven of the last eight votes are a `no` we consider this "round" to be a conclusive `no`.
- Neutral votes are not counted towards the conclusive yes or no.
- If the round was conclusive and agrees with our current state (either `Accepted` or `Rejected`) then we increment the confidence counter for this transaction.
- If the round was conclusive and disagrees with our current state then we flip our state to match the result of the round and reset our confidence counter to zero.
- If the confidence counter equals 128 we consider the transaction finalized. This means the transaction is either permanently accepted or permanently rejected.

Assuming there are not any `no` votes then we expect it to take 134 queries to finalize a transaction. Since the event loop fires every 10 milliseconds this means it will 
take a minimum of 1.34 seconds to send off enough requests to finalize the transaction. Add network latency on top of that and we expect most transactions
to finalize in 2 to 3 seconds.

If a transaction flips state from `Rejected` to `Accepted` the node must mark all double spends of that transaction as `Rejected` and
reset their confidence counters to zero.

When `Accepted` transactions are finalized they should be added into the mempool and double spends of the transaction
should be removed.

When `Rejected` transactions are finalized they should be removed from the mempool.

Because the event loop is firing off requests every 10ms it will send more requests than are required to finalized the transaction. If any responses arrive
after the transaction has been finalized they should be ignored and not affect the state of the transaction.
