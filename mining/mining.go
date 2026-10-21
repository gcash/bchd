// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"container/heap"
	"fmt"
	"math"
	"time"

	"sort"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

const (
	// blockHeaderOverhead is the max number of bytes it takes to serialize
	// a block header and max possible transaction count.
	blockHeaderOverhead = wire.MaxBlockHeaderPayload + wire.MaxVarIntPayload
)

var (
	// CoinbaseFlags is added to the coinbase script of a generated block
	// and is used to monitor BIP16 support as well as blocks that are
	// generated via bchd.
	CoinbaseFlags = "/bchd/"
)

// TxDesc is a descriptor about a transaction in a transaction source along with
// additional metadata.
type TxDesc struct {
	// Tx is the transaction associated with the entry.
	Tx *bchutil.Tx

	// Added is the time when the entry was added to the source pool.
	Added time.Time

	// Height is the block height when the entry was added to the source
	// pool.
	Height int32

	// Fee is the total fee the transaction associated with the entry pays.
	Fee int64

	// FeePerKB is the fee the transaction pays in Satoshi per 1000 bytes.
	FeePerKB int64
}

// TxSource represents a source of transactions to consider for inclusion in
// new blocks.
//
// The interface contract requires that all of these methods are safe for
// concurrent access with respect to the source.
type TxSource interface {
	// LastUpdated returns the last time a transaction was added to or
	// removed from the source pool.
	LastUpdated() time.Time

	// MiningDescs returns a slice of mining descriptors for all the
	// transactions in the source pool.
	MiningDescs() []*TxDesc

	// HaveTransaction returns whether or not the passed transaction hash
	// exists in the source pool.
	HaveTransaction(hash *chainhash.Hash) bool
}

// txPrioItem houses a transaction along with extra information that allows the
// transaction to be prioritized by fee and track dependencies on other
// transactions which have not been mined into a block yet.
type txPrioItem struct {
	tx       *bchutil.Tx
	fee      int64
	feePerKB int64

	// dependsOn holds a map of transaction hashes which this one depends
	// on.  It will only be set when the transaction references other
	// transactions in the source pool and hence must come after them in
	// a block.
	dependsOn map[chainhash.Hash]struct{}
}

// txPriorityQueue implements a priority queue of txPrioItem elements ordered
// by fee per kilobyte, highest first.
type txPriorityQueue struct {
	items []*txPrioItem
}

// Len returns the number of items in the priority queue.  It is part of the
// heap.Interface implementation.
func (pq *txPriorityQueue) Len() int {
	return len(pq.items)
}

// Less returns whether the item in the priority queue with index i should sort
// before the item with index j.  It is part of the heap.Interface
// implementation.
func (pq *txPriorityQueue) Less(i, j int) bool {
	// Using > here so that pop gives the highest fee item as opposed
	// to the lowest.
	return pq.items[i].feePerKB > pq.items[j].feePerKB
}

