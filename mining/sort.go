package mining

import "github.com/gcash/bchutil"

// TxSorter implements sort.Interface to allow a slice of block headers to
// be sorted by timestamp.
type TxSorter []*bchutil.Tx

// Len returns the number of txs in the slice.  It is part of the
// sort.Interface implementation.
func (s TxSorter) Len() int {
	return len(s)
}

// Swap swaps the txs at the passed indices.  It is part of the
// sort.Interface implementation.
func (s TxSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the txs with index i should sort before the
// tx with index j.  It is part of the sort.Interface implementation.
func (s TxSorter) Less(i, j int) bool {
	return s[i].Hash().Compare(s[j].Hash()) < 0
}

// templateTxSorter implements sort.Interface to sort the transactions of a
// block template by txid, like TxSorter, while keeping the fee and signature
// check count of each transaction at the same index.
type templateTxSorter struct {
	txs       []*bchutil.Tx
	fees      []int64
	sigChecks []int64
}

// Len returns the number of txs.  It is part of the sort.Interface
// implementation.
func (s templateTxSorter) Len() int {
	return len(s.txs)
}

// Swap swaps the txs, and their fees and signature check counts, at the
// passed indices.  It is part of the sort.Interface implementation.
func (s templateTxSorter) Swap(i, j int) {
	s.txs[i], s.txs[j] = s.txs[j], s.txs[i]
	s.fees[i], s.fees[j] = s.fees[j], s.fees[i]
	s.sigChecks[i], s.sigChecks[j] = s.sigChecks[j], s.sigChecks[i]
}

// Less returns whether the tx with index i should sort before the tx with
// index j.  It is part of the sort.Interface implementation.
func (s templateTxSorter) Less(i, j int) bool {
	return s.txs[i].Hash().Compare(s.txs[j].Hash()) < 0
}
