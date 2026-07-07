//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/repository"
	"github.com/a4anthony/go-link-shortener/internal/service"
	"github.com/a4anthony/go-link-shortener/internal/testutil"
)

// TestRedirectFlow_CacheBackfillAndInvalidation exercises the full redirect hot
// path against real Postgres and Redis: cold miss + backfill, warm hit, and
// cache invalidation on delete.
func TestRedirectFlow_CacheBackfillAndInvalidation(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	rdb, _ := testutil.Redis(t)
	ctx := context.Background()

	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	cache := repository.NewLinkCache(rdb, time.Hour, 30*time.Second)

	tenant, err := tenants.Create(ctx, "acme")
	require.NoError(t, err)
	require.NoError(t, links.Create(ctx, &domain.Link{
		TenantID: tenant.ID, Code: "hot123", TargetURL: "https://example.com", RedirectType: 302,
	}))

	redirects := service.NewRedirectService(links, cache, nil, testutil.Logger())

	// Cold: cache miss, DB fallback, backfill.
	got, err := redirects.Resolve(ctx, "hot123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", got.TargetURL)

	// The entry is now cached: a direct cache read is a positive hit.
	cached, found, err := cache.Get(ctx, "hot123")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, cached)
	assert.Equal(t, "https://example.com", cached.TargetURL)

	// Warm hit resolves the same target.
	got, err = redirects.Resolve(ctx, "hot123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", got.TargetURL)

	// Invalidate (as the link service does on delete) and confirm eviction.
	require.NoError(t, cache.Invalidate(ctx, "hot123"))
	_, found, err = cache.Get(ctx, "hot123")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestRedirectFlow_NegativeCache(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	rdb, _ := testutil.Redis(t)
	ctx := context.Background()

	links := repository.NewLinkRepository(pool)
	cache := repository.NewLinkCache(rdb, time.Hour, 30*time.Second)
	redirects := service.NewRedirectService(links, cache, nil, testutil.Logger())

	_, err := redirects.Resolve(ctx, "ghost")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// A negative marker is now cached (authoritative "not found").
	_, found, err := cache.Get(ctx, "ghost")
	require.NoError(t, err)
	assert.True(t, found, "missing code should be negatively cached")
}
