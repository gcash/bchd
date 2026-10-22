package bchrpc

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// spendOfWatchedAddress returns a transaction spending prevOut, the pkScript
// that funded it, and the watched address.  Nothing in the transaction pays
// back to the address, which is what normally masks this problem: a wallet
// spending from an address usually sends change to itself, so the notification
// fires on the change output rather than on the spend.
func spendOfWatchedAddress(t *testing.T, params *chaincfg.Params,
	prevOut wire.OutPoint) (*bchutil.Tx, []byte, bchutil.Address) {

	t.Helper()

	var hash160 [20]byte
	for i := range hash160 {
		hash160[i] = byte(i + 1)
	}

	// A p2sh address, as reported, though nothing here is specific to the
	// script type.
	addr, err := bchutil.NewAddressScriptHashFromHash(hash160[:], params)
	if err != nil {
		t.Fatalf("unable to create address: %v", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("unable to build pkScript: %v", err)
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut})
	tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})

	return bchutil.NewTx(tx), pkScript, addr
}

// TestFilterMatchesSpendOfUnseenOutput ensures a transaction spending an output
// the filter never watched arrive still matches on the address that funded the
// input.  The outpoint map only holds outputs the filter saw arrive, so on its
// own it cannot match a spend of anything the address held before the
// subscription started.
func TestFilterMatchesSpendOfUnseenOutput(t *testing.T) {
	params := &chaincfg.MainNetParams
	prevOut := wire.OutPoint{Hash: chainhash.Hash{0xaa}, Index: 0}
	tx, pkScript, addr := spendOfWatchedAddress(t, params, prevOut)

	// Without a resolver the spend cannot be matched.
	f := newTxFilter()
	f.AddAddress(addr)
	if f.MatchAndUpdate(tx, params, nil) {
		t.Fatal("filter matched a spend it had no way to resolve")
	}

	// With one it matches.
	f = newTxFilter()
	f.AddAddress(addr)
	resolve := func(op wire.OutPoint) []byte {
		if op == prevOut {
			return pkScript
		}
		return nil
	}
	if !f.MatchAndUpdate(tx, params, resolve) {
		t.Fatal("filter did not match a spend of the watched address")
	}
}

// TestFilterSkipsPrevOutLookupWithoutAddresses ensures the previous output is
// not resolved when no addresses are being watched, since the lookup can hit
// disk and would otherwise be paid on every input of every transaction.
func TestFilterSkipsPrevOutLookupWithoutAddresses(t *testing.T) {
	params := &chaincfg.MainNetParams
	prevOut := wire.OutPoint{Hash: chainhash.Hash{0xaa}, Index: 0}
	tx, pkScript, _ := spendOfWatchedAddress(t, params, prevOut)

	called := false
	resolve := func(op wire.OutPoint) []byte {
		called = true
		return pkScript
	}

	f := newTxFilter()
	f.AddDataElement([]byte{0x01})
	if f.MatchAndUpdate(tx, params, resolve) {
		t.Fatal("filter matched on an unrelated data element")
	}
	if called {
		t.Fatal("previous output was resolved with no addresses watched")
	}
}

// TestFilterMatchesAddressFormats ensures a filter given a legacy address, an
// slp P2SH32 address or a pubkey matches outputs paying to them, and that
// removing the same address stops the match.
func TestFilterMatchesAddressFormats(t *testing.T) {
	params := &chaincfg.MainNetParams
	hash := bytes.Repeat([]byte{0xab}, 32)
	_, pubKey := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{0x01}, 32))
	serialized := pubKey.SerializeCompressed()

	legacy, err := bchutil.NewLegacyAddressPubKeyHash(hash[:20], params)
	if err != nil {
		t.Fatalf("NewLegacyAddressPubKeyHash: %v", err)
	}
	slp32, err := bchutil.NewSlpAddressScriptHash32FromHash(hash, params)
	if err != nil {
		t.Fatalf("NewSlpAddressScriptHash32FromHash: %v", err)
	}

	tests := []struct {
		name     string
		address  string
		pkScript []byte
	}{
		{"legacy p2pkh", legacy.EncodeAddress(), append(append([]byte{
			txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20},
			hash[:20]...), txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)},
		{"slp p2sh32", slp32.EncodeAddress(), append(append([]byte{
			txscript.OP_HASH256, txscript.OP_DATA_32}, hash...),
			txscript.OP_EQUAL)},
		{"pubkey", hex.EncodeToString(serialized), append(append([]byte{
			txscript.OP_DATA_33}, serialized...), txscript.OP_CHECKSIG)},
	}
	for _, test := range tests {
		tx := wire.NewMsgTx(wire.TxVersion)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{
			Hash: chainhash.Hash{0xaa}}})
		tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: test.pkScript})

		rpcFilter := &pb.TransactionFilter{Addresses: []string{test.address}}
		f := newTxFilter()
		if err := f.AddRPCFilter(rpcFilter, params); err != nil {
			t.Fatalf("%s: AddRPCFilter: %v", test.name, err)
		}
		if !f.MatchAndUpdate(bchutil.NewTx(tx), params, nil) {
			t.Errorf("%s: filter did not match an output paying to %s",
				test.name, test.address)
		}

		f = newTxFilter()
		if err := f.AddRPCFilter(rpcFilter, params); err != nil {
			t.Fatalf("%s: AddRPCFilter: %v", test.name, err)
		}
		if err := f.RemoveRPCFilter(rpcFilter, params); err != nil {
			t.Fatalf("%s: RemoveRPCFilter: %v", test.name, err)
		}
		if f.MatchAndUpdate(bchutil.NewTx(tx), params, nil) {
			t.Errorf("%s: filter matched after the address was removed",
				test.name)
		}
	}
}

