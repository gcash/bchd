package bchrpc

import (
	"testing"

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
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
