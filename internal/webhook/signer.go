// Package webhook delivers event notifications to tenant-registered endpoints
// with HMAC-signed payloads, retries (exponential backoff + jitter), and a
// dead-letter state after repeated failures.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignatureHeader carries the payload signature; TimestampHeader and EventHeader
// carry delivery metadata.
const (
	SignatureHeader = "X-Webhook-Signature"
	TimestampHeader = "X-Webhook-Timestamp"
	EventHeader     = "X-Webhook-Event"
	IDHeader        = "X-Webhook-Id"
)

// Sign returns the HMAC-SHA256 signature of payload keyed by secret, formatted
// as "sha256=<hex>". Receivers recompute this over the raw body to verify
// authenticity.
func Sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether signature matches payload under secret, using a
// constant-time comparison.
func Verify(secret string, payload []byte, signature string) bool {
	expected := Sign(secret, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}