// Swap swaps the items at the passed indices in the priority queue.  It is
// part of the heap.Interface implementation.
func (pq *txPriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

// Push pushes the passed item onto the priority queue.  It is part of the
// heap.Interface implementation.
func (pq *txPriorityQueue) Push(x interface{}) {
	pq.items = append(pq.items, x.(*txPrioItem))
}

// Pop removes the highest fee item from the priority queue and returns it.
// It is part of the heap.Interface implementation.
func (pq *txPriorityQueue) Pop() interface{} {
	n := len(pq.items)
	item := pq.items[n-1]
	pq.items[n-1] = nil
	pq.items = pq.items[0 : n-1]
	return item
}

// minTxFee returns the fee a transaction of the given size must pay at the
// given fee rate per 1000 bytes, rounded down.  The mempool rounds down too,
// but asks for at least one satoshi, so every transaction it takes passes.
func minTxFee(size int64, feePerKB bchutil.Amount) int64 {
	if feePerKB > 0 && size > math.MaxInt64/int64(feePerKB) {
		return math.MaxInt64
	}
	return size * int64(feePerKB) / 1000
}

// newTxPriorityQueue returns a new transaction priority queue that reserves the
// passed amount of space for the elements.  The priority queue can grow larger
// than the reserved space, but extra copies of the underlying array can be
// avoided by reserving a sane value.
func newTxPriorityQueue(reserve int) *txPriorityQueue {
	return &txPriorityQueue{
		items: make([]*txPrioItem, 0, reserve),
	}
}

// BlockTemplate houses a block that has yet to be solved along with additional
// details about the fees and the number of signature operations for each
// transaction in the block.
type BlockTemplate struct {
	// Block is a block that is ready to be solved by miners.  Thus, it is
	// completely valid with the exception of satisfying the proof-of-work
	// requirement.
	Block *wire.MsgBlock

	// Fees contains the amount of fees each transaction in the generated
	// template pays in base units.  Since the first transaction is the
	// coinbase, the first entry (offset 0) will contain the negative of the
	// sum of the fees of all other transactions.
	Fees []int64

	// SigChecks is the total number of sigchecks for each transaction in
	// the generated template.
	SigChecks []int64

	// Height is the height at which the block template connects to the main
	// chain.
	Height int32

	// ValidPayAddress indicates whether or not the template coinbase pays
	// to an address or is redeemable by anyone.  See the documentation on
	// NewBlockTemplate for details on which this can be useful to generate
	// templates without a coinbase payment address.
	ValidPayAddress bool

	// MaxBlockSize is the maximum size of the block allowed by the
	// consensus rules.
	MaxBlockSize uint64

	// MaxSigChecks is the total sigchecks allowed in the block given the
	// consensus rules.
	MaxSigChecks uint32
}

// mergeUtxoView adds all of the entries in viewB to viewA.  The result is that
// viewA will contain all of its original entries plus all of the entries
// in viewB.  It will replace any entries in viewB which also exist in viewA
// if the entry in viewA is spent.
func mergeUtxoView(viewA *blockchain.UtxoViewpoint, viewB *blockchain.UtxoViewpoint) {
	viewAEntries := viewA.Entries()
	for outpoint, entryB := range viewB.Entries() {
		if entryA, exists := viewAEntries[outpoint]; !exists ||
			entryA == nil || entryA.IsSpent() {

			viewAEntries[outpoint] = entryB
		}
	}
}

// standardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks and adds
// the extra nonce as well as additional coinbase flags.
func standardCoinbaseScript(nextBlockHeight int32, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(int64(nextBlockHeight)).
		AddInt64(int64(extraNonce)).AddData([]byte(CoinbaseFlags)).
		Script()
}

// createCoinbaseTx returns a coinbase transaction paying an appropriate subsidy
// based on the passed block height to the provided address.  When the address
// is nil, the coinbase transaction will instead be redeemable by anyone.
//
// See the comment for NewBlockTemplate for more information about why the nil
// address handling is useful.
func createCoinbaseTx(params *chaincfg.Params, coinbaseScript []byte, nextBlockHeight int32, addr bchutil.Address) (*bchutil.Tx, error) {
	// Create the script to pay to the provided payment address if one was
	// specified.  Otherwise create a script that allows the coinbase to be
	// redeemable by anyone.
	var pkScript []byte
	if addr != nil {
		var err error
		pkScript, err = txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		scriptBuilder := txscript.NewScriptBuilder()
		pkScript, err = scriptBuilder.AddOp(txscript.OP_TRUE).Script()
		if err != nil {
			return nil, err
		}
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex),
		SignatureScript: coinbaseScript,
		Sequence:        wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    blockchain.CalcBlockSubsidy(nextBlockHeight, params),
		PkScript: pkScript,
	})
	padCoinbaseScript(tx, minCoinbaseSize(params, nextBlockHeight))

	return bchutil.NewTx(tx), nil
}

// minCoinbaseSize returns the minimum serialized transaction size the coinbase
// must reach for a block at the provided height.  Upgrade9 lowered the minimum,
// so until it activates the larger Magnetic Anomaly minimum still applies.
//
// This matters well beyond the historical fork window: simnet sets
// Upgrade9ForkHeight far in the future, so there the larger minimum applies to
// every block mined above the Magnetic Anomaly height.
func minCoinbaseSize(params *chaincfg.Params, nextBlockHeight int32) int {
	if nextBlockHeight > params.Upgrade9ForkHeight {
		return blockchain.MinTransactionSize
	}
	return blockchain.MagneticAnomalyMinTransactionSize
}

// padCoinbaseScript makes sure the coinbase script is above the minimum tx size
// threshold that applies to the block being built.
func padCoinbaseScript(tx *wire.MsgTx, minSize int) {
	if tx.SerializeSize() < minSize {
		tx.TxIn[0].SignatureScript = append(tx.TxIn[0].SignatureScript,
			make([]byte, minSize-tx.SerializeSize())...)
	}
}

