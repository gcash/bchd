package txscript

import (
	"bytes"
	"testing"

	"github.com/gcash/bchd/bchec"
	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
)

// TestParsePkScript ensures that the supported script types can be parsed
// correctly and re-derived into its raw byte representation.
func TestParsePkScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pkScript []byte
		valid    bool
	}{
		{
			name:     "empty output script",
			pkScript: []byte{},
			valid:    false,
		},
		{
			name: "valid P2PKH",
			pkScript: []byte{
				// OP_DUP
				0x76,
				// OP_HASH160
				0xa9,
				// OP_DATA_20
				0x14,
				// <20-byte pubkey hash>
				0xf0, 0x7a, 0xb8, 0xce, 0x72, 0xda, 0x4e, 0x76,
				0x0b, 0x74, 0x7d, 0x48, 0xd6, 0x65, 0xec, 0x96,
				0xad, 0xf0, 0x24, 0xf5,
				// OP_EQUALVERIFY
				0x88,
				// OP_CHECKSIG
				0xac,
			},
			valid: true,
		},
		// Invalid P2PKH - same as above but replaced OP_CHECKSIG with
		// OP_CHECKSIGVERIFY.
		{
			name: "invalid P2PKH",
			pkScript: []byte{
				// OP_DUP
				0x76,
				// OP_HASH160
				0xa9,
				// OP_DATA_20
				0x14,
				// <20-byte pubkey hash>
				0xf0, 0x7a, 0xb8, 0xce, 0x72, 0xda, 0x4e, 0x76,
				0x0b, 0x74, 0x7d, 0x48, 0xd6, 0x65, 0xec, 0x96,
				0xad, 0xf0, 0x24, 0xf5,
				// OP_EQUALVERIFY
				0x88,
				// OP_CHECKSIGVERIFY
				0xad,
			},
			valid: false,
		},
		{
			name: "valid P2SH",
			pkScript: []byte{
				// OP_HASH160
				0xA9,
				// OP_DATA_20
				0x14,
				// <20-byte script hash>
				0xec, 0x6f, 0x7a, 0x5a, 0xa8, 0xf2, 0xb1, 0x0c,
				0xa5, 0x15, 0x04, 0x52, 0x3a, 0x60, 0xd4, 0x03,
				0x06, 0xf6, 0x96, 0xcd,
				// OP_EQUAL
				0x87,
			},
			valid: true,
		},
		// Invalid P2SH - same as above but replaced OP_EQUAL with
		// OP_EQUALVERIFY.
		{
			name: "invalid P2SH",
			pkScript: []byte{
				// OP_HASH160
				0xA9,
				// OP_DATA_20
				0x14,
				// <20-byte script hash>
				0xec, 0x6f, 0x7a, 0x5a, 0xa8, 0xf2, 0xb1, 0x0c,
				0xa5, 0x15, 0x04, 0x52, 0x3a, 0x60, 0xd4, 0x03,
				0x06, 0xf6, 0x96, 0xcd,
				// OP_EQUALVERIFY
				0x88,
			},
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pkScript, err := ParsePkScript(test.pkScript, nil)
			switch {
			case err != nil && test.valid:
				t.Fatalf("unable to parse valid pkScript=%x: %v",
					test.pkScript, err)
			case err == nil && !test.valid:
				t.Fatalf("successfully parsed invalid pkScript=%x",
					test.pkScript)
			}

			if !test.valid {
				return
			}

			if !bytes.Equal(pkScript.Script(), test.pkScript) {
				t.Fatalf("expected to re-derive pkScript=%x, "+
					"got pkScript=%x", test.pkScript,
					pkScript.Script())
			}
		})
	}
}

