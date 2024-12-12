// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"fmt"
	"math"
	"sync"
	"time"

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

	subsystemLogger bchlog.Logger
	progressAction  string
	sync.Mutex
}

// newBlockProgressLogger returns a new block progress logger.
// The progress message is templated as follows:
// {progressAction} {numProcessed} {blocks|block} in the last {timePeriod}
// ({numTxs}, height {lastBlockHeight}, {lastBlockTimeStamp})
func newBlockProgressLogger(progressMessage string, logger bchlog.Logger) *blockProgressLogger {
	return &blockProgressLogger{
		lastBlockLogTime: time.Now(),
		progressAction:   progressMessage,
		subsystemLogger:  logger,
	}
}

// LogBlockHeight logs a new block height as an information message to show
// progress to the user. In order to prevent spam, it limits logging to one
// message every 10 seconds with duration and totals included.
func (b *blockProgressLogger) LogBlockHeight(block *bchutil.Block, bestHeight uint64) {
	b.Lock()
	defer b.Unlock()

	b.receivedLogBlocks++
	b.receivedLogTx += int64(len(block.MsgBlock().Transactions))

	now := time.Now()
	duration := now.Sub(b.lastBlockLogTime)
	if duration < time.Second*10 {
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
		progress = math.Min(float64(block.Height())/float64(bestHeight), 1.0) * 100
	}

	var heightStr string

	if uint64(block.Height()) >= bestHeight {
		// sync is up to date so shorten the height output
		heightStr = fmt.Sprintf("%d (%.2f%%)", block.Height(), progress)
	} else {
		// sync is partial and in progress
		heightStr = fmt.Sprintf("%d/%d (%.2f%%)", block.Height(),
			bestHeight, progress)
	}

	b.subsystemLogger.Infof("%s %d %s in the last %s (%d %s, height %s, %s)",
		b.progressAction, b.receivedLogBlocks, blockStr, tDuration, b.receivedLogTx,
		txStr, heightStr, block.MsgBlock().Header.Timestamp)

	b.receivedLogBlocks = 0
	b.receivedLogTx = 0
	b.lastBlockLogTime = now
}
