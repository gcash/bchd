package bchrpc

import (
	"encoding/hex"
	"slices"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/gcash/goslp/v1parser"
	"golang.org/x/crypto/ripemd160"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// dataElementUnits returns how much a data element counts toward a filter's
// size: once for each 20 bytes, rounded up, and at least once, since a data
// element can be 500 times as long as an address hash.
func dataElementUnits(dataElement []byte) int {
	return max(1, (len(dataElement)+19)/20)
}

// txFilter is used to filter transactions based on a clients interest.
// It supports filtering for TX outpoints and all kinds of addresses.
type txFilter struct {
	outpoints           map[wire.OutPoint]struct{}
	pubKeyHashes        map[[ripemd160.Size]byte]struct{}
	scriptHashes        map[[ripemd160.Size]byte]struct{}
	compressedPubKeys   map[[33]byte]struct{}
	uncompressedPubKeys map[[65]byte]struct{}
	dataElements        map[string]struct{}
	fallbacks           map[string]struct{}

	// dataElementSize is the sum of dataElementUnits over the data
	// elements.
	dataElementSize int

	// matched and matchedOld hold the outpoints of the outputs
	// MatchAndUpdate matched, so that their spends match too, the newer in
	// matched.  They are apart from the outpoints requests add, as they
	// are not the client's to bound, and they hold at most maxSize
	// together: once matched holds half of it, the older are dropped and
	// matched becomes matchedOld.
	matched    map[wire.OutPoint]struct{}
	matchedOld map[wire.OutPoint]struct{}

	// maxSize is the most the filter may hold, as size counts it, and the
	// most matched and matchedOld hold together.
	maxSize int

	matchAll bool

	matchAllSlp bool
	slpTokenIds map[[32]byte]struct{}
}

// newTxFilter creates a new txFilter that holds at most maxSize, as size
// counts it.
func newTxFilter(maxSize int) *txFilter {
	return &txFilter{
		outpoints:           map[wire.OutPoint]struct{}{},
		pubKeyHashes:        map[[ripemd160.Size]byte]struct{}{},
		scriptHashes:        map[[ripemd160.Size]byte]struct{}{},
		compressedPubKeys:   map[[33]byte]struct{}{},
		uncompressedPubKeys: map[[65]byte]struct{}{},
		dataElements:        map[string]struct{}{},
		fallbacks:           map[string]struct{}{},
		slpTokenIds:         map[[32]byte]struct{}{},
		matched:             map[wire.OutPoint]struct{}{},
		matchedOld:          map[wire.OutPoint]struct{}{},
		maxSize:             maxSize,
	}
}

// AddOutpoint adds a new outpoint to the filter.
func (f *txFilter) AddOutpoint(op wire.OutPoint) {
	f.outpoints[op] = struct{}{}
}

// RemoveOutpoint removes an outpoint from the filter, whether a request added
// it or MatchAndUpdate did, and returns whether the filter held it.
func (f *txFilter) RemoveOutpoint(op wire.OutPoint) bool {
	_, added := f.outpoints[op]
	_, matched := f.matched[op]
	_, matchedOld := f.matchedOld[op]
	delete(f.outpoints, op)
	delete(f.matched, op)
	delete(f.matchedOld, op)
	return added || matched || matchedOld
}

// addMatchedOutpoint keeps the outpoint of an output MatchAndUpdate matched,
// dropping the older half of those kept once they would be more than maxSize.
func (f *txFilter) addMatchedOutpoint(op wire.OutPoint) {
	if f.maxSize <= 0 {
		return
	}
	f.matched[op] = struct{}{}
	if len(f.matched) >= max(1, f.maxSize/2) {
		f.matchedOld = f.matched
		f.matched = make(map[wire.OutPoint]struct{})
	}
}

// AddDataElement adds a new data element to the filter.
func (f *txFilter) AddDataElement(dataElement []byte) {
	if len(dataElement) > txscript.MaxScriptElementSize {
		return
	}
	key := hex.EncodeToString(dataElement)
	if _, ok := f.dataElements[key]; !ok {
		f.dataElements[key] = struct{}{}
		f.dataElementSize += dataElementUnits(dataElement)
	}
}

// RemoveDataElement removes a data element from the filter.
func (f *txFilter) RemoveDataElement(dataElement []byte) {
	key := hex.EncodeToString(dataElement)
	if _, ok := f.dataElements[key]; ok {
		delete(f.dataElements, key)
		f.dataElementSize -= dataElementUnits(dataElement)
	}
}

// size returns how much the filter holds of what requests added: each
// outpoint, address and token ID counts once, and each data element as
// dataElementUnits says.  The outputs MatchAndUpdate matched are not counted.
func (f *txFilter) size() int {
	return len(f.outpoints) + len(f.pubKeyHashes) + len(f.scriptHashes) +
		len(f.compressedPubKeys) + len(f.uncompressedPubKeys) +
		len(f.fallbacks) + len(f.slpTokenIds) + f.dataElementSize
}

func (f *txFilter) AddSlpTokenID(tokenID []byte) {
	var id [32]byte
	copy(id[:], tokenID)
	f.slpTokenIds[id] = struct{}{}
}

func (f *txFilter) RemoveSlpTokenID(tokenID []byte) {
	var id [32]byte
	copy(id[:], tokenID)
	delete(f.slpTokenIds, id)
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
		// is added.  A filter can hold any number of addresses, so
		// this is logged at debug.
		addrStr := addr.EncodeAddress()
		log.Debugf("Unknown address type: %v", addrStr)
		f.fallbacks[addrStr] = struct{}{}
	}
}

