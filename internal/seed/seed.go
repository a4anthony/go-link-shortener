// Package seed provisions the shared demo ("playground") tenant. It always runs
// in dev mode so the service is instantly demoable, and can be enabled in prod
// via SEED_DEMO_TENANT for keyless portfolio deployments: the web console
// defaults to the well-known demo key, so visitors can use the service without
// creating an account. Prod deployments that seed the playground should pair it
// with the demo link-TTL cap and janitor so the shared tenant stays bounded.
package seed

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// DemoTenantName is the name of the seeded demo tenant.
const DemoTenantName = "Demo Tenant"

// DemoAPIKey is the fixed, well-known API key of the demo tenant. It is
// deterministic so the same key works across restarts and can be baked into the
// web console as its default. It is kept deliberately short and hyphenated so
// it cannot be mistaken for a real Stripe live key by secret scanners
// (production keys are long and random).
const DemoAPIKey = "sk_live_demo-seed-key" //nolint:gosec // intentionally public demo-playground key

// TenantStore is the tenant persistence the seeder needs.
type TenantStore interface {
	FindByName(ctx context.Context, name string) (*domain.Tenant, error)
	Create(ctx context.Context, name string) (*domain.Tenant, error)
}

// APIKeyStore is the API-key persistence the seeder needs.
type APIKeyStore interface {
	GetByHash(ctx context.Context, hash string) (*domain.APIKey, error)
	Create(ctx context.Context, tenantID uuid.UUID, name, prefix, hash string) (*domain.APIKey, error)
}

// Demo ensures the demo tenant and its API key exist, logs the key, and returns
// the tenant so callers can attach per-tenant demo policies. It is idempotent:
// repeated runs converge on the same tenant and key.
func Demo(ctx context.Context, tenants TenantStore, keys APIKeyStore, log *slog.Logger) (*domain.Tenant, error) {
	tenant, err := tenants.FindByName(ctx, DemoTenantName)
	if errors.Is(err, domain.ErrNotFound) {
		tenant, err = tenants.Create(ctx, DemoTenantName)
	}
	if err != nil {
		return nil, err
	}

	hash := domain.HashAPIKey(DemoAPIKey)
	if _, err := keys.GetByHash(ctx, hash); errors.Is(err, domain.ErrNotFound) {
		if _, err := keys.Create(ctx, tenant.ID, "demo-seed", domain.PrefixOf(DemoAPIKey), hash); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	log.Warn("demo tenant ready — use this API key to authenticate",
		"tenant_id", tenant.ID.String(),
		"api_key", DemoAPIKey,
	)
	return tenant, nil
}
