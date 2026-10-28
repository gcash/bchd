package bchrpc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/mempool"
	"github.com/gcash/bchd/mining"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"github.com/gcash/goslp"
	"github.com/gcash/goslp/metadatamaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestSlpCallsWithoutSlpIndex ensures the SLP calls return Unavailable
// instead of panicking when the SLP index is not enabled.
func TestSlpCallsWithoutSlpIndex(t *testing.T) {
	s := &GrpcServer{}
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"GetSlpGraphSearch", func() error {
			_, err := s.GetSlpGraphSearch(ctx,
				&pb.GetSlpGraphSearchRequest{Hash: make([]byte, 32)})
			return err
		}},
		{"GetSlpTokenMetadata", func() error {
			_, err := s.GetSlpTokenMetadata(ctx,
				&pb.GetSlpTokenMetadataRequest{TokenIds: [][]byte{make([]byte, 32)}})
			return err
		}},
		{"GetSlpTrustedValidation", func() error {
			_, err := s.GetSlpTrustedValidation(ctx,
				&pb.GetSlpTrustedValidationRequest{
					Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
						PrevOutHash: make([]byte, 32),
					}},
					IncludeGraphsearchCount: true,
				})
			return err
		}},
		{"CheckSlpTransaction", func() error {
			_, err := s.CheckSlpTransaction(ctx,
				&pb.CheckSlpTransactionRequest{})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s panicked: %v", test.name, r)
				}
			}()
			if got := status.Code(test.call()); got != codes.Unavailable {
				t.Fatalf("got code %v, want %v", got, codes.Unavailable)
			}
		})
	}
}

// TestGetSlpTokenIDNotGenesis ensures getSlpToken returns an error instead
// of panicking for an output of a valid SEND whose token ID names another
// SEND.  A SEND of zero tokens that spends no token inputs is valid whatever
// token ID it names.
func TestGetSlpTokenIDNotGenesis(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	s := &GrpcServer{db: db, slpIndex: idx}

	// zeroSend returns a transaction whose first output sends zero tokens
	// of tokenID.
	zeroSend := func(tokenID []byte) *wire.MsgTx {
		script, err := metadatamaker.TokenType1Send(tokenID, []uint64{0})
		if err != nil {
			t.Fatalf("TokenType1Send: %v", err)
		}
		tx := wire.NewMsgTx(1)
		tx.AddTxOut(&wire.TxOut{PkScript: script})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		return tx
	}

	sendA := zeroSend(bytes.Repeat([]byte{0x01}, 32))
	// A token ID is written in the byte order a transaction hash is
	// displayed in.
	tokenIDA, err := hex.DecodeString(sendA.TxHash().String())
	if err != nil {
		t.Fatal(err)
	}
	sendB := zeroSend(tokenIDA)

	for _, tx := range []*wire.MsgTx{sendA, sendB} {
		err := db.View(func(dbTx database.Tx) error {
			valid, err := idx.AddPotentialSlpEntries(dbTx, tx)
			if err == nil && !valid {
				err = fmt.Errorf("SEND %v is not valid", tx.TxHash())
			}
			return err
		})
		if err != nil {
			t.Fatalf("AddPotentialSlpEntries: %v", err)
		}
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("getSlpToken panicked: %v", r)
		}
	}()
	hashB := sendB.TxHash()
	if _, err := s.getSlpToken(&hashB, 1, nil); err == nil {
		t.Fatal("getSlpToken returned a token whose token ID is not a GENESIS, want an error")
	}
}

// TestTokenMetadataNotGenesis ensures metadata asked for a token ID that names
// no GENESIS, as a valid SEND of zero tokens may, is reported as missing
// rather than as an error of the node.
func TestTokenMetadataNotGenesis(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	s := &GrpcServer{db: db, slpIndex: idx}

	script, err := metadatamaker.TokenType1Send(bytes.Repeat([]byte{0x01}, 32),
		[]uint64{0})
	if err != nil {
		t.Fatalf("TokenType1Send: %v", err)
	}
	send := wire.NewMsgTx(1)
	send.AddTxOut(&wire.TxOut{PkScript: script})
	send.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	err = db.View(func(dbTx database.Tx) error {
		_, err := idx.AddPotentialSlpEntries(dbTx, send)
		return err
	})
	if err != nil {
		t.Fatalf("AddPotentialSlpEntries: %v", err)
	}

	// The SEND's own hash is a token ID naming no GENESIS.
	if tm := s.tokenMetadataOrNil(send.TxHash()); tm != nil {
		t.Fatalf("got metadata %v for a token ID naming no GENESIS", tm)
	}
	// One naming no transaction at all.
	if tm := s.tokenMetadataOrNil(chainhash.Hash{0x09}); tm != nil {
		t.Fatalf("got metadata %v for an unknown token ID", tm)
	}
}

// newGraphSearchServer returns a server whose slp index holds a GENESIS from
// the memory pool, with graph search loaded and ready, and that GENESIS.
func newGraphSearchServer(t *testing.T) (*GrpcServer, *wire.MsgTx) {
	t.Helper()
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100,
		SlpGraphSearchEnabled: true})
	if err := db.Update(idx.Create); err != nil {
		t.Fatalf("Create: %v", err)
	}

	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatalf("TokenType1Genesis: %v", err)
	}
	genesis := wire.NewMsgTx(1)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{1}}, nil))
	genesis.AddTxOut(&wire.TxOut{PkScript: script})
	genesis.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	err = db.View(func(dbTx database.Tx) error {
		valid, err := idx.AddPotentialSlpEntries(dbTx, genesis)
		if err == nil && !valid {
			err = errors.New("the GENESIS is not valid")
		}
		return err
	})
	if err != nil {
		t.Fatalf("AddPotentialSlpEntries: %v", err)
	}

	fetch := func(hashes []chainhash.Hash) ([][]byte, error) {
		var buf bytes.Buffer
		err := genesis.Serialize(&buf)
		txns := make([][]byte, len(hashes))
		for i := range txns {
			txns[i] = buf.Bytes()
		}
		return txns, err
	}
	idx.LoadSlpGraphSearchDb(fetch, nil, nil)
	idx.AddGraphSearchTxn(genesis)
	if _, err := idx.GetGraphSearchDb(); err != nil {
		t.Fatalf("GetGraphSearchDb: %v", err)
	}
	return &GrpcServer{db: db, slpIndex: idx, txIndex: indexers.NewTxIndex(db)},
		genesis
}

