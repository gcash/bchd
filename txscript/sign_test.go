// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"math/rand"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchd/chaincfg/chainhash"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil"
)

type addressToKey struct {
	key        *bchec.PrivateKey
	compressed bool
}

func mkGetKey(keys map[string]addressToKey) KeyDB {
	if keys == nil {
		return KeyClosure(func(addr bchutil.Address) (*bchec.PrivateKey,
			bool, error) {
			return nil, false, errors.New("nope")
		})
	}
	return KeyClosure(func(addr bchutil.Address) (*bchec.PrivateKey,
		bool, error) {
		a2k, ok := keys[addr.EncodeAddress()]
		if !ok {
			return nil, false, errors.New("nope")
		}
		return a2k.key, a2k.compressed, nil
	})
}

func mkGetScript(scripts map[string][]byte) ScriptDB {
	if scripts == nil {
		return ScriptClosure(func(addr bchutil.Address) ([]byte, error) {
			return nil, errors.New("nope")
		})
	}
	return ScriptClosure(func(addr bchutil.Address) ([]byte, error) {
		script, ok := scripts[addr.EncodeAddress()]
		if !ok {
			return nil, errors.New("nope")
		}
		return script, nil
	})
}

func checkScripts(msg string, tx *wire.MsgTx, idx int, inputAmt int64, sigScript, pkScript []byte) error {
	tx.TxIn[idx].SignatureScript = sigScript
	vm, err := NewEngine(pkScript, tx, idx,
		ScriptBip16|ScriptVerifyDERSignatures|ScriptVerifyBip143SigHash|ScriptVerifySchnorr, nil, nil, nil, inputAmt)
	if err != nil {
		return fmt.Errorf("failed to make script engine for %s: %v",
			msg, err)
	}

	err = vm.Execute()
	if err != nil {
		return fmt.Errorf("invalid script signature for %s: %v", msg,
			err)
	}

	return nil
}

func signAndCheck(msg string, tx *wire.MsgTx, idx int, inputAmt int64, pkScript []byte,
	hashType SigHashType, kdb KeyDB, sdb ScriptDB,
	previousScript []byte) error {

	sigScript, err := SignTxOutput(&chaincfg.TestNet3Params, tx, idx,
		inputAmt, pkScript, hashType, kdb, sdb, nil)
	if err != nil {
		return fmt.Errorf("failed to sign output %s: %v", msg, err)
	}

	return checkScripts(msg, tx, idx, inputAmt, sigScript, pkScript)
}