// hasAddresses returns whether the filter is watching any addresses at all.
// Resolving previous outputs to match spends is only worth the lookups when it
// is.
func (f *txFilter) hasAddresses() bool {
	return len(f.pubKeyHashes) > 0 || len(f.scriptHashes) > 0 ||
		len(f.compressedPubKeys) > 0 || len(f.uncompressedPubKeys) > 0 ||
		len(f.fallbacks) > 0
}

// matchAddresses returns whether any of the provided addresses is watched by
// the filter.
func (f *txFilter) matchAddresses(addrs []bchutil.Address) bool {
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

			// If the output pays to the pubkey of a rescanned P2PKH
			// address, include it as well.
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
			if _, ok := f.fallbacks[addrStr]; !ok {
				continue
			}
		}

		return true
	}

	return false
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
		// is added.  A filter can hold any number of addresses, so
		// this is logged at debug.
		addrStr := addr.EncodeAddress()
		log.Debugf("Unknown address type: %v", addrStr)
		delete(f.fallbacks, addrStr)
	}
}

// decodeFilterAddress decodes the passed address for a filter.  A legacy or
// slp address is converted to cash address format, which is how the addresses
// of outputs are extracted, while a pubkey is kept so bare pubkey outputs
// still match.
func decodeFilterAddress(addrStr string, params *chaincfg.Params) (bchutil.Address, error) {
	addr, err := bchutil.DecodeAddress(addrStr, params)
	if err != nil {
		return nil, err
	}
	if _, ok := addr.(*bchutil.AddressPubKey); ok {
		return addr, nil
	}
	return toCashAddress(addr, params)
}

// invalidOutpointErr returns the error for a filter outpoint whose hash cannot
// be decoded.  It is an invalid argument, which the server logs at debug.
func invalidOutpointErr(err error) error {
	return status.Errorf(codes.InvalidArgument, "invalid outpoint hash: %v", err)
}

// invalidAddressErr returns the error for the address at index i of a filter
// when it cannot be decoded.  The address is named by its index rather than
// quoted, as it can be as long as the request; the decoding errors are short.
func invalidAddressErr(i int, err error) error {
	return status.Errorf(codes.InvalidArgument, "invalid address at index %d: %v",
		i, err)
}

// filterFullErr is the error for a filter that would hold more than its
// maxSize.
func (f *txFilter) filterFullErr() error {
	return status.Errorf(codes.ResourceExhausted,
		"filter holds more than %d entries", f.maxSize)
}

