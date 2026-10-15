// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchec

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"slices"
	"strings"
	"testing"
)

// goldenVector is a test vector in testdata/golden.json.gz.  The vectors were
// generated with this package's original pure Go field and point arithmetic.
// Big integers are encoded as hex without padding and byte strings as hex.
type goldenVector struct {
	Op  string   `json:"op"`
	In  []string `json:"in"`
	Out []string `json:"out,omitempty"`
	Err string   `json:"err,omitempty"`
	Ok  bool     `json:"ok,omitempty"`
}

func loadGoldenVectors(t *testing.T) []goldenVector {
	t.Helper()
	f, err := os.Open("testdata/golden.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var vecs []goldenVector
	dec := json.NewDecoder(zr)
	for {
		var v goldenVector
		if err := dec.Decode(&v); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		vecs = append(vecs, v)
	}
	return vecs
}

func goldenBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func goldenInt(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("bad hex integer " + s)
	}
	return v
}

func goldenPad32(v *big.Int) []byte {
	return paddedAppend(32, nil, v.Bytes())
}

func goldenErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestGoldenVectors checks parsing, verification, signing, key recovery,
// curve operations, ECMH, MuSig and ECDH against the golden vectors.
func TestGoldenVectors(t *testing.T) {
	curve := S256()
	vecs := loadGoldenVectors(t)
	if len(vecs) == 0 {
		t.Fatal("no vectors")
	}
	for i, v := range vecs {
		var out []string
		var errStr string
		var ok bool
		in := v.In
		pt := func(x, y *big.Int) []string { return []string{x.Text(16), y.Text(16)} }
		switch v.Op {
		case "pubkey":
			pk, err := ParsePubKey(goldenBytes(in[0]), curve)
			if errStr = goldenErr(err); err == nil {
				out = pt(pk.X, pk.Y)
			}
		case "der", "ber", "schnorrsig":
			b := goldenBytes(in[0])
			var sig *Signature
			var err error
			switch v.Op {
			case "der":
				sig, err = ParseDERSignature(b, curve)
			case "ber":
				sig, err = ParseBERSignature(b, curve)
			default:
				sig, err = ParseSchnorrSignature(b)
			}
			if errStr = goldenErr(err); err == nil {
				out = []string{sig.R.Text(16), sig.S.Text(16),
					hex.EncodeToString(sig.Serialize())}
			}
		case "ecdsa", "schnorr":
			pub := &PublicKey{Curve: curve, X: goldenInt(in[1]), Y: goldenInt(in[2])}
			r, s := goldenInt(in[3]), goldenInt(in[4])
			sig := &Signature{R: r, S: s}
			if v.Op == "schnorr" {
				sig, _ = ParseSchnorrSignature(append(goldenPad32(r), goldenPad32(s)...))
			}
			ok = sig.Verify(goldenBytes(in[0]), pub)
		case "sign":
			priv, _ := PrivKeyFromBytes(curve, goldenBytes(in[0]))
			hash := goldenBytes(in[1])
			esig, err := priv.SignECDSA(hash)
			ssig, _ := priv.SignSchnorr(hash)
			out = []string{"", hex.EncodeToString(ssig.Serialize())}
			if errStr = goldenErr(err); err == nil {
				out[0] = hex.EncodeToString(esig.Serialize())
			}
		case "signcompact":
			priv, _ := PrivKeyFromBytes(curve, goldenBytes(in[0]))
			cs, err := SignCompact(curve, priv, goldenBytes(in[1]), in[2] == "01")
			if errStr = goldenErr(err); err == nil {
				out = []string{hex.EncodeToString(cs)}
			}
		case "recover":
			pk, comp, err := RecoverCompact(curve, goldenBytes(in[0]), goldenBytes(in[1]))
			if errStr = goldenErr(err); err == nil {
				out = append(pt(pk.X, pk.Y), map[bool]string{false: "00", true: "01"}[comp])
			}
		case "sbmult":
			out = pt(curve.ScalarBaseMult(goldenBytes(in[0])))
		case "privkey":
			priv, _ := PrivKeyFromBytes(curve, goldenBytes(in[0]))
			out = pt(priv.X, priv.Y)
		case "smult":
			out = pt(curve.ScalarMult(goldenInt(in[0]), goldenInt(in[1]), goldenBytes(in[2])))
		case "add":
			out = pt(curve.Add(goldenInt(in[0]), goldenInt(in[1]), goldenInt(in[2]),
				goldenInt(in[3])))
		case "double":
			out = pt(curve.Double(goldenInt(in[0]), goldenInt(in[1])))
		case "oncurve":
			ok = curve.IsOnCurve(goldenInt(in[0]), goldenInt(in[1]))
		case "hashtopoint":
			out = pt(hashToPoint(curve, goldenBytes(in[0])))
		case "ecmh":
			ms := NewMultiset(curve)
			for _, step := range in {
				parts := strings.Split(step, ":")
				switch parts[0] {
				case "a":
					ms.Add(goldenBytes(parts[1]))
				case "r":
					ms.Remove(goldenBytes(parts[1]))
				case "m":
					ms.Merge(NewMultisetFromPoint(curve, goldenInt(parts[1]),
						goldenInt(parts[2])))
				}
				h := ms.Hash()
				out = append(out, hex.EncodeToString(h[:]))
			}
			out = append(out, pt(ms.Point())...)
		case "musigagg":
			keys := make([]*PublicKey, 0, len(in)/2)
			for j := 0; j < len(in); j += 2 {
				keys = append(keys, &PublicKey{Curve: curve, X: goldenInt(in[j]),
					Y: goldenInt(in[j+1])})
			}
			agg2, _ := aggregatePubkeys(nil, slices.Clone(keys)...)
			agg, _ := AggregatePublicKeys(keys...)
			out = append(pt(agg.X, agg.Y), pt(agg2.X, agg2.Y)...)
		case "musig":
			hash := goldenBytes(in[0])
			sessionID := [32]byte(goldenBytes(in[1]))
			count := len(in) - 2
			privs := make([]*PrivateKey, count)
			pubs := make([]*PublicKey, count)
			for j := range count {
				privs[j], pubs[j] = PrivKeyFromBytes(curve, goldenBytes(in[2+j]))
			}
			agg, _ := AggregatePublicKeys(pubs...)
			out = pt(agg.X, agg.Y)
			sessions := make([]*Session, count)
			commitments := make([][]byte, count)
			for j := range count {
				sessions[j], _ = NewMuSession(pubs, privs[j], sessionID)
				commitments[j] = sessions[j].NonceCommitment(hash)
				out = append(out, hex.EncodeToString(commitments[j]))
			}
			nonces := make([]*PublicKey, count)
			for j := range count {
				sessions[j].SetNonceCommitments(commitments...)
				nonces[j], _ = sessions[j].Nonce()
			}
			svals := make([]*big.Int, count)
			for j := range count {
				if err := sessions[j].SetNonces(nonces...); err != nil {
					t.Fatal(err)
				}
				svals[j], _ = sessions[j].Sign(hash)
				out = append(out, svals[j].Text(16))
			}
			sig := sessions[0].AggregateSignature(svals...)
			out = append(out, hex.EncodeToString(sig.Serialize()))
		case "ecdh":
			priv, _ := PrivKeyFromBytes(curve, goldenBytes(in[0]))
			pub := &PublicKey{Curve: curve, X: goldenInt(in[1]), Y: goldenInt(in[2])}
			out = []string{hex.EncodeToString(GenerateSharedSecret(priv, pub))}
		case "decrypt":
			priv, _ := PrivKeyFromBytes(curve, goldenBytes(in[0]))
			plain, err := Decrypt(priv, goldenBytes(in[1]))
			if errStr = goldenErr(err); err == nil {
				out = []string{hex.EncodeToString(plain)}
			}
		default:
			t.Fatalf("vector %d: unknown op %q", i, v.Op)
		}
		if errStr != v.Err || ok != v.Ok || !slices.Equal(out, v.Out) {
			t.Fatalf("vector %d (%s %v) mismatch:\ngot  %v %q %v\nwant %v %q %v",
				i, v.Op, in, out, errStr, ok, v.Out, v.Err, v.Ok)
		}
	}
}