// TestSlpGraphSearchLogLevel ensures the lines that GetSlpGraphSearch and the
// graph search count of GetSlpTrustedValidation log for each call are at
// debug, so that at the default level the log does not grow with every call.
func TestSlpGraphSearchLogLevel(t *testing.T) {
	s, genesis := newGraphSearchServer(t)
	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("GRPC")
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)

	ctx := context.Background()
	hash := genesis.TxHash()
	calls := func() {
		t.Helper()
		_, err := s.GetSlpGraphSearch(ctx,
			&pb.GetSlpGraphSearchRequest{Hash: hash[:]})
		if err != nil {
			t.Fatalf("GetSlpGraphSearch: %v", err)
		}
		_, err = s.GetSlpTrustedValidation(ctx,
			&pb.GetSlpTrustedValidationRequest{
				Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
					PrevOutHash: hash[:],
					PrevOutVout: 1,
				}},
				IncludeGraphsearchCount: true,
			})
		if err != nil {
			t.Fatalf("GetSlpTrustedValidation: %v", err)
		}
	}

	logger.SetLevel(bchlog.LevelInfo)
	calls()
	if buf.Len() != 0 {
		t.Errorf("calls logged at the info level: %q", buf.String())
	}

	logger.SetLevel(bchlog.LevelDebug)
	calls()
	for _, want := range []string{
		"[DBG] GRPC: SLP graph search returned 1 transactions for txid " +
			hash.String(),
		"[DBG] GRPC: SLP graph search count is 1 transactions for txid " +
			hash.String(),
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("debug log %q, want a line with %q", buf.String(), want)
		}
	}
}

// TestSlpTrustedValidationBadValidHash ensures a graph search valid hash of
// the wrong length is refused as an invalid argument, with a message that
// does not repeat the input, rather than as an internal error that is logged
// in full at the error level.
func TestSlpTrustedValidationBadValidHash(t *testing.T) {
	s, genesis := newGraphSearchServer(t)
	hash := genesis.TxHash()
	_, err := s.GetSlpTrustedValidation(context.Background(),
		&pb.GetSlpTrustedValidationRequest{
			Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
				PrevOutHash:            hash[:],
				PrevOutVout:            1,
				GraphsearchValidHashes: [][]byte{make([]byte, 100000)},
			}},
			IncludeGraphsearchCount: true,
		})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %.100v, want InvalidArgument", err)
	}
	want := "invalid graph search valid hash: invalid hash length of 100000, " +
		"want 32"
	if got := status.Convert(err).Message(); got != want {
		t.Fatalf("got message %.100q, want %q", got, want)
	}
}

// TestZeroSendNoGenesisLogLevel ensures showing a SEND of zero tokens of a
// token that has no GENESIS, which is valid, logs at debug rather than as
// critical, for a type 1 and an NFT1 child token, in the memory pool and
// confirmed.  Its outputs then carry no token.
func TestZeroSendNoGenesisLogLevel(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	s := &GrpcServer{chainParams: &chaincfg.RegressionNetParams, db: db,
		slpIndex: idx}

	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("GRPC")
	logger.SetLevel(bchlog.LevelInfo)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)

	// show checks the transaction as the SLP calls show it.
	show := func(name string, tx *wire.MsgTx) {
		t.Helper()
		buf.Reset()
		resp := marshalTransaction(bchutil.NewTx(tx), 0, nil, 0, s)
		if got := resp.SlpTransactionInfo.ValidityJudgement; got !=
			pb.SlpTransactionInfo_VALID {

			t.Errorf("%s: the SEND is %v, want valid", name, got)
		}
		if resp.Outputs[1].SlpToken != nil {
			t.Errorf("%s: output 1 has token %v", name,
				resp.Outputs[1].SlpToken)
		}
		if buf.Len() != 0 {
			t.Errorf("%s: logged at the info level or above: %q", name,
				buf.String())
		}
	}
	n := byte(0)
	zeroSend := func(send func([]byte, []uint64) ([]byte, error),
		tokenID []byte) *wire.MsgTx {

		t.Helper()
		script, err := send(tokenID, []uint64{0})
		if err != nil {
			t.Fatal(err)
		}
		n++
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee, n}}, nil))
		tx.AddTxOut(&wire.TxOut{PkScript: script})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		return tx
	}
	for i, typ := range []struct {
		name string
		send func([]byte, []uint64) ([]byte, error)
	}{
		{"type 1", metadatamaker.TokenType1Send},
		{"nft1 child", metadatamaker.NFT1ChildSend},
	} {
		// A token ID that names no transaction.
		unknown := bytes.Repeat([]byte{0x70 + byte(i)}, 32)

		pooled := zeroSend(typ.send, unknown)
		err := db.View(func(dbTx database.Tx) error {
			valid, err := idx.AddPotentialSlpEntries(dbTx, pooled)
			if err == nil && !valid {
				err = errors.New("the SEND is not valid")
			}
			return err
		})
		if err != nil {
			t.Fatalf("%s: AddPotentialSlpEntries: %v", typ.name, err)
		}
		show(typ.name+" in the memory pool", pooled)

		confirmed := zeroSend(typ.send, unknown)
		block := bchutil.NewBlock(&wire.MsgBlock{
			Transactions: []*wire.MsgTx{confirmed}})
		err = db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, block, nil)
		})
		if err != nil {
			t.Fatalf("%s: ConnectBlock: %v", typ.name, err)
		}
		show(typ.name+" confirmed", confirmed)
	}
}

// TestNftChildGenesisNoGroupLogLevel ensures an NFT1 child GENESIS whose token
// metadata has no group, which only an inconsistent index has, is still
// logged as critical when shown.
func TestNftChildGenesisNoGroupLogLevel(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s := &GrpcServer{chainParams: &chaincfg.RegressionNetParams, db: db,
		slpIndex: idx}

	groupScript, err := metadatamaker.NFT1GroupGenesis([]byte("G"),
		[]byte("Group"), nil, nil, 0, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	group := wire.NewMsgTx(1)
	group.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee}}, nil))
	group.AddTxOut(&wire.TxOut{PkScript: groupScript})
	group.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	childScript, err := metadatamaker.NFT1ChildGenesis([]byte("C"),
		[]byte("Child"), nil, nil, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	child := wire.NewMsgTx(1)
	child.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: group.TxHash(), Index: 1}, nil))
	child.AddTxOut(&wire.TxOut{PkScript: childScript})
	child.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	block := bchutil.NewBlock(&wire.MsgBlock{
		Transactions: []*wire.MsgTx{group, child}})

	// Connect both, then store the child token's metadata without its
	// group: its token ID and token type only.
	childHash := child.TxHash()
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.ConnectBlock(dbTx, block, nil); err != nil {
			return err
		}
		entry, err := idx.GetSlpIndexEntry(dbTx, &childHash)
		if err != nil {
			return err
		}
		var key [4]byte
		binary.LittleEndian.PutUint32(key[:], entry.TokenID)
		value := make([]byte, 34)
		copy(value, entry.TokenIDHash[:])
		binary.LittleEndian.PutUint16(value[32:], 0x41)
		return dbTx.Metadata().Bucket([]byte("tokenhashbyididx")).Put(key[:],
			value)
	})
	if err != nil {
		t.Fatalf("index: %v", err)
	}

	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("GRPC")
	logger.SetLevel(bchlog.LevelInfo)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)
	marshalTransaction(bchutil.NewTx(child), 0, nil, 0, s)
	want := "[CRT] GRPC: missing group id in token metadata for nft child " +
		childHash.String()
	if !strings.Contains(buf.String(), want) {
		t.Errorf("log %q, want a line with %q", buf.String(), want)
	}
}