func TestSignTxOutput(t *testing.T) {
	t.Parallel()

	// make key
	// make script based on key.
	// sign with magic pixie dust.
	hashTypes := []SigHashType{
		SigHashOld, // no longer used but should act like all
		SigHashAll,
		SigHashNone,
		SigHashSingle,
		SigHashAll | SigHashAnyOneCanPay,
		SigHashNone | SigHashAnyOneCanPay,
		SigHashSingle | SigHashAnyOneCanPay,
	}
	inputAmounts := []int64{5, 10, 15}
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0,
				},
				Sequence: 4294967295,
			},
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 1,
				},
				Sequence: 4294967295,
			},
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 2,
				},
				Sequence: 4294967295,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value: 1,
			},
			{
				Value: 2,
			},
			{
				Value: 3,
			},
		},
		LockTime: 0,
	}

	// Pay to Pubkey Hash (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (uncompressed) (merging with correct)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i],
				sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (compressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i],
				sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// As before, but with p2sh now.
	// Pay to Pubkey Hash (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
				break
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (uncompressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
				break
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			_, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i],
				sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKeyHash(
				bchutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			_, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i],
				sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			_, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i],
				sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*bchec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := bchutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			_, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i],
				sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Basic Multisig
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key1, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk1 := (*bchec.PublicKey)(&key1.PublicKey).
				SerializeCompressed()
			address1, err := bchutil.NewAddressPubKey(pk1,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			key2, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey 2 for %s: %v",
					msg, err)
				break
			}

			pk2 := (*bchec.PublicKey)(&key2.PublicKey).
				SerializeCompressed()
			address2, err := bchutil.NewAddressPubKey(pk2,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address 2 for %s: %v",
					msg, err)
				break
			}

			pkScript, err := MultiSigScript(
				[]*bchutil.AddressPubKey{address1, address2},
				2)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
					address2.EncodeAddress(): {key2, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Two part multisig, sign with one key then the other.
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key1, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk1 := (*bchec.PublicKey)(&key1.PublicKey).
				SerializeCompressed()
			address1, err := bchutil.NewAddressPubKey(pk1,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			key2, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey 2 for %s: %v",
					msg, err)
				break
			}

			pk2 := (*bchec.PublicKey)(&key2.PublicKey).
				SerializeCompressed()
			address2, err := bchutil.NewAddressPubKey(pk2,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address 2 for %s: %v",
					msg, err)
				break
			}

			pkScript, err := MultiSigScript(
				[]*bchutil.AddressPubKey{address1, address2},
				2)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// Only 1 out of 2 signed, this *should* fail.
			if checkScripts(msg, tx, i, inputAmounts[i], sigScript,
				scriptPkScript) == nil {
				t.Errorf("part signed script valid for %s", msg)
				break
			}

			// Sign with the other key and merge
			sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address2.EncodeAddress(): {key2, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript,
				scriptPkScript)
			if err != nil {
				t.Errorf("fully signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Two part multisig, sign with one key then both, check key dedup
	// correctly.
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key1, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk1 := (*bchec.PublicKey)(&key1.PublicKey).
				SerializeCompressed()
			address1, err := bchutil.NewAddressPubKey(pk1,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			key2, err := bchec.NewPrivateKey(bchec.S256())
			if err != nil {
				t.Errorf("failed to make privKey 2 for %s: %v",
					msg, err)
				break
			}

			pk2 := (*bchec.PublicKey)(&key2.PublicKey).
				SerializeCompressed()
			address2, err := bchutil.NewAddressPubKey(pk2,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address 2 for %s: %v",
					msg, err)
				break
			}

			pkScript, err := MultiSigScript(
				[]*bchutil.AddressPubKey{address1, address2},
				2)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := bchutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := PayToAddrScript(scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// Only 1 out of 2 signed, this *should* fail.
			if checkScripts(msg, tx, i, inputAmounts[i], sigScript,
				scriptPkScript) == nil {
				t.Errorf("part signed script valid for %s", msg)
				break
			}

			// Sign with the other key and merge
			sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
				tx, i, inputAmounts[i], scriptPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
					address2.EncodeAddress(): {key2, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg, err)
				break
			}

			// Now we should pass.
			err = checkScripts(msg, tx, i, inputAmounts[i],
				sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("fully signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}
}

type tstInput struct {
	txout              *wire.TxOut
	sigscriptGenerates bool
	inputValidates     bool
	indexOutOfRange    bool
}

type tstSigScript struct {
	name               string
	inputs             []tstInput
	hashType           SigHashType
	compress           bool
	scriptAtWrongIndex bool
}

//nolint:unused // retained for future tests
var coinbaseOutPoint = &wire.OutPoint{
	Index: (1 << 32) - 1,
}

// Pregenerated private key, with associated public key and pkScripts
// for the uncompressed and compressed hash160.
var (
	privKeyD = []byte{0x6b, 0x0f, 0xd8, 0xda, 0x54, 0x22, 0xd0, 0xb7,
		0xb4, 0xfc, 0x4e, 0x55, 0xd4, 0x88, 0x42, 0xb3, 0xa1, 0x65,
		0xac, 0x70, 0x7f, 0x3d, 0xa4, 0x39, 0x5e, 0xcb, 0x3b, 0xb0,
		0xd6, 0x0e, 0x06, 0x92}
	//nolint:unused // retained for future tests
	pubkeyX = []byte{0xb2, 0x52, 0xf0, 0x49, 0x85, 0x78, 0x03, 0x03, 0xc8,
		0x7d, 0xce, 0x51, 0x7f, 0xa8, 0x69, 0x0b, 0x91, 0x95, 0xf4,
		0xf3, 0x5c, 0x26, 0x73, 0x05, 0x05, 0xa2, 0xee, 0xbc, 0x09,
		0x38, 0x34, 0x3a}
	//nolint:unused // retained for future tests
	pubkeyY = []byte{0xb7, 0xc6, 0x7d, 0xb2, 0xe1, 0xff, 0xc8, 0x43, 0x1f,
		0x63, 0x32, 0x62, 0xaa, 0x60, 0xc6, 0x83, 0x30, 0xbd, 0x24,
		0x7e, 0xef, 0xdb, 0x6f, 0x2e, 0x8d, 0x56, 0xf0, 0x3c, 0x9f,
		0x6d, 0xb6, 0xf8}
	uncompressedPkScript = []byte{0x76, 0xa9, 0x14, 0xd1, 0x7c, 0xb5,
		0xeb, 0xa4, 0x02, 0xcb, 0x68, 0xe0, 0x69, 0x56, 0xbf, 0x32,
		0x53, 0x90, 0x0e, 0x0a, 0x86, 0xc9, 0xfa, 0x88, 0xac}
	compressedPkScript = []byte{0x76, 0xa9, 0x14, 0x27, 0x4d, 0x9f, 0x7f,
		0x61, 0x7e, 0x7c, 0x7a, 0x1c, 0x1f, 0xb2, 0x75, 0x79, 0x10,
		0x43, 0x65, 0x68, 0x27, 0x9d, 0x86, 0x88, 0xac}
	shortPkScript = []byte{0x76, 0xa9, 0x14, 0xd1, 0x7c, 0xb5,
		0xeb, 0xa4, 0x02, 0xcb, 0x68, 0xe0, 0x69, 0x56, 0xbf, 0x32,
		0x53, 0x90, 0x0e, 0x0a, 0x88, 0xac}
	//nolint:unused // retained for future tests
	uncompressedAddrStr = "bitcoincash:qrghed0t5spvk68qd9tt7vjnjq8q4pkflgdlgejcmk"
	//nolint:unused // retained for future tests
	compressedAddrStr = "bitcoincash:qqn5m8mlv9l8c7sur7e827gsgdjksfuascqhpjhttt"
)

// Pretend output amounts.
const coinbaseVal = 2500000000
const fee = 5000000

var sigScriptTests = []tstSigScript{
	{
		name: "one input uncompressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "two inputs uncompressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "one input compressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, compressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "two inputs compressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, compressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, compressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashNone",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashNone,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashSingle",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashSingle,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashAnyoneCanPay",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAnyOneCanPay,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType non-standard",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           0x04,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "invalid compression",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     false,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "short PkScript",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, shortPkScript, wire.TokenData{}),
				sigscriptGenerates: false,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "valid script at wrong index",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: true,
	},
	{
		name: "index out of range",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript, wire.TokenData{}),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: true,
	},
}

func newOutpoint() *wire.OutPoint {
	return &wire.OutPoint{
		Index: uint32(rand.Int31()),
	}
}

// Test the sigscript generation for valid and invalid inputs, all
// hashTypes, and with and without compression.  This test creates
// sigscripts to spend fake coinbase inputs, as sigscripts cannot be
// created for the MsgTxs in txTests, since they come from the blockchain
// and we don't have the private keys.
func TestSignatureScript(t *testing.T) {
	t.Parallel()

	privKey, _ := bchec.PrivKeyFromBytes(bchec.S256(), privKeyD)

nexttest:
	for i := range sigScriptTests {
		tx := wire.NewMsgTx(wire.TxVersion)

		output := wire.NewTxOut(500, []byte{OP_RETURN}, wire.TokenData{})
		tx.AddTxOut(output)

		for range sigScriptTests[i].inputs {
			txin := wire.NewTxIn(newOutpoint(), nil)
			tx.AddTxIn(txin)
		}

		var script []byte
		var err error
		for j := range tx.TxIn {
			var idx int
			if sigScriptTests[i].inputs[j].indexOutOfRange {
				t.Errorf("at test %v", sigScriptTests[i].name)
				idx = len(sigScriptTests[i].inputs)
			} else {
				idx = j
			}

			script, err = SignatureScript(tx, idx, 0,
				sigScriptTests[i].inputs[j].txout.PkScript,
				sigScriptTests[i].hashType, privKey,
				sigScriptTests[i].compress)

			if (err == nil) != sigScriptTests[i].inputs[j].sigscriptGenerates {
				if err == nil {
					t.Errorf("passed test '%v' incorrectly",
						sigScriptTests[i].name)
				} else {
					t.Errorf("failed test '%v': %v",
						sigScriptTests[i].name, err)
				}
				continue nexttest
			}
			if !sigScriptTests[i].inputs[j].sigscriptGenerates {
				// done with this test
				continue nexttest
			}
			tx.TxIn[j].SignatureScript = script
		}

		// If testing using a correct sigscript but for an incorrect
		// index, use last input script for first input.  Requires > 0
		// inputs for test.
		if sigScriptTests[i].scriptAtWrongIndex {
			tx.TxIn[0].SignatureScript = script
			sigScriptTests[i].inputs[0].inputValidates = false
		}

		// Validate tx input scripts
		scriptFlags := ScriptBip16 | ScriptVerifyDERSignatures | ScriptVerifyBip143SigHash | ScriptVerifySchnorr
		for j := range tx.TxIn {
			vm, err := NewEngine(sigScriptTests[i].
				inputs[j].txout.PkScript, tx, j, scriptFlags, nil, nil, nil, 0)
			if err != nil {
				t.Errorf("cannot create script vm for test %v: %v",
					sigScriptTests[i].name, err)
				continue nexttest
			}
			err = vm.Execute()
			if (err == nil) != sigScriptTests[i].inputs[j].inputValidates {
				if err == nil {
					t.Errorf("passed test '%v' validation incorrectly: %v",
						sigScriptTests[i].name, err)
				} else {
					t.Errorf("failed test '%v' validation: %v",
						sigScriptTests[i].name, err)
				}
				continue nexttest
			}
		}
	}
}

// tokenSpendTx returns a transaction whose first input spends an output with
// tokens, and second an output without, both locked by pkScript, and the
// outputs it spends.
func tokenSpendTx(pkScript []byte) (*wire.MsgTx, *UtxoCache) {
	tokenData := wire.TokenData{
		CategoryID: chainhash.DoubleHashH([]byte("category")),
		Commitment: []byte{0x01, 0x02},
		Amount:     1000,
		BitField: wire.HAS_AMOUNT | wire.HAS_NFT | wire.HAS_COMMITMENT_LENGTH |
			wire.MUTABLE,
	}
	tx := wire.NewMsgTx(2)
	utxos := NewUtxoCache()
	for i, spent := range []wire.TokenData{tokenData, {}} {
		prevHash := chainhash.DoubleHashH([]byte{byte(i)})
		tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&prevHash, 0), nil))
		utxos.AddEntry(i, *wire.NewTxOut(100000, pkScript, spent))
	}
	tx.AddTxOut(wire.NewTxOut(90000, pkScript, tokenData))
	tx.AddTxOut(wire.NewTxOut(90000, pkScript, wire.TokenData{}))
	return tx, utxos
}

