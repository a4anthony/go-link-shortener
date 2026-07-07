package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	k1, err := GenerateAPIKey()
	require.NoError(t, err)
	k2, err := GenerateAPIKey()
	require.NoError(t, err)

	// Well-formed and prefixed.
	assert.True(t, strings.HasPrefix(k1.Plaintext, KeyPrefix))
	assert.True(t, ValidKeyFormat(k1.Plaintext))

	// Unique across calls.
	assert.NotEqual(t, k1.Plaintext, k2.Plaintext)
	assert.NotEqual(t, k1.Hash, k2.Hash)

	// Hash matches recomputation; prefix is derived from the plaintext.
	assert.Equal(t, HashAPIKey(k1.Plaintext), k1.Hash)
	assert.Equal(t, PrefixOf(k1.Plaintext), k1.Prefix)
	assert.True(t, strings.HasPrefix(k1.Plaintext, k1.Prefix))
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	const key = "sk_live_abcdef0123456789"
	assert.Equal(t, HashAPIKey(key), HashAPIKey(key))
	assert.NotEqual(t, HashAPIKey(key), HashAPIKey(key+"x"))
	assert.Len(t, HashAPIKey(key), 64) // hex-encoded sha256
}

func TestValidKeyFormat(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"valid", "sk_live_abcdefghijklmnop", true},
		{"wrong prefix", "sk_test_abcdefghijklmnop", false},
		{"no prefix", "abcdefghijklmnop", false},
		{"just prefix", KeyPrefix, false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidKeyFormat(tt.token))
		})
	}
}

func TestConstantTimeHashEqual(t *testing.T) {
	h := HashAPIKey("sk_live_something")
	assert.True(t, ConstantTimeHashEqual(h, h))
	assert.False(t, ConstantTimeHashEqual(h, HashAPIKey("sk_live_other")))
	assert.False(t, ConstantTimeHashEqual(h, "short"))
}
