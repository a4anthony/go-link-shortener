//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/repository"
	"github.com/a4anthony/go-link-shortener/internal/testutil"
)

// TestCrossTenantIsolation_APIKeys proves that a resource created under one
// tenant is invisible to another tenant through the tenant-scoped repository
// methods — cross-tenant access is impossible, not merely discouraged.
func TestCrossTenantIsolation_APIKeys(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()

	tenants := repository.NewTenantRepository(pool)
	keys := repository.NewAPIKeyRepository(pool)

	tenantA, err := tenants.Create(ctx, "Tenant A")
	require.NoError(t, err)
	tenantB, err := tenants.Create(ctx, "Tenant B")
	require.NoError(t, err)

	genA, err := domain.GenerateAPIKey()
	require.NoError(t, err)
	keyA, err := keys.Create(ctx, tenantA.ID, "a-key", genA.Prefix, genA.Hash)
	require.NoError(t, err)

	genB, err := domain.GenerateAPIKey()
	require.NoError(t, err)
	keyB, err := keys.Create(ctx, tenantB.ID, "b-key", genB.Prefix, genB.Hash)
	require.NoError(t, err)

	t.Run("tenant sees only its own keys", func(t *testing.T) {
		listA, err := keys.ListByTenant(ctx, tenantA.ID)
		require.NoError(t, err)
		require.Len(t, listA, 1)
		assert.Equal(t, keyA.ID, listA[0].ID)

		listB, err := keys.ListByTenant(ctx, tenantB.ID)
		require.NoError(t, err)
		require.Len(t, listB, 1)
		assert.Equal(t, keyB.ID, listB[0].ID)
	})

	t.Run("fetching another tenant's key by id is not found", func(t *testing.T) {
		// Tenant A tries to read Tenant B's key by its real id.
		_, err := keys.GetByIDForTenant(ctx, tenantA.ID, keyB.ID)
		assert.ErrorIs(t, err, domain.ErrNotFound)

		// The same id scoped to the owning tenant resolves fine.
		got, err := keys.GetByIDForTenant(ctx, tenantB.ID, keyB.ID)
		require.NoError(t, err)
		assert.Equal(t, keyB.ID, got.ID)
	})

	t.Run("auth lookup by hash still resolves the correct tenant", func(t *testing.T) {
		got, err := keys.GetByHash(ctx, genA.Hash)
		require.NoError(t, err)
		assert.Equal(t, tenantA.ID, got.TenantID)
	})
}
