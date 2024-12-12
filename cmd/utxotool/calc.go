package main

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	"github.com/gcash/bchd/wire"
)

var (
	utxoSetBucketName = []byte("utxosetv2")
	byteBuf32         = make([]byte, 4)
	byteBuf64         = make([]byte, 8)
	buf               bytes.Buffer
)

// serializeV0Utxo returns a Utxo serialized into the v0 Utxo commitment format
func serializeV0Utxo(entry *blockchain.UtxoEntry, outpoint *wire.OutPoint) []byte {
	buf = bytes.Buffer{}

	buf.Write(outpoint.Hash.CloneBytes())

	binary.LittleEndian.PutUint32(byteBuf32, outpoint.Index)
	buf.Write(byteBuf32)

	binary.LittleEndian.PutUint32(byteBuf32, uint32(entry.BlockHeight()))
	// If this is a coinbase then the least significant bit of the height should be set to 1
	if entry.IsCoinBase() {
		byteBuf32[3] |= 0x01
	}
	buf.Write(byteBuf32)

	binary.LittleEndian.PutUint64(byteBuf64, uint64(entry.Amount()))
	buf.Write(byteBuf64)

	binary.LittleEndian.PutUint32(byteBuf32, uint32(len(entry.PkScript())))
	buf.Write(byteBuf32)

	buf.Write(entry.PkScript())
	return buf.Bytes()
}

// CalcUtxoSet rolls back the chain to the given block height then loads
// the Utxo set and calculates the ECMH hash.
func CalcUtxoSet(db database.DB, height int32, utxoWriter io.Writer) (*chainhash.Hash, int, error) {
	chain, err := blockchain.New(&blockchain.Config{
		DB:          db,
		ChainParams: activeNetParams,
		TimeSource:  blockchain.NewMedianTime(),
		// No nice way to get the main configuration here.
		// For now just accept up to the default.
		ExcessiveBlockSize: 32000000 * 4, // TODO TODO, is it needed to do that here really?
	})
	if err != nil {
		return nil, 0, err
	}

	view, err := chain.RollbackUtxoSet(height)
	if err != nil {
		return nil, 0, err
	}
	log.Info("Loading Utxo set from disk. This is going to take a while...")
	m := bchec.NewMultiset(bchec.S256())

	// Let's avoid allocating new memory when iterating over utxos
	var (
		entry          *blockchain.UtxoEntry
		viewEntry      *blockchain.UtxoEntry
		outpoint       *wire.OutPoint
		serializedUtxo []byte
		totalSize      int
	)

	err = db.View(func(tx database.Tx) error {
		utxoBucket := tx.Metadata().Bucket(utxoSetBucketName)
		return utxoBucket.ForEach(func(k, v []byte) error {
			entry, err = blockchain.DeserializeUtxoEntry(v)
			if err != nil {
				return err
			}
			outpoint = blockchain.DeserializeOutpointKey(k)

			// If it's in the view let's just exit as we'll deal
			// with the view later.
			viewEntry = view.LookupEntry(*outpoint)
			if viewEntry != nil {
				return nil
			}
			serializedUtxo = serializeV0Utxo(entry, outpoint)
			m.Add(serializedUtxo)
			totalSize += len(serializedUtxo)

			if utxoWriter != nil {
				_, err = utxoWriter.Write(serializedUtxo)
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
	if err != nil {
		return nil, 0, err
	}

	// Finally loop through the entries in the view and add all that aren't spent.
	for outpoint, entry := range view.Entries() {
		if entry.IsSpent() {
			continue
		}
		serializedUtxo = serializeV0Utxo(entry, &outpoint)
		m.Add(serializedUtxo)
		totalSize += len(serializedUtxo)

		if utxoWriter != nil {
			_, err = utxoWriter.Write(serializedUtxo)
			if err != nil {
				return nil, 0, err
			}
		}
	}

	h := m.Hash()
	return &h, totalSize, nil
}