// waitFor waits up to 5 seconds for cond to hold.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSlpMempoolMintLeftPool ensures the token metadata a MINT set while in
// the memory pool is dropped at the next block once the MINT has left the
// memory pool without being confirmed, so that the mint baton is reported
// where the chain has it rather than on an output that does not exist.
func TestSlpMempoolMintLeftPool(t *testing.T) {
	s := newDispatcherTestServer(t)
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.chainParams = &chaincfg.RegressionNetParams
	s.db, s.slpIndex = db, idx
	// The memory pool does not hold the MINT, as when a conflicting
	// transaction was confirmed or the MINT expired.
	s.txMemPool = mempool.New(&mempool.Config{})
	// The server's cleanup waits for the handler, which ends once the
	// server quits, as it does for the server's own goroutines.
	s.wg.Go(s.slpEventHandler)
	waitFor(t, "the slp event handler to subscribe", func() bool {
		return s.numSubscriptions() == 1
	})

	batonTx := func(prev wire.OutPoint, script []byte, err error) *wire.MsgTx {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&prev, nil))
		tx.AddTxOut(&wire.TxOut{PkScript: script})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		return tx
	}
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, metadatamaker.NewMintBatonVout(2), 1000)
	genesis := batonTx(wire.OutPoint{Hash: chainhash.Hash{0xee}}, script, err)
	id, err := goslp.GetSlpTokenID(genesis)
	if err != nil {
		t.Fatal(err)
	}
	token, err := chainhash.NewHash(id)
	if err != nil {
		t.Fatal(err)
	}
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{genesis}})
	err = db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	})
	if err != nil {
		t.Fatalf("ConnectBlock: %v", err)
	}
	script, err = metadatamaker.TokenType1Mint(token[:],
		metadatamaker.NewMintBatonVout(2), 10)
	mint := batonTx(wire.OutPoint{Hash: genesis.TxHash(), Index: 2}, script, err)

	baton := func() wire.OutPoint {
		t.Helper()
		var tm *indexers.TokenMetadata
		err := db.View(func(dbTx database.Tx) error {
			var err error
			tm, err = idx.GetTokenMetadata(dbTx,
				&indexers.SlpTxEntry{TokenIDHash: *token})
			return err
		})
		if err != nil || tm.MintBatonHash == nil {
			t.Fatalf("token metadata %v, %v", tm, err)
		}
		return wire.OutPoint{Hash: *tm.MintBatonHash, Index: tm.MintBatonVout}
	}
	s.dispatchEvent(&rpcEventTxAccepted{TxDesc: &mempool.TxDesc{
		TxDesc: mining.TxDesc{Tx: bchutil.NewTx(mint)}}})
	waitFor(t, "the baton on the MINT", func() bool {
		return baton() == wire.OutPoint{Hash: mint.TxHash(), Index: 2}
	})

	s.dispatchEvent(&rpcEventBlockConnected{Block: bchutil.NewBlock(&wire.MsgBlock{})})
	waitFor(t, "the baton back on the GENESIS", func() bool {
		return baton() == wire.OutPoint{Hash: genesis.TxHash(), Index: 2}
	})
}

// TestSlpBlockEventPoolDeadlock ensures the slp event handler, which asks the
// memory pool on each block event whether the transactions in the slp cache
// are still there, cannot deadlock with block connection and a submission.  A
// submission, as SubmitTransaction and sendrawtransaction make, holds the
// memory pool's lock while it waits for the chain lock, and block connection
// holds the chain lock while it takes the slp cache lock, so the handler must
// not hold the slp cache lock while it waits for the memory pool's.  The test
// plays only the chain lock, around the slp index's ConnectBlock.
func TestSlpBlockEventPoolDeadlock(t *testing.T) {
	s := newDispatcherTestServer(t)
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.chainParams = &chaincfg.RegressionNetParams
	s.db, s.slpIndex = db, idx

	// Block connection holds the chain lock until chainUnlocked is closed,
	// and the memory pool waits for it to read the outputs a transaction
	// spends.  The wait also ends on abort, so that a deadlock found can
	// be undone.
	chainUnlocked := make(chan struct{})
	abort := make(chan struct{})
	stop := sync.OnceFunc(func() { close(abort) })
	t.Cleanup(stop)
	poolWaits := make(chan struct{})
	s.txMemPool = mempool.New(&mempool.Config{
		Policy:         mempool.Policy{AcceptNonStd: true, MaxTxVersion: 2},
		ChainParams:    &chaincfg.RegressionNetParams,
		BestHeight:     func() int32 { return 1 },
		MedianTimePast: time.Now,
		FetchUtxoView: func(*bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			close(poolWaits)
			select {
			case <-chainUnlocked:
			case <-abort:
			}
			return nil, errors.New("the test's chain has no outputs")
		},
	})
	// The server's cleanup waits for the handler, which ends once the
	// server quits, as it does for the server's own goroutines.
	s.wg.Go(s.slpEventHandler)
	waitFor(t, "the slp event handler to subscribe", func() bool {
		return s.numSubscriptions() == 1
	})

	// The handler puts a GENESIS that the memory pool does not hold in the
	// slp cache, as one that has left it.
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	genesis := wire.NewMsgTx(1)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee}}, nil))
	genesis.AddTxOut(&wire.TxOut{PkScript: script})
	genesis.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	id, err := goslp.GetSlpTokenID(genesis)
	if err != nil {
		t.Fatal(err)
	}
	token, err := chainhash.NewHash(id)
	if err != nil {
		t.Fatal(err)
	}
	// cached reports whether the slp cache has the token metadata the
	// GENESIS set, which the index's database does not have.
	cached := func() bool {
		return db.View(func(dbTx database.Tx) error {
			_, err := idx.GetTokenMetadata(dbTx,
				&indexers.SlpTxEntry{TokenIDHash: *token})
			return err
		}) == nil
	}
	s.dispatchEvent(&rpcEventTxAccepted{TxDesc: &mempool.TxDesc{
		TxDesc: mining.TxDesc{Tx: bchutil.NewTx(genesis)}}})
	waitFor(t, "the GENESIS in the slp cache", cached)

	// A submission takes the memory pool's lock and waits for the chain
	// lock.
	spend := wire.NewMsgTx(1)
	spend.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xdd}},
		bytes.Repeat([]byte{txscript.OP_TRUE}, 100)))
	spend.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})
	submitted := make(chan error, 1)
	go func() {
		_, err := s.txMemPool.ProcessTransaction(bchutil.NewTx(spend), false, 0)
		submitted <- err
	}()
	select {
	case <-poolWaits:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the submission to wait for the chain lock")
	}

	// The handler gets the event of the block connected before the one
	// being connected, and asks the memory pool about the GENESIS.
	s.dispatchEvent(&rpcEventBlockConnected{Block: bchutil.NewBlock(&wire.MsgBlock{})})
	waitFor(t, "the slp event handler to ask the memory pool", func() bool {
		var stacks bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&stacks, 2)
		return strings.Contains(stacks.String(), "(*TxPool).IsTransactionInPool")
	})

	// Block connection reaches the slp index, which looks up the inputs of
	// a transaction that is not slp in the slp cache.
	other := wire.NewMsgTx(1)
	other.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xcc}}, nil))
	other.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{other}})
	connected := make(chan error, 1)
	go func() {
		connected <- db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, block, nil)
		})
		close(chainUnlocked)
	}()
	select {
	case err = <-connected:
	case <-time.After(5 * time.Second):
		t.Error("deadlock: block connection waits for the slp cache lock, " +
			"which the slp event handler holds while it waits for the " +
			"memory pool's lock, which the submission holds while it " +
			"waits for the chain lock")
		stop()
		err = <-connected
	}
	if err != nil {
		t.Errorf("ConnectBlock: %v", err)
	}
	if err := <-submitted; err == nil {
		t.Error("the submission was accepted")
	}
	waitFor(t, "the GENESIS to leave the slp cache", func() bool {
		return !cached()
	})
}

