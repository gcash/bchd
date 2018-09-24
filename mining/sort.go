package mining

import "github.com/gcash/bchutil"

// txSorter implements sort.Interface to allow a slice of block headers to
// be sorted by timestamp.
type txSorter []*bchutil.Tx

// Len returns the number of txs in the slice.  It is part of the
// sort.Interface implementation.
func (s txSorter) Len() int {
	return len(s)
}

// Swap swaps the txs at the passed indices.  It is part of the
// sort.Interface implementation.
func (s txSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the txs with index i should sort before the
// tx with index j.  It is part of the sort.Interface implementation.
func (s txSorter) Less(i, j int) bool {
	return s[i].Hash().Compare(s[j].Hash()) < 0
}
