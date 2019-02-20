package blockchain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"

	"github.com/avast/retry-go"
	"github.com/btcsuite/go-socks/socks"
	"github.com/cavaliercoder/grab"
)

const numWorkers = 8

// fastSyncUtxoSet will download the UTXO set from the sources provided in the checkpoint. Each
// UTXO will be saved to the database and the ECMH hash of the UTXO set will be validated against
// the checkpoint. If a proxyAddr is provided it will use that proxy for the HTTP connection.
func (b *BlockChain) fastSyncUtxoSet(checkpoint *chaincfg.Checkpoint, proxyAddr string) error {
	// If the UTXO set is already caught up with the last checkpoint then
	// we can just close the done chan and exit.
	if b.utxoCache.lastFlushHash.IsEqual(checkpoint.Hash) {
		close(b.fastSyncDone)
		return nil
	}

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

	tmpPath := os.TempDir()
	fileName, err := downloadUtxoSet(checkpoint.UtxoSetSources, proxy, tmpPath)
	if err != nil {
		log.Errorf("Error downloading UTXO set: %s", err.Error())
		return err
	}
	file, err := os.Open(fileName)
	if err != nil {
		log.Errorf("Error opening temp UTXO file: %s", err.Error())
		return err
	}

	defer func() {
		file.Close()
		os.Remove(tmpPath)
	}()

	var (
		maxScriptLen   = uint32(1000000)
		buf52          = make([]byte, 52)
		pkScript       []byte
		serializedUtxo []byte
		n              int
		totalRead      int
		scriptLen      uint32
		progress       float64
		progressStr    string
	)

	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			progress = math.Min(float64(totalRead)/float64(checkpoint.UtxoSetSize), 1.0) * 100
			progressStr = fmt.Sprintf("%d/%d MiB (%.2f%%)", totalRead/(1024*1024)+1, checkpoint.UtxoSetSize/(1024*1024)+1, progress)
			log.Infof("UTXO verification progress: processed %s", progressStr)
		}
	}()

	resultsChan := make(chan *result)
	jobsChan := make(chan []byte)
	for i := 0; i < numWorkers; i++ {
		go worker(b.utxoCache, jobsChan, resultsChan)
	}

	// In this loop we're going read each serialized UTXO off the reader and then
	// pass it off to a worker to deserialize, calculate the ECMH hash, and save
	// to the UTXO cache.
	for {
		// Read the first 52 bytes of the utxo
		n, err = file.Read(buf52)
		if err == io.EOF { // We've hit the end
			break
		} else if err != nil {
			log.Errorf("Error reading UTXO set: %s", err.Error())
			return err
		}
		totalRead += n

		// The last four bytes that we read is the length of the script
		scriptLen = binary.LittleEndian.Uint32(buf52[48:])
		if scriptLen > maxScriptLen {
			log.Error("Read invalid UTXO script length", totalRead)
			return errors.New("invalid script length")
		}

		// Read the script
		pkScript = make([]byte, scriptLen)
		n, err = file.Read(pkScript)
		if err != nil {
			log.Errorf("Error reading UTXO set: %s", err.Error())
			return err
		}
		totalRead += n

		serializedUtxo = make([]byte, 52+scriptLen)
		serializedUtxo = append(buf52, pkScript...)

		jobsChan <- serializedUtxo
	}
	close(jobsChan)

	// Read each result and add the returned hash to the
	// existing multiset.
	m := bchec.NewMultiset(bchec.S256())
	for i := 0; i < numWorkers; i++ {
		result := <-resultsChan
		if result.err != nil {
			log.Errorf("Error processing UTXO set: %s", err.Error())
			return err
		}
		m.Merge(result.m)
	}
	close(resultsChan)

	if err = b.utxoCache.Flush(FlushRequired, &BestState{Hash: *checkpoint.Hash}); err != nil {
		log.Errorf("Error processing UTXO set: %s", err.Error())
		return err
	}

	if err = b.index.flushToDB(); err != nil {
		log.Errorf("Error processing UTXO set: %s", err.Error())
		return err
	}

	utxoHash := m.Hash()

	// Make sure the hash of the UTXO set we downloaded matches the expected hash.
	if !checkpoint.UtxoSetHash.IsEqual(&utxoHash) {
		log.Errorf("Downloaded UTXO set hash does not match checkpoint."+
			" Expected %s, got %s.", checkpoint.UtxoSetHash.String(), m.Hash().String())
		return AssertError("downloaded invalid UTXO set")
	}

	log.Infof("Verification complete. UTXO hash %s.", m.Hash().String())

	// Signal fastsync complete
	close(b.fastSyncDone)

	return nil
}

// result holds a multiset with a hash of all the UTXOs read by
// this worker and a possible error.
type result struct {
	m   *bchec.Multiset
	err error
}

// worker handles the work of deserializing the UTXO, calculating the ECMH hash of
// each serialized UTXO as well as saving it into the utxoCache. The resulting
// multiset or an error will be returned over the results chan when the jobs
// chan is closed.
func worker(cache *utxoCache, jobs <-chan []byte, results chan<- *result) {
	var (
		err      error
		m        = bchec.NewMultiset(bchec.S256())
		entry    *UtxoEntry
		outpoint *wire.OutPoint
		state    = &BestState{Hash: chainhash.Hash{}}
	)
	for serializedUtxo := range jobs {
		m.Add(serializedUtxo)

		outpoint, entry, err = deserializeUtxoCommitmentFormat(serializedUtxo)
		if err != nil {
			log.Errorf("Error deserializing UTXO: %s", err.Error())
			results <- &result{err: err}
			return
		}

		if err = cache.AddEntry(*outpoint, entry, true); err != nil {
			results <- &result{err: err}
			return
		}

		if err = cache.Flush(FlushIfNeeded, state); err != nil {
			results <- &result{err: err}
			return
		}
	}
	results <- &result{m: m}
}

// downloadUtxoSet will attempt to connect to make an HTTP GET request to the
// provided sources one at a time and download the UTXO set to the provided path.
// If a proxy is provided it will use it for the HTTP connection.
func downloadUtxoSet(sources []string, proxy *socks.Proxy, pth string) (string, error) {
	var fileName string

	for _, src := range sources {
		log.Infof("Downloading UTXO set from %s", src)
		retry.Do(
			func() error {
				resp, err := grab.Get(pth, src)
				if err != nil {
					if strings.Contains(err.Error(), "connection refused") {
						return nil
					}
					return err
				}

				fileName = resp.Filename
				log.Info("UTXO download complete. Verifying integrity...")

				return nil
			},
			retry.Attempts(3),
		)
		if fileName != "" {
			return fileName, nil
		}
	}

	return "", AssertError("all UTXO sources are unavailable")
}