// TestSignTokenInputs ensures the signing functions that take the signature
// hashes of a transaction sign an input that spends tokens, and with
// SigHashUTXO, for each script class SignTxOutput signs, multisig signatures
// merged from two signers included.  It also ensures SignTxOutput signs an
// input that spends no tokens as they do, that the functions without the
// hashes return an error for SigHashUTXO, and that those with them return one
// for SigHashUTXO with SigHashAnyOneCanPay, which the network refuses.
func TestSignTokenInputs(t *testing.T) {
	t.Parallel()

	params := &chaincfg.TestNet3Params
	key1, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{1}, 32))
	key2, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{2}, 32))
	signer := func(key *bchec.PrivateKey) (*bchutil.AddressPubKey, KeyDB) {
		addr, err := bchutil.NewAddressPubKey(
			(*bchec.PublicKey)(&key.PublicKey).SerializeCompressed(), params)
		if err != nil {
			t.Fatal(err)
		}
		return addr, mkGetKey(map[string]addressToKey{
			addr.EncodeAddress():                     {key, true},
			addr.AddressPubKeyHash().EncodeAddress(): {key, true},
		})
	}
	addr1, kdb1 := signer(key1)
	addr2, kdb2 := signer(key2)

	p2pkh, err := PayToAddrScript(addr1.AddressPubKeyHash())
	if err != nil {
		t.Fatal(err)
	}
	p2pk, err := PayToAddrScript(addr1)
	if err != nil {
		t.Fatal(err)
	}
	multiSig, err := MultiSigScript([]*bchutil.AddressPubKey{addr1, addr2}, 2)
	if err != nil {
		t.Fatal(err)
	}
	scriptAddr, err := bchutil.NewAddressScriptHash(multiSig, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh, err := PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}
	scriptAddr32, err := bchutil.NewAddressScriptHash32(multiSig, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh32, err := PayToAddrScript(scriptAddr32)
	if err != nil {
		t.Fatal(err)
	}
	p2pkhAddr32, err := bchutil.NewAddressScriptHash32(p2pkh, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh32P2PKH, err := PayToAddrScript(p2pkhAddr32)
	if err != nil {
		t.Fatal(err)
	}
	sdb := mkGetScript(map[string][]byte{
		scriptAddr.EncodeAddress():   multiSig,
		scriptAddr32.EncodeAddress(): multiSig,
		p2pkhAddr32.EncodeAddress():  p2pkh,
	})

	tests := []struct {
		name     string
		pkScript []byte
		signers  []KeyDB // Sign in turn, each merging the script before.
	}{
		{"p2pkh", p2pkh, []KeyDB{kdb1}},
		{"p2pk", p2pk, []KeyDB{kdb1}},
		{"multisig", multiSig, []KeyDB{kdb1, kdb2}},
		{"p2sh multisig", p2sh, []KeyDB{kdb1, kdb2}},
		{"p2sh32 multisig", p2sh32, []KeyDB{kdb1, kdb2}},
		{"p2sh32 p2pkh", p2sh32P2PKH, []KeyDB{kdb1, kdb1}},
	}
	for _, test := range tests {
		tx, utxos := tokenSpendTx(test.pkScript)
		withUtxos := NewTxSigHashes(tx).WithUtxos(tx, utxos)

		// sign signs input idx with each signer in turn, with the passed
		// hashes, or with SignTxOutput if they are nil.
		sign := func(sigHashes *TxSigHashes, idx int,
			hashType SigHashType) ([]byte, error) {

			var script []byte
			for _, kdb := range test.signers {
				var err error
				if sigHashes == nil {
					script, err = SignTxOutput(params, tx, idx, 100000,
						test.pkScript, hashType, kdb, sdb, script)
				} else {
					script, err = SignTxOutputWithSigHashes(params, tx,
						sigHashes, idx, 100000, test.pkScript, hashType,
						kdb, sdb, script)
				}
				if err != nil {
					return nil, err
				}
			}
			return script, nil
		}
		verify := func(idx int, sigScript []byte) error {
			tx.TxIn[idx].SignatureScript = sigScript
			vm, err := NewEngine(test.pkScript, tx, idx,
				StandardVerifyFlags|ScriptAllowCashTokens, nil, nil, utxos,
				100000)
			if err != nil {
				return err
			}
			return vm.Execute()
		}

		for _, base := range []SigHashType{SigHashAll, SigHashNone, SigHashSingle} {
			for _, extra := range []SigHashType{0, SigHashAnyOneCanPay,
				SigHashUTXO, SigHashUTXO | SigHashAnyOneCanPay} {

				hashType := base | extra
				for idx := range tx.TxIn {
					msg := fmt.Sprintf("%s, hash type %#x, input %d",
						test.name, hashType, idx)
					sigScript, err := sign(withUtxos, idx, hashType)
					if extra == SigHashUTXO|SigHashAnyOneCanPay {
						if err == nil {
							t.Errorf("%s: signed with SigHashUTXO and "+
								"SigHashAnyOneCanPay", msg)
						}
						continue
					}
					if err != nil {
						t.Errorf("%s: %v", msg, err)
						continue
					}
					if err := verify(idx, sigScript); err != nil {
						t.Errorf("%s: %v", msg, err)
					}

					old, err := sign(nil, idx, hashType)
					if extra == SigHashUTXO {
						if err == nil {
							t.Errorf("%s: SignTxOutput signed with "+
								"SigHashUTXO", msg)
						}
						_, err := sign(NewTxSigHashes(tx), idx, hashType)
						if err == nil {
							t.Errorf("%s: signed with SigHashUTXO "+
								"without the spent outputs", msg)
						}
						continue
					}
					if err != nil {
						t.Errorf("%s: SignTxOutput: %v", msg, err)
						continue
					}
					if idx == 0 {
						if verify(idx, old) == nil {
							t.Errorf("%s: SignTxOutput signed the "+
								"token input validly", msg)
						}
					} else if !bytes.Equal(old, sigScript) {
						t.Errorf("%s: SignTxOutput signed differently", msg)
					}
				}
			}
		}
	}

	// The functions that sign a single signature without the hashes
	// can't sign with SigHashUTXO either.
	tx, _ := tokenSpendTx(p2pkh)
	hashType := SigHashAll | SigHashUTXO
	if _, err := RawTxInECDSASignature(tx, 1, p2pkh, hashType, key1, 100000); err == nil {
		t.Error("RawTxInECDSASignature signed with SigHashUTXO")
	}
	if _, err := RawTxInSchnorrSignature(tx, 1, p2pkh, hashType, key1, 100000); err == nil {
		t.Error("RawTxInSchnorrSignature signed with SigHashUTXO")
	}
	if _, err := SignatureScript(tx, 1, 100000, p2pkh, hashType, key1, true); err == nil {
		t.Error("SignatureScript signed with SigHashUTXO")
	}
}

// TestSignSigHashesChecks ensures the signing functions that take the
// signature hashes of a transaction return an error, instead of panicking or
// signing invalidly, for hashes computed before the transaction got another
// input or lost one, for SigHashUTXO with hashes that lack an output the
// transaction spends, and for an input whose spent output the hashes lack
// while they include the spent outputs.  It also ensures they validly sign an
// input whose spent output the hashes include while they lack another's.
func TestSignSigHashesChecks(t *testing.T) {
	t.Parallel()

	params := &chaincfg.TestNet3Params
	key, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{1}, 32))
	addr, err := bchutil.NewAddressPubKey(
		(*bchec.PublicKey)(&key.PublicKey).SerializeCompressed(), params)
	if err != nil {
		t.Fatal(err)
	}
	kdb := mkGetKey(map[string]addressToKey{
		addr.EncodeAddress():                     {key, true},
		addr.AddressPubKeyHash().EncodeAddress(): {key, true},
	})
	p2pkh, err := PayToAddrScript(addr.AddressPubKeyHash())
	if err != nil {
		t.Fatal(err)
	}
	multiSig, err := MultiSigScript([]*bchutil.AddressPubKey{addr}, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Each signs input idx of tx, which spends p2pkh, with the hashes.
	// SignTxOutputWithSigHashes signs a multisig script as well, as it
	// skips the keys it fails to sign with.
	type signFunc func(tx *wire.MsgTx, sigHashes *TxSigHashes, idx int,
		hashType SigHashType) ([]byte, error)
	signers := []struct {
		name string
		sign signFunc
	}{
		{"SignTxOutputWithSigHashes", func(tx *wire.MsgTx,
			sigHashes *TxSigHashes, idx int, hashType SigHashType) ([]byte, error) {

			return SignTxOutputWithSigHashes(params, tx, sigHashes, idx,
				100000, p2pkh, hashType, kdb, nil, nil)
		}},
		{"SignTxOutputWithSigHashes multisig", func(tx *wire.MsgTx,
			sigHashes *TxSigHashes, idx int, hashType SigHashType) ([]byte, error) {

			return SignTxOutputWithSigHashes(params, tx, sigHashes, idx,
				100000, multiSig, hashType, kdb, nil, nil)
		}},
		{"SignatureScriptWithSigHashes", func(tx *wire.MsgTx,
			sigHashes *TxSigHashes, idx int, hashType SigHashType) ([]byte, error) {

			return SignatureScriptWithSigHashes(tx, sigHashes, idx, 100000,
				p2pkh, hashType, key, true)
		}},
		{"RawTxInECDSASignatureWithSigHashes", func(tx *wire.MsgTx,
			sigHashes *TxSigHashes, idx int, hashType SigHashType) ([]byte, error) {

			return RawTxInECDSASignatureWithSigHashes(tx, sigHashes, idx,
				p2pkh, hashType, key, 100000)
		}},
		{"RawTxInSchnorrSignatureWithSigHashes", func(tx *wire.MsgTx,
			sigHashes *TxSigHashes, idx int, hashType SigHashType) ([]byte, error) {

			return RawTxInSchnorrSignatureWithSigHashes(tx, sigHashes, idx,
				p2pkh, hashType, key, 100000)
		}},
	}

	// The first input spends tokens, the second doesn't.
	tx, utxos := tokenSpendTx(p2pkh)
	first := tx.Copy()
	first.TxIn = first.TxIn[:1]
	partial := NewUtxoCache()
	entry, err := utxos.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	partial.AddEntry(1, entry)
	withPartial := NewTxSigHashes(tx).WithUtxos(tx, partial)

	tests := []struct {
		name      string
		tx        *wire.MsgTx // The one signed, tx if nil.
		sigHashes *TxSigHashes
		idx       int
		hashType  SigHashType
		valid     bool
	}{
		{"partial hashes, input whose spent output they have", nil,
			withPartial, 1, SigHashAll, true},
		{"partial hashes, SigHashUTXO", nil, withPartial, 1,
			SigHashAll | SigHashUTXO, false},
		{"partial hashes, input whose spent output they lack", nil,
			withPartial, 0, SigHashAll, false},
		{"hashes without any spent output", nil,
			NewTxSigHashes(tx).WithUtxos(tx, nil), 1, SigHashAll, false},
		{"hashes before the second input was removed", first,
			NewTxSigHashes(tx).WithUtxos(tx, utxos), 0, SigHashAll, false},
		{"hashes before the second input, first input", nil,
			NewTxSigHashes(first).WithUtxos(first, utxos), 0, SigHashAll,
			false},
		{"hashes before the second input, second input", nil,
			NewTxSigHashes(first).WithUtxos(first, utxos), 1, SigHashAll,
			false},
	}
	for _, test := range tests {
		signTx := test.tx
		if signTx == nil {
			signTx = tx
		}
		for _, signer := range signers {
			msg := fmt.Sprintf("%s: %s", test.name, signer.name)
			_, err := signer.sign(signTx, test.sigHashes, test.idx,
				test.hashType)
			if test.valid && err != nil {
				t.Errorf("%s: %v", msg, err)
			} else if !test.valid && err == nil {
				t.Errorf("%s: signed", msg)
			}
		}
		if !test.valid {
			continue
		}

		sigScript, err := signers[0].sign(signTx, test.sigHashes, test.idx,
			test.hashType)
		if err != nil {
			continue
		}
		signTx.TxIn[test.idx].SignatureScript = sigScript
		vm, err := NewEngine(p2pkh, signTx, test.idx,
			StandardVerifyFlags|ScriptAllowCashTokens, nil, nil, utxos, 100000)
		if err == nil {
			err = vm.Execute()
		}
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
	}
}

