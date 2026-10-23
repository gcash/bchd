package bchrpc

import (
	"bytes"
	"encoding/hex"
	"fmt"
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

// maxFilterSize is the limit the tests give their filters, the default of
// --grpcmaxfilterentries.
const maxFilterSize = 100000

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
	f := newTxFilter(maxFilterSize)
	f.AddAddress(addr)
	if f.MatchAndUpdate(tx, params, nil) {
		t.Fatal("filter matched a spend it had no way to resolve")
	}

	// With one it matches.
	f = newTxFilter(maxFilterSize)
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

	f := newTxFilter(maxFilterSize)
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
		f := newTxFilter(maxFilterSize)
		if err := f.AddRPCFilter(rpcFilter, params); err != nil {
			t.Fatalf("%s: AddRPCFilter: %v", test.name, err)
		}
		if !f.MatchAndUpdate(bchutil.NewTx(tx), params, nil) {
			t.Errorf("%s: filter did not match an output paying to %s",
				test.name, test.address)
		}

		f = newTxFilter(maxFilterSize)
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
			{"add", newTxFilter(maxFilterSize).AddRPCFilter},
			{"remove", newTxFilter(maxFilterSize).RemoveRPCFilter},
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
	f := newTxFilter(maxFilterSize)
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

// filterOutpoints returns a filter of n outpoints, all different from the
// ones other calls with a different seed return.
func filterOutpoints(seed byte, n int) *pb.TransactionFilter {
	ops := make([]*pb.Transaction_Input_Outpoint, n)
	for i := range ops {
		hash := make([]byte, chainhash.HashSize)
		hash[0] = seed
		hash[1], hash[2], hash[3] = byte(i), byte(i>>8), byte(i>>16)
		ops[i] = &pb.Transaction_Input_Outpoint{Hash: hash}
	}
	return &pb.TransactionFilter{Outpoints: ops}
}

// TestFilterSize ensures a filter's size counts each outpoint, address and
// token ID once, and a data element once for each 20 bytes it holds, rounded
// up and at least once, and that removing an entry takes it off.
func TestFilterSize(t *testing.T) {
	params := &chaincfg.MainNetParams
	key, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	compressed, err := bchutil.NewAddressPubKey(
		key.PubKey().SerializeCompressed(), params)
	if err != nil {
		t.Fatalf("NewAddressPubKey: %v", err)
	}
	uncompressed, err := bchutil.NewAddressPubKey(
		key.PubKey().SerializeUncompressed(), params)
	if err != nil {
		t.Fatalf("NewAddressPubKey: %v", err)
	}
	hash := bytes.Repeat([]byte{0xab}, 32)
	pkh, err := bchutil.NewAddressPubKeyHash(hash[:20], params)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: %v", err)
	}
	sh, err := bchutil.NewAddressScriptHashFromHash(hash[:20], params)
	if err != nil {
		t.Fatalf("NewAddressScriptHashFromHash: %v", err)
	}
	sh32, err := bchutil.NewAddressScriptHash32FromHash(hash, params)
	if err != nil {
		t.Fatalf("NewAddressScriptHash32FromHash: %v", err)
	}

	f := newTxFilter(maxFilterSize)
	want := 0
	check := func(what string) {
		t.Helper()
		if got := f.size(); got != want {
			t.Fatalf("after %s: size %d, want %d", what, got, want)
		}
	}
	f.AddOutpoint(wire.OutPoint{Index: 1})
	want++
	check("an outpoint")
	for _, addr := range []bchutil.Address{compressed, uncompressed, pkh, sh, sh32} {
		f.AddAddress(addr)
		want++
		check(addr.String())
	}
	f.AddSlpTokenID(hash)
	want++
	check("a token ID")
	for _, test := range []struct {
		size, units int
	}{{0, 1}, {1, 1}, {20, 1}, {21, 2}, {40, 2}, {41, 3},
		{txscript.MaxScriptElementSize, 500}} {

		element := bytes.Repeat([]byte{0xcd}, test.size)
		f.AddDataElement(element)
		want += test.units
		check(fmt.Sprintf("a data element of %d bytes", test.size))
		f.AddDataElement(element)
		check(fmt.Sprintf("the data element of %d bytes again", test.size))
	}
	f.AddDataElement(make([]byte, txscript.MaxScriptElementSize+1))
	check("a data element too long to match")

	f.RemoveDataElement(bytes.Repeat([]byte{0xcd}, 41))
	want -= 3
	check("removing a data element")
	f.RemoveDataElement(bytes.Repeat([]byte{0xcd}, 41))
	check("removing the data element again")
	f.RemoveAddress(sh32)
	want--
	check("removing an address")
	f.RemoveOutpoint(wire.OutPoint{Index: 1})
	want--
	check("removing an outpoint")
}

