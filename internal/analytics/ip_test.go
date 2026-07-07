package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnonymizeIP(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"ipv4 masks last octet", "203.0.113.42", "203.0.113.0"},
		{"ipv4 already zero", "10.1.2.0", "10.1.2.0"},
		{"ipv6 masks host bits", "2001:db8:1234:5678:9abc:def0:1234:5678", "2001:db8:1234::"},
		{"invalid", "not-an-ip", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnonymizeIP(tt.in)
			if tt.want == "" {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestHashIP(t *testing.T) {
	// Same IP + salt is stable; masking means the last octet does not change it.
	h1 := HashIP("203.0.113.42", "salt")
	h2 := HashIP("203.0.113.99", "salt")
	assert.Equal(t, h1, h2, "IPs in the same /24 hash identically after masking")
	assert.Len(t, h1, 64)

	// Different salt changes the hash.
	assert.NotEqual(t, h1, HashIP("203.0.113.42", "other-salt"))

	// Different subnet changes the hash.
	assert.NotEqual(t, h1, HashIP("198.51.100.42", "salt"))

	// Unparseable IP yields empty.
	assert.Equal(t, "", HashIP("garbage", "salt"))
}