// TestSignTxOutputMerge ensures SignTxOutput keeps a signature script passed
// in previousScript for a pay-to-pubkey or pay-to-pubkey-hash script, bare or
// in P2SH or P2SH32, if it is valid, as BCHN keeps a valid signature, and
// replaces it if it isn't, also when both have the same length, as Schnorr
// signature scripts do: for a signature with another amount, by another key,
// or without the fork id, and for an extra push.  It also ensures a previous
// signature with SigHashUTXO is checked with the hashes passed in, so it is
// kept with the spent outputs and replaced without them.
func TestSignTxOutputMerge(t *testing.T) {
	t.Parallel()

	params := &chaincfg.TestNet3Params
	key1, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{1}, 32))
	key2, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{2}, 32))
	pubKey1 := (*bchec.PublicKey)(&key1.PublicKey).SerializeCompressed()
	pubKey2 := (*bchec.PublicKey)(&key2.PublicKey).SerializeCompressed()
	addr, err := bchutil.NewAddressPubKey(pubKey1, params)
	if err != nil {
		t.Fatal(err)
	}
	kdb := mkGetKey(map[string]addressToKey{
		addr.EncodeAddress():                     {key1, true},
		addr.AddressPubKeyHash().EncodeAddress(): {key1, true},
	})
	p2pkh, err := PayToAddrScript(addr.AddressPubKeyHash())
	if err != nil {
		t.Fatal(err)
	}
	p2pk, err := PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}

	// Each lock is a script the signatures are made for, bare or as the
	// redeem script of a P2SH or P2SH32 one.
	type lock struct {
		name     string
		pkScript []byte
		redeem   []byte
	}
	var locks []lock
	scripts := make(map[string][]byte)
	for _, inner := range []lock{{"p2pkh", p2pkh, nil}, {"p2pk", p2pk, nil}} {
		sh, err := bchutil.NewAddressScriptHash(inner.pkScript, params)
		if err != nil {
			t.Fatal(err)
		}
		sh32, err := bchutil.NewAddressScriptHash32(inner.pkScript, params)
		if err != nil {
			t.Fatal(err)
		}
		locks = append(locks, inner)
		for _, a := range []bchutil.Address{sh, sh32} {
			pkScript, err := PayToAddrScript(a)
			if err != nil {
				t.Fatal(err)
			}
			scripts[a.EncodeAddress()] = inner.pkScript
			name := "p2sh "
			if a == bchutil.Address(sh32) {
				name = "p2sh32 "
			}
			locks = append(locks, lock{name + inner.name, pkScript,
				inner.pkScript})
		}
	}
	sdb := mkGetScript(scripts)

	for _, l := range locks {
		// Input 1 spends no tokens.
		const idx = 1
		tx, utxos := tokenSpendTx(l.pkScript)
		withUtxos := NewTxSigHashes(tx).WithUtxos(tx, utxos)
		subScript := l.pkScript
		if l.redeem != nil {
			subScript = l.redeem
		}

		// sigScript returns the signature script of sig, after the
		// pushes of extra, with the public key for pay-to-pubkey-hash.
		sigScript := func(sig, pubKey []byte, extra ...[]byte) []byte {
			b := NewScriptBuilder()
			for _, data := range extra {
				b.AddData(data)
			}
			b.AddData(sig)
			if isPubKeyHashScript(subScript) {
				b.AddData(pubKey)
			}
			if l.redeem != nil {
				b.AddData(l.redeem)
			}
			script, err := b.Script()
			if err != nil {
				t.Fatal(err)
			}
			return script
		}
		rawSig := func(key *bchec.PrivateKey, amt int64) []byte {
			sig, err := RawTxInSchnorrSignature(tx, idx, subScript,
				SigHashAll, key, amt)
			if err != nil {
				t.Fatal(err)
			}
			return sig
		}
		sign := func(sigHashes *TxSigHashes, hashType SigHashType,
			prev []byte) []byte {

			var script []byte
			var err error
			if sigHashes == nil {
				script, err = SignTxOutput(params, tx, idx, 100000,
					l.pkScript, hashType, kdb, sdb, prev)
			} else {
				script, err = SignTxOutputWithSigHashes(params, tx,
					sigHashes, idx, 100000, l.pkScript, hashType, kdb,
					sdb, prev)
			}
			if err != nil {
				t.Fatalf("%s: %v", l.name, err)
			}
			return script
		}

		// A signature without the fork id over the hash with it, which
		// verifies but breaks the encoding rules.
		hash, _, err := CalcSignatureHash(subScript, NewTxSigHashes(tx),
			SigHashAll, tx, idx, 100000, true)
		if err != nil {
			t.Fatal(err)
		}
		noForkID, err := key1.SignSchnorr(hash)
		if err != nil {
			t.Fatal(err)
		}

		fresh := sign(nil, SigHashAll, nil)
		withUTXO := sign(withUtxos, SigHashAll|SigHashUTXO, nil)
		tests := []struct {
			name      string
			sigHashes *TxSigHashes
			prev      []byte
			keep      bool
		}{
			{"valid", nil, sign(nil, SigHashAll|SigHashAnyOneCanPay, nil),
				true},
			{"other amount", nil, sigScript(rawSig(key1, 99999), pubKey1),
				false},
			{"other key", nil, sigScript(rawSig(key2, 100000), pubKey2),
				false},
			{"no fork id", nil, sigScript(append(noForkID.Serialize(),
				byte(SigHashAll)), pubKey1), false},
			{"extra push", nil, sigScript(rawSig(key1, 100000), pubKey1,
				[]byte{0xff}), false},
			{"SigHashUTXO with the spent outputs", withUtxos, withUTXO,
				true},
			{"SigHashUTXO without them", nil, withUTXO, false},
		}
		for _, test := range tests {
			msg := fmt.Sprintf("%s, %s", l.name, test.name)
			got := sign(test.sigHashes, SigHashAll, test.prev)
			want := fresh
			if test.keep {
				want = test.prev
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s: kept %v, want %v", msg,
					bytes.Equal(got, test.prev), test.keep)
			}

			tx.TxIn[idx].SignatureScript = got
			vm, err := NewEngine(l.pkScript, tx, idx,
				StandardVerifyFlags|ScriptAllowCashTokens, nil, nil, utxos,
				100000)
			if err == nil {
				err = vm.Execute()
			}
			if err != nil {
				t.Errorf("%s: %v", msg, err)
			}
		}
	}
}

