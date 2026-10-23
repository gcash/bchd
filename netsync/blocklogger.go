// Copyright (c) 2015-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package netsync

import (
	"fmt"
	"sync"
	"time"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// blockProgressLogger provides periodic logging for other services in order
// to show users progress of certain "actions" involving some or all current
// blocks. Ex: syncing to best chain, indexing all blocks, etc.
type blockProgressLogger struct {
	receivedLogBlocks int64
	receivedLogTx     int64
	lastBlockLogTime  time.Time

	// synced is whether the last message showed the node at the height it
	// syncs to, and it has stayed there since.
	synced bool

	subsystemLogger bchlog.Logger
	progressAction  string
	sync.Mutex
}

// newBlockProgressLogger returns a new block progress logger.
// The progress message is templated as follows:
//
//	{progressAction} {numProcessed} {blocks|block} in the last {timePeriod}
//	({numTxs}, height {lastBlockHeight}, {lastBlockTimeStamp})
func newBlockProgressLogger(progressMessage string, logger bchlog.Logger) *blockProgressLogger {
	return &blockProgressLogger{
		lastBlockLogTime: time.Now(),
		progressAction:   progressMessage,
		subsystemLogger:  logger,
	}
}

// LogBlockHeight logs a new block height as an information message to show
// progress to the user. In order to prevent spam, it limits logging to one
// message every 10 seconds with duration and totals included.  The block that
// brings the node to the height it syncs to is logged at once, unless the last
// message showed the node there and it has stayed there since, so the log
// shows the end of a sync without waiting for the next block.
func (b *blockProgressLogger) LogBlockHeight(block *bchutil.Block, bestHeight uint64, chain *blockchain.BlockChain) {
	b.Lock()
	defer b.Unlock()

	b.receivedLogBlocks++
	b.receivedLogTx += int64(len(block.MsgBlock().Transactions))

	// Processing a block on the main chain also connects the orphans that
	// build on it.  At the end of a sync the sync peer's tip is one: the
	// peer announces it after each batch of blocks it sends, so it arrives
	// before its parents.  The progress is then that of the chain's tip.
	height := block.Height()
	timestamp := block.MsgBlock().Header.Timestamp
	if best := chain.BestSnapshot(); best.Height > height &&
		chain.MainChainHasBlock(block.Hash()) {

		header, err := chain.HeaderByHash(&best.Hash)
		if err == nil {
			height = best.Height
			timestamp = header.Timestamp
		}
	}

	reached := bestHeight > 0 && uint64(height) >= bestHeight
	if !reached {
		b.synced = false
	}

	now := time.Now()
	duration := now.Sub(b.lastBlockLogTime)
	if duration < time.Second*10 && (!reached || b.synced) {
		return
	}

	// Truncate the duration to 10s of milliseconds.
	durationMillis := int64(duration / time.Millisecond)
	tDuration := 10 * time.Millisecond * time.Duration(durationMillis/10)

	// Log information about new block height.
	blockStr := "blocks"
	if b.receivedLogBlocks == 1 {
		blockStr = "block"
	}
	txStr := "transactions"
	if b.receivedLogTx == 1 {
		txStr = "transaction"
	}

	progress := float64(0.0)

	if bestHeight > 0 {
		// Truncated to hundredths of a percent, so it is only 100% at the
		// height the node syncs to.
		progress = float64(min(uint64(height)*10000/bestHeight, 10000)) / 100
	}

	var heightStr string

	if bestHeight == 0 {
		// We don't have a best height due to no sync peer. Don't log the percentage.
		heightStr = fmt.Sprintf("%d", height)
	} else if reached {
		// sync is up to date so shorten the height output
		heightStr = fmt.Sprintf("%d (%.2f%%)", height, progress)
	} else {
		// sync is partial and in progress
		heightStr = fmt.Sprintf("%d/%d (%.2f%%)", height,
			bestHeight, progress)
	}

	cacheSizeStr := fmt.Sprintf("~%d MiB", chain.CachedStateSize()/1024/1024)
	b.subsystemLogger.Infof("%s %d %s in %s (%d %s, height %s, %s, %s cache)",
		b.progressAction, b.receivedLogBlocks, blockStr, tDuration, b.receivedLogTx,
		txStr, heightStr, timestamp, cacheSizeStr)

	b.synced = reached
	b.receivedLogBlocks = 0
	b.receivedLogTx = 0
	b.lastBlockLogTime = now
}

func (b *blockProgressLogger) SetLastLogTime(time time.Time) {
	b.lastBlockLogTime = time
}