// TestSlpGraphSearchCallEnds ensures GetSlpGraphSearch, and the graph search
// count of GetSlpTrustedValidation, stop once their call's context ends, as
// when the client goes away or its deadline passes, and answer with the
// context's status, as a search can take the whole of a large token's graph.
func TestSlpGraphSearchCallEnds(t *testing.T) {
	s, genesis := newGraphSearchServer(t)
	hash := genesis.TxHash()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(),
		time.Now().Add(-time.Second))
	defer stop()
	for _, test := range []struct {
		name string
		ctx  context.Context
		want codes.Code
	}{
		{"canceled", canceled, codes.Canceled},
		{"deadline passed", expired, codes.DeadlineExceeded},
	} {
		_, err := s.GetSlpGraphSearch(test.ctx,
			&pb.GetSlpGraphSearchRequest{Hash: hash[:]})
		if status.Code(err) != test.want {
			t.Errorf("GetSlpGraphSearch, %s: got %v, want %v", test.name,
				err, test.want)
		}
		_, err = s.GetSlpTrustedValidation(test.ctx,
			&pb.GetSlpTrustedValidationRequest{
				Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
					PrevOutHash: hash[:],
					PrevOutVout: 1,
				}},
				IncludeGraphsearchCount: true,
			})
		if status.Code(err) != test.want {
			t.Errorf("GetSlpTrustedValidation, %s: got %v, want %v",
				test.name, err, test.want)
		}
	}
}

// failingViewDB is a database whose read transactions fail with err.
type failingViewDB struct {
	database.DB
	err error
}

func (db *failingViewDB) View(func(database.Tx) error) error {
	return db.err
}

