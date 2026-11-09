package client

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sort"

	"devhive-tunnel/pkg/crypto"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// groupSeal encrypts network-wide messages with the network PSK.
type groupSeal struct{ aead cipher.AEAD }

func newGroupSeal(networkID string, psk [32]byte) (*groupSeal, error) {
	key := make([]byte, chacha20poly1305.KeySize)
	h := hkdf.New(sha256.New, psk[:], []byte(crypto.HKDFSalt), []byte("devhive-group/v1|"+networkID))
	if _, err := h.Read(key); err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	return &groupSeal{aead: aead}, nil
}

// pairwiseSeal computes the deterministic DM/file key between two peers.
func pairwiseSeal(id *crypto.Identity, peerX [32]byte, networkID, myFP, peerFP string) (cipher.AEAD, error) {
	shared, err := id.SharedSecret(peerX)
	if err != nil {
		return nil, err
	}
	fps := []string{myFP, peerFP}
	sort.Strings(fps)
	info := "devhive-pair/v1|" + networkID + "|" + fps[0] + "|" + fps[1]
	key := make([]byte, chacha20poly1305.KeySize)
	h := hkdf.New(sha256.New, shared[:], []byte(crypto.HKDFSalt), []byte(info))
	if _, err := h.Read(key); err != nil {
		return nil, err
	}
	return chacha20poly1305.NewX(key)
}

// seal encrypts plaintext with a random XChaCha nonce prefix.
func seal(aead cipher.AEAD, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

// open decrypts a blob produced by seal.
func open(aead cipher.AEAD, blob []byte) ([]byte, error) {
	if len(blob) < aead.NonceSize() {
		return nil, fmt.Errorf("devhive: sealed blob too short")
	}
	nonce := blob[:aead.NonceSize()]
	ct := blob[aead.NonceSize():]
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("devhive: cannot decrypt message")
	}
	return pt, nil
}