// AddRPCFilter adds all filter properties from the pb.TransactionFilter
// to the filter.  It fails once the filter holds more than its maxSize,
// with some of the entries added, so the caller must then drop the filter.
func (f *txFilter) AddRPCFilter(rpcFilter *pb.TransactionFilter, params *chaincfg.Params) error {
	if rpcFilter == nil {
		return nil
	}
	// Add outpoints.
	for _, op := range rpcFilter.GetOutpoints() {
		hash, err := chainhash.NewHash(op.GetHash())
		if err != nil {
			return invalidOutpointErr(err)
		}
		f.AddOutpoint(wire.OutPoint{Hash: *hash, Index: op.GetIndex()})
		if f.size() > f.maxSize {
			return f.filterFullErr()
		}
	}

	// Interpret and add addresses.
	for i, addrStr := range rpcFilter.GetAddresses() {
		addr, err := decodeFilterAddress(addrStr, params)
		if err != nil {
			return invalidAddressErr(i, err)
		}
		f.AddAddress(addr)
		if f.size() > f.maxSize {
			return f.filterFullErr()
		}
	}

	// Add data elements
	for _, dataElement := range rpcFilter.GetDataElements() {
		f.AddDataElement(dataElement)
		if f.size() > f.maxSize {
			return f.filterFullErr()
		}
	}

	f.matchAll = rpcFilter.AllTransactions

	// handle SLP
	if !f.matchAll {
		f.matchAllSlp = rpcFilter.AllSlpTransactions
		if !f.matchAllSlp {
			for _, tokenID := range rpcFilter.GetSlpTokenIds() {
				f.AddSlpTokenID(tokenID)
				if f.size() > f.maxSize {
					return f.filterFullErr()
				}
			}
		}
	}

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
			return invalidOutpointErr(err)
		}
		f.RemoveOutpoint(wire.OutPoint{Hash: *hash, Index: op.GetIndex()})
	}

	// Interpret and remove addresses.
	for i, addrStr := range rpcFilter.GetAddresses() {
		addr, err := decodeFilterAddress(addrStr, params)
		if err != nil {
			return invalidAddressErr(i, err)
		}
		f.RemoveAddress(addr)
	}

	// Remove data elements
	for _, dataElement := range rpcFilter.GetDataElements() {
		f.RemoveDataElement(dataElement)
	}

	f.matchAll = rpcFilter.AllTransactions

	// handle SLP
	for _, tokenID := range rpcFilter.GetSlpTokenIds() {
		f.RemoveSlpTokenID(tokenID)
	}

	return nil
}

// prevOutScriptFunc returns the pkScript of the output referenced by the passed
// outpoint, or nil when it cannot be resolved.
type prevOutScriptFunc func(wire.OutPoint) []byte

// MatchAndUpdate returns whether the transaction matches against the filter.
// When the tx contains any matching outputs, all these outputs are added to the
// filter as outpoints for matching further spends of these outputs, of which
// it keeps the newest, at most maxSize.  All matching outpoints are removed
// from the filter.
//
// prevOutScript is used to match spends of outputs the filter did not see
// arrive, which is every output a watched address already held when the
// subscription started.  It may be nil, in which case only outputs observed by
// this filter can match on being spent.
func (f *txFilter) MatchAndUpdate(tx *bchutil.Tx, params *chaincfg.Params,
	prevOutScript prevOutScriptFunc) bool {
	// We don't return early on a match because we prefer full processing:
	// - all matching outputs need to be added to the filter for later matching
	// - all matching inputs can be removed from the filter for later efficiency

	matched := f.matchAll

	if len(tx.MsgTx().TxOut) > 0 {
		slpMsg, err := v1parser.ParseSLP(tx.MsgTx().TxOut[0].PkScript)
		if err == nil && slpMsg != nil {
			if f.matchAllSlp {
				matched = true
			} else {
				var tokenID [32]byte
				switch msg := slpMsg.(type) {
				case *v1parser.SlpGenesis:
					txnHash := tx.Hash().CloneBytes()
					var txid []byte
					for _, v := range slices.Backward(txnHash) {
						txid = append(txid, v)
					}
					copy(tokenID[:], txid)
				case *v1parser.SlpMint:
					copy(tokenID[:], msg.TokenID())
				case *v1parser.SlpSend:
					copy(tokenID[:], msg.TokenID())
				}
				_, ok := f.slpTokenIds[tokenID]
				if ok {
					matched = true
				}
			}
		}
	}

	for _, txin := range tx.MsgTx().TxIn {
		if f.RemoveOutpoint(txin.PreviousOutPoint) {
			matched = true
			continue
		}

		// The outpoint map only holds outputs this filter watched arrive, so on
		// its own it cannot match a spend of an output the address already held
		// when the subscription started.  Resolve the previous output and match
		// on its address instead.
		if prevOutScript == nil || !f.hasAddresses() {
			continue
		}
		pkScript := prevOutScript(txin.PreviousOutPoint)
		if pkScript == nil {
			continue
		}
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript, params)
		if err == nil && f.matchAddresses(addrs) {
			matched = true
		}
	}

	for txOutIdx, txout := range tx.MsgTx().TxOut {
		_, addrs, _, _ := txscript.ExtractPkScriptAddrs(txout.PkScript, params)
		outputMatch := f.matchAddresses(addrs)

		dataElements, err := txscript.ExtractDataElements(txout.PkScript)
		if err == nil && len(dataElements) > 0 {
			for _, dataElement := range dataElements {
				_, ok := f.dataElements[hex.EncodeToString(dataElement)]
				if ok {
					outputMatch = true
					break
				}
			}
		}

		// Matching output should be added to filter.
		if outputMatch {
			outpoint := wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(txOutIdx),
			}
			f.addMatchedOutpoint(outpoint)

			matched = true
		}
	}

	return matched
}
