// Package shortcode generates URL-safe base62 short codes and validates custom
// aliases. Collision handling is a service-layer concern (generate, check, retry);
// this package guarantees only that generated codes are uniformly random.
package shortcode

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

// alphabet is the base62 set: digits, uppercase, lowercase. Index positions are
// stable so codes are reproducible from the same byte source.
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const base = byte(len(alphabet)) // 62

// maxUnbiased is the largest multiple of base that fits in a byte. Bytes at or
// above it are rejected to remove modulo bias from the mapping to [0,62).
const maxUnbiased = (256 / 62) * 62 // 248

// aliasPattern constrains custom aliases: 3–64 chars of URL-safe characters.
var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,64}$`)

// reserved holds path segments that must never be used as an alias because they
// collide with real routes.
var reserved = map[string]struct{}{
	"api": {}, "healthz": {}, "readyz": {}, "metrics": {},
	"admin": {}, "static": {}, "favicon.ico": {}, "robots.txt": {},
	// Web console routes and its asset prefix: when the console and the
	// redirect path share one origin (production nginx), these segments belong
	// to the SPA and must never resolve as short links.
	"links": {}, "webhooks": {}, "settings": {}, "assets": {}, "going-private": {},
}

// Generator produces random base62 codes of a fixed length.
type Generator struct {
	length int
}

// NewGenerator returns a Generator producing codes of the given length. A length
// below 1 is clamped to 1.
func NewGenerator(length int) *Generator {
	if length < 1 {
		length = 1
	}
	return &Generator{length: length}
}

// Generate returns a fresh random base62 code. It draws cryptographically random
// bytes and rejection-samples them to avoid modulo bias.
func (g *Generator) Generate() (string, error) {
	out := make([]byte, g.length)
	// Over-read to amortise rejections; refill if we run short.
	buf := make([]byte, g.length*2)
	filled := 0

	for filled < g.length {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("read random bytes: %w", err)
		}
		for _, b := range buf {
			if b >= maxUnbiased {
				continue // reject to keep the distribution uniform
			}
			out[filled] = alphabet[b%base]
			filled++
			if filled == g.length {
				break
			}
		}
	}
	return string(out), nil
}

// Length returns the configured code length.
func (g *Generator) Length() int { return g.length }

// ValidAlias reports whether s is an acceptable custom alias: correct shape and
// not a reserved path.
func ValidAlias(s string) bool {
	if !aliasPattern.MatchString(s) {
		return false
	}
	_, isReserved := reserved[s]
	return !isReserved
}

// IsReserved reports whether s is a reserved path segment.
func IsReserved(s string) bool {
	_, ok := reserved[s]
	return ok
}
