package bchec

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"sort"
)

// AggregatePublicKeys aggregates the given public keys using
// the MuSig aggregating protocol.
func AggregatePublicKeys(keys ...*PublicKey) (*PublicKey, error) {
	sortPubkeys(keys)
	tweak := computeTweak(keys...)
	return aggregatePubkeys(tweak, keys...)
}

// SignMuSig creates a MuSig aggregate signature for the provided message
// hash using the provided private keys.
func SignMuSig(hash []byte, keys ...*PrivateKey) (*Signature, error) {
	sessionID := make([]byte, 32)
	rand.Read(sessionID)

	// Sort the private keys by their corresponding public keys
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].PubKey().X.Cmp(keys[j].PubKey().X) < 0
	})

	pubkeys := make([]*PublicKey, 0, len(keys))
	noncePrivkeys := make([]*PrivateKey, 0, len(keys))
	noncePubkeys := make([]*PublicKey, 0, len(keys))
	sVals := make([]*big.Int, len(keys))
	for _, key := range keys {
		pubkeys = append(pubkeys, key.PubKey())
	}

	tweak := computeTweak(pubkeys...)
	aggregatePubkey, err := aggregatePubkeys(tweak, pubkeys...)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		noncePriv := createNonceKeyPair(key, pubkeys, sessionID, hash)
		noncePrivkeys = append(noncePrivkeys, noncePriv)
		noncePubkeys = append(noncePubkeys, noncePriv.PubKey())
	}

	aggregateNoncePubkey, err := calculateAggregateNonce(noncePubkeys...)
	if err != nil {
		return nil, err
	}

	for i, key := range keys {
		aggNonce, _ := ParsePubKey(aggregateNoncePubkey.SerializeUncompressed(), S256())
		sVals[i] = calculateSignature(hash, aggregatePubkey, aggNonce, key, noncePrivkeys[i], tweak)
	}

	return calculateAggregateSignature(aggregateNoncePubkey, sVals...), nil
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
	aggregatePubkey  *PublicKey
	pubkeys          []*PublicKey
	privKey          *PrivateKey
	noncePriv        *PrivateKey
	aggregateNonce   *PublicKey
	sessionID        [32]byte
	nonceCommitments [][]byte
	tweak            []byte
}

// NewMuSession gets instantiated with the public keys of each participant and
// the private key of this specific user. The session ID must either be purely
// random or a counter that is incremented for every session using the same
// private key. The choice is left up to the user.
func NewMuSession(pubKeys []*PublicKey, privKey *PrivateKey, sessionID [32]byte) (*Session, error) {
	sortPubkeys(pubKeys)
	tweak := computeTweak(pubKeys...)
	agg, err := aggregatePubkeys(tweak, pubKeys...)
	if err != nil {
		return nil, err
	}

	return &Session{
		aggregatePubkey: agg,
		pubkeys:         pubKeys,
		privKey:         privKey,
		sessionID:       sessionID,
		tweak:           tweak,
	}, nil
}

// AggregatePublicKey returns the aggregate public key for this session.
func (sess *Session) AggregatePublicKey() *PublicKey {
	return sess.aggregatePubkey
}

// NonceCommitment deterministically generates the nonce and returns
// the hash. The nonce private key is derived from the private key,
// each public key in the session, the message, and the session ID.
func (sess *Session) NonceCommitment(message []byte) []byte {
	sess.noncePriv = createNonceKeyPair(sess.privKey, sess.pubkeys, sess.sessionID[:], message)
	h := sha256.Sum256(sess.noncePriv.PubKey().SerializeCompressed())
	return h[:]
}

func createNonceKeyPair(privKey *PrivateKey, pubKeys []*PublicKey, sessionID, message []byte) *PrivateKey {
	preimage := privKey.Serialize()
	for _, pubkey := range pubKeys {
		preimage = append(preimage, pubkey.SerializeCompressed()...)
	}
	preimage = append(preimage, message...)
	preimage = append(preimage, sessionID...)
	r := sha256.Sum256(preimage)

	priv, _ := PrivKeyFromBytes(S256(), r[:])
	return priv
}

// Nonce returns the nonce public key for this session.
func (sess *Session) Nonce() (*PublicKey, error) {
	if sess.nonceCommitments == nil {
		return nil, errors.New("nonce commitments must be set before revealing the nonce")
	}
	return sess.noncePriv.PubKey(), nil
}

