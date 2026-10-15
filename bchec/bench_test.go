// Copyright 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchec

import (
	"testing"
)

// BenchmarkScalarBaseMult benchmarks the secp256k1 curve ScalarBaseMult
// function.
func BenchmarkScalarBaseMult(b *testing.B) {
	k := fromHex("d74bf844b0862475103d96a611cf2d898447e288d34b360bc885cb8ce7c00575")
	curve := S256()
	for b.Loop() {
		curve.ScalarBaseMult(k.Bytes())
	}
}

// BenchmarkScalarBaseMultLarge benchmarks the secp256k1 curve ScalarBaseMult
// function with abnormally large k values.
func BenchmarkScalarBaseMultLarge(b *testing.B) {
	k := fromHex("d74bf844b0862475103d96a611cf2d898447e288d34b360bc885cb8ce7c005751111111011111110")
	curve := S256()
	for b.Loop() {
		curve.ScalarBaseMult(k.Bytes())
	}
}

// BenchmarkScalarMult benchmarks the secp256k1 curve ScalarMult function.
func BenchmarkScalarMult(b *testing.B) {
	x := fromHex("34f9460f0e4f08393d192b3c5133a6ba099aa0ad9fd54ebccfacdfa239ff49c6")
	y := fromHex("0b71ea9bd730fd8923f6d25a7a91e7dd7728a960686cb5a901bb419e0f2ca232")
	k := fromHex("d74bf844b0862475103d96a611cf2d898447e288d34b360bc885cb8ce7c00575")
	curve := S256()
	for b.Loop() {
		curve.ScalarMult(x, y, k.Bytes())
	}
}

// BenchmarkNAF benchmarks the NAF function.
func BenchmarkNAF(b *testing.B) {
	k := fromHex("d74bf844b0862475103d96a611cf2d898447e288d34b360bc885cb8ce7c00575")
	for b.Loop() {
		NAF(k.Bytes())
	}
}

// BenchmarkECDSASigVerify benchmarks how long it takes the secp256k1 curve to
// verify signatures.
func BenchmarkECDSASigVerify(b *testing.B) {

	// Randomly generated keypair.
	// Private key: 9e0699c91ca1e3b7e3c9ba71eb71c89890872be97576010fe593fbf3fd57e66d
	pubKey := PublicKey{
		Curve: S256(),
		X:     fromHex("d2e670a19c6d753d1a6d8b20bd045df8a08fb162cf508956c31268c6d81ffdab"),
		Y:     fromHex("ab65528eefbb8057aa85d597258a3fbd481a24633bc9b47a9aa045c91371de52"),
	}

	// Double sha256 of []byte{0x01, 0x02, 0x03, 0x04}
	msgHash := fromHex("8de472e2399610baaa7f84840547cd409434e31f5d3bd71e4d947f283874f9c0")
	sig := Signature{
		R:       fromHex("fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c"),
		S:       fromHex("d47563f52aac6b04b55de236b7c515eb9311757db01e02cff079c3ca6efb063f"),
		sigType: SignatureTypeECDSA,
	}

	if !sig.Verify(msgHash.Bytes(), &pubKey) {
		b.Errorf("Signature failed to verify")
		return
	}

	for b.Loop() {
		sig.Verify(msgHash.Bytes(), &pubKey)
	}
}

// BenchmarkSchnorrSigVerify benchmarks how long it takes the secp256k1 curve to
// verify signatures.
func BenchmarkSchnorrSigVerify(b *testing.B) {

	// Randomly generated keypair.
	// Private key: 9e0699c91ca1e3b7e3c9ba71eb71c89890872be97576010fe593fbf3fd57e66d
	pubKey := PublicKey{
		Curve: S256(),
		X:     fromHex("d2e670a19c6d753d1a6d8b20bd045df8a08fb162cf508956c31268c6d81ffdab"),
		Y:     fromHex("ab65528eefbb8057aa85d597258a3fbd481a24633bc9b47a9aa045c91371de52"),
	}

	// Double sha256 of []byte{0x01, 0x02, 0x03, 0x04}
	msgHash := fromHex("8de472e2399610baaa7f84840547cd409434e31f5d3bd71e4d947f283874f9c0")
	sig := Signature{
		R:       fromHex("a4769f67e977cb196bd241d8513924af353ad944667dcc40d277c697a6489b93"),
		S:       fromHex("d57f7444ffa09464ef6f49c0a036fb90ac985e89ef89b087b4139f91f53fac6f"),
		sigType: SignatureTypeSchnorr,
	}

	if !sig.Verify(msgHash.Bytes(), &pubKey) {
		b.Errorf("Signature failed to verify")
		return
	}

	for b.Loop() {
		sig.Verify(msgHash.Bytes(), &pubKey)
	}
}

// BenchmarkFieldNormalize benchmarks how long it takes the internal field
// to perform normalization (which includes modular reduction).
func BenchmarkFieldNormalize(b *testing.B) {
	// The normalize function is constant time so default value is fine.
	f := new(fieldVal)
	for b.Loop() {
		f.Normalize()
	}
}

// benchmarkParsePubKey benchmarks parsing the public key of the private key
// used in the signature benchmarks in the given encoding.
func benchmarkParsePubKey(b *testing.B, compressed bool) {
	pubKey := PublicKey{
		Curve: S256(),
		X:     fromHex("d2e670a19c6d753d1a6d8b20bd045df8a08fb162cf508956c31268c6d81ffdab"),
		Y:     fromHex("ab65528eefbb8057aa85d597258a3fbd481a24633bc9b47a9aa045c91371de52"),
	}
	serialized := pubKey.SerializeUncompressed()
	if compressed {
		serialized = pubKey.SerializeCompressed()
	}
	for b.Loop() {
		if _, err := ParsePubKey(serialized, S256()); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParsePubKeyCompressed benchmarks parsing a compressed public key.
func BenchmarkParsePubKeyCompressed(b *testing.B) {
	benchmarkParsePubKey(b, true)
}

// BenchmarkParsePubKeyUncompressed benchmarks parsing an uncompressed public
// key.
func BenchmarkParsePubKeyUncompressed(b *testing.B) {
	benchmarkParsePubKey(b, false)
}