// TestSignInputIndexRange ensures the signing functions, and
// CalcSignatureHash with either algorithm, return an error for an input index
// out of range, negative ones included, instead of panicking or signing.
func TestSignInputIndexRange(t *testing.T) {
	t.Parallel()

	params := &chaincfg.TestNet3Params
	key, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{1}, 32))
	addr, err := bchutil.NewAddressPubKey(
		(*bchec.PublicKey)(&key.PublicKey).SerializeCompressed(), params)
	if err != nil {
		t.Fatal(err)
	}
	kdb := mkGetKey(map[string]addressToKey{
		addr.EncodeAddress():                     {key, true},
		addr.AddressPubKeyHash().EncodeAddress(): {key, true},
	})
	p2pkh, err := PayToAddrScript(addr.AddressPubKeyHash())
	if err != nil {
		t.Fatal(err)
	}
	p2pk, err := PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}
	multiSig, err := MultiSigScript([]*bchutil.AddressPubKey{addr}, 1)
	if err != nil {
		t.Fatal(err)
	}
	scriptAddr, err := bchutil.NewAddressScriptHash(multiSig, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh, err := PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}
	sdb := mkGetScript(map[string][]byte{scriptAddr.EncodeAddress(): multiSig})
	tx, utxos := tokenSpendTx(p2pkh)
	withUtxos := NewTxSigHashes(tx).WithUtxos(tx, utxos)

	type signFunc func(idx int, hashType SigHashType) error
	funcs := []struct {
		name string
		sign signFunc
	}{
		{"RawTxInECDSASignature", func(idx int, hashType SigHashType) error {
			_, err := RawTxInECDSASignature(tx, idx, p2pkh, hashType, key,
				100000)
			return err
		}},
		{"RawTxInSchnorrSignature", func(idx int, hashType SigHashType) error {
			_, err := RawTxInSchnorrSignature(tx, idx, p2pkh, hashType, key,
				100000)
			return err
		}},
		{"RawTxInECDSASignatureWithSigHashes", func(idx int,
			hashType SigHashType) error {

			_, err := RawTxInECDSASignatureWithSigHashes(tx, withUtxos, idx,
				p2pkh, hashType, key, 100000)
			return err
		}},
		{"RawTxInSchnorrSignatureWithSigHashes", func(idx int,
			hashType SigHashType) error {

			_, err := RawTxInSchnorrSignatureWithSigHashes(tx, withUtxos, idx,
				p2pkh, hashType, key, 100000)
			return err
		}},
		{"SignatureScript", func(idx int, hashType SigHashType) error {
			_, err := SignatureScript(tx, idx, 100000, p2pkh, hashType, key,
				true)
			return err
		}},
		{"SignatureScriptWithSigHashes", func(idx int,
			hashType SigHashType) error {

			_, err := SignatureScriptWithSigHashes(tx, withUtxos, idx, 100000,
				p2pkh, hashType, key, true)
			return err
		}},
		{"LegacyTxInSignature", func(idx int, hashType SigHashType) error {
			_, err := LegacyTxInSignature(tx, idx, p2pkh, hashType, key)
			return err
		}},
		{"LegacySignatureScript", func(idx int, hashType SigHashType) error {
			_, err := LegacySignatureScript(tx, idx, p2pkh, hashType, key,
				true)
			return err
		}},
		{"CalcSignatureHash", func(idx int, hashType SigHashType) error {
			_, _, err := CalcSignatureHash(p2pkh, withUtxos,
				hashType|SigHashForkID, tx, idx, 100000, true)
			return err
		}},
		{"CalcSignatureHash legacy", func(idx int, hashType SigHashType) error {
			_, _, err := CalcSignatureHash(p2pkh, nil, hashType, tx, idx,
				100000, false)
			return err
		}},
	}
	for _, s := range []struct {
		name     string
		pkScript []byte
	}{{"p2pkh", p2pkh}, {"p2pk", p2pk}, {"multisig", multiSig},
		{"p2sh multisig", p2sh}} {

		funcs = append(funcs, struct {
			name string
			sign signFunc
		}{"SignTxOutput " + s.name, func(idx int, hashType SigHashType) error {
			_, err := SignTxOutput(params, tx, idx, 100000, s.pkScript,
				hashType, kdb, sdb, nil)
			return err
		}}, struct {
			name string
			sign signFunc
		}{"SignTxOutputWithSigHashes " + s.name, func(idx int,
			hashType SigHashType) error {

			_, err := SignTxOutputWithSigHashes(params, tx, withUtxos, idx,
				100000, s.pkScript, hashType, kdb, sdb, nil)
			return err
		}})
	}

	for _, f := range funcs {
		for _, idx := range []int{-1, -2, len(tx.TxIn)} {
			for _, hashType := range []SigHashType{SigHashAll,
				SigHashAll | SigHashAnyOneCanPay, SigHashSingle} {

				msg := fmt.Sprintf("%s, input %d, hash type %#x", f.name,
					idx, hashType)
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("%s: panicked: %v", msg, r)
						}
					}()
					if err := f.sign(idx, hashType); err == nil {
						t.Errorf("%s: no error", msg)
					}
				}()
			}
		}
	}
}

