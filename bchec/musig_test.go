package bchec

import (
	crand "crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
	"math/rand"
	"testing"
)

var errTestRandomness = errors.New("randomness unavailable")

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errTestRandomness
}

func TestMuSession(t *testing.T) {
	m := sha256.Sum256([]byte("hello world"))

	for range 32 {
		r := rand.Intn(9) + 1
		sessions := make([]*Session, r)
		privkeys := make([]*PrivateKey, r)
		pubkeys := make([]*PublicKey, r)
		commitments := make([][]byte, r)
		nonces := make([]*PublicKey, r)
		svals := make([]*big.Int, r)

		for x := range r {
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

		for x := range r {
			var b [32]byte
			if _, err := crand.Read(b[:]); err != nil {
				t.Fatal(err)
			}

			sess, err := NewMuSession(pubkeys, privkeys[x], b)
			if err != nil {
				t.Fatal(err)
			}
			sessions[x] = sess
			commitments[x] = sess.NonceCommitment(m[:])
		}

		for x := range r {
			sessions[x].SetNonceCommitments(commitments...)
			nonces[x], err = sessions[x].Nonce()
			if err != nil {
				t.Fatal(err)
			}
		}

		for x := range r {
			_ = sessions[x].SetNonces(nonces...)
			svals[x], err = sessions[x].Sign(m[:])
			if err != nil {
				t.Fatal(err)
			}
		}

		for x := range r {
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

	for range 32 {
		privkeys := make([]*PrivateKey, 3)
		pubkeys := make([]*PublicKey, 3)

		for x := range 3 {
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

func TestSignMuSigRandomnessError(t *testing.T) {
	priv, err := NewPrivateKey(S256())
	if err != nil {
		t.Fatal(err)
	}

	originalReader := crand.Reader
	crand.Reader = errorReader{}
	t.Cleanup(func() {
		crand.Reader = originalReader
	})

	_, err = SignMuSig(make([]byte, 32), priv)
	if !errors.Is(err, errTestRandomness) {
		t.Fatalf("expected randomness error, got %v", err)
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
		_, _ = AggregatePublicKeys(priv1.PubKey(), priv2.PubKey(), priv3.PubKey())
	}
}
