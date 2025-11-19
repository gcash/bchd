## Wallet Operation
The following describes the operation of a hypothetical wallet built using the gRPC API.

#### Server Trusting Wallet
*Startup*
- Make a `GetBlockchainInfo` call to get the current best hash and height.
- For each address in the wallet's key chain make a `GetAddressTransactions` call. If there are any hits, extend the keychain, make another `GetAddressTransactions` call for the new address. Repeat until there are no more hits. 
- Calculate the wallet's UTXOs by iterating over all the returned transactions. For each output which matches an address, build the UTXO and add it to a UTXOs list. Next iterate over the transaction again. This time for each input spending a matched output, delete the UTXO from the UTXO list. The remaining UTXOs will be the full list of the wallet's UTXOs. There is a `GetAddressUnspentOutputs` RPC call but this is much less efficient and slower than just calculating the UTXOs locally. 

*Operation*
- Subscribe to the `SubscribeTransactionStream` RPC. This endpoint pushes a transaction whenever a new unconfirmed transaction comes in or an unconfirmed transaction is confirmed. Update the wallet's transactions accordingly.
- Subscribe to the `SubscribeBlocks` RPC. When a new message is received, for each disconnected block iterate over the wallet's transaction, if any transactions were mined in this block, set the confirmation count to zero. If these transaction re-confirm they will be sent in a `TransactionNotification`. For each connected block, update the best hash and height and update the confirmation count for each transaction. Persist the hash and height to disk.

*Next startup*
- Make a `GetBlockInfo` for the last saved block hash. Make sure the block is still in the best chain (confirmations > 0). If so make the `GetAddressTransactions` like before but using the last saved block hash as the start block. If it's not in the best chain, re-download all transactions from the genesis as there was a reorg. 

### SPV Wallet
Similar operation as above with the following exceptions:

*Startup*
- Make consecutive `GetHeaders` calls, constructing the block locator appropriately, to sync the header chain to the tip. Validate the header chain.
- For each confirmed transaction make a  `GetMerkleProof` to download the merkle proof. Validate the proof. 

*Operation*
- Upon receiving a `BlockNotification` disconnect or connect the headers to your chain as appropriate. 
- Upon receiving a `TransactionNotification` for a confirmed transaction request the merkle proof using the `GetMerkleProof` RPC.

*Next startup*
- Sync the headers to the tip using `GetHeaders`. If there was a reorg while you were away, find the reorg block and set the confirmations of any transaction that confirmed after the reorg point to zero. Make your `GetAddressTransactions` call starting from the reorg block. 