// TestComputePkScript ensures that we can correctly re-derive an output's
// pkScript by looking at the input's signature script/witness attempting to
// spend it.
func TestComputePkScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sigScript []byte
		class     ScriptClass
		pkScript  []byte
	}{
		{
			name:      "empty sigScript and witness",
			sigScript: nil,
			class:     NonStandardTy,
			pkScript:  nil,
		},
		{
			name: "P2PKH sigScript",
			sigScript: []byte{
				// OP_DATA_71,
				0x47,
				// <71-byte sig>
				0x30, 0x44, 0x02, 0x20, 0x65, 0x92, 0xd8, 0x8e,
				0x1d, 0x0a, 0x4a, 0x3c, 0xc5, 0x9f, 0x92, 0xae,
				0xfe, 0x62, 0x54, 0x74, 0xa9, 0x4d, 0x13, 0xa5,
				0x9f, 0x84, 0x97, 0x78, 0xfc, 0xe7, 0xdf, 0x4b,
				0xe0, 0xc2, 0x28, 0xd8, 0x02, 0x20, 0x2d, 0xea,
				0x36, 0x96, 0x19, 0x1f, 0xb7, 0x00, 0xc5, 0xa7,
				0x7e, 0x22, 0xd9, 0xfb, 0x6b, 0x42, 0x67, 0x42,
				0xa4, 0x2c, 0xac, 0xdb, 0x74, 0xa2, 0x7c, 0x43,
				0xcd, 0x89, 0xa0, 0xf9, 0x44, 0x54, 0x01,
				// OP_DATA_33
				0x21,
				// <33-byte compressed pubkey>
				0x02, 0x7d, 0x56, 0x12, 0x09, 0x75, 0x31, 0xc2,
				0x17, 0xfd, 0xd4, 0xd2, 0xe1, 0x7a, 0x35, 0x4b,
				0x17, 0xf2, 0x7a, 0xef, 0x30, 0x9f, 0xb2, 0x7f,
				0x1f, 0x1f, 0x7b, 0x73, 0x7d, 0x9a, 0x24, 0x49,
				0x90,
			},
			class: PubKeyHashTy,
			pkScript: []byte{
				// OP_DUP
				0x76,
				// OP_HASH160
				0xa9,
				// OP_DATA_20
				0x14,
				// <20-byte pubkey hash>
				0xf0, 0x7a, 0xb8, 0xce, 0x72, 0xda, 0x4e, 0x76,
				0x0b, 0x74, 0x7d, 0x48, 0xd6, 0x65, 0xec, 0x96,
				0xad, 0xf0, 0x24, 0xf5,
				// OP_EQUALVERIFY
				0x88,
				// OP_CHECKSIG
				0xac,
			},
		},
		{
			name: "P2SH sigScript",
			sigScript: []byte{
				0x00, 0x49, 0x30, 0x46, 0x02, 0x21, 0x00, 0xda,
				0xe6, 0xb6, 0x14, 0x1b, 0xa7, 0x24, 0x4f, 0x54,
				0x62, 0xb6, 0x2a, 0x3b, 0x27, 0x59, 0xde, 0xe4,
				0x46, 0x76, 0x19, 0x4e, 0x6c, 0x56, 0x8d, 0x5b,
				0x1c, 0xda, 0x96, 0x2d, 0x4f, 0x6d, 0x79, 0x02,
				0x21, 0x00, 0xa6, 0x6f, 0x60, 0x34, 0x46, 0x09,
				0x0a, 0x22, 0x3c, 0xec, 0x30, 0x33, 0xd9, 0x86,
				0x24, 0xd2, 0x73, 0xa8, 0x91, 0x55, 0xa5, 0xe6,
				0x96, 0x66, 0x0b, 0x6a, 0x50, 0xa3, 0x46, 0x45,
				0xbb, 0x67, 0x01, 0x48, 0x30, 0x45, 0x02, 0x21,
				0x00, 0xe2, 0x73, 0x49, 0xdb, 0x93, 0x82, 0xe1,
				0xf8, 0x8d, 0xae, 0x97, 0x5c, 0x71, 0x19, 0xb7,
				0x79, 0xb6, 0xda, 0x43, 0xa8, 0x4f, 0x16, 0x05,
				0x87, 0x11, 0x9f, 0xe8, 0x12, 0x1d, 0x85, 0xae,
				0xee, 0x02, 0x20, 0x6f, 0x23, 0x2d, 0x0a, 0x7b,
				0x4b, 0xfa, 0xcd, 0x56, 0xa0, 0x72, 0xcc, 0x2a,
				0x44, 0x81, 0x31, 0xd1, 0x0d, 0x73, 0x35, 0xf9,
				0xa7, 0x54, 0x8b, 0xee, 0x1f, 0x70, 0xc5, 0x71,
				0x0b, 0x37, 0x9e, 0x01, 0x47, 0x52, 0x21, 0x03,
				0xab, 0x11, 0x5d, 0xa6, 0xdf, 0x4f, 0x54, 0x0b,
				0xd6, 0xc9, 0xc4, 0xbe, 0x5f, 0xdd, 0xcc, 0x24,
				0x58, 0x8e, 0x7c, 0x2c, 0xaf, 0x13, 0x82, 0x28,
				0xdd, 0x0f, 0xce, 0x29, 0xfd, 0x65, 0xb8, 0x7c,
				0x21, 0x02, 0x15, 0xe8, 0xb7, 0xbf, 0xfe, 0x8d,
				0x9b, 0xbd, 0x45, 0x81, 0xf9, 0xc3, 0xb6, 0xf1,
				0x6d, 0x67, 0x08, 0x36, 0xc3, 0x0b, 0xb2, 0xe0,
				0x3e, 0xfd, 0x9d, 0x41, 0x03, 0xb5, 0x59, 0xeb,
				0x67, 0xcd, 0x52, 0xae,
			},
			class: ScriptHashTy,
			pkScript: []byte{
				// OP_HASH160
				0xA9,
				// OP_DATA_20
				0x14,
				// <20-byte script hash>
				0x12, 0xd6, 0x9c, 0xd3, 0x38, 0xa3, 0x8d, 0x0d,
				0x77, 0x83, 0xcf, 0x22, 0x64, 0x97, 0x63, 0x3d,
				0x3c, 0x20, 0x79, 0xea,
				// OP_EQUAL
				0x87,
			},
		},
		// Invalid P2SH (non push-data only script).
		{
			name:      "invalid P2SH sigScript",
			sigScript: []byte{0x6b, 0x65, 0x6b}, // kek
			class:     NonStandardTy,
			pkScript:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			valid := test.pkScript != nil
			pkScript, err := ComputePkScript(test.sigScript)
			if err != nil && valid {
				t.Fatalf("unable to compute pkScript: %v", err)
			}

			if !valid {
				return
			}

			if pkScript.Class() != test.class {
				t.Fatalf("expected pkScript of type %v, got %v",
					test.class, pkScript.Class())
			}
			if !bytes.Equal(pkScript.Script(), test.pkScript) {
				t.Fatalf("expected pkScript=%x, got pkScript=%x",
					test.pkScript, pkScript.Script())
			}
		})
	}
}