// TestFilterSizeLimit ensures a filter holds at most maxFilterSize, whatever
// the entries a request adds, and that removing entries makes room again.  A
// SubscribeTransactionStream call can add to its filter without end, and a
// request of small entries holds several times its size in the filter.
func TestFilterSizeLimit(t *testing.T) {
	params := &chaincfg.MainNetParams
	full := func(room int) *txFilter {
		t.Helper()
		f := newTxFilter(maxFilterSize)
		for i := 0; i < 4; i++ {
			n := maxFilterSize / 4
			if i == 3 {
				n = maxFilterSize - 3*(maxFilterSize/4) - room
			}
			if err := f.AddRPCFilter(filterOutpoints(byte(i), n), params); err != nil {
				t.Fatalf("filling the filter: %v", err)
			}
		}
		return f
	}
	key, err := bchec.NewPrivateKey(bchec.S256())
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	pubKey := hex.EncodeToString(key.PubKey().SerializeCompressed())
	tests := []struct {
		name   string
		room   int
		filter *pb.TransactionFilter
		refuse bool
	}{
		{"outpoint", 0, filterOutpoints(9, 1), true},
		{"outpoint with room", 1, filterOutpoints(9, 1), false},
		{"address", 0, &pb.TransactionFilter{Addresses: []string{
			"bitcoincash:qp3wjpa3tjlj042z2wv7hahsldgwhwy0rq9sywjpyy"}}, true},
		{"pubkey", 0, &pb.TransactionFilter{Addresses: []string{pubKey}}, true},
		{"token ID", 0, &pb.TransactionFilter{SlpTokenIds: [][]byte{
			bytes.Repeat([]byte{1}, 32)}}, true},
		{"empty data element", 0, &pb.TransactionFilter{
			DataElements: [][]byte{{}}}, true},
		{"data element", 0, &pb.TransactionFilter{
			DataElements: [][]byte{{1}}}, true},
		{"500 byte data element", 25, &pb.TransactionFilter{
			DataElements: [][]byte{make([]byte, 500)}}, false},
		{"501 byte data element", 25, &pb.TransactionFilter{
			DataElements: [][]byte{make([]byte, 501)}}, true},
		{"data element too long to match", 0, &pb.TransactionFilter{
			DataElements: [][]byte{make([]byte, txscript.MaxScriptElementSize+1)}},
			false},
	}
	for _, test := range tests {
		f := full(test.room)
		err := f.AddRPCFilter(test.filter, params)
		if !test.refuse {
			if err != nil {
				t.Errorf("%s: refused: %v", test.name, err)
			}
			continue
		}
		if status.Code(err) != codes.ResourceExhausted {
			t.Errorf("%s: got %v, want ResourceExhausted", test.name, err)
			continue
		}
		want := fmt.Sprintf("filter holds more than %d entries", maxFilterSize)
		if got := status.Convert(err).Message(); got != want {
			t.Errorf("%s: got message %q, want %q", test.name, got, want)
		}
	}

	// Removing entries makes room.
	f := full(0)
	if err := f.RemoveRPCFilter(filterOutpoints(0, 1), params); err != nil {
		t.Fatalf("RemoveRPCFilter: %v", err)
	}
	if err := f.AddRPCFilter(filterOutpoints(9, 1), params); err != nil {
		t.Errorf("refused after an entry was removed: %v", err)
	}
}

// dataTx returns a transaction spending prevOut whose output pushes data, or
// pushes nothing when data is nil.
func dataTx(prevOut wire.OutPoint, data []byte) *wire.MsgTx {
	script := []byte{txscript.OP_TRUE}
	if data != nil {
		script = append([]byte{txscript.OP_RETURN, byte(len(data))}, data...)
	}
	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut})
	tx.AddTxOut(&wire.TxOut{PkScript: script})
	return tx
}