// TestFetchGraphSearchTxns ensures slp graph search reads a memory pool
// transaction, which its graph holds before a block does, from the memory
// pool, also once a block holds it, and the others from their blocks, all in
// one database transaction and without the chain, as the transaction index
// holds only the blocks of the main chain, in the order asked for.  A
// transaction in neither is not found, by its hash, and the database failing
// is a load error that leaves out the database's own, which can name the
// node's files, and which is logged as an error instead.
func TestFetchGraphSearchTxns(t *testing.T) {
	pool, tx1, _, spend, funding := newDSProofPool(t)
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.MainNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	txIndex := indexers.NewTxIndex(db)
	if err := db.Update(txIndex.Create); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// A block holds two transactions outside the memory pool and tx1.
	confirmed := spend(wire.OutPoint{Hash: funding.TxHash(), Index: 1},
		100000000, 1000)
	other := spend(wire.OutPoint{Hash: confirmed.TxHash()}, 99999000, 1000)
	block := bchutil.NewBlock(&wire.MsgBlock{
		Transactions: []*wire.MsgTx{confirmed, other, tx1}})
	err = db.Update(func(dbTx database.Tx) error {
		if err := dbTx.StoreBlock(block); err != nil {
			return err
		}
		return txIndex.ConnectBlock(dbTx, block, nil)
	})
	if err != nil {
		t.Fatalf("connecting the block: %v", err)
	}
	counted := &viewCountingDB{DB: db}
	s := &GrpcServer{db: counted, txMemPool: pool, txIndex: txIndex}

	for _, test := range []struct {
		name  string
		txs   []*wire.MsgTx
		views int32
	}{
		{"the memory pool transaction", []*wire.MsgTx{tx1}, 0},
		{"all three", []*wire.MsgTx{other, tx1, confirmed}, 1},
	} {
		hashes := make([]chainhash.Hash, len(test.txs))
		for i, tx := range test.txs {
			hashes[i] = tx.TxHash()
		}
		counted.views.Store(0)
		got, err := s.fetchGraphSearchTxns(hashes)
		if err != nil || len(got) != len(test.txs) {
			t.Fatalf("%s: got %d transactions and %v, want %d", test.name,
				len(got), err, len(test.txs))
		}
		for i, tx := range test.txs {
			var want bytes.Buffer
			if err := tx.Serialize(&want); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got[i], want.Bytes()) {
				t.Errorf("%s: transaction %d is %x, want %x", test.name, i,
					got[i], want.Bytes())
			}
		}
		if views := counted.views.Load(); views != test.views {
			t.Errorf("%s: read in %d database transactions, want %d",
				test.name, views, test.views)
		}
	}

	// Asking the memory pool about confirmed transactions allocates
	// nothing: reading one more allocates as much as reading one more
	// block region does.
	read := func(hashes ...chainhash.Hash) func() {
		return func() {
			_ = db.View(func(dbTx database.Tx) error {
				regions := make([]database.BlockRegion, len(hashes))
				for i := range hashes {
					region, err := txIndex.TxBlockRegionIn(dbTx, &hashes[i])
					if err != nil {
						return err
					}
					regions[i] = *region
				}
				_, err := dbTx.FetchBlockRegions(regions)
				return err
			})
		}
	}
	fetch := func(hashes ...chainhash.Hash) func() {
		return func() { _, _ = s.fetchGraphSearchTxns(hashes) }
	}
	one, two := confirmed.TxHash(), other.TxHash()
	readMore := testing.AllocsPerRun(10, read(one, two)) -
		testing.AllocsPerRun(10, read(one))
	fetchMore := testing.AllocsPerRun(10, fetch(one, two)) -
		testing.AllocsPerRun(10, fetch(one))
	if fetchMore > readMore {
		t.Errorf("reading one more confirmed transaction made %v "+
			"allocations, one more block region %v", fetchMore, readMore)
	}

	// The transaction tx1 spends is in no block the index holds.
	counted.views.Store(0)
	missing := funding.TxHash()
	got, err := s.fetchGraphSearchTxns([]chainhash.Hash{one, missing, two})
	var notFound errGraphSearchTxnNotFound
	if !errors.As(err, &notFound) || chainhash.Hash(notFound) != missing ||
		got != nil {

		t.Errorf("a transaction in neither: got %d transactions and %v, "+
			"want it not found", len(got), err)
	}
	if views := counted.views.Load(); views != 1 {
		t.Errorf("a transaction in neither: read in %d database "+
			"transactions, want 1", views)
	}

	// The index entry of the confirmed transaction runs past the end of
	// its block.
	err = db.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket([]byte("txbyhashidx"))
		entry := bytes.Clone(bucket.Get(one[:]))
		binary.LittleEndian.PutUint32(entry[8:], 1<<20)
		return bucket.Put(one[:], entry)
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.fetchGraphSearchTxns([]chainhash.Hash{two, one})
	if err != errGraphSearchTxnLoad || got != nil {
		t.Errorf("a region past the end of its block: got %d transactions "+
			"and %v, want %v", len(got), err, errGraphSearchTxnLoad)
	}

	failed := "read /home/node/.bchd/data/mainnet/blocks_ffldb/000000001.fdb: " +
		"input/output error"
	s.db = &failingViewDB{DB: db, err: errors.New(failed)}
	var logged bytes.Buffer
	saved := log
	UseLogger(bchlog.NewBackend(&logged).Logger("GRPC"))
	defer UseLogger(saved)
	got, err = s.fetchGraphSearchTxns([]chainhash.Hash{two})
	if err != errGraphSearchTxnLoad || got != nil {
		t.Errorf("the database failing: got %d transactions and %v, want %v",
			len(got), err, errGraphSearchTxnLoad)
	}
	if want := "[ERR] GRPC: Failed to load graph search transactions: " +
		failed; !strings.Contains(logged.String(), want) {

		t.Errorf("the database failing: log %q, want a line with %q",
			logged.String(), want)
	}
}

// TestSlpGraphSearchTxnNotFound ensures GetSlpGraphSearch answers NotFound,
// naming the transaction, when a transaction of the search is in neither the
// memory pool nor the transaction index, as one a block confirms is until the
// block is committed, and Internal when the index fails, with a message that
// leaves out the database's own error, which can name the node's files.
func TestSlpGraphSearchTxnNotFound(t *testing.T) {
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100,
		SlpGraphSearchEnabled: true})
	txIndex := indexers.NewTxIndex(db)
	for _, create := range []func(database.Tx) error{idx.Create, txIndex.Create} {
		if err := db.Update(create); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatalf("TokenType1Genesis: %v", err)
	}
	genesis := wire.NewMsgTx(1)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{1}}, nil))
	genesis.AddTxOut(&wire.TxOut{PkScript: script})
	genesis.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	err = db.View(func(dbTx database.Tx) error {
		valid, err := idx.AddPotentialSlpEntries(dbTx, genesis)
		if err == nil && !valid {
			err = errors.New("the GENESIS is not valid")
		}
		return err
	})
	if err != nil {
		t.Fatalf("AddPotentialSlpEntries: %v", err)
	}

	// The memory pool does not hold the GENESIS, and neither does the
	// transaction index.
	s := &GrpcServer{db: db, slpIndex: idx, txIndex: txIndex,
		txMemPool: mempool.New(&mempool.Config{})}
	idx.LoadSlpGraphSearchDb(s.fetchGraphSearchTxns, nil, nil)
	idx.AddGraphSearchTxn(genesis)
	if _, err := idx.GetGraphSearchDb(); err != nil {
		t.Fatalf("GetGraphSearchDb: %v", err)
	}
	hash := genesis.TxHash()
	req := &pb.GetSlpGraphSearchRequest{Hash: hash[:]}
	_, err = s.GetSlpGraphSearch(context.Background(), req)
	want := "transaction " + hash.String() + " of the graph search is in " +
		"neither the memory pool nor the transaction index"
	if status.Code(err) != codes.NotFound || status.Convert(err).Message() != want {
		t.Errorf("got %v, want NotFound with %q", err, want)
	}

	// The transaction index entry of the GENESIS is cut short.
	err = db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().Bucket([]byte("txbyhashidx")).Put(hash[:],
			[]byte{1})
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.GetSlpGraphSearch(context.Background(), req)
	msg := status.Convert(err).Message()
	if status.Code(err) != codes.Internal ||
		!strings.Contains(msg, errGraphSearchTxnLoad.Error()) ||
		strings.Contains(msg, "corrupt") || strings.Contains(msg, "rpc error") {

		t.Errorf("got %v, want Internal with %q and not the database's "+
			"error", err, errGraphSearchTxnLoad)
	}
}

