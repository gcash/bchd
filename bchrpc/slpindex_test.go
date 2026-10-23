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

	fetch := func(*chainhash.Hash) ([]byte, error) {
		var buf bytes.Buffer
		err := genesis.Serialize(&buf)
		return buf.Bytes(), err
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
	go s.slpEventHandler()
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
	go s.slpEventHandler()
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
