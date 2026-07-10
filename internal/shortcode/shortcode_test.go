package shortcode

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_LengthAndAlphabet(t *testing.T) {
	for _, length := range []int{4, 7, 10, 16} {
		g := NewGenerator(length)
		code, err := g.Generate()
		require.NoError(t, err)
		assert.Len(t, code, length)
		for _, r := range code {
			assert.True(t, strings.ContainsRune(alphabet, r), "char %q not in base62 alphabet", r)
		}
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	g := NewGenerator(8)
	seen := make(map[string]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		code, err := g.Generate()
		require.NoError(t, err)
		_, dup := seen[code]
		require.False(t, dup, "unexpected duplicate code %q at iteration %d", code, i)
		seen[code] = struct{}{}
	}
}

func TestGenerate_DistributionAcrossAlphabet(t *testing.T) {
	// Sanity check that generated characters cover most of the alphabet, i.e. we
	// are not stuck on a biased subset.
	g := NewGenerator(16)
	used := make(map[rune]struct{})
	for i := 0; i < 500; i++ {
		code, err := g.Generate()
		require.NoError(t, err)
		for _, r := range code {
			used[r] = struct{}{}
		}
	}
	assert.Greater(t, len(used), 55, "expected coverage of most of the 62-char alphabet")
}

func TestNewGenerator_ClampsLength(t *testing.T) {
	g := NewGenerator(0)
	assert.Equal(t, 1, g.Length())
	code, err := g.Generate()
	require.NoError(t, err)
	assert.Len(t, code, 1)
}

func TestValidAlias(t *testing.T) {
	tests := []struct {
		name  string
		alias string
		want  bool
	}{
		{"simple", "my-link", true},
		{"underscores", "my_link_2", true},
		{"alnum", "abc123", true},
		{"min length", "abc", true},
		{"too short", "ab", false},
		{"has space", "my link", false},
		{"has slash", "my/link", false},
		{"has dot", "my.link", false},
		{"reserved api", "api", false},
		{"reserved metrics", "metrics", false},
		{"reserved console links", "links", false},
		{"reserved console webhooks", "webhooks", false},
		{"reserved console settings", "settings", false},
		{"reserved console assets", "assets", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidAlias(tt.alias))
		})
	}
}

func TestValidAlias_MaxLength(t *testing.T) {
	assert.True(t, ValidAlias(strings.Repeat("a", 64)))
	assert.False(t, ValidAlias(strings.Repeat("a", 65)))
}
