package bchec

import (
	"crypto/sha256"
	"math/big"
	"math/rand"
	"testing"
)

func TestMuSession(t *testing.T) {
	m := sha256.Sum256([]byte("hello world"))

	for i := 0; i < 32; i++ {
		r := rand.Intn(9) + 1
		sessions := make([]*Session, r)
		privkeys := make([]*PrivateKey, r)
		pubkeys := make([]*PublicKey, r)
		commitments := make([][]byte, r)
		nonces := make([]*PublicKey, r)
		svals := make([]*big.Int, r)

		for x := 0; x < r; x++ {
			priv, err := NewPrivateKey(S256())
			if err != nil {
				t.Fatal(err)
			}
			privkeys[x] = priv
			pubkeys[x] = priv.PubKey()
		}

		aggPubkey, err := AggregatePublicKeys(pubkeys...)
		if err != nil {
			t.Fatal(err)
		}

		for x := 0; x < r; x++ {
			var b [32]byte
			rand.Read(b[:])

			sess, err := NewMuSession(pubkeys, privkeys[x], b)
			if err != nil {
				t.Fatal(err)
			}
			sessions[x] = sess
			commitments[x] = sess.NonceCommitment(m[:])
		}

		for x := 0; x < r; x++ {
			sessions[x].SetNonceCommitments(commitments...)
			nonces[x], err = sessions[x].Nonce()
			if err != nil {
				t.Fatal(err)
			}
		}

		for x := 0; x < r; x++ {
			sessions[x].SetNonces(nonces...)
			svals[x], err = sessions[x].Sign(m[:])
			if err != nil {
				t.Fatal(err)
			}
		}

		for x := 0; x < r; x++ {
			sig := sessions[x].AggregateSignature(svals...)
			valid := sig.Verify(m[:], aggPubkey)
			if !valid {
				t.Fatal("invalid signature")
			}
		}
	}
}

func TestSignMuSig(t *testing.T) {
	m := sha256.Sum256([]byte("hello world"))

	for i := 0; i < 32; i++ {
		privkeys := make([]*PrivateKey, 3)
		pubkeys := make([]*PublicKey, 3)

		for x := 0; x < 3; x++ {
			priv, err := NewPrivateKey(S256())
			if err != nil {
				t.Fatal(err)
			}
			privkeys[x] = priv
			pubkeys[x] = priv.PubKey()
		}

		signature, err := SignMuSig(m[:], privkeys...)
		if err != nil {
			t.Fatal(err)
		}

		pubkey, err := AggregatePublicKeys(pubkeys...)
		if err != nil {
			t.Fatal(err)
		}

		valid := signature.Verify(m[:], pubkey)
		if !valid {
			t.Fatal("invalid signature")
		}
	}
}

func BenchmarkAggregatePublicKeys(b *testing.B) {
	priv1, err := NewPrivateKey(S256())
	if err != nil {
		b.Fatal(err)
	}
	priv2, err := NewPrivateKey(S256())
	if err != nil {
		b.Fatal(err)
	}
	priv3, err := NewPrivateKey(S256())
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		AggregatePublicKeys(priv1.PubKey(), priv2.PubKey(), priv3.PubKey())
	}
}
