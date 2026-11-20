// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"github.com/gcash/bchutil"
)

const (
	// UnminedHeight is the height used for the "block" height field of the
	// contextual transaction information provided in a transaction store
	// when it has not yet been mined into a block.
	UnminedHeight = 0x7fffffff

	// policyBlockInterval is the target interval in seconds BlockMaxSize
	// is given for.
	policyBlockInterval = 600
)

// Policy houses the policy (configuration parameters) which is used to control
// the generation of block templates.  See the documentation for
// NewBlockTemplate for more details on each of these parameters are used.
type Policy struct {
	// BlockMaxSize is the maximum block size to be used when generating a
	// block template, for a 600-second block.  Blocks with a shorter target
	// interval get a share of it in proportion.  Zero means the block size
	// limit less 1000 bytes.
	BlockMaxSize uint32

	// TxMinFreeFee is the minimum fee in Satoshi/1000 bytes that is
	// required for a transaction to be included in a block template.
	TxMinFreeFee bchutil.Amount
}

// blockMaxSize returns the configured maximum size of a block with the passed
// target interval in seconds, or zero when none is configured.
func (p *Policy) blockMaxSize(interval int64) uint64 {
	return uint64(p.BlockMaxSize) * uint64(interval) / policyBlockInterval
}
