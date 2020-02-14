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

		aggPubkey := AggregatePublicKeys(pubkeys...)

		for x := 0; x < r; x++ {
			sess, err := NewMuSession(pubkeys, privkeys[x])
			if err != nil {
				t.Fatal(err)
			}
			sessions[x] = sess
			nonces[x] = sess.Nonce()
		}

		for x := 0; x < r; x++ {
			sessions[x].SetNonces(nonces...)
			svals[x] = sessions[x].Sign(m[:])
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
