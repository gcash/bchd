package avalanche

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

// utxoFetcher provides a method for getting current UTXOs from their outpoints.
type utxoFetcher interface {
	FetchUtxoEntry(wire.OutPoint) (*blockchain.UtxoEntry, error)
}

// Identity represents an Identity combined with a stake of UTXOs.
type Identity struct {
	privKey   bchec.PrivateKey
	PubKey    bchec.PublicKey `json:"pubkey"`
	Version   int8            `json:"version"`
	Sequence  int64           `json:"sequence"`
	OutPoints []wire.OutPoint `json:"out_points"`
}

// NewIdentity creates a new Identity for the given Identity and stake.
func NewIdentity(privKey bchec.PrivateKey, outPoints []wire.OutPoint) Identity {
	return Identity{
		privKey:   privKey,
		PubKey:    *privKey.PubKey(),
		Version:   version,
		Sequence:  time.Now().UnixNano(),
		OutPoints: outPoints,
	}
}

// Sign signs the given message with the private Identity key.
func (si Identity) Sign(d []byte) (*bchec.Signature, error) { return si.privKey.SignSchnorr(d) }

// func (si Identity) Sign(d []byte) (*bchec.Signature, error) { return si.privKey.SignECDSA(d) }

// Serialize returns a byte slice of the a canonically serialized Identity.
func (si Identity) Serialize() ([]byte, error) { return json.Marshal(si) }

// SignedIdentity represents a Identity that has been signed with the Identity
// private key and every private key required to spend the staked UTXOs.
type SignedIdentity struct {
	Identity           `json:"staked_identity"`
	IdentitySignature  *bchec.Signature   `json:"identity_signature"`
	OutPointSignatures []*bchec.Signature `json:"out_points_signatures"`
}

// NewSignedIdentity signs the given Identity with the given keys and returns a
// SignedIdentity.
func NewSignedIdentity(i Identity, keys []bchec.PrivateKey) (*SignedIdentity, error) {
	ser, err := i.Serialize()
	if err != nil {
		return nil, err
	}

	ssi := &SignedIdentity{Identity: i}
	ssi.IdentitySignature, err = i.Sign(ser)
	if err != nil {
		return nil, err
	}

	ssi.OutPointSignatures = make([]*bchec.Signature, len(ssi.OutPoints))
	for i, key := range keys {
		ssi.OutPointSignatures[i], err = key.SignECDSA(ser)
		if err != nil {
			return nil, err
		}
	}

	return ssi, nil
}

// Validate ensures the ssi satisfies the  requirements of an Ava peer.
func (ssi *SignedIdentity) Validate(fetcher utxoFetcher) error {
	var stakedBitcoinBlocks uint64
	utxoPubkeyHashes := map[bchutil.Address]struct{}{}

	ser, err := ssi.Identity.Serialize()
	if err != nil {
		return err
	}

	serHashArr := sha256.Sum256(ser)
	ser = serHashArr[:]

	// Create set of all possible hashes for the pubkeys we have signatures for
	for _, s := range ssi.OutPointSignatures {
		_, _, err := bchec.RecoverCompact(bchec.S256(), s.Serialize(), ser)
		if err != nil {
			return err
		}
	}

	// Get each outpoint, validate that its unlocking pubkey is one we have a
	// signature for in our map, and then add its value to our accumulator
	for _, outpoint := range ssi.OutPoints {
		// Make sure utxo exists and hasn't been spent
		utxo, err := fetcher.FetchUtxoEntry(outpoint)
		if err != nil {
			return errors.New("stake not found")
		}

		if utxo.IsSpent() {
			return errors.New("stake has been spent")
		}

		// Get pubkey hash for the UTXO. Currently we only support UTXOs encumbered
		// by a single pubkey and of a known standard type.
		scriptType, pkHashes, requiredSigCount, err := txscript.ExtractPkScriptAddrs(utxo.PkScript(), &chaincfg.MainNetParams)
		if err != nil {
			return err
		}

		if requiredSigCount != 1 {
			return errors.New("unsupported utxo script type")
		}

		switch scriptType {
		case txscript.PubKeyTy:
		case txscript.MultiSigTy:
		case txscript.PubKeyHashTy:
		default:
			return errors.New("unsupported utxo script type")
		}

		// Check that this UTXO's pkHash is one we have a signature for
		if _, ok := utxoPubkeyHashes[pkHashes[0]]; !ok {
			return errors.New("not all conflictTxs were signed")
		}

		// TODO: Multiply by utxo age in blocks
		stakedBitcoinBlocks += uint64(utxo.Amount())
	}

	if stakedBitcoinBlocks < minStakeAmount {
		return errors.New("insufficient stake amount")
	}

	return nil
}
