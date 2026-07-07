// Package seed provisions development fixtures. In dev mode it ensures a demo
// tenant and a well-known API key exist so the service is instantly demoable,
// and prints the key to the logs.
package seed

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// DevTenantName is the name of the seeded demo tenant.
const DevTenantName = "Demo Tenant"

// DevAPIKey is the fixed development API key. It is deterministic so the same
// key works across restarts; it is only ever created in dev mode. It is kept
// deliberately short and hyphenated so it cannot be mistaken for a real Stripe
// live key by secret scanners (production keys are long and random).
const DevAPIKey = "sk_live_demo-seed-key" //nolint:gosec // fixed dev-only demo key, never seeded in prod

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

// Dev ensures the demo tenant and its API key exist, then logs the key. It is
// idempotent: repeated runs converge on the same tenant and key.
func Dev(ctx context.Context, tenants TenantStore, keys APIKeyStore, log *slog.Logger) error {
	tenant, err := tenants.FindByName(ctx, DevTenantName)
	if errors.Is(err, domain.ErrNotFound) {
		tenant, err = tenants.Create(ctx, DevTenantName)
	}
	if err != nil {
		return err
	}

	hash := domain.HashAPIKey(DevAPIKey)
	if _, err := keys.GetByHash(ctx, hash); errors.Is(err, domain.ErrNotFound) {
		if _, err := keys.Create(ctx, tenant.ID, "dev-seed", domain.PrefixOf(DevAPIKey), hash); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	log.Warn("dev seed ready — use this API key to authenticate",
		"tenant_id", tenant.ID.String(),
		"api_key", DevAPIKey,
	)
	return nil
}