// spendTransaction updates the passed view by marking the inputs to the passed
// transaction as spent.  It also adds all outputs in the passed transaction
// which are not provably unspendable as available unspent transaction outputs.
func spendTransaction(utxoView *blockchain.UtxoViewpoint, tx *bchutil.Tx, height int32) error {
	for _, txIn := range tx.MsgTx().TxIn {
		entry := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if entry != nil {
			entry.Spend()
		}
	}

	utxoView.AddTxOuts(tx, height)
	return nil
}

// logSkippedDeps logs any dependencies which are also skipped as a result of
// skipping a transaction while generating a block template at the trace level.
func logSkippedDeps(tx *bchutil.Tx, deps map[chainhash.Hash]*txPrioItem) {
	if deps == nil {
		return
	}

	for _, item := range deps {
		log.Tracef("Skipping tx %s since it depends on %s\n",
			item.tx.Hash(), tx.Hash())
	}
}

// MinimumMedianTime returns the minimum allowed timestamp for a block building
// on the end of the provided best chain.  In particular, it is one second after
// the median timestamp of the last several blocks per the chain consensus
// rules.
func MinimumMedianTime(chainState *blockchain.BestState) time.Time {
	return chainState.MedianTime.Add(time.Second)
}

// medianAdjustedTime returns the current time adjusted to ensure it is at least
// one second after the median timestamp of the last several blocks per the
// chain consensus rules.
func medianAdjustedTime(chainState *blockchain.BestState, timeSource blockchain.MedianTimeSource) time.Time {
	// The timestamp for the block must not be before the median timestamp
	// of the last several blocks.  Thus, choose the maximum between the
	// current time and one second after the past median time.  The current
	// timestamp is truncated to a second boundary before comparison since a
	// block timestamp does not supported a precision greater than one
	// second.
	newTimestamp := timeSource.AdjustedTime()
	minTimestamp := MinimumMedianTime(chainState)
	if newTimestamp.Before(minTimestamp) {
		newTimestamp = minTimestamp
	}

	return newTimestamp
}

// BlkTmplGenerator provides a type that can be used to generate block templates
// based on a given mining policy and source of transactions to choose from.
// It also houses additional state required in order to ensure the templates
// are built on top of the current best chain and adhere to the consensus rules.
type BlkTmplGenerator struct {
	policy      *Policy
	chainParams *chaincfg.Params
	txSource    TxSource
	chain       *blockchain.BlockChain
	timeSource  blockchain.MedianTimeSource
	sigCache    *txscript.SigCache
	hashCache   *txscript.HashCache
}

// NewBlkTmplGenerator returns a new block template generator for the given
// policy using transactions from the provided transaction source.
//
// The additional state-related fields are required in order to ensure the
// templates are built on top of the current best chain and adhere to the
// consensus rules.
func NewBlkTmplGenerator(policy *Policy, params *chaincfg.Params,
	txSource TxSource, chain *blockchain.BlockChain,
	timeSource blockchain.MedianTimeSource,
	sigCache *txscript.SigCache,
	hashCache *txscript.HashCache) *BlkTmplGenerator {

	return &BlkTmplGenerator{
		policy:      policy,
		chainParams: params,
		txSource:    txSource,
		chain:       chain,
		timeSource:  timeSource,
		sigCache:    sigCache,
		hashCache:   hashCache,
	}
}