// newOpTruePool returns a memory pool that takes nonstandard transactions
// and a confirmed transaction it spends the outputs of, two of 100,000,000
// satoshis each that anyone can spend.
func newOpTruePool(t *testing.T) (*mempool.TxPool, *wire.MsgTx) {
	t.Helper()
	funding := wire.NewMsgTx(1)
	funding.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee}}, nil))
	for range 2 {
		funding.AddTxOut(&wire.TxOut{Value: 100000000,
			PkScript: []byte{txscript.OP_TRUE}})
	}
	utxos := blockchain.NewUtxoViewpoint()
	utxos.AddTxOuts(bchutil.NewTx(funding), 1)
	pool := mempool.New(&mempool.Config{
		Policy:      mempool.Policy{AcceptNonStd: true, MaxTxVersion: 2},
		ChainParams: &chaincfg.RegressionNetParams,
		FetchUtxoView: func(tx *bchutil.Tx) (*blockchain.UtxoViewpoint, error) {
			view := blockchain.NewUtxoViewpoint()
			for _, in := range tx.MsgTx().TxIn {
				view.Entries()[in.PreviousOutPoint] =
					utxos.LookupEntry(in.PreviousOutPoint).Clone()
			}
			return view, nil
		},
		BestHeight:     func() int32 { return 1000 },
		MedianTimePast: time.Now,
		CalcSequenceLock: func(*bchutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {
			return &blockchain.SequenceLock{Seconds: -1, BlockHeight: -1}, nil
		},
	})
	return pool, funding
}

// TestSlpGraphSearchFromPool ensures the graph search that the slp event
// handler loads at the first memory pool transaction finds a GENESIS and a
// SEND accepted into the memory pool, and reads them from it, as neither is
// in a block.  The next slp transaction after the load makes graph search
// ready.
func TestSlpGraphSearchFromPool(t *testing.T) {
	s := newDispatcherTestServer(t)
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100,
		SlpGraphSearchEnabled: true})
	txIndex := indexers.NewTxIndex(db)
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		if err := txIndex.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	opTrue := []byte{txscript.OP_TRUE}
	pool, funding := newOpTruePool(t)
	s.chainParams = &chaincfg.RegressionNetParams
	s.db, s.slpIndex, s.txIndex, s.txMemPool = db, idx, txIndex, pool
	go s.slpEventHandler()
	waitFor(t, "the slp event handler to subscribe", func() bool {
		return s.numSubscriptions() == 1
	})

	// accept puts tx in the memory pool, whose event the slp event
	// handler gets.
	accept := func(script []byte, err error, prev ...wire.OutPoint) *wire.MsgTx {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		tx := wire.NewMsgTx(1)
		for i := range prev {
			tx.AddTxIn(wire.NewTxIn(&prev[i], nil))
		}
		tx.AddTxOut(&wire.TxOut{PkScript: script})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: opTrue})
		tx.AddTxOut(&wire.TxOut{Value: 90000000, PkScript: opTrue})
		if _, err := pool.ProcessTransaction(bchutil.NewTx(tx), false, 0); err != nil {
			t.Fatalf("ProcessTransaction: %v", err)
		}
		hash := tx.TxHash()
		desc, err := pool.FetchTxDesc(&hash)
		if err != nil {
			t.Fatal(err)
		}
		s.dispatchEvent(&rpcEventTxAccepted{TxDesc: desc})
		return tx
	}
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	genesis := accept(script, err, wire.OutPoint{Hash: funding.TxHash()})
	waitFor(t, "graph search to load", func() bool {
		gsDb, _ := idx.GetGraphSearchDb()
		return gsDb != nil && gsDb.IsLoaded()
	})

	id, err := goslp.GetSlpTokenID(genesis)
	if err != nil {
		t.Fatal(err)
	}
	script, err = metadatamaker.TokenType1Send(id, []uint64{1000})
	send := accept(script, err, wire.OutPoint{Hash: genesis.TxHash(), Index: 1},
		wire.OutPoint{Hash: genesis.TxHash(), Index: 2})
	hash := send.TxHash()
	var resp *pb.GetSlpGraphSearchResponse
	waitFor(t, "the SEND in graph search", func() bool {
		resp, err = s.GetSlpGraphSearch(context.Background(),
			&pb.GetSlpGraphSearchRequest{Hash: hash[:]})
		return err == nil
	})
	if len(resp.Txdata) != 2 {
		t.Fatalf("the search found %d transactions, want the SEND and the "+
			"GENESIS", len(resp.Txdata))
	}
	for i, tx := range []*wire.MsgTx{send, genesis} {
		var want bytes.Buffer
		if err := tx.Serialize(&want); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(resp.Txdata[i], want.Bytes()) {
			t.Errorf("transaction %d of the search is %x, want %x", i,
				resp.Txdata[i], want.Bytes())
		}
	}
}

// TestSlpTrustedValidationLeftPool ensures an output of a GENESIS that left
// the memory pool without being confirmed is no longer valid to
// GetSlpTrustedValidation once the next block's event comes, rather than
// until the slp cache evicts the GENESIS's entry.
func TestSlpTrustedValidationLeftPool(t *testing.T) {
	s := newDispatcherTestServer(t)
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.chainParams = &chaincfg.RegressionNetParams
	s.db, s.slpIndex = db, idx
	// The memory pool does not hold the GENESIS, as when a conflicting
	// transaction was confirmed or the GENESIS expired.
	s.txMemPool = mempool.New(&mempool.Config{})
	go s.slpEventHandler()
	waitFor(t, "the slp event handler to subscribe", func() bool {
		return s.numSubscriptions() == 1
	})

	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	genesis := wire.NewMsgTx(1)
	genesis.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee}}, nil))
	genesis.AddTxOut(&wire.TxOut{PkScript: script})
	genesis.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	hash := genesis.TxHash()
	validate := func() error {
		_, err := s.GetSlpTrustedValidation(context.Background(),
			&pb.GetSlpTrustedValidationRequest{
				Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
					PrevOutHash: hash[:],
					PrevOutVout: 1,
				}},
			})
		return err
	}
	s.dispatchEvent(&rpcEventTxAccepted{TxDesc: &mempool.TxDesc{
		TxDesc: mining.TxDesc{Tx: bchutil.NewTx(genesis)}}})
	waitFor(t, "the GENESIS to be valid", func() bool { return validate() == nil })

	s.dispatchEvent(&rpcEventBlockConnected{Block: bchutil.NewBlock(&wire.MsgBlock{})})
	waitFor(t, "the GENESIS to be no longer valid", func() bool {
		return status.Code(validate()) == codes.NotFound
	})
}

