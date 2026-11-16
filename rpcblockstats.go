// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"cmp"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// blockStatsUTXOOverhead is what getblockstats adds to the size of each output
// for utxo_size_inc, as Bitcoin Cash Node does: the size of an outpoint, a
// height and a coinbase flag.
const blockStatsUTXOOverhead = chainhash.HashSize + 4 + 4 + 1

// blockStatsSpentStats are the getblockstats statistics that need the outputs
// a block spends, which are read from its spend journal, as Bitcoin Cash Node
// reads them from its undo data only for these.
var blockStatsSpentStats = []string{"avgfee", "avgfeerate",
	"feerate_percentiles", "maxfee", "maxfeerate", "medianfee", "minfee",
	"minfeerate", "totalfee", "utxo_size_inc"}

// blockStatsFeeRate is the fee rate of a transaction, in satoshis per byte,
// with its size.
type blockStatsFeeRate struct {
	rate int64
	size int64
}

// blockStatsBlock returns the block of the main chain that a getblockstats
// hash or height names, with its height set, refusing what Bitcoin Cash Node
// refuses with its errors.
func blockStatsBlock(s *rpcServer, hashOrHeight btcjson.HashOrHeight) (*bchutil.Block, error) {
	chain := s.cfg.Chain
	var hash *chainhash.Hash
	switch value := hashOrHeight.Value.(type) {
	case int:
		tip := chain.BestSnapshot().Height
		if value < 0 {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("Target block height %d is "+
					"negative", value),
			}
		}
		if value > int(tip) {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("Target block height %d after "+
					"current tip %d", value, tip),
			}
		}
		var err error
		hash, err = chain.BlockHashByHeight(int32(value))
		if err != nil {
			context := "Failed to obtain block hash"
			return nil, internalRPCError(err.Error(), context)
		}

	case string:
		if len(value) != 2*chainhash.HashSize {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("hash_or_height must be of "+
					"length %d (not %d, for '%s')",
					2*chainhash.HashSize, len(value), value),
			}
		}
		if _, err := hex.DecodeString(value); err != nil {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("hash_or_height must be "+
					"hexadecimal string (not '%s')", value),
			}
		}
		var err error
		hash, err = chainhash.NewHashFromStr(value)
		if err != nil {
			return nil, rpcDecodeHexError(value)
		}

	default:
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: "hash_or_height must be a block hash or height",
		}
	}

	block, err := chain.BlockByHash(hash)
	if err == nil {
		return block, nil
	}
	switch {
	case chain.MainChainHasBlock(hash):
		// The block is in the main chain but its data isn't stored,
		// which can only be because it was pruned or skipped by a fast
		// sync.
		if chain.IsPruned() {
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCMisc,
				Message: "Block not available (pruned data)",
			}
		}
		context := "Failed to load block"
		return nil, internalRPCError(err.Error(), context)

	case chain.HaveHeader(hash):
		return nil, &btcjson.RPCError{
			Code: btcjson.ErrRPCInvalidParameter,
			Message: fmt.Sprintf("Block is not in chain %s",
				s.cfg.ChainParams.Name),
		}

	default:
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}
}

// spentOutputSize returns the serialized size of the output of a spend
// journal entry, whose script starts with the output's token data, if any, as
// it does on the wire.
func spentOutputSize(stxo *blockchain.SpentTxOut) int {
	return 8 + wire.VarIntSerializeSize(uint64(len(stxo.PkScript))) +
		len(stxo.PkScript)
}

// truncatedMedian returns the median of the values, the lower of the two
// middle values when their mean isn't whole, or 0 without values, as Bitcoin
// Cash Node's CalculateTruncatedMedian does.  It sorts the values.
func truncatedMedian(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	slices.Sort(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2
	}
	return values[middle]
}

