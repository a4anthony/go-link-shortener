package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// APIKeyRepository persists and looks up API keys.
type APIKeyRepository struct {
	db Querier
}

// NewAPIKeyRepository builds an APIKeyRepository over the given pool.
func NewAPIKeyRepository(db Querier) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create inserts a new API key row (hash + prefix already derived by the caller).
func (r *APIKeyRepository) Create(ctx context.Context, tenantID uuid.UUID, name, prefix, hash string) (*domain.APIKey, error) {
	const q = `
		INSERT INTO api_keys (tenant_id, name, prefix, key_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, name, prefix, key_hash, created_at, last_used_at, revoked_at`
	var k domain.APIKey
	err := r.db.QueryRow(ctx, q, tenantID, name, prefix, hash).Scan(
		&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Hash, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	return &k, nil
}

// GetByHash resolves an API key by its hash for authentication. Revoked keys are
// still returned (the caller decides), but missing keys yield ErrNotFound.
func (r *APIKeyRepository) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	const q = `
		SELECT id, tenant_id, name, prefix, key_hash, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1`
	var k domain.APIKey
	err := r.db.QueryRow(ctx, q, hash).Scan(
		&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Hash, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select api key: %w", err)
	}
	return &k, nil
}

// ListByTenant returns every API key belonging to a tenant. It is strictly
// tenant-scoped: the tenant_id predicate is not optional.
func (r *APIKeyRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.APIKey, error) {
	const q = `
		SELECT id, tenant_id, name, prefix, key_hash, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE tenant_id = $1
		ORDER BY created_at`
	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []domain.APIKey
	for rows.Next() {
		var k domain.APIKey
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Hash, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

// GetByIDForTenant loads a single API key by id, scoped to a tenant. A key that
// belongs to another tenant is indistinguishable from a missing one
// (domain.ErrNotFound) — this is the core cross-tenant isolation guarantee.
func (r *APIKeyRepository) GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.APIKey, error) {
	const q = `
		SELECT id, tenant_id, name, prefix, key_hash, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE id = $1 AND tenant_id = $2`
	var k domain.APIKey
	err := r.db.QueryRow(ctx, q, id, tenantID).Scan(
		&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Hash, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select api key for tenant: %w", err)
	}
	return &k, nil
}

// TouchLastUsed records that a key was just used to authenticate. It is
// best-effort: callers typically ignore the error to avoid failing a request on
// a bookkeeping write.
func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE api_keys SET last_used_at = now() WHERE id = $1`
	if _, err := r.db.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("touch api key: %w", err)
	}
	return nil
}