// TestComputePkScriptSigShapes ensures P2PKH signature scripts are recognised
// by their signature and public key pushes and anything else is taken as
// P2SH.
func TestComputePkScriptSigShapes(t *testing.T) {
	t.Parallel()

	priv, _ := bchec.PrivKeyFromBytes(bchec.S256(), bytes.Repeat([]byte{0x01}, 32))
	pubKey := priv.PubKey().SerializeCompressed()
	uncompressed := priv.PubKey().SerializeUncompressed()
	notOnCurve := append([]byte{0x02}, bytes.Repeat([]byte{0xff}, 32)...)

	// derSig returns a DER shaped signature of n bytes with sighash byte.
	derSig := func(n int) []byte {
		sig := bytes.Repeat([]byte{0x01}, n)
		sig[0], sig[1] = 0x30, byte(n-3)
		return sig
	}
	schnorrSig := bytes.Repeat([]byte{0x01}, 65)

	pushes := func(data ...[]byte) []byte {
		b := NewScriptBuilder()
		for _, d := range data {
			b.AddData(d)
		}
		script, err := b.Script()
		if err != nil {
			t.Fatalf("unable to build script: %v", err)
		}
		return script
	}
	p2pkh := func(pk []byte) []byte {
		script, _ := payToPubKeyHashScript(hash160(pk))
		return script
	}
	p2sh := func(redeem []byte) []byte {
		script, _ := payToScriptHashScript(hash160(redeem))
		return script
	}

	tests := []struct {
		name      string
		sigScript []byte
		pkScript  []byte
	}{
		{"schnorr", pushes(schnorrSig, pubKey), p2pkh(pubKey)},
		{"ecdsa 71", pushes(derSig(71), pubKey), p2pkh(pubKey)},
		{"ecdsa 72", pushes(derSig(72), pubKey), p2pkh(pubKey)},
		{"ecdsa 73", pushes(derSig(73), pubKey), p2pkh(pubKey)},
		{"ecdsa 9", pushes(derSig(9), pubKey), p2pkh(pubKey)},
		{"schnorr uncompressed", pushes(schnorrSig, uncompressed), p2pkh(uncompressed)},
		{"ecdsa uncompressed", pushes(derSig(72), uncompressed), p2pkh(uncompressed)},
		{"ecdsa 74", pushes(derSig(74), pubKey), p2sh(pubKey)},
		{"ecdsa 8", pushes(derSig(8), pubKey), p2sh(pubKey)},
		{"not der", pushes(bytes.Repeat([]byte{0x01}, 71), pubKey), p2sh(pubKey)},
		{"not on curve", pushes(schnorrSig, notOnCurve), p2sh(notOnCurve)},
		{"three pushes", pushes(nil, schnorrSig, pubKey), p2sh(pubKey)},
		{"redeem script", pushes(schnorrSig, []byte{OP_TRUE}), p2sh([]byte{OP_TRUE})},
		{"not push only", append(pushes(schnorrSig, pubKey), OP_NOP), nil},
		{"bad push", []byte{OP_DATA_2, 0x01}, nil},
	}

	for _, test := range tests {
		pkScript, err := ComputePkScript(test.sigScript)
		if test.pkScript == nil {
			if err == nil {
				t.Errorf("%s: computed %x, want error", test.name,
					pkScript.Script())
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unable to compute pkScript: %v", test.name, err)
			continue
		}
		if !bytes.Equal(pkScript.Script(), test.pkScript) {
			t.Errorf("%s: got pkScript %x, want %x", test.name,
				pkScript.Script(), test.pkScript)
		}
	}
}

// TestPkScriptScriptHash32 ensures a P2SH32 script round trips through
// ParsePkScript and gives its address.
func TestPkScriptScriptHash32(t *testing.T) {
	t.Parallel()

	params := &chaincfg.MainNetParams
	addr, err := bchutil.NewAddressScriptHash32([]byte{OP_TRUE}, params)
	if err != nil {
		t.Fatalf("unable to create address: %v", err)
	}
	script, err := PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("unable to create script: %v", err)
	}

	pkScript, err := ParsePkScript(script, params)
	if err != nil {
		t.Fatalf("unable to parse P2SH32 script: %v", err)
	}
	if pkScript.Class() != ScriptHash32Ty {
		t.Fatalf("got class %v, want %v", pkScript.Class(), ScriptHash32Ty)
	}
	if !bytes.Equal(pkScript.Script(), script) {
		t.Fatalf("got script %x, want %x", pkScript.Script(), script)
	}
	gotAddr, err := pkScript.Address(params)
	if err != nil {
		t.Fatalf("unable to get address: %v", err)
	}
	if gotAddr.String() != addr.String() {
		t.Fatalf("got address %v, want %v", gotAddr, addr)
	}
}
