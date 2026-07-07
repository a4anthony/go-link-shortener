//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/repository"
	"github.com/a4anthony/go-link-shortener/internal/testutil"
)

func TestRedisRateLimiter_SlidingWindow(t *testing.T) {
	rdb, _ := testutil.Redis(t)
	ctx := context.Background()
	limiter := repository.NewRedisRateLimiter(rdb)

	const limit = 3
	window := 500 * time.Millisecond
	key := "tenant-abc"

	// First `limit` requests are allowed, remaining counts down.
	for i := 0; i < limit; i++ {
		allowed, remaining, _, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i)
		assert.Equal(t, limit-i-1, remaining)
	}

	// The next request is denied with a positive retry-after.
	allowed, _, retryAfter, err := limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Greater(t, retryAfter, time.Duration(0))

	// A different key has its own independent budget.
	allowed, _, _, err = limiter.Allow(ctx, "other-tenant", limit, window)
	require.NoError(t, err)
	assert.True(t, allowed)

	// After the window slides past, the original key is allowed again.
	time.Sleep(window + 100*time.Millisecond)
	allowed, _, _, err = limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "window should have slid; request allowed again")
}
