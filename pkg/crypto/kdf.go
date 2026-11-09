package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters tuned for phones as well as desktops.
const (
	argonTime    = 2
	argonMemory  = 48 * 1024 // KiB => 48 MiB
	argonThreads = 1
	argonKeyLen  = 32
)

// DeriveNetworkPSK derives the 32-byte pre-shared key for a network from
// its ID and password. Same inputs always derive the same key.
func DeriveNetworkPSK(networkID, password string) [argonKeyLen]byte {
	salt := sha256.Sum256([]byte("devhive-tunnel/psk/v1|" + networkID))
	var key [argonKeyLen]byte
	copy(key[:], argon2.IDKey([]byte(password), salt[:], argonTime, argonMemory, argonThreads, argonKeyLen))
	return key
}

// NewNetworkID returns a new typeable network ID like "dh-K4XqZ2mMT9".
func NewNetworkID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return IDPrefix + "-" + base62(b)
}

// NewAdminKey returns a random admin access key, base62, 15 chars.
func NewAdminKey() string {
	b := make([]byte, 15)
	_, _ = rand.Read(b)
	return base62(b)
}

// NewGMKey returns a longer hub superuser (GM) login key, 24 chars.
func NewGMKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base62(b)
}

// HashAdminKey hashes an admin key for safe storage on the hub.
func HashAdminKey(key string) string {
	h := sha256.Sum256([]byte("devhive-tunnel/admin/v1|" + key))
	return base64.RawStdEncoding.EncodeToString(h[:])
}

// HashPSK produces a storable verifier hash for a network pre-shared key.
func HashPSK(psk [argonKeyLen]byte) string {
	h := sha256.Sum256(append([]byte("devhive-tunnel/psk-hash/v1|"), psk[:]...))
	return base64.RawStdEncoding.EncodeToString(h[:])
}

// PSKMatches checks a supplied network password against a stored HSAN hash.
func PSKMatches(networkID, password, storedHash string) bool {
	return hmac.Equal([]byte(HashPSK(DeriveNetworkPSK(networkID, password))), []byte(storedHash))
}

// CheckAdminKey compares an admin key against a stored hash.
func CheckAdminKey(key, hash string) bool {
	return hmac.Equal([]byte(HashAdminKey(key)), []byte(hash))
}

// ConstantTimeEquals compares two strings without timing leaks.
func ConstantTimeEquals(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

const base62chars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func base62(b []byte) string {
	var sb strings.Builder
	for _, x := range b {
		sb.WriteByte(base62chars[int(x)%len(base62chars)])
	}
	return sb.String()
}

// NewSessionToken issues random tokens for MCP/dashboard logins.
func NewSessionToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
