package seed

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeTenants struct {
	byName  map[string]*domain.Tenant
	creates int
}

func (f *fakeTenants) FindByName(_ context.Context, name string) (*domain.Tenant, error) {
	if t, ok := f.byName[name]; ok {
		return t, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeTenants) Create(_ context.Context, name string) (*domain.Tenant, error) {
	f.creates++
	t := &domain.Tenant{ID: uuid.New(), Name: name}
	f.byName[name] = t
	return t, nil
}

type fakeKeys struct {
	byHash  map[string]*domain.APIKey
	creates int
}

func (f *fakeKeys) GetByHash(_ context.Context, hash string) (*domain.APIKey, error) {
	if k, ok := f.byHash[hash]; ok {
		return k, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeKeys) Create(_ context.Context, tenantID uuid.UUID, name, prefix, hash string) (*domain.APIKey, error) {
	f.creates++
	k := &domain.APIKey{ID: uuid.New(), TenantID: tenantID, Name: name, Prefix: prefix, Hash: hash}
	f.byHash[hash] = k
	return k, nil
}

func discardLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func TestDev_CreatesOnFirstRun(t *testing.T) {
	tenants := &fakeTenants{byName: map[string]*domain.Tenant{}}
	keys := &fakeKeys{byHash: map[string]*domain.APIKey{}}

	require.NoError(t, Dev(context.Background(), tenants, keys, discardLogger()))
	assert.Equal(t, 1, tenants.creates)
	assert.Equal(t, 1, keys.creates)

	// The seeded key hashes to the well-known dev key.
	_, ok := keys.byHash[domain.HashAPIKey(DevAPIKey)]
	assert.True(t, ok)
	assert.True(t, domain.ValidKeyFormat(DevAPIKey), "dev key must be a valid API key format")
}

func TestDev_Idempotent(t *testing.T) {
	tenants := &fakeTenants{byName: map[string]*domain.Tenant{}}
	keys := &fakeKeys{byHash: map[string]*domain.APIKey{}}

	require.NoError(t, Dev(context.Background(), tenants, keys, discardLogger()))
	require.NoError(t, Dev(context.Background(), tenants, keys, discardLogger()))

	assert.Equal(t, 1, tenants.creates, "second run must not recreate the tenant")
	assert.Equal(t, 1, keys.creates, "second run must not recreate the key")
}
