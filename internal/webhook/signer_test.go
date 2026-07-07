package webhook

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignAndVerify(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"type":"link.created","data":{"code":"abc"}}`)

	sig := Sign(secret, payload)
	assert.True(t, strings.HasPrefix(sig, "sha256="))
	assert.True(t, Verify(secret, payload, sig))
}

func TestSign_Deterministic(t *testing.T) {
	assert.Equal(t, Sign("s", []byte("body")), Sign("s", []byte("body")))
}

func TestVerify_Rejects(t *testing.T) {
	payload := []byte("body")
	sig := Sign("secret", payload)

	assert.False(t, Verify("wrong-secret", payload, sig), "wrong secret")
	assert.False(t, Verify("secret", []byte("tampered"), sig), "tampered payload")
	assert.False(t, Verify("secret", payload, "sha256=deadbeef"), "wrong signature")
}
