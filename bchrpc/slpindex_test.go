package bchrpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/blockchain/indexers"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/database"
	_ "github.com/gcash/bchd/database/ffldb"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
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