// FuzzSignVerifyRoundTrip checks that signatures made with a key derived from
// the input verify, and that keys and signatures survive a serialize and
// parse round trip.
func FuzzSignVerifyRoundTrip(f *testing.F) {
	f.Add([]byte{1}, []byte("hash"), uint8(0))
	f.Add(bytes.Repeat([]byte{0xff}, 32), make([]byte, 32), uint8(1))
	f.Fuzz(func(t *testing.T, key, hash []byte, flip uint8) {
		curve := S256()
		k := new(big.Int).SetBytes(key)
		k.Mod(k, curve.N)
		if k.Sign() == 0 {
			return
		}
		priv, pub := PrivKeyFromBytes(curve, goldenPad32(k))
		for _, enc := range [][]byte{pub.SerializeCompressed(),
			pub.SerializeUncompressed(), pub.SerializeHybrid()} {

			parsed, err := ParsePubKey(enc, curve)
			if err != nil || !parsed.IsEqual(pub) {
				t.Fatalf("pubkey %x round trip failed: %v", enc, err)
			}
		}

		esig, err := priv.SignECDSA(hash)
		if err == nil {
			der := esig.Serialize()
			parsed, err := ParseDERSignature(der, curve)
			if err != nil || !bytes.Equal(parsed.Serialize(), der) {
				t.Fatalf("signature %x round trip failed: %v", der, err)
			}
			if len(hash) > 0 && !parsed.Verify(hash, pub) {
				t.Fatal("ECDSA signature rejected")
			}
		}
		ssig, err := priv.SignSchnorr(hash)
		if err != nil {
			t.Fatal(err)
		}
		raw := ssig.Serialize()
		parsed, err := ParseSchnorrSignature(raw)
		if err != nil || !bytes.Equal(parsed.Serialize(), raw) {
			t.Fatalf("schnorr signature %x round trip failed: %v", raw, err)
		}
		if !parsed.Verify(hash, pub) {
			t.Fatal("Schnorr signature rejected")
		}

		// A changed signature must not verify.
		raw[int(flip)%64] ^= 1 << (flip % 8)
		if changed, _ := ParseSchnorrSignature(raw); changed.Verify(hash, pub) {
			t.Fatal("changed Schnorr signature accepted")
		}
	})
}
