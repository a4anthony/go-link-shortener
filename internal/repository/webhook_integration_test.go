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

func TestWebhookRepository_CRUDAndDispatchLookup(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	hooks := repository.NewWebhookRepository(pool)

	tenant, err := tenants.Create(ctx, "acme")
	require.NoError(t, err)

	wh := &domain.Webhook{
		TenantID: tenant.ID, URL: "https://hooks.example.com", Secret: "whsec_x",
		Events: []domain.EventType{domain.EventLinkCreated, domain.EventLinkClicked},
	}
	require.NoError(t, hooks.Create(ctx, wh))
	assert.True(t, wh.Active)
	require.Len(t, wh.Events, 2)

	list, err := hooks.ListByTenant(ctx, tenant.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Dispatch lookup returns subscribers for the event.
	created, err := hooks.ListActiveForEvent(ctx, tenant.ID, domain.EventLinkCreated)
	require.NoError(t, err)
	require.Len(t, created, 1)
	assert.Equal(t, wh.ID, created[0].ID)

	// No subscribers for an event the webhook isn't registered to would be empty;
	// here both events are registered, so link.clicked also matches.
	clicked, err := hooks.ListActiveForEvent(ctx, tenant.ID, domain.EventLinkClicked)
	require.NoError(t, err)
	assert.Len(t, clicked, 1)

	require.NoError(t, hooks.DeleteForTenant(ctx, tenant.ID, wh.ID))
	assert.ErrorIs(t, hooks.DeleteForTenant(ctx, tenant.ID, wh.ID), domain.ErrNotFound)
}

func TestWebhookRepository_FailureDeadLetters(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	hooks := repository.NewWebhookRepository(pool)

	tenant, err := tenants.Create(ctx, "acme")
	require.NoError(t, err)
	wh := &domain.Webhook{
		TenantID: tenant.ID, URL: "https://x.com", Secret: "s",
		Events: []domain.EventType{domain.EventLinkCreated},
	}
	require.NoError(t, hooks.Create(ctx, wh))

	const threshold = 3
	// First two failures keep it active.
	for i := 0; i < threshold-1; i++ {
		disabled, err := hooks.RecordFailure(ctx, wh.ID, threshold)
		require.NoError(t, err)
		assert.False(t, disabled)
	}
	// The threshold-th failure dead-letters it.
	disabled, err := hooks.RecordFailure(ctx, wh.ID, threshold)
	require.NoError(t, err)
	assert.True(t, disabled)

	// A dead-lettered webhook is no longer an active subscriber.
	active, err := hooks.ListActiveForEvent(ctx, tenant.ID, domain.EventLinkCreated)
	require.NoError(t, err)
	assert.Empty(t, active)

	// Success resets the failure counter (but does not re-enable).
	require.NoError(t, hooks.RecordSuccess(ctx, wh.ID))
	got, err := hooks.GetByIDForTenant(ctx, tenant.ID, wh.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, got.FailureCount)
}

func TestWebhookRepository_TenantIsolation(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	hooks := repository.NewWebhookRepository(pool)

	tenantA, err := tenants.Create(ctx, "a")
	require.NoError(t, err)
	tenantB, err := tenants.Create(ctx, "b")
	require.NoError(t, err)

	whB := &domain.Webhook{TenantID: tenantB.ID, URL: "https://b.com", Secret: "s", Events: []domain.EventType{domain.EventLinkCreated}}
	require.NoError(t, hooks.Create(ctx, whB))

	_, err = hooks.GetByIDForTenant(ctx, tenantA.ID, whB.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.ErrorIs(t, hooks.DeleteForTenant(ctx, tenantA.ID, whB.ID), domain.ErrNotFound)

	listA, err := hooks.ListByTenant(ctx, tenantA.ID)
	require.NoError(t, err)
	assert.Empty(t, listA)
}
