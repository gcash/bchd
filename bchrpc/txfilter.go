package bchrpc

import (
	"encoding/hex"
	"fmt"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
	"github.com/simpleledgerinc/GoSlp/parser"
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
	dataElements        map[string]struct{}
	fallbacks           map[string]struct{}

	matchAll bool

	matchAllSlp bool
	slpTokenIds map[[32]byte]struct{}
}

// newTxFilter creates a new txFilter.
func newTxFilter() *txFilter {
	return &txFilter{
		outpoints:           map[wire.OutPoint]struct{}{},
		pubKeyHashes:        map[[ripemd160.Size]byte]struct{}{},
		scriptHashes:        map[[ripemd160.Size]byte]struct{}{},
		compressedPubKeys:   map[[33]byte]struct{}{},
		uncompressedPubKeys: map[[65]byte]struct{}{},
		dataElements:        map[string]struct{}{},
		fallbacks:           map[string]struct{}{},
		slpTokenIds:         map[[32]byte]struct{}{},
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

// AddDataElement adds a new data element to the filter.
func (f *txFilter) AddDataElement(dataElement []byte) {
	if len(dataElement) > txscript.MaxScriptElementSize {
		return
	}
	f.dataElements[hex.EncodeToString(dataElement)] = struct{}{}
}

// RemoveDataElement removes a data element from the filter.
func (f *txFilter) RemoveDataElement(dataElement []byte) {
	delete(f.dataElements, hex.EncodeToString(dataElement))
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
			// TODO: handle addresses in slpAddr format
			return fmt.Errorf("Unable to decode address '%v': %v", addrStr, err)
		}
		f.AddAddress(addr)
	}

	// Add data elements
	for _, dataElement := range rpcFilter.GetDataElements() {
		f.AddDataElement(dataElement)
	}

	f.matchAll = rpcFilter.AllTransactions

	// handle SLP
	if !f.matchAll {
		f.matchAllSlp = rpcFilter.AllSlpTransactions
		if !f.matchAllSlp {
			for _, tokenID := range rpcFilter.GetSlpTokenIds() {
				f.AddSlpTokenID(tokenID)
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
			return err
		}
		f.RemoveOutpoint(wire.OutPoint{Hash: *hash, Index: op.GetIndex()})
	}

	// Interpret and remove addresses.
	for _, addrStr := range rpcFilter.GetAddresses() {
		addr, err := bchutil.DecodeAddress(addrStr, params)
		if err != nil {
			// TODO: handle addresses in slpAddr format
			return fmt.Errorf("unable to decode address '%v': %v", addrStr, err)
		}
		f.RemoveAddress(addr)
	}

	// Remove data elements
	for _, dataElement := range rpcFilter.GetDataElements() {
		f.RemoveDataElement(dataElement)
	}

	f.matchAll = rpcFilter.AllTransactions

	// handle SLP
	for _, tokenId := range rpcFilter.GetSlpTokenIds() {
		f.RemoveSlpTokenID(tokenId)
	}

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

	slpMsg, err := parser.ParseSLP(tx.MsgTx().TxOut[0].PkScript)
	if err == nil {
		if f.matchAllSlp {
			matched = true
		} else {
			var tokenID [32]byte
			if slpMsg.TransactionType == "SEND" {
				copy(tokenID[:], slpMsg.Data.(parser.SlpSend).TokenID)
			} else if slpMsg.TransactionType == "MINT" {
				copy(tokenID[:], slpMsg.Data.(parser.SlpMint).TokenID)
			} else if slpMsg.TransactionType == "GENESIS" {
				txnHash := tx.Hash().CloneBytes()
				var txid []byte
				for i := len(txnHash) - 1; i >= 0; i-- {
					txid = append(txid, txnHash[i])
				}
				copy(tokenID[:], txid)
			}
			_, ok := f.slpTokenIds[tokenID]
			if ok {
				matched = true
			}
		}
	}

	for _, txin := range tx.MsgTx().TxIn {
		if _, ok := f.outpoints[txin.PreviousOutPoint]; ok {
			delete(f.outpoints, txin.PreviousOutPoint)

			matched = true
		}
	}

	for txOutIdx, txout := range tx.MsgTx().TxOut {
		outputMatch := false
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
			outputMatch = true
			break
		}

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
			f.outpoints[outpoint] = struct{}{}

			matched = true
		}
	}

	return matched
}