// NewBlockTemplate returns a new block template that is ready to be solved
// using the transactions from the passed transaction source pool and a coinbase
// that either pays to the passed address if it is not nil, or a coinbase that
// is redeemable by anyone if the passed address is nil.  The nil address
// functionality is useful since there are cases such as the getblocktemplate
// RPC where external mining software is responsible for creating their own
// coinbase which will replace the one generated for the block template.  Thus
// the need to have configured address can be avoided.
//
// Transactions are selected by fee per kilobyte, highest first.  Transactions
// which only spend outputs from other transactions already in the block chain
// are immediately added to a priority queue.  Transactions which spend outputs
// from other transactions in the source pool are added to a dependency map so
// they can be added to the priority queue once the transactions they depend on
// have been included.
//
// Transactions paying less than the TxMinFreeFee policy setting per kilobyte
// are skipped.  Any transactions which would cause the block to exceed the
// BlockMaxSize policy setting or the block size limit less 1000 bytes, exceed
// the maximum allowed signature checks per block, or otherwise cause the block
// to be invalid are skipped.
func (g *BlkTmplGenerator) NewBlockTemplate(payToAddress bchutil.Address) (*BlockTemplate, error) {
	// Extend the most recently known best block.
	best := g.chain.BestSnapshot()
	nextBlockHeight := best.Height + 1

	ts := medianAdjustedTime(best, g.timeSource)

	// Use the limits block validation applies to a block on the tip.
	maxBlockSize, err := g.chain.NextBlockSizeLimit(&best.Hash)
	if err != nil {
		return nil, err
	}
	maxSigChecks := min(maxBlockSize/blockchain.BlockMaxBytesMaxSigChecksRatio,
		math.MaxUint32)

	// Select transactions up to the configured size, but no further than
	// the limit less room for the coinbase.
	selectMaxSize := maxBlockSize - 1000
	if g.policy.BlockMaxSize != 0 {
		selectMaxSize = min(selectMaxSize, uint64(g.policy.BlockMaxSize))
	}
	selectMaxSize = min(max(selectMaxSize, 1000), math.MaxUint32)

	// Create a standard coinbase transaction paying to the provided
	// address.  NOTE: The coinbase value will be updated to include the
	// fees from the selected transactions later after they have actually
	// been selected.  It is created here to detect any errors early
	// before potentially doing a lot of work below.  The extra nonce helps
	// ensure the transaction is not a duplicate transaction (paying the
	// same value to the same public key address would otherwise be an
	// identical transaction for block version 1).
	extraNonce := uint64(0)
	coinbaseScript, err := standardCoinbaseScript(nextBlockHeight, extraNonce)
	if err != nil {
		return nil, err
	}
	coinbaseTx, err := createCoinbaseTx(g.chainParams, coinbaseScript,
		nextBlockHeight, payToAddress)
	if err != nil {
		return nil, err
	}

	// Get the current source transactions and create a priority queue to
	// hold the transactions which are ready for inclusion into a block
	// along with their fee metadata.  Reserve the same number of items that
	// are available for the priority queue.
	sourceTxns := g.txSource.MiningDescs()
	priorityQueue := newTxPriorityQueue(len(sourceTxns))

	// Create a slice to hold the transactions to be included in the
	// generated block with reserved space.  Also create a utxo view to
	// house all of the input transactions so multiple lookups can be
	// avoided.
	blockTxns := make([]*bchutil.Tx, 0, len(sourceTxns))
	blockUtxos := blockchain.NewUtxoViewpoint()

	// dependers is used to track transactions which depend on another
	// transaction in the source pool.  This, in conjunction with the
	// dependsOn map kept with each dependent transaction helps quickly
	// determine which dependent transactions are now eligible for inclusion
	// in the block once each transaction has been included.
	dependers := make(map[chainhash.Hash]map[chainhash.Hash]*txPrioItem)

	// Create slices to hold the fees and number of signature operations
	// for each of the selected transactions and add an entry for the
	// coinbase.  This allows the code below to simply append details about
	// a transaction as it is selected for inclusion in the final block.
	// However, since the total fees aren't known yet, use a dummy value for
	// the coinbase fee which will be updated later.
	txFees := make([]int64, 0, len(sourceTxns))
	txSigChecks := make([]int64, 0, len(sourceTxns))
	txFees = append(txFees, -1)          // Updated once known
	txSigChecks = append(txSigChecks, 0) // Coinbase has zero sigchecks

	log.Debugf("Considering %d transactions for inclusion to new block",
		len(sourceTxns))

mempoolLoop:
	for _, txDesc := range sourceTxns {
		// A block can't have more than one coinbase or contain
		// non-finalized transactions.
		tx := txDesc.Tx
		if blockchain.IsCoinBase(tx) {
			log.Tracef("Skipping coinbase tx %s", tx.Hash())
			continue
		}
		if !blockchain.IsFinalizedTransaction(tx, nextBlockHeight,
			best.MedianTime) {

			log.Tracef("Skipping non-finalized tx %s", tx.Hash())
			continue
		}

		// Fetch all of the utxos referenced by the this transaction.
		// NOTE: This intentionally does not fetch inputs from the
		// mempool since a transaction which depends on other
		// transactions in the mempool must come after those
		// dependencies in the final generated block.
		utxos, err := g.chain.FetchUtxoView(tx)
		if err != nil {
			log.Warnf("Unable to fetch utxo view for tx %s: %v",
				tx.Hash(), err)
			continue
		}

		// Setup dependencies for any transactions which reference
		// other transactions in the mempool so they can be properly
		// ordered below.
		prioItem := &txPrioItem{tx: tx}
		for _, txIn := range tx.MsgTx().TxIn {
			originHash := &txIn.PreviousOutPoint.Hash
			entry := utxos.LookupEntry(txIn.PreviousOutPoint)
			if entry == nil || entry.IsSpent() {
				if !g.txSource.HaveTransaction(originHash) {
					log.Tracef("Skipping tx %s because it "+
						"references unspent output %s "+
						"which is not available",
						tx.Hash(), txIn.PreviousOutPoint)
					continue mempoolLoop
				}

				// The transaction is referencing another
				// transaction in the source pool, so setup an
				// ordering dependency.
				deps, exists := dependers[*originHash]
				if !exists {
					deps = make(map[chainhash.Hash]*txPrioItem)
					dependers[*originHash] = deps
				}
				deps[*prioItem.tx.Hash()] = prioItem
				if prioItem.dependsOn == nil {
					prioItem.dependsOn = make(
						map[chainhash.Hash]struct{})
				}
				prioItem.dependsOn[*originHash] = struct{}{}

				// Skip the check below. We already know the
				// referenced transaction is available.
				continue
			}
		}

		// Calculate the fee in Satoshi/kB.
		prioItem.feePerKB = txDesc.FeePerKB
		prioItem.fee = txDesc.Fee

		// Add the transaction to the priority queue to mark it ready
		// for inclusion in the block unless it has dependencies.
		if prioItem.dependsOn == nil {
			heap.Push(priorityQueue, prioItem)
		}

		// Merge the referenced outputs from the input transactions to
		// this transaction into the block utxo view.  This allows the
		// code below to avoid a second lookup.
		mergeUtxoView(blockUtxos, utxos)
	}

	log.Tracef("Priority queue len %d, dependers len %d",
		priorityQueue.Len(), len(dependers))

	// The starting block size is the size of the block header plus the max
	// possible transaction count size, plus the size of the coinbase
	// transaction, with at least 1000 bytes kept for both so a miner can
	// extend the coinbase.
	blockSize := uint32(max(blockHeaderOverhead+coinbaseTx.MsgTx().SerializeSize(),
		1000))
	blockSigChecks := int64(0)
	totalFees := int64(0)

	// Consensus script-verification flags for the block being built. Mirror the
	// mempool (mempool.AcceptTransaction): the bare StandardVerifyFlags omits
	// the post-Upgrade9 CashTokens flag (and the May2025/May2026 flags), so
	// token transactions — valid and already accepted into the mempool — would
	// fail ValidateTransactionScripts below and be dropped from the template,
	// yielding empty blocks on token-active networks (e.g. chipnet).
	scriptFlags := txscript.StandardVerifyFlags
	if nextBlockHeight > g.chainParams.Upgrade9ForkHeight {
		scriptFlags |= txscript.ScriptAllowCashTokens
	}
	if best.MedianTime.Unix() >= int64(g.chainParams.Upgrade11ActivationTime) {
		scriptFlags |= txscript.ScriptAllowMay2025
	}
	if best.MedianTime.Unix() >= int64(g.chainParams.Upgrade12ActivationTime) {
		scriptFlags |= txscript.ScriptAllowMay2026
	}

	// Choose which transactions make it into the block.
	for priorityQueue.Len() > 0 {
		// Grab the highest fee per kilobyte transaction.
		prioItem := heap.Pop(priorityQueue).(*txPrioItem)
		tx := prioItem.tx

		// Grab any transactions which depend on this one.
		deps := dependers[*tx.Hash()]

		// Enforce maximum block size.  Also check for overflow.
		txSize := uint32(tx.MsgTx().SerializeSize())
		blockPlusTxSize := blockSize + txSize
		if blockPlusTxSize < txSize ||
			uint64(blockPlusTxSize) >= selectMaxSize {

			log.Tracef("Skipping tx %s because it would exceed "+
				"the max block size", tx.Hash())
			logSkippedDeps(tx, deps)
			continue
		}

		// Skip transactions paying less than the minimum fee rate.
		if prioItem.fee < minTxFee(int64(txSize), g.policy.TxMinFreeFee) {
			log.Tracef("Skipping tx %s with feePerKB %d "+
				"< TxMinFreeFee %d", tx.Hash(), prioItem.feePerKB,
				g.policy.TxMinFreeFee)
			logSkippedDeps(tx, deps)
			continue
		}

		// Ensure the transaction inputs pass all of the necessary
		// preconditions before allowing it to be added to the block.
		_, err = blockchain.CheckTransactionInputs(tx, nextBlockHeight,
			blockUtxos, g.chainParams)
		if err != nil {
			log.Tracef("Skipping tx %s due to error in "+
				"CheckTransactionInputs: %v", tx.Hash(), err)
			logSkippedDeps(tx, deps)
			continue
		}
		seqLock, err := g.chain.NextBlockSequenceLock(&best.Hash, tx,
			blockUtxos)
		if err != nil || !blockchain.SequenceLockActive(seqLock,
			nextBlockHeight, best.MedianTime) {

			log.Tracef("Skipping tx %s because its sequence locks "+
				"are not met: %v", tx.Hash(), err)
			logSkippedDeps(tx, deps)
			continue
		}
		sigchecks, err := blockchain.ValidateTransactionScripts(tx, blockUtxos,
			scriptFlags, g.sigCache,
			g.hashCache, g.chainParams.Upgrade9ForkHeight)
		if err != nil {
			log.Tracef("Skipping tx %s due to error in "+
				"ValidateTransactionScripts: %v", tx.Hash(), err)
			logSkippedDeps(tx, deps)
			continue
		}

		if blockSigChecks+int64(sigchecks) < blockSigChecks ||
			blockSigChecks+int64(sigchecks) > int64(maxSigChecks) {
			log.Tracef("Skipping tx %s because it would "+
				"exceed the maximum sigchecks per block", tx.Hash())
			logSkippedDeps(tx, deps)
			continue
		}

		// Spend the transaction inputs in the block utxo view and add
		// an entry for it to ensure any transactions which reference
		// this one have it available as an input and can ensure they
		// aren't double spending.
		_ = spendTransaction(blockUtxos, tx, nextBlockHeight)

		// Add the transaction to the block, increment counters, and
		// save the fees and signature operation counts to the block
		// template.
		blockTxns = append(blockTxns, tx)
		blockSize = blockPlusTxSize
		blockSigChecks += int64(sigchecks)
		totalFees += prioItem.fee
		txFees = append(txFees, prioItem.fee)
		txSigChecks = append(txSigChecks, int64(sigchecks))

		log.Tracef("Adding tx %s (feePerKB %d)", prioItem.tx.Hash(),
			prioItem.feePerKB)

		// Add transactions which depend on this one (and also do not
		// have any other unsatisified dependencies) to the priority
		// queue.
		for _, item := range deps {
			// Add the transaction to the priority queue if there
			// are no more dependencies after this one.
			delete(item.dependsOn, *tx.Hash())
			if len(item.dependsOn) == 0 {
				heap.Push(priorityQueue, item)
			}
		}
	}

	// Now that the actual transactions have been selected, update the
	// block size for the real transaction count and coinbase value with
	// the total fees accordingly.
	blockSize -= wire.MaxVarIntPayload - uint32(wire.VarIntSerializeSize(uint64(len(blockTxns))))
	coinbaseTx.MsgTx().TxOut[0].Value += totalFees
	txFees[0] = -totalFees

	// Calculate the required difficulty for the block.  The timestamp
	// is potentially adjusted to ensure it comes after the median time of
	// the last several blocks per the chain consensus rules.
	reqDifficulty, err := g.chain.CalcNextRequiredDifficulty(ts)
	if err != nil {
		return nil, err
	}

	// Calculate the next expected block version based on the state of the
	// rule change deployments.
	nextBlockVersion, err := g.chain.CalcNextBlockVersion()
	if err != nil {
		return nil, err
	}

	// If MagneticAnomaly is enabled we need to sort transactions by txid to
	// comply with the CTOR consensus rule.  Each transaction keeps its fee
	// and signature check count, which follow the coinbase's entries.
	if nextBlockHeight > g.chainParams.MagneticAnonomalyForkHeight {
		sort.Sort(templateTxSorter{blockTxns, txFees[1:], txSigChecks[1:]})
	}
	blockTxns = append([]*bchutil.Tx{coinbaseTx}, blockTxns...)

	// Create a new block ready to be solved.
	merkles := blockchain.BuildMerkleTreeStore(blockTxns)
	var msgBlock wire.MsgBlock
	msgBlock.Header = wire.BlockHeader{
		Version:    nextBlockVersion,
		PrevBlock:  best.Hash,
		MerkleRoot: *merkles[len(merkles)-1],
		Timestamp:  ts,
		Bits:       reqDifficulty,
	}
	for _, tx := range blockTxns {
		if err := msgBlock.AddTransaction(tx.MsgTx()); err != nil {
			return nil, err
		}
	}

	// Finally, perform a full check on the created block against the chain
	// consensus rules to ensure it properly connects to the current best
	// chain with no issues.
	block := bchutil.NewBlock(&msgBlock)
	block.SetHeight(nextBlockHeight)
	if err := g.chain.CheckConnectBlockTemplate(block); err != nil {
		return nil, err
	}

	log.Debugf("Created new block template (%d transactions, %d in "+
		"fees, %d signature checks, %d size, target difficulty "+
		"%064x)", len(msgBlock.Transactions), totalFees, blockSigChecks,
		blockSize, blockchain.CompactToBig(msgBlock.Header.Bits))

	return &BlockTemplate{
		Block:           &msgBlock,
		Fees:            txFees,
		SigChecks:       txSigChecks,
		MaxSigChecks:    uint32(maxSigChecks),
		Height:          nextBlockHeight,
		ValidPayAddress: payToAddress != nil,
		MaxBlockSize:    maxBlockSize,
	}, nil
}