// SetNonceCommitments saves the nonce commitments in the session. We
// use them to check the hash of the nonce against the these commitments
// when SetNonces is called.
func (sess *Session) SetNonceCommitments(nonceCommitments ...[]byte) {
	sess.nonceCommitments = nonceCommitments
}

// SetNonces saves the nonces for each peer. This should be called by each
// participant after the nonces have been shared.
func (sess *Session) SetNonces(noncePubkeys ...*PublicKey) error {
	if len(noncePubkeys) != len(sess.nonceCommitments) {
		return errors.New("nonce public keys must be the same length of nonce commitments")
	}
	if noncePubkeys[0] == nil {
		return errors.New("noncePubkey is nil")
	}
	for i, pubkey := range noncePubkeys {
		h := sha256.Sum256(pubkey.SerializeCompressed())
		if !bytes.Equal(sess.nonceCommitments[i], h[:]) {
			return fmt.Errorf("key %d does not match the commitment", i)
		}
	}

	aggregateNoncePubkey, err := calculateAggregateNonce(noncePubkeys...)
	if err != nil {
		return err
	}

	sess.aggregateNonce = aggregateNoncePubkey
	return nil
}

func calculateAggregateNonce(noncePubkeys ...*PublicKey) (*PublicKey, error) {
	aggregateNoncePubkey := *noncePubkeys[0]
	for _, pubkey := range noncePubkeys[1:] {
		if pubkey == nil {
			return nil, errors.New("noncePubkey is nil")
		}
		aggregateNoncePubkey.X, aggregateNoncePubkey.Y = aggregateNoncePubkey.Curve.Add(aggregateNoncePubkey.X, aggregateNoncePubkey.Y, pubkey.X, pubkey.Y)
	}
	return &aggregateNoncePubkey, nil
}

// Sign returns the S value for this node. Technically we don't need to return the
// R value as it's calculated by each node using the nonce public keys.
func (sess *Session) Sign(hash []byte) (*big.Int, error) {
	if sess.aggregatePubkey == nil || sess.aggregateNonce == nil || sess.privKey == nil || sess.noncePriv == nil {
		return nil, errors.New("state not fully set")
	}
	return calculateSignature(hash, sess.aggregatePubkey, sess.aggregateNonce, sess.privKey, sess.noncePriv, sess.tweak), nil
}

func calculateSignature(hash []byte, aggregatePubkey, aggregateNonce *PublicKey, privKey, noncePriv *PrivateKey, tweak []byte) *big.Int {
	// If R's y coordinate has jacobi symbol -1, then all parties negate k and R_i
	r := new(big.Int).SetBytes(noncePriv.Serialize())
	if big.Jacobi(aggregateNonce.Y, S256().P) == -1 {
		aggregateNonce.Y.Neg(aggregateNonce.Y)
		r.Neg(r)
	}

	// Compute scalar e = Hash(AggregateNoncePubkey.x || AggregatePubkey || m) mod N
	eBytes := sha256.Sum256(append(append(padIntBytes(aggregateNonce.X), aggregatePubkey.SerializeCompressed()...), hash...))
	e := new(big.Int).SetBytes(eBytes[:])
	e.Mod(e, aggregatePubkey.Params().N)

	// Compute x =  Hash(L || Pubkey) * privkey
	tweaki := sha256.Sum256(append(tweak, privKey.PubKey().SerializeCompressed()...))

	x := new(big.Int).SetBytes(privKey.Serialize())
	x = x.Mul(x, new(big.Int).SetBytes(tweaki[:]))

	// Compute s = (r + e * x) mod N
	s := e.Mul(e, x)
	s.Add(s, r)
	s.Mod(s, S256().N)
	return s
}

// AggregateSignature aggregates the S and R values and returns a signature
// that is value for the aggregate public key.
func (sess *Session) AggregateSignature(svals ...*big.Int) *Signature {
	return calculateAggregateSignature(sess.aggregateNonce, svals...)
}

func calculateAggregateSignature(aggregateNonce *PublicKey, svals ...*big.Int) *Signature {
	s := new(big.Int)

	for _, v := range svals {
		s.Add(s, v)
	}
	s.Mod(s, S256().N)

	return &Signature{
		R:       aggregateNonce.X,
		S:       s,
		sigType: SignatureTypeSchnorr,
	}
}

func sortPubkeys(keys []*PublicKey) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].X.Cmp(keys[j].X) < 0
	})
}
