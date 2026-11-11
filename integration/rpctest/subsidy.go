// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpctest

import (
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
)

// upgrade2027ArgTime returns the upgrade 2027 activation time that args pass
// to bchd, the last one when there are several.
func upgrade2027ArgTime(args []string) uint64 {
	activationTime := uint64(Upgrade2027Never)
	for i, arg := range args {
		var value string
		switch {
		case arg == upgrade2027Flag && i+1 < len(args):
			value = args[i+1]
		case strings.HasPrefix(arg, upgrade2027Flag+"="):
			value = strings.TrimPrefix(arg, upgrade2027Flag+"=")
		default:
			continue
		}
		if t, err := strconv.ParseUint(value, 10, 64); err == nil {
			activationTime = t
		}
	}
	return activationTime
}

// upgrade2027Block is the activation block of the node's best chain.
type upgrade2027Block struct {
	hash   chainhash.Hash
	height int32
}

// nextBlockSubsidy returns the subsidy of the block after the one with the
// passed hash and height on the node's best chain.
func (h *Harness) nextBlockSubsidy(prevHash *chainhash.Hash,
	prevHeight int32) (int64, error) {

	activation, err := h.upgrade2027Height(prevHash, prevHeight)
	if err != nil {
		return 0, err
	}
	interval := int64(600)
	if activation >= 0 {
		interval = 60
	}
	ticks := heightToTicks(activation, prevHeight+1)
	return blockchain.CalcBlockSubsidy(ticks, interval, h.ActiveNet), nil
}

// heightToTicks returns the ticks of the block at the passed height on a
// chain whose activation block is at activation, which is -1 when it has
// none.
func heightToTicks(activation, height int32) int64 {
	if activation < 0 || height <= activation {
		return int64(height) * 600
	}
	return int64(activation)*600 + int64(height-activation)*60
}

// ticksToHeight returns the height of the last block whose ticks are not
// above the passed ticks on a chain whose activation block is at
// activation, or -1 when there is none.
func ticksToHeight(activation int32, ticks int64) int32 {
	switch {
	case ticks < 0:
		return -1
	case activation < 0 || ticks <= int64(activation)*600:
		return int32(ticks / 600)
	default:
		return activation + int32((ticks-int64(activation)*600)/60)
	}
}

// lastMatureCoinbase returns the height of the last block whose coinbase
// outputs the wallet spends when the tip is at the passed height: the last
// block at least the coinbase maturity in ticks below the tip.  The node
// would take them one block earlier, but the wallet has always waited for
// the tip itself to reach the maturity.
//
// The wallet calls it with its own lock held, so code holding the harness
// lock must not take the wallet's.
func (h *Harness) lastMatureCoinbase(height int32) int32 {
	if h.upgrade2027Time >= Upgrade2027Never {
		return height - int32(h.ActiveNet.CoinbaseMaturity)
	}

	h.Lock()
	defer h.Unlock()
	hash, err := h.Node.GetBlockHash(int64(height))
	if err != nil {
		log.Printf("Cannot find the mature coinbase outputs: %v", err)
		return -1
	}
	activation, err := h.upgrade2027Height(hash, height)
	if err != nil {
		log.Printf("Cannot find the mature coinbase outputs: %v", err)
		return -1
	}
	maturity := int64(h.ActiveNet.CoinbaseMaturity) * 600
	return ticksToHeight(activation,
		heightToTicks(activation, height)-maturity)
}

// coinbaseMaturityBlocks returns a number of blocks after which the wallet
// is sure to spend a coinbase output.  Once 1-minute blocks may activate,
// that is ten times the coinbase maturity.
func (h *Harness) coinbaseMaturityBlocks() uint32 {
	blocks := uint32(h.ActiveNet.CoinbaseMaturity)
	if h.upgrade2027Time < Upgrade2027Never {
		blocks *= 10
	}
	return blocks
}

// upgrade2027Height returns the height of the upgrade 2027 activation block
// up to the block with the passed hash and height, the first block whose
// median time past reaches the activation time, or -1 when there is none.
func (h *Harness) upgrade2027Height(prevHash *chainhash.Hash,
	prevHeight int32) (int32, error) {

	if h.upgrade2027Time >= Upgrade2027Never {
		return -1, nil
	}
	if b := h.upgrade2027Block; b != nil && b.height <= prevHeight {
		hash, err := h.Node.GetBlockHash(int64(b.height))
		if err != nil {
			return 0, err
		}
		if *hash == b.hash {
			return b.height, nil
		}
	}

	// Walk back from the passed block while the median time past still
	// reaches the activation time.  Blocks are fetched newest first, so
	// times[i] is the time of the block at prevHeight-i.
	var times []int64
	var hashes []chainhash.Hash
	hash := *prevHash
	activation := int32(-1)
	for i := 0; i <= int(prevHeight); {
		// The median time past of a block takes the times of the 11
		// blocks up to it, or of all of them near the genesis block.
		lastHeight := int(prevHeight) - len(times) + 1
		if len(times) < i+11 && lastHeight > 0 {
			header, err := h.Node.GetBlockHeader(&hash)
			if err != nil {
				return 0, err
			}
			times = append(times, header.Timestamp.Unix())
			hashes = append(hashes, hash)
			hash = header.PrevBlock
			continue
		}
		window := slices.Clone(times[i:min(i+11, len(times))])
		slices.Sort(window)
		if uint64(window[len(window)/2]) < h.upgrade2027Time {
			break
		}
		activation = prevHeight - int32(i)
		i++
	}
	if activation >= 0 {
		h.upgrade2027Block = &upgrade2027Block{
			hash:   hashes[prevHeight-activation],
			height: activation,
		}
	}
	return activation, nil
}
