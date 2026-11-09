// Package crypto implements devhive-tunnel cryptography:
// Ed25519 identities, X25519 handshakes, ChaCha20-Poly1305 AEAD sessions,
// and Argon2id network password derivation (WireGuard-style).
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const IDPrefix = "dh"

// DHResultLen is the size of a Curve25519 public key or DH result.
const DHResultLen = curve25519.PointSize

// ErrBadHandshake marks any malformed or rejected handshake message.
var ErrBadHandshake = errors.New("devhive: bad handshake")

// HKDFSalt is the global protocol salt.
const HKDFSalt = "devhive-tunnel/hkdf/v1"

// Identity is a long-term node identity: Ed25519 for signing and
// X25519 for key agreement, both derived deterministically from one seed.
type Identity struct {
	Seed  [32]byte
	sign  ed25519.PrivateKey
	xPriv [DHResultLen]byte
	xPub  [DHResultLen]byte
}

// NewIdentity generates a fresh random identity.
func NewIdentity() (*Identity, error) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	return NewIdentityFromSeed(seed)
}

// NewIdentityFromSeed reconstructs a deterministic identity from its seed.
func NewIdentityFromSeed(seed [32]byte) (*Identity, error) {
	id := &Identity{Seed: seed}
	id.sign = ed25519.NewKeyFromSeed(seed[:])

	priv, err := hkdfSha256(seed[:], "devhive-tunnel/x25519/v1", DHResultLen)
	if err != nil {
		return nil, err
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	copy(id.xPriv[:], priv)
	copy(id.xPub[:], pub)
	return id, nil
}

// Sign signs msg with Ed25519 and returns the 64-byte signature.
func (id *Identity) Sign(msg []byte) []byte { return ed25519.Sign(id.sign, msg) }

// SignPub returns the Ed25519 public key.
func (id *Identity) SignPub() ed25519.PublicKey {
	pk, _ := id.sign.Public().(ed25519.PublicKey)
	return pk
}

// XPub returns the X25519 static public key.
func (id *Identity) XPub() [DHResultLen]byte { return id.xPub }

// SharedSecret computes the X25519 shared secret with a peer static key.
func (id *Identity) SharedSecret(peerX [DHResultLen]byte) ([DHResultLen]byte, error) {
	var out [DHResultLen]byte
	s, err := curve25519.X25519(id.xPriv[:], peerX[:])
	if err != nil {
		return out, fmt.Errorf("devhive: weak peer key")
	}
	copy(out[:], s)
	return out, nil
}

// Fingerprint is a short stable printable ID derived from the identity.
func (id *Identity) Fingerprint() string { return FingerprintOfPub(id.SignPub()) }

// FingerprintOfPub derives a short fingerprint from an Ed25519 public key.
func FingerprintOfPub(pk ed25519.PublicKey) string {
	d := sha256.Sum256(pk)
	return hex.EncodeToString(d[:6])
}

// SaveSeedHexText returns the seed encoded to hex for persistence.
func (id *Identity) SaveSeedHexText() string { return hex.EncodeToString(id.Seed[:]) }

// LoadSeedHexText parses a persisted hex seed.
func LoadSeedHexText(s string) (*Identity, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("devhive: bad identity seed")
	}
	var seed [32]byte
	copy(seed[:], b)
	return NewIdentityFromSeed(seed)
}

// FriendlyName gives a stable, print-friendly peer alias from a public key.
func FriendlyName(pk ed25519.PublicKey) string {
	if len(pk) < 8 {
		return base64.RawURLEncoding.EncodeToString(pk)
	}
	return base64.RawURLEncoding.EncodeToString(pk)[:8]
}

func hkdfSha256(secret []byte, info string, n int) ([]byte, error) {
	out := make([]byte, n)
	h := hkdf.New(sha256.New, secret, []byte(HKDFSalt), []byte(info))
	if _, err := h.Read(out); err != nil {
		return nil, err
	}
	return out, nil
}