// UpdateBlockTime updates the timestamp in the header of the passed block to
// the current time while taking into account the median time of the last
// several blocks to ensure the new time is after that time per the chain
// consensus rules.  Finally, it will update the target difficulty if needed
// based on the new time for the test networks since their target difficulty can
// change based upon time.
func (g *BlkTmplGenerator) UpdateBlockTime(msgBlock *wire.MsgBlock) error {
	// The new timestamp is potentially adjusted to ensure it comes after
	// the median time of the last several blocks per the chain consensus
	// rules.
	newTime := medianAdjustedTime(g.chain.BestSnapshot(), g.timeSource)
	msgBlock.Header.Timestamp = newTime

	// Recalculate the difficulty if running on a network that requires it.
	if g.chainParams.ReduceMinDifficulty {
		difficulty, err := g.chain.CalcNextRequiredDifficulty(newTime)
		if err != nil {
			return err
		}
		msgBlock.Header.Bits = difficulty
	}

	return nil
}

// UpdateExtraNonce updates the extra nonce in the coinbase script of the passed
// block by regenerating the coinbase script with the passed value and block
// height.  It also recalculates and updates the new merkle root that results
// from changing the coinbase script.
func (g *BlkTmplGenerator) UpdateExtraNonce(msgBlock *wire.MsgBlock, blockHeight int32, extraNonce uint64) error {
	coinbaseScript, err := standardCoinbaseScript(blockHeight, extraNonce)
	if err != nil {
		return err
	}
	if len(coinbaseScript) > blockchain.MaxCoinbaseScriptLen {
		return fmt.Errorf("coinbase transaction script length "+
			"of %d is out of range (min: %d, max: %d)",
			len(coinbaseScript), blockchain.MinCoinbaseScriptLen,
			blockchain.MaxCoinbaseScriptLen)
	}
	msgBlock.Transactions[0].TxIn[0].SignatureScript = coinbaseScript

	// Make sure the coinbase is above the minimum size threshold.
	padCoinbaseScript(msgBlock.Transactions[0],
		minCoinbaseSize(g.chainParams, blockHeight))

	// TODO(davec): A bchutil.Block should use saved in the state to avoid
	// recalculating all of the other transaction hashes.
	// block.Transactions[0].InvalidateCache()

	// Recalculate the merkle root with the updated extra nonce.
	block := bchutil.NewBlock(msgBlock)
	merkles := blockchain.BuildMerkleTreeStore(block.Transactions())
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]
	return nil
}

// BestSnapshot returns information about the current best chain block and
// related state as of the current point in time using the chain instance
// associated with the block template generator.  The returned state must be
// treated as immutable since it is shared by all callers.
//
// This function is safe for concurrent access.
func (g *BlkTmplGenerator) BestSnapshot() *blockchain.BestState {
	return g.chain.BestSnapshot()
}

// TxSource returns the associated transaction source.
//
// This function is safe for concurrent access.
func (g *BlkTmplGenerator) TxSource() TxSource {
	return g.txSource
}
