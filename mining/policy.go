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
)

// Policy houses the policy (configuration parameters) which is used to control
// the generation of block templates.  See the documentation for
// NewBlockTemplate for more details on each of these parameters are used.
type Policy struct {
	// BlockMaxSize is the maximum block size to be used when generating a
	// block template.  Zero means the block size limit less 1000 bytes.
	BlockMaxSize uint32

	// TxMinFreeFee is the minimum fee in Satoshi/1000 bytes that is
	// required for a transaction to be included in a block template.
	TxMinFreeFee bchutil.Amount
}
