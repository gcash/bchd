package bchec

import (
	"crypto/sha256"
	"errors"
	"math/big"
	"sort"
)

// AggregatePublicKeys aggregates the given public keys using
// the MuSig aggregating protocol.
func AggregatePublicKeys(keys ...*PublicKey) (*PublicKey, error) {
	lexagraphicalSortPubkeys(keys)
	tweak := computeTweak(keys...)
	return aggregatePubkeys(tweak, keys...)
}

func aggregatePubkeys(tweak []byte, keys ...*PublicKey) (*PublicKey, error) {
	if len(keys) == 0 {
		return nil, errors.New("pubkeys is nil")
	}
	k := *keys[0]

	tweak0 := sha256.Sum256(append(tweak, k.SerializeCompressed()...))

	x0, y0, z0 := S256().scalarMultJacobian(k.X, k.Y, tweak0[:])

	for _, key := range keys[1:] {
		tweaki := sha256.Sum256(append(tweak, key.SerializeCompressed()...))
		x, y, z := S256().scalarMultJacobian(key.X, key.Y, tweaki[:])
		S256().addJacobian(x, y, z, x0, y0, z0, x0, y0, z0)
	}

	x, y := S256().fieldJacobianToBigAffine(x0, y0, z0)

	return &PublicKey{
		X:     x,
		Y:     y,
		Curve: S256(),
	}, nil
}

func computeTweak(keys ...*PublicKey) []byte {
	lexagraphicalSortPubkeys(keys)

	preimage := make([]byte, 0, 33*len(keys))
	for _, key := range keys {
		preimage = append(preimage, key.SerializeCompressed()...)
	}
	tweak := sha256.Sum256(preimage)
	return tweak[:]
}

// Session represents a MuSig signing session. Each party to the singing
// needs one of these objects.
type Session struct {
	aggregatePubkey *PublicKey
	privKey         *PrivateKey
	noncePriv       *PrivateKey
	aggregateNonce  *PublicKey
	tweak           []byte
}

// NewMuSession gets instantiated with the public keys of each participant and
// the private key of this specific user.
func NewMuSession(pubKeys []*PublicKey, privKey *PrivateKey) (*Session, error) {
	lexagraphicalSortPubkeys(pubKeys)
	tweak := computeTweak(pubKeys...)
	agg, err := aggregatePubkeys(tweak, pubKeys...)
	if err != nil {
		return nil, err
	}

	priv, err := NewPrivateKey(S256())
	if err != nil {
		return nil, err
	}

	return &Session{
		aggregatePubkey: agg,
		privKey:         privKey,
		noncePriv:       priv,
		tweak:           tweak,
	}, nil
}

// AggregatePublicKey returns the aggregate public key for this session.
func (sess *Session) AggregatePublicKey() *PublicKey {
	return sess.aggregatePubkey
}

// Nonce returns the nonce public key for this session.
func (sess *Session) Nonce() *PublicKey {
	return sess.noncePriv.PubKey()
}

// NewNonce generates and saves a new nonce in case you need
// to regenerate it.
func (sess *Session) NewNonce() (*PublicKey, error) {
	priv, err := NewPrivateKey(S256())
	if err != nil {
		return nil, err
	}
	sess.noncePriv = priv
	return priv.PubKey(), nil
}

// SetNonce allows the user to generate a nonce public key outside
// of this class and import it.
//
// IMPORTANT: The key must be random and NOT derived using RFC6979
// or any other deterministic algorithm. This is to prevent a
// potential attack.
func (sess *Session) SetNonce(priv *PrivateKey) {
	sess.noncePriv = priv
}

// SetNonces saves the nonces for each peer. This should be called by each
// participant after the nonces have been shared.
func (sess *Session) SetNonces(noncePubkeys ...*PublicKey) error {
	if len(noncePubkeys) == 0 || noncePubkeys[0] == nil {
		return errors.New("noncePubkey is nil")
	}
	aggregateNoncePubkey := *noncePubkeys[0]
	for _, pubkey := range noncePubkeys[1:] {
		if pubkey == nil {
			return errors.New("noncePubkey is nil")
		}
		aggregateNoncePubkey.X, aggregateNoncePubkey.Y = aggregateNoncePubkey.Curve.Add(aggregateNoncePubkey.X, aggregateNoncePubkey.Y, pubkey.X, pubkey.Y)
	}
	sess.aggregateNonce = &aggregateNoncePubkey
	return nil
}

// Sign returns the S value for this node. Technically we don't need to return the
// R value as it's calculated by each node using the nonce public keys.
func (sess *Session) Sign(hash []byte) (*big.Int, error) {
	if sess.aggregatePubkey == nil || sess.aggregateNonce == nil || sess.privKey == nil || sess.noncePriv == nil {
		return nil, errors.New("state not fully set")
	}

	// If R's y coordinate has jacobi symbol -1, then all parties negate k and R_i
	r := new(big.Int).SetBytes(sess.noncePriv.Serialize())
	if big.Jacobi(sess.aggregateNonce.Y, S256().P) == -1 {
		sess.aggregateNonce.Y.Neg(sess.aggregateNonce.Y)
		r.Neg(r)
	}

	// Compute scalar e = Hash(AggregateNoncePubkey.x || AggregatePubkey || m) mod N
	eBytes := sha256.Sum256(append(append(padIntBytes(sess.aggregateNonce.X), sess.aggregatePubkey.SerializeCompressed()...), hash...))
	e := new(big.Int).SetBytes(eBytes[:])
	e.Mod(e, sess.aggregatePubkey.Params().N)

	// Compute x =  Hash(L || Pubkey) * privkey
	tweaki := sha256.Sum256(append(sess.tweak, sess.privKey.PubKey().SerializeCompressed()...))

	x := new(big.Int).SetBytes(sess.privKey.Serialize())
	x = x.Mul(x, new(big.Int).SetBytes(tweaki[:]))

	// Compute s = (r + e * x) mod N
	s := e.Mul(e, x)
	s.Add(s, r)
	s.Mod(s, S256().N)

	return s, nil
}

// AggregateSignature aggregates the S and R values and returns a signature
// that is value for the aggregate public key.
func (sess *Session) AggregateSignature(svals ...*big.Int) *Signature {
	s := new(big.Int)

	for _, v := range svals {
		s.Add(s, v)
	}
	s.Mod(s, S256().N)

	return &Signature{
		R:       sess.aggregateNonce.X,
		S:       s,
		sigType: SignatureTypeSchnorr,
	}
}

func lexagraphicalSortPubkeys(keys []*PublicKey) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].X.Cmp(keys[j].X) < 0
	})
}