// TestFilterMatchedOutputsNotCounted ensures the outputs a filter matched do
// not count toward its size, which bounds only what requests add, so a request
// is still accepted however many outputs the filter has matched.
func TestFilterMatchedOutputsNotCounted(t *testing.T) {
	params := &chaincfg.MainNetParams
	data := []byte{1, 2, 3}
	watch := &pb.TransactionFilter{DataElements: [][]byte{data}}
	f := newTxFilter(3)
	if err := f.AddRPCFilter(watch, params); err != nil {
		t.Fatalf("AddRPCFilter: %v", err)
	}
	for i := 0; i < 10; i++ {
		tx := bchutil.NewTx(dataTx(wire.OutPoint{Index: uint32(i)}, data))
		if !f.MatchAndUpdate(tx, params, nil) {
			t.Fatalf("output %d not matched", i)
		}
	}
	if got := f.size(); got != 1 {
		t.Errorf("size %d after matching outputs, want 1", got)
	}
	if err := f.AddRPCFilter(watch, params); err != nil {
		t.Errorf("the entry again: %v", err)
	}
	if err := f.AddRPCFilter(filterOutpoints(1, 2), params); err != nil {
		t.Errorf("two entries more: %v", err)
	}
	err := f.AddRPCFilter(filterOutpoints(2, 1), params)
	if status.Code(err) != codes.ResourceExhausted {
		t.Errorf("an entry over the limit: got %v, want ResourceExhausted", err)
	}
}

// TestFilterMatchedOutpointsBounded ensures a filter keeps at most maxSize of
// the outputs it matched, so that their spends match too, dropping the oldest
// first: the newest half of them, and at least the newest, are always kept,
// and none older than the newest maxSize, so a bound of 0 keeps none.
func TestFilterMatchedOutpointsBounded(t *testing.T) {
	params := &chaincfg.MainNetParams
	data := []byte{1, 2, 3}
	for _, bound := range []int{0, 1, 2, 3, 4, 10} {
		f := newTxFilter(bound)
		f.AddDataElement(data)
		const n = 25
		var outs []wire.OutPoint
		for i := 0; i < n; i++ {
			tx := bchutil.NewTx(dataTx(wire.OutPoint{Index: uint32(i)}, data))
			if !f.MatchAndUpdate(tx, params, nil) {
				t.Fatalf("bound %d: output %d not matched", bound, i)
			}
			outs = append(outs, wire.OutPoint{Hash: *tx.Hash()})
		}
		kept := 0
		if bound > 0 {
			kept = max(1, bound/2)
		}
		dropped, missing := 0, 0
		for i, op := range outs {
			matched := f.MatchAndUpdate(bchutil.NewTx(dataTx(op, nil)), params, nil)
			if i < n-bound && matched {
				dropped++
			}
			if i >= n-kept && !matched {
				missing++
			}
		}
		if dropped != 0 || missing != 0 {
			t.Errorf("bound %d: spends of %d of the oldest %d outputs "+
				"matched, of %d of the newest %d did not", bound, dropped,
				n-bound, missing, kept)
		}
	}
}

// TestFilterRemovesMatchedOutpoint ensures a request that removes an outpoint
// also removes it when the filter matched its output, as when both were one
// set, and that a spend of an outpoint a request added matches and takes it
// out of the filter.
func TestFilterRemovesMatchedOutpoint(t *testing.T) {
	params := &chaincfg.MainNetParams
	data := []byte{1, 2, 3}
	f := newTxFilter(maxFilterSize)
	if err := f.AddRPCFilter(filterOutpoints(1, 2), params); err != nil {
		t.Fatalf("AddRPCFilter: %v", err)
	}
	var added wire.OutPoint
	added.Hash[0] = 1
	if !f.MatchAndUpdate(bchutil.NewTx(dataTx(added, nil)), params, nil) {
		t.Error("the spend of an outpoint a request added did not match")
	}
	if got := f.size(); got != 1 {
		t.Errorf("size %d after the spend, want 1", got)
	}
	f.AddDataElement(data)
	tx := bchutil.NewTx(dataTx(wire.OutPoint{}, data))
	if !f.MatchAndUpdate(tx, params, nil) {
		t.Fatal("output not matched")
	}
	op := &pb.Transaction_Input_Outpoint{Hash: tx.Hash()[:]}
	err := f.RemoveRPCFilter(&pb.TransactionFilter{
		Outpoints: []*pb.Transaction_Input_Outpoint{op}}, params)
	if err != nil {
		t.Fatalf("RemoveRPCFilter: %v", err)
	}
	spend := dataTx(wire.OutPoint{Hash: *tx.Hash()}, nil)
	if f.MatchAndUpdate(bchutil.NewTx(spend), params, nil) {
		t.Error("the spend of a removed output matched")
	}
}
