package blockchain

import (
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/go-socks/socks"
	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/wire"
	"io"
	"math"
	"net"
	"net/http"
	"time"
)

// fastSyncUtxoSet will download the UTXO set from the sources provided in the checkpoint. Each
// UTXO will be saved to the database and the ECMH hash of the UTXO set will be validated against
// the checkpoint. If a proxyAddr is provided it will use that proxy for the HTTP connection.
func (b *BlockChain) fastSyncUtxoSet(checkpoint *chaincfg.Checkpoint, proxyAddr string) error {
	if checkpoint.UtxoSetHash == nil {
		return AssertError("cannot perform fast sync with nil UTXO set hash")
	}
	if len(checkpoint.UtxoSetSources) == 0 {
		return AssertError("no UTXO download sources provided")
	}
	if checkpoint.UtxoSetSize == 0 {
		return AssertError("expected UTXO set size is zero")
	}
	var proxy *socks.Proxy
	if proxyAddr != "" {
		proxy = &socks.Proxy{Addr: proxyAddr}
	}

	utxoReader, err := getUtxoReader(checkpoint.UtxoSetSources, proxy)
	if err != nil {
		log.Errorf("Error downloading UTXO set: %s", err.Error())
		return err
	}

	var (
		m           = bchec.NewMultiset(bchec.S256())
		buf52       = make([]byte, 52)
		pkScript    []byte
		n           int
		totalRead   int
		scriptLen   uint32
		progress    float64
		progressStr string
		entry       *UtxoEntry
		outpoint    *wire.OutPoint
		state       = &BestState{Hash: *checkpoint.UtxoSetHash}
	)

	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			progress = math.Min(float64(totalRead)/float64(checkpoint.UtxoSetSize), 1.0) * 100
			progressStr = fmt.Sprintf("%d/%d bytes (%.2f%%)", totalRead, checkpoint.UtxoSetSize, progress)
			log.Infof("UTXO download progress: processed %s", progressStr)
		}
	}()

	for {
		// Read the first 52 bytes of the utxo
		n, err = utxoReader.Read(buf52)
		if err == io.EOF { // We've hit the end
			break
		} else if err != nil {
			log.Errorf("Error reading UTXO set: %s", err.Error())
			return err
		}
		totalRead += n

		// The last four bytes that we read is the length of the script
		scriptLen = binary.LittleEndian.Uint32(buf52[48:])

		// Read the script
		pkScript = make([]byte, scriptLen)
		n, err = utxoReader.Read(pkScript)
		if err != nil {
			log.Errorf("Error reading UTXO set: %s", err.Error())
			return err
		}
		totalRead += n

		// Add the serialized utxo to the multiset
		m.Add(append(buf52, pkScript...))

		// Deserialize
		outpoint, entry, err = deserializeUtxoCommitmentFormat(append(buf52, pkScript...))
		if err != nil {
			log.Errorf("Error deserializing UTXO: %s", err.Error())
			return err
		}

		// Add the utxo to the cache
		err = b.utxoCache.addEntry(*outpoint, entry, true)
		if err != nil {
			log.Errorf("Error adding UTXO the cache: %s", err.Error())
			return err
		}

		// Maybe flush cache to disk
		b.utxoCache.Flush(FlushIfNeeded, state)
		if err != nil {
			log.Errorf("Error flushing the UTXO cache: %s", err.Error())
			return err
		}
	}

	utxoHash := m.Hash()

	// Make sure the hash of the UTXO set we downloaded matches the expected hash.
	if checkpoint.UtxoSetHash.IsEqual(&utxoHash) {
		log.Errorf("Downloaded UTXO set hash does not match checkpoint."+
			" Expected %s, got %s.", checkpoint.UtxoSetHash.String(), m.Hash().String())
		return AssertError("downloaded invalid UTXO set")
	}

	// Signal fastsync complete
	close(b.fastSyncDone)

	return nil
}

// getUTXOReader will attempt to connect to make an HTTP GET request to the
// provided sources on at a time and return a reader upon successful connection.
// If a proxy is provided it will use it for the HTTP connection.
func getUtxoReader(sources []string, proxy *socks.Proxy) (io.Reader, error) {
	var reader io.Reader

	dialFunc := net.Dial
	if proxy != nil {
		dialFunc = proxy.Dial
	}
	tr := &http.Transport{
		Dial: dialFunc,
	}
	client := &http.Client{Transport: tr}
	for _, src := range sources {
		resp, err := client.Get(src)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			continue
		}
		reader = resp.Body
		break
	}
	if reader == nil {
		return nil, AssertError("all UTXO sources are unavailable")
	}
	return reader, nil
}