// feeRatePercentiles returns the fee rates at the 10th, 25th, 50th, 75th and
// 90th percentile of the total size of the transactions, as Bitcoin Cash
// Node's CalculatePercentilesBySize does.  It sorts the fee rates.
func feeRatePercentiles(feeRates []blockStatsFeeRate, totalSize int64) [5]int64 {
	var result [5]int64
	if len(feeRates) == 0 {
		return result
	}
	slices.SortFunc(feeRates, func(a, b blockStatsFeeRate) int {
		return cmp.Or(cmp.Compare(a.rate, b.rate),
			cmp.Compare(a.size, b.size))
	})

	size := float64(totalSize)
	weights := [5]float64{size / 10.0, size / 4.0, size / 2.0,
		size * 3.0 / 4.0, size * 9.0 / 10.0}
	var next int
	var cumulative int64
	for _, feeRate := range feeRates {
		cumulative += feeRate.size
		for next < len(weights) && float64(cumulative) >= weights[next] {
			result[next] = feeRate.rate
			next++
		}
	}

	// Fill any remaining percentiles with the last value.
	for ; next < len(result); next++ {
		result[next] = feeRates[len(feeRates)-1].rate
	}
	return result
}

// handleGetBlockStats implements the getblockstats command, as Bitcoin Cash
// Node does, with the outputs a block spends from its spend journal.
func handleGetBlockStats(s *rpcServer, cmd interface{}, closeNotifier <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetBlockStatsCmd)
	block, err := blockStatsBlock(s, c.HashOrHeight)
	if err != nil {
		return nil, err
	}

	var stats []string
	if c.Stats != nil {
		stats = *c.Stats
	}
	needSpent := len(stats) == 0
	for _, stat := range stats {
		if slices.Contains(blockStatsSpentStats, stat) {
			needSpent = true
			break
		}
	}

	txns := block.Transactions()
	var stxos []blockchain.SpentTxOut
	if needSpent {
		var numInputs int
		for _, tx := range txns[1:] {
			numInputs += len(tx.MsgTx().TxIn)
		}
		stxos, err = s.cfg.Chain.FetchSpendJournal(block)
		if err != nil || len(stxos) != numInputs {
			rpcsLog.Debugf("Can't read the spend journal of block %v "+
				"(%d entries for %d inputs): %v", block.Hash(),
				len(stxos), numInputs, err)
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCMisc,
				Message: "Can't read undo data from disk",
			}
		}
	}

	var (
		ins, outs, utxoSizeInc              int64
		totalOut, totalFee, maxFee, maxRate int64
		minFee, minRate                     int64 = math.MaxInt64, math.MaxInt64
		totalSize, maxTxSize                int64
		minTxSize                           int64 = math.MaxInt64
		fees, txSizes                       []int64
		feeRates                            []blockStatsFeeRate
		stxoIdx                             int
	)
	for i, tx := range txns {
		msgTx := tx.MsgTx()
		outs += int64(len(msgTx.TxOut))
		var txTotalOut int64
		for _, txOut := range msgTx.TxOut {
			txTotalOut += txOut.Value
			utxoSizeInc += int64(txOut.SerializeSize() +
				blockStatsUTXOOverhead)
		}

		// The coinbase counts only for its outputs: its input spends
		// nothing, and its outputs, the block's reward, are left out of
		// total_out.
		if i == 0 {
			continue
		}
		ins += int64(len(msgTx.TxIn))
		totalOut += txTotalOut

		txSize := int64(msgTx.SerializeSize())
		txSizes = append(txSizes, txSize)
		maxTxSize = max(maxTxSize, txSize)
		minTxSize = min(minTxSize, txSize)
		totalSize += txSize

		if !needSpent {
			continue
		}
		var txTotalIn int64
		for range msgTx.TxIn {
			stxo := &stxos[stxoIdx]
			stxoIdx++
			txTotalIn += stxo.Amount
			utxoSizeInc -= int64(spentOutputSize(stxo) +
				blockStatsUTXOOverhead)
		}

		// Fees and fee rates are whole satoshis, the fee rates rounded
		// down to satoshis per byte.
		fee := txTotalIn - txTotalOut
		fees = append(fees, fee)
		maxFee = max(maxFee, fee)
		minFee = min(minFee, fee)
		totalFee += fee

		feeRate := fee / txSize
		feeRates = append(feeRates, blockStatsFeeRate{feeRate, txSize})
		maxRate = max(maxRate, feeRate)
		minRate = min(minRate, feeRate)
	}

	// The statistics of a block with no transaction but its coinbase are
	// zero.
	numTxs := int64(len(txns) - 1)
	var avgFee, avgFeeRate, avgTxSize int64
	if numTxs > 0 {
		avgFee = totalFee / numTxs
		avgTxSize = totalSize / numTxs
		avgFeeRate = totalFee / totalSize
	} else {
		minFee, minRate, minTxSize = 0, 0, 0
	}
	percentiles := feeRatePercentiles(feeRates, totalSize)
	toBCH := func(satoshis int64) float64 {
		return bchutil.Amount(satoshis).ToBCH()
	}

	medianTime, err := s.cfg.Chain.MedianTimeByHash(block.Hash())
	if err != nil {
		context := "Failed to obtain median time"
		return nil, internalRPCError(err.Error(), context)
	}

	result := &btcjson.GetBlockStatsResult{
		AvgFee:     toBCH(avgFee),
		AvgFeeRate: toBCH(avgFeeRate),
		AvgTxSize:  avgTxSize,
		BlockHash:  block.Hash().String(),
		FeeRatePercentiles: []float64{toBCH(percentiles[0]),
			toBCH(percentiles[1]), toBCH(percentiles[2]),
			toBCH(percentiles[3]), toBCH(percentiles[4])},
		Height:       int64(block.Height()),
		Ins:          ins,
		MaxFee:       toBCH(maxFee),
		MaxFeeRate:   toBCH(maxRate),
		MaxTxSize:    maxTxSize,
		MedianFee:    toBCH(truncatedMedian(fees)),
		MedianTime:   medianTime.Unix(),
		MedianTxSize: truncatedMedian(txSizes),
		MinFee:       toBCH(minFee),
		MinFeeRate:   toBCH(minRate),
		MinTxSize:    minTxSize,
		Outs:         outs,
		Subsidy: toBCH(blockchain.CalcBlockSubsidy(block.Height(),
			s.cfg.ChainParams)),
		Time:             block.MsgBlock().Header.Timestamp.Unix(),
		TotalOut:         toBCH(totalOut),
		TotalSize:        totalSize,
		TotalFee:         toBCH(totalFee),
		Txs:              int64(len(txns)),
		UTXOIncrease:     outs - ins,
		UTXOSizeIncrease: utxoSizeInc,
	}
	if len(stats) == 0 {
		return result, nil
	}
	return selectBlockStats(result, stats)
}

// selectBlockStats returns an object of only the statistics of the result
// that stats names, refusing a name that isn't one, as Bitcoin Cash Node
// does.  As in Bitcoin Cash Node, it is sorted by name, and a statistic named
// twice is given once.
func selectBlockStats(result *btcjson.GetBlockStatsResult, stats []string) (map[string]json.RawMessage, error) {
	marshalled, err := json.Marshal(result)
	if err != nil {
		context := "Failed to marshal block stats"
		return nil, internalRPCError(err.Error(), context)
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(marshalled, &all); err != nil {
		context := "Failed to unmarshal block stats"
		return nil, internalRPCError(err.Error(), context)
	}

	sorted := slices.Clone(stats)
	slices.Sort(sorted)
	selected := make(map[string]json.RawMessage, len(sorted))
	for _, stat := range sorted {
		value, ok := all[stat]
		if !ok {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInvalidParameter,
				Message: fmt.Sprintf("Invalid selected statistic %s",
					stat),
			}
		}
		selected[stat] = value
	}
	return selected, nil
}
