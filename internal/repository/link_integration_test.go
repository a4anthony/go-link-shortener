//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/repository"
	"github.com/a4anthony/go-link-shortener/internal/testutil"
)

func newTenant(t *testing.T, repo *repository.TenantRepository, name string) uuid.UUID {
	t.Helper()
	tenant, err := repo.Create(context.Background(), name)
	require.NoError(t, err)
	return tenant.ID
}

func TestLinkRepository_CRUD(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	tenant := newTenant(t, tenants, "acme")

	link := &domain.Link{TenantID: tenant, Code: "abc1234", TargetURL: "https://example.com", RedirectType: 302}
	require.NoError(t, links.Create(ctx, link))
	assert.NotEqual(t, uuid.Nil, link.ID)

	got, err := links.GetByIDForTenant(ctx, tenant, link.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", got.TargetURL)

	byCode, err := links.GetByCode(ctx, "abc1234")
	require.NoError(t, err)
	assert.Equal(t, link.ID, byCode.ID)

	// Update.
	link.TargetURL = "https://updated.com"
	require.NoError(t, links.Update(ctx, link))
	got, err = links.GetByIDForTenant(ctx, tenant, link.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://updated.com", got.TargetURL)

	// Soft delete hides the link but frees the code.
	require.NoError(t, links.SoftDelete(ctx, tenant, link.ID))
	_, err = links.GetByCode(ctx, "abc1234")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	exists, err := links.ExistsByCode(ctx, "abc1234")
	require.NoError(t, err)
	assert.False(t, exists, "deleted code should be reusable")
}

func TestLinkRepository_DuplicateCodeConflict(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	tenantA := newTenant(t, tenants, "a")
	tenantB := newTenant(t, tenants, "b")

	require.NoError(t, links.Create(ctx, &domain.Link{TenantID: tenantA, Code: "shared", TargetURL: "https://a.com", RedirectType: 302}))

	// Codes are globally unique among live links, even across tenants.
	err := links.Create(ctx, &domain.Link{TenantID: tenantB, Code: "shared", TargetURL: "https://b.com", RedirectType: 302})
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestLinkRepository_CrossTenantIsolation(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	tenantA := newTenant(t, tenants, "a")
	tenantB := newTenant(t, tenants, "b")

	linkB := &domain.Link{TenantID: tenantB, Code: "bcode00", TargetURL: "https://b.com", RedirectType: 302}
	require.NoError(t, links.Create(ctx, linkB))

	// Tenant A cannot read, update, or delete tenant B's link.
	_, err := links.GetByIDForTenant(ctx, tenantA, linkB.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	spoof := *linkB
	spoof.TenantID = tenantA
	spoof.TargetURL = "https://hijacked.com"
	assert.ErrorIs(t, links.Update(ctx, &spoof), domain.ErrNotFound)

	assert.ErrorIs(t, links.SoftDelete(ctx, tenantA, linkB.ID), domain.ErrNotFound)

	// Tenant A's listing never includes tenant B's link.
	list, err := links.ListByTenant(ctx, tenantA, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestLinkRepository_ExpiryAndExhaustion(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	tenant := newTenant(t, tenants, "acme")

	past := time.Now().Add(-time.Hour)
	max := int64(5)
	link := &domain.Link{
		TenantID: tenant, Code: "exp0000", TargetURL: "https://x.com",
		RedirectType: 301, ExpiresAt: &past, MaxClicks: &max,
	}
	require.NoError(t, links.Create(ctx, link))

	got, err := links.GetByCode(ctx, "exp0000")
	require.NoError(t, err)
	require.NotNil(t, got.ExpiresAt)
	assert.True(t, got.IsExpired(time.Now()))
	require.NotNil(t, got.MaxClicks)
	assert.Equal(t, int64(5), *got.MaxClicks)
}
