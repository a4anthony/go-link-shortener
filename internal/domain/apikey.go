package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// KeyPrefix is the identifiable prefix carried by every issued API key. It is
// stored in cleartext alongside the hash so keys can be listed and referenced
// without exposing the secret.
const KeyPrefix = "sk_live_"

// prefixStoredLen is how many leading characters of the full key are persisted
// as the display prefix (e.g. "sk_live_ab12cd34").
const prefixStoredLen = len(KeyPrefix) + 8

// APIKey is a hashed-at-rest credential belonging to a tenant. The plaintext
// secret is returned exactly once, at creation time, and never stored.
type APIKey struct {
	ID         uuid.UUID  `json:"id"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Hash       string     `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// Revoked reports whether the key has been revoked and must be rejected.
func (k APIKey) Revoked() bool { return k.RevokedAt != nil }

// GeneratedKey bundles the one-time plaintext secret with the derived fields
// that get persisted.
type GeneratedKey struct {
	Plaintext string
	Prefix    string
	Hash      string
}

// GenerateAPIKey mints a new random API key. It returns the plaintext (shown to
// the caller once) plus the prefix and hash to persist.
func GenerateAPIKey() (GeneratedKey, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return GeneratedKey{}, fmt.Errorf("generate api key: %w", err)
	}
	// URL-safe base64 without padding keeps the key a clean single token.
	secret := base64.RawURLEncoding.EncodeToString(buf)
	plaintext := KeyPrefix + secret

	return GeneratedKey{
		Plaintext: plaintext,
		Prefix:    plaintext[:prefixStoredLen],
		Hash:      HashAPIKey(plaintext),
	}, nil
}

// HashAPIKey returns the hex-encoded SHA-256 of a plaintext key. Lookups compare
// against this deterministic hash; no per-key salt is used so the hash can be
// indexed for O(1) authentication.
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// PrefixOf returns the storable display prefix for a plaintext key, or the whole
// key if it is shorter than the prefix window.
func PrefixOf(plaintext string) string {
	if len(plaintext) < prefixStoredLen {
		return plaintext
	}
	return plaintext[:prefixStoredLen]
}

// ValidKeyFormat reports whether a token looks like one of our API keys. It is a
// cheap pre-check before hitting the database.
func ValidKeyFormat(token string) bool {
	return strings.HasPrefix(token, KeyPrefix) && len(token) > prefixStoredLen
}

// ConstantTimeHashEqual compares two hex hashes without leaking timing. Used
// when verifying a presented key against a stored hash in memory.
func ConstantTimeHashEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