// TestRPCFilterErrors ensures a filter with a bad outpoint hash or address is
// refused as an invalid argument, with a message that does not repeat the
// input.  An error of another kind is logged in full at the error level, and
// an address can be as long as the request.
func TestRPCFilterErrors(t *testing.T) {
	params := &chaincfg.MainNetParams
	outpoint := func(hash []byte) *pb.TransactionFilter {
		return &pb.TransactionFilter{Outpoints: []*pb.Transaction_Input_Outpoint{
			{Hash: hash}}}
	}
	address := func(addrs ...string) *pb.TransactionFilter {
		return &pb.TransactionFilter{Addresses: addrs}
	}
	long := strings.Repeat("q", 100000)
	tests := []struct {
		name   string
		filter *pb.TransactionFilter
		want   string
	}{
		{"short outpoint hash", outpoint(make([]byte, 31)),
			"invalid outpoint hash: invalid hash length of 31, want 32"},
		{"long outpoint hash", outpoint(make([]byte, 100000)),
			"invalid outpoint hash: invalid hash length of 100000, want 32"},
		{"long address", address(long),
			"invalid address at index 0: invalid length address"},
		{"second address bad", address(
			"bitcoincash:qp3wjpa3tjlj042z2wv7hahsldgwhwy0rq9sywjpyy",
			"bitcoincash:qp3wjpa3tjlj042z2wv7hahsldgwhwy0rq9sywjpyz"),
			"invalid address at index 1: checksum mismatch"},
	}
	for _, test := range tests {
		for _, op := range []struct {
			name  string
			apply func(*pb.TransactionFilter, *chaincfg.Params) error
		}{
			{"add", newTxFilter().AddRPCFilter},
			{"remove", newTxFilter().RemoveRPCFilter},
		} {
			err := op.apply(test.filter, params)
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("%s: %s: got %.100v, want InvalidArgument",
					test.name, op.name, err)
				continue
			}
			if got := status.Convert(err).Message(); got != test.want {
				t.Errorf("%s: %s: got message %.100q, want %q",
					test.name, op.name, got, test.want)
			}
		}
	}
}

// TestFilterFallbackAddressLogLevel ensures adding or removing an address the
// filter matches by its encoding, such as a P2SH32 address, logs at debug, as
// a filter can hold any number of them.
func TestFilterFallbackAddressLogLevel(t *testing.T) {
	params := &chaincfg.MainNetParams
	addr, err := bchutil.NewAddressScriptHash32FromHash(
		bytes.Repeat([]byte{0xab}, 32), params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash32FromHash: %v", err)
	}
	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("GRPC")
	logger.SetLevel(bchlog.LevelInfo)
	saved := log
	UseLogger(logger)
	defer UseLogger(saved)

	rpcFilter := &pb.TransactionFilter{Addresses: []string{addr.EncodeAddress()}}
	f := newTxFilter()
	if err := f.AddRPCFilter(rpcFilter, params); err != nil {
		t.Fatalf("AddRPCFilter: %v", err)
	}
	if len(f.fallbacks) != 1 {
		t.Fatalf("%d addresses matched by their encoding, want 1",
			len(f.fallbacks))
	}
	if err := f.RemoveRPCFilter(rpcFilter, params); err != nil {
		t.Fatalf("RemoveRPCFilter: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("logged at the info level: %q", buf.String())
	}
}
