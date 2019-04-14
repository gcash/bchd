package bchrpc

import (
	"fmt"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"golang.org/x/crypto/ripemd160"
)

// txFilter is used to filter transactions based on a clients interest.
// It supports filtering for TX outpoints and all kinds of addresses.
type txFilter struct {
	outpoints           map[wire.OutPoint]struct{}
	pubKeyHashes        map[[ripemd160.Size]byte]struct{}
	scriptHashes        map[[ripemd160.Size]byte]struct{}
	compressedPubKeys   map[[33]byte]struct{}
	uncompressedPubKeys map[[65]byte]struct{}
	fallbacks           map[string]struct{}

	matchAll bool
}

// newTxFilter creates a new txFilter.
func newTxFilter() *txFilter {
	return &txFilter{
		outpoints:           map[wire.OutPoint]struct{}{},
		pubKeyHashes:        map[[ripemd160.Size]byte]struct{}{},
		scriptHashes:        map[[ripemd160.Size]byte]struct{}{},
		compressedPubKeys:   map[[33]byte]struct{}{},
		uncompressedPubKeys: map[[65]byte]struct{}{},
		fallbacks:           map[string]struct{}{},
	}
}

// AddOutpoint adds a new outpoint to the filter.
func (f *txFilter) AddOutpoint(op wire.OutPoint) {
	f.outpoints[op] = struct{}{}
}

// RemoveOutpoint removes an outpoint from the filter.
func (f *txFilter) RemoveOutpoint(op wire.OutPoint) {
	delete(f.outpoints, op)
}

// AddAddress adds a new address to the filter.
func (f *txFilter) AddAddress(addr bchutil.Address) {
	switch a := addr.(type) {
	case *bchutil.AddressPubKeyHash:
		f.pubKeyHashes[*a.Hash160()] = struct{}{}

	case *bchutil.AddressScriptHash:
		f.scriptHashes[*a.Hash160()] = struct{}{}

	case *bchutil.AddressPubKey:
		pubkeyBytes := a.ScriptAddress()
		switch len(pubkeyBytes) {
		case 33: // Compressed
			var compressedPubkey [33]byte
			copy(compressedPubkey[:], pubkeyBytes)
			f.compressedPubKeys[compressedPubkey] = struct{}{}

		case 65: // Uncompressed
			var uncompressedPubkey [65]byte
			copy(uncompressedPubkey[:], pubkeyBytes)
			f.uncompressedPubKeys[uncompressedPubkey] = struct{}{}
		}

	default:
		// A new address type must have been added.  Use encoded
		// payment address string as a fallback until a fast path
		// is added.
		addrStr := addr.EncodeAddress()
		log.Infof("Unknown address type: %v", addrStr)
		f.fallbacks[addrStr] = struct{}{}
	}
}

// RemoveAddress removes an address from the filter.
func (f *txFilter) RemoveAddress(addr bchutil.Address) {
	switch a := addr.(type) {
	case *bchutil.AddressPubKeyHash:
		delete(f.pubKeyHashes, *a.Hash160())

	case *bchutil.AddressScriptHash:
		delete(f.scriptHashes, *a.Hash160())

	case *bchutil.AddressPubKey:
		pubkeyBytes := a.ScriptAddress()
		switch len(pubkeyBytes) {
		case 33: // Compressed
			var compressedPubkey [33]byte
			copy(compressedPubkey[:], pubkeyBytes)
			delete(f.compressedPubKeys, compressedPubkey)

		case 65: // Uncompressed
			var uncompressedPubkey [65]byte
			copy(uncompressedPubkey[:], pubkeyBytes)
			delete(f.uncompressedPubKeys, uncompressedPubkey)
		}

	default:
		// A new address type must have been added.  Use encoded
		// payment address string as a fallback until a fast path
		// is added.
		addrStr := addr.EncodeAddress()
		log.Infof("Unknown address type: %v", addrStr)
		delete(f.fallbacks, addrStr)
	}
}