// TestSlpTrustedValidationDisconnected ensures an output of a GENESIS that a
// reorg disconnects is still valid to GetSlpTrustedValidation once the
// memory pool has taken the GENESIS back, and is no longer valid from the
// disconnection's event when the memory pool has not, rather than until the
// slp cache evicts the GENESIS's entry.
func TestSlpTrustedValidationDisconnected(t *testing.T) {
	s := newDispatcherTestServer(t)
	db, err := database.Create("ffldb", t.TempDir(), chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("database.Create: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	idx := indexers.NewSlpIndex(db, &indexers.SlpConfig{MaxCacheSize: 100})
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		return err
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pool, funding := newOpTruePool(t)
	s.chainParams = &chaincfg.RegressionNetParams
	s.db, s.slpIndex, s.txMemPool = db, idx, pool
	go s.slpEventHandler()
	waitFor(t, "the slp event handler to subscribe", func() bool {
		return s.numSubscriptions() == 1
	})

	// A block confirms two GENESIS transactions, each spending an output
	// of the funding transaction, and a reorg disconnects it.
	script, err := metadatamaker.TokenType1Genesis([]byte("T"), []byte("Token"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	genesis := func(i uint32) *wire.MsgTx {
		tx := wire.NewMsgTx(1)
		tx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: funding.TxHash(),
			Index: i}, nil))
		tx.AddTxOut(&wire.TxOut{PkScript: script})
		tx.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		tx.AddTxOut(&wire.TxOut{Value: 90000000, PkScript: []byte{txscript.OP_TRUE}})
		return tx
	}
	back, gone := genesis(0), genesis(1)
	block := bchutil.NewBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{back, gone}})
	for _, update := range []func(database.Tx, *bchutil.Block, []blockchain.SpentTxOut) error{
		idx.ConnectBlock, idx.DisconnectBlock,
	} {
		err := db.Update(func(dbTx database.Tx) error {
			return update(dbTx, block, nil)
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// The reorg puts the first back in the memory pool, as it does the
	// transactions of a block it disconnects, and not the second, as one
	// that a block of the other chain double-spends.
	if _, _, err := pool.MaybeAcceptTransaction(bchutil.NewTx(back), false); err != nil {
		t.Fatalf("MaybeAcceptTransaction: %v", err)
	}
	validate := func(tx *wire.MsgTx) error {
		hash := tx.TxHash()
		_, err := s.GetSlpTrustedValidation(context.Background(),
			&pb.GetSlpTrustedValidationRequest{
				Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
					PrevOutHash: hash[:],
					PrevOutVout: 1,
				}},
			})
		return err
	}
	s.dispatchEvent(&rpcEventBlockDisconnected{block})
	waitFor(t, "the GENESIS the memory pool did not take back to be no "+
		"longer valid", func() bool {
		return status.Code(validate(gone)) == codes.NotFound
	})
	if err := validate(back); err != nil {
		t.Errorf("the GENESIS back in the memory pool: %v", err)
	}
}

// TestSlpGraphSearchClientErrors ensures GetSlpGraphSearch, and the graph
// search count of GetSlpTrustedValidation, answer a search they cannot make
// for what the client asked as the client's error, which is logged at debug,
// rather than as an internal one, which is logged as an error: a valid hash
// that is not in the token's graph as an invalid argument, a transaction that
// is not in it as not found, and a hash of the wrong length as an invalid
// argument, with a message that does not repeat it.
func TestSlpGraphSearchClientErrors(t *testing.T) {
	s, genesis := newGraphSearchServer(t)
	hash := genesis.TxHash()

	// A GENESIS that is valid slp, but that graph search does not hold, as
	// one a block confirms for a moment.
	script, err := metadatamaker.TokenType1Genesis([]byte("O"), []byte("Other"),
		nil, nil, 0, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	other := wire.NewMsgTx(1)
	other.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{2}}, nil))
	other.AddTxOut(&wire.TxOut{PkScript: script})
	other.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	err = s.db.View(func(dbTx database.Tx) error {
		valid, err := s.slpIndex.AddPotentialSlpEntries(dbTx, other)
		if err == nil && !valid {
			err = errors.New("the GENESIS is not valid")
		}
		return err
	})
	if err != nil {
		t.Fatalf("AddPotentialSlpEntries: %v", err)
	}
	otherHash := other.TxHash()
	outside := chainhash.Hash{0xff}

	ctx := context.Background()
	search := func(hash []byte, valid ...[]byte) error {
		_, err := s.GetSlpGraphSearch(ctx,
			&pb.GetSlpGraphSearchRequest{Hash: hash, ValidHashes: valid})
		return err
	}
	count := func(hash []byte, valid ...[]byte) error {
		_, err := s.GetSlpTrustedValidation(ctx,
			&pb.GetSlpTrustedValidationRequest{
				Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
					PrevOutHash:            hash,
					PrevOutVout:            1,
					GraphsearchValidHashes: valid,
				}},
				IncludeGraphsearchCount: true,
			})
		return err
	}
	long := make([]byte, 100000)
	notValid := "valid hash " + outside.String() + " is not in the token's graph"
	notInGraph := "transaction " + otherHash.String() + " is not in the " +
		"token's graph"
	for _, test := range []struct {
		name string
		err  error
		code codes.Code
		msg  string
	}{
		{"a search with a valid hash outside the graph",
			search(hash[:], outside[:]), codes.InvalidArgument, notValid},
		{"a count with a valid hash outside the graph",
			count(hash[:], outside[:]), codes.InvalidArgument, notValid},
		{"a search from a transaction outside the graph",
			search(otherHash[:]), codes.NotFound, notInGraph},
		{"a count from a transaction outside the graph",
			count(otherHash[:]), codes.NotFound, notInGraph},
		{"a search from a hash of the wrong length", search(long),
			codes.InvalidArgument, "invalid graph search hash: invalid " +
				"hash length of 100000, want 32"},
		{"a search with a valid hash of the wrong length",
			search(hash[:], long), codes.InvalidArgument, "invalid graph " +
				"search valid hash: invalid hash length of 100000, want 32"},
	} {
		if status.Code(test.err) != test.code ||
			status.Convert(test.err).Message() != test.msg {

			t.Errorf("%s: got %.200v, want %v with %q", test.name, test.err,
				test.code, test.msg)
		}
	}
}

// TestSlpIndexEntryErrors ensures GetSlpGraphSearch and GetSlpTrustedValidation
// answer a transaction the slp index does not hold as not found, naming it,
// and a failure of the index as an internal error, with a message that leaves
// out the database's own error, which can name the node's files, rather than
// both as Aborted with the index's error.
func TestSlpIndexEntryErrors(t *testing.T) {
	s, _ := newGraphSearchServer(t)
	ctx := context.Background()
	search := func(hash chainhash.Hash) error {
		_, err := s.GetSlpGraphSearch(ctx,
			&pb.GetSlpGraphSearchRequest{Hash: hash[:]})
		return err
	}
	validate := func(hash chainhash.Hash) error {
		_, err := s.GetSlpTrustedValidation(ctx,
			&pb.GetSlpTrustedValidationRequest{
				Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
					PrevOutHash: hash[:],
					PrevOutVout: 1,
				}},
			})
		return err
	}

	// The index has no entry for one, and a corrupt one for the other.
	unknown, corrupt := chainhash.Hash{0xee, 1}, chainhash.Hash{0xee, 2}
	err := s.db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().Bucket([]byte("slptxbyhashidx")).Put(corrupt[:],
			[]byte{1})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct {
		name string
		call func(chainhash.Hash) error
	}{
		{"GetSlpGraphSearch", search},
		{"GetSlpTrustedValidation", validate},
	} {
		err := call.call(unknown)
		want := "transaction " + unknown.String() + " is not in the slp index"
		if status.Code(err) != codes.NotFound || status.Convert(err).Message() != want {
			t.Errorf("%s of a transaction the index does not hold: got %v, "+
				"want NotFound with %q", call.name, err, want)
		}
		err = call.call(corrupt)
		want = "failed to load slp index entry"
		if status.Code(err) != codes.Internal || status.Convert(err).Message() != want {
			t.Errorf("%s of a corrupt entry: got %v, want Internal with %q",
				call.name, err, want)
		}
	}
}