// TestSignInvalidPubKey ensures SignTxOutput returns an error, instead of
// panicking, for a pay-to-pubkey script, bare or in P2SH, whose public key
// has a valid size but isn't on the curve, so no key can sign for it.
func TestSignInvalidPubKey(t *testing.T) {
	t.Parallel()

	params := &chaincfg.TestNet3Params
	pubKey := make([]byte, 33)
	pubKey[0] = 0x02
	for pubKey[32] = 1; pubKey[32] != 0; pubKey[32]++ {
		if _, err := bchec.ParsePubKey(pubKey, bchec.S256()); err != nil {
			break
		}
	}
	if _, err := bchec.ParsePubKey(pubKey, bchec.S256()); err == nil {
		t.Fatal("no public key off the curve")
	}
	p2pk, err := NewScriptBuilder().AddData(pubKey).AddOp(OP_CHECKSIG).Script()
	if err != nil {
		t.Fatal(err)
	}
	if class := GetScriptClass(p2pk); class != PubKeyTy {
		t.Fatalf("script class %v", class)
	}
	scriptAddr, err := bchutil.NewAddressScriptHash(p2pk, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh, err := PayToAddrScript(scriptAddr)
	if err != nil {
		t.Fatal(err)
	}
	sdb := mkGetScript(map[string][]byte{scriptAddr.EncodeAddress(): p2pk})

	for _, pkScript := range [][]byte{p2pk, p2sh} {
		tx, _ := tokenSpendTx(pkScript)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%x: panicked: %v", pkScript, r)
				}
			}()
			_, err := SignTxOutput(params, tx, 1, 100000, pkScript,
				SigHashAll, mkGetKey(nil), sdb, nil)
			if err == nil {
				t.Errorf("%x: signed", pkScript)
			}
		}()
	}
}
