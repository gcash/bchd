package main

import (
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"io"
	"sync"
	"time"
)

// calcResults houses the stats and result of the utxo calculation operation.
type results struct {
	utxoHash *chainhash.Hash
	err      error
}

type utxoProcessor struct {
	db                database.DB
	chain             *blockchain.BlockChain
	r                 io.ReadSeeker
	processQueue      chan []byte
	doneChan          chan bool
	errChan           chan error
	quit              chan struct{}
	wg                sync.WaitGroup
	blocksProcessed   int64
	receivedLogBlocks int64
	receivedLogTx     int64
	lastHeight        int64
	lastBlockTime     time.Time
	lastLogTime       time.Time
	utxoHash          *chainhash.Hash
}

func (up *utxoProcessor) statusHandler(resultsChan chan *results) {
	select {
	// An error from either of the goroutines means we're done so signal
	// caller with the error and signal all goroutines to quit.
	case err := <-up.errChan:
		resultsChan <- &results{
			err:  err,
		}
		close(up.quit)

		// The import finished normally.
	case <-up.doneChan:
		resultsChan <- &results{
			utxoHash: up.utxoHash,
			err:             nil,
		}
	}
}

// on which the results will be returned when the operation has completed.
func (up *utxoProcessor) CalcUtxoSet(height int) chan *results {
	up.wg.Add(2)
	go up.readHandler()
	go up.processHandler()

	// Start the status handler and return the result channel that it will
	// send the results on when the import is done.
	resultChan := make(chan *results)
	go up.statusHandler(resultChan)
	return resultChan
}

// newUtxoProcessor returns a new processor for the provided database.
func newUtxoProcessor(db database.DB) (*utxoProcessor, error) {

	chain, err := blockchain.New(&blockchain.Config{
		DB:           db,
		ChainParams:  activeNetParams,
		TimeSource:   blockchain.NewMedianTime(),
		// No nice way to get the main configuration here.
		// For now just accept up to the default.
		ExcessiveBlockSize: 32000000,
	})
	if err != nil {
		return nil, err
	}

	return &utxoProcessor{
		db:           db,
		processQueue: make(chan []byte, 2),
		doneChan:     make(chan bool),
		errChan:      make(chan error),
		quit:         make(chan struct{}),
		chain:        chain,
		lastLogTime:  time.Now(),
	}, nil
}