// TestSlpTrustedValidationBadHash ensures a query hash of the wrong length is
// refused as an invalid argument, with a message that does not repeat it,
// rather than with Aborted and the query's bytes.
func TestSlpTrustedValidationBadHash(t *testing.T) {
	s, _ := newGraphSearchServer(t)
	_, err := s.GetSlpTrustedValidation(context.Background(),
		&pb.GetSlpTrustedValidationRequest{
			Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
				PrevOutHash: make([]byte, 100000),
				PrevOutVout: 1,
			}},
		})
	want := "invalid trusted validation hash: invalid hash length of 100000, " +
		"want 32"
	if status.Code(err) != codes.InvalidArgument || status.Convert(err).Message() != want {
		t.Fatalf("got %.100q, want InvalidArgument with %q", err, want)
	}
}

// TestSlpTokenMetadataErrors ensures GetSlpTokenMetadata answers a token the
// slp index holds no GENESIS of, as one whose token ID names no transaction
// or one that is not a GENESIS, as not found, a failure of the index as an
// internal error with a fixed message, logging the failure, and a token ID of
// the wrong length as an invalid argument, with a message that does not repeat
// it, rather than all with Aborted.
func TestSlpTokenMetadataErrors(t *testing.T) {
	s, genesis := newGraphSearchServer(t)

	// tokenID returns the token ID that names tx, in the byte order the
	// call takes it in.
	tokenID := func(hash chainhash.Hash) []byte {
		id, err := hex.DecodeString(hash.String())
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	// A SEND of zero tokens names a GENESIS that does not exist, and the
	// index has a corrupt entry for another transaction.
	script, err := metadatamaker.TokenType1Send(bytes.Repeat([]byte{0x01}, 32),
		[]uint64{0})
	if err != nil {
		t.Fatal(err)
	}
	send := wire.NewMsgTx(1)
	send.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{3}}, nil))
	send.AddTxOut(&wire.TxOut{PkScript: script})
	send.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
	corrupt := chainhash.Hash{0xee, 3}
	err = s.db.Update(func(dbTx database.Tx) error {
		// The bucket of index tips, which the index manager creates.
		_, err := dbTx.Metadata().CreateBucketIfNotExists([]byte("idxtips"))
		if err != nil {
			return err
		}
		valid, err := s.slpIndex.AddPotentialSlpEntries(dbTx, send)
		if err == nil && !valid {
			err = errors.New("the SEND is not valid")
		}
		if err != nil {
			return err
		}
		return dbTx.Metadata().Bucket([]byte("slptxbyhashidx")).Put(corrupt[:],
			[]byte{1})
	})
	if err != nil {
		t.Fatal(err)
	}

	var logged bytes.Buffer
	saved := log
	UseLogger(bchlog.NewBackend(&logged).Logger("GRPC"))
	defer UseLogger(saved)
	metadata := func(id []byte) error {
		_, err := s.GetSlpTokenMetadata(context.Background(),
			&pb.GetSlpTokenMetadataRequest{TokenIds: [][]byte{id}})
		return err
	}
	if err := metadata(tokenID(genesis.TxHash())); err != nil {
		t.Fatalf("the GENESIS's token: %v", err)
	}
	unknown := bytes.Repeat([]byte{0x77}, 32)
	for _, test := range []struct {
		name string
		id   []byte
		code codes.Code
		msg  string
	}{
		{"a token ID naming no transaction", unknown, codes.NotFound,
			"token ID " + hex.EncodeToString(unknown) + " has no GENESIS in " +
				"the slp index"},
		{"a token ID naming a SEND", tokenID(send.TxHash()), codes.NotFound,
			"token ID " + send.TxHash().String() + " has no GENESIS in the " +
				"slp index"},
		{"a corrupt entry", tokenID(corrupt), codes.Internal,
			"failed to load slp token metadata"},
		{"a token ID of the wrong length", make([]byte, 100000),
			codes.InvalidArgument, "invalid token ID: invalid hash length " +
				"of 100000, want 32"},
	} {
		err := metadata(test.id)
		if status.Code(err) != test.code || status.Convert(err).Message() != test.msg {
			t.Errorf("%s: got %.200v, want %v with %q", test.name, err,
				test.code, test.msg)
		}
	}
	want := "[ERR] GRPC: Failed to load slp token metadata of token ID " +
		corrupt.String() + ": corrupt slp index entry for " + corrupt.String()
	if !strings.Contains(logged.String(), want) {
		t.Errorf("log %q, want a line with %q", logged.String(), want)
	}

	// The GENESIS is in the memory pool, and the cache, which holds 100
	// tokens' metadata, no longer holds that of its token once 150 others
	// come, so the index has none.
	for i := 0; i < 150; i++ {
		script, err := metadatamaker.TokenType1Genesis([]byte("T"),
			[]byte("Token"), nil, nil, 0, nil, 1000)
		if err != nil {
			t.Fatal(err)
		}
		other := wire.NewMsgTx(1)
		other.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: chainhash.Hash{0xee,
			byte(i), 0x20}}, nil))
		other.AddTxOut(&wire.TxOut{PkScript: script})
		other.AddTxOut(&wire.TxOut{Value: 546, PkScript: []byte{txscript.OP_TRUE}})
		err = s.db.View(func(dbTx database.Tx) error {
			_, err := s.slpIndex.AddPotentialSlpEntries(dbTx, other)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	err = metadata(tokenID(genesis.TxHash()))
	want = "token ID " + hex.EncodeToString(tokenID(genesis.TxHash())) +
		" has no metadata in the slp index"
	if status.Code(err) != codes.NotFound || status.Convert(err).Message() != want {
		t.Errorf("a token of the memory pool whose metadata the cache no "+
			"longer holds: got %v, want NotFound with %q", err, want)
	}
}