// AddRPCFilter adds all filter properties from the pb.TransactionFilter
// to the filter.
func (f *txFilter) AddRPCFilter(rpcFilter *pb.TransactionFilter, params *chaincfg.Params) error {
	if rpcFilter == nil {
		return nil
	}
	// Add outpoints.
	for _, op := range rpcFilter.GetOutpoints() {
		hash, err := chainhash.NewHash(op.GetHash())
		if err != nil {
			return err
		}
		f.AddOutpoint(wire.OutPoint{Hash: *hash, Index: op.GetIndex()})
	}

	// Interpret and add addresses.
	for _, addrStr := range rpcFilter.GetAddresses() {
		addr, err := bchutil.DecodeAddress(addrStr, params)
		if err != nil {
			return fmt.Errorf("Unable to decode address '%v': %v", addrStr, err)
		}
		f.AddAddress(addr)
	}

	f.matchAll = rpcFilter.AllTransactions

	return nil
}

// RemoveRPCFilter removes all filter properties from the
// pb.TransactionFilter from the filter.
func (f *txFilter) RemoveRPCFilter(rpcFilter *pb.TransactionFilter, params *chaincfg.Params) error {
	if rpcFilter == nil {
		return nil
	}
	// Remove outpoints.
	for _, op := range rpcFilter.GetOutpoints() {
		hash, err := chainhash.NewHash(op.GetHash())
		if err != nil {
			return err
		}
		f.RemoveOutpoint(wire.OutPoint{Hash: *hash, Index: op.GetIndex()})
	}

	// Interpret and remove addresses.
	for _, addrStr := range rpcFilter.GetAddresses() {
		addr, err := bchutil.DecodeAddress(addrStr, params)
		if err != nil {
			return fmt.Errorf("Unable to decode address '%v': %v", addrStr, err)
		}
		f.RemoveAddress(addr)
	}

	f.matchAll = rpcFilter.AllTransactions

	return nil
}

// MatchAndUpdate returns whether the transaction matches against the filter.
// When the tx contains any matching outputs, all these outputs are added to the
// filter as outpoints for matching further spends of these outputs.
// All matching outpoints are removed from the filter.
func (f *txFilter) MatchAndUpdate(tx *bchutil.Tx, params *chaincfg.Params) bool {
	// We don't return early on a match because we prefer full processing:
	// - all matching outputs need to be added to the filter for later matching
	// - all matching inputs can be removed from the filter for later efficiency

	matched := f.matchAll

	for _, txin := range tx.MsgTx().TxIn {
		if _, ok := f.outpoints[txin.PreviousOutPoint]; ok {
			delete(f.outpoints, txin.PreviousOutPoint)

			matched = true
		}
	}

	for txOutIdx, txout := range tx.MsgTx().TxOut {
		_, addrs, _, _ := txscript.ExtractPkScriptAddrs(txout.PkScript, params)

		for _, addr := range addrs {
			switch a := addr.(type) {
			case *bchutil.AddressPubKeyHash:
				if _, ok := f.pubKeyHashes[*a.Hash160()]; !ok {
					continue
				}

			case *bchutil.AddressScriptHash:
				if _, ok := f.scriptHashes[*a.Hash160()]; !ok {
					continue
				}

			case *bchutil.AddressPubKey:
				found := false
				switch sa := a.ScriptAddress(); len(sa) {
				case 33: // Compressed
					var key [33]byte
					copy(key[:], sa)
					if _, ok := f.compressedPubKeys[key]; ok {
						found = true
					}

				case 65: // Uncompressed
					var key [65]byte
					copy(key[:], sa)
					if _, ok := f.uncompressedPubKeys[key]; ok {
						found = true
					}

				default:
					log.Warnf("Skipping rescanned pubkey of unknown "+
						"serialized length %d", len(sa))
					continue
				}

				// If the transaction output pays to the pubkey of
				// a rescanned P2PKH address, include it as well.
				if !found {
					pkh := a.AddressPubKeyHash()
					if _, ok := f.pubKeyHashes[*pkh.Hash160()]; !ok {
						continue
					}
				}

			default:
				// A new address type must have been added.  Encode as a
				// payment address string and check the fallback map.
				addrStr := addr.EncodeAddress()
				_, ok := f.fallbacks[addrStr]
				if !ok {
					continue
				}
			}

			// Matching address.
			break
		}

		// Matching output should be added to filter.
		outpoint := wire.OutPoint{
			Hash:  *tx.Hash(),
			Index: uint32(txOutIdx),
		}
		f.outpoints[outpoint] = struct{}{}

		matched = true
	}

	return matched
}